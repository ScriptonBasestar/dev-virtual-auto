package lifecycle

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// UpOptions configures orchestrator Up behavior.
type UpOptions struct {
	DryRun      bool
	Force       bool
	Wait        bool
	IncludeTags []string
	ExcludeTags []string
	Mode        string
	Env         string
}

// DownOptions configures orchestrator Down behavior.
type DownOptions struct {
	DryRun       bool
	Volumes      bool // also remove named volumes
	RemoveImages bool // also remove locally built images
	IncludeTags  []string
	ExcludeTags  []string
	Mode         string
	Env          string
}

// StopOptions configures orchestrator Stop behavior.
type StopOptions struct {
	DryRun      bool
	IncludeTags []string
	ExcludeTags []string
	Mode        string
	Env         string
}

// Orchestrator coordinates lifecycle plugin execution in order.
type Orchestrator struct {
	entries []config.LifecycleEntry
	cfg     *config.Config
	env     *config.Environment
	logger  *slog.Logger
	hc      *HealthChecker
}

// NewOrchestrator creates a new orchestrator from config.
func NewOrchestrator(cfg *config.Config, env *config.Environment) *Orchestrator {
	entries := cfg.SortedStack()

	return &Orchestrator{
		entries: entries,
		cfg:     cfg,
		env:     env,
		logger:  slog.Default(),
		hc:      &HealthChecker{},
	}
}

// Up starts all matching lifecycle entries in order.
func (o *Orchestrator) Up(ctx context.Context, opts UpOptions) error {
	filtered := o.filterEntries(opts.IncludeTags, opts.ExcludeTags, opts.Mode, opts.Env)
	if len(filtered) == 0 {
		fmt.Fprintln(os.Stderr, "[warn] no lifecycle entries matched filters")
		return nil
	}

	// Clone env so exports accumulate without mutating the original
	envClone := cloneEnv(o.env)

	// Resolve mode-derived compose hints
	var modeProfiles []string
	var modeServices *[]string
	if opts.Mode != "" {
		if m, ok := o.cfg.Modes[opts.Mode]; ok {
			modeProfiles = m.ComposeProfiles
			modeServices = m.ComposeServices
		}
	}

	for _, entry := range filtered {
		pluginType := entry.DetectPlugin()
		plugin, err := NewPlugin(pluginType)
		if err != nil {
			return fmt.Errorf("entry %q: %w", entry.Name, err)
		}

		pctx := &PluginContext{
			Entry:           &entry,
			Env:             envClone,
			ConfigDir:       o.cfg.FileDir(),
			DryRun:          opts.DryRun,
			Force:           opts.Force,
			Wait:            opts.Wait,
			ComposeProfiles: modeProfiles,
			ComposeServices: modeServices,
			Logger:          o.logger.With("entry", entry.Name, "plugin", pluginType),
		}

		fmt.Fprintf(os.Stderr, "[lifecycle] %s (%s)\n", entry.Name, pluginType)

		result, err := plugin.Up(ctx, pctx)
		if err != nil {
			return fmt.Errorf("entry %q up failed: %w", entry.Name, err)
		}

		// Merge dynamic exports from plugin result
		if result != nil && len(result.Exports) > 0 {
			envClone.MergeVars(result.Exports)
		}

		// Merge static exports from entry config (with interpolation)
		if len(entry.Exports) > 0 {
			envClone.MergeVars(entry.Exports)
		}

		// Run health checks for this entry and wait if needed
		if len(entry.HealthChecks) > 0 && opts.Wait {
			results := o.hc.WaitUntilReady(ctx, entry.HealthChecks)
			allReady := true
			for _, r := range results {
				if !r.Ready {
					allReady = false
					break
				}
			}
			if !allReady {
				fmt.Fprintf(os.Stderr, "[warn] some health checks not ready for entry %q\n", entry.Name)
			}
		}
	}

	// Start native processes defined in mode health_checks with start commands
	if err := o.startModeProcesses(ctx, opts, envClone); err != nil {
		return err
	}

	// Start applications only if the active mode explicitly declares application strategies.
	// Modes without an "applications" field (e.g., infra) skip app startup entirely.
	if len(o.cfg.Applications) > 0 && opts.Mode != "" {
		if m, ok := o.cfg.Modes[opts.Mode]; ok && m.HasApplications() {
			am := NewAppManager(o.cfg, envClone)
			strategy := m.AppStrategy("")
			if err := am.StartApps(ctx, AppStartOptions{
				Strategy: strategy,
				Wait:     opts.Wait,
				DryRun:   opts.DryRun,
				Mode:     opts.Mode,
			}); err != nil {
				fmt.Fprintf(os.Stderr, "[warn] application start: %v\n", err)
			}
		}
	}

	return nil
}

