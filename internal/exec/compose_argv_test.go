// Package exec — tests for the consolidated compose argv builder (TASK-115).
//
// The four builders this replaces each carried the same two bugs, so the cases that matter
// most here are the two that used to be wrong everywhere: a single-token `command:` left the
// "compose" seed in place, and a `command:` that splits to nothing indexed into a nil slice.
package exec

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

func TestComposeArgv(t *testing.T) {
	base := "/work/proj"
	cases := []struct {
		name     string
		cc       *config.ComposePluginConfig
		wantCmd  string
		wantArgs []string
	}{
		{
			name: "no config falls back to docker compose",
			cc:   nil, wantCmd: "docker", wantArgs: []string{"compose"},
		},
		{
			name: "no command keeps the subcommand seed",
			cc:   &config.ComposePluginConfig{}, wantCmd: "docker", wantArgs: []string{"compose"},
		},
		{
			// The regression. Before: podman-compose **compose** … — a word the user never
			// wrote, from a tool they had configured correctly.
			name:    "single-token command drops the seed",
			cc:      &config.ComposePluginConfig{Command: "podman-compose"},
			wantCmd: "podman-compose", wantArgs: nil,
		},
		{
			name:    "single-token docker-compose drops the seed",
			cc:      &config.ComposePluginConfig{Command: "docker-compose"},
			wantCmd: "docker-compose", wantArgs: nil,
		},
		{
			name:    "multi-token command keeps its own tail",
			cc:      &config.ComposePluginConfig{Command: "podman compose"},
			wantCmd: "podman", wantArgs: []string{"compose"},
		},
		{
			name:    "multi-token command with flags",
			cc:      &config.ComposePluginConfig{Command: "docker --context remote compose"},
			wantCmd: "docker", wantArgs: []string{"--context", "remote", "compose"},
		},
		{
			name:    "relative files resolve against baseDir",
			cc:      &config.ComposePluginConfig{Files: []string{"compose.yml", "override.yml"}},
			wantCmd: "docker",
			wantArgs: []string{
				"compose",
				"-f", filepath.Join(base, "compose.yml"),
				"-f", filepath.Join(base, "override.yml"),
			},
		},
		{
			name:    "absolute files are left alone",
			cc:      &config.ComposePluginConfig{Files: []string{"/etc/compose.yml"}},
			wantCmd: "docker", wantArgs: []string{"compose", "-f", "/etc/compose.yml"},
		},
		{
			// filepath.Join also cleans, which is the behaviour the one builder that
			// already used it produced; the other three concatenated and did not.
			name:    "dot-relative files are cleaned",
			cc:      &config.ComposePluginConfig{Files: []string{"./compose.yml"}},
			wantCmd: "docker", wantArgs: []string{"compose", "-f", filepath.Join(base, "compose.yml")},
		},
		{
			name:    "project name is appended",
			cc:      &config.ComposePluginConfig{ProjectName: "myapp"},
			wantCmd: "docker", wantArgs: []string{"compose", "--project-name", "myapp"},
		},
		{
			name: "command, files and project name together",
			cc: &config.ComposePluginConfig{
				Command: "podman-compose", Files: []string{"compose.yml"}, ProjectName: "myapp",
			},
			wantCmd:  "podman-compose",
			wantArgs: []string{"-f", filepath.Join(base, "compose.yml"), "--project-name", "myapp"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := config.NewEnvironment(nil, base, base)
			cmd, args, err := ComposeArgv(env, tc.cc, base)
			if err != nil {
				t.Fatalf("ComposeArgv returned an error: %v", err)
			}
			if cmd != tc.wantCmd {
				t.Errorf("cmd = %q, want %q", cmd, tc.wantCmd)
			}
			if strings.Join(args, " ") != strings.Join(tc.wantArgs, " ") {
				t.Errorf("args = %v, want %v", args, tc.wantArgs)
			}
		})
	}
}

// TestComposeArgvInterpolates covers the one thing the builder does that is not
// concatenation: files and the project name go through the environment first.
func TestComposeArgvInterpolates(t *testing.T) {
	base := "/work/proj"
	env := config.NewEnvironment(map[string]string{"APP_NAME": "myapp", "STAGE": "dev"}, base, base)
	cc := &config.ComposePluginConfig{
		Files:       []string{"compose.${STAGE}.yml"},
		ProjectName: "${APP_NAME}",
	}

	_, args, err := ComposeArgv(env, cc, base)
	if err != nil {
		t.Fatalf("ComposeArgv returned an error: %v", err)
	}
	joined := strings.Join(args, " ")
	for _, want := range []string{filepath.Join(base, "compose.dev.yml"), "--project-name myapp"} {
		if !strings.Contains(joined, want) {
			t.Errorf("args do not contain %q:\n%s", want, joined)
		}
	}
}

// TestComposeArgvRejectsCommandWithoutAWord is the panic. The guard used to be
// `Command != ""`, which is true for every input below, while SplitCommand returns nil for
// each of them — so parts[0] produced a Go stack trace where a config error belonged. A
// YAML author who writes an empty quoted string, or who deletes a value and leaves whitespace
// behind, must get a message.
func TestComposeArgvRejectsCommandWithoutAWord(t *testing.T) {
	base := "/work/proj"
	for _, command := range []string{" ", "   ", "\t", " \t ", "''", `""`, `'' ""`} {
		t.Run(strings.ReplaceAll(command, "\t", "<tab>"), func(t *testing.T) {
			env := config.NewEnvironment(nil, base, base)
			_, _, err := ComposeArgv(env, &config.ComposePluginConfig{Command: command}, base)
			if err == nil {
				t.Fatalf("ComposeArgv(%q) returned no error; before TASK-115 this input panicked", command)
			}
			if !strings.Contains(err.Error(), "command") {
				t.Errorf("error does not name the field:\n%s", err.Error())
			}
		})
	}
}
