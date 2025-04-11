package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"

	"github.com/jfyne/live"
	"github.com/jfyne/live/examples/chat"
	"gocloud.dev/pubsub"
	_ "gocloud.dev/pubsub/mempubsub"
)

const app = "chat-app"

type CloudTransport struct {
	topic *pubsub.Topic
}

func NewCloudTransport(ctx context.Context) (*CloudTransport, error) {
	topic, err := pubsub.OpenTopic(ctx, "mem://broadcast")
	if err != nil {
		return nil, err
	}
	return &CloudTransport{
		topic: topic,
	}, nil
}

func (c *CloudTransport) Publish(ctx context.Context, topic string, msg live.Event) error {
	data, err := json.Marshal(live.TransportMessage{Topic: topic, Msg: msg})
	if err != nil {
		return fmt.Errorf("could not publish event: %w", err)
	}
	return c.topic.Send(ctx, &pubsub.Message{
		Body: data,
		Metadata: map[string]string{
			"topic": topic,
		},
	})
}

func (c *CloudTransport) Listen(ctx context.Context, p *live.PubSub) error {
	sub, err := pubsub.OpenSubscription(ctx, "mem://broadcast")
	if err != nil {
		return fmt.Errorf("could not open subscription: %w", err)
	}
	for {
		msg, err := sub.Receive(ctx)
		if err != nil {
			log.Println("receive message failed: %w", err)
			break
		}

		var t live.TransportMessage
		if err := json.Unmarshal(msg.Body, &t); err != nil {
			log.Println("malformed message received: %w", err)
			continue
		}
		p.Recieve(t.Topic, t.Msg)
		msg.Ack()
	}
	return fmt.Errorf("stopped receiving messages")
}

func main() {
	// Here we are creating three of the same handler to show
	// how they can all receive the same broadcast messages.
	chat1 := live.NewHttpHandler(context.Background(), chat.NewHandler())
	chat2 := live.NewHttpHandler(context.Background(), chat.NewHandler())
	chat3 := live.NewHttpHandler(context.Background(), chat.NewHandler())

	ctx := context.Background()

	// We use the cloud transport defined above, which is
	// simulating a pub sub type system in memory.
	t, err := NewCloudTransport(ctx)
	if err != nil {
		log.Fatal(err)
	}

	// Create the pubsub instance.
	pubsub := live.NewPubSub(ctx, t)

	// Now subscribe each handler to the same "topic", this
	// will then set them up to receive broadcasted events
	// from each other.
	pubsub.Subscribe(app, chat1)
	pubsub.Subscribe(app, chat2)
	pubsub.Subscribe(app, chat3)

	// Run the server.
	http.Handle("/one", chat1)
	http.Handle("/two", chat2)
	http.Handle("/three", chat3)
	http.Handle("/live.js", live.Javascript{})
	http.Handle("/auto.js.map", live.JavascriptMap{})
	slog.Info("server", "link", "http://localhost:8080")
	http.ListenAndServe(":8080", nil)
}
