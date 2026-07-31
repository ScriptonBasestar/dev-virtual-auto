package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// stageVar is the interpolated variable these tests use. It is deliberately not named STAGE:
// Environment.MergeVars lets the OS environment win over `vars:`, so a developer with STAGE
// exported in their shell would silently change what the fixture expands to and the test
// would be measuring their shell rather than the code.
const stageVar = "DVA_TEST_STAGE"

// resetDoctorGlobals clears the two package globals the doctor checks read through.
//
// loadEnv caches into env and returns the cached pointer on every later call (root.go), so
// without this the second subtest here would interpolate with the first subtest's variables
// against the first subtest's directory — and would pass while doing it, which is the exact
// failure mode these tests exist to rule out.
func resetDoctorGlobals(t *testing.T) {
	t.Helper()
	cfg, env = nil, nil
	t.Cleanup(func() { cfg, env = nil, nil })
}

// shimFailVar makes every shim installed below exit 1 and print the variable's value on
// stderr, so a test can drive the branch where `compose config` rejects the files.
const shimFailVar = "DVA_TEST_SHIM_FAIL"

// composeShims installs an executable for each name in a fresh directory, makes that
// directory the entire PATH, and returns a reader for the argv every shim recorded.
//
// Naming more than one binary is the point rather than convenience: the defect this covers is
// that doctor ran `docker` when the config said otherwise, and that is only observable when
// docker is also present and could have answered. PATH is replaced rather than prepended so
// a real docker installed on the machine cannot decide the result.
func composeShims(t *testing.T, names ...string) func() []string {
	t.Helper()

	dir := t.TempDir()
	logPath := filepath.Join(dir, "invocations.log")
	if err := os.WriteFile(logPath, nil, 0o644); err != nil {
		t.Fatalf("create shim log: %v", err)
	}

	for _, n := range names {
		// ${0##*/} rather than basename, and [ rather than test: PATH is about to become
		// this directory alone, so the shim cannot call anything it does not ship. printf
		// and [ are shell builtins.
		script := "#!/bin/sh\n" +
			"printf '%s %s\\n' \"${0##*/}\" \"$*\" >> \"" + logPath + "\"\n" +
			"if [ -n \"${" + shimFailVar + ":-}\" ]; then\n" +
			"  printf '%s\\n' \"${" + shimFailVar + "}\" >&2\n" +
			"  exit 1\n" +
			"fi\n"
		if err := os.WriteFile(filepath.Join(dir, n), []byte(script), 0o755); err != nil {
			t.Fatalf("write %s shim: %v", n, err)
		}
	}

	t.Setenv("PATH", dir)

	return func() []string {
		t.Helper()
		b, err := os.ReadFile(logPath)
		if err != nil {
			t.Fatalf("read shim log: %v", err)
		}
		var lines []string
		for l := range strings.SplitSeq(string(b), "\n") {
			if strings.TrimSpace(l) != "" {
				lines = append(lines, l)
			}
		}
		return lines
	}
}

// writeDoctorComposeFixture writes a dva.yml with a single compose stack entry and loads it.
// command and files are written verbatim so a ${VAR} survives into the file unexpanded.
func writeDoctorComposeFixture(t *testing.T, dir, command string, files ...string) *config.Config {
	t.Helper()

	var b strings.Builder
	fmt.Fprintf(&b, "vars:\n  %s: dev\n\nstack:\n  db:\n    order: 10\n    default_runner: compose\n    runners:\n      compose:\n", stageVar)
	if command != "" {
		fmt.Fprintf(&b, "        command: %q\n", command)
	}
	b.WriteString("        files:\n")
	for _, f := range files {
		fmt.Fprintf(&b, "          - %q\n", f)
	}
	b.WriteString("        project_name: task119\n")

	if err := os.WriteFile(filepath.Join(dir, config.FileName), []byte(b.String()), 0o644); err != nil {
		t.Fatalf("write dva.yml: %v", err)
	}

	c, err := config.Load(dir, config.SkipVersionCheck())
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}
	return c
}

