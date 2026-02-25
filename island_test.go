package live

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestPropsString(t *testing.T) {
	tests := []struct {
		name     string
		props    Props
		key      string
		expected string
	}{
		{
			name:     "existing string key",
			props:    Props{"name": "test"},
			key:      "name",
			expected: "test",
		},
		{
			name:     "missing key",
			props:    Props{"name": "test"},
			key:      "missing",
			expected: "",
		},
		{
			name:     "non-string value",
			props:    Props{"count": 42},
			key:      "count",
			expected: "",
		},
		{
			name:     "empty string value",
			props:    Props{"empty": ""},
			key:      "empty",
			expected: "",
		},
		{
			name:     "nil props",
			props:    nil,
			key:      "any",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.props.String(tt.key)
			if result != tt.expected {
				t.Errorf("Props.String(%q) = %q, want %q", tt.key, result, tt.expected)
			}
		})
	}
}

func TestPropsInt(t *testing.T) {
	tests := []struct {
		name     string
		props    Props
		key      string
		expected int
	}{
		{
			name:     "int value",
			props:    Props{"count": 42},
			key:      "count",
			expected: 42,
		},
		{
			name:     "string int value",
			props:    Props{"count": "42"},
			key:      "count",
			expected: 42,
		},
		{
			name:     "float64 value",
			props:    Props{"count": 42.0},
			key:      "count",
			expected: 42,
		},
		{
			name:     "float32 value",
			props:    Props{"count": float32(42.0)},
			key:      "count",
			expected: 42,
		},
		{
			name:     "missing key",
			props:    Props{"count": 42},
			key:      "missing",
			expected: 0,
		},
		{
			name:     "invalid string",
			props:    Props{"count": "invalid"},
			key:      "count",
			expected: 0,
		},
		{
			name:     "nil props",
			props:    nil,
			key:      "any",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.props.Int(tt.key)
			if result != tt.expected {
				t.Errorf("Props.Int(%q) = %d, want %d", tt.key, result, tt.expected)
			}
		})
	}
}

func TestPropsBool(t *testing.T) {
	tests := []struct {
		name     string
		props    Props
		key      string
		expected bool
	}{
		{
			name:     "bool true",
			props:    Props{"enabled": true},
			key:      "enabled",
			expected: true,
		},
		{
			name:     "bool false",
			props:    Props{"enabled": false},
			key:      "enabled",
			expected: false,
		},
		{
			name:     "string true",
			props:    Props{"enabled": "true"},
			key:      "enabled",
			expected: true,
		},
		{
			name:     "string false",
			props:    Props{"enabled": "false"},
			key:      "enabled",
			expected: false,
		},
		{
			name:     "missing key",
			props:    Props{"enabled": true},
			key:      "missing",
			expected: false,
		},
		{
			name:     "invalid type",
			props:    Props{"enabled": 1},
			key:      "enabled",
			expected: false,
		},
		{
			name:     "nil props",
			props:    nil,
			key:      "any",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.props.Bool(tt.key)
			if result != tt.expected {
				t.Errorf("Props.Bool(%q) = %v, want %v", tt.key, result, tt.expected)
			}
		})
	}
}

func TestPropsFloat32(t *testing.T) {
	tests := []struct {
		name     string
		props    Props
		key      string
		expected float32
	}{
		{
			name:     "float32 value",
			props:    Props{"value": float32(3.14)},
			key:      "value",
			expected: 3.14,
		},
		{
			name:     "float64 value",
			props:    Props{"value": 3.14},
			key:      "value",
			expected: 3.14,
		},
		{
			name:     "string float value",
			props:    Props{"value": "3.14"},
			key:      "value",
			expected: 3.14,
		},
		{
			name:     "missing key",
			props:    Props{"value": float32(3.14)},
			key:      "missing",
			expected: 0.0,
		},
		{
			name:     "invalid string",
			props:    Props{"value": "invalid"},
			key:      "value",
			expected: 0.0,
		},
		{
			name:     "nil props",
			props:    nil,
			key:      "any",
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.props.Float32(tt.key)
			if result != tt.expected {
				t.Errorf("Props.Float32(%q) = %f, want %f", tt.key, result, tt.expected)
			}
		})
	}
}

