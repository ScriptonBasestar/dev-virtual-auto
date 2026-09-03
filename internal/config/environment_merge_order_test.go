package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The fixtures below use a `T277_` prefix rather than short names like `A`. MergeVars gives
// the OS environment priority over the merged value, so a bare `A` exported in the shell
// running `go test` would take the assertions over silently.

// TestMergeVarsResolvesSiblingReferencesRegardlessOfMapOrder is the direct regression for
// TASK-277.
//
// Every MergeVars call site hands it a map, so the batch arrives without declaration order.
// The previous implementation interpolated while ranging that map, which made the answer a
// function of Go's randomized iteration: a key visited before the sibling it references kept
// the literal `${...}`. Only the env_file path runs a repair pass afterwards, so on the other
// call sites that literal was final — this test therefore exercises MergeVars alone, with no
// pass behind it to hide the defect.
//
// The repetition is the assertion. One run of the old code passed most of the time; what
// distinguished it was that the answer changed between runs.
func TestMergeVarsResolvesSiblingReferencesRegardlessOfMapOrder(t *testing.T) {
	batch := map[string]string{
		"T277_BASE":  "root",
		"T277_ONE":   "${T277_BASE}/1",
		"T277_TWO":   "${T277_ONE}/2",
		"T277_THREE": "${T277_TWO}/3",
		"T277_FOUR":  "${T277_THREE}/4",
		"T277_FIVE":  "${T277_FOUR}/5",
		"T277_SIX":   "${T277_FIVE}/6",
	}
	want := map[string]string{
		"T277_BASE":  "root",
		"T277_ONE":   "root/1",
		"T277_TWO":   "root/1/2",
		"T277_THREE": "root/1/2/3",
		"T277_FOUR":  "root/1/2/3/4",
		"T277_FIVE":  "root/1/2/3/4/5",
		"T277_SIX":   "root/1/2/3/4/5/6",
	}

	for i := range 200 {
		env := NewEnvironment(nil, t.TempDir(), t.TempDir())
		env.MergeVars(batch)
		for k, w := range want {
			if env.Vars[k] != w {
				t.Fatalf("run %d: %s = %q, want %q", i, k, env.Vars[k], w)
			}
		}
	}
}

// TestMergeVarsKeepsDeclarationScopeAcrossBatches pins TASK-277 semantics (A) against (B).
//
// Each MergeVars call is one batch, and a batch resolves when it merges. A later batch
// redefining the source must not reach back and rewrite a value an earlier batch already
// derived. Under (B) — merge everything, then interpolate once — T277_DERIVED would read
// "second-derived", so this test is what stops a future refactor from switching the two
// readings without noticing.
func TestMergeVarsKeepsDeclarationScopeAcrossBatches(t *testing.T) {
	env := NewEnvironment(nil, t.TempDir(), t.TempDir())

	env.MergeVars(map[string]string{"T277_SRC": "first", "T277_DERIVED": "${T277_SRC}-derived"})
	env.MergeVars(map[string]string{"T277_SRC": "second"})

	if env.Vars["T277_SRC"] != "second" {
		t.Errorf("T277_SRC = %q, want the later batch to win", env.Vars["T277_SRC"])
	}
	if env.Vars["T277_DERIVED"] != "first-derived" {
		t.Errorf("T277_DERIVED = %q, want the value in scope when it was declared", env.Vars["T277_DERIVED"])
	}
}

// TestMergeVarsSelfReferenceReadsPreMergeValue covers a key that refers to itself.
//
// Resolving a batch by dependency has to treat that as a reference to the environment the
// batch is merging into, not to the batch entry being computed. Recursing there would either
// loop or append onto its own raw text.
//
// The key is deliberately one the OS does not define. When the OS *does* define it — which
// is the case for the `PATH=${PATH}:/x` shape a dotenv file would actually write — the
// declaration is discarded before this guard is reachable; that half is
// TestMergeVarsOSEnvShadowsSelfReferentialDeclaration.
func TestMergeVarsSelfReferenceReadsPreMergeValue(t *testing.T) {
	env := NewEnvironment(nil, t.TempDir(), t.TempDir())
	env.MergeVars(map[string]string{"T277_PATH": "/usr/bin"})
	env.MergeVars(map[string]string{"T277_PATH": "${T277_PATH}:/opt/bin"})

	if got, want := env.Vars["T277_PATH"], "/usr/bin:/opt/bin"; got != want {
		t.Errorf("T277_PATH = %q, want %q", got, want)
	}
}

