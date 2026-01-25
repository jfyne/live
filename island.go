package live

import (
	"context"
	"io"
	"sync"
)

// Props are the properties passed to an island instance.
// Props are extracted from the island element's attributes.
type Props map[string]any

// String returns the string value for a key, or empty string if not found
// or not a string.
func (p Props) String(key string) string {
	v, ok := p[key]
	if !ok {
		return ""
	}
	out, ok := v.(string)
	if !ok {
		return ""
	}
	return out
}

// Int returns the int value for a key, or 0 if not found or cannot be
// converted to int.
func (p Props) Int(key string) int {
	return mapInt(p, key)
}

// Bool returns the boolean value for a key. Returns true if the value
// is the string "true" or boolean true.
func (p Props) Bool(key string) bool {
	v, ok := p[key]
	if !ok {
		return false
	}
	switch val := v.(type) {
	case bool:
		return val
	case string:
		return val == "true"
	}
	return false
}

// Float32 returns the float32 value for a key, or 0.0 if not found or
// cannot be converted.
func (p Props) Float32(key string) float32 {
	return mapFloat32(p, key)
}

// IslandConfig applies configuration to an Island.
type IslandConfig func(i *Island) error

// IslandMountHandler is called when an island instance is mounted.
// It receives the context, props, and any initial children content.
// Returns the initial state for the island instance.
type IslandMountHandler func(ctx context.Context, props Props, children string) (any, error)

// IslandUnmountHandler is called when an island instance is unmounted.
type IslandUnmountHandler func(ctx context.Context, state any) error

// IslandRenderHandler renders the island's current state.
// It receives the render context containing the state and returns the
// rendered HTML.
type IslandRenderHandler func(ctx context.Context, rc *IslandRenderContext) (io.Reader, error)

// IslandEventHandler handles events from the client.
// Returns the new state after handling the event.
type IslandEventHandler func(ctx context.Context, state any, params Params) (any, error)

// IslandSelfHandler handles self-directed events from the server.
// Returns the new state after handling the event.
type IslandSelfHandler func(ctx context.Context, state any, data any) (any, error)

// IslandRenderContext provides context for rendering an island.
type IslandRenderContext struct {
	// State is the current state of the island instance.
	State any
	// Props are the properties passed to the island.
	Props Props
	// Children is any content nested within the island element.
	Children string
}

// Island defines the behavior and handlers for an island type.
// Islands are registered once and can be instantiated multiple times.
type Island struct {
	// Name is the unique identifier for this island type.
	Name string

	// Mount is called when an island instance is created.
	Mount IslandMountHandler

	// Unmount is called when an island instance is destroyed.
	Unmount IslandUnmountHandler

	// Render generates the HTML for the island's current state.
	Render IslandRenderHandler

	// eventHandlers maps event names to their handlers.
	eventHandlers map[string]IslandEventHandler

	// selfHandlers maps self event names to their handlers.
	selfHandlers map[string]IslandSelfHandler

	// mu protects concurrent access to handler maps.
	mu sync.RWMutex
}

// NewIsland creates a new Island with the given name and optional configuration.
func NewIsland(name string, configs ...IslandConfig) (*Island, error) {
	i := &Island{
		Name:          name,
		eventHandlers: make(map[string]IslandEventHandler),
		selfHandlers:  make(map[string]IslandSelfHandler),
		Mount: func(ctx context.Context, props Props, children string) (any, error) {
			return nil, nil
		},
		Unmount: func(ctx context.Context, state any) error {
			return nil
		},
		Render: func(ctx context.Context, rc *IslandRenderContext) (io.Reader, error) {
			return nil, ErrNoRenderer
		},
	}

	for _, config := range configs {
		if err := config(i); err != nil {
			return nil, err
		}
	}

	return i, nil
}

// HandleEvent registers an event handler for the given event name.
// Returns an error if a handler is already registered for that event.
func (i *Island) HandleEvent(event string, handler IslandEventHandler) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	if _, exists := i.eventHandlers[event]; exists {
		return ErrDuplicateEventHandler
	}

	i.eventHandlers[event] = handler
	return nil
}

// HandleSelf registers a self handler for the given event name.
// Returns an error if a handler is already registered for that event.
func (i *Island) HandleSelf(event string, handler IslandSelfHandler) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	if _, exists := i.selfHandlers[event]; exists {
		return ErrDuplicateSelfHandler
	}

	i.selfHandlers[event] = handler
	return nil
}

// GetEventHandler returns the event handler for the given event name.
// Returns ErrNoEventHandler if no handler is registered.
func (i *Island) GetEventHandler(event string) (IslandEventHandler, error) {
	i.mu.RLock()
	defer i.mu.RUnlock()

	handler, ok := i.eventHandlers[event]
	if !ok {
		return nil, ErrNoEventHandler
	}
	return handler, nil
}

// GetSelfHandler returns the self handler for the given event name.
// Returns ErrNoSelfHandler if no handler is registered.
func (i *Island) GetSelfHandler(event string) (IslandSelfHandler, error) {
	i.mu.RLock()
	defer i.mu.RUnlock()

	handler, ok := i.selfHandlers[event]
	if !ok {
		return nil, ErrNoSelfHandler
	}
	return handler, nil
}

// EventHandlers returns a copy of the event handler names.
func (i *Island) EventHandlers() []string {
	i.mu.RLock()
	defer i.mu.RUnlock()

	names := make([]string, 0, len(i.eventHandlers))
	for name := range i.eventHandlers {
		names = append(names, name)
	}
	return names
}

// SelfHandlers returns a copy of the self handler names.
func (i *Island) SelfHandlers() []string {
	i.mu.RLock()
	defer i.mu.RUnlock()

	names := make([]string, 0, len(i.selfHandlers))
	for name := range i.selfHandlers {
		names = append(names, name)
	}
	return names
}

// WithMount sets the mount handler for the island.
func WithMount(handler IslandMountHandler) IslandConfig {
	return func(i *Island) error {
		i.Mount = handler
		return nil
	}
}

// WithUnmount sets the unmount handler for the island.
func WithUnmount(handler IslandUnmountHandler) IslandConfig {
	return func(i *Island) error {
		i.Unmount = handler
		return nil
	}
}

// WithRender sets the render handler for the island.
func WithRender(handler IslandRenderHandler) IslandConfig {
	return func(i *Island) error {
		i.Render = handler
		return nil
	}
}
