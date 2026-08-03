package runner

import (
	"fmt"
	"io"
	"os"
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

// planWriter accumulates the first write error across the text plan's prints, so the branch
// can report a failed write without an if-block after each of its 24 fmt calls.
//
// Sticky rather than fail-fast at the call site: once a write has failed the rest are skipped,
// and the error that surfaces is the first one, which is the one that says what went wrong.
// Writes go through an io.Writer captured at call time rather than fmt.Print*'s implicit
// os.Stdout, which is what lets a test point the plan at a writer that fails.
type planWriter struct {
	w   io.Writer
	err error
}

func (p *planWriter) printf(format string, a ...any) {
	if p.err != nil {
		return
	}
	_, p.err = fmt.Fprintf(p.w, format, a...)
}

func (p *planWriter) println(a ...any) {
	if p.err != nil {
		return
	}
	_, p.err = fmt.Fprintln(p.w, a...)
}

// Explain prints the execution plan without running anything.
//
// Both branches report a failed write. The JSON branch got this in TASK-121, where
// output.PrintJSON can fail at the write (a full filesystem under stdout, TASK-114); the text
// branch kept dropping its fmt errors until TASK-158, on the reasoning that a human-facing
// path is protected by SIGPIPE from a closed downstream pipe. That reasoning was wrong by
// omission. SIGPIPE covers EPIPE on fd 1 and nothing else: point stdout at a read-only
// descriptor and every write returns EBADF, which is a failure, not a signal. Measured on
// v0.1.44 before the fix — `--json` reported it and exited 1, the text branch delivered 0
// bytes and exited 0, which is the same silent success TASK-121 was filed to remove.
//
// The cost that argument was really protecting against — an error check after each print — is
// paid once by planWriter instead of 24 times.
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

	// os.Stdout is read here rather than held in a package variable so that a test swapping it
	// still reaches this branch, the same live resolution fmt.Print* had.
	p := &planWriter{w: os.Stdout}

	p.println("=== Command Execution Plan ===")
	// A steps-only interaction has no single Command:, and a blank `Command:` line invites the
	// reading that nothing will run. State that the interaction is step-driven instead; the steps
	// themselves are listed below (TASK-146).
	switch {
	case cmd.Command != "":
		p.printf("Command: %s\n", cmd.Command)
	case len(cmd.Steps) > 0:
		p.println("Command: (step-driven — see Steps below)")
	default:
		p.println("Command:")
	}
	if cmd.Description != "" {
		p.printf("Description: %s\n", cmd.Description)
	}
	p.printf("Runner: %s\n", runner)
	if cmd.Service != "" {
		p.printf("Service: %s\n", cmd.Service)
		p.printf("Compose Method: %s\n", cmd.Compose.Method)
	}
	if cmd.Pod != "" {
		p.printf("Pod: %s\n", cmd.Pod)
	}
	// Same source as the exec path — see the JSON branch above. Annotated when they came from
	// default_args rather than from the invocation, because those are precisely the arguments
	// the user did not type and would otherwise have no way to account for. TASK-101.
	if args := commandArgs(cmd); len(args) > 0 {
		origin := ""
		if len(cmd.Argv) == 0 {
			origin = "  (from default_args)"
		}
		p.printf("Arguments: %s%s\n", strings.Join(args, " "), origin)
	}
	p.printf("Shell Mode: %v\n", cmd.Shell)
	if len(cmd.Environment) > 0 {
		p.println("Environment Variables:")
		for k, v := range cmd.Environment {
			p.printf("  %s=%s\n", k, v)
		}
	}
	if len(cmd.Steps) > 0 {
		explainSteps(p, cmd)
	}
	return p.err
}

// explainSteps renders a step-driven interaction's plan without running anything, mirroring the
// labels runStepLoop prints on the exec path (TASK-146). A steps-only interaction has no single
// Command:, so the plan used to print a blank Command: line and name no step — the one tool for
// checking what is about to happen hid the declared work. Steps run through the shared loop on
// every runner, so this rendering is runner-independent; Explain must not re-fork what execution
// unified. A note renders as `  → label: note`, the same line the executing path prints.
func explainSteps(p *planWriter, cmd *ResolvedCommand) {
	p.println("Steps:")
	for i := range cmd.Steps {
		step := &cmd.Steps[i]
		label := step.Step
		if label == "" {
			label = fmt.Sprintf("step %d", i+1)
		}
		if step.IsInert() {
			p.printf("  → %s\n", label)
			p.printf("    ⚠ %s\n", config.InertStepMessage)
			continue
		}
		if step.Note != "" {
			p.printf("  → %s: %s\n", label, step.Note)
		} else {
			p.printf("  → %s\n", label)
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
			p.printf("    compose up: %s\n", strings.Join(step.ComposeUp, " "))
		case step.ComposeExec != "":
			p.printf("    compose exec: %s\n", step.ComposeExec)
		case step.ComposeRun != "":
			p.printf("    compose run: %s\n", step.ComposeRun)
		default:
			for _, c := range runCmds {
				p.printf("    run: %s\n", c)
			}
			if step.Echo != "" {
				p.printf("    echo: %s\n", step.Echo)
			}
			if step.Cmd != "" {
				p.printf("    cmd: %s\n", step.Cmd)
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
