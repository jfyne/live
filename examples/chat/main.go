package main

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"net/http"

	"github.com/gorilla/sessions"
	"github.com/jfyne/live"
	"github.com/rs/xid"
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
				{ID: "1", User: "Room", Msg: "Welcome to chat"},
				{ID: "2", User: "Room", Msg: "Start typing to talk to other users who are connected"},
			},
		}
	}
	return m
}

func main() {
	cookieStore := sessions.NewCookieStore([]byte("weak-secret"))
	cookieStore.Options.HttpOnly = true
	cookieStore.Options.Secure = true
	cookieStore.Options.SameSite = http.SameSiteStrictMode

	t, err := template.ParseFiles("examples/chat/layout.html", "examples/chat/view.html")
	if err != nil {
		log.Fatal(err)
	}

	view, err := live.NewView(
		t,
		"session-key",
		cookieStore,
		live.WithRootTemplate("layout.html"))
	if err != nil {
		log.Fatal(err)
	}
	// Set the mount function for this view.
	view.Mount = func(ctx context.Context, v *live.View, r *http.Request, s *live.Socket, connected bool) (interface{}, error) {
		// This will initialise the chat for this socket.
		return NewChatInstance(s), nil
	}

	// Handle user sending a message.
	view.HandleEvent(send, func(s *live.Socket, p map[string]interface{}) (interface{}, error) {
		m := NewChatInstance(s)
		msg := live.ParamString(p, "message")
		if msg == "" {
			return m, nil
		}
		view.Broadcast(live.Event{T: newmessage, Data: map[string]interface{}{"message": Message{ID: xid.New().String(), User: s.Session.ID, Msg: msg}}})
		return m, nil
	})

	// Handle the broadcasted events.
	view.HandleSelf(newmessage, func(s *live.Socket, p map[string]interface{}) (interface{}, error) {
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
	http.Handle("/chat", view)
	http.Handle("/live.js", live.Javascript{})
	http.Handle("/live.js.map", live.JavascriptMap{})
	http.ListenAndServe(":8080", nil)
}
