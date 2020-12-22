package live

import (
	"net/http"

	"github.com/jfyne/live/internal/embed"
)

// Javascript handles serving the client side
// portion of live.
type Javascript struct {
}

// ServeHTTP.
func (j Javascript) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/javascript")
	w.Write(embed.Get("/auto.js"))
}

// JavascriptMap handles serving source map.
type JavascriptMap struct {
}

// ServeHTTP.
func (j JavascriptMap) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	w.Write(embed.Get("/auto.js.map"))
}
