package main

import (
	"github.com/tomasmota/go-grpc-broker/broker"
)

func main() {
	broker := broker.New("3030")
	broker.Start()
}
