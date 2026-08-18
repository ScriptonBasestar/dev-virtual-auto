package cli

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// useBufferedSlog points the default logger at a buffer for the duration of the test and
// restores it after. The orchestrator reads slog.Default() when it is constructed, so this
// must be in place before the run under test starts.
func useBufferedSlog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(old) })
	return &buf
}

// purgeFixture builds a one-entry plan plus a provision marker, and returns the loaded
// config, its environment and the marker path.
//
// The marker is the load-bearing artefact in every direction below: a declined prompt, an
// unanswerable one and a dry run must all leave it, and only a confirmed --purge may remove
// it. The script runner is `true` so the teardown itself is a no-op and every assertion is
// about the safeguard rather than about docker being installed.
func purgeFixture(t *testing.T) (*config.Config, *config.Environment, string) {
	t.Helper()
	c := loadTestConfig(t, `version: "0.1.44"
stack:
  demo:
    default_runner: script
    runners:
      script:
        up: "true"
        down: "true"
plans:
  demo:
    entries:
      - name: demo
`)
	markerDir := filepath.Join(c.FileDir(), config.DotDirName)
	if err := os.MkdirAll(markerDir, 0o755); err != nil {
		t.Fatalf("mkdir marker dir: %v", err)
	}
	marker := filepath.Join(markerDir, "provisioned-default")
	if err := os.WriteFile(marker, []byte("x"), 0o644); err != nil {
		t.Fatalf("write marker: %v", err)
	}
	return c, config.NewEnvironment(nil, c.FileDir(), c.FileDir()), marker
}

// TestPlanDownPurgeAsksBeforeDestroying is the safeguard `clean` carried, now on the command
// that replaces it. `down --volumes` never prompted and still does not; --purge is the
// stronger flag — images and provision markers go too — and it inherits clean's prompt
// rather than starting a new policy.
//
// The marker assertion proves the decline stopped the work rather than merely printing about
// it: clearProvisionMarkers runs a few lines below the prompt, so a gate that fell through
// would delete this file for real.
func TestPlanDownPurgeAsksBeforeDestroying(t *testing.T) {
	c, e, marker := purgeFixture(t)
	stdinFrom(t, "n\n")

	var err error
	stderr := captureBothStreams(t, func() { err = runPlanDown(c, e, "demo", []string{"--purge"}) })

	if err != nil {
		t.Fatalf("an answered decline is not a failure: %v", err)
	}
	for _, want := range []string{"VOLUMES (data loss!)", "locally built images", "Continue?", "Aborted."} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr is missing %q — --purge must say what it destroys:\n%s", want, stderr)
		}
	}
	if _, statErr := os.Stat(marker); statErr != nil {
		t.Errorf("the declined run deleted the provision marker anyway: %v", statErr)
	}
}

// TestPlanDownPurgeEOFIsNotADecline is TASK-171's decision applied to the new path: with no
// terminal, nobody declined, so reporting a decline would be inventing an answer. The
// assertion is on the returned error rather than on the output, because the output was never
// the problem — it is the exit code a script reads.
func TestPlanDownPurgeEOFIsNotADecline(t *testing.T) {
	c, e, marker := purgeFixture(t)
	stdinEOF(t)

	var err error
	stderr := captureBothStreams(t, func() { err = runPlanDown(c, e, "demo", []string{"--purge"}) })

	if err == nil {
		t.Fatalf("--purge with no terminal returned nil, so a script is told the volumes were "+
			"removed when nothing was. stderr:\n%s", stderr)
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Errorf("the error must name the way to proceed non-interactively, got: %v", err)
	}
	// The remedy has to be typeable. 'dva clean --force' was the old one; on this path the
	// plan name is part of the invocation and an error that omitted it would send the reader
	// to a command that does not exist.
	if !strings.Contains(err.Error(), "dva down demo --purge") {
		t.Errorf("the error must name this invocation, not clean's, got: %v", err)
	}
	if _, statErr := os.Stat(marker); statErr != nil {
		t.Errorf("the unanswerable prompt still deleted the provision marker: %v", statErr)
	}
}

// TestPlanDownPurgeDryRunSkipsThePromptAndPreviewsMarkers is TASK-170 on the new path.
//
// Note the absence of --force. Requiring it to reach a preview would mean the only flag that
// makes the preview reachable from a script is the flag that makes the real run unstoppable.
func TestPlanDownPurgeDryRunSkipsThePromptAndPreviewsMarkers(t *testing.T) {
	c, e, marker := purgeFixture(t)
	stdinEOF(t)

	var err error
	stderr := captureBothStreams(t, func() {
		err = runPlanDown(c, e, "demo", []string{"--purge", "--dry-run"})
	})

	if err != nil {
		t.Fatalf("dry run failed: %v", err)
	}
	for _, unwanted := range []string{"Continue?", "Aborted."} {
		if strings.Contains(stderr, unwanted) {
			t.Errorf("a preview must not ask consent for a deletion it will not perform, but "+
				"stderr has %q:\n%s", unwanted, stderr)
		}
	}
	// Assert the preview and not merely the prompt's absence: a run that returned early for
	// some unrelated reason would also print no prompt and would pass a test checking silence.
	if !strings.Contains(stderr, "would delete provision marker") {
		t.Errorf("the preview never ran, so skipping the prompt bought nothing:\n%s", stderr)
	}
	if _, statErr := os.Stat(marker); statErr != nil {
		t.Errorf("dry run deleted the provision marker: %v", statErr)
	}
}

