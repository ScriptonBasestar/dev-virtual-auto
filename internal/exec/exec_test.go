package exec

import (
	"strings"
	"testing"
	"time"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// ---------------------------------------------------------------------------
// SplitCommand
// ---------------------------------------------------------------------------

func TestSplitCommand_Simple(t *testing.T) {
	got := SplitCommand("docker compose")
	want := []string{"docker", "compose"}
	if len(got) != len(want) {
		t.Fatalf("SplitCommand() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestSplitCommand_SingleWord(t *testing.T) {
	got := SplitCommand("docker")
	if len(got) != 1 || got[0] != "docker" {
		t.Errorf("SplitCommand() = %v, want [docker]", got)
	}
}

func TestSplitCommand_Empty(t *testing.T) {
	got := SplitCommand("")
	if len(got) != 0 {
		t.Errorf("SplitCommand('') = %v, want []", got)
	}
}

func TestSplitCommand_SingleQuotes(t *testing.T) {
	got := SplitCommand("echo 'hello world'")
	want := []string{"echo", "hello world"}
	if len(got) != len(want) {
		t.Fatalf("SplitCommand() = %v, want %v", got, want)
	}
	if got[1] != "hello world" {
		t.Errorf("got[1] = %q, want 'hello world'", got[1])
	}
}

func TestSplitCommand_DoubleQuotes(t *testing.T) {
	got := SplitCommand(`echo "hello world"`)
	if len(got) != 2 {
		t.Fatalf("SplitCommand() = %v, want 2 parts", got)
	}
	if got[1] != "hello world" {
		t.Errorf("got[1] = %q, want 'hello world'", got[1])
	}
}

func TestSplitCommand_MultipleSpaces(t *testing.T) {
	got := SplitCommand("a  b\tc")
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("SplitCommand() = %v, want %v", got, want)
	}
}

func TestSplitCommand_TrailingSpaces(t *testing.T) {
	got := SplitCommand("  echo  ")
	if len(got) != 1 || got[0] != "echo" {
		t.Errorf("SplitCommand() = %v, want [echo]", got)
	}
}

// ---------------------------------------------------------------------------
// buildCommandLine (tested indirectly via ExecSubprocess path helpers,
// and directly through the exported SplitCommand that it relies on)
// ---------------------------------------------------------------------------

func TestBuildCommandLine_ShellMode(t *testing.T) {
	// We verify shell mode by running a real command via ExecSubprocess
	// with shell=true — simpler: inspect output of ExecSubprocessOutput
	out, err := ExecSubprocessOutput("sh", "-c", "echo shell_mode_ok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "shell_mode_ok" {
		t.Errorf("output = %q, want 'shell_mode_ok'", out)
	}
}

func TestBuildCommandLine_Interpolation(t *testing.T) {
	env := config.NewEnvironment(map[string]string{"GREETING": "hello"}, "/tmp", "/tmp")
	// ExecSubprocess with shell=true uses buildCommandLine + sh -c
	// We run a command that echoes the interpolated variable
	err := ExecSubprocess(env, "echo $GREETING", nil, true)
	// No error expected; real execution happens here (echo to stdout)
	if err != nil {
		t.Errorf("unexpected error from ExecSubprocess(shell): %v", err)
	}
}

// ---------------------------------------------------------------------------
// ExecSubprocessOutput
// ---------------------------------------------------------------------------

func TestExecSubprocessOutput_Success(t *testing.T) {
	out, err := ExecSubprocessOutput("echo", "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "hello" {
		t.Errorf("output = %q, want 'hello'", out)
	}
}

func TestExecSubprocessOutput_Trim(t *testing.T) {
	// printf with trailing newline — TrimSpace should strip it
	out, err := ExecSubprocessOutput("sh", "-c", "printf 'trimmed\\n'")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "trimmed" {
		t.Errorf("output = %q, want 'trimmed'", out)
	}
}

func TestExecSubprocessOutput_CommandNotFound(t *testing.T) {
	_, err := ExecSubprocessOutput("__nonexistent_binary_xyz__")
	if err == nil {
		t.Error("expected error for nonexistent command")
	}
}

// ---------------------------------------------------------------------------
// ExecSubprocess
// ---------------------------------------------------------------------------

func TestExecSubprocess_Success(t *testing.T) {
	env := config.NewEnvironment(nil, "/tmp", "/tmp")
	if err := ExecSubprocess(env, "true", nil, false); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestExecSubprocessInDir_DoesNotInheritStdinUnderTest(t *testing.T) {
	// cat with an inherited never-EOF stdin blocks forever — the GitHub Actions
	// 46m job abort. Under `go test` stdin must stay closed so the child exits.
	env := config.NewEnvironment(nil, t.TempDir(), t.TempDir())
	done := make(chan error, 1)
	go func() {
		done <- ExecSubprocessInDir(env, "", "cat", nil, false)
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("cat with closed stdin: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("cat inherited stdin and blocked")
	}
}

func TestExecSubprocess_Failure(t *testing.T) {
	env := config.NewEnvironment(nil, "/tmp", "/tmp")
	err := ExecSubprocess(env, "false", nil, false)
	if err == nil {
		t.Error("expected error from 'false'")
	}
	if !strings.Contains(err.Error(), "executed with error") {
		t.Errorf("error = %q, want to contain 'executed with error'", err.Error())
	}
}

func TestExecSubprocess_CommandNotFound(t *testing.T) {
	env := config.NewEnvironment(nil, "/tmp", "/tmp")
	// non-existent binary; shell=false so LookPath is not called before exec.Command
	err := ExecSubprocess(env, "__nonexistent_binary_xyz__", nil, false)
	if err == nil {
		t.Error("expected error for nonexistent command")
	}
}

func TestExecSubprocess_ShellMode_Success(t *testing.T) {
	env := config.NewEnvironment(nil, "/tmp", "/tmp")
	if err := ExecSubprocess(env, "true", nil, true); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestExecSubprocess_ShellMode_WithArgs(t *testing.T) {
	env := config.NewEnvironment(nil, "/tmp", "/tmp")
	// shell mode concatenates cmd + args into one string for sh -c
	if err := ExecSubprocess(env, "echo", []string{"hello"}, true); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestExecSubprocess_EnvVarPassed(t *testing.T) {
	env := config.NewEnvironment(map[string]string{"MY_TEST_VAR": "testvalue"}, "/tmp", "/tmp")
	// sh -c reads env vars from the process environment (passed via cmd.Env)
	out, err := ExecSubprocessOutput("sh", "-c", "echo $MY_TEST_VAR")
	_ = out
	_ = err
	// Can't rely on ExecSubprocessOutput for custom env; use ExecSubprocess and
	// verify indirectly that no error occurs when the env is set.
	if err2 := ExecSubprocess(env, "sh -c 'echo $MY_TEST_VAR'", nil, false); err2 != nil {
		t.Errorf("unexpected error: %v", err2)
	}
}

// ---------------------------------------------------------------------------
// ExecReplace (only test the error path — success would replace the process)
// ---------------------------------------------------------------------------

func TestExecReplace_CommandNotFound(t *testing.T) {
	env := config.NewEnvironment(nil, "/tmp", "/tmp")
	err := ExecReplace(env, "__nonexistent_binary_xyz__", nil, false)
	if err == nil {
		t.Error("expected error for nonexistent command")
	}
	if !strings.Contains(err.Error(), "command not found") {
		t.Errorf("error = %q, want 'command not found'", err.Error())
	}
}

func TestExecReplace_Debug_Enabled(t *testing.T) {
	old := Debug
	Debug = true
	defer func() { Debug = old }()

	env := config.NewEnvironment(nil, "/tmp", "/tmp")
	err := ExecReplace(env, "__nonexistent_binary_xyz__", nil, false)
	if err == nil {
		t.Error("expected error for nonexistent command")
	}
}

// ---------------------------------------------------------------------------
// Debug flag
// ---------------------------------------------------------------------------

func TestExecSubprocess_Debug_Enabled(t *testing.T) {
	// Smoke test: Debug=true should not cause panics or unexpected errors.
	old := Debug
	Debug = true
	defer func() { Debug = old }()

	env := config.NewEnvironment(nil, "/tmp", "/tmp")
	// 'true' exits 0; Debug logging goes through slog (not raw os.Stderr write)
	if err := ExecSubprocess(env, "true", nil, false); err != nil {
		t.Errorf("unexpected error with Debug=true: %v", err)
	}
}
