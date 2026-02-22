package live

import (
	"net/http"
	"testing"
)

func TestGetSessionIDFromRequest(t *testing.T) {
	t.Run("extracts session ID from cookie", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "http://example.com", nil)
		req.AddCookie(&http.Cookie{Name: "live_session", Value: "abc-123"})

		got := GetSessionIDFromRequest(req)
		if got != "abc-123" {
			t.Errorf("expected abc-123, got %s", got)
		}
	})

	t.Run("extracts session ID from header", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "http://example.com", nil)
		req.Header.Set("X-Live-Session", "header-456")

		got := GetSessionIDFromRequest(req)
		if got != "header-456" {
			t.Errorf("expected header-456, got %s", got)
		}
	})

	t.Run("cookie takes priority over header", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "http://example.com", nil)
		req.AddCookie(&http.Cookie{Name: "live_session", Value: "cookie-val"})
		req.Header.Set("X-Live-Session", "header-val")

		got := GetSessionIDFromRequest(req)
		if got != "cookie-val" {
			t.Errorf("expected cookie-val, got %s", got)
		}
	})

	t.Run("returns empty when neither exists", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "http://example.com", nil)

		got := GetSessionIDFromRequest(req)
		if got != "" {
			t.Errorf("expected empty string, got %s", got)
		}
	})
}