// writeComposeFile drops a minimal compose file into dir under the given name.
func writeComposeFile(t *testing.T, dir, name string) {
	t.Helper()
	body := "name: task119\nservices:\n  db:\n    image: postgres:16\n"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// TestDoctorComposeConfigRunsTheConfiguredCommand covers TASK-119's first and third defects
// together, because one invocation shows both: the binary that ran, and the file name it was
// handed. The check used to hardcode docker and to pass cc.Files through unexpanded, so a
// config saying `command: podman-compose` with `files: [compose.${VAR}.yml]` was validated by
// running docker against a file name containing a dollar sign.
func TestDoctorComposeConfigRunsTheConfiguredCommand(t *testing.T) {
	resetDoctorGlobals(t)

	dir := t.TempDir()
	writeComposeFile(t, dir, "compose.dev.yml")
	c := writeDoctorComposeFixture(t, dir, "podman-compose", "compose.${"+stageVar+"}.yml")

	invocations := composeShims(t, "docker", "podman-compose")

	results := checkComposeConfigResolves(c)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d: %+v", len(results), results)
	}
	if !results[0].Passed {
		t.Errorf("expected a pass, got %+v", results[0])
	}

	got := invocations()
	if len(got) != 1 {
		t.Fatalf("expected exactly 1 binary invocation, got %d: %v", len(got), got)
	}
	if !strings.HasPrefix(got[0], "podman-compose ") {
		t.Errorf("doctor ran the wrong binary: %q", got[0])
	}
	if !strings.Contains(got[0], "config --quiet") {
		t.Errorf("argv does not end in the config subcommand: %q", got[0])
	}

	// The interpolation half. Asserting the absence of the dollar sign as well as the
	// presence of the expanded name matters: appending the resolved path while still
	// passing the literal one would satisfy a Contains check on its own.
	if !strings.Contains(got[0], filepath.Join(dir, "compose.dev.yml")) {
		t.Errorf("argv does not name the expanded compose file: %q", got[0])
	}
	if strings.Contains(got[0], "${") {
		t.Errorf("argv still carries an unexpanded variable: %q", got[0])
	}
}

// TestDoctorComposeConfigReportsWhatTheCommandSaid covers the branch the check exists for: the
// files do not resolve. It is here because the whole point of routing through ComposeArgv is
// that this diagnosis now comes from the tool the user actually runs, so the message quoted
// back has to be that tool's, not a stand-in's.
func TestDoctorComposeConfigReportsWhatTheCommandSaid(t *testing.T) {
	resetDoctorGlobals(t)

	dir := t.TempDir()
	writeComposeFile(t, dir, "compose.dev.yml")
	c := writeDoctorComposeFixture(t, dir, "podman-compose", "compose.${"+stageVar+"}.yml")

	composeShims(t, "docker", "podman-compose")
	t.Setenv(shimFailVar, "include: ./missing.yml not found\nsecond line")

	results := checkComposeConfigResolves(c)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d: %+v", len(results), results)
	}
	if results[0].Passed {
		t.Errorf("expected a failure, got %+v", results[0])
	}
	if !strings.HasPrefix(results[0].FixHint, "include: ./missing.yml not found") {
		t.Errorf("hint does not lead with the tool's own first line: %q", results[0].FixHint)
	}
	if strings.Contains(results[0].FixHint, "second line") {
		t.Errorf("hint should carry one line, not the whole transcript: %q", results[0].FixHint)
	}
}

// TestDoctorComposeMissingBinaryIsReportedNotSkipped covers TASK-119's second defect. The old
// code returned nil when its hardcoded docker was absent, so a podman-only machine got no
// compose check and no note that one had been dropped — a check that silently does not run.
func TestDoctorComposeMissingBinaryIsReportedNotSkipped(t *testing.T) {
	resetDoctorGlobals(t)

	dir := t.TempDir()
	writeComposeFile(t, dir, "compose.dev.yml")
	c := writeDoctorComposeFixture(t, dir, "podman-compose", "compose.${"+stageVar+"}.yml")

	// Only docker is installed, and the config asks for podman-compose.
	invocations := composeShims(t, "docker")

	results := checkComposeConfigResolves(c)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d: %+v", len(results), results)
	}
	if !strings.Contains(results[0].Name, "podman-compose") {
		t.Errorf("result does not name the missing binary: %q", results[0].Name)
	}
	if !strings.Contains(results[0].Name, "skipped") {
		t.Errorf("result does not say the check was skipped: %q", results[0].Name)
	}
	if results[0].FixHint == "" {
		t.Error("skipped result carries no hint for --json consumers")
	}

	// No silent fallback to the binary that happens to be installed. A check reporting on
	// docker while the user runs podman-compose is the defect, not a degraded mode of it.
	if got := invocations(); len(got) != 0 {
		t.Errorf("expected no binary to run, got %v", got)
	}
}

