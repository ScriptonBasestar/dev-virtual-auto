package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ScriptonBasestar/dva/internal/config"
	"github.com/ScriptonBasestar/dva/internal/lifecycle"
	"github.com/ScriptonBasestar/dva/internal/output"
)

var statusCmd = &cobra.Command{
	Use:   "status [NAME]",
	Short: "Display workspace status (config, lifecycle entries, services)",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := loadConfig()
		if err == nil {
			e := loadEnv(c)
				if planName, _, ok := detectPlanRoute(c, args); ok {
					return runPlanStatus(c, e, planName)
				}
				if err := rejectSuppressedDefaultPlan(c, "status", args); err != nil {
					return err
				}
				if err := rejectUnknownPlanArg(c, args); err != nil {
					return err
				}
		}

		if jsonOutput {
			statusData := map[string]any{
				"dva_version":  config.Version,
				"config_found": err == nil,
			}
			var stackStatus *lifecycle.AggregatedStatus
			if err == nil {
				statusData["config_path"] = c.FilePath()
				statusData["config_version"] = c.Version
				statusData["commands_count"] = len(c.Interaction)
				statusData["stack_count"] = len(c.Stack)

				e := loadEnv(c)
				orch := lifecycle.NewOrchestrator(c, e)
				status, statusErr := orch.Status(context.Background())
				if statusErr == nil {
					stackStatus = status
					statusData["stack"] = status.Entries
				}
				if len(c.Applications) > 0 {
					am := lifecycle.NewAppManager(c, e)
					statusData["applications"] = am.AppStatuses()
				}
			}
			if printErr := output.PrintJSON(statusData); printErr != nil {
				return printErr
			}
			return lifecycle.StatusExitError(stackStatus)
		}

		fmt.Printf("DVA v%s\n\n", config.Version)

		if err != nil {
			fmt.Println("Config: not found")
			fmt.Println("   Run 'dva init' to create a dva.yml")
			return nil
		}

		fmt.Printf("Config: %s\n", c.FilePath())
		if c.Version != "" {
			fmt.Printf("   Version: %s\n", c.Version)
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
				fmt.Printf("     - %s -> %s%s\n", name, sub.Path, tags)
			}
		}

		// Lifecycle status via orchestrator
		e := loadEnv(c)
		orch := lifecycle.NewOrchestrator(c, e)
		status, statusErr := orch.Status(context.Background())
		if statusErr != nil {
			fmt.Println("\nLifecycle: (error querying status)")
		} else {
			lifecycle.PrintStatus(status, c.FileDir())
		}

		// Applications status
		if len(c.Applications) > 0 {
			am := lifecycle.NewAppManager(c, e)
			statuses := am.AppStatuses()
			fmt.Println()
			printAppStatuses(statuses)
		}

		if len(c.Endpoints) > 0 {
			allHC := checkEndpointHealth(c.Endpoints)
			printEndpointTable(c.Endpoints, nil, allHC)
		}

		if statusErr == nil {
			return lifecycle.StatusExitError(status)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
