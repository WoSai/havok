.PHONY: test
test:
	go test -covermode=atomic -v -coverprofile=coverage.txt ./...

.PHONY: benchmark
benchmark:
	go test -bench=. -run=^Benchmark ./...

.PHONY: pb
pb:
	@go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.28.1
	@go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.2.0