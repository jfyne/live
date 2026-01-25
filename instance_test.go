package live

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
)

// counterState is a test state type for counter islands.
type counterState struct {
	Count int
}

// createTestRegistry creates a fresh registry with a test counter island.
func createTestRegistry() *IslandRegistry {
	registry := NewIslandRegistry()
	_ = registry.Register("counter", func() (*Island, error) {
		island, _ := NewIsland("counter",
			WithMount(func(ctx context.Context, props Props, children string) (any, error) {
				initial := props.Int("initial")
				return &counterState{Count: initial}, nil
			}),
			WithRender(func(ctx context.Context, rc *IslandRenderContext) (io.Reader, error) {
				state := rc.State.(*counterState)
				return strings.NewReader("<div>" + string(rune('0'+state.Count)) + "</div>"), nil
			}),
			WithUnmount(func(ctx context.Context, state any) error {
				return nil
			}),
		)
		_ = island.HandleEvent("increment", func(ctx context.Context, state any, params Params) (any, error) {
			s := state.(*counterState)
			return &counterState{Count: s.Count + 1}, nil
		})
		_ = island.HandleEvent("decrement", func(ctx context.Context, state any, params Params) (any, error) {
			s := state.(*counterState)
			return &counterState{Count: s.Count - 1}, nil
		})
		_ = island.HandleEvent("set", func(ctx context.Context, state any, params Params) (any, error) {
			value := params.Int("value")
			return &counterState{Count: value}, nil
		})
		_ = island.HandleSelf("refresh", func(ctx context.Context, state any, data any) (any, error) {
			newCount := data.(int)
			return &counterState{Count: newCount}, nil
		})
		return island, nil
	})
	return registry
}

func TestNewIslandInstance(t *testing.T) {
	registry := createTestRegistry()

	t.Run("creates instance with props", func(t *testing.T) {
		instance, err := NewIslandInstanceFromRegistry("counter-1", "counter", Props{"initial": 5}, registry)
		if err != nil {
			t.Fatalf("NewIslandInstanceFromRegistry() error = %v", err)
		}

		if instance.ID != "counter-1" {
			t.Errorf("ID = %q, want %q", instance.ID, "counter-1")
		}
		if instance.Type != "counter" {
			t.Errorf("Type = %q, want %q", instance.Type, "counter")
		}
		if instance.island == nil {
			t.Error("island is nil")
		}
		if instance.Props()["initial"] != 5 {
			t.Errorf("Props[initial] = %v, want 5", instance.Props()["initial"])
		}
	})

	t.Run("creates instance with nil props", func(t *testing.T) {
		instance, err := NewIslandInstanceFromRegistry("counter-1", "counter", nil, registry)
		if err != nil {
			t.Fatalf("NewIslandInstanceFromRegistry() error = %v", err)
		}

		if instance.Props() == nil {
			t.Error("Props() is nil, should be empty map")
		}
	})

	t.Run("returns error for unknown island type", func(t *testing.T) {
		_, err := NewIslandInstanceFromRegistry("unknown-1", "unknown", Props{}, registry)
		if !errors.Is(err, ErrIslandNotFound) {
			t.Errorf("error = %v, want %v", err, ErrIslandNotFound)
		}
	})

	t.Run("creates instance with children", func(t *testing.T) {
		// Register an island that uses children
		childrenRegistry := NewIslandRegistry()
		_ = childrenRegistry.Register("container", func() (*Island, error) {
			return NewIsland("container",
				WithMount(func(ctx context.Context, props Props, children string) (any, error) {
					return children, nil
				}),
				WithRender(func(ctx context.Context, rc *IslandRenderContext) (io.Reader, error) {
					return strings.NewReader("<div>" + rc.Children + "</div>"), nil
				}),
			)
		})

		instance, err := NewIslandInstanceFromRegistry("container-1", "container", Props{}, childrenRegistry)
		if err != nil {
			t.Fatalf("NewIslandInstanceFromRegistry() error = %v", err)
		}
		instance.children = "<span>nested content</span>"

		if instance.Children() != "<span>nested content</span>" {
			t.Errorf("Children() = %q, want %q", instance.Children(), "<span>nested content</span>")
		}
	})
}

