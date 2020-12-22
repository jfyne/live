package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/sessions"
	"github.com/jfyne/live"
)

const (
	send       = "send"
	newmessage = "newmessage"
)

type Message struct {
	User string
	Msg  string
}

type ChatInstance struct {
	Messages []Message
}

func NewChatInstance(s *live.Socket) *ChatInstance {
	m, ok := s.Data.(*ChatInstance)
	if !ok {
		return &ChatInstance{
			Messages: []Message{},
		}
	}
	return m
}

func main() {
	cookieStore := sessions.NewCookieStore([]byte("weak-secret"))
	cookieStore.Options.HttpOnly = true
	cookieStore.Options.Secure = true
	cookieStore.Options.SameSite = http.SameSiteStrictMode

	view, err := live.NewView([]string{"examples/root.html", "examples/chat/view.html"}, "session-key", cookieStore)
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
		view.Broadcast(live.Event{T: newmessage, Data: map[string]interface{}{"message": Message{User: s.Session.ID, Msg: msg}}})
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
		m.Messages = append(m.Messages, msg)
		return m, nil
	})

	// Run the server.
	http.Handle("/chat", view)
	http.Handle("/live.js", live.Javascript{})
	http.Handle("/auto.js.map", live.JavascriptMap{})
	http.ListenAndServe(":8080", nil)
}
