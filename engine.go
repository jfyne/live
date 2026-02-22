package live

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// IslandEngine is the main orchestration layer for the v2 islands architecture.
// It manages:
// - Multiple sessions (each session represents a client connection)
// - Island lifecycle (mount/unmount islands within sessions)
// - Event routing (route events to the correct island in the correct session)
// - State persistence (via IslandStateStore)
// - Broadcasting (send events to multiple islands)
//
// The engine is thread-safe and uses mutex-protected operations for session management.
type IslandEngine struct {
	// registry holds the island type definitions
	registry *IslandRegistry

	// stateStore persists island state across reconnections
	stateStore IslandStateStore

	// sessions maps session IDs to active sessions
	sessions map[SessionID]*Session

	// mu protects concurrent access to the sessions map and stateTTL
	mu sync.RWMutex

	// ctx is the engine context for lifecycle management
	ctx context.Context

	// cancel cancels the engine context
	cancel context.CancelFunc

	// stateTTL is the default TTL for island state
	stateTTL time.Duration
}

// NewIslandEngine creates a new island engine with the given registry and state store.
func NewIslandEngine(ctx context.Context, registry *IslandRegistry, stateStore IslandStateStore) *IslandEngine {
	engineCtx, cancel := context.WithCancel(ctx)

	e := &IslandEngine{
		registry:   registry,
		stateStore: stateStore,
		sessions:   make(map[SessionID]*Session),
		ctx:        engineCtx,
		cancel:     cancel,
		stateTTL:   24 * time.Hour, // Default 24 hour TTL
	}

	return e
}

// AddSession registers a new session with the engine.
// The session will be available for island mounting and event routing.
func (e *IslandEngine) AddSession(session *Session) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.sessions[session.ID] = session
}

// GetSession retrieves a session by ID.
// Returns the session and true if found, nil and false otherwise.
func (e *IslandEngine) GetSession(sessionID SessionID) (*Session, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	session, ok := e.sessions[sessionID]
	return session, ok
}

// DeleteSession removes a session from the engine.
// The session is closed and removed from the engine's session map,
// but island state is preserved in the state store for reconnection.
// State is cleaned up automatically by the store's TTL mechanism.
func (e *IslandEngine) DeleteSession(sessionID SessionID) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if session, ok := e.sessions[sessionID]; ok {
		_ = session.Close()
		delete(e.sessions, sessionID)
		// State is intentionally NOT deleted here to allow
		// state restoration when the client reconnects with
		// a new session ID. The state store's TTL will handle
		// cleanup of stale state.
	}
}

