package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var migrateFix bool

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Detect legacy config format and generate a migration guide",
	Long:  `Scans for legacy dva.yml fields and outputs a migration guide. Use --fix to automatically migrate the file.`,
	RunE:  runMigrate,
}

func runMigrate(_ *cobra.Command, _ []string) error {
	wd, _ := os.Getwd()

	// Check current dva.yml for legacy fields
	dvaPath := filepath.Join(wd, "dva.yml")
	if _, err := os.Stat(dvaPath); err == nil {
		return migrateDva(dvaPath, migrateFix)
	}

	fmt.Println("No legacy config found (dva.yml).")
	fmt.Println("Run 'dva init' to create a new dva.yml.")
	return nil
}

func migrateDva(path string, doFix bool) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}

	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("parsing %s: %w", path, err)
	}

	hints := 0
	legacyKeys := map[string]string{
		"scripts":  "interaction",
		"commands": "interaction",
		"env":      "environment",
		"services": "compose.services (move to docker-compose.yml)",
	}

	toFix := map[string]string{}

	if !doFix {
		fmt.Printf("Scanning: %s\n\n", path)
		fmt.Println("Legacy field check:")
		fmt.Println("──────────────────────────────────────")

		for old, newKey := range legacyKeys {
			if _, ok := raw[old]; ok {
				fmt.Printf("\n  '%s' → '%s'\n", old, newKey)
				hints++
			}
		}

		// Check for old-style interaction without description
		if interaction, ok := raw["interaction"].(map[string]any); ok {
			for name, v := range interaction {
				if cmd, ok := v.(map[string]any); ok {
					if _, hasDesc := cmd["description"]; !hasDesc {
						fmt.Printf("\n  interaction.%s: missing 'description' field (recommended)\n", name)
						hints++
					}
				}
			}
		}

		if hints == 0 {
			fmt.Println("  No legacy patterns detected. Config looks up-to-date.")
			fmt.Println("\nTip: Run 'dva validate' to check schema compliance.")
		} else {
			fmt.Printf("\n%d item(s) to review.\n", hints)
			fmt.Println("Run 'dva migrate --fix' to automatically apply naming changes, or edit manually.")
		}
		return nil
	}

	// Do Fix
	for old, newKey := range map[string]string{
		"scripts":  "interaction",
		"commands": "interaction",
		"env":      "environment",
	} {
		if _, ok := raw[old]; ok {
			toFix[old] = newKey
		}
	}

	if len(toFix) == 0 {
		fmt.Println("Nothing to fix. Config looks up-to-date.")
		return nil
	}

	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("yaml node parsing failed: %w", err)
	}

	modified := false
	if len(root.Content) > 0 && root.Content[0].Kind == yaml.MappingNode {
		mapping := root.Content[0]
		for i := 0; i < len(mapping.Content); i += 2 {
			keyNode := mapping.Content[i]
			if newName, exists := toFix[keyNode.Value]; exists {
				fmt.Printf("Migrating key: '%s' → '%s'\n", keyNode.Value, newName)
				keyNode.Value = newName
				modified = true
			}
		}
	}

	if modified {
		out, err := yaml.Marshal(&root)
		if err != nil {
			return fmt.Errorf("yaml marshal failed: %w", err)
		}
		if err := os.WriteFile(path, out, 0644); err != nil {
			return fmt.Errorf("failed writing file: %w", err)
		}
		fmt.Println("Successfully auto-fixed dva.yml!")
		fmt.Println("Note: For 'services' migration, you must manually move them to docker-compose.yml.")
	}

	return nil
}

func init() {
	migrateCmd.Flags().BoolVar(&migrateFix, "fix", false, "Auto-fix legacy schema keys in place")
	migrateCmd.GroupID = "advanced"
	rootCmd.AddCommand(migrateCmd)
}
