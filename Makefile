.PHONY: build test test-cover lint dev clean install release vet

# Variables
BINARY_NAME=herald
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME=$(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS=-ldflags "-s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildTime=$(BUILD_TIME)"

# Build
build:
	CGO_ENABLED=0 go build $(LDFLAGS) -o bin/$(BINARY_NAME) ./cmd/herald

# Test
test:
	go test ./... -race -count=1

test-cover:
	go test ./... -race -count=1 -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Quality
vet:
	go vet ./...

lint:
	@which golangci-lint > /dev/null 2>&1 || (echo "Install: https://golangci-lint.run/usage/install/" && exit 1)
	golangci-lint run

# Dev
dev:
	@which air > /dev/null 2>&1 || (echo "Install: go install github.com/air-verse/air@latest" && exit 1)
	air -c .air.toml

run: build
	./bin/$(BINARY_NAME) serve

# Install
install: build
	cp bin/$(BINARY_NAME) $(GOPATH)/bin/$(BINARY_NAME)

# Clean
clean:
	rm -rf bin/ coverage.out coverage.html

# Release
release:
	@which goreleaser > /dev/null 2>&1 || (echo "Install: https://goreleaser.com/install/" && exit 1)
	goreleaser release --snapshot --clean
