.PHONY: all build tools tag test test-integration
VERSION ?= $(shell cat cmd/debugger/VERSION)
LINTER_VERSION ?= v2.4.0

all: lint test build

lint:
	@echo "Running linters..."
	@golangci-lint run ./...

build:
	@echo "Building the project..."
	@go build -o build/remote-debugger-mcp ./cmd/debugger/*.go

build-dir:
	@if [ ! -d build/ ]; then mkdir -p build; fi

tools:
	@echo "Running tools..."
	@curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b $(shell go env GOPATH)/bin $(LINTER_VERSION)

tag:
	@echo "Tagging the current version..."
	git tag -a "v$(VERSION)" -m "Release version $(VERSION)"; \
	git push origin "v$(VERSION)"

test: build-dir
	@echo "Running tests with coverage..."
	@go test -v -short -race -coverprofile=build/coverage.out ./...
	@go tool cover -html=build/coverage.out -o build/coverage.html

test-integration:
	@echo "Running integration tests (requires SSH access)..."
	@go test -v -race -timeout 30s ./...
