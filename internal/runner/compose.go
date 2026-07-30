package runner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ScriptonBasestar/dva/internal/config"
	dvaexec "github.com/ScriptonBasestar/dva/internal/exec"
)

// composeArgv builds the compose command and its arguments from the config's compose
// settings. Split out from execCompose so the two execution strategies below cannot drift:
// they must differ only in how they hand off, never in what they run.
func composeArgv(env *config.Environment, cfg *config.Config, args []string) (string, []string) {
	composeCmd := "docker"
	var fullArgs []string
	fullArgs = append(fullArgs, "compose")

	if cfg != nil {
		cc := cfg.PrimaryComposeConfig()
		if cc != nil {
			if cc.Command != "" {
				parts := dvaexec.SplitCommand(cc.Command)
				composeCmd = parts[0]
				if len(parts) > 1 {
					fullArgs = parts[1:]
				}
			}

			cfgDir := cfg.FileDir()
			for _, f := range cc.Files {
				f = env.Interpolate(f)
				if !filepath.IsAbs(f) {
					f = filepath.Join(cfgDir, f)
				}
				fullArgs = append(fullArgs, "-f", f)
			}

			if cc.ProjectName != "" {
				fullArgs = append(fullArgs, "--project-name", env.Interpolate(cc.ProjectName))
			}
		}
	}

	fullArgs = append(fullArgs, args...)

	if dvaexec.Debug {
		fmt.Fprintf(os.Stderr, "[debug] compose: %s %s\n", composeCmd, strings.Join(fullArgs, " "))
	}

	return composeCmd, fullArgs
}

// execCompose replaces the current process with a docker compose command. Correct for the
// single-command path, where handing over the tty and signals is the point — and only there:
// syscall.Exec does not return, so this can never be called in a loop.
func execCompose(env *config.Environment, cfg *config.Config, args []string) error {
	composeCmd, fullArgs := composeArgv(env, cfg, args)
	return dvaexec.ExecReplace(env, composeCmd, fullArgs, false)
}

// execComposeStep runs a docker compose command as a subprocess and waits for it, so the
// caller survives to run the next one. Used by the steps path: it used to call execCompose,
// which replaced the process on the first command and left every later step unreachable —
// silently, with exit 0. See TASK-091. LocalRunner has always drawn this same distinction.
func execComposeStep(env *config.Environment, cfg *config.Config, args []string) error {
	composeCmd, fullArgs := composeArgv(env, cfg, args)
	return dvaexec.ExecSubprocess(env, composeCmd, fullArgs, false)
}
