package live

import (
	"context"
	"sync"
)

// BroadcastTransport is the interface for broadcast message delivery.
// Implementations deliver messages from a publisher to all subscribers.
type BroadcastTransport interface {
	Publish(ctx context.Context, topic string, msg Event) error
	Listen(ctx context.Context, b *Broadcast) error
}

// BroadcastMessage wraps a topic and event for transport delivery.
type BroadcastMessage struct {
	Topic string
	Msg   Event
}

// subscription links an island type to an engine for a topic.
type subscription struct {
	islandType string
	engine     *IslandEngine
}

// Broadcast orchestrates pub/sub message delivery to island engines.
// It manages topic subscriptions and routes incoming messages to the
// correct engines via BroadcastSelfToIslandType.
type Broadcast struct {
	transport BroadcastTransport
	mu        sync.RWMutex
	handlers  map[string][]subscription
}

// NewBroadcast creates a new Broadcast and starts the transport listener
// in a background goroutine.
func NewBroadcast(ctx context.Context, transport BroadcastTransport) *Broadcast {
	b := &Broadcast{
		transport: transport,
		handlers:  make(map[string][]subscription),
	}
	go transport.Listen(ctx, b)
	return b
}

// Subscribe registers an engine to receive messages on a topic for a specific
// island type. When a message arrives on the topic, BroadcastSelfToIslandType
// is called on the engine for the given islandType.
func (b *Broadcast) Subscribe(topic string, islandType string, engine *IslandEngine) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[topic] = append(b.handlers[topic], subscription{
		islandType: islandType,
		engine:     engine,
	})
}

// Publish sends a message to the transport for delivery to all subscribers.
func (b *Broadcast) Publish(ctx context.Context, topic string, msg Event) error {
	return b.transport.Publish(ctx, topic, msg)
}

// Receive is called by the transport when a message arrives.
// It routes the event to all subscribed engines as a self-event via
// BroadcastSelfToIslandType.
func (b *Broadcast) Receive(topic string, msg Event) {
	b.mu.RLock()
	subs := b.handlers[topic]
	b.mu.RUnlock()

	for _, sub := range subs {
		sub.engine.BroadcastSelfToIslandType(sub.islandType, msg)
	}
}

// LocalTransport is an in-memory broadcast transport using an unbuffered channel.
// Publish blocks until Listen receives the message, ensuring delivery ordering.
type LocalTransport struct {
	queue chan BroadcastMessage
}

// NewLocalTransport creates a new LocalTransport with an unbuffered channel.
func NewLocalTransport() *LocalTransport {
	return &LocalTransport{
		queue: make(chan BroadcastMessage), // unbuffered: Publish blocks until Listen receives
	}
}

// Publish sends a message to the local channel.
// It blocks until the Listen goroutine receives the message or the context is cancelled.
func (l *LocalTransport) Publish(ctx context.Context, topic string, msg Event) error {
	select {
	case l.queue <- BroadcastMessage{Topic: topic, Msg: msg}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Listen reads messages from the channel and calls b.Receive for each one.
// It runs until the channel is closed or the context is cancelled.
func (l *LocalTransport) Listen(ctx context.Context, b *Broadcast) error {
	for {
		select {
		case msg, ok := <-l.queue:
			if !ok {
				return nil
			}
			b.Receive(msg.Topic, msg.Msg)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
