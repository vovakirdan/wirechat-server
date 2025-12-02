.PHONY: all test build lint fmt run bench ci race docker migrate migrate-up migrate-down migrate-status migrate-create

GO ?= go
DB_PATH ?= data/wirechat.db
MIGRATIONS_DIR ?= migrations

GOBIN := $(shell $(GO) env GOBIN)
ifeq ($(GOBIN),)
GOBIN := $(shell $(GO) env GOPATH)/bin
endif

GOLANGCI_LINT := $(GOBIN)/golangci-lint
GOLANGCI_LINT_VERSION := v1.62.2

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

lint: $(GOLANGCI_LINT)
	@echo ">> Running linters"
	$(GOLANGCI_LINT) run --config .golangci.yaml

ci: fmt lint test

docker:
	docker build -t wirechat-server:latest .

# Database migrations (using goose)
migrate: migrate-up

migrate-up:
	@echo ">> Running migrations up"
	@mkdir -p $(dir $(DB_PATH))
	@goose -dir $(MIGRATIONS_DIR) sqlite3 $(DB_PATH) up

migrate-down:
	@echo ">> Rolling back last migration"
	@goose -dir $(MIGRATIONS_DIR) sqlite3 $(DB_PATH) down

migrate-status:
	@echo ">> Checking migration status"
	@goose -dir $(MIGRATIONS_DIR) sqlite3 $(DB_PATH) status

migrate-create:
	@echo ">> Creating new migration (usage: make migrate-create NAME=migration_name)"
	@goose -dir $(MIGRATIONS_DIR) create $(NAME) sql
