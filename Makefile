.PHONY: help tidy run build test lint fmt docker-build docker-up docker-down

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?##' Makefile | sort | awk 'BEGIN {FS = ":.*?##"} {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

tidy: ## go mod tidy
	go mod tidy

run: ## Run the API locally
	go run ./cmd/api

build: ## Build the API binary
	CGO_ENABLED=0 go build -ldflags="-s -w" -o ./bin/api ./cmd/api

test: ## Run unit tests with race detector
	go test -race -count=1 ./...

lint: ## Run golangci-lint (must be installed)
	golangci-lint run ./...

fmt: ## Format code
	gofmt -s -w .

docker-build: ## Build the api image
	docker build -t caretexnursing-core:dev .

docker-up: ## Start db + api
	docker compose up -d --build

docker-down: ## Stop and remove containers
	docker compose down -v
