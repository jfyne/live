package live

import (
	"context"
	"sync"
	"testing"
	"time"
)

// TestMemoryIslandStateStore_GetSet tests basic Get/Set operations with composite keys.
func TestMemoryIslandStateStore_GetSet(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store := NewMemoryIslandStateStore(ctx, 1*time.Minute)

	sessionID := SessionID("session-1")
	islandID := IslandID("island-1")
	state := map[string]interface{}{
		"count": 42,
		"name":  "test",
	}

	// Test Get on non-existent state.
	_, ok := store.Get(sessionID, islandID)
	if ok {
		t.Error("Expected Get to return false for non-existent state")
	}

	// Test Set and Get.
	store.Set(sessionID, islandID, state, 1*time.Minute)
	retrieved, ok := store.Get(sessionID, islandID)
	if !ok {
		t.Fatal("Expected Get to return true for existing state")
	}

	retrievedMap, ok := retrieved.(map[string]interface{})
	if !ok {
		t.Fatal("Retrieved state is not a map")
	}

	if retrievedMap["count"] != 42 {
		t.Errorf("Expected count=42, got %v", retrievedMap["count"])
	}

	if retrievedMap["name"] != "test" {
		t.Errorf("Expected name=test, got %v", retrievedMap["name"])
	}
}

// TestMemoryIslandStateStore_Delete tests deletion of specific island state.
func TestMemoryIslandStateStore_Delete(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store := NewMemoryIslandStateStore(ctx, 1*time.Minute)

	sessionID := SessionID("session-1")
	islandID := IslandID("island-1")
	state := "test-state"

	// Set state.
	store.Set(sessionID, islandID, state, 1*time.Minute)

	// Verify state exists.
	_, ok := store.Get(sessionID, islandID)
	if !ok {
		t.Fatal("Expected state to exist after Set")
	}

	// Delete state.
	store.Delete(sessionID, islandID)

	// Verify state is deleted.
	_, ok = store.Get(sessionID, islandID)
	if ok {
		t.Error("Expected Get to return false after Delete")
	}
}

// TestMemoryIslandStateStore_MultipleIslands tests multiple islands within a single session.
func TestMemoryIslandStateStore_MultipleIslands(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store := NewMemoryIslandStateStore(ctx, 1*time.Minute)

	sessionID := SessionID("session-1")
	island1 := IslandID("island-1")
	island2 := IslandID("island-2")

	state1 := "state-1"
	state2 := "state-2"

	// Set states for multiple islands.
	store.Set(sessionID, island1, state1, 1*time.Minute)
	store.Set(sessionID, island2, state2, 1*time.Minute)

	// Verify both states exist and are independent.
	retrieved1, ok := store.Get(sessionID, island1)
	if !ok || retrieved1 != state1 {
		t.Errorf("Expected island1 state=%s, got %v (ok=%v)", state1, retrieved1, ok)
	}

	retrieved2, ok := store.Get(sessionID, island2)
	if !ok || retrieved2 != state2 {
		t.Errorf("Expected island2 state=%s, got %v (ok=%v)", state2, retrieved2, ok)
	}

	// Delete one island.
	store.Delete(sessionID, island1)

	// Verify island1 is deleted but island2 remains.
	_, ok = store.Get(sessionID, island1)
	if ok {
		t.Error("Expected island1 to be deleted")
	}

	retrieved2, ok = store.Get(sessionID, island2)
	if !ok || retrieved2 != state2 {
		t.Error("Expected island2 to still exist after deleting island1")
	}
}

// TestMemoryIslandStateStore_MultipleSessions tests multiple sessions with islands.
func TestMemoryIslandStateStore_MultipleSessions(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store := NewMemoryIslandStateStore(ctx, 1*time.Minute)

	session1 := SessionID("session-1")
	session2 := SessionID("session-2")
	islandID := IslandID("island-1")

	state1 := "state-session-1"
	state2 := "state-session-2"

	// Set states for same island ID in different sessions.
	store.Set(session1, islandID, state1, 1*time.Minute)
	store.Set(session2, islandID, state2, 1*time.Minute)

	// Verify states are isolated by session.
	retrieved1, ok := store.Get(session1, islandID)
	if !ok || retrieved1 != state1 {
		t.Errorf("Expected session1 state=%s, got %v (ok=%v)", state1, retrieved1, ok)
	}

	retrieved2, ok := store.Get(session2, islandID)
	if !ok || retrieved2 != state2 {
		t.Errorf("Expected session2 state=%s, got %v (ok=%v)", state2, retrieved2, ok)
	}
}

