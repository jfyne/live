package live

import (
	"context"
	"sync"
	"time"
)

// State represents island state as a flexible key-value map.
// It can store any type of data needed by an island instance.
type State = any

// IslandStateStore defines the interface for storing and retrieving island state.
// State is stored using a composite key of (SessionID, IslandID) to isolate
// state per island instance within a session.
type IslandStateStore interface {
	// Get retrieves the state for a specific island instance within a session.
	// Returns the state and true if found, or nil and false if not found.
	Get(sessionID SessionID, islandID IslandID) (State, bool)

	// Set stores the state for a specific island instance with a TTL.
	// The state will be automatically removed after the TTL expires.
	Set(sessionID SessionID, islandID IslandID, state State, ttl time.Duration)

	// Delete removes the state for a specific island instance.
	Delete(sessionID SessionID, islandID IslandID)

	// DeleteSession removes all island states for a given session.
	// This is typically called when a session ends.
	DeleteSession(sessionID SessionID)
}

// stateEntry represents a single state entry in the store with expiration tracking.
type stateEntry struct {
	state     State
	expiresAt time.Time
}

// MemoryIslandStateStore is an in-memory implementation of IslandStateStore.
// It uses a nested map structure: map[SessionID]map[IslandID]stateEntry
// and includes a janitor goroutine for automatic TTL-based cleanup.
type MemoryIslandStateStore struct {
	// mu protects concurrent access to the store.
	mu sync.RWMutex

	// store maps SessionID -> IslandID -> stateEntry.
	store map[SessionID]map[IslandID]stateEntry

	// ctx is the context for lifecycle management.
	ctx context.Context

	// cleanupInterval determines how often the janitor runs.
	cleanupInterval time.Duration
}

var _ IslandStateStore = (*MemoryIslandStateStore)(nil)

// NewMemoryIslandStateStore creates a new in-memory island state store.
// The janitor goroutine starts automatically and runs at the specified cleanup interval.
// The janitor stops when the provided context is canceled.
func NewMemoryIslandStateStore(ctx context.Context, cleanupInterval time.Duration) *MemoryIslandStateStore {
	if cleanupInterval <= 0 {
		cleanupInterval = 1 * time.Minute // reasonable default
	}

	store := &MemoryIslandStateStore{
		store:           make(map[SessionID]map[IslandID]stateEntry),
		ctx:             ctx,
		cleanupInterval: cleanupInterval,
	}

	// Start the janitor goroutine for TTL-based cleanup.
	go store.janitor()

	return store
}

// Get retrieves the state for a specific island instance within a session.
// Returns the state and true if found and not expired, or nil and false otherwise.
func (m *MemoryIslandStateStore) Get(sessionID SessionID, islandID IslandID) (State, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessionStates, ok := m.store[sessionID]
	if !ok {
		return nil, false
	}

	entry, ok := sessionStates[islandID]
	if !ok {
		return nil, false
	}

	// Check if the entry has expired.
	if time.Now().After(entry.expiresAt) {
		return nil, false
	}

	return entry.state, true
}

// Set stores the state for a specific island instance with a TTL.
// The state will be automatically removed by the janitor after the TTL expires.
func (m *MemoryIslandStateStore) Set(sessionID SessionID, islandID IslandID, state State, ttl time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Ensure the session map exists.
	if m.store[sessionID] == nil {
		m.store[sessionID] = make(map[IslandID]stateEntry)
	}

	// Store the state with expiration time.
	m.store[sessionID][islandID] = stateEntry{
		state:     state,
		expiresAt: time.Now().Add(ttl),
	}
}

// Delete removes the state for a specific island instance.
func (m *MemoryIslandStateStore) Delete(sessionID SessionID, islandID IslandID) {
	m.mu.Lock()
	defer m.mu.Unlock()

	sessionStates, ok := m.store[sessionID]
	if !ok {
		return
	}

	delete(sessionStates, islandID)

	// Clean up the session map if it's empty.
	if len(sessionStates) == 0 {
		delete(m.store, sessionID)
	}
}

// DeleteSession removes all island states for a given session.
func (m *MemoryIslandStateStore) DeleteSession(sessionID SessionID) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.store, sessionID)
}

// janitor runs periodically to clean up expired state entries.
// It scans all entries and removes those that have passed their expiration time.
// The janitor stops when the store's context is canceled.
func (m *MemoryIslandStateStore) janitor() {
	ticker := time.NewTicker(m.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.cleanup()
		case <-m.ctx.Done():
			return
		}
	}
}

// cleanup removes all expired state entries from the store.
func (m *MemoryIslandStateStore) cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()

	// Iterate through all sessions.
	for sessionID, sessionStates := range m.store {
		// Iterate through all islands in the session.
		for islandID, entry := range sessionStates {
			// Remove expired entries.
			if now.After(entry.expiresAt) {
				delete(sessionStates, islandID)
			}
		}

		// Clean up empty session maps.
		if len(sessionStates) == 0 {
			delete(m.store, sessionID)
		}
	}
}
