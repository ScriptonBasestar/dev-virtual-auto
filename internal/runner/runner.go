package runner

import (
	"fmt"
	"strings"

	"github.com/ScriptonBasestar/dva/internal/config"
	dvaexec "github.com/ScriptonBasestar/dva/internal/exec"
	"github.com/ScriptonBasestar/dva/internal/output"
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
	Config  *config.Config
}

// Explain prints the execution plan without running anything.
//
// It returns an error only from the JSON branch, where output.PrintJSON can fail at the write
// (a full filesystem under stdout, TASK-114). The text branch's dozen fmt.Print* calls return
// errors too and ignore them, and that is deliberate rather than a copy of this function's old
// shape: that branch is human-facing, so a closed downstream pipe already kills the process via
// SIGPIPE — a silent success needs the write to succeed-and-be-lost, which a tty or a regular
// file does not produce. Propagating fmt errors here would widen this change into every caller
// of every print the text plan makes, for a failure mode that is already noisy. TASK-121.
func Explain(cmd *ResolvedCommand, jsonOutput bool) error {
	runner := DetectRunnerType(cmd)

	if jsonOutput {
		plan := map[string]any{
			"command":     cmd.Command,
			"description": cmd.Description,
			"runner":      runner,
			"service":     cmd.Service,
			"pod":         cmd.Pod,
			"shell_mode":  cmd.Shell,
			"environment": cmd.Environment,
			// The effective arguments, not cmd.Argv: default_args is passed to every runner
			// through commandArgs when Argv is empty, so reporting Argv alone described a
			// plan the exec would not follow. That gap is why TASK-101 stayed invisible to
			// anyone checking `dva run rails console --explain` by hand.
			"arguments": commandArgs(cmd),
		}
		if cmd.Service != "" {
			plan["compose_method"] = cmd.Compose.Method
		}
		return output.PrintJSON(plan)
	}

	fmt.Println("=== Command Execution Plan ===")
	// A steps-only interaction has no single Command:, and a blank `Command:` line invites the
	// reading that nothing will run. State that the interaction is step-driven instead; the steps
	// themselves are listed below (TASK-146).
	switch {
	case cmd.Command != "":
		fmt.Printf("Command: %s\n", cmd.Command)
	case len(cmd.Steps) > 0:
		fmt.Println("Command: (step-driven — see Steps below)")
	default:
		fmt.Println("Command:")
	}
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
	// Same source as the exec path — see the JSON branch above. Annotated when they came from
	// default_args rather than from the invocation, because those are precisely the arguments
	// the user did not type and would otherwise have no way to account for. TASK-101.
	if args := commandArgs(cmd); len(args) > 0 {
		origin := ""
		if len(cmd.Argv) == 0 {
			origin = "  (from default_args)"
		}
		fmt.Printf("Arguments: %s%s\n", strings.Join(args, " "), origin)
	}
	fmt.Printf("Shell Mode: %v\n", cmd.Shell)
	if len(cmd.Environment) > 0 {
		fmt.Println("Environment Variables:")
		for k, v := range cmd.Environment {
			fmt.Printf("  %s=%s\n", k, v)
		}
	}
	if len(cmd.Steps) > 0 {
		explainSteps(cmd)
	}
	return nil
}

// explainSteps renders a step-driven interaction's plan without running anything, mirroring the
// labels runStepLoop prints on the exec path (TASK-146). A steps-only interaction has no single
// Command:, so the plan used to print a blank Command: line and name no step — the one tool for
// checking what is about to happen hid the declared work. Steps run through the shared loop on
// every runner, so this rendering is runner-independent; Explain must not re-fork what execution
// unified. A note renders as `  → label: note`, the same line the executing path prints.
func explainSteps(cmd *ResolvedCommand) {
	fmt.Println("Steps:")
	for i := range cmd.Steps {
		step := &cmd.Steps[i]
		label := step.Step
		if label == "" {
			label = fmt.Sprintf("step %d", i+1)
		}
		if step.IsInert() {
			fmt.Printf("  → %s\n", label)
			fmt.Printf("    ⚠ %s\n", config.InertStepMessage)
			continue
		}
		if step.Note != "" {
			fmt.Printf("  → %s: %s\n", label, step.Note)
		} else {
			fmt.Printf("  → %s\n", label)
		}
		// What the step will execute, mirroring runStepLoop's dispatch exactly. A compose key
		// short-circuits the whole step — runComposeStepKeys returns handled and the loop continues,
		// skipping run:/echo:/cmd: — and the three compose keys are mutually exclusive
		// (runComposeStepKeys is a switch, first one wins). So a step carrying compose_up AND run:
		// runs only compose_up; the dry-run must say the same, or it misleads the one user who
		// reached for it because they do not trust what will run.
		runCmds := step.RunCommands()
		if len(runCmds) == 0 && step.Raw != "" {
			runCmds = []string{step.Raw}
		}
		switch {
		case len(step.ComposeUp) > 0:
			fmt.Printf("    compose up: %s\n", strings.Join(step.ComposeUp, " "))
		case step.ComposeExec != "":
			fmt.Printf("    compose exec: %s\n", step.ComposeExec)
		case step.ComposeRun != "":
			fmt.Printf("    compose run: %s\n", step.ComposeRun)
		default:
			for _, c := range runCmds {
				fmt.Printf("    run: %s\n", c)
			}
			if step.Echo != "" {
				fmt.Printf("    echo: %s\n", step.Echo)
			}
			if step.Cmd != "" {
				fmt.Printf("    cmd: %s\n", step.Cmd)
			}
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
