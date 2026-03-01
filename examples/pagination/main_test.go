package main

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jfyne/live"
)

// paginationMockTransport is a transport implementation for pagination example
// testing. It tracks sent events and provides a channel for injecting inbound
// events.
type paginationMockTransport struct {
	events chan live.Event
	sent   []live.Event
	mu     sync.Mutex
	closed bool
}

func newPaginationMockTransport() *paginationMockTransport {
	return &paginationMockTransport{
		events: make(chan live.Event, 16),
		sent:   []live.Event{},
	}
}

func (m *paginationMockTransport) Send(e live.Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, e)
	return nil
}

func (m *paginationMockTransport) Events() <-chan live.Event {
	return m.events
}

func (m *paginationMockTransport) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.closed {
		close(m.events)
		m.closed = true
	}
	return nil
}

func (m *paginationMockTransport) GetSent() []live.Event {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]live.Event{}, m.sent...)
}

// ---------------------------------------------------------------------------
// Regression tests (must PASS with current framework before pagination exists)
// ---------------------------------------------------------------------------

// TestFramework_NewIsland_Pagination verifies that the live package NewIsland
// constructor works correctly with WithHandleParams — a regression test for the
// HandleParams API that was implemented in T8.
//
// Scenario: Framework supports HandleParams (regression)
func TestFramework_NewIsland_Pagination(t *testing.T) {
	paramsCalled := false

	island, err := live.NewIsland("pagination-reg",
		live.WithMount(func(ctx context.Context, props live.Props, children string) (any, error) {
			return map[string]any{"page": 0}, nil
		}),
		live.WithHandleParams(func(ctx context.Context, state any, params live.Params) (any, error) {
			paramsCalled = true
			return state, nil
		}),
	)
	if err != nil {
		t.Fatalf("NewIsland() with WithHandleParams error = %v", err)
	}
	if island == nil {
		t.Fatal("expected non-nil island")
	}

	handler := island.GetParamsHandler()
	if handler == nil {
		t.Fatal("GetParamsHandler() = nil, want non-nil after WithHandleParams")
	}

	_, _ = handler(context.Background(), map[string]any{"page": 0}, live.Params{})
	if !paramsCalled {
		t.Error("params handler was not callable")
	}
}

// TestFramework_RouteEventParams_Pagination verifies that an EventParams event
// routed through the engine reaches the island's params handler — a regression
// that confirms the T8 session routing for EventParams.
//
// Scenario: EventParams routing via engine (regression)
func TestFramework_RouteEventParams_Pagination(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	paramsCalled := false
	registry := live.NewIslandRegistry()
	err := registry.Register("reg-page", func() (*live.Island, error) {
		return live.NewIsland("reg-page",
			live.WithMount(func(ctx context.Context, props live.Props, children string) (any, error) {
				return map[string]any{"page": 0}, nil
			}),
			live.WithHandleParams(func(ctx context.Context, state any, params live.Params) (any, error) {
				paramsCalled = true
				return state, nil
			}),
		)
	})
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	stateStore := live.NewMemoryIslandStateStore(ctx, 1*time.Minute)
	engine := live.NewIslandEngine(ctx, registry, stateStore)
	defer engine.Close()

	transport := newPaginationMockTransport()
	session := live.NewSession(ctx, "session-reg-pagination", transport)
	engine.AddSession(session)
	time.Sleep(10 * time.Millisecond)

	_, err = engine.MountIsland("session-reg-pagination", "reg-page-1", "reg-page", live.Props{})
	if err != nil {
		t.Fatalf("MountIsland failed: %v", err)
	}

	paramsData, _ := json.Marshal(live.Params{"page": "1"})
	err = engine.RouteEvent("session-reg-pagination", live.Event{
		T:      live.EventParams,
		Island: "reg-page-1",
		Data:   paramsData,
	})
	if err != nil {
		t.Fatalf("RouteEvent(EventParams) error = %v", err)
	}

	if !paramsCalled {
		t.Error("expected params handler to be called after routing EventParams, but it was not")
	}
}

// ---------------------------------------------------------------------------
// New feature tests (RED — fail to compile until examples/pagination/main.go
// is created)
// ---------------------------------------------------------------------------

