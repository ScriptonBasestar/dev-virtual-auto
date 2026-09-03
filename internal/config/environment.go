package config

import (
	"fmt"
	"maps"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
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
//
// The batch is resolved by dependency, not by map iteration order, and that is the point of
// the shape below. `vars` reaches every one of this function's call sites as a map: dotenv
// parsing discards declaration order, and a YAML `environment:` block never had one. The
// previous form interpolated each value while ranging that map, so Go's randomized iteration
// decided whether a reference to a sibling in the same batch saw the sibling's new value or
// its old one — the same `dva.yml` and the same `.env` could hand a child process two
// different environments on two runs (TASK-277).
//
// Resolving by dependency removes the ordering question rather than answering it
// arbitrarily: a value referencing a sibling always sees that sibling's merged value,
// whichever order the range happens to visit them in. A reference that closes a cycle,
// including a key referring to itself, falls back to the pre-merge environment rather than
// recursing into the entry being computed. The top-level walk is sorted only so that a
// genuine mutual cycle, where no starting point is more correct than another, still lands
// on the same answer every run.
//
// The OS check comes first and takes the whole entry, exactly as it did before. A key the OS
// already defines never has its declared value looked at, so it is also never resolved on a
// sibling's behalf — a batch's `${K}` and its final `e.Vars[K]` cannot disagree. That is also
// why `PATH=${PATH}:/x` in a dotenv file is not the case the cycle guard rescues: PATH is in
// the OS environment, so the declaration is discarded before the guard is reachable. The
// guard covers a self-reference to a key the OS does *not* define, and
// TestMergeVarsOSEnvShadowsSelfReferentialDeclaration pins the shadowed half.
//
// Order *between* batches is unchanged and still carries meaning: an earlier file's derived
// value is resolved when that file merges, so a later file redefining the source does not
// retroactively rewrite it. That is TASK-277's semantics (A), the reading
// TestLoadEnvFileKeepsSuccessfulPrecedence already asserted before this function could
// honor it reliably.
func (e *Environment) MergeVars(vars map[string]string) {
	resolved := make(map[string]string, len(vars))
	resolving := make(map[string]bool, len(vars))

	var resolve func(key string) string
	resolve = func(key string) string {
		if v, ok := resolved[key]; ok {
			return v
		}
		if envVal, ok := os.LookupEnv(key); ok {
			resolved[key] = envVal
			return envVal
		}
		resolving[key] = true
		v := interpolateWith(vars[key], func(name string) (string, bool) {
			// A sibling still being resolved means the reference closes a cycle; fall
			// through to the pre-merge scope instead of recursing into it.
			if _, inBatch := vars[name]; inBatch && !resolving[name] {
				return resolve(name), true
			}
			return e.lookup(name)
		})
		delete(resolving, key)
		resolved[key] = v
		return v
	}

	for _, key := range slices.Sorted(maps.Keys(vars)) {
		resolve(key)
	}
	// Written only once every value is resolved, so that a cyclic reference reads the
	// pre-merge value rather than a half-applied batch.
	maps.Copy(e.Vars, resolved)
}

// Interpolate replaces $VAR and ${VAR} in a string value.
func (e *Environment) Interpolate(value string) string {
	return interpolateWith(value, e.lookup)
}

// lookup resolves one name against the merged vars first and the OS environment second.
func (e *Environment) lookup(name string) (string, bool) {
	if v, ok := e.Vars[name]; ok {
		return v, true
	}
	return os.LookupEnv(name)
}

// interpolateWith expands $VAR and ${VAR} using lookup, leaving a reference written
// literally when lookup reports the name is out of scope.
//
// Leaving the match in place rather than emptying it is what lets ApplyEnvFiles run a repair
// pass over the merged result: an unresolved reference is still recognizable as one on the
// next pass. It is also why MergeVars resolves its own batch instead of deferring — only the
// `env_file` path has such a pass, so anywhere else an unresolved reference would reach a
// child process verbatim and stay that way.
func interpolateWith(value string, lookup func(name string) (string, bool)) string {
	return varRegex.ReplaceAllStringFunc(value, func(match string) string {
		name := varRegex.FindStringSubmatch(match)[1]
		if v, ok := lookup(name); ok {
			return v
		}
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
