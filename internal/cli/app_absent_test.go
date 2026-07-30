package cli

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// noAppsConfig declares a lifecycle and an interaction and no applications: section. That is
// not an incomplete config — shared-guardrails.md forbids generating applications: for new
// configs — so every `dva app` subcommand must answer it as a normal state.
const noAppsConfig = `version: "0.1.0"
stack:
  s1:
    default_runner: script
    runners:
      script:
        up: echo MARKERS1
interaction:
  hello:
    description: say hello
    steps:
      - step: echo hi
`

// useAppConfig is useConfig plus the cached Environment. The app subcommands build it before
// they reach the empty-applications guard, so leaving it set would hand the next test an
// Environment rooted in a temp dir that no longer exists.
func useAppConfig(t *testing.T, yml string) {
	t.Helper()
	useConfig(t, yml)
	env = nil
	t.Cleanup(func() { env = nil })
}

func TestAppUpBareOnAbsentApplications(t *testing.T) {
	useAppConfig(t, noAppsConfig)

	var err error
	out := captureOutput(t, func() { err = appUpCmd.RunE(appUpCmd, []string{}) })

	if err != nil {
		t.Fatalf("bare 'dva app up' must not fail on an absent applications: section: %v", err)
	}
	if !strings.Contains(out, "dva up") {
		t.Errorf("output = %q, want a command the user can actually run", out)
	}
	if !strings.Contains(out, "stack (1)") {
		t.Errorf("output = %q, want the count of what the config does declare", out)
	}
}

func TestAppUpNamedOnAbsentApplications(t *testing.T) {
	useAppConfig(t, noAppsConfig)

	err := appUpCmd.RunE(appUpCmd, []string{"myapp"})
	if err == nil {
		t.Fatal("'dva app up myapp' returned nil; a named target that cannot exist must fail, or a typo passes silently")
	}
	if !strings.Contains(err.Error(), "myapp") {
		t.Errorf("error = %q, want it to name the target the user asked for", err)
	}
}

func TestAppLogOnAbsentApplications(t *testing.T) {
	useAppConfig(t, noAppsConfig)

	err := appLogCmd.RunE(appLogCmd, []string{"myapp"})
	if err == nil {
		t.Fatal("'dva app log myapp' returned nil; there is no log to show")
	}
	// The old message was "application 'myapp' not found", which reports the app as missing
	// from a set of applications when the set itself does not exist.
	if strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, must not blame the name for an absent section", err)
	}
}

// absentApplicationsCases is every path that can meet the guard: each subcommand in the form
// that names a target and, where the subcommand accepts it, the bare form. `ls` appears in both
// forms with the same expectation, because it does not act on a named target either way.
var absentApplicationsCases = []struct {
	label   string
	cmd     *cobra.Command
	args    []string
	wantErr bool
}{
	{"app ls", appLsCmd, nil, false},
	// ls with an argument stays a success: it ignores positional arguments when applications
	// exist, so an absent section must not be the only case where it rejects one.
	{"app ls myapp", appLsCmd, []string{"myapp"}, false},
	{"app up", appUpCmd, nil, false},
	{"app stop", appStopCmd, nil, false},
	{"app down", appDownCmd, nil, false},
	{"app restart", appRestartCmd, nil, false},
	{"app build", appBuildCmd, nil, false},
	{"app up myapp", appUpCmd, []string{"myapp"}, true},
	{"app stop myapp", appStopCmd, []string{"myapp"}, true},
	{"app down myapp", appDownCmd, []string{"myapp"}, true},
	{"app restart myapp", appRestartCmd, []string{"myapp"}, true},
	{"app build myapp", appBuildCmd, []string{"myapp"}, true},
	{"app log myapp", appLogCmd, []string{"myapp"}, true},
}

// runAbsentApplicationsCase returns everything the user sees, from either stream, plus the
// exit outcome. The two forms carry the message differently — one prints it, one returns it —
// so an assertion that read only one of them would cover half the paths.
func runAbsentApplicationsCase(t *testing.T, cmd *cobra.Command, args []string) (string, error) {
	t.Helper()
	var err error
	out := captureOutput(t, func() { err = cmd.RunE(cmd, args) })
	if err != nil {
		out += err.Error()
	}
	return out, err
}

// TestAbsentApplicationsMessageNamesNoFile pins the one thing the message must not say.
// Config is the merge of modules: and subprojects:, so the file that would need an
// applications: block is not knowable from the loaded config — naming dva.yml is the same
// misdirection TASK-073 removed from the version error.
func TestAbsentApplicationsMessageNamesNoFile(t *testing.T) {
	for _, tc := range absentApplicationsCases {
		t.Run(tc.label, func(t *testing.T) {
			useAppConfig(t, noAppsConfig)
			out, _ := runAbsentApplicationsCase(t, tc.cmd, tc.args)

			if out == "" {
				t.Fatalf("'dva %s' said nothing at all", tc.label)
			}
			if strings.Contains(out, config.FileName) {
				t.Errorf("'dva %s' output = %q, must not name a config file", tc.label, out)
			}
		})
	}
}

// TestAbsentApplicationsRoutesToCurrentModel checks the route is the model the project
// actually recommends. `dva stack up` is not it (the stack is a declaration store), and
// another `dva app` subcommand lands the reader back on this same message.
func TestAbsentApplicationsRoutesToCurrentModel(t *testing.T) {
	for _, tc := range absentApplicationsCases {
		t.Run(tc.label, func(t *testing.T) {
			useAppConfig(t, noAppsConfig)
			out, err := runAbsentApplicationsCase(t, tc.cmd, tc.args)

			if !strings.Contains(out, "dva up") {
				t.Errorf("'dva %s' output = %q, want the current-model route", tc.label, out)
			}
			for _, dead := range []string{"dva stack", "dva app"} {
				if strings.Contains(out, dead) {
					t.Errorf("'dva %s' output = %q, must not route to %q", tc.label, out, dead)
				}
			}
			if tc.wantErr && err == nil {
				t.Errorf("'dva %s' returned nil; naming a target that cannot exist must fail", tc.label)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("'dva %s' returned %v; acting on zero targets is a no-op, not a failure", tc.label, err)
			}
		})
	}
}

// TestAbsentApplicationsAdviceSuggestsOnlyWorkingInvocations covers the branches that decide
// the route. Each expectation was checked against the real binary: bare `dva up` refuses with
// "multiple plans configured" when plans exist without a default, and reports
// "(no entries configured)" when only interactions are declared — so neither config may be
// answered with a bare `dva up`.
func TestAbsentApplicationsAdviceSuggestsOnlyWorkingInvocations(t *testing.T) {
	stack := map[string]*config.LifecycleEntry{"s1": {}}

	cases := []struct {
		name string
		c    *config.Config
		want string
	}{
		{
			name: "one plan is named outright",
			c:    &config.Config{Stack: stack, Plans: map[string]*config.PlanConfig{"local-dev": {}}},
			want: "run 'dva up local-dev'",
		},
		{
			name: "explicit default wins over the other plans",
			c: &config.Config{Stack: stack, DefaultPlanName: "ci", Plans: map[string]*config.PlanConfig{
				"local-dev": {}, "ci": {},
			}},
			want: "run 'dva up ci'",
		},
		{
			name: "several plans and no default list the choice, sorted",
			c: &config.Config{Stack: stack, Plans: map[string]*config.PlanConfig{
				"local-dev": {}, "ci": {},
			}},
			want: "run 'dva up <ci|local-dev>'",
		},
		{
			name: "a stack without plans takes a bare up",
			c:    &config.Config{Stack: stack},
			want: "run 'dva up'",
		},
		{
			name: "interactions alone are not a lifecycle",
			c:    &config.Config{Interaction: map[string]*config.InteractionCommand{"hello": {}}},
			want: "run 'dva ls'",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := absentApplicationsAdvice(tc.c); !strings.Contains(got, tc.want) {
				t.Errorf("advice = %q, want it to contain %q", got, tc.want)
			}
		})
	}
}

// A config declaring nothing runnable has no route, and inventing one would be the defect
// this task removed. It must say so instead of suggesting a command that does nothing.
func TestAbsentApplicationsAdvicePromisesNothingWhenNothingIsDeclared(t *testing.T) {
	got := absentApplicationsAdvice(&config.Config{})

	if !strings.Contains(got, "no plans, stack entries, or interactions") {
		t.Errorf("advice = %q, want it to state that nothing is declared", got)
	}
	if strings.Contains(got, "run '") {
		t.Errorf("advice = %q, must not offer a command when there is nothing to run", got)
	}
}
