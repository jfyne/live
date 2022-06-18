package page

import (
	"context"
	"net/http"

	"github.com/jfyne/live"
)

type Greeter struct {
	Name string

	Component
}

func NewGreeter(name string) *Greeter {
	return &Greeter{
		Name: name,
	}
}

func (g Greeter) Render() RenderFunc {
	return HTML(`
        <div class="greeter">Hello {{.Name}}</div>
        <script src="/live.js"></script>
    `, g)
}

func Example() {
	h := NewHandler(func(_ context.Context, _ *live.Handler, _ live.Socket) (ComponentLifecycle, error) {
		root := NewGreeter("World!")
		return root, nil
	})

	http.Handle("/", live.NewHttpHandler(live.NewCookieStore("session-name", []byte("weak-secret")), h))
	http.Handle("/live.js", live.Javascript{})
	http.ListenAndServe(":8080", nil)
}
