// Package cli — regression tests for TASK-113.
//
// `dva up` and the `dva app` family set DisableFlagParsing and hand-roll their argument
// loops. Neither loop had a default case, so `dva up --forse` ran the whole stack as if no
// flag had been given, and `dva app up --dev=true` turned the flag into an application
// name, matched nothing, printed a status table and exited 0. `dva app up nosuchapp`
// produced byte-for-byte the same output with no flag involved at all.
//
// Every case below exited 0 before the fix. The reject paths return before any process is
// started, so these tests run without docker and without leaving a process behind — which
// is also why the accepting cases are expressed as "the error names the app, not the flag"
// rather than as a successful start.
package cli

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// twoAppsConfig declares applications whose run commands would announce themselves if they
// ever executed. No test here should reach them.
const twoAppsConfig = `version: "0.1.0"
applications:
  web:
    port: 13113
    run:
      native: echo web-should-not-have-started
  api:
    port: 13114
    run:
      native: echo api-should-not-have-started
`

// TestAppRejectsUnknownFlags covers the loop that turned a mistyped flag into an app name.
func TestAppRejectsUnknownFlags(t *testing.T) {
	cases := []struct {
		name string
		cmd  *cobra.Command
		args []string
		want []string
	}{
		{"up: --dev=true is not --dev", appUpCmd, []string{"--dev=true"},
			[]string{"unknown flag", "--dev=true", "--dev"}},
		{"up: mistyped --dev after a real name", appUpCmd, []string{"web", "--dve"},
			[]string{"unknown flag", "--dve", "--dev"}},
		{"restart: --dev=true", appRestartCmd, []string{"--dev=true"},
			[]string{"unknown flag", "--dev=true"}},
		{"build: mistyped --docker", appBuildCmd, []string{"--dokcer"},
			[]string{"unknown flag", "--dokcer", "--docker"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			useAppConfig(t, twoAppsConfig)

			var err error
			out := captureOutput(t, func() { err = tc.cmd.RunE(tc.cmd, tc.args) })
			if err == nil {
				t.Fatalf("args %v returned nil; the flag was read as an app name, matched nothing and the command reported success", tc.args)
			}
			for _, want := range tc.want {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("message does not mention %q:\n%s", want, err.Error())
				}
			}
			if strings.Contains(out, "should-not-have-started") {
				t.Errorf("an application ran before the argument was rejected:\n%s", out)
			}
		})
	}
}

// TestAppRejectsUnknownNames is the half that no amount of flag parsing reaches. selectApps
// logs a Debug line and skips, so "nothing matched" and "no names given, start everything"
// both arrived at StartApps as an empty map — and the second is a success.
func TestAppRejectsUnknownNames(t *testing.T) {
	cases := []struct {
		name string
		cmd  *cobra.Command
		args []string
		want []string
	}{
		{"up", appUpCmd, []string{"nosuchapp"}, []string{"no such application", "nosuchapp", "web"}},
		{"up: near miss gets a suggestion", appUpCmd, []string{"wbe"}, []string{"no such application", "dva app up web"}},
		{"restart", appRestartCmd, []string{"nosuchapp"}, []string{"no such application", "nosuchapp"}},
		{"build", appBuildCmd, []string{"nosuchapp"}, []string{"no such application", "nosuchapp"}},
		{"stop", appStopCmd, []string{"nosuchapp"}, []string{"no such application", "nosuchapp"}},
		{"down", appDownCmd, []string{"nosuchapp"}, []string{"no such application", "nosuchapp"}},
		{"one real name and one typo still fails", appUpCmd, []string{"web", "nosuchapp"},
			[]string{"no such application", "nosuchapp"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			useAppConfig(t, twoAppsConfig)

			var err error
			out := captureOutput(t, func() { err = tc.cmd.RunE(tc.cmd, tc.args) })
			if err == nil {
				t.Fatalf("args %v returned nil; an unresolved name is indistinguishable from success", tc.args)
			}
			for _, want := range tc.want {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("message does not mention %q:\n%s", want, err.Error())
				}
			}
			if strings.Contains(out, "should-not-have-started") {
				t.Errorf("an application ran before the name was rejected:\n%s", out)
			}
		})
	}
}

// TestAppConsumesItsOwnFlagsBeforeRejecting is the control that keeps the two tests above
// from being satisfied by a command that rejects everything. rejectUnknownFlags fires on any
// dash-prefixed argument and runs before the name check, so a --dev that the loop failed to
// consume would surface as "unknown flag". Getting "no such application" instead proves the
// flag was recognised and removed — asserted this way rather than by a successful start,
// which would spawn a process for a unit test to clean up.
func TestAppConsumesItsOwnFlagsBeforeRejecting(t *testing.T) {
	cases := []struct {
		name string
		cmd  *cobra.Command
		args []string
	}{
		{"up honours --dev", appUpCmd, []string{"--dev", "nosuchapp"}},
		{"restart honours --dev", appRestartCmd, []string{"--dev", "nosuchapp"}},
		{"build honours --docker", appBuildCmd, []string{"--docker", "nosuchapp"}},
		{"up honours --mode via parseDvaFlags", appUpCmd, []string{"--mode", "dev", "nosuchapp"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			useAppConfig(t, twoAppsConfig)

			var err error
			captureOutput(t, func() { err = tc.cmd.RunE(tc.cmd, tc.args) })
			if err == nil {
				t.Fatalf("args %v returned nil", tc.args)
			}
			if strings.Contains(err.Error(), "unknown flag") {
				t.Errorf("the command's own flag was not consumed before the guard:\n%s", err.Error())
			}
			if !strings.Contains(err.Error(), "no such application") {
				t.Errorf("want the name to be the complaint, got:\n%s", err.Error())
			}
		})
	}
}

// TestUpRejectsUnknownFlags covers the other loop. `dva up` has no positional names either,
// so both guards are asserted: the flag one, and the positional one behind a flag —
// rejectUpPositionalArg runs earlier on args[0] and returns nil when that is a flag, so
// `dva up --dev nosuchthing` reached the loop and had the name dropped.
func TestUpRejectsUnknownFlags(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want []string
	}{
		{"mistyped --force", []string{"--forse"}, []string{"unknown flag", "--forse", "--force"}},
		{"the habitual --force=true", []string{"--force=true"}, []string{"unknown flag", "--force=true", "--force"}},
		{"positional hidden behind a flag", []string{"--dev", "nosuchthing"}, []string{"nosuchthing", "takes no positional arguments"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			useAppConfig(t, noAppsConfig)

			var err error
			out := captureOutput(t, func() { err = upCmd.RunE(upCmd, tc.args) })
			if err == nil {
				t.Fatalf("dva up %v returned nil; the argument was discarded and the whole stack started as if it had not been passed", tc.args)
			}
			for _, want := range tc.want {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("message does not mention %q:\n%s", want, err.Error())
				}
			}
			if strings.Contains(out, "MARKERS1") {
				t.Errorf("the stack ran before the argument was rejected:\n%s", out)
			}
		})
	}
}
