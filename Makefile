.PHONY: test
test: 
	@go test -race -covermode=atomic -v -coverprofile=coverage.txt ./... || exit 1;
	@for dir in `find . -type f -name "go.mod" -exec dirname {} \;`; do \
		if [ $$dir != "." ]; then \
			cd $$dir; \
			go test -race -covermode=atomic -v -coverprofile=coverage.txt ./... || exit 1; \
			cd - > /dev/null ;\
			lines=`cat $$dir/coverage.txt | wc -l`; \
			lines=`expr $$lines - 1`; \
			tail -n $$lines $$dir/coverage.txt >> coverage.txt; \
		fi; \
	done

.PHONY: benchmark
benchmark: 
	@for dir in `find . -type f -name "go.mod" -exec dirname {} \;`; do \
		cd $$dir; \
		go test -bench=. -run=^Benchmark ./...; \
		cd - > /dev/null; \
	done

.PHONY: pb
pb:
	@go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.28
	@go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.2

.PHONY: api
api: pb
	@protoc --proto_path=api/protobuf/ api/protobuf/havok.proto \
	--go_out=pkg/genproto \
	--go_opt=paths=source_relative \
	--go-grpc_opt=require_unimplemented_servers=false \
	--go-grpc_out=pkg/genproto \
	--go-grpc_opt=paths=source_relative