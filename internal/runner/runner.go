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
		// command is what formCommand would run. For every other form it is empty rather
		// than the inherited/scalar leftover — a list used to report its first line
		// (TASK-178), a scripted child used to report the parent's command (TASK-174), and
		// both described work the exec would not do. Sibling keys carry the real work.
		form := classifyForm(cmd)
		single := ""
		if form == formCommand {
			single = cmd.Command
		}
		plan := map[string]any{
			"command":     single,
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
		// Form-specific keys appear only when that form wins, so other interactions stay
		// byte-identical on the keys they already had.
		if form == formCommandList {
			plan["command_lines"] = cmd.CommandLines
		}
		if form == formScript {
			plan["script"] = cmd.Script
		}
		if form == formScriptFile {
			// Declared path, not FileDir-resolved: --explain is about the config the author
			// wrote, and an absolute rewrite would make two hosts disagree on the same file.
			plan["script_file"] = cmd.ScriptFile
		}
		if form == formSteps {
			// Labels only — the full step payload is for execution; JSON stays scannable.
			labels := make([]string, 0, len(cmd.Steps))
			for i := range cmd.Steps {
				labels = append(labels, cmd.Steps[i].Step)
			}
			plan["steps"] = labels
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
	//
	// The form comes from classifyForm rather than from the fields, so the plan cannot disagree
	// with the exec about which form wins. It did: a list `command:` leaves cmd.Command holding
	// its own first line, so `cmd.Command != ""` matched here and the plan named one line of
	// several — describing the very execution TASK-178 was filed to stop.
	// Nothing-to-run is said here so --explain stays a diagnosis tool (TASK-173): it must
	// not fail the way Execute now does. The message is the same shape as ErrNothingToRun.
	if !cmd.HasExecutionTarget() {
		p.println("Command: (nothing to run — add command:, script:, script_file:, steps:, service:, pod:, or default_args:)")
	} else {
		switch classifyForm(cmd) {
		case formCommand:
			if cmd.Command != "" {
				p.printf("Command: %s\n", cmd.Command)
			} else {
				p.println("Command:")
			}
		case formSteps:
			p.println("Command: (step-driven — see Steps below)")
		case formCommandList:
			p.printf("Command: (%d commands — see Commands below)\n", len(cmd.CommandLines))
		case formScriptFile:
			// Same vocabulary as the steps arm (TASK-146 / TASK-176): name the form, then
			// point at the section that carries the work — never leave Command: blank and
			// never print an inherited parent command the exec will not run (TASK-174).
			p.println("Command: (script_file-driven — see Script File below)")
		case formScript:
			p.println("Command: (script-driven — see Script below)")
		default:
			p.printf("Command: (unhandled %s)\n", classifyForm(cmd))
		}
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
	// Only the winning form's body is listed — classifyForm is the same pick the exec uses.
	switch classifyForm(cmd) {
	case formSteps:
		explainSteps(p, cmd)
	case formCommandList:
		explainCommandLines(p, cmd)
	case formScriptFile:
		explainScriptFile(p, cmd)
	case formScript:
		explainScript(p, cmd)
	}
	return p.err
}

// explainScriptFile names the declared script_file path (TASK-176). Declared, not absolute:
// the plan is about the config text, and path resolution is environment-dependent.
func explainScriptFile(p *planWriter, cmd *ResolvedCommand) {
	p.printf("Script File: %s\n", cmd.ScriptFile)
}

// explainScript prints an inline script. Full body, not a head: steps list every step, and
// truncating here would hide the work the plan exists to show. Indent matches explainSteps.
func explainScript(p *planWriter, cmd *ResolvedCommand) {
	p.println("Script:")
	for line := range strings.SplitSeq(cmd.Script, "\n") {
		p.printf("  → %s\n", line)
	}
}

// explainCommandLines lists every line of a list `command:`, in the arrow form explainSteps
// uses for steps — the two are the same sequence with and without labels, and giving them two
// vocabularies would suggest they run differently. They do not: every runner takes both one
// command at a time, in order, stopping at the first failure.
//
// Every line, not a truncated head: the whole reason this exists is that the plan used to show
// one line of several, and a plan that shows three of five is the same defect with a larger
// constant.
func explainCommandLines(p *planWriter, cmd *ResolvedCommand) {
	p.println("Commands:")
	for _, c := range cmd.CommandLines {
		p.printf("  → %s\n", c)
	}
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
