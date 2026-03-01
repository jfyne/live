package live

// IslandID is a unique identifier for an island instance within a session.
// Format: user-defined string ID (e.g., "counter-1", "chat-sidebar")
type IslandID string

// SessionID is a unique identifier for a client session.
// A session maintains state across transport connections (WebSocket, SSE, etc.)
// and can host multiple island instances.
type SessionID string
