package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/ScriptonBasestar/dva/internal/config"
	dvaexec "github.com/ScriptonBasestar/dva/internal/exec"
)

var composeCmd = &cobra.Command{
	Use:                "compose [ARGS...]",
	Short:              "Run Docker Compose commands",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		c := mustLoadConfig()
		e := loadEnv(c)
		return execComposePassthrough(e, c, args)
	},
}

var upCmd = &cobra.Command{
	Use:                "up [OPTIONS] [SERVICE...]",
	Short:              "Run `docker compose up` (default: -d --wait)",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		c := mustLoadConfig()
		e := loadEnv(c)

		// Check for --foreground / -f flag
		foreground := false
		var filteredArgs []string
		for _, a := range args {
			if a == "--foreground" || a == "-f" {
				foreground = true
			} else {
				filteredArgs = append(filteredArgs, a)
			}
		}

		if !foreground {
			defaults := c.Compose.UpOptions
			if len(defaults) == 0 {
				defaults = []string{"-d", "--wait"}
			}
			// Prepend defaults if not already present
			existing := make(map[string]bool)
			for _, a := range filteredArgs {
				existing[a] = true
			}
			for i := len(defaults) - 1; i >= 0; i-- {
				if !existing[defaults[i]] {
					filteredArgs = append([]string{defaults[i]}, filteredArgs...)
				}
			}
		}

		return execComposePassthrough(e, c, append([]string{"up"}, filteredArgs...))
	},
}

var downCmd = &cobra.Command{
	Use:                "down [OPTIONS]",
	Short:              "Run `docker compose down`",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		c := mustLoadConfig()
		e := loadEnv(c)
		return execComposePassthrough(e, c, append([]string{"down", "--remove-orphans"}, args...))
	},
}

var stopCmd = &cobra.Command{
	Use:                "stop [OPTIONS] [SERVICE...]",
	Short:              "Run `docker compose stop`",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		c := mustLoadConfig()
		e := loadEnv(c)
		return execComposePassthrough(e, c, append([]string{"stop"}, args...))
	},
}

var buildCmd = &cobra.Command{
	Use:                "build [OPTIONS] [SERVICE...]",
	Short:              "Run `docker compose build`",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		c := mustLoadConfig()
		e := loadEnv(c)
		return execComposePassthrough(e, c, append([]string{"build"}, args...))
	},
}

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Remove all containers, networks, and optionally volumes",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := mustLoadConfig()
		e := loadEnv(c)

		cleanArgs := []string{"down", "--remove-orphans"}

		volumes, _ := cmd.Flags().GetBool("volumes")
		images, _ := cmd.Flags().GetBool("images")

		if volumes {
			cleanArgs = append(cleanArgs, "--volumes")
		}
		if images {
			cleanArgs = append(cleanArgs, "--rmi", "local")
		}

		return execComposePassthrough(e, c, cleanArgs)
	},
}

func init() {
	cleanCmd.Flags().BoolP("volumes", "v", false, "Also remove volumes (WARNING: data loss)")
	cleanCmd.Flags().BoolP("images", "i", false, "Also remove images built by docker compose")
	cleanCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")
}

// execComposePassthrough builds and execs a docker compose command using config.
func execComposePassthrough(e *config.Environment, c *config.Config, args []string) error {
	composeCmd := "docker"
	composeArgs := []string{"compose"}

	if c.Compose.Command != "" {
		parts := dvaexec.SplitCommand(c.Compose.Command)
		composeCmd = parts[0]
		if len(parts) > 1 {
			composeArgs = parts[1:]
		}
	}

	// Compose files
	cfgDir := c.FileDir()
	for _, f := range c.Compose.Files {
		f = e.Interpolate(f)
		if !isAbsPath(f) {
			f = cfgDir + "/" + f
		}
		composeArgs = append(composeArgs, "-f", f)
	}

	// Project name
	if c.Compose.ProjectName != "" {
		composeArgs = append(composeArgs, "--project-name", e.Interpolate(c.Compose.ProjectName))
	}

	composeArgs = append(composeArgs, args...)

	if dvaexec.Debug {
		fmt.Fprintf(os.Stderr, "[debug] compose: %s %v\n", composeCmd, composeArgs)
	}

	return dvaexec.ExecReplace(e, composeCmd, composeArgs, false)
}

func isAbsPath(p string) bool {
	return len(p) > 0 && p[0] == '/'
}
