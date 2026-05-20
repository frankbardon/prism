build:
	go build -o bin/prism ./cmd/prism

test:
	go test ./...

test-race:
	go test -race ./...

lint:
	staticcheck ./...
	go vet ./...

fmt:
	gofmt -w .

fmt-check:
	gofmt -d .

proto:
	@if ! command -v protoc >/dev/null 2>&1; then \
		echo "proto: protoc not installed (brew install protobuf)."; \
		echo "proto: generated files are committed; build does not require protoc."; \
		exit 0; \
	fi
	@if ! command -v protoc-gen-go >/dev/null 2>&1; then \
		echo "proto: protoc-gen-go not installed."; \
		echo "proto:   go install google.golang.org/protobuf/cmd/protoc-gen-go@latest"; \
		echo "proto: generated files are committed; build does not require protoc."; \
		exit 0; \
	fi
	@if ! command -v protoc-gen-twirp >/dev/null 2>&1; then \
		echo "proto: protoc-gen-twirp not installed."; \
		echo "proto:   go install github.com/twitchtv/twirp/protoc-gen-twirp@latest"; \
		echo "proto: generated files are committed; build does not require protoc."; \
		exit 0; \
	fi
	protoc --go_out=paths=source_relative:. --twirp_out=paths=source_relative:. rpc/service.proto

clean:
	rm -rf bin/

.PHONY: build test test-race lint fmt fmt-check proto clean
