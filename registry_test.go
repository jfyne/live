package live

import (
	"errors"
	"sync"
	"testing"
)

func TestIslandRegistryRegister(t *testing.T) {
	t.Run("register success", func(t *testing.T) {
		registry := NewIslandRegistry()
		err := registry.Register("counter", func() (*Island, error) {
			return NewIsland("counter")
		})
		if err != nil {
			t.Errorf("Register() error = %v, want nil", err)
		}
	})

	t.Run("register multiple islands", func(t *testing.T) {
		registry := NewIslandRegistry()

		err := registry.Register("counter", func() (*Island, error) {
			return NewIsland("counter")
		})
		if err != nil {
			t.Errorf("Register(counter) error = %v", err)
		}

		err = registry.Register("todo", func() (*Island, error) {
			return NewIsland("todo")
		})
		if err != nil {
			t.Errorf("Register(todo) error = %v", err)
		}

		err = registry.Register("chat", func() (*Island, error) {
			return NewIsland("chat")
		})
		if err != nil {
			t.Errorf("Register(chat) error = %v", err)
		}
	})

	t.Run("duplicate registration error", func(t *testing.T) {
		registry := NewIslandRegistry()

		err := registry.Register("counter", func() (*Island, error) {
			return NewIsland("counter")
		})
		if err != nil {
			t.Fatalf("first Register() error = %v", err)
		}

		err = registry.Register("counter", func() (*Island, error) {
			return NewIsland("counter")
		})
		if !errors.Is(err, ErrIslandAlreadyRegistered) {
			t.Errorf("second Register() error = %v, want %v", err, ErrIslandAlreadyRegistered)
		}
	})
}

func TestIslandRegistryGet(t *testing.T) {
	t.Run("get existing island", func(t *testing.T) {
		registry := NewIslandRegistry()
		expectedName := "counter"

		err := registry.Register(expectedName, func() (*Island, error) {
			return NewIsland(expectedName)
		})
		if err != nil {
			t.Fatalf("Register() error = %v", err)
		}

		constructor, err := registry.Get(expectedName)
		if err != nil {
			t.Errorf("Get() error = %v, want nil", err)
		}
		if constructor == nil {
			t.Fatal("Get() returned nil constructor")
		}

		// Verify the constructor works
		island, err := constructor()
		if err != nil {
			t.Errorf("constructor() error = %v", err)
		}
		if island.Name != expectedName {
			t.Errorf("island.Name = %q, want %q", island.Name, expectedName)
		}
	})

	t.Run("get non-existent island", func(t *testing.T) {
		registry := NewIslandRegistry()

		_, err := registry.Get("nonexistent")
		if !errors.Is(err, ErrIslandNotFound) {
			t.Errorf("Get() error = %v, want %v", err, ErrIslandNotFound)
		}
	})

	t.Run("get from empty registry", func(t *testing.T) {
		registry := NewIslandRegistry()

		_, err := registry.Get("anything")
		if !errors.Is(err, ErrIslandNotFound) {
			t.Errorf("Get() error = %v, want %v", err, ErrIslandNotFound)
		}
	})
}

func TestIslandRegistryList(t *testing.T) {
	t.Run("list empty registry", func(t *testing.T) {
		registry := NewIslandRegistry()

		names := registry.List()
		if len(names) != 0 {
			t.Errorf("List() returned %d names, want 0", len(names))
		}
	})

	t.Run("list single island", func(t *testing.T) {
		registry := NewIslandRegistry()
		_ = registry.Register("counter", func() (*Island, error) {
			return NewIsland("counter")
		})

		names := registry.List()
		if len(names) != 1 {
			t.Fatalf("List() returned %d names, want 1", len(names))
		}
		if names[0] != "counter" {
			t.Errorf("List()[0] = %q, want %q", names[0], "counter")
		}
	})

	t.Run("list multiple islands sorted", func(t *testing.T) {
		registry := NewIslandRegistry()
		// Register in non-alphabetical order
		_ = registry.Register("zebra", func() (*Island, error) {
			return NewIsland("zebra")
		})
		_ = registry.Register("alpha", func() (*Island, error) {
			return NewIsland("alpha")
		})
		_ = registry.Register("middle", func() (*Island, error) {
			return NewIsland("middle")
		})

		names := registry.List()
		if len(names) != 3 {
			t.Fatalf("List() returned %d names, want 3", len(names))
		}

		// Should be sorted alphabetically
		expected := []string{"alpha", "middle", "zebra"}
		for i, name := range names {
			if name != expected[i] {
				t.Errorf("List()[%d] = %q, want %q", i, name, expected[i])
			}
		}
	})
}

