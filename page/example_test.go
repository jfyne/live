package page

import (
	"context"
	"io"
	"log"
	"net/http"

	"github.com/jfyne/live"
)

// NewGreeter creates a new greeter component.
func NewGreeter(ID string, h *live.Handler, s *live.Socket, name string) (Component, error) {
	return NewComponent(
		ID,
		h,
		s,
		WithMount(func(ctx context.Context, c *Component, r *http.Request, connected bool) error {
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
	h, err := live.NewHandler(
		live.NewCookieStore("session-name", []byte("weak-secret")),
		WithComponentMount(func(ctx context.Context, h *live.Handler, r *http.Request, s *live.Socket) (Component, error) {
			return NewGreeter("hello-id", h, s, "World!")
		}),
		WithComponentRenderer(),
	)
	if err != nil {
		log.Fatal(err)
	}

	http.Handle("/", h)
	http.Handle("/live.js", live.Javascript{})
	http.ListenAndServe(":8080", nil)
}
