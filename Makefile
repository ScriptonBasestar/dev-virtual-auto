BINARY     := dva
MODULE     := github.com/ScriptonBasestar/dva
BUILD_DIR  := ./bin
GOFLAGS     := -trimpath

# Both destinations remain configurable for an isolated install fixture. The defaults are the
# public installation paths; callers may set LOCAL_BIN_DIR and GO_BIN_DIR without changing their
# real HOME or Go toolchain configuration. GO_BIN_DIR resolution is intentionally delayed until
# the installer runs: `make help` must not need a working Go command.
LOCAL_BIN_DIR ?= $(HOME)/.local/bin
GO_BIN_DIR ?=

# Workflow source directories (single source of truth)
WF_LIBRARY  := agent-mesh-flows/shared/library

# Generated flow and embeddable library files (output of make generate)
# library_reference.txt: DVA CLI library output and corpus for removed_keys_test
# dva_guide_template.txt: embedded by ai_docs.go (static, hand-authored)
GEN_DIR         := internal/cli
GEN_LIBRARY     := $(GEN_DIR)/library_reference.txt
WF_PUBLIC_FLOWS := agent-mesh-flows/dva-diagnose.yaml \
	agent-mesh-flows/dva-discover.yaml \
	agent-mesh-flows/dva-improve.yaml \
	agent-mesh-flows/dva-improve-guided.yaml \
	agent-mesh-flows/dva-improve-guided/00-analyze.yaml \
	agent-mesh-flows/dva-improve-guided/30-configure.yaml

.PHONY: install-hooks build install install-binary test test-integration test-skill-dogfood lint clean fmt fmt-check vet help generate check-generate doc-check commit-check release-check release-preflight release-clean release-postflight dogfood-skill-install

## build: Build the dva binary (CI)
build: generate
	$(eval VERSION := $(shell grep -E '^[[:space:]]+Version = ' internal/config/version.go | cut -d'"' -f2))
	$(eval COMMIT  := $(shell git rev-parse HEAD 2>/dev/null || echo "none"))
	$(eval BUILD_DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ))
	go build $(GOFLAGS) -ldflags '-s -w -X $(MODULE)/internal/config.Version=$(VERSION) -X $(MODULE)/internal/config.Commit=$(COMMIT) -X $(MODULE)/internal/config.BuildDate=$(BUILD_DATE)' -o $(BUILD_DIR)/$(BINARY) ./cmd/dva

## install: Atomically replace each dva destination with a verified binary
install: build
	@$(MAKE) --no-print-directory install-binary INSTALL_SOURCE="$(abspath $(BUILD_DIR)/$(BINARY))"