// Down stops all matching lifecycle entries in reverse order.
func (o *Orchestrator) Down(ctx context.Context, opts DownOptions) error {
	// Stop applications first (reverse of startup order)
	if len(o.cfg.Applications) > 0 {
		am := NewAppManager(o.cfg, o.env)
		am.StopApps()
	}

	// Stop native processes
	o.stopModeProcesses(opts.Mode)

	filtered := o.filterEntries(opts.IncludeTags, opts.ExcludeTags, opts.Mode, opts.Env)

	// Reverse order for teardown
	for i, j := 0, len(filtered)-1; i < j; i, j = i+1, j-1 {
		filtered[i], filtered[j] = filtered[j], filtered[i]
	}

	for _, entry := range filtered {
		pluginType := entry.DetectPlugin()
		plugin, err := NewPlugin(pluginType)
		if err != nil {
			return fmt.Errorf("entry %q: %w", entry.Name, err)
		}

		pctx := &PluginContext{
			Entry:        &entry,
			Env:          o.env,
			ConfigDir:    o.cfg.FileDir(),
			DryRun:       opts.DryRun,
			Volumes:      opts.Volumes,
			RemoveImages: opts.RemoveImages,
			Logger:       o.logger.With("entry", entry.Name, "plugin", pluginType),
		}

		fmt.Fprintf(os.Stderr, "[lifecycle] stopping %s (%s)\n", entry.Name, pluginType)

		if err := plugin.Down(ctx, pctx); err != nil {
			fmt.Fprintf(os.Stderr, "[warn] entry %q down failed: %v\n", entry.Name, err)
			// Continue with other entries — don't abort on single failure during teardown
		}
	}

	return nil
}

// Stop stops all matching lifecycle entries in reverse order without removing resources.
func (o *Orchestrator) Stop(ctx context.Context, opts StopOptions) error {
	// Stop applications first
	if len(o.cfg.Applications) > 0 {
		am := NewAppManager(o.cfg, o.env)
		am.StopApps()
	}

	// Stop native processes
	o.stopModeProcesses(opts.Mode)

	filtered := o.filterEntries(opts.IncludeTags, opts.ExcludeTags, opts.Mode, opts.Env)

	// Reverse order
	for i, j := 0, len(filtered)-1; i < j; i, j = i+1, j-1 {
		filtered[i], filtered[j] = filtered[j], filtered[i]
	}

	for _, entry := range filtered {
		pluginType := entry.DetectPlugin()
		plugin, err := NewPlugin(pluginType)
		if err != nil {
			return fmt.Errorf("entry %q: %w", entry.Name, err)
		}

		pctx := &PluginContext{
			Entry:     &entry,
			Env:       o.env,
			ConfigDir: o.cfg.FileDir(),
			DryRun:    opts.DryRun,
			Logger:    o.logger.With("entry", entry.Name, "plugin", pluginType),
		}

		fmt.Fprintf(os.Stderr, "[lifecycle] stopping %s (%s)\n", entry.Name, pluginType)

		if err := plugin.Stop(ctx, pctx); err != nil {
			fmt.Fprintf(os.Stderr, "[warn] entry %q stop failed: %v\n", entry.Name, err)
		}
	}

	return nil
}

