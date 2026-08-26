BINARY     := dva
MODULE     := github.com/ScriptonBasestar/dva
BUILD_DIR  := ./bin
GOFLAGS     := -trimpath

# Workflow source directories (single source of truth)
WF_LIBRARY  := agent-mesh-flows/shared/library

# Generated embeddable files (output of make generate)
# library_reference.txt: read by am flows, corpus for removed_keys_test (no longer embedded)
# dva_guide_template.txt: embedded by ai_docs.go (static, hand-authored)
GEN_DIR         := internal/cli
GEN_LIBRARY     := $(GEN_DIR)/library_reference.txt

.PHONY: build install test test-integration test-skill-dogfood lint clean fmt fmt-check vet help generate check-generate doc-check commit-check dogfood-skill-install

## build: Build the dva binary (CI)
build: generate
	$(eval VERSION := $(shell grep -E '^[[:space:]]+Version = ' internal/config/version.go | cut -d'"' -f2))
	$(eval COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none"))
	$(eval BUILD_DATE := $(shell date +%Y-%m-%dT%H:%M:%S))
	go build $(GOFLAGS) -ldflags '-s -w -X $(MODULE)/internal/config.Version=$(VERSION) -X $(MODULE)/internal/config.Commit=$(COMMIT) -X $(MODULE)/internal/config.BuildDate=$(BUILD_DATE)' -o $(BUILD_DIR)/$(BINARY) ./cmd/dva

## install: Install dva to ~/.local/bin
install: build
	@mkdir -p $(HOME)/.local/bin
	rm -f $(HOME)/.local/bin/$(BINARY)
	cp $(BUILD_DIR)/$(BINARY) $(HOME)/.local/bin/$(BINARY)
	@GO_BIN_DIR=$$(go env GOBIN); \
		[ -n "$$GO_BIN_DIR" ] || GO_BIN_DIR="$$(go env GOPATH)/bin"; \
		mkdir -p "$$GO_BIN_DIR"; \
		rm -f "$$GO_BIN_DIR/$(BINARY)"; \
		cp $(BUILD_DIR)/$(BINARY) "$$GO_BIN_DIR/$(BINARY)"

## dogfood-skill-install: Black-box test a selected SHA-pinned DVA executable against a stable flow repository
dogfood-skill-install:
	@test -n "$(DVA_BIN)" || { echo "ERROR: set DVA_BIN to the absolute path of the selected dva executable" >&2; exit 2; }
	@test -n "$(DVA_SHA256)" || { echo "ERROR: set DVA_SHA256 to the independently recorded SHA-256 of DVA_BIN" >&2; exit 2; }
	@test -n "$(FLOW_ROOT)" || { echo "ERROR: set FLOW_ROOT to an absolute flow Git repository root whose state will remain stable" >&2; exit 2; }
	go run ./tools/skilldogfood --dva-bin "$(DVA_BIN)" --expected-sha256 "$(DVA_SHA256)" --flow-root "$(FLOW_ROOT)"

## test: Run all tests (CI)
test:
	go test -race -cover ./...

## test-skill-dogfood: Run the built executable through a hermetic skill-installer round-trip (CI)
test-skill-dogfood: build
	DVA_DOGFOOD_BIN="$(abspath $(BUILD_DIR)/$(BINARY))" go test -run '^TestBuiltExecutableDogfood$$' ./tools/skilldogfood

## test-integration: Run integration tests (requires build tag) (CI)
test-integration:
	go test -tags=integration -race ./internal/integration/...

## lint: Run linters (golangci-lint v2 + gopls check, both pinned in .mise.toml)
lint: vet fmt-check
	@# A go binary that disagrees with the GOROOT mise exports makes golangci-lint fail
	@# with "could not import os/strings/..." on every cold analysis, which reads as if
	@# the tree does not compile. Measured: the same binary on the same cold cache
	@# reports 0 issues when run without the wrapper, so the failure is wrapper-specific
	@# rather than a property of the tree. TASK-204.
	@#
	@# The check must resolve go the way golangci-lint does — through PATH, in a
	@# subshell, under the wrapper. `mise exec -- go version` resolves a go passed
	@# directly as its argument through mise's own tool table rather than through the
	@# PATH it constructs, so it reports a MATCHED pair on a machine where the linter is
	@# about to fail; a check written that way could never fire. head -1 is required,
	@# not cosmetic: $$(go env GOROOT)/VERSION is two lines (the version, then a build
	@# timestamp), so comparing it whole against one-line `go version` output would fail
	@# the gate on a correctly paired machine.
	@#
	@# An unreadable pairing must fail, and with its own message. If either substitution
	@# fails it yields the empty string, and two empty strings compare EQUAL — so without
	@# the -z check below a `go` that is missing, or present but exiting non-zero, makes
	@# this guard report health having verified nothing, indistinguishable from a real
	@# match. Measured: a `#!/bin/sh exit 3` stub named go gives tool=[] root=[] rc=0.
	@# That is the failure shape of TASK-205 one level up — hiding the diagnostic rather
	@# than the result — and the same shape the gopls rc check below rejects. Note it
	@# degrades the wrong way if left alone: a PARTIAL failure makes the strings differ
	@# and fires loudly about a mismatch that does not exist, while a TOTAL failure passes
	@# in silence. The reachable case is not an exotic PATH — it is any machine where go
	@# comes only through mise, since `mise exec` reverts toward the pre-activation PATH.
	@if command -v mise >/dev/null 2>&1 && mise which golangci-lint >/dev/null 2>&1; then \
		mise exec -- sh -c 'tool=$$(go version | cut -d" " -f3); root=$$(head -1 "$$(go env GOROOT)/VERSION"); if [ -z "$$tool" ] || [ -z "$$root" ]; then echo "make lint: cannot read the go/GOROOT pairing under mise exec" >&2; echo "  go version     -> [$$tool] (empty: go did not run)" >&2; echo "  GOROOT/VERSION -> [$$root] (empty: GOROOT unset, or VERSION unreadable)" >&2; exit 1; fi; if [ "$$tool" != "$$root" ]; then echo "make lint: go and GOROOT disagree - go tool is $$tool, GOROOT holds $$root" >&2; echo "  go:     $$(command -v go)" >&2; echo "  GOROOT: $$(go env GOROOT)" >&2; echo "  Unchecked, this surfaces as could-not-import errors about stdlib packages. TASK-204." >&2; exit 1; fi' || exit 1; \
	fi
	@# golangci-lint's cache is machine-wide by default (~/Library/Caches/golangci-lint)
	@# and its entries carry the absolute paths they were analysed at. The git workflow
	@# reclaims a worktree per completed task, so those paths keep dying, and the next
	@# run in any checkout on this machine replays them. A replayed finding cannot be
	@# suppressed: //nolint is resolved by re-reading the source at report time, and the
	@# source is gone. Scoping the cache here means a reclaimed worktree takes its cache
	@# with it. The other two tools in this target cannot have the defect — go vet
	@# renders positions relative to the module it was invoked in, and gopls is handed
	@# an explicit file list found under this checkout. TASK-203.
	@# Default-if-unset, not an unconditional assignment: a caller forcing a cold run
	@# with GOLANGCI_LINT_CACHE=<dir> must not have it silently discarded and get the
	@# checkout's warm cache — that reads as a pass without having re-analysed anything.
	@# With nothing exported the path is exactly what TASK-203 set, so a reclaimed
	@# worktree still takes its cache with it. TASK-205.
	@GOLANGCI_LINT_CACHE="$${GOLANGCI_LINT_CACHE:-$(CURDIR)/tmp/golangci-lint-cache}"; export GOLANGCI_LINT_CACHE; \
	if command -v mise >/dev/null 2>&1 && mise which golangci-lint >/dev/null 2>&1; then \
		mise exec -- golangci-lint run ./...; \
	elif command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "Install golangci-lint v2 (https://golangci-lint.run/usage/install/) or run 'mise install'"; exit 1; \
	fi
	@# gopls check covers modernize-analyzer findings golangci-lint's vendored
	@# copy misses (e.g. strings.SplitN where it only catches strings.Index). TASK-130.
	@# gopls check reports findings on stdout and exits 0 even when it has some, so
	@# the finding text is what decides the verdict. The corollary is that a non-zero
	@# exit means the tool itself failed, and it fails with stdout empty (measured:
	@# exit 2, nothing on stdout, message on stderr). make runs recipes under /bin/sh
	@# without -e, so without the rc check below the assignment's failure is discarded
	@# and an unrunnable gopls reads as a clean lint. That is the exact shape TASK-130
	@# rejected option C for.
	@if command -v mise >/dev/null 2>&1 && mise which gopls >/dev/null 2>&1; then \
		gopls_cmd="mise exec -- gopls"; \
	elif command -v gopls >/dev/null 2>&1; then \
		gopls_cmd="gopls"; \
	else \
		echo "Install gopls (https://pkg.go.dev/golang.org/x/tools/gopls) or run 'mise install'"; exit 1; \
	fi; \
	findings=$$($$gopls_cmd check -severity=hint $$(find cmd internal tools -name '*.go')); rc=$$?; \
	if [ $$rc -ne 0 ]; then \
		echo "ERROR: gopls check could not run (exit $$rc); see stderr above."; exit 1; \
	fi; \
	if [ -n "$$findings" ]; then \
		echo "ERROR: gopls check found issues:"; \
		printf '%s\n' "$$findings" | sed 's/^/  /'; \
		exit 1; \
	fi

## vet: Run go vet
vet:
	go vet ./...

## fmt: Format code
fmt:
	gofmt -w -s .

## fmt-check: Verify every Go file satisfies gofmt -s (CI)
fmt-check:
	@total=$$(find . -path ./tmp -prune -o -name '*.go' -print | wc -l | tr -d ' '); \
	if [ "$$total" -eq 0 ]; then \
		echo "ERROR: fmt-check found no .go files — it would have passed vacuously"; exit 1; \
	fi; \
	bad=$$(gofmt -s -l .); \
	if [ -n "$$bad" ]; then \
		echo "ERROR: not gofmt -s formatted — run 'make fmt' and commit:"; \
		printf '%s\n' "$$bad" | sed 's/^/  /'; \
		exit 1; \
	fi; \
	echo "gofmt -s: $$total files checked, 0 unformatted"

## clean: Remove build artifacts
clean:
	rm -rf $(BUILD_DIR)
	go clean -cache
	@# go clean -cache clears GOCACHE only. Once lint has its own cache (see the lint
	@# target), that cache has no other route out through make, and per-checkout scoping
	@# does not remove the need: deleting a source file inside one checkout strands its
	@# cached findings by the same mechanism, just less often. TASK-203.
	rm -rf $(CURDIR)/tmp/golangci-lint-cache

## generate: Generate embeddable library reference from agent-mesh-flows/shared/library/
generate:
	@echo "Regenerating Go-sourced fact blocks in shared-guardrails.md..."
	@go run ./tools/libgen
	@echo "Generating embeddable files from $(WF_LIBRARY)/..."
	@# Library reference: shared rules + schema + presets + examples (read by am flows; corpus for removed_keys_test)
	@{ echo "# DO NOT EDIT — generated by 'make generate' from $(WF_LIBRARY)/"; \
	   echo ""; \
	   cat $(WF_LIBRARY)/shared-guardrails.md; \
	   echo ""; \
	   cat $(WF_LIBRARY)/shared-checklist.md; \
	   echo ""; \
	   cat $(WF_LIBRARY)/dva-schema.md; \
	   echo ""; \
	   cat $(WF_LIBRARY)/naming-presets.md; \
	   echo ""; \
	   cat $(WF_LIBRARY)/reference-examples.md; \
	} > $(GEN_LIBRARY)
	@echo "Generated: $(GEN_LIBRARY)"
	@echo "Generating platform skill artifacts from skills/..."
	@go run ./tools/skillgen

## check-generate: Verify generated files are up-to-date
check-generate:
	@set -e; \
		before=$$(git diff --binary --no-ext-diff -- $(GEN_LIBRARY) $(WF_LIBRARY)/shared-guardrails.md AGENTS.md .agents/skills claude-plugin/skills | git hash-object --stdin); \
		$(MAKE) generate; \
		after=$$(git diff --binary --no-ext-diff -- $(GEN_LIBRARY) $(WF_LIBRARY)/shared-guardrails.md AGENTS.md .agents/skills claude-plugin/skills | git hash-object --stdin); \
		[ "$$before" = "$$after" ] || { echo "ERROR: generated files are stale — run 'make generate' and commit"; exit 1; }

## doc-check: Enforce doc size limits, markdown links, CI labels and flow decision gates (TASK-090) (CI)
doc-check:
	go run ./tools/doccheck
	go run ./tools/cilabels
	go run ./tools/flowcheck

## commit-check: Hold commit subjects since the gate's baseline to the format SSOT
commit-check:
	@# Deliberately not labelled (CI) and deliberately absent from ci.yml. The check reads
	@# git history, and CI clones are routinely shallow — there the pinned baseline is
	@# simply not present and the range would resolve to zero commits, which prints
	@# identically to a clean repository. commitcheck exits 2 rather than pass in that
	@# case, so wiring it into CI would trade a real local gate for a red build that says
	@# nothing about the commits. Run it locally and before integrating.
	go run ./tools/commitcheck

## help: Show this help
help:
	@echo "Available targets:"
	@grep -E '^##' $(MAKEFILE_LIST) | sed 's/## /  /' | column -t -s ':'