# install-binary deliberately has no build/generate prerequisites. `install` is the public
# entrypoint; this narrow target lets tests exercise only installation against a disposable,
# prebuilt executable without writing generated source files in the checkout.
install-binary:
	@set -eu; \
		replacement_ledger='none'; \
		rollback_ledger='none'; rollback_failed=0; \
		fail() { \
			printf '%s\n' "make install: ERROR: $$*" >&2; \
			if [ "$$replacement_ledger" = none ]; then printf '%s\n' "make install: replacement ledger: none; no destination was updated" >&2; \
			else printf '%s\n' "make install: completed replacement ledger: $$replacement_ledger" >&2; fi; \
			if [ "$$rollback_ledger" != none ]; then printf '%s\n' "make install: rollback ledger: $$rollback_ledger" >&2; fi; \
			exit 1; \
		}; \
		sha256() { \
			if command -v sha256sum >/dev/null 2>&1; then \
				hash_output=$$(sha256sum "$$1") || return $$?; digest=$${hash_output%% *}; \
			elif command -v shasum >/dev/null 2>&1; then \
				hash_output=$$(shasum -a 256 "$$1") || return $$?; digest=$${hash_output%% *}; \
			elif command -v openssl >/dev/null 2>&1; then \
				hash_output=$$(openssl dgst -sha256 "$$1") || return $$?; digest=$${hash_output##* }; \
			else return 127; fi; \
			[ $${#digest} -eq 64 ] || return 1; \
			case "$$digest" in *[!0123456789abcdef]*) return 1;; esac; \
			printf '%s\n' "$$digest"; \
		}; \
		resolve_dir() { \
			mkdir -p "$$1" || fail "cannot create destination directory $$1"; \
			(cd "$$1" && pwd -P) || fail "cannot resolve destination directory $$1"; \
		}; \
		resolve_go_bin_dir() { \
			if [ -n "$(GO_BIN_DIR)" ]; then printf '%s\n' "$(GO_BIN_DIR)"; \
			elif [ -n "$${GOBIN:-}" ]; then printf '%s\n' "$$GOBIN"; \
			else \
				configured_gobin=$$(go env GOBIN) || fail "cannot resolve Go bin directory with go env GOBIN"; \
				if [ -n "$$configured_gobin" ]; then printf '%s\n' "$$configured_gobin"; \
				else \
					configured_gopath=$$(go env GOPATH) || fail "cannot resolve Go bin directory with go env GOPATH"; \
					first_gopath=$${configured_gopath%%:*}; \
					[ -n "$$first_gopath" ] || fail "go env GOPATH returned an empty first path"; \
					printf '%s/bin\n' "$$first_gopath"; \
				fi; \
			fi; \
		}; \
		preflight_target() { \
			target="$$1"; label="$$2"; \
			if [ -d "$$target" ]; then fail "refusing $$label destination directory: $$target"; fi; \
			if [ -L "$$target" ]; then fail "refusing $$label destination symbolic link: $$target"; fi; \
			if [ -e "$$target" ] && [ ! -f "$$target" ]; then fail "refusing non-regular $$label destination: $$target"; fi; \
		}; \
		source="$(INSTALL_SOURCE)"; \
		[ -n "$$source" ] || fail "set INSTALL_SOURCE to the prebuilt dva executable"; \
		[ -f "$$source" ] || fail "built binary is missing: $$source"; \
		[ -x "$$source" ] || fail "built binary is not executable: $$source"; \
		local_dir=$$(resolve_dir "$(LOCAL_BIN_DIR)"); \
		requested_go_dir=$$(resolve_go_bin_dir); \
		go_dir=$$(resolve_dir "$$requested_go_dir"); \
		local_target="$$local_dir/$(BINARY)"; \
		go_target="$$go_dir/$(BINARY)"; \
		preflight_target "$$local_target" local; \
		preflight_target "$$go_target" Go; \
		source_sha=$$(sha256 "$$source") || fail "cannot hash built binary $$source"; \
		source_version=$$("$$source" version) || fail "built binary does not report its version"; \
		stage_local=''; stage_go=''; backup_local=''; backup_go=''; \
		preserve_backup_local=no; preserve_backup_go=no; \
		local_existed=no; go_existed=no; local_replaced=no; go_replaced=no; \
		cleanup() { \
			[ -z "$$stage_local" ] || rm -f "$$stage_local"; \
			[ -z "$$stage_go" ] || rm -f "$$stage_go"; \
			if [ "$$preserve_backup_local" = no ]; then [ -z "$$backup_local" ] || rm -f "$$backup_local"; fi; \
			if [ "$$preserve_backup_go" = no ]; then [ -z "$$backup_go" ] || rm -f "$$backup_go"; fi; \
		}; \
		trap cleanup EXIT HUP INT TERM; \
		stage_candidate() { \
			stage_dir="$$1"; stage_slot="$$2"; \
			stage_file=$$(mktemp "$$stage_dir/.dva-install.XXXXXX") || fail "cannot stage candidate in $$stage_dir"; \
			case "$$stage_slot" in local) stage_local="$$stage_file";; go) stage_go="$$stage_file";; *) fail "unknown staging ledger slot $$stage_slot";; esac; \
			cp "$$source" "$$stage_file" || fail "cannot copy candidate into $$stage_dir"; \
			chmod 755 "$$stage_file" || fail "cannot make staged candidate executable in $$stage_dir"; \
			[ -x "$$stage_file" ] || fail "staged candidate is not executable in $$stage_dir"; \
			candidate_sha=$$(sha256 "$$stage_file") || fail "cannot hash staged candidate in $$stage_dir"; \
			[ "$$candidate_sha" = "$$source_sha" ] || fail "staged candidate SHA-256 differs in $$stage_dir"; \
			candidate_version=$$("$$stage_file" version) || fail "staged candidate does not report its version in $$stage_dir"; \
			[ "$$candidate_version" = "$$source_version" ] || fail "staged candidate version differs in $$stage_dir"; \
			printf '%s\n' "make install: staged and verified $$stage_file" >&2; \
		}; \
		stage_candidate "$$local_dir" local; \
		if [ "$$go_target" = "$$local_target" ]; then \
			printf '%s\n' "make install: local and Go destinations are the same path; installing once" >&2; \
		else \
			stage_candidate "$$go_dir" go; \
		fi; \
		snapshot_target() { \
			target="$$1"; target_dir="$$2"; target_slot="$$3"; \
			if [ -e "$$target" ]; then \
				backup_file=$$(mktemp "$$target_dir/.dva-install-backup.XXXXXX") || fail "cannot stage rollback backup in $$target_dir"; \
				case "$$target_slot" in local) backup_local="$$backup_file"; local_existed=yes;; go) backup_go="$$backup_file"; go_existed=yes;; *) fail "unknown rollback ledger slot $$target_slot";; esac; \
				cp -p "$$target" "$$backup_file" || fail "cannot snapshot existing $$target_slot destination $$target"; \
				cmp -s "$$target" "$$backup_file" || fail "rollback backup bytes differ for $$target_slot destination $$target"; \
				printf '%s\n' "make install: snapshotted existing $$target_slot destination for rollback" >&2; \
			fi; \
		}; \
		snapshot_target "$$local_target" "$$local_dir" local; \
		if [ -n "$$stage_go" ]; then snapshot_target "$$go_target" "$$go_dir" go; fi; \
		append_ledger() { \
			ledger_name="$$1"; ledger_value="$$2"; \
			case "$$ledger_name" in \
				replacement) if [ "$$replacement_ledger" = none ]; then replacement_ledger="$$ledger_value"; else replacement_ledger="$$replacement_ledger, $$ledger_value"; fi;; \
				rollback) if [ "$$rollback_ledger" = none ]; then rollback_ledger="$$ledger_value"; else rollback_ledger="$$rollback_ledger, $$ledger_value"; fi;; \
				*) fail "unknown ledger $$ledger_name";; \
			esac; \
		}; \
		rollback_one() { \
			backup_file="$$1"; existed="$$2"; target="$$3"; label="$$4"; \
			if [ "$$existed" = yes ]; then \
				if mv -f "$$backup_file" "$$target"; then \
					case "$$label" in local) backup_local='';; Go) backup_go='';; esac; \
					append_ledger rollback "restored $$label destination $$target"; \
				else \
					case "$$label" in local) preserve_backup_local=yes;; Go) preserve_backup_go=yes;; esac; \
					append_ledger rollback "recovery backup retained for $$label destination: $$backup_file"; \
					printf '%s\n' "make install: rollback failed: cannot restore $$label destination $$target; recovery backup retained at $$backup_file" >&2; rollback_failed=1; \
				fi; \
			elif rm -f "$$target"; then \
				append_ledger rollback "removed newly created $$label destination $$target"; \
			else \
				printf '%s\n' "make install: rollback failed: cannot remove newly created $$label destination $$target" >&2; rollback_failed=1; \
			fi; \
		}; \
		rollback_replacements() { \
			if [ "$$go_replaced" = yes ]; then rollback_one "$$backup_go" "$$go_existed" "$$go_target" Go; fi; \
			if [ "$$local_replaced" = yes ]; then rollback_one "$$backup_local" "$$local_existed" "$$local_target" local; fi; \
		}; \
		rollback_and_fail() { \
			failure_message="$$1"; \
			rollback_replacements; \
			if [ "$$rollback_failed" -eq 0 ]; then fail "$$failure_message; rollback succeeded"; \
			else fail "$$failure_message; rollback failed"; fi; \
		}; \
		replace_candidate() { \
			stage_file="$$1"; target="$$2"; label="$$3"; \
			failed_target="$$target"; \
			if ! mv -f "$$stage_file" "$$target"; then \
				rollback_and_fail "atomic replacement failed for $$failed_target"; \
			fi; \
			case "$$label" in local) local_replaced=yes;; Go) go_replaced=yes;; *) fail "unknown replacement ledger slot $$label";; esac; \
			append_ledger replacement "$$target"; \
		}; \
		replace_candidate "$$stage_local" "$$local_target" local; stage_local=''; \
		if [ -n "$$stage_go" ]; then replace_candidate "$$stage_go" "$$go_target" Go; stage_go=''; fi; \
		verify_target() { \
			target="$$1"; label="$$2"; \
			failed_target="$$target"; \
			[ -f "$$target" ] || rollback_and_fail "$$label destination is missing after replacement: $$failed_target"; \
			[ -x "$$target" ] || rollback_and_fail "$$label destination is not executable after replacement: $$failed_target"; \
			installed_sha=$$(sha256 "$$target") || rollback_and_fail "cannot hash $$label destination after replacement: $$failed_target"; \
			[ "$$installed_sha" = "$$source_sha" ] || rollback_and_fail "$$label destination SHA-256 differs after replacement: $$failed_target"; \
			installed_version=$$("$$target" version) || rollback_and_fail "$$label destination does not report its version after replacement: $$failed_target"; \
			[ "$$installed_version" = "$$source_version" ] || rollback_and_fail "$$label destination version differs after replacement: $$failed_target"; \
			printf 'make install: verified %s destination: %s (sha256=%s)\n' "$$label" "$$target" "$$installed_sha"; \
		}; \
		verify_target "$$local_target" local; \
		if [ "$$go_target" != "$$local_target" ]; then verify_target "$$go_target" Go; fi; \
		printf '%s\n' "make install: installed version evidence:"; \
		printf '%s\n' "$$source_version"

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

