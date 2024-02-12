package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/google/uuid"
	pb "github.com/tomasmota/go-grpc-broker/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/types/known/emptypb"
)

type Consumer struct {
    id uuid.UUID
    start time.Time
}

type Broker struct {
    pb.UnimplementedBrokerServer
    consumers map[string]Consumer
}

func (b *Broker) Publish(ctx context.Context, pr *pb.PublishRequest) (*emptypb.Empty, error) {
    if (pr.Producer == nil) {
        return nil, errors.New("Producer not set")
    }
    slog.Info("Publishing", "contents", pr.Data)
    return &emptypb.Empty{}, nil
}

func (b *Broker) Subscribe(ctx context.Context, sr *pb.SubscribeRequest) (*emptypb.Empty, error) {
    if (sr.Consumer == nil) {
        return nil, errors.New("Consumer not set")
    }
    slog.Info("New subscriber", "name", sr.Consumer.Name)
    _, exists := b.consumers[sr.Consumer.Name]
    if exists {
        return nil, fmt.Errorf("Consumer with the same name already exists", "name", sr.Consumer.Name) // somehow communicate a warning instead
    }

    b.consumers[sr.Consumer.Name] = Consumer{id: uuid.New(), start: time.Now()}
    slog.Info("New consumer added", "name", sr.Consumer.Name)
    return &emptypb.Empty{}, nil
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
