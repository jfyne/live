package chat

import (
	"context"
	"fmt"
	"html/template"
	"log"

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

func NewMessage(data any) Message {
	// This can handle both the chat example, and the cluster example.
	switch m := data.(type) {
	case Message:
		return m
	case map[string]any:
		return Message{
			ID:   m["ID"].(string),
			User: m["User"].(string),
			Msg:  m["Msg"].(string),
		}
	}
	return Message{}
}

type ChatInstance struct {
	Messages []Message
}

func NewChatInstance(s *live.Socket) *ChatInstance {
	m, ok := s.Assigns().(*ChatInstance)
	if !ok {
		return &ChatInstance{
			Messages: []Message{
				{ID: live.NewID(), User: "Room", Msg: "Welcome to chat " + string(s.ID())},
			},
		}
	}
	return m
}

func NewHandler() *live.Handler {
	t, err := template.ParseFiles("chat/layout.html", "chat/view.html")
	if err != nil {
		log.Fatal(err)
	}

	h := live.NewHandler(live.WithTemplateRenderer(t))

	// Set the mount function for this handler.
	h.MountHandler = func(ctx context.Context, s *live.Socket) (any, error) {
		// This will initialise the chat for this socket.
		return NewChatInstance(s), nil
	}

	// Handle user sending a message.
	h.HandleEvent(send, func(ctx context.Context, s *live.Socket, p live.Params) (any, error) {
		m := NewChatInstance(s)
		msg := p.String("message")
		if msg == "" {
			return m, nil
		}
		data := Message{
			ID:   live.NewID(),
			User: string(s.ID()),
			Msg:  msg,
		}
		if err := s.Broadcast(newmessage, data); err != nil {
			return m, fmt.Errorf("failed braodcasting new message: %w", err)
		}
		return m, nil
	})

	// Handle the broadcasted events.
	h.HandleSelf(newmessage, func(ctx context.Context, s *live.Socket, data any) (any, error) {
		m := NewChatInstance(s)

		// Here we don't append to messages as we don't want to use
		// loads of memory. `live-update="append"` handles the appending
		// of messages in the DOM.
		m.Messages = []Message{NewMessage(data)}
		return m, nil
	})

	return h
}
