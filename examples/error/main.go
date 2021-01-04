package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"

	"github.com/jfyne/live"
)

const (
	problem = "problem"
)

func main() {
	t, err := template.ParseFiles("examples/root.html", "examples/error/view.html")
	if err != nil {
		log.Fatal(err)
	}

	h, err := live.NewHandler(t, live.NewCookieStore("session-name", []byte("weak-secret")))
	if err != nil {
		log.Fatal(err)
	}

	h.HandleEvent(problem, func(s *live.Socket, _ map[string]interface{}) (interface{}, error) {
		return nil, fmt.Errorf("hello")
	})

	http.Handle("/error", h)
	http.Handle("/live.js", live.Javascript{})
	http.Handle("/auto.js.map", live.JavascriptMap{})
	http.ListenAndServe(":8080", nil)
}