// TestPlanDownPurgeForceRemovesMarkers is the control for all three tests above: they assert
// the marker survives, which a --purge that had quietly stopped clearing markers would also
// satisfy. This is the one run that must remove it.
func TestPlanDownPurgeForceRemovesMarkers(t *testing.T) {
	c, e, marker := purgeFixture(t)
	stdinEOF(t)

	var err error
	stderr := captureBothStreams(t, func() {
		err = runPlanDown(c, e, "demo", []string{"--purge", "--force"})
	})

	if err != nil {
		t.Fatalf("runPlanDown --purge --force failed: %v\n%s", err, stderr)
	}
	if strings.Contains(stderr, "Continue?") {
		t.Errorf("--force must answer the prompt, not raise it:\n%s", stderr)
	}
	if _, statErr := os.Stat(marker); !os.IsNotExist(statErr) {
		t.Errorf("--purge left the provision marker behind (stat err %v), so the next `up` skips "+
			"provisioning on a slate it was told was clean", statErr)
	}
}

// TestPlanDownPurgeRemovesVolumesAndImages is the assertion the marker tests cannot make.
// They prove --purge asked before acting and cleared the provision markers; none of them can
// tell `RemoveImages: flags.purge` from `RemoveImages: false`, because a script runner has no
// images. This reads the argv the compose plugin would run.
//
// --dry-run rather than a stub docker: buildArgs is the code under test and it runs in full
// on the dry-run path, so the argv asserted here is the argv a real run would execute.
func TestPlanDownPurgeRemovesVolumesAndImages(t *testing.T) {
	c := loadTestConfig(t, `version: "0.1.44"
stack:
  demo:
    default_runner: compose
    runners:
      compose:
        files: [compose.yml]
plans:
  demo:
    entries:
      - name: demo
`)
	if err := os.WriteFile(filepath.Join(c.FileDir(), "compose.yml"), []byte("services: {}\n"), 0o644); err != nil {
		t.Fatalf("write compose file: %v", err)
	}
	e := config.NewEnvironment(nil, c.FileDir(), c.FileDir())

	// NewPlanOrchestrator captures slog.Default() at construction, so redirecting it here
	// reaches the dry-run line. os.Stderr cannot be swapped for this: the default handler
	// holds the writer the log package captured at init, not the current value of the
	// variable, so captureBothStreams sees nothing of it.
	logs := useBufferedSlog(t)

	if err := runPlanDown(c, e, "demo", []string{"--purge", "--force", "--dry-run"}); err != nil {
		t.Fatalf("runPlanDown failed: %v", err)
	}

	out := logs.String()
	for _, want := range []string{"--volumes", "--rmi", "local"} {
		if !strings.Contains(out, want) {
			t.Errorf("compose argv is missing %q, so --purge is not the replacement for "+
				"`clean --volumes --images` it is documented to be:\n%s", want, out)
		}
	}
}

// TestPlanUpRejectsDownOnlyFlags: a destructive flag that is parsed and ignored is worse than
// one that is unknown. `dva up demo --purge` used to be impossible only because --purge did
// not exist; parsePlanFlags is shared by every plan verb, so adding it there made it valid
// everywhere at once.
//
// --volumes is in the same table. restart already rejected it while up and stop dropped it
// silently, which is the gap this closes rather than a second flag added to it.
func TestPlanUpRejectsDownOnlyFlags(t *testing.T) {
	for _, tc := range []struct {
		name string
		run  func(*config.Config, *config.Environment, []string) error
	}{
		{"up", func(c *config.Config, e *config.Environment, a []string) error { return runPlanUp(c, e, "demo", a) }},
		{"stop", func(c *config.Config, e *config.Environment, a []string) error { return runPlanStop(c, e, "demo", a) }},
		{"restart", func(c *config.Config, e *config.Environment, a []string) error {
			return runPlanRestart(c, e, "demo", a)
		}},
	} {
		for _, flag := range []string{"--purge", "--volumes"} {
			t.Run(tc.name+" "+flag, func(t *testing.T) {
				c, e, _ := purgeFixture(t)

				var err error
				captureBothStreams(t, func() { err = tc.run(c, e, []string{flag}) })

				if err == nil {
					t.Fatalf("%s accepted %s and did something else instead", tc.name, flag)
				}
				if !strings.Contains(err.Error(), flag) {
					t.Errorf("the error must name the flag it rejected, got: %v", err)
				}
			})
		}
	}
}