// Restart stops then starts all matching entries.
func (o *Orchestrator) Restart(ctx context.Context, opts UpOptions) error {
	stopOpts := StopOptions{
		DryRun:      opts.DryRun,
		IncludeTags: opts.IncludeTags,
		ExcludeTags: opts.ExcludeTags,
		Mode:        opts.Mode,
	}
	if err := o.Stop(ctx, stopOpts); err != nil {
		return err
	}
	return o.Up(ctx, opts)
}

// Status queries the status of all lifecycle entries.
func (o *Orchestrator) Status(ctx context.Context) (*AggregatedStatus, error) {
	status := &AggregatedStatus{}

	for _, entry := range o.entries {
		pluginType := entry.DetectPlugin()
		plugin, err := NewPlugin(pluginType)
		if err != nil {
			continue
		}

		pctx := &PluginContext{
			Entry:     &entry,
			Env:       o.env,
			ConfigDir: o.cfg.FileDir(),
			Logger:    o.logger.With("entry", entry.Name, "plugin", pluginType),
		}

		services, _ := plugin.Status(ctx, pctx)

		var healthResults []HealthCheckResult
		if len(entry.HealthChecks) > 0 {
			healthResults = o.hc.Check(entry.HealthChecks)
		}

		status.Entries = append(status.Entries, EntryStatus{
			Name:     entry.Name,
			Plugin:   pluginType,
			Services: services,
			Health:   healthResults,
		})
	}

	return status, nil
}

// filterEntries returns lifecycle entries matching the given tag, mode, and env filters.
func (o *Orchestrator) filterEntries(includeTags, excludeTags []string, mode, env string) []config.LifecycleEntry {
	entries := o.entries

	// Filter by env (stack entry names)
	if env != "" {
		if ep, ok := o.cfg.Environments[env]; ok && len(ep.StackEntries()) > 0 {
			nameSet := make(map[string]bool, len(ep.StackEntries()))
			for _, name := range ep.StackEntries() {
				nameSet[name] = true
			}
			var filtered []config.LifecycleEntry
			for _, e := range entries {
				if nameSet[e.Name] {
					filtered = append(filtered, e)
				}
			}
			entries = filtered
		}
	}

	// Filter by mode (stack entry names) — narrows further if both env and mode specify
	if mode != "" {
		if m, ok := o.cfg.Modes[mode]; ok && len(m.StackEntries()) > 0 {
			nameSet := make(map[string]bool, len(m.StackEntries()))
			for _, name := range m.StackEntries() {
				nameSet[name] = true
			}
			var filtered []config.LifecycleEntry
			for _, e := range entries {
				if nameSet[e.Name] {
					filtered = append(filtered, e)
				}
			}
			entries = filtered
		}
	}

	// Filter by include tags
	if len(includeTags) > 0 {
		tagSet := make(map[string]bool, len(includeTags))
		for _, t := range includeTags {
			tagSet[t] = true
		}
		var filtered []config.LifecycleEntry
		for _, e := range entries {
			if hasAnyTag(e.Tags, tagSet) {
				filtered = append(filtered, e)
			}
		}
		entries = filtered
	}

	// Filter by exclude tags
	if len(excludeTags) > 0 {
		tagSet := make(map[string]bool, len(excludeTags))
		for _, t := range excludeTags {
			tagSet[t] = true
		}
		var filtered []config.LifecycleEntry
		for _, e := range entries {
			if !hasAnyTag(e.Tags, tagSet) {
				filtered = append(filtered, e)
			}
		}
		entries = filtered
	}

	return entries
}

