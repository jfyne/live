package live

import (
	"bytes"
	"context"
	"html/template"
	"io"
	"log"
)

// Example_temperature shows a simple temperature control using the
// "live-click" event.
func Example_temperature() {
	// Model of our thermostat.
	type ThermoModel struct {
		C float32
	}

	// Helper function to get the model from the socket data.
	hydrate := func(s *Socket) *ThermoModel {
		m, ok := s.Data.(*ThermoModel)
		if !ok {
			m = &ThermoModel{
				C: 19.5,
			}
		}
		return m
	}

	view, err := NewView("/thermostat", []string{})
	if err != nil {
		log.Fatal("could not create view")
	}

	// By default the view will automatically render any template degined in the
	// NewView function. However you can override and
	view.Render = func(ctc context.Context, t *template.Template, s *Socket) (io.Reader, error) {
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
		if err := tmpl.Execute(&buf, s.Data); err != nil {
			return nil, err
		}
		return &buf, nil
	}

	// Mount function is called on initial HTTP load and then initial web
	// socket connection.
	view.Mount = func(ctx context.Context, v *View, params map[string]string, s *Socket, connected bool) error {
		s.Data = hydrate(s)
		return nil
	}

	view.HandleEvent("temp-up", func(s *Socket, _ map[string]interface{}) error {
		model := hydrate(s)
		model.C += 0.1
		s.Data = model
		return nil
	})

	view.HandleEvent("temp-down", func(s *Socket, _ map[string]interface{}) error {
		model := hydrate(s)
		model.C -= 0.1
		s.Data = model
		return nil
	})

	// Create our server.
	l := NewServer("app", []byte("weak-secret"))

	// Add the view
	l.Add(view)

	RunServer(l)
}
