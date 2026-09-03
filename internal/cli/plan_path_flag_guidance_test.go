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

	// Values that LOOK like flags but are not. parseDvaFlags takes the token after a
	// value-taking selector whatever it spells unless DVA owns the name, so `--tag -5`
	// declares a tag literally called "-5" and the whole-stack path accepts it — it simply
	// matches no entry. The strip walk used to leave such a token behind, and the suggestion
	// carried a `-5` that nothing accepts. Measured against the built binary before it was
	// written down; the first draft of TASK-283 called `-5` malformed and was wrong.
	{"tag-dash-value", []string{"--tag", "-5"}},
	{"tag-inline-dash-value", []string{"--tag=-5"}},
	{"exclude-tag-dash-value", []string{"--exclude-tag", "-5"}},
}

// selectorRejectingCommands are the plan-aware commands whose whole-stack path reads none of
// the four selectors, so the guard must not tell their users that it does.
//
// They are kept apart from planAwareLifecycleCommands rather than folded into it because the
// manifest assertions below count that map's members against a fixed number of option strings,
// and because the property under test here is the opposite one.
var selectorRejectingCommands = map[string]*cobra.Command{
	"logs":   logsCmd,
	"status": statusCmd,
}

var planAwareLifecycleCommands = map[string]*cobra.Command{
	"up":      upCmd,
	"down":    downCmd,
	"stop":    stopCmd,
	"restart": restartCmd,
}

// withUserTypedDryRun clears the dry-run global so that the only thing which can set it is a
// --dry-run written into ARGS, the way a user sets it.
//
// The distinction is the subject of a defect, not a nicety. wrapWithHooks calls
// consumeDryRunFlag before a hookable command's RunE runs, which REMOVES --dry-run from args
// and records it in the package variable; every suggestion the guard printed was built from
// the args that survived, so it silently dropped the flag and following the advice performed
// the change the user had asked to preview. withDryRun below assigns that variable directly,
// which is why no case in this file could observe it: the token never entered the args the
// guard reads, so its disappearance from the suggestion was unobservable by construction.
// TASK-283.
//
// Every test that reads a printed suggestion uses this one and writes the flag out. The
// tests that only need the fixtures' script runners to print keep withDryRun.
func withUserTypedDryRun(t *testing.T, fn func()) {
	t.Helper()
	old := dryRun
	dryRun = false
	defer func() { dryRun = old }()
	fn()
}

// dryRunArgs writes --dry-run at the front of an invocation, as a user would.
func dryRunArgs(args ...string) []string {
	return append([]string{"--dry-run"}, args...)
}

