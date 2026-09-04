package config

import (
	"fmt"
	"maps"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
)

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

// interpolateWith expands `$VAR`, `${VAR}`, `${VAR:-default}` and `${VAR-default}` using
// lookup, leaving a reference written literally when lookup reports the name is out of scope
// and the reference carries no default.
//
// Leaving the match in place rather than emptying it is what lets ApplyEnvFiles run a repair
// pass over the merged result: an unresolved reference is still recognizable as one on the
// next pass. It is also why MergeVars resolves its own batch instead of deferring — only the
// `env_file` path has such a pass, so anywhere else an unresolved reference would reach a
// child process verbatim and stay that way.
//
// The default operators follow POSIX shell: `:-` substitutes when the variable is unset or
// empty, `-` only when it is unset. The default text is itself interpolated, so
// `${A:-${B}:5432}` nests. This used to be a single regex with an optional closing brace;
// that regex matched `${POSTGRES_USER` out of `${POSTGRES_USER:-gorisa}` and left
// `:-gorisa}` behind, so a *set* variable produced `gorisa:-gorisa}` (TASK-303).
func interpolateWith(value string, lookup func(name string) (string, bool)) string {
	if !strings.Contains(value, "$") {
		return value
	}
	var b strings.Builder
	b.Grow(len(value))
	for i := 0; i < len(value); {
		if value[i] != '$' || i+1 >= len(value) {
			b.WriteByte(value[i])
			i++
			continue
		}
		if value[i+1] == '{' {
			ref, end, ok := parseBracedRef(value, i)
			if !ok {
				// Malformed (no closing brace): keep the text literally.
				b.WriteString(value[i:])
				break
			}
			b.WriteString(ref.expand(lookup))
			i = end
			continue
		}
		name := scanVarName(value[i+1:])
		if name == "" {
			b.WriteByte('$')
			i++
			continue
		}
		if v, ok := lookup(name); ok {
			b.WriteString(v)
		} else {
			b.WriteString(value[i : i+1+len(name)])
		}
		i += 1 + len(name)
	}
	return b.String()
}

// varRef is one parsed `${...}` reference.
type varRef struct {
	name    string
	op      string // "", ":-" or "-"
	def     string // raw default text, interpolated on use
	literal string // the source text, returned when unresolved and without a default
}

func (r varRef) expand(lookup func(name string) (string, bool)) string {
	v, ok := lookup(r.name)
	switch r.op {
	case ":-":
		if !ok || v == "" {
			return interpolateWith(r.def, lookup)
		}
		return v
	case "-":
		if !ok {
			return interpolateWith(r.def, lookup)
		}
		return v
	}
	if ok {
		return v
	}
	return r.literal
}

// scanVarName returns the leading identifier of s, or "" when s does not start with one.
func scanVarName(s string) string {
	n := 0
	for n < len(s) && isVarByte(s[n], n == 0) {
		n++
	}
	return s[:n]
}

func isVarByte(c byte, first bool) bool {
	if c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
		return true
	}
	return !first && c >= '0' && c <= '9'
}

// parseBracedRef parses the `${...}` reference beginning at value[start] (which must be
// `$`). It returns the reference and the index just past the closing brace. ok is false
// when the reference has no matching closing brace or no valid name; braces inside the
// default text nest, so `${A:-${B}}` closes at the second `}`.
func parseBracedRef(value string, start int) (varRef, int, bool) {
	i := start + 2 // past "${"
	name := scanVarName(value[i:])
	if name == "" {
		return varRef{}, 0, false
	}
	i += len(name)
	if i >= len(value) {
		return varRef{}, 0, false
	}
	ref := varRef{name: name}
	switch {
	case value[i] == '}':
		ref.literal = value[start : i+1]
		return ref, i + 1, true
	case strings.HasPrefix(value[i:], ":-"):
		ref.op = ":-"
		i += 2
	case value[i] == '-':
		ref.op = "-"
		i++
	default:
		// `${NAME` followed by something that is neither `}` nor a supported operator
		// (e.g. `${A:+x}`, `${A:?x}`, `${A.b}`) is not a reference we expand.
		return varRef{}, 0, false
	}
	depth := 0
	for j := i; j < len(value); j++ {
		switch {
		case value[j] == '$' && j+1 < len(value) && value[j+1] == '{':
			depth++
		case value[j] == '}' && depth > 0:
			depth--
		case value[j] == '}':
			ref.def = value[i:j]
			ref.literal = value[start : j+1]
			return ref, j + 1, true
		}
	}
	return varRef{}, 0, false
}

// findVarRefs returns the source text of every variable reference in value that this
// package's expander recognizes, in order of appearance. It is the read-only counterpart of
// interpolateWith, used by validation to report references that survived expansion.
func findVarRefs(value string) []string {
	var refs []string
	for i := 0; i < len(value); {
		if value[i] != '$' || i+1 >= len(value) {
			i++
			continue
		}
		if value[i+1] == '{' {
			ref, end, ok := parseBracedRef(value, i)
			if !ok {
				i++
				continue
			}
			refs = append(refs, ref.literal)
			i = end
			continue
		}
		if name := scanVarName(value[i+1:]); name != "" {
			refs = append(refs, value[i:i+1+len(name)])
			i += 1 + len(name)
			continue
		}
		i++
	}
	return refs
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
