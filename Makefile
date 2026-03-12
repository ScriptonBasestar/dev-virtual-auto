BINARY     := dva
MODULE     := github.com/ScriptonBasestar/dva
VERSION    := $(shell grep 'Version = ' internal/config/config.go | cut -d'"' -f2)
BUILD_DIR  := ./bin
GOFLAGS    := -trimpath
LDFLAGS    := -s -w -X $(MODULE)/internal/config.Version=$(VERSION)

.PHONY: build install test lint clean fmt vet help

## build: Build the dva binary
build:
	go build $(GOFLAGS) -ldflags '$(LDFLAGS)' -o $(BUILD_DIR)/$(BINARY) ./cmd/dva

## install: Install dva to $GOPATH/bin
install:
	go install $(GOFLAGS) -ldflags '$(LDFLAGS)' ./cmd/dva

## test: Run all tests
test:
	go test -race -cover ./...

## lint: Run linters
lint: vet
	@which golangci-lint > /dev/null 2>&1 || echo "Install golangci-lint: https://golangci-lint.run/usage/install/"
	golangci-lint run ./...

## vet: Run go vet
vet:
	go vet ./...

## fmt: Format code
fmt:
	gofmt -w -s .

## clean: Remove build artifacts
clean:
	rm -rf $(BUILD_DIR)
	go clean -cache

## help: Show this help
help:
	@echo "Available targets:"
	@grep -E '^##' $(MAKEFILE_LIST) | sed 's/## /  /' | column -t -s ':'
