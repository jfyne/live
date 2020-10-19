package live

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"io"
)

// MountHandler when mount is reached.
type MountHandler func(ctx context.Context, view *View, params map[string]string, c *Socket, connected bool) error

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

	// selfHandlers handle messages send to this view by server-side
	// entities.
	selfHandlers map[Event]EventHandler

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
		selfHandlers:  make(map[Event]EventHandler),
		Mount: func(ctx context.Context, view *View, params map[string]string, c *Socket, connected bool) error {
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

// HandleEvent handles an event that comes from the client. For example a click.
func (v *View) HandleEvent(e Event, handler EventHandler) {
	v.eventHandlers[e] = handler
}

// HandleSelf handles an event that comes from the view. This enables us to push
// updates if needed.
func (v *View) HandleSelf(e Event, handler EventHandler) {
	v.selfHandlers[e] = handler
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

// handleSelf route an event to the correct handler.
func (v View) handleSelf(e Event, sock *Socket, msg SocketMessage) error {
	handler, ok := v.selfHandlers[e]
	if !ok {
		return fmt.Errorf("no event handler for %s: %w", e, ErrNoEventHandler)
	}

	if err := handler(sock, msg); err != nil {
		return fmt.Errorf("view event handler error [%s]: %w", e, err)
	}

	return nil
}

// Self sends a message to the view to action something on the socket.
func (v View) Self(sock *Socket, msg SocketMessage) {
	v.emitter <- ViewEvent{
		S:   sock,
		Msg: msg,
	}
}
