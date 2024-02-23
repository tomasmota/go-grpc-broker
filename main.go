package main

import (
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	pb "github.com/tomasmota/go-grpc-broker/proto"

	"github.com/tomasmota/go-grpc-broker/broker"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	listener, err := net.Listen("tcp", ":3030")
	if err != nil {
		slog.Error("Failed to create listener: %v", err)
		os.Exit(1)
	}

	grpcServer := grpc.NewServer()
	reflection.Register(grpcServer)

	pb.RegisterBrokerServer(grpcServer, broker.New())

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		slog.Info("Shutting down server...")

		grpcServer.GracefulStop()
	}()

	slog.Info("Starting server")
	if err := grpcServer.Serve(listener); err != nil {
		slog.Error("Grpc server error: %v", err)
		os.Exit(1)
	}
}