func TestNewIslandInstanceWithChildren(t *testing.T) {
	registry := createTestRegistry()

	t.Run("creates instance with children content", func(t *testing.T) {
		// First register an island that uses children
		_ = registry.Register("wrapper", func() (*Island, error) {
			return NewIsland("wrapper",
				WithMount(func(ctx context.Context, props Props, children string) (any, error) {
					return map[string]string{"children": children}, nil
				}),
			)
		})

		instance, err := NewIslandInstanceFromRegistry("wrapper-1", "wrapper", Props{}, registry)
		if err != nil {
			t.Fatalf("NewIslandInstanceFromRegistry() error = %v", err)
		}
		instance.children = "<p>Hello</p>"

		err = instance.Mount(context.Background())
		if err != nil {
			t.Fatalf("Mount() error = %v", err)
		}

		state := instance.State().(map[string]string)
		if state["children"] != "<p>Hello</p>" {
			t.Errorf("state[children] = %q, want %q", state["children"], "<p>Hello</p>")
		}
	})
}

func TestIslandInstanceMount(t *testing.T) {
	registry := createTestRegistry()

	t.Run("mount calls handler and stores state", func(t *testing.T) {
		instance, _ := NewIslandInstanceFromRegistry("counter-1", "counter", Props{"initial": 10}, registry)

		err := instance.Mount(context.Background())
		if err != nil {
			t.Fatalf("Mount() error = %v", err)
		}

		state := instance.State().(*counterState)
		if state.Count != 10 {
			t.Errorf("state.Count = %d, want 10", state.Count)
		}

		if !instance.IsMounted() {
			t.Error("IsMounted() = false, want true")
		}
	})

	t.Run("mount with default props", func(t *testing.T) {
		instance, _ := NewIslandInstanceFromRegistry("counter-1", "counter", Props{}, registry)

		err := instance.Mount(context.Background())
		if err != nil {
			t.Fatalf("Mount() error = %v", err)
		}

		state := instance.State().(*counterState)
		if state.Count != 0 {
			t.Errorf("state.Count = %d, want 0", state.Count)
		}
	})

	t.Run("double mount returns error", func(t *testing.T) {
		instance, _ := NewIslandInstanceFromRegistry("counter-1", "counter", Props{}, registry)

		err := instance.Mount(context.Background())
		if err != nil {
			t.Fatalf("first Mount() error = %v", err)
		}

		err = instance.Mount(context.Background())
		if !errors.Is(err, ErrAlreadyMounted) {
			t.Errorf("second Mount() error = %v, want %v", err, ErrAlreadyMounted)
		}
	})

	t.Run("mount propagates handler error", func(t *testing.T) {
		errorRegistry := NewIslandRegistry()
		expectedErr := errors.New("mount failed")
		_ = errorRegistry.Register("failing", func() (*Island, error) {
			return NewIsland("failing",
				WithMount(func(ctx context.Context, props Props, children string) (any, error) {
					return nil, expectedErr
				}),
			)
		})

		instance, _ := NewIslandInstanceFromRegistry("failing-1", "failing", Props{}, errorRegistry)

		err := instance.Mount(context.Background())
		if !errors.Is(err, expectedErr) {
			t.Errorf("Mount() error = %v, want %v", err, expectedErr)
		}

		if instance.IsMounted() {
			t.Error("IsMounted() = true, want false after mount error")
		}
	})
}

