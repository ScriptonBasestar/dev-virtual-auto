package lifecycle

import (
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

func TestResolveStrategy_DefaultNative(t *testing.T) {
	cfg := &config.Config{}
	env := config.NewEnvironment(nil, "/tmp", "/tmp")
	am := NewAppManager(cfg, env)

	got := am.resolveStrategy("api", AppStartOptions{})
	if got != "native" {
		t.Errorf("resolveStrategy() = %q, want %q", got, "native")
	}
}

func TestResolveStrategy_GlobalOverride(t *testing.T) {
	cfg := &config.Config{}
	env := config.NewEnvironment(nil, "/tmp", "/tmp")
	am := NewAppManager(cfg, env)

	got := am.resolveStrategy("api", AppStartOptions{Strategy: "docker"})
	if got != "docker" {
		t.Errorf("resolveStrategy() = %q, want %q", got, "docker")
	}
}

func TestResolveStrategy_ModePerApp(t *testing.T) {
	cfg := &config.Config{
		Modes: map[string]config.ModeConfig{
			"hybrid": {
				Applications: map[string]any{
					"api":    "native",
					"worker": "docker",
				},
			},
		},
	}
	env := config.NewEnvironment(nil, "/tmp", "/tmp")
	am := NewAppManager(cfg, env)

	opts := AppStartOptions{Mode: "hybrid", Strategy: "docker"}

	// Per-app override takes precedence over global strategy
	if got := am.resolveStrategy("api", opts); got != "native" {
		t.Errorf("resolveStrategy(api) = %q, want %q", got, "native")
	}
	if got := am.resolveStrategy("worker", opts); got != "docker" {
		t.Errorf("resolveStrategy(worker) = %q, want %q", got, "docker")
	}
	// Unknown app falls back to global strategy
	if got := am.resolveStrategy("unknown", opts); got != "docker" {
		t.Errorf("resolveStrategy(unknown) = %q, want %q", got, "docker")
	}
}

func TestResolveNativeCommand(t *testing.T) {
	cfg := &config.Config{}
	env := config.NewEnvironment(nil, "/tmp", "/tmp")
	am := NewAppManager(cfg, env)

	app := &config.ApplicationConfig{
		Run: config.AppExecPaths{Native: "cargo run -p api"},
		Dev: config.AppExecPaths{Native: "cargo watch -x 'run -p api'"},
	}

	// Dev mode prefers dev path
	if got := am.resolveNativeCommand(app, true); got != "cargo watch -x 'run -p api'" {
		t.Errorf("resolveNativeCommand(devMode=true) = %q", got)
	}

	// Non-dev mode uses run path
	if got := am.resolveNativeCommand(app, false); got != "cargo run -p api" {
		t.Errorf("resolveNativeCommand(devMode=false) = %q", got)
	}
}

func TestResolveNativeCommand_FallbackTodev(t *testing.T) {
	cfg := &config.Config{}
	env := config.NewEnvironment(nil, "/tmp", "/tmp")
	am := NewAppManager(cfg, env)

	app := &config.ApplicationConfig{
		Dev: config.AppExecPaths{Native: "cargo watch -x 'run -p api'"},
	}

	// No run path → falls back to dev even without dev mode
	if got := am.resolveNativeCommand(app, false); got != "cargo watch -x 'run -p api'" {
		t.Errorf("resolveNativeCommand(devMode=false, noRun) = %q", got)
	}
}

func TestTopoSort_NoDeps(t *testing.T) {
	cfg := &config.Config{
		Applications: map[string]*config.ApplicationConfig{
			"api":    {},
			"worker": {},
		},
	}
	env := config.NewEnvironment(nil, "/tmp", "/tmp")
	am := NewAppManager(cfg, env)

	result := am.topoSort(cfg.Applications)
	if len(result) != 2 {
		t.Fatalf("topoSort() returned %d items, want 2", len(result))
	}
}

func TestTopoSort_WithDeps(t *testing.T) {
	cfg := &config.Config{
		Applications: map[string]*config.ApplicationConfig{
			"api":    {},
			"worker": {DependsOn: []string{"api"}},
		},
	}
	env := config.NewEnvironment(nil, "/tmp", "/tmp")
	am := NewAppManager(cfg, env)

	result := am.topoSort(cfg.Applications)
	if len(result) != 2 {
		t.Fatalf("topoSort() returned %d items, want 2", len(result))
	}
	// api must come before worker
	apiIdx, workerIdx := -1, -1
	for i, name := range result {
		if name == "api" {
			apiIdx = i
		}
		if name == "worker" {
			workerIdx = i
		}
	}
	if apiIdx >= workerIdx {
		t.Errorf("api (idx=%d) should come before worker (idx=%d)", apiIdx, workerIdx)
	}
}

func TestSelectApps_All(t *testing.T) {
	cfg := &config.Config{
		Applications: map[string]*config.ApplicationConfig{
			"api":    {},
			"worker": {},
		},
	}
	env := config.NewEnvironment(nil, "/tmp", "/tmp")
	am := NewAppManager(cfg, env)

	result := am.selectApps(nil)
	if len(result) != 2 {
		t.Errorf("selectApps(nil) returned %d apps, want 2", len(result))
	}
}

func TestSelectApps_Named(t *testing.T) {
	cfg := &config.Config{
		Applications: map[string]*config.ApplicationConfig{
			"api":    {},
			"worker": {},
			"web":    {},
		},
	}
	env := config.NewEnvironment(nil, "/tmp", "/tmp")
	am := NewAppManager(cfg, env)

	result := am.selectApps([]string{"api", "web"})
	if len(result) != 2 {
		t.Errorf("selectApps([api,web]) returned %d apps, want 2", len(result))
	}
	if _, ok := result["worker"]; ok {
		t.Error("selectApps should not include worker")
	}
}

func TestBuildDockerCommand(t *testing.T) {
	cfg := &config.Config{}
	env := config.NewEnvironment(nil, "/tmp", "/tmp")
	am := NewAppManager(cfg, env)

	tests := []struct {
		name string
		ref  config.AppDockerRef
		want string
	}{
		{
			name: "raw command",
			ref:  config.AppDockerRef{Command: "docker compose build api"},
			want: "docker compose build api",
		},
		{
			name: "service only",
			ref:  config.AppDockerRef{Service: "api-rs"},
			want: "docker compose up -d api-rs",
		},
		{
			name: "service with profile",
			ref:  config.AppDockerRef{Service: "api-rs", Profile: "rust"},
			want: "docker compose --profile rust up -d api-rs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := am.buildDockerCommand(tt.ref)
			if got != tt.want {
				t.Errorf("buildDockerCommand() = %q, want %q", got, tt.want)
			}
		})
	}
}