func TestNewIsland(t *testing.T) {
	t.Run("basic creation", func(t *testing.T) {
		island, err := NewIsland("counter")
		if err != nil {
			t.Fatalf("NewIsland() error = %v", err)
		}
		if island.Name != "counter" {
			t.Errorf("Island.Name = %q, want %q", island.Name, "counter")
		}
	})

	t.Run("default handlers", func(t *testing.T) {
		island, err := NewIsland("counter")
		if err != nil {
			t.Fatalf("NewIsland() error = %v", err)
		}

		// Test default mount handler
		state, err := island.Mount(context.Background(), Props{}, "")
		if err != nil {
			t.Errorf("default Mount() error = %v", err)
		}
		if state != nil {
			t.Errorf("default Mount() state = %v, want nil", state)
		}

		// Test default unmount handler
		err = island.Unmount(context.Background(), nil)
		if err != nil {
			t.Errorf("default Unmount() error = %v", err)
		}

		// Test default render handler returns ErrNoRenderer
		_, err = island.Render(context.Background(), &IslandRenderContext{})
		if !errors.Is(err, ErrNoRenderer) {
			t.Errorf("default Render() error = %v, want %v", err, ErrNoRenderer)
		}
	})

	t.Run("with mount config", func(t *testing.T) {
		expectedState := map[string]int{"count": 0}
		mountCalled := false

		island, err := NewIsland("counter", WithMount(func(ctx context.Context, props Props, children string) (any, error) {
			mountCalled = true
			return expectedState, nil
		}))
		if err != nil {
			t.Fatalf("NewIsland() error = %v", err)
		}

		state, err := island.Mount(context.Background(), Props{}, "")
		if err != nil {
			t.Errorf("Mount() error = %v", err)
		}
		if !mountCalled {
			t.Error("mount handler was not called")
		}
		if state == nil {
			t.Error("Mount() state = nil, want non-nil")
		}
	})

	t.Run("with render config", func(t *testing.T) {
		expectedHTML := "<div>Hello</div>"

		island, err := NewIsland("counter", WithRender(func(ctx context.Context, rc *IslandRenderContext) (io.Reader, error) {
			return strings.NewReader(expectedHTML), nil
		}))
		if err != nil {
			t.Fatalf("NewIsland() error = %v", err)
		}

		reader, err := island.Render(context.Background(), &IslandRenderContext{})
		if err != nil {
			t.Errorf("Render() error = %v", err)
		}
		buf := new(strings.Builder)
		_, _ = io.Copy(buf, reader)
		if buf.String() != expectedHTML {
			t.Errorf("Render() = %q, want %q", buf.String(), expectedHTML)
		}
	})

	t.Run("with unmount config", func(t *testing.T) {
		unmountCalled := false

		island, err := NewIsland("counter", WithUnmount(func(ctx context.Context, state any) error {
			unmountCalled = true
			return nil
		}))
		if err != nil {
			t.Fatalf("NewIsland() error = %v", err)
		}

		err = island.Unmount(context.Background(), nil)
		if err != nil {
			t.Errorf("Unmount() error = %v", err)
		}
		if !unmountCalled {
			t.Error("unmount handler was not called")
		}
	})

	t.Run("with multiple configs", func(t *testing.T) {
		mountCalled := false
		renderCalled := false

		island, err := NewIsland("counter",
			WithMount(func(ctx context.Context, props Props, children string) (any, error) {
				mountCalled = true
				return nil, nil
			}),
			WithRender(func(ctx context.Context, rc *IslandRenderContext) (io.Reader, error) {
				renderCalled = true
				return strings.NewReader(""), nil
			}),
		)
		if err != nil {
			t.Fatalf("NewIsland() error = %v", err)
		}

		_, _ = island.Mount(context.Background(), Props{}, "")
		_, _ = island.Render(context.Background(), &IslandRenderContext{})

		if !mountCalled {
			t.Error("mount handler was not called")
		}
		if !renderCalled {
			t.Error("render handler was not called")
		}
	})

	t.Run("config error", func(t *testing.T) {
		expectedErr := errors.New("config error")

		_, err := NewIsland("counter", func(i *Island) error {
			return expectedErr
		})
		if err == nil {
			t.Error("NewIsland() expected error, got nil")
		}
		if !errors.Is(err, expectedErr) {
			t.Errorf("NewIsland() error = %v, want %v", err, expectedErr)
		}
	})
}

