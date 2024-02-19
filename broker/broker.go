package broker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/tomasmota/go-grpc-broker/consumer"
	pb "github.com/tomasmota/go-grpc-broker/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type Broker struct {
	listener   net.Listener
	grpcServer *grpc.Server

	pb.UnimplementedBrokerServer

	mu        sync.RWMutex
	consumers map[string]consumer.Consumer
}

func New(port string) *Broker {
	broker := &Broker{
		consumers: make(map[string]consumer.Consumer),
	}

	listener, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		slog.Error("Failed to create listener: %v", err)
		os.Exit(1)
	}
	broker.listener = listener
	broker.grpcServer = grpc.NewServer()
	reflection.Register(broker.grpcServer)
	return broker
}

func (b *Broker) Start() {
	pb.RegisterBrokerServer(b.grpcServer, b)

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		slog.Info("Shutting down server...")

		b.grpcServer.GracefulStop()
	}()

	slog.Info("Starting server")
	err := b.grpcServer.Serve(b.listener)
	if err != nil {
		slog.Error("Grpc server error: %v", err)
		os.Exit(1)
	}
}

func (b *Broker) Publish(ctx context.Context, pr *pb.PublishRequest) (*pb.Ack, error) {
	if pr.Producer == nil {
		return nil, errors.New("producer not set")
	}
	slog.Info("Publishing", "contents", pr.Data)

	b.mu.RLock()
	defer b.mu.RUnlock()
	for name, c := range b.consumers {
		slog.Info("sending stuff to consumer", "name", name)
		err := c.Stream.Send(&pb.Message{
			Data: pr.Data,
		})
		if err != nil {
			slog.Error("Error sending message to consumer", "name", name, "error", err)
		}
	}

	return &pb.Ack{}, nil
}

func (b *Broker) Subscribe(sr *pb.SubscribeRequest, ss pb.Broker_SubscribeServer) error {
	if sr.Consumer == nil {
		return errors.New("consumer not set")
	}

	b.mu.Lock()
	_, exists := b.consumers[sr.Consumer.Name]
	if exists {
		b.mu.Unlock()
		return fmt.Errorf("consumer with the same name already exists. name=%s", sr.Consumer.Name) // somehow communicate a warning instead
	}

	finCh := make(chan bool)
	b.consumers[sr.Consumer.Name] = consumer.Consumer{
		Start:    time.Now(),
		Stream:   ss,
		Finished: finCh,
	}
	b.mu.Unlock()

	slog.Info("New consumer added", "name", sr.Consumer.Name)

	go func() {
		select {
		case <-finCh:
			slog.Info("Finishing up consumer", "name", sr.Consumer.Name)
		case <-ss.Context().Done():
			slog.Info("Subscriber has disconnected", "name", sr.Consumer.Name)
		}

		b.mu.Lock()
		defer b.mu.Unlock()
		slog.Info("Removing consumer", "name", sr.Consumer.Name)
		delete(b.consumers, sr.Consumer.Name)
	}()

	return nil
}
