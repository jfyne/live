package main

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"net/http"

	"github.com/jfyne/live"
)

const (
	send       = "send"
	newmessage = "newmessage"
)

type Message struct {
	ID   string // Unique ID per message so that we can use `live-update`.
	User string
	Msg  string
}

type ChatInstance struct {
	Messages []Message
}

func NewChatInstance(s *live.Socket) *ChatInstance {
	m, ok := s.Assigns().(*ChatInstance)
	if !ok {
		return &ChatInstance{
			Messages: []Message{
				{ID: live.NewID(), User: "Room", Msg: "Welcome to chat " + s.Session.ID},
			},
		}
	}
	return m
}

func main() {
	t, err := template.ParseFiles("examples/chat/layout.html", "examples/chat/view.html")
	if err != nil {
		log.Fatal(err)
	}

	h, err := live.NewHandler(t, live.NewCookieStore("session-name", []byte("weak-secret")), live.WithRootTemplate("layout.html"))
	if err != nil {
		log.Fatal(err)
	}
	// Set the mount function for this handler.
	h.Mount = func(ctx context.Context, h *live.Handler, r *http.Request, s *live.Socket, connected bool) (interface{}, error) {
		// This will initialise the chat for this socket.
		return NewChatInstance(s), nil
	}

	// Handle user sending a message.
	h.HandleEvent(send, func(s *live.Socket, p map[string]interface{}) (interface{}, error) {
		m := NewChatInstance(s)
		msg := live.ParamString(p, "message")
		if msg == "" {
			return m, nil
		}
		h.Broadcast(live.Event{T: newmessage, Data: map[string]interface{}{"message": Message{ID: live.NewID(), User: s.Session.ID, Msg: msg}}})
		return m, nil
	})

	// Handle the broadcasted events.
	h.HandleSelf(newmessage, func(s *live.Socket, p map[string]interface{}) (interface{}, error) {
		m := NewChatInstance(s)
		data, ok := p["message"]
		if !ok {
			return m, fmt.Errorf("no message key")
		}
		msg, ok := data.(Message)
		if !ok {
			return m, fmt.Errorf("malformed message")
		}
		// Here we don't append to messages as we don't want to use
		// loads of memory. `live-update="append"` handles the appending
		// of messages in the DOM.
		m.Messages = []Message{msg}
		return m, nil
	})

	// Run the server.
	http.Handle("/chat", h)
	http.Handle("/live.js", live.Javascript{})
	http.Handle("/auto.js.map", live.JavascriptMap{})
	http.ListenAndServe(":8080", nil)
}
