package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/ScriptonBasestar/dva/internal/config"
	"github.com/ScriptonBasestar/dva/internal/output"
)

// TASK-248 acceptance suite for the TASK-247 env-input contract.
//
// The fixtures are built around markers rather than around return values. "Fails
// closed" is not a claim about an error string — it is a claim that nothing ran, and
// the only way to check that is to give every hook, health check and runner a side
// effect and then assert the side effects are absent. A test that only asserted the
// error would still pass if `up` refused *after* starting the backend.

const envPolicyFrozenMessage = "environment inputs are incomplete\n  - .env: missing required file"

// envPolicyFixture builds a config whose every observable step leaves a file behind.
// The required .env is never created, so every route below is on the incomplete path.
func envPolicyFixture(t *testing.T) *config.Config {
	t.Helper()
	c := loadTestConfig(t, `version: "0.1.45"
env_file:
  files:
    - path: .env
      required: true
stack:
  svc:
    default_runner: script
    health_checks:
      ready:
        type: command
        command: touch health-ran
    runners:
      script:
        up: touch runner-up
        down: touch runner-down
        stop: touch runner-stop
        build: touch runner-build
        logs: touch runner-logs
plans:
  dev:
    entries: [{name: svc}]
interaction:
  up:
    before: [{run: touch hook-up-before}]
    after: [{run: touch hook-up-after}]
  down:
    before: [{run: touch hook-down-before}]
  logs:
    before: [{run: touch hook-logs-before}]
`)
	return c
}

// envPolicySideEffects lists every marker the fixture could have left. Reading the
// directory rather than stat-ing a known list is deliberate: a future runner that
// starts a child nobody remembered to enumerate still shows up here.
func envPolicySideEffects(t *testing.T, c *config.Config) []string {
	t.Helper()
	entries, err := os.ReadDir(c.FileDir())
	if err != nil {
		t.Fatal(err)
	}
	var found []string
	for _, e := range entries {
		if e.Name() == config.FileName {
			continue
		}
		found = append(found, e.Name())
	}
	sort.Strings(found)
	return found
}

// withEnvPolicyGlobals isolates the package-level command state these routes read.
// env in particular is a cache: leaving one test's load in place would let the next
// test resolve a report it never built.
func withEnvPolicyGlobals(t *testing.T, c *config.Config, wantJSON bool) {
	t.Helper()
	oldCfg, oldEnv, oldJSON, oldDryRun := cfg, env, jsonOutput, dryRun
	cfg, env, jsonOutput, dryRun = c, nil, wantJSON, false
	output.ResetStdoutDocument()
	t.Cleanup(func() {
		cfg, env, jsonOutput, dryRun = oldCfg, oldEnv, oldJSON, oldDryRun
		output.ResetStdoutDocument()
	})
}

// TestExecutionAndTeardownRoutesFailClosed covers the whole fail-closed half of the
// §4 route table in one table, including teardown — which is the row most worth
// pinning, because "just clean up anyway" is the intuitive behavior and it is wrong:
// a partial environment can resolve a different project name and tear down the wrong
// resources.
func TestExecutionAndTeardownRoutesFailClosed(t *testing.T) {
	for _, tt := range []struct {
		name   string
		invoke func(c *config.Config, el *envLoad) error
	}{
		{"up", func(c *config.Config, el *envLoad) error { return runPlanUp(c, el, "dev", nil) }},
		{"down", func(c *config.Config, el *envLoad) error { return runPlanDown(c, el, "dev", nil) }},
		{"stop", func(c *config.Config, el *envLoad) error { return runPlanStop(c, el, "dev", nil) }},
		{"restart", func(c *config.Config, el *envLoad) error { return runPlanRestart(c, el, "dev", nil) }},
		{"build", func(c *config.Config, el *envLoad) error { return runPlanBuild(c, el, "dev", nil) }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c := envPolicyFixture(t)
			withEnvPolicyGlobals(t, c, false)

			var err error
			captureBothStreams(t, func() { err = tt.invoke(c, rootEnvLoad(c)) })

			if err == nil {
				t.Fatalf("%s returned nil on incomplete environment inputs", tt.name)
			}
			if err.Error() != envPolicyFrozenMessage {
				t.Errorf("error =\n%s\nwant\n%s", err.Error(), envPolicyFrozenMessage)
			}
			if side := envPolicySideEffects(t, c); len(side) != 0 {
				t.Errorf("%s started something before refusing: %v", tt.name, side)
			}
		})
	}
}

