.PHONY: all build dev test test-integration lint clean

# Default Go binary location
GO ?= go

all: build

build:
	$(GO) build -v -o bin/gateflow ./cmd/gateway
	cd frontend && npm run build

dev:
	$(GO) run ./cmd/gateway

test:
	$(GO) test -v ./internal/...

test-integration:
	$(GO) test -v ./tests/integration/...

lint:
	$(GO) vet ./...
	cd frontend && npm run build

clean:
	rm -rf bin/ frontend/dist
