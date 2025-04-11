package main

import (
	"context"
	"html/template"
	"log"
	"log/slog"
	"net/http"
	"time"

	"github.com/jfyne/live"
)

const (
	tick = "tick"
)

type clock struct {
	Time time.Time
}

func newClock(s *live.Socket) *clock {
	c, ok := s.Assigns().(*clock)
	if !ok {
		return &clock{
			Time: time.Now(),
		}
	}
	return c
}

func (c clock) FormattedTime() string {
	return c.Time.Format("15:04:05")
}

func mount(ctx context.Context, s *live.Socket) (any, error) {
	// Take the socket data and tranform it into our view model if it is
	// available.
	c := newClock(s)

	// If we are mouting the websocket connection, trigger the first tick
	// event.
	if s.Connected() {
		go func() {
			time.Sleep(1 * time.Second)
			s.Self(ctx, tick, time.Now())
		}()
	}
	return c, nil
}

func main() {
	t, err := template.ParseFiles("root.html", "clock/view.html")
	if err != nil {
		log.Fatal(err)
	}

	h := live.NewHandler(live.WithTemplateRenderer(t))

	// Set the mount function for this handler.
	h.MountHandler = mount

	// Server side events.

	// tick event updates the clock every second.
	h.HandleSelf(tick, func(ctx context.Context, s *live.Socket, d any) (any, error) {
		// Get our model
		c := newClock(s)
		// Update the time.
		c.Time = d.(time.Time)
		// Send ourselves another tick in a second.
		go func(sock *live.Socket) {
			time.Sleep(1 * time.Second)
			sock.Self(ctx, tick, time.Now())
		}(s)
		return c, nil
	})

	// Run the server.
	http.Handle("/", live.NewHttpHandler(context.Background(), h))
	http.Handle("/live.js", live.Javascript{})
	http.Handle("/auto.js.map", live.JavascriptMap{})
	slog.Info("server", "link", "http://localhost:8080")
	http.ListenAndServe(":8080", nil)
}
