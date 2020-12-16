package main

import (
	"context"
	"log"
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
	c, ok := s.Data.(*clock)
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

func mount(ctx context.Context, view *live.View, params map[string]string, s *live.Socket, connected bool) error {
	// Take the socket data and tranform it into our view model if it is
	// available.
	c := newClock(s)

	// Set the socket data to our view model, as this could be first load.
	s.Data = c

	// If we are mouting the websocket connection, trigger the first tick
	// event.
	if connected {
		go func() {
			time.Sleep(1 * time.Second)
			view.Self(s, live.Event{T: tick})
		}()
	}
	return nil
}

func main() {
	view, err := live.NewView("/clock", []string{"examples/root.html", "examples/clock/view.html"})
	if err != nil {
		log.Fatal(err)
	}

	// Set the mount function for this view.
	view.Mount = mount

	// Server side events.

	// tick event updates the clock every second.
	view.HandleSelf(tick, func(s *live.Socket, _ map[string]interface{}) (interface{}, error) {
		// Get our view model
		c := newClock(s)
		// Update the time.
		c.Time = time.Now()
		// Send ourselves another tick in a second.
		go func() {
			time.Sleep(1 * time.Second)
			view.Self(s, live.Event{T: tick})
		}()
		return c, nil
	})

	// Run the server.
	server := live.NewServer("session-key", []byte("weak-secret"))
	server.Add(view)
	if err := live.RunServer(server); err != nil {
		log.Fatal(err)
	}
}
