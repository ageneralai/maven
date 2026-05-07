.PHONY: build build-release package package-all run gateway tunnel test setup clean docker-up docker-down lint

BINARY    := maven
BUILD_DIR := .
CONFIG    := $(HOME)/.maven/config.json
FEISHU_PORT ?= 9876
DIST_DIR  := dist
RELEASE_LDFLAGS := -s -w
PLATFORMS ?= darwin/arm64 linux/amd64 linux/arm64

## Build
build:
	go build -o $(BINARY) ./cmd/maven

## Build optimized release binary (smaller size)
build-release:
	go build -trimpath -ldflags="$(RELEASE_LDFLAGS)" -o $(BINARY) ./cmd/maven

## Package optimized binary (gzip)
package: build-release
	@mkdir -p $(DIST_DIR)
	@os=$$(go env GOOS); arch=$$(go env GOARCH); out="$(DIST_DIR)/$(BINARY)-$$os-$$arch"; \
	cp $(BINARY) "$$out"; \
	gzip -f -9 "$$out"; \
	ls -lh "$$out.gz"

## Package optimized binaries for multiple platforms
package-all:
	@mkdir -p $(DIST_DIR)
	@set -e; for platform in $(PLATFORMS); do \
		os=$${platform%/*}; arch=$${platform#*/}; out="$(DIST_DIR)/$(BINARY)-$$os-$$arch"; \
		echo "Building $$os/$$arch..."; \
		GOOS=$$os GOARCH=$$arch go build -trimpath -ldflags="$(RELEASE_LDFLAGS)" -o "$$out" ./cmd/maven; \
		gzip -f -9 "$$out"; \
		ls -lh "$$out.gz"; \
	done

## Run agent REPL
run: build
	./$(BINARY) agent

## Run gateway (channels + cron + heartbeat)
gateway: build
	./$(BINARY) gateway

## Run onboard to initialize config and workspace
onboard: build
	./$(BINARY) onboard

## Show status
status: build
	./$(BINARY) status

## Start cloudflared tunnel for Feishu webhook
tunnel:
	@command -v cloudflared >/dev/null 2>&1 || { echo "Install cloudflared: brew install cloudflared"; exit 1; }
	@echo "Starting cloudflared tunnel -> http://localhost:$(FEISHU_PORT)"
	@echo "Copy the https://*.trycloudflare.com URL to Feishu event subscription"
	cloudflared tunnel --url http://localhost:$(FEISHU_PORT)

## Interactive setup: generate config.json
setup:
	@bash scripts/setup.sh

## Run all tests
test:
	go test ./... -count=1

## Run tests with race detection
test-race:
	go test -race ./... -count=1

## Run tests with coverage
test-cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

## Docker: build and start
docker-up:
	docker compose up -d --build

## Docker: start with cloudflared tunnel
docker-up-tunnel:
	docker compose --profile tunnel up -d --build

## Docker: stop
docker-down:
	docker compose down

## Clean build artifacts
clean:
	rm -f $(BINARY) coverage.out

## Lint (requires golangci-lint)
lint:
	@command -v golangci-lint >/dev/null 2>&1 || { echo "Install: brew install golangci-lint"; exit 1; }
	golangci-lint run ./...

## Help
help:
	@echo "Maven Makefile targets:"
	@echo ""
	@echo "  build           Build binary"
	@echo "  build-release   Build optimized release binary"
	@echo "  package         Package optimized binary as .gz"
	@echo "  package-all     Package optimized binaries for multiple platforms"
	@echo "  run             Run agent REPL"
	@echo "  gateway         Start gateway (channels + cron)"
	@echo "  onboard         Initialize config and workspace"
	@echo "  status          Show Maven status"
	@echo "  setup           Interactive config setup"
	@echo "  tunnel          Start cloudflared tunnel for Feishu"
	@echo "  test            Run tests"
	@echo "  test-race       Run tests with race detection"
	@echo "  test-cover      Run tests with coverage report"
	@echo "  docker-up       Docker build and start"
	@echo "  docker-up-tunnel Docker start with cloudflared tunnel"
	@echo "  docker-down     Docker stop"
	@echo "  clean           Remove build artifacts"
	@echo "  lint            Run golangci-lint"
