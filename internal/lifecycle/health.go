package lifecycle

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"sync"
	"time"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// HealthCheckResult holds the outcome of a single health check.
type HealthCheckResult struct {
	Name      string `json:"name"`
	Ready     bool   `json:"ready"`
	Started   bool   `json:"started,omitempty"`
	StartHint string `json:"start_hint,omitempty"`
}

// HealthChecker runs health checks against configured endpoints.
type HealthChecker struct{}

// Check runs all configured health checks concurrently and returns results.
func (hc *HealthChecker) Check(checks map[string]config.HealthCheckConfig) []HealthCheckResult {
	if len(checks) == 0 {
		return nil
	}

	results := make([]HealthCheckResult, 0, len(checks))
	var mu sync.Mutex
	var wg sync.WaitGroup

	for name, check := range checks {
		wg.Add(1)
		go func(name string, check config.HealthCheckConfig) {
			defer wg.Done()

			result := HealthCheckResult{
				Name:      name,
				StartHint: check.StartHint,
			}

			timeout := time.Duration(check.Timeout) * time.Second
			if timeout == 0 {
				timeout = 2 * time.Second
			}

			switch check.Type {
			case "http":
				result.Ready = CheckHTTP(check.URL, timeout)
			case "tcp":
				result.Ready = CheckTCP(check.Address, timeout)
			case "command":
				result.Ready = checkCommand(check.Command, timeout)
			default:
				fmt.Fprintf(os.Stderr, "[warn] unknown health check type %q for %s\n", check.Type, name)
			}

			mu.Lock()
			results = append(results, result)
			mu.Unlock()
		}(name, check)
	}

	wg.Wait()

	sort.Slice(results, func(i, j int) bool {
		return results[i].Name < results[j].Name
	})

	return results
}

// WaitUntilReady polls health checks until all pass or context is cancelled.
func (hc *HealthChecker) WaitUntilReady(ctx context.Context, checks map[string]config.HealthCheckConfig) []HealthCheckResult {
	for {
		results := hc.Check(checks)
		allReady := true
		for _, r := range results {
			if !r.Ready {
				allReady = false
				break
			}
		}
		if allReady {
			return results
		}

		select {
		case <-ctx.Done():
			return results
		case <-time.After(2 * time.Second):
			// poll again
		}
	}
}

// CheckHTTP reports whether an HTTP GET to url returns a 2xx/3xx status within
// timeout. Exported so the CLI endpoint table shares one implementation.
func CheckHTTP(url string, timeout time.Duration) bool {
	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode >= 200 && resp.StatusCode < 400
}

// CheckHTTPReachable reports whether url answers an HTTP request at all within
// timeout, regardless of status code. Any response — including 4xx/5xx — proves
// a server is listening and speaking HTTP. This is the liveness question the
// endpoint table asks (is something serving here?), as opposed to CheckHTTP's
// stricter 2xx/3xx health verdict: an API server that 404s on "/" is up, not
// down. Only a transport error (connection refused, timeout) counts as down.
func CheckHTTPReachable(url string, timeout time.Duration) bool {
	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return true
}

// CheckTCP reports whether a TCP connection to address succeeds within timeout.
func CheckTCP(address string, timeout time.Duration) bool {
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func checkCommand(command string, timeout time.Duration) bool {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return exec.CommandContext(ctx, "sh", "-c", command).Run() == nil
}
