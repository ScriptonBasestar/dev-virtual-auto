package lifecycle

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

func TestProcessBackedPluginsDryRunPreservesPidState(t *testing.T) {
	tests := []struct {
		name   string
		plugin LifecyclePlugin
		entry  *config.LifecycleEntry
		action func(LifecyclePlugin, *PluginContext) error
	}{
		{"process down", &ProcessPlugin{}, &config.LifecycleEntry{Name: "preview"}, func(p LifecyclePlugin, c *PluginContext) error { return p.Down(context.Background(), c) }},
		{"sam down", &SAMPlugin{}, &config.LifecycleEntry{Name: "preview", SAM: &config.SAMPluginConfig{}}, func(p LifecyclePlugin, c *PluginContext) error { return p.Down(context.Background(), c) }},
		{"sam stop", &SAMPlugin{}, &config.LifecycleEntry{Name: "preview", SAM: &config.SAMPluginConfig{}}, func(p LifecyclePlugin, c *PluginContext) error { return p.Stop(context.Background(), c) }},
		{"serverless down", &ServerlessPlugin{}, &config.LifecycleEntry{Name: "preview", Serverless: &config.ServerlessPluginConfig{}}, func(p LifecyclePlugin, c *PluginContext) error { return p.Down(context.Background(), c) }},
		{"serverless stop", &ServerlessPlugin{}, &config.LifecycleEntry{Name: "preview", Serverless: &config.ServerlessPluginConfig{}}, func(p LifecyclePlugin, c *PluginContext) error { return p.Stop(context.Background(), c) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			pidDir := filepath.Join(tmpDir, config.DotDirName, config.PidsDirName)
			if err := os.MkdirAll(pidDir, 0755); err != nil {
				t.Fatal(err)
			}
			pidFile := filepath.Join(pidDir, "preview.pid")
			if err := os.WriteFile(pidFile, []byte("invalid-pid"), 0644); err != nil {
				t.Fatal(err)
			}

			pctx := &PluginContext{
				Entry:     tt.entry,
				ConfigDir: tmpDir,
				DryRun:    true,
				Logger:    slog.Default(),
			}
			if err := tt.action(tt.plugin, pctx); err != nil {
				t.Fatalf("dry-run action failed: %v", err)
			}
			if _, err := os.Stat(pidFile); err != nil {
				t.Fatalf("dry-run removed PID state: %v", err)
			}
		})
	}
}
