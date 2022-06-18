package live

import (
	"bytes"
	"context"
	"html/template"
	"io"
	"net/http"
)

// Model of our thermostat.
type ThermoModel struct {
	C float32
}

// Helper function to get the model from the socket data.
func NewThermoModel(s Socket) *ThermoModel {
	m, ok := s.Assigns().(*ThermoModel)
	// If we haven't already initialised set up.
	if !ok {
		m = &ThermoModel{
			C: 19.5,
		}
	}
	return m
}

// thermoMount initialises the thermostat state. Data returned in the mount function will
// automatically be assigned to the socket.
func thermoMount(ctx context.Context, s Socket) (any, error) {
	return NewThermoModel(s), nil
}

// tempUp on the temp up event, increase the thermostat temperature by .1 C. An EventHandler function
// is called with the original request context of the socket, the socket itself containing the current
// state and and params that came from the event. Params contain query string parameters and any
// `live-value-` bindings.
func tempUp(ctx context.Context, s Socket, p Params) (any, error) {
	model := NewThermoModel(s)
	model.C += 0.1
	return model, nil
}

// tempDown on the temp down event, decrease the thermostat temperature by .1 C.
func tempDown(ctx context.Context, s Socket, p Params) (any, error) {
	model := NewThermoModel(s)
	model.C -= 0.1
	return model, nil
}

// Example shows a simple temperature control using the
// "live-click" event.
func Example() {

	// Setup the handler.
	h := NewHandler()

	// Mount function is called on initial HTTP load and then initial web
	// socket connection. This should be used to create the initial state,
	// the socket Connected func will be true if the mount call is on a web
	// socket connection.
	h.HandleMount(thermoMount)

	// Provide a render function. Here we are doing it manually, but there is a
	// provided WithTemplateRenderer which can be used to work with `html/template`
	h.HandleRender(func(ctx context.Context, data *RenderContext) (io.Reader, error) {
		tmpl, err := template.New("thermo").Parse(`
            <div>{{.Assigns.C}}</div>
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
	})

	// This handles the `live-click="temp-up"` button. First we load the model from
	// the socket, increment the temperature, and then return the new state of the
	// model. Live will now calculate the diff between the last time it rendered and now,
	// produce a set of diffs and push them to the browser to update.
	h.HandleEvent("temp-up", tempUp)

	// This handles the `live-click="temp-down"` button.
	h.HandleEvent("temp-down", tempDown)

	http.Handle("/thermostat", NewHttpHandler(NewCookieStore("session-name", []byte("weak-secret")), h))

	// This serves the JS needed to make live work.
	http.Handle("/live.js", Javascript{})

	http.ListenAndServe(":8080", nil)
}
