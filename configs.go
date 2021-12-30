package live

import (
	"bytes"
	"context"
	"html/template"
	"io"
)

// WithTemplateRenderer set the handler to use an `html/template` renderer.
func WithTemplateRenderer(t *template.Template) HandlerConfig {
	return func(h Handler) error {
		h.HandleRender(func(ctx context.Context, data interface{}) (io.Reader, error) {
			var buf bytes.Buffer
			if err := t.Execute(&buf, data); err != nil {
				return nil, err
			}
			return &buf, nil
		})
		return nil
	}
}
