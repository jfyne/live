package main

import (
	"context"
	"html/template"
	"log"
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
	t, err := template.ParseFiles("examples/root.html", "examples/buttons/view.html")
	if err != nil {
		log.Fatal(err)
	}

	h, err := live.NewHandler(live.NewCookieStore("session-name", []byte("weak-secret")), live.WithTemplateRenderer(t))
	if err != nil {
		log.Fatal(err)
	}

	// Set the mount function for this handler.
	h.Mount = func(ctx context.Context, h *live.Handler, r *http.Request, s *live.Socket, connected bool) (interface{}, error) {
		// This will initialise the counter if needed.
		return newCounter(s), nil
	}

	// Client side events.

	// Increment event. Each click will increment the count by one.
	h.HandleEvent(inc, func(s *live.Socket, _ map[string]interface{}) (interface{}, error) {
		// Get this sockets counter struct.
		c := newCounter(s)

		// Increment the value by one.
		c.Value += 1

		// Set the counter struct back to the socket data.
		return c, nil
	})

	// Decrement event. Each click will increment the count by one.
	h.HandleEvent(dec, func(s *live.Socket, _ map[string]interface{}) (interface{}, error) {
		// Get this sockets counter struct.
		c := newCounter(s)

		// Decrement the value by one.
		c.Value -= 1

		// Set the counter struct back to the socket data.
		return c, nil
	})

	// Run the server.
	http.Handle("/buttons", h)
	http.Handle("/live.js", live.Javascript{})
	http.Handle("/auto.js.map", live.JavascriptMap{})
	http.ListenAndServe(":8080", nil)
}
