package main

import (
	"context"
	"net/http"

	"github.com/jfyne/live"
	"github.com/jfyne/live/examples"
	"github.com/jfyne/live/page"
)

func main() {
	h := page.NewHandler(func(_ context.Context, _ *live.Handler, _ live.Socket) (page.Component, error) {
		root := examples.NewApp()
		return root, nil
	})

	http.Handle("/", live.NewHttpHandler(live.NewCookieStore("session-name", []byte("weak-secret")), h))
	http.Handle("/live.js", live.Javascript{})
	http.ListenAndServe(":8080", nil)
}
