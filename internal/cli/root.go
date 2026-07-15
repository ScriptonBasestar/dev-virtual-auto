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

// isTopLevelCommand reports whether name is a built-in DVA command.
// Delegates to config.IsReservedCommand for single-source-of-truth.
func isTopLevelCommand(name string) bool {
	return config.IsReservedCommand(name)
}

var rootCmd = &cobra.Command{
	Use:   "dva",
	Short: "DVA: Developer Workspace Automator",
	Long: `DVA (Dev Virtual Auto) is a comprehensive developer workspace automation tool.
It simplifies complex workflows involving Docker Compose and Kubernetes by providing 
intuitive shortcuts and uniform environments defined in 'dva.yml'.

DVA ensures robust and reproducible development environments, 
making it easy to onboard and manage projects.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		logger.Init(debug, jsonOutput)
		if debug {
			_ = os.Setenv(config.EnvDebugKey, "1")
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
	lifecycleGroup := &cobra.Group{ID: "lifecycle", Title: "Lifecycle"}
	integrationGroup := &cobra.Group{ID: "integration", Title: "Integration Tools"}
	advancedGroup := &cobra.Group{ID: "advanced", Title: "Advanced Utilities"}

	rootCmd.AddGroup(coreGroup, projectGroup, lifecycleGroup, integrationGroup, advancedGroup)

	runCmd.GroupID = "core"
	lsCmd.GroupID = "core"
	versionCmd.GroupID = "core"

	showCmd.GroupID = "project"
	statusCmd.GroupID = "project"
	configCmd.GroupID = "project"
	upCmd.GroupID = "lifecycle"
	downCmd.GroupID = "lifecycle"
	stopCmd.GroupID = "lifecycle"
	restartCmd.GroupID = "lifecycle"
	logsCmd.GroupID = "lifecycle"
	buildCmd.GroupID = "lifecycle"
	cleanCmd.GroupID = "lifecycle"
	appCmd.GroupID = "lifecycle"
	stackCmd.GroupID = "lifecycle"

	composeCmd.GroupID = "integration"
	ktlCmd.GroupID = "integration"
	infraCmd.GroupID = "integration"
	sshCmd.GroupID = "integration"

	manifestCmd.GroupID = "advanced"
	consoleCmd.GroupID = "advanced"
	provisionCmd.GroupID = "advanced"

	rootCmd.AddCommand(showCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(lsCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(composeCmd)
	rootCmd.AddCommand(upCmd)
	rootCmd.AddCommand(downCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(restartCmd)
	rootCmd.AddCommand(logsCmd)
	rootCmd.AddCommand(buildCmd)
	rootCmd.AddCommand(cleanCmd)
	rootCmd.AddCommand(provisionCmd)
	rootCmd.AddCommand(manifestCmd)
	rootCmd.AddCommand(ktlCmd)
	rootCmd.AddCommand(sshCmd)
	rootCmd.AddCommand(infraCmd)
	rootCmd.AddCommand(consoleCmd)
	rootCmd.AddCommand(appCmd)
	rootCmd.AddCommand(stackCmd)

	// Wrap hookable lifecycle commands with before/replace/after hook execution.
	// hookableCommands (config.HookableCommands) is the single source of truth;
	// this map only provides the Go variable binding.
	hookableCmds := map[string]*cobra.Command{
		"up": upCmd, "down": downCmd, "stop": stopCmd,
		"restart": restartCmd, "build": buildCmd, "clean": cleanCmd,
		config.LogsDirName: logsCmd,
	}
	for name, cmd := range hookableCmds {
		if !config.IsHookableCommand(name) {
			panic(fmt.Sprintf("BUG: hookableCmds contains '%s' which is not in config.hookableCommands", name))
		}
		wrapWithHooks(name, cmd)
	}
	// Verify all hookable commands are registered
	for name := range config.HookableCommands() {
		if hookableCmds[name] == nil {
			panic(fmt.Sprintf("BUG: config.hookableCommands contains '%s' but no cobra.Command is registered in hookableCmds", name))
		}
	}

	// Cobra does not intercept --help when flag parsing is disabled. Wrap every
	// passthrough command so a direct help request cannot fall through to a
	// lifecycle operation or an external tool.
	manualFlagCommands := []*cobra.Command{
		composeCmd,
		upCmd, downCmd, stopCmd, restartCmd, buildCmd, logsCmd,
		stackUpCmd, stackStopCmd, stackDownCmd, stackLogCmd,
		appUpCmd, appRestartCmd, appBuildCmd,
		infraUpCmd, infraDownCmd,
		ktlCmd,
	}
	for _, cmd := range manualFlagCommands {
		wrapDirectHelp(cmd)
	}

	cobra.AddTemplateFunc("colorTitle", func(s string) string {
		if !jsonOutput && isTerminal(os.Stdout) && os.Getenv("NO_COLOR") == "" {
			return "\033[1;36m" + s + "\033[0m"
		}
		return s
	})
	cobra.AddTemplateFunc("commandNameIs", commandNameIs)
	cobra.AddTemplateFunc("isFeaturedLifecycle", isFeaturedLifecycleCommand)
	cobra.AddTemplateFunc("featuredLifecycleHint", featuredLifecycleHint)
	rootCmd.SetUsageTemplate(dvaUsageTemplate)
}

// wrapDirectHelp restores Cobra's normal `command --help` behavior for
// commands using DisableFlagParsing. Arguments after another token remain
// untouched so passthrough commands such as `dva compose ps --help` still work.
func wrapDirectHelp(cmd *cobra.Command) {
	if cmd == nil || !cmd.DisableFlagParsing || cmd.RunE == nil {
		return
	}
	original := cmd.RunE
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
			return cmd.Help()
		}
		return original(cmd, args)
	}
}

// Execute is the main entry point for the CLI.
func Execute() {
	args := os.Args[1:]

	// Dynamic routing: if first arg is not a top-level command,
	// check if it's an interaction command or namespace:command and prepend "run"
	if len(args) > 0 {
		firstArg := args[0]
		if !isTopLevelCommand(firstArg) && !isFlag(firstArg) {
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

		// Only suggest similar commands for unknown/unrecognized commands, not execution failures
		if len(args) > 0 && !isFlag(args[0]) && strings.Contains(errMsg, "unknown command") {
			if suggestions := suggestCommands(args[0]); len(suggestions) > 0 {
				fmt.Fprintf(os.Stderr, "\nDid you mean?\n")
				for _, s := range suggestions {
					fmt.Fprintf(os.Stderr, "  dva %s\n", s)
				}
			}
		}

		// Hint for dva init if no config found
		if strings.Contains(errMsg, "could not find dva.yml") {
			fmt.Fprintf(os.Stderr, "\nHint: run 'dva init' to create a dva.yml\n")
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
	// Check if .sb/dva is in .gitignore and warn if not
	checkGitignoreForWarning(cfg.FileDir())
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
	if c.EnvFile != nil {
		if err := config.LoadEnvFile(c.EnvFile, c.FileDir(), env); err != nil {
			fmt.Fprintf(os.Stderr, "WARN: env_file: %s\n", err)
		}
	}
	return env
}

// suggestCommands returns commands similar to the input using Levenshtein distance.
func suggestCommands(input string) []string {
	var suggestions []string
	for cmd := range config.ReservedCommands() {
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

func isFeaturedLifecycleCommand(cmd *cobra.Command) bool {
	if cmd == nil || cmd.GroupID != "lifecycle" {
		return false
	}
	switch cmd.Name() {
	case "up", "stack", "app", "down":
		return true
	default:
		return false
	}
}

func featuredLifecycleHint(cmd *cobra.Command) string {
	if cmd == nil || !isFeaturedLifecycleCommand(cmd) {
		return cmd.Short
	}
	switch cmd.Name() {
	case "up":
		return "[start] " + cmd.Short
	case "stack":
		return "[infra] " + cmd.Short
	case "app":
		return "[apps] " + cmd.Short
	case "down":
		return "[down] " + cmd.Short
	default:
		return cmd.Short
	}
}

func commandNameIs(cmd *cobra.Command, name string) bool {
	return cmd != nil && cmd.Name() == name
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

{{colorTitle .Title}}{{if eq $group.ID "lifecycle"}}
  Recommended Flow
{{- range $cmds}}{{if (and (eq .GroupID $group.ID) (or .IsAvailableCommand (eq .Name "help")) (commandNameIs . "up"))}}
  {{rpad .Name .NamePadding }} {{featuredLifecycleHint .}}{{end}}{{end}}
{{- range $cmds}}{{if (and (eq .GroupID $group.ID) (or .IsAvailableCommand (eq .Name "help")) (commandNameIs . "stack"))}}
  {{rpad .Name .NamePadding }} {{featuredLifecycleHint .}}{{end}}{{end}}
{{- range $cmds}}{{if (and (eq .GroupID $group.ID) (or .IsAvailableCommand (eq .Name "help")) (commandNameIs . "app"))}}
  {{rpad .Name .NamePadding }} {{featuredLifecycleHint .}}{{end}}{{end}}
{{- range $cmds}}{{if (and (eq .GroupID $group.ID) (or .IsAvailableCommand (eq .Name "help")) (commandNameIs . "down"))}}
  {{rpad .Name .NamePadding }} {{featuredLifecycleHint .}}{{end}}{{end}}
  Other Commands
{{range $cmds}}{{if (and (eq .GroupID $group.ID) (or .IsAvailableCommand (eq .Name "help")) (not (isFeaturedLifecycle .)))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{else}}{{range $cmds}}{{if (and (eq .GroupID $group.ID) (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{end}}{{if not .AllChildCommandsHaveGroup}}

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
