package live

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"io"

	"golang.org/x/net/html"
)

// RenderContext contains the sockets current data for rendering.
type RenderContext struct {
	Socket  Socket
	Uploads UploadContext
	Assigns interface{}
}

// RenderSocket takes the engine and current socket and renders it to html.
func RenderSocket(ctx context.Context, e Engine, s Socket) (*html.Node, error) {
	rc := &RenderContext{
		Socket:  s,
		Uploads: s.Uploads(),
		Assigns: s.Assigns(),
	}

	output, err := e.Render()(ctx, rc)
	if err != nil {
		return nil, fmt.Errorf("render error: %w", err)
	}
	render, err := html.Parse(output)
	if err != nil {
		return nil, fmt.Errorf("html parse error: %w", err)
	}
	shapeTree(render)

	if s.LatestRender() != nil {
		patches, err := Diff(s.LatestRender(), render)
		if err != nil {
			return nil, fmt.Errorf("diff error: %w", err)
		}
		if len(patches) != 0 {
			s.Send(EventPatch, patches)
		}
	} else {
		anchorTree(render, newAnchorGenerator())
	}

	return render, nil
}

// WithTemplateRenderer set the handler to use an `html/template` renderer.
func WithTemplateRenderer(t *template.Template) HandlerConfig {
	return func(h Handler) error {
		h.HandleRender(func(ctx context.Context, rc *RenderContext) (io.Reader, error) {
			var buf bytes.Buffer
			if err := t.Execute(&buf, rc); err != nil {
				return nil, err
			}
			return &buf, nil
		})
		return nil
	}
}
