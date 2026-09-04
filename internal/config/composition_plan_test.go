package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeDvaYML(t *testing.T, dir string, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCompositionPlanEntriesAndComposesAreExclusive(t *testing.T) {
	dir := t.TempDir()
	writeDvaYML(t, dir, `
version: "0.1.0"
plans:
  leaf:
    entries:
      - name: svc
  mixed:
    entries:
      - name: svc
    composes:
      - plan: leaf
`)

	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for a plan declaring both entries: and composes:, got nil")
	}
	if !strings.Contains(err.Error(), `"mixed"`) || !strings.Contains(err.Error(), "entries:") || !strings.Contains(err.Error(), "composes:") {
		t.Fatalf("error = %v, want it to name plan %q and both entries: and composes:", err, "mixed")
	}
}

func TestCompositionPlanRejectsSelfConfigAndDuplicates(t *testing.T) {
	t.Run("environment", func(t *testing.T) {
		dir := t.TempDir()
		writeDvaYML(t, dir, `
version: "0.1.0"
plans:
  leaf:
    entries:
      - name: svc
  release:
    environment: dev
    composes:
      - plan: leaf
`)
		_, err := Load(dir)
		if err == nil || !strings.Contains(err.Error(), "environment:") {
			t.Fatalf("expected environment: rejection, got %v", err)
		}
	})

	t.Run("site", func(t *testing.T) {
		dir := t.TempDir()
		writeDvaYML(t, dir, `
version: "0.1.0"
plans:
  leaf:
    entries:
      - name: svc
  release:
    site: local
    composes:
      - plan: leaf
`)
		_, err := Load(dir)
		if err == nil || !strings.Contains(err.Error(), "site:") {
			t.Fatalf("expected site: rejection, got %v", err)
		}
	})

	t.Run("top-level vars", func(t *testing.T) {
		dir := t.TempDir()
		writeDvaYML(t, dir, `
version: "0.1.0"
plans:
  leaf:
    entries:
      - name: svc
  release:
    vars:
      FOO: bar
    composes:
      - plan: leaf
`)
		_, err := Load(dir)
		if err == nil || !strings.Contains(err.Error(), "vars:") {
			t.Fatalf("expected top-level vars: rejection, got %v", err)
		}
	})

	t.Run("duplicate plan value in one composes list", func(t *testing.T) {
		dir := t.TempDir()
		writeDvaYML(t, dir, `
version: "0.1.0"
plans:
  leaf:
    entries:
      - name: svc
  release:
    composes:
      - plan: leaf
      - plan: leaf
`)
		_, err := Load(dir)
		if err == nil || !strings.Contains(err.Error(), "duplicate") {
			t.Fatalf("expected duplicate composed plan rejection, got %v", err)
		}
	})

	t.Run("two aliases resolving to the same canonical import", func(t *testing.T) {
		workspace := t.TempDir()
		parentDir := filepath.Join(workspace, "root")
		childDir := filepath.Join(workspace, "api")
		writeDvaYML(t, childDir, `
version: "0.1.0"
plans:
  deploy:
    entries:
      - name: api-server
`)
		writeDvaYML(t, parentDir, `
version: "0.1.0"
subprojects:
  api:
    path: ../api
    import:
      plans:
        - name: deploy
          as: api-deploy
plans:
  release:
    composes:
      - plan: api/deploy
      - plan: api-deploy
`)
		_, err := Load(parentDir)
		if err == nil || !strings.Contains(err.Error(), "already composed") {
			t.Fatalf("expected alias-duplicate rejection, got %v", err)
		}
	})
}

func TestCompositionEntryRequiresImportedOrLocalPlan(t *testing.T) {
	t.Run("unknown local plan", func(t *testing.T) {
		dir := t.TempDir()
		writeDvaYML(t, dir, `
version: "0.1.0"
plans:
  release:
    composes:
      - plan: nope
`)
		_, err := Load(dir)
		if err == nil || !strings.Contains(err.Error(), `"nope"`) {
			t.Fatalf("expected rejection naming the unresolved plan, got %v", err)
		}
	})

	t.Run("non-imported project/plan reference", func(t *testing.T) {
		workspace := t.TempDir()
		parentDir := filepath.Join(workspace, "root")
		childDir := filepath.Join(workspace, "api")
		writeDvaYML(t, childDir, `
version: "0.1.0"
plans:
  deploy:
    entries:
      - name: api-server
`)
		// api/dva.yml exists on disk, but root never imports it — composes:
		// must not become a back door around explicit import (TASK-263 §4).
		writeDvaYML(t, parentDir, `
version: "0.1.0"
subprojects:
  api:
    path: ../api
plans:
  release:
    composes:
      - plan: api/deploy
`)
		_, err := Load(parentDir)
		if err == nil || !strings.Contains(err.Error(), `"api/deploy"`) {
			t.Fatalf("expected rejection of non-imported project/plan reference, got %v", err)
		}
	})
}

