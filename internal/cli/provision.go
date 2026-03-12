package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

var provisionCmd = &cobra.Command{
	Use:   "provision [PROFILE]",
	Short: "Execute the provisioning steps defined in 'dva.yml'",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := mustLoadConfig()

		profile := "default"
		if len(args) > 0 {
			profile = args[0]
		}

		steps, ok := c.Provision[profile]
		if !ok {
			// Check if provision is legacy array format (no profiles)
			if profile == "default" && len(c.Provision) == 0 {
				return fmt.Errorf("no provision commands defined in dva.yml")
			}
			available := make([]string, 0, len(c.Provision))
			for k := range c.Provision {
				available = append(available, k)
			}
			return fmt.Errorf("provision profile '%s' not found. Available: %s", profile, strings.Join(available, ", "))
		}

		fmt.Printf("🚀 Running provision profile: %s\n\n", profile)

		for i, step := range steps {
			// Display step name
			if step.Step != "" {
				fmt.Printf("  [%d/%d] %s\n", i+1, len(steps), step.Step)
			}

			// Display note
			if step.Note != "" {
				fmt.Println()
				for _, line := range strings.Split(step.Note, "\n") {
					fmt.Printf("    %s\n", line)
				}
				fmt.Println()
			}

			// Execute commands
			cmds := step.RunCommands()
			for _, cmdStr := range cmds {
				fmt.Printf("    $ %s\n", cmdStr)
				if err := runShellCommand(cmdStr); err != nil {
					return fmt.Errorf("provision step '%s' failed: %w", step.Step, err)
				}
			}

			// Legacy format: echo
			if step.Echo != "" {
				fmt.Printf("    %s\n", step.Echo)
			}

			// Legacy format: cmd
			if step.Cmd != "" {
				fmt.Printf("    $ %s\n", step.Cmd)
				if err := runShellCommand(step.Cmd); err != nil {
					return fmt.Errorf("provision command failed: %w", err)
				}
			}
		}

		fmt.Println("\n✅ Provision complete!")
		return nil
	},
}

func runShellCommand(cmdStr string) error {
	c := exec.Command("sh", "-c", cmdStr)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}
