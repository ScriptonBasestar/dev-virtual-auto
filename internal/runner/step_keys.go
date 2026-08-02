package runner

import (
	"fmt"
	"strings"

	"github.com/ScriptonBasestar/dva/internal/config"
	dvaexec "github.com/ScriptonBasestar/dva/internal/exec"
)

// A `ProvisionItem` carries seven payload keys, but only one of them — `run:` — means something
// different depending on who executes it: a local shell for LocalRunner, `compose exec <service>`
// for DockerComposeRunner. The other five are runner-independent. `echo:` prints, `cmd:` is a
// shell command, and the three compose keys invoke compose no matter which runner reached them.
//
// The runners used to implement none of the five, so an interaction step written with any of them
// produced zero bytes and exit 0 while the identical item under `provision:` worked (TASK-085).
// These two functions are the runner-independent half, factored out so the two runners cannot
// disagree about it — that disagreement, repeated key by key, is what produced TASK-085,
// TASK-086, TASK-089 and TASK-091.
//
// `internal/cli/provision.go:134-201` is the reference implementation. Its *ordering* is part of
// the contract and is reproduced exactly: the compose keys short-circuit the whole step, then
// `run:`, then `echo:`, then `cmd:`.

// hasStepKeys reports whether the item carries any of the five runner-independent payloads.
// Both step loops need this because "no `run:` commands" is not the same as "no work" — that
// conflation is precisely what made these keys silent.
func hasStepKeys(step config.ProvisionItem) bool {
	return len(step.ComposeUp) > 0 ||
		step.ComposeExec != "" ||
		step.ComposeRun != "" ||
		step.Echo != "" ||
		step.Cmd != ""
}

// runComposeStepKeys executes `compose_up:`, `compose_exec:` or `compose_run:`.
//
// A true first return means the step is finished and the caller must not execute anything else
// for it. provision.go returns immediately after each of these three, so a step that says "start
// postgres" does not additionally fall through to some other key on the same item.
func runComposeStepKeys(env *config.Environment, cfg *config.Config, step config.ProvisionItem) (bool, error) {
	var args []string
	switch {
	case len(step.ComposeUp) > 0:
		args = append([]string{"up", "-d"}, step.ComposeUp...)
	case step.ComposeExec != "":
		args = append([]string{"exec"}, strings.Fields(step.ComposeExec)...)
	case step.ComposeRun != "":
		args = append([]string{"run"}, strings.Fields(step.ComposeRun)...)
	default:
		return false, nil
	}

	// execComposeStep, never execCompose. Both callers are step loops, and execCompose ends in
	// syscall.Exec, which would make every later step unreachable — silently, with exit 0
	// (TASK-091).
	//
	// No project override: these keys are reached from provision, which runs no container
	// detection, so the config's project_name is the only name there is.
	return true, execComposeStep(env, cfg, "", args)
}

// runLegacyStepKeys executes `echo:` and then `cmd:`, matching provision.go's order. Neither key
// short-circuits: an item may carry both, and provision.go runs both.
func runLegacyStepKeys(env *config.Environment, step config.ProvisionItem) error {
	if step.Echo != "" {
		fmt.Printf("    %s\n", step.Echo)
	}
	if step.Cmd == "" {
		return nil
	}
	fmt.Printf("    $ %s\n", step.Cmd)
	// shell=true is what makes this equivalent to provision.go's runShellCommand (`sh -c`).
	return dvaexec.ExecSequential(env, []string{step.Cmd}, true)
}
