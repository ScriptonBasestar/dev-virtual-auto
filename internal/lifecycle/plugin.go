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
	Entry     *config.LifecycleEntry
	Env       *config.Environment
	ConfigDir string
	DryRun    bool
	Force     bool
	Wait      bool
	Logger    *slog.Logger
}

// Result holds the output of a plugin Up() call.
type Result struct {
	Exports  map[string]string // variables to pass to subsequent plugins
	Services []ServiceStatus
}

// ServiceStatus represents the state of a single managed service.
type ServiceStatus struct {
	Name   string            // "postgres", "api-server"
	State  string            // running, stopped, error
	Health string            // healthy, unhealthy, unknown
	Ports  map[int]int       // host:container port mappings
	Extra  map[string]string // plugin-specific metadata
}
