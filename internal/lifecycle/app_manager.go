package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// AppManager manages application lifecycle (start/stop/status) for the
// applications: section of dva.yml.
type AppManager struct {
	cfg    *config.Config
	env    *config.Environment
	hc     *HealthChecker
	logger *slog.Logger
}

// AppStartOptions configures how applications are started.
type AppStartOptions struct {
	Strategy string   // global strategy: "native" or "docker"
	Names    []string // start only these apps (empty = all)
	DevMode  bool     // prefer dev exec path over run
	DryRun   bool
	Wait     bool
	Mode     string // mode name for per-app strategy lookup
}

// AppStatus represents the current state of a managed application.
type AppStatus struct {
	Name     string
	Strategy string // "native", "docker", "stopped"
	Running  bool
	Healthy  bool
	PID      int
	LogFile  string
}

// NewAppManager creates an AppManager for the given config and environment.
func NewAppManager(cfg *config.Config, env *config.Environment) *AppManager {
	return &AppManager{
		cfg:    cfg,
		env:    env,
		hc:     &HealthChecker{},
		logger: slog.Default(),
	}
}

// StartApps starts applications in dependency order, with independent apps
// launched concurrently within the same wave.
func (am *AppManager) StartApps(ctx context.Context, opts AppStartOptions) error {
	apps := am.selectApps(opts.Names)
	if len(apps) == 0 {
		return nil
	}

	waves := am.topoSortWaves(apps)

	for _, wave := range waves {
		if err := am.startWave(ctx, wave, opts); err != nil {
			return err
		}
	}

	return nil
}

// startWave starts all apps in a wave concurrently, then waits for all to
// complete startup (including health checks).
func (am *AppManager) startWave(ctx context.Context, names []string, opts AppStartOptions) error {
	waveCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var mu sync.Mutex
	var errs []error
	var wg sync.WaitGroup

	for _, name := range names {
		app := am.cfg.Applications[name]
		strategy := am.resolveStrategy(name, opts)
		command := am.resolveCommand(app, strategy, opts.DevMode)

		if command == "" {
			am.logger.Warn("no command for strategy", "app", name, "strategy", strategy)
			continue
		}

		if opts.DryRun {
			fmt.Fprintf(os.Stderr, "[app] (dry-run) would start %s [%s]: %s\n", name, strategy, command)
			continue
		}

		// Check if already running; clean stale PID files
		pidFile := am.pidPath(name)
		if data, err := os.ReadFile(pidFile); err == nil {
			pid, _ := strconv.Atoi(strings.TrimSpace(string(data)))
			if pid > 0 && IsProcessRunning(pid) {
				fmt.Fprintf(os.Stderr, "[app] %s already running (pid %d)\n", name, pid)
				continue
			}
			// Stale PID file — process dead, clean up and restart
			os.Remove(pidFile)
		}

		wg.Add(1)
		go func(name, strategy, command string, app *config.ApplicationConfig) {
			defer wg.Done()

			var err error
			if strategy == "docker" {
				err = am.startDockerApp(waveCtx, name, app, command)
			} else {
				err = am.startNativeApp(name, app, command)
			}

			if err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("app start %s (%s): %w", name, strategy, err))
				mu.Unlock()
				cancel()
				return
			}

			fmt.Fprintf(os.Stderr, "[+] started app %s [%s]\n", name, strategy)

			// Wait for health check readiness
			if opts.Wait && app.Health != nil {
				checks := map[string]config.HealthCheckConfig{name: *app.Health}
				timeout := time.Duration(app.Health.ReadyTimeout) * time.Second
				if timeout == 0 {
					timeout = 30 * time.Second
				}
				waitCtx, wCancel := context.WithTimeout(waveCtx, timeout)
				results := am.hc.WaitUntilReady(waitCtx, checks)
				wCancel()
				for _, r := range results {
					if !r.Ready {
						fmt.Fprintf(os.Stderr, "[warn] app %s not ready after %s\n", name, timeout)
					}
				}
			}
		}(name, strategy, command, app)
	}

	wg.Wait()

	return errors.Join(errs...)
}

