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

// Message represents a single chat message.
type Message struct {
	ID   string
	User string
	Msg  string
}

// ChatState holds the state for a chat island instance.
type ChatState struct {
	Messages []Message
}

// NewChatIsland creates a new chat island definition.
func NewChatIsland() (*live.Island, error) {
	island, err := live.NewIsland(
		"chat",
		live.WithMount(func(ctx context.Context, props live.Props, children string) (any, error) {
			user := props.String("user")
			return &ChatState{
				Messages: []Message{
					{ID: "welcome", User: "system", Msg: "Welcome, " + user + "!"},
				},
			}, nil
		}),
		live.WithRender(func(ctx context.Context, rc *live.IslandRenderContext) (io.Reader, error) {
			state := rc.State.(*ChatState)

			tmpl, err := template.ParseFS(content, "chat.html")
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

	// Handle "send" events from the client — validation only.
	// Broadcast handles the actual message distribution.
	err = island.HandleEvent("send", func(ctx context.Context, state any, params live.Params) (any, error) {
		msg := params.String("message")
		if msg == "" {
			// Empty message: return state unchanged
			return state, nil
		}
		// Valid message: return state unchanged — broadcast handles display
		return state, nil
	})
	if err != nil {
		return nil, err
	}

	// Handle "newmessage" self events from the broadcast.
	// Sets state.Messages to a single-element slice so the re-rendered
	// fragment contains only the new message, which live-update="append"
	// appends to the existing DOM.
	err = island.HandleSelf("newmessage", func(ctx context.Context, state any, data any) (any, error) {
		s := state.(*ChatState)
		msg, ok := data.(Message)
		if !ok {
			return nil, fmt.Errorf("newmessage: expected Message, got %T", data)
		}
		s.Messages = []Message{msg}
		return s, nil
	})
	if err != nil {
		return nil, err
	}

	return island, nil
}

func startServer(ctx context.Context, addr string, broadcast *live.Broadcast) {
	stateStore := live.NewMemoryIslandStateStore(ctx, 1*time.Minute)
	registry := live.DefaultRegistry()
	engine := live.NewIslandEngine(ctx, registry, stateStore)
	defer engine.Close()

	broadcast.Subscribe("chat-room", "chat", engine)

	mux := http.NewServeMux()

	// WebSocket transport endpoint
	wsConfig := live.DefaultTransportConfig()
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
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
				log.Printf("[%s] Received event: type=%s, island=%s", addr, event.T, event.Island)

				if event.T == "subscribe" && event.Island != "" {
					islandID := live.IslandID(event.Island)
					params, _ := event.Params()
					islandType := params.String("type")

					if islandType == "" {
						log.Printf("Warning: subscribe event for %s missing type, cannot mount", islandID)
						continue
					}

					_, err := engine.MountIsland(sessionID, islandID, islandType, live.Props{"user": string(sessionID)})
					if err != nil {
						log.Printf("Failed to mount island %s: %v", islandID, err)
					}
					continue
				}

				if event.T == "send" {
					params, _ := event.Params()
					msg := params.String("message")
					if msg != "" {
						broadcast.Publish(ctx, "chat-room", live.Event{
							T:        "newmessage",
							SelfData: Message{ID: fmt.Sprintf("msg-%d", time.Now().UnixNano()), User: string(sessionID), Msg: msg},
						})
					}
				}

				if err := engine.RouteEvent(sessionID, event); err != nil {
					log.Printf("Event routing error: %v", err)
				}
			}
		}()

		<-r.Context().Done()
	})

	// SSE transport endpoints
	sseConfig := live.DefaultTransportConfig()
	sseFactory := live.NewSSETransportFactory(sseConfig)

	mux.HandleFunc("/live/sse", func(w http.ResponseWriter, r *http.Request) {
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
				log.Printf("[%s] SSE received event: type=%s, island=%s", addr, event.T, event.Island)

				if event.T == "subscribe" && event.Island != "" {
					islandID := live.IslandID(event.Island)
					params, _ := event.Params()
					islandType := params.String("type")

					if islandType == "" {
						log.Printf("Warning: subscribe event for %s missing type, cannot mount", islandID)
						continue
					}

					_, err := engine.MountIsland(sessionID, islandID, islandType, live.Props{"user": string(sessionID)})
					if err != nil {
						log.Printf("Failed to mount island %s: %v", islandID, err)
					}
					continue
				}

				if event.T == "send" {
					params, _ := event.Params()
					msg := params.String("message")
					if msg != "" {
						broadcast.Publish(ctx, "chat-room", live.Event{
							T:        "newmessage",
							SelfData: Message{ID: fmt.Sprintf("msg-%d", time.Now().UnixNano()), User: string(sessionID), Msg: msg},
						})
					}
				}

				if err := engine.RouteEvent(sessionID, event); err != nil {
					log.Printf("Event routing error: %v", err)
				}
			}
		}()

		<-r.Context().Done()
	})

	mux.HandleFunc("/live/post", func(w http.ResponseWriter, r *http.Request) {
		sseFactory.HandlePost(w, r)
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
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

	mux.Handle("/live.js", live.Javascript{})

	log.Printf("Chat example server starting on http://localhost%s", addr)
	server := &http.Server{Addr: addr, Handler: mux}
	if err := server.ListenAndServe(); err != nil {
		log.Printf("Server on %s stopped: %v", addr, err)
	}
}

func main() {
	ctx := context.Background()

	err := live.RegisterIsland("chat", NewChatIsland)
	if err != nil {
		log.Fatal("Failed to register chat island:", err)
	}

	broadcast := live.NewBroadcast(ctx, live.NewLocalTransport())

	go startServer(ctx, ":8080", broadcast)
	startServer(ctx, ":8081", broadcast)
}
