SHELL := /bin/bash

GO ?= go
DOCKER_COMPOSE ?= docker-compose
GOCACHE ?= /private/tmp/learning-agent-gocache
CHAT_MESSAGE ?= 请帮我制定一个 Go 并发学习计划
DEV_ADDR ?=
PG_CONTAINER ?= postgres
PG_USER ?= learning_agent
PG_DB ?= learning_agent

.DEFAULT_GOAL := help

.PHONY: help tidy fmt test dev web-dev web-build chat migrate local-pg-up local-pg-down local-pg-logs local-pg-psql

help:
	@echo "Learning Agent local commands:"
	@echo "  make tidy   - Download and tidy Go modules"
	@echo "  make fmt    - Format Go code"
	@echo "  make test   - Run all Go tests"
	@echo "  make dev    - Start REST and WebSocket server with .env"
	@echo "  make web-dev - Start Vite frontend dev server"
	@echo "  make web-build - Build Vite frontend"
	@echo "  make chat   - Run one CLI chat request with .env"
	@echo "  make migrate - Apply migrations to DATABASE_URL"
	@echo "  make local-pg-up   - Start local PostgreSQL helper"
	@echo "  make local-pg-down - Stop local PostgreSQL helper"
	@echo "  make local-pg-psql - Open psql in local PostgreSQL helper"

tidy:
	$(GO) mod tidy

fmt:
	gofmt -w cmd internal

test:
	GOCACHE=$(GOCACHE) $(GO) test ./...

dev:
	@if [ -f .env ]; then \
		set -a; source .env; set +a; \
		$(GO) run ./cmd/learning-agent server $(if $(DEV_ADDR),--addr $(DEV_ADDR),); \
	else \
		$(GO) run ./cmd/learning-agent server $(if $(DEV_ADDR),--addr $(DEV_ADDR),); \
	fi

web-dev:
	cd web && npm run dev

web-build:
	cd web && npm run build

chat:
	@if [ -f .env ]; then \
		set -a; source .env; set +a; \
		$(GO) run ./cmd/learning-agent chat "$(CHAT_MESSAGE)"; \
	else \
		$(GO) run ./cmd/learning-agent chat "$(CHAT_MESSAGE)"; \
	fi

migrate:
	@if [ -f .env ]; then \
		set -a; source .env; set +a; \
		$(GO) run ./cmd/learning-agent migrate; \
	else \
		$(GO) run ./cmd/learning-agent migrate; \
	fi

local-pg-up:
	$(DOCKER_COMPOSE) up -d $(PG_CONTAINER)

local-pg-down:
	$(DOCKER_COMPOSE) down

local-pg-logs:
	$(DOCKER_COMPOSE) logs -f $(PG_CONTAINER)

local-pg-psql:
	$(DOCKER_COMPOSE) exec $(PG_CONTAINER) psql -U $(PG_USER) -d $(PG_DB)
