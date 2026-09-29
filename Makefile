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

.DEFAULT_GOAL := help

.PHONY: help dev build run start release install test test-race cover \
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

release: ## Compila binário otimizado (sem símbolos de debug)
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags="-s -w" $(GOFLAGS_EXTRA) -o $(BIN) $(PKG)

install: ## Instala o noct no GOPATH/bin (ou GOBIN)
	$(GO) install $(GOFLAGS_EXTRA) $(PKG)

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
	rm -rf $(BIN_DIR) coverage.out coverage.html
