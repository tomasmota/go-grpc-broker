package broker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	pb "github.com/tomasmota/go-grpc-broker/proto"
)

type Consumer struct {
	Start time.Time

	Stream pb.Broker_SubscribeServer
}

type Broker struct {
	mu        sync.RWMutex
	consumers map[string]Consumer

	pb.UnimplementedBrokerServer
	finished chan struct{}
}

func New() *Broker {
	broker := &Broker{
		consumers: make(map[string]Consumer),
		finished: make(chan struct{}),
	}

	return broker
}

func (b* Broker) Shutdown() {
	close(b.finished)
}

func (b *Broker) broadcast(ctx context.Context, msg []byte) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for name, c := range b.consumers {
		slog.Info("Sending stuff to consumer", "name", name)
		err := c.Stream.Send(&pb.Message{
			Data: msg,
		})
		if err != nil {
			slog.Error("error sending message to consumer", "name", name, "error", err)
		}
	}
}

func (b *Broker) Publish(ctx context.Context, pr *pb.PublishRequest) (*pb.Ack, error) {
	if pr.Producer == nil {
		return nil, errors.New("producer not set")
	}
	slog.Info("Publishing", "contents", pr.Data)
	go b.broadcast(ctx, pr.Data)

	return &pb.Ack{}, nil
}

func (b *Broker) Subscribe(sr *pb.SubscribeRequest, ss pb.Broker_SubscribeServer) error {
	if sr.Consumer == nil {
		return errors.New("consumer not set")
	}

	slog.Info("New subscriber (before lock)", "name", sr.Consumer.Name)
	b.mu.Lock()
	slog.Info("New subscriber (after lock)", "name", sr.Consumer.Name)
	_, exists := b.consumers[sr.Consumer.Name]
	if exists {
		b.mu.Unlock()
		return fmt.Errorf("consumer with the same name already exists. name=%s", sr.Consumer.Name) // somehow communicate a warning instead
	}

	b.consumers[sr.Consumer.Name] = Consumer{
		Start:  time.Now(),
		Stream: ss,
	}
	b.mu.Unlock()

	slog.Info("New consumer added", "name", sr.Consumer.Name)

	// block until the client disconnects
	select {
	case <-ss.Context().Done():
		slog.Info("Subscriber has disconnected", "name", sr.Consumer.Name)
	case <-b.finished:
		slog.Info("Server shutting down, disconnecting consumer", "name", sr.Consumer.Name)
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	slog.Info("Removing consumer", "name", sr.Consumer.Name)
	delete(b.consumers, sr.Consumer.Name)

	return nil
}