func TestCompositionPlanCannotComposeAComposition(t *testing.T) {
	t.Run("local composition of composition", func(t *testing.T) {
		dir := t.TempDir()
		writeDvaYML(t, dir, `
version: "0.1.0"
plans:
  leaf:
    entries:
      - name: svc
  release:
    composes:
      - plan: leaf
  release-all:
    composes:
      - plan: release
`)
		_, err := Load(dir)
		if err == nil || !strings.Contains(err.Error(), `"release"`) || !strings.Contains(err.Error(), "composition plan") {
			t.Fatalf("expected composition-of-composition rejection naming %q, got %v", "release", err)
		}
	})

	t.Run("resolveSubprojectImports rejects importing a composition plan", func(t *testing.T) {
		workspace := t.TempDir()
		parentDir := filepath.Join(workspace, "root")
		childDir := filepath.Join(workspace, "api")
		writeDvaYML(t, childDir, `
version: "0.1.0"
plans:
  leaf:
    entries:
      - name: api-server
  bundle:
    composes:
      - plan: leaf
`)
		writeDvaYML(t, parentDir, `
version: "0.1.0"
subprojects:
  api:
    path: ../api
    import:
      plans:
        - name: bundle
`)
		_, err := Load(parentDir)
		if err == nil || !strings.Contains(err.Error(), "composition plan") || !strings.Contains(err.Error(), "cannot be imported") {
			t.Fatalf("expected import-time composition-plan rejection, got %v", err)
		}
	})
}

// TestCompositionPlanTaskDecisionFixtures reproduces TASK-260 §3's accepted and
// rejected YAML fixtures verbatim (adjusted only for a version: this repository's
// binary actually satisfies — the Decision Record's "1.5" is illustrative, not a
// real released version).
func TestCompositionPlanTaskDecisionFixtures(t *testing.T) {
	t.Run("accepted: two-project depends_on composition", func(t *testing.T) {
		workspace := t.TempDir()
		rootDir := filepath.Join(workspace, "root")
		apiDir := filepath.Join(workspace, "api")
		webDir := filepath.Join(workspace, "web")

		writeDvaYML(t, apiDir, `
version: "0.1.0"
plans:
  deploy:
    entries:
      - name: api-server
`)
		writeDvaYML(t, webDir, `
version: "0.1.0"
plans:
  deploy:
    entries:
      - name: web-server
`)
		writeDvaYML(t, rootDir, `
version: "0.1.0"
subprojects:
  api:
    path: ../api
    import:
      plans:
        - name: deploy
  web:
    path: ../web
    import:
      plans:
        - name: deploy
plans:
  release:
    composes:
      - plan: api/deploy
        order: 0
      - plan: web/deploy
        order: 1
        depends_on: ["api/deploy"]
`)

		cfg, err := Load(rootDir)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		release, ok := cfg.Plans["release"]
		if !ok {
			t.Fatal("plan \"release\" not found")
		}
		if len(release.Composes) != 2 {
			t.Fatalf("release.Composes = %d entries, want 2", len(release.Composes))
		}
		if release.Composes[0].Plan != "api/deploy" || release.Composes[0].Order != 0 {
			t.Errorf("composes[0] = %+v, want plan api/deploy order 0", release.Composes[0])
		}
		if release.Composes[1].Plan != "web/deploy" || release.Composes[1].Order != 1 {
			t.Errorf("composes[1] = %+v, want plan web/deploy order 1", release.Composes[1])
		}
		if len(release.Composes[1].DependsOn) != 1 || release.Composes[1].DependsOn[0] != "api/deploy" {
			t.Errorf("composes[1].DependsOn = %v, want [api/deploy]", release.Composes[1].DependsOn)
		}
	})

	t.Run("rejected: composing a composition plan", func(t *testing.T) {
		workspace := t.TempDir()
		rootDir := filepath.Join(workspace, "root")
		apiDir := filepath.Join(workspace, "api")
		webDir := filepath.Join(workspace, "web")

		writeDvaYML(t, apiDir, `
version: "0.1.0"
plans:
  deploy:
    entries:
      - name: api-server
`)
		writeDvaYML(t, webDir, `
version: "0.1.0"
plans:
  deploy:
    entries:
      - name: web-server
`)
		writeDvaYML(t, rootDir, `
version: "0.1.0"
subprojects:
  api:
    path: ../api
    import:
      plans:
        - name: deploy
  web:
    path: ../web
    import:
      plans:
        - name: deploy
plans:
  release:
    composes:
      - plan: api/deploy
      - plan: web/deploy
  release-all:
    composes:
      - plan: release
`)

		_, err := Load(rootDir)
		if err == nil {
			t.Fatal("expected \"composition plans cannot compose another composition plan\" rejection, got nil")
		}
		if !strings.Contains(err.Error(), `"release-all"`) || !strings.Contains(err.Error(), `"release"`) {
			t.Fatalf("error = %v, want it to name both release-all and release", err)
		}
	})
}
