package live

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestSSETransport_Upgrade tests the SSE upgrade handshake.
func TestSSETransport_Upgrade(t *testing.T) {
	config := DefaultTransportConfig()
	factory := NewSSETransportFactory(config)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set a session cookie for testing
		http.SetCookie(w, &http.Cookie{
			Name:  "live_session",
			Value: "test-session-123",
		})

		transport, err := factory.Upgrade(r.Context(), w, r)
		if err != nil {
			t.Errorf("upgrade failed: %v", err)
			return
		}
		defer transport.Close()

		// Keep connection alive briefly
		time.Sleep(100 * time.Millisecond)
	}))
	defer server.Close()

	// Make a GET request to establish SSE connection
	req, err := http.NewRequest("GET", server.URL, nil)
	if err != nil {
		t.Fatalf("create request failed: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// Verify SSE headers
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("expected Content-Type text/event-stream, got %s", ct)
	}

	if cc := resp.Header.Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("expected Cache-Control no-cache, got %s", cc)
	}

	// Read the connect event
	scanner := bufio.NewScanner(resp.Body)
	var eventData string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			eventData = strings.TrimPrefix(line, "data: ")
			break
		}
	}

	if eventData == "" {
		t.Fatal("no event data received")
	}

	var event Event
	if err := json.Unmarshal([]byte(eventData), &event); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if event.T != EventConnect {
		t.Errorf("expected connect event, got %s", event.T)
	}
}

// TestSSETransport_SendEvent tests sending events from server to client.
func TestSSETransport_SendEvent(t *testing.T) {
	config := DefaultTransportConfig()
	factory := NewSSETransportFactory(config)

	// Channel to pass the transport to the test
	transportCh := make(chan Transport, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{
			Name:  "live_session",
			Value: "test-session-456",
		})

		transport, err := factory.Upgrade(r.Context(), w, r)
		if err != nil {
			t.Errorf("upgrade failed: %v", err)
			return
		}

		transportCh <- transport

		// Keep connection alive
		<-r.Context().Done()
	}))
	defer server.Close()

	// Start reading the SSE stream in a goroutine
	eventsCh := make(chan Event, 10)
	errCh := make(chan error, 1)
	clientDone := make(chan struct{})

	go func() {
		defer close(clientDone)
		req, _ := http.NewRequest("GET", server.URL, nil)
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			errCh <- err
			return
		}
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "data: ") {
				eventData := strings.TrimPrefix(line, "data: ")
				var event Event
				if err := json.Unmarshal([]byte(eventData), &event); err != nil {
					errCh <- err
					return
				}
				eventsCh <- event
			}
		}

		if err := scanner.Err(); err != nil && err != io.EOF {
			errCh <- err
		}
	}()

	// Wait for the transport
	var serverTransport Transport
	select {
	case serverTransport = <-transportCh:
	case err := <-errCh:
		t.Fatalf("client error: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for transport")
	}
	defer serverTransport.Close()

	// Read the connect event
	select {
	case event := <-eventsCh:
		if event.T != EventConnect {
			t.Errorf("expected connect event, got %s", event.T)
		}
	case err := <-errCh:
		t.Fatalf("client error: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for connect event")
	}

	// Send a custom event
	testEvent := Event{
		T:    "test-event",
		ID:   42,
		Data: json.RawMessage(`{"message":"hello from server"}`),
	}

	if err := serverTransport.Send(testEvent); err != nil {
		t.Fatalf("send failed: %v", err)
	}

	// Client receives the event
	select {
	case receivedEvent := <-eventsCh:
		if receivedEvent.T != testEvent.T {
			t.Errorf("expected event type %s, got %s", testEvent.T, receivedEvent.T)
		}
		if receivedEvent.ID != testEvent.ID {
			t.Errorf("expected event ID %d, got %d", testEvent.ID, receivedEvent.ID)
		}
	case err := <-errCh:
		t.Fatalf("client error: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for test event")
	}
}

