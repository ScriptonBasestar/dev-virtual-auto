package runner

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// captureStdout redirects os.Stdout for the duration of fn. executeSteps reports through
// fmt.Printf rather than a writer it is handed, so intercepting the file is the only way
// to see what a caller would see.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("close pipe: %v", err)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read pipe: %v", err)
	}
	return string(out)
}

// TestStepWithoutRunIsReported covers TASK-083: a hook or provision item written as
// `- step: "make build"` is a label with no commands, and both runners used to reach their
// emptiness check and `continue` without printing anything at all — so the step vanished,
// and the profile reported success.
//
// The table runs through EVERY runner on purpose. The three executeSteps differ only in how they
// hand a command off — since TASK-094 they literally share one loop, runStepLoop — and the task's
// acceptance criterion is that they take the same branch; a fix applied to one and not the others
// passes a single-runner test and fails this one.
func TestStepWithoutRunIsReported(t *testing.T) {
	env := &config.Environment{}

	// The compose command is `echo`, so the compose_up case below is observable without
	// contacting docker. Before TASK-085 these runners ignored compose_up entirely and a nil
	// config was harmless; now they execute it, and a nil config would have meant a real
	// `docker compose up -d postgres` from a unit test.
	cfg := composeConfig("echo")
	runners := stepRunners(t, cfg)

	for name, executeSteps := range runners {
		t.Run(name, func(t *testing.T) {
			t.Run("a label with no run: is reported, not skipped in silence", func(t *testing.T) {
				out := captureStdout(t, func() {
					if err := executeSteps(env, []config.ProvisionItem{{Step: "make build"}}); err != nil {
						t.Fatalf("executeSteps: %v", err)
					}
				})
				// Naming the step matters as much as the notice: a profile can hold many, and
				// "something did nothing" is not actionable without saying which.
				if !strings.Contains(out, "make build") {
					t.Errorf("the report must name the step; got %q", out)
				}
				if !strings.Contains(out, config.InertStepMessage) {
					t.Errorf("no inert notice printed; got %q", out)
				}
			})

			t.Run("an item with a note is not inert", func(t *testing.T) {
				out := captureStdout(t, func() {
					steps := []config.ProvisionItem{{Step: "Manual step", Note: "rotate the token first"}}
					if err := executeSteps(env, steps); err != nil {
						t.Fatalf("executeSteps: %v", err)
					}
				})
				// The documented label-only form (examples/provision-step-syntax.yml:62-63).
				// Reporting it would punish the shape the docs tell people to write.
				if strings.Contains(out, config.InertStepMessage) {
					t.Errorf("a note is a payload; it must not be reported as inert: %q", out)
				}
				if !strings.Contains(out, "rotate the token first") {
					t.Errorf("the note must still print; got %q", out)
				}
			})

			t.Run("a compose_up item is a payload, not an inert label", func(t *testing.T) {
				out := captureStdout(t, func() {
					steps := []config.ProvisionItem{{Step: "Start db", ComposeUp: []string{"postgres"}}}
					if err := executeSteps(env, steps); err != nil {
						t.Fatalf("executeSteps: %v", err)
					}
				})
				// compose_up is a payload, so the item is not inert. This is the case that
				// fails if IsInert is narrowed to "no run commands" — it would libel a
				// working config.
				//
				// This subtest was named "…whose payload these runners do not handle…" until
				// TASK-085, when the runners started handling it. The inert assertion was
				// always the point and is unchanged; what is new is that the step now
				// actually runs, so the test asserts that too rather than only what must
				// not appear.
				if strings.Contains(out, config.InertStepMessage) {
					t.Errorf("compose_up is a payload; got %q", out)
				}
				if !strings.Contains(out, "compose up -d postgres") {
					t.Errorf("compose_up must now execute, not merely avoid the notice; got %q", out)
				}
			})
		})
	}

	// Only LocalRunner: the compose runner would hand `true` to `docker compose exec` and
	// fail for reasons that have nothing to do with this task.
	t.Run("local/a step that runs something is not reported", func(t *testing.T) {
		out := captureStdout(t, func() {
			r := &LocalRunner{Cmd: &ResolvedCommand{}}
			// `true` is the least eventful command available: no output, no side effect.
			if err := r.executeSteps(env, []config.ProvisionItem{{Step: "Building", Run: "true"}}); err != nil {
				t.Fatalf("executeSteps: %v", err)
			}
		})
		if strings.Contains(out, config.InertStepMessage) {
			t.Errorf("a step with run: must never be reported as inert; got %q", out)
		}
		if !strings.Contains(out, "Building") {
			t.Errorf("the executing form still prints its label; got %q", out)
		}
	})
}
