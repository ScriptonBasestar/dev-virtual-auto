package main

import (
	"strings"
	"testing"
)

func TestCIMakeTargetsOneLineRun(t *testing.T) {
	got, err := ciMakeTargets(strings.NewReader(`
      - name: Documentation
        run: make doc-check
      - name: Test
        run: make test
`))
	if err != nil {
		t.Fatal(err)
	}
	assertTargets(t, got, "doc-check", "test")
}

func TestCIMakeTargetsBlockScalar(t *testing.T) {
	// The 46m hang follow-up used `run: |` plus a make invocation. The old
	// scanner only counted a one-line `run: make`, so Documentation failed
	// while Test never ran.
	got, err := ciMakeTargets(strings.NewReader(`
      - name: Test
        timeout-minutes: 15
        run: |
          set -euo pipefail
          exec </dev/null
          make test
      - name: Integration Test
        run: make test-integration
`))
	if err != nil {
		t.Fatal(err)
	}
	assertTargets(t, got, "test", "test-integration")
}

func TestCIMakeTargetsFoldedBlock(t *testing.T) {
	got, err := ciMakeTargets(strings.NewReader(`
        run: >
          make fmt-check
`))
	if err != nil {
		t.Fatal(err)
	}
	assertTargets(t, got, "fmt-check")
}

func TestCIMakeTargetsIgnoresMakeInCommentsAndEcho(t *testing.T) {
	got, err := ciMakeTargets(strings.NewReader(`
        run: |
          # make hidden
          echo make leftover
          make commit-check
        run: make build
`))
	if err != nil {
		t.Fatal(err)
	}
	assertTargets(t, got, "commit-check", "build")
}

func TestCIMakeTargetsBlockChompingIndicator(t *testing.T) {
	got, err := ciMakeTargets(strings.NewReader(`
        run: |-
          make release-check
`))
	if err != nil {
		t.Fatal(err)
	}
	assertTargets(t, got, "release-check")
}

func assertTargets(t *testing.T, got map[string]bool, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("targets = %v, want %v", sortedKeys(got), want)
	}
	for _, name := range want {
		if !got[name] {
			t.Errorf("missing target %q in %v", name, sortedKeys(got))
		}
	}
}
