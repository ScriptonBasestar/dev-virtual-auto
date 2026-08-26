package cli

import (
	"reflect"
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
	"github.com/ScriptonBasestar/dva/internal/skillinstall"
)

func TestSkillOptions(t *testing.T) {
	t.Run("defaults to all runtimes", func(t *testing.T) {
		options, err := skillOptions("user", nil, true)
		if err != nil {
			t.Fatal(err)
		}
		if options.Scope != skillinstall.ScopeUser || !options.DryRun || len(options.Runtimes) != 0 {
			t.Fatalf("options = %#v; zero runtimes must delegate the engine default", options)
		}
	})

	t.Run("normalizes repeated comma lists", func(t *testing.T) {
		options, err := skillOptions("project", []string{"opencode,codex", "codex"}, false)
		if err != nil {
			t.Fatal(err)
		}
		want := []skillinstall.Runtime{skillinstall.RuntimeCodex, skillinstall.RuntimeOpenCode}
		if !reflect.DeepEqual(options.Runtimes, want) {
			t.Fatalf("runtimes = %v, want %v", options.Runtimes, want)
		}
	})

	for _, test := range []struct {
		name     string
		scope    string
		runtimes []string
		contains string
	}{
		{name: "scope", scope: "machine", contains: "supported scopes"},
		{name: "runtime", scope: "user", runtimes: []string{"agent-mesh"}, contains: "supported runtimes"},
		{name: "empty runtime", scope: "user", runtimes: []string{"codex,"}, contains: "empty name"},
	} {
		t.Run("rejects "+test.name, func(t *testing.T) {
			_, err := skillOptions(test.scope, test.runtimes, false)
			if err == nil || !strings.Contains(err.Error(), test.contains) {
				t.Fatalf("error = %v, want text %q", err, test.contains)
			}
		})
	}
}

func TestSkillManifestDescribesRealSubcommands(t *testing.T) {
	entry := buildManifest(&config.Config{}).StaticCommands["skill"]
	if len(entry.Options) != 0 {
		t.Fatalf("skill parent advertises flags it does not accept: %v", entry.Options)
	}
	children := map[string]string{
		"install":   skillInstallCmd.Short,
		"status":    skillStatusCmd.Short,
		"uninstall": skillUninstallCmd.Short,
	}
	types := map[string]string{"install": "mutation", "status": "query", "uninstall": "mutation"}
	for name, want := range children {
		subcommand, ok := entry.Subcommands[name]
		if !ok {
			t.Errorf("manifest omits skill %s", name)
			continue
		}
		if subcommand.Description != want {
			t.Errorf("skill %s description = %q, want %q", name, subcommand.Description, want)
		}
		if subcommand.Type != types[name] {
			t.Errorf("skill %s type = %q, want %q", name, subcommand.Type, types[name])
		}
		for _, flagName := range []string{"scope", "runtime"} {
			if strings.TrimSpace(subcommand.Options[flagName]) == "" {
				t.Errorf("skill %s omits --%s", name, flagName)
			}
		}
	}
	if len(entry.Subcommands) != len(children) {
		t.Fatalf("manifest skill subcommands = %v", entry.Subcommands)
	}
}

func TestSkillCommandContract(t *testing.T) {
	if skillCmd.GroupID != "advanced" {
		t.Fatalf("skill group = %q", skillCmd.GroupID)
	}
	for _, command := range []*struct {
		name  string
		scope string
	}{
		{name: skillInstallCmd.Name(), scope: skillInstallScope},
		{name: skillStatusCmd.Name(), scope: skillStatusScope},
		{name: skillUninstallCmd.Name(), scope: skillRemoveScope},
	} {
		if command.scope != "user" {
			t.Errorf("%s default scope = %q, want user", command.name, command.scope)
		}
	}
	if isTopLevelCommand("skill") != true {
		t.Fatal("skill command is not reserved")
	}
}

func TestSkillHumanOutputExplainsPartialRuntimeState(t *testing.T) {
	oldJSON := jsonOutput
	jsonOutput = false
	t.Cleanup(func() { jsonOutput = oldJSON })
	result := skillinstall.Result{
		Scope: skillinstall.ScopeProject,
		Destinations: []skillinstall.DestinationResult{{
			Destination: "/project/.agents/skills",
			Runtimes:    []skillinstall.Runtime{skillinstall.RuntimeAntigravity, skillinstall.RuntimeCodex},
			Status:      "partial",
			RuntimeStatuses: []skillinstall.RuntimeStatus{
				{Runtime: skillinstall.RuntimeAntigravity, Status: "absent"},
				{Runtime: skillinstall.RuntimeCodex, Status: "installed"},
			},
		}},
	}
	out := captureOutput(t, func() {
		if err := printSkillResult("status", false, result); err != nil {
			t.Fatal(err)
		}
	})
	for _, text := range []string{"partial", "antigravity", "absent", "codex", "installed"} {
		if !strings.Contains(out, text) {
			t.Errorf("human output omits %q:\n%s", text, out)
		}
	}
}
