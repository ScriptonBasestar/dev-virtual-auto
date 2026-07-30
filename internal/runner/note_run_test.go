package runner

import (
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// TestNoteDoesNotSuppressRun covers TASK-089: both runners used to treat `note:` as a
// replacement for the step's work — they printed the note and `continue`d — while the same
// item under `dva provision` printed the note and executed. Adding a comment to a working
// step silently stopped it working, with exit 0 and nothing reported.
//
// The two runners need different evidence that execution was *attempted*:
//
//   - LocalRunner runs a real subprocess (ExecSequential → ExecSubprocess → cmd.Run), so the
//     marker its command echoes is visible on captured stdout.
//   - DockerComposeRunner ends in execCompose → ExecReplace → syscall.Exec, which on success
//     REPLACES the running process. Letting it reach a real binary would delete this test
//     binary mid-run. Pointing its compose command at a name that cannot resolve makes
//     exec.LookPath fail *before* the exec, so the attempt surfaces as an error and nothing
//     is ever executed.
func TestNoteDoesNotSuppressRun(t *testing.T) {
	env := &config.Environment{}

	const (
		marker         = "NOTE-RUN-EXECUTED"
		absentCompose  = "dva-absent-compose-binary"
		label          = "deploy the thing"
		noteText       = "rotate the token first"
		runCommandText = "echo " + marker
	)

	composeCfg := &config.Config{
		Stack: map[string]*config.LifecycleEntry{
			"infra": {Compose: &config.ComposePluginConfig{Command: absentCompose}},
		},
	}

	cases := []struct {
		runner string
		exec   func(*config.Environment, []config.ProvisionItem) error
		// attempted reports whether the evidence shows the step's command was reached.
		attempted func(out string, err error) bool
	}{
		{
			runner: "local",
			exec:   (&LocalRunner{Cmd: &ResolvedCommand{}}).executeSteps,
			attempted: func(out string, _ error) bool {
				return strings.Contains(out, marker)
			},
		},
		{
			runner: "docker_compose",
			exec: (&DockerComposeRunner{
				Cmd:  &ResolvedCommand{},
				Opts: RunOptions{Config: composeCfg},
			}).executeSteps,
			attempted: func(_ string, err error) bool {
				return err != nil && strings.Contains(err.Error(), absentCompose)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.runner, func(t *testing.T) {
			t.Run("a note does not suppress the run", func(t *testing.T) {
				var err error
				out := captureStdout(t, func() {
					err = tc.exec(env, []config.ProvisionItem{
						{Step: label, Note: noteText, Run: runCommandText},
					})
				})
				if !strings.Contains(out, noteText) {
					t.Errorf("the note must still be printed; got %q", out)
				}
				if !tc.attempted(out, err) {
					t.Errorf("the note swallowed the run — no execution attempted; out=%q err=%v", out, err)
				}
			})

			t.Run("a note-only step still runs nothing", func(t *testing.T) {
				// The other half of the fix: dropping the `continue` must not turn a
				// documented manual step into something that executes.
				var err error
				out := captureStdout(t, func() {
					err = tc.exec(env, []config.ProvisionItem{{Step: label, Note: noteText}})
				})
				if !strings.Contains(out, noteText) {
					t.Errorf("the note must be printed; got %q", out)
				}
				if err != nil {
					t.Errorf("a note-only step must not fail; got %v", err)
				}
				if tc.attempted(out, err) {
					t.Errorf("a note-only step must execute nothing; out=%q err=%v", out, err)
				}
			})

			t.Run("run without a note is unchanged", func(t *testing.T) {
				var err error
				out := captureStdout(t, func() {
					err = tc.exec(env, []config.ProvisionItem{{Step: label, Run: runCommandText}})
				})
				if !tc.attempted(out, err) {
					t.Errorf("control failed: a plain run: must execute; out=%q err=%v", out, err)
				}
			})

			t.Run("the step is named once, not twice", func(t *testing.T) {
				// The note line already carries the label. Falling through to the plain
				// label print would report the same step twice.
				out := captureStdout(t, func() {
					_ = tc.exec(env, []config.ProvisionItem{
						{Step: label, Note: noteText, Run: runCommandText},
					})
				})
				if got := strings.Count(out, label); got != 1 {
					t.Errorf("step labelled %d times, want 1; got %q", got, out)
				}
			})
		})
	}
}
