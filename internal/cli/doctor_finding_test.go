package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// writeGitignore puts content in a fresh dir and returns the dir, for driving
// checkGitignoreStatus down both branches.
func writeGitignore(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(content), 0o644); err != nil {
		t.Fatalf("write .gitignore: %v", err)
	}
	return dir
}

// TestDoctorFailRow_CannotStateTheOppositeOfWhatHappened is the guard TASK-139 exists for.
//
// Every check here renders on both outcomes, so its Name has to be assertion-shaped for the
// [pass] row to read as English. The property under test is that the FAIL row does not reuse
// that sentence: printing "Docker daemon accessible" after [FAIL] states the desired state as
// though it had been observed, leaving a four-character tag as the only carrier of the
// negation. Comparing the failing line against the *passing* row's Name is what makes this
// non-vacuous — a fix that only reworded Name, or that set Finding to a copy of Name, fails
// here rather than passing on the strength of some substring.
func TestDoctorFailRow_CannotStateTheOppositeOfWhatHappened(t *testing.T) {
	for _, tt := range []struct {
		name string
		pass func(t *testing.T) DoctorResult
		fail func(t *testing.T) DoctorResult
	}{
		{
			name: "gitignore",
			pass: func(t *testing.T) DoctorResult {
				return checkGitignoreStatus(writeGitignore(t, config.DotDirName+"/\n"))
			},
			fail: func(t *testing.T) DoctorResult {
				return checkGitignoreStatus(writeGitignore(t, "node_modules/\n"))
			},
		},
		{
			name: "gitignore absent entirely",
			pass: func(t *testing.T) DoctorResult {
				return checkGitignoreStatus(writeGitignore(t, config.DotDirName+"/\n"))
			},
			fail: func(t *testing.T) DoctorResult {
				return checkGitignoreStatus(t.TempDir())
			},
		},
		{
			name: "file_exists (the path the built-in devcontainer check shares with user checks)",
			pass: func(t *testing.T) DoctorResult {
				dir := t.TempDir()
				if err := os.WriteFile(filepath.Join(dir, "here.json"), []byte("{}"), 0o644); err != nil {
					t.Fatal(err)
				}
				return runSingleCheck(config.DoctorCheck{
					Name: "here.json exists", Type: "file_exists", Path: "here.json",
				}, dir)
			},
			fail: func(t *testing.T) DoctorResult {
				return runSingleCheck(config.DoctorCheck{
					Name: "here.json exists", Type: "file_exists", Path: "here.json",
				}, t.TempDir())
			},
		},
		{
			name: "compose file",
			pass: func(t *testing.T) DoctorResult {
				c := composeFileConfig(t, true)
				return onlyResult(t, checkComposeFiles(c))
			},
			fail: func(t *testing.T) DoctorResult {
				c := composeFileConfig(t, false)
				return onlyResult(t, checkComposeFiles(c))
			},
		},
		{
			name: "env file",
			pass: func(t *testing.T) DoctorResult {
				c := envFileConfig(t, true)
				return onlyResult(t, checkEnvFiles(c))
			},
			fail: func(t *testing.T) DoctorResult {
				c := envFileConfig(t, false)
				return onlyResult(t, checkEnvFiles(c))
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			pass := tt.pass(t)
			fail := tt.fail(t)

			if !pass.Passed {
				t.Fatalf("the pass fixture did not pass, so this case proves nothing: %+v", pass)
			}
			if fail.Passed {
				t.Fatalf("the fail fixture did not fail, so this case proves nothing: %+v", fail)
			}

			if line := fail.failureLine(); line == pass.Name {
				t.Errorf("[FAIL] %s\n  is the same sentence the passing row prints; only the tag carries the negation", line)
			}
			if fail.Finding == "" {
				t.Errorf("a check that also renders a passing row must record what it observed; got Finding=\"\" for %+v", fail)
			}
			if fail.Finding == fail.Name {
				t.Errorf("Finding is a copy of Name (%q) — it has to state the observation, not the assertion", fail.Name)
			}
			// The passing row must stay clean: a finding describes something wrong, and one
			// left on a passing row would reach --json consumers as a contradiction.
			if pass.Finding != "" {
				t.Errorf("passing row carries Finding=%q", pass.Finding)
			}
		})
	}
}

