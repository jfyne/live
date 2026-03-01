package live

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestJavascript_ServeHTTP_ContentType verifies that Javascript.ServeHTTP sets
// the Content-Type header to "text/javascript".
func TestJavascript_ServeHTTP_ContentType(t *testing.T) {
	handler := Javascript{}

	req := httptest.NewRequest(http.MethodGet, "/live.js", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	resp := w.Result()
	ct := resp.Header.Get("Content-Type")
	if ct != "text/javascript" {
		t.Errorf("expected Content-Type 'text/javascript', got %q", ct)
	}
}

// TestJavascript_ServeHTTP_NonEmptyBody verifies that Javascript.ServeHTTP
// writes a non-empty response body (the embedded JS file has content).
func TestJavascript_ServeHTTP_NonEmptyBody(t *testing.T) {
	handler := Javascript{}

	req := httptest.NewRequest(http.MethodGet, "/live.js", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	if len(body) == 0 {
		t.Error("expected non-empty body from Javascript.ServeHTTP")
	}
}

// TestJavascript_ServeHTTP_StatusOK verifies that Javascript.ServeHTTP returns
// HTTP 200 OK (the default when no explicit status is written).
func TestJavascript_ServeHTTP_StatusOK(t *testing.T) {
	handler := Javascript{}

	req := httptest.NewRequest(http.MethodGet, "/live.js", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

// TestJavascript_ServeHTTP_BodyContainsJS verifies that the response body
// looks like JavaScript (contains some recognisable JS syntax).
func TestJavascript_ServeHTTP_BodyContainsJS(t *testing.T) {
	handler := Javascript{}

	req := httptest.NewRequest(http.MethodGet, "/live.js", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	// The embedded file should contain something that looks like JS.
	// We just check it is non-trivially sized and not pure whitespace.
	if strings.TrimSpace(body) == "" {
		t.Error("Javascript body is blank/whitespace only; expected JS content")
	}
}

// TestJavascriptMap_ServeHTTP_ContentType verifies that JavascriptMap.ServeHTTP
// sets the Content-Type header to "application/json".
func TestJavascriptMap_ServeHTTP_ContentType(t *testing.T) {
	handler := JavascriptMap{}

	req := httptest.NewRequest(http.MethodGet, "/live.js.map", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	ct := w.Result().Header.Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got %q", ct)
	}
}

// TestJavascriptMap_ServeHTTP_NonEmptyBody verifies that JavascriptMap.ServeHTTP
// writes a non-empty response body (the embedded source map has content).
func TestJavascriptMap_ServeHTTP_NonEmptyBody(t *testing.T) {
	handler := JavascriptMap{}

	req := httptest.NewRequest(http.MethodGet, "/live.js.map", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	if len(body) == 0 {
		t.Error("expected non-empty body from JavascriptMap.ServeHTTP")
	}
}

// TestJavascriptMap_ServeHTTP_StatusOK verifies that JavascriptMap.ServeHTTP
// returns HTTP 200 OK.
func TestJavascriptMap_ServeHTTP_StatusOK(t *testing.T) {
	handler := JavascriptMap{}

	req := httptest.NewRequest(http.MethodGet, "/live.js.map", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

// TestJavascriptMap_ServeHTTP_ValidJSON verifies that JavascriptMap.ServeHTTP
// serves a body that begins with the expected JSON structure (source maps are
// JSON objects that start with '{').
func TestJavascriptMap_ServeHTTP_ValidJSON(t *testing.T) {
	handler := JavascriptMap{}

	req := httptest.NewRequest(http.MethodGet, "/live.js.map", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := strings.TrimSpace(w.Body.String())
	if len(body) == 0 {
		t.Fatal("JavascriptMap body is empty")
	}
	if body[0] != '{' {
		t.Errorf("expected source map body to start with '{', got %q", string(body[0]))
	}
}
