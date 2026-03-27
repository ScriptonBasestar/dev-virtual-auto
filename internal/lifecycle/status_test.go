package lifecycle

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func captureStdout(fn func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	return buf.String()
}

func TestPrintStatus_Empty(t *testing.T) {
	out := captureStdout(func() {
		PrintStatus(&AggregatedStatus{}, "/tmp")
	})

	if !strings.Contains(out, "no entries configured") {
		t.Errorf("expected 'no entries configured' message, got %q", out)
	}
}

func TestPrintStatus_WithEntries(t *testing.T) {
	status := &AggregatedStatus{
		Entries: []EntryStatus{
			{
				Name:   "db",
				Plugin: "compose",
				Services: []ServiceStatus{
					{Name: "postgres", State: "running", Health: "healthy"},
				},
			},
			{
				Name:   "cache",
				Plugin: "compose",
				Services: []ServiceStatus{
					{Name: "redis", State: "running"},
				},
			},
		},
	}

	out := captureStdout(func() {
		PrintStatus(status, "/tmp")
	})

	if !strings.Contains(out, "Lifecycle:") {
		t.Error("expected 'Lifecycle:' header")
	}
	if !strings.Contains(out, "[db]") {
		t.Error("expected '[db]' entry")
	}
	if !strings.Contains(out, "postgres") {
		t.Error("expected 'postgres' service")
	}
	if !strings.Contains(out, "healthy") {
		t.Error("expected 'healthy' health status")
	}
	if !strings.Contains(out, "[cache]") {
		t.Error("expected '[cache]' entry")
	}
	if !strings.Contains(out, "redis") {
		t.Error("expected 'redis' service")
	}
}

func TestPrintStatus_WithHealthChecks(t *testing.T) {
	status := &AggregatedStatus{
		Entries: []EntryStatus{
			{
				Name:   "web",
				Plugin: "process",
				Health: []HealthCheckResult{
					{Name: "api", Ready: true},
					{Name: "db", Ready: false, Started: true},
				},
			},
		},
	}

	out := captureStdout(func() {
		PrintStatus(status, "/tmp")
	})

	if !strings.Contains(out, "ready") {
		t.Error("expected 'ready' status")
	}
	if !strings.Contains(out, "starting") {
		t.Error("expected 'starting' status for non-ready started service")
	}
}

func TestPrintStatus_ServiceWithEmptyHealth(t *testing.T) {
	status := &AggregatedStatus{
		Entries: []EntryStatus{
			{
				Name:   "app",
				Plugin: "compose",
				Services: []ServiceStatus{
					{Name: "api", State: "running", Health: ""},
				},
			},
		},
	}

	out := captureStdout(func() {
		PrintStatus(status, "/tmp")
	})

	// Empty health should display as "-"
	if !strings.Contains(out, "-") {
		t.Error("expected '-' for empty health")
	}
}