// TestHooksDoNotFireOnIncompleteEnvironment pins the part hooks own. The wrapper runs
// before the command, so without a guard there a before-hook fires for an `up` that is
// about to refuse — the one ordering the command itself cannot fix.
func TestHooksDoNotFireOnIncompleteEnvironment(t *testing.T) {
	c := envPolicyFixture(t)
	withEnvPolicyGlobals(t, c, false)

	up := &cobra.Command{RunE: func(_ *cobra.Command, args []string) error {
		return runPlanUp(c, rootEnvLoad(c), "dev", args)
	}}
	wrapWithHooks("up", up)

	var err error
	captureBothStreams(t, func() { err = up.RunE(up, nil) })

	if err == nil {
		t.Fatal("wrapped up returned nil on incomplete environment inputs")
	}
	if side := envPolicySideEffects(t, c); len(side) != 0 {
		t.Errorf("hook wrapper ran something before the command refused: %v", side)
	}
}

// TestNamedPlanStatusPartialDocument pins the observation half: a document that says
// it is partial, exit 1, and — the part a snapshot would miss — no `[plan: ...]`
// header, which would otherwise announce a query that never happened.
func TestNamedPlanStatusPartialDocument(t *testing.T) {
	t.Run("human", func(t *testing.T) {
		c := envPolicyFixture(t)
		withEnvPolicyGlobals(t, c, false)

		var err error
		stdout, stderr := captureStreams(t, func() { err = runPlanStatus(c, rootEnvLoad(c), "dev") })

		if err == nil {
			t.Fatal("named plan status returned nil on incomplete environment inputs")
		}
		if want := "Plan: dev (not queried: environment inputs incomplete)\n"; stdout != want {
			t.Errorf("stdout = %q, want %q", stdout, want)
		}
		if strings.Contains(stderr, "[plan:") {
			t.Errorf("stderr announced a plan header for a query that did not happen:\n%s", stderr)
		}
		if side := envPolicySideEffects(t, c); len(side) != 0 {
			t.Errorf("status started something: %v", side)
		}
	})

	t.Run("json", func(t *testing.T) {
		c := envPolicyFixture(t)
		withEnvPolicyGlobals(t, c, true)

		var err error
		stdout, _ := captureStreams(t, func() { err = runPlanStatus(c, rootEnvLoad(c), "dev") })

		if err == nil {
			t.Fatal("named plan status returned nil on incomplete environment inputs")
		}
		doc := decodeSingleJSONDocument(t, stdout)
		if doc["action"] != "status" || doc["plan"] != "dev" {
			t.Errorf("action/plan = %v/%v, want status/dev", doc["action"], doc["plan"])
		}
		assertPartialEnvBlocks(t, doc)
		if _, ok := doc["status"]; ok {
			t.Errorf("partial document carried a runtime status field: %v", doc)
		}
	})
}

// TestNamedPlanStatusPreservesInvokedSpelling pins that an alias route reports the
// alias. The name in the diagnostic is the one the user can retype; rewriting it to
// the canonical name hands them a string their own config may not accept.
func TestNamedPlanStatusPreservesInvokedSpelling(t *testing.T) {
	root, _ := loadEnvOwnerFixture(t, false, true)
	withEnvPolicyGlobals(t, root, false)

	var err error
	stdout, _ := captureStreams(t, func() { err = runPlanStatus(root, rootEnvLoad(root), "child-dev") })

	if err == nil {
		t.Fatal("alias status returned nil on incomplete child inputs")
	}
	if !strings.Contains(stdout, "Plan: child-dev (") {
		t.Errorf("stdout = %q, want the invoked alias spelling", stdout)
	}
}

