package main

import (
	"context"
	"log"

	"github.com/jfyne/live"
)

const (
	inc = "inc"
	dec = "dec"
)

type counter struct {
	Value int
}

func newCounter(s *live.Socket) *counter {
	c, ok := s.Data.(*counter)
	if !ok {
		return &counter{}
	}
	return c
}

func main() {
	view, err := live.NewView("/buttons", []string{"examples/root.html", "examples/buttons/view.html"})
	if err != nil {
		log.Fatal(err)
	}

	// Set the mount function for this view.
	view.Mount = func(ctx context.Context, v *live.View, params map[string]string, s *live.Socket, connected bool) error {
		// This will initialise the counter if needed.
		s.Data = newCounter(s)
		return nil
	}

	// Client side events.

	// Increment event. Each click will increment the count by one.
	view.HandleEvent(inc, func(s *live.Socket, _ map[string]interface{}) error {
		// Get this sockets counter struct.
		c := newCounter(s)

		// Increment the value by one.
		c.Value += 1

		// Set the counter struct back to the socket data.
		s.Data = c

		return nil
	})

	// Decrement event. Each click will increment the count by one.
	view.HandleEvent(dec, func(s *live.Socket, _ map[string]interface{}) error {
		// Get this sockets counter struct.
		c := newCounter(s)

		// Decrement the value by one.
		c.Value -= 1

		// Set the counter struct back to the socket data.
		s.Data = c

		return nil
	})

	// Run the server.
	server := live.NewServer("session-key", []byte("weak-secret"))
	server.Add(view)
	if err := live.RunServer(server); err != nil {
		log.Fatal(err)
	}
}
