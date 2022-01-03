package page

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/jfyne/live"
)

// ComponentConfig configures a component.
type ComponentConfig func(c *Component) error

// WithRegister set a register handler on the component.
func WithRegister(fn RegisterHandler) ComponentConfig {
	return func(c *Component) error {
		c.Register = fn
		return nil
	}
}

// WithMount set a mounnt handler on the component.
func WithMount(fn MountHandler) ComponentConfig {
	return func(c *Component) error {
		c.Mount = fn
		return nil
	}
}

// WithRender set a render handler on the component.
func WithRender(fn RenderHandler) ComponentConfig {
	return func(c *Component) error {
		c.Render = fn
		return nil
	}
}

// WithComponentMount set the live.Handler to mount the root component.
func WithComponentMount(construct ComponentConstructor) live.HandlerConfig {
	return func(h live.Handler) error {
		h.HandleMount(func(ctx context.Context, s live.Socket) (interface{}, error) {
			root, err := construct(ctx, h, s)
			if err != nil {
				return nil, fmt.Errorf("failed to construct root component: %w", err)
			}
			if s.Connected() {
				if err := root.Register(root); err != nil {
					return nil, err
				}
			}
			if err := root.Mount(ctx, root); err != nil {
				return nil, err
			}
			return root, nil
		})
		return nil
	}
}

// WithComponentRenderer set the live.Handler to use a root component to render.
func WithComponentRenderer() live.HandlerConfig {
	return func(h live.Handler) error {
		h.HandleRender(func(_ context.Context, data interface{}) (io.Reader, error) {
			c, ok := data.(*Component)
			if !ok {
				return nil, fmt.Errorf("root render data is not a component")
			}
			var buf bytes.Buffer
			if err := c.Render(&buf, c); err != nil {
				return nil, err
			}
			return &buf, nil
		})
		return nil
	}
}
