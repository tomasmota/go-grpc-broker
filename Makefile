.PHONY: run
run: 
	@go run main.go

.PHONY: proto
proto:
	@protoc \
		--go_out=broker \
		--go_opt=paths=source_relative \
		--go-grpc_out=broker \
		--go-grpc_opt=paths=source_relative \
		broker.proto

.PHONY: check
check:
	@grpcurl --plaintext -d '{"data": "YmxhYWE="}' localhost:3030 broker.Broker.Publish
