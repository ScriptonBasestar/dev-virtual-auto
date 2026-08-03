package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// Hooks are the second executor of a []ProvisionItem. `steps:` goes through
// runner.runStepLoop, which internal/runner tests cover; `before:`/`replace:`/`after:` go
// through runHookSteps here, and the first cut of TASK-140 warned only in the first place.
// `dva up` with a parallel-marked before-hook then ran at half the advertised speed in
// silence while `dva validate` warned about it — which inverts the whole argument for having
// a runtime notice, since validate is the surface an author may never visit.
//
// Deliberately driving runHookSteps and reading os.Stderr rather than asserting on
// config.StepsIgnoreParallel: the predicate is not the thing that regressed, the call to it
// is. A test of the predicate alone stays green when this file's call is deleted.
func TestHookStepsAnnounceIgnoredParallel(t *testing.T) {
	c := loadTestConfig(t, hookTestConfig)
	e := config.NewEnvironment(nil, c.FileDir(), c.FileDir())

	marked := []config.ProvisionItem{
		{Step: "prep", Run: "echo PREP", Parallel: true},
		{Step: "warm", Run: "echo WARM", Parallel: true},
	}
	plain := []config.ProvisionItem{
		{Step: "prep", Run: "echo PREP"},
		{Step: "warm", Run: "echo WARM"},
	}

	for _, tc := range []struct {
		name  string
		steps []config.ProvisionItem
		want  int
	}{
		{"a parallel step draws the notice", marked, 1},
		// Once for the list, not once per marked step: both steps above ask for it.
		{"no parallel step, no notice", plain, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			old := os.Stderr
			r, w, _ := os.Pipe()
			os.Stderr = w

			err := runHookSteps(e, c, "before", "up", tc.steps)

			w.Close()
			os.Stderr = old
			var buf bytes.Buffer
			buf.ReadFrom(r)
			out := buf.String()

			if err != nil {
				t.Fatalf("runHookSteps: %v", err)
			}
			if got := strings.Count(out, config.IgnoredParallelMessage); got != tc.want {
				t.Errorf("notice count = %d, want %d\n%s", got, tc.want, out)
			}
			// The notice must not turn into a reason to skip the work — that is the
			// silent-discard shape TASK-085 fixed. Both hooks still announce themselves,
			// in declaration order.
			prep := strings.Index(out, "prep")
			warm := strings.Index(out, "warm")
			if prep < 0 || warm < 0 || prep > warm {
				t.Errorf("hook steps did not both run in declaration order:\n%s", out)
			}
		})
	}
}
