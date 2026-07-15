package lifecycle

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
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
	Names       []string // specific stack entry names (empty = all)
	IncludeTags []string
	ExcludeTags []string
	Mode        string
	Env         string
}

// DownOptions configures orchestrator Down behavior.
type DownOptions struct {
	DryRun       bool
	Volumes      bool     // also remove named volumes
	RemoveImages bool     // also remove locally built images
	Names        []string // specific stack entry names (empty = all)
	IncludeTags  []string
	ExcludeTags  []string
	Mode         string
	Env          string
}

// StopOptions configures orchestrator Stop behavior.
type StopOptions struct {
	DryRun      bool
	Names       []string // specific stack entry names (empty = all)
	IncludeTags []string
	ExcludeTags []string
	Mode        string
	Env         string
}

// Orchestrator coordinates lifecycle plugin execution in order.
type Orchestrator struct {
	entries         []config.LifecycleEntry
	composeServices map[string][]string
	cfg             *config.Config
	env             *config.Environment
	logger          *slog.Logger
	hc              *HealthChecker
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
	filtered, err := o.filterEntries(opts.Names, opts.IncludeTags, opts.ExcludeTags, opts.Mode, opts.Env)
	if err != nil {
		return err
	}
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

		entryComposeServices := modeServices
		if services, ok := o.composeServices[entry.Name]; ok {
			selected := append([]string(nil), services...)
			entryComposeServices = &selected
		}

		pctx := &PluginContext{
			Entry:           &entry,
			Env:             envClone,
			ConfigDir:       o.cfg.FileDir(),
			DryRun:          opts.DryRun,
			Force:           opts.Force,
			Wait:            opts.Wait,
			ComposeProfiles: modeProfiles,
			ComposeServices: entryComposeServices,
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

	return nil
}

// Down stops all matching lifecycle entries in reverse order.
func (o *Orchestrator) Down(ctx context.Context, opts DownOptions) error {
	o.stopModeProcesses(opts.Mode)

	filtered, err := o.filterEntries(opts.Names, opts.IncludeTags, opts.ExcludeTags, opts.Mode, opts.Env)
	if err != nil {
		return err
	}

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

		var entryComposeServices *[]string
		if services, ok := o.composeServices[entry.Name]; ok {
			selected := append([]string(nil), services...)
			entryComposeServices = &selected
		}

		pctx := &PluginContext{
			Entry:           &entry,
			Env:             o.env,
			ConfigDir:       o.cfg.FileDir(),
			DryRun:          opts.DryRun,
			Volumes:         opts.Volumes,
			RemoveImages:    opts.RemoveImages,
			ComposeServices: entryComposeServices,
			Logger:          o.logger.With("entry", entry.Name, "plugin", pluginType),
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
	o.haltModeProcesses(opts.Mode)

	filtered, err := o.filterEntries(opts.Names, opts.IncludeTags, opts.ExcludeTags, opts.Mode, opts.Env)
	if err != nil {
		return err
	}

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

		var entryComposeServices *[]string
		if services, ok := o.composeServices[entry.Name]; ok {
			selected := append([]string(nil), services...)
			entryComposeServices = &selected
		}

		pctx := &PluginContext{
			Entry:           &entry,
			Env:             o.env,
			ConfigDir:       o.cfg.FileDir(),
			DryRun:          opts.DryRun,
			ComposeServices: entryComposeServices,
			Logger:          o.logger.With("entry", entry.Name, "plugin", pluginType),
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

		services, err := plugin.Status(ctx, pctx)
		if err != nil {
			o.logger.Warn("plugin status query failed", "entry", entry.Name, "plugin", pluginType, "error", err)
		}

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

// filterEntries returns lifecycle entries matching the given name, tag, mode, and env filters.
// It also applies StackOverrides for the given environment if configured.
func (o *Orchestrator) filterEntries(names, includeTags, excludeTags []string, mode, env string) ([]config.LifecycleEntry, error) {
	entries := o.entries

	// Filter by explicit entry names
	if len(names) > 0 {
		entries = filterByNames(entries, names)
	}

	// Filter by env (stack entry names)
	if env != "" {
		if ep, ok := o.cfg.Environments[env]; ok && len(ep.StackEntries()) > 0 {
			entries = filterByNames(entries, ep.StackEntries())
		}
	}

	// Filter by mode (stack entry names) — narrows further if both env and mode specify
	if mode != "" {
		if m, ok := o.cfg.Modes[mode]; ok && len(m.StackEntries()) > 0 {
			entries = filterByNames(entries, m.StackEntries())
		}
	}

	// Filter by include tags
	if len(includeTags) > 0 {
		entries = filterByTags(entries, includeTags, false)
	}

	// Filter by exclude tags
	if len(excludeTags) > 0 {
		entries = filterByTags(entries, excludeTags, true)
	}

	// Apply overrides after filtering is complete
	if env != "" {
		if ep, ok := o.cfg.Environments[env]; ok && len(ep.StackOverrides) > 0 {
			for i := range entries {
				if override, exists := ep.StackOverrides[entries[i].Name]; exists {
					merged, err := config.MergeLifecycleEntry(&entries[i], override)
					if err != nil {
						return nil, fmt.Errorf("applying env %q stack_override for %q: %w", env, entries[i].Name, err)
					}
					entries[i] = *merged
				}
			}
		}
	}

	return entries, nil
}

// filterByNames retains only the entries whose names exist in targetNames.
func filterByNames(entries []config.LifecycleEntry, targetNames []string) []config.LifecycleEntry {
	nameSet := make(map[string]bool, len(targetNames))
	for _, n := range targetNames {
		nameSet[n] = true
	}
	var filtered []config.LifecycleEntry
	for _, e := range entries {
		if nameSet[e.Name] {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

// filterByTags retains entries based on tag matching. If exclude is true, matching entries are excluded.
func filterByTags(entries []config.LifecycleEntry, tags []string, exclude bool) []config.LifecycleEntry {
	tagSet := make(map[string]bool, len(tags))
	for _, t := range tags {
		tagSet[t] = true
	}
	var filtered []config.LifecycleEntry
	for _, e := range entries {
		hasMatch := hasAnyTag(e.Tags, tagSet)
		if (exclude && !hasMatch) || (!exclude && hasMatch) {
			filtered = append(filtered, e)
		}
	}
	return filtered
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
		pidFile := filepath.Join(o.cfg.FileDir(), config.DotDirName, config.PidsDirName, hcName+".pid")
		if data, err := os.ReadFile(pidFile); err == nil {
			pidStr := strings.TrimSpace(string(data))
			if pid := 0; true {
				_, _ = fmt.Sscanf(pidStr, "%d", &pid)
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

// haltModeProcesses sends SIGTERM to mode health_check processes but preserves
// PID files so they can be restarted by the next `up` call (halt semantics).
func (o *Orchestrator) haltModeProcesses(mode string) {
	o.signalModeProcesses(mode, false)
}

// stopModeProcesses sends SIGTERM to mode health_check processes and removes
// PID files (destroy semantics).
func (o *Orchestrator) stopModeProcesses(mode string) {
	o.signalModeProcesses(mode, true)
}

// signalModeProcesses terminates health_check native processes. When removePID
// is true, the PID files are deleted after signalling (down semantics).
func (o *Orchestrator) signalModeProcesses(mode string, removePID bool) {
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

		pidFile := filepath.Join(o.cfg.FileDir(), config.DotDirName, config.PidsDirName, hcName+".pid")
		data, err := os.ReadFile(pidFile)
		if err != nil {
			continue
		}

		var pid int
		_, _ = fmt.Sscanf(strings.TrimSpace(string(data)), "%d", &pid)
		if pid > 0 {
			if err := syscall.Kill(-pid, syscall.SIGTERM); err == nil {
				fmt.Fprintf(os.Stderr, "[-] stopped %s (pid %d)\n", hcName, pid)
			}
			if removePID {
				_ = os.Remove(pidFile)
			}
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
