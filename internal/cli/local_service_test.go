package cli

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ScriptonBasestar/dva/internal/config"
)

func TestStartLocalService(t *testing.T) {
	tmpDir := t.TempDir()

	err := startLocalService("test-svc", "sleep 30", tmpDir)
	if err != nil {
		t.Fatalf("failed to start service: %v", err)
	}

	// Verify PID file created
	pidFile := filepath.Join(tmpDir, ".dva", "pids", "test-svc.pid")
	data, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatalf("PID file not created: %v", err)
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		t.Fatalf("invalid PID in file: %s", string(data))
	}

	// Verify process is running
	if !isProcessRunning(pid) {
		t.Error("expected process to be running")
	}

	// Verify log file created
	logFile := filepath.Join(tmpDir, ".dva", "logs", "test-svc.log")
	if _, err := os.Stat(logFile); err != nil {
		t.Errorf("log file not created: %v", err)
	}

	// Cleanup
	stopLocalServices(tmpDir)

	// Verify process stopped
	time.Sleep(100 * time.Millisecond)
	if isProcessRunning(pid) {
		t.Error("expected process to be stopped after stopLocalServices")
	}

	// Verify PID file cleaned up
	if _, err := os.Stat(pidFile); !os.IsNotExist(err) {
		t.Error("expected PID file to be removed")
	}
}

func TestStopLocalServices_NoPidDir(t *testing.T) {
	// Should not panic when .dva/pids doesn't exist
	stopLocalServices(t.TempDir())
}

func TestStopLocalServices_StalePid(t *testing.T) {
	tmpDir := t.TempDir()
	pidDir := filepath.Join(tmpDir, ".dva", "pids")
	os.MkdirAll(pidDir, 0755)

	// Write a PID that doesn't exist (99999999)
	pidFile := filepath.Join(pidDir, "stale.pid")
	os.WriteFile(pidFile, []byte("99999999"), 0644)

	stopLocalServices(tmpDir)

	// PID file should be cleaned up
	if _, err := os.Stat(pidFile); !os.IsNotExist(err) {
		t.Error("expected stale PID file to be removed")
	}
}

func TestIsProcessRunning(t *testing.T) {
	// Current process should be running
	if !isProcessRunning(os.Getpid()) {
		t.Error("expected current process to be running")
	}

	// Non-existent PID should not be running
	if isProcessRunning(99999999) {
		t.Error("expected non-existent PID to not be running")
	}
}

func TestStartUnreadyServices_NoStart(t *testing.T) {
	checks := map[string]config.HealthCheckConfig{
		"web": {Type: "tcp", Address: "127.0.0.1:1"},
	}
	results := []HealthCheckResult{
		{Name: "web", Ready: false},
	}

	started := startUnreadyServices(checks, results, t.TempDir())
	if len(started) != 0 {
		t.Error("expected no services started when no start command configured")
	}
}

func TestStartUnreadyServices_AlreadyReady(t *testing.T) {
	checks := map[string]config.HealthCheckConfig{
		"web": {Type: "tcp", Address: "127.0.0.1:1", Start: "sleep 30"},
	}
	results := []HealthCheckResult{
		{Name: "web", Ready: true},
	}

	started := startUnreadyServices(checks, results, t.TempDir())
	if len(started) != 0 {
		t.Error("expected no services started when already ready")
	}
}

func TestStartUnreadyServices_WithStart(t *testing.T) {
	tmpDir := t.TempDir()

	checks := map[string]config.HealthCheckConfig{
		"worker": {Type: "command", Command: "false", Start: "sleep 30"},
	}
	results := []HealthCheckResult{
		{Name: "worker", Ready: false},
	}

	started := startUnreadyServices(checks, results, tmpDir)
	if !started["worker"] {
		t.Error("expected worker to be started")
	}

	// Cleanup
	stopLocalServices(tmpDir)
}

func TestMaxReadyTimeout_NoStarted(t *testing.T) {
	checks := map[string]config.HealthCheckConfig{
		"pg": {ReadyTimeout: 60},
	}
	got := maxReadyTimeout(checks, nil)
	if got != defaultReadyTimeout {
		t.Errorf("maxReadyTimeout = %v, want %v", got, defaultReadyTimeout)
	}
}

func TestMaxReadyTimeout_UsesConfigValue(t *testing.T) {
	checks := map[string]config.HealthCheckConfig{
		"pg":    {ReadyTimeout: 60},
		"redis": {ReadyTimeout: 10},
	}
	started := map[string]bool{"pg": true, "redis": true}
	got := maxReadyTimeout(checks, started)
	want := 60 * time.Second
	if got != want {
		t.Errorf("maxReadyTimeout = %v, want %v", got, want)
	}
}

func TestMaxReadyTimeout_FallsBackToDefault(t *testing.T) {
	checks := map[string]config.HealthCheckConfig{
		"pg": {ReadyTimeout: 0},
	}
	started := map[string]bool{"pg": true}
	got := maxReadyTimeout(checks, started)
	if got != defaultReadyTimeout {
		t.Errorf("maxReadyTimeout = %v, want %v", got, defaultReadyTimeout)
	}
}

func TestMaxReadyTimeout_MissingCheck(t *testing.T) {
	checks := map[string]config.HealthCheckConfig{}
	started := map[string]bool{"unknown": true}
	got := maxReadyTimeout(checks, started)
	if got != defaultReadyTimeout {
		t.Errorf("maxReadyTimeout = %v, want %v", got, defaultReadyTimeout)
	}
}

func TestStartUnreadyServices_SkipsAlreadyRunning(t *testing.T) {
	tmpDir := t.TempDir()

	// Start a service first
	err := startLocalService("worker", "sleep 30", tmpDir)
	if err != nil {
		t.Fatalf("failed to start: %v", err)
	}

	checks := map[string]config.HealthCheckConfig{
		"worker": {Type: "command", Command: "false", Start: "sleep 30"},
	}
	results := []HealthCheckResult{
		{Name: "worker", Ready: false},
	}

	// Should skip because PID file exists and process is running
	started := startUnreadyServices(checks, results, tmpDir)
	if started["worker"] {
		t.Error("expected worker to be skipped since already running")
	}

	// Cleanup
	stopLocalServices(tmpDir)
}
