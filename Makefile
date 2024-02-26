.PHONY: run
run: 
	@go run main.go

.PHONY: proto
proto:
	@protoc \
		--go_out=. \
		--go_opt=paths=source_relative \
		--go-grpc_out=. \
		--go-grpc_opt=paths=source_relative \
		proto/broker.proto

.PHONY: check
check:
	@grpcurl --plaintext -d '{"data": "YmxhYWE=", "producer": {"name": "prod"}}' localhost:3030 broker.Broker.Publish

.PHONY: test
test:
	@go test ./...
