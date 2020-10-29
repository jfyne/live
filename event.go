package live

// ET is an action that happens.
type ET string

// EventHandler a function to handle events.
type EventHandler func(*Socket, map[string]interface{}) error

// Live events.
const (
	ETError = "err"
	ETPing  = "ping"
	ETPatch = "patch"
)

// Event messages that are sent and received by the
// socket.
type Event struct {
	T    ET          `json:"t"`
	Data interface{} `json:"d"`
}

// Params extract params from inbound message.
func (e Event) Params() (map[string]interface{}, error) {
	if e.Data == nil {
		return map[string]interface{}{}, nil
	}
	p, ok := e.Data.(map[string]interface{})
	if !ok {
		return nil, ErrMessageMalformed
	}
	return p, nil
}