func TestIslandHandleEvent(t *testing.T) {
	t.Run("register and retrieve handler", func(t *testing.T) {
		island, _ := NewIsland("counter")
		handlerCalled := false

		err := island.HandleEvent("increment", func(ctx context.Context, state any, params Params) (any, error) {
			handlerCalled = true
			return state, nil
		})
		if err != nil {
			t.Fatalf("HandleEvent() error = %v", err)
		}

		handler, err := island.GetEventHandler("increment")
		if err != nil {
			t.Fatalf("GetEventHandler() error = %v", err)
		}

		_, _ = handler(context.Background(), nil, Params{})
		if !handlerCalled {
			t.Error("retrieved handler was not callable")
		}
	})

	t.Run("duplicate registration error", func(t *testing.T) {
		island, _ := NewIsland("counter")

		err := island.HandleEvent("increment", func(ctx context.Context, state any, params Params) (any, error) {
			return state, nil
		})
		if err != nil {
			t.Fatalf("first HandleEvent() error = %v", err)
		}

		err = island.HandleEvent("increment", func(ctx context.Context, state any, params Params) (any, error) {
			return state, nil
		})
		if !errors.Is(err, ErrDuplicateEventHandler) {
			t.Errorf("second HandleEvent() error = %v, want %v", err, ErrDuplicateEventHandler)
		}
	})

	t.Run("missing handler error", func(t *testing.T) {
		island, _ := NewIsland("counter")

		_, err := island.GetEventHandler("nonexistent")
		if !errors.Is(err, ErrNoEventHandler) {
			t.Errorf("GetEventHandler() error = %v, want %v", err, ErrNoEventHandler)
		}
	})

	t.Run("multiple handlers", func(t *testing.T) {
		island, _ := NewIsland("counter")

		_ = island.HandleEvent("increment", func(ctx context.Context, state any, params Params) (any, error) {
			return "increment", nil
		})
		_ = island.HandleEvent("decrement", func(ctx context.Context, state any, params Params) (any, error) {
			return "decrement", nil
		})

		incHandler, _ := island.GetEventHandler("increment")
		decHandler, _ := island.GetEventHandler("decrement")

		incResult, _ := incHandler(context.Background(), nil, Params{})
		decResult, _ := decHandler(context.Background(), nil, Params{})

		if incResult != "increment" {
			t.Errorf("increment handler result = %v, want %q", incResult, "increment")
		}
		if decResult != "decrement" {
			t.Errorf("decrement handler result = %v, want %q", decResult, "decrement")
		}
	})

	t.Run("list event handlers", func(t *testing.T) {
		island, _ := NewIsland("counter")

		_ = island.HandleEvent("increment", func(ctx context.Context, state any, params Params) (any, error) {
			return state, nil
		})
		_ = island.HandleEvent("decrement", func(ctx context.Context, state any, params Params) (any, error) {
			return state, nil
		})

		handlers := island.EventHandlers()
		if len(handlers) != 2 {
			t.Errorf("EventHandlers() returned %d handlers, want 2", len(handlers))
		}

		handlerMap := make(map[string]bool)
		for _, h := range handlers {
			handlerMap[h] = true
		}
		if !handlerMap["increment"] {
			t.Error("EventHandlers() missing 'increment'")
		}
		if !handlerMap["decrement"] {
			t.Error("EventHandlers() missing 'decrement'")
		}
	})
}

