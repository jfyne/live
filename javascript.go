package live

import (
	_ "embed"
	"net/http"
)

var (
	// JS is the contents of auto.js
	//go:embed web/browser/auto.js
	JS []byte

	// JSMap is the contents of auto.js.map
	//go:embed web/browser/auto.js.map
	JSMap []byte
)

// Javascript handles serving the client side
// portion of live.
type Javascript struct {
}

// ServeHTTP.
func (j Javascript) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/javascript")
	w.Write(JS)
}

// JavascriptMap handles serving source map.
type JavascriptMap struct {
}

// ServeHTTP.
func (j JavascriptMap) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	w.Write(JSMap)
}
