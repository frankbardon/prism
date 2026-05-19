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
	# Twirp generation, no-op until P14
	@echo "proto generation: TBD in P14"

clean:
	rm -rf bin/

.PHONY: build test test-race lint fmt fmt-check proto clean