// TestPaginationIsland_NewPaginationIsland verifies that NewPaginationIsland
// constructs a valid non-nil Island without error.
//
// Scenario: NewPaginationIsland construction succeeds
func TestPaginationIsland_NewPaginationIsland(t *testing.T) {
	island, err := NewPaginationIsland()
	if err != nil {
		t.Fatalf("NewPaginationIsland() error = %v", err)
	}
	if island == nil {
		t.Fatal("expected non-nil island")
	}
}

// TestPaginationIsland_Mount verifies that mounting the pagination island with
// no props initializes PaginationState.Page to 0 and populates Items with a
// non-empty default set.
//
// Scenario: Mount handler initializes page 0 with default items
func TestPaginationIsland_Mount(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	island, err := NewPaginationIsland()
	if err != nil {
		t.Fatalf("NewPaginationIsland() failed: %v", err)
	}

	state, err := island.Mount(ctx, live.Props{}, "")
	if err != nil {
		t.Fatalf("Mount() error = %v", err)
	}

	ps, ok := state.(*PaginationState)
	if !ok {
		t.Fatalf("expected *PaginationState from Mount, got %T", state)
	}

	if ps.Page != 0 {
		t.Errorf("expected initial Page = 0, got %d", ps.Page)
	}

	if len(ps.Items) == 0 {
		t.Error("expected Items to be non-empty after mount")
	}
}

// TestPaginationIsland_HandleParams verifies that the params handler with
// page=2 updates PaginationState.Page to 2 and refreshes Items.
//
// Scenario: HandleParams with page=2 updates the current page and items
func TestPaginationIsland_HandleParams(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	island, err := NewPaginationIsland()
	if err != nil {
		t.Fatalf("NewPaginationIsland() failed: %v", err)
	}

	handler := island.GetParamsHandler()
	if handler == nil {
		t.Fatal("expected a params handler to be registered on the pagination island")
	}

	// Set up an initial state at page 0.
	initialState, err := island.Mount(ctx, live.Props{}, "")
	if err != nil {
		t.Fatalf("Mount() error = %v", err)
	}

	// Call the params handler with page=2.
	newState, err := handler(ctx, initialState, live.Params{"page": "2"})
	if err != nil {
		t.Fatalf("params handler error = %v", err)
	}

	ps, ok := newState.(*PaginationState)
	if !ok {
		t.Fatalf("expected *PaginationState from params handler, got %T", newState)
	}

	if ps.Page != 2 {
		t.Errorf("expected Page = 2 after HandleParams with page=2, got %d", ps.Page)
	}

	if len(ps.Items) == 0 {
		t.Error("expected Items to be non-empty after HandleParams with page=2")
	}
}

// TestPaginationIsland_NextPage verifies that the "next-page" event handler
// increments PaginationState.Page by 1 and calls PatchURL to update the URL.
// We verify the page increment is applied to the returned state.
//
// Scenario: "next-page" event handler calls PatchURL and increments page
func TestPaginationIsland_NextPage(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	island, err := NewPaginationIsland()
	if err != nil {
		t.Fatalf("NewPaginationIsland() failed: %v", err)
	}

	handler, err := island.GetEventHandler("next-page")
	if err != nil {
		t.Fatalf("GetEventHandler('next-page') error = %v", err)
	}

	// Start on page 1 with some items and a TotalPages > 1.
	state := &PaginationState{
		Page:       1,
		Items:      []string{"item-a", "item-b"},
		TotalPages: 5,
	}

	newState, err := handler(ctx, state, live.Params{})
	if err != nil {
		t.Fatalf("next-page handler error = %v", err)
	}

	ps, ok := newState.(*PaginationState)
	if !ok {
		t.Fatalf("expected *PaginationState from next-page handler, got %T", newState)
	}

	if ps.Page != 2 {
		t.Errorf("expected Page = 2 after next-page from page 1, got %d", ps.Page)
	}
}

