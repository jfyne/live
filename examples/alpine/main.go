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
	"strings"
	"time"

	"github.com/jfyne/live"
)

//go:embed *.html
var content embed.FS

// Item represents a selectable item in the autocomplete list.
type Item struct {
	ID   string
	Name string
}

// Match returns true if the item's name contains the search string (case-insensitive).
func (i Item) Match(search string) bool {
	return strings.Contains(strings.ToLower(i.Name), strings.ToLower(search))
}

// AlpineState holds the state for an alpine island instance.
type AlpineState struct {
	Items       []Item
	Suggestions []Item
	Selected    []Item
}

// defaultItems is the predefined list of programming languages.
var defaultItems = []Item{
	{ID: "go", Name: "Go"},
	{ID: "javascript", Name: "JavaScript"},
	{ID: "python", Name: "Python"},
	{ID: "rust", Name: "Rust"},
	{ID: "typescript", Name: "TypeScript"},
}

// NewAlpineIsland creates a new alpine island definition.
func NewAlpineIsland() (*live.Island, error) {
	island, err := live.NewIsland(
		"alpine",
		live.WithMount(func(ctx context.Context, props live.Props, children string) (any, error) {
			return &AlpineState{Items: defaultItems}, nil
		}),
		live.WithRender(func(ctx context.Context, rc *live.IslandRenderContext) (io.Reader, error) {
			state := rc.State.(*AlpineState)

			tmpl, err := template.ParseFS(content, "alpine.html")
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

	// suggest: filter defaultItems by search string
	err = island.HandleEvent("suggest", func(ctx context.Context, state any, params live.Params) (any, error) {
		s := state.(*AlpineState)
		search := params.String("search")
		suggestions := []Item{}
		for _, item := range defaultItems {
			if item.Match(search) {
				suggestions = append(suggestions, item)
			}
		}
		s.Suggestions = suggestions
		return s, nil
	})
	if err != nil {
		return nil, err
	}

	// selected: add item to Selected list, deduplicating by ID
	err = island.HandleEvent("selected", func(ctx context.Context, state any, params live.Params) (any, error) {
		s := state.(*AlpineState)
		id := params.String("id")

		// Find item in Items list
		var found *Item
		for _, item := range s.Items {
			if item.ID == id {
				cp := item
				found = &cp
				break
			}
		}
		if found == nil {
			return s, nil
		}

		// Add to Selected if not already present (deduplicate by ID)
		for _, sel := range s.Selected {
			if sel.ID == id {
				return s, nil
			}
		}
		s.Selected = append(s.Selected, *found)
		return s, nil
	})
	if err != nil {
		return nil, err
	}

	// submit: return state unchanged
	err = island.HandleEvent("submit", func(ctx context.Context, state any, params live.Params) (any, error) {
		return state, nil
	})
	if err != nil {
		return nil, err
	}

	return island, nil
}

func main() {
	// Register the alpine island with the global registry
	err := live.RegisterIsland("alpine", NewAlpineIsland)
	if err != nil {
		log.Fatal("Failed to register alpine island:", err)
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

	// Set up SSE transport endpoints
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

	// SSE POST handler for client-to-server events
	http.HandleFunc("/live/post", func(w http.ResponseWriter, r *http.Request) {
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
		if err := tmpl.Execute(w, nil); err != nil {
			slog.Error("failed to execute template", "err", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
	})

	// Serve the v2 client library
	http.Handle("/live.js", live.Javascript{})

	// Start server
	addr := ":8080"
	log.Printf("Alpine example server starting on http://localhost%s", addr)
	log.Printf("Visit http://localhost%s to see the example", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatal(err)
	}
}
