.PHONY: build up down up-prod down-prod logs lint fmt format fmt-docker clean verify-apis verify-apis-nginx

# Go toolchain image (used when `go` is not on PATH)
GO_VERSION ?= 1.21
GO_IMAGE ?= golang:$(GO_VERSION)-bookworm

# Build the Nakama Docker image with the Go plugin
build:
	docker compose build

# Start the Nakama server (detached)
up: build
	docker compose up -d

# Stop the Nakama server
down:
	docker compose down

# Production (external Postgres / Neon): requires .env with DATABASE_ADDRESS and keys
up-prod:
	docker compose -f docker-compose.prod.yml up -d --build

down-prod:
	docker compose -f docker-compose.prod.yml down

# View Nakama server logs
logs:
	docker compose logs -f nakama

# HTTP smoke test: device auth + create_match + get_leaderboard + get_match_history (requires Nakama on 7350)
verify-apis:
	./scripts/verify-apis.sh

# Same checks through local nginx :80 -> Nakama (set on hosts that proxy 80 -> 7350)
verify-apis-nginx:
	NAKAMA_HOST=127.0.0.1 NAKAMA_PORT=80 ./scripts/verify-apis.sh

# Run golangci-lint (requires golangci-lint installed locally)
lint:
	golangci-lint run ./...

# Format all Go sources (Nakama module): gofmt + optional goimports
# Uses local `go` if available; otherwise runs `go fmt` inside Docker.
fmt format: fmt-local

fmt-local:
	@if command -v go >/dev/null 2>&1; then \
		go fmt ./...; \
		find . -name '*.go' -not -path './vendor/*' -print0 | xargs -0 -r gofmt -s -w; \
		if command -v goimports >/dev/null 2>&1; then \
			find . -name '*.go' -not -path './vendor/*' -print0 | xargs -0 -r goimports -w; \
		fi; \
		echo "Formatted with local go toolchain"; \
	else \
		$(MAKE) fmt-docker; \
	fi

fmt-docker:
	@echo "No local Go: formatting via Docker ($(GO_IMAGE))"
	docker run --rm \
		-v "$$(pwd)":/src -w /src \
		$(GO_IMAGE) \
		sh -c 'go fmt ./... && find . -name "*.go" -not -path "./vendor/*" -print0 | xargs -0 -r gofmt -s -w'

# Clean up Docker resources
clean:
	docker compose down -v --rmi all