// TestPaginationIsland_NextPageSendsPatchURL verifies that calling the
// "next-page" event handler via the engine causes an EventParams event to be
// sent to the transport (i.e. PatchURL was called).
//
// Scenario: "next-page" event handler calls PatchURL and increments page
func TestPaginationIsland_NextPageSendsPatchURL(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	registry := live.NewIslandRegistry()
	err := registry.Register("pagination", NewPaginationIsland)
	if err != nil {
		t.Fatalf("registry.Register failed: %v", err)
	}

	stateStore := live.NewMemoryIslandStateStore(ctx, 1*time.Minute)
	engine := live.NewIslandEngine(ctx, registry, stateStore)
	defer engine.Close()

	transport := newPaginationMockTransport()
	session := live.NewSession(ctx, "session-pagination-next", transport)
	engine.AddSession(session)
	time.Sleep(10 * time.Millisecond)

	_, err = engine.MountIsland("session-pagination-next", "pagination-1", "pagination", live.Props{})
	if err != nil {
		t.Fatalf("MountIsland failed: %v", err)
	}

	// Clear mount events so we can focus on events from "next-page".
	transport.mu.Lock()
	transport.sent = []live.Event{}
	transport.mu.Unlock()

	// Route the "next-page" event through the engine.
	err = engine.RouteEvent("session-pagination-next", live.Event{
		T:      "next-page",
		Island: "pagination-1",
		Data:   []byte(`{}`),
	})
	if err != nil {
		t.Fatalf("RouteEvent('next-page') failed: %v", err)
	}

	// Verify that an EventParams event was sent (PatchURL was called).
	sent := transport.GetSent()
	hasParams := false
	for _, e := range sent {
		if e.T == live.EventParams {
			hasParams = true
			break
		}
	}
	if !hasParams {
		t.Error("expected an EventParams event after next-page (PatchURL should be called), but none was sent")
	}
}

// TestPaginationIsland_PrevPage verifies that the "prev-page" event handler
// decrements PaginationState.Page by 1.
//
// Scenario: "prev-page" event handler calls PatchURL and decrements page
func TestPaginationIsland_PrevPage(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	island, err := NewPaginationIsland()
	if err != nil {
		t.Fatalf("NewPaginationIsland() failed: %v", err)
	}

	handler, err := island.GetEventHandler("prev-page")
	if err != nil {
		t.Fatalf("GetEventHandler('prev-page') error = %v", err)
	}

	// Start on page 3.
	state := &PaginationState{
		Page:       3,
		Items:      []string{"item-a", "item-b"},
		TotalPages: 5,
	}

	newState, err := handler(ctx, state, live.Params{})
	if err != nil {
		t.Fatalf("prev-page handler error = %v", err)
	}

	ps, ok := newState.(*PaginationState)
	if !ok {
		t.Fatalf("expected *PaginationState from prev-page handler, got %T", newState)
	}

	if ps.Page != 2 {
		t.Errorf("expected Page = 2 after prev-page from page 3, got %d", ps.Page)
	}
}

// TestPaginationIsland_PrevPageMinZero verifies that "prev-page" does not
// decrement below page 0 (minimum page constraint).
//
// Scenario: "prev-page" event handler calls PatchURL and decrements page (min 0)
func TestPaginationIsland_PrevPageMinZero(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	island, err := NewPaginationIsland()
	if err != nil {
		t.Fatalf("NewPaginationIsland() failed: %v", err)
	}

	handler, err := island.GetEventHandler("prev-page")
	if err != nil {
		t.Fatalf("GetEventHandler('prev-page') error = %v", err)
	}

	// Start on page 0 (already at minimum).
	state := &PaginationState{
		Page:       0,
		Items:      []string{"item-a"},
		TotalPages: 3,
	}

	newState, err := handler(ctx, state, live.Params{})
	if err != nil {
		t.Fatalf("prev-page handler error = %v", err)
	}

	ps, ok := newState.(*PaginationState)
	if !ok {
		t.Fatalf("expected *PaginationState from prev-page handler, got %T", newState)
	}

	if ps.Page != 0 {
		t.Errorf("expected Page = 0 (min) after prev-page from page 0, got %d", ps.Page)
	}
}

