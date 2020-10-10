package live

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"io"
)

// MountHandler when mount is reached.
type MountHandler func(ctx context.Context, params map[string]string, c *Socket, connected bool) error

// RenderHandler when the view is asked to render.
type RenderHandler func(ctx context.Context, t *template.Template, c *Socket) (io.Reader, error)

// ViewOption applies config to a view.
type ViewConfig func(v *View) error

// ViewEvent an event sent by the view to the server.
type ViewEvent struct {
	S   *Socket
	Msg SocketMessage
}

// View to be handled by the server.
type View struct {
	// The path that this view lives at. Essentially the
	// ID for the view.
	path string
	t    *template.Template

	// emitter is a channel to send messages back to the server.
	emitter chan ViewEvent

	// eventHandlers the map of event handlers.
	eventHandlers map[Event]EventHandler

	Mount  MountHandler
	Render RenderHandler
}

// NewView creates a new live view.
func NewView(path string, files []string, configs ...ViewConfig) (*View, error) {
	t, err := template.ParseFiles(files...)
	if err != nil {
		return nil, fmt.Errorf("could not create view: %w", err)
	}
	v := &View{
		t:             t,
		path:          path,
		emitter:       make(chan ViewEvent),
		eventHandlers: make(map[Event]EventHandler),
		Mount: func(ctx context.Context, params map[string]string, c *Socket, connected bool) error {
			return nil
		},
		Render: func(ctx context.Context, t *template.Template, c *Socket) (io.Reader, error) {
			var buf bytes.Buffer
			if err := t.ExecuteTemplate(&buf, "base.html", c.Data); err != nil {
				return nil, err
			}
			return &buf, nil
		},
	}

	for _, conf := range configs {
		if err := conf(v); err != nil {
			return nil, fmt.Errorf("could not apply config: %w", err)
		}
	}

	return v, nil
}

// HandleEvent handles an event.
func (v *View) HandleEvent(e Event, handler EventHandler) {
	v.eventHandlers[e] = handler
}

// handleEvent route an event to the correct handler.
func (v View) handleEvent(e Event, sock *Socket, msg SocketMessage) error {
	handler, ok := v.eventHandlers[e]
	if !ok {
		return fmt.Errorf("no event handler for %s: %w", e, ErrNoEventHandler)
	}

	if err := handler(sock, msg); err != nil {
		return fmt.Errorf("view event handler error [%s]: %w", e, err)
	}

	return nil
}

// Emit emits an event to the server.
func (v View) Emit(sock *Socket, msg SocketMessage) {
	v.emitter <- ViewEvent{
		S:   sock,
		Msg: msg,
	}
}