// TestSSETransport_POSTEvent tests receiving events from client via POST.
func TestSSETransport_POSTEvent(t *testing.T) {
	config := DefaultTransportConfig()
	factory := NewSSETransportFactory(config)

	// Create a test server with both SSE and POST endpoints
	mux := http.NewServeMux()

	transportCh := make(chan Transport, 1)

	// SSE endpoint
	mux.HandleFunc("/sse", func(w http.ResponseWriter, r *http.Request) {
		// Set session cookie before upgrade
		http.SetCookie(w, &http.Cookie{
			Name:  "live_session",
			Value: "test-session-789",
			Path:  "/",
		})

		transport, err := factory.Upgrade(r.Context(), w, r)
		if err != nil {
			t.Errorf("upgrade failed: %v", err)
			return
		}

		transportCh <- transport

		// Keep connection alive
		<-r.Context().Done()
	})

	// POST endpoint
	mux.HandleFunc("/post", func(w http.ResponseWriter, r *http.Request) {
		factory.HandlePost(w, r)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	// Cookie jar for session management
	jar := &testCookieJar{cookies: make(map[string][]*http.Cookie)}
	client := &http.Client{
		Jar:     jar,
		Timeout: 5 * time.Second,
	}

	// Establish SSE connection with session cookie
	done := make(chan struct{})
	go func() {
		defer close(done)
		req, _ := http.NewRequest("GET", server.URL+"/sse", nil)
		// Set session cookie on initial request
		req.AddCookie(&http.Cookie{
			Name:  "live_session",
			Value: "test-session-789",
		})
		resp, err := client.Do(req)
		if err != nil {
			return
		}
		defer resp.Body.Close()

		io.ReadAll(resp.Body) // Keep connection alive
	}()

	// Wait for the transport
	var serverTransport Transport
	select {
	case serverTransport = <-transportCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for transport")
	}
	defer serverTransport.Close()

	// Give some time for session registration
	time.Sleep(100 * time.Millisecond)

	// Send event from client via POST
	clientEvent := Event{
		T:    "client-event",
		ID:   123,
		Data: json.RawMessage(`{"message":"hello from client"}`),
	}

	eventData, _ := json.Marshal(clientEvent)
	postReq, _ := http.NewRequest("POST", server.URL+"/post", bytes.NewReader(eventData))

	// Add the session cookie explicitly
	postReq.AddCookie(&http.Cookie{
		Name:  "live_session",
		Value: "test-session-789",
	})

	resp, err := client.Do(postReq)
	if err != nil {
		t.Fatalf("POST request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("expected status 200, got %d: %s", resp.StatusCode, string(body))
	}

	// Server receives the event
	select {
	case receivedEvent := <-serverTransport.Events():
		if receivedEvent.T != clientEvent.T {
			t.Errorf("expected event type %s, got %s", clientEvent.T, receivedEvent.T)
		}
		if receivedEvent.ID != clientEvent.ID {
			t.Errorf("expected event ID %d, got %d", clientEvent.ID, receivedEvent.ID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for client event")
	}
}

// testCookieJar is a simple cookie jar for testing
type testCookieJar struct {
	mu      sync.Mutex
	cookies map[string][]*http.Cookie
}

func (j *testCookieJar) SetCookies(u *url.URL, cookies []*http.Cookie) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.cookies[u.Host] = cookies
}

func (j *testCookieJar) Cookies(u *url.URL) []*http.Cookie {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.cookies[u.Host]
}

// TestSSETransport_LastEventID tests reconnection with Last-Event-ID header.
func TestSSETransport_LastEventID(t *testing.T) {
	config := DefaultTransportConfig()
	factory := NewSSETransportFactory(config)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{
			Name:  "live_session",
			Value: "test-session-reconnect",
		})

		transport, err := factory.Upgrade(r.Context(), w, r)
		if err != nil {
			t.Errorf("upgrade failed: %v", err)
			return
		}
		defer transport.Close()

		// Send a few events
		for i := 1; i <= 3; i++ {
			event := Event{
				T:    "event",
				ID:   i,
				Data: json.RawMessage(fmt.Sprintf(`{"number":%d}`, i)),
			}
			if err := transport.Send(event); err != nil {
				t.Errorf("send failed: %v", err)
				return
			}
		}

		time.Sleep(100 * time.Millisecond)
	}))
	defer server.Close()

	// First connection - read events
	req1, _ := http.NewRequest("GET", server.URL, nil)
	resp1, err := http.DefaultClient.Do(req1)
	if err != nil {
		t.Fatalf("first request failed: %v", err)
	}

	scanner := bufio.NewScanner(resp1.Body)
	lastEventID := ""
	eventCount := 0

	// Read until we get 2 events (connect + first event)
	for scanner.Scan() && eventCount < 2 {
		line := scanner.Text()
		if strings.HasPrefix(line, "id: ") {
			lastEventID = strings.TrimPrefix(line, "id: ")
			eventCount++
		}
	}
	resp1.Body.Close()

	if lastEventID == "" {
		t.Fatal("no event ID received")
	}

	// Reconnect with Last-Event-ID header
	req2, _ := http.NewRequest("GET", server.URL, nil)
	req2.Header.Set("Last-Event-ID", lastEventID)

	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("reconnect request failed: %v", err)
	}
	defer resp2.Body.Close()

	// The server should have received the Last-Event-ID header
	// and started from that point (though in this simple implementation,
	// we don't actually replay events - we just log it)
	scanner2 := bufio.NewScanner(resp2.Body)
	reconnectEventReceived := false

	for scanner2.Scan() {
		line := scanner2.Text()
		if strings.HasPrefix(line, "data: ") {
			reconnectEventReceived = true
			break
		}
	}

	if !reconnectEventReceived {
		t.Error("no event received after reconnection")
	}
}

