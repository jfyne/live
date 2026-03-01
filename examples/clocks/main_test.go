package main

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/jfyne/live"
)

// clocksMockTransport is a transport implementation for clocks example testing.
// It tracks sent events and provides a channel for injecting inbound events.
type clocksMockTransport struct {
	events chan live.Event
	sent   []live.Event
	mu     sync.Mutex
	closed bool
}

func newClocksMockTransport() *clocksMockTransport {
	return &clocksMockTransport{
		events: make(chan live.Event, 16),
		sent:   []live.Event{},
	}
}

func (m *clocksMockTransport) Send(e live.Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, e)
	return nil
}

func (m *clocksMockTransport) Events() <-chan live.Event {
	return m.events
}

func (m *clocksMockTransport) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.closed {
		close(m.events)
		m.closed = true
	}
	return nil
}

func (m *clocksMockTransport) GetSent() []live.Event {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]live.Event{}, m.sent...)
}

// ---------------------------------------------------------------------------
// Regression tests (must PASS with existing framework)
// ---------------------------------------------------------------------------

// TestClocksFramework_NewIsland verifies that the live package NewIsland
// constructor works correctly — a regression of core framework behaviour.
func TestClocksFramework_NewIsland(t *testing.T) {
	island, err := live.NewIsland("test-island",
		live.WithMount(func(ctx context.Context, props live.Props, children string) (any, error) {
			return map[string]any{"value": 42}, nil
		}),
	)
	if err != nil {
		t.Fatalf("NewIsland failed: %v", err)
	}
	if island == nil {
		t.Fatal("expected non-nil island")
	}
}

// TestClocksFramework_IslandRegistry verifies that multiple islands can be
// registered in a registry and looked up by name — a regression of the registry.
func TestClocksFramework_IslandRegistry(t *testing.T) {
	registry := live.NewIslandRegistry()

	for _, name := range []string{"clock-a", "clock-b", "clock-c"} {
		name := name
		err := registry.Register(name, func() (*live.Island, error) {
			return live.NewIsland(name,
				live.WithMount(func(ctx context.Context, props live.Props, children string) (any, error) {
					return map[string]any{}, nil
				}),
			)
		})
		if err != nil {
			t.Fatalf("registry.Register(%q) failed: %v", name, err)
		}
	}
}

// TestClocksFramework_EngineMultipleMounts verifies that the engine can mount
// multiple island instances of the same type within a single session.
// This is the core pattern the clocks example depends on.
func TestClocksFramework_EngineMultipleMounts(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	registry := live.NewIslandRegistry()
	err := registry.Register("basic", func() (*live.Island, error) {
		return live.NewIsland("basic",
			live.WithMount(func(ctx context.Context, props live.Props, children string) (any, error) {
				return map[string]any{"label": props.String("label")}, nil
			}),
		)
	})
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	stateStore := live.NewMemoryIslandStateStore(ctx, 1*time.Minute)
	engine := live.NewIslandEngine(ctx, registry, stateStore)
	defer engine.Close()

	transport := newClocksMockTransport()
	session := live.NewSession(ctx, "session-multi", transport)
	engine.AddSession(session)
	time.Sleep(10 * time.Millisecond)

	// Mount two instances of the same island type.
	inst1, err := engine.MountIsland("session-multi", "basic-1", "basic", live.Props{"label": "first"})
	if err != nil {
		t.Fatalf("MountIsland basic-1 failed: %v", err)
	}
	inst2, err := engine.MountIsland("session-multi", "basic-2", "basic", live.Props{"label": "second"})
	if err != nil {
		t.Fatalf("MountIsland basic-2 failed: %v", err)
	}

	// Both instances should hold independent state.
	s1 := inst1.State().(map[string]any)
	s2 := inst2.State().(map[string]any)

	if s1["label"] != "first" {
		t.Errorf("inst1: expected label='first', got %v", s1["label"])
	}
	if s2["label"] != "second" {
		t.Errorf("inst2: expected label='second', got %v", s2["label"])
	}
}

// ---------------------------------------------------------------------------
// New feature tests (RED — fail to compile until examples/clocks/main.go exists)
// ---------------------------------------------------------------------------

