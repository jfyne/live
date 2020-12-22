package main

import (
	"context"
	"log"
	"net/http"

	"github.com/gorilla/sessions"
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
	cookieStore := sessions.NewCookieStore([]byte("weak-secret"))
	cookieStore.Options.HttpOnly = true
	cookieStore.Options.Secure = true
	cookieStore.Options.SameSite = http.SameSiteStrictMode

	view, err := live.NewView([]string{"examples/root.html", "examples/buttons/view.html"}, "session-key", cookieStore)
	if err != nil {
		log.Fatal(err)
	}

	// Set the mount function for this view.
	view.Mount = func(ctx context.Context, v *live.View, r *http.Request, s *live.Socket, connected bool) (interface{}, error) {
		// This will initialise the counter if needed.
		return newCounter(s), nil
	}

	// Client side events.

	// Increment event. Each click will increment the count by one.
	view.HandleEvent(inc, func(s *live.Socket, _ map[string]interface{}) (interface{}, error) {
		// Get this sockets counter struct.
		c := newCounter(s)

		// Increment the value by one.
		c.Value += 1

		// Set the counter struct back to the socket data.
		return c, nil
	})

	// Decrement event. Each click will increment the count by one.
	view.HandleEvent(dec, func(s *live.Socket, _ map[string]interface{}) (interface{}, error) {
		// Get this sockets counter struct.
		c := newCounter(s)

		// Decrement the value by one.
		c.Value -= 1

		// Set the counter struct back to the socket data.
		return c, nil
	})

	// Run the server.
	http.Handle("/buttons", view)
	http.Handle("/live.js", live.Javascript{})
	http.Handle("/auto.js.map", live.JavascriptMap{})
	http.ListenAndServe(":8080", nil)
}
