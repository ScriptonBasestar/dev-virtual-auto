package exec

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"syscall"

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
	defer os.Remove(f.Name())

	// Auto-prepend shebang when absent so the script is always executable
	// regardless of which shell the user happens to be running.
	if !strings.HasPrefix(strings.TrimSpace(script), "#!") {
		script = "#!/bin/sh\n" + script
	}

	if _, err := f.WriteString(script); err != nil {
		f.Close()
		return fmt.Errorf("writing temp script: %w", err)
	}
	f.Close()

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