func TestIslandHandleSelf(t *testing.T) {
	t.Run("register and retrieve handler", func(t *testing.T) {
		island, _ := NewIsland("counter")
		handlerCalled := false

		err := island.HandleSelf("refresh", func(ctx context.Context, state any, data any) (any, error) {
			handlerCalled = true
			return state, nil
		})
		if err != nil {
			t.Fatalf("HandleSelf() error = %v", err)
		}

		handler, err := island.GetSelfHandler("refresh")
		if err != nil {
			t.Fatalf("GetSelfHandler() error = %v", err)
		}

		_, _ = handler(context.Background(), nil, nil)
		if !handlerCalled {
			t.Error("retrieved handler was not callable")
		}
	})

	t.Run("duplicate registration error", func(t *testing.T) {
		island, _ := NewIsland("counter")

		err := island.HandleSelf("refresh", func(ctx context.Context, state any, data any) (any, error) {
			return state, nil
		})
		if err != nil {
			t.Fatalf("first HandleSelf() error = %v", err)
		}

		err = island.HandleSelf("refresh", func(ctx context.Context, state any, data any) (any, error) {
			return state, nil
		})
		if !errors.Is(err, ErrDuplicateSelfHandler) {
			t.Errorf("second HandleSelf() error = %v, want %v", err, ErrDuplicateSelfHandler)
		}
	})

	t.Run("missing handler error", func(t *testing.T) {
		island, _ := NewIsland("counter")

		_, err := island.GetSelfHandler("nonexistent")
		if !errors.Is(err, ErrNoSelfHandler) {
			t.Errorf("GetSelfHandler() error = %v, want %v", err, ErrNoSelfHandler)
		}
	})

	t.Run("list self handlers", func(t *testing.T) {
		island, _ := NewIsland("counter")

		_ = island.HandleSelf("refresh", func(ctx context.Context, state any, data any) (any, error) {
			return state, nil
		})
		_ = island.HandleSelf("update", func(ctx context.Context, state any, data any) (any, error) {
			return state, nil
		})

		handlers := island.SelfHandlers()
		if len(handlers) != 2 {
			t.Errorf("SelfHandlers() returned %d handlers, want 2", len(handlers))
		}

		handlerMap := make(map[string]bool)
		for _, h := range handlers {
			handlerMap[h] = true
		}
		if !handlerMap["refresh"] {
			t.Error("SelfHandlers() missing 'refresh'")
		}
		if !handlerMap["update"] {
			t.Error("SelfHandlers() missing 'update'")
		}
	})
}

func TestIslandRenderContext(t *testing.T) {
	t.Run("render with context", func(t *testing.T) {
		type CounterState struct {
			Count int
		}

		island, _ := NewIsland("counter", WithRender(func(ctx context.Context, rc *IslandRenderContext) (io.Reader, error) {
			state := rc.State.(*CounterState)
			name := rc.Props.String("name")
			return strings.NewReader("<div>" + name + ": " + string(rune('0'+state.Count)) + "</div>"), nil
		}))

		rc := &IslandRenderContext{
			State:    &CounterState{Count: 5},
			Props:    Props{"name": "counter"},
			Children: "",
		}

		reader, err := island.Render(context.Background(), rc)
		if err != nil {
			t.Fatalf("Render() error = %v", err)
		}

		buf := new(strings.Builder)
		_, _ = io.Copy(buf, reader)
		if buf.String() != "<div>counter: 5</div>" {
			t.Errorf("Render() = %q, want %q", buf.String(), "<div>counter: 5</div>")
		}
	})
}

