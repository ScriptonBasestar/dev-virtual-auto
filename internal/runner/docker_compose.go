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

	return execCompose(env, args, r.Cmd.Shell)
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
