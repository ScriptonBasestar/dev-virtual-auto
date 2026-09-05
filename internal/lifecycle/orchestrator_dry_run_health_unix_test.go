//go:build !windows

package lifecycle

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// TestUpDryRunSkipsEntryHealthWait covers the entry-level health wait in Orchestrator.Up.
//
// `dva --dry-run up <plan>` starts nothing, yet the wait was gated only on opts.Wait (the
// CLI default), so a native entry with an http health check polled an address that could
// never come up until ctx was cancelled — the TASK-312 hang. Dry-run must report the wait
// it would perform and return immediately.
func TestUpDryRunSkipsEntryHealthWait(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{
		Stack: map[string]*config.LifecycleEntry{
			"api": {
				Order:  1,
				Script: &config.ScriptPluginConfig{Up: "true"},
				HealthChecks: map[string]config.HealthCheckConfig{
					// Port 9 (discard) is closed on a dev box; the address only has to be unreachable.
					"api": {Type: "http", URL: "http://127.0.0.1:9/health/live", ReadyTimeout: 1},
				},
			},
		},
	}
	orch := NewOrchestrator(cfg, config.NewEnvironment(nil, dir, dir))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	start := time.Now()
	out, err := captureStderr(t, func() error {
		return orch.Up(ctx, UpOptions{DryRun: true, Wait: true})
	})
	if err != nil {
		t.Fatalf("dry-run Up returned error: %v\noutput:\n%s", err, out)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("dry-run Up waited on health checks for %s; output:\n%s", elapsed, out)
	}
	want := `[health] (dry-run) would wait for entry "api": api=http http://127.0.0.1:9/health/live (ready_timeout=1s)`
	if !strings.Contains(out, want) {
		t.Fatalf("dry-run output missing %q; got:\n%s", want, out)
	}
	if strings.Contains(out, "some health checks not ready") {
		t.Fatalf("dry-run still ran the health wait; output:\n%s", out)
	}
}