// TestWholeStackStatusPartialDocument covers the root-owned observation route. The
// config metadata stays because it is not derived from the environment; the stack
// list goes because it is.
func TestWholeStackStatusPartialDocument(t *testing.T) {
	t.Run("json", func(t *testing.T) {
		c := stackOnlyEnvPolicyFixture(t)
		withEnvPolicyGlobals(t, c, true)

		var err error
		stdout, _ := captureStreams(t, func() { err = statusCmd.RunE(statusCmd, nil) })

		if err == nil {
			t.Fatal("whole-stack status returned nil on incomplete environment inputs")
		}
		doc := decodeSingleJSONDocument(t, stdout)
		if doc["target"] != "stack" {
			t.Errorf("target = %v, want stack", doc["target"])
		}
		if doc["config_found"] != true || doc["stack_count"] != float64(1) {
			t.Errorf("config metadata did not survive: %v", doc)
		}
		if _, ok := doc["stack"]; ok {
			t.Errorf("runtime-derived stack key survived a not-queried document: %v", doc)
		}
		assertPartialEnvBlocks(t, doc)
	})

	t.Run("human", func(t *testing.T) {
		c := stackOnlyEnvPolicyFixture(t)
		withEnvPolicyGlobals(t, c, false)

		var err error
		stdout, _ := captureStreams(t, func() { err = statusCmd.RunE(statusCmd, nil) })

		if err == nil {
			t.Fatal("whole-stack status returned nil on incomplete environment inputs")
		}
		if !strings.Contains(stdout, "Lifecycle: (not queried: environment inputs incomplete)") {
			t.Errorf("stdout did not mark the lifecycle table as not queried:\n%s", stdout)
		}
		if !strings.Contains(stdout, "Config: ") {
			t.Errorf("stdout dropped config metadata that does not depend on the environment:\n%s", stdout)
		}
	})
}

// stackOnlyEnvPolicyFixture declares no plans, so `dva status` with no arguments takes
// the whole-stack route rather than resolving a default plan. The plan and stack routes
// print different documents, and a fixture that declares one plan silently tests the
// wrong one.
func stackOnlyEnvPolicyFixture(t *testing.T) *config.Config {
	t.Helper()
	return loadTestConfig(t, `version: "0.1.45"
env_file:
  files:
    - path: .env
      required: true
stack:
  svc:
    default_runner: script
    runners:
      script:
        up: touch runner-up
`)
}

// TestLogsPartialTargetsAreFixed pins the rule that keeps DVA from guessing: the
// target is the plan as invoked or the literal word `stack`, whatever argv follows.
// `dva logs api -f` must not report a target of `api` or `-f`.
func TestLogsPartialTargetsAreFixed(t *testing.T) {
	for _, tt := range []struct {
		name string
		args []string
		want string
	}{
		{"plan route", nil, "logs not queried for plan dev: environment inputs are incomplete"},
		{"plan route with trailing argv", []string{"svc", "-f"}, "logs not queried for plan dev: environment inputs are incomplete"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c := envPolicyFixture(t)
			withEnvPolicyGlobals(t, c, false)

			var err error
			stdout, _ := captureStreams(t, func() { err = runPlanLogs(c, rootEnvLoad(c), "dev", tt.args) })

			if err == nil || err.Error() != tt.want {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
			if stdout != "" {
				t.Errorf("partial logs wrote to stdout: %q", stdout)
			}
			if side := envPolicySideEffects(t, c); len(side) != 0 {
				t.Errorf("logs read or started something: %v", side)
			}
		})
	}

	t.Run("stack route", func(t *testing.T) {
		c := envPolicyFixture(t)
		withEnvPolicyGlobals(t, c, false)

		var err error
		stdout, _ := captureStreams(t, func() { err = logsCmd.RunE(logsCmd, []string{"api", "-f"}) })

		want := "logs not queried for stack: environment inputs are incomplete"
		if err == nil || err.Error() != want {
			t.Fatalf("error = %v, want %q", err, want)
		}
		if stdout != "" {
			t.Errorf("partial logs wrote to stdout: %q", stdout)
		}
	})
}

