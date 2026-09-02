package lifecycle

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

func TestImportedPlanResolvesAgainstOwningProject(t *testing.T) {
	parentDir := t.TempDir()
	childDir := filepath.Join(parentDir, "backend")
	writeImportedPlanConfig(t, childDir, `
version: "0.1.0"
vars:
  SHARED: child-global
  CHILD_ONLY: child
environments:
  dev:
    environment:
      SHARED: child-environment
      CHILD_ENV: child
sites:
  local:
    vars:
      SHARED: child-site
      CHILD_SITE: child
stack:
  api:
    default_runner: process
    runners:
      process:
        command: echo CHILD
plans:
  local-dev:
    environment: dev
    site: local
    vars:
      SHARED: child-plan
    entries:
      - name: api
`)
	writeImportedPlanConfig(t, parentDir, `
version: "0.1.0"
vars:
  SHARED: parent-global
environments:
  dev:
    environment:
      CHILD_ENV: parent
sites:
  local:
    vars:
      CHILD_SITE: parent
stack:
  api:
    default_runner: process
    runners:
      process:
        command: echo PARENT
subprojects:
  backend:
    path: backend
    import:
      plans:
        - name: local-dev
          as: dev
`)

	parent, err := config.Load(parentDir)
	if err != nil {
		t.Fatalf("Load parent: %v", err)
	}

	plan, err := ResolvePlan(parent, "backend/local-dev", map[string]string{"SHARED": "cli"})
	if err != nil {
		t.Fatalf("ResolvePlan imported canonical: %v", err)
	}
	if plan.Name != "backend/local-dev" {
		t.Fatalf("plan name = %q, want requested canonical name", plan.Name)
	}
	if got := plan.OwnerConfig(parent).FileDir(); got != childDir {
		t.Fatalf("owner directory = %q, want child %q", got, childDir)
	}
	if got := plan.Entries[0].RunnerConfig.(*config.ProcessPluginConfig).Command; got != "echo CHILD" {
		t.Fatalf("resolved runner command = %q, want child declaration", got)
	}
	if got := plan.EnvVars["SHARED"]; got != "cli" {
		t.Fatalf("SHARED = %q, want CLI value", got)
	}
	if got := plan.EnvVars["CHILD_ONLY"]; got != "child" {
		t.Fatalf("CHILD_ONLY = %q, want child global value", got)
	}
	if got := plan.EnvVars["CHILD_ENV"]; got != "child" {
		t.Fatalf("CHILD_ENV = %q, want child environment value", got)
	}
	if got := plan.EnvVars["CHILD_SITE"]; got != "child" {
		t.Fatalf("CHILD_SITE = %q, want child site value", got)
	}

	alias, err := ResolvePlan(parent, "dev", nil)
	if err != nil {
		t.Fatalf("ResolvePlan imported alias: %v", err)
	}
	if alias.Name != "dev" {
		t.Fatalf("alias plan name = %q, want requested alias", alias.Name)
	}
	if alias.OwnerConfig(parent) != plan.OwnerConfig(parent) {
		t.Fatal("canonical and alias plans must keep the same owning config")
	}
}

func TestImportedPlanOwnerIsolation(t *testing.T) {
	parentDir := t.TempDir()
	childDir := filepath.Join(parentDir, "child")
	writeImportedPlanConfig(t, childDir, `
version: "0.1.0"
stack:
  app:
    default_runner: process
    runners:
      process:
        command: echo CHILD
plans:
  dev:
    entries:
      - name: app
`)
	writeImportedPlanConfig(t, parentDir, `
version: "0.1.0"
stack:
  app:
    default_runner: process
    runners:
      process:
        command: echo PARENT
subprojects:
  child:
    path: child
    import:
      plans: [dev]
`)

	parent, err := config.Load(parentDir)
	if err != nil {
		t.Fatalf("Load parent: %v", err)
	}
	plan, err := ResolvePlan(parent, "child/dev", nil)
	if err != nil {
		t.Fatalf("ResolvePlan: %v", err)
	}

	if _, err := NewPlanOrchestrator(parent, config.NewEnvironment(nil, parentDir, parentDir), plan); err == nil {
		t.Fatal("NewPlanOrchestrator accepted a parent-rooted environment for an imported plan")
	}
	owner := plan.OwnerConfig(parent)
	orch, err := NewPlanOrchestrator(parent, config.NewEnvironment(nil, owner.FileDir(), owner.FileDir()), plan)
	if err != nil {
		t.Fatalf("NewPlanOrchestrator with child environment: %v", err)
	}
	if orch.cfg == parent {
		t.Fatal("orchestrator retained parent config for imported plan")
	}
	if got := orch.entries[0].Process.Command; got != "echo CHILD" {
		t.Fatalf("orchestrator process command = %q, want child declaration", got)
	}
}

func TestImportedPlanExternalChildRoot(t *testing.T) {
	workspace := t.TempDir()
	childDir := filepath.Join(workspace, "external-child")
	writeImportedPlanConfig(t, childDir, `
version: "0.1.0"
stack:
  app:
    default_runner: native
    runners:
      native:
        dir: app
        run: "true"
plans:
  dev:
    entries: [{name: app}]
`)

	for _, absolute := range []bool{false, true} {
		name := "parent-relative"
		if absolute {
			name = "absolute"
		}
		t.Run(name, func(t *testing.T) {
			parentDir := filepath.Join(workspace, name, "parent")
			childPath := childDir
			if !absolute {
				var err error
				childPath, err = filepath.Rel(parentDir, childDir)
				if err != nil {
					t.Fatal(err)
				}
				if !strings.HasPrefix(childPath, "..") {
					t.Fatalf("relative fixture path = %q, want parent escape", childPath)
				}
			}
			writeImportedPlanConfig(t, parentDir, fmt.Sprintf(`
version: "0.1.0"
subprojects:
  child:
    path: %q
    import:
      plans: [dev]
`, childPath))

			parent, err := config.Load(parentDir)
			if err != nil {
				t.Fatalf("Load parent: %v", err)
			}
			plan, err := ResolvePlan(parent, "child/dev", nil)
			if err != nil {
				t.Fatalf("ResolvePlan: %v", err)
			}
			if got := plan.OwnerConfig(parent).FileDir(); got != childDir {
				t.Fatalf("owner root = %q, want %q", got, childDir)
			}
			if got := plan.Entries[0].WorkingDir; got != "app" {
				t.Fatalf("relative runner directory = %q, want app", got)
			}
		})
	}
}

func writeImportedPlanConfig(t *testing.T, dir, contents string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, config.FileName), []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile %s: %v", dir, err)
	}
}