func TestIslandConcurrentAccess(t *testing.T) {
	island, _ := NewIsland("counter")

	var wg sync.WaitGroup
	numGoroutines := 100

	// Concurrent handler registration
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			// Each goroutine tries to register a unique event
			eventName := "event" + string(rune('a'+idx%26)) + string(rune('0'+idx/26))
			_ = island.HandleEvent(eventName, func(ctx context.Context, state any, params Params) (any, error) {
				return state, nil
			})
		}(i)
	}

	// Concurrent handler retrieval
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			eventName := "event" + string(rune('a'+idx%26)) + string(rune('0'+idx/26))
			_, _ = island.GetEventHandler(eventName)
		}(i)
	}

	// Concurrent list operations
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = island.EventHandlers()
			_ = island.SelfHandlers()
		}()
	}

	wg.Wait()
	// Test passes if no race conditions detected
}

func TestIslandEventHandlerWithParams(t *testing.T) {
	island, _ := NewIsland("counter")

	_ = island.HandleEvent("set", func(ctx context.Context, state any, params Params) (any, error) {
		value := params.Int("value")
		return value, nil
	})

	handler, _ := island.GetEventHandler("set")
	result, err := handler(context.Background(), nil, Params{"value": 42})
	if err != nil {
		t.Fatalf("handler error = %v", err)
	}
	if result != 42 {
		t.Errorf("handler result = %v, want 42", result)
	}
}

func TestIslandSelfHandlerWithData(t *testing.T) {
	island, _ := NewIsland("counter")

	_ = island.HandleSelf("update", func(ctx context.Context, state any, data any) (any, error) {
		return data, nil
	})

	handler, _ := island.GetSelfHandler("update")
	result, err := handler(context.Background(), nil, "new state")
	if err != nil {
		t.Fatalf("handler error = %v", err)
	}
	if result != "new state" {
		t.Errorf("handler result = %v, want %q", result, "new state")
	}
}

// --- New feature tests (RED) ---

func TestWithErrorHandler(t *testing.T) {
	t.Run("custom error handler is set", func(t *testing.T) {
		customHandlerCalled := false
		customHandler := func(ctx context.Context, err error) Event {
			customHandlerCalled = true
			return Event{T: EventError}
		}

		island, err := NewIsland("counter", WithErrorHandler(customHandler))
		if err != nil {
			t.Fatalf("NewIsland() error = %v", err)
		}

		if island.errorHandler == nil {
			t.Fatal("island.errorHandler = nil, want non-nil")
		}

		// Call the stored handler to confirm it is our custom one
		island.errorHandler(context.Background(), errors.New("test"))
		if !customHandlerCalled {
			t.Error("custom error handler was not stored / not called")
		}
	})

	t.Run("default error handler returns error event", func(t *testing.T) {
		island, err := NewIsland("counter")
		if err != nil {
			t.Fatalf("NewIsland() error = %v", err)
		}

		if island.errorHandler == nil {
			t.Fatal("island.errorHandler = nil, want default handler")
		}

		testErr := errors.New("test error")
		event := island.errorHandler(context.Background(), testErr)

		if event.T != EventError {
			t.Errorf("default error handler event.T = %q, want %q", event.T, EventError)
		}

		// Data should be JSON {"err": "test error"}
		if event.Data == nil {
			t.Fatal("default error handler event.Data = nil, want JSON payload")
		}

		var payload map[string]string
		if err := json.Unmarshal(event.Data, &payload); err != nil {
			t.Fatalf("failed to unmarshal event.Data: %v", err)
		}

		if payload["err"] != testErr.Error() {
			t.Errorf("event.Data[\"err\"] = %q, want %q", payload["err"], testErr.Error())
		}
	})
}

