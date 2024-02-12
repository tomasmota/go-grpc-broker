package main

import (
	"context"
	"log/slog"
	"net"

	"github.com/tomasmota/go-grpc-broker/broker"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
    "google.golang.org/grpc/reflection"
)

type Broker struct {
    broker.UnimplementedBrokerServer
}

func (b *Broker) Publish(ctx context.Context, req *broker.PublishRequest) (*emptypb.Empty, error) {
    slog.Info("Publishing", "contents", req.Data)
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

    broker.RegisterBrokerServer(grpcServer, &Broker{})
    err = grpcServer.Serve(listener)
    if err != nil {
        slog.Error("Grpc server error: %v", err)
    }
}
