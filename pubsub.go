package live

import (
	"context"
	"log"
)

// PubSubTransport is how the messages should be sent to the listeners.
type PubSubTransport interface {
	// Publish a message onto the given topic.
	Publish(ctx context.Context, topic string, msg Event) error
	// Listen will be called in a go routine so should be written to
	// block.
	Listen(ctx context.Context, p *PubSub) error
}

// PubSub handles communication between handlers. Depending on the given
// transport this could be between handlers in an application, or across
// nodes in a cluster.
type PubSub struct {
	transport PubSubTransport
	handlers  map[string][]Engine
}

// NewPubSub creates a new PubSub handler.
func NewPubSub(ctx context.Context, t PubSubTransport) *PubSub {
	p := &PubSub{
		transport: t,
		handlers:  map[string][]Engine{},
	}
	go func(ctx context.Context, ps *PubSub) {
		if err := t.Listen(ctx, ps); err != nil {
			log.Fatal("could not listen on pubsub: %w", err)
		}
	}(ctx, p)
	return p
}

// Publish send a message on a topic.
func (p *PubSub) Publish(ctx context.Context, topic string, msg Event) error {
	return p.transport.Publish(ctx, topic, msg)
}

// Subscribe adds a handler to a PubSub topic.
func (p *PubSub) Subscribe(topic string, h Engine) {
	p.handlers[topic] = append(p.handlers[topic], h)

	// This adjusts the handlers broadcast function to publish onto the
	// given topic.
	h.HandleBroadcast(func(ctx context.Context, h Engine, msg Event) {
		if err := p.transport.Publish(ctx, topic, msg); err != nil {
			log.Println("could not publish broadcast:", err)
		}
	})
}

// Receice a message from the transport.
func (p *PubSub) Recieve(topic string, msg Event) {
	ctx := context.Background()
	for _, node := range p.handlers[topic] {
		node.self(ctx, nil, msg)
	}
}

// TransportMessage a userful container to send live events.
type TransportMessage struct {
	Topic string
	Msg   Event
}

// LocalTransport a pubsub transport that allows handlers to communicate
// locally.
type LocalTransport struct {
	ctx   context.Context
	queue chan TransportMessage
}

// NewLocalTransport create a new LocalTransport.
func NewLocalTransport() *LocalTransport {
	return &LocalTransport{
		queue: make(chan TransportMessage),
	}
}

// Publish send a message to all handlers subscribed to a topic.
func (l *LocalTransport) Publish(ctx context.Context, topic string, msg Event) error {
	l.queue <- TransportMessage{Topic: topic, Msg: msg}
	return nil
}

// Listen listen for new published messages.
func (l *LocalTransport) Listen(ctx context.Context, p *PubSub) error {
	for {
		select {
		case msg := <-l.queue:
			p.Recieve(msg.Topic, msg.Msg)
		case <-ctx.Done():
			return nil
		}
	}
}
