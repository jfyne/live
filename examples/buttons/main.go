package main

import (
	"context"
	"html/template"
	"log"
	"log/slog"
	"net/http"

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
	c, ok := s.Assigns().(*counter)
	if !ok {
		return &counter{}
	}
	return c
}

func main() {
	t, err := template.ParseFiles("root.html", "buttons/view.html")
	if err != nil {
		log.Fatal(err)
	}

	h := live.NewHandler(live.WithTemplateRenderer(t))

	// Set the mount function for this handler.
	h.MountHandler = func(ctx context.Context, s *live.Socket) (any, error) {
		// This will initialise the counter if needed.
		return newCounter(s), nil
	}

	// Client side events.

	// Increment event. Each click will increment the count by one.
	h.HandleEvent(inc, func(ctx context.Context, s *live.Socket, _ live.Params) (any, error) {
		// Get this sockets counter struct.
		c := newCounter(s)

		// Increment the value by one.
		c.Value += 1

		// Set the counter struct back to the socket data.
		return c, nil
	})

	// Decrement event. Each click will increment the count by one.
	h.HandleEvent(dec, func(ctx context.Context, s *live.Socket, _ live.Params) (any, error) {
		// Get this sockets counter struct.
		c := newCounter(s)

		// Decrement the value by one.
		c.Value -= 1

		// Set the counter struct back to the socket data.
		return c, nil
	})

	// Run the server.
	ctx := context.Background()
	http.Handle("/", live.NewHttpHandler(ctx, h))
	http.Handle("/live.js", live.Javascript{})
	http.Handle("/auto.js.map", live.JavascriptMap{})
	slog.Info("server", "link", "http://localhost:8080")
	http.ListenAndServe(":8080", nil)
}