// withDryRun runs fn with the global dry-run flag set, so the fixtures' script runners print
// rather than execute. Do not use it where a printed suggestion is read back — see
// withUserTypedDryRun.
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
				withUserTypedDryRun(t, func() {
					err := cmd.RunE(cmd, dryRunArgs(tc.args...))
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
					// The user typed --dry-run and the hook wrapper ate it before the guard
					// could see it. A suggestion that omits it is a suggestion to perform the
					// change they asked to preview.
					if !slices.Contains(rerun, "--dry-run") {
						t.Fatalf("suggestion %v drops the --dry-run the invocation carried", rerun)
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
			withUserTypedDryRun(t, func() {
				err := cmd.RunE(cmd, dryRunArgs(tc.args...))
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
	withUserTypedDryRun(t, func() {
		err := upCmd.RunE(upCmd, dryRunArgs("--tag", "app", "--no-wait", "--var", "K=V", "--mode=native"))
		if err == nil {
			t.Fatal("dva up with leading flags returned nil")
		}
		msg := err.Error()
		rerun := suggestedArgs(t, "up", msg)
		// --dry-run comes last because the guard appends what the hook wrapper consumed after
		// the arguments that survived; its position is part of the pinned string.
		want := []string{"p1", "--no-wait", "--var", "K=V", "--dry-run"}
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

// noSuggestion asserts that a guard message proposes no command to run.
//
// The governing property of TASK-283 is that a suggestion must never be printed unless it is
// known to run. Where the guard cannot establish that, printing nothing is the requirement —
// so "there is no ': dva ' clause" is an assertion about behaviour, not about wording, and it
// is the exact complement of what suggestedArgs looks for.
func noSuggestion(t *testing.T, msg string) {
	t.Helper()
	if i := strings.LastIndex(msg, ": dva "); i >= 0 {
		t.Fatalf("guard proposes %q, but nothing here is known to run: %q", msg[i+2:], msg)
	}
}

// TestSuppressedDefaultPlanDefersMalformedSelectorsToTheParser pins the step-aside.
//
// A selector with no usable value is parseDvaFlags' to report: it knows which rule was broken
// and names it. The guard used to preempt that with advice built on the assumption that the
// pair was well-formed, and produced a suggestion the plan path rejects — `dva up --tag -T web`
// was answered with `dva up p1 web`, which strands the `web` that was never a plan-path
// argument. Stepping aside costs nothing: the invocation still fails, so the whole-stack
// fallthrough this guard exists to refuse cannot happen.
func TestSuppressedDefaultPlanDefersMalformedSelectorsToTheParser(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"no value", []string{"--tag"}, "--tag requires a value"},
		{"flag as value", []string{"--tag", "-T", "web"}, "--tag requires a value, got the flag -T"},
		{"inline empty", []string{"--tag="}, "--tag requires a non-empty value"},
		{"empty next token", []string{"--tag", ""}, "--tag requires a non-empty value"},
		{"blank next token", []string{"--mode", "  "}, "--mode requires a non-blank value"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			useConfig(t, taggedPlanStackConfig)
			withUserTypedDryRun(t, func() {
				err := upCmd.RunE(upCmd, dryRunArgs(tc.args...))
				if err == nil {
					t.Fatalf("dva up %v returned nil; a malformed selector must be reported", tc.args)
				}
				msg := err.Error()
				if !strings.Contains(msg, tc.want) {
					t.Fatalf("message %q does not name the broken rule (want %q)", msg, tc.want)
				}
				noSuggestion(t, msg)
			})
		})
	}
}

// TestSuppressedDefaultPlanRefusesToSuggestWhenASelectorAteAFlag covers the one anomaly
// parseDvaFlags does NOT reject.
//
// `dva up --tag --no-wait` runs on the whole-stack path with a tag literally named
// "--no-wait", so stepping aside would be the silent fallthrough this guard forbids. Dropping
// the selector is not available either: `dva up p1 --no-wait` runs, and means something the
// user did not write, because in their invocation --no-wait was a value. The guard says what
// happened and offers no command.
func TestSuppressedDefaultPlanRefusesToSuggestWhenASelectorAteAFlag(t *testing.T) {
	t.Run("up-honours-the-swallowed-token", func(t *testing.T) {
		useConfig(t, taggedPlanStackConfig)
		withUserTypedDryRun(t, func() {
			err := upCmd.RunE(upCmd, dryRunArgs("--tag", "--no-wait"))
			if err == nil {
				t.Fatal("dva up --tag --no-wait returned nil; the default plan is suppressed")
			}
			msg := err.Error()
			for _, want := range []string{"--tag", "--no-wait", "--tag=--no-wait"} {
				if !strings.Contains(msg, want) {
					t.Errorf("message does not mention %s: %q", want, msg)
				}
			}
			noSuggestion(t, msg)
		})
	})

	// The same shape on stop, where the refusal above would be wrong.
	//
	// TASK-279 made parsePlanFlags verb-aware: --no-wait means nothing once the direction is
	// teardown, so stop and down reject it. That changes this guard's correct answer and is
	// the reason it asks the parser per verb instead of holding its own list. On up, dropping
	// --tag would arm a flag the user wrote as a value, so no command can be offered. On stop,
	// restoring --no-wait restores nothing the verb would honour — the token is only ever a tag
	// value there — so dropping the pair is faithful and `dva stop p1` is a real suggestion.
	//
	// This is the interaction test for the two cards. Whichever of them a later change touches,
	// silently collapsing these two answers into one fails here.
	t.Run("stop-rejects-it-so-the-pair-is-droppable", func(t *testing.T) {
		useConfig(t, taggedPlanStackConfig)
		withUserTypedDryRun(t, func() {
			err := stopCmd.RunE(stopCmd, dryRunArgs("--tag", "--no-wait"))
			if err == nil {
				t.Fatal("dva stop --tag --no-wait returned nil; the default plan is suppressed")
			}
			rerun := suggestedArgs(t, "stop", err.Error())
			if slices.Contains(rerun, "--no-wait") {
				t.Fatalf("suggestion %v restores --no-wait, which stop rejects outright", rerun)
			}
			if !slices.Contains(rerun, "--dry-run") {
				t.Fatalf("suggestion %v drops the --dry-run the invocation carried", rerun)
			}
			if err := stopCmd.RunE(stopCmd, rerun); err != nil {
				t.Fatalf("the printed suggestion 'dva stop %s' fails: %v",
					strings.Join(rerun, " "), err)
			}
		})
	})
}

// TestSuppressedDefaultPlanRefusesToSuggestWhatThePlanPathRejects is the leftover half.
//
// Stripping the selectors can leave behind a token the plan path will not take, and the guard
// printed it anyway: `dva up --tag app web` proposed `dva up p1 web`, which parsePlanFlags
// answers with `unexpected argument in plan mode: web`. Following the advice was what broke
// the invocation, which is the same failure TASK-273 was filed for in a different spelling.
//
// restart is here because the first draft made it an exception, on the reasoning that
// `dva restart web` names an entry so `dva restart p1 web` should name one too. It does not —
// the plan path takes the plan name and flags, and nothing else — and the test said so before
// the code shipped. That is the whole reason the check calls parsePlanFlags rather than
// re-deriving its rule.
func TestSuppressedDefaultPlanRefusesToSuggestWhatThePlanPathRejects(t *testing.T) {
	for _, tc := range []struct {
		command string
		cmd     *cobra.Command
		args    []string
	}{
		{"up", upCmd, []string{"--tag", "app", "web"}},
		{"restart", restartCmd, []string{"--tag", "app", "web"}},
		{"down", downCmd, []string{"--mode", "native", "web"}},
	} {
		t.Run(tc.command, func(t *testing.T) {
			useConfig(t, taggedPlanStackConfig)
			withUserTypedDryRun(t, func() {
				err := tc.cmd.RunE(tc.cmd, dryRunArgs(tc.args...))
				if err == nil {
					t.Fatalf("dva %s %v returned nil", tc.command, tc.args)
				}
				msg := err.Error()
				if !strings.Contains(msg, "web") {
					t.Errorf("message does not name the leftover token: %q", msg)
				}
				// The plan path's own words, so the user learns what is wrong with the token
				// rather than only that no advice is available.
				if !strings.Contains(msg, "unexpected argument in plan mode") {
					t.Errorf("message does not carry the plan path's reason: %q", msg)
				}
				noSuggestion(t, msg)
			})
		})
	}
}

// TestSuppressedDefaultPlanKeepsAFlagsOwnValue is the other direction of the same check, and
// the reason it is delegated to parsePlanFlags rather than hand-written.
//
// A hand-written "is any bare word left over?" test rejected `--var K=V`, because K=V is a bare
// word — it is also --var's value, and the plan path accepts the pair. Both directions were
// wrong in one measurement, which is what a second copy of a parser's rules buys.
func TestSuppressedDefaultPlanKeepsAFlagsOwnValue(t *testing.T) {
	useConfig(t, taggedPlanStackConfig)
	withUserTypedDryRun(t, func() {
		err := upCmd.RunE(upCmd, dryRunArgs("--tag", "app", "--var", "K=V"))
		if err == nil {
			t.Fatal("dva up --tag app --var K=V returned nil")
		}
		rerun := suggestedArgs(t, "up", err.Error())
		if !slices.Contains(rerun, "--var") || !slices.Contains(rerun, "K=V") {
			t.Fatalf("suggestion %v dropped --var or its value", rerun)
		}
		if err := upCmd.RunE(upCmd, rerun); err != nil {
			t.Fatalf("the printed suggestion 'dva up %s' fails: %v", strings.Join(rerun, " "), err)
		}
	})
}

// TestSuppressedDefaultPlanDoesNotClaimSelectorsWorkWhereTheyDoNot covers the commands whose
// whole-stack path reads none of the four.
//
// `dva logs` forwards its argv to docker compose and calls parseDvaFlags on none of it, so
// --tag is docker's token on both paths. The guard nonetheless stripped it and reported that
// it "works only on the whole-stack path", which is false, and proposed `dva logs p1` — an
// invocation that has quietly lost the flag the user typed. What the guard actually knows here
// is only that the default plan is suppressed, so it says that and leaves the argv alone.
//
// The suggestion is not re-run. Following it reaches execComposePassthrough, and a unit test
// that shells out to docker would be measuring docker's flag set rather than DVA's guidance —
// which is the same reason the guard must not predict that flag set either.
func TestSuppressedDefaultPlanDoesNotClaimSelectorsWorkWhereTheyDoNot(t *testing.T) {
	for command, cmd := range selectorRejectingCommands {
		t.Run(command, func(t *testing.T) {
			useConfig(t, taggedPlanStackConfig)
			withUserTypedDryRun(t, func() {
				err := cmd.RunE(cmd, dryRunArgs("--tag", "app"))
				if err == nil {
					t.Fatalf("dva %s --tag app returned nil; the default plan is suppressed", command)
				}
				msg := err.Error()
				if strings.Contains(msg, "whole-stack path") {
					t.Errorf("dva %s does not read --tag on either path, so the message must not "+
						"say it works on one: %q", command, msg)
				}
				rerun := suggestedArgs(t, command, msg)
				if !slices.Contains(rerun, "--tag") || !slices.Contains(rerun, "app") {
					t.Fatalf("suggestion %v silently dropped the flag the user typed", rerun)
				}
				if n := slices.Index(rerun, "--dry-run"); n < 0 {
					t.Fatalf("suggestion %v drops the --dry-run the invocation carried", rerun)
				}
				if got := countToken(rerun, "--dry-run"); got != 1 {
					t.Fatalf("suggestion %v repeats --dry-run %d times", rerun, got)
				}
			})
		})
	}
}

// countToken counts exact occurrences of a token.
func countToken(args []string, token string) int {
	n := 0
	for _, a := range args {
		if a == token {
			n++
		}
	}
	return n
}

// TestSuppressedDefaultPlanOnBuildNeverSeesASelector closes the third command the coverage
// criterion named, and it closes it by measurement rather than by adding build to a table.
//
// build cannot be driven through its RunE here the way logs and status are: no selector ever
// reaches the guard on that route, so reproducing the input would mean letting the invocation
// fall through to docker. The routing fact is the assertion instead. buildCmd calls
// parseDvaFlags on the raw args before detectPlanRoute (internal/cli/compose.go), which
// consumes all four selectors; what it hands the guard therefore contains none of them, the
// guard's `len(head) == 0` early return fires when they were the whole argv, and `res.removed`
// is empty when they were not. Neither path can produce the false "works only on the
// whole-stack path" claim that §2 filed against logs, which is why build needs no entry in
// selectorAwarePlanCommands.
//
// That the flag the user typed vanishes from `dva build --tag app` entirely is real and is not
// this card's: build discards --env/--tag/--exclude-tag at the parse site, filed as TASK-279 §3.
// The guard is faithful to that behaviour — it echoes what build will act on — and repairing
// the behaviour is what makes the echo worth reading.
func TestSuppressedDefaultPlanOnBuildNeverSeesASelector(t *testing.T) {
	_, _, _, _, remaining, err := parseDvaFlags([]string{"--tag", "app", "--no-cache"})
	if err != nil {
		t.Fatalf("parseDvaFlags: %v", err)
	}
	for _, sel := range stackPathOnlySelectorFlags {
		if slices.Contains(remaining, sel) {
			t.Fatalf("build hands the guard %v, which still carries %s; the guard would then "+
				"have to reason about a selector on this route", remaining, sel)
		}
	}

	c := &config.Config{Plans: map[string]*config.PlanConfig{"p1": {}}}
	guardErr := rejectSuppressedDefaultPlan(c, "build", remaining)
	if guardErr == nil {
		t.Fatalf("rejectSuppressedDefaultPlan(build, %v) returned nil", remaining)
	}
	msg := guardErr.Error()
	if strings.Contains(msg, "whole-stack path") {
		t.Errorf("no selector reached the guard, so it must claim nothing about where one "+
			"applies: %q", msg)
	}
	if !strings.Contains(msg, "dva build p1 --no-cache") {
		t.Errorf("the guard must echo what survived the parse: %q", msg)
	}
}
