package live

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
)

// SSETransport implements Transport using Server-Sent Events.
type SSETransport struct {
	w        http.ResponseWriter
	flusher  http.Flusher
	in       chan Event
	closed   chan struct{}
}

// NewSSETransport creates a new SSE transport.
func NewSSETransport(w http.ResponseWriter) (*SSETransport, error) {
	f, ok := w.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("streaming not supported")
	}
	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Flush headers immediately
	f.Flush()

	return &SSETransport{
		w:       w,
		flusher: f,
		in:      make(chan Event, 16),
		closed:  make(chan struct{}),
	}, nil
}

func (t *SSETransport) ReadMessage(ctx context.Context) (Event, error) {
	select {
	case msg := <-t.in:
		return msg, nil
	case <-ctx.Done():
		return Event{}, ctx.Err()
	case <-t.closed:
		return Event{}, fmt.Errorf("transport closed")
	}
}

func (t *SSETransport) WriteMessage(ctx context.Context, msg Event) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	// SSE format: data: <payload>\n\n
	// JSON marshaled data shouldn't contain raw newlines, but if it did, it might break SSE framing.
	// `json.Marshal` escapes characters properly, so `data` (as bytes) shouldn't contain `\n` unless strictly formatted.

	if _, err := t.w.Write([]byte("data: ")); err != nil {
		return err
	}
	if _, err := t.w.Write(data); err != nil {
		return err
	}
	if _, err := t.w.Write([]byte("\n\n")); err != nil {
		return err
	}
	t.flusher.Flush()
	return nil
}

func (t *SSETransport) Close(ctx context.Context, reason string) error {
	select {
	case <-t.closed:
		// Already closed
	default:
		close(t.closed)
	}
	return nil
}

// PostMessage injects a message into the transport, to be read by ReadMessage.
func (t *SSETransport) PostMessage(msg Event) {
	select {
	case t.in <- msg:
	default:
		slog.Warn("dropping message to SSE transport, buffer full")
	}
}
