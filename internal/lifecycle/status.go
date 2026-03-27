package lifecycle

import (
	"fmt"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// AggregatedStatus holds the combined status from all lifecycle entries.
type AggregatedStatus struct {
	Entries []EntryStatus
}

// EntryStatus holds the status for a single lifecycle entry.
type EntryStatus struct {
	Name     string
	Plugin   string
	Services []ServiceStatus
	Health   []HealthCheckResult
}

// PrintStatus prints the aggregated lifecycle status to stderr.
func PrintStatus(status *AggregatedStatus, configDir string) {
	if len(status.Entries) == 0 {
		fmt.Println("Lifecycle: (no entries configured)")
		return
	}

	fmt.Println("Lifecycle:")

	for _, entry := range status.Entries {
		fmt.Printf("\n  [%s] %s\n", entry.Name, entry.Plugin)

		if len(entry.Services) > 0 {
			var buf strings.Builder
			tw := tabwriter.NewWriter(&buf, 2, 0, 3, ' ', 0)
			fmt.Fprintf(tw, "  SERVICE\tSTATE\tHEALTH\n")
			for _, s := range entry.Services {
				health := s.Health
				if health == "" {
					health = "-"
				}
				fmt.Fprintf(tw, "  %s\t%s\t%s\n", s.Name, s.State, health)
			}
			tw.Flush()
			fmt.Print(buf.String())
		}

		if len(entry.Health) > 0 {
			printHealthCheckResults(entry.Health, configDir)
		}
	}

	fmt.Println()
}

// printHealthCheckResults prints health check results as a table.
func printHealthCheckResults(results []HealthCheckResult, configDir string) {
	if len(results) == 0 {
		return
	}

	var buf strings.Builder
	tw := tabwriter.NewWriter(&buf, 2, 0, 3, ' ', 0)
	fmt.Fprintf(tw, "  SERVICE\tSTATUS\n")
	for _, r := range results {
		var status string
		switch {
		case r.Ready:
			status = "ready"
		case r.Started:
			status = "starting"
		default:
			status = "not ready"
		}
		fmt.Fprintf(tw, "  %s\t%s\n", r.Name, status)
	}
	tw.Flush()
	fmt.Print(buf.String())

	// Show hints for services that are not ready
	hasHints := false
	for _, r := range results {
		if r.Ready {
			continue
		}
		if !hasHints {
			fmt.Println()
			hasHints = true
		}
		if r.Started {
			fmt.Printf("  %s -> log: %s\n", r.Name, filepath.Join(configDir, config.DotDirName, "logs", r.Name+".log"))
		} else if r.StartHint != "" {
			fmt.Printf("  %s -> %s\n", r.Name, r.StartHint)
		}
	}
}
