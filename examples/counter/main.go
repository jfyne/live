package main

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/jfyne/live"
)

//go:embed *.html
var content embed.FS

// CounterState holds the state for a counter island instance.
type CounterState struct {
	Count int
}

// NewCounterIsland creates a new counter island definition.
func NewCounterIsland() (*live.Island, error) {
	island, err := live.NewIsland(
		"counter",
		live.WithMount(func(ctx context.Context, props live.Props, children string) (any, error) {
			// Extract initial value from props, default to 0
			initialValue := props.Int("initial-value")
			return &CounterState{Count: initialValue}, nil
		}),
		live.WithRender(func(ctx context.Context, rc *live.IslandRenderContext) (io.Reader, error) {
			state := rc.State.(*CounterState)

			// Parse and execute the counter template
			tmpl, err := template.ParseFS(content, "counter.html")
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

	// Register increment event handler
	err = island.HandleEvent("inc", func(ctx context.Context, state any, params live.Params) (any, error) {
		s := state.(*CounterState)
		s.Count++
		return s, nil
	})
	if err != nil {
		return nil, err
	}

	// Register decrement event handler
	err = island.HandleEvent("dec", func(ctx context.Context, state any, params live.Params) (any, error) {
		s := state.(*CounterState)
		s.Count--
		return s, nil
	})
	if err != nil {
		return nil, err
	}

	return island, nil
}

func main() {
	// Register the counter island with the global registry
	err := live.RegisterIsland("counter", NewCounterIsland)
	if err != nil {
		log.Fatal("Failed to register counter island:", err)
	}

	// Create context for engine lifecycle
	ctx := context.Background()

	// Create state store with 1-minute cleanup interval
	stateStore := live.NewMemoryIslandStateStore(ctx, 1*time.Minute)

	// Create island engine
	registry := live.DefaultRegistry()
	engine := live.NewIslandEngine(ctx, registry, stateStore)
	defer engine.Close()

	// Set up WebSocket transport endpoint
	wsConfig := live.DefaultTransportConfig()
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		// Upgrade to WebSocket
		transport, err := live.UpgradeWebSocket(r.Context(), w, r, wsConfig)
		if err != nil {
			http.Error(w, "WebSocket upgrade failed", http.StatusBadRequest)
			return
		}

		// Create a session ID (in production, you'd get this from cookies or generate it)
		sessionID := live.SessionID(fmt.Sprintf("session-%d", time.Now().UnixNano()))

		// Create a session with the transport
		session := live.NewSession(r.Context(), sessionID, transport)

		// Add session to engine
		engine.AddSession(session)

		// Clean up when connection closes
		defer func() {
			engine.DeleteSession(sessionID)
		}()

		// Process events from the session
		go func() {
			for event := range transport.Events() {
				log.Printf("Received event: type=%s, island=%s", event.T, event.Island)

				// Check if this is a subscribe event (island mount request)
				if event.T == "subscribe" && event.Island != "" {
					islandID := live.IslandID(event.Island)

					// Try to get props from event data
					// In the current v2 implementation, the client needs to send type and props
					params, _ := event.Params()
					islandType := params.String("type")

					if islandType == "" {
						// If type not in params, try to infer from island element attributes
						// For this example, we'll use a default or log an error
						log.Printf("Warning: subscribe event for %s missing type, cannot mount", islandID)
						continue
					}

					// Extract props from params (client should send all data-* attributes)
					props := make(live.Props)
					for key, value := range params {
						if key != "type" && key != "id" {
							props[key] = value
						}
					}

					// Mount the island
					_, err := engine.MountIsland(sessionID, islandID, islandType, props)
					if err != nil {
						log.Printf("Failed to mount island %s: %v", islandID, err)
					}
					continue
				}

				// Route the event to the appropriate island
				if err := engine.RouteEvent(sessionID, event); err != nil {
					log.Printf("Event routing error: %v", err)
				}
			}
		}()

		// Keep connection alive until context is done
		<-r.Context().Done()
	})

	// Set up SSE transport endpoints
	sseConfig := live.DefaultTransportConfig()
	sseFactory := live.NewSSETransportFactory(sseConfig)

	http.HandleFunc("/sse", func(w http.ResponseWriter, r *http.Request) {
		// Similar to WebSocket handler, but for SSE
		transport, err := sseFactory.Upgrade(r.Context(), w, r)
		if err != nil {
			http.Error(w, "SSE upgrade failed", http.StatusBadRequest)
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
				log.Printf("SSE received event: type=%s, island=%s", event.T, event.Island)

				if event.T == "subscribe" && event.Island != "" {
					islandID := live.IslandID(event.Island)
					params, _ := event.Params()
					islandType := params.String("type")

					if islandType == "" {
						log.Printf("Warning: subscribe event for %s missing type, cannot mount", islandID)
						continue
					}

					props := make(live.Props)
					for key, value := range params {
						if key != "type" && key != "id" {
							props[key] = value
						}
					}

					_, err := engine.MountIsland(sessionID, islandID, islandType, props)
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

	// SSE POST handler for client-to-server events
	http.HandleFunc("/sse/post", func(w http.ResponseWriter, r *http.Request) {
		sseFactory.HandlePost(w, r)
	})

	// Serve the main HTML page
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
		tmpl.Execute(w, nil)
	})

	// Serve the custom island script
	http.HandleFunc("/custom-island.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		http.ServeFile(w, r, "custom-island.js")
	})

	// Start server
	addr := ":8080"
	log.Printf("Counter example server starting on http://localhost%s", addr)
	log.Printf("Visit http://localhost%s to see the example", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatal(err)
	}
}
