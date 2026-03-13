package cli

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ScriptonBasestar/dva/internal/config"
	dvaexec "github.com/ScriptonBasestar/dva/internal/exec"
	"github.com/ScriptonBasestar/dva/internal/logger"
)

var (
	debug      bool
	dryRun     bool
	jsonOutput bool
	cfg        *config.Config
	env        *config.Environment
)

// topLevelCommands lists all built-in command names.
var topLevelCommands = map[string]bool{
	"help": true, "version": true, "ls": true, "compose": true,
	"up": true, "stop": true, "down": true, "build": true, "clean": true,
	"run": true, "provision": true, "validate": true, "manifest": true,
	"ktl": true, "ssh": true, "infra": true, "console": true,
	"completion": true, "cmd": true, "init": true, "status": true, "config": true,
}

var rootCmd = &cobra.Command{
	Use:   "dva",
	Short: "DVA: Developer Workspace Automator",
	Long: `DVA (Docker Virtual Auto) is a comprehensive developer workspace automation tool.
It simplifies complex workflows involving Docker Compose and Kubernetes by providing 
intuitive shortcuts and uniform environments defined in 'dva.yml'.

DVA ensures robust and reproducible development environments, 
making it easy to onboard and manage projects.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		logger.Init(debug, jsonOutput)
		if debug {
			os.Setenv("DVA_DEBUG", "1")
			dvaexec.Debug = true
			slog.Debug("debug mode enabled", "json", jsonOutput)
		}
	},
	// When unknown command is invoked, treat as dynamic "run" command
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "Enable debug logging")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "Show execution plan without running")
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output in JSON format (LLM-optimized)")

	coreGroup := &cobra.Group{ID: "core", Title: "Core Commands"}
	projectGroup := &cobra.Group{ID: "project", Title: "Project Management"}
	lifecycleGroup := &cobra.Group{ID: "lifecycle", Title: "Lifecycle (Docker Compose)"}
	integrationGroup := &cobra.Group{ID: "integration", Title: "Integration Tools"}
	advancedGroup := &cobra.Group{ID: "advanced", Title: "Advanced Utilities"}

	rootCmd.AddGroup(coreGroup, projectGroup, lifecycleGroup, integrationGroup, advancedGroup)

	initCmd.GroupID = "core"
	runCmd.GroupID = "core"
	lsCmd.GroupID = "core"
	versionCmd.GroupID = "core"

	statusCmd.GroupID = "project"
	configCmd.GroupID = "project"

	upCmd.GroupID = "lifecycle"
	downCmd.GroupID = "lifecycle"
	stopCmd.GroupID = "lifecycle"
	buildCmd.GroupID = "lifecycle"
	cleanCmd.GroupID = "lifecycle"

	composeCmd.GroupID = "integration"
	ktlCmd.GroupID = "integration"
	infraCmd.GroupID = "integration"
	sshCmd.GroupID = "integration"

	manifestCmd.GroupID = "advanced"
	consoleCmd.GroupID = "advanced"
	provisionCmd.GroupID = "advanced"
	validateCmd.GroupID = "advanced"

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
	rootCmd.AddCommand(initCmd)

	cobra.AddTemplateFunc("colorTitle", func(s string) string {
		if !jsonOutput && isTerminal(os.Stdout) && os.Getenv("NO_COLOR") == "" {
			return "\033[1;36m" + s + "\033[0m"
		}
		return s
	})
	rootCmd.SetUsageTemplate(dvaUsageTemplate)
}

// Execute is the main entry point for the CLI.
func Execute() {
	args := os.Args[1:]

	// Dynamic routing: if first arg is not a top-level command,
	// check if it's an interaction command or namespace:command and prepend "run"
	if len(args) > 0 {
		firstArg := args[0]
		if !topLevelCommands[firstArg] && !isFlag(firstArg) {
			shouldRoute := false
			c, err := loadConfig()
			if err == nil {
				// Check direct interaction command
				if c.Interaction[firstArg] != nil {
					shouldRoute = true
				}
				// Check namespace:command syntax (e.g., "engine:test")
				if !shouldRoute && strings.Contains(firstArg, ":") {
					parts := strings.SplitN(firstArg, ":", 2)
					if _, ok := c.Subprojects[parts[0]]; ok {
						shouldRoute = true
					}
				}
			}
			if shouldRoute {
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
		errMsg := err.Error()
		fmt.Fprintf(os.Stderr, "\nERROR: %s\n", errMsg)

		// Suggest similar commands if unknown command
		if len(args) > 0 && !isFlag(args[0]) {
			if suggestions := suggestCommands(args[0]); len(suggestions) > 0 {
				fmt.Fprintf(os.Stderr, "\nDid you mean?\n")
				for _, s := range suggestions {
					fmt.Fprintf(os.Stderr, "  dva %s\n", s)
				}
			}

			// Hint for dva init if no config found
			if strings.Contains(errMsg, "could not find dva.yml") {
				fmt.Fprintf(os.Stderr, "\nHint: run 'dva init' to create a dva.yml\n")
			}
		}
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

// suggestCommands returns commands similar to the input using Levenshtein distance.
func suggestCommands(input string) []string {
	var suggestions []string
	for cmd := range topLevelCommands {
		if levenshtein(input, cmd) <= 2 {
			suggestions = append(suggestions, cmd)
		}
	}
	return suggestions
}

// levenshtein calculates the edit distance between two strings.
func levenshtein(a, b string) int {
	la, lb := len(a), len(b)
	d := make([][]int, la+1)
	for i := range d {
		d[i] = make([]int, lb+1)
		d[i][0] = i
	}
	for j := 0; j <= lb; j++ {
		d[0][j] = j
	}
	for i := 1; i <= la; i++ {
		for j := 1; j <= lb; j++ {
			cost := 0
			if a[i-1] != b[j-1] {
				cost = 1
			}
			d[i][j] = min(d[i-1][j]+1, min(d[i][j-1]+1, d[i-1][j-1]+cost))
		}
	}
	return d[la][lb]
}

func isTerminal(file *os.File) bool {
	stat, err := file.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) != 0
}

const dvaUsageTemplate = `Usage:{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

Aliases:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

Examples:
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}{{$cmds := .Commands}}{{if eq (len .Groups) 0}}

Available Commands:{{range $cmds}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{else}}{{range $group := .Groups}}

{{colorTitle .Title}}{{range $cmds}}{{if (and (eq .GroupID $group.ID) (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if not .AllChildCommandsHaveGroup}}

Additional Commands:{{range $cmds}}{{if (and (eq .GroupID "") (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

Additional help topics:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`