// BuildApps runs the build command for each selected application.
// Independent apps within the same dependency wave are built concurrently.
func (am *AppManager) BuildApps(ctx context.Context, opts AppStartOptions) error {
	apps := am.selectApps(opts.Names)
	if len(apps) == 0 {
		return nil
	}

	waves := am.topoSortWaves(apps)

	for _, wave := range waves {
		var mu sync.Mutex
		var firstErr error
		var wg sync.WaitGroup

		for _, name := range wave {
			app := am.cfg.Applications[name]
			strategy := am.resolveStrategy(name, opts)

			var command string
			if strategy == "docker" {
				command = am.resolveDockerCommand(app, false)
			} else {
				command = app.Build.Native
			}

			if command == "" {
				am.logger.Info("no build command", "app", name, "strategy", strategy)
				continue
			}

			if opts.DryRun {
				fmt.Fprintf(os.Stderr, "[app] (dry-run) would build %s [%s]: %s\n", name, strategy, command)
				continue
			}

			wg.Add(1)
			go func(name, strategy, command string, app *config.ApplicationConfig) {
				defer wg.Done()

				fmt.Fprintf(os.Stderr, "[app] building %s [%s]...\n", name, strategy)
				dir := am.resolveDir(app)

				cmd := exec.CommandContext(ctx, "sh", "-c", command)
				cmd.Dir = dir
				cmd.Stdout = os.Stderr
				cmd.Stderr = os.Stderr
				if am.env != nil {
					cmd.Env = am.env.EnvSlice()
				}

				if err := cmd.Run(); err != nil {
					mu.Lock()
					if firstErr == nil {
						firstErr = fmt.Errorf("build %s: %w", name, err)
					}
					mu.Unlock()
					return
				}
				fmt.Fprintf(os.Stderr, "[+] built %s\n", name)
			}(name, strategy, command, app)
		}

		wg.Wait()
		if firstErr != nil {
			return firstErr
		}
	}

	return nil
}

// HaltApps sends SIGTERM but preserves PID files so apps can be
// restarted quickly via `dva app up` (Vagrant halt semantics).
func (am *AppManager) HaltApps(names ...string) {
	apps := am.selectApps(names)

	for name := range apps {
		pidFile := am.pidPath(name)
		data, err := os.ReadFile(pidFile)
		if err != nil {
			continue
		}

		pid, _ := strconv.Atoi(strings.TrimSpace(string(data)))
		if pid > 0 && IsProcessRunning(pid) {
			if err := syscall.Kill(-pid, syscall.SIGTERM); err == nil {
				fmt.Fprintf(os.Stderr, "[-] stopped app %s (pid %d)\n", name, pid)
			}
		}
		// PID file preserved — app can be restarted by `dva app up`
	}
}

// DownApps sends SIGTERM and removes PID/log files (Vagrant destroy semantics).
func (am *AppManager) DownApps(names ...string) {
	apps := am.selectApps(names)

	for name := range apps {
		pidFile := am.pidPath(name)
		logFile := am.logPath(name)

		data, err := os.ReadFile(pidFile)
		if err != nil {
			continue
		}

		pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
		if err != nil {
			os.Remove(pidFile)
			continue
		}

		if pid > 0 {
			if err := syscall.Kill(-pid, syscall.SIGTERM); err == nil {
				fmt.Fprintf(os.Stderr, "[-] removed app %s (pid %d)\n", name, pid)
			}
		}

		os.Remove(pidFile)
		os.Remove(logFile)
	}
}

