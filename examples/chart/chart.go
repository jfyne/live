package main

import (
	"context"
	"html/template"
	"log"
	"log/slog"
	"math/rand"
	"net/http"
	"time"

	"github.com/jfyne/live"
)

const (
	regenerate = "regenerate"
)

type RandomEngine struct {
	*live.Engine
}

func NewRandomEngine(h *live.Handler) *RandomEngine {
	e := &RandomEngine{
		live.NewHttpHandler(context.Background(), h),
	}
	return e
}

func (e *RandomEngine) Start() {
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		for {
			<-ticker.C
			e.Broadcast(regenerate, rand.Perm(9))
		}
	}()
}

type chartData struct {
	Sales []int
}

func newChartData(s *live.Socket) *chartData {
	d, ok := s.Assigns().(*chartData)
	if !ok {
		return &chartData{
			Sales: rand.Perm(9),
		}
	}
	return d
}

func main() {
	t, err := template.ParseFiles("root.html", "chart/view.html")
	if err != nil {
		log.Fatal(err)
	}

	h := live.NewHandler(live.WithTemplateRenderer(t))

	// Set the mount function for this handler.
	h.MountHandler = func(ctx context.Context, s *live.Socket) (any, error) {
		// This will initialise the chart data if needed.
		return newChartData(s), nil
	}

	// Client side events.

	// Regenerate event, creates new random sales data.
	h.HandleSelf(regenerate, func(ctx context.Context, s *live.Socket, d any) (any, error) {
		// Get this sockets counter struct.
		c := newChartData(s)

		// Generate new sales data.
		c.Sales = d.([]int)

		// Set the new chart data back to the socket.
		return c, nil
	})

	e := NewRandomEngine(h)
	e.Start()

	// Run the server.
	http.Handle("/", e)
	http.Handle("/live.js", live.Javascript{})
	http.Handle("/auto.js.map", live.JavascriptMap{})
	slog.Info("server", "link", "http://localhost:8080")
	http.ListenAndServe(":8080", nil)
}
