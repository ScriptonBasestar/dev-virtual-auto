package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ScriptonBasestar/dva/internal/config"
	"github.com/ScriptonBasestar/dva/internal/lifecycle"
	"github.com/ScriptonBasestar/dva/internal/output"
)

// parseComposePS parses docker compose ps JSON output (handles both array and JSON lines).
func parseComposePS(out []byte) any {
	var psData any
	if err := json.Unmarshal(out, &psData); err == nil {
		return psData
	}
	// Fallback: JSON lines format
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var services []any
	for _, line := range lines {
		if line == "" {
			continue
		}
		var s any
		if err := json.Unmarshal([]byte(line), &s); err == nil {
			services = append(services, s)
		}
	}
	return services
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Display workspace status (config, active services, containers)",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := loadConfig()

		if jsonOutput {
			statusData := map[string]any{
				"dva_version":  config.Version,
				"config_found": err == nil,
			}
			if err == nil {
				statusData["config_path"] = c.FilePath()
				statusData["config_version"] = c.Version
				statusData["project_name"] = c.Compose.ProjectName
				statusData["compose_files"] = c.Compose.Files
				statusData["commands_count"] = len(c.Interaction)

				e := loadEnv(c)
				composeCmd, composeArgs := buildComposeArgs(e, c, []string{"ps", "--format", "json"})
				if out, execErr := exec.Command(composeCmd, composeArgs...).Output(); execErr == nil {
					statusData["services"] = parseComposePS(out)
				} else {
					statusData["services"] = nil
				}

				if len(c.HealthChecks) > 0 {
					statusData["health_checks"] = runHealthChecks(c.HealthChecks)
				}
			}
			return output.PrintJSON(statusData)
		}

		fmt.Printf("DVA v%s\n\n", config.Version)

		if err != nil {
			fmt.Println("📄 Config: not found")
			fmt.Println("   Run 'dva init' to create a dva.yml")
			return nil
		}

		fmt.Printf("📄 Config: %s\n", c.FilePath())
		if c.Version != "" {
			fmt.Printf("   Version: %s\n", c.Version)
		}
		if c.Compose.ProjectName != "" {
			fmt.Printf("   Project: %s\n", c.Compose.ProjectName)
		}
		if len(c.Compose.Files) > 0 {
			fmt.Printf("   Compose files: %s\n", strings.Join(c.Compose.Files, ", "))
		}

		cmdCount := len(c.Interaction)
		if cmdCount > 0 {
			fmt.Printf("   Commands: %d defined\n", cmdCount)
		}

		if len(c.Subprojects) > 0 {
			fmt.Printf("   Subprojects: %d\n", len(c.Subprojects))
			for name, sub := range c.Subprojects {
				tags := ""
				if len(sub.ExcludeTags) > 0 {
					tags = fmt.Sprintf(" (exclude: %s)", strings.Join(sub.ExcludeTags, ", "))
				}
				fmt.Printf("     ▸ %s → %s%s\n", name, sub.Path, tags)
			}
		}

		e := loadEnv(c)

		// Use orchestrator status when lifecycle entries are configured
		if useOrchestrator(c) {
			orch := lifecycle.NewOrchestrator(c, e)
			status, err := orch.Status(context.Background())
			if err != nil {
				fmt.Fprintf(os.Stderr, "[warn] could not query lifecycle status: %v\n", err)
			} else {
				lifecycle.PrintStatus(status, c.FileDir())
			}

			if len(c.Endpoints) > 0 {
				// Collect health check results from all entries for endpoint display
				var allHC []HealthCheckResult
				for _, entry := range status.Entries {
					for _, h := range entry.Health {
						allHC = append(allHC, HealthCheckResult{
							Name:  h.Name,
							Ready: h.Ready,
						})
					}
				}
				printEndpointTable(c.Endpoints, nil, allHC)
			}

			return nil
		}

		// Legacy compose path
		fmt.Println("\nServices:")
		services, svcErr := queryComposeServices(e, c)
		if svcErr != nil || len(services) == 0 {
			fmt.Println("   (no containers running or docker not available)")
		} else {
			printServiceTable(services, c.Compose.ProjectName, false, c.Compose.Services)
		}

		var hcResults []HealthCheckResult
		if len(c.HealthChecks) > 0 {
			fmt.Println()
			hcResults = runHealthChecks(c.HealthChecks)
			printHealthCheckResults(hcResults, c.FileDir())
		}

		if len(c.Endpoints) > 0 {
			printEndpointTable(c.Endpoints, nil, hcResults)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
