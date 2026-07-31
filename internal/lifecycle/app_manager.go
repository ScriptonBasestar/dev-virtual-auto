package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
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
	Port     int
	LogFile  string
	// PortPID is the PID currently listening on Port (0 = none/undeterminable).
	// PortOwned reports whether that listener belongs to the tracked process
	// group. PortPID > 0 && !PortOwned means a foreign process (e.g. a stale
	// orphan from a previous run) holds the port.
	PortPID   int
	PortOwned bool
}

// PortConflict describes an application whose declared port is held by a
// process that dva did not start (or that outlived the process dva tracked).
type PortConflict struct {
	App        string
	Port       int
	TrackedPID int // PID dva recorded for the app (0 if none / dead)
	ForeignPID int // PID actually holding the port, outside dva's group
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
		if err := am.startWave(ctx, wave, apps, opts); err != nil {
			return err
		}
	}

	return nil
}

// startWave starts all apps in a wave concurrently, then waits for all to
// complete startup (including health checks).
func (am *AppManager) startWave(ctx context.Context, names []string, apps map[string]*config.ApplicationConfig, opts AppStartOptions) error {
	waveCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var mu sync.Mutex
	var errs []error
	var wg sync.WaitGroup

	// recordErr is the only way a failure reaches the exit code: startWave ends in
	// errors.Join(errs...), so anything not appended here is a message and nothing more.
	// Before TASK-117 only the two pre-start failures below called it (open-coded), while
	// all three post-start [FAIL] branches wrote to stderr alone — DVA waited the full
	// timeout, correctly concluded the process never listened, printed a precise message
	// with a log path, and exited 0. Called from the per-app goroutines, hence the lock.
	recordErr := func(err error) {
		mu.Lock()
		errs = append(errs, err)
		mu.Unlock()
	}

	for _, name := range names {
		app := apps[name]
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
			_ = os.Remove(pidFile)
		}

		// Preflight: a foreign process already holding the port would make the
		// child crash on bind, leaving an untracked orphan serving the port.
		// Refuse to spawn a doomed process and tell the user how to reclaim it.
		if strategy != "docker" {
			if port := effectivePort(app); port > 0 && portOwnershipSupported() {
				if foreign, owned := resolvePortOwnership(0, port); foreign > 0 && !owned {
					fmt.Fprintf(os.Stderr, "[app] %s: port %d already held by PID %d (not started by dva) — skipping. Run 'dva app down %s' to reclaim the port, then retry.\n", name, port, foreign, name)
					// Record the skip as an error so `up` exits non-zero. The
					// post-start PortConflicts check can't see it: no pidfile is
					// written for an app we never spawned.
					recordErr(fmt.Errorf("app %s: port %d held by PID %d not started by dva", name, port, foreign))
					continue
				}
			}
		}

		wg.Add(1)
		go func(name, strategy, command string, app *config.ApplicationConfig) {
			defer wg.Done()

			var pid int
			var err error
			if strategy == "docker" {
				err = am.startDockerApp(waveCtx, name, app, command)
			} else {
				pid, err = am.startNativeApp(name, app, command)
			}

			if err != nil {
				recordErr(fmt.Errorf("app start %s (%s): %w", name, strategy, err))
				cancel()
				return
			}

			fmt.Fprintf(os.Stderr, "[+] started app %s [%s]\n", name, strategy)

			if !opts.Wait {
				return
			}

			timeout := 30 * time.Second
			if app.Health != nil && app.Health.ReadyTimeout > 0 {
				timeout = time.Duration(app.Health.ReadyTimeout) * time.Second
			}

			crashReported := false

			// Wait for the configured health check to pass (if any). Cancel the
			// wait the instant a native process exits: a process that crashes on
			// startup (e.g. a failed port bind) would otherwise be polled until
			// the full timeout elapses — a silent, minutes-long wait with no
			// output. (waitForPortOwnership below already self-cancels on death;
			// this extends the same fail-fast to the health-check wait.)
			if app.Health != nil {
				fmt.Fprintf(os.Stderr, "[app] waiting for %s to become healthy (up to %s)...\n", name, timeout)
				checks := map[string]config.HealthCheckConfig{name: *app.Health}
				waitCtx, wCancel := context.WithTimeout(waveCtx, timeout)
				if strategy != "docker" && pid > 0 {
					go watchProcessExit(waitCtx, pid, wCancel)
				}
				results := am.hc.WaitUntilReady(waitCtx, checks)
				wCancel()
				for _, r := range results {
					if r.Ready {
						continue
					}
					if strategy != "docker" && pid > 0 && !IsProcessRunning(pid) {
						fmt.Fprintf(os.Stderr, "[FAIL] app %s exited during startup — see %s. A common cause is the process binding a different port than its health check expects; set PORT to match the health URL.\n", name, am.logPath(name))
						crashReported = true
						// Terser than the printed line on purpose: the [FAIL] above is
						// written for a human at a terminal, this one is what `dva up`
						// hands back to the shell. Both are emitted once.
						recordErr(fmt.Errorf("app %s exited during startup (see %s)", name, am.logPath(name)))
					} else {
						// Deliberately still a warning, decided on the record in TASK-117:
						// the process is alive and only the health probe is unhappy, which
						// DVA cannot distinguish from "slow to warm up". The sharper signal
						// for a genuinely broken start is the port-ownership check below,
						// which does error. Promoting this one would change the exit code of
						// every existing setup with a flaky probe, so it is a product
						// decision rather than a defect — filed as TASK-118, which also
						// records the hole this leaves: an app that binds its port but
						// never answers its probe is caught by neither branch.
						fmt.Fprintf(os.Stderr, "[warn] app %s not ready after %s\n", name, timeout)
					}
				}
			}

			// Verify the process dva started actually owns its port. This
			// catches a child that crashed on bind and a green health probe
			// that is really being answered by a foreign orphan — both of which
			// would otherwise be reported as a successful start. Skip when the
			// health wait already reported the crash, to avoid a duplicate FAIL.
			if !crashReported && strategy != "docker" && pid > 0 && portOwnershipSupported() {
				if port := effectivePort(app); port > 0 && !waitForPortOwnership(waveCtx, pid, port, timeout) {
					if foreign, _ := resolvePortOwnership(pid, port); foreign > 0 {
						fmt.Fprintf(os.Stderr, "[FAIL] app %s: port %d is held by PID %d, not the process dva started — likely a stale orphan or a crash on bind. See %s\n", name, port, foreign, am.logPath(name))
						recordErr(fmt.Errorf("app %s: port %d held by PID %d, not the process dva started", name, port, foreign))
					} else {
						fmt.Fprintf(os.Stderr, "[FAIL] app %s: process did not listen on port %d within %s — see %s\n", name, port, timeout, am.logPath(name))
						recordErr(fmt.Errorf("app %s: process did not listen on port %d within %s", name, port, timeout))
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
			app := apps[name]
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
// Beyond signalling the tracked process group, it reclaims each app's declared
// port from any survivor or stale orphan the group signal did not reach — this
// is what frees a port held by a process from a previous run that a plain
// group-kill would otherwise leave behind.
func (am *AppManager) DownApps(names ...string) {
	apps := am.selectApps(names)

	for name, app := range apps {
		pidFile := am.pidPath(name)
		logFile := am.logPath(name)

		// Terminate the tracked process group if we recorded one. A pidfile is
		// written only for native apps, so its presence also gates the port
		// reclaim below.
		hadPidfile := false
		if data, err := os.ReadFile(pidFile); err == nil {
			hadPidfile = true
			if pid, perr := strconv.Atoi(strings.TrimSpace(string(data))); perr == nil && pid > 0 {
				if err := syscall.Kill(-pid, syscall.SIGTERM); err == nil {
					fmt.Fprintf(os.Stderr, "[-] removed app %s (pid %d)\n", name, pid)
				}
			}
		}

		// Reclaim the declared port (SIGTERM then SIGKILL survivors) — but only
		// for apps dva started natively. A docker app's port is held by
		// docker-proxy; signalling it (or an unrelated process sharing the port)
		// would be wrong and potentially destructive.
		if hadPidfile {
			if port := effectivePort(app); port > 0 && portOwnershipSupported() {
				if killed := reclaimPort(port); len(killed) > 0 {
					fmt.Fprintf(os.Stderr, "[-] freed port %d for app %s (killed pid %v)\n", port, name, killed)
				}
			}
		}

		_ = os.Remove(pidFile)
		_ = os.Remove(logFile)
	}
}

// AppStatuses returns the current status of all configured applications.
func (am *AppManager) AppStatuses() []AppStatus {
	var statuses []AppStatus

	for name, app := range am.cfg.Applications {
		status := AppStatus{
			Name:    name,
			Port:    app.Port,
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

		am.resolveOwnership(&status, app)

		statuses = append(statuses, status)

		// Add variant entries
		for vName, variant := range app.Variants {
			fullName := name + "." + vName
			vApp := config.ResolveVariant(app, variant)
			vStatus := AppStatus{
				Name:    fullName,
				Port:    vApp.Port,
				LogFile: am.logPath(fullName),
			}

			vPidFile := am.pidPath(fullName)
			vData, vErr := os.ReadFile(vPidFile)
			if vErr == nil {
				pid, _ := strconv.Atoi(strings.TrimSpace(string(vData)))
				if pid > 0 && IsProcessRunning(pid) {
					vStatus.Running = true
					vStatus.PID = pid
					vStatus.Strategy = "native"
				}
			}

			if !vStatus.Running {
				vStatus.Strategy = "stopped"
			}

			am.resolveOwnership(&vStatus, vApp)

			statuses = append(statuses, vStatus)
		}
	}

	return statuses
}

// resolveOwnership fills in port-ownership fields and the health verdict for a
// status. Health requires the tracked process to be alive, the configured
// health probe to pass, and — when port ownership can be determined — the port
// not to be answered by a foreign process. A green probe served by an orphan
// outside dva's process group is reported unhealthy rather than masking it.
func (am *AppManager) resolveOwnership(status *AppStatus, app *config.ApplicationConfig) {
	port := effectivePort(app)
	if status.Port == 0 {
		status.Port = port
	}

	foreign := false
	// Port-ownership reasoning only applies to apps dva runs as native processes.
	// A docker app's published port is held by docker-proxy — outside dva's
	// process group — and would always look "foreign". A live tracked PID or a
	// recorded pidfile (even for a since-crashed process) marks native management.
	managed := status.PID > 0 || am.pidFileExists(status.Name)
	if managed && port > 0 && portOwnershipSupported() {
		status.PortPID, status.PortOwned = resolvePortOwnership(status.PID, port)
		foreign = status.PortPID > 0 && !status.PortOwned
	}

	if status.Running && app.Health != nil {
		checks := map[string]config.HealthCheckConfig{status.Name: *app.Health}
		for _, r := range am.hc.Check(checks) {
			if r.Ready && !foreign {
				status.Healthy = true
			}
		}
	}
}

// PortConflicts returns applications whose declared port is currently held by a
// process outside dva's tracking (a stale orphan, or a child that outlived the
// tracked group). names filters the set; empty means all applications. Returns
// nil when port ownership cannot be determined on this host.
func (am *AppManager) PortConflicts(names ...string) []PortConflict {
	if !portOwnershipSupported() {
		return nil
	}
	var conflicts []PortConflict
	for name, app := range am.selectApps(names) {
		// Only apps dva runs natively (a pidfile was recorded) can be reasoned
		// about by port ownership; a docker app's port is held by docker-proxy,
		// which is never in dva's process group.
		if !am.pidFileExists(name) {
			continue
		}
		port := effectivePort(app)
		if port == 0 {
			continue
		}
		tracked := am.trackedPID(name)
		if foreign, owned := resolvePortOwnership(tracked, port); foreign > 0 && !owned {
			conflicts = append(conflicts, PortConflict{
				App:        name,
				Port:       port,
				TrackedPID: tracked,
				ForeignPID: foreign,
			})
		}
	}
	sort.Slice(conflicts, func(i, j int) bool { return conflicts[i].App < conflicts[j].App })
	return conflicts
}

// trackedPID returns the live PID dva recorded for an app, or 0 when there is
// no pidfile or the recorded process is dead.
func (am *AppManager) trackedPID(name string) int {
	data, err := os.ReadFile(am.pidPath(name))
	if err != nil {
		return 0
	}
	pid, _ := strconv.Atoi(strings.TrimSpace(string(data)))
	if pid > 0 && IsProcessRunning(pid) {
		return pid
	}
	return 0
}

// pidFileExists reports whether dva has a pidfile recorded for an app. Only
// startNativeApp writes one, so its presence means dva is managing the app as a
// native process — the precondition for port-ownership reasoning. It stays true
// even when the recorded process has since died (a crashed-on-bind native app),
// which is exactly the stale-orphan case ownership checks must still catch.
func (am *AppManager) pidFileExists(name string) bool {
	_, err := os.Stat(am.pidPath(name))
	return err == nil
}

// startNativeApp starts an application as a background process using the
// same PID/log infrastructure as the process plugin. It returns the PID of the
// spawned process-group leader (the `sh -c` wrapper) so callers can verify the
// process actually came up and owns its port.
func (am *AppManager) startNativeApp(name string, app *config.ApplicationConfig, command string) (int, error) {
	configDir := am.cfg.FileDir()
	pidDir := filepath.Join(configDir, config.DotDirName, config.PidsDirName)
	logDir := filepath.Join(configDir, config.DotDirName, config.LogsDirName)

	if err := os.MkdirAll(pidDir, 0755); err != nil {
		return 0, fmt.Errorf("create pid dir: %w", err)
	}
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return 0, fmt.Errorf("create log dir: %w", err)
	}

	logFile, err := os.Create(filepath.Join(logDir, "app-"+name+".log"))
	if err != nil {
		return 0, fmt.Errorf("create log file: %w", err)
	}

	dir := am.resolveDir(app)

	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = dir
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	// Build environment: base env + app-specific vars
	appEnv := cloneEnv(am.env)
	maps.Copy(appEnv.Vars, app.Environment)
	cmd.Env = appEnv.EnvSlice()

	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return 0, fmt.Errorf("start: %w", err)
	}

	pid := cmd.Process.Pid
	pidPath := filepath.Join(pidDir, "app-"+name+".pid")
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(pid)), 0644); err != nil {
		_ = syscall.Kill(-pid, syscall.SIGTERM)
		_ = logFile.Close()
		return 0, fmt.Errorf("save pid: %w", err)
	}

	_ = logFile.Close()

	// Reap zombie in background goroutine
	go func() { _ = cmd.Wait() }()

	return pid, nil
}

// watchProcessExit cancels the wait once pid is no longer running, so a
// readiness wait stops polling a process that has already crashed instead of
// blocking for the full timeout. It returns when the process exits or ctx is
// done, whichever comes first.
func watchProcessExit(ctx context.Context, pid int, cancel context.CancelFunc) {
	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !IsProcessRunning(pid) {
				cancel()
				return
			}
		}
	}
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
	return filepath.Join(am.cfg.FileDir(), config.DotDirName, config.PidsDirName, "app-"+name+".pid")
}

// logPath returns the log file path for an app.
func (am *AppManager) logPath(name string) string {
	return filepath.Join(am.cfg.FileDir(), config.DotDirName, config.LogsDirName, "app-"+name+".log")
}

// selectApps returns the subset of configured applications matching the given names.
// Supports "app.variant" dot notation for variant resolution.
// If names is empty, returns all applications (including expanded variants).
func (am *AppManager) selectApps(names []string) map[string]*config.ApplicationConfig {
	if len(names) == 0 {
		return am.expandAllApps()
	}
	selected := make(map[string]*config.ApplicationConfig)
	for _, n := range names {
		resolvedName, app, err := am.cfg.ResolveApp(n)
		if err != nil {
			am.logger.Debug("app not found", "name", n, "err", err)
			continue
		}
		selected[resolvedName] = app
	}
	return selected
}

// expandAllApps returns all applications with variants expanded as separate entries.
func (am *AppManager) expandAllApps() map[string]*config.ApplicationConfig {
	result := make(map[string]*config.ApplicationConfig)
	for name, app := range am.cfg.Applications {
		result[name] = app
		for vName, variant := range app.Variants {
			fullName := name + "." + vName
			result[fullName] = config.ResolveVariant(app, variant)
		}
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
