// Package cli — regression tests for TASK-086.
//
// `note:` is the one key whose entire purpose is to be seen, and two of the three paths
// that execute a ProvisionItem never read it: executeParallelBatch and compose.go's
// native build loop. Marking a step `parallel: true` — a scheduling hint — therefore
// deleted its message while still printing its label, so the step looked like it had
// reported and only its content was gone.
//
// Every test here loads the task's own fixture through the real YAML loader rather than
// building ProvisionItem literals, so a rename or a parse change cannot leave the tests
// passing against a shape the config layer no longer produces.
package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// noteFixture is the fixture from tasks/.../086, verbatim. The sibling `run:` step is not
// decoration: it is the control that separates "the note is missing" from "the batch never
// ran", which is the reading the original measurement needed to rule out.
const noteFixture = `version: "0.1.44"
provision:
  sequential:
    - step: "a note on the sequential path"
      note: "SEQ-NOTE-VISIBLE"
  parallelbatch:
    - step: "a note on the parallel path"
      parallel: true
      note: "PAR-NOTE-VISIBLE"
    - step: "a second parallel item so a batch forms"
      parallel: true
      run: "echo PAR-CONTROL-RAN"
`

// loadNoteFixture writes a dva.yml, chdirs into it and returns the parsed config.
func loadNoteFixture(t *testing.T, body string) *config.Config {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "dva.yml"), []byte(body), 0644); err != nil {
		t.Fatal(err)
	}

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	oldDryRun := dryRun
	cfg, env = nil, nil
	t.Cleanup(func() {
		os.Chdir(oldWd)
		cfg, env = nil, nil
		dryRun = oldDryRun
	})
	return mustLoadConfig()
}

// TestParallelBatchPrintsNote is the core guard: both rows printed the label and dropped
// the note before the fix.
func TestParallelBatchPrintsNote(t *testing.T) {
	for _, tc := range []struct {
		name   string
		dryRun bool
		// wantRan is what proves the batch still did its work. Under --dry-run the command
		// is only echoed as a plan line, so the marker still appears — a note fix must not
		// be readable as "the batch stopped executing" in either mode.
		wantRan string
	}{
		{"executing", false, "$ echo PAR-CONTROL-RAN"},
		{"dry-run", true, "[dry-run] $ echo PAR-CONTROL-RAN"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := loadNoteFixture(t, noteFixture)
			batch := c.Provision.Profiles["parallelbatch"]
			if len(batch) != 2 {
				t.Fatalf("fixture parsed to %d steps, want 2", len(batch))
			}

			out := captureStdout(t, func() {
				if err := executeParallelBatch(loadEnv(c), c, batch, 0, len(batch), tc.dryRun); err != nil {
					t.Errorf("executeParallelBatch: %v", err)
				}
			})

			// The exact block, not just the substring: the note has to arrive rendered the
			// way the sequential path renders it, or the two paths have merely stopped
			// disagreeing about whether to print and started disagreeing about how.
			if want := "\n    PAR-NOTE-VISIBLE\n\n"; !strings.Contains(out, want) {
				t.Errorf("note block %q missing from parallel output:\n%s", want, out)
			}
			if !strings.Contains(out, tc.wantRan) {
				t.Errorf("%q missing: the batch stopped executing, so the note result is not readable\n%s", tc.wantRan, out)
			}
			// The label was never the problem — it printed all along. If it went missing,
			// the note was added by replacing the announcement rather than joining it.
			if !strings.Contains(out, "a note on the parallel path") {
				t.Errorf("step label missing from output:\n%s", out)
			}
		})
	}
}

