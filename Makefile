.PHONY: build clean test test-race cover fmt fmt-check vet lint proto docs docs-scenes docs-serve docs-clean

BINARY_NAME=prism
BUILD_DIR=bin
GO=go
LDFLAGS=-s -w
BUILD_FLAGS=-trimpath -ldflags="$(LDFLAGS)"

# Prism is pure Go — no CGO dependency in the build graph. Disabling CGO
# globally makes that a contract: any future import that pulls in a C
# toolchain fails the build instead of silently re-introducing the
# dependency. Override on the command line if a downstream consumer
# really needs CGO (e.g. `make build CGO_ENABLED=1`).
export CGO_ENABLED=0

ifneq (,$(wildcard ./.env))
    include .env
    export
endif

build:
	$(GO) build $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/prism

clean:
	rm -rf $(BUILD_DIR) coverage.out

test:
	$(GO) test ./...

test-race:
	CGO_ENABLED=1 $(GO) test -race ./...

cover:
	$(GO) test -coverprofile=coverage.out ./...
	$(GO) tool cover -func=coverage.out

fmt:
	$(GO) fmt ./...

fmt-check:
	gofmt -d .

vet:
	$(GO) vet ./...

lint: vet
	$(GO) run honnef.co/go/tools/cmd/staticcheck@latest ./...

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

GALLERY_DIR=docs/src/gallery
GALLERY_SPECS=$(shell find $(GALLERY_DIR) -name '*.prism.json' 2>/dev/null)
GALLERY_SCENES=$(patsubst %.prism.json,%.scene.json,$(GALLERY_SPECS))

# docs-scenes compiles every gallery spec to its SceneDoc JSON
# counterpart so the live <prism-chart> web component can render it
# (the JS bundle has no spec compiler — it consumes scene IR). The
# pattern rule runs `prism scene` per fixture and is incremental:
# only specs newer than their .scene.json are recompiled. Specs that
# reference unavailable cohorts (e.g. multi-source/*pulse*) emit a
# warning and skip without failing the build.
docs-scenes: $(GALLERY_SCENES)

$(GALLERY_DIR)/%.scene.json: $(GALLERY_DIR)/%.prism.json $(BUILD_DIR)/$(BINARY_NAME)
	@$(BUILD_DIR)/$(BINARY_NAME) scene --out $@ $< 2>/dev/null || { \
		echo "docs-scenes: skip $< (compile failed — likely missing cohort)"; \
		rm -f $@; \
	}

docs: build docs-scenes
	mdbook build docs

docs-serve: build docs-scenes
	mdbook serve docs --open

docs-clean:
	rm -rf docs/book
	find $(GALLERY_DIR) -name '*.scene.json' -delete 2>/dev/null || true

.DEFAULT_GOAL := build
