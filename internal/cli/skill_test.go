package cli

import (
	"reflect"
	"strings"
	"testing"

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
