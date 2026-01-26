package live

import (
	"context"
	"html/template"
	"io"
	"sync"
)

// IslandInstance represents a running instance of an island.
// Each instance has its own isolated state and lifecycle.
type IslandInstance struct {
	// ID is the unique identifier for this instance.
	ID string

	// Type is the island type name (used to look up the island definition).
	Type string

	// island is the reference to the Island definition.
	island *Island

	// state is the current instance state.
	state any

	// props are the Props passed at creation.
	props Props

	// children is any content nested within the island element.
	children string

	// mu protects concurrent access to state.
	mu sync.RWMutex

	// mounted tracks whether the instance has been mounted.
	mounted bool

	// lastRenderedHTML stores the last rendered HTML for diffing.
	lastRenderedHTML template.HTML
}

// NewIslandInstance creates a new island instance with the given ID, type, and props.
// It looks up the island constructor from the registry and creates the Island definition.
// Returns ErrIslandNotFound if the island type is not registered.
func NewIslandInstance(id, islandType string, props Props) (*IslandInstance, error) {
	return NewIslandInstanceFromRegistry(id, islandType, props, defaultRegistry)
}

// NewIslandInstanceFromRegistry creates a new island instance using a specific registry.
// This is useful for testing or when using multiple registries.
func NewIslandInstanceFromRegistry(id, islandType string, props Props, registry *IslandRegistry) (*IslandInstance, error) {
	constructor, err := registry.Get(islandType)
	if err != nil {
		return nil, err
	}

	island, err := constructor()
	if err != nil {
		return nil, err
	}

	if props == nil {
		props = Props{}
	}

	return &IslandInstance{
		ID:       id,
		Type:     islandType,
		island:   island,
		props:    props,
		children: "",
		mounted:  false,
	}, nil
}

// NewIslandInstanceWithChildren creates a new island instance with nested children content.
func NewIslandInstanceWithChildren(id, islandType string, props Props, children string) (*IslandInstance, error) {
	instance, err := NewIslandInstance(id, islandType, props)
	if err != nil {
		return nil, err
	}
	instance.children = children
	return instance, nil
}

// Mount initializes the island instance by calling the island's mount handler.
// The mount handler receives the props and children and returns the initial state.
// Mount should only be called once per instance.
func (i *IslandInstance) Mount(ctx context.Context) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	if i.mounted {
		return ErrAlreadyMounted
	}

	state, err := i.island.Mount(ctx, i.props, i.children)
	if err != nil {
		return err
	}

	i.state = state
	i.mounted = true
	return nil
}

// Render generates the HTML for the island's current state.
// Returns the rendered HTML as template.HTML for safe embedding.
func (i *IslandInstance) Render(ctx context.Context) (template.HTML, error) {
	i.mu.RLock()
	state := i.state
	props := i.props
	children := i.children
	i.mu.RUnlock()

	rc := &IslandRenderContext{
		State:    state,
		Props:    props,
		Children: children,
	}

	reader, err := i.island.Render(ctx, rc)
	if err != nil {
		return "", err
	}

	if reader == nil {
		return "", nil
	}

	content, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}

	result := template.HTML(content)

	// Store the rendered HTML for diffing
	i.mu.Lock()
	i.lastRenderedHTML = result
	i.mu.Unlock()

	return result, nil
}

// Unmount cleans up the island instance by calling the island's unmount handler.
// After unmounting, the instance should not be used.
func (i *IslandInstance) Unmount(ctx context.Context) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	if !i.mounted {
		return nil
	}

	err := i.island.Unmount(ctx, i.state)
	if err != nil {
		return err
	}

	i.mounted = false
	i.state = nil
	return nil
}

// CallEvent handles an event from the client.
// It looks up the event handler, calls it with the current state and params,
// and updates the state with the returned value.
// Returns ErrNoEventHandler if no handler is registered for the event.
func (i *IslandInstance) CallEvent(ctx context.Context, event string, params Params) error {
	handler, err := i.island.GetEventHandler(event)
	if err != nil {
		return err
	}

	i.mu.Lock()
	defer i.mu.Unlock()

	newState, err := handler(ctx, i.state, params)
	if err != nil {
		return err
	}

	i.state = newState
	return nil
}

// CallSelf handles a self-directed event from the server.
// It looks up the self handler, calls it with the current state and data,
// and updates the state with the returned value.
// Returns ErrNoSelfHandler if no handler is registered for the event.
func (i *IslandInstance) CallSelf(ctx context.Context, event string, data any) error {
	handler, err := i.island.GetSelfHandler(event)
	if err != nil {
		return err
	}

	i.mu.Lock()
	defer i.mu.Unlock()

	newState, err := handler(ctx, i.state, data)
	if err != nil {
		return err
	}

	i.state = newState
	return nil
}

// State returns the current state of the island instance.
// This method is thread-safe.
func (i *IslandInstance) State() any {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.state
}

// SetState updates the state of the island instance.
// This method is thread-safe.
func (i *IslandInstance) SetState(state any) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.state = state
}

// Props returns the props passed to the island instance.
func (i *IslandInstance) Props() Props {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.props
}

// Children returns the nested children content.
func (i *IslandInstance) Children() string {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.children
}

// IsMounted returns whether the instance has been mounted.
func (i *IslandInstance) IsMounted() bool {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.mounted
}

// Island returns the underlying island definition.
func (i *IslandInstance) Island() *Island {
	return i.island
}
