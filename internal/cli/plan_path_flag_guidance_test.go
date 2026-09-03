package cli

import (
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// taggedPlanStackConfig has both paths reachable from one file: two tagged stack entries and a
// single plan, so `dva up` routes to the plan and `dva up --tag app` is the invocation the
// suppressed-default-plan guard answers. modes/environments are declared because --mode and
// --env resolve against those sections, and a fixture without them can only ever show them
// failing.
const taggedPlanStackConfig = `version: "0.1.0"
modes:
  native:
    description: run natively
environments:
  dev:
    description: development
stack:
  web:
    default_runner: script
    tags: [app]
    runners:
      script:
        up: echo WEB-UP
        down: echo WEB-DOWN
        stop: echo WEB-STOP
  db:
    default_runner: script
    tags: [infra]
    runners:
      script:
        up: echo DB-UP
        down: echo DB-DOWN
        stop: echo DB-STOP
plans:
  p1:
    entries:
      - name: web
      - name: db
`

// taggedStackOnlyConfig is the same stack with no plans:, so every invocation takes the
// whole-stack path and the four selectors are the only thing under test.
const taggedStackOnlyConfig = `version: "0.1.0"
modes:
  native:
    description: run natively
environments:
  dev:
    description: development
stack:
  web:
    default_runner: script
    tags: [app]
    runners:
      script:
        up: echo WEB-UP
  db:
    default_runner: script
    tags: [infra]
    runners:
      script:
        up: echo DB-UP
`

// stackPathOnlyCases are the four selectors, written the way a user writes them.
var stackPathOnlyCases = []struct {
	name string
	args []string
}{
	{"tag", []string{"--tag", "app"}},
	{"tag-inline", []string{"--tag=app"}},
	{"tag-short", []string{"-T", "app"}},
	{"exclude-tag", []string{"--exclude-tag", "infra"}},
	{"mode", []string{"--mode", "native"}},
	{"mode-short", []string{"-M", "native"}},
	{"env", []string{"--env", "dev"}},
	{"env-short", []string{"-E", "dev"}},
}

var planAwareLifecycleCommands = map[string]*cobra.Command{
	"up":      upCmd,
	"down":    downCmd,
	"stop":    stopCmd,
	"restart": restartCmd,
}

// withDryRun runs fn with the global dry-run flag set, so the fixtures' script runners print
// rather than execute.
func withDryRun(t *testing.T, fn func()) {
	t.Helper()
	old := dryRun
	dryRun = true
	defer func() { dryRun = old }()
	fn()
}

// suggestedArgs pulls the command the guard tells the user to run back out of its message and
// returns it split for a second RunE call.
//
// Re-running the printed text is the whole point: an assertion on the wording would have
// passed for the message this card was filed against, which named the plan correctly and
// proposed an invocation the plan path rejects. The suggestion is the last ": dva " clause so
// that a message containing an earlier command name cannot be mistaken for it.
func suggestedArgs(t *testing.T, command, msg string) []string {
	t.Helper()
	i := strings.LastIndex(msg, ": dva ")
	if i < 0 {
		t.Fatalf("guard message proposes no command to run: %q", msg)
	}
	fields := strings.Fields(msg[i+len(": dva "):])
	if len(fields) == 0 || fields[0] != command {
		t.Fatalf("suggestion %q is not a %q invocation (from %q)", fields, command, msg)
	}
	return fields[1:]
}

// TestSuppressedDefaultPlanSuggestionRuns follows the guard's advice literally, for all four
// selectors and every command that prints it.
//
// Before TASK-273 the guard echoed the user's args untouched, so `dva up --tag app` — a form
// that works whenever no plan is declared — was answered with `dva up p1 --tag app`, and the
// plan path answers that with `unsupported plan flag: --tag`. Following the advice was what
// broke the invocation, which is a worse failure than no advice at all.
func TestSuppressedDefaultPlanSuggestionRuns(t *testing.T) {
	for command, cmd := range planAwareLifecycleCommands {
		for _, tc := range stackPathOnlyCases {
			t.Run(command+"/"+tc.name, func(t *testing.T) {
				useConfig(t, taggedPlanStackConfig)
				withDryRun(t, func() {
					err := cmd.RunE(cmd, tc.args)
					if err == nil {
						t.Fatalf("dva %s %v returned nil; a leading flag suppresses the default plan and must be reported",
							command, tc.args)
					}
					msg := err.Error()
					if !strings.Contains(msg, "whole-stack path") {
						t.Errorf("message does not say where the flag applies: %q", msg)
					}
					if !strings.Contains(msg, "rejects") {
						t.Errorf("message does not say the plan path rejects the flag: %q", msg)
					}

					rerun := suggestedArgs(t, command, msg)
					if slicesContainsAny(rerun, tc.args) {
						t.Fatalf("suggestion %v still carries the flag the plan path rejects", rerun)
					}
					if err := cmd.RunE(cmd, rerun); err != nil {
						t.Fatalf("the printed suggestion 'dva %s %s' fails: %v",
							command, strings.Join(rerun, " "), err)
					}
				})
			})
		}
	}
}

// slicesContainsAny reports whether any token of want survives in got.
func slicesContainsAny(got, want []string) bool {
	for _, w := range want {
		if slices.Contains(got, w) {
			return true
		}
	}
	return false
}

// TestSuppressedDefaultPlanKeepsWorkingSuggestions pins the other half. The guard's original
// message is correct for every flag the plan path accepts, so narrowing it to the four
// selectors must not disturb those: `dva up --force` still gets `dva up p1 --force`, which
// runs.
func TestSuppressedDefaultPlanKeepsWorkingSuggestions(t *testing.T) {
	for _, tc := range []struct {
		command string
		args    []string
	}{
		{"up", []string{"--force"}},
		{"up", []string{"--no-wait"}},
		{"up", []string{"--var", "K=V"}},
		{"down", []string{"--purge"}},
		{"down", []string{"-v"}},
		{"restart", []string{"--no-wait"}},
	} {
		t.Run(tc.command+strings.Join(tc.args, ""), func(t *testing.T) {
			useConfig(t, taggedPlanStackConfig)
			cmd := planAwareLifecycleCommands[tc.command]
			withDryRun(t, func() {
				err := cmd.RunE(cmd, tc.args)
				if err == nil {
					t.Fatalf("dva %s %v returned nil; the default plan is suppressed", tc.command, tc.args)
				}
				rerun := suggestedArgs(t, tc.command, err.Error())
				if !slicesContainsAny(rerun, tc.args) {
					t.Fatalf("suggestion %v dropped a flag the plan path accepts", rerun)
				}
				if err := cmd.RunE(cmd, rerun); err != nil {
					t.Fatalf("the printed suggestion 'dva %s %s' fails: %v",
						tc.command, strings.Join(rerun, " "), err)
				}
			})
		})
	}
}

// TestSuppressedDefaultPlanStripsOnlyTheSelectors keeps a mixed invocation honest: the four go,
// everything else stays, and the surviving command is still the one the user meant.
func TestSuppressedDefaultPlanStripsOnlyTheSelectors(t *testing.T) {
	useConfig(t, taggedPlanStackConfig)
	withDryRun(t, func() {
		err := upCmd.RunE(upCmd, []string{"--tag", "app", "--no-wait", "--var", "K=V", "--mode=native"})
		if err == nil {
			t.Fatal("dva up with leading flags returned nil")
		}
		msg := err.Error()
		rerun := suggestedArgs(t, "up", msg)
		want := []string{"p1", "--no-wait", "--var", "K=V"}
		if strings.Join(rerun, " ") != strings.Join(want, " ") {
			t.Fatalf("suggestion = %v, want %v (from %q)", rerun, want, msg)
		}
		// Both removed names are reported, so the user learns which flags went and why —
		// a suggestion that silently drops arguments is its own kind of dead end.
		for _, name := range []string{"--tag", "--mode"} {
			if !strings.Contains(msg, name) {
				t.Errorf("message does not name the removed flag %s: %q", name, msg)
			}
		}
		if err := upCmd.RunE(upCmd, rerun); err != nil {
			t.Fatalf("the printed suggestion fails: %v", err)
		}
	})
}

// TestStackPathSelectorsNarrowAndResolve pins what the four options actually do on the path
// where they work, so no later cleanup can read the plan-path rejection as "these are dead".
//
// An earlier revision of TASK-273 proposed deleting them from the manifest on exactly that
// reading, from a plan-path-only measurement. --tag and --exclude-tag change which entries
// run; --mode and --env resolve against the dva.yml sections and fail by name when the section
// does not declare them.
func TestStackPathSelectorsNarrowAndResolve(t *testing.T) {
	// Baseline. Without it the two filter assertions below cannot tell narrowing from a
	// fixture that only ever runs one entry.
	t.Run("unfiltered runs both entries", func(t *testing.T) {
		useConfig(t, taggedStackOnlyConfig)
		var out string
		withDryRun(t, func() {
			out = captureOutput(t, func() {
				if err := upCmd.RunE(upCmd, nil); err != nil {
					t.Fatalf("dva up: %v", err)
				}
			})
		})
		if got := ranEntries(out); len(got) != 2 {
			t.Fatalf("dva up ran %v; want both entries\n--- output ---\n%s", got, out)
		}
	})

	t.Run("tag narrows the execution set", func(t *testing.T) {
		useConfig(t, taggedStackOnlyConfig)
		var out string
		withDryRun(t, func() {
			out = captureOutput(t, func() {
				if err := upCmd.RunE(upCmd, []string{"--tag", "app"}); err != nil {
					t.Fatalf("dva up --tag app: %v", err)
				}
			})
		})
		assertRanOnly(t, out, "web", "--tag app")
	})

	t.Run("exclude-tag narrows the execution set", func(t *testing.T) {
		useConfig(t, taggedStackOnlyConfig)
		var out string
		withDryRun(t, func() {
			out = captureOutput(t, func() {
				if err := upCmd.RunE(upCmd, []string{"--exclude-tag", "infra"}); err != nil {
					t.Fatalf("dva up --exclude-tag infra: %v", err)
				}
			})
		})
		assertRanOnly(t, out, "web", "--exclude-tag infra")
	})

	for _, tc := range []struct {
		name    string
		args    []string
		unknown []string
		section string
	}{
		{"mode", []string{"--mode", "native"}, []string{"--mode", "nope"}, "mode"},
		{"env", []string{"--env", "dev"}, []string{"--env", "nope"}, "env"},
	} {
		t.Run(tc.name+" resolves against its config section", func(t *testing.T) {
			useConfig(t, taggedStackOnlyConfig)
			withDryRun(t, func() {
				if err := upCmd.RunE(upCmd, tc.args); err != nil {
					t.Fatalf("dva up %s: %v", strings.Join(tc.args, " "), err)
				}
				err := upCmd.RunE(upCmd, tc.unknown)
				if err == nil {
					t.Fatalf("dva up %s returned nil; an undeclared %s must be named, not ignored",
						strings.Join(tc.unknown, " "), tc.section)
				}
				if !strings.Contains(err.Error(), "nope") {
					t.Errorf("error does not name the undeclared value: %v", err)
				}
			})
		})
	}
}

// TestManifestQualifiesStackPathOnlySelectors is the manifest half of the agreement criterion:
// every command whose plan path rejects the four says so in its option descriptions.
//
// build is the exception and is asserted as one. It calls parseDvaFlags before detectPlanRoute,
// so --mode is consumed off the raw args and works on both paths; appending the qualifier to
// the shared constant would have published a false claim there, which is why optModeBuild
// exists.
func TestManifestQualifiesStackPathOnlySelectors(t *testing.T) {
	m := buildManifest(&config.Config{})
	checked := 0
	for command := range planAwareLifecycleCommands {
		entry, ok := m.StaticCommands[command]
		if !ok {
			t.Fatalf("%q has no static_commands entry", command)
		}
		for _, opt := range []string{"mode", "env", "tag", "exclude-tag"} {
			desc, present := entry.Options[opt]
			if !present {
				t.Errorf("static_commands[%q].options is missing %q", command, opt)
				continue
			}
			checked++
			if !strings.Contains(desc, optStackPathOnly) {
				t.Errorf("%s --%s does not say the plan path rejects it: %q", command, opt, desc)
			}
			// The word matters as much as the qualifier. --var is plan-path-only and is
			// silently ignored off its path; these four are hard-rejected, and borrowing
			// --var's "ignored" would swap one missing fact for a false one.
			if strings.Contains(desc, "ignored when no plan") {
				t.Errorf("%s --%s claims the --var failure mode: %q", command, opt, desc)
			}
		}
	}
	if checked != 16 {
		t.Fatalf("checked %d option strings, want 16 (4 commands x 4 options)", checked)
	}

	build, ok := m.StaticCommands["build"]
	if !ok {
		t.Fatal("build has no static_commands entry")
	}
	if desc := build.Options["mode"]; strings.Contains(desc, optStackPathOnly) {
		t.Errorf("build --mode is accepted on both paths, so the qualifier is false there: %q", desc)
	}
}

// TestLongHelpAgreesWithTheManifest closes the loop the criterion asks for: the prose a human
// reads and the manifest an agent reads must not disagree about where the four apply.
func TestLongHelpAgreesWithTheManifest(t *testing.T) {
	for command, cmd := range planAwareLifecycleCommands {
		t.Run(command, func(t *testing.T) {
			long := cmd.Long
			header := "Whole-stack-path flags (rejected, not ignored, once a plan is named):"
			i := strings.Index(long, header)
			if i < 0 {
				t.Fatalf("dva %s --help does not qualify the whole-stack-path flags", command)
			}
			section := long[i:]
			for _, flag := range []string{"--mode", "--env", "--tag", "--exclude-tag"} {
				if !strings.Contains(section, flag) {
					t.Errorf("%s is not listed under the qualified heading in 'dva %s --help'", flag, command)
				}
			}
		})
	}
}

// assertRanOnly reads the execution set out of captured output.
//
// It matches the "[lifecycle] <entry> (<plugin>)" lines the runner prints per entry it
// actually executes, not bare entry names: every run also prints a "Lifecycle:" summary that
// lists every DECLARED entry, so a substring test for "db" reports the excluded entry as run
// and a test for "web" alone passes even when the filter does nothing. Both directions were
// observed while writing this.
func assertRanOnly(t *testing.T, out, wantEntry, invocation string) {
	t.Helper()
	ran := ranEntries(out)
	if len(ran) != 1 || ran[0] != wantEntry {
		t.Fatalf("dva up %s ran %v; want [%s] only\n--- output ---\n%s", invocation, ran, wantEntry, out)
	}
}

// ranEntries lists the entries the runner reported executing, in order.
func ranEntries(out string) []string {
	var ran []string
	for line := range strings.SplitSeq(out, "\n") {
		if !strings.HasPrefix(line, "[lifecycle] ") {
			continue
		}
		name, _, _ := strings.Cut(strings.TrimPrefix(line, "[lifecycle] "), " ")
		ran = append(ran, name)
	}
	return ran
}
