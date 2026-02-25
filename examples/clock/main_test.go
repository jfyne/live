package main

import (
	"context"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/jfyne/live"
)

// mockTransport is a transport implementation for clock example testing.
// It tracks sent events and provides a channel for injecting inbound events.
type mockTransport struct {
	events chan live.Event
	sent   []live.Event
	mu     sync.Mutex
	closed bool
}

func newMockTransport() *mockTransport {
	return &mockTransport{
		events: make(chan live.Event, 16),
		sent:   []live.Event{},
	}
}

func (m *mockTransport) Send(e live.Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, e)
	return nil
}

func (m *mockTransport) Events() <-chan live.Event {
	return m.events
}

func (m *mockTransport) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.closed {
		close(m.events)
		m.closed = true
	}
	return nil
}

func (m *mockTransport) GetSent() []live.Event {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]live.Event{}, m.sent...)
}

// ---------------------------------------------------------------------------
// Regression tests (must PASS with existing framework before clock is created)
// ---------------------------------------------------------------------------

// TestFramework_NewIsland verifies that the live package NewIsland constructor
// works correctly — a basic regression of core framework behaviour.
func TestFramework_NewIsland(t *testing.T) {
	island, err := live.NewIsland("test-island",
		live.WithMount(func(ctx context.Context, props live.Props, children string) (any, error) {
			return map[string]any{"value": 42}, nil
		}),
		live.WithRender(func(ctx context.Context, rc *live.IslandRenderContext) (io.Reader, error) {
			return nil, nil
		}),
	)
	if err != nil {
		t.Fatalf("NewIsland failed: %v", err)
	}
	if island == nil {
		t.Fatal("expected non-nil island")
	}
}

// TestFramework_IslandRegistry verifies that registry registration and lookup
// work correctly — a basic regression of the island registry.
func TestFramework_IslandRegistry(t *testing.T) {
	registry := live.NewIslandRegistry()

	err := registry.Register("my-island", func() (*live.Island, error) {
		return live.NewIsland("my-island",
			live.WithMount(func(ctx context.Context, props live.Props, children string) (any, error) {
				return map[string]any{}, nil
			}),
		)
	})
	if err != nil {
		t.Fatalf("registry.Register failed: %v", err)
	}
}

// TestFramework_MountIsland verifies that the engine mounts an island, calls the
// mount handler, and returns a valid instance — regression for the engine mount path.
func TestFramework_MountIsland(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	registry := live.NewIslandRegistry()
	err := registry.Register("basic", func() (*live.Island, error) {
		return live.NewIsland("basic",
			live.WithMount(func(ctx context.Context, props live.Props, children string) (any, error) {
				return map[string]any{"ok": true}, nil
			}),
		)
	})
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	stateStore := live.NewMemoryIslandStateStore(ctx, 1*time.Minute)
	engine := live.NewIslandEngine(ctx, registry, stateStore)
	defer engine.Close()

	transport := newMockTransport()
	session := live.NewSession(ctx, "session-reg", transport)
	engine.AddSession(session)
	time.Sleep(10 * time.Millisecond)

	instance, err := engine.MountIsland("session-reg", "basic-1", "basic", live.Props{})
	if err != nil {
		t.Fatalf("MountIsland failed: %v", err)
	}
	state := instance.State().(map[string]any)
	if state["ok"] != true {
		t.Errorf("expected state ok=true, got %v", state["ok"])
	}
}

