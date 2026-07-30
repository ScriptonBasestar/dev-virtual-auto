package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// writeConfig writes a dva.yml declaring one compose stack entry, plus the compose files it
// references, since the loader validates that an entry's source exists.
func writeConfig(t *testing.T, dir, projectName string, composeFiles []string, extra string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	var files strings.Builder
	for _, f := range composeFiles {
		fmt.Fprintf(&files, "          - %s\n", f)
		path := filepath.Join(dir, f)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("services:\n  app:\n    image: nginx\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	body := "stack:\n" +
		"  compose:\n" +
		"    order: 10\n" +
		"    default_runner: compose\n" +
		"    runners:\n" +
		"      compose:\n" +
		"        files:\n" + files.String() +
		"        project_name: " + projectName + "\n" +
		extra
	if err := os.WriteFile(filepath.Join(dir, config.FileName), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func loadParent(t *testing.T, dir string) *config.Config {
	t.Helper()
	c, err := config.Load(dir, config.SkipVersionCheck())
	if err != nil {
		t.Fatalf("load parent: %v", err)
	}
	return c
}

// TestCheckSubprojectComposeProjectNames covers TASK-059: a subproject declaring its
// parent's compose project name while describing a different stack, which makes `dva down`
// in the child reap the parent's containers.
//
// The check lives in doctor rather than validate because validate never opens the child's
// file — a subproject is only loaded when it declares an import: block, and this collision
// occurs in subprojects that declare just a path.
func TestCheckSubprojectComposeProjectNames(t *testing.T) {
	t.Run("collision: same project name, different compose files", func(t *testing.T) {
		root := t.TempDir()
		writeConfig(t, root, "shared-proj", []string{"parent-compose.yaml"},
			"subprojects:\n  child:\n    path: child\n")
		writeConfig(t, filepath.Join(root, "child"), "shared-proj", []string{"child-compose.yaml"}, "")

		results := checkSubprojectComposeProjectNames(loadParent(t, root))
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d: %+v", len(results), results)
		}
		if results[0].Passed {
			t.Error("expected the collision to be reported as not passed")
		}
		for _, want := range []string{"child", "shared-proj"} {
			if !strings.Contains(results[0].Name, want) {
				t.Errorf("result name %q does not mention %q", results[0].Name, want)
			}
		}
	})

	t.Run("silent: same project name and the same compose file", func(t *testing.T) {
		// An overlay-style split — both configs hand docker the same file, so one project
		// name is correct and must not be flagged.
		root := t.TempDir()
		writeConfig(t, root, "shared-proj", []string{"compose.yaml"},
			"subprojects:\n  child:\n    path: child\n")
		writeConfig(t, filepath.Join(root, "child"), "shared-proj", []string{"../compose.yaml"}, "")

		if results := checkSubprojectComposeProjectNames(loadParent(t, root)); len(results) != 0 {
			t.Errorf("expected no results for an overlay split, got %+v", results)
		}
	})

	t.Run("silent: different project names", func(t *testing.T) {
		root := t.TempDir()
		writeConfig(t, root, "parent-proj", []string{"parent-compose.yaml"},
			"subprojects:\n  child:\n    path: child\n")
		writeConfig(t, filepath.Join(root, "child"), "child-proj", []string{"child-compose.yaml"}, "")

		if results := checkSubprojectComposeProjectNames(loadParent(t, root)); len(results) != 0 {
			t.Errorf("expected no results when names differ, got %+v", results)
		}
	})

	t.Run("one unloadable subproject does not suppress the others", func(t *testing.T) {
		// LoadSubprojects returns nil, err on any single failure, so loading the whole map
		// at once would let the missing child hide the colliding one entirely.
		root := t.TempDir()
		writeConfig(t, root, "shared-proj", []string{"parent-compose.yaml"},
			"subprojects:\n  absent:\n    path: absent\n  zcollide:\n    path: zcollide\n")
		writeConfig(t, filepath.Join(root, "zcollide"), "shared-proj", []string{"child-compose.yaml"}, "")
		// "absent" gets a directory but no dva.yml, so its load fails.
		if err := os.MkdirAll(filepath.Join(root, "absent"), 0o755); err != nil {
			t.Fatal(err)
		}

		results := checkSubprojectComposeProjectNames(loadParent(t, root))
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d: %+v", len(results), results)
		}
		if !strings.Contains(results[0].Name, "zcollide") {
			t.Errorf("expected the collision on zcollide, got %q", results[0].Name)
		}
	})

	t.Run("silent: no subprojects", func(t *testing.T) {
		root := t.TempDir()
		writeConfig(t, root, "solo-proj", []string{"compose.yaml"}, "")

		if results := checkSubprojectComposeProjectNames(loadParent(t, root)); len(results) != 0 {
			t.Errorf("expected no results without subprojects, got %+v", results)
		}
	})
}

// TestSameStringSet pins the set comparison, including the empty case. An empty side must not
// read as "the stacks match" — that would silence the warning for a config with no compose
// files at all, which is the opposite of a safe default.
func TestSameStringSet(t *testing.T) {
	for _, tt := range []struct {
		name string
		a, b []string
		want bool
	}{
		{"identical", []string{"/x/a.yml"}, []string{"/x/a.yml"}, true},
		{"order differs", []string{"/x/a.yml", "/x/b.yml"}, []string{"/x/b.yml", "/x/a.yml"}, true},
		{"duplicates ignored", []string{"/x/a.yml", "/x/a.yml"}, []string{"/x/a.yml"}, true},
		{"disjoint", []string{"/x/a.yml"}, []string{"/y/b.yml"}, false},
		{"b is a subset", []string{"/x/a.yml", "/x/b.yml"}, []string{"/x/a.yml"}, false},
		{"a is a subset", []string{"/x/a.yml"}, []string{"/x/a.yml", "/x/b.yml"}, false},
		{"both empty", nil, nil, false},
		{"a empty", nil, []string{"/x/a.yml"}, false},
		{"b empty", []string{"/x/a.yml"}, nil, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := sameStringSet(tt.a, tt.b); got != tt.want {
				t.Errorf("sameStringSet(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}
