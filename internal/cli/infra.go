package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

var infraCmd = &cobra.Command{
	Use:   "infra",
	Short: "Manage infrastructure services",
}

var infraUpCmd = &cobra.Command{
	Use:                "up [SERVICE] [OPTIONS]",
	Short:              "Start infrastructure service",
	Args:               cobra.MinimumNArgs(1),
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		c := mustLoadConfig()
		serviceName := args[0]
		extraArgs := args[1:]

		svc, ok := c.Infra[serviceName]
		if !ok {
			return fmt.Errorf("infra service '%s' not found", serviceName)
		}

		location := resolveInfraPath(svc.Path, c.FileDir())

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
		c := mustLoadConfig()
		serviceName := args[0]
		extraArgs := args[1:]

		svc, ok := c.Infra[serviceName]
		if !ok {
			return fmt.Errorf("infra service '%s' not found", serviceName)
		}

		location := resolveInfraPath(svc.Path, c.FileDir())
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

		location := resolveInfraPath(svc.Path, c.FileDir())

		if _, err := os.Stat(location); err == nil {
			// Pull
			fmt.Printf("Updating %s...\n", serviceName)
			if err := runInDir(location, "git", "checkout", "."); err != nil {
				return err
			}
			return runInDir(location, "git", "pull", "--rebase")
		}

		// Clone
		fmt.Printf("Cloning %s...\n", serviceName)
		os.MkdirAll(filepath.Dir(location), 0755)
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

func resolveInfraPath(path, cfgDir string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(cfgDir, path)
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
