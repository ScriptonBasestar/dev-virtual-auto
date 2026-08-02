package runner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// serviceRunningProject decides whether `dva run` exec's into the container that is up or
// starts a throwaway one, and until TASK-133 it decided that from a bare
// `docker compose ps` — no -f, no --project-name, and a hardcoded `docker` binary. Every other
// compose call in the tree goes through ComposeArgv, so the question "is this service running?"
// was asked about whatever project the working directory implied, and the answer was used on
// the configured one.
//
// A false negative is silent, which is what makes it worth testing rather than noticing: an
// empty answer reads as "not running", `method: run` stands, and the command succeeds in a
// fresh container from the image with --rm deleting it afterwards. Measured on a fixture whose
// compose file is reached through `files:` — container up as dva-task133-app-1
// (hostname b27734163afa), bare ps exits 1 with "no configuration file provided", and the
// interaction printed ebf2997233b0 from a container it had just created.
//
// So these cases assert on the detection argv, which is where the defect lives and the only
// place it is visible without a daemon.

// detectTestConfig loads a config declaring files, a project name and a compose binary — the
// three things the old detection dropped. Loaded through config.Load because FileDir(), which
// resolves the -f paths, is only set there.
func detectTestConfig(t *testing.T, command string) *config.Config {
	t.Helper()
	dir := t.TempDir()
	yaml := fmt.Sprintf(`version: "0.1.22"
stack:
  infra:
    default_runner: compose
    runners:
      compose:
        files: [compose/docker-compose.yml]
        project_name: declared-project
        command: %q
`, command)
	if err := os.WriteFile(filepath.Join(dir, config.FileName), []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}
	c, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func detectRunner(cfg *config.Config) *DockerComposeRunner {
	return &DockerComposeRunner{
		Cmd: &ResolvedCommand{
			Service: "app",
			Command: "hostname",
			Compose: ComposeOpts{Method: "run"},
		},
		Opts: RunOptions{Config: cfg},
	}
}

// The whole defect in one case: the query must name the same project, the same compose file and
// the same binary the invocation it informs will use.
func TestDetectArgvCarriesTheConfiguredProject(t *testing.T) {
	env := config.NewEnvironment(nil, "/tmp/dva-work", "/tmp/dva-work")
	cfg := detectTestConfig(t, "docker")
	r := detectRunner(cfg)

	cmd, args, err := r.detectArgv(env)
	if err != nil {
		t.Fatalf("detectArgv returned an error: %v", err)
	}
	argv := strings.Join(args, " ")

	wantFile := filepath.Join(cfg.FileDir(), "compose/docker-compose.yml")
	if !strings.Contains(argv, "-f "+wantFile) {
		t.Errorf("detection does not name the configured compose file %q\nargv: %s %s", wantFile, cmd, argv)
	}
	if !strings.Contains(argv, "--project-name declared-project") {
		t.Errorf("detection does not name the configured project\nargv: %s %s", cmd, argv)
	}
	if !strings.HasSuffix(argv, "ps --filter status=running --format {{.Project}} app") {
		t.Errorf("detection does not end in the ps query for the service\nargv: %s %s", cmd, argv)
	}
}

// The binary is the third dropped field and the one a test is most likely to miss, because the
// default makes it invisible: `docker` is right by accident for everyone who has not configured
// something else.
func TestDetectArgvUsesTheConfiguredBinary(t *testing.T) {
	env := config.NewEnvironment(nil, "/tmp/dva-work", "/tmp/dva-work")
	r := detectRunner(detectTestConfig(t, "podman-compose"))

	cmd, args, err := r.detectArgv(env)
	if err != nil {
		t.Fatalf("detectArgv returned an error: %v", err)
	}
	if cmd != "podman-compose" {
		t.Errorf("detection runs %q, want %q — it asks a different tool than the one that executes\nargv: %s %s",
			cmd, "podman-compose", cmd, strings.Join(args, " "))
	}
}

// Detection passes no override, because detection is what produces one. Were it to pass the
// project it has not detected yet, the flag would be empty rather than the config's.
func TestDetectArgvPassesNoOverride(t *testing.T) {
	env := config.NewEnvironment(nil, "/tmp/dva-work", "/tmp/dva-work")
	r := detectRunner(detectTestConfig(t, "docker"))
	r.detectedProject = "stale-from-a-previous-call"

	_, args, err := r.detectArgv(env)
	if err != nil {
		t.Fatalf("detectArgv returned an error: %v", err)
	}
	if got := projectNames(args); len(got) != 1 || got[0] != "declared-project" {
		t.Errorf("detection names %v, want exactly [declared-project]\nargv: %s", got, strings.Join(args, " "))
	}
}

// Without a config there is nothing to name, and detection has to stay the plain default rather
// than fail: an interaction can run outside a loaded project.
func TestDetectArgvWithoutConfig(t *testing.T) {
	env := config.NewEnvironment(nil, "/tmp/dva-work", "/tmp/dva-work")
	r := detectRunner(nil)

	cmd, args, err := r.detectArgv(env)
	if err != nil {
		t.Fatalf("detectArgv returned an error: %v", err)
	}
	if cmd != "docker" {
		t.Errorf("cmd = %q, want %q", cmd, "docker")
	}
	want := "compose ps --filter status=running --format {{.Project}} app"
	if got := strings.Join(args, " "); got != want {
		t.Errorf("args = %q, want %q", got, want)
	}
}
