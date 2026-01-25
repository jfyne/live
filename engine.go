package live

import (
	"context"
	"encoding/json"
	"fmt"
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
// The engine is thread-safe and uses channel-based operations for session management.
type IslandEngine struct {
	// registry holds the island type definitions
	registry *IslandRegistry

	// stateStore persists island state across reconnections
	stateStore IslandStateStore

	// sessions maps session IDs to active sessions
	sessions map[SessionID]*Session

	// mu protects concurrent access to the sessions map
	mu sync.RWMutex

	// ctx is the engine context for lifecycle management
	ctx context.Context

	// cancel cancels the engine context
	cancel context.CancelFunc

	// sessionOps channels for thread-safe session operations
	addSessionCh    chan *Session
	deleteSessionCh chan SessionID

	// done signals when the engine has shut down
	done chan struct{}

	// stateTTL is the default TTL for island state
	stateTTL time.Duration
}

// NewIslandEngine creates a new island engine with the given registry and state store.
// The engine starts a session management goroutine that processes add/remove operations.
func NewIslandEngine(ctx context.Context, registry *IslandRegistry, stateStore IslandStateStore) *IslandEngine {
	engineCtx, cancel := context.WithCancel(ctx)

	e := &IslandEngine{
		registry:        registry,
		stateStore:      stateStore,
		sessions:        make(map[SessionID]*Session),
		ctx:             engineCtx,
		cancel:          cancel,
		addSessionCh:    make(chan *Session, 16),
		deleteSessionCh: make(chan SessionID, 16),
		done:            make(chan struct{}),
		stateTTL:        24 * time.Hour, // Default 24 hour TTL
	}

	// Start the session management goroutine
	go e.sessionManager()

	return e
}

// sessionManager is the main goroutine that handles session operations.
// It processes add/delete operations via channels to ensure thread safety.
func (e *IslandEngine) sessionManager() {
	defer close(e.done)

	for {
		select {
		case <-e.ctx.Done():
			// Engine shutdown - close all sessions
			e.mu.Lock()
			for _, session := range e.sessions {
				_ = session.Close()
			}
			e.sessions = make(map[SessionID]*Session)
			e.mu.Unlock()
			return

		case session := <-e.addSessionCh:
			e.mu.Lock()
			e.sessions[session.ID] = session
			e.mu.Unlock()

		case sessionID := <-e.deleteSessionCh:
			e.mu.Lock()
			if session, ok := e.sessions[sessionID]; ok {
				_ = session.Close()
				delete(e.sessions, sessionID)
				// Clean up all state for this session
				e.stateStore.DeleteSession(sessionID)
			}
			e.mu.Unlock()
		}
	}
}

// AddSession registers a new session with the engine.
// The session will be available for island mounting and event routing.
func (e *IslandEngine) AddSession(session *Session) {
	select {
	case e.addSessionCh <- session:
	case <-e.ctx.Done():
		// Engine is shutting down
	}
}

// GetSession retrieves a session by ID.
// Returns the session and true if found, nil and false otherwise.
func (e *IslandEngine) GetSession(sessionID SessionID) (*Session, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	session, ok := e.sessions[sessionID]
	return session, ok
}

// DeleteSession removes a session from the engine and cleans up all its resources.
// This includes:
// - Closing the session
// - Removing all islands from the session
// - Deleting all state from the state store
func (e *IslandEngine) DeleteSession(sessionID SessionID) {
	select {
	case e.deleteSessionCh <- sessionID:
	case <-e.ctx.Done():
		// Engine is shutting down
	}
}

// MountIsland creates and mounts a new island instance within a session.
// The flow is:
// 1. Get the session
// 2. Create the island instance from the registry
// 3. Call the island's Mount() handler
// 4. Add the instance to the session
// 5. Save the initial state to the state store
// 6. Render the island and send the initial patch to the client
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

	// Mount the island (calls the island's mount handler)
	if err := instance.Mount(session.Context()); err != nil {
		return nil, fmt.Errorf("failed to mount island: %w", err)
	}

	// Add the instance to the session
	session.AddIsland(instance)

	// Save the initial state to the state store
	e.stateStore.Set(sessionID, islandID, instance.State(), e.stateTTL)

	// Render the island and send the initial patch to the client
	if err := e.renderAndSendIsland(session, instance); err != nil {
		// Non-fatal - the island is mounted but the client won't see the initial render
		_ = err
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
	e.stateStore.Set(sessionID, islandID, instance.State(), e.stateTTL)

	// Re-render the island and send the patch to the client
	if err := e.renderAndSendIsland(session, instance); err != nil {
		return fmt.Errorf("failed to render island: %w", err)
	}

	return nil
}

// renderAndSendIsland renders an island instance and sends a patch event to the client.
// This is used both for initial mount and after state changes.
func (e *IslandEngine) renderAndSendIsland(session *Session, instance *IslandInstance) error {
	// Render the island
	html, err := RenderIsland(session.Context(), instance)
	if err != nil {
		return fmt.Errorf("render error: %w", err)
	}

	// Create a patch event with the rendered HTML
	// The client will use the island ID to apply the patch to the correct element
	patchData := map[string]any{
		"island": instance.ID,
		"html":   html,
	}

	data, err := json.Marshal(patchData)
	if err != nil {
		return fmt.Errorf("failed to marshal patch data: %w", err)
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
				// Set the island field on the event
				event.Island = instance.ID
				// Send the event to the session (non-blocking)
				_ = session.Send(event)
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
			// Set the island field on the event
			event.Island = instance.ID
			// Send the event to the session (non-blocking)
			_ = session.Send(event)
		}
	}
}

// Close shuts down the engine gracefully.
// It:
// - Cancels the engine context
// - Waits for the session manager to finish
// - Closes all active sessions
func (e *IslandEngine) Close() error {
	e.cancel()
	<-e.done
	return nil
}

// SetStateTTL sets the default TTL for island state in the state store.
func (e *IslandEngine) SetStateTTL(ttl time.Duration) {
	e.stateTTL = ttl
}

// Context returns the engine's context.
func (e *IslandEngine) Context() context.Context {
	return e.ctx
}
