// Package cli — regression tests for TASK-098.
//
// TASK-087 fixed the orchestrator-backed stack subcommands; these are the two exit-0 paths it
// scoped out, because each escapes by a different mechanism. Measured at 97e8649:
//
//	dva stack status <typo>   -> exit 0, an empty table indistinguishable from an empty stack
//	dva stack nosuchsub       -> exit 0, 1222 bytes of help
//
// `status` filters inline rather than through orchestrator.filterEntries, so neither helper
// TASK-087 added reached it. The parent escapes through cobra: legacyArgs() lets a non-root
// command with subcommands take arbitrary args, and execute() returns flag.ErrHelp for a
// command with no Run/RunE *before* it would ever call ValidateArgs.
package cli

import (
	"io"
	"strings"
	"testing"
)

// TestStackStatusRejectsUnknownName drives the real RunE. The rejection returns before the
// orchestrator is built, so no plugin is consulted for the case under test.
func TestStackStatusRejectsUnknownName(t *testing.T) {
	cases := []struct {
		name string
		args []string
		// reject: whether validateStackNames must refuse this input. The two false rows are
		// the controls that separate "rejects typos" from "rejects everything" — without
		// them, a status command that failed unconditionally would pass.
		reject bool
	}{
		{"a name matching nothing is rejected", []string{"nosuchentry"}, true},
		{"a real name is accepted", []string{"infra"}, false},
		{"no name at all is accepted", nil, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			writeStackArgsConfig(t)

			err := stackStatusCmd.RunE(stackStatusCmd, tc.args)

			// Asserting on the message rather than on err != nil: the accepted rows may
			// still fail via StatusExitError, which is a different verdict about a real
			// entry and not this task's business.
			const marker = "no such stack entry"
			got := err != nil && strings.Contains(err.Error(), marker)
			if got != tc.reject {
				if tc.reject {
					t.Errorf("stack status %v returned %v, want an error naming %q — an empty "+
						"table reads exactly like an empty stack", tc.args, err, marker)
				} else {
					t.Errorf("stack status %v was rejected with %v; this name exists", tc.args, err)
				}
			}
			if tc.reject && err != nil && !strings.Contains(err.Error(), "nosuchentry") {
				t.Errorf("error %q does not name the offending entry", err)
			}
		})
	}
}

// TestStackParentRejectsUnknownSubcommand goes through rootCmd, because what is under test is
// cobra's dispatch, not a RunE body — calling stackCmd.RunE directly would skip the very
// ValidateArgs step that does the work.
func TestStackParentRejectsUnknownSubcommand(t *testing.T) {
	cases := []struct {
		name string
		args []string
		// wantErr empty means the invocation must succeed.
		wantErr string
	}{
		{"unknown subcommand is an error", []string{"stack", "nosuchsub"}, `unknown command "nosuchsub"`},
		{"bare stack prints help and succeeds", []string{"stack"}, ""},
		{"an explicit help request is not an error", []string{"stack", "--help"}, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rootCmd.SetArgs(tc.args)
			rootCmd.SetOut(io.Discard)
			rootCmd.SetErr(io.Discard)
			t.Cleanup(func() {
				rootCmd.SetArgs(nil)
				rootCmd.SetOut(nil)
				rootCmd.SetErr(nil)
			})

			_, err := rootCmd.ExecuteC()

			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("dva %s returned %v, want success", strings.Join(tc.args, " "), err)
				}
				return
			}
			if err == nil {
				t.Fatalf("dva %s exited 0 over a subcommand that does not exist",
					strings.Join(tc.args, " "))
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error = %q, want it to contain %q", err, tc.wantErr)
			}
		})
	}
}

// TestUnknownCommandToken pins the parser behind the suggestion guard in root.go. The empty
// returns matter as much as the hit: callers must read "" as "cannot tell", and a message that
// is not in cobra's shape must not be mistaken for a top-level miss.
func TestUnknownCommandToken(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{`unknown command "nosuchsub" for "dva stack"`, "nosuchsub"},
		{`unknown command "stat" for "dva"`, "stat"},
		{`unknown command with no quotes`, ""},
		{`unterminated "quote`, ""},
		{``, ""},
	}
	for _, tc := range cases {
		if got := unknownCommandToken(tc.in); got != tc.want {
			t.Errorf("unknownCommandToken(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
