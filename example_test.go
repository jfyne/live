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

	// Parsing nil as a template to new view will error if we do not set
	// a render function ourselves.
	view, err := NewView(nil, "session-key", cookieStore)
	if err != nil {
		log.Fatal("could not create view")
	}

	// By default the view will automatically render any template degined in the
	// NewView function. However you can override and
	view.Render = func(ctc context.Context, t *template.Template, data interface{}) (io.Reader, error) {
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
	// socket connection.
	view.Mount = func(ctx context.Context, v *View, r *http.Request, s *Socket, connected bool) (interface{}, error) {
		return NewThermoModel(s), nil
	}

	view.HandleEvent("temp-up", func(s *Socket, _ map[string]interface{}) (interface{}, error) {
		model := NewThermoModel(s)
		model.C += 0.1
		return model, nil
	})

	view.HandleEvent("temp-down", func(s *Socket, _ map[string]interface{}) (interface{}, error) {
		model := NewThermoModel(s)
		model.C -= 0.1
		return model, nil
	})

	http.Handle("/thermostat", view)

	// This serves the JS needed to make live work.
	http.Handle("/live.js", Javascript{})
	http.ListenAndServe(":8080", nil)
}
