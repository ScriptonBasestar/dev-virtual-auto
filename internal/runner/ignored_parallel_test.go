package runner

import (
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// TestIgnoredParallelIsAnnouncedAtRuntime covers the half of TASK-140 that `validate`
// cannot reach.
//
// An inert step betrays itself: nothing happens, and the author goes looking. A step
// marked `parallel:` produces byte-identical output to the concurrent run the author
// expected and merely takes twice as long — measured, two `sleep 1` steps take 2.02s
// under `dva run` and 1.01s under `dva provision`, off one config. Nothing but this
// notice tells them the key was read and dropped.
//
// Every runner, for the reason TestStepWithoutRunIsReported gives: the three executeSteps
// share runStepLoop, and the criterion is that they take the same branch. A notice printed
// by one and not the others passes a single-runner test and fails this one.
func TestIgnoredParallelIsAnnouncedAtRuntime(t *testing.T) {
	env := &config.Environment{}
	cfg := composeConfig("echo")
	runners := stepRunners(t, cfg)

	for name, executeSteps := range runners {
		t.Run(name, func(t *testing.T) {
			t.Run("a parallel step draws the notice", func(t *testing.T) {
				out := captureStdout(t, func() {
					steps := []config.ProvisionItem{
						{Step: "a", Run: "true", Parallel: true},
						{Step: "b", Run: "true", Parallel: true},
					}
					if err := executeSteps(env, steps); err != nil {
						t.Fatalf("executeSteps: %v", err)
					}
				})
				if !strings.Contains(out, config.IgnoredParallelMessage) {
					t.Errorf("no notice printed for a parallel step; got %q", out)
				}
				// Once for the list, not once per step. The key describes how the list is
				// scheduled, so a per-step notice would grow with the config while repeating
				// one fact — and it would bury the step labels it sits among.
				if n := strings.Count(out, config.IgnoredParallelMessage); n != 1 {
					t.Errorf("notice printed %d times, want exactly 1:\n%s", n, out)
				}
			})

			t.Run("the steps still run, in order", func(t *testing.T) {
				out := captureStdout(t, func() {
					steps := []config.ProvisionItem{
						{Step: "first", Run: "true", Parallel: true},
						{Step: "second", Run: "true", Parallel: true},
					}
					if err := executeSteps(env, steps); err != nil {
						t.Fatalf("executeSteps: %v", err)
					}
				})
				// The notice reports a scheduling decision; it must not become an excuse to
				// skip the work, which is the silent-discard shape TASK-085 closed.
				fi, si := strings.Index(out, "first"), strings.Index(out, "second")
				if fi < 0 || si < 0 {
					t.Fatalf("a marked step went missing; got %q", out)
				}
				if fi > si {
					t.Errorf("steps ran out of declaration order; got %q", out)
				}
			})

			t.Run("a list with no parallel step is silent", func(t *testing.T) {
				out := captureStdout(t, func() {
					steps := []config.ProvisionItem{{Step: "a", Run: "true"}}
					if err := executeSteps(env, steps); err != nil {
						t.Fatalf("executeSteps: %v", err)
					}
				})
				if strings.Contains(out, config.IgnoredParallelMessage) {
					t.Errorf("notice printed for a config that never asked for concurrency: %q", out)
				}
			})
		})
	}
}
