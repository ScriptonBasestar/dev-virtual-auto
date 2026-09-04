package config

import (
	"fmt"
	"strings"
)

// EnvFileOriginKind names which declaration file won the whole-replace merge of
// env_file. TASK-245 §5-1 admits only three of these as write targets; the rest
// exist so the bridge can say why it refused instead of guessing an owner.
type EnvFileOriginKind string

const (
	// EnvOriginUnknown means no file was recorded as the declaring origin. It is
	// the zero value on purpose: a Config assembled outside Load — in a test, or
	// by a future caller — must not inherit write permission by default.
	EnvOriginUnknown EnvFileOriginKind = ""
	// EnvOriginRoot is the discovered dva.yml.
	EnvOriginRoot EnvFileOriginKind = "root"
	// EnvOriginModule is a .sb/dva/<mod>.yml listed under modules:.
	EnvOriginModule EnvFileOriginKind = "module"
	// EnvOriginOverride is the sibling dva.override.yml.
	EnvOriginOverride EnvFileOriginKind = "override"
	// EnvOriginSubproject is an imported child config. Its owner, path anchor and
	// git repository can all differ from the parent session's, so the parent
	// cannot prove the §5-3 guarantees on its behalf.
	EnvOriginSubproject EnvFileOriginKind = "subproject"
)

// EnvFileOrigin identifies the single file whose env_file declaration survived
// the merge. env_file is replaced as a whole (config.go mergeFrom), never merged
// element-wise, so exactly one file owns the effective declaration and this is
// recoverable rather than ambiguous.
type EnvFileOrigin struct {
	Kind EnvFileOriginKind
	// Path is the declaring file as Load resolved it. It is diagnostic only —
	// the bridge anchors every path it touches at the root config directory
	// (§5-3), never at the origin's own directory.
	Path string
}

// Writable reports whether the bridge may mutate a target declared by this
// origin. Subproject and unknown origins are excluded, which narrows what may be
// written without narrowing what may be loaded.
func (o EnvFileOrigin) Writable() bool {
	switch o.Kind {
	case EnvOriginRoot, EnvOriginModule, EnvOriginOverride:
		return true
	default:
		return false
	}
}

// EnvFileOrigin returns the recorded declaring file for the effective env_file.
func (c *Config) EnvFileOrigin() EnvFileOrigin { return c.envFileOrigin }

// setEnvFileOrigin records src as the declaring file when decl actually carries a
// declaration. A merge source with no env_file leaves the previous winner in
// place, mirroring mergeFrom's `if other.EnvFile != nil` guard — otherwise a
// module that says nothing about env_file would erase the root's provenance.
func (c *Config) setEnvFileOrigin(decl any, kind EnvFileOriginKind, src string) {
	if decl == nil {
		return
	}
	c.envFileOrigin = EnvFileOrigin{Kind: kind, Path: src}
}

// EncryptedEnvEntries returns the effective entries that declare a sops_source,
// in declaration order.
func (c *Config) EncryptedEnvEntries() []EnvFileConfig {
	var out []EnvFileConfig
	for _, e := range c.AllEnvFileConfigs() {
		if e.Encrypted() {
			out = append(out, e)
		}
	}
	return out
}

// envFileSopsSites walks a raw env_file declaration and reports where sops_source
// appears, using the two shape positions a reader can point at in dva.yml.
//
// It reads the raw `any` rather than the normalized entries because the wrapper
// position has no normalized representation at all: normalizeEnvFileConfig
// flattens {files:…} into entries and would silently drop a wrapper-level key.
// Dropping it is exactly the failure mode TASK-245 §2-3 R1 forbids — a source
// that appears to claim several targets and is in fact ignored.
func envFileSopsSites(decl any) (entry bool, wrapper bool) {
	switch v := decl.(type) {
	case []any:
		for _, item := range v {
			if m, ok := item.(map[string]any); ok {
				if s, _ := m["sops_source"].(string); s != "" {
					entry = true
				}
			}
		}
	case map[string]any:
		if s, _ := v["sops_source"].(string); s != "" {
			wrapper = true
		}
		e, w := envFileSopsSites(v["files"])
		entry = entry || e
		wrapper = wrapper || w
	}
	return entry, wrapper
}

// validateEnvSourceDeclarations enforces every rule that can be decided from the
// declaration alone, so `dva config validate` and every other command reject the
// same configs the bridge would — running the bridge is not a precondition for
// seeing these errors (TASK-245 §7-1).
func validateEnvSourceDeclarations(c *Config) error {
	if _, wrapper := envFileSopsSites(c.EnvFile); wrapper {
		return fmt.Errorf("env_file: source_not_on_entry: sops_source belongs on a single entry object, not on the files wrapper")
	}
	return validateEnvSourceEntries(c.AllEnvFileConfigs())
}

// validateEnvSourceEntries closes TASK-245 §2-3 R2 through R5 on the effective
// entry list.
//
// Every rule here is conditioned on an encrypted entry taking part. A config that
// declares no sops_source keeps today's permissive handling of a repeated path,
// because tightening it would reject working configs for a feature they never
// opted into.
func validateEnvSourceEntries(entries []EnvFileConfig) error {
	targets := make(map[string]int, len(entries))
	for _, e := range entries {
		targets[e.Path]++
	}

	sources := make(map[string]bool, len(entries))
	for _, e := range entries {
		if !e.Encrypted() {
			continue
		}
		// R5 first: when one entry names the same file twice, reporting it as a
		// chain (R4) would describe two entries where there is one.
		if e.SopsSource == e.Path {
			return fmt.Errorf("env_file: source_is_target: entry %q declares the same file as path and sops_source", e.Path)
		}
		if targets[e.Path] > 1 {
			return fmt.Errorf("env_file: duplicate_env_target: %d entries declare target %q and one of them is encrypted", targets[e.Path], e.Path)
		}
		if sources[e.SopsSource] {
			return fmt.Errorf("env_file: duplicate_env_source: two entries declare sops_source %q", e.SopsSource)
		}
		sources[e.SopsSource] = true
		if targets[e.SopsSource] > 0 {
			return fmt.Errorf("env_file: env_source_is_target: %q is declared as another entry's target, so unsealing it would overwrite its own input", e.SopsSource)
		}
	}
	return nil
}

// DeclaredEncryptedTargets lists the declared target strings of every encrypted
// entry, for the selector diagnostics that have to tell a user what they could
// have named instead.
func (c *Config) DeclaredEncryptedTargets() []string {
	entries := c.EncryptedEnvEntries()
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Path)
	}
	return out
}

// JoinTargets renders a target list for a diagnostic. Declared strings only —
// never an expanded local absolute path (TASK-245 §7-2, TASK-247 §2).
func JoinTargets(targets []string) string { return strings.Join(targets, ", ") }
