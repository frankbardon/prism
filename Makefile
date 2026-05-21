.PHONY: build build-wasm clean test test-race cover fmt fmt-check vet lint proto docs docs-scenes docs-wasm-stage docs-serve docs-clean

BINARY_NAME=prism
WASM_BINARY=prism.wasm
BUILD_DIR=bin
GO=go
LDFLAGS=-s -w
BUILD_FLAGS=-trimpath -ldflags="$(LDFLAGS)"
WASM_BUILD_FLAGS=-trimpath -buildvcs=false -ldflags="$(LDFLAGS)"

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

# build-wasm cross-compiles cmd/prismwasm to GOOS=js GOARCH=wasm and
# copies the toolchain's wasm_exec.js into bin/ alongside the binary
# so `make build-wasm` produces both halves of a runnable bundle.
# The companion wasm_exec.js path varies across Go releases (1.24+
# lives under $GOROOT/lib/wasm/, earlier under $GOROOT/misc/wasm/);
# we resolve it dynamically and fall back to the legacy location if
# the new one is absent.
build-wasm:
	@mkdir -p $(BUILD_DIR)
	GOOS=js GOARCH=wasm CGO_ENABLED=0 $(GO) build $(WASM_BUILD_FLAGS) -o $(BUILD_DIR)/$(WASM_BINARY) ./cmd/prismwasm
	@WASM_EXEC="$$($(GO) env GOROOT)/lib/wasm/wasm_exec.js"; \
	if [ ! -f "$$WASM_EXEC" ]; then WASM_EXEC="$$($(GO) env GOROOT)/misc/wasm/wasm_exec.js"; fi; \
	if [ -f "$$WASM_EXEC" ]; then cp "$$WASM_EXEC" $(BUILD_DIR)/wasm_exec.js; \
	else echo "build-wasm: warning — wasm_exec.js not found under GOROOT (looked at lib/wasm and misc/wasm)"; fi
	@ls -lh $(BUILD_DIR)/$(WASM_BINARY)

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

# Regenerate geodata bundles + manifest from upstream Natural Earth.
# Currently a placeholder pointing at the documented manual procedure;
# the committed `geodata/*.geo.json` + `geodata/manifest.json` artifacts
# are usable as-is and refreshed via this target when admin levels
# change. Full automated ingestion lands in a follow-up.
geodata:
	@echo "geodata: see internal/tools/build_geodata/README.md for the regeneration procedure."
	@echo "geodata: committed artifacts under geodata/ are the input to 'make build'; no network required."

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

# docs-wasm-stage copies the WASM build outputs from bin/ into
# static/vendor/prism/ so mdBook picks them up via the
# docs/src/static symlink. The files are gitignored (P17): they
# only exist locally to serve the live <prism-chart> gallery
# (docs/src/gallery/index.html). docs/docs-serve depend on this
# so a fresh checkout + `make docs-serve` works without manual
# staging steps.
docs-wasm-stage: build-wasm
	@cp $(BUILD_DIR)/$(WASM_BINARY) static/vendor/prism/$(WASM_BINARY)
	@cp $(BUILD_DIR)/wasm_exec.js   static/vendor/prism/wasm_exec.js
	@echo "docs-wasm-stage: staged prism.wasm + wasm_exec.js into static/vendor/prism/"

docs: build docs-scenes docs-wasm-stage
	mdbook build docs

docs-serve: build docs-scenes docs-wasm-stage
	mdbook serve docs --open

docs-clean:
	rm -rf docs/book
	rm -f static/vendor/prism/$(WASM_BINARY) static/vendor/prism/wasm_exec.js
	find $(GALLERY_DIR) -name '*.scene.json' -delete 2>/dev/null || true

.DEFAULT_GOAL := build
