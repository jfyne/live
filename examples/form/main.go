package main

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"net/http"

	"github.com/gorilla/sessions"
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
	cookieStore := sessions.NewCookieStore([]byte("weak-secret"))
	cookieStore.Options.HttpOnly = true
	cookieStore.Options.Secure = true
	cookieStore.Options.SameSite = http.SameSiteStrictMode

	t, err := template.ParseFiles("examples/root.html", "examples/form/view.html")
	if err != nil {
		log.Fatal(err)
	}

	view, err := live.NewHandler(t, "session-key", cookieStore)
	if err != nil {
		log.Fatal(err)
	}
	// Set the mount function for this view.
	view.Mount = func(ctx context.Context, h *live.Handler, r *http.Request, s *live.Socket, connected bool) (interface{}, error) {
		// This will initialise the form.
		return newModel(s), nil
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
	http.Handle("/form", view)
	http.Handle("/live.js", live.Javascript{})
	http.Handle("/auto.js.map", live.JavascriptMap{})
	http.ListenAndServe(":8080", nil)
}