// AppStatuses returns the current status of all configured applications.
func (am *AppManager) AppStatuses() []AppStatus {
	var statuses []AppStatus

	for name, app := range am.cfg.Applications {
		status := AppStatus{
			Name:    name,
			LogFile: am.logPath(name),
		}

		pidFile := am.pidPath(name)
		data, err := os.ReadFile(pidFile)
		if err == nil {
			pid, _ := strconv.Atoi(strings.TrimSpace(string(data)))
			if pid > 0 && IsProcessRunning(pid) {
				status.Running = true
				status.PID = pid
				status.Strategy = "native" // PID-tracked = native
			}
		}

		if !status.Running {
			status.Strategy = "stopped"
		}

		// Check health if configured and running
		if status.Running && app.Health != nil {
			checks := map[string]config.HealthCheckConfig{name: *app.Health}
			results := am.hc.Check(checks)
			for _, r := range results {
				if r.Ready {
					status.Healthy = true
				}
			}
		}

		statuses = append(statuses, status)
	}

	return statuses
}

// startNativeApp starts an application as a background process using the
// same PID/log infrastructure as the process plugin.
func (am *AppManager) startNativeApp(name string, app *config.ApplicationConfig, command string) error {
	configDir := am.cfg.FileDir()
	pidDir := filepath.Join(configDir, config.DotDirName, "pids")
	logDir := filepath.Join(configDir, config.DotDirName, "logs")

	if err := os.MkdirAll(pidDir, 0755); err != nil {
		return fmt.Errorf("create pid dir: %w", err)
	}
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("create log dir: %w", err)
	}

	logFile, err := os.Create(filepath.Join(logDir, "app-"+name+".log"))
	if err != nil {
		return fmt.Errorf("create log file: %w", err)
	}

	dir := am.resolveDir(app)

	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = dir
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	// Build environment: base env + app-specific vars
	appEnv := cloneEnv(am.env)
	for k, v := range app.Environment {
		appEnv.Vars[k] = v
	}
	cmd.Env = appEnv.EnvSlice()

	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		logFile.Close()
		return fmt.Errorf("start: %w", err)
	}

	pidPath := filepath.Join(pidDir, "app-"+name+".pid")
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(cmd.Process.Pid)), 0644); err != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
		logFile.Close()
		return fmt.Errorf("save pid: %w", err)
	}

	logFile.Close()

	// Reap zombie in background goroutine
	go cmd.Wait()

	return nil
}

// startDockerApp starts an application via docker compose.
func (am *AppManager) startDockerApp(ctx context.Context, name string, app *config.ApplicationConfig, command string) error {
	dir := am.resolveDir(app)

	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = dir
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr

	if am.env != nil {
		cmd.Env = am.env.EnvSlice()
	}

	return cmd.Run()
}

// resolveStrategy determines the execution strategy for an app.
// Priority: mode per-app → mode global → "native" (default).
func (am *AppManager) resolveStrategy(name string, opts AppStartOptions) string {
	// 1. Check mode per-app override
	if opts.Mode != "" {
		if m, ok := am.cfg.Modes[opts.Mode]; ok {
			if s := m.AppStrategy(name); s != "" {
				return s
			}
		}
	}

	// 2. Check global option
	if opts.Strategy != "" {
		return opts.Strategy
	}

	// 3. Default to native
	return "native"
}

// resolveCommand picks the shell command to execute for the given app and strategy.
func (am *AppManager) resolveCommand(app *config.ApplicationConfig, strategy string, devMode bool) string {
	if strategy == "docker" {
		return am.resolveDockerCommand(app, devMode)
	}
	return am.resolveNativeCommand(app, devMode)
}

func (am *AppManager) resolveNativeCommand(app *config.ApplicationConfig, devMode bool) string {
	if devMode && app.Dev.HasNative() {
		return app.Dev.Native
	}
	if app.Run.HasNative() {
		return app.Run.Native
	}
	// Fallback: dev path even when not in dev mode
	if app.Dev.HasNative() {
		return app.Dev.Native
	}
	return ""
}

func (am *AppManager) resolveDockerCommand(app *config.ApplicationConfig, devMode bool) string {
	// Prefer dev docker path in dev mode
	if devMode && app.Dev.HasDocker() {
		return am.buildDockerCommand(app.Dev.Docker)
	}
	if app.Run.HasDocker() {
		return am.buildDockerCommand(app.Run.Docker)
	}
	if app.Dev.HasDocker() {
		return am.buildDockerCommand(app.Dev.Docker)
	}
	return ""
}

