package broker

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	pb "github.com/tomasmota/go-grpc-broker/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

func createServer(t *testing.T, ctx context.Context) (pb.BrokerClient, func()) {
	listener := bufconn.Listen(1024 * 1024)
	grpcServer := grpc.NewServer()
	pb.RegisterBrokerServer(grpcServer, New())

	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			t.Errorf("Failed to start test server: %v", err)
		}
	}()

	conn, err := grpc.DialContext(ctx, "",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return listener.Dial()
		}), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Printf("error connecting to server: %v", err)
	}

	closer := func() {
		err := listener.Close()
		if err != nil {
			t.Errorf("error closing listener: %v", err)
		}
		grpcServer.Stop()
	}

	return pb.NewBrokerClient(conn), closer
}

func TestPublishing(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	c, closer := createServer(t, ctx)
	defer closer()

	pr := &pb.PublishRequest{
		Producer: &pb.Producer{
			Name: "test producer",
		},
		Topic: "simple-test",
		Data: []byte("test data"),
	}

	const numSubscribers = 500

	done := make(chan error, numSubscribers)
	readyWg := sync.WaitGroup{}
	readyWg.Add(numSubscribers)
	for i := 0; i < numSubscribers; i++ {
		go func(i int) {
			sr := &pb.SubscribeRequest{
				Consumer: &pb.Consumer{
					Name: fmt.Sprintf("test consumer %d", i),
				},
				Topic: "simple-test",
			}

			sc, err := c.Subscribe(ctx, sr)
			if err != nil {
				done <- err
				return
			}

			// Make sure subscription has been registered in server
			// TODO: check if there is a way to check that the 
			// subscription has been registered
			time.Sleep(time.Millisecond * 100)
			readyWg.Done()

			msg := &pb.Message{}
			err = sc.RecvMsg(msg)
			if err != nil {
				done <- err
				return
			}

			if !bytes.Equal(pr.Data, msg.Data) {
				done <- fmt.Errorf("data mismatch: expected %v, got %v", pr.Data, msg.Data)
				return
			}
			done <- nil
		}(i)
	}

	// wait for all subscribers to be ready before publishing
	readyWg.Wait()

	_, err := c.Publish(ctx, pr)
	require.NoError(t, err)

	for i := 0; i < numSubscribers; i++ {
		select {
		case err := <-done:
			require.NoError(t, err)
		case <-ctx.Done():
			t.Fatal("test timed out")
		}
	}
}
