package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// Every fixture here declares `shell: false` and points `command:` at a binary that does not
// exist, and both halves are load-bearing.
//
// ExecReplace panics under `go test` unless it is crossing a subprocess boundary (TASK-144):
// syscall.Exec would replace the test binary and report the package `ok` whatever the
// substituted program did. The guard sits *after* exec.LookPath precisely so a command that
// cannot resolve stops at "command not found" instead, which is the seam these tests use.
//
// ShellEnabled() defaults to true (config.go:478), which would make the command line
// `sh -c <whatever>` — `sh` resolves, so execution would reach the guard and panic. With
// shell: false the sentinel name is argv[0] itself, LookPath fails, and ExecReplace returns
// `command not found: <sentinel>` without executing anything.
//
// The sentinel is distinct per key, so the error text names *which* interaction resolved.
// That is the whole claim under test. An assertion that stopped at "no error" — or at "some
// error" — would pass against a run.go that resolved the key and then ran something else.
func sentinelYAML(key, sentinel string) string {
	return fmt.Sprintf("interaction:\n  %q:\n    shell: false\n    command: %s\n", key, sentinel)
}

// runInFixture writes a dva.yml, chdirs into it, and drives runCmd.RunE with the key
// unsplit — the same entry point TASK-137's TestUnroutableKeyFailsBothInvocationForms uses,
// and for the same reason: the colon has to be split (or not) by the code under test. A
// helper that pre-split the key would pass no matter what run.go does with it.
//
// No version: key in any fixture here. It declares the minimum dva version, not a schema
// version, and a failed load is os.Exit(1) inside mustLoadConfig — which takes the test
// binary down with no output to explain it.
func runInFixture(t *testing.T, yml string, args ...string) error {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "dva.yml"), []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	// loadConfig/loadEnv memoise into the cfg/env package globals (root.go:301, :357) for the
	// life of the test binary, so without this every case after the first runs against the
	// first one's Config — pointing at a TempDir already removed by cleanup. The symptom is
	// not an error but silence, which reads as "the command did nothing".
	//
	// projectName is the --project flag's variable, and run.go consults it before deciding
	// whether to split: a leftover value from another test would route every case here to a
	// subproject and the split would never be exercised at all.
	oldCfg, oldEnv, oldProject := cfg, env, projectName
	cfg, env, projectName = nil, nil, ""
	t.Cleanup(func() { cfg, env, projectName = oldCfg, oldEnv, oldProject })

	return runCmd.RunE(runCmd, args)
}

// TestFreePrefixColonKeyRunsTheLiteralKey covers TASK-167's headline defect: run.go split
// every key on ':' before asking whether the literal key existed, so a colon key whose
// prefix named no subproject was reachable by nothing while validate called the file valid
// and the manifest advertised `usage_example: dva mytool:fast`.
func TestFreePrefixColonKeyRunsTheLiteralKey(t *testing.T) {
	for _, tc := range []struct {
		name     string
		key      string
		sentinel string
	}{
		{
			name:     "free prefix",
			key:      "mytool:fast",
			sentinel: "dva-resolved-mytool-fast",
		},
		{
			// schema.json's interaction-key pattern permits a leading colon, and
			// UnroutableNamespacePrefix guards with idx <= 0, so nothing covered this. The
			// empty prefix mangled cmdName rather than producing a subproject lookup, which
			// is why the old failure read `command "build" not recognized` instead of the
			// subproject error the other rows got.
			name:     "leading colon, no prefix at all",
			key:      ":build",
			sentinel: "dva-resolved-leading-colon",
		},
		{
			// The row that decided the task. RenameSuggestion tells the author of
			// `app:sub:cmd` to write exactly this, so before TASK-167 following DVA's own
			// advice traded a loud error for a silent one.
			name:     "the form RenameSuggestion produces",
			key:      "app-sub:cmd",
			sentinel: "dva-resolved-app-sub-cmd",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := runInFixture(t, sentinelYAML(tc.key, tc.sentinel), tc.key)
			if err == nil {
				t.Fatalf("`dva run %s` returned nil; the sentinel binary %q cannot exist, "+
					"so reaching execution at all should have failed with command not found",
					tc.key, tc.sentinel)
			}
			if !strings.Contains(err.Error(), tc.sentinel) {
				t.Errorf("`dva run %s` failed with %q; want an error naming %q, which is the "+
					"only proof the literal key resolved rather than being split into a "+
					"subproject reference", tc.key, err, tc.sentinel)
			}
		})
	}
}

// TestReservedPrefixColonKeyStaysUnroutable is the exception, and the reason plain "route
// the literal key" was not the fix.
//
// `dva config validate` rejects this config outright (reserved command conflict, rc 1).
// Routing it here would ship a file that one surface calls a hard error while another runs
// it happily. It would also make `unroutable_reason` — "prefix is a reserved DVA command" —
// a claim about a key that runs.
//
// TASK-137's TestUnroutableKeyFailsBothInvocationForms asserts the same outcome from the
// mark's side. This one asserts it from the router's, so deleting the exception fails in
// both places rather than only in the file whose task it was.
//
// The subject was `app:build` until `dva app` was removed; `compose:ps` replaces it because
// `compose` is still a reserved command. The prefix is not decoration here — the exception
// is keyed off the live reserved set, so a test written against a name DVA no longer owns
// stops testing the exception and starts testing the ordinary path. See
// TestRemovedBuiltinPrefixBecomesRoutable for where `app:build` went.
func TestReservedPrefixColonKeyStaysUnroutable(t *testing.T) {
	const sentinel = "dva-resolved-compose-ps"
	err := runInFixture(t, sentinelYAML("compose:ps", sentinel), "compose:ps")
	if err == nil {
		t.Fatal("`dva run compose:ps` succeeded; validate rejects this config, so routing it " +
			"would make the two surfaces disagree")
	}
	if strings.Contains(err.Error(), sentinel) {
		t.Fatalf("`dva run compose:ps` resolved the literal key (error names %q); the reserved "+
			"prefix is excepted precisely so it does not", sentinel)
	}
	if !strings.Contains(err.Error(), "subproject `compose` not found") {
		t.Errorf("failed with %q, want the subproject error ConflictAdvice cites", err)
	}
}

