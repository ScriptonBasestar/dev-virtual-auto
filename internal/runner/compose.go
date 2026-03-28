package runner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ScriptonBasestar/dva/internal/config"
	dvaexec "github.com/ScriptonBasestar/dva/internal/exec"
)

// execCompose runs a docker compose command with the config's compose settings.
func execCompose(env *config.Environment, cfg *config.Config, args []string) error {
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

	return dvaexec.ExecReplace(env, composeCmd, fullArgs, false)
}
