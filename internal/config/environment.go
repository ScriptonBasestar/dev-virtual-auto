package config

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

var varRegex = regexp.MustCompile(`\$\{?([a-zA-Z_][a-zA-Z0-9_]*)\}?`)

// Environment manages environment variables and interpolation.
type Environment struct {
	Vars    map[string]string
	workDir string
	cfgDir  string
}

// WorkDir returns the working directory.
func (e *Environment) WorkDir() string {
	return e.workDir
}

// NewEnvironment creates a new Environment with merged variables from config.
func NewEnvironment(defaultVars map[string]string, workDir, cfgDir string) *Environment {
	env := &Environment{
		Vars:    make(map[string]string),
		workDir: workDir,
		cfgDir:  cfgDir,
	}

	// Set special variables
	env.Vars["DVA_OS"] = runtime.GOOS
	if rel, err := filepath.Rel(cfgDir, workDir); err == nil {
		env.Vars["DVA_WORK_DIR_REL_PATH"] = rel
	}
	if u, err := user.Current(); err == nil {
		env.Vars["DVA_CURRENT_USER"] = u.Username
		env.Vars["DVA_CURRENT_UID"] = u.Uid
	}

	// Merge default vars from config
	env.MergeVars(defaultVars)

	return env
}

// MergeVars merges new variables. For each key, ENV takes priority,
// then the provided value (with interpolation).
func (e *Environment) MergeVars(vars map[string]string) {
	for k, v := range vars {
		if envVal, ok := os.LookupEnv(k); ok {
			e.Vars[k] = envVal
		} else {
			e.Vars[k] = e.Interpolate(v)
		}
	}
}

// Interpolate replaces $VAR and ${VAR} in a string value.
func (e *Environment) Interpolate(value string) string {
	return varRegex.ReplaceAllStringFunc(value, func(match string) string {
		varName := varRegex.FindStringSubmatch(match)[1]

		// Check our vars first
		if v, ok := e.Vars[varName]; ok {
			return v
		}
		// Then check OS env
		if v, ok := os.LookupEnv(varName); ok {
			return v
		}
		// Return original if not found
		return match
	})
}

// EnvSlice returns environment variables as KEY=VALUE slice for exec.
// Config vars override OS environment variables with the same key.
func (e *Environment) EnvSlice() []string {
	osEnv := os.Environ()
	result := make([]string, 0, len(osEnv)+len(e.Vars))

	// Pass through OS env vars, skipping keys that config will override
	for _, kv := range osEnv {
		key, _, _ := strings.Cut(kv, "=")
		if _, overridden := e.Vars[key]; !overridden {
			result = append(result, kv)
		}
	}

	// Append config vars (these take priority)
	for k, v := range e.Vars {
		result = append(result, fmt.Sprintf("%s=%s", k, v))
	}

	return result
}