// startModeProcesses launches native processes for mode health_checks that have a start command.
// This bridges the gap between compose-managed infra and natively-run app services.
func (o *Orchestrator) startModeProcesses(ctx context.Context, opts UpOptions, env *config.Environment) error {
	if opts.Mode == "" {
		return nil
	}
	mode, ok := o.cfg.Modes[opts.Mode]
	if !ok || len(mode.HealthChecks) == 0 {
		return nil
	}

	type nativeProc struct {
		name string
		hc   config.HealthCheckConfig
	}

	// Collect startable processes
	var procs []nativeProc
	for _, hcName := range mode.HealthChecks {
		hc, ok := o.cfg.HealthChecks[hcName]
		if !ok || hc.Start == "" {
			continue
		}

		if opts.DryRun {
			fmt.Fprintf(os.Stderr, "[native] (dry-run) would start %s: %s\n", hcName, hc.Start)
			continue
		}

		// Check if already running
		pidFile := fmt.Sprintf("%s/%s/pids/%s.pid", o.cfg.FileDir(), config.DotDirName, hcName)
		if data, err := os.ReadFile(pidFile); err == nil {
			pidStr := strings.TrimSpace(string(data))
			if pid := 0; true {
				fmt.Sscanf(pidStr, "%d", &pid)
				if pid > 0 && IsProcessRunning(pid) {
					fmt.Fprintf(os.Stderr, "[native] %s already running (pid %d)\n", hcName, pid)
					continue
				}
			}
		}

		procs = append(procs, nativeProc{name: hcName, hc: hc})
	}

	// Start all native processes concurrently
	var mu sync.Mutex
	var firstErr error
	var wg sync.WaitGroup

	for _, p := range procs {
		wg.Add(1)
		go func(p nativeProc) {
			defer wg.Done()

			dir := o.cfg.FileDir()
			pctx := &PluginContext{
				Entry: &config.LifecycleEntry{
					Name: p.name,
				},
				Env:       env,
				ConfigDir: o.cfg.FileDir(),
				DryRun:    opts.DryRun,
				Logger:    o.logger.With("native", p.name),
			}

			fmt.Fprintf(os.Stderr, "[native] starting %s\n", p.name)
			if err := startLocalProcess(p.name, p.hc.Start, dir, pctx); err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = fmt.Errorf("native start %s: %w", p.name, err)
				}
				mu.Unlock()
				return
			}
			fmt.Fprintf(os.Stderr, "[+] started %s\n", p.name)

			// Wait for health check readiness if --wait
			if opts.Wait {
				checks := map[string]config.HealthCheckConfig{p.name: p.hc}
				readyTimeout := time.Duration(p.hc.ReadyTimeout) * time.Second
				if readyTimeout == 0 {
					readyTimeout = 30 * time.Second
				}
				waitCtx, cancel := context.WithTimeout(ctx, readyTimeout)
				results := o.hc.WaitUntilReady(waitCtx, checks)
				cancel()
				for _, r := range results {
					if !r.Ready {
						fmt.Fprintf(os.Stderr, "[warn] %s not ready after %s\n", p.name, readyTimeout)
					}
				}
			}
		}(p)
	}

	wg.Wait()
	return firstErr
}

// stopModeProcesses stops native processes that were started via mode health_checks.
func (o *Orchestrator) stopModeProcesses(mode string) {
	if mode == "" {
		return
	}
	m, ok := o.cfg.Modes[mode]
	if !ok || len(m.HealthChecks) == 0 {
		return
	}

	for _, hcName := range m.HealthChecks {
		hc, ok := o.cfg.HealthChecks[hcName]
		if !ok || hc.Start == "" {
			continue
		}

		pidFile := fmt.Sprintf("%s/%s/pids/%s.pid", o.cfg.FileDir(), config.DotDirName, hcName)
		data, err := os.ReadFile(pidFile)
		if err != nil {
			continue
		}

		var pid int
		fmt.Sscanf(strings.TrimSpace(string(data)), "%d", &pid)
		if pid > 0 {
			if err := syscall.Kill(-pid, syscall.SIGTERM); err == nil {
				fmt.Fprintf(os.Stderr, "[-] stopped %s (pid %d)\n", hcName, pid)
			}
			os.Remove(pidFile)
		}
	}
}

// hasAnyTag returns true if any of the entry's tags exist in the tag set.
func hasAnyTag(tags []string, tagSet map[string]bool) bool {
	for _, t := range tags {
		if tagSet[t] {
			return true
		}
	}
	return false
}

// cloneEnv creates a shallow copy of an Environment with a new Vars map.
func cloneEnv(e *config.Environment) *config.Environment {
	clone := config.NewEnvironment(nil, e.WorkDir(), e.CfgDir())
	for k, v := range e.Vars {
		clone.Vars[k] = v
	}
	return clone
}
