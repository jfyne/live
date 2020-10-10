package live

// Event is an action that happens.
type Event string

// EventHandler a function to handle events.
type EventHandler func(Event, *Socket)

// Live events.
const (
	EventListen = "listen"
)
