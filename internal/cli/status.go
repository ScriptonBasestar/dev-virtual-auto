package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ScriptonBasestar/dva/internal/config"
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

		fmt.Println()

		fmt.Println("🐳 Services:")
		e := loadEnv(c)
		composeCmd, composeArgs := buildComposeArgs(e, c, []string{"ps", "--format", "table"})
		ps := exec.Command(composeCmd, composeArgs...)
		ps.Stdout = os.Stdout
		ps.Stderr = os.Stderr
		if err := ps.Run(); err != nil {
			fmt.Println("   (no containers running or docker not available)")
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