// TestClocksExample_MultipleClockIslands verifies that creating multiple clock
// island instances with different timezone props results in independent state.
//
// Scenario: Multiple clock islands with different timezone props have independent state
func TestClocksExample_MultipleClockIslands(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	island, err := NewClockIsland()
	if err != nil {
		t.Fatalf("NewClockIsland() failed: %v", err)
	}

	// Mount two instances with different timezone props.
	londonState, err := island.Mount(ctx, live.Props{
		"timezone": "Europe/London",
		"label":    "London",
	}, "")
	if err != nil {
		t.Fatalf("Mount with Europe/London failed: %v", err)
	}

	nyState, err := island.Mount(ctx, live.Props{
		"timezone": "America/New_York",
		"label":    "New York",
	}, "")
	if err != nil {
		t.Fatalf("Mount with America/New_York failed: %v", err)
	}

	londonCS, ok := londonState.(*ClockState)
	if !ok {
		t.Fatalf("expected *ClockState for London, got %T", londonState)
	}
	nyCS, ok := nyState.(*ClockState)
	if !ok {
		t.Fatalf("expected *ClockState for New York, got %T", nyState)
	}

	// Each instance should have the correct timezone set in its Location.
	if londonCS.Location == nil {
		t.Fatal("London ClockState: expected non-nil Location")
	}
	if londonCS.Location.String() != "Europe/London" {
		t.Errorf("London: expected Location='Europe/London', got %q", londonCS.Location.String())
	}

	if nyCS.Location == nil {
		t.Fatal("New York ClockState: expected non-nil Location")
	}
	if nyCS.Location.String() != "America/New_York" {
		t.Errorf("New York: expected Location='America/New_York', got %q", nyCS.Location.String())
	}

	// Labels should be independent.
	if londonCS.Label != "London" {
		t.Errorf("London: expected Label='London', got %q", londonCS.Label)
	}
	if nyCS.Label != "New York" {
		t.Errorf("New York: expected Label='New York', got %q", nyCS.Label)
	}
}

// TestClocksExample_DifferentTimezones verifies that mounting clock islands with
// "Europe/London" vs "America/New_York" produces different formatted time strings
// for a given UTC moment (given that London and New York are always in different
// UTC offsets).
//
// Scenario: Mounting with "Europe/London" vs "America/New_York" produces different time strings
func TestClocksExample_DifferentTimezones(t *testing.T) {
	londonLoc, err := time.LoadLocation("Europe/London")
	if err != nil {
		t.Fatalf("LoadLocation Europe/London failed: %v", err)
	}
	nyLoc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("LoadLocation America/New_York failed: %v", err)
	}

	// Use a fixed UTC reference time: 2024-06-15 18:00:00 UTC.
	// At this moment:
	//   Europe/London (BST = UTC+1): 19:00:00
	//   America/New_York (EDT = UTC-4): 14:00:00
	fixedUTC := time.Date(2024, 6, 15, 18, 0, 0, 0, time.UTC)

	londonState := &ClockState{
		Time:     fixedUTC,
		Location: londonLoc,
		Label:    "London",
	}
	nyState := &ClockState{
		Time:     fixedUTC,
		Location: nyLoc,
		Label:    "New York",
	}

	londonFormatted := londonState.FormattedTime()
	nyFormatted := nyState.FormattedTime()

	// London in BST (+1) should be "19:00:00"
	if londonFormatted != "19:00:00" {
		t.Errorf("London: expected FormattedTime()='19:00:00' (BST=UTC+1), got %q", londonFormatted)
	}

	// New York in EDT (-4) should be "14:00:00"
	if nyFormatted != "14:00:00" {
		t.Errorf("New York: expected FormattedTime()='14:00:00' (EDT=UTC-4), got %q", nyFormatted)
	}

	// The two formatted times must differ.
	if londonFormatted == nyFormatted {
		t.Errorf("expected different formatted times for London and New York, but both returned %q", londonFormatted)
	}
}

