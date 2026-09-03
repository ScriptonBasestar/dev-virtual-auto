package cli

import (
	"os"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// checkSealEnabled and checkShowEnabled are TASK-281 §3-5's gate check. Each
// runs before every other preflight step of its own command — before even the
// platform check — because a disabled command must fail the same deterministic
// way regardless of config or OS state; if it varied, the gate would not be
// explaining what it does.
func checkSealEnabled(c *config.Config) error {
	if c.EnvBridge != nil && c.EnvBridge.AllowSeal {
		return nil
	}
	return bridgeErr(codeSealNotEnabled,
		"seal is disabled; set env_bridge.allow_seal: true in dva.yml to enable it")
}

func checkShowEnabled(c *config.Config) error {
	if c.EnvBridge != nil && c.EnvBridge.AllowShow {
		return nil
	}
	return bridgeErr(codeShowNotEnabled,
		"show is disabled; set env_bridge.allow_show: true in dva.yml to enable it")
}

// checkEnvBridgeOriginAndVersion enforces TASK-281 §3-1/§3-2: env_bridge may
// only be declared in the root dva.yml, and a config that declares it must
// also declare a satisfying version:.
//
// It is shared by seal, show and `dva validate` (§3-2's "폭발 반경" — the two
// codes below report only from these three places, never from an ordinary
// lifecycle command). Called after the gate-boolean check for seal/show, so by
// the time it runs there Kind is never Unknown: a true gate boolean means the
// root itself declared env_bridge, which always records at least EnvBridgeOriginRoot.
// `dva validate` calls it unconditionally and does see Unknown, for a config
// that never mentions env_bridge at all.
func checkEnvBridgeOriginAndVersion(c *config.Config) error {
	origin := c.EnvBridgeOrigin()
	switch origin.Kind {
	case config.EnvBridgeOriginUnknown:
		return nil
	case config.EnvBridgeOriginRoot:
		if !c.EnvBridgeVersionSatisfied() {
			return bridgeErr(codeEnvBridgeRequiresVer,
				"dva.yml declares env_bridge but not a satisfying version: (declared %q, requires at least %s)",
				c.Version, config.EnvBridgeIntroducedVersion)
		}
		return nil
	default:
		return bridgeErr(codeEnvBridgeOriginNotRoot,
			"env_bridge must be declared only in the root dva.yml; also declared by %s (%s)", origin.Kind, origin.Path)
	}
}

// bridgeAgentEnvVars is the frozen, closed advisory signal list (TASK-281
// §3-6). Presence alone is the test — the value is never inspected — and the
// list stays short on purpose: an editor signal like TERM_PROGRAM would flag a
// human typing at a real terminal, which is the opposite of what this is for.
var bridgeAgentEnvVars = []string{"CLAUDECODE", "CLAUDE_CODE_ENTRYPOINT", "AI_AGENT"}

// detectAgentEnvironment is advisory only — TASK-281 §3-6 forbids treating any
// caller-identity signal as a security boundary, and this function is never
// the last line of defense for anything. It has no bypass flag by design.
func detectAgentEnvironment() bool {
	for _, name := range bridgeAgentEnvVars {
		if _, ok := os.LookupEnv(name); ok {
			return true
		}
	}
	return false
}

// bridgeOpenTTY opens the controlling terminal directly, bypassing stdin and
// stdout entirely. `show`'s decrypted output and `seal`'s confirmation prompt
// both go through this one seam: `>`, `|` and `$(...)` all redirect stdout,
// none of them redirect /dev/tty, and using a single mechanism for "is there a
// human at a real terminal" means there is only one place to get it right.
//
// A package var rather than a constant call so tests can simulate both "no
// controlling terminal" (return an error) and "here is one, and I can read
// what was written to it" (return a file the test controls).
var bridgeOpenTTY = func() (*os.File, error) {
	return os.OpenFile("/dev/tty", os.O_RDWR, 0)
}