// TestDoctorReportsEnvInputsAndSkipsDependentProbes pins the diagnostic row shape and
// the advisory/strict split. Doctor is the command people run to find out why `up`
// refused, so a built-in env failure must not by itself make doctor exit non-zero —
// that would make the exit code useless for the exact case it exists to explain.
func TestDoctorReportsEnvInputsAndSkipsDependentProbes(t *testing.T) {
	c := loadTestConfig(t, `version: "0.1.45"
env_file:
  files:
    - path: .env
      required: true
stack:
  web:
    default_runner: compose
    runners:
      compose:
        files: [compose.yml]
`)
	withEnvPolicyGlobals(t, c, false)

	results := runDoctorChecks(c)

	byName := map[string]DoctorResult{}
	for _, r := range results {
		byName[r.Name] = r
	}

	envRow, ok := byName["Environment input loads: .env"]
	if !ok {
		t.Fatalf("no env-input row in %+v", results)
	}
	if envRow.Passed {
		t.Errorf("env row passed for a missing required file: %+v", envRow)
	}
	if envRow.Finding != "Environment input is UNAVAILABLE: missing required file" {
		t.Errorf("Finding = %q, want the frozen unavailable diagnostic", envRow.Finding)
	}
	if envRow.FixHint != "Fix env_file entry: .env" {
		t.Errorf("FixHint = %q, want the configured path as written", envRow.FixHint)
	}

	for _, name := range []string{
		"Compose file existence (skipped: environment input unavailable)",
		"Compose config resolves (skipped: environment input unavailable)",
	} {
		row, ok := byName[name]
		if !ok {
			t.Errorf("missing skip row %q; a check that silently does not run reads as one that passed", name)
			continue
		}
		if !row.Passed {
			t.Errorf("skip row %q failed; not running a check is not evidence of a broken file", name)
		}
	}
	// The env-dependent probes must not have produced their real rows.
	for name := range byName {
		if strings.HasPrefix(name, "Compose file exists:") {
			t.Errorf("compose existence probe ran against an environment DVA refused to use: %q", name)
		}
	}

	if err := doctorExitError(results, false); err != nil {
		t.Errorf("default doctor exit = %v, want 0 for a built-in-only env failure", err)
	}
	if err := doctorExitError(results, true); err == nil {
		t.Error("doctor --strict exit = 0, want non-zero: strict is the env availability gate")
	}
}

// TestEnvInputOwnerIsolation is the §3 contract in both directions. Each half fails
// without the other's guard, which is why they are one test: a fix that routes every
// report through the root passes the first and fails the second.
func TestEnvInputOwnerIsolation(t *testing.T) {
	t.Run("root failure does not reach an imported plan", func(t *testing.T) {
		root, child := loadEnvOwnerFixture(t, true, false)
		withEnvPolicyGlobals(t, root, false)

		var err error
		captureBothStreams(t, func() { err = runPlanUp(root, rootEnvLoad(root), "child/dev", nil) })

		if err != nil {
			t.Fatalf("imported plan blocked by the root's own env failure: %v", err)
		}
		if _, statErr := os.Stat(filepath.Join(child, "child-up")); statErr != nil {
			t.Errorf("child runner did not run: %v", statErr)
		}
	})

	t.Run("child failure does not reach a root plan", func(t *testing.T) {
		root, _ := loadEnvOwnerFixture(t, false, true)
		withEnvPolicyGlobals(t, root, false)

		var err error
		captureBothStreams(t, func() { err = runPlanUp(root, rootEnvLoad(root), "parent", nil) })

		if err != nil {
			t.Fatalf("root plan blocked by an imported child's env failure: %v", err)
		}
		if _, statErr := os.Stat(filepath.Join(root.FileDir(), "parent-up")); statErr != nil {
			t.Errorf("root runner did not run: %v", statErr)
		}
	})
}

