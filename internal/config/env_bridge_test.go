package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestSubprojectEnvBridgeDoesNotEnableParentGate locks TASK-281 §3-2 / TASK-282
// completion criterion 3: a subproject's own env_bridge declaration is real
// only for a session run directly inside that subproject's directory (see
// LoadSubprojects' own setEnvBridgeOrigin call), and has no representation on
// the parent's side at all. LoadSubprojects returns an independent map that
// Load never merges back into the parent Config, so the parent's own
// c.EnvBridge and c.EnvBridgeOrigin() must stay at their zero/unset values
// even when a subproject aggressively declares allow_seal/allow_show: true
// with a version that would otherwise satisfy the gate.
func TestSubprojectEnvBridgeDoesNotEnableParentGate(t *testing.T) {
	parentDir := t.TempDir()

	subDir := filepath.Join(parentDir, "sub")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("mkdir sub: %v", err)
	}
	if err := os.WriteFile(filepath.Join(subDir, FileName), []byte(`
version: "0.1.48"
env_bridge:
  allow_seal: true
  allow_show: true
`), 0o644); err != nil {
		t.Fatalf("write sub dva.yml: %v", err)
	}

	if err := os.WriteFile(filepath.Join(parentDir, FileName), []byte(`
version: "0.1.0"
subprojects:
  sub:
    path: sub
`), 0o644); err != nil {
		t.Fatalf("write parent dva.yml: %v", err)
	}

	cfg, err := Load(parentDir)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if cfg.EnvBridge != nil {
		t.Fatalf("parent Config.EnvBridge = %+v, want nil — a subproject's env_bridge must not leak into the parent", cfg.EnvBridge)
	}
	if origin := cfg.EnvBridgeOrigin(); origin.Kind != EnvBridgeOriginUnknown {
		t.Fatalf("parent EnvBridgeOrigin() = %+v, want zero value — a subproject's declaration must not be recorded as the parent's own origin", origin)
	}

	// The subproject's own standalone Config, loaded directly, does see its own
	// declaration — this is what makes it "real for a session run directly
	// inside that subproject's directory" rather than inert everywhere.
	//
	// SkipVersionCheck: the running dva's own compiled-in config.Version
	// (0.1.47 as of this writing) is itself behind EnvBridgeIntroducedVersion
	// (0.1.48) — a pre-release blocker unrelated to what this test proves, and
	// already documented in TASK-282's completion notes. Loading the sub's own
	// declared version: 0.1.48 with the version check enabled would refuse
	// before reaching the assertions below, so this test — like the rest of
	// the env_bridge suite — bypasses that unrelated check to isolate the
	// property actually under test.
	subCfg, err := Load(subDir, SkipVersionCheck())
	if err != nil {
		t.Fatalf("Load(sub) error: %v", err)
	}
	if subCfg.EnvBridge == nil || !subCfg.EnvBridge.AllowSeal || !subCfg.EnvBridge.AllowShow {
		t.Fatalf("subCfg.EnvBridge = %+v, want allow_seal/allow_show true when loaded directly", subCfg.EnvBridge)
	}

	// LoadSubprojects itself also never surfaces the subproject's env_bridge
	// back onto anything the parent merges: it returns an independent map, and
	// the parent's own fields (asserted above) are already proof of that, but
	// this also confirms the returned entry itself carries no parent-facing gate.
	subs := map[string]SubprojectConfig{"sub": {Path: "sub"}}
	result, err := LoadSubprojects(parentDir, subs, SkipVersionCheck())
	if err != nil {
		t.Fatalf("LoadSubprojects error: %v", err)
	}
	if result["sub"].EnvBridge == nil || !result["sub"].EnvBridge.AllowSeal {
		t.Fatalf("LoadSubprojects()[\"sub\"].EnvBridge = %+v, want the subproject's own declaration preserved on its own standalone Config", result["sub"].EnvBridge)
	}
}
