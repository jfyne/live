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

func writeTimeout(ctx context.Context, timeout time.Duration, c *websocket.Conn, msg Event) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	data, err := json.Marshal(&msg)
	if err != nil {
		return fmt.Errorf("failed writeTimeout: %w", err)
	}

	return c.Write(ctx, websocket.MessageText, data)
}
