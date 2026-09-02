package cli

import (
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type commandFlagSpec struct {
	Name     string
	DefValue string
	Usage    string
}

func TestRootCommandRegistersValidate(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"validate"})
	if err != nil {
		t.Fatalf("find validate command: %v", err)
	}
	if cmd == nil || cmd.CommandPath() != "dva validate" {
		t.Fatalf("root validate command = %v, want dva validate", cmd)
	}
}

func TestRootValidateMatchesConfigValidate(t *testing.T) {
	rootValidate := mustFindCommand(t, []string{"validate"})
	configValidate := mustFindCommand(t, []string{"config", "validate"})

	if rootValidate.Use != configValidate.Use {
		t.Fatalf("root validate Use = %q, want %q", rootValidate.Use, configValidate.Use)
	}
	if rootValidate.Short != configValidate.Short {
		t.Fatalf("root validate Short = %q, want %q", rootValidate.Short, configValidate.Short)
	}

	rootFlags := flagSpecs(rootValidate)
	configFlags := flagSpecs(configValidate)
	if !reflect.DeepEqual(rootFlags, configFlags) {
		t.Fatalf("root validate flags = %#v, want %#v", rootFlags, configFlags)
	}
}

// TestCommandHelpGroupsAndDiscoveryDescriptions keeps the user-visible command
// taxonomy and the two complementary discovery descriptions deliberate. It also
// records the compatibility boundaries of this presentation-only change: route
// spelling, aliases, reservation, and manifest schema stay as they were.
func TestCommandHelpGroupsAndDiscoveryDescriptions(t *testing.T) {
	commands := []struct {
		name        string
		use         string
		group       string
		description string
		flags       []commandFlagSpec
	}{
		{
			name: "manifest", use: "manifest", group: "core",
			flags: []commandFlagSpec{{Name: "format", DefValue: "json", Usage: "Output format (json, yaml)"}},
		},
		{name: "show", use: "show", group: "project", description: "Show declared workspace configuration", flags: []commandFlagSpec{}},
		{name: "status", use: "status [NAME]", group: "lifecycle", description: "Display current workspace and runtime status", flags: []commandFlagSpec{}},
	}

	for _, want := range commands {
		cmd := mustFindCommand(t, []string{want.name})
		if cmd.Use != want.use {
			t.Errorf("%s Use = %q, want unchanged %q", want.name, cmd.Use, want.use)
		}
		if len(cmd.Aliases) != 0 {
			t.Errorf("%s aliases = %q, want no aliases", want.name, cmd.Aliases)
		}
		if cmd.Args != nil {
			t.Errorf("%s acquired an argument validator; want the existing unrestricted Args contract", want.name)
		}
		if got := flagSpecs(cmd); !reflect.DeepEqual(got, want.flags) {
			t.Errorf("%s flags = %#v, want unchanged %#v", want.name, got, want.flags)
		}
		if cmd.GroupID != want.group {
			t.Errorf("%s group = %q, want %q", want.name, cmd.GroupID, want.group)
		}
		if want.description != "" && cmd.Short != want.description {
			t.Errorf("%s Short = %q, want %q", want.name, cmd.Short, want.description)
		}
		if !config.IsReservedCommand(want.name) {
			t.Errorf("%s is no longer reserved", want.name)
		}
	}

	manifest := buildManifest(&config.Config{})
	if manifest.SchemaVersion != "1.4" {
		t.Errorf("manifest schema version = %q, want unchanged 1.4", manifest.SchemaVersion)
	}
	for _, name := range []string{"manifest", "show", "status"} {
		entry, ok := manifest.StaticCommands[name]
		if !ok {
			t.Errorf("manifest static_commands lost %q", name)
			continue
		}
		if got, want := entry.Description, mustFindCommand(t, []string{name}).Short; got != want {
			t.Errorf("manifest static_commands[%q].description = %q, want command Short %q", name, got, want)
		}
	}

	help := rootCmd.UsageString()
	core := strings.Index(help, "Core Commands")
	project := strings.Index(help, "Project Management")
	lifecycle := strings.Index(help, "Lifecycle")
	integration := strings.Index(help, "Integration Tools")
	advanced := strings.Index(help, "Advanced Utilities")
	if core < 0 || project < 0 || lifecycle < 0 || integration < 0 || advanced < 0 ||
		core >= project || project >= lifecycle || lifecycle >= integration || integration >= advanced {
		t.Fatalf("help group order = core:%d project:%d lifecycle:%d integration:%d advanced:%d, "+
			"want that order:\n%s", core, project, lifecycle, integration, advanced, help)
	}
	other := strings.Index(help, "Other Commands")
	if other < lifecycle || integration < other {
		t.Fatalf("lifecycle Other Commands block not found before Integration Tools:\n%s", help)
	}
	for _, want := range []struct {
		name       string
		groupStart int
		groupEnd   int
	}{
		{name: "manifest", groupStart: core, groupEnd: project},
		{name: "show", groupStart: project, groupEnd: lifecycle},
		{name: "status", groupStart: other, groupEnd: integration},
	} {
		at := indexOfLine(t, help, want.name)
		if at <= want.groupStart || (want.groupEnd >= 0 && at >= want.groupEnd) {
			t.Errorf("%s is at %d, want it in its help group (%d..%d):\n%s", want.name, at, want.groupStart, want.groupEnd, help)
		}
	}
}

