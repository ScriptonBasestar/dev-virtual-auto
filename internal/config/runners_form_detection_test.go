// Package config — regression tests for TASK-102.
//
// resolveRunnerPlugin, which teaches an entry declared in the modern default_runner:/runners:
// shape which plugin it is, runs only inside SortedStack and only on the copies it returns.
// Entries reached by name — FindStackEntry is a bare c.Stack read — stayed raw, so
// DetectPlugin returned "" and the typed configs were nil. Measured symptoms on 0.1.44:
// `dva stack log <modern-process-entry>` fell through to the docker compose default, and
// `dva ktl` silently dropped a runners.kubectl namespace.
//
// Every fixture below pairs the modern shape with the deprecated one. The legacy rows are not
// filler: this fix must not change how the old shape resolves, and without them a "fix" that
// simply routed everything through the runners map would pass.
package config

import (
	"os"
	"path/filepath"
	"testing"
)

// bothShapesFixture declares each plugin twice — once deprecated, once modern — so the two can
// be asserted equal rather than merely non-empty.
const bothShapesFixture = `version: "0.1.44"
stack:
  legacyproc:
    order: 1
    process:
      command: sleep 1
  modernproc:
    order: 2
    default_runner: process
    runners:
      process:
        command: sleep 1
  modernnative:
    order: 3
    default_runner: native
    runners:
      native:
        run: sleep 1
  legacyk8s:
    order: 4
    kubectl:
      namespace: my-namespace
  modernk8s:
    order: 5
    default_runner: kubectl
    runners:
      kubectl:
        namespace: my-namespace
`

func loadStackFixture(t *testing.T, body string) *Config {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "dva.yml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return cfg
}

func TestDetectPluginResolvesRunnersFormOnNameLookup(t *testing.T) {
	cfg := loadStackFixture(t, bothShapesFixture)

	cases := []struct {
		entry string
		want  string
	}{
		{"legacyproc", "process"},   // control — the deprecated shape must be unaffected
		{"modernproc", "process"},   // was "" before the fix
		{"modernnative", "process"}, // runners.native aliases to the process plugin
		{"legacyk8s", "kubectl"},    // control
		{"modernk8s", "kubectl"},    // was ""
	}
	for _, tc := range cases {
		t.Run(tc.entry, func(t *testing.T) {
			e := cfg.FindStackEntry(tc.entry)
			if e == nil {
				t.Fatalf("FindStackEntry(%q) = nil", tc.entry)
			}
			if got := e.DetectPlugin(); got != tc.want {
				t.Errorf("DetectPlugin() = %q, want %q — an entry reached by name must resolve "+
					"like the same entry reached through SortedStack", got, tc.want)
			}
		})
	}
}

// TestNameLookupAgreesWithSortedStack states the underlying invariant rather than a list of
// cases, so an entry shape added later is covered without editing this file. SortedStack is the
// reference because it is the path that was always correct — the orchestrator uses it.
func TestNameLookupAgreesWithSortedStack(t *testing.T) {
	cfg := loadStackFixture(t, bothShapesFixture)

	sorted := cfg.SortedStack()
	if len(sorted) == 0 {
		t.Fatal("control failed: SortedStack returned nothing, so this test asserts nothing")
	}
	t.Logf("comparing %d entries", len(sorted))

	for _, resolved := range sorted {
		want := resolved.DetectPlugin()
		if want == "" {
			t.Errorf("%s: SortedStack itself resolved nothing — the reference is broken", resolved.Name)
			continue
		}
		byName := cfg.FindStackEntry(resolved.Name)
		if byName == nil {
			t.Errorf("%s: in SortedStack but not findable by name", resolved.Name)
			continue
		}
		if got := byName.DetectPlugin(); got != want {
			t.Errorf("%s: DetectPlugin() = %q by name but %q via SortedStack — the two access "+
				"paths disagree, which is the whole defect", resolved.Name, got, want)
		}
	}
}

// TestKubectlConfigReadsBothShapes covers the accessor and the two helpers that used to test
// e.Kubectl directly. PrimaryKubectlConfig and KubectlEntries are what `dva ktl` consults, so
// their blindness is what dropped the namespace.
func TestKubectlConfigReadsBothShapes(t *testing.T) {
	cfg := loadStackFixture(t, bothShapesFixture)

	t.Run("accessor resolves the runners shape", func(t *testing.T) {
		for _, name := range []string{"legacyk8s", "modernk8s"} {
			kc := cfg.FindStackEntry(name).KubectlConfig()
			if kc == nil {
				t.Errorf("%s: KubectlConfig() = nil", name)
				continue
			}
			if kc.Namespace != "my-namespace" {
				t.Errorf("%s: namespace = %q, want %q", name, kc.Namespace, "my-namespace")
			}
		}
	})

	t.Run("accessor is nil for a non-kubectl entry", func(t *testing.T) {
		// The negative control. Without it, an accessor that returned a zero-value config for
		// everything would satisfy the subtest above.
		if kc := cfg.FindStackEntry("modernproc").KubectlConfig(); kc != nil {
			t.Errorf("KubectlConfig() = %+v for a process entry, want nil", kc)
		}
	})

	t.Run("KubectlEntries finds both shapes", func(t *testing.T) {
		var names []string
		for _, e := range cfg.KubectlEntries() {
			names = append(names, e.Name)
		}
		t.Logf("KubectlEntries: %v", names)
		if len(names) != 2 {
			t.Fatalf("KubectlEntries() = %v, want both legacyk8s and modernk8s", names)
		}
	})

	t.Run("PrimaryKubectlConfig resolves", func(t *testing.T) {
		kc := cfg.PrimaryKubectlConfig()
		if kc == nil {
			t.Fatal("PrimaryKubectlConfig() = nil")
		}
		if kc.Namespace != "my-namespace" {
			t.Errorf("namespace = %q, want %q", kc.Namespace, "my-namespace")
		}
	})
}

// TestRunnersOnlyEntryStillLoads guards the shape with no default_runner: a single runner is
// unambiguous, so runnerPluginName resolves it from the sole key.
func TestRunnersOnlyEntryStillLoads(t *testing.T) {
	cfg := loadStackFixture(t, `version: "0.1.44"
stack:
  solo:
    order: 1
    runners:
      kubectl:
        namespace: solo-ns
`)
	e := cfg.FindStackEntry("solo")
	if e == nil {
		t.Fatal("FindStackEntry(solo) = nil")
	}
	if got := e.DetectPlugin(); got != "kubectl" {
		t.Errorf("DetectPlugin() = %q, want kubectl", got)
	}
	if kc := e.KubectlConfig(); kc == nil || kc.Namespace != "solo-ns" {
		t.Errorf("KubectlConfig() = %+v, want namespace solo-ns", kc)
	}
}
