package skillinstall

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"

	bundled "github.com/ScriptonBasestar/dva/skills"
)

func TestRuntimePaths(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	project := t.TempDir()
	tests := []struct {
		scope   Scope
		runtime Runtime
		want    string
	}{
		{ScopeUser, RuntimeClaudeCode, ".claude/skills"},
		{ScopeUser, RuntimeCodex, ".agents/skills"},
		{ScopeUser, RuntimeOpenCode, ".config/opencode/skills"},
		{ScopeUser, RuntimeGrok, ".grok/skills"},
		{ScopeUser, RuntimeAntigravity, ".gemini/config/skills"},
		{ScopeProject, RuntimeClaudeCode, ".claude/skills"},
		{ScopeProject, RuntimeCodex, ".agents/skills"},
		{ScopeProject, RuntimeOpenCode, ".opencode/skills"},
		{ScopeProject, RuntimeGrok, ".grok/skills"},
		{ScopeProject, RuntimeAntigravity, ".agents/skills"},
	}
	for _, test := range tests {
		t.Run(string(test.scope)+"/"+string(test.runtime), func(t *testing.T) {
			got, err := runtimePath(test.runtime, test.scope, home, project)
			if err != nil {
				t.Fatal(err)
			}
			base := home
			if test.scope == ScopeProject {
				base = project
			}
			if want := filepath.Join(base, test.want); got != want {
				t.Fatalf("path = %q, want %q", got, want)
			}
		})
	}
}

func TestProjectDeduplicatesSharedAgentsDestination(t *testing.T) {
	t.Parallel()
	options := testOptions(t, ScopeProject, RuntimeCodex, RuntimeAntigravity)
	_, destinations, err := resolve(options)
	if err != nil {
		t.Fatal(err)
	}
	if len(destinations) != 1 {
		t.Fatalf("destinations = %d, want 1", len(destinations))
	}
	if got, want := destinations[0].runtimes, []Runtime{RuntimeAntigravity, RuntimeCodex}; !sameRuntimes(got, want) {
		t.Fatalf("runtimes = %v, want %v", got, want)
	}
}

