SHELL := /bin/bash

GOCACHE ?= /tmp/go-build
GOTMPDIR ?= /tmp
VERSION ?= dev
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X github.com/kristianvld/dtask/internal/version.Version=$(VERSION) -X github.com/kristianvld/dtask/internal/version.Commit=$(COMMIT) -X github.com/kristianvld/dtask/internal/version.Date=$(DATE)

.PHONY: test lint build fmt publish

fmt:
	gofmt -w cmd internal

test:
	GOCACHE=$(GOCACHE) GOTMPDIR=$(GOTMPDIR) go test ./...

lint:
	@command -v golangci-lint >/dev/null 2>&1 || (echo "golangci-lint not found. Install from https://golangci-lint.run/welcome/install/"; exit 1)
	GOCACHE=$(GOCACHE) GOTMPDIR=$(GOTMPDIR) golangci-lint run

build:
	mkdir -p bin
	GOCACHE=$(GOCACHE) GOTMPDIR=$(GOTMPDIR) go build -ldflags "$(LDFLAGS)" -o bin/dtask ./cmd/dtask

publish:
	./scripts/publish.sh
