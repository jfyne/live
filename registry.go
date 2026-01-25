package live

import (
	"sort"
	"sync"
)

// IslandConstructor is a function that creates a new Island.
// Constructors are registered with the IslandRegistry and called
// when an island instance needs to be created.
type IslandConstructor func() (*Island, error)

// IslandRegistry manages the registration and retrieval of island constructors.
// It is thread-safe for concurrent access.
type IslandRegistry struct {
	mu           sync.RWMutex
	constructors map[string]IslandConstructor
}

// NewIslandRegistry creates a new empty IslandRegistry.
func NewIslandRegistry() *IslandRegistry {
	return &IslandRegistry{
		constructors: make(map[string]IslandConstructor),
	}
}

// Register adds an island constructor to the registry.
// Returns ErrIslandAlreadyRegistered if an island with the same name
// is already registered.
func (r *IslandRegistry) Register(name string, constructor IslandConstructor) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.constructors[name]; exists {
		return ErrIslandAlreadyRegistered
	}

	r.constructors[name] = constructor
	return nil
}

// Get retrieves an island constructor from the registry.
// Returns ErrIslandNotFound if no island with the given name is registered.
func (r *IslandRegistry) Get(name string) (IslandConstructor, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	constructor, exists := r.constructors[name]
	if !exists {
		return nil, ErrIslandNotFound
	}

	return constructor, nil
}

// List returns a sorted list of all registered island names.
func (r *IslandRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.constructors))
	for name := range r.constructors {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// defaultRegistry is the global singleton registry instance.
var defaultRegistry = NewIslandRegistry()

// RegisterIsland registers an island constructor with the global registry.
// Returns ErrIslandAlreadyRegistered if an island with the same name
// is already registered.
func RegisterIsland(name string, constructor IslandConstructor) error {
	return defaultRegistry.Register(name, constructor)
}

// GetIsland retrieves an island constructor from the global registry.
// Returns ErrIslandNotFound if no island with the given name is registered.
func GetIsland(name string) (IslandConstructor, error) {
	return defaultRegistry.Get(name)
}

// ListIslands returns a sorted list of all registered island names
// from the global registry.
func ListIslands() []string {
	return defaultRegistry.List()
}

// DefaultRegistry returns the global singleton registry instance.
// This can be used for testing or advanced use cases where direct
// registry access is needed.
func DefaultRegistry() *IslandRegistry {
	return defaultRegistry
}
