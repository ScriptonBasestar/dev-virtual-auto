package runner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// composeArgv had no direct test before TASK-115 — which is how it carried the same two
// bugs as the three other copies of this builder for as long as it did. The delegation to
// dvaexec.ComposeArgv is covered there; what these cases pin is this function's own share
// of the work: the nil-config path, and that a rejected command reaches the caller as an
// error rather than a panic.

// argvTestConfig loads a config through config.Load rather than building the struct
// literally, because FileDir() reads an unexported path that only Load sets — and
// baseDir is half of what this builder does.
func argvTestConfig(t *testing.T, command string) *config.Config {
	t.Helper()
	dir := t.TempDir()
	yaml := fmt.Sprintf(`version: "0.1.22"
stack:
  infra:
    default_runner: compose
    runners:
      compose:
        command: %q
        files: [compose.yml]
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

func TestComposeArgv_NilConfig(t *testing.T) {
	env := config.NewEnvironment(nil, "/tmp", "/tmp")

	cmd, args, err := composeArgv(env, nil, "", []string{"ps"})
	if err != nil {
		t.Fatalf("composeArgv returned an error: %v", err)
	}
	if cmd != "docker" {
		t.Errorf("cmd = %q, want %q", cmd, "docker")
	}
	if got := strings.Join(args, " "); got != "compose ps" {
		t.Errorf("args = %q, want %q", got, "compose ps")
	}
}

// The seed bug, from the runner's side. `docker-compose` is one word, so the old code
// took the `len(parts) > 1` branch never and left "compose" in place: the user got
// `docker-compose compose ps`, a word they did not write.
func TestComposeArgv_SingleTokenCommandDropsSeed(t *testing.T) {
	env := config.NewEnvironment(nil, "/tmp", "/tmp")
	c := argvTestConfig(t, "docker-compose")

	cmd, args, err := composeArgv(env, c, "", []string{"ps"})
	if err != nil {
		t.Fatalf("composeArgv returned an error: %v", err)
	}
	if cmd != "docker-compose" {
		t.Errorf("cmd = %q, want %q", cmd, "docker-compose")
	}
	want := "-f " + filepath.Join(c.FileDir(), "compose.yml") + " ps"
	if got := strings.Join(args, " "); got != want {
		t.Errorf("args = %q, want %q — a leading 'compose' means the seed survived", got, want)
	}
}

// The panic, from the runner's side. `dva run <name>` on a config with a whitespace-only
// command used to produce a Go stack trace; it must produce an error naming the field.
func TestComposeArgv_RejectsCommandWithoutAWord(t *testing.T) {
	env := config.NewEnvironment(nil, "/tmp", "/tmp")
	for _, command := range []string{" ", "\t", "''"} {
		t.Run(strings.ReplaceAll(command, "\t", "<tab>"), func(t *testing.T) {
			c := argvTestConfig(t, command)

			_, _, err := composeArgv(env, c, "", []string{"ps"})
			if err == nil {
				t.Fatalf("composeArgv(%q) returned no error; before TASK-115 this input panicked", command)
			}
			if !strings.Contains(err.Error(), "command") {
				t.Errorf("error does not name the field:\n%s", err.Error())
			}
		})
	}
}
