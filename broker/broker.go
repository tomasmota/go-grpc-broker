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
	consumerMu sync.RWMutex
	consumers  map[string]Consumer

	topicMu sync.RWMutex
	topics  map[string][]string // topic -> consumer name

	pb.UnimplementedBrokerServer
	finished chan struct{}
}

func New() *Broker {
	broker := &Broker{
		consumers: make(map[string]Consumer),
		topics:    make(map[string][]string),
		finished:  make(chan struct{}),
	}

	return broker
}

func (b *Broker) Shutdown() {
	close(b.finished)
}

func (b *Broker) Publish(ctx context.Context, pr *pb.PublishRequest) (*pb.Ack, error) {
	if pr.Producer == nil {
		return nil, errors.New("producer not set")
	}
	go b.broadcast(ctx, pr.Topic, pr.Data)

	return &pb.Ack{}, nil
}

func (b *Broker) broadcast(ctx context.Context, topic string, msg []byte) {
	slog.Info("Broadcasting message", "topic", topic)

	b.topicMu.RLock()
	defer b.topicMu.RUnlock()
	consumers := b.topics[topic]

	for _, c := range consumers {
		slog.Info("Sending message to consumer", "name", c)
		b.consumerMu.RLock()
		defer b.consumerMu.RUnlock()
		err := b.consumers[c].Stream.Send(&pb.Message{
			Data: msg,
		})
		if err != nil {
			slog.Error("error sending message to consumer", "name", c, "error", err)
		}
	}
}

func (b *Broker) Subscribe(sr *pb.SubscribeRequest, ss pb.Broker_SubscribeServer) error {
	if sr.Consumer == nil {
		return errors.New("consumer not set")
	}

	b.consumerMu.Lock()
	_, exists := b.consumers[sr.Consumer.Name]
	if !exists {
		b.consumers[sr.Consumer.Name] = Consumer{
			Start:  time.Now(),
			Stream: ss,
		}
	}
	b.consumerMu.Unlock()

	b.registerTopic(sr.Topic, sr.Consumer.Name)

	slog.Info("New consumer added", "name", sr.Consumer.Name)

	// block until the client disconnects
	select {
	case <-ss.Context().Done():
		slog.Info("Subscriber has disconnected", "name", sr.Consumer.Name)
	case <-b.finished:
		slog.Info("Server shutting down, disconnecting consumer", "name", sr.Consumer.Name)
	}

	b.removeConsumer(sr.Consumer.Name)

	return nil
}

func (b *Broker) removeConsumer(consumer string) {
	slog.Info("Removing consumer", "name", consumer)
	b.consumerMu.Lock()
	delete(b.consumers, consumer)
	b.consumerMu.Unlock()

	b.topicMu.Lock()
	delete(b.topics, consumer)
	b.topicMu.Unlock()
}


func (b *Broker) registerTopic(topic string, consumer string) {
	b.topicMu.Lock()
	defer b.topicMu.Unlock()
	fmt.Println("adding consumer to topic", topic, consumer)
	b.topics[topic] = append(b.topics[topic], consumer)
}
