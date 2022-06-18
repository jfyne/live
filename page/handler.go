package page

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/jfyne/live"
)

// ComponentConstructor a func for creating a new component.
type ComponentConstructor func(ctx context.Context, h *live.Handler, s live.Socket) (ComponentLifecycle, error)

// NewHandler creates a new handler for components.
func NewHandler(construct ComponentConstructor) *live.Handler {
	return live.NewHandler(
		withComponentMount(construct),
		withComponentRenderer(),
	)
}

// withComponentMount set the live.Handler to mount the root component.
func withComponentMount(construct ComponentConstructor) live.HandlerConfig {
	return func(h *live.Handler) error {
		h.HandleMount(func(ctx context.Context, s live.Socket) (any, error) {
			comp, err := construct(ctx, h, s)
			if err != nil {
				return nil, fmt.Errorf("could not create root component: %w", err)
			}
			comp.init("root", h, s)
			if s.Connected() {
				if err := comp.register("root", h, s, comp); err != nil {
					return nil, err
				}
			}
			if err := comp.Mount(ctx); err != nil {
				return nil, err
			}
			return comp, nil
		})
		return nil
	}
}

// withComponentRenderer set the live.Handler to use a root component to render.
func withComponentRenderer() live.HandlerConfig {
	return func(h *live.Handler) error {
		h.HandleRender(func(_ context.Context, data *live.RenderContext) (io.Reader, error) {
			c, ok := data.Assigns.(ComponentLifecycle)
			if !ok {
				return nil, fmt.Errorf("root render data is not a component")
			}
			c._assignUploads(data.Uploads)
			var buf bytes.Buffer
			if err := c.Render()(&buf); err != nil {
				return nil, err
			}
			return &buf, nil
		})
		return nil
	}
}
