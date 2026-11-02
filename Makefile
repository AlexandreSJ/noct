# noct — development shortcuts.
# Run `make` or `make help` to see the targets.

BINARY  := noct
BIN_DIR := bin
BIN     := $(BIN_DIR)/$(BINARY)
PKG     := ./cmd/noct
GO      ?= go

# Arguments passed to noct: make dev ARGS="--flag value"
ARGS ?=

# Extra build flags: make build GOFLAGS_EXTRA="-tags foo"
GOFLAGS_EXTRA ?=

# Version written into a release binary: the commit's tag (v0.1.0) or, outside one, dev.
VERSION   ?= $(shell git describe --tags --exact-match 2>/dev/null || echo dev)
# VERSION reaches the recipes' shell as text, so it must be a plain version before any of them
# runs: a tag named v1.2.3`cmd` would otherwise run cmd. The checks read it from the
# environment, where it is only data.
export VERSION
SEMVER    := v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.]+)?
LDFLAGS   := -s -w -X main.version=$(VERSION)
DIST_DIR  := dist
PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64

.DEFAULT_GOAL := help

.PHONY: help dev build run start release dist release-notes install test test-race cover \
        fmt fmt-check vet check tidy clean demo

help: ## Show this help
	@printf "Usage: make \033[36m<target>\033[0m [ARGS=\"...\"]\n\n"
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

## --- Running -----------------------------------------------------------------

dev: build ## Build and run noct (the quick development loop)
	@./$(BIN) $(ARGS)

build: ## Build the binary into bin/noct
	@mkdir -p $(BIN_DIR)
	$(GO) build $(GOFLAGS_EXTRA) -o $(BIN) $(PKG)

run: ## Run the binary already built, without building it again
	@test -x $(BIN) || { echo "$(BIN) does not exist: run 'make build' first"; exit 1; }
	@./$(BIN) $(ARGS)

start: run ## Same as run

release: ## Build an optimized binary (no debug symbols) with the VERSION
	@printf '%s\n' "$$VERSION" | grep -Eqx 'dev|$(SEMVER)' || { echo "VERSION must be dev or vX.Y.Z" >&2; exit 1; }
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags="$(LDFLAGS)" $(GOFLAGS_EXTRA) -o $(BIN) $(PKG)

dist: ## Build a release's archives into dist/ (make dist VERSION=v0.1.0)
	@printf '%s\n' "$$VERSION" | grep -Eqx '$(SEMVER)' || { echo "dist needs VERSION=vX.Y.Z (or a tag on the commit)" >&2; exit 1; }
	@rm -rf $(DIST_DIR) && mkdir -p $(DIST_DIR)
	@for platform in $(PLATFORMS); do \
		os=$${platform%/*}; arch=$${platform#*/}; name=noct_$(VERSION)_$${os}_$${arch}; \
		echo "  $$name.tar.gz"; \
		mkdir -p $(DIST_DIR)/$$name && \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch $(GO) build -trimpath -ldflags="$(LDFLAGS)" -o $(DIST_DIR)/$$name/noct $(PKG) && \
		cp LICENSE NOTICE README.md $(DIST_DIR)/$$name/ && \
		tar -czf $(DIST_DIR)/$$name.tar.gz -C $(DIST_DIR)/$$name noct LICENSE NOTICE README.md && \
		rm -rf $(DIST_DIR)/$$name || exit 1; \
	done
	@cd $(DIST_DIR) && sha256sum *.tar.gz > checksums.txt
	@echo "  checksums.txt"

release-notes: ## Print the VERSION's section of CHANGELOG.md (the release's text on GitHub)
	@printf '%s\n' "$$VERSION" | grep -Eqx '$(SEMVER)' || { echo "release-notes needs VERSION=vX.Y.Z" >&2; exit 1; }
	@notes=$$(awk -v version="$$VERSION" '$$1 == "##" { if (found) exit; found = ($$2 == version); next } found && (started || NF) { started = 1; print }' CHANGELOG.md); \
	if [ -z "$$notes" ]; then echo "CHANGELOG.md has no section ## $$VERSION" >&2; exit 1; fi; \
	printf '%s\n' "$$notes"

install: ## Install noct into GOPATH/bin (or GOBIN) and, in a terminal, open noct setup
	$(GO) install $(GOFLAGS_EXTRA) $(PKG)
	@bin="$$($(GO) env GOBIN)"; [ -n "$$bin" ] || bin="$$($(GO) env GOPATH)/bin"; \
	if [ -t 0 ] && [ -t 1 ]; then "$$bin/noct" setup; fi

## --- Quality -----------------------------------------------------------------

test: ## Run the tests
	$(GO) test ./...

test-race: ## Run the tests with the race detector (required by CONTRIBUTING)
	$(GO) test -race ./...

cover: ## Write a coverage report to coverage.html
	$(GO) test -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@$(GO) tool cover -func=coverage.out | tail -1

fmt: ## Format the code with gofmt
	gofmt -w .

fmt-check: ## Fail when a file is not formatted
	@out=$$(gofmt -l .); if [ -n "$$out" ]; then echo "Files not formatted with gofmt:"; echo "$$out"; exit 1; fi

vet: ## Run go vet
	$(GO) vet ./...

check: fmt-check vet test-race ## Everything a change must pass (fmt + vet + race)

demo: ## Record the README GIFs again (docs/demo/*.tape; needs vhs, ttyd and ffmpeg)
	docs/demo/record.sh

## --- Maintenance -------------------------------------------------------------

tidy: ## Tidy go.mod and go.sum
	$(GO) mod tidy

clean: ## Remove binaries and coverage files
	rm -rf $(BIN_DIR) $(DIST_DIR) coverage.out coverage.html