## generate: Generate DVA library references and self-contained Agent Mesh flows
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
	@echo "Rendering self-contained Agent Mesh DVA flows..."
	@go run ./tools/flowgen
	@echo "Generating platform skill artifacts from skills/..."
	@go run ./tools/skillgen
	@echo "Rendering agent-runtime deny-rule coverage doc from internal/agentdeny..."
	@go run ./tools/agentdenygen

## check-generate: Verify generated files are up-to-date
check-generate:
	@set -e; \
		before=$$(git diff --binary --no-ext-diff -- $(GEN_LIBRARY) $(WF_LIBRARY)/shared-guardrails.md $(WF_PUBLIC_FLOWS) AGENTS.md .agents/skills claude-plugin/skills docs/agent-deny-rules.md | git hash-object --stdin); \
		$(MAKE) generate; \
		after=$$(git diff --binary --no-ext-diff -- $(GEN_LIBRARY) $(WF_LIBRARY)/shared-guardrails.md $(WF_PUBLIC_FLOWS) AGENTS.md .agents/skills claude-plugin/skills docs/agent-deny-rules.md | git hash-object --stdin); \
		[ "$$before" = "$$after" ] || { echo "ERROR: generated files are stale — run 'make generate' and commit"; exit 1; }

## doc-check: Enforce doc size limits, markdown links, CI labels and flow decision gates (TASK-090) (CI)
doc-check:
	go run ./tools/doccheck
	go run ./tools/cilabels
	go run ./tools/flowcheck

