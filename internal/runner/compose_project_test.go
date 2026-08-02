package runner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// Nothing asserted on --project-name before TASK-132, and two places were emitting it:
// dvaexec.ComposeArgv from the config's project_name, and DockerComposeRunner from the project
// it had detected on a running container. Both fired whenever a config declared a name and the
// service was already up — the ordinary dev-loop state — so the argv carried
// `--project-name X --project-name X`, visible to the user in any failing step's error message.
//
// The behaviour was nonetheless correct, because docker takes the last occurrence and the last
// occurrence was the detected one. That is what makes these cases worth having: the right value
// won by argv ordering, and nothing anywhere said so. Reorder the appends and the wrong project
// silently wins, with no test to notice.
//
// So each case checks both halves — that the flag appears exactly once, and which value
// survived — on both paths that build an invocation.

// projectTestConfig loads a config whose compose runner declares projectName. Loaded through
// config.Load rather than built as a literal because FileDir() reads an unexported field that
// only Load sets, and ComposeArgv resolves the -f paths against it.
func projectTestConfig(t *testing.T, projectName string) *config.Config {
	t.Helper()
	dir := t.TempDir()
	yaml := fmt.Sprintf(`version: "0.1.22"
stack:
  infra:
    default_runner: compose
    runners:
      compose:
        files: [compose.yml]
        project_name: %q
`, projectName)
	if err := os.WriteFile(filepath.Join(dir, config.FileName), []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}
	c, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// projectRunner builds a runner with detected already set, standing in for the state
// autoDetectComposeMethod leaves behind when it finds the service running. Nothing here
// contacts docker: executeArgs and buildStepArgs are pure builders.
func projectRunner(detected string) *DockerComposeRunner {
	return &DockerComposeRunner{
		Cmd: &ResolvedCommand{
			Service: "app",
			Command: "rspec",
			Compose: ComposeOpts{Method: "exec"},
		},
		detectedProject: detected,
	}
}

// projectNames returns the value following every --project-name in argv, in argv order. The
// count matters as much as the values: a length of 2 is the defect, even when both entries
// hold the same string.
func projectNames(argv []string) []string {
	var out []string
	for i := 0; i+1 < len(argv); i++ {
		if argv[i] == "--project-name" {
			out = append(out, argv[i+1])
		}
	}
	return out
}

// assertSoleProject checks the flag appears once and carries want.
func assertSoleProject(t *testing.T, argv []string, want string) {
	t.Helper()
	got := projectNames(argv)
	if len(got) != 1 {
		t.Fatalf("--project-name appears %d times, want 1: %v\nargv: %s",
			len(got), got, strings.Join(argv, " "))
	}
	if got[0] != want {
		t.Errorf("--project-name = %q, want %q\nargv: %s", got[0], want, strings.Join(argv, " "))
	}
}

// The dev-loop case: a config names its project and the container is already running, so both
// emitters used to fire. The detected name must win, because it is the project the running
// container actually belongs to.
func TestProjectNameNotDuplicatedWhenDetectionAgreesWithConfig(t *testing.T) {
	env := config.NewEnvironment(nil, "/tmp/dva-work", "/tmp/dva-work")
	cfg := projectTestConfig(t, "declared-project")
	r := projectRunner("detected-project")

	t.Run("execute", func(t *testing.T) {
		_, argv, err := composeArgv(env, cfg, r.detectedProject, r.executeArgs(env))
		if err != nil {
			t.Fatalf("composeArgv returned an error: %v", err)
		}
		assertSoleProject(t, argv, "detected-project")
	})

	t.Run("steps", func(t *testing.T) {
		_, argv, err := composeArgv(env, cfg, r.detectedProject, r.buildStepArgs(env, "rspec"))
		if err != nil {
			t.Fatalf("composeArgv returned an error: %v", err)
		}
		assertSoleProject(t, argv, "detected-project")
	})
}

// Detection found nothing — the service is not running, or the interaction never detects. The
// config's name is then the only one there is, and it must still reach the argv: an override
// that replaces unconditionally would have deleted it.
func TestProjectNameFallsBackToConfigWithoutDetection(t *testing.T) {
	env := config.NewEnvironment(nil, "/tmp/dva-work", "/tmp/dva-work")
	cfg := projectTestConfig(t, "declared-project")
	r := projectRunner("")

	_, argv, err := composeArgv(env, cfg, r.detectedProject, r.executeArgs(env))
	if err != nil {
		t.Fatalf("composeArgv returned an error: %v", err)
	}
	assertSoleProject(t, argv, "declared-project")
}

// The reverse: no project_name in config, but a container was detected. The detected name has
// to survive on its own, not only as an override of something already present.
func TestProjectNameUsesDetectionWhenConfigDeclaresNone(t *testing.T) {
	env := config.NewEnvironment(nil, "/tmp/dva-work", "/tmp/dva-work")
	cfg := projectTestConfig(t, "")
	r := projectRunner("detected-project")

	_, argv, err := composeArgv(env, cfg, r.detectedProject, r.executeArgs(env))
	if err != nil {
		t.Fatalf("composeArgv returned an error: %v", err)
	}
	assertSoleProject(t, argv, "detected-project")
}

// The override must not write itself back into the shared config. One interaction's detection
// result becoming another's declared project_name would be a far worse bug than the duplicate
// flag, and a shallow copy is the only thing preventing it.
func TestProjectOverrideDoesNotMutateConfig(t *testing.T) {
	env := config.NewEnvironment(nil, "/tmp/dva-work", "/tmp/dva-work")
	cfg := projectTestConfig(t, "declared-project")

	if _, _, err := composeArgv(env, cfg, "detected-project", []string{"ps"}); err != nil {
		t.Fatalf("composeArgv returned an error: %v", err)
	}

	if got := cfg.PrimaryComposeConfig().ProjectName; got != "declared-project" {
		t.Errorf("config project_name = %q after an override, want %q — the override wrote through", got, "declared-project")
	}
}

// A project name only ever reaches the argv through the shared builder now. Without a config
// and without detection there is nothing to name, and the flag must be absent rather than
// present with an empty value — `--project-name ""` is not the same command.
func TestProjectNameAbsentWithoutConfigOrDetection(t *testing.T) {
	env := config.NewEnvironment(nil, "/tmp/dva-work", "/tmp/dva-work")
	r := projectRunner("")

	_, argv, err := composeArgv(env, nil, r.detectedProject, r.executeArgs(env))
	if err != nil {
		t.Fatalf("composeArgv returned an error: %v", err)
	}
	if got := projectNames(argv); len(got) != 0 {
		t.Errorf("--project-name appears %d times, want 0: %v\nargv: %s",
			len(got), got, strings.Join(argv, " "))
	}
}