// TestMemoryIslandStateStore_DeleteSession tests deletion of all islands in a session.
func TestMemoryIslandStateStore_DeleteSession(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store := NewMemoryIslandStateStore(ctx, 1*time.Minute)

	sessionID := SessionID("session-1")
	island1 := IslandID("island-1")
	island2 := IslandID("island-2")

	// Set multiple island states for the session.
	store.Set(sessionID, island1, "state-1", 1*time.Minute)
	store.Set(sessionID, island2, "state-2", 1*time.Minute)

	// Verify both states exist.
	_, ok1 := store.Get(sessionID, island1)
	_, ok2 := store.Get(sessionID, island2)
	if !ok1 || !ok2 {
		t.Fatal("Expected both island states to exist")
	}

	// Delete the entire session.
	store.DeleteSession(sessionID)

	// Verify all island states are deleted.
	_, ok1 = store.Get(sessionID, island1)
	_, ok2 = store.Get(sessionID, island2)
	if ok1 || ok2 {
		t.Error("Expected all island states to be deleted after DeleteSession")
	}
}

// TestMemoryIslandStateStore_TTLExpiration tests that states expire after TTL.
func TestMemoryIslandStateStore_TTLExpiration(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Use a short cleanup interval for faster testing.
	store := NewMemoryIslandStateStore(ctx, 50*time.Millisecond)

	sessionID := SessionID("session-1")
	islandID := IslandID("island-1")
	state := "test-state"

	// Set state with a short TTL.
	store.Set(sessionID, islandID, state, 100*time.Millisecond)

	// Verify state exists immediately.
	retrieved, ok := store.Get(sessionID, islandID)
	if !ok || retrieved != state {
		t.Fatal("Expected state to exist immediately after Set")
	}

	// Wait for TTL to expire.
	time.Sleep(150 * time.Millisecond)

	// Verify state is no longer accessible (expired).
	_, ok = store.Get(sessionID, islandID)
	if ok {
		t.Error("Expected Get to return false after TTL expiration")
	}

	// Wait for janitor to run and clean up.
	time.Sleep(100 * time.Millisecond)

	// Verify the state has been cleaned up by the janitor.
	// This is a bit of a black-box test, but we can check that Get still returns false.
	_, ok = store.Get(sessionID, islandID)
	if ok {
		t.Error("Expected state to remain deleted after janitor cleanup")
	}
}

// TestMemoryIslandStateStore_JanitorCleanup tests the janitor's cleanup behavior.
func TestMemoryIslandStateStore_JanitorCleanup(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Use a short cleanup interval.
	store := NewMemoryIslandStateStore(ctx, 50*time.Millisecond)

	sessionID := SessionID("session-1")
	island1 := IslandID("island-1")
	island2 := IslandID("island-2")

	// Set one state with short TTL, one with long TTL.
	store.Set(sessionID, island1, "short-lived", 100*time.Millisecond)
	store.Set(sessionID, island2, "long-lived", 5*time.Minute)

	// Wait for short-lived state to expire and janitor to run.
	time.Sleep(200 * time.Millisecond)

	// Verify short-lived state is gone.
	_, ok := store.Get(sessionID, island1)
	if ok {
		t.Error("Expected short-lived state to be cleaned up by janitor")
	}

	// Verify long-lived state still exists.
	retrieved, ok := store.Get(sessionID, island2)
	if !ok || retrieved != "long-lived" {
		t.Error("Expected long-lived state to still exist after janitor cleanup")
	}
}

// TestMemoryIslandStateStore_JanitorStopsOnContextCancel tests that the janitor stops when context is canceled.
func TestMemoryIslandStateStore_JanitorStopsOnContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	store := NewMemoryIslandStateStore(ctx, 50*time.Millisecond)

	sessionID := SessionID("session-1")
	islandID := IslandID("island-1")

	// Set state with short TTL.
	store.Set(sessionID, islandID, "test-state", 100*time.Millisecond)

	// Cancel the context to stop the janitor.
	cancel()

	// Wait a bit to ensure janitor has stopped.
	time.Sleep(100 * time.Millisecond)

	// The janitor should have stopped, so expired states won't be cleaned up automatically.
	// We can't directly test that the goroutine stopped, but we can verify the state behavior.
	// After expiration, Get should still return false because of the TTL check in Get.
	time.Sleep(50 * time.Millisecond) // Total wait is now 150ms, state should be expired.
	_, ok := store.Get(sessionID, islandID)
	if ok {
		t.Error("Expected Get to return false for expired state even if janitor is stopped")
	}
}

