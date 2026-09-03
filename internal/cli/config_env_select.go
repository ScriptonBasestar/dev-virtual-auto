package cli

import (
	"github.com/ScriptonBasestar/dva/internal/config"
)

// selectEncryptedEntry resolves the argv target to exactly one encrypted entry,
// or fails. TASK-245 §3 freezes the whole table; the ordering below is what
// makes each row reachable.
//
// The comparison is against the declared path string exactly as written. Not a
// cleaned path, not an absolutized one, not the source name, not an index: the
// string a user can retry is the string they typed into dva.yml, and matching
// anything else would accept a spelling that dva.yml does not contain.
func selectEncryptedEntry(c *config.Config, target string) (config.EnvFileConfig, error) {
	entries := c.AllEnvFileConfigs()
	encrypted := c.EncryptedEnvEntries()

	// The zero case wins over every other diagnostic, including a target that
	// happens to name a plaintext entry. With no encrypted entry at all there is
	// no selection problem to report — the feature is simply not configured.
	if len(encrypted) == 0 {
		return config.EnvFileConfig{}, bridgeErr(codeNoEncryptedEntry, "no env_file entry declares sops_source")
	}

	if target == "" {
		if len(encrypted) > 1 {
			return config.EnvFileConfig{}, bridgeErr(codeAmbiguousSelector,
				"multiple encrypted env_file entries; name one: %s", config.JoinTargets(c.DeclaredEncryptedTargets()))
		}
		return encrypted[0], nil
	}

	for _, e := range encrypted {
		if e.Path == target {
			return e, nil
		}
	}
	// Naming a real but plaintext entry is a different mistake from naming
	// nothing, and the fix differs too — add sops_source versus correct the
	// spelling — so the two do not share a code.
	for _, e := range entries {
		if e.Path == target {
			return config.EnvFileConfig{}, bridgeErr(codeTargetNotEncrypted, "env target %q declares no sops_source", target)
		}
	}
	return config.EnvFileConfig{}, bridgeErr(codeUnknownTarget,
		"unknown env target %q; declared encrypted targets: %s", target, config.JoinTargets(c.DeclaredEncryptedTargets()))
}

// checkWritableOrigin enforces §5-1. Load records which file's declaration
// survived the whole-replace merge; a declaration the parent session cannot
// prove ownership of is refused rather than written on a guess.
func checkWritableOrigin(c *config.Config) error {
	origin := c.EnvFileOrigin()
	switch {
	case origin.Kind == config.EnvOriginUnknown:
		return bridgeErr(codeUnknownOrigin, "cannot determine which file declares env_file")
	case !origin.Writable():
		return bridgeErr(codeUnsupportedOrigin, "env_file is declared by an origin that cannot be written: %s", origin.Kind)
	}
	return nil
}