func TestIslandInstanceRender(t *testing.T) {
	registry := createTestRegistry()

	t.Run("render calls handler with current state", func(t *testing.T) {
		instance, _ := NewIslandInstanceFromRegistry("counter-1", "counter", Props{"initial": 5}, registry)
		_ = instance.Mount(context.Background())

		html, err := instance.Render(context.Background())
		if err != nil {
			t.Fatalf("Render() error = %v", err)
		}

		expected := "<div>5</div>"
		if string(html) != expected {
			t.Errorf("Render() = %q, want %q", html, expected)
		}
	})

	t.Run("render after state change", func(t *testing.T) {
		instance, _ := NewIslandInstanceFromRegistry("counter-1", "counter", Props{"initial": 0}, registry)
		_ = instance.Mount(context.Background())

		// Change state via event
		_ = instance.CallEvent(context.Background(), "increment", Params{})
		_ = instance.CallEvent(context.Background(), "increment", Params{})

		html, err := instance.Render(context.Background())
		if err != nil {
			t.Fatalf("Render() error = %v", err)
		}

		expected := "<div>2</div>"
		if string(html) != expected {
			t.Errorf("Render() = %q, want %q", html, expected)
		}
	})

	t.Run("render propagates handler error", func(t *testing.T) {
		errorRegistry := NewIslandRegistry()
		expectedErr := errors.New("render failed")
		_ = errorRegistry.Register("failing", func() (*Island, error) {
			return NewIsland("failing",
				WithRender(func(ctx context.Context, rc *IslandRenderContext) (io.Reader, error) {
					return nil, expectedErr
				}),
			)
		})

		instance, _ := NewIslandInstanceFromRegistry("failing-1", "failing", Props{}, errorRegistry)
		_ = instance.Mount(context.Background())

		_, err := instance.Render(context.Background())
		if !errors.Is(err, expectedErr) {
			t.Errorf("Render() error = %v, want %v", err, expectedErr)
		}
	})

	t.Run("render with nil reader returns empty", func(t *testing.T) {
		nilRegistry := NewIslandRegistry()
		_ = nilRegistry.Register("nil-render", func() (*Island, error) {
			return NewIsland("nil-render",
				WithRender(func(ctx context.Context, rc *IslandRenderContext) (io.Reader, error) {
					return nil, nil
				}),
			)
		})

		instance, _ := NewIslandInstanceFromRegistry("nil-1", "nil-render", Props{}, nilRegistry)
		_ = instance.Mount(context.Background())

		html, err := instance.Render(context.Background())
		if err != nil {
			t.Fatalf("Render() error = %v", err)
		}

		if string(html) != "" {
			t.Errorf("Render() = %q, want empty string", html)
		}
	})
}

func TestIslandInstanceCallEvent(t *testing.T) {
	registry := createTestRegistry()

	t.Run("calls handler and updates state", func(t *testing.T) {
		instance, _ := NewIslandInstanceFromRegistry("counter-1", "counter", Props{"initial": 5}, registry)
		_ = instance.Mount(context.Background())

		err := instance.CallEvent(context.Background(), "increment", Params{})
		if err != nil {
			t.Fatalf("CallEvent() error = %v", err)
		}

		state := instance.State().(*counterState)
		if state.Count != 6 {
			t.Errorf("state.Count = %d, want 6", state.Count)
		}
	})

	t.Run("calls handler with params", func(t *testing.T) {
		instance, _ := NewIslandInstanceFromRegistry("counter-1", "counter", Props{"initial": 0}, registry)
		_ = instance.Mount(context.Background())

		err := instance.CallEvent(context.Background(), "set", Params{"value": 42})
		if err != nil {
			t.Fatalf("CallEvent() error = %v", err)
		}

		state := instance.State().(*counterState)
		if state.Count != 42 {
			t.Errorf("state.Count = %d, want 42", state.Count)
		}
	})

	t.Run("returns error for unknown event", func(t *testing.T) {
		instance, _ := NewIslandInstanceFromRegistry("counter-1", "counter", Props{}, registry)
		_ = instance.Mount(context.Background())

		err := instance.CallEvent(context.Background(), "unknown", Params{})
		if !errors.Is(err, ErrNoEventHandler) {
			t.Errorf("CallEvent() error = %v, want %v", err, ErrNoEventHandler)
		}
	})

	t.Run("propagates handler error", func(t *testing.T) {
		errorRegistry := NewIslandRegistry()
		expectedErr := errors.New("event failed")
		_ = errorRegistry.Register("failing", func() (*Island, error) {
			island, _ := NewIsland("failing")
			_ = island.HandleEvent("fail", func(ctx context.Context, state any, params Params) (any, error) {
				return nil, expectedErr
			})
			return island, nil
		})

		instance, _ := NewIslandInstanceFromRegistry("failing-1", "failing", Props{}, errorRegistry)
		_ = instance.Mount(context.Background())

		err := instance.CallEvent(context.Background(), "fail", Params{})
		if !errors.Is(err, expectedErr) {
			t.Errorf("CallEvent() error = %v, want %v", err, expectedErr)
		}
	})

	t.Run("multiple events in sequence", func(t *testing.T) {
		instance, _ := NewIslandInstanceFromRegistry("counter-1", "counter", Props{"initial": 0}, registry)
		_ = instance.Mount(context.Background())

		_ = instance.CallEvent(context.Background(), "increment", Params{})
		_ = instance.CallEvent(context.Background(), "increment", Params{})
		_ = instance.CallEvent(context.Background(), "decrement", Params{})

		state := instance.State().(*counterState)
		if state.Count != 1 {
			t.Errorf("state.Count = %d, want 1", state.Count)
		}
	})
}

