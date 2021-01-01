package live

import (
	"bytes"
	"context"
	"html/template"
	"io"
	"log"
	"net/http"

	"github.com/gorilla/sessions"
)

// Example_temperature shows a simple temperature control using the
// "live-click" event.
func Example_temperature() {
	// Model of our thermostat.
	type ThermoModel struct {
		C float32
	}

	// Helper function to get the model from the socket data.
	NewThermoModel := func(s *Socket) *ThermoModel {
		m, ok := s.Assigns().(*ThermoModel)
		if !ok {
			m = &ThermoModel{
				C: 19.5,
			}
		}
		return m
	}

	cookieStore := sessions.NewCookieStore([]byte("weak-secret"))
	cookieStore.Options.HttpOnly = true
	cookieStore.Options.Secure = true
	cookieStore.Options.SameSite = http.SameSiteStrictMode

	// Parsing nil as a template to NewHandler will error if we do not set
	// a render function ourselves.
	h, err := NewHandler(nil, "session-key", cookieStore)
	if err != nil {
		log.Fatal("could not create handler")
	}

	// By default the handler will automatically render any template parsed into the
	// NewHandler function. However you can override and render an HTML string like
	// this.
	h.Render = func(ctc context.Context, t *template.Template, data interface{}) (io.Reader, error) {
		tmpl, err := template.New("thermo").Parse(`
            <div>{{.C}}</div>
            <button live-click="temp-up">+</button>
            <button live-click="temp-down">-</button>
            <!-- Include to make live work -->
            <script src="/live.js"></script>
        `)
		if err != nil {
			return nil, err
		}
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, data); err != nil {
			return nil, err
		}
		return &buf, nil
	}

	// Mount function is called on initial HTTP load and then initial web
	// socket connection. This should be used to create the initial state,
	// the connected variable will be true if the mount call is on a web
	// socket connection.
	h.Mount = func(ctx context.Context, h *Handler, r *http.Request, s *Socket, connected bool) (interface{}, error) {
		return NewThermoModel(s), nil
	}

	// This handles the `live-click="temp-up"` button. First we load the model from
	// the socket, increment the temperature, and then return the new state of the
	// model. Live will now calculate the diff between the last time it rendered and now,
	// produce a set of diffs and push them to the browser to update.
	h.HandleEvent("temp-up", func(s *Socket, _ map[string]interface{}) (interface{}, error) {
		model := NewThermoModel(s)
		model.C += 0.1
		return model, nil
	})

	// This handles the `live-click="temp-down"` button.
	h.HandleEvent("temp-down", func(s *Socket, _ map[string]interface{}) (interface{}, error) {
		model := NewThermoModel(s)
		model.C -= 0.1
		return model, nil
	})

	http.Handle("/thermostat", h)

	// This serves the JS needed to make live work.
	http.Handle("/live.js", Javascript{})
	http.ListenAndServe(":8080", nil)
}
