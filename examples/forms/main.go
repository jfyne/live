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

// Task represents a single to-do item.
type Task struct {
	ID       string
	Name     string
	Complete bool
}

// TodoState holds the state for a todo island instance.
type TodoState struct {
	Tasks  []Task
	Errors map[string]string
	NextID int
}

// NewTodoIsland creates a new todo island definition.
func NewTodoIsland() (*live.Island, error) {
	island, err := live.NewIsland(
		"todo",
		live.WithMount(func(ctx context.Context, props live.Props, children string) (any, error) {
			return &TodoState{
				NextID: 1,
				Errors: make(map[string]string),
			}, nil
		}),
		live.WithRender(func(ctx context.Context, rc *live.IslandRenderContext) (io.Reader, error) {
			state := rc.State.(*TodoState)

			tmpl, err := template.ParseFS(content, "todo.html")
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

	// validate: check task name length
	err = island.HandleEvent("validate", func(ctx context.Context, state any, params live.Params) (any, error) {
		s := state.(*TodoState)
		name := params.String("task")
		if len(name) == 0 {
			s.Errors["message"] = "Task name is required"
		} else if len(name) >= 100 {
			s.Errors["message"] = "Task name must be under 100 characters"
		} else {
			delete(s.Errors, "message")
		}
		return s, nil
	})
	if err != nil {
		return nil, err
	}

	// save: validate and append task
	err = island.HandleEvent("save", func(ctx context.Context, state any, params live.Params) (any, error) {
		s := state.(*TodoState)
		name := params.String("task")
		if len(name) == 0 {
			s.Errors["message"] = "Task name is required"
			return s, nil
		}
		if len(name) >= 100 {
			s.Errors["message"] = "Task name must be under 100 characters"
			return s, nil
		}
		delete(s.Errors, "message")
		s.Tasks = append(s.Tasks, Task{
			ID:   fmt.Sprintf("task-%d", s.NextID),
			Name: name,
		})
		s.NextID++
		return s, nil
	})
	if err != nil {
		return nil, err
	}

	// done: toggle task completion
	err = island.HandleEvent("done", func(ctx context.Context, state any, params live.Params) (any, error) {
		s := state.(*TodoState)
		id := params.String("id")
		for i := range s.Tasks {
			if s.Tasks[i].ID == id {
				s.Tasks[i].Complete = !s.Tasks[i].Complete
				break
			}
		}
		return s, nil
	})
	if err != nil {
		return nil, err
	}

	return island, nil
}

// PrefillState holds the state for a prefill island instance.
type PrefillState struct {
	Name       string
	Age        int
	Validation string
}

// NewPrefillIsland creates a new prefill island definition.
func NewPrefillIsland() (*live.Island, error) {
	island, err := live.NewIsland(
		"prefill",
		live.WithMount(func(ctx context.Context, props live.Props, children string) (any, error) {
			return &PrefillState{
				Name: props.String("name"),
				Age:  props.Int("age"),
			}, nil
		}),
		live.WithRender(func(ctx context.Context, rc *live.IslandRenderContext) (io.Reader, error) {
			state := rc.State.(*PrefillState)

			tmpl, err := template.ParseFS(content, "prefill.html")
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

	// validate: check name field
	err = island.HandleEvent("validate", func(ctx context.Context, state any, params live.Params) (any, error) {
		s := state.(*PrefillState)
		name := params.String("name")
		if len(name) == 0 {
			s.Validation = "Name is required"
		} else if len(name) > 200 {
			s.Validation = "Name must be under 200 characters"
		} else {
			s.Validation = ""
		}
		return s, nil
	})
	if err != nil {
		return nil, err
	}

	// save: update name and age
	err = island.HandleEvent("save", func(ctx context.Context, state any, params live.Params) (any, error) {
		s := state.(*PrefillState)
		s.Name = params.String("name")
		s.Age = params.Int("age")
		return s, nil
	})
	if err != nil {
		return nil, err
	}

	return island, nil
}

func main() {
	// Register islands with the global registry
	err := live.RegisterIsland("todo", NewTodoIsland)
	if err != nil {
		log.Fatal("Failed to register todo island:", err)
	}

	err = live.RegisterIsland("prefill", NewPrefillIsland)
	if err != nil {
		log.Fatal("Failed to register prefill island:", err)
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

					var props live.Props
					switch islandType {
					case "prefill":
						props = live.Props{"name": "Test User", "age": 35}
					default:
						props = live.Props{}
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

					var props live.Props
					switch islandType {
					case "prefill":
						props = live.Props{"name": "Test User", "age": 35}
					default:
						props = live.Props{}
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
	log.Printf("Forms example server starting on http://localhost%s", addr)
	log.Printf("Visit http://localhost%s to see the example", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatal(err)
	}
}
