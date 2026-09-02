package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
	"github.com/ScriptonBasestar/dva/internal/lifecycle"
)

// buildPlanConfig covers every answer planBuildTargets can give:
//
//	infra    compose, so `docker compose build`
//	api      native with a build command — the field nothing executed until now
//	worker   native without one, so nothing to build and no complaint about it
//	chart    helm, which has no build field at all
//	failing  native whose build fails, for the fail-fast case
//
// api's build command is `touch built-${TAG}` rather than anything realistic because the
// marker's name and location are the assertion: the file exists only if the command ran, it
// is named built-v9 only if runners.native.env reached the shell, and it lands in
// services/api only if the entry's dir was used instead of the process's cwd.
const buildPlanConfig = `version: "0.1.44"
stack:
  infra:
    default_runner: compose
    runners:
      compose:
        files: [compose.yml]
  api:
    default_runner: native
    runners:
      native:
        dir: services/api
        build: touch built-${DVA_TEST_BUILD_TAG}
        run: ./api
        env:
          DVA_TEST_BUILD_TAG: v9
  worker:
    default_runner: native
    runners:
      native:
        run: ./worker
  chart:
    default_runner: helm
    runners:
      helm:
        chart: ./chart
        release: demo
  failing:
    default_runner: native
    runners:
      native:
        build: exit 1
        run: ./failing
plans:
  full:
    entries:
      - name: infra
        services: [db, cache]
        order: 1
      - name: api
        order: 2
      - name: worker
        order: 3
      - name: chart
        order: 4
  native_only:
    entries:
      - name: api
  nothing:
    entries:
      - name: worker
      - name: chart
  chain:
    entries:
      - name: failing
        order: 1
      - name: api
        order: 2
`

// apiMarker is where api's build command leaves its evidence.
func apiMarker(c *config.Config) string {
	return filepath.Join(c.FileDir(), "services", "api", "built-v9")
}

// buildTestConfig loads the fixture and creates what the entries reference on disk: the
// compose file ComposeArgv resolves, and api's directory, which must exist before a command
// can be run in it.
func buildTestConfig(t *testing.T) *config.Config {
	t.Helper()
	c := loadTestConfig(t, buildPlanConfig)
	prepareBuildTree(t, c.FileDir())
	return c
}

func prepareBuildTree(t *testing.T, dir string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "compose.yml"), []byte("services: {}\n"), 0o644); err != nil {
		t.Fatalf("write compose file: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "services", "api"), 0o755); err != nil {
		t.Fatalf("mkdir entry dir: %v", err)
	}
}

func buildTestEnv(c *config.Config) *config.Environment {
	return config.NewEnvironment(nil, c.FileDir(), c.FileDir())
}

func resolveBuildPlan(t *testing.T, c *config.Config, name string) *lifecycle.ExecutionPlan {
	t.Helper()
	plan, err := lifecycle.ResolvePlan(c, name, nil)
	if err != nil {
		t.Fatalf("ResolvePlan(%s): %v", name, err)
	}
	return plan
}

// TestPlanBuildTargetsSkipsEntriesWithNothingToBuild.
//
// worker and chart are the point: a native entry with no build command and a runner with no
// build concept are both "nothing to do here", not failures. Reporting them would make
// `dva build <plan>` noisy on every plan that mixes a built service with a packaged one.
func TestPlanBuildTargetsSkipsEntriesWithNothingToBuild(t *testing.T) {
	plan := resolveBuildPlan(t, buildTestConfig(t), "full")

	// The filter is only under test if what it filters actually arrived.
	if len(plan.Entries) != 4 {
		t.Fatalf("plan resolved %d entries, want 4 — the skipped runners never reached the filter", len(plan.Entries))
	}

	got := planBuildTargetNames(planBuildTargets(plan))
	want := []string{"api", "infra"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("buildable entries = %v, want %v", got, want)
	}
}

