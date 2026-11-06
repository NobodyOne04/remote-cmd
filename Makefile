# tcp-pipe — run `make help`.
SHELL := /bin/bash
.DEFAULT_GOAL := help
BIN := tcp-pipe

.PHONY: help build test check fmt vet clean

help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_-]+:.*##/ {printf "  \033[36m%-8s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the ./$(BIN) binary
	go build -o $(BIN) .

test: ## Run the tests with the race detector
	go test -race ./...

check: ## What CI runs: gofmt, vet, race tests
	@test -z "$$(gofmt -l .)" || { echo "gofmt needed:"; gofmt -l .; exit 1; }
	go vet ./...
	go test -race ./...

fmt: ## Format the code
	gofmt -w .

clean: ## Remove the built binary
	rm -f $(BIN)
