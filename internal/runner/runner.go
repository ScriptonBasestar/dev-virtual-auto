package runner

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ScriptonBasestar/dva/internal/config"
	dvaexec "github.com/ScriptonBasestar/dva/internal/exec"
)

// Runner type constants.
const (
	RunnerDockerCompose = "docker_compose"
	RunnerKubectl       = "kubectl"
	RunnerLocal         = "local"
)

// Runner type display names.
const (
	RunnerNameDockerCompose = "DockerCompose"
	RunnerNameKubectl       = "Kubectl"
	RunnerNameLocal         = "Local"
)

// Runner is the interface for command execution strategies.
type Runner interface {
	Execute(env *config.Environment) error
}

// NewRunner creates the appropriate runner based on the resolved command.
func NewRunner(cmd *ResolvedCommand, opts RunOptions) Runner {
	if cmd.RunnerName != "" {
		switch strings.ToLower(cmd.RunnerName) {
		case RunnerDockerCompose:
			return &DockerComposeRunner{Cmd: cmd, Opts: opts}
		case RunnerKubectl:
			return &KubectlRunner{Cmd: cmd, Opts: opts}
		case RunnerLocal:
			return &LocalRunner{Cmd: cmd, Opts: opts}
		default:
			return &DockerComposeRunner{Cmd: cmd, Opts: opts}
		}
	}

	if cmd.Service != "" {
		return &DockerComposeRunner{Cmd: cmd, Opts: opts}
	}
	if cmd.Pod != "" {
		return &KubectlRunner{Cmd: cmd, Opts: opts}
	}
	return &LocalRunner{Cmd: cmd, Opts: opts}
}

// DetectRunnerType returns the display name for the runner that would handle this command.
func DetectRunnerType(cmd *ResolvedCommand) string {
	if cmd.RunnerName != "" {
		return cmd.RunnerName
	}
	if cmd.Service != "" {
		return RunnerNameDockerCompose
	}
	if cmd.Pod != "" {
		return RunnerNameKubectl
	}
	return RunnerNameLocal
}

// RunOptions holds runtime options for command execution.
type RunOptions struct {
	Publish []string
	Explain bool
}

// Explain prints the execution plan without running anything.
func Explain(cmd *ResolvedCommand, jsonOutput bool) {
	runner := DetectRunnerType(cmd)

	if jsonOutput {
		plan := map[string]interface{}{
			"command":     cmd.Command,
			"description": cmd.Description,
			"runner":      runner,
			"service":     cmd.Service,
			"pod":         cmd.Pod,
			"shell_mode":  cmd.Shell,
			"environment": cmd.Environment,
			"arguments":   cmd.Argv,
		}
		if cmd.Service != "" {
			plan["compose_method"] = cmd.Compose.Method
		}
		data, _ := json.MarshalIndent(plan, "", "  ")
		fmt.Println(string(data))
		return
	}

	fmt.Println("=== Command Execution Plan ===")
	fmt.Printf("Command: %s\n", cmd.Command)
	if cmd.Description != "" {
		fmt.Printf("Description: %s\n", cmd.Description)
	}
	fmt.Printf("Runner: %s\n", runner)
	if cmd.Service != "" {
		fmt.Printf("Service: %s\n", cmd.Service)
		fmt.Printf("Compose Method: %s\n", cmd.Compose.Method)
	}
	if cmd.Pod != "" {
		fmt.Printf("Pod: %s\n", cmd.Pod)
	}
	if len(cmd.Argv) > 0 {
		fmt.Printf("Arguments: %s\n", strings.Join(cmd.Argv, " "))
	}
	fmt.Printf("Shell Mode: %v\n", cmd.Shell)
	if len(cmd.Environment) > 0 {
		fmt.Println("Environment Variables:")
		for k, v := range cmd.Environment {
			fmt.Printf("  %s=%s\n", k, v)
		}
	}
}

// commandArgs returns the effective arguments for a command.
func commandArgs(cmd *ResolvedCommand) []string {
	if len(cmd.Argv) > 0 {
		return cmd.Argv
	}
	if cmd.DefaultArgs != "" {
		return splitCommand(cmd.DefaultArgs)
	}
	return nil
}

// splitCommand splits a command string respecting shell quoting.
func splitCommand(s string) []string {
	return dvaexec.SplitCommand(s)
}
