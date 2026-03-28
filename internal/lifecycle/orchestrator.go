package lifecycle

import (
	"context"
	"fmt"
	"log/slog"
	"os"

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
}

// DownOptions configures orchestrator Down behavior.
type DownOptions struct {
	DryRun       bool
	Volumes      bool // also remove named volumes
	RemoveImages bool // also remove locally built images
	IncludeTags  []string
	ExcludeTags  []string
	Mode         string
}

// StopOptions configures orchestrator Stop behavior.
type StopOptions struct {
	DryRun      bool
	IncludeTags []string
	ExcludeTags []string
	Mode        string
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
	entries := cfg.SortedLifecycle()

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
	filtered := o.filterEntries(opts.IncludeTags, opts.ExcludeTags, opts.Mode)
	if len(filtered) == 0 {
		fmt.Fprintln(os.Stderr, "[warn] no lifecycle entries matched filters")
		return nil
	}

	// Clone env so exports accumulate without mutating the original
	envClone := cloneEnv(o.env)

	for _, entry := range filtered {
		pluginType := entry.DetectPlugin()
		plugin, err := NewPlugin(pluginType)
		if err != nil {
			return fmt.Errorf("entry %q: %w", entry.Name, err)
		}

		pctx := &PluginContext{
			Entry:     &entry,
			Env:       envClone,
			ConfigDir: o.cfg.FileDir(),
			DryRun:    opts.DryRun,
			Force:     opts.Force,
			Wait:      opts.Wait,
			Logger:    o.logger.With("entry", entry.Name, "plugin", pluginType),
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

	return nil
}

// Down stops all matching lifecycle entries in reverse order.
func (o *Orchestrator) Down(ctx context.Context, opts DownOptions) error {
	filtered := o.filterEntries(opts.IncludeTags, opts.ExcludeTags, opts.Mode)

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
	filtered := o.filterEntries(opts.IncludeTags, opts.ExcludeTags, opts.Mode)

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

// filterEntries returns lifecycle entries matching the given tag and mode filters.
func (o *Orchestrator) filterEntries(includeTags, excludeTags []string, mode string) []config.LifecycleEntry {
	entries := o.entries

	// Filter by mode (lifecycle entry names)
	if mode != "" {
		if m, ok := o.cfg.Modes[mode]; ok && len(m.Lifecycle) > 0 {
			nameSet := make(map[string]bool, len(m.Lifecycle))
			for _, name := range m.Lifecycle {
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
