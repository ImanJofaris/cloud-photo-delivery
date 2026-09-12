.DEFAULT_GOAL := help

GOBIN := $(shell go env GOPATH)/bin

.PHONY: help
help:
	@echo "Targets:"
	@echo "  up          Start Postgres + MinIO"
	@echo "  down        Stop containers"
	@echo "  run         Run the API server"
	@echo "  worker      Run the worker"
	@echo "  build       Build api + worker binaries"
	@echo "  test        Run unit tests"
	@echo "  test-race   Run unit tests with -race"
	@echo "  test-integration  Run integration tests (requires docker)"
	@echo "  cover       Unit tests with coverage profile"
	@echo "  lint        Run go vet + golangci-lint (if installed)"
	@echo "  fmt         Format code"
	@echo "  migrate-up  Apply migrations"
	@echo "  migrate-down Roll back one migration"

.PHONY: up
up:
	docker compose up -d

.PHONY: down
down:
	docker compose down

.PHONY: run
run:
	go run ./cmd/api

.PHONY: worker
worker:
	go run ./cmd/worker

.PHONY: build
build:
	go build -o bin/api ./cmd/api
	go build -o bin/worker ./cmd/worker

.PHONY: test
test:
	go test ./...

.PHONY: test-race
test-race:
	go test -race ./...

.PHONY: test-integration
test-integration:
	go test -tags=integration -race ./...

.PHONY: cover
cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

.PHONY: lint
lint:
	go vet ./...
	@if command -v golangci-lint >/dev/null 2>&1; then golangci-lint run; else echo "golangci-lint not installed, skipping"; fi

.PHONY: fmt
fmt:
	gofmt -s -w .

.PHONY: migrate-up
migrate-up:
	@if command -v goose >/dev/null 2>&1; then \
		goose -dir migrations postgres "$$DATABASE_URL" up; \
	else \
		echo "goose not installed: go install github.com/pressly/goose/v3/cmd/goose@latest"; exit 1; \
	fi

.PHONY: migrate-down
migrate-down:
	@if command -v goose >/dev/null 2>&1; then \
		goose -dir migrations postgres "$$DATABASE_URL" down; \
	else \
		echo "goose not installed: go install github.com/pressly/goose/v3/cmd/goose@latest"; exit 1; \
	fi