func TestIslandInstanceCallSelf(t *testing.T) {
	registry := createTestRegistry()

	t.Run("calls handler and updates state", func(t *testing.T) {
		instance, _ := NewIslandInstanceFromRegistry("counter-1", "counter", Props{"initial": 0}, registry)
		_ = instance.Mount(context.Background())

		err := instance.CallSelf(context.Background(), "refresh", 100)
		if err != nil {
			t.Fatalf("CallSelf() error = %v", err)
		}

		state := instance.State().(*counterState)
		if state.Count != 100 {
			t.Errorf("state.Count = %d, want 100", state.Count)
		}
	})

	t.Run("returns error for unknown event", func(t *testing.T) {
		instance, _ := NewIslandInstanceFromRegistry("counter-1", "counter", Props{}, registry)
		_ = instance.Mount(context.Background())

		err := instance.CallSelf(context.Background(), "unknown", nil)
		if !errors.Is(err, ErrNoSelfHandler) {
			t.Errorf("CallSelf() error = %v, want %v", err, ErrNoSelfHandler)
		}
	})

	t.Run("propagates handler error", func(t *testing.T) {
		errorRegistry := NewIslandRegistry()
		expectedErr := errors.New("self event failed")
		_ = errorRegistry.Register("failing", func() (*Island, error) {
			island, _ := NewIsland("failing")
			_ = island.HandleSelf("fail", func(ctx context.Context, state any, data any) (any, error) {
				return nil, expectedErr
			})
			return island, nil
		})

		instance, _ := NewIslandInstanceFromRegistry("failing-1", "failing", Props{}, errorRegistry)
		_ = instance.Mount(context.Background())

		err := instance.CallSelf(context.Background(), "fail", nil)
		if !errors.Is(err, expectedErr) {
			t.Errorf("CallSelf() error = %v, want %v", err, expectedErr)
		}
	})
}

func TestIslandInstanceUnmount(t *testing.T) {
	registry := createTestRegistry()

	t.Run("calls unmount handler", func(t *testing.T) {
		unmountCalled := false
		unmountRegistry := NewIslandRegistry()
		_ = unmountRegistry.Register("tracked", func() (*Island, error) {
			return NewIsland("tracked",
				WithUnmount(func(ctx context.Context, state any) error {
					unmountCalled = true
					return nil
				}),
			)
		})

		instance, _ := NewIslandInstanceFromRegistry("tracked-1", "tracked", Props{}, unmountRegistry)
		_ = instance.Mount(context.Background())

		err := instance.Unmount(context.Background())
		if err != nil {
			t.Fatalf("Unmount() error = %v", err)
		}

		if !unmountCalled {
			t.Error("unmount handler was not called")
		}
	})

	t.Run("clears state after unmount", func(t *testing.T) {
		instance, _ := NewIslandInstanceFromRegistry("counter-1", "counter", Props{"initial": 10}, registry)
		_ = instance.Mount(context.Background())

		err := instance.Unmount(context.Background())
		if err != nil {
			t.Fatalf("Unmount() error = %v", err)
		}

		if instance.State() != nil {
			t.Errorf("State() = %v, want nil", instance.State())
		}

		if instance.IsMounted() {
			t.Error("IsMounted() = true, want false")
		}
	})

	t.Run("unmount without mount is no-op", func(t *testing.T) {
		instance, _ := NewIslandInstanceFromRegistry("counter-1", "counter", Props{}, registry)

		err := instance.Unmount(context.Background())
		if err != nil {
			t.Errorf("Unmount() error = %v, want nil", err)
		}
	})

	t.Run("propagates unmount error", func(t *testing.T) {
		errorRegistry := NewIslandRegistry()
		expectedErr := errors.New("unmount failed")
		_ = errorRegistry.Register("failing", func() (*Island, error) {
			return NewIsland("failing",
				WithUnmount(func(ctx context.Context, state any) error {
					return expectedErr
				}),
			)
		})

		instance, _ := NewIslandInstanceFromRegistry("failing-1", "failing", Props{}, errorRegistry)
		_ = instance.Mount(context.Background())

		err := instance.Unmount(context.Background())
		if !errors.Is(err, expectedErr) {
			t.Errorf("Unmount() error = %v, want %v", err, expectedErr)
		}
	})
}

