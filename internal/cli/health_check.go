package cli

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// HealthCheckResult holds the outcome of a single health check.
type HealthCheckResult struct {
	Name      string `json:"name"`
	Ready     bool   `json:"ready"`
	StartHint string `json:"start_hint,omitempty"`
}

// runHealthChecks executes all configured health checks and returns results.
func runHealthChecks(checks map[string]config.HealthCheckConfig) []HealthCheckResult {
	if len(checks) == 0 {
		return nil
	}

	results := make([]HealthCheckResult, 0, len(checks))
	for name, hc := range checks {
		result := HealthCheckResult{
			Name:      name,
			StartHint: hc.StartHint,
		}

		timeout := time.Duration(hc.Timeout) * time.Second
		if timeout == 0 {
			timeout = 2 * time.Second
		}

		switch hc.Type {
		case "http":
			result.Ready = checkHTTP(hc.URL, timeout)
		case "tcp":
			result.Ready = checkTCP(hc.Address, timeout)
		case "command":
			result.Ready = checkCommand(hc.Command, timeout)
		}

		results = append(results, result)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Name < results[j].Name
	})

	return results
}

func checkHTTP(url string, timeout time.Duration) bool {
	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode < 500
}

func checkTCP(address string, timeout time.Duration) bool {
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func checkCommand(command string, timeout time.Duration) bool {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return exec.CommandContext(ctx, "sh", "-c", command).Run() == nil
}

// printHealthCheckResults prints health check results as a table.
func printHealthCheckResults(results []HealthCheckResult) {
	if len(results) == 0 {
		return
	}

	fmt.Println("Health Checks:")

	var buf strings.Builder
	tw := tabwriter.NewWriter(&buf, 2, 0, 3, ' ', 0)
	fmt.Fprintf(tw, "  SERVICE\tSTATUS\n")
	for _, r := range results {
		status := "ready"
		if !r.Ready {
			status = "not ready"
		}
		fmt.Fprintf(tw, "  %s\t%s\n", r.Name, status)
	}
	tw.Flush()
	fmt.Print(buf.String())

	// Show start hints for services that are not ready
	hasHints := false
	for _, r := range results {
		if !r.Ready && r.StartHint != "" {
			if !hasHints {
				fmt.Println()
				hasHints = true
			}
			fmt.Printf("  %s -> %s\n", r.Name, r.StartHint)
		}
	}

	fmt.Println()
}