// TestSSETransport_ConcurrentSends tests thread safety with concurrent sends.
func TestSSETransport_ConcurrentSends(t *testing.T) {
	config := DefaultTransportConfig()
	factory := NewSSETransportFactory(config)

	transportCh := make(chan Transport, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{
			Name:  "live_session",
			Value: "test-session-concurrent",
		})

		transport, err := factory.Upgrade(r.Context(), w, r)
		if err != nil {
			t.Errorf("upgrade failed: %v", err)
			return
		}

		transportCh <- transport
		<-r.Context().Done()
	}))
	defer server.Close()

	// Start reading the SSE stream
	eventsCh := make(chan Event, 200)
	go func() {
		req, _ := http.NewRequest("GET", server.URL, nil)
		client := &http.Client{Timeout: 10 * time.Second}
		resp, _ := client.Do(req)
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "data: ") {
				eventData := strings.TrimPrefix(line, "data: ")
				var event Event
				json.Unmarshal([]byte(eventData), &event)
				eventsCh <- event
			}
		}
	}()

	// Wait for transport
	var serverTransport Transport
	select {
	case serverTransport = <-transportCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for transport")
	}
	defer serverTransport.Close()

	// Read connect event
	<-eventsCh

	// Send many messages concurrently
	const numGoroutines = 10
	const messagesPerGoroutine = 10

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	errors := make(chan error, numGoroutines*messagesPerGoroutine)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < messagesPerGoroutine; j++ {
				event := Event{
					T:    fmt.Sprintf("event-%d-%d", id, j),
					ID:   id*1000 + j,
					Data: json.RawMessage(fmt.Sprintf(`{"id":%d,"msg":%d}`, id, j)),
				}
				if err := serverTransport.Send(event); err != nil {
					errors <- err
				}
			}
		}(i)
	}

	// Wait for all sends to complete
	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("send error: %v", err)
	}

	// Read all the messages from the client side
	received := 0
	timeout := time.After(5 * time.Second)

	for received < numGoroutines*messagesPerGoroutine {
		select {
		case <-eventsCh:
			received++
		case <-timeout:
			t.Fatalf("timeout after receiving %d/%d messages", received, numGoroutines*messagesPerGoroutine)
		}
	}

	if received != numGoroutines*messagesPerGoroutine {
		t.Errorf("expected %d messages, got %d", numGoroutines*messagesPerGoroutine, received)
	}
}

// TestSSETransport_Close tests proper cleanup when closing.
func TestSSETransport_Close(t *testing.T) {
	config := DefaultTransportConfig()
	factory := NewSSETransportFactory(config)

	transportCh := make(chan Transport, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{
			Name:  "live_session",
			Value: "test-session-close",
		})

		transport, err := factory.Upgrade(r.Context(), w, r)
		if err != nil {
			t.Errorf("upgrade failed: %v", err)
			return
		}

		transportCh <- transport
		<-r.Context().Done()
	}))
	defer server.Close()

	// Establish connection
	go func() {
		req, _ := http.NewRequest("GET", server.URL, nil)
		client := &http.Client{Timeout: 5 * time.Second}
		resp, _ := client.Do(req)
		defer resp.Body.Close()
		io.ReadAll(resp.Body)
	}()

	// Wait for transport
	var serverTransport Transport
	select {
	case serverTransport = <-transportCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for transport")
	}

	// Close the transport
	if err := serverTransport.Close(); err != nil {
		t.Errorf("close failed: %v", err)
	}

	// Verify that Send returns an error after close
	err := serverTransport.Send(Event{T: "test"})
	if err == nil {
		t.Error("expected error when sending to closed transport")
	}

	// Verify that Events channel is eventually closed
	timeout := time.After(2 * time.Second)
	eventsClosed := false

	for !eventsClosed {
		select {
		case _, ok := <-serverTransport.Events():
			if !ok {
				eventsClosed = true
			}
		case <-timeout:
			t.Fatal("events channel not closed after timeout")
		}
	}

	// Verify that calling Close again is safe (idempotent)
	if err := serverTransport.Close(); err != nil {
		t.Errorf("second close failed: %v", err)
	}
}

