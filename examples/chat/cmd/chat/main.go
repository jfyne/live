package main

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/jfyne/live"
	"github.com/jfyne/live/examples/chat"
)

func main() {
	// Run the server.
	http.Handle("/", live.NewHttpHandler(context.Background(), chat.NewHandler()))
	http.Handle("/live.js", live.Javascript{})
	http.Handle("/auto.js.map", live.JavascriptMap{})
	slog.Info("server", "link", "http://localhost:8080")
	http.ListenAndServe(":8080", nil)
}
