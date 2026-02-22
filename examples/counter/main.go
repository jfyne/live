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

// CounterState holds the state for a counter island instance.
type CounterState struct {
	Count int
}

// counterConfig defines a counter island's server-side configuration.
type counterConfig struct {
	ID      string
	Label   string
	Initial int
}

// Server-side configuration: the server owns the initial state.
var counters = []counterConfig{
	{ID: "counter-1", Label: "Counter 1 (default, starts at 0)", Initial: 0},
	{ID: "counter-2", Label: "Counter 2 (starts at 5)", Initial: 5},
	{ID: "counter-3", Label: "Counter 3 (starts at 10)", Initial: 10},
}

// counterInitialValues maps island IDs to their server-defined initial values.
var counterInitialValues = func() map[string]int {
	m := make(map[string]int)
	for _, c := range counters {
		m[c.ID] = c.Initial
	}
	return m
}()

// NewCounterIsland creates a new counter island definition.
func NewCounterIsland() (*live.Island, error) {
	island, err := live.NewIsland(
		"counter",
		live.WithMount(func(ctx context.Context, props live.Props, children string) (any, error) {
			// Initial value comes from server-side configuration.
			// The island ID is passed as a prop by the subscribe handler.
			initialValue := props.Int("initial")
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

		// Each WebSocket connection gets its own unique session ID.
		// Cookies are shared across tabs, so using cookie-based IDs
		// would cause tab collisions (AddSession overwrites).
		sessionID := live.SessionID(fmt.Sprintf("session-%d", time.Now().UnixNano()))

		// Create a session with the transport
		session := live.NewSession(r.Context(), sessionID, transport)

		// Add session to engine
		engine.AddSession(session)

		// Clean up when connection closes
		defer func() {
			engine.DeleteSession(sessionID)
		}()

		// Process events from the transport
		// Read events and route them through the engine
		go func() {
			for event := range transport.Events() {
				log.Printf("Received event: type=%s, island=%s", event.T, event.Island)

				// Check if this is a subscribe event (island mount request)
				if event.T == "subscribe" && event.Island != "" {
					islandID := live.IslandID(event.Island)

					params, _ := event.Params()
					islandType := params.String("type")

					if islandType == "" {
						log.Printf("Warning: subscribe event for %s missing type, cannot mount", islandID)
						continue
					}

					// Build props from server-side configuration.
					// The server is the source of truth for initial state.
					props := live.Props{
						"initial": counterInitialValues[string(islandID)],
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

					props := live.Props{
						"initial": counterInitialValues[string(islandID)],
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
	http.HandleFunc("/live/post", func(w http.ResponseWriter, r *http.Request) {
		sseFactory.HandlePost(w, r)
	})

	// Serve the main HTML page, rendered with server-side counter configuration
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
		if err := tmpl.Execute(w, counters); err != nil {
			slog.Error("failed to execute template", "err", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
	})

	// Serve the v2 client library
	http.Handle("/live.js", live.Javascript{})

	// Start server
	addr := ":8080"
	log.Printf("Counter example server starting on http://localhost%s", addr)
	log.Printf("Visit http://localhost%s to see the example", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatal(err)
	}
}
