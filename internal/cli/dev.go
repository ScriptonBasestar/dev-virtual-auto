package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/ScriptonBasestar/dva/internal/lifecycle"
)

var devCmd = &cobra.Command{
	Use:   "dev [APP...]",
	Short: "Start infrastructure and applications in dev mode",
	Long: `Start infrastructure via lifecycle plugins and then launch applications in dev mode.

DVA-specific flags:
  --mode, -M MODE           Use a named mode from dva.yml modes section
  --env, -E ENV             Use a named environment from dva.yml environments section
  --docker                  Force docker strategy for all apps
  --no-wait                 Skip health check wait after starting apps
  --tag, -T TAG[,TAG]       Include only lifecycle entries matching any of the given tags
  --exclude-tag TAG[,TAG]   Exclude lifecycle entries matching any of the given tags`,
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		c := mustLoadConfig()
		e := loadEnv(c)

		if len(c.Applications) == 0 {
			return fmt.Errorf("no applications defined in dva.yml")
		}

		mode, envName, includeTags, excludeTags, args := parseDvaFlags(args)
		mode, isDefault := applyDefaultMode(c, mode)

		// Parse dev-specific flags
		docker := false
		noWait := false
		var appNames []string
		for _, a := range args {
			switch a {
			case "--docker":
				docker = true
			case "--no-wait":
				noWait = true
			default:
				appNames = append(appNames, a)
			}
		}

		if err := applyEnv(e, c, envName); err != nil {
			return err
		}

		rm, err := resolveMode(c, mode)
		if err != nil {
			return err
		}
		if rm.Mode != nil {
			if isDefault {
				fmt.Fprintf(os.Stderr, "[mode: %s (default)] %s\n", mode, rm.Mode.Description)
			} else {
				fmt.Fprintf(os.Stderr, "[mode: %s] %s\n", mode, rm.Mode.Description)
			}
			if len(rm.Mode.Environment) > 0 {
				e.MergeVars(rm.Mode.Environment)
			}
			if rm.Mode.Provision != "" {
				suggestProvision(c, rm.Mode.Provision)
			}
		}

		// Phase 1: Start infrastructure via orchestrator
		orch := lifecycle.NewOrchestrator(c, e)
		if err := orch.Up(context.Background(), lifecycle.UpOptions{
			DryRun:      dryRun,
			Wait:        !noWait,
			IncludeTags: includeTags,
			ExcludeTags: excludeTags,
			Mode:        mode,
			Env:         envName,
		}); err != nil {
			return fmt.Errorf("infrastructure start: %w", err)
		}

		// Phase 2: Start applications in dev mode
		am := lifecycle.NewAppManager(c, e)
		strategy := ""
		if docker {
			strategy = "docker"
		}

		if err := am.StartApps(context.Background(), lifecycle.AppStartOptions{
			Strategy: strategy,
			Names:    appNames,
			DevMode:  true,
			DryRun:   dryRun,
			Wait:     !noWait,
			Mode:     mode,
		}); err != nil {
			return fmt.Errorf("application start: %w", err)
		}

		// Print summary
		fmt.Fprintln(os.Stderr)
		statuses := am.AppStatuses()
		printAppStatuses(statuses)

		return nil
	},
}
