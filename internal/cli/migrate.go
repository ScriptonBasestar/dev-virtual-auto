package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Detect legacy config format and generate a migration guide",
	Long:  `Scans for .hip.yml or legacy dva.yml fields and outputs a migration guide to the current format.`,
	RunE:  runMigrate,
}

func runMigrate(_ *cobra.Command, _ []string) error {
	wd, _ := os.Getwd()

	// 1. Check for .hip.yml (Hip CLI format)
	hipPath := filepath.Join(wd, ".hip.yml")
	if _, err := os.Stat(hipPath); err == nil {
		return migrateHip(hipPath)
	}

	// 2. Check current dva.yml for legacy fields
	dvaPath := filepath.Join(wd, "dva.yml")
	if _, err := os.Stat(dvaPath); err == nil {
		return migrateDva(dvaPath)
	}

	fmt.Println("No legacy config found (.hip.yml or dva.yml).")
	fmt.Println("Run 'dva init' to create a new dva.yml.")
	return nil
}

func migrateHip(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}

	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("parsing %s: %w", path, err)
	}

	fmt.Printf("Found Hip CLI config: %s\n\n", path)
	fmt.Println("Migration guide (.hip.yml → dva.yml):")
	fmt.Println("──────────────────────────────────────")

	hints := 0

	if scripts, ok := raw["scripts"]; ok {
		fmt.Println("\n[scripts] → [interaction]")
		fmt.Println("  Rename the top-level 'scripts' key to 'interaction'.")
		printYAMLSection("  Old", "scripts", scripts)
		hints++
	}
	if cmds, ok := raw["commands"]; ok {
		fmt.Println("\n[commands] → [interaction]")
		fmt.Println("  Rename the top-level 'commands' key to 'interaction'.")
		printYAMLSection("  Old", "commands", cmds)
		hints++
	}
	if svcs, ok := raw["services"]; ok {
		fmt.Println("\n[services] → [compose.services] (or docker-compose.yml)")
		fmt.Println("  Service definitions belong in docker-compose.yml, not dva config.")
		fmt.Println("  If you need compose file paths, use:")
		fmt.Println("    compose:")
		fmt.Println("      files: [docker-compose.yml]")
		printYAMLSection("  Old", "services", svcs)
		hints++
	}
	if env, ok := raw["env"]; ok {
		fmt.Println("\n[env] → [environment]")
		fmt.Println("  Rename the top-level 'env' key to 'environment'.")
		printYAMLSection("  Old", "env", env)
		hints++
	}

	if hints == 0 {
		fmt.Println("No known legacy patterns detected.")
		fmt.Println("Consider running 'dva validate' after creating dva.yml.")
	} else {
		fmt.Printf("\n%d migration item(s) found.\n", hints)
		fmt.Println("After updating, run: dva validate")
	}

	return nil
}

func migrateDva(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}

	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("parsing %s: %w", path, err)
	}

	fmt.Printf("Scanning: %s\n\n", path)
	fmt.Println("Legacy field check:")
	fmt.Println("──────────────────────────────────────")

	hints := 0

	legacyKeys := map[string]string{
		"scripts":  "interaction",
		"commands": "interaction",
		"env":      "environment",
		"services": "compose.services (move to docker-compose.yml)",
	}
	for old, newKey := range legacyKeys {
		if _, ok := raw[old]; ok {
			fmt.Printf("\n  '%s' → '%s'\n", old, newKey)
			hints++
		}
	}

	// Check for removed EnvFile field still present
	if _, ok := raw["env_file"]; ok {
		// env_file is actually supported — this is fine
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
		fmt.Println("Run 'dva validate' after making changes.")
	}

	return nil
}

func printYAMLSection(prefix, key string, val any) {
	out, err := yaml.Marshal(map[string]any{key: val})
	if err != nil {
		return
	}
	for _, line := range splitLines(string(out)) {
		fmt.Printf("%s  %s\n", prefix, line)
	}
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i, c := range s {
		if c == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func init() {
	migrateCmd.GroupID = "advanced"
	rootCmd.AddCommand(migrateCmd)
}