## commit-check: Hold commit subjects since the gate's baseline to the format SSOT (CI)
commit-check:
	@# This was deliberately absent from ci.yml, because the check reads git history and CI
	@# clones are routinely shallow — there the pinned baseline is simply not present, the
	@# range would resolve to zero commits, and commitcheck exits 2 rather than reporting a
	@# clean run. That objection was about the clone, not the check: the ci.yml job that runs
	@# this target checks out with fetch-depth: 0, so the baseline is reachable and the range
	@# is real. Do not add it to a job that checks out shallowly.
	@#
	@# CI is the backstop, not the gate. It runs after a push, and this repository's branches
	@# integrate straight to master, so by the time CI speaks the subject is already published
	@# and unfixable. The gate that can still be acted on is the commit-msg hook — see
	@# install-hooks.
	go run ./tools/commitcheck

## install-hooks: Point this clone's git hooks at .githooks (subject gate; run once per clone)
install-hooks:
	@# Not automatic, because git will not let a repository install its own hooks — that is a
	@# code-execution boundary, and it is the reason this cannot be guaranteed by checkout
	@# alone. One command per clone; worktrees created from it inherit the setting.
	git config core.hooksPath .githooks
	@echo "hooks: core.hooksPath = .githooks (commit-msg enforces the subject format SSOT)"

