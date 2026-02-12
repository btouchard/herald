.PHONY: build build-debug test test-unit test-integration test-leaks test-cover lint fix run dev clean install release

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -s -w -X main.version=$(VERSION)

build:
	CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o bin/herald ./cmd/herald

build-debug:
	CGO_ENABLED=0 GOEXPERIMENT=goroutineleakprofile go build -o bin/herald-debug ./cmd/herald

test:
	go test ./... -v -race -count=1

test-unit:
	go test ./internal/... -v -short -race -count=1

test-integration:
	go test ./tests/integration/... -tags=integration -v -race -count=1

test-leaks:
	GOEXPERIMENT=goroutineleakprofile go test ./tests/integration/... -v -run TestNoGoroutineLeak

test-cover:
	go test ./... -race -count=1 -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

lint:
	golangci-lint run ./...

fix:
	go fix ./...

run:
	go run ./cmd/herald serve

dev:
	air -c .air.toml

clean:
	rm -rf bin/ tmp/ coverage.out coverage.html

install: build
	cp bin/herald $(GOPATH)/bin/herald 2>/dev/null || cp bin/herald /usr/local/bin/herald

release:
	goreleaser release --snapshot --clean
