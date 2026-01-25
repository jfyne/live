package live

import (
	"context"
	"fmt"
	"sync"
)

// Session represents a transport-agnostic connection that can multiplex
// multiple island instances over a single transport connection.
//
// A Session:
// - Has a unique SessionID
// - Manages multiple IslandInstance objects
// - Routes events from the transport to the correct island
// - Provides thread-safe access to islands
type Session struct {
	// ID is the unique session identifier
	ID SessionID

	// transport is the underlying transport connection
	transport Transport

	// islands maps island IDs to their instances
	islands map[IslandID]*IslandInstance

	// mu protects concurrent access to the islands map
	mu sync.RWMutex

	// ctx is the session context
	ctx context.Context

	// cancel cancels the session context
	cancel context.CancelFunc

	// done signals when the event loop has stopped
	done chan struct{}

	// closeOnce ensures Close() is idempotent
	closeOnce sync.Once
}

// NewSession creates a new session with the given ID and transport.
// The session starts processing events from the transport immediately.
func NewSession(ctx context.Context, sessionID SessionID, transport Transport) *Session {
	sessionCtx, cancel := context.WithCancel(ctx)

	s := &Session{
		ID:        sessionID,
		transport: transport,
		islands:   make(map[IslandID]*IslandInstance),
		ctx:       sessionCtx,
		cancel:    cancel,
		done:      make(chan struct{}),
	}

	// Start the event loop
	go s.eventLoop()

	return s
}

// eventLoop reads events from the transport and routes them to islands.
// It runs until the session is closed or the transport closes.
func (s *Session) eventLoop() {
	defer close(s.done)

	for {
		select {
		case <-s.ctx.Done():
			// Session closed
			return

		case event, ok := <-s.transport.Events():
			if !ok {
				// Transport closed
				return
			}

			// Route event to the appropriate handler
			if err := s.handleEvent(event); err != nil {
				// Log error or send error event back to client
				// For now, we'll continue processing other events
				_ = err
			}
		}
	}
}

// handleEvent routes an event to the correct island or handles session-level events.
func (s *Session) handleEvent(event Event) error {
	// If the event has an island field, route it to that island
	if event.Island != "" {
		return s.routeToIsland(event)
	}

	// Session-level events (no island field) can be handled here
	// For now, these are not used in the v2 architecture
	return nil
}

// routeToIsland routes an event to a specific island instance.
func (s *Session) routeToIsland(event Event) error {
	islandID := IslandID(event.Island)

	// Get the island instance
	instance, ok := s.GetIsland(islandID)
	if !ok {
		return fmt.Errorf("island not found: %s", islandID)
	}

	// Parse event params
	params, err := event.Params()
	if err != nil {
		return err
	}

	// Handle the event based on whether it has self data (server event)
	// or is a client event
	if event.SelfData != nil {
		// Self-targeted event from server
		return instance.CallSelf(s.ctx, event.T, event.SelfData)
	}

	// Client event
	return instance.CallEvent(s.ctx, event.T, params)
}

// AddIsland registers an island instance with this session.
// This makes the island available for event routing.
func (s *Session) AddIsland(instance *IslandInstance) {
	s.mu.Lock()
	defer s.mu.Unlock()

	islandID := IslandID(instance.ID)
	s.islands[islandID] = instance
}

// GetIsland retrieves an island instance by ID.
// Returns the instance and true if found, nil and false otherwise.
func (s *Session) GetIsland(islandID IslandID) (*IslandInstance, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	instance, ok := s.islands[islandID]
	return instance, ok
}

// RemoveIsland unregisters an island instance from this session.
func (s *Session) RemoveIsland(islandID IslandID) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.islands, islandID)
}

// ListIslands returns a slice of all island instances in this session.
// The returned slice is a snapshot at the time of the call.
func (s *Session) ListIslands() []*IslandInstance {
	s.mu.RLock()
	defer s.mu.RUnlock()

	instances := make([]*IslandInstance, 0, len(s.islands))
	for _, instance := range s.islands {
		instances = append(instances, instance)
	}
	return instances
}

// Send transmits an event to the client via the transport.
// This method is thread-safe and wraps the transport's Send method.
func (s *Session) Send(event Event) error {
	return s.transport.Send(event)
}

// Close performs a clean shutdown of the session.
// It:
// - Stops the event loop
// - Closes the underlying transport
// - Cleans up all resources
//
// Close is safe to call multiple times.
func (s *Session) Close() error {
	var err error

	s.closeOnce.Do(func() {
		// Cancel the session context to stop the event loop
		s.cancel()

		// Wait for the event loop to finish
		<-s.done

		// Close the transport
		err = s.transport.Close()

		// Clear the islands map
		s.mu.Lock()
		s.islands = make(map[IslandID]*IslandInstance)
		s.mu.Unlock()
	})

	return err
}

// Context returns the session's context.
// The context is cancelled when the session is closed.
func (s *Session) Context() context.Context {
	return s.ctx
}
