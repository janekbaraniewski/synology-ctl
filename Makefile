SHELL          := /bin/bash
BIN_DIR        := bin
BIN            := $(BIN_DIR)/synoctl
PKG            := github.com/janbaraniewski/synology-ctl
MAIN           := ./cmd/synoctl
GO             ?= go
GOFLAGS        ?=
VERSION        ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT         := $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE           := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS        := -s -w \
                  -X $(PKG)/internal/cli.version=$(VERSION) \
                  -X $(PKG)/internal/cli.commit=$(COMMIT) \
                  -X $(PKG)/internal/cli.date=$(DATE)

.PHONY: help build run install tidy fmt vet lint test discover clean all

help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nsynoctl — make targets\n\n"} \
	  /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2 } \
	  /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) }' $(MAKEFILE_LIST)

##@ Build

build: $(BIN) ## Compile a debug binary to ./bin/synoctl

$(BIN): $(shell find . -name '*.go' -not -path './bin/*' 2>/dev/null) go.mod go.sum | $(BIN_DIR)
	@echo "» building $(BIN) ($(VERSION) / $(COMMIT))"
	@$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BIN) $(MAIN)

$(BIN_DIR):
	@mkdir -p $(BIN_DIR)

install: ## Install to $GOPATH/bin (or $HOME/go/bin)
	@echo "» installing synoctl ($(VERSION))"
	@$(GO) install -ldflags "$(LDFLAGS)" $(MAIN)

##@ Run

run: build ## Launch the TUI against the configured (or auto-discovered) NAS
	@$(BIN)

discover: build ## Scan the local network for Synology devices
	@$(BIN) discover

login: build ## Configure credentials for a NAS profile
	@$(BIN) login

##@ Quality

tidy: ## go mod tidy
	@$(GO) mod tidy

fmt: ## gofmt -w .
	@$(GO) fmt ./...

vet: ## go vet ./...
	@$(GO) vet ./...

lint: vet ## Run all available linters
	@if command -v staticcheck >/dev/null 2>&1; then \
	  echo "» staticcheck"; \
	  staticcheck ./...; \
	else \
	  echo "» staticcheck not installed (go install honnef.co/go/tools/cmd/staticcheck@latest)"; \
	fi
	@if command -v golangci-lint >/dev/null 2>&1; then \
	  echo "» golangci-lint"; \
	  golangci-lint run; \
	else \
	  echo "» golangci-lint not installed (https://golangci-lint.run)"; \
	fi

test: ## Run unit tests
	@$(GO) test ./... -race -count=1

##@ Housekeeping

clean: ## Remove build artefacts
	@rm -rf $(BIN_DIR)

all: tidy fmt vet test build ## tidy → fmt → vet → test → build

.DEFAULT_GOAL := help