// TestFramework_WithEventDelay verifies that WithEventDelay is accepted by the
// island constructor without error — regression for the event delay config.
func TestFramework_WithEventDelay(t *testing.T) {
	_, err := live.NewIsland("delayed",
		live.WithMount(func(ctx context.Context, props live.Props, children string) (any, error) {
			return nil, nil
		}),
		live.WithEventDelay("tick", 50*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("NewIsland with WithEventDelay failed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// New feature tests (RED — fail to compile until clock/main.go is created)
// ---------------------------------------------------------------------------

// TestClockIsland_MountWithTimezone verifies that mounting NewClockIsland with
// timezone "America/New_York" sets the state Location and Label correctly.
//
// Scenario: Multiple clocks display different timezones
func TestClockIsland_MountWithTimezone(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	island, err := NewClockIsland()
	if err != nil {
		t.Fatalf("NewClockIsland failed: %v", err)
	}

	// Call the mount handler directly with timezone props
	state, err := island.Mount(ctx, live.Props{
		"timezone": "America/New_York",
		"label":    "NYC",
	}, "")
	if err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	clockState, ok := state.(*ClockState)
	if !ok {
		t.Fatalf("expected *ClockState, got %T", state)
	}

	if clockState.Location == nil {
		t.Fatal("expected non-nil Location")
	}
	if clockState.Location.String() != "America/New_York" {
		t.Errorf("expected Location 'America/New_York', got %q", clockState.Location.String())
	}
	if clockState.Label != "NYC" {
		t.Errorf("expected Label 'NYC', got %q", clockState.Label)
	}
}

// TestClockIsland_MountUTC verifies that mounting with timezone "UTC" sets
// the state Location to UTC.
//
// Scenario: Multiple clocks display different timezones
func TestClockIsland_MountUTC(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	island, err := NewClockIsland()
	if err != nil {
		t.Fatalf("NewClockIsland failed: %v", err)
	}

	state, err := island.Mount(ctx, live.Props{
		"timezone": "UTC",
		"label":    "UTC",
	}, "")
	if err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	clockState, ok := state.(*ClockState)
	if !ok {
		t.Fatalf("expected *ClockState, got %T", state)
	}

	if clockState.Location == nil {
		t.Fatal("expected non-nil Location")
	}
	if clockState.Location.String() != "UTC" {
		t.Errorf("expected Location 'UTC', got %q", clockState.Location.String())
	}
	if clockState.Label != "UTC" {
		t.Errorf("expected Label 'UTC', got %q", clockState.Label)
	}
}

// TestClockIsland_SendSelfOnMount verifies that mounting a clock island via the
// engine results in patch events being sent to the transport. The clock's mount
// handler calls SendSelf("tick", nil), and the engine drains the self-event
// queue after mount completes, triggering the tick handler which renders a patch.
//
// Scenario: Clock updates every second
func TestClockIsland_SendSelfOnMount(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	registry := live.NewIslandRegistry()
	err := registry.Register("clock", NewClockIsland)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	stateStore := live.NewMemoryIslandStateStore(ctx, 1*time.Minute)
	engine := live.NewIslandEngine(ctx, registry, stateStore)
	defer engine.Close()

	transport := newMockTransport()
	session := live.NewSession(ctx, "session-clock", transport)
	engine.AddSession(session)
	time.Sleep(10 * time.Millisecond)

	_, err = engine.MountIsland("session-clock", "clock-utc", "clock", live.Props{
		"timezone": "UTC",
		"label":    "UTC",
	})
	if err != nil {
		t.Fatalf("MountIsland failed: %v", err)
	}

	// Wait for the self-event (tick) to be dispatched and for at least one patch
	// event to arrive from the tick handler.
	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		sent := transport.GetSent()
		patchCount := 0
		for _, e := range sent {
			if e.T == live.EventPatch {
				patchCount++
			}
		}
		// We expect at least 2 patches: one for mount, one for tick self-event
		if patchCount >= 2 {
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
	t.Errorf("expected at least 2 patch events (mount + tick self-event), got %d total events (%d patches)", len(sent), patchCount)
}

// TestClockIsland_TickUpdatesTime verifies that the "tick" self handler
// updates state.Time to approximately the current time.
//
// Scenario: Clock updates every second
func TestClockIsland_TickUpdatesTime(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	island, err := NewClockIsland()
	if err != nil {
		t.Fatalf("NewClockIsland failed: %v", err)
	}

	// Get the "tick" self handler
	tickHandler, err := island.GetSelfHandler("tick")
	if err != nil {
		t.Fatalf("GetSelfHandler('tick') failed: %v", err)
	}

	// Set up a ClockState with a past time
	pastTime := time.Now().Add(-10 * time.Second)
	loc := time.UTC
	initialState := &ClockState{
		Time:     pastTime,
		Location: loc,
		Label:    "UTC",
	}

	before := time.Now()
	newState, err := tickHandler(ctx, initialState, nil)
	after := time.Now()

	if err != nil {
		t.Fatalf("tick handler returned error: %v", err)
	}

	clockState, ok := newState.(*ClockState)
	if !ok {
		t.Fatalf("expected *ClockState from tick handler, got %T", newState)
	}

	// The updated time should be approximately now (between before and after)
	if clockState.Time.Before(before) || clockState.Time.After(after) {
		t.Errorf("expected Time to be approximately now (between %v and %v), got %v",
			before, after, clockState.Time)
	}
}

// TestClockIsland_FormattedTimeUsesLocation verifies that FormattedTime returns
// the time formatted in the island's configured timezone.
//
// Scenario: Multiple clocks display different timezones
func TestClockIsland_FormattedTimeUsesLocation(t *testing.T) {
	tokyoLoc, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatalf("LoadLocation Asia/Tokyo failed: %v", err)
	}

	// Create a known UTC time: 2024-01-15 12:00:00 UTC
	// In JST (UTC+9) this should be 21:00:00
	knownUTC := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)

	state := &ClockState{
		Time:     knownUTC,
		Location: tokyoLoc,
		Label:    "Tokyo",
	}

	formatted := state.FormattedTime()

	// JST is UTC+9, so 12:00:00 UTC = 21:00:00 JST
	expected := "21:00:00"
	if formatted != expected {
		t.Errorf("expected FormattedTime() == %q (JST), got %q", expected, formatted)
	}
}

// TestClockIsland_StopsOnDisconnect verifies that after unmounting the clock island,
// no further patch events are sent (the tick timer stops).
//
// Scenario: Clock stops on disconnect
func TestClockIsland_StopsOnDisconnect(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	registry := live.NewIslandRegistry()
	err := registry.Register("clock", NewClockIsland)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	stateStore := live.NewMemoryIslandStateStore(ctx, 1*time.Minute)
	engine := live.NewIslandEngine(ctx, registry, stateStore)
	defer engine.Close()

	transport := newMockTransport()
	session := live.NewSession(ctx, "session-stop", transport)
	engine.AddSession(session)
	time.Sleep(10 * time.Millisecond)

	_, err = engine.MountIsland("session-stop", "clock-utc", "clock", live.Props{
		"timezone": "UTC",
		"label":    "UTC",
	})
	if err != nil {
		t.Fatalf("MountIsland failed: %v", err)
	}

	// Wait for at least one tick to fire (using a short event delay in the test island)
	time.Sleep(150 * time.Millisecond)

	// Unmount the island to cancel timers
	err = engine.UnmountIsland("session-stop", "clock-utc")
	if err != nil {
		t.Fatalf("UnmountIsland failed: %v", err)
	}

	// Record how many events were sent before and after unmount
	sentBeforeUnmount := len(transport.GetSent())

	// Wait a bit to see if any further events fire after unmount
	time.Sleep(150 * time.Millisecond)

	sentAfterWait := len(transport.GetSent())

	if sentAfterWait > sentBeforeUnmount {
		t.Errorf("expected no further events after unmount (clock timer should be cancelled), got %d additional events",
			sentAfterWait-sentBeforeUnmount)
	}
}