// TestMergeVarsMutualCycleIsDeterministic covers the one case where no resolution order is
// more correct than another. A cycle has no right answer; it must at least have the same
// wrong answer every run, or a config with a typo becomes a source of run-to-run drift.
func TestMergeVarsMutualCycleIsDeterministic(t *testing.T) {
	batch := map[string]string{
		"T277_LEFT":  "${T277_RIGHT}",
		"T277_RIGHT": "${T277_LEFT}",
	}

	first := NewEnvironment(nil, t.TempDir(), t.TempDir())
	first.MergeVars(batch)

	for i := range 200 {
		env := NewEnvironment(nil, t.TempDir(), t.TempDir())
		env.MergeVars(batch)
		for _, k := range []string{"T277_LEFT", "T277_RIGHT"} {
			if env.Vars[k] != first.Vars[k] {
				t.Fatalf("run %d: %s = %q, first run had %q", i, k, env.Vars[k], first.Vars[k])
			}
		}
	}
}

// TestMergeVarsOSEnvStillWinsOverBatch pins the precedence rule the rewrite had to carry
// through: the OS environment outranks a merged value, both when it supplies the key being
// merged and when it supplies a name that another key in the same batch references.
func TestMergeVarsOSEnvStillWinsOverBatch(t *testing.T) {
	t.Setenv("T277_FROM_OS", "os-value")

	env := NewEnvironment(nil, t.TempDir(), t.TempDir())
	env.MergeVars(map[string]string{
		"T277_FROM_OS": "config-value",
		"T277_USES_OS": "${T277_FROM_OS}/tail",
	})

	if got, want := env.Vars["T277_FROM_OS"], "os-value"; got != want {
		t.Errorf("T277_FROM_OS = %q, want %q", got, want)
	}
	if got, want := env.Vars["T277_USES_OS"], "os-value/tail"; got != want {
		t.Errorf("T277_USES_OS = %q, want %q", got, want)
	}
}

// TestLoadEnvFileMultiKeyChainIsOrderStable is the end-to-end form of the first test: a
// single dotenv file whose keys form a chain longer than the two-key case that surfaced the
// bug. It goes through LoadEnvFile so the trailing repair pass is in play, which is what
// makes it a check on the *stability* of the whole load path rather than on MergeVars alone.
func TestLoadEnvFileMultiKeyChainIsOrderStable(t *testing.T) {
	dir := t.TempDir()
	var body strings.Builder
	body.WriteString("T277_ROOT=base\n")
	want := map[string]string{"T277_ROOT": "base"}
	prev, acc := "T277_ROOT", "base"
	for i := range 8 {
		key := fmt.Sprintf("T277_LINK%d", i)
		fmt.Fprintf(&body, "%s=${%s}/%d\n", key, prev, i)
		acc = fmt.Sprintf("%s/%d", acc, i)
		want[key] = acc
		prev = key
	}
	if err := os.WriteFile(filepath.Join(dir, "chain.env"), []byte(body.String()), 0o644); err != nil {
		t.Fatal(err)
	}

	for run := range 50 {
		env := NewEnvironment(nil, dir, dir)
		if err := LoadEnvFile([]any{"chain.env"}, dir, env); err != nil {
			t.Fatalf("run %d: LoadEnvFile: %v", run, err)
		}
		for k, w := range want {
			if env.Vars[k] != w {
				t.Fatalf("run %d: %s = %q, want %q", run, k, env.Vars[k], w)
			}
		}
	}
}

// TestMergeVarsOSEnvShadowsSelfReferentialDeclaration pins pre-existing behavior that the
// rewrite had to leave alone, and that is easy to misread as something the cycle guard fixes.
//
// `PATH=${PATH}:/opt/bin` in a dotenv file does not append to PATH. MergeVars gives the OS
// environment priority over the whole entry, so a key the OS defines never has its declared
// value examined at all — the `:/opt/bin` is dropped, and always was. Without this test the
// comment on MergeVars asserting that ordering claim would go unverified.
func TestMergeVarsOSEnvShadowsSelfReferentialDeclaration(t *testing.T) {
	t.Setenv("T277_SHADOWED", "from-os")

	env := NewEnvironment(nil, t.TempDir(), t.TempDir())
	env.MergeVars(map[string]string{"T277_SHADOWED": "${T277_SHADOWED}:/opt/bin"})

	if got, want := env.Vars["T277_SHADOWED"], "from-os"; got != want {
		t.Errorf("T277_SHADOWED = %q, want %q — the OS value takes the whole entry", got, want)
	}
}
