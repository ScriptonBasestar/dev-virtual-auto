package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
	"github.com/ScriptonBasestar/dva/internal/lifecycle"
)

// logPlanConfig is a plan over a compose entry, a native one and a helm one. The helm entry
// is the point of the fixture: it has no log file and no compose project, so it must not be
// selectable — and it is the case the stack path used to answer by showing some other
// entry's logs.
const logPlanConfig = `version: "0.1.44"
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
        run: "go run ./cmd/api"
  chart:
    default_runner: helm
    runners:
      helm:
        chart: ./chart
        release: demo
plans:
  full:
    entries:
      - name: infra
        services: [db, cache]
      - name: api
      - name: chart
  solo:
    entries:
      - name: api
`

func logTestConfig(t *testing.T) *config.Config {
	t.Helper()
	c := loadTestConfig(t, logPlanConfig)
	if err := os.WriteFile(filepath.Join(c.FileDir(), "compose.yml"), []byte("services: {}\n"), 0o644); err != nil {
		t.Fatalf("write compose file: %v", err)
	}
	return c
}

func writeEntryLog(t *testing.T, c *config.Config, name, content string) {
	t.Helper()
	path := entryLogFile(c, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir logs: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write log: %v", err)
	}
}

func resolveLogPlan(t *testing.T, c *config.Config, name string) *lifecycle.ExecutionPlan {
	t.Helper()
	plan, err := lifecycle.ResolvePlan(c, name, nil)
	if err != nil {
		t.Fatalf("ResolvePlan(%s): %v", name, err)
	}
	return plan
}

// TestPlanLogTargetsSkipsRunnersWithNoReachableLogs pins the difference from the stack path.
//
// `dva stack log <name>` switched on compose/process/script and fell through to the primary
// compose project for everything else, so asking a helm entry for logs answered with another
// entry's output and said nothing about the substitution. Absence from this list is what
// turns that into the explicit error the callers below raise.
func TestPlanLogTargetsSkipsRunnersWithNoReachableLogs(t *testing.T) {
	plan := resolveLogPlan(t, logTestConfig(t), "full")

	// The filter is only being tested if the thing it filters actually arrived. Without this
	// a plan that silently dropped the helm entry during resolution would produce the same
	// two names and pass.
	if len(plan.Entries) != 3 {
		t.Fatalf("plan resolved %d entries, want 3 — the helm entry never reached the filter", len(plan.Entries))
	}

	targets := planLogTargets(plan)

	got := planLogTargetNames(targets)
	want := []string{"api", "infra"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("selectable entries = %v, want %v — helm has neither a log file nor a compose "+
			"project, so it must not be offered", got, want)
	}
}

// TestPlanLogTargetsCarriesTheResolvedRunnerConfig: routing reads the runner the plan chose,
// not the entry's declaration. An entry may declare several runners, and reading the
// declaration could route to one this plan never started.
func TestPlanLogTargetsCarriesTheResolvedRunnerConfig(t *testing.T) {
	targets := planLogTargets(resolveLogPlan(t, logTestConfig(t), "full"))

	for _, target := range targets {
		switch target.name {
		case "infra":
			if target.compose == nil {
				t.Error("the compose entry carries no compose config, so its logs cannot be reached")
			}
			if strings.Join(target.services, ",") != "db,cache" {
				t.Errorf("services = %v, want the plan's subset [db cache]", target.services)
			}
		case "api":
			if target.compose != nil {
				t.Error("the native entry was given a compose config; it has no compose project")
			}
		}
	}
}