// TestSSETransport_Heartbeat tests keepalive heartbeat functionality.
func TestSSETransport_Heartbeat(t *testing.T) {
	// Use a short ping interval for testing
	config := DefaultTransportConfig()
	config.PingInterval = 100 * time.Millisecond

	factory := NewSSETransportFactory(config)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{
			Name:  "live_session",
			Value: "test-session-heartbeat",
		})

		transport, err := factory.Upgrade(r.Context(), w, r)
		if err != nil {
			t.Errorf("upgrade failed: %v", err)
			return
		}
		defer transport.Close()

		// Keep connection alive for a bit
		time.Sleep(500 * time.Millisecond)
	}))
	defer server.Close()

	req, _ := http.NewRequest("GET", server.URL, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// Read and look for heartbeat comments
	scanner := bufio.NewScanner(resp.Body)
	heartbeatFound := false
	lineCount := 0
	maxLines := 100 // Prevent infinite loop

	for scanner.Scan() && lineCount < maxLines {
		line := scanner.Text()
		lineCount++

		// SSE comments start with ":"
		if strings.HasPrefix(line, ": heartbeat") {
			heartbeatFound = true
			break
		}
	}

	if !heartbeatFound {
		t.Error("no heartbeat comment found in SSE stream")
	}
}

// TestSSETransport_EventIDIncrement tests that event IDs increment correctly.
func TestSSETransport_EventIDIncrement(t *testing.T) {
	config := DefaultTransportConfig()
	factory := NewSSETransportFactory(config)

	transportCh := make(chan Transport, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{
			Name:  "live_session",
			Value: "test-session-eventid",
		})

		transport, err := factory.Upgrade(r.Context(), w, r)
		if err != nil {
			t.Errorf("upgrade failed: %v", err)
			return
		}

		transportCh <- transport
		<-r.Context().Done()
	}))
	defer server.Close()

	// Collect event IDs
	eventIDsCh := make(chan string, 10)
	go func() {
		req, _ := http.NewRequest("GET", server.URL, nil)
		client := &http.Client{Timeout: 5 * time.Second}
		resp, _ := client.Do(req)
		if resp != nil {
			defer resp.Body.Close()

			scanner := bufio.NewScanner(resp.Body)
			for scanner.Scan() {
				line := scanner.Text()
				if strings.HasPrefix(line, "id: ") {
					eventID := strings.TrimPrefix(line, "id: ")
					eventIDsCh <- eventID
				}
			}
		}
	}()

	// Wait for transport
	var serverTransport Transport
	select {
	case serverTransport = <-transportCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for transport")
	}
	defer serverTransport.Close()

	// Collect first event ID (from connect event)
	var eventIDs []string
	select {
	case id := <-eventIDsCh:
		eventIDs = append(eventIDs, id)
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for first event ID")
	}

	// Send 3 more events
	for i := 0; i < 3; i++ {
		serverTransport.Send(Event{T: "test", ID: i})

		select {
		case id := <-eventIDsCh:
			eventIDs = append(eventIDs, id)
		case <-time.After(1 * time.Second):
			t.Fatalf("timeout waiting for event ID %d", i)
		}
	}

	// Verify IDs are sequential
	if len(eventIDs) != 4 {
		t.Fatalf("expected 4 event IDs, got %d", len(eventIDs))
	}

	for i := 1; i < len(eventIDs); i++ {
		prev, _ := strconv.Atoi(eventIDs[i-1])
		curr, _ := strconv.Atoi(eventIDs[i])

		if curr != prev+1 {
			t.Errorf("event IDs not sequential: %d -> %d", prev, curr)
		}
	}
}
