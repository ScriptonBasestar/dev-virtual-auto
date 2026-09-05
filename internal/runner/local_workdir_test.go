package runner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// workdirFixture writes a loadable dva.yml into a fresh directory with a `sub/` child and
// returns the loaded config plus the symlink-resolved fixture root (macOS TempDir lives under
// /var, which pwd reports as /private/var).
func workdirFixture(t *testing.T) (*config.Config, string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0755); err != nil {
		t.Fatal(err)
	}
	yaml := fmt.Sprintf("version: %q\ninteraction:\n  x:\n    runner: local\n    workdir: sub\n    command: pwd\n", config.Version)
	if err := os.WriteFile(filepath.Join(dir, config.FileName), []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	real, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	return cfg, real
}

// TestLocalRunnerWorkdir covers TASK-313: interaction.workdir was consumed only by the compose
// runner (`--workdir` inside the container) and silently ignored on the host, so
// `x: {runner: local, workdir: sub, command: pwd}` printed the project root. A relative
// workdir is anchored at the dva.yml directory, not at the caller's cwd.
func TestLocalRunnerWorkdir(t *testing.T) {
	cfg, root := workdirFixture(t)
	// Invoke from a directory that is neither the config dir nor the workdir, so a runner that
	// anchored at cwd (or ignored workdir) is told apart from one that anchors at the config.
	elsewhere := t.TempDir()
	t.Chdir(elsewhere)

	env := config.NewEnvironment(nil, root, root)
	out := filepath.Join(root, "pwd.txt")
	r := &LocalRunner{
		Cmd: &ResolvedCommand{
			Workdir:      "sub",
			CommandLines: []string{"pwd > " + out},
			Shell:        true,
		},
		Opts: RunOptions{Config: cfg},
	}
	if err := r.Execute(env); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read pwd capture: %v", err)
	}
	if want := filepath.Join(root, "sub"); strings.TrimSpace(string(got)) != want {
		t.Fatalf("local runner ran in %q, want workdir %q", strings.TrimSpace(string(got)), want)
	}
}

// TestLocalRunnerWorkdirMissing: a workdir that does not exist is a config error named as
// such, not a shell "No such file" from inside the command.
func TestLocalRunnerWorkdirMissing(t *testing.T) {
	cfg, root := workdirFixture(t)
	env := config.NewEnvironment(nil, root, root)
	r := &LocalRunner{
		Cmd:  &ResolvedCommand{Workdir: "nope", CommandLines: []string{"true"}, Shell: true},
		Opts: RunOptions{Config: cfg},
	}
	err := r.Execute(env)
	if err == nil {
		t.Fatal("Execute succeeded with a missing workdir")
	}
	for _, want := range []string{`workdir "nope"`, "directory not found", filepath.Join(cfg.FileDir(), "nope")} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q does not mention %q", err, want)
		}
	}
}

// TestComposeScriptFallbackDropsWorkdir: the compose runner's script fallback runs on the host,
// where the container `--workdir` path means nothing; it must not chdir (and must not fail).
func TestComposeScriptFallbackDropsWorkdir(t *testing.T) {
	cfg, root := workdirFixture(t)
	env := config.NewEnvironment(nil, root, root)
	r := &DockerComposeRunner{
		Cmd:  &ResolvedCommand{Service: "web", Workdir: "/app", Script: "true"},
		Opts: RunOptions{Config: cfg},
	}
	if err := r.Execute(env); err != nil {
		t.Fatalf("script fallback failed on container workdir: %v", err)
	}
}
