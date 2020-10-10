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

// View to be handled by the server.
type View struct {
	// The path that this view lives at. Essentially the
	// ID for the view.
	path string
	t    *template.Template

	// emitter is a channel to send messages back to the server.
	emitter chan SocketMessage

	// handleEvent called to handle an incoming event.
	handleEvent EventHandler

	Mount  MountHandler
	Render RenderHandler
}

func NewView(path string, files []string, configs ...ViewConfig) (*View, error) {
	t, err := template.ParseFiles(files...)
	if err != nil {
		return nil, fmt.Errorf("could not create view: %w", err)
	}
	v := &View{
		t:           t,
		path:        path,
		emitter:     make(chan SocketMessage),
		handleEvent: func(e Event, c *Socket) {},
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

func (v *View) 
