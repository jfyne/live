package main

import (
	"context"
	"fmt"
	"log"

	"github.com/jfyne/live"
)

const (
	validate = "validate"
	save     = "save"
)

type form struct {
	Errors map[string]string
}

type model struct {
	Messages []string
	Form     form
}

func newModel(s *live.Socket) *model {
	m, ok := s.Data.(*model)
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
	view, err := live.NewView("/form", []string{"examples/root.html", "examples/form/view.html"})
	if err != nil {
		log.Fatal(err)
	}
	// Set the mount function for this view.
	view.Mount = func(ctx context.Context, v *live.View, params map[string]string, s *live.Socket, connected bool) error {
		// This will initialise the counter if needed.
		s.Data = newModel(s)
		return nil
	}

	// Client side events.
	validateMessage := func(msg string) string {
		if len(msg) < 10 {
			return fmt.Sprintf("Length of 10 required, have %d", len(msg))
		}
		if len(msg) > 20 {
			return fmt.Sprintf("Your message is too long > 20, have %d", len(msg))
		}
		return ""
	}

	// Validate the form.
	view.HandleEvent(validate, func(s *live.Socket, p map[string]interface{}) (interface{}, error) {
		m := newModel(s)
		msg := live.ParamString(p, "message")
		vm := validateMessage(msg)
		if vm != "" {
			m.Form.Errors["message"] = vm
		}
		return m, nil
	})

	// Handle form saving.
	view.HandleEvent(save, func(s *live.Socket, p map[string]interface{}) (interface{}, error) {
		m := newModel(s)
		msg := live.ParamString(p, "message")
		vm := validateMessage(msg)
		if vm != "" {
			m.Form.Errors["message"] = vm
		} else {
			m.Messages = append(m.Messages, msg)
		}
		return m, nil
	})

	// Run the server.
	server := live.NewServer("session-key", []byte("weak-secret"))
	server.Add(view)
	if err := live.RunServer(server); err != nil {
		log.Fatal(err)
	}
}