// TestDoctorCheckNameIsStableAcrossOutcomes pins the second defect, which is invisible from
// the human output: checkComposeProjectNameAlignment used to return the observation *as* its
// Name on the failure path, so one check reported itself under two different "name" values in
// --json and a consumer could not correlate a failing run with a passing one.
func TestDoctorCheckNameIsStableAcrossOutcomes(t *testing.T) {
	pass := checkComposeProjectNameAlignment(composeProjectNameConfig(t, "demo"))
	fail := checkComposeProjectNameAlignment(composeProjectNameConfig(t, ""))

	if !pass.Passed || fail.Passed {
		t.Fatalf("fixtures did not straddle the branch: pass=%+v fail=%+v", pass, fail)
	}
	if pass.Name != fail.Name {
		t.Errorf("same check, two JSON names: pass %q vs fail %q", pass.Name, fail.Name)
	}
	if fail.Finding == "" {
		t.Error("the failing row lost the observation that used to live in Name")
	}
	if !strings.Contains(fail.failureLine(), "compose.yml") {
		t.Errorf("the failing line no longer names the offending file: %q", fail.failureLine())
	}
}

func TestPrintDoctorResults_FailRowPrintsFindingAndPassRowPrintsName(t *testing.T) {
	results := []DoctorResult{
		{Name: "Docker daemon accessible", Passed: true},
		{
			Name:    ".sb/dva/ is ignored in .gitignore",
			Finding: ".sb/dva/ is NOT ignored in .gitignore",
			Passed:  false,
			FixHint: "Add '.sb/dva/' to .gitignore",
		},
	}
	out := captureStdout(t, func() { printDoctorResults(results) })

	if !strings.Contains(out, "[FAIL] .sb/dva/ is NOT ignored in .gitignore") {
		t.Errorf("FAIL row does not print the finding:\n%s", out)
	}
	if strings.Contains(out, "[FAIL] .sb/dva/ is ignored in .gitignore") {
		t.Errorf("FAIL row still prints the assertion, which reads as its own opposite:\n%s", out)
	}
	if !strings.Contains(out, "[pass] Docker daemon accessible") {
		t.Errorf("pass row should print Name unchanged:\n%s", out)
	}
}

// TestPrintDoctorResults_FailRowFallsBackToName covers the rows that only ever render on
// failure — one per offending app or subproject. Their Name is already the observation, so
// requiring a Finding of them would be duplication; the fallback is what lets them stay.
func TestPrintDoctorResults_FailRowFallsBackToName(t *testing.T) {
	results := []DoctorResult{
		{Name: `App "web" port 3000 held by a process dva did not start`, Passed: false},
	}
	out := captureStdout(t, func() { printDoctorResults(results) })
	if !strings.Contains(out, `[FAIL] App "web" port 3000 held by a process dva did not start`) {
		t.Errorf("fail-only row lost its name:\n%s", out)
	}
}

func TestRunSingleCheck_FindingPerCheckType(t *testing.T) {
	dir := t.TempDir()
	for _, tt := range []struct {
		name  string
		check config.DoctorCheck
		want  string
	}{
		{"file_exists", config.DoctorCheck{Name: "x exists", Type: "file_exists", Path: "nope.txt"}, "no file at nope.txt"},
		{"command", config.DoctorCheck{Name: "tool works", Type: "command", Command: "exit 3"}, "command exited non-zero: exit 3"},
		{"unknown type verifies nothing", config.DoctorCheck{Name: "mystery", Type: "wat"}, "nothing was verified"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := runSingleCheck(tt.check, dir)
			if r.Passed {
				t.Fatalf("expected failure: %+v", r)
			}
			if !strings.Contains(r.Finding, tt.want) {
				t.Errorf("Finding=%q, want it to contain %q", r.Finding, tt.want)
			}
		})
	}
}

func TestRunSingleCheck_PassingCheckCarriesNoFinding(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ok.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := runSingleCheck(config.DoctorCheck{Name: "ok", Type: "file_exists", Path: "ok.txt"}, dir)
	if !r.Passed {
		t.Fatalf("fixture should pass: %+v", r)
	}
	if r.Finding != "" {
		t.Errorf("passing check carries Finding=%q", r.Finding)
	}
}