// TestRunPlanLogsRequiresANameWhenSeveralEntriesQualify: with more than one candidate there
// is no defensible default, and picking one silently would answer a question about the plan
// with the output of one arbitrary part of it.
func TestRunPlanLogsRequiresANameWhenSeveralEntriesQualify(t *testing.T) {
	c := logTestConfig(t)
	e := config.NewEnvironment(nil, c.FileDir(), c.FileDir())

	err := runPlanLogs(c, e, "full", nil)

	if err == nil {
		t.Fatal("an ambiguous plan produced logs anyway, without saying whose")
	}
	for _, want := range []string{"dva logs full", "api", "infra"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error must name the choices the user has; missing %q in: %v", want, err)
		}
	}
	// The unreachable runner must not be offered as a choice the user can then be refused.
	if strings.Contains(err.Error(), "chart") {
		t.Errorf("the error offers %q, whose logs dva cannot reach: %v", "chart", err)
	}
}

// TestRunPlanLogsReadsTheLogFileOfANamedProcessEntry is the whole path end to end for the
// non-compose half: name resolution, target selection, and the reader lifted out of stack.go.
func TestRunPlanLogsReadsTheLogFileOfANamedProcessEntry(t *testing.T) {
	c := logTestConfig(t)
	e := config.NewEnvironment(nil, c.FileDir(), c.FileDir())
	writeEntryLog(t, c, "api", "listening on :8080\n")

	var err error
	out := captureStdout(t, func() { err = runPlanLogs(c, e, "full", []string{"api"}) })

	if err != nil {
		t.Fatalf("runPlanLogs failed: %v", err)
	}
	if !strings.Contains(out, "listening on :8080") {
		t.Errorf("the entry's log file was not printed:\n%s", out)
	}
}

// TestRunPlanLogsUsesTheSoleEntryWithoutBeingNamed: a plan with one candidate has no
// ambiguity to resolve, so requiring the name would be ceremony.
func TestRunPlanLogsUsesTheSoleEntryWithoutBeingNamed(t *testing.T) {
	c := logTestConfig(t)
	e := config.NewEnvironment(nil, c.FileDir(), c.FileDir())
	writeEntryLog(t, c, "api", "sole entry\n")

	var err error
	out := captureStdout(t, func() { err = runPlanLogs(c, e, "solo", nil) })

	if err != nil {
		t.Fatalf("runPlanLogs failed: %v", err)
	}
	if !strings.Contains(out, "sole entry") {
		t.Errorf("the plan's only log-producing entry was not read:\n%s", out)
	}
}

// TestRunPlanLogsRejectsPassthroughArgsOnAFileBackedEntry: the reader prints a fixed tail and
// cannot follow. Accepting `-f` would print 100 lines and stop, which looks like a stream
// that ended rather than a flag that was ignored — the failure mode that costs the most time
// to diagnose.
func TestRunPlanLogsRejectsPassthroughArgsOnAFileBackedEntry(t *testing.T) {
	c := logTestConfig(t)
	e := config.NewEnvironment(nil, c.FileDir(), c.FileDir())
	writeEntryLog(t, c, "api", "x\n")

	err := runPlanLogs(c, e, "full", []string{"api", "-f"})

	if err == nil {
		t.Fatal("-f was accepted and ignored on a file-backed entry")
	}
	if !strings.Contains(err.Error(), "tail -f") {
		t.Errorf("the error must point at what does work, got: %v", err)
	}
}

// TestPlanComposeLogArgs pins the precedence between the plan's service subset and the
// caller's own arguments. compose reads positionals as the service list, so these two cannot
// both be appended: doing so would widen an explicit selection, and adding the subset after
// `-f` would name services the caller left out.
func TestPlanComposeLogArgs(t *testing.T) {
	target := planLogTarget{name: "infra", runner: "compose", services: []string{"db", "cache"}}

	for _, tc := range []struct {
		name        string
		passthrough []string
		want        string
	}{
		{"no arguments: the plan's subset filters", nil, "logs db cache"},
		{"an explicit service replaces the subset", []string{"db"}, "logs db"},
		{"a flag suppresses the subset too", []string{"-f"}, "logs -f"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := strings.Join(planComposeLogArgs(target, tc.passthrough), " "); got != tc.want {
				t.Errorf("argv = %q, want %q", got, tc.want)
			}
		})
	}
}
