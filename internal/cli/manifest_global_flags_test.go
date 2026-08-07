package cli

import (
	"testing"

	"github.com/spf13/pflag"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// TASK-151: root persistent flags must appear once as global_flags, derived from cobra.
func TestManifestPublishesEveryPersistentFlag(t *testing.T) {
	m := buildManifest(&config.Config{})

	type flagMeta struct{ name, typ, usage string }
	var registered []flagMeta
	want := map[string]bool{}
	rootCmd.PersistentFlags().VisitAll(func(f *pflag.Flag) {
		if f.Name == "help" {
			return
		}
		registered = append(registered, flagMeta{f.Name, f.Value.Type(), f.Usage})
		want[f.Name] = true
	})

	if len(m.GlobalFlags) == 0 {
		t.Fatal("global_flags is empty — agents cannot discover --json")
	}
	got := map[string]ManifestFlag{}
	for _, g := range m.GlobalFlags {
		got[g.Name] = g
	}
	for _, r := range registered {
		g, ok := got[r.name]
		if !ok {
			t.Errorf("persistent --%s is registered but missing from global_flags", r.name)
			continue
		}
		if g.Type != r.typ {
			t.Errorf("global_flags[%s].type = %q, want %q", r.name, g.Type, r.typ)
		}
		if g.Description != r.usage {
			t.Errorf("global_flags[%s].description drifted:\n manifest: %q\n cobra:    %q", r.name, g.Description, r.usage)
		}
	}
	for name := range got {
		if !want[name] {
			t.Errorf("global_flags has %q which is not a root persistent flag", name)
		}
	}

	// Per-command lists still exclude them (TASK-105 filter intact).
	for name, cmd := range m.StaticCommands {
		for _, g := range m.GlobalFlags {
			if _, ok := cmd.Options[g.Name]; ok {
				t.Errorf("static_commands[%q].options still lists global --%s; filter regressed", name, g.Name)
			}
		}
	}
	t.Logf("global_flags=%d persistent=%d", len(m.GlobalFlags), len(registered))
}