// TestRemovedBuiltinPrefixBecomesRoutable pins the consequence of taking a name out of the
// reserved set, which the restructure did to `stack`, `app`, `infra` and `clean`.
//
// `app:build` was the canonical unroutable key: `dva config validate` rejected it, and
// `dva run app:build` failed with the subproject error asserted above. Neither is true now.
// UnroutableNamespacePrefix reads the live reserved set, so removing the built-in moved this
// key out of the exception and into LiteralKeyWins — it is an ordinary interaction, and it
// runs.
//
// That is intended: the prefix names nothing DVA owns any more, and a key that DVA has no
// claim on belongs to its author. But it is a config that used to be a hard error and is now
// a working command, which is the kind of change that is discovered rather than read. Hence
// a test, not a comment. The reserved.go note on LiteralKeyWins points here.
func TestRemovedBuiltinPrefixBecomesRoutable(t *testing.T) {
	const sentinel = "dva-resolved-app-build"
	err := runInFixture(t, sentinelYAML("app:build", sentinel), "app:build")
	if err == nil {
		t.Fatal("`dva run app:build` returned nil; the sentinel binary cannot exist, so " +
			"reaching execution at all should have failed with command not found")
	}
	if strings.Contains(err.Error(), "subproject `app` not found") {
		t.Fatalf("`dva run app:build` still takes the unroutable path (%q), but `app` is no "+
			"longer a reserved command — the exception is keyed off the reserved set and "+
			"should no longer cover this key", err)
	}
	if !strings.Contains(err.Error(), sentinel) {
		t.Errorf("`dva run app:build` failed with %q; want an error naming %q, which is the "+
			"only proof the literal key resolved rather than being split into a subproject "+
			"reference", err, sentinel)
	}
}

// TestSubprojectNamespaceStillRoutes is the control. run.go:31's split exists for this
// case, and a change that fixed the literal key by breaking it would be no fix at all.
//
// The parent declares no `engine:test` key — the command lives in the child's dva.yml —
// which is exactly why literal-first leaves this path alone.
func TestSubprojectNamespaceStillRoutes(t *testing.T) {
	const sentinel = "dva-resolved-child-subproject"
	dir := t.TempDir()
	parent := "subprojects:\n  engine:\n    path: ./engine\n"
	if err := os.WriteFile(filepath.Join(dir, "dva.yml"), []byte(parent), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "engine"), 0o755); err != nil {
		t.Fatal(err)
	}
	child := sentinelYAML("test", sentinel)
	if err := os.WriteFile(filepath.Join(dir, "engine", "dva.yml"), []byte(child), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	oldCfg, oldEnv, oldProject := cfg, env, projectName
	cfg, env, projectName = nil, nil, ""
	t.Cleanup(func() { cfg, env, projectName = oldCfg, oldEnv, oldProject })

	err := runCmd.RunE(runCmd, []string{"engine:test"})
	if err == nil {
		t.Fatal("`dva run engine:test` returned nil; the child's sentinel cannot resolve, so " +
			"reaching it should have failed with command not found")
	}
	if !strings.Contains(err.Error(), sentinel) {
		t.Errorf("`dva run engine:test` failed with %q; want an error naming %q, which is the "+
			"only proof the split still reaches the subproject's own interaction", err, sentinel)
	}
}

// TestLiteralKeyWins covers the predicate directly, including the shapes the RunE tests
// above cannot reach cheaply.
func TestLiteralKeyWins(t *testing.T) {
	c := &config.Config{Interaction: map[string]*config.InteractionCommand{
		"mytool:fast": {Command: "echo free"},
		"compose:ps":  {Command: "echo reserved"},
		"app:build":   {Command: "echo formerly reserved"},
		"plain":       {Command: "echo plain"},
	}}

	for _, tc := range []struct {
		name string
		key  string
		want bool
	}{
		{"declared, free prefix", "mytool:fast", true},
		{"declared, reserved prefix", "compose:ps", false},
		// This row read `{"declared, reserved prefix", "app:build", false}` while `dva app`
		// existed. The predicate did not change; the reserved set did. Kept as its own row
		// rather than edited into the one above, because the pair is the point: the same key
		// shape answers differently on either side of the removal.
		{"declared, prefix of a removed built-in", "app:build", true},
		{"no colon at all: nothing to split, so nothing to win", "plain", false},
		{"colon but undeclared: the subproject reading is all that is left", "engine:test", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := config.LiteralKeyWins(c, tc.key); got != tc.want {
				t.Errorf("LiteralKeyWins(%q) = %v, want %v", tc.key, got, tc.want)
			}
		})
	}
}
