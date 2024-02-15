package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	pb "github.com/tomasmota/go-grpc-broker/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/types/known/emptypb"
)

type Consumer struct {
	start time.Time

	stream   pb.Broker_SubscribeServer
	finished chan<- bool
}

type Broker struct {
	pb.UnimplementedBrokerServer

	mu        sync.RWMutex
	consumers map[string]Consumer
}

func (b *Broker) Publish(ctx context.Context, pr *pb.PublishRequest) (*emptypb.Empty, error) {
	if pr.Producer == nil {
		return nil, errors.New("producer not set")
	}
	slog.Info("Publishing", "contents", pr.Data)

	b.mu.RLock()
	for name, c := range b.consumers {
		slog.Info("sending stuff to consumer", "name", name)
        c.stream.Send(&pb.Message{
            Data: pr.Data,
        })
	}
	b.mu.RUnlock()

	return &emptypb.Empty{}, nil
}

func (b *Broker) Subscribe(sr *pb.SubscribeRequest, ss pb.Broker_SubscribeServer) error {
	if sr.Consumer == nil {
		return errors.New("Consumer not set")
	}

	b.mu.Lock()
	_, exists := b.consumers[sr.Consumer.Name]
	if exists {
		return fmt.Errorf("Consumer with the same name already exists. name=%s", sr.Consumer.Name) // somehow communicate a warning instead
	}

    finCh := make(chan bool)
	b.consumers[sr.Consumer.Name] = Consumer{
		start:    time.Now(),
		stream:   ss,
		finished: finCh,
	}
	b.mu.Unlock()

	slog.Info("New consumer added", "name", sr.Consumer.Name)
	for {
		select {
		case <-finCh:
			slog.Info("Closing stream for subscriber", "name", sr.Consumer.Name)
		case <-ss.Context().Done():
			slog.Info("Subscriber has disconnected", "name", sr.Consumer.Name)
			return nil
		}
	}
}

func main() {
	slog.Info("Starting server")

	listener, err := net.Listen("tcp", ":3030")
	if err != nil {
		slog.Error("Failed to create listener: %v", err)
	}

	grpcServer := grpc.NewServer()
	reflection.Register(grpcServer)

	pb.RegisterBrokerServer(grpcServer, &Broker{
		consumers: make(map[string]Consumer),
	})
	err = grpcServer.Serve(listener)
	if err != nil {
		slog.Error("Grpc server error: %v", err)
	}
}
