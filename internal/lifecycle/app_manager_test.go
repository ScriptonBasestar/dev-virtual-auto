package lifecycle

import (
	"context"
	"os"
	"testing"
	"time"

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

func TestTopoSortWaves_FlatNoDeps(t *testing.T) {
	cfg := &config.Config{
		Applications: map[string]*config.ApplicationConfig{
			"api":    {},
			"worker": {},
		},
	}
	env := config.NewEnvironment(nil, "/tmp", "/tmp")
	am := NewAppManager(cfg, env)

	waves := am.topoSortWaves(cfg.Applications)
	total := 0
	for _, w := range waves {
		total += len(w)
	}
	if total != 2 {
		t.Fatalf("topoSortWaves() returned %d total items, want 2", total)
	}
}

func TestTopoSortWaves_FlatWithDeps(t *testing.T) {
	cfg := &config.Config{
		Applications: map[string]*config.ApplicationConfig{
			"api":    {},
			"worker": {DependsOn: []string{"api"}},
		},
	}
	env := config.NewEnvironment(nil, "/tmp", "/tmp")
	am := NewAppManager(cfg, env)

	waves := am.topoSortWaves(cfg.Applications)
	// Flatten and verify order
	var result []string
	for _, w := range waves {
		result = append(result, w...)
	}
	if len(result) != 2 {
		t.Fatalf("topoSortWaves() returned %d total items, want 2", len(result))
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

func TestTopoSortWaves_NoDeps(t *testing.T) {
	cfg := &config.Config{
		Applications: map[string]*config.ApplicationConfig{
			"api":    {},
			"worker": {},
			"web":    {},
		},
	}
	env := config.NewEnvironment(nil, "/tmp", "/tmp")
	am := NewAppManager(cfg, env)

	waves := am.topoSortWaves(cfg.Applications)
	if len(waves) != 1 {
		t.Fatalf("topoSortWaves() returned %d waves, want 1 (all independent)", len(waves))
	}
	if len(waves[0]) != 3 {
		t.Fatalf("wave[0] has %d apps, want 3", len(waves[0]))
	}
}

func TestTopoSortWaves_WithDeps(t *testing.T) {
	cfg := &config.Config{
		Applications: map[string]*config.ApplicationConfig{
			"db":     {},
			"api":    {DependsOn: []string{"db"}},
			"worker": {DependsOn: []string{"db"}},
			"web":    {DependsOn: []string{"api"}},
		},
	}
	env := config.NewEnvironment(nil, "/tmp", "/tmp")
	am := NewAppManager(cfg, env)

	waves := am.topoSortWaves(cfg.Applications)
	// wave 0: db, wave 1: api+worker, wave 2: web
	if len(waves) != 3 {
		t.Fatalf("topoSortWaves() returned %d waves, want 3", len(waves))
	}
	if len(waves[0]) != 1 || waves[0][0] != "db" {
		t.Errorf("wave[0] = %v, want [db]", waves[0])
	}
	if len(waves[1]) != 2 {
		t.Errorf("wave[1] has %d apps, want 2 (api, worker)", len(waves[1]))
	}
	if len(waves[2]) != 1 || waves[2][0] != "web" {
		t.Errorf("wave[2] = %v, want [web]", waves[2])
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

func TestSelectApps_VariantName(t *testing.T) {
	cfg := &config.Config{
		Applications: map[string]*config.ApplicationConfig{
			"proxynd": {
				Dir: "nd-stack-rs",
				Run: config.AppExecPaths{Native: "cargo run -p proxynd"},
				Variants: map[string]*config.AppVariant{
					"json": {
						Port: 11401,
						Run:  config.AppExecPaths{Native: "cargo run -p proxynd-json"},
					},
				},
			},
		},
	}
	env := config.NewEnvironment(nil, "/tmp", "/tmp")
	am := NewAppManager(cfg, env)

	result := am.selectApps([]string{"proxynd.json"})
	if len(result) != 1 {
		t.Fatalf("selectApps([proxynd.json]) returned %d apps, want 1", len(result))
	}
	app, ok := result["proxynd.json"]
	if !ok {
		t.Fatal("expected proxynd.json in result")
	}
	if app.Run.Native != "cargo run -p proxynd-json" {
		t.Errorf("run = %q, want variant run command", app.Run.Native)
	}
	if app.Dir != "nd-stack-rs" {
		t.Errorf("dir = %q, want inherited dir", app.Dir)
	}
}

func TestSelectApps_AllWithVariants(t *testing.T) {
	cfg := &config.Config{
		Applications: map[string]*config.ApplicationConfig{
			"api": {
				Run: config.AppExecPaths{Native: "cargo run -p api"},
				Variants: map[string]*config.AppVariant{
					"worker": {
						Run: config.AppExecPaths{Native: "cargo run -p api-worker"},
					},
				},
			},
		},
	}
	env := config.NewEnvironment(nil, "/tmp", "/tmp")
	am := NewAppManager(cfg, env)

	// Empty names = all apps including expanded variants
	result := am.selectApps(nil)
	if len(result) != 2 {
		t.Errorf("selectApps(nil) returned %d apps, want 2 (api + api.worker)", len(result))
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

// TestWatchProcessExit_CancelsWhenProcessDead is the fail-fast regression: a
// process that has already exited must cancel the readiness wait promptly,
// rather than let the health check poll a dead process for the full timeout.
func TestWatchProcessExit_CancelsWhenProcessDead(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	start := time.Now()
	watchProcessExit(ctx, 999999999, cancel) // returns once it cancels
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("watchProcessExit took %s to detect a dead process; want prompt cancel", elapsed)
	}
	if ctx.Err() == nil {
		t.Error("expected context to be cancelled after watching a dead process")
	}
}

// TestWatchProcessExit_ReturnsWhenCtxDone verifies the watcher does not cancel a
// live process and simply unwinds when the surrounding context ends.
func TestWatchProcessExit_ReturnsWhenCtxDone(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		watchProcessExit(ctx, os.Getpid(), cancel) // our own PID stays alive
		close(done)
	}()

	// A couple of poll cycles must pass without the watcher returning, since the
	// process is alive.
	select {
	case <-done:
		t.Fatal("watchProcessExit returned while its process was still alive")
	case <-time.After(700 * time.Millisecond):
	}

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Error("watchProcessExit did not return after context cancel")
	}
}
