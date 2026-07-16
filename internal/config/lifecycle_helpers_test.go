package config

import (
	"os"
	"path/filepath"
	"testing"
)

func loadStackConfig(t *testing.T, content string) *Config {
	t.Helper()
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, FileName), []byte(content), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	return cfg
}

func sortedEntry(t *testing.T, cfg *Config, name string) LifecycleEntry {
	t.Helper()
	for _, e := range cfg.SortedStack() {
		if e.Name == name {
			return e
		}
	}
	t.Fatalf("stack entry %q not found", name)
	return LifecycleEntry{}
}

func TestSortedStackResolvesRunnerPlugin(t *testing.T) {
	tests := []struct {
		name       string
		content    string
		wantPlugin string
	}{
		{
			name: "script via default_runner",
			content: `version: "0.1.0"
stack:
  web:
    default_runner: script
    runners:
      script:
        up: "echo up"
`,
			wantPlugin: "script",
		},
		{
			name: "script via sole runner",
			content: `version: "0.1.0"
stack:
  web:
    runners:
      script:
        up: "echo up"
`,
			wantPlugin: "script",
		},
		{
			name: "compose via default_runner",
			content: `version: "0.1.0"
stack:
  web:
    default_runner: compose
    runners:
      compose:
        files:
          - docker-compose.yml
`,
			wantPlugin: "compose",
		},
		{
			name: "nested shape",
			content: `version: "0.1.0"
stack:
  web:
    script:
      up: "echo up"
`,
			wantPlugin: "script",
		},
		{
			name: "flat shape",
			content: `version: "0.1.0"
stack:
  web:
    plugin: script
    up: "echo up"
`,
			wantPlugin: "script",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := sortedEntry(t, loadStackConfig(t, tt.content), "web")
			if entry.Plugin != tt.wantPlugin {
				t.Errorf("Plugin = %q, want %q", entry.Plugin, tt.wantPlugin)
			}
			if entry.DetectPlugin() != tt.wantPlugin {
				t.Errorf("DetectPlugin() = %q, want %q", entry.DetectPlugin(), tt.wantPlugin)
			}
		})
	}
}

func TestSortedStackRunnerConfigIsApplied(t *testing.T) {
	cfg := loadStackConfig(t, `version: "0.1.0"
stack:
  web:
    default_runner: script
    runners:
      script:
        up: "touch ./ran.txt"
`)
	entry := sortedEntry(t, cfg, "web")
	if entry.Script == nil {
		t.Fatal("Script config not populated from runners shape")
	}
	if entry.Script.Up != "touch ./ran.txt" {
		t.Errorf("Script.Up = %q, want %q", entry.Script.Up, "touch ./ran.txt")
	}
}

func TestSortedStackRunnerPluginPrefersDefaultRunner(t *testing.T) {
	cfg := loadStackConfig(t, `version: "0.1.0"
stack:
  web:
    default_runner: script
    runners:
      script:
        up: "echo up"
      compose:
        files:
          - docker-compose.yml
`)
	entry := sortedEntry(t, cfg, "web")
	if entry.Plugin != "script" {
		t.Errorf("Plugin = %q, want %q", entry.Plugin, "script")
	}
}

func TestSortedStackDoesNotNameUnservableRunner(t *testing.T) {
	// docker/native runners decode to runner-only configs that no lifecycle
	// plugin reads. Naming them would make Up a silent no-op instead of an error.
	for _, runner := range []string{"docker", "native"} {
		t.Run(runner, func(t *testing.T) {
			cfg := loadStackConfig(t, `version: "0.1.0"
stack:
  cache:
    runners:
      `+runner+`:
        image: redis:7
`)
			entry := sortedEntry(t, cfg, "cache")
			if entry.Plugin != "" {
				t.Fatalf("Plugin = %q, want empty for unservable runner %q", entry.Plugin, runner)
			}
			if got := entry.DetectPlugin(); got != "" {
				t.Fatalf("DetectPlugin() = %q, want empty: naming a plugin with a nil typed config makes Up a silent no-op", got)
			}
		})
	}
}

func TestSortedStackComposeFallbackWithoutDefaultRunner(t *testing.T) {
	cfg := loadStackConfig(t, `version: "0.1.0"
stack:
  web:
    runners:
      compose:
        files:
          - docker-compose.yml
      script:
        up: "true"
`)
	entry := sortedEntry(t, cfg, "web")
	if entry.Plugin != "compose" {
		t.Fatalf("Plugin = %q, want compose fallback for multi-runner entry without default_runner", entry.Plugin)
	}
	if entry.ComposeConfig() == nil {
		t.Fatal("ComposeConfig() = nil, want compose config preserved")
	}
}
