BINARY     := dva
MODULE     := github.com/ScriptonBasestar/dva
VERSION    := $(shell grep 'Version =' internal/config/version.go | cut -d'"' -f2)
COMMIT     := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
BUILD_DATE := $(shell date +%Y-%m-%dT%H:%M:%S)
BUILD_DIR  := ./bin
GOFLAGS    := -trimpath
LDFLAGS    := -s -w \
              -X $(MODULE)/internal/config.Version=$(VERSION) \
              -X $(MODULE)/internal/config.Commit=$(COMMIT) \
              -X $(MODULE)/internal/config.BuildDate=$(BUILD_DATE)

.PHONY: build install test lint clean fmt vet help bump-version

## build: Build the dva binary
build:
	go build $(GOFLAGS) -ldflags '$(LDFLAGS)' -o $(BUILD_DIR)/$(BINARY) ./cmd/dva

## bump-version: Bump micro version if there are changes or new commit
bump-version:
	@LAST_COMMIT=$$(cat .last_built_commit 2>/dev/null || echo ""); \
	CURRENT_COMMIT=$$(git rev-parse HEAD 2>/dev/null || echo "none"); \
	DIRTY=$$(git status --porcelain | grep -v 'internal/config/version.go'); \
	if [ "$$CURRENT_COMMIT" != "$$LAST_COMMIT" ] || [ -n "$$DIRTY" ]; then \
		echo "Changes or new commit detected, bumping version..."; \
		CURRENT_VERSION=$$(grep 'Version =' internal/config/version.go | cut -d'"' -f2); \
		MAJOR=$$(echo $$CURRENT_VERSION | cut -d. -f1); \
		MINOR=$$(echo $$CURRENT_VERSION | cut -d. -f2); \
		PATCH=$$(echo $$CURRENT_VERSION | cut -d. -f3); \
		NEW_PATCH=$$(($$PATCH + 1)); \
		NEW_VERSION="$$MAJOR.$$MINOR.$$NEW_PATCH"; \
		sed -i '' "s/Version = \"$$CURRENT_VERSION\"/Version = \"$$NEW_VERSION\"/" internal/config/version.go; \
		echo "$$CURRENT_COMMIT" > .last_built_commit; \
		echo "Version bumped to $$NEW_VERSION"; \
	fi

## install: Install dva to $GOPATH/bin (bumps version if changes detected)
install: bump-version
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
