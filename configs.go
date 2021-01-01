package live

import (
	"bytes"
	"context"
	"html/template"
	"io"
)

// WithRootTemplate set the renderer to use a different root template. This changes the handlers
// Render function.
func WithRootTemplate(rootTemplate string) HandlerConfig {
	return func(h *Handler) error {
		h.Render = func(ctx context.Context, t *template.Template, data interface{}) (io.Reader, error) {
			var buf bytes.Buffer
			if err := t.ExecuteTemplate(&buf, rootTemplate, data); err != nil {
				return nil, err
			}
			return &buf, nil
		}
		return nil
	}
}
