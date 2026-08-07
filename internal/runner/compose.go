package runner

import (
	"fmt"
	"os"
	"strings"

	"github.com/ScriptonBasestar/dva/internal/config"
	dvaexec "github.com/ScriptonBasestar/dva/internal/exec"
)

// composeArgv builds the compose command and its arguments from the config's compose
// settings. Split out from execCompose so the two execution strategies below cannot drift:
// they must differ only in how they hand off, never in what they run.
//
// projectOverride is the project name DockerComposeRunner detected from a running container;
// empty means none was detected. It replaces the config's project_name rather than being
// appended after it, because that is the value the running container actually belongs to and
// a compose invocation may name its project only once. Callers used to append their own
// --project-name to args, which left the flag in the argv twice and made the right value win
// only because docker takes the last occurrence — correct by argv ordering, which nothing
// enforced (TASK-132).
//
// When non-empty and config has no project_name:, this string is the sole --project-name
// source (TASK-163). Do not reduce callers' detectedProject field to a boolean.
func composeArgv(env *config.Environment, cfg *config.Config, projectOverride string, args []string) (string, []string, error) {
	// A nil cfg means the interaction runs outside a loaded project; dvaexec.ComposeArgv
	// then yields the plain `docker compose` default, which is what the old code did too.
	var cc *config.ComposePluginConfig
	var baseDir string
	if cfg != nil {
		cc = cfg.PrimaryComposeConfig()
		baseDir = cfg.FileDir()
	}

	if projectOverride != "" {
		// Copy: cfg is shared with everything else holding this config, and a detection
		// result from one interaction must not become another's declared project_name.
		// The shallow copy is enough — ComposeArgv only reads Files and Command, and
		// neither is written here.
		var override config.ComposePluginConfig
		if cc != nil {
			override = *cc
		}
		override.ProjectName = projectOverride
		cc = &override
	}

	composeCmd, fullArgs, err := dvaexec.ComposeArgv(env, cc, baseDir)
	if err != nil {
		return "", nil, err
	}
	fullArgs = append(fullArgs, args...)

	if dvaexec.Debug {
		fmt.Fprintf(os.Stderr, "[debug] compose: %s %s\n", composeCmd, strings.Join(fullArgs, " "))
	}

	return composeCmd, fullArgs, nil
}

// execCompose replaces the current process with a docker compose command. Correct for the
// single-command path, where handing over the tty and signals is the point — and only there:
// syscall.Exec does not return, so this can never be called in a loop.
func execCompose(env *config.Environment, cfg *config.Config, projectOverride string, args []string) error {
	composeCmd, fullArgs, err := composeArgv(env, cfg, projectOverride, args)
	if err != nil {
		return err
	}
	return dvaexec.ExecReplace(env, composeCmd, fullArgs, false)
}

// execComposeStep runs a docker compose command as a subprocess and waits for it, so the
// caller survives to run the next one. Used by the steps path: it used to call execCompose,
// which replaced the process on the first command and left every later step unreachable —
// silently, with exit 0. See TASK-091. LocalRunner has always drawn this same distinction.
func execComposeStep(env *config.Environment, cfg *config.Config, projectOverride string, args []string) error {
	composeCmd, fullArgs, err := composeArgv(env, cfg, projectOverride, args)
	if err != nil {
		return err
	}
	return dvaexec.ExecSubprocess(env, composeCmd, fullArgs, false)
}
