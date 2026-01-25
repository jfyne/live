package live

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// SSETransport implements Transport interface using Server-Sent Events (SSE).
// It provides bidirectional communication using:
// - SSE (EventSource) for server-to-client streaming
// - HTTP POST for client-to-server messages
//
// Thread-safety: All public methods are safe for concurrent use.
type SSETransport struct {
	config TransportConfig
	meta   ConnectionMetadata

	// SSE response writer and flusher
	w       http.ResponseWriter
	flusher http.Flusher

	// Channel for incoming events from the client (via POST)
	events chan Event

	// Event ID tracking for reconnection
	lastEventID int
	eventIDMu   sync.Mutex

	// Context and cancellation
	ctx    context.Context
	cancel context.CancelFunc

	// Synchronization
	closeOnce sync.Once
	closed    chan struct{}

	// Write mutex protects concurrent writes to the SSE stream
	writeMu sync.Mutex

	// Heartbeat ticker for keepalive
	heartbeatTicker *time.Ticker
	heartbeatDone   chan struct{}
}

// NewSSETransport creates a new SSE transport from an HTTP response writer.
// The response writer must support the http.Flusher interface for SSE streaming.
func NewSSETransport(ctx context.Context, w http.ResponseWriter, r *http.Request, config TransportConfig, meta ConnectionMetadata) (*SSETransport, error) {
	// Check if the response writer supports flushing (required for SSE)
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("response writer does not support flushing")
	}

	transportCtx, cancel := context.WithCancel(ctx)

	bufferSize := config.EventBufferSize
	if bufferSize == 0 {
		bufferSize = DefaultTransportConfig().EventBufferSize
	}

	// Check for Last-Event-ID header for reconnection support
	lastEventID := 0
	if lastEventIDStr := r.Header.Get("Last-Event-ID"); lastEventIDStr != "" {
		if id, err := strconv.Atoi(lastEventIDStr); err == nil {
			lastEventID = id
			slog.Debug("sse reconnection detected", "last_event_id", lastEventID)
		}
	}

	t := &SSETransport{
		config:      config,
		meta:        meta,
		w:           w,
		flusher:     flusher,
		events:      make(chan Event, bufferSize),
		lastEventID: lastEventID,
		ctx:         transportCtx,
		cancel:      cancel,
		closed:      make(chan struct{}),
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering

	// Start heartbeat to keep connection alive
	if config.PingInterval > 0 {
		t.heartbeatTicker = time.NewTicker(config.PingInterval)
		t.heartbeatDone = make(chan struct{})
		go t.heartbeatPump()
	}

	return t, nil
}