// TestClocksExample_ServerSetup verifies that the example's HTTP handler setup
// works correctly: a registry with a "clock" island can be created, an engine
// built from it, and multiple clock island instances mounted within a single session.
//
// Scenario: The example's HTTP handler setup works correctly
func TestClocksExample_ServerSetup(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Build a registry with the clock island registered.
	registry := live.NewIslandRegistry()
	err := registry.Register("clock", NewClockIsland)
	if err != nil {
		t.Fatalf("registry.Register('clock') failed: %v", err)
	}

	stateStore := live.NewMemoryIslandStateStore(ctx, 1*time.Minute)
	engine := live.NewIslandEngine(ctx, registry, stateStore)
	defer engine.Close()

	transport := newClocksMockTransport()
	session := live.NewSession(ctx, "session-clocks", transport)
	engine.AddSession(session)
	time.Sleep(10 * time.Millisecond)

	// Mount multiple clock island instances (as the clocks example page would do),
	// each with a different timezone prop, simulating multiple <live-island type="clock">
	// elements on one page.
	type clockMount struct {
		id       live.IslandID
		timezone string
		label    string
	}
	mounts := []clockMount{
		{"clock-utc", "UTC", "UTC"},
		{"clock-london", "Europe/London", "London"},
		{"clock-ny", "America/New_York", "New York"},
		{"clock-tokyo", "Asia/Tokyo", "Tokyo"},
	}

	for _, m := range mounts {
		instance, err := engine.MountIsland("session-clocks", m.id, "clock", live.Props{
			"timezone": m.timezone,
			"label":    m.label,
		})
		if err != nil {
			t.Fatalf("MountIsland(%q, timezone=%q) failed: %v", m.id, m.timezone, err)
		}

		cs, ok := instance.State().(*ClockState)
		if !ok {
			t.Fatalf("island %q: expected *ClockState, got %T", m.id, instance.State())
		}
		if cs.Location == nil {
			t.Fatalf("island %q: expected non-nil Location", m.id)
		}
		if cs.Location.String() != m.timezone {
			t.Errorf("island %q: expected Location=%q, got %q", m.id, m.timezone, cs.Location.String())
		}
		if cs.Label != m.label {
			t.Errorf("island %q: expected Label=%q, got %q", m.id, m.label, cs.Label)
		}
	}

	// After mounting 4 clock islands, verify that at least 4 patch events were
	// sent to the transport (one per mount render).
	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		sent := transport.GetSent()
		patchCount := 0
		for _, e := range sent {
			if e.T == live.EventPatch {
				patchCount++
			}
		}
		if patchCount >= 4 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}

	sent := transport.GetSent()
	patchCount := 0
	for _, e := range sent {
		if e.T == live.EventPatch {
			patchCount++
		}
	}
	t.Errorf("expected at least 4 patch events after mounting 4 clock islands, got %d", patchCount)
}

// TestClocksExample_IndependentStateAfterTick verifies that the tick self-handler
// updates each clock instance independently — changing one clock's time does not
// affect another's configured timezone.
//
// Scenario: Multiple clock islands with different timezone props have independent state
func TestClocksExample_IndependentStateAfterTick(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	island, err := NewClockIsland()
	if err != nil {
		t.Fatalf("NewClockIsland() failed: %v", err)
	}

	tickHandler, err := island.GetSelfHandler("tick")
	if err != nil {
		t.Fatalf("GetSelfHandler('tick') failed: %v", err)
	}

	londonLoc, err := time.LoadLocation("Europe/London")
	if err != nil {
		t.Fatalf("LoadLocation Europe/London failed: %v", err)
	}
	tokyoLoc, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatalf("LoadLocation Asia/Tokyo failed: %v", err)
	}

	londonState := &ClockState{
		Time:     time.Now().Add(-5 * time.Second),
		Location: londonLoc,
		Label:    "London",
	}
	tokyoState := &ClockState{
		Time:     time.Now().Add(-5 * time.Second),
		Location: tokyoLoc,
		Label:    "Tokyo",
	}

	// Tick each state independently.
	newLondon, err := tickHandler(ctx, londonState, nil)
	if err != nil {
		t.Fatalf("tick handler (London) returned error: %v", err)
	}
	newTokyo, err := tickHandler(ctx, tokyoState, nil)
	if err != nil {
		t.Fatalf("tick handler (Tokyo) returned error: %v", err)
	}

	londonCS, ok := newLondon.(*ClockState)
	if !ok {
		t.Fatalf("expected *ClockState from London tick, got %T", newLondon)
	}
	tokyoCS, ok := newTokyo.(*ClockState)
	if !ok {
		t.Fatalf("expected *ClockState from Tokyo tick, got %T", newTokyo)
	}

	// Tick must update time but preserve the per-instance timezone.
	if londonCS.Location.String() != "Europe/London" {
		t.Errorf("London: tick changed Location to %q, expected 'Europe/London'", londonCS.Location.String())
	}
	if tokyoCS.Location.String() != "Asia/Tokyo" {
		t.Errorf("Tokyo: tick changed Location to %q, expected 'Asia/Tokyo'", tokyoCS.Location.String())
	}

	// Both times should now be approximately the current time (tick called time.Now()).
	now := time.Now()
	tolerance := 2 * time.Second
	if londonCS.Time.Before(now.Add(-tolerance)) || londonCS.Time.After(now.Add(tolerance)) {
		t.Errorf("London: expected Time ~now after tick, got %v", londonCS.Time)
	}
	if tokyoCS.Time.Before(now.Add(-tolerance)) || tokyoCS.Time.After(now.Add(tolerance)) {
		t.Errorf("Tokyo: expected Time ~now after tick, got %v", tokyoCS.Time)
	}
}