func TestWithEventDelay(t *testing.T) {
	t.Run("configures delay for event", func(t *testing.T) {
		island, err := NewIsland("counter", WithEventDelay("tick", 100*time.Millisecond))
		if err != nil {
			t.Fatalf("NewIsland() error = %v", err)
		}

		delay, ok := island.GetEventDelay("tick")
		if !ok {
			t.Fatal("GetEventDelay(\"tick\") ok = false, want true")
		}
		if delay != 100*time.Millisecond {
			t.Errorf("GetEventDelay(\"tick\") = %v, want %v", delay, 100*time.Millisecond)
		}
	})

	t.Run("multiple delays can be configured", func(t *testing.T) {
		island, err := NewIsland("counter",
			WithEventDelay("tick", 100*time.Millisecond),
			WithEventDelay("refresh", 5*time.Second),
		)
		if err != nil {
			t.Fatalf("NewIsland() error = %v", err)
		}

		tickDelay, ok := island.GetEventDelay("tick")
		if !ok {
			t.Fatal("GetEventDelay(\"tick\") ok = false, want true")
		}
		if tickDelay != 100*time.Millisecond {
			t.Errorf("GetEventDelay(\"tick\") = %v, want %v", tickDelay, 100*time.Millisecond)
		}

		refreshDelay, ok := island.GetEventDelay("refresh")
		if !ok {
			t.Fatal("GetEventDelay(\"refresh\") ok = false, want true")
		}
		if refreshDelay != 5*time.Second {
			t.Errorf("GetEventDelay(\"refresh\") = %v, want %v", refreshDelay, 5*time.Second)
		}
	})
}

func TestGetEventDelay(t *testing.T) {
	t.Run("returns delay for configured event", func(t *testing.T) {
		island, err := NewIsland("counter", WithEventDelay("tick", 200*time.Millisecond))
		if err != nil {
			t.Fatalf("NewIsland() error = %v", err)
		}

		delay, ok := island.GetEventDelay("tick")
		if !ok {
			t.Fatal("GetEventDelay(\"tick\") ok = false, want true")
		}
		if delay != 200*time.Millisecond {
			t.Errorf("GetEventDelay(\"tick\") = %v, want %v", delay, 200*time.Millisecond)
		}
	})

	t.Run("returns false for unconfigured event", func(t *testing.T) {
		island, err := NewIsland("counter")
		if err != nil {
			t.Fatalf("NewIsland() error = %v", err)
		}

		delay, ok := island.GetEventDelay("unknown")
		if ok {
			t.Errorf("GetEventDelay(\"unknown\") ok = true, want false")
		}
		if delay != 0 {
			t.Errorf("GetEventDelay(\"unknown\") delay = %v, want 0", delay)
		}
	})
}

func TestSendSelf(t *testing.T) {
	t.Run("enqueues event when queue is in context", func(t *testing.T) {
		queue := &[]Event{}
		ctx := context.Background()
		ctx = contextWithSelfEventQueue(ctx, queue)
		ctx = contextWithIslandID(ctx, IslandID("counter-1"))

		SendSelf(ctx, "tick", "payload")

		if len(*queue) != 1 {
			t.Fatalf("queue length = %d, want 1", len(*queue))
		}

		event := (*queue)[0]
		if event.T != "tick" {
			t.Errorf("event.T = %q, want %q", event.T, "tick")
		}
		if event.Island != "counter-1" {
			t.Errorf("event.Island = %q, want %q", event.Island, "counter-1")
		}
		if event.SelfData != "payload" {
			t.Errorf("event.SelfData = %v, want %q", event.SelfData, "payload")
		}
	})

	t.Run("no panic when queue not in context", func(t *testing.T) {
		// SendSelf should be a silent no-op when queue is not in context
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("SendSelf panicked with plain context: %v", r)
			}
		}()
		SendSelf(context.Background(), "tick", "payload")
	})
}

func TestDefaultErrorHandler(t *testing.T) {
	t.Run("returns error event with message", func(t *testing.T) {
		testErr := errors.New("something went wrong")
		event := defaultErrorHandler(context.Background(), testErr)

		if event.T != EventError {
			t.Errorf("defaultErrorHandler event.T = %q, want %q", event.T, EventError)
		}

		if event.Data == nil {
			t.Fatal("defaultErrorHandler event.Data = nil, want JSON payload")
		}

		var payload map[string]string
		if err := json.Unmarshal(event.Data, &payload); err != nil {
			t.Fatalf("failed to unmarshal event.Data: %v", err)
		}

		if payload["err"] != testErr.Error() {
			t.Errorf("event.Data[\"err\"] = %q, want %q", payload["err"], testErr.Error())
		}
	})
}
