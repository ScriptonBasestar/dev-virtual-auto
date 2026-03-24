package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/ScriptonBasestar/dva/internal/config"
)

var addCmd = &cobra.Command{
	Use:       "add <feature>",
	Short:     "Add optional features to an existing dva.yml",
	Long:      "Add optional configuration features to an existing dva.yml.\n\nAvailable features: devcontainer",
	Args:      cobra.ExactArgs(1),
	ValidArgs: []string{"devcontainer"},
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "devcontainer":
			return runAddDevcontainer()
		default:
			return fmt.Errorf("unknown feature: %s\nAvailable: devcontainer", args[0])
		}
	},
}

func runAddDevcontainer() error {
	c := mustLoadConfig()

	if len(c.Devcontainer) > 0 {
		fmt.Println("ℹ️  devcontainer section already exists in dva.yml")
		fmt.Println("   Edit dva.yml directly to modify, then run: dva validate")
		return nil
	}

	service := detectPrimaryService(c)
	section := devcontainerYAMLSection(service)

	dvaYmlPath := c.FilePath()
	existing, err := os.ReadFile(dvaYmlPath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", dvaYmlPath, err)
	}

	if err := os.WriteFile(dvaYmlPath, append(existing, []byte(section)...), 0644); err != nil {
		return fmt.Errorf("failed to update %s: %w", dvaYmlPath, err)
	}

	// Use a minimal default map matching what we just appended
	dc := map[string]any{
		"enabled":         true,
		"name":            "Development Environment",
		"service":         service,
		"workspaceFolder": "/workspace",
	}
	if err := writeDevcontainerFiles(dc, c.Compose.Files, c.FileDir()); err != nil {
		return err
	}

	fmt.Println("✅ Added devcontainer support")
	fmt.Println("   dva.yml: devcontainer section added")
	fmt.Println("   .devcontainer/devcontainer.json: created")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  Edit dva.yml devcontainer section to customize")
	fmt.Println("  dva validate — verify configuration")
	return nil
}

// detectPrimaryService returns the first service name found in compose files, or "app".
func detectPrimaryService(c *config.Config) string {
	for _, f := range c.Compose.Files {
		if services := extractComposeServices(f); len(services) > 0 {
			return services[0]
		}
	}
	return "app"
}