func TestIslandRegistryConcurrentAccess(t *testing.T) {
	registry := NewIslandRegistry()
	var wg sync.WaitGroup
	numGoroutines := 100

	// Concurrent registration
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			name := "island" + string(rune('a'+idx%26)) + string(rune('0'+idx/26))
			_ = registry.Register(name, func() (*Island, error) {
				return NewIsland(name)
			})
		}(i)
	}

	// Concurrent retrieval
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			name := "island" + string(rune('a'+idx%26)) + string(rune('0'+idx/26))
			_, _ = registry.Get(name)
		}(i)
	}

	// Concurrent listing
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = registry.List()
		}()
	}

	wg.Wait()
	// Test passes if no race conditions detected
}

func TestIslandRegistryConstructorIsCalled(t *testing.T) {
	registry := NewIslandRegistry()
	constructorCalled := false

	err := registry.Register("counter", func() (*Island, error) {
		constructorCalled = true
		return NewIsland("counter")
	})
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	// Constructor should not be called on registration
	if constructorCalled {
		t.Error("constructor was called on registration, should only be called on Get")
	}

	constructor, err := registry.Get("counter")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	// Constructor should still not be called - only Get returns it
	if constructorCalled {
		t.Error("constructor was called on Get, should only be called when invoked")
	}

	// Now invoke the constructor
	_, err = constructor()
	if err != nil {
		t.Fatalf("constructor() error = %v", err)
	}

	if !constructorCalled {
		t.Error("constructor was not called when invoked")
	}
}

func TestIslandRegistryConstructorError(t *testing.T) {
	registry := NewIslandRegistry()
	expectedErr := errors.New("construction failed")

	err := registry.Register("failing", func() (*Island, error) {
		return nil, expectedErr
	})
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	constructor, err := registry.Get("failing")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	_, err = constructor()
	if !errors.Is(err, expectedErr) {
		t.Errorf("constructor() error = %v, want %v", err, expectedErr)
	}
}

// Tests for global registry functions
func TestGlobalRegistry(t *testing.T) {
	// Note: These tests use the global registry, so we need unique names
	// to avoid conflicts with other tests

	t.Run("RegisterIsland and GetIsland", func(t *testing.T) {
		uniqueName := "global_test_counter_1"
		err := RegisterIsland(uniqueName, func() (*Island, error) {
			return NewIsland(uniqueName)
		})
		if err != nil {
			t.Errorf("RegisterIsland() error = %v", err)
		}

		constructor, err := GetIsland(uniqueName)
		if err != nil {
			t.Errorf("GetIsland() error = %v", err)
		}

		island, err := constructor()
		if err != nil {
			t.Fatalf("constructor() error = %v", err)
		}
		if island.Name != uniqueName {
			t.Errorf("island.Name = %q, want %q", island.Name, uniqueName)
		}
	})

	t.Run("RegisterIsland duplicate", func(t *testing.T) {
		uniqueName := "global_test_counter_2"
		err := RegisterIsland(uniqueName, func() (*Island, error) {
			return NewIsland(uniqueName)
		})
		if err != nil {
			t.Fatalf("first RegisterIsland() error = %v", err)
		}

		err = RegisterIsland(uniqueName, func() (*Island, error) {
			return NewIsland(uniqueName)
		})
		if !errors.Is(err, ErrIslandAlreadyRegistered) {
			t.Errorf("second RegisterIsland() error = %v, want %v", err, ErrIslandAlreadyRegistered)
		}
	})

	t.Run("GetIsland not found", func(t *testing.T) {
		_, err := GetIsland("nonexistent_global_island")
		if !errors.Is(err, ErrIslandNotFound) {
			t.Errorf("GetIsland() error = %v, want %v", err, ErrIslandNotFound)
		}
	})

	t.Run("ListIslands includes registered", func(t *testing.T) {
		uniqueName := "global_test_counter_3"
		err := RegisterIsland(uniqueName, func() (*Island, error) {
			return NewIsland(uniqueName)
		})
		if err != nil {
			t.Fatalf("RegisterIsland() error = %v", err)
		}

		names := ListIslands()
		found := false
		for _, name := range names {
			if name == uniqueName {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("ListIslands() does not contain %q", uniqueName)
		}
	})

	t.Run("DefaultRegistry returns singleton", func(t *testing.T) {
		r1 := DefaultRegistry()
		r2 := DefaultRegistry()
		if r1 != r2 {
			t.Error("DefaultRegistry() returned different instances")
		}
	})
}

func TestNewIslandRegistry(t *testing.T) {
	registry := NewIslandRegistry()
	if registry == nil {
		t.Fatal("NewIslandRegistry() returned nil")
	}
	if registry.constructors == nil {
		t.Error("NewIslandRegistry() constructors map is nil")
	}
}