func TestEmbeddedSkillsContainCanonicalFiles(t *testing.T) {
	t.Parallel()
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate test source")
	}
	repoSkills := filepath.Join(filepath.Dir(sourceFile), "..", "..", "skills")
	var embeddedPaths []string
	if err := fs.WalkDir(bundled.Files, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || path == "embed.go" {
			return err
		}
		embeddedPaths = append(embeddedPaths, path)
		embedded, err := fs.ReadFile(bundled.Files, path)
		if err != nil {
			return err
		}
		disk, err := os.ReadFile(filepath.Join(repoSkills, filepath.FromSlash(path)))
		if err != nil {
			return err
		}
		if !bytes.Equal(embedded, disk) {
			t.Errorf("embedded %s differs from canonical disk source", path)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	var diskPaths []string
	for _, name := range bundled.Names {
		if err := filepath.WalkDir(filepath.Join(repoSkills, name), func(path string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return err
			}
			relative, err := filepath.Rel(repoSkills, path)
			if err != nil {
				return err
			}
			diskPaths = append(diskPaths, filepath.ToSlash(relative))
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	sort.Strings(embeddedPaths)
	sort.Strings(diskPaths)
	if !sameStrings(embeddedPaths, diskPaths) {
		t.Fatalf("embedded file inventory differs\nembedded: %v\ndisk: %v", embeddedPaths, diskPaths)
	}
}

func TestInstallStatusAndUninstallSharedDestination(t *testing.T) {
	t.Parallel()
	options := testOptions(t, ScopeProject, RuntimeCodex, RuntimeAntigravity)
	result, err := Install(options)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Destinations) != 1 || result.Destinations[0].Status != "installed" {
		t.Fatalf("install result = %#v", result)
	}
	destination := result.Destinations[0].Destination
	if _, err := os.Stat(filepath.Join(destination, "dva", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(destination, "dva-config", "references", "diagnosis.md")); err != nil {
		t.Fatal(err)
	}
	result, err = Install(options)
	if err != nil {
		t.Fatal(err)
	}
	if result.Destinations[0].Status != "up-to-date" {
		t.Fatalf("second install = %s", result.Destinations[0].Status)
	}
	result, err = Status(options)
	if err != nil {
		t.Fatal(err)
	}
	if result.Destinations[0].Status != "installed" {
		t.Fatalf("status = %s", result.Destinations[0].Status)
	}

	codexOnly := options
	codexOnly.Runtimes = []Runtime{RuntimeCodex}
	result, err = Uninstall(codexOnly)
	if err != nil {
		t.Fatal(err)
	}
	if result.Destinations[0].Status != "unlinked" {
		t.Fatalf("partial uninstall = %s", result.Destinations[0].Status)
	}
	if _, err := os.Stat(filepath.Join(destination, "dva")); err != nil {
		t.Fatalf("shared skill was removed: %v", err)
	}
	antigravityOnly := options
	antigravityOnly.Runtimes = []Runtime{RuntimeAntigravity}
	result, err = Uninstall(antigravityOnly)
	if err != nil {
		t.Fatal(err)
	}
	if result.Destinations[0].Status != "uninstalled" {
		t.Fatalf("last uninstall = %s", result.Destinations[0].Status)
	}
	if _, err := os.Stat(filepath.Join(destination, "dva")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("skill remains after uninstall: %v", err)
	}
}

func TestDryRunDoesNotMutate(t *testing.T) {
	t.Parallel()
	options := testOptions(t, ScopeUser, RuntimeClaudeCode)
	options.DryRun = true
	result, err := Install(options)
	if err != nil {
		t.Fatal(err)
	}
	if result.Destinations[0].Status != "would-install" {
		t.Fatalf("status = %s", result.Destinations[0].Status)
	}
	if _, err := os.Stat(result.Destinations[0].Destination); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("dry-run created destination: %v", err)
	}
	if _, err := os.Stat(options.StateRoot); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("dry-run created state: %v", err)
	}
	status, err := Status(options)
	if err != nil {
		t.Fatal(err)
	}
	if status.Destinations[0].Status != "absent" {
		t.Fatalf("status after dry-run = %s", status.Destinations[0].Status)
	}
}

func TestUninstallDryRunDoesNotMutate(t *testing.T) {
	t.Parallel()
	options := testOptions(t, ScopeUser, RuntimeClaudeCode)
	installed, err := Install(options)
	if err != nil {
		t.Fatal(err)
	}
	destination := installed.Destinations[0].Destination
	receiptFile := receiptPath(options.StateRoot, destination)
	receiptBefore, err := os.ReadFile(receiptFile)
	if err != nil {
		t.Fatal(err)
	}
	options.DryRun = true
	result, err := Uninstall(options)
	if err != nil {
		t.Fatal(err)
	}
	if result.Destinations[0].Status != "would-uninstall" {
		t.Fatalf("status = %s", result.Destinations[0].Status)
	}
	if _, err := os.Stat(filepath.Join(destination, "dva", "SKILL.md")); err != nil {
		t.Fatalf("dry-run removed skill: %v", err)
	}
	receiptAfter, err := os.ReadFile(receiptFile)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(receiptBefore, receiptAfter) {
		t.Fatal("dry-run changed receipt")
	}
}

func TestCollisionAndDriftAreRefused(t *testing.T) {
	t.Parallel()
	options := testOptions(t, ScopeProject, RuntimeGrok)
	_, destinations, err := resolve(options)
	if err != nil {
		t.Fatal(err)
	}
	collision := filepath.Join(destinations[0].path, "dva")
	if err := os.MkdirAll(collision, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(options); err == nil {
		t.Fatal("collision install succeeded")
	}
	if err := os.RemoveAll(collision); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(options); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(destinations[0].path, "dva", "SKILL.md"), []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Uninstall(options); err == nil {
		t.Fatal("drifted uninstall succeeded")
	}
	status, err := Status(options)
	if err != nil {
		t.Fatal(err)
	}
	if status.Destinations[0].Status != "drifted" {
		t.Fatalf("status = %s", status.Destinations[0].Status)
	}
}

func TestInvalidReceiptReportedAndRefused(t *testing.T) {
	t.Parallel()
	options := testOptions(t, ScopeUser, RuntimeOpenCode)
	_, destinations, err := resolve(options)
	if err != nil {
		t.Fatal(err)
	}
	path := receiptPath(options.StateRoot, destinations[0].path)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(options); err == nil {
		t.Fatal("install with invalid receipt succeeded")
	}
	status, err := Status(options)
	if err != nil {
		t.Fatal(err)
	}
	if status.Destinations[0].Status != "invalid-receipt" {
		t.Fatalf("status = %s", status.Destinations[0].Status)
	}
}

func TestStatusIdentifiesForeignCollisionAndReceiptPaths(t *testing.T) {
	t.Parallel()
	options := testOptions(t, ScopeProject, RuntimeClaudeCode)
	_, destinations, err := resolve(options)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(destinations[0].path, "dva"), 0o755); err != nil {
		t.Fatal(err)
	}
	status, err := Status(options)
	if err != nil {
		t.Fatal(err)
	}
	if status.Destinations[0].Status != "foreign-conflict" {
		t.Fatalf("status = %s", status.Destinations[0].Status)
	}
	for _, value := range []string{"dva/SKILL.md", "dva-config/references/a.md"} {
		if !validReceiptPath(value) {
			t.Fatalf("valid receipt path rejected: %q", value)
		}
	}
	for _, value := range []string{"../dva/SKILL.md", "dva/../x", "dva-config/../../x", "/dva/SKILL.md", "other/SKILL.md"} {
		if validReceiptPath(value) {
			t.Fatalf("invalid receipt path accepted: %q", value)
		}
	}
}

func TestSymlinkCollisionAndLegacyConfigAreNeverReplaced(t *testing.T) {
	t.Parallel()
	options := testOptions(t, ScopeUser, RuntimeCodex)
	_, destinations, err := resolve(options)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(destinations[0].path, 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(destinations[0].path, "config")
	if err := os.MkdirAll(legacy, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "SKILL.md"), []byte("legacy"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(options); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(legacy, "SKILL.md")); err != nil {
		t.Fatalf("legacy config skill was removed: %v", err)
	}

	symlinkOptions := testOptions(t, ScopeUser, RuntimeGrok)
	_, symlinkDestinations, err := resolve(symlinkOptions)
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "target")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(symlinkDestinations[0].path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, symlinkDestinations[0].path); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := Install(symlinkOptions); err == nil {
		t.Fatal("symlink destination install succeeded")
	}
}

func testOptions(t *testing.T, scope Scope, runtimes ...Runtime) Options {
	t.Helper()
	root := t.TempDir()
	return Options{
		Scope:       scope,
		Runtimes:    runtimes,
		HomeDir:     filepath.Join(root, "home"),
		ProjectRoot: filepath.Join(root, "project"),
		StateRoot:   filepath.Join(root, "state"),
	}
}

func sameRuntimes(left, right []Runtime) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func sameStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
