package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// TASK-267 item 1: buildManifestSubprojectCommands used to emit
// `usage_example: "dva <project>:<key>"` unconditionally, the same defect interactionUsage
// already exists to fix for local (non-subproject) keys. The fixture below reproduces the
// shape that exposes it — a parent declaring both a literal `engine:test` interaction key
// AND a same-named `engine` subproject whose own `test` entry the literal key shadows —
// measured against ./bin/dva before this fix:
//
//	dva engine:test               -> PARENT-LITERAL      (parent's literal key wins)
//	dva engine:build               -> CHILD-ENGINE-BUILD  (colon form reaches the child)
//	dva run --project engine test  -> CHILD-ENGINE-TEST
//
// yet the manifest's "test" entry advertised `usage_example: "dva engine:test"`, which
// provably ran the parent's command instead.

// writeShadowedSubprojectFixture writes a parent dva.yml with a literal `engine:test` key
// and an `engine` subproject whose `test`/`build` entries the literal key partially shadows,
// plus a `run` subproject — named after a reserved command, to pin D1: there is no
// unroutable state for a subproject command, so a subproject sharing a name with a built-in
// must come through unmarked. Returns the loaded parent config.
func writeShadowedSubprojectFixture(t *testing.T) *config.Config {
	t.Helper()
	tmpDir := t.TempDir()

	engineDir := filepath.Join(tmpDir, "engine")
	if err := os.MkdirAll(engineDir, 0o755); err != nil {
		t.Fatalf("create engine dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(engineDir, config.FileName), []byte(`
version: "0.1.0"
interaction:
  test:
    description: "child engine test"
    command: "echo CHILD-ENGINE-TEST"
  build:
    description: "child engine build"
    command: "echo CHILD-ENGINE-BUILD"
`), 0o644); err != nil {
		t.Fatalf("write engine dva.yml: %v", err)
	}

	runDir := filepath.Join(tmpDir, "run")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("create run dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, config.FileName), []byte(`
version: "0.1.0"
interaction:
  go:
    description: "child run go"
    command: "echo CHILD-RUN-GO"
`), 0o644); err != nil {
		t.Fatalf("write run dva.yml: %v", err)
	}

	if err := os.WriteFile(filepath.Join(tmpDir, config.FileName), []byte(`
version: "0.1.0"
interaction:
  engine:test:
    description: "parent literal"
    command: "echo PARENT-LITERAL"
subprojects:
  engine:
    path: ./engine
  run:
    path: ./run
`), 0o644); err != nil {
		t.Fatalf("write parent dva.yml: %v", err)
	}

	c, err := config.Load(tmpDir)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	return c
}

// TestManifestSubprojectShadowedKeyUsesWorkingUsage pins D1/D5: a shadowed subproject key's
// emitted usage_example must invoke the child entry, not the shadowing parent key, and the
// entry must carry shadowed_by_literal_key while carrying NEITHER unroutable NOR
// shadowed_by_builtin (D1 — a subproject command has no unroutable state, and
// ShadowedByBuiltin names a static_commands entry, which a parent interaction key is not).
func TestManifestSubprojectShadowedKeyUsesWorkingUsage(t *testing.T) {
	c := writeShadowedSubprojectFixture(t)
	m := buildManifest(c)

	entry, ok := m.Subprojects["engine"].Commands["test"]
	if !ok {
		t.Fatalf("engine subproject commands = %v, missing 'test'", m.Subprojects["engine"].Commands)
	}
	const wantUsage = "dva run --project engine test"
	if entry.UsageExample != wantUsage {
		t.Errorf("usage_example = %q, want %q — the pre-fix form (\"dva engine:test\") provably runs the parent's PARENT-LITERAL command instead", entry.UsageExample, wantUsage)
	}
	if entry.ShadowedByLiteralKey != "engine:test" {
		t.Errorf("shadowed_by_literal_key = %q, want %q", entry.ShadowedByLiteralKey, "engine:test")
	}
	if entry.Unroutable != "" {
		t.Errorf("unroutable = %q, want empty — D1: a subproject command has no unroutable state, `dva run --project engine test` always reaches it", entry.Unroutable)
	}
	if entry.ShadowedByBuiltin != "" {
		t.Errorf("shadowed_by_builtin = %q, want empty — that field names a static_commands entry, and the parent's `engine:test` interaction key is not one", entry.ShadowedByBuiltin)
	}
}

// TestManifestSubprojectUnshadowedKeyKeepsColonForm pins the counterpart: a subproject key
// the parent does NOT also declare as a literal interaction key keeps the plain
// `dva <project>:<key>` form and sets no shadow marker at all.
func TestManifestSubprojectUnshadowedKeyKeepsColonForm(t *testing.T) {
	c := writeShadowedSubprojectFixture(t)
	m := buildManifest(c)

	entry, ok := m.Subprojects["engine"].Commands["build"]
	if !ok {
		t.Fatalf("engine subproject commands = %v, missing 'build'", m.Subprojects["engine"].Commands)
	}
	const wantUsage = "dva engine:build"
	if entry.UsageExample != wantUsage {
		t.Errorf("usage_example = %q, want %q", entry.UsageExample, wantUsage)
	}
	if entry.ShadowedByLiteralKey != "" {
		t.Errorf("shadowed_by_literal_key = %q, want empty — the parent declares no literal `engine:build` key", entry.ShadowedByLiteralKey)
	}
}

// TestManifestSubprojectNamedAfterReservedCommandIsUnmarked pins D1's other half: a
// subproject whose name collides with a reserved command name (here `run`) still routes —
// measured, `dva run:go` reaches it — so it must not be marked unroutable or shadowed.
func TestManifestSubprojectNamedAfterReservedCommandIsUnmarked(t *testing.T) {
	c := writeShadowedSubprojectFixture(t)
	m := buildManifest(c)

	entry, ok := m.Subprojects["run"].Commands["go"]
	if !ok {
		t.Fatalf("run subproject commands = %v, missing 'go'", m.Subprojects["run"].Commands)
	}
	const wantUsage = "dva run:go"
	if entry.UsageExample != wantUsage {
		t.Errorf("usage_example = %q, want %q — a subproject named after a reserved command still routes", entry.UsageExample, wantUsage)
	}
	if entry.ShadowedByLiteralKey != "" || entry.Unroutable != "" || entry.ShadowedByBuiltin != "" {
		t.Errorf("entry = %+v, want no marker set", entry)
	}
}

// TestLsProjectFlag_Registered pins item 2's other requirement: `--project` must actually be
// registered on lsCmd, not just documented. Before this fix, run.go's recovery hint ("Run
// 'dva ls --project %s'") named a flag that only existed on runCmd, and `dva ls --project x`
// exited non-zero with an unknown-flag error.
func TestLsProjectFlag_Registered(t *testing.T) {
	flag := lsCmd.Flags().Lookup("project")
	if flag == nil {
		t.Fatal("lsCmd has no --project flag registered")
	}
	if flag.Shorthand != "p" {
		t.Errorf("--project shorthand = %q, want %q", flag.Shorthand, "p")
	}
}

// TestLsProject_ListsSubprojectInteractions covers runLsProject end to end: it must load the
// named subproject the same way `dva run --project` does and list its interaction keys,
// applying the same subprojectUsage marking the manifest test above pins. Matching
// printTable's own convention (see printTable's shadowedBy/unroutable marks above), the
// working usage string only surfaces inside a row's parenthetical mark when subprojectUsage
// reports a shadow — an unshadowed row prints just "<key>  # <description>" and never spells
// out "dva <project>:<key>" literally, so this only asserts the mark's presence/absence, not a
// literal usage string on every row.
func TestLsProject_ListsSubprojectInteractions(t *testing.T) {
	c := writeShadowedSubprojectFixture(t)

	oldDetailed := lsDetailed
	lsDetailed = false
	t.Cleanup(func() { lsDetailed = oldDetailed })

	var runErr error
	out := captureOutput(t, func() {
		runErr = runLsProject(c, "engine")
	})
	if runErr != nil {
		t.Fatalf("runLsProject(engine) error: %v", runErr)
	}
	const wantBuildLine = "build  # child engine build\n"
	if !strings.Contains(out, wantBuildLine) {
		t.Errorf("output = %q, want the unshadowed row unmarked: %q", out, wantBuildLine)
	}
	const wantTestLine = "test   # child engine test  (parent key 'engine:test' takes this name; run: dva run --project engine test)\n"
	if !strings.Contains(out, wantTestLine) {
		t.Errorf("output = %q, want the shadowed entry's mark naming the working usage form: %q", out, wantTestLine)
	}
}

// TestLsProject_UnknownProject pins the error shape: an unknown project must produce the
// same "subproject `%s` not found. Available: ..." message run.go already uses, because
// loadSubprojectConfig is now the single source both callers go through.
func TestLsProject_UnknownProject(t *testing.T) {
	c := &config.Config{
		Subprojects: map[string]config.SubprojectConfig{
			"engine": {Path: "./engine"},
		},
	}
	err := runLsProject(c, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent subproject")
	}
	const want = "subproject `nonexistent` not found. Available: engine"
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

// TestSubprojectUsage covers the predicate directly, including shapes the manifest-level
// tests above do not exercise cheaply.
func TestSubprojectUsage(t *testing.T) {
	parent := &config.Config{Interaction: map[string]*config.InteractionCommand{
		"engine:test": {Command: "echo parent"},
	}}

	usage, shadowed := subprojectUsage(parent, "engine", "test")
	if usage != "dva run --project engine test" || shadowed != "engine:test" {
		t.Errorf("shadowed case: usage=%q shadowed=%q", usage, shadowed)
	}

	usage, shadowed = subprojectUsage(parent, "engine", "build")
	if usage != "dva engine:build" || shadowed != "" {
		t.Errorf("unshadowed case: usage=%q shadowed=%q", usage, shadowed)
	}
}
