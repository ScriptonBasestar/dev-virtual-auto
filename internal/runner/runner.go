package runner

import (
	"fmt"
	"strings"

	"github.com/ScriptonBasestar/dva/internal/config"
	dvaexec "github.com/ScriptonBasestar/dva/internal/exec"
)

// Runner is the interface for command execution strategies.
type Runner interface {
	Execute(env *config.Environment) error
}

// NewRunner creates the appropriate runner based on the resolved command.
func NewRunner(cmd *ResolvedCommand, opts RunOptions) Runner {
	if cmd.RunnerName != "" {
		// Explicit runner specified
		switch strings.ToLower(cmd.RunnerName) {
		case "docker_compose":
			return &DockerComposeRunner{Cmd: cmd, Opts: opts}
		case "kubectl":
			return &KubectlRunner{Cmd: cmd, Opts: opts}
		case "local":
			return &LocalRunner{Cmd: cmd, Opts: opts}
		default:
			// Try camelCase to snake_case matching
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

// RunOptions holds runtime options for command execution.
type RunOptions struct {
	Publish []string
	Explain bool
}

// Explain prints the execution plan without running anything.
func Explain(cmd *ResolvedCommand) {
	fmt.Println("=== Command Execution Plan ===")
	fmt.Printf("Command: %s\n", cmd.Command)
	if cmd.Description != "" {
		fmt.Printf("Description: %s\n", cmd.Description)
	}
	runner := "LocalRunner"
	if cmd.Service != "" {
		runner = "DockerComposeRunner"
	} else if cmd.Pod != "" {
		runner = "KubectlRunner"
	}
	if cmd.RunnerName != "" {
		runner = cmd.RunnerName
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
