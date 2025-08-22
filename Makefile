.PHONY: all build tools tag
VERSION ?= $(shell cat cmd/debugger/VERSION)

all: lint build

lint:
	@echo "Running linters..."
	@golangci-lint run ./...

build:
	@echo "Building the project..."
	@go build -o build/remote-debugger-mcp ./cmd/debugger/*.go

tools:
	@echo "Running tools..."
	@curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b $(shell go env GOPATH)/bin v2.3.1

tag:
	@echo "Tagging the current version..."
	git tag -a "v$(VERSION)" -m "Release version $(VERSION)"; \
	git push origin "v$(VERSION)"

test:
	@echo "Running tests..."
	@go test -race -v ./...
