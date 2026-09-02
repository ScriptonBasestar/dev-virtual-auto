package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCloneImportedPlanCopiesEndpointTags(t *testing.T) {
	original := &PlanConfig{EndpointTags: []string{"app"}}

	cloned := cloneImportedPlan(original, nil, "/tmp/subproject")
	cloned.EndpointTags[0] = "changed"

	if got := original.EndpointTags[0]; got != "app" {
		t.Errorf("original endpoint tag = %q, want app", got)
	}
}

func TestImportedPlanExternalChildRoot(t *testing.T) {
	workspace := t.TempDir()
	parentDir := filepath.Join(workspace, "parent")
	childDir := filepath.Join(workspace, "outside-child")
	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(childDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(childDir, FileName), []byte(`
version: "0.1.0"
stack:
  child-process:
    default_runner: process
    runners:
      process:
        command: echo child
endpoints:
  app:
    source: child-process:8080
plans:
  dev:
    entries:
      - name: child-process
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(parentDir, FileName), []byte(`
version: "0.1.0"
subprojects:
  child:
    path: ../outside-child
    import:
      plans: [dev]
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(parentDir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	plan := cfg.Plans["child/dev"]
	if plan == nil {
		t.Fatal("imported plan not found")
	}
	if got := plan.OwnerConfig(cfg).FileDir(); got != childDir {
		t.Fatalf("owner directory = %q, want %q", got, childDir)
	}
	if got := plan.OwnerConfig(cfg).Stack["child-process"].Name; got != "child-process" {
		t.Fatalf("child stack entry name = %q, want finalization to populate it", got)
	}
	if got := plan.OwnerConfig(cfg).Stack["child-process"].DetectPlugin(); got != "process" {
		t.Fatalf("child stack plugin = %q, want finalization to retain its resolved runner", got)
	}
	if got := plan.OwnerConfig(cfg).Endpoints["app"].URL; got != "http://localhost:8080" {
		t.Fatalf("child endpoint URL = %q, want finalization to resolve endpoint source", got)
	}
}

func TestImportedPlanMissingAndCollisionFail(t *testing.T) {
	for _, tc := range []struct {
		name      string
		child     string
		parent    string
		wantError string
	}{
		{
			name: "missing child plan",
			child: `
version: "0.1.0"
stack: {}
`,
			parent: `
version: "0.1.0"
subprojects:
  child:
    path: child
    import:
      plans: [dev]
`,
			wantError: `plan "dev" not found`,
		},
		{
			name: "canonical collision",
			child: `
version: "0.1.0"
plans:
  dev: {}
`,
			parent: `
version: "0.1.0"
plans:
  "child/dev": {}
subprojects:
  child:
    path: child
    import:
      plans: [dev]
`,
			wantError: `plan name collision: "child/dev"`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parentDir := t.TempDir()
			childDir := filepath.Join(parentDir, "child")
			if err := os.MkdirAll(childDir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(childDir, FileName), []byte(tc.child), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(parentDir, FileName), []byte(tc.parent), 0o644); err != nil {
				t.Fatal(err)
			}

			_, err := Load(parentDir)
			if err == nil {
				t.Fatal("Load error = nil")
			}
			if !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("Load error = %q, want %q", err, tc.wantError)
			}
		})
	}
}
