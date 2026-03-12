package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/ScriptonBasestar/dva/internal/config"
	dvaexec "github.com/ScriptonBasestar/dva/internal/exec"
)

var (
	debug bool
	cfg   *config.Config
	env   *config.Environment
)

// topLevelCommands lists all built-in command names.
var topLevelCommands = map[string]bool{
	"help": true, "version": true, "ls": true, "compose": true,
	"up": true, "stop": true, "down": true, "build": true, "clean": true,
	"run": true, "provision": true, "validate": true, "manifest": true,
	"ktl": true, "ssh": true, "infra": true, "console": true, "migrate": true,
	"completion": true, "cmd": true,
}

var rootCmd = &cobra.Command{
	Use:   "dva",
	Short: "DVA - Docker Virtual Auto CLI wrapper",
	Long:  "DVA (Docker Virtual Auto) wraps Docker Compose and Kubernetes commands with simple shortcuts defined in dva.yml.",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if debug {
			os.Setenv("DVA_DEBUG", "1")
			dvaexec.Debug = true
			fmt.Fprintln(os.Stderr, "[debug] debug mode enabled")
		}
	},
	// When unknown command is invoked, treat as dynamic "run" command
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "Enable debug logging")

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(lsCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(composeCmd)
	rootCmd.AddCommand(upCmd)
	rootCmd.AddCommand(downCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(buildCmd)
	rootCmd.AddCommand(cleanCmd)
	rootCmd.AddCommand(provisionCmd)
	rootCmd.AddCommand(validateCmd)
	rootCmd.AddCommand(manifestCmd)
	rootCmd.AddCommand(ktlCmd)
	rootCmd.AddCommand(sshCmd)
	rootCmd.AddCommand(infraCmd)
	rootCmd.AddCommand(consoleCmd)
	rootCmd.AddCommand(migrateCmd)
}

// Execute is the main entry point for the CLI.
func Execute() {
	args := os.Args[1:]

	// Dynamic routing: if first arg is not a top-level command,
	// check if it's an interaction command and prepend "run"
	if len(args) > 0 {
		firstArg := args[0]
		if !topLevelCommands[firstArg] && !isFlag(firstArg) {
			c, err := loadConfig()
			if err == nil && c.Interaction[firstArg] != nil {
				// Separate flags and non-flags
				var flags, nonFlags []string
				for _, a := range args {
					if isFlag(a) {
						flags = append(flags, a)
					} else {
						nonFlags = append(nonFlags, a)
					}
				}
				args = append([]string{"run"}, append(flags, nonFlags...)...)
				os.Args = append([]string{os.Args[0]}, args...)
			}
		}
	}

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "\nERROR: %s\n", err)
		os.Exit(1)
	}
}

func isFlag(s string) bool {
	return len(s) > 0 && s[0] == '-'
}

func loadConfig() (*config.Config, error) {
	if cfg != nil {
		return cfg, nil
	}
	var err error
	cfg, err = config.Load(".")
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

func mustLoadConfig() *config.Config {
	c, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "\nERROR: %s\n", err)
		os.Exit(1)
	}
	return c
}

func loadEnv(c *config.Config) *config.Environment {
	if env != nil {
		return env
	}
	wd, _ := os.Getwd()
	env = config.NewEnvironment(c.Environment, wd, c.FileDir())
	return env
}
