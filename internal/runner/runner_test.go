package runner

import (
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

func TestInteractionTreeFind(t *testing.T) {
	entries := map[string]*config.InteractionCommand{
		"shell": {
			Description: "Open shell",
			Service:     "app",
			Command:     "/bin/bash",
		},
		"test": {
			Description: "Run tests",
			Service:     "app",
			Command:     "bundle exec rspec",
			Subcommands: map[string]*config.InteractionCommand{
				"lint": {
					Command: "bundle exec rubocop",
				},
			},
		},
	}

	tree := NewInteractionTree(entries)

	t.Run("simple command", func(t *testing.T) {
		cmd := tree.Find("shell")
		if cmd == nil {
			t.Fatal("expected to find 'shell'")
		}
		if cmd.Command != "/bin/bash" {
			t.Errorf("command = %s, want /bin/bash", cmd.Command)
		}
		if cmd.Service != "app" {
			t.Errorf("service = %s, want app", cmd.Service)
		}
	})

	t.Run("not found", func(t *testing.T) {
		cmd := tree.Find("nonexistent")
		if cmd != nil {
			t.Error("expected nil for nonexistent command")
		}
	})

	t.Run("subcommand", func(t *testing.T) {
		cmd := tree.Find("test", "lint")
		if cmd == nil {
			t.Fatal("expected to find 'test lint'")
		}
		if cmd.Command != "bundle exec rubocop" {
			t.Errorf("command = %s, want bundle exec rubocop", cmd.Command)
		}
		// Should inherit parent's service
		if cmd.Service != "app" {
			t.Errorf("service = %s, want app (inherited from parent)", cmd.Service)
		}
	})

	t.Run("command with extra args", func(t *testing.T) {
		cmd := tree.Find("shell", "extra", "args")
		if cmd == nil {
			t.Fatal("expected to find 'shell' with extra args")
		}
		if len(cmd.Argv) != 2 || cmd.Argv[0] != "extra" || cmd.Argv[1] != "args" {
			t.Errorf("Argv = %v, want [extra args]", cmd.Argv)
		}
	})
}

func TestInteractionTreeList(t *testing.T) {
	entries := map[string]*config.InteractionCommand{
		"shell": {
			Description: "Open shell",
			Service:     "app",
			Command:     "/bin/bash",
		},
		"test": {
			Description: "Run tests",
			Service:     "app",
			Command:     "bundle exec rspec",
			Subcommands: map[string]*config.InteractionCommand{
				"lint": {
					Description: "Run linter",
					Command:     "bundle exec rubocop",
				},
			},
		},
	}

	tree := NewInteractionTree(entries)
	list := tree.List()

	// Should have 3 entries: shell, test, test lint
	if len(list) != 3 {
		t.Errorf("list count = %d, want 3", len(list))
	}

	if _, ok := list["shell"]; !ok {
		t.Error("missing 'shell' in list")
	}
	if _, ok := list["test"]; !ok {
		t.Error("missing 'test' in list")
	}
	if _, ok := list["test lint"]; !ok {
		t.Error("missing 'test lint' in list")
	}
}

func TestNewRunnerSelection(t *testing.T) {
	t.Run("docker compose runner", func(t *testing.T) {
		cmd := &ResolvedCommand{Service: "app"}
		r := NewRunner(cmd, RunOptions{})
		if _, ok := r.(*DockerComposeRunner); !ok {
			t.Errorf("expected DockerComposeRunner, got %T", r)
		}
	})

	t.Run("kubectl runner", func(t *testing.T) {
		cmd := &ResolvedCommand{Pod: "my-pod:container"}
		r := NewRunner(cmd, RunOptions{})
		if _, ok := r.(*KubectlRunner); !ok {
			t.Errorf("expected KubectlRunner, got %T", r)
		}
	})

	t.Run("local runner", func(t *testing.T) {
		cmd := &ResolvedCommand{Command: "echo hello"}
		r := NewRunner(cmd, RunOptions{})
		if _, ok := r.(*LocalRunner); !ok {
			t.Errorf("expected LocalRunner, got %T", r)
		}
	})
}

func TestParsePod(t *testing.T) {
	tests := []struct {
		input    string
		wantPod  string
		wantCont string
	}{
		{"my-pod", "my-pod", ""},
		{"my-pod:my-container", "my-pod", "my-container"},
	}

	for _, tt := range tests {
		pod, container := parsePod(tt.input)
		if pod != tt.wantPod || container != tt.wantCont {
			t.Errorf("parsePod(%q) = (%q, %q), want (%q, %q)", tt.input, pod, container, tt.wantPod, tt.wantCont)
		}
	}
}
