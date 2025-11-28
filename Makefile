.PHONY: all test build lint fmt run bench ci race docker

GO ?= go

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
	$(GO) test ./...

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
