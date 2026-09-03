package config

// EnvBridgeConfig is the env_bridge: root section (TASK-281 §3-1). Its presence
// switches on nothing by itself: each field is its own boolean gate, both
// default false, and a config that omits the section entirely is unaffected —
// omission and `env_bridge: {allow_seal: false, allow_show: false}` load
// identically.
type EnvBridgeConfig struct {
	AllowSeal bool `yaml:"allow_seal"`
	AllowShow bool `yaml:"allow_show"`
}

// EnvBridgeIntroducedVersion is the DVA release env_bridge shipped in
// (TASK-281 §3-1). A config that declares env_bridge must also declare
// `version:` at least this value, so an older DVA that would otherwise ignore
// the unknown top-level key converges on one clear "please upgrade" message
// instead of two silent failure modes.
const EnvBridgeIntroducedVersion = "0.1.48"

// EnvBridgeOriginKind names which file declared env_bridge.
//
// There is no Writable() here the way EnvFileOrigin has one: TASK-281 §3-2
// admits exactly one valid kind, full stop, so a caller only ever needs to ask
// "is this root".
type EnvBridgeOriginKind string

const (
	// EnvBridgeOriginUnknown means no file in this load declared env_bridge.
	EnvBridgeOriginUnknown EnvBridgeOriginKind = ""
	// EnvBridgeOriginRoot is the discovered dva.yml — the only valid origin.
	EnvBridgeOriginRoot EnvBridgeOriginKind = "root"
	// EnvBridgeOriginModule is a .sb/dva/<mod>.yml listed under modules:.
	EnvBridgeOriginModule EnvBridgeOriginKind = "module"
	// EnvBridgeOriginOverride is the sibling dva.override.yml.
	EnvBridgeOriginOverride EnvBridgeOriginKind = "override"
)

// EnvBridgeOrigin identifies the file whose env_bridge declaration this Config
// last observed while loading.
//
// Unlike EnvFileOrigin, this is not "which declaration won a merge" — env_bridge
// is never merged (see mergeFrom), so c.EnvBridge always holds the root's own
// value regardless of what a module or override wrote. This field instead
// answers a narrower question for the gate-origin check: did any non-root file
// also declare the section? Load() calls setEnvBridgeOrigin once per file it
// reads, in root → module → override order, so the last non-nil declaration
// wins here even though it never touches c.EnvBridge itself.
type EnvBridgeOrigin struct {
	Kind EnvBridgeOriginKind
	// Path is the declaring file as Load resolved it, for diagnostics only.
	Path string
}

// EnvBridgeOrigin returns the file that most recently declared env_bridge
// during this load, or the zero value when nothing did.
func (c *Config) EnvBridgeOrigin() EnvBridgeOrigin { return c.envBridgeOrigin }

// setEnvBridgeOrigin records src as having declared env_bridge, when decl is
// non-nil. A merge source with no env_bridge leaves the previously recorded
// origin in place — mirroring setEnvFileOrigin's guard — so that a module which
// says nothing about env_bridge cannot erase the root's own record of having
// declared it.
func (c *Config) setEnvBridgeOrigin(decl *EnvBridgeConfig, kind EnvBridgeOriginKind, src string) {
	if decl == nil {
		return
	}
	c.envBridgeOrigin = EnvBridgeOrigin{Kind: kind, Path: src}
}

// EnvBridgeVersionSatisfied reports whether c's declared `version:` meets
// EnvBridgeIntroducedVersion (TASK-281 §3-1). An empty or unparseable version
// does not satisfy it — omitting `version:` is exactly the condition the gate
// exists to reject once env_bridge is declared.
func (c *Config) EnvBridgeVersionSatisfied() bool {
	if c.Version == "" {
		return false
	}
	have, err := parseVersion(c.Version)
	if err != nil {
		return false
	}
	need, err := parseVersion(EnvBridgeIntroducedVersion)
	if err != nil {
		return false
	}
	for i := range 3 {
		if have[i] != need[i] {
			return have[i] > need[i]
		}
	}
	return true
}
