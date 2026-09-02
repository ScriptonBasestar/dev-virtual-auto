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
	"github.com/ScriptonBasestar/dva/internal/output"
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
		// DisableFlagParsing commands never let cobra parse root persistent
		// flags. Pre-scan os.Args so --debug/--json reach logger.Init.
		// Do not consume --dry-run here: composeCmd forwards it to docker.
		applyRootPersistentFlagsFromArgs(os.Args[1:])
		runningCommand = cmd.Name()
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

// runningCommand is the name of the cobra command PersistentPreRun dispatched to. It exists
// for pre-command warnings that need to know whether the command they are warning ahead of
// is itself the one that reports that finding properly — see gitignoreWarningSuppressedFor.
var runningCommand string

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

	composeCmd.GroupID = "integration"
	ktlCmd.GroupID = "integration"
	sshCmd.GroupID = "integration"

	manifestCmd.GroupID = "advanced"
	consoleCmd.GroupID = "advanced"
	provisionCmd.GroupID = "advanced"
	skillCmd.GroupID = "advanced"

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
	rootCmd.AddCommand(provisionCmd)
	rootCmd.AddCommand(manifestCmd)
	rootCmd.AddCommand(ktlCmd)
	rootCmd.AddCommand(sshCmd)
	rootCmd.AddCommand(consoleCmd)
	rootCmd.AddCommand(skillCmd)

	// Wrap hookable lifecycle commands with before/replace/after hook execution.
	// hookableCommands (config.HookableCommands) is the single source of truth;
	// this map only provides the Go variable binding.
	hookableCmds := map[string]*cobra.Command{
		"up": upCmd, "down": downCmd, "stop": stopCmd,
		"restart": restartCmd, "build": buildCmd,
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
	// Group parents must be wired after every package init has registered their children
	// (init file order is alphabetical, so a call from config_dump.go would miss later
	// subcommands). SuggestionsFor needs the full set (TASK-148).
	setGroupParentBehavior(configCmd)
	setGroupParentBehavior(sshCmd)

	args := os.Args[1:]

	// Dynamic routing: if first arg is not a top-level command,
	// check if it's an interaction command or namespace:command and prepend "run".
	// Keep the lookup here, where its error is intentionally non-fatal; the pure rewrite below
	// is also used by tests to compare shorthand routing with explicit `dva run`.
	if len(args) > 0 {
		firstArg := args[0]
		if !isTopLevelCommand(firstArg) && !isFlag(firstArg) {
			if c, err := loadConfig(); err == nil {
				args = dynamicRunArgs(args, c)
				os.Args = append([]string{os.Args[0]}, args...)
			}
		}
	}

	if err := rootCmd.Execute(); err != nil {
		errMsg := err.Error()
		emitFailureJSON(errMsg)
		fmt.Fprintf(os.Stderr, "\nERROR: %s\n", errMsg)

		// Cobra already prints its own "Did you mean this?" block for an unknown command, so dva
		// no longer prints a second one. The two disagreed: for `dva sta`, cobra offered five names
		// including `status` while dva offered four and omitted it, because `levenshtein("sta",
		// "status")` is 3 and dva's cutoff was 2 — cobra also matches on prefix. Two answers to one
		// question, with nothing telling the reader which to trust. TASK-108, TASK-098.
		//
		// One suggestion is lost by design: cobra never offers `help`, because IsAvailableCommand()
		// returns false for the parent's helpCommand and SuggestionsFor only considers available
		// commands. `dva hlep` therefore gets no suggestion at all. Measured, not assumed.

		// Hint for dva init if no config found
		if strings.Contains(errMsg, "could not find dva.yml") {
			fmt.Fprintf(os.Stderr, "\nHint: run 'dva init' to create a dva.yml\n")
		}

		os.Exit(1)
	}
}

func isFlag(s string) bool {
	return len(s) > 1 && s[0] == '-'
}