// TestValidateDoesNotReadEnvFiles pins §4's structural-validation row. Validation
// answers "is this config well formed", which is a question about the file in front of
// it; making it depend on a file that may not exist yet would make a correct config
// unverifiable on a fresh checkout.
func TestValidateDoesNotReadEnvFiles(t *testing.T) {
	c := loadTestConfig(t, `version: "0.1.45"
env_file: .env
stack:
  web:
    default_runner: compose
    runners:
      compose:
        files: [compose.yml]
`)
	// Malformed, not missing: a missing optional file would prove nothing, since
	// skipping it is correct on every route.
	if err := os.WriteFile(filepath.Join(c.FileDir(), ".env"), []byte("not an assignment\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(c.FileDir(), "compose.yml"), []byte("services: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	withEnvPolicyGlobals(t, c, false)

	if err := c.Validate(); err != nil {
		t.Fatalf("Validate() failed because of an env file it should not have opened: %v", err)
	}
	files, _, complete := configuredComposeFiles(c)
	if !complete || len(files) != 1 {
		t.Fatalf("configuredComposeFiles = %v (complete=%v), want the one declared file resolved without env-file I/O", files, complete)
	}
}

// TestPartialOutputCarriesNoSecrets is the sentinel check. The loaded file holds a
// key and value; the failing file is what stops the route. Nothing on either stream
// may name either, and no count of what merged first may appear.
func TestPartialOutputCarriesNoSecrets(t *testing.T) {
	c := loadTestConfig(t, `version: "0.1.45"
env_file:
  files:
    - path: loaded.env
    - path: .env
      required: true
stack:
  svc:
    default_runner: script
    runners:
      script:
        up: touch runner-up
plans:
  dev:
    entries: [{name: svc}]
`)
	if err := os.WriteFile(filepath.Join(c.FileDir(), "loaded.env"), []byte("DVA_SENTINEL_KEY=dva-sentinel-value\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	withEnvPolicyGlobals(t, c, true)

	var err error
	stdout, stderr := captureStreams(t, func() { err = runPlanStatus(c, rootEnvLoad(c), "dev") })
	if err == nil {
		t.Fatal("status returned nil on incomplete environment inputs")
	}

	for stream, body := range map[string]string{"stdout": stdout, "stderr": stderr, "error": err.Error()} {
		for _, leak := range []string{"DVA_SENTINEL_KEY", "dva-sentinel-value"} {
			if strings.Contains(body, leak) {
				t.Errorf("%s leaked %q:\n%s", stream, leak, body)
			}
		}
		if strings.Contains(body, "loaded.env") {
			t.Errorf("%s named a file that loaded, which discloses how far the merge got:\n%s", stream, body)
		}
	}
}

// decodeSingleJSONDocument fails if stdout is not exactly one JSON value. Two
// concatenated documents is the failure mode `jq` hits, and it is invisible to a test
// that only unmarshals the first one.
func decodeSingleJSONDocument(t *testing.T, stdout string) map[string]any {
	t.Helper()
	dec := json.NewDecoder(strings.NewReader(stdout))
	var doc map[string]any
	if err := dec.Decode(&doc); err != nil {
		t.Fatalf("stdout is not a JSON document (%v):\n%s", err, stdout)
	}
	var extra any
	if err := dec.Decode(&extra); err == nil {
		t.Fatalf("stdout carried more than one JSON document:\n%s", stdout)
	}
	return doc
}

func assertPartialEnvBlocks(t *testing.T, doc map[string]any) {
	t.Helper()
	envBlock, _ := doc["environment"].(map[string]any)
	if envBlock == nil || envBlock["state"] != "partial" {
		t.Fatalf("environment block = %v, want state partial", doc["environment"])
	}
	failures, _ := envBlock["failures"].([]any)
	if len(failures) != 1 {
		t.Fatalf("failures = %v, want exactly the one failing declaration", envBlock["failures"])
	}
	failure, _ := failures[0].(map[string]any)
	if failure["file"] != ".env" || failure["required"] != true || failure["kind"] != "missing_required" {
		t.Errorf("failure = %v, want the configured path, required flag and kind", failure)
	}

	runtimeBlock, _ := doc["runtime"].(map[string]any)
	if runtimeBlock == nil || runtimeBlock["queried"] != false || runtimeBlock["reason"] != "environment_incomplete" {
		t.Errorf("runtime block = %v, want queried:false with a reason", doc["runtime"])
	}

	errBlock, _ := doc["error"].(map[string]any)
	if errBlock == nil || errBlock["message"] != "environment inputs are incomplete" || errBlock["exit_code"] != float64(1) {
		t.Errorf("error block = %v, want the frozen message and exit code", doc["error"])
	}
}

// loadEnvOwnerFixture builds a parent that imports a child, with the required .env
// missing on whichever side the caller names. Both sides declare the same file name
// on purpose: if ownership were resolved by filename rather than by owner, the two
// halves of the isolation test would be indistinguishable.
func loadEnvOwnerFixture(t *testing.T, breakRoot, breakChild bool) (*config.Config, string) {
	t.Helper()
	parent := t.TempDir()
	child := filepath.Join(parent, "child")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}

	write := func(dir, name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	write(parent, config.FileName, `version: "0.1.45"
env_file:
  files:
    - path: .env
      required: true
stack:
  first:
    default_runner: script
    runners:
      script:
        up: touch parent-up
plans:
  parent:
    entries: [{name: first}]
subprojects:
  child:
    path: child
    import:
      plans:
        - name: dev
          as: child-dev
`)
	write(child, config.FileName, `version: "0.1.45"
env_file:
  files:
    - path: .env
      required: true
stack:
  first:
    default_runner: script
    runners:
      script:
        up: touch child-up
plans:
  dev:
    entries: [{name: first}]
`)
	if !breakRoot {
		write(parent, ".env", "PARENT=1\n")
	}
	if !breakChild {
		write(child, ".env", "CHILD=1\n")
	}

	c, err := config.Load(parent)
	if err != nil {
		t.Fatal(err)
	}
	return c, child
}
