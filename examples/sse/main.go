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
	c := newClock(s)

	if s.Connected() {
		go func() {
			time.Sleep(1 * time.Second)
			s.Self(ctx, tick, time.Now())
		}()
	}
	return c, nil
}

func main() {
	t, err := template.ParseFiles("root.html", "view.html")
	if err != nil {
		log.Fatal(err)
	}

	h := live.NewHandler(live.WithTemplateRenderer(t))
	h.MountHandler = mount

	h.HandleSelf(tick, func(ctx context.Context, s *live.Socket, d any) (any, error) {
		c := newClock(s)
		c.Time = d.(time.Time)
		go func(sock *live.Socket) {
			time.Sleep(1 * time.Second)
			sock.Self(ctx, tick, time.Now())
		}(s)
		return c, nil
	})

	http.Handle("/", live.NewHttpHandler(context.Background(), h))
	http.Handle("/live.js", live.Javascript{})
	http.Handle("/auto.js.map", live.JavascriptMap{})
	slog.Info("server", "link", "http://localhost:8080")
	http.ListenAndServe(":8080", nil)
}