// This list mirrors root.go's manualFlagCommands, which is a local in init() and so cannot be
// read from here. It is written out rather than exported because a test that iterates the
// production list asserts nothing about that list's contents — dropping a command from
// root.go would silently drop its coverage too.
//
// The copy costs an edit per change and the compiler only charges for it on a deletion or a
// rename, which is what happened when `dva stack`/`app`/`infra` went: nine entries here, none
// in root.go. An *addition* is the shape that goes quiet — a new DisableFlagParsing command
// added to root.go and not here is untested, not failing.
func TestDirectHelpDoesNotExecuteManualFlagCommands(t *testing.T) {
	commands := []*cobra.Command{
		composeCmd,
		upCmd, downCmd, stopCmd, restartCmd, buildCmd, logsCmd,
		ktlCmd,
	}

	for _, command := range commands {
		t.Run(command.CommandPath(), func(t *testing.T) {
			helpCalled := false
			originalHelp := command.HelpFunc()
			command.SetHelpFunc(func(*cobra.Command, []string) { helpCalled = true })
			t.Cleanup(func() { command.SetHelpFunc(originalHelp) })

			if err := command.RunE(command, []string{"--help"}); err != nil {
				t.Fatalf("direct help returned error: %v", err)
			}
			if !helpCalled {
				t.Fatal("direct help reached command execution instead of the help handler")
			}
		})
	}
}

func TestRootValidateMatchesConfigValidateBehavior(t *testing.T) {
	validConfig := writeValidateConfigForTest(t, `version: "0.1.44"
stack:
  core-compose:
    default_runner: compose
    runners:
      compose:
        files: [compose.yml]
        project_name: app
plans:
  local-dev:
    entries:
      - name: core-compose
        runner: compose
        services: [postgres]
`)
	assertSameValidateBehavior(t, validConfig, false)

	invalidConfig := writeValidateConfigForTest(t, `version: "0.1.44"
environments:
  dev:
    vars:
      SHOULD_FAIL: "1"
`)
	assertSameValidateBehavior(t, invalidConfig, true)
}

func mustFindCommand(t *testing.T, args []string) *cobra.Command {
	t.Helper()
	cmd, _, err := rootCmd.Find(args)
	if err != nil {
		t.Fatalf("find command %v: %v", args, err)
	}
	if cmd == nil {
		t.Fatalf("find command %v returned nil", args)
	}
	return cmd
}

func flagSpecs(cmd *cobra.Command) []commandFlagSpec {
	specs := []commandFlagSpec{}
	cmd.Flags().VisitAll(func(flag *pflag.Flag) {
		specs = append(specs, commandFlagSpec{
			Name:     flag.Name,
			DefValue: flag.DefValue,
			Usage:    flag.Usage,
		})
	})
	sort.Slice(specs, func(i, j int) bool {
		return specs[i].Name < specs[j].Name
	})
	return specs
}

type validateCommandResult struct {
	stdout string
	stderr string
	err    string
}

func assertSameValidateBehavior(t *testing.T, configPath string, wantErr bool) {
	t.Helper()
	rootResult := runValidateCommandForTest(t, configPath, "validate")
	configResult := runValidateCommandForTest(t, configPath, "config", "validate")

	if rootResult != configResult {
		t.Fatalf("root validate result = %#v, want %#v", rootResult, configResult)
	}
	if wantErr {
		if rootResult.err == "" {
			t.Fatal("validate succeeded, want error")
		}
		return
	}
	if rootResult.err != "" {
		t.Fatalf("validate returned error: %s", rootResult.err)
	}
	if !strings.Contains(rootResult.stdout, "dva.yml is valid") {
		t.Fatalf("validate stdout = %q, want success message", rootResult.stdout)
	}
}

func runValidateCommandForTest(t *testing.T, configPath string, args ...string) validateCommandResult {
	t.Helper()
	t.Setenv(config.EnvFileKey, configPath)

	oldCfg, oldEnv, oldStrict := cfg, env, validateStrict
	cfg, env = nil, nil
	validateStrict = false
	defer func() {
		cfg, env = oldCfg, oldEnv
		validateStrict = oldStrict
	}()

	for _, path := range [][]string{{"validate"}, {"config", "validate"}} {
		resetValidateFlagsForTest(t, mustFindCommand(t, path))
	}
	rootCmd.SetArgs(args)
	defer rootCmd.SetArgs(nil)

	stdout, stderr, err := captureValidateOutput(t, func() error {
		_, executeErr := rootCmd.ExecuteC()
		return executeErr
	})
	result := validateCommandResult{stdout: stdout, stderr: stderr}
	if err != nil {
		result.err = err.Error()
	}
	return result
}

func resetValidateFlagsForTest(t *testing.T, cmd *cobra.Command) {
	t.Helper()
	if err := cmd.Flags().Set("fix", "false"); err != nil {
		t.Fatalf("reset --fix: %v", err)
	}
	if err := cmd.Flags().Set("strict", "false"); err != nil {
		t.Fatalf("reset --strict: %v", err)
	}
}

func writeValidateConfigForTest(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), config.FileName)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write validate config: %v", err)
	}
	return path
}

func captureValidateOutput(t *testing.T, run func() error) (string, string, error) {
	t.Helper()
	oldStdout, oldStderr := os.Stdout, os.Stderr
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("open stdout pipe: %v", err)
	}
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("open stderr pipe: %v", err)
	}

	stdoutDone := readPipeForTest(t, stdoutReader)
	stderrDone := readPipeForTest(t, stderrReader)
	os.Stdout, os.Stderr = stdoutWriter, stderrWriter
	runErr := run()
	_ = stdoutWriter.Close()
	_ = stderrWriter.Close()
	os.Stdout, os.Stderr = oldStdout, oldStderr

	return <-stdoutDone, <-stderrDone, runErr
}

func readPipeForTest(t *testing.T, reader *os.File) <-chan string {
	t.Helper()
	done := make(chan string, 1)
	go func() {
		data, err := io.ReadAll(reader)
		if err != nil {
			done <- err.Error()
			return
		}
		done <- string(data)
	}()
	return done
}
