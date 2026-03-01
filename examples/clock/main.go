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

// ClockState holds the state for a clock island instance.
type ClockState struct {
	Time     time.Time
	Location *time.Location
	Label    string
}

// FormattedTime returns the current time formatted in the island's timezone.
func (s *ClockState) FormattedTime() string {
	return s.Time.In(s.Location).Format("15:04:05")
}

// clockConfig defines a clock island's server-side configuration.
type clockConfig struct {
	ID       string
	Label    string
	Timezone string
}

// Server-side configuration: the server owns the clock configurations.
var clocks = []clockConfig{
	{ID: "clock-utc", Label: "UTC", Timezone: "UTC"},
	{ID: "clock-nyc", Label: "New York", Timezone: "America/New_York"},
	{ID: "clock-london", Label: "London", Timezone: "Europe/London"},
	{ID: "clock-tokyo", Label: "Tokyo", Timezone: "Asia/Tokyo"},
}

// clockConfigByID maps island IDs to their server-defined configurations.
var clockConfigByID = func() map[string]clockConfig {
	m := make(map[string]clockConfig)
	for _, c := range clocks {
		m[c.ID] = c
	}
	return m
}()

// NewClockIsland creates a new clock island definition.
func NewClockIsland() (*live.Island, error) {
	island, err := live.NewIsland(
		"clock",
		live.WithMount(func(ctx context.Context, props live.Props, children string) (any, error) {
			timezone := props.String("timezone")
			if timezone == "" {
				timezone = "UTC"
			}
			loc, err := time.LoadLocation(timezone)
			if err != nil {
				return nil, fmt.Errorf("invalid timezone %q: %w", timezone, err)
			}
			label := props.String("label")

			// Schedule the first tick immediately after mount.
			live.SendSelf(ctx, "tick", nil)

			return &ClockState{
				Time:     time.Now(),
				Location: loc,
				Label:    label,
			}, nil
		}),
		live.WithRender(func(ctx context.Context, rc *live.IslandRenderContext) (io.Reader, error) {
			state := rc.State.(*ClockState)

			tmpl, err := template.ParseFS(content, "clock.html")
			if err != nil {
				return nil, err
			}

			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, state); err != nil {
				return nil, err
			}

			return &buf, nil
		}),
		// Re-deliver the "tick" self-event after 1 second.
		live.WithEventDelay("tick", 1*time.Second),
	)
	if err != nil {
		return nil, err
	}

	// Register the tick self-event handler.
	// The next tick is re-scheduled automatically by WithEventDelay("tick", 1s)
	// after each handler invocation. No need to call SendSelf here.
	err = island.HandleSelf("tick", func(ctx context.Context, state any, data any) (any, error) {
		s := state.(*ClockState)
		s.Time = time.Now()
		return s, nil
	})
	if err != nil {
		return nil, err
	}

	return island, nil
}

func main() {
	// Register the clock island with the global registry.
	err := live.RegisterIsland("clock", NewClockIsland)
	if err != nil {
		log.Fatal("Failed to register clock island:", err)
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

					// Look up the clock config by island ID.
					config, ok := clockConfigByID[string(islandID)]
					props := live.Props{}
					if ok {
						props["timezone"] = config.Timezone
						props["label"] = config.Label
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

					config, ok := clockConfigByID[string(islandID)]
					props := live.Props{}
					if ok {
						props["timezone"] = config.Timezone
						props["label"] = config.Label
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

	// SSE POST handler for client-to-server events.
	http.HandleFunc("/live/post", func(w http.ResponseWriter, r *http.Request) {
		sseFactory.HandlePost(w, r)
	})

	// Serve the main HTML page, rendered with server-side clock configuration.
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
		if err := tmpl.Execute(w, clocks); err != nil {
			slog.Error("failed to execute template", "err", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
	})

	// Serve the v2 client library.
	http.Handle("/live.js", live.Javascript{})

	// Start server.
	addr := ":8081"
	log.Printf("Clock example server starting on http://localhost%s", addr)
	log.Printf("Visit http://localhost%s to see the example", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatal(err)
	}
}