// TestDoctorComposeCommandWithNoWordsFails pins the fourth item of the proposed fix: the
// error ComposeArgv returns for a command: that splits to nothing has to surface, because
// `dva doctor` is the command people run to find out what is wrong.
func TestDoctorComposeCommandWithNoWordsFails(t *testing.T) {
	resetDoctorGlobals(t)

	dir := t.TempDir()
	writeComposeFile(t, dir, "compose.dev.yml")
	c := writeDoctorComposeFixture(t, dir, "   ", "compose.${"+stageVar+"}.yml")

	invocations := composeShims(t, "docker", "podman-compose")

	results := checkComposeConfigResolves(c)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d: %+v", len(results), results)
	}
	if results[0].Passed {
		t.Errorf("expected a failure, got %+v", results[0])
	}
	if !strings.Contains(results[0].FixHint, "command:") {
		t.Errorf("hint does not point at the offending key: %q", results[0].FixHint)
	}
	if got := invocations(); len(got) != 0 {
		t.Errorf("expected no binary to run, got %v", got)
	}
}

// TestDoctorComposeFileExistenceUsesTheExpandedName covers the same interpolation gap on the
// other check in this file. checkComposeFiles was fixed alongside checkComposeConfigResolves
// because it has the same root cause — no environment threaded in — and TASK-119's criterion
// names no single check. Untouched, it reported [FAIL] against a config that runs perfectly.
func TestDoctorComposeFileExistenceUsesTheExpandedName(t *testing.T) {
	t.Run("expanded name is on disk", func(t *testing.T) {
		resetDoctorGlobals(t)

		dir := t.TempDir()
		writeComposeFile(t, dir, "compose.dev.yml")
		c := writeDoctorComposeFixture(t, dir, "", "compose.${"+stageVar+"}.yml")

		results := checkComposeFiles(c)
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d: %+v", len(results), results)
		}
		if !results[0].Passed {
			t.Errorf("expected a pass for a file that exists, got %+v", results[0])
		}
		// The written form, not the expanded one: that is the line the user has to edit.
		if !strings.Contains(results[0].Name, "${"+stageVar+"}") {
			t.Errorf("result should report the name as written, got %q", results[0].Name)
		}
	})

	t.Run("expanded name is absent", func(t *testing.T) {
		resetDoctorGlobals(t)

		dir := t.TempDir()
		// compose.dev.yml is deliberately not created. Interpolating and then finding
		// nothing must still fail, or the fix would have traded a false failure for a
		// blanket pass.
		c := writeDoctorComposeFixture(t, dir, "", "compose.${"+stageVar+"}.yml")

		results := checkComposeFiles(c)
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d: %+v", len(results), results)
		}
		if results[0].Passed {
			t.Errorf("expected a failure for a missing file, got %+v", results[0])
		}
		if results[0].FixHint == "" {
			t.Error("failing result carries no hint")
		}
	})

	t.Run("two spellings of one file collapse to one result", func(t *testing.T) {
		// The dedup key is the resolved path, so a config naming the same file twice —
		// once through a variable — reports once. Keying on the written form instead
		// would print the same missing file twice under two names.
		resetDoctorGlobals(t)

		dir := t.TempDir()
		writeComposeFile(t, dir, "compose.dev.yml")
		c := writeDoctorComposeFixture(t, dir, "", "compose.${"+stageVar+"}.yml", "compose.dev.yml")

		results := checkComposeFiles(c)
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d: %+v", len(results), results)
		}
		if !strings.Contains(results[0].Name, "${"+stageVar+"}") {
			t.Errorf("first spelling should win, got %q", results[0].Name)
		}
	})
}
