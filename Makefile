.PHONY: all test build lint fmt run bench ci race docker migrate migrate-up migrate-down migrate-status migrate-create livekit-up livekit-down livekit-logs

GO ?= go
DB_PATH ?= data/wirechat.db
MIGRATIONS_DIR ?= migrations

GOBIN := $(shell $(GO) env GOBIN)
ifeq ($(GOBIN),)
GOBIN := $(shell $(GO) env GOPATH)/bin
endif

GOLANGCI_LINT := $(GOBIN)/golangci-lint
GOLANGCI_LINT_VERSION := v1.62.2
GOOSE := $(GOBIN)/goose
GOOSE_VERSION := v3.26.0

all: build

build:
	$(GO) build -o bin/wirechat-server cmd/server/main.go

run:
	$(GO) run ./cmd/server

test:
	$(GO) test ./... -timeout 30s

race:
	$(GO) test -race ./...

bench:
	$(GO) test -bench=. ./...

fmt:
	gofmt -w cmd internal scripts

$(GOLANGCI_LINT):
	@echo ">> Installing golangci-lint $(GOLANGCI_LINT_VERSION)"
	$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

$(GOOSE):
	@echo ">> Installing goose $(GOOSE_VERSION)"
	$(GO) install github.com/pressly/goose/v3/cmd/goose@$(GOOSE_VERSION)

lint: $(GOLANGCI_LINT)
	@echo ">> Running linters"
	$(GOLANGCI_LINT) run --config .golangci.yaml

ci: fmt lint test

docker:
	docker build -t wirechat-server:latest .

# Database migrations (using goose)
migrate: migrate-up

migrate-up: $(GOOSE)
	@echo ">> Running migrations up"
	@mkdir -p $(dir $(DB_PATH))
	@$(GOOSE) -dir $(MIGRATIONS_DIR) sqlite3 $(DB_PATH) up

migrate-down: $(GOOSE)
	@echo ">> Rolling back last migration"
	@$(GOOSE) -dir $(MIGRATIONS_DIR) sqlite3 $(DB_PATH) down

migrate-status: $(GOOSE)
	@echo ">> Checking migration status"
	@$(GOOSE) -dir $(MIGRATIONS_DIR) sqlite3 $(DB_PATH) status

migrate-create: $(GOOSE)
	@echo ">> Creating new migration (usage: make migrate-create NAME=migration_name)"
	@$(GOOSE) -dir $(MIGRATIONS_DIR) create $(NAME) sql

# LiveKit infrastructure (dev)
livekit-up:
	@$(MAKE) -C infra/livekit up

livekit-down:
	@$(MAKE) -C infra/livekit down

livekit-logs:
	@$(MAKE) -C infra/livekit logs
