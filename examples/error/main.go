package main

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"log/slog"
	"net/http"

	"github.com/jfyne/live"
)

const (
	problem = "problem"
)

func main() {
	t, err := template.ParseFiles("root.html", "error/view.html")
	if err != nil {
		log.Fatal(err)
	}

	h := live.NewHandler(live.WithTemplateRenderer(t))

	// Uncomment the below to see the server respond with an error immediately.

	//h.HandleMount(func(ctx context.Context, s live.Socket) (any, error) {
	//	return nil, fmt.Errorf("mount failure")
	//})

	h.ErrorHandler = func(ctx context.Context, err error) {
		w := live.Writer(ctx)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("this is a bad request: " + err.Error()))
	}

	h.HandleEvent(problem, func(ctx context.Context, s *live.Socket, _ live.Params) (any, error) {
		return nil, fmt.Errorf("hello")
	})

	http.Handle("/", live.NewHttpHandler(context.Background(), h))
	http.Handle("/live.js", live.Javascript{})
	http.Handle("/auto.js.map", live.JavascriptMap{})
	slog.Info("server", "link", "http://localhost:8080")
	http.ListenAndServe(":8080", nil)
}
