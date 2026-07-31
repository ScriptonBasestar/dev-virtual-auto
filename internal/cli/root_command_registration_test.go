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

func TestDirectHelpDoesNotExecuteManualFlagCommands(t *testing.T) {
	commands := []*cobra.Command{
		composeCmd,
		upCmd, downCmd, stopCmd, restartCmd, buildCmd, logsCmd,
		stackUpCmd, stackStopCmd, stackDownCmd, stackLogCmd,
		appUpCmd, appRestartCmd, appBuildCmd,
		infraUpCmd, infraDownCmd,
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
