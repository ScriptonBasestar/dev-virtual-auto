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
	Short: "Display current workspace and runtime status",
	Long: `Show DVA's version and config summary, then query each stack entry's live runtime
state (container/process status) through the lifecycle orchestrator, plus endpoint health
checks when endpoints: are declared.

With no NAME it queries the effective default plan when one can be selected, or the whole
declared stack otherwise; with NAME it scopes the query to that named plan's entries only.
It requires a complete environment (vars/environment/env_file) — an incomplete one is
reported and the command exits non-zero without querying runtime state.

See USAGE.md's "named execution entry" section for status usage alongside up/down/stop.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := loadConfig()
		var rootLoad *envLoad
		if err == nil {
			el := rootEnvLoad(c)
			if planName, _, ok := detectPlanRoute(c, args); ok {
				return runPlanStatus(c, el, planName)
			}
			// Whole-stack route: root-owned, so the root report governs. Observation does
			// not fail before printing — it prints a document explicitly marked partial and
			// then exits 1, so a reader is never left to guess whether an empty stack list
			// means "nothing declared" or "nothing asked".
			rootLoad = el
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

				if rootLoad.report.Incomplete() {
					// Config metadata stays; the runtime-derived "stack" key is omitted
					// rather than emitted empty. One document, exit 1.
					statusData["target"] = "stack"
					statusData["environment"] = envPartialJSON(rootLoad.report)
					statusData["runtime"] = envNotQueriedJSON()
					statusData["error"] = envErrorJSON()
					if printErr := output.PrintJSON(statusData); printErr != nil {
						return printErr
					}
					return envIncompleteError(rootLoad.report)
				}

				orch := lifecycle.NewOrchestrator(c, rootLoad.env)
				status, statusErr := orch.Status(context.Background())
				if statusErr == nil {
					stackStatus = status
					statusData["stack"] = status.Entries
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
		if rootLoad.report.Incomplete() {
			// The endpoint table is omitted too: it would start HTTP health checks, and
			// observation on incomplete inputs starts no child and probes no network.
			fmt.Println("\nLifecycle: (not queried: environment inputs incomplete)")
			return envIncompleteError(rootLoad.report)
		}
		orch := lifecycle.NewOrchestrator(c, rootLoad.env)
		status, statusErr := orch.Status(context.Background())
		if statusErr != nil {
			fmt.Println("\nLifecycle: (error querying status)")
		} else {
			lifecycle.PrintStatus(status, c.FileDir())
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