// TestPaginationIsland_PrevPageSendsPatchURL verifies that calling the
// "prev-page" event handler via the engine causes an EventParams event to be
// sent to the transport (i.e. PatchURL was called).
//
// Scenario: "prev-page" event handler calls PatchURL and decrements page
func TestPaginationIsland_PrevPageSendsPatchURL(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	registry := live.NewIslandRegistry()
	err := registry.Register("pagination", NewPaginationIsland)
	if err != nil {
		t.Fatalf("registry.Register failed: %v", err)
	}

	stateStore := live.NewMemoryIslandStateStore(ctx, 1*time.Minute)
	engine := live.NewIslandEngine(ctx, registry, stateStore)
	defer engine.Close()

	transport := newPaginationMockTransport()
	session := live.NewSession(ctx, "session-pagination-prev", transport)
	engine.AddSession(session)
	time.Sleep(10 * time.Millisecond)

	instance, err := engine.MountIsland("session-pagination-prev", "pagination-1", "pagination", live.Props{})
	if err != nil {
		t.Fatalf("MountIsland failed: %v", err)
	}

	// Advance to page 2 by manipulating state so prev-page has room to move.
	_ = instance
	// We route a next-page first to advance to page 1 (assuming mount starts at 0).
	_ = engine.RouteEvent("session-pagination-prev", live.Event{
		T:      "next-page",
		Island: "pagination-1",
		Data:   []byte(`{}`),
	})

	// Clear events so we can focus on events from "prev-page".
	transport.mu.Lock()
	transport.sent = []live.Event{}
	transport.mu.Unlock()

	// Route the "prev-page" event through the engine.
	err = engine.RouteEvent("session-pagination-prev", live.Event{
		T:      "prev-page",
		Island: "pagination-1",
		Data:   []byte(`{}`),
	})
	if err != nil {
		t.Fatalf("RouteEvent('prev-page') failed: %v", err)
	}

	// Verify that an EventParams event was sent (PatchURL was called).
	sent := transport.GetSent()
	hasParams := false
	for _, e := range sent {
		if e.T == live.EventParams {
			hasParams = true
			break
		}
	}
	if !hasParams {
		t.Error("expected an EventParams event after prev-page (PatchURL should be called), but none was sent")
	}
}

// TestPaginationIsland_MountAndRender verifies the full mount-and-render path
// through the engine: registering the island, mounting it, and confirming that
// at least one patch event containing valid HTML is sent to the transport.
//
// Scenario: Mount and render via engine produces valid HTML
func TestPaginationIsland_MountAndRender(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	registry := live.NewIslandRegistry()
	err := registry.Register("pagination", NewPaginationIsland)
	if err != nil {
		t.Fatalf("registry.Register failed: %v", err)
	}

	stateStore := live.NewMemoryIslandStateStore(ctx, 1*time.Minute)
	engine := live.NewIslandEngine(ctx, registry, stateStore)
	defer engine.Close()

	transport := newPaginationMockTransport()
	session := live.NewSession(ctx, "session-pagination-render", transport)
	engine.AddSession(session)
	time.Sleep(10 * time.Millisecond)

	instance, err := engine.MountIsland("session-pagination-render", "pagination-1", "pagination", live.Props{})
	if err != nil {
		t.Fatalf("MountIsland failed: %v", err)
	}

	// Verify the mounted state has page initialized to 0.
	ps, ok := instance.State().(*PaginationState)
	if !ok {
		t.Fatalf("expected *PaginationState, got %T", instance.State())
	}
	if ps.Page != 0 {
		t.Errorf("expected initial Page = 0 after mount, got %d", ps.Page)
	}

	// Verify at least one patch event was sent to the transport after mount.
	sent := transport.GetSent()
	if len(sent) == 0 {
		t.Fatal("expected at least one event sent to transport after mount")
	}

	// Find a patch event and confirm its data contains HTML.
	var patchEvent *live.Event
	for i := range sent {
		if sent[i].T == live.EventPatch {
			patchEvent = &sent[i]
			break
		}
	}
	if patchEvent == nil {
		t.Fatal("expected at least one EventPatch event to be sent after mount")
	}

	// The patch data should be non-empty HTML.
	html := strings.TrimSpace(string(patchEvent.Data))
	if html == "" {
		t.Error("expected non-empty HTML in patch event data after mount")
	}
}
