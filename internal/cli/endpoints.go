package cli

import (
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/ScriptonBasestar/dva/internal/config"
)

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
	fmt.Fprintf(tw, "  NAME\tURL\tSTATUS\n")

	for _, name := range names {
		ep := filtered[name]
		status := ""
		if r, ok := hcMap[name]; ok {
			if r.Ready {
				status = "ready"
			} else if r.Started {
				status = "starting"
			} else {
				status = "not ready"
			}
		}

		fmt.Fprintf(tw, "  %s\t%s\t%s\n", ep.Label, ep.URL, status)

		// Sub-paths
		if len(ep.Paths) > 0 {
			paths := make([]string, 0, len(ep.Paths))
			for p := range ep.Paths {
				paths = append(paths, p)
			}
			sort.Strings(paths)
			for _, p := range paths {
				fmt.Fprintf(tw, "  \t  %s\t%s\n", p, ep.Paths[p])
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