// dynamicRunArgs rewrites a dynamic-command shorthand into the explicit run form.
//
// A lone "-" is a supported interaction or entry name, not a flag. More generally, shorthand
// must preserve the explicit form's token order: Cobra already parses run and persistent flags
// in interspersed position and honours `--`, while reordering can detach a value from its flag
// or move a child argument ahead of the interaction name.
func dynamicRunArgs(args []string, c *config.Config) []string {
	if len(args) == 0 {
		return args
	}
	firstArg := args[0]
	if isTopLevelCommand(firstArg) || isFlag(firstArg) || c == nil {
		return args
	}

	shouldRoute := c.Interaction[firstArg] != nil
	if !shouldRoute && strings.Contains(firstArg, ":") {
		parts := strings.SplitN(firstArg, ":", 2)
		_, shouldRoute = c.Subprojects[parts[0]]
	}
	if !shouldRoute {
		return args
	}

	return append([]string{"run"}, args...)
}

// applyRootPersistentFlagsFromArgs sets root persistent --debug and --json from
// a raw arg slice. Used when DisableFlagParsing prevents cobra from parsing them
// before PersistentPreRun runs logger.Init. Does not mutate args or touch --dry-run
// (compose passthrough must keep docker's own --dry-run).
//
// Best-effort by design: it runs before any command has been chosen, so it cannot fail the
// invocation. A malformed value is skipped here and rejected by consumeRootPersistentFlags,
// which runs inside RunE and can return an error.
func applyRootPersistentFlagsFromArgs(args []string) {
	end := dvaFlagEnd(args)
	for _, a := range args[:end] {
		name, value, hasValue := splitFlagToken(a)
		v, ok := flagBoolValue(value, hasValue)
		if !ok {
			continue
		}
		switch name {
		case "--debug":
			debug = v
		case "--json":
			jsonOutput = v
		}
	}
}

// consumeRootPersistentFlags is applyRootPersistentFlagsFromArgs for the commands that hand
// their arguments to another program: it applies --debug/--json and returns args with those
// two removed, so DVA's own flags do not become the plugin's.
//
// It exists separately from parseDvaFlags because the raw passthroughs cannot use that one.
// parseDvaFlags also consumes --mode, --env and the tag flags; for `dva compose` and
// `dva logs` those belong to docker, and eating them would break the passthrough this is
// meant to protect. --dry-run is left in place for the same reason, which
// applyRootPersistentFlagsFromArgs already documents: compose has its own.
//
// That last one reaches docker from `dva compose` and nowhere else. This comment used to
// name `dva logs` and `dva stack log` alongside it; only stack log ever behaved that way,
// and it is gone. `dva logs` is a hookable built-in, so wrapWithHooks replaces its RunE and
// consumeDryRunFlag (hooks.go:28) claims the token before the body runs — as it does for
// every other hookable command. Leaving this function's treatment of --dry-run alone is
// still right: it is not this function's job to know which of its callers is wrapped.
//
// This is the only one of the four consumers that drops the `--` terminator, because it is
// the only one whose output is the external argv. `--` ends DVA's flags, so DVA eats it and
// forwards the rest; a caller who wants a literal `--` to reach docker writes `-- --`. The
// other three hand their output back into DVA, where a later consumer still needs to see the
// terminator to know where to stop.
//
// It rejects `--debug=notabool` rather than forwarding it, for the same reason: past this
// point the token would be docker's problem, and docker would report it as its own unknown
// flag. This is the last place that knows the token is DVA's.
func consumeRootPersistentFlags(args []string) ([]string, error) {
	end := dvaFlagEnd(args)
	filtered := make([]string, 0, len(args))
	for i := range end {
		a := args[i]
		name, value, hasValue := splitFlagToken(a)
		if name != "--debug" && name != "--json" {
			filtered = append(filtered, a)
			continue
		}
		v, ok := flagBoolValue(value, hasValue)
		if !ok {
			return nil, fmt.Errorf("invalid boolean value %q for %s", value, name)
		}
		if name == "--debug" {
			debug = v
		} else {
			jsonOutput = v
		}
	}
	if end < len(args) {
		filtered = append(filtered, args[end+1:]...)
	}
	return filtered, nil
}

