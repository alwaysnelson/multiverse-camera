# Multiverse Camera — developer entry points.
# Run `make` or `make help` to list targets.

SHELL := /bin/bash
GO    ?= $(shell command -v go 2>/dev/null || echo $(HOME)/sdk/go/bin/go)

.DEFAULT_GOAL := help

.PHONY: help setup dev dev-https server web build run test lint fmt check clean

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

setup: ## Install frontend dependencies and create .env from the example
	cd web && npm install
	@test -f .env || (cp .env.example .env && echo "Created .env — add your OPENAI_API_KEY")

dev: ## Run API + Vite dev server (camera works on localhost)
	scripts/dev.sh

dev-https: ## Same as dev, over HTTPS so phones on the LAN can use the camera
	scripts/dev.sh --https

server: ## Run only the Go API
	cd server && $(GO) run ./cmd/server

web: ## Run only the Vite dev server
	cd web && npm run dev

build: ## Build the frontend into web/dist and the server binary into server/bin
	cd web && npm run build
	cd server && $(GO) build -o bin/server ./cmd/server

run: build ## Build everything and serve the app from the Go binary on :8080
	cd server && ./bin/server

test: ## Run Go and frontend tests
	cd server && $(GO) test ./...
	cd web && npm test

lint: ## Lint Go (vet + gofmt) and frontend (oxlint + prettier + tsc)
	cd server && $(GO) vet ./... && test -z "$$(gofmt -l .)" || (gofmt -l . && exit 1)
	cd web && npm run typecheck && npm run lint && npm run format:check

fmt: ## Format all code
	cd server && gofmt -w .
	cd web && npm run format

check: lint test ## Everything CI runs

clean: ## Remove build artefacts
	rm -rf web/dist server/bin
