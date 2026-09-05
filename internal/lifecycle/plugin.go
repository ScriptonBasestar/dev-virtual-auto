package lifecycle

import (
	"context"
	"log/slog"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// LifecyclePlugin defines the interface for lifecycle management plugins.
// Each plugin handles a specific runtime backend (compose, process, k8s, script).
type LifecyclePlugin interface {
	Name() string
	Up(ctx context.Context, pctx *PluginContext) (*Result, error)
	Down(ctx context.Context, pctx *PluginContext) error
	Stop(ctx context.Context, pctx *PluginContext) error
	Status(ctx context.Context, pctx *PluginContext) ([]ServiceStatus, error)
}

// PluginContext provides runtime context to each plugin invocation.
type PluginContext struct {
	Entry        *config.LifecycleEntry
	Env          *config.Environment
	ConfigDir    string
	DryRun       bool
	Force        bool
	Wait         bool
	Volumes      bool // clean: also remove named volumes
	RemoveImages bool // clean: also remove locally built images
	// Purge tears down the whole compose project — every service, named volume, network and
	// local image — even when ComposeServices selects a subset. `compose rm` cannot reach
	// project-scoped resources, so a plan-scoped down that must leave a clean slate has to
	// widen to `compose down` (TASK-311).
	Purge  bool
	Logger *slog.Logger

	// Mode-derived compose hints (set by orchestrator when a mode is active)
	ComposeProfiles []string  // --profile flags for docker compose
	ComposeServices *[]string // service names to append to compose up (nil=all)
}

// Result holds the output of a plugin Up() call.
type Result struct {
	Exports  map[string]string // variables to pass to subsequent plugins
	Services []ServiceStatus
}

// ServiceStatus represents the state of a single managed service.
type ServiceStatus struct {
	Name   string            `json:"name"`             // "postgres", "api-server"
	State  string            `json:"state"`            // running, stopped, error
	Health string            `json:"health,omitempty"` // healthy, unhealthy, unknown
	Ports  map[int]int       `json:"ports,omitempty"`  // host:container port mappings
	Extra  map[string]string `json:"extra,omitempty"`  // plugin-specific metadata
}
