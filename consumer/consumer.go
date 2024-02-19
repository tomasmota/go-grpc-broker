package consumer

import (
	pb "github.com/tomasmota/go-grpc-broker/proto"
	"time"
)

type Consumer struct {
	Start time.Time

	Stream   pb.Broker_SubscribeServer
	Finished chan<- bool
}
