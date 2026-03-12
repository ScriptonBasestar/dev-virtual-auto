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
func execCompose(env *config.Environment, args []string, shell bool) error {
	cfg, err := config.Load(".")
	if err != nil {
		// If config not available, run without config
		return dvaexec.ExecReplace(env, "docker", append([]string{"compose"}, args...), false)
	}

	composeCmd := "docker"
	composeSubCmd := "compose"
	if cfg.Compose.Command != "" {
		parts := dvaexec.SplitCommand(cfg.Compose.Command)
		composeCmd = parts[0]
		if len(parts) > 1 {
			composeSubCmd = parts[1]
		}
	}

	var fullArgs []string
	fullArgs = append(fullArgs, composeSubCmd)

	// Compose files
	cfgDir := cfg.FileDir()
	for _, f := range cfg.Compose.Files {
		f = env.Interpolate(f)
		if !filepath.IsAbs(f) {
			f = filepath.Join(cfgDir, f)
		}
		fullArgs = append(fullArgs, "-f", f)
	}

	// Project name
	if cfg.Compose.ProjectName != "" {
		fullArgs = append(fullArgs, "--project-name", env.Interpolate(cfg.Compose.ProjectName))
	}

	// Kubectl namespace for compose
	if ns := cfg.Kubectl.Namespace; ns != "" {
		os.Setenv("KUBE_NAMESPACE", env.Interpolate(ns))
	}

	fullArgs = append(fullArgs, args...)

	if dvaexec.Debug {
		fmt.Fprintf(os.Stderr, "[debug] compose: %s %s\n", composeCmd, strings.Join(fullArgs, " "))
	}

	return dvaexec.ExecReplace(env, composeCmd, fullArgs, false)
}
