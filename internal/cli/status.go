package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ScriptonBasestar/dva/internal/config"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Display workspace status (config, active services, containers)",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Config info
		c, err := loadConfig()

		if jsonOutput {
			statusData := map[string]interface{}{
				"dva_version":  config.Version,
				"config_found": err == nil,
			}
			if err == nil {
				statusData["config_path"] = c.FilePath()
				statusData["config_version"] = c.Version
				statusData["project_name"] = c.Compose.ProjectName
				statusData["compose_files"] = c.Compose.Files
				statusData["commands_count"] = len(c.Interaction)

				composeCmd := "docker"
				composeArgs := []string{"compose"}
				for _, f := range c.Compose.Files {
					composeArgs = append(composeArgs, "-f", f)
				}
				if c.Compose.ProjectName != "" {
					composeArgs = append(composeArgs, "-p", c.Compose.ProjectName)
				}
				composeArgs = append(composeArgs, "ps", "--format", "json")
				if out, execErr := exec.Command(composeCmd, composeArgs...).Output(); execErr == nil {
					var psData interface{}
					// Try parsing as array
					if jsonErr := json.Unmarshal(out, &psData); jsonErr == nil {
						statusData["services"] = psData
					} else {
						// Or JSON lines
						lines := strings.Split(strings.TrimSpace(string(out)), "\n")
						var services []interface{}
						for _, line := range lines {
							if line == "" {
								continue
							}
							var s interface{}
							if jsonErr := json.Unmarshal([]byte(line), &s); jsonErr == nil {
								services = append(services, s)
							}
						}
						statusData["services"] = services
					}
				} else {
					statusData["services"] = nil
				}
			}
			data, _ := json.MarshalIndent(statusData, "", "  ")
			fmt.Println(string(data))
			return nil
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

		// Count interaction commands
		cmdCount := len(c.Interaction)
		if cmdCount > 0 {
			fmt.Printf("   Commands: %d defined\n", cmdCount)
		}

		fmt.Println()

		// Docker Compose status
		fmt.Println("🐳 Services:")
		composeCmd := "docker"
		composeArgs := []string{"compose"}

		for _, f := range c.Compose.Files {
			composeArgs = append(composeArgs, "-f", f)
		}
		if c.Compose.ProjectName != "" {
			composeArgs = append(composeArgs, "-p", c.Compose.ProjectName)
		}
		composeArgs = append(composeArgs, "ps", "--format", "table")

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