func helpRequested(args []string) bool {
	for _, a := range args {
		if a == "--help" || a == "-h" {
			return true
		}
	}
	return false
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

// emitFailureJSON puts a parseable failure on stdout when the caller asked for --json.
// The flag's audience is a program (internal/cli/CLAUDE.md: "LLM 파이프라인용"), and a
// failed command used to leave stdout empty, making the exit code the only signal — one
// that cannot say why. The message is the same string stderr gets, so everything recent
// work put into it (TASK-073's build-vs-config blame, TASK-074's route) travels with it.
//
// Two conditions, and the second is the one worth explaining. A command that already
// printed its document must not get a second one appended: `dva doctor --json` exits 1
// with a complete {"checks": …} object whose "passed": false IS the failure — measured,
// 488 bytes on stdout with exit 1 — and adding an envelope there would make stdout two
// concatenated documents, which is not what a consumer piping to `jq` reads. The envelope
// exists to keep stdout from being empty, so it yields to a stdout that is not.
//
// Its error is dropped deliberately: the map below holds only strings and an int and
// cannot fail to marshal, and both callers print the same message to stderr and exit 1
// immediately after, so there is no path where dropping it loses the failure.
func emitFailureJSON(message string) {
	if !jsonOutput || output.StdoutHasDocument() {
		return
	}
	_ = output.PrintJSON(map[string]any{
		"error": map[string]any{
			"message":   message,
			"exit_code": 1,
		},
	})
}

func mustLoadConfig() *config.Config {
	c, err := loadConfig()
	if err != nil {
		// The second failure choke point, and not reachable from the first: this exits
		// before rootCmd.Execute() returns, so the envelope has to be emitted here too.
		// Thirty-four call sites across fourteen files reach it.
		errMsg := err.Error()
		emitFailureJSON(errMsg)
		fmt.Fprintf(os.Stderr, "\nERROR: %s\n", errMsg)
		os.Exit(1)
	}
	return c
}

func loadEnv(c *config.Config) *config.Environment {
	if env != nil {
		return env
	}
	env = newConfigEnvironment(c)
	return env
}

// newConfigEnvironment builds an uncached environment for one effective config.
// Imported plans use this instead of the package-global root environment so child
// vars and env_file paths cannot be contaminated by a parent invocation.
func newConfigEnvironment(c *config.Config) *config.Environment {
	wd, _ := os.Getwd()
	return newConfigEnvironmentAt(c, wd)
}

// newOwnedConfigEnvironment gives an imported project the same default working
// directory it has when invoked directly from its own root.
func newOwnedConfigEnvironment(c *config.Config) *config.Environment {
	return newConfigEnvironmentAt(c, c.FileDir())
}

func newConfigEnvironmentAt(c *config.Config, workDir string) *config.Environment {
	// Precedence (lowest → highest among config layers):
	// vars < environment: < env_file; OS env still wins per MergeVars.
	result := config.NewEnvironment(c.Vars, workDir, c.FileDir())
	result.MergeVars(c.Environment)
	if c.EnvFile != nil {
		if err := config.LoadEnvFile(c.EnvFile, c.FileDir(), result); err != nil {
			fmt.Fprintf(os.Stderr, "WARN: env_file: %s\n", err)
		}
	}
	return result
}

// levenshtein calculates the edit distance between two strings.
// Retained after TASK-108 removed suggestCommands: stack.go and provision.go still use it to
// suggest near-miss stack entries and plan names, which cobra knows nothing about.
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

// isFeaturedLifecycleCommand reports whether the lifecycle help block lists cmd
// explicitly, under "Recommended Flow". The template uses it to keep those out of
// "Other Commands", which lists whatever the explicit block did not.
//
// It used to feature four: up/down plus a "Direct Access" pair, stack and app. Those two
// commands are gone, and with them the reason for a second block — every lifecycle verb
// left takes the same plan name, so there is no longer a backend-shaped way in to steer
// people away from.
func isFeaturedLifecycleCommand(cmd *cobra.Command) bool {
	if cmd == nil || cmd.GroupID != "lifecycle" {
		return false
	}
	switch cmd.Name() {
	case "up", "down":
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
{{- range $cmds}}{{if (and (eq .GroupID $group.ID) (or .IsAvailableCommand (eq .Name "help")) (commandNameIs . "down"))}}
  {{rpad .Name .NamePadding }} {{featuredLifecycleHint .}}{{end}}{{end}}
  Other Commands
{{- range $cmds}}{{if (and (eq .GroupID $group.ID) (or .IsAvailableCommand (eq .Name "help")) (not (isFeaturedLifecycle .)))}}
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