// MountIsland creates and mounts an island instance within a session.
// If existing state is found in the state store (e.g., from a previous
// session before reconnection), it is restored instead of using the
// mount handler's initial state.
//
// The flow is:
// 1. Get the session
// 2. Create the island instance from the registry
// 3. Call the island's Mount() handler (creates initial state)
// 4. Check the state store for existing state and restore if found
// 5. Add the instance to the session
// 6. Save the state to the state store under the new session ID
// 7. Render the island and send the initial patch to the client
//
// Returns the mounted instance or an error.
func (e *IslandEngine) MountIsland(sessionID SessionID, islandID IslandID, islandType string, props Props) (*IslandInstance, error) {
	// Get the session
	session, ok := e.GetSession(sessionID)
	if !ok {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	// Create the island instance from the registry
	instance, err := NewIslandInstanceFromRegistry(string(islandID), islandType, props, e.registry)
	if err != nil {
		return nil, fmt.Errorf("failed to create island instance: %w", err)
	}

	// Mount the island (calls the island's mount handler to create initial state)
	if err := instance.Mount(session.Context()); err != nil {
		return nil, fmt.Errorf("failed to mount island: %w", err)
	}

	// Check for existing state from a previous session (reconnection scenario).
	// First check the current session, then search across all sessions.
	if existingState, ok := e.stateStore.Get(sessionID, islandID); ok {
		instance.SetState(existingState)
	} else if existingState, ok := e.stateStore.GetByIslandID(islandID); ok {
		instance.SetState(existingState)
	}

	// Add the instance to the session
	session.AddIsland(instance)

	// Save the state to the state store under the current session ID
	e.mu.RLock()
	ttl := e.stateTTL
	e.mu.RUnlock()
	e.stateStore.Set(sessionID, islandID, instance.State(), ttl)

	// Render the island and send the initial patch to the client
	if err := e.renderAndSendIsland(session, instance); err != nil {
		// Non-fatal - the island is mounted but the client won't see the initial render
		slog.Error("failed to render and send island on mount",
			"island", instance.ID,
			"err", err)
	}

	return instance, nil
}

// UnmountIsland unmounts an island instance from a session.
// The flow is:
// 1. Get the session
// 2. Get the island instance from the session
// 3. Call the island's Unmount() handler
// 4. Remove the instance from the session
// 5. Delete the state from the state store
func (e *IslandEngine) UnmountIsland(sessionID SessionID, islandID IslandID) error {
	// Get the session
	session, ok := e.GetSession(sessionID)
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	// Get the island instance
	instance, ok := session.GetIsland(islandID)
	if !ok {
		return fmt.Errorf("island not found: %s", islandID)
	}

	// Call the island's unmount handler
	if err := instance.Unmount(session.Context()); err != nil {
		return fmt.Errorf("failed to unmount island: %w", err)
	}

	// Remove the instance from the session
	session.RemoveIsland(islandID)

	// Delete the state from the state store
	e.stateStore.Delete(sessionID, islandID)

	return nil
}

// RouteEvent routes an event to the correct island in a session.
// The session handles the actual event dispatch to the island.
// After the event is processed, the island is re-rendered and a patch is sent to the client.
func (e *IslandEngine) RouteEvent(sessionID SessionID, event Event) error {
	// Get the session
	session, ok := e.GetSession(sessionID)
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	// Extract the island ID from the event
	if event.Island == "" {
		return fmt.Errorf("event has no island field")
	}
	islandID := IslandID(event.Island)

	// Get the island instance before processing the event
	instance, ok := session.GetIsland(islandID)
	if !ok {
		return fmt.Errorf("island not found: %s", islandID)
	}

	// Let the session handle the event (this updates the island state)
	if err := session.handleEvent(event); err != nil {
		return fmt.Errorf("failed to handle event: %w", err)
	}

	// Save the updated state to the state store
	e.mu.RLock()
	ttl := e.stateTTL
	e.mu.RUnlock()
	e.stateStore.Set(sessionID, islandID, instance.State(), ttl)

	// Re-render the island and send the patch to the client
	if err := e.renderAndSendIsland(session, instance); err != nil {
		return fmt.Errorf("failed to render island: %w", err)
	}

	return nil
}

// renderAndSendIsland renders an island instance and sends a patch event to the client.
// This is used both for initial mount and after state changes.
func (e *IslandEngine) renderAndSendIsland(session *Session, instance *IslandInstance) error {
	// Get the previous HTML from the instance BEFORE rendering
	previousHTML := string(instance.lastRenderedHTML)

	// Render the island (this will update instance.lastRenderedHTML with the new value)
	html, err := RenderIsland(session.Context(), instance)
	if err != nil {
		return fmt.Errorf("render error: %w", err)
	}

	// Compute patches using DiffIsland
	patches, err := DiffIsland(IslandID(instance.ID), previousHTML, string(html))
	if err != nil {
		return fmt.Errorf("failed to compute diff: %w", err)
	}

	// Marshal the patches array (not a map with island/html)
	data, err := json.Marshal(patches)
	if err != nil {
		return fmt.Errorf("failed to marshal patches: %w", err)
	}

	patchEvent := Event{
		T:      EventPatch,
		Island: instance.ID,
		Data:   data,
	}

	return session.Send(patchEvent)
}

// BroadcastToIslandType sends an event to all islands of a specific type across all sessions.
// This is useful for broadcasting server-side events to all instances of an island type.
func (e *IslandEngine) BroadcastToIslandType(islandType string, event Event) {
	e.mu.RLock()
	sessions := make([]*Session, 0, len(e.sessions))
	for _, session := range e.sessions {
		sessions = append(sessions, session)
	}
	e.mu.RUnlock()

	// Iterate through all sessions and find matching islands
	for _, session := range sessions {
		instances := session.ListIslands()
		for _, instance := range instances {
			if instance.Type == islandType {
				// Create a copy of the event for this island to avoid shared Data slice mutation
				eventCopy := event
				eventCopy.Island = instance.ID
				// Send the event to the session (non-blocking)
				if err := session.Send(eventCopy); err != nil {
					slog.Error("failed to send broadcast event", "island_type", islandType, "session_id", session.ID, "err", err)
				}
			}
		}
	}
}

// BroadcastToIsland sends an event to a specific island ID across all sessions.
// This is useful for broadcasting to all instances with the same ID (e.g., "chat-room-123").
func (e *IslandEngine) BroadcastToIsland(islandID IslandID, event Event) {
	e.mu.RLock()
	sessions := make([]*Session, 0, len(e.sessions))
	for _, session := range e.sessions {
		sessions = append(sessions, session)
	}
	e.mu.RUnlock()

	// Iterate through all sessions and find matching islands
	for _, session := range sessions {
		if instance, ok := session.GetIsland(islandID); ok {
			// Create a copy of the event for this island to avoid shared Data slice mutation
			eventCopy := event
			eventCopy.Island = instance.ID
			// Send the event to the session (non-blocking)
			if err := session.Send(eventCopy); err != nil {
				slog.Error("failed to send broadcast event", "island_id", islandID, "session_id", session.ID, "err", err)
			}
		}
	}
}

// Close shuts down the engine gracefully.
// It:
// - Cancels the engine context
// - Closes all active sessions
// - Clears the sessions map
func (e *IslandEngine) Close() error {
	e.cancel()

	e.mu.Lock()
	defer e.mu.Unlock()

	for _, session := range e.sessions {
		_ = session.Close()
	}
	e.sessions = make(map[SessionID]*Session)

	return nil
}

// SetStateTTL sets the default TTL for island state in the state store.
func (e *IslandEngine) SetStateTTL(ttl time.Duration) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.stateTTL = ttl
}

// Context returns the engine's context.
func (e *IslandEngine) Context() context.Context {
	return e.ctx
}
