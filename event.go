package live

// Event is an action that happens.
type Event string

// EventHandler a function to handle events.
type EventHandler func(*Socket, SocketMessage) error

// Live events.
const (
	EventError = "err"
	EventPing  = "ping"
	EventPatch = "patch"
)
