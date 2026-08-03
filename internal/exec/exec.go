package exec

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// ExecReplace replaces the current process with the given command.
// Equivalent to Ruby's Kernel.exec.
func ExecReplace(env *config.Environment, cmd string, args []string, shell bool) error {
	cmdLine := buildCommandLine(env, cmd, args, shell)

	if Debug {
		slog.Debug("exec replacing process", "command", strings.Join(cmdLine, " "))
	}

	binary, err := exec.LookPath(cmdLine[0])
	if err != nil {
		return fmt.Errorf("command not found: %s", cmdLine[0])
	}

	// Test-integrity guard (TASK-144). syscall.Exec replaces the running process image and never
	// returns, so a test that reaches this line in-process is gone the instant syscall.Exec runs:
	// the substituted program's exit status becomes the test binary's, and `go test` reports the
	// package as `ok` whatever it did. That masks the exact regressions a test exists to catch —
	// TASK-091 and TASK-094 each shipped green this way once, three emitted `--- FAIL` lines and
	// all still reported `ok`.
	//
	// The guard sits AFTER LookPath on purpose. Four tests point at a binary exec.LookPath cannot
	// resolve precisely so execution stops at the error above (two in exec_test.go, two in
	// execution_paths_test.go); the guard must not turn their expected "command not found" into a
	// panic. It sits BEFORE syscall.Exec so the process is never actually replaced — under `go test`
	// it refuses unless the caller set DVA_EXEC_REPLACE_OK, the explicit, visible opt-in a
	// child-process test that genuinely needs the replacement would set. The ktl passthrough child
	// (ktl_flag_passthrough_test.go) is the sole legitimate caller today; the opt-in is named so the
	// next one cannot be written silently, and it belongs on a single cmd.Env — exporting it in a
	// shell disables the guard for the whole `go test` run.
	if testing.Testing() && os.Getenv("DVA_EXEC_REPLACE_OK") != "1" {
		panic("dvaexec.ExecReplace reached under `go test` without a subprocess boundary: " +
			"syscall.Exec would replace the test binary and mask every failure recorded so far " +
			"(TASK-144). Run the code under test in a child process, or point it at a binary " +
			"exec.LookPath cannot resolve; set DVA_EXEC_REPLACE_OK=1 only in a child that " +
			"genuinely intends the replacement.")
	}

	envSlice := env.EnvSlice()
	return syscall.Exec(binary, cmdLine, envSlice)
}

// ExecSubprocess runs a command as a subprocess and waits for it.
// Equivalent to Ruby's Kernel.system.
func ExecSubprocess(env *config.Environment, cmd string, args []string, shell bool) error {
	return ExecSubprocessInDir(env, "", cmd, args, shell)
}

// ExecSubprocessInDir runs a command as a subprocess in the given working
// directory and waits for it. An empty dir inherits the current directory.
func ExecSubprocessInDir(env *config.Environment, dir, cmd string, args []string, shell bool) error {
	cmdLine := buildCommandLine(env, cmd, args, shell)

	if Debug {
		slog.Debug("exec subprocess", "dir", dir, "command", strings.Join(cmdLine, " "))
	}

	c := exec.Command(cmdLine[0], cmdLine[1:]...)
	c.Dir = dir
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Env = env.EnvSlice()

	if err := c.Run(); err != nil {
		return fmt.Errorf("command '%s' executed with error: %w", strings.Join(cmdLine, " "), err)
	}
	return nil
}

// ExecSubprocessCaptureInDir runs a command in dir and returns its combined
// stdout+stderr instead of streaming to the terminal. Used for preflight/probe
// commands (e.g. `docker compose config`) whose output should be inspected and
// reformatted into an actionable dva error rather than leaked raw. Interpolation
// and env handling match ExecSubprocessInDir; an empty dir inherits the cwd.
func ExecSubprocessCaptureInDir(env *config.Environment, dir, cmd string, args []string, shell bool) (string, error) {
	cmdLine := buildCommandLine(env, cmd, args, shell)

	if Debug {
		slog.Debug("exec subprocess (capture)", "dir", dir, "command", strings.Join(cmdLine, " "))
	}

	c := exec.Command(cmdLine[0], cmdLine[1:]...)
	c.Dir = dir
	c.Env = env.EnvSlice()

	out, err := c.CombinedOutput()
	return string(out), err
}