// TestRunPlanBuildExecutesTheNativeBuildCommand is the D0 closure.
//
// schema.json advertised native_runner_config.build as "Build command" and decodeRunnerNode
// decoded it, but both native→process desugar points dropped it, so the field parsed,
// validated, and did nothing. Nothing failed — that is what made it expensive: the operator
// declared a build, dva accepted it, and no build ever ran.
//
// The marker path carries three assertions at once, so a partial implementation cannot pass:
// existence proves execution, the v9 suffix proves runners.native.env was delivered, and the
// services/api location proves runners.native.dir was honoured rather than the cwd.
func TestRunPlanBuildExecutesTheNativeBuildCommand(t *testing.T) {
	c := buildTestConfig(t)
	marker := apiMarker(c)
	if _, err := os.Stat(marker); err == nil {
		t.Fatalf("%s exists before the build; the test proves nothing", marker)
	}

	var err error
	captureBothStreams(t, func() { err = runPlanBuild(c, planEnv(buildTestEnv(c)), "native_only", nil) })

	if err != nil {
		t.Fatalf("runPlanBuild failed: %v", err)
	}
	if _, statErr := os.Stat(marker); statErr != nil {
		t.Fatalf("runners.native.build produced no %s: %v — the field is still being ignored", marker, statErr)
	}
}

// TestRunPlanBuildBuildsEveryEntryAndKeepsGoing runs the whole loop against a docker shim.
//
// Two things are being proven. The compose entry gets the plan's service subset as compose's
// own filter, and the loop reaches the native entry afterwards — the second is why
// buildComposeTarget does not reuse execComposePassthroughForEntry, which ends in ExecReplace
// and would turn the dva process into docker before api was ever built.
//
// Note that the shim fixture sets forceSubprocess, so this test would pass either way; it
// pins the outcome, not the mechanism. The mechanism cannot be tested in-process, because
// the failure it guards against replaces the test binary.
func TestRunPlanBuildBuildsEveryEntryAndKeepsGoing(t *testing.T) {
	dockerArgv := composePassthroughFixtureWith(t, buildPlanConfig)
	prepareBuildTree(t, ".")
	c := mustLoadConfig()
	prepareBuildTree(t, c.FileDir())

	var err error
	captureBothStreams(t, func() { err = runPlanBuild(c, planEnv(buildTestEnv(c)), "full", nil) })

	if err != nil {
		t.Fatalf("runPlanBuild failed: %v", err)
	}
	argv := strings.Join(dockerArgv(), " ")
	if argv == "" {
		t.Fatal("docker was never invoked, so the compose entry was not built")
	}
	for _, want := range []string{"build", "db", "cache"} {
		if !strings.Contains(argv, want) {
			t.Errorf("%q missing from docker argv %q", want, argv)
		}
	}
	if _, statErr := os.Stat(apiMarker(c)); statErr != nil {
		t.Fatalf("the native entry after the compose one was never built: %v", statErr)
	}
}

// TestRunPlanBuildStopsAtTheFirstFailure: entries are built in plan order, which is start
// order, so a later entry may consume what an earlier one produced. Continuing past a failure
// would build against a tree already known to be wrong and then report whatever the last
// entry returned.
func TestRunPlanBuildStopsAtTheFirstFailure(t *testing.T) {
	c := buildTestConfig(t)

	var err error
	captureBothStreams(t, func() { err = runPlanBuild(c, planEnv(buildTestEnv(c)), "chain", nil) })

	if err == nil {
		t.Fatal("a failing build command reported success")
	}
	if !strings.Contains(err.Error(), "failing") {
		t.Errorf("the error must name the entry that failed, got: %v", err)
	}
	if _, statErr := os.Stat(apiMarker(c)); statErr == nil {
		t.Error("the entry after the failure was built anyway")
	}
}