// TestSequentialAndParallelNotesAgree pins the refactor itself. executeProvisionStep's
// inline note block was replaced by writeNote; the two paths must now emit the same bytes
// for the same note, which is what the task's byte-identical criterion asserts by diff
// against a pre-change capture and what this locks in for the future.
func TestSequentialAndParallelNotesAgree(t *testing.T) {
	c := loadNoteFixture(t, noteFixture)
	seq := c.Provision.Profiles["sequential"]
	if len(seq) != 1 {
		t.Fatalf("fixture parsed to %d sequential steps, want 1", len(seq))
	}

	seqOut := captureStdout(t, func() {
		if err := executeProvisionStep(loadEnv(c), c, seq[0], 0, 1, false); err != nil {
			t.Errorf("executeProvisionStep: %v", err)
		}
	})

	// Same note text through both renderers, compared as whole blocks.
	var direct strings.Builder
	writeNote(&direct, "SEQ-NOTE-VISIBLE")
	if !strings.Contains(seqOut, direct.String()) {
		t.Errorf("sequential path no longer renders the note the way writeNote does\nsequential:\n%q\nwriteNote:\n%q", seqOut, direct.String())
	}
	if direct.String() != "\n    SEQ-NOTE-VISIBLE\n\n" {
		t.Errorf("writeNote rendering drifted from the sequential reference: %q", direct.String())
	}
}

// TestWriteNoteHandlesMultilineAndEmpty covers the two shapes the call sites rely on: an
// absent note must produce nothing at all (every call site invokes writeNote
// unconditionally, so an empty note that printed blank lines would add them to every
// step), and a multi-line note must indent each line rather than only the first.
func TestWriteNoteHandlesMultilineAndEmpty(t *testing.T) {
	var empty strings.Builder
	writeNote(&empty, "")
	if empty.String() != "" {
		t.Errorf("an empty note produced %q; call sites pass it unconditionally", empty.String())
	}

	var multi strings.Builder
	writeNote(&multi, "first\nsecond")
	if want := "\n    first\n    second\n\n"; multi.String() != want {
		t.Errorf("multi-line note rendered %q, want %q", multi.String(), want)
	}
}

// TestNativeBuildLoopPrintsNote covers the third call site, and documents the one
// condition under which it runs at all.
//
// `dva build` is wrapped by wrapWithHooks, whose replace phase fires on exactly the same
// condition as compose.go's native-build branch (len(ic.Replace) > 0). The wrapper wins,
// so in a normal invocation runHookSteps prints the note — to stderr, two-space indented,
// after the commands — and compose.go's loop is never reached. It is reached when the
// wrapper's recursion guard trips, i.e. when `dva build` is invoked from inside a hook
// step, which is what DVA_HOOK_DEPTH simulates here. Filed as TASK-093; this test asserts
// the branch behaves correctly rather than asserting the duplication is acceptable.
func TestNativeBuildLoopPrintsNote(t *testing.T) {
	c := loadNoteFixture(t, `version: "0.1.44"
modes:
  nativemode:
    build: native
interaction:
  build:
    replace:
      - step: "a note on the native build path"
        note: "BUILD-NOTE-VISIBLE"
      - step: "a sibling that actually builds"
        run: "echo BUILD-CONTROL-RAN"
`)
	if ic := c.Interaction["build"]; ic == nil || len(ic.Replace) != 2 {
		t.Fatal("fixture did not parse into interaction.build.replace")
	}
	t.Setenv(config.EnvHookDepthKey, "1")

	out := captureStdout(t, func() {
		if err := buildCmd.RunE(buildCmd, []string{"--mode", "nativemode"}); err != nil {
			t.Errorf("dva build --mode nativemode: %v", err)
		}
	})

	if want := "\n    BUILD-NOTE-VISIBLE\n\n"; !strings.Contains(out, want) {
		t.Errorf("note block %q missing from the native build loop:\n%s", want, out)
	}
	// Control: the loop ran. Without it a note printed by a loop that had stopped
	// building would read as a pass.
	if !strings.Contains(out, "$ echo BUILD-CONTROL-RAN") {
		t.Errorf("the native build loop stopped running its commands:\n%s", out)
	}
}
