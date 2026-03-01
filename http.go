package live

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/coder/websocket"
)

func httpContext(w http.ResponseWriter, r *http.Request) context.Context {
	ctx := r.Context()
	ctx = contextWithRequest(ctx, r)
	ctx = contextWithWriter(ctx, w)
	return ctx
}

// GetSessionIDFromRequest extracts the session ID from the request.
// It first checks for a "live_session" cookie, then falls back to the
// "X-Live-Session" header. Returns an empty string if neither is present.
func GetSessionIDFromRequest(r *http.Request) string {
	// Try to get session ID from cookie
	if cookie, err := r.Cookie("live_session"); err == nil {
		return cookie.Value
	}

	// Try to get session ID from header
	if sessionID := r.Header.Get("X-Live-Session"); sessionID != "" {
		return sessionID
	}

	return ""
}

func writeTimeout(ctx context.Context, timeout time.Duration, c *websocket.Conn, msg Event) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	data, err := json.Marshal(&msg)
	if err != nil {
		return fmt.Errorf("failed writeTimeout: %w", err)
	}

	return c.Write(ctx, websocket.MessageText, data)
}