func TestIslandInstanceStateAccess(t *testing.T) {
	registry := createTestRegistry()

	t.Run("State returns current state", func(t *testing.T) {
		instance, _ := NewIslandInstanceFromRegistry("counter-1", "counter", Props{"initial": 5}, registry)
		_ = instance.Mount(context.Background())

		state := instance.State().(*counterState)
		if state.Count != 5 {
			t.Errorf("State().Count = %d, want 5", state.Count)
		}
	})

	t.Run("SetState updates state", func(t *testing.T) {
		instance, _ := NewIslandInstanceFromRegistry("counter-1", "counter", Props{"initial": 0}, registry)
		_ = instance.Mount(context.Background())

		instance.SetState(&counterState{Count: 99})

		state := instance.State().(*counterState)
		if state.Count != 99 {
			t.Errorf("State().Count = %d, want 99", state.Count)
		}
	})

	t.Run("State before mount returns nil", func(t *testing.T) {
		instance, _ := NewIslandInstanceFromRegistry("counter-1", "counter", Props{}, registry)

		if instance.State() != nil {
			t.Errorf("State() = %v, want nil", instance.State())
		}
	})
}

func TestIslandInstanceStateIsolation(t *testing.T) {
	registry := createTestRegistry()

	t.Run("instances have isolated state", func(t *testing.T) {
		instance1, _ := NewIslandInstanceFromRegistry("counter-1", "counter", Props{"initial": 0}, registry)
		instance2, _ := NewIslandInstanceFromRegistry("counter-2", "counter", Props{"initial": 100}, registry)

		_ = instance1.Mount(context.Background())
		_ = instance2.Mount(context.Background())

		// Modify instance1
		_ = instance1.CallEvent(context.Background(), "increment", Params{})
		_ = instance1.CallEvent(context.Background(), "increment", Params{})
		_ = instance1.CallEvent(context.Background(), "increment", Params{})

		// Modify instance2
		_ = instance2.CallEvent(context.Background(), "decrement", Params{})

		// Verify independent states
		state1 := instance1.State().(*counterState)
		state2 := instance2.State().(*counterState)

		if state1.Count != 3 {
			t.Errorf("instance1 state.Count = %d, want 3", state1.Count)
		}
		if state2.Count != 99 {
			t.Errorf("instance2 state.Count = %d, want 99", state2.Count)
		}
	})

	t.Run("same type different IDs", func(t *testing.T) {
		instances := make([]*IslandInstance, 5)
		for i := 0; i < 5; i++ {
			id := "counter-" + string(rune('a'+i))
			instances[i], _ = NewIslandInstanceFromRegistry(id, "counter", Props{"initial": i * 10}, registry)
			_ = instances[i].Mount(context.Background())
		}

		// Modify each instance differently
		for i, inst := range instances {
			for j := 0; j <= i; j++ {
				_ = inst.CallEvent(context.Background(), "increment", Params{})
			}
		}

		// Verify each has correct count
		expectedCounts := []int{1, 12, 23, 34, 45}
		for i, inst := range instances {
			state := inst.State().(*counterState)
			if state.Count != expectedCounts[i] {
				t.Errorf("instance[%d] state.Count = %d, want %d", i, state.Count, expectedCounts[i])
			}
		}
	})
}

