package main

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"html/template"
	"io"
	"log"
	"log/slog"
	"net/http"
	"time"

	"github.com/jfyne/live"
)

//go:embed *.html
var content embed.FS

// UploadsState holds the state for an uploads island instance.
type UploadsState struct {
	Uploads    live.UploadContext
	Errors     []error
	SavedFiles []string
}

// NewUploadsIsland creates a new uploads island definition.
func NewUploadsIsland() (*live.Island, error) {
	island, err := live.NewIsland(
		"uploads",
		live.WithMount(func(ctx context.Context, props live.Props, children string) (any, error) {
			return &UploadsState{
				Uploads:    live.UploadContext{},
				Errors:     []error{},
				SavedFiles: []string{},
			}, nil
		}),
		live.WithUploadConfig(&live.UploadConfig{
			Name:     "photos",
			MaxFiles: 3,
			MaxSize:  1 * 1024 * 1024,
			Accept:   []string{"image/png"},
		}),
		live.WithRender(func(ctx context.Context, rc *live.IslandRenderContext) (io.Reader, error) {
			state := rc.State.(*UploadsState)

			tmpl, err := template.ParseFS(content, "uploads.html")
			if err != nil {
				return nil, err
			}

			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, state); err != nil {
				return nil, err
			}

			return &buf, nil
		}),
	)
	if err != nil {
		return nil, err
	}

	// validate: validate upload metadata from the client and store the result.
	err = island.HandleEvent("validate", func(ctx context.Context, state any, params live.Params) (any, error) {
		s := state.(*UploadsState)

		uploadCtx, err := live.ValidateUploads(params, island.UploadConfigs())
		if err != nil {
			return nil, err
		}

		s.Uploads = uploadCtx
		s.Errors = []error{}

		// Collect any per-upload validation errors into state.Errors.
		for _, uploads := range uploadCtx {
			for _, u := range uploads {
				s.Errors = append(s.Errors, u.Errors...)
			}
		}

		return s, nil
	})
	if err != nil {
		return nil, err
	}

	// save: consume the staged uploads and record file names in SavedFiles.
	err = island.HandleEvent("save", func(ctx context.Context, state any, params live.Params) (any, error) {
		s := state.(*UploadsState)

		errs := live.ConsumeUploads(s.Uploads, "photos", func(u *live.Upload) error {
			s.SavedFiles = append(s.SavedFiles, u.Name)
			return nil
		})

		if len(errs) > 0 {
			s.Errors = append(s.Errors, errs...)
		}

		// Clear the uploads context after consuming.
		s.Uploads = live.UploadContext{}

		return s, nil
	})
	if err != nil {
		return nil, err
	}

	return island, nil
}

func main() {
	// Register the uploads island with the global registry.
	err := live.RegisterIsland("uploads", NewUploadsIsland)
	if err != nil {
		log.Fatal("Failed to register uploads island:", err)
	}

	// Create context for engine lifecycle.
	ctx := context.Background()

	// Create state store with 1-minute cleanup interval.
	stateStore := live.NewMemoryIslandStateStore(ctx, 1*time.Minute)

	// Create island engine.
	registry := live.DefaultRegistry()
	engine := live.NewIslandEngine(ctx, registry, stateStore)
	defer engine.Close()

	// Set up WebSocket transport endpoint.
	wsConfig := live.DefaultTransportConfig()
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		transport, err := live.UpgradeWebSocket(r.Context(), w, r, wsConfig)
		if err != nil {
			http.Error(w, "WebSocket upgrade failed", http.StatusBadRequest)
			return
		}

		sessionID := live.SessionID(fmt.Sprintf("session-%d", time.Now().UnixNano()))
		session := live.NewSession(r.Context(), sessionID, transport)
		engine.AddSession(session)

		defer func() {
			engine.DeleteSession(sessionID)
		}()

		go func() {
			for event := range transport.Events() {
				log.Printf("Received event: type=%s, island=%s", event.T, event.Island)

				if event.T == "subscribe" && event.Island != "" {
					islandID := live.IslandID(event.Island)
					params, _ := event.Params()
					islandType := params.String("type")

					if islandType == "" {
						log.Printf("Warning: subscribe event for %s missing type, cannot mount", islandID)
						continue
					}

					_, err := engine.MountIsland(sessionID, islandID, islandType, live.Props{})
					if err != nil {
						log.Printf("Failed to mount island %s: %v", islandID, err)
					}
					continue
				}

				if err := engine.RouteEvent(sessionID, event); err != nil {
					log.Printf("Event routing error: %v", err)
				}
			}
		}()

		<-r.Context().Done()
	})

	// Set up SSE transport endpoints.
	sseConfig := live.DefaultTransportConfig()
	sseFactory := live.NewSSETransportFactory(sseConfig)

	http.HandleFunc("/live/sse", func(w http.ResponseWriter, r *http.Request) {
		transport, err := sseFactory.Upgrade(r.Context(), w, r)
		if err != nil {
			http.Error(w, "SSE upgrade failed", http.StatusBadRequest)
			return
		}

		sessionID := live.SessionID(live.GetSessionIDFromRequest(r))
		if sessionID == "" {
			sessionID = live.SessionID(fmt.Sprintf("session-%d", time.Now().UnixNano()))
		}
		session := live.NewSession(r.Context(), sessionID, transport)
		engine.AddSession(session)

		defer func() {
			engine.DeleteSession(sessionID)
		}()

		go func() {
			for event := range transport.Events() {
				log.Printf("SSE received event: type=%s, island=%s", event.T, event.Island)

				if event.T == "subscribe" && event.Island != "" {
					islandID := live.IslandID(event.Island)
					params, _ := event.Params()
					islandType := params.String("type")

					if islandType == "" {
						log.Printf("Warning: subscribe event for %s missing type, cannot mount", islandID)
						continue
					}

					_, err := engine.MountIsland(sessionID, islandID, islandType, live.Props{})
					if err != nil {
						log.Printf("Failed to mount island %s: %v", islandID, err)
					}
					continue
				}

				if err := engine.RouteEvent(sessionID, event); err != nil {
					log.Printf("Event routing error: %v", err)
				}
			}
		}()

		<-r.Context().Done()
	})

	// SSE POST handler for client-to-server events.
	http.HandleFunc("/live/post", func(w http.ResponseWriter, r *http.Request) {
		sseFactory.HandlePost(w, r)
	})

	// Serve the main HTML page.
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		tmpl, err := template.ParseFS(content, "index.html")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := tmpl.Execute(w, nil); err != nil {
			slog.Error("failed to execute template", "err", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
	})

	// Serve the v2 client library.
	http.Handle("/live.js", live.Javascript{})

	// Start server.
	addr := ":8080"
	log.Printf("Uploads example server starting on http://localhost%s", addr)
	log.Printf("Visit http://localhost%s to see the example", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatal(err)
	}
}