// ExecSubprocessOutput runs a command and captures output.
func ExecSubprocessOutput(cmd string, args ...string) (string, error) {
	c := exec.Command(cmd, args...)
	out, err := c.Output()
	return strings.TrimSpace(string(out)), err
}

// buildCommandLine constructs the command line with interpolation.
func buildCommandLine(env *config.Environment, cmd string, args []string, shell bool) []string {
	cmd = env.Interpolate(cmd)
	interpolated := make([]string, len(args))
	for i, a := range args {
		interpolated[i] = env.Interpolate(a)
	}

	if shell {
		fullCmd := cmd
		if len(interpolated) > 0 {
			fullCmd = fullCmd + " " + strings.Join(interpolated, " ")
		}
		fullCmd = strings.TrimSpace(fullCmd)
		return []string{"sh", "-c", fullCmd}
	}

	result := make([]string, 0, 1+len(interpolated))
	result = append(result, SplitCommand(cmd)...)
	result = append(result, interpolated...)
	return result
}

// SplitCommand splits a command string respecting quotes.
func SplitCommand(cmd string) []string {
	var parts []string
	var current strings.Builder
	inQuote := false
	quoteChar := byte(0)

	for i := 0; i < len(cmd); i++ {
		ch := cmd[i]
		if inQuote {
			if ch == quoteChar {
				inQuote = false
			} else {
				current.WriteByte(ch)
			}
		} else if ch == '\'' || ch == '"' {
			inQuote = true
			quoteChar = ch
		} else if ch == ' ' || ch == '\t' {
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
		} else {
			current.WriteByte(ch)
		}
	}
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}
	return parts
}

// ExecSequential runs multiple commands as subprocesses in order.
// Stops and returns an error on first failure.
func ExecSequential(env *config.Environment, cmds []string, shell bool) error {
	for _, cmd := range cmds {
		cmd = strings.TrimSpace(cmd)
		if cmd == "" {
			continue
		}
		if err := ExecSubprocess(env, cmd, nil, shell); err != nil {
			return err
		}
	}
	return nil
}

// ExecScriptInline writes the script content to a temporary file and executes it.
// The temporary file is removed after execution.
// If the script does not begin with a shebang (#!), #!/bin/sh is prepended automatically.
func ExecScriptInline(env *config.Environment, script string) error {
	f, err := os.CreateTemp("", "dva-script-*.sh")
	if err != nil {
		return fmt.Errorf("creating temp script: %w", err)
	}
	defer func() { _ = os.Remove(f.Name()) }()

	// Auto-prepend shebang when absent so the script is always executable
	// regardless of which shell the user happens to be running.
	if !strings.HasPrefix(strings.TrimSpace(script), "#!") {
		script = "#!/bin/sh\n" + script
	}

	if _, err := f.WriteString(script); err != nil {
		_ = f.Close()
		return fmt.Errorf("writing temp script: %w", err)
	}
	// Checked rather than deferred or dropped: this file is chmod'ed and executed
	// three lines below. A buffered write can surface a short write or ENOSPC only
	// at Close, so ignoring it here means running a truncated script as if it were
	// the whole one — a shell script that stops early is not a script that failed,
	// it is a different script. TASK-127.
	if err := f.Close(); err != nil {
		return fmt.Errorf("closing temp script: %w", err)
	}

	if err := os.Chmod(f.Name(), 0700); err != nil {
		return fmt.Errorf("chmod temp script: %w", err)
	}

	return ExecSubprocess(env, f.Name(), nil, false)
}

// ExecScriptFile runs an external shell script file.
func ExecScriptFile(env *config.Environment, path string) error {
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("script_file %q: %w", path, err)
	}
	return ExecSubprocess(env, path, nil, false)
}

// Debug enables debug logging for exec operations.
var Debug bool
