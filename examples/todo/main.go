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

type form struct {
	Errors map[string]string
}

type task struct {
	ID       string
	Name     string
	Complete bool
}

type model struct {
	Tasks []task
	Form  form
}

func newModel(s *live.Socket) *model {
	m, ok := s.Assigns().(*model)
	if !ok {
		return &model{
			Form: form{
				Errors: map[string]string{},
			},
		}
	}
	// Clear errors on each event as we recheck each
	// time.
	m.Form.Errors = map[string]string{}
	return m
}

func main() {
	t, err := template.ParseFiles("root.html", "todo/view.html")
	if err != nil {
		log.Fatal(err)
	}

	h := live.NewHandler(live.WithTemplateRenderer(t))

	// Set the mount function for this handler.
	h.MountHandler = func(ctx context.Context, s *live.Socket) (any, error) {
		// This will initialise the form.
		return newModel(s), nil
	}

	// Client side events.
	validateMessage := func(msg string) string {
		if len(msg) < 10 {
			return fmt.Sprintf("Length of 10 required, have %d", len(msg))
		}
		if len(msg) > 20 {
			return fmt.Sprintf("Your task name is too long > 20, have %d", len(msg))
		}
		return ""
	}

	// Validate the form.
	h.HandleEvent(validate, func(ctx context.Context, s *live.Socket, p live.Params) (any, error) {
		m := newModel(s)
		t := p.String("task")
		vm := validateMessage(t)
		if vm != "" {
			m.Form.Errors["message"] = vm
		}
		return m, nil
	})

	// Handle form saving.
	h.HandleEvent(save, func(ctx context.Context, s *live.Socket, p live.Params) (any, error) {
		m := newModel(s)
		ts := p.String("task")
		complete := p.Checkbox("complete")
		vm := validateMessage(ts)
		if vm != "" {
			m.Form.Errors["message"] = vm
		} else {
			t := task{
				ID:       live.NewID(),
				Name:     ts,
				Complete: complete,
			}
			m.Tasks = append(m.Tasks, t)
		}
		return m, nil
	})

	// Handle completing tasks.
	h.HandleEvent(done, func(ctx context.Context, s *live.Socket, p live.Params) (any, error) {
		m := newModel(s)
		ID := p.String("id")
		for idx, t := range m.Tasks {
			if t.ID != ID {
				continue
			}
			m.Tasks[idx].Complete = !m.Tasks[idx].Complete
		}
		return m, nil
	})

	// Run the server.
	http.Handle("/", live.NewHttpHandler(context.Background(), h))
	http.Handle("/live.js", live.Javascript{})
	http.Handle("/auto.js.map", live.JavascriptMap{})
	slog.Info("server", "link", "http://localhost:8080")
	http.ListenAndServe(":8080", nil)
}