## release-check: Build and verify GoReleaser snapshot archives and checksums (CI)
release-check: build
	@set -eu; \
		tag=$$(git describe --tags --exact-match 2>/dev/null || true); \
		commit=$$(git rev-parse HEAD); \
		short_commit=$$(git show --format=%h HEAD --quiet); \
		version=$$(grep -E '^[[:space:]]+Version = ' internal/config/version.go | cut -d'"' -f2); \
		snapshot_version=$$(go run ./tools/releasecheck snapshot-version --tag "$$tag" --commit "$$commit" --short-commit "$$short_commit"); \
		go run ./tools/releasecheck stamping; \
		go run ./tools/releasecheck version --tag "$$tag"; \
		go run ./tools/releasecheck binary --binary ./bin/dva --commit "$$commit" --version "$$version"; \
		DVA_SNAPSHOT_VERSION="$$snapshot_version" goreleaser release --snapshot --clean; \
		go run ./tools/releasecheck artifacts --dist dist; \
		host_os=$$(go env GOOS); host_arch=$$(go env GOARCH); \
		archive="dist/dva_$${host_os}_$${host_arch}.tar.gz"; \
		tmp_dir=$$(mktemp -d); trap 'rm -rf "$$tmp_dir"' EXIT; \
		tar -xzf "$$archive" -C "$$tmp_dir"; \
		go run ./tools/releasecheck binary --binary "$$tmp_dir/dva" --commit "$$commit" --version "$$snapshot_version" --snapshot

## release-preflight: Validate a clean detached release worktree and its non-persisting GitHub publication prerequisites
release-preflight:
	@test -n "$(RELEASE_TAG)" || { echo "ERROR: set RELEASE_TAG (for example v0.1.47)" >&2; exit 2; }
	@test -n "$(RELEASE_COMMIT)" || { echo "ERROR: set RELEASE_COMMIT to the full immutable commit SHA" >&2; exit 2; }
	@test -n "$(RELEASE_NOTES)" || { echo "ERROR: set RELEASE_NOTES to the reviewed release notes path" >&2; exit 2; }
	@test -n "$(RELEASE_NOTES_SHA256)" || { echo "ERROR: set RELEASE_NOTES_SHA256 to its recorded SHA-256" >&2; exit 2; }
	@go run ./tools/releaseworkflow preflight --tag "$(RELEASE_TAG)" --commit "$(RELEASE_COMMIT)" --release-notes "$(RELEASE_NOTES)" --release-notes-sha256 "$(RELEASE_NOTES_SHA256)" --cleanup-path dist --cleanup-path bin --cleanup-path tmp

## release-postflight: Verify the final published release identity, exact assets, and local cleanup after manual publication
release-postflight:
	@test -n "$(RELEASE_TAG)" || { echo "ERROR: set RELEASE_TAG (for example v0.1.47)" >&2; exit 2; }
	@test -n "$(RELEASE_COMMIT)" || { echo "ERROR: set RELEASE_COMMIT to the full immutable commit SHA" >&2; exit 2; }
	@go run ./tools/releaseworkflow postflight --tag "$(RELEASE_TAG)" --commit "$(RELEASE_COMMIT)" --cleanup-path dist --cleanup-path bin --cleanup-path tmp

## release-clean: Remove only the local release workflow outputs before postflight verification
release-clean:
	@go run ./tools/releaseworkflow clean

## help: Show this help
help:
	@echo "Available targets:"
	@grep -E '^##' $(MAKEFILE_LIST) | sed 's/## /  /' | column -t -s ':'
