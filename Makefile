# noct — atalhos de desenvolvimento.
# Rode `make` ou `make help` para ver os alvos disponíveis.

BINARY  := noct
BIN_DIR := bin
BIN     := $(BIN_DIR)/$(BINARY)
PKG     := ./cmd/noct
GO      ?= go

# Argumentos repassados ao noct: make dev ARGS="--flag valor"
ARGS ?=

# Flags extras de build: make build GOFLAGS_EXTRA="-tags foo"
GOFLAGS_EXTRA ?=

# Versão gravada no binário de release: a tag do commit (v0.1.0) ou, fora de uma, dev.
VERSION   ?= $(shell git describe --tags --exact-match 2>/dev/null || echo dev)
LDFLAGS   := -s -w -X main.version=$(VERSION)
DIST_DIR  := dist
PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64

.DEFAULT_GOAL := help

.PHONY: help dev build run start release dist release-notes install test test-race cover \
        fmt fmt-check vet check tidy clean demo

help: ## Mostra esta ajuda
	@printf "Uso: make \033[36m<alvo>\033[0m [ARGS=\"...\"]\n\n"
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

## --- Execução ---------------------------------------------------------------

dev: build ## Compila e roda o noct (ciclo rápido de desenvolvimento)
	@./$(BIN) $(ARGS)

build: ## Compila o binário em bin/noct
	@mkdir -p $(BIN_DIR)
	$(GO) build $(GOFLAGS_EXTRA) -o $(BIN) $(PKG)

run: ## Roda o binário já compilado, sem rebuild
	@test -x $(BIN) || { echo "$(BIN) não existe — rode 'make build' antes"; exit 1; }
	@./$(BIN) $(ARGS)

start: run ## Alias de run

release: ## Compila binário otimizado (sem símbolos de debug), com a VERSION
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags="$(LDFLAGS)" $(GOFLAGS_EXTRA) -o $(BIN) $(PKG)

dist: ## Gera os arquivos de uma release em dist/ (make dist VERSION=v0.1.0)
	@case "$(VERSION)" in v*.*.*) ;; *) echo "dist precisa de VERSION=vX.Y.Z (ou de uma tag no commit)"; exit 1;; esac
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

release-notes: ## Imprime a seção da VERSION no CHANGELOG.md (o texto da release no GitHub)
	@notes=$$(awk -v version="$(VERSION)" '$$1 == "##" { if (found) exit; found = ($$2 == version); next } found && (started || NF) { started = 1; print }' CHANGELOG.md); \
	if [ -z "$$notes" ]; then echo "CHANGELOG.md não tem a seção ## $(VERSION)" >&2; exit 1; fi; \
	printf '%s\n' "$$notes"

install: ## Instala o noct no GOPATH/bin (ou GOBIN) e, num terminal, abre o noct setup
	$(GO) install $(GOFLAGS_EXTRA) $(PKG)
	@bin="$$($(GO) env GOBIN)"; [ -n "$$bin" ] || bin="$$($(GO) env GOPATH)/bin"; \
	if [ -t 0 ] && [ -t 1 ]; then "$$bin/noct" setup; fi

## --- Qualidade --------------------------------------------------------------

test: ## Roda os testes
	$(GO) test ./...

test-race: ## Roda os testes com o race detector (exigido no CONTRIBUTING)
	$(GO) test -race ./...

cover: ## Gera relatório de cobertura em coverage.html
	$(GO) test -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@$(GO) tool cover -func=coverage.out | tail -1

fmt: ## Formata o código com gofmt
	gofmt -w .

fmt-check: ## Falha se algum arquivo não estiver formatado
	@out=$$(gofmt -l .); if [ -n "$$out" ]; then echo "Arquivos sem gofmt:"; echo "$$out"; exit 1; fi

vet: ## Roda go vet
	$(GO) vet ./...

check: fmt-check vet test-race ## Tudo que um PR precisa passar (fmt + vet + race)

demo: ## Regrava os GIFs do README (docs/demo/*.tape; precisa de vhs, ttyd e ffmpeg)
	docs/demo/record.sh

## --- Manutenção -------------------------------------------------------------

tidy: ## Organiza go.mod/go.sum
	$(GO) mod tidy

clean: ## Remove binários e artefatos de cobertura
	rm -rf $(BIN_DIR) $(DIST_DIR) coverage.out coverage.html
