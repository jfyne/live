package page

import (
	"context"
	"io"
	"net/http"

	"github.com/jfyne/live"
)

// NewGreeter creates a new greeter component.
func NewGreeter(ID string, h live.Handler, s live.Socket, name string) (*Component, error) {
	return NewComponent(
		ID,
		h,
		s,
		WithMount(func(ctx context.Context, c *Component) error {
			c.State = name
			return nil
		}),
		WithRender(func(w io.Writer, c *Component) error {
			// Render the greeter, here we are including the script just to make this toy example work.
			return HTML(`
                <div class="greeter">Hello {{.}}</div>
                <script src="/live.js"></script>
            `, c).Render(w)
		}),
	)
}

func Example() {
	h := live.NewHandler(
		WithComponentMount(func(ctx context.Context, h live.Handler, s live.Socket) (*Component, error) {
			return NewGreeter("hello-id", h, s, "World!")
		}),
		WithComponentRenderer(),
	)

	http.Handle("/", live.NewHttpHandler(live.NewCookieStore("session-name", []byte("weak-secret")), h))
	http.Handle("/live.js", live.Javascript{})
	http.ListenAndServe(":8080", nil)
}
