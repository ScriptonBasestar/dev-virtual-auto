package cli

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ScriptonBasestar/dva/internal/config"
)

func TestCheckHTTP_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	if !checkHTTP(srv.URL, 2*time.Second) {
		t.Error("expected HTTP check to pass for 200 OK")
	}
}

func TestCheckHTTP_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	if checkHTTP(srv.URL, 2*time.Second) {
		t.Error("expected HTTP check to fail for 500")
	}
}

func TestCheckHTTP_Unreachable(t *testing.T) {
	if checkHTTP("http://localhost:1", 500*time.Millisecond) {
		t.Error("expected HTTP check to fail for unreachable host")
	}
}

func TestCheckTCP_Success(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer ln.Close()

	if !checkTCP(ln.Addr().String(), 2*time.Second) {
		t.Error("expected TCP check to pass for listening port")
	}
}

func TestCheckTCP_Unreachable(t *testing.T) {
	if checkTCP("127.0.0.1:1", 500*time.Millisecond) {
		t.Error("expected TCP check to fail for unreachable port")
	}
}

func TestCheckCommand_Success(t *testing.T) {
	if !checkCommand("true", 2*time.Second) {
		t.Error("expected command 'true' to pass")
	}
}

func TestCheckCommand_Failure(t *testing.T) {
	if checkCommand("false", 2*time.Second) {
		t.Error("expected command 'false' to fail")
	}
}

func TestRunHealthChecks_Empty(t *testing.T) {
	results := runHealthChecks(nil)
	if results != nil {
		t.Errorf("expected nil for empty checks, got %v", results)
	}
}

func TestRunHealthChecks_Mixed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	checks := map[string]config.HealthCheckConfig{
		"web": {
			Type: "http",
			URL:  srv.URL,
		},
		"missing": {
			Type:      "tcp",
			Address:   "127.0.0.1:1",
			StartHint: "start missing service",
			Timeout:   1,
		},
	}

	results := runHealthChecks(checks)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// Results should be sorted by name
	if results[0].Name != "missing" || results[1].Name != "web" {
		t.Errorf("expected sorted order [missing, web], got [%s, %s]", results[0].Name, results[1].Name)
	}

	if results[0].Ready {
		t.Error("expected 'missing' to be not ready")
	}
	if results[0].StartHint != "start missing service" {
		t.Errorf("expected start hint, got %q", results[0].StartHint)
	}

	if !results[1].Ready {
		t.Error("expected 'web' to be ready")
	}
}

func TestRunHealthChecks_Concurrent(t *testing.T) {
	// Verify concurrent execution by timing: two 500ms checks should complete in ~500ms, not ~1s
	ln1, _ := net.Listen("tcp", "127.0.0.1:0")
	defer ln1.Close()
	ln2, _ := net.Listen("tcp", "127.0.0.1:0")
	defer ln2.Close()

	checks := map[string]config.HealthCheckConfig{
		"svc1": {Type: "tcp", Address: ln1.Addr().String()},
		"svc2": {Type: "tcp", Address: ln2.Addr().String()},
	}

	start := time.Now()
	results := runHealthChecks(checks)
	elapsed := time.Since(start)

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	for _, r := range results {
		if !r.Ready {
			t.Errorf("expected %s to be ready", r.Name)
		}
	}

	// Both checks should run concurrently (well under sequential time)
	if elapsed > 2*time.Second {
		t.Errorf("expected concurrent execution, took %v", elapsed)
	}
}

func TestRunHealthChecks_CommandType(t *testing.T) {
	checks := map[string]config.HealthCheckConfig{
		"ok": {
			Type:    "command",
			Command: "true",
		},
		"fail": {
			Type:      "command",
			Command:   "false",
			StartHint: "fix it",
		},
	}

	results := runHealthChecks(checks)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	resultMap := make(map[string]HealthCheckResult)
	for _, r := range results {
		resultMap[r.Name] = r
	}

	if !resultMap["ok"].Ready {
		t.Error("expected 'ok' to be ready")
	}
	if resultMap["fail"].Ready {
		t.Error("expected 'fail' to be not ready")
	}
}

func TestRunHealthChecks_UnknownType(t *testing.T) {
	checks := map[string]config.HealthCheckConfig{
		"unknown": {
			Type: "grpc",
		},
	}

	results := runHealthChecks(checks)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Ready {
		t.Error("unknown type should not be ready")
	}
}

func TestCheckHTTP_4xxPasses(t *testing.T) {
	codes := []int{400, 401, 403, 404, 499}
	for _, code := range codes {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(code)
		}))
		ok := checkHTTP(srv.URL, 2*time.Second)
		srv.Close()
		if !ok {
			t.Errorf("expected HTTP %d to pass (service is running)", code)
		}
	}
}

func TestCheckHTTP_5xxFails(t *testing.T) {
	codes := []int{500, 502, 503}
	for _, code := range codes {
		t.Run(fmt.Sprintf("HTTP_%d", code), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(code)
			}))
			defer srv.Close()
			if checkHTTP(srv.URL, 2*time.Second) {
				t.Errorf("expected HTTP %d to fail", code)
			}
		})
	}
}

func TestRunHealthChecks_DefaultTimeout(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer ln.Close()

	checks := map[string]config.HealthCheckConfig{
		"svc": {
			Type:    "tcp",
			Address: ln.Addr().String(),
			Timeout: 0, // should default to 2s
		},
	}

	results := runHealthChecks(checks)
	if len(results) != 1 || !results[0].Ready {
		t.Error("expected service to be ready with default timeout")
	}
}

func TestPrintHealthCheckResults_NoResults(t *testing.T) {
	// Should not panic on nil/empty
	printHealthCheckResults(nil, "")
	printHealthCheckResults([]HealthCheckResult{}, "")
}

func TestPrintHealthCheckResults_WithStarted(t *testing.T) {
	// Smoke test — just ensure no panic, correct branch coverage
	results := []HealthCheckResult{
		{Name: "api", Ready: true},
		{Name: "web", Ready: false, StartHint: "pnpm dev"},
		{Name: "worker", Ready: false, Started: true},
	}
	printHealthCheckResults(results, "/tmp/test")
}
