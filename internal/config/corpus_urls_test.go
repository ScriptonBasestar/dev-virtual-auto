package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// The repository is dev-virtual-auto. github.com/ScriptonBasestar/dva is the Go module
// path and resolves to nothing on GitHub, so it is never valid inside a URL. The default
// branch is master; this repo has never had a main.
const (
	canonicalRepo   = "dev-virtual-auto"
	canonicalBranch = "master"
)

// rawURL and blobURL capture owner/repo/ref/path from the two GitHub URL shapes DVA
// writes about itself. Both require the https:// prefix so Go import paths and
// `go install` lines, where ScriptonBasestar/dva is correct, are left alone.
var (
	rawURL  = regexp.MustCompile(`https://raw\.githubusercontent\.com/([\w.-]+)/([\w.-]+)/(?:refs/heads/)?([\w.-]+)/([^\s"'` + "`" + `)\]]+)`)
	blobURL = regexp.MustCompile(`https://github\.com/([\w.-]+)/([\w.-]+)/blob/([\w.-]+)/([^\s"'` + "`" + `)\]]+)`)
)

// urlAuditRoots are the places DVA states its own URLs: the corpus that teaches an AI to
// write dva.yml, and the Go sources that print links to users.
//
// Root README.md is deliberately absent. It is human-owned (doc-protection: ai=deny) and
// carries a release-download URL on the unresolvable repo name, which is a decision about
// what the canonical repo should be — not something a test may quietly hold hostage.
func urlAuditRoots() []string {
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(file), "..", "..")
	return append(generatorCorpus(),
		filepath.Join(root, "internal"),
		filepath.Join(root, "docs"),
		filepath.Join(root, "dva.yml"),
	)
}

// TestGeneratorCorpusURLs fails when a self-referencing URL names a repository or branch
// that does not exist, or points a $schema at a path absent from the tree.
//
// TestRemovedKeysAbsentFromGeneratorCorpus already stops the corpus teaching removed
// keys. Nothing checked that the URLs it teaches resolve, and the same pipeline spread
// both: authored library file -> make generate -> library_reference.txt -> embedded in
// the binary -> AI flows stamp it into user configs. A dead $schema is silent by
// construction — the editor produces no diagnostics rather than an error — so it reached
// 56 of 83 real configs before anyone noticed.
//
// String checks only, no network. A test that reaches the internet fails on a plane and
// passes when GitHub is having a bad day, which is the opposite of a guard.
func TestGeneratorCorpusURLs(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(file), "..", "..")

	check := func(path string, lineNo int, line string, m []string) {
		owner, repo, ref, target := m[1], m[2], m[3], m[4]
		if repo != canonicalRepo {
			t.Errorf("%s:%d URL names repository %q, but %s/%s is the repo (%q is the Go module path and 404s on GitHub)\n  %s",
				path, lineNo, repo, owner, canonicalRepo, repo, strings.TrimSpace(line))
		}
		if ref != canonicalBranch {
			t.Errorf("%s:%d URL names branch %q; this repo's default branch is %s\n  %s",
				path, lineNo, ref, canonicalBranch, strings.TrimSpace(line))
		}
		// A URL may legitimately point into a directory or carry an anchor; only the
		// file part is verifiable offline.
		clean, _, _ := strings.Cut(target, "#")
		if clean == "" || strings.HasSuffix(clean, "/") {
			return
		}
		if _, err := os.Stat(filepath.Join(repoRoot, clean)); err != nil {
			t.Errorf("%s:%d URL points at %q, which is not in the tree\n  %s",
				path, lineNo, clean, strings.TrimSpace(line))
		}
	}

	var files, urls int
	seen := map[string]bool{}
	for _, root := range urlAuditRoots() {
		err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			switch filepath.Ext(path) {
			case ".md", ".yml", ".yaml", ".txt", ".go", ".json":
			default:
				return nil
			}
			// Test files carry deliberately broken URLs as fixtures, including this
			// file's planted defects. Auditing them would make the guard fail on its
			// own evidence.
			if strings.HasSuffix(path, "_test.go") {
				return nil
			}
			if seen[path] { // roots overlap once library_reference.txt is listed twice
				return nil
			}
			seen[path] = true
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			files++
			for i, line := range strings.Split(string(content), "\n") {
				for _, re := range []*regexp.Regexp{rawURL, blobURL} {
					for _, m := range re.FindAllStringSubmatch(line, -1) {
						urls++
						check(path, i+1, line, m)
					}
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}

	// Both counters matter: zero files means the paths moved, zero URLs means the
	// regexes stopped matching. Either way the test would pass forever while guarding
	// nothing, which is exactly the failure it exists to prevent.
	if files == 0 {
		t.Fatal("URL audit walked no files — the paths moved and this test stopped guarding anything")
	}
	if urls == 0 {
		t.Fatal("URL audit matched no URLs — the patterns no longer describe how DVA writes its own links")
	}
	t.Logf("audited %d URLs across %d files", urls, files)
}

// TestGeneratorCorpusURLsDetectsPlantedDefects pins the detector itself. Without this the
// guard above could be silently defanged by a regex edit and still report success.
func TestGeneratorCorpusURLsDetectsPlantedDefects(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(file), "..", "..")

	tests := []struct {
		name string
		line string
		want string
	}{
		{
			name: "wrong repository — the Go module path used as a repo name",
			line: `const u = "https://github.com/ScriptonBasestar/dva/blob/master/docs/40-declarative-stack-and-plans.md#11-migration"`,
			want: "names repository",
		},
		{
			name: "branch that does not exist",
			line: `const u = "https://github.com/ScriptonBasestar/dev-virtual-auto/blob/main/docs/40-declarative-stack-and-plans.md"`,
			want: "names branch",
		},
		{
			name: "$schema path absent from the tree",
			line: `# yaml-language-server: $schema=https://raw.githubusercontent.com/ScriptonBasestar/dev-virtual-auto/master/schema.json`,
			want: "not in the tree",
		},
		{
			name: "refs/heads form is unwrapped before the branch is judged",
			line: `# $schema=https://raw.githubusercontent.com/ScriptonBasestar/dev-virtual-auto/refs/heads/main/internal/config/schema.json`,
			want: "names branch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []string
			report := func(format string, args ...any) { got = append(got, fmt.Sprintf(format, args...)) }

			for _, re := range []*regexp.Regexp{rawURL, blobURL} {
				for _, m := range re.FindAllStringSubmatch(tt.line, -1) {
					owner, repo, ref, target := m[1], m[2], m[3], m[4]
					if repo != canonicalRepo {
						report("names repository %q (owner %s)", repo, owner)
					}
					if ref != canonicalBranch {
						report("names branch %q", ref)
					}
					clean, _, _ := strings.Cut(target, "#")
					if clean != "" && !strings.HasSuffix(clean, "/") {
						if _, err := os.Stat(filepath.Join(repoRoot, clean)); err != nil {
							report("%q is not in the tree", clean)
						}
					}
				}
			}

			if len(got) == 0 {
				t.Fatalf("planted defect went undetected: %s", tt.line)
			}
			if !strings.Contains(strings.Join(got, "; "), tt.want) {
				t.Errorf("expected a %q complaint, got %v", tt.want, got)
			}
		})
	}
}