// TestRunPlanBuildDryRunPreviewsWithoutBuilding: --dry-run on a command that shells out is
// the one invocation whose whole purpose is to say what would happen, so it has to name both
// halves — the docker argv and the native command with its directory — and run neither.
func TestRunPlanBuildDryRunPreviewsWithoutBuilding(t *testing.T) {
	setDryRun(t, true)
	c := buildTestConfig(t)

	var err error
	stderr := captureBothStreams(t, func() { err = runPlanBuild(c, planEnv(buildTestEnv(c)), "full", nil) })

	if err != nil {
		t.Fatalf("runPlanBuild failed: %v", err)
	}
	for _, want := range []string{"build", "db cache", "touch built-v9", "services/api"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("the preview omits %q:\n%s", want, stderr)
		}
	}
	if _, statErr := os.Stat(apiMarker(c)); statErr == nil {
		t.Error("--dry-run built something")
	}
}

// TestRunPlanBuildRejectsExtraArgsOnANativeEntry: runners.native.build is a string handed to
// a shell, not an argv dva can extend. Appending `--no-cache` to `go build ./cmd/api` would
// produce a command the user never wrote.
func TestRunPlanBuildRejectsExtraArgsOnANativeEntry(t *testing.T) {
	c := buildTestConfig(t)

	err := runPlanBuild(c, planEnv(buildTestEnv(c)), "full", []string{"api", "--no-cache"})

	if err == nil {
		t.Fatal("--no-cache was accepted on a native entry")
	}
	if !strings.Contains(err.Error(), "runners.native.build") {
		t.Errorf("the error must point at where such arguments belong, got: %v", err)
	}
	if _, statErr := os.Stat(apiMarker(c)); statErr == nil {
		t.Error("the entry was built despite the rejected argument")
	}
}

// TestRunPlanBuildRefusesUnroutableArgsAcrossSeveralEntries: with two things to build there
// is no entry for `--no-cache` to belong to. Sending it to both would build the compose
// entry with a flag intended for the native one, and dropping it silently is worse.
func TestRunPlanBuildRefusesUnroutableArgsAcrossSeveralEntries(t *testing.T) {
	c := buildTestConfig(t)

	err := runPlanBuild(c, planEnv(buildTestEnv(c)), "full", []string{"--no-cache"})

	if err == nil {
		t.Fatal("an argument that belongs to no entry was accepted")
	}
	for _, want := range []string{"--no-cache", "dva build full", "api", "infra"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error must show how to route it; missing %q in: %v", want, err)
		}
	}
}

// TestRunPlanBuildReportsAPlanWithNothingToBuild: silence would be indistinguishable from a
// build that succeeded, and `dva build` on a plan that cannot be built is a mistake worth
// naming rather than a no-op worth hiding.
func TestRunPlanBuildReportsAPlanWithNothingToBuild(t *testing.T) {
	c := buildTestConfig(t)

	err := runPlanBuild(c, planEnv(buildTestEnv(c)), "nothing", nil)

	if err == nil {
		t.Fatal("a plan with nothing to build reported success")
	}
	if !strings.Contains(err.Error(), "runners.native.build") {
		t.Errorf("the error must say what would make an entry buildable, got: %v", err)
	}
}

// TestPlanComposeBuildArgs pins the precedence between the plan's service subset and the
// caller's own arguments, the same way TestPlanComposeLogArgs does for logs. Building more
// services than the plan runs is the wider of the two mistakes, so an explicit selection
// replaces the subset rather than adding to it.
func TestPlanComposeBuildArgs(t *testing.T) {
	target := planBuildTarget{name: "infra", runner: "compose", services: []string{"db", "cache"}}

	for _, tc := range []struct {
		name        string
		passthrough []string
		want        string
	}{
		{"no arguments: the plan's subset is built", nil, "build db cache"},
		{"an explicit service replaces the subset", []string{"db"}, "build db"},
		{"a flag suppresses the subset too", []string{"--no-cache"}, "build --no-cache"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := strings.Join(planComposeBuildArgs(target, tc.passthrough), " "); got != tc.want {
				t.Errorf("argv = %q, want %q", got, tc.want)
			}
		})
	}
}
