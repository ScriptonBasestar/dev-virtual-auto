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

func TestSortedStackResolvesDockerRunnerToPlugin(t *testing.T) {
	// Option A (TASK-017): runners.docker decodes as DockerPluginConfig so
	// stack up can run the registered docker plugin (same as nested docker:).
	cfg := loadStackConfig(t, `version: "0.1.0"
stack:
  cache:
    default_runner: docker
    runners:
      docker:
        image: redis:7
        name: dva-redis
        ports:
          - "6379:6379"
`)
	entry := sortedEntry(t, cfg, "cache")
	if entry.Plugin != "docker" {
		t.Fatalf("Plugin = %q, want docker", entry.Plugin)
	}
	if entry.DetectPlugin() != "docker" {
		t.Fatalf("DetectPlugin() = %q, want docker", entry.DetectPlugin())
	}
	if entry.Docker == nil {
		t.Fatal("Docker config not populated from runners.docker")
	}
	if entry.Docker.Image != "redis:7" {
		t.Errorf("Docker.Image = %q, want redis:7", entry.Docker.Image)
	}
	if entry.Docker.Name != "dva-redis" {
		t.Errorf("Docker.Name = %q, want dva-redis", entry.Docker.Name)
	}
	if len(entry.Docker.Ports) != 1 || entry.Docker.Ports[0] != "6379:6379" {
		t.Errorf("Docker.Ports = %v, want [6379:6379]", entry.Docker.Ports)
	}
	// Runners map must hold the same plugin config type so plan materialization works.
	rc, err := entry.GetRunnerConfig("docker")
	if err != nil {
		t.Fatalf("GetRunnerConfig(docker): %v", err)
	}
	if _, ok := rc.(*DockerPluginConfig); !ok {
		t.Fatalf("runners.docker type = %T, want *DockerPluginConfig", rc)
	}
}

func TestSortedStackResolvesNativeRunnerToProcessPlugin(t *testing.T) {
	// Option A (TASK-050): runners.native aliases to the process plugin
	// (Command=run, Dir=dir). NativeRunnerConfig type is kept for plan WorkingDir.
	cfg := loadStackConfig(t, `version: "0.1.0"
stack:
  api:
    default_runner: native
    runners:
      native:
        dir: apps/api
        run: go run ./cmd/api
`)
	entry := sortedEntry(t, cfg, "api")
	if entry.Plugin != "process" {
		t.Fatalf("Plugin = %q, want process", entry.Plugin)
	}
	if entry.DetectPlugin() != "process" {
		t.Fatalf("DetectPlugin() = %q, want process", entry.DetectPlugin())
	}
	if entry.Process == nil {
		t.Fatal("Process config not populated from runners.native")
	}
	if entry.Process.Command != "go run ./cmd/api" {
		t.Errorf("Process.Command = %q, want %q", entry.Process.Command, "go run ./cmd/api")
	}
	if entry.Process.Dir != "apps/api" {
		t.Errorf("Process.Dir = %q, want %q", entry.Process.Dir, "apps/api")
	}
	// Runners map still holds NativeRunnerConfig for plan WorkingDir path.
	rc, err := entry.GetRunnerConfig("native")
	if err != nil {
		t.Fatalf("GetRunnerConfig(native): %v", err)
	}
	if _, ok := rc.(*NativeRunnerConfig); !ok {
		t.Fatalf("runners.native type = %T, want *NativeRunnerConfig", rc)
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

// TestDefaultRunnerNameMatchesRunnerNames pins the invariant show relies on and schema.json states:
// default_runner "Must reference a key in the runners map". The schema accepts both podman-compose
// and podman_compose as that key, so the two fields can name one runner in two spellings — and a
// caller comparing them raw concludes the default points at a runner the entry never declared.
func TestDefaultRunnerNameMatchesRunnerNames(t *testing.T) {
	cfg := loadStackConfig(t, `version: "0.1.44"
stack:
  vms:
    default_runner: podman_compose
    runners:
      podman_compose:
        files:
          - compose.yml
`)
	entry := sortedEntry(t, cfg, "vms")
	names := entry.RunnerNames()
	if len(names) != 1 || names[0] != "podman-compose" {
		t.Fatalf("RunnerNames() = %v, want [podman-compose]", names)
	}
	if got := entry.DefaultRunnerName(); got != names[0] {
		t.Errorf("DefaultRunnerName() = %q, RunnerNames()[0] = %q; the two must agree so callers can compare them", got, names[0])
	}
	if raw := entry.DefaultRunner; raw != "podman_compose" {
		t.Errorf("DefaultRunner = %q, want the author's spelling %q preserved on the raw field", raw, "podman_compose")
	}
}