func TestIslandInstanceConcurrentAccess(t *testing.T) {
	registry := createTestRegistry()

	t.Run("concurrent event calls", func(t *testing.T) {
		instance, _ := NewIslandInstanceFromRegistry("counter-1", "counter", Props{"initial": 0}, registry)
		_ = instance.Mount(context.Background())

		var wg sync.WaitGroup
		numGoroutines := 100

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = instance.CallEvent(context.Background(), "increment", Params{})
			}()
		}

		wg.Wait()

		state := instance.State().(*counterState)
		if state.Count != numGoroutines {
			t.Errorf("state.Count = %d, want %d", state.Count, numGoroutines)
		}
	})

	t.Run("concurrent state access", func(t *testing.T) {
		instance, _ := NewIslandInstanceFromRegistry("counter-1", "counter", Props{"initial": 0}, registry)
		_ = instance.Mount(context.Background())

		var wg sync.WaitGroup
		numGoroutines := 100

		// Concurrent reads
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = instance.State()
			}()
		}

		// Concurrent writes
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				instance.SetState(&counterState{Count: idx})
			}(i)
		}

		wg.Wait()
		// Test passes if no race conditions detected
	})

	t.Run("concurrent render calls", func(t *testing.T) {
		instance, _ := NewIslandInstanceFromRegistry("counter-1", "counter", Props{"initial": 5}, registry)
		_ = instance.Mount(context.Background())

		var wg sync.WaitGroup
		numGoroutines := 50

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, _ = instance.Render(context.Background())
			}()
		}

		wg.Wait()
		// Test passes if no race conditions detected
	})
}

func TestIslandInstanceLifecycle(t *testing.T) {
	lifecycleEvents := []string{}
	mu := sync.Mutex{}
	record := func(event string) {
		mu.Lock()
		defer mu.Unlock()
		lifecycleEvents = append(lifecycleEvents, event)
	}

	lifecycleRegistry := NewIslandRegistry()
	_ = lifecycleRegistry.Register("lifecycle", func() (*Island, error) {
		island, _ := NewIsland("lifecycle",
			WithMount(func(ctx context.Context, props Props, children string) (any, error) {
				record("mount")
				return "mounted", nil
			}),
			WithRender(func(ctx context.Context, rc *IslandRenderContext) (io.Reader, error) {
				record("render")
				return strings.NewReader("<div>rendered</div>"), nil
			}),
			WithUnmount(func(ctx context.Context, state any) error {
				record("unmount")
				return nil
			}),
		)
		_ = island.HandleEvent("update", func(ctx context.Context, state any, params Params) (any, error) {
			record("event:update")
			return state, nil
		})
		_ = island.HandleSelf("refresh", func(ctx context.Context, state any, data any) (any, error) {
			record("self:refresh")
			return state, nil
		})
		return island, nil
	})

	t.Run("full lifecycle", func(t *testing.T) {
		lifecycleEvents = []string{}

		instance, _ := NewIslandInstanceFromRegistry("lifecycle-1", "lifecycle", Props{}, lifecycleRegistry)

		// Mount
		_ = instance.Mount(context.Background())

		// Render
		_, _ = instance.Render(context.Background())

		// Events
		_ = instance.CallEvent(context.Background(), "update", Params{})
		_ = instance.CallSelf(context.Background(), "refresh", nil)

		// Another render
		_, _ = instance.Render(context.Background())

		// Unmount
		_ = instance.Unmount(context.Background())

		expected := []string{"mount", "render", "event:update", "self:refresh", "render", "unmount"}
		if len(lifecycleEvents) != len(expected) {
			t.Fatalf("lifecycle events count = %d, want %d", len(lifecycleEvents), len(expected))
		}
		for i, event := range expected {
			if lifecycleEvents[i] != event {
				t.Errorf("lifecycle event %d = %q, want %q", i, lifecycleEvents[i], event)
			}
		}
	})
}

func TestIslandInstanceIsland(t *testing.T) {
	registry := createTestRegistry()

	t.Run("returns underlying island", func(t *testing.T) {
		instance, _ := NewIslandInstanceFromRegistry("counter-1", "counter", Props{}, registry)

		island := instance.Island()
		if island == nil {
			t.Fatal("Island() returned nil")
		}
		if island.Name != "counter" {
			t.Errorf("Island().Name = %q, want %q", island.Name, "counter")
		}
	})
}