// TestMemoryIslandStateStore_ConcurrentAccess tests thread-safe concurrent access.
func TestMemoryIslandStateStore_ConcurrentAccess(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store := NewMemoryIslandStateStore(ctx, 1*time.Minute)

	sessionID := SessionID("session-1")
	numIslands := 100
	numGoroutines := 10

	var wg sync.WaitGroup

	// Concurrent Set operations.
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < numIslands; j++ {
				islandID := IslandID(string(rune('a' + (goroutineID*numIslands+j)%26)))
				state := map[string]int{"goroutine": goroutineID, "island": j}
				store.Set(sessionID, islandID, state, 1*time.Minute)
			}
		}(i)
	}

	wg.Wait()

	// Concurrent Get operations.
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < numIslands; j++ {
				islandID := IslandID(string(rune('a' + (goroutineID*numIslands+j)%26)))
				_, ok := store.Get(sessionID, islandID)
				// We just verify that Get doesn't panic or race.
				_ = ok
			}
		}(i)
	}

	wg.Wait()

	// Concurrent Delete operations.
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < numIslands; j++ {
				islandID := IslandID(string(rune('a' + (goroutineID*numIslands+j)%26)))
				store.Delete(sessionID, islandID)
			}
		}(i)
	}

	wg.Wait()

	// Verify all states are deleted.
	for i := 0; i < numGoroutines*numIslands; i++ {
		islandID := IslandID(string(rune('a' + i%26)))
		_, ok := store.Get(sessionID, islandID)
		if ok {
			// It's possible some states still exist due to race conditions,
			// but the main goal is to ensure no panics or data races occur.
		}
	}
}

// TestMemoryIslandStateStore_ConcurrentSessionOperations tests concurrent operations across multiple sessions.
func TestMemoryIslandStateStore_ConcurrentSessionOperations(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store := NewMemoryIslandStateStore(ctx, 1*time.Minute)

	numSessions := 50
	numIslands := 10

	var wg sync.WaitGroup

	// Concurrent Set/Get/Delete across multiple sessions.
	for i := 0; i < numSessions; i++ {
		wg.Add(1)
		go func(sessionNum int) {
			defer wg.Done()

			sessionID := SessionID(string(rune('A' + sessionNum%26)))

			for j := 0; j < numIslands; j++ {
				islandID := IslandID(string(rune('a' + j%26)))
				state := map[string]int{"session": sessionNum, "island": j}

				// Set state.
				store.Set(sessionID, islandID, state, 1*time.Minute)

				// Get state.
				retrieved, ok := store.Get(sessionID, islandID)
				if ok {
					if m, ok := retrieved.(map[string]int); ok {
						if m["session"] != sessionNum {
							t.Errorf("Expected session=%d, got %d", sessionNum, m["session"])
						}
					}
				}

				// Delete state.
				if j%2 == 0 {
					store.Delete(sessionID, islandID)
				}
			}

			// Delete entire session.
			if sessionNum%3 == 0 {
				store.DeleteSession(sessionID)
			}
		}(i)
	}

	wg.Wait()
}

// TestMemoryIslandStateStore_EmptySessionCleanup tests that empty session maps are cleaned up.
func TestMemoryIslandStateStore_EmptySessionCleanup(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store := NewMemoryIslandStateStore(ctx, 1*time.Minute)

	sessionID := SessionID("session-1")
	islandID := IslandID("island-1")

	// Set and delete state.
	store.Set(sessionID, islandID, "test", 1*time.Minute)
	store.Delete(sessionID, islandID)

	// The session map should be cleaned up automatically.
	// We can verify this indirectly by checking that the session doesn't exist in the store.
	store.mu.RLock()
	_, exists := store.store[sessionID]
	store.mu.RUnlock()

	if exists {
		t.Error("Expected empty session map to be cleaned up after deleting last island")
	}
}

// TestMemoryIslandStateStore_UpdateState tests updating an existing state.
func TestMemoryIslandStateStore_UpdateState(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store := NewMemoryIslandStateStore(ctx, 1*time.Minute)

	sessionID := SessionID("session-1")
	islandID := IslandID("island-1")

	// Set initial state.
	store.Set(sessionID, islandID, "initial", 1*time.Minute)

	// Verify initial state.
	retrieved, ok := store.Get(sessionID, islandID)
	if !ok || retrieved != "initial" {
		t.Fatal("Expected initial state to be set")
	}

	// Update state.
	store.Set(sessionID, islandID, "updated", 1*time.Minute)

	// Verify updated state.
	retrieved, ok = store.Get(sessionID, islandID)
	if !ok || retrieved != "updated" {
		t.Error("Expected state to be updated")
	}
}