// TestApplyDoctorFixes_ClearsTheFinding: once --fix repairs the state, the finding describes
// something that is no longer true. Leaving it would emit passed:true beside a finding saying
// the opposite — the exact contradiction the field was added to remove.
func TestApplyDoctorFixes_ClearsTheFinding(t *testing.T) {
	results := []DoctorResult{{
		Name:    ".sb/dva/ is ignored in .gitignore",
		Finding: ".sb/dva/ is NOT ignored in .gitignore",
		Passed:  false,
		Fixable: true,
		fixFunc: func() error { return nil },
	}}
	applyDoctorFixes(results)

	if !results[0].Fixed || !results[0].Passed {
		t.Fatalf("fixture did not go through the fix path: %+v", results[0])
	}
	if results[0].Finding != "" {
		t.Errorf("fixed row still carries Finding=%q", results[0].Finding)
	}
}

// TestDoctorResultJSON_KeysUnchangedAndFindingOmittedWhenEmpty pins the --json surface, whose
// audience is a program (internal/cli/CLAUDE.md). "finding" is additive: every key that
// existed still exists, and a passing row serialises exactly as it did before.
func TestDoctorResultJSON_KeysUnchangedAndFindingOmittedWhenEmpty(t *testing.T) {
	passing, err := json.Marshal(DoctorResult{Name: "Docker daemon accessible", Passed: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := string(passing); got != `{"name":"Docker daemon accessible","passed":true}` {
		t.Errorf("passing row changed shape: %s", got)
	}

	failing, err := json.Marshal(DoctorResult{
		Name:    "Docker daemon accessible",
		Finding: "Docker daemon is NOT accessible ('docker info' failed)",
		Passed:  false,
		FixHint: "Start Docker Desktop",
	})
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(failing, &decoded); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"name", "passed", "fix_hint", "finding"} {
		if _, ok := decoded[k]; !ok {
			t.Errorf("failing row is missing key %q: %s", k, failing)
		}
	}
	if decoded["name"] != "Docker daemon accessible" {
		t.Errorf(`"name" must stay the stable identity, got %v`, decoded["name"])
	}
}

// TestGitignoreWarningSuppressedFor covers the double-report: doctor emits the gitignore
// state as its own row, so the stderr banner made it the one finding reported twice.
func TestGitignoreWarningSuppressedFor(t *testing.T) {
	if !gitignoreWarningSuppressedFor("doctor") {
		t.Error("doctor reports this finding itself; the banner duplicates it")
	}
	for _, cmd := range []string{"up", "run", "validate", "stack", ""} {
		if gitignoreWarningSuppressedFor(cmd) {
			t.Errorf("%q does not report the gitignore state, so it still needs the banner", cmd)
		}
	}
}

// --- fixtures -------------------------------------------------------------------------

func onlyResult(t *testing.T, results []DoctorResult) DoctorResult {
	t.Helper()
	if len(results) != 1 {
		t.Fatalf("expected exactly one result, got %d: %+v", len(results), results)
	}
	return results[0]
}

func composeFileConfig(t *testing.T, present bool) *config.Config {
	t.Helper()
	c := loadTestConfig(t, `version: "0.1.22"
stack:
  infra:
    default_runner: compose
    runners:
      compose:
        files: [compose.yml]
`)
	if present {
		if err := os.WriteFile(filepath.Join(c.FileDir(), "compose.yml"), []byte("services: {}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return c
}

func envFileConfig(t *testing.T, present bool) *config.Config {
	t.Helper()
	c := loadTestConfig(t, `version: "0.1.22"
env_file: .env.local
`)
	if present {
		if err := os.WriteFile(filepath.Join(c.FileDir(), ".env.local"), []byte("A=1\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return c
}

// composeProjectNameConfig writes a compose.yml whose `name:` is composeName ("" omits the
// key, which is the condition ValidateComposeProjectNames warns about) against a dva.yml
// declaring project_name: demo.
func composeProjectNameConfig(t *testing.T, composeName string) *config.Config {
	t.Helper()
	c := loadTestConfig(t, `version: "0.1.22"
stack:
  infra:
    default_runner: compose
    runners:
      compose:
        project_name: demo
        files: [compose.yml]
`)
	body := "services:\n  app:\n    image: nginx\n"
	if composeName != "" {
		body = "name: " + composeName + "\n" + body
	}
	if err := os.WriteFile(filepath.Join(c.FileDir(), "compose.yml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return c
}
