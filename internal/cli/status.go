package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ScriptonBasestar/dva/internal/config"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show project status (config, services, containers)",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("DVA v%s\n\n", config.Version)

		// Config info
		c, err := loadConfig()
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