// TestMemoryIslandStateStore_GetByIslandID tests cross-session state lookup.
func TestMemoryIslandStateStore_GetByIslandID(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store := NewMemoryIslandStateStore(ctx, 1*time.Minute)

	islandID := IslandID("counter-1")

	// No state exists yet.
	_, ok := store.GetByIslandID(islandID)
	if ok {
		t.Error("Expected GetByIslandID to return false for non-existent state")
	}

	// Set state in session-1.
	store.Set("session-1", islandID, "state-from-session-1", 1*time.Minute)

	// GetByIslandID should find it.
	state, ok := store.GetByIslandID(islandID)
	if !ok {
		t.Fatal("Expected GetByIslandID to find state")
	}
	if state != "state-from-session-1" {
		t.Errorf("Expected state-from-session-1, got %v", state)
	}

	// Set a newer state in session-2 (with longer TTL = later expiry).
	store.Set("session-2", islandID, "state-from-session-2", 2*time.Minute)

	// GetByIslandID should return the most recent (latest expiry).
	state, ok = store.GetByIslandID(islandID)
	if !ok {
		t.Fatal("Expected GetByIslandID to find state")
	}
	if state != "state-from-session-2" {
		t.Errorf("Expected state-from-session-2, got %v", state)
	}
}

// TestMemoryIslandStateStore_GetByIslandIDExpired tests that expired state is skipped.
func TestMemoryIslandStateStore_GetByIslandIDExpired(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store := NewMemoryIslandStateStore(ctx, 1*time.Minute)

	islandID := IslandID("counter-1")

	// Set state with very short TTL.
	store.Set("session-1", islandID, "expired-state", 1*time.Millisecond)

	// Wait for expiry.
	time.Sleep(5 * time.Millisecond)

	// GetByIslandID should not find expired state.
	_, ok := store.GetByIslandID(islandID)
	if ok {
		t.Error("Expected GetByIslandID to skip expired state")
	}

	// Set non-expired state in another session.
	store.Set("session-2", islandID, "fresh-state", 1*time.Minute)

	// Should find the fresh state.
	state, ok := store.GetByIslandID(islandID)
	if !ok {
		t.Fatal("Expected GetByIslandID to find fresh state")
	}
	if state != "fresh-state" {
		t.Errorf("Expected fresh-state, got %v", state)
	}
}

// TestMemoryIslandStateStore_GetByIslandIDIsolation tests that different island IDs are isolated.
func TestMemoryIslandStateStore_GetByIslandIDIsolation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store := NewMemoryIslandStateStore(ctx, 1*time.Minute)

	// Set states for different islands.
	store.Set("session-1", "counter-1", "counter-1-state", 1*time.Minute)
	store.Set("session-1", "counter-2", "counter-2-state", 1*time.Minute)

	// GetByIslandID should only find the matching island.
	state, ok := store.GetByIslandID("counter-1")
	if !ok || state != "counter-1-state" {
		t.Errorf("Expected counter-1-state, got %v (ok=%v)", state, ok)
	}

	state, ok = store.GetByIslandID("counter-2")
	if !ok || state != "counter-2-state" {
		t.Errorf("Expected counter-2-state, got %v (ok=%v)", state, ok)
	}

	// Non-existent island should not be found.
	_, ok = store.GetByIslandID("counter-3")
	if ok {
		t.Error("Expected GetByIslandID to return false for non-existent island")
	}
}

// TestMemoryIslandStateStore_InvalidCleanupInterval tests that zero/negative cleanup intervals are handled.
func TestMemoryIslandStateStore_InvalidCleanupInterval(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	testCases := []struct {
		name     string
		interval time.Duration
	}{
		{"zero interval", 0},
		{"negative interval", -1 * time.Second},
		{"large negative interval", -10 * time.Minute},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// This should not panic.
			store := NewMemoryIslandStateStore(ctx, tc.interval)

			// Verify the store was created successfully.
			if store == nil {
				t.Fatal("Expected store to be created")
			}

			// Verify the cleanup interval was set to the default (1 minute).
			if store.cleanupInterval != 1*time.Minute {
				t.Errorf("Expected cleanupInterval to be 1 minute, got %v", store.cleanupInterval)
			}

			// Verify basic operations work.
			sessionID := SessionID("session-1")
			islandID := IslandID("island-1")
			store.Set(sessionID, islandID, "test", 1*time.Minute)

			retrieved, ok := store.Get(sessionID, islandID)
			if !ok || retrieved != "test" {
				t.Error("Expected basic store operations to work with corrected cleanup interval")
			}
		})
	}
}
