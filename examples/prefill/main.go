package main

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"log/slog"
	"net/http"

	"github.com/jfyne/live"
)

const (
	validate = "validate"
	save     = "save"
	done     = "done"
)

type model struct {
	Validation string
	Name       string
	Age        int
}

func newModel(s *live.Socket) *model {
	m, ok := s.Assigns().(*model)
	if !ok {
		return &model{
			Validation: "",
		}
	}
	// Clear validation on each event.
	m.Validation = ""
	return m
}

func main() {
	t, err := template.ParseFiles("root.html", "prefill/view.html")
	if err != nil {
		log.Fatal(err)
	}

	h := live.NewHandler(live.WithTemplateRenderer(t))

	// Set the mount function for this handler.
	h.MountHandler = func(ctx context.Context, s *live.Socket) (any, error) {
		// This will initialise the form.
		m := newModel(s)

		// Here we would get the user from the db or something.
		m.Name = "Test User"
		m.Age = 35

		return m, nil
	}

	// Validate the form.
	h.HandleEvent(validate, func(ctx context.Context, s *live.Socket, p live.Params) (any, error) {
		m := newModel(s)

		nameLen := len(p.String("name"))
		if nameLen <= 5 {
			m.Validation = fmt.Sprintf("short name (%d)", nameLen)
		}
		if nameLen > 10 {
			m.Validation = fmt.Sprintf("long name (%d)", nameLen)
		}

		return m, nil
	})

	// Handle form saving.
	h.HandleEvent(save, func(ctx context.Context, s *live.Socket, p live.Params) (any, error) {
		m := newModel(s)
		m.Name = p.String("name")
		m.Age = p.Int("age")
		return m, nil
	})

	// Run the server.
	http.Handle("/", live.NewHttpHandler(context.Background(), h))
	http.Handle("/live.js", live.Javascript{})
	http.Handle("/auto.js.map", live.JavascriptMap{})
	slog.Info("server", "link", "http://localhost:8080")
	http.ListenAndServe(":8080", nil)
}
