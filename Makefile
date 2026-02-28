SHELL := /bin/bash

GOCACHE ?= /tmp/go-build
GOTMPDIR ?= /tmp
VERSION ?= dev
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X github.com/kristianvld/dtask/internal/version.Version=$(VERSION) -X github.com/kristianvld/dtask/internal/version.Commit=$(COMMIT) -X github.com/kristianvld/dtask/internal/version.Date=$(DATE)

.PHONY: test lint build fmt

fmt:
	gofmt -w cmd internal

test:
	GOCACHE=$(GOCACHE) GOTMPDIR=$(GOTMPDIR) go test ./...

lint:
	GOCACHE=$(GOCACHE) GOTMPDIR=$(GOTMPDIR) go vet ./...

build:
	mkdir -p bin
	GOCACHE=$(GOCACHE) GOTMPDIR=$(GOTMPDIR) go build -ldflags "$(LDFLAGS)" -o bin/dtask ./cmd/dtask
