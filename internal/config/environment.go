package config

import (
	"fmt"
	"maps"
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

// CfgDir returns the config directory.
func (e *Environment) CfgDir() string {
	return e.cfgDir
}

// NewEnvironment creates a new Environment with merged variables from config.
func NewEnvironment(defaultVars map[string]string, workDir, cfgDir string) *Environment {
	env := &Environment{
		Vars:    make(map[string]string),
		workDir: workDir,
		cfgDir:  cfgDir,
	}

	// Set special variables
	env.Vars[EnvRuntimeOS] = runtime.GOOS
	if rel, err := filepath.Rel(cfgDir, workDir); err == nil {
		env.Vars[EnvRuntimeWorkDirRelPath] = rel
	}
	if u, err := user.Current(); err == nil {
		env.Vars[EnvRuntimeCurrentUser] = u.Username
		env.Vars[EnvRuntimeCurrentUID] = u.Uid
	}

	// Merge default vars from config
	env.MergeVars(defaultVars)

	return env
}

// Clone returns a copy of e with its own Vars map.
//
// MergeVars mutates in place, so anything that merges entry-scoped declarations needs one of
// these per entry: without it the first entry's runners.<name>.env would still be set while
// the second entry runs. Both the orchestrator and `dva build` walk entries that way, which
// is why the copy lives on the type rather than beside one of them.
func (e *Environment) Clone() *Environment {
	clone := NewEnvironment(nil, e.workDir, e.cfgDir)
	maps.Copy(clone.Vars, e.Vars)
	return clone
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

// WithHookDepth returns a copy of e whose Vars carry the hook recursion guard, so that
// processes started from a hook step inherit it and every other child does not.
//
// It must copy rather than mutate: cli.loadEnv caches one *Environment in a package global
// and hands the same pointer to the hook executor and to the built-in command path, so
// setting the key on e would put it straight back into the ExecReplace'd target's
// environment — the leak this exists to close.
func (e *Environment) WithHookDepth() *Environment {
	c := *e
	c.Vars = make(map[string]string, len(e.Vars)+1)
	maps.Copy(c.Vars, e.Vars)
	c.Vars[EnvHookDepthKey] = "1"
	return &c
}

// EnvSlice returns environment variables as KEY=VALUE slice for exec.
// Config vars override OS environment variables with the same key.
func (e *Environment) EnvSlice() []string {
	osEnv := os.Environ()
	result := make([]string, 0, len(osEnv)+len(e.Vars))

	// Pass through OS env vars, skipping keys that config will override
	for _, kv := range osEnv {
		key, _, _ := strings.Cut(kv, "=")
		// The hook recursion guard is this process's state. cli.wrapWithHooks sets it with
		// os.Setenv and cleans up with a defer, but the hookable commands end in
		// syscall.Exec, which replaces the process image so that defer can never run — the
		// value would otherwise be inherited by docker/kubectl and by everything they spawn,
		// and a nested dva down there would read it and silently skip its own hooks.
		//
		// Dropping it here rather than at each ExecReplace call site makes not-leaking the
		// default. Hook steps, which are the one consumer that genuinely needs the guard to
		// cross a process boundary, opt back in through WithHookDepth: Vars are appended
		// below and are not subject to this filter.
		if key == EnvHookDepthKey {
			continue
		}
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
