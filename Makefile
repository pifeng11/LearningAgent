SHELL := /bin/bash

GO ?= go
GOCACHE ?= /private/tmp/learning-agent-gocache
CHAT_MESSAGE ?= 请帮我制定一个 Go 并发学习计划

.DEFAULT_GOAL := help

.PHONY: help tidy fmt test dev chat

help:
	@echo "Learning Agent local commands:"
	@echo "  make tidy   - Download and tidy Go modules"
	@echo "  make fmt    - Format Go code"
	@echo "  make test   - Run all Go tests"
	@echo "  make dev    - Start REST and WebSocket server with .env"
	@echo "  make chat   - Run one CLI chat request with .env"

tidy:
	$(GO) mod tidy

fmt:
	gofmt -w cmd internal

test:
	GOCACHE=$(GOCACHE) $(GO) test ./...

dev:
	@if [ -f .env ]; then \
		set -a; source .env; set +a; \
		$(GO) run ./cmd/learning-agent server; \
	else \
		$(GO) run ./cmd/learning-agent server; \
	fi

chat:
	@if [ -f .env ]; then \
		set -a; source .env; set +a; \
		$(GO) run ./cmd/learning-agent chat "$(CHAT_MESSAGE)"; \
	else \
		$(GO) run ./cmd/learning-agent chat "$(CHAT_MESSAGE)"; \
	fi