// Send transmits an event to the client over the SSE stream.
// This method is thread-safe and can be called concurrently.
func (t *SSETransport) Send(event Event) error {
	select {
	case <-t.closed:
		return fmt.Errorf("transport closed")
	default:
	}

	t.writeMu.Lock()
	defer t.writeMu.Unlock()

	// Increment event ID
	t.eventIDMu.Lock()
	t.lastEventID++
	eventID := t.lastEventID
	t.eventIDMu.Unlock()

	// Marshal event to JSON
	data, err := json.Marshal(&event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	// Write SSE format:
	// id: <eventID>
	// data: <json>
	// (blank line)
	if _, err := fmt.Fprintf(t.w, "id: %d\n", eventID); err != nil {
		return fmt.Errorf("write event id: %w", err)
	}

	if _, err := fmt.Fprintf(t.w, "data: %s\n\n", data); err != nil {
		return fmt.Errorf("write event data: %w", err)
	}

	// Flush to ensure the event is sent immediately
	t.flusher.Flush()

	return nil
}

// Events returns a receive-only channel of events from the client.
// The channel is closed when the transport is closed.
func (t *SSETransport) Events() <-chan Event {
	return t.events
}

// Close terminates the SSE connection and cleans up resources.
// It is safe to call Close multiple times.
func (t *SSETransport) Close() error {
	var err error
	t.closeOnce.Do(func() {
		// Cancel the context to signal all goroutines to stop
		t.cancel()

		// Close the closed channel to signal that we're shutting down
		close(t.closed)

		// Stop heartbeat ticker and wait for heartbeat goroutine to finish
		if t.heartbeatTicker != nil {
			t.heartbeatTicker.Stop()
			// Wait for heartbeat goroutine to finish
			select {
			case <-t.heartbeatDone:
				// Heartbeat finished
			case <-time.After(100 * time.Millisecond):
				// Timeout waiting for heartbeat
			}
		}

		// Close the events channel after a brief delay to allow pending events to be read
		// This is done in a goroutine to avoid blocking
		go func() {
			time.Sleep(100 * time.Millisecond)
			close(t.events)
		}()
	})
	return err
}

// heartbeatPump sends periodic comments to keep the SSE connection alive
// and detect dead connections.
func (t *SSETransport) heartbeatPump() {
	defer close(t.heartbeatDone)

	for {
		select {
		case <-t.heartbeatTicker.C:
			// Check if closed before attempting to write
			select {
			case <-t.closed:
				return
			case <-t.ctx.Done():
				return
			default:
			}

			t.writeMu.Lock()
			// Double-check after acquiring lock
			select {
			case <-t.closed:
				t.writeMu.Unlock()
				return
			case <-t.ctx.Done():
				t.writeMu.Unlock()
				return
			default:
			}

			// Send a comment (lines starting with : are comments in SSE)
			if _, err := fmt.Fprintf(t.w, ": heartbeat\n\n"); err != nil {
				t.writeMu.Unlock()
				slog.Debug("heartbeat failed, closing connection", "err", err)
				t.Close()
				return
			}
			t.flusher.Flush()
			t.writeMu.Unlock()

		case <-t.ctx.Done():
			return
		case <-t.closed:
			return
		}
	}
}

// receiveEvent is called by the POST handler to deliver events from the client.
// This is an internal method used to bridge the POST endpoint to the transport.
func (t *SSETransport) receiveEvent(event Event) error {
	select {
	case <-t.closed:
		return fmt.Errorf("transport closed")
	case <-t.ctx.Done():
		return fmt.Errorf("transport context done")
	case t.events <- event:
		return nil
	}
}

// SSETransportFactory creates SSE transports by upgrading HTTP connections.
type SSETransportFactory struct {
	config TransportConfig
	// Map to track active SSE transports by session ID
	// This is needed to route POST requests to the correct transport
	transports   map[string]*SSETransport
	transportsMu sync.RWMutex
}

// NewSSETransportFactory creates a new SSE transport factory with the given configuration.
func NewSSETransportFactory(config TransportConfig) *SSETransportFactory {
	return &SSETransportFactory{
		config:     config,
		transports: make(map[string]*SSETransport),
	}
}

// Upgrade converts an HTTP request into an SSE transport.
// It sets up the SSE stream for server-to-client events.
// Client-to-server events should be sent via POST to HandlePost.
func (f *SSETransportFactory) Upgrade(ctx context.Context, w http.ResponseWriter, r *http.Request) (Transport, error) {
	// Build connection metadata
	meta := ConnectionMetadata{
		RemoteAddr:  r.RemoteAddr,
		UserAgent:   r.UserAgent(),
		Protocol:    "sse",
		ConnectedAt: time.Now(),
		Headers:     r.Header,
	}

	// Create the transport
	transport, err := NewSSETransport(ctx, w, r, f.config, meta)
	if err != nil {
		return nil, fmt.Errorf("create sse transport: %w", err)
	}

	// Get or generate session ID from cookies
	// In a real implementation, this would be coordinated with the session manager
	sessionID := getSessionIDFromRequest(r)
	if sessionID != "" {
		f.transportsMu.Lock()
		f.transports[sessionID] = transport
		f.transportsMu.Unlock()

		// Clean up when transport closes
		go func() {
			<-transport.ctx.Done()
			f.transportsMu.Lock()
			delete(f.transports, sessionID)
			f.transportsMu.Unlock()
		}()
	}

	// Send the initial connect event to the client
	if err := transport.Send(Event{T: EventConnect}); err != nil {
		return nil, fmt.Errorf("send connect event: %w", err)
	}

	return transport, nil
}

// HandlePost handles POST requests from the client and routes events to the appropriate transport.
// This should be mounted at a separate endpoint from the SSE stream.
func (f *SSETransportFactory) HandlePost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get session ID from request
	sessionID := getSessionIDFromRequest(r)
	if sessionID == "" {
		http.Error(w, "session not found", http.StatusBadRequest)
		return
	}

	// Find the transport for this session
	f.transportsMu.RLock()
	transport, ok := f.transports[sessionID]
	f.transportsMu.RUnlock()

	if !ok {
		http.Error(w, "transport not found", http.StatusNotFound)
		return
	}

	// Read the event from the request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Parse the event
	var event Event
	if err := json.Unmarshal(body, &event); err != nil {
		http.Error(w, "invalid event json", http.StatusBadRequest)
		return
	}

	// Deliver the event to the transport
	if err := transport.receiveEvent(event); err != nil {
		http.Error(w, "failed to deliver event", http.StatusInternalServerError)
		return
	}

	// Return success
	w.WriteHeader(http.StatusOK)
}

// getSessionIDFromRequest extracts the session ID from the request.
// This looks for a session cookie or header.
func getSessionIDFromRequest(r *http.Request) string {
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
