package cli

import (
	"fmt"
	"net"
	"sort"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// checkEndpointHealth probes each endpoint URL/port concurrently and returns
// HealthCheckResults keyed by endpoint name. HTTP endpoints (url starts with
// http) get an HTTP check; others get a TCP check on the resolved port.
func checkEndpointHealth(endpoints map[string]config.EndpointConfig) []HealthCheckResult {
	if len(endpoints) == 0 {
		return nil
	}

	results := make([]HealthCheckResult, 0, len(endpoints))
	var mu sync.Mutex
	var wg sync.WaitGroup

	const timeout = 2 * time.Second

	for name, ep := range endpoints {
		wg.Add(1)
		go func(name string, ep config.EndpointConfig) {
			defer wg.Done()

			result := HealthCheckResult{Name: name}

			url := ep.URL
			if url == "" {
				// No URL resolved — cannot check
				mu.Lock()
				results = append(results, result)
				mu.Unlock()
				return
			}

			if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
				result.Ready = checkHTTP(url, timeout)
			} else {
				// Treat as host:port for TCP check
				host, port, err := net.SplitHostPort(url)
				if err != nil {
					// Try as bare port
					host = "localhost"
					port = url
				}
				result.Ready = checkTCP(net.JoinHostPort(host, port), timeout)
			}

			mu.Lock()
			results = append(results, result)
			mu.Unlock()
		}(name, ep)
	}

	wg.Wait()

	sort.Slice(results, func(i, j int) bool {
		return results[i].Name < results[j].Name
	})

	return results
}

// printEndpointTable prints declared endpoints with optional health check status.
// endpointTags filters which endpoints to show; empty means show all.
// healthResults links endpoint keys to health check outcomes (may be nil).
func printEndpointTable(endpoints map[string]config.EndpointConfig, endpointTags []string, healthResults []HealthCheckResult) {
	if len(endpoints) == 0 {
		return
	}

	// Filter by tags if specified
	filtered := filterEndpoints(endpoints, endpointTags)
	if len(filtered) == 0 {
		return
	}

	// Build health check lookup
	hcMap := make(map[string]HealthCheckResult)
	for _, r := range healthResults {
		hcMap[r.Name] = r
	}

	// Sort endpoint names for stable output
	names := make([]string, 0, len(filtered))
	for k := range filtered {
		names = append(names, k)
	}
	sort.Strings(names)

	fmt.Println("Endpoints:")

	var buf strings.Builder
	tw := tabwriter.NewWriter(&buf, 2, 0, 3, ' ', 0)
	fmt.Fprintf(tw, "  NAME\tSTATUS\tURL\n")

	for _, name := range names {
		ep := filtered[name]
		status := ""
		if r, ok := hcMap[name]; ok {
			if r.Ready {
				status = "🟢"
			} else if r.Started {
				status = "🟡"
			} else {
				status = "🔴"
			}
		}

		fmt.Fprintf(tw, "  %s\t%s\t%s\n", ep.Label, status, ep.URL)

		// Sub-paths: combine with base URL so terminals render clickable links
		if len(ep.Paths) > 0 {
			baseURL := strings.TrimRight(ep.URL, "/")
			paths := make([]string, 0, len(ep.Paths))
			for p := range ep.Paths {
				paths = append(paths, p)
			}
			sort.Strings(paths)
			for _, p := range paths {
				subPath := p
				if !strings.HasPrefix(subPath, "/") {
					subPath = "/" + subPath
				}
				fullURL := baseURL + subPath
				fmt.Fprintf(tw, "  \t\t  %s  %s\n", fullURL, ep.Paths[p])
			}
		}
	}

	tw.Flush()
	fmt.Print(buf.String())
	fmt.Println()
}

// filterEndpoints returns endpoints matching any of the given tags.
// If tags is empty, all endpoints are returned.
func filterEndpoints(endpoints map[string]config.EndpointConfig, tags []string) map[string]config.EndpointConfig {
	if len(tags) == 0 {
		return endpoints
	}

	tagSet := make(map[string]bool, len(tags))
	for _, t := range tags {
		tagSet[t] = true
	}

	result := make(map[string]config.EndpointConfig)
	for k, ep := range endpoints {
		for _, t := range ep.Tags {
			if tagSet[t] {
				result[k] = ep
				break
			}
		}
	}
	return result
}
