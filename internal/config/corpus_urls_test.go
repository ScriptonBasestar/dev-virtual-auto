package config

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// canonicalBranch is master; this repo has never had a main.
const canonicalBranch = "master"

// canonicalRepo is read from go.mod rather than transcribed, because a hand-written
// constant is the very defect this file guards against — and it already bit once. The
// constant said "dev-virtual-auto" while TASK-060 was open; when the repo was renamed to
// dva the guard kept enforcing the dead name, rejecting correct URLs and accepting stale
// ones. A test cannot notice that its own constant went out of date.
//
// Deriving it is sound under either answer TASK-060 could have taken: renaming the module
// to match the repo and renaming the repo to match the module both end at repo name ==
// last element of the module path. So this reads the fact from the file that has to be
// right for the build to work at all.
func canonicalRepo(t *testing.T) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	gomod := filepath.Join(filepath.Dir(file), "..", "..", "go.mod")
	content, err := os.ReadFile(gomod)
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	for line := range strings.SplitSeq(string(content), "\n") {
		if repo := repoFromModuleDirective(line); repo != "" {
			return repo
		}
	}
	t.Fatalf("no usable module directive in %s", gomod)
	return ""
}

// repoFromModuleDirective returns the repository name a go.mod line implies, or "" if the
// line is not a usable module directive.
//
// Splitting on fields rather than slicing after "module " is deliberate: the directive may
// be tab-separated and may carry a trailing comment, and it may quote the path. A bare
// path.Base over the rest of the line handles none of those — `…/dva // pinned` yields
// " pinned" and `"…/dva"` yields `dva"`, either of which would make the guard reject every
// URL in the repo while reporting a nonsense expected-name.
func repoFromModuleDirective(line string) string {
	fields := strings.Fields(line)
	if len(fields) < 2 || fields[0] != "module" || strings.HasPrefix(fields[1], "//") {
		return ""
	}
	repo := path.Base(strings.Trim(fields[1], `"`))
	if repo == "" || repo == "." || repo == "/" {
		return ""
	}
	return repo
}

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
// Root README.md is deliberately absent: it is human-owned (doc-protection: ai=deny), so a
// failure there would be one no agent may fix. Its install and release URLs both name dva
// and became correct when the repo was renamed (TASK-060), so nothing is lost by omitting
// it — but that is a happy accident, not a guarantee the guard provides.
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
// both: authored library file -> make generate -> library_reference.txt -> AI flows read
// it from disk and stamp it into user configs. A dead $schema is silent by construction —
// the editor produces no diagnostics rather than an error — so it reached 56 of 83 real
// configs before anyone noticed.
//
// String checks only, no network. A test that reaches the internet fails on a plane and
// passes when GitHub is having a bad day, which is the opposite of a guard.
func TestGeneratorCorpusURLs(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(file), "..", "..")
	wantRepo := canonicalRepo(t)

	check := func(where string, lineNo int, line string, m []string) {
		owner, repo, ref, target := m[1], m[2], m[3], m[4]
		if repo != wantRepo {
			t.Errorf("%s:%d URL names repository %q, but the module path in go.mod makes %s/%s canonical\n  %s",
				where, lineNo, repo, owner, wantRepo, strings.TrimSpace(line))
		}
		if ref != canonicalBranch {
			t.Errorf("%s:%d URL names branch %q; this repo's default branch is %s\n  %s",
				where, lineNo, ref, canonicalBranch, strings.TrimSpace(line))
		}
		// A URL may legitimately point into a directory or carry an anchor; only the
		// file part is verifiable offline.
		clean, _, _ := strings.Cut(target, "#")
		if clean == "" || strings.HasSuffix(clean, "/") {
			return
		}
		if _, err := os.Stat(filepath.Join(repoRoot, clean)); err != nil {
			t.Errorf("%s:%d URL points at %q, which is not in the tree\n  %s",
				where, lineNo, clean, strings.TrimSpace(line))
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

// TestRepoFromModuleDirective covers the legal go.mod spellings this repo does not happen
// to use. Our own go.mod is the plain form, so without a table here the quoted and
// commented cases would only be exercised the day someone edits go.mod — and the symptom
// then is every URL failing at once, which reads like the URLs are wrong.
func TestRepoFromModuleDirective(t *testing.T) {
	for _, tt := range []struct{ line, want string }{
		{"module github.com/ScriptonBasestar/dva", "dva"},
		{"module github.com/ScriptonBasestar/dva // pinned for now", "dva"},
		{`module "github.com/ScriptonBasestar/dva"`, "dva"},
		{"module\tgithub.com/ScriptonBasestar/dva", "dva"},
		{"  module   github.com/x/y  ", "y"},
		{"module dva", "dva"}, // single-element path is still a name
		{"go 1.26.4", ""},
		{"// module github.com/x/y", ""},
		{"module", ""},
		{"module //", ""},
		{"require github.com/x/y v1.0.0", ""},
		{"", ""},
	} {
		if got := repoFromModuleDirective(tt.line); got != tt.want {
			t.Errorf("repoFromModuleDirective(%q) = %q, want %q", tt.line, got, tt.want)
		}
	}
}

// TestGeneratorCorpusURLsDetectsPlantedDefects pins the detector itself. Without this the
// guard above could be silently defanged by a regex edit and still report success.
func TestGeneratorCorpusURLsDetectsPlantedDefects(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(file), "..", "..")
	wantRepo := canonicalRepo(t)

	tests := []struct {
		name string
		line string
		want string
	}{
		{
			name: "wrong repository — the pre-rename name, which now survives only on GitHub's redirect",
			line: `const u = "https://github.com/ScriptonBasestar/dev-virtual-auto/blob/master/docs/40-declarative-stack-and-plans.md#11-migration"`,
			want: "names repository",
		},
		{
			name: "branch that does not exist",
			line: `const u = "https://github.com/ScriptonBasestar/dva/blob/main/docs/40-declarative-stack-and-plans.md"`,
			want: "names branch",
		},
		{
			name: "$schema path absent from the tree",
			line: `# yaml-language-server: $schema=https://raw.githubusercontent.com/ScriptonBasestar/dva/master/schema.json`,
			want: "not in the tree",
		},
		{
			name: "refs/heads form is unwrapped before the branch is judged",
			line: `# $schema=https://raw.githubusercontent.com/ScriptonBasestar/dva/refs/heads/main/internal/config/schema.json`,
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
					if repo != wantRepo {
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
