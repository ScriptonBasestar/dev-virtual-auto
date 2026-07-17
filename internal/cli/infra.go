package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ScriptonBasestar/dva/internal/config"
	"github.com/spf13/cobra"
)

var infraCmd = &cobra.Command{
	Use:   "infra",
	Short: "Manage shared background infrastructure services",
}

var infraUpCmd = &cobra.Command{
	Use:                "up [SERVICE] [OPTIONS]",
	Short:              "Start infrastructure service",
	Args:               cobra.MinimumNArgs(1),
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if helpRequested(args) {
			return cmd.Help()
		}
		c := mustLoadConfig()
		serviceName := args[0]
		extraArgs := args[1:]

		svc, ok := c.Infra[serviceName]
		if !ok {
			return fmt.Errorf("infra service '%s' not found", serviceName)
		}

		location, err := infraServiceLocation(svc, serviceName, c.FileDir())
		if err != nil {
			return err
		}

		// Create network (ignore errors)
		networkName := serviceName + "_default"
		_ = execInfraCmd("docker", "network", "create", networkName)

		// docker compose up in service dir
		composeArgs := append([]string{"compose", "up", "--detach"}, extraArgs...)
		return runInDir(location, "docker", composeArgs...)
	},
}

var infraDownCmd = &cobra.Command{
	Use:                "down [SERVICE] [OPTIONS]",
	Short:              "Stop infrastructure service",
	Args:               cobra.MinimumNArgs(1),
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if helpRequested(args) {
			return cmd.Help()
		}
		c := mustLoadConfig()
		serviceName := args[0]
		extraArgs := args[1:]

		svc, ok := c.Infra[serviceName]
		if !ok {
			return fmt.Errorf("infra service '%s' not found", serviceName)
		}

		location, err := infraServiceLocation(svc, serviceName, c.FileDir())
		if err != nil {
			return err
		}
		composeArgs := append([]string{"compose", "down"}, extraArgs...)
		return runInDir(location, "docker", composeArgs...)
	},
}

var infraUpdateCmd = &cobra.Command{
	Use:   "update [SERVICE]",
	Short: "Update infrastructure service (git pull or clone)",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c := mustLoadConfig()
		serviceName := args[0]

		svc, ok := c.Infra[serviceName]
		if !ok {
			return fmt.Errorf("infra service '%s' not found", serviceName)
		}

		if svc.Git == "" {
			return fmt.Errorf("infra service '%s' has no git URL", serviceName)
		}

		location, err := infraServiceLocation(svc, serviceName, c.FileDir())
		if err != nil {
			return err
		}

		if _, err := os.Stat(location); err == nil {
			// Check for uncommitted changes before updating
			statusCmd := exec.Command("git", "status", "--porcelain")
			statusCmd.Dir = location
			statusOut, _ := statusCmd.Output()
			if len(statusOut) > 0 {
				fmt.Fprintf(os.Stderr, "[warn] %s has uncommitted changes in %s:\n%s\n", serviceName, location, string(statusOut))
				fmt.Fprintf(os.Stderr, "Stash changes before updating? [y/N] ")
				var answer string
				_, _ = fmt.Scanln(&answer)
				answer = strings.ToLower(strings.TrimSpace(answer))
				if answer != "y" && answer != "yes" {
					return fmt.Errorf("aborted: %s has uncommitted changes in %s. Use 'git stash' manually or commit changes first", serviceName, location)
				}
				if err := runInDir(location, "git", "stash"); err != nil {
					return fmt.Errorf("git stash failed: %w", err)
				}
				fmt.Printf("  Changes stashed. Run 'git stash pop' in %s to restore.\n", location)
			}

			fmt.Printf("Updating %s...\n", serviceName)
			return runInDir(location, "git", "pull", "--rebase")
		}

		fmt.Printf("Cloning %s...\n", serviceName)
		if err := os.MkdirAll(filepath.Dir(location), 0755); err != nil {
			return fmt.Errorf("creating directory for %s: %w", serviceName, err)
		}
		cloneArgs := []string{"clone", "--single-branch", "--depth", "1"}
		if svc.Ref != "" {
			cloneArgs = append(cloneArgs, "--branch", svc.Ref)
		}
		cloneArgs = append(cloneArgs, svc.Git, location)
		return execInfraCmd("git", cloneArgs...)
	},
}

func init() {
	infraCmd.AddCommand(infraUpCmd, infraDownCmd, infraUpdateCmd)
}

// infraServiceLocation: git-only → .sb/dva/infra/<name>/; path must not resolve to cfgDir.
func infraServiceLocation(svc config.InfraConfig, serviceName, cfgDir string) (string, error) {
	path := strings.TrimSpace(svc.Path)
	if svc.Git != "" && path == "" {
		return filepath.Join(cfgDir, config.DotDirName, "infra", serviceName), nil
	}
	if path == "" {
		return "", fmt.Errorf("infra service %q has neither git nor path", serviceName)
	}

	location := resolveInfraPath(path, cfgDir)
	if sameInfraDir(location, cfgDir) {
		return "", fmt.Errorf(
			"infra service %q path resolves to the project directory (%s); refuse to operate on the config directory",
			serviceName, location,
		)
	}
	return location, nil
}

func resolveInfraPath(path, cfgDir string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(cfgDir, path)
}

func sameInfraDir(a, b string) bool {
	return filepath.Clean(a) == filepath.Clean(b)
}

func runInDir(dir, name string, args ...string) error {
	c := exec.Command(name, args...)
	c.Dir = dir
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

func execInfraCmd(name string, args ...string) error {
	c := exec.Command(name, args...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}
