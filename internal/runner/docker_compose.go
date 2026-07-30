package runner

import (
	"fmt"
	"os"
	"strings"

	"github.com/ScriptonBasestar/dva/internal/config"
	dvaexec "github.com/ScriptonBasestar/dva/internal/exec"
)

// DockerComposeRunner executes commands via Docker Compose.
type DockerComposeRunner struct {
	Cmd  *ResolvedCommand
	Opts RunOptions

	detectedProject string
}

// Execute builds and runs the docker compose command.
func (r *DockerComposeRunner) Execute(env *config.Environment) error {
	cmd := r.Cmd

	// steps: run each step as a separate docker compose exec
	if len(cmd.Steps) > 0 {
		return r.executeSteps(env, cmd.Steps)
	}

	// script/script_file in docker context: not supported natively;
	// fall back to local execution as a convenience.
	if cmd.Script != "" || cmd.ScriptFile != "" {
		local := &LocalRunner{Cmd: r.Cmd, Opts: r.Opts}
		return local.Execute(env)
	}

	// Auto-detect if container is running → switch run to exec
	r.autoDetectComposeMethod()

	var args []string

	// Profiles
	args = append(args, r.composeProfiles()...)

	// Detected project name override
	if r.detectedProject != "" {
		args = append(args, "--project-name", r.detectedProject)
	}

	// Method (run/exec/up)
	args = append(args, r.Cmd.Compose.Method)

	// Arguments
	args = append(args, r.composeArguments(env)...)

	return execCompose(env, r.Opts.Config, args)
}

// executeSteps runs each step as a separate docker compose exec command.
// Does NOT mutate r.Cmd; constructs args independently per command.
func (r *DockerComposeRunner) executeSteps(env *config.Environment, steps []config.ProvisionItem) error {
	// Ensure container state is detected once up front.
	r.autoDetectComposeMethod()

	for i, step := range steps {
		label := step.Step
		if label == "" {
			label = fmt.Sprintf("step %d", i+1)
		}
		// Same branch, same wording as LocalRunner.executeSteps — these two loops are
		// otherwise line-for-line identical, and the acceptance criterion for TASK-083 is
		// that they take the same branch.
		if step.IsInert() {
			fmt.Printf("  → %s\n", label)
			fmt.Printf("    ⚠ %s\n", config.InertStepMessage)
			continue
		}
		// Same fall-through as LocalRunner.executeSteps, for the same reason (TASK-089):
		// a note must not swallow the step's work.
		noted := step.Note != ""
		if noted {
			fmt.Printf("  → %s: %s\n", label, step.Note)
		}
		cmds := step.RunCommands()
		if len(cmds) == 0 && step.Raw != "" {
			cmds = []string{step.Raw}
		}
		if len(cmds) == 0 {
			continue
		}
		if !noted {
			fmt.Printf("  → %s\n", label)
		}
		for _, c := range cmds {
			c = strings.TrimSpace(c)
			if c == "" {
				continue
			}
			args := r.buildStepArgs(env, c)
			if err := execCompose(env, r.Opts.Config, args); err != nil {
				return fmt.Errorf("step %q failed: %w", label, err)
			}
		}
	}
	return nil
}

// buildStepArgs builds docker compose exec args for a single command string.
// Does NOT mutate r.Cmd state.
func (r *DockerComposeRunner) buildStepArgs(env *config.Environment, cmd string) []string {
	var args []string
	if r.detectedProject != "" {
		args = append(args, "--project-name", r.detectedProject)
	}
	// Always use exec for steps (container must be running)
	args = append(args, "exec")
	// User / workdir overrides
	if r.Cmd.User != "" {
		args = append(args, "--user", r.Cmd.User)
	}
	if r.Cmd.Workdir != "" {
		args = append(args, "--workdir", r.Cmd.Workdir)
	}
	// Service
	args = append(args, r.Cmd.Service)
	// Command
	if r.Cmd.Shell {
		args = append(args, "sh", "-c", cmd)
	} else {
		args = append(args, dvaexec.SplitCommand(cmd)...)
	}
	return args
}

func (r *DockerComposeRunner) composeProfiles() []string {
	if len(r.Cmd.Compose.Profiles) == 0 {
		return nil
	}

	// When using profiles, method must be "up" and command is cleared
	r.Cmd.Compose.Method = "up"
	r.Cmd.Command = ""
	r.Cmd.Compose.RunOptions = nil

	var args []string
	for _, p := range r.Cmd.Compose.Profiles {
		args = append(args, "--profile", p)
	}
	return args
}

func (r *DockerComposeRunner) composeArguments(env *config.Environment) []string {
	var argv []string

	// Run options
	argv = append(argv, r.Cmd.Compose.RunOptions...)

	method := r.Cmd.Compose.Method
	if method == "run" {
		// Add runtime env vars
		argv = append(argv, r.runVars(env)...)
		// Publish ports
		for _, p := range r.Opts.Publish {
			argv = append(argv, "--publish="+p)
		}
		argv = append(argv, "--rm")
	}

	// User and workdir
	if r.Cmd.User != "" {
		argv = append(argv, "--user", r.Cmd.User)
	}
	if r.Cmd.Workdir != "" {
		argv = append(argv, "--workdir", r.Cmd.Workdir)
	}

	// Service name
	argv = append(argv, r.Cmd.Service)

	// Command and args
	cmd := strings.TrimSpace(r.Cmd.Command)
	if cmd != "" {
		cArgs := commandArgs(r.Cmd)
		if r.Cmd.Shell {
			// Wrap with sh -c for shell mode
			fullCmd := cmd
			if len(cArgs) > 0 {
				fullCmd = fullCmd + " " + strings.Join(cArgs, " ")
			}
			argv = append(argv, "sh", "-c", fullCmd)
		} else {
			argv = append(argv, dvaexec.SplitCommand(cmd)...)
			argv = append(argv, cArgs...)
		}
	}

	return argv
}

func (r *DockerComposeRunner) runVars(env *config.Environment) []string {
	// Pass through non-DVA_ vars that are already set in the OS environment
	var args []string
	for k, v := range env.Vars {
		if strings.HasPrefix(k, "DVA_") {
			continue
		}
		if os.Getenv(k) == "" {
			continue
		}
		args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
	}
	return args
}

func (r *DockerComposeRunner) autoDetectComposeMethod() {
	if r.Cmd.Compose.Method != "run" {
		return
	}
	if r.Cmd.Service == "" {
		return
	}

	// Check if container is already running
	project := serviceRunningProject(r.Cmd.Service)
	if project != "" {
		r.Cmd.Compose.Method = "exec"
		r.detectedProject = project
		// Remove --rm from run_options
		var filtered []string
		for _, o := range r.Cmd.Compose.RunOptions {
			if !strings.Contains(o, "--rm") {
				filtered = append(filtered, o)
			}
		}
		r.Cmd.Compose.RunOptions = filtered
	}
}

// serviceRunningProject checks if a service has a running container and returns
// its Docker Compose project name. Empty string = not running.
func serviceRunningProject(service string) string {
	// docker compose ps --filter "status=running" --format "{{.Project}}" for the service
	out, err := dvaexec.ExecSubprocessOutput("docker", "compose", "ps", "--filter", "status=running", "--format", "{{.Project}}", service)
	if err != nil || out == "" {
		return ""
	}
	// Return first line (project name)
	lines := strings.Split(out, "\n")
	if len(lines) > 0 && lines[0] != "" {
		return strings.TrimSpace(lines[0])
	}
	return ""
}