// buildDockerCommand constructs a docker compose command from AppDockerRef.
func (am *AppManager) buildDockerCommand(ref config.AppDockerRef) string {
	// If a raw command is specified, use it directly
	if ref.Command != "" && ref.Service == "" {
		return ref.Command
	}

	// Build docker compose up command
	var parts []string
	parts = append(parts, "docker compose")
	if ref.Profile != "" {
		parts = append(parts, "--profile", ref.Profile)
	}
	parts = append(parts, "up", "-d")
	if ref.Service != "" {
		parts = append(parts, ref.Service)
	}
	return strings.Join(parts, " ")
}

// resolveDir returns the working directory for an app.
func (am *AppManager) resolveDir(app *config.ApplicationConfig) string {
	configDir := am.cfg.FileDir()
	if app.Dir == "" {
		return configDir
	}
	if filepath.IsAbs(app.Dir) {
		return app.Dir
	}
	return filepath.Join(configDir, app.Dir)
}

// pidPath returns the PID file path for an app.
func (am *AppManager) pidPath(name string) string {
	return filepath.Join(am.cfg.FileDir(), config.DotDirName, "pids", "app-"+name+".pid")
}

// logPath returns the log file path for an app.
func (am *AppManager) logPath(name string) string {
	return filepath.Join(am.cfg.FileDir(), config.DotDirName, "logs", "app-"+name+".log")
}

// selectApps returns the subset of configured applications matching the given names.
// If names is empty, returns all applications.
func (am *AppManager) selectApps(names []string) map[string]*config.ApplicationConfig {
	if len(names) == 0 {
		return am.cfg.Applications
	}
	selected := make(map[string]*config.ApplicationConfig)
	for _, n := range names {
		if app, ok := am.cfg.Applications[n]; ok {
			selected[n] = app
		}
	}
	return selected
}

// topoSort returns app names in dependency order (apps with no deps first).
// Flattened from topoSortWaves for callers that need a simple list.
func (am *AppManager) topoSort(apps map[string]*config.ApplicationConfig) []string {
	waves := am.topoSortWaves(apps)
	var result []string
	for _, wave := range waves {
		result = append(result, wave...)
	}
	return result
}

// topoSortWaves returns app names grouped into dependency waves.
// Apps within the same wave have no mutual dependencies and can run concurrently.
// Kahn's algorithm; cycles are broken by emitting remaining nodes in a final wave.
func (am *AppManager) topoSortWaves(apps map[string]*config.ApplicationConfig) [][]string {
	// Build in-degree map
	inDegree := make(map[string]int)
	for name := range apps {
		inDegree[name] = 0
	}
	for name, app := range apps {
		for _, dep := range app.DependsOn {
			if _, ok := apps[dep]; ok {
				_ = name
				inDegree[name]++
			}
		}
	}

	// Collect initial wave: nodes with 0 in-degree
	var queue []string
	for name, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, name)
		}
	}

	emitted := make(map[string]bool)
	var waves [][]string

	for len(queue) > 0 {
		wave := queue
		queue = nil
		waves = append(waves, wave)

		for _, node := range wave {
			emitted[node] = true
		}

		// Decrease in-degree for dependents, collect next wave
		for name, app := range apps {
			if emitted[name] {
				continue
			}
			for _, dep := range app.DependsOn {
				for _, node := range wave {
					if dep == node {
						inDegree[name]--
					}
				}
			}
			if inDegree[name] == 0 {
				queue = append(queue, name)
			}
		}
	}

	// Append any remaining (cycle) nodes as a final wave
	var remaining []string
	for name := range apps {
		if !emitted[name] {
			remaining = append(remaining, name)
		}
	}
	if len(remaining) > 0 {
		waves = append(waves, remaining)
	}

	return waves
}
