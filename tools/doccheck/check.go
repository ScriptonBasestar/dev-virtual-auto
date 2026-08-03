package main

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

// CheckInput is the pure input to Check (root + inventory).
type CheckInput struct {
	Root      string
	Inventory []InventoryEntry
}

// Result is the structured outcome of a documentation gate run.
type Result struct {
	OK                 bool
	MarkdownCandidates int
	MarkdownChecked    int
	LinksChecked       int
	SymlinksSkipped    int
	BrokenLinks        int
	OversizedDocs      int
	TestFilesSwept     int
	TestFuncsFound     int
	RunPatternsChecked int
	UnmatchedRunFlags  int
	Errors             []string
	BrokenDetail       []string
	OversizedDetail    []string
	UnmatchedRunDetail []string
}

// Check validates repository-wide relative markdown links against the git
// inventory, and option-B size limits on docs/ and workflows/ only.
// Inventory is the resolution set so ignored tmp files cannot mask a miss.
func Check(in CheckInput) Result {
	var res Result
	pathSet := make(map[string]struct{}, len(in.Inventory))
	var scanFiles []InventoryEntry

	for _, e := range in.Inventory {
		p := path.Clean("/" + e.Path)
		p = strings.TrimPrefix(p, "/")
		if p == "." || p == "" {
			continue
		}
		pathSet[p] = struct{}{}
		if !isMarkdownPath(p) {
			continue
		}
		res.MarkdownCandidates++
		if isSymlinkMode(e.Mode) {
			res.SymlinksSkipped++
			continue
		}
		// Every non-symlink markdown candidate is link-scanned.
		scanFiles = append(scanFiles, InventoryEntry{Path: p, Mode: e.Mode})
	}

	if res.MarkdownCandidates == 0 {
		res.Errors = append(res.Errors, "vacuous: no markdown candidates in inventory")
		res.OK = false
		return res
	}

	bodies := make(map[string]string, len(scanFiles))
	anchors := make(map[string]map[string]struct{}, len(scanFiles))

	for _, e := range scanFiles {
		full := filepath.Join(in.Root, filepath.FromSlash(e.Path))
		data, err := os.ReadFile(full)
		if err != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("%s: read: %v", e.Path, err))
			continue
		}
		res.MarkdownChecked++
		body := string(data)
		bodies[e.Path] = body
		anchors[e.Path] = collectAnchors(body)

		if sizeEnforced(e.Path) {
			lines := countLines(body)
			nbytes := len(data)
			if lines > maxDocLines || nbytes > maxDocBytes {
				res.OversizedDocs++
				res.OversizedDetail = append(res.OversizedDetail,
					fmt.Sprintf("%s: %d lines, %d bytes (limits %d lines, %d bytes)",
						e.Path, lines, nbytes, maxDocLines, maxDocBytes))
			}
		}
	}

	loadAnchors := func(target string) (map[string]struct{}, bool) {
		if a, ok := anchors[target]; ok {
			return a, true
		}
		if body, ok := bodies[target]; ok {
			a := collectAnchors(body)
			anchors[target] = a
			return a, true
		}
		tf := filepath.Join(in.Root, filepath.FromSlash(target))
		data, err := os.ReadFile(tf)
		if err != nil {
			return nil, false
		}
		a := collectAnchors(string(data))
		anchors[target] = a
		bodies[target] = string(data)
		return a, true
	}

	for _, e := range scanFiles {
		body, ok := bodies[e.Path]
		if !ok {
			continue
		}
		for _, link := range extractLinks(body) {
			if isExternalLink(link.Target) {
				continue
			}
			res.LinksChecked++
			if errMsg := checkOneLink(e.Path, link, pathSet, loadAnchors); errMsg != "" {
				res.BrokenLinks++
				res.BrokenDetail = append(res.BrokenDetail, errMsg)
			}
		}
	}

	// Task paths written inside inline code — verify: bindings above all — are invisible to the
	// link scan (code is blanked before link extraction). Resolve each against the inventory so a
	// binding pointing at a moved task fails loudly instead of rotting silently (TASK-143).
	for _, e := range scanFiles {
		if body, ok := bodies[e.Path]; ok {
			for _, msg := range checkCodeTaskPaths(e.Path, body, pathSet) {
				res.BrokenLinks++
				res.BrokenDetail = append(res.BrokenDetail, msg)
			}
		}
	}

	// A `go test … -run …` binding whose pattern selects nothing exits 0, so a verify: line can
	// name a test that never existed and still read as green. Resolve each pattern against the
	// tests actually declared in the tree (TASK-136).
	testNames, testFiles, nameErrs := collectTestNames(in.Root, in.Inventory)
	res.Errors = append(res.Errors, nameErrs...)
	res.TestFilesSwept = testFiles
	res.TestFuncsFound = len(testNames)
	for _, e := range scanFiles {
		if body, ok := bodies[e.Path]; ok {
			n, msgs := checkRunPatterns(e.Path, body, testNames)
			res.RunPatternsChecked += n
			res.UnmatchedRunFlags += len(msgs)
			res.UnmatchedRunDetail = append(res.UnmatchedRunDetail, msgs...)
		}
	}

	if res.LinksChecked == 0 {
		res.Errors = append(res.Errors, "vacuous: zero links checked")
	}
	// Only a tree that has _test.go files owes test names. Finding files but no names means the
	// declaration regex stopped matching, which would pass every -run pattern silently — the one
	// way this check can be green for the wrong reason.
	if res.TestFilesSwept > 0 && res.TestFuncsFound == 0 {
		res.Errors = append(res.Errors, fmt.Sprintf("vacuous: %d _test.go file(s) swept, zero test functions found", res.TestFilesSwept))
	}
	if res.UnmatchedRunFlags > 0 {
		res.Errors = append(res.Errors, fmt.Sprintf("%d -run pattern(s) selecting no test", res.UnmatchedRunFlags))
	}
	if res.BrokenLinks > 0 {
		res.Errors = append(res.Errors, fmt.Sprintf("%d broken link(s)", res.BrokenLinks))
	}
	if res.OversizedDocs > 0 {
		res.Errors = append(res.Errors, fmt.Sprintf("%d oversized doc(s)", res.OversizedDocs))
	}

	res.OK = len(res.Errors) == 0
	return res
}

func checkOneLink(
	from string,
	link linkRef,
	pathSet map[string]struct{},
	loadAnchors func(string) (map[string]struct{}, bool),
) string {
	rawPath, anchor, err := splitLink(link.Target)
	if err != nil {
		return fmt.Sprintf("%s:%d: bad link encoding %q: %v", from, link.Line, link.Target, err)
	}

	var targetPath string
	if rawPath == "" {
		targetPath = from
	} else {
		dir := path.Dir(from)
		if dir == "." {
			targetPath = path.Clean(rawPath)
		} else {
			targetPath = path.Clean(path.Join(dir, rawPath))
		}
		if targetPath == ".." || strings.HasPrefix(targetPath, "../") {
			return fmt.Sprintf("%s:%d: link escapes repository: %q", from, link.Line, link.Target)
		}
	}

	// A link into the tasks tree whose literal target moved to another state directory resolves to
	// the file's actual location before it is declared broken. A task's identity is its number
	// (NNN-slug.md); its directory is its state (todo/done/_archive/…), which is expected to change
	// when it is worked or archived. One basename match resolves the link; zero is a genuine broken
	// link; more than one is an ambiguity the checker refuses to guess (TASK-143).
	if _, ok := pathSet[targetPath]; !ok {
		if resolved, found, ambiguous := resolveTaskLink(targetPath, pathSet); ambiguous {
			return fmt.Sprintf("%s:%d: ambiguous task link %q: basename matches several files under tasks/", from, link.Line, link.Target)
		} else if found {
			targetPath = resolved
		}
	}

	if _, ok := pathSet[targetPath]; !ok {
		if !inventoryHasPrefix(pathSet, targetPath+"/") {
			return fmt.Sprintf("%s:%d: missing target %q (from %q)", from, link.Line, targetPath, link.Target)
		}
		if anchor != "" {
			return fmt.Sprintf("%s:%d: anchor on directory target %q", from, link.Line, link.Target)
		}
		return ""
	}

	if anchor == "" {
		return ""
	}
	if !isMarkdownPath(targetPath) {
		return fmt.Sprintf("%s:%d: anchor on non-markdown target %q", from, link.Line, link.Target)
	}
	a, ok := loadAnchors(targetPath)
	if !ok {
		return fmt.Sprintf("%s:%d: cannot read anchors for %q", from, link.Line, targetPath)
	}
	if _, hit := a[anchor]; !hit {
		return fmt.Sprintf("%s:%d: missing anchor #%s in %s", from, link.Line, anchor, targetPath)
	}
	return ""
}

func inventoryHasPrefix(pathSet map[string]struct{}, prefix string) bool {
	for p := range pathSet {
		if strings.HasPrefix(p, prefix) {
			return true
		}
	}
	return false
}

// resolveTaskLink resolves a link into the tasks tree whose literal target has moved to another
// state directory. Returns the resolved path when exactly one tasks/ file shares the basename,
// found=true; found=false, ambiguous=false for zero matches (a genuine broken link, or a non-tasks
// link the caller leaves alone); ambiguous=true when the basename matches more than one file, so
// the checker can refuse rather than silently pick (TASK-143).
func resolveTaskLink(target string, pathSet map[string]struct{}) (resolved string, found bool, ambiguous bool) {
	if !strings.HasPrefix(target, "tasks/") {
		return target, false, false
	}
	base := path.Base(target)
	if !isTaskFileBasename(base) {
		return target, false, false
	}
	var hits []string
	for p := range pathSet {
		if strings.HasPrefix(p, "tasks/") && path.Base(p) == base {
			hits = append(hits, p)
		}
	}
	switch len(hits) {
	case 0:
		return target, false, false
	case 1:
		return hits[0], true, false
	default:
		return target, false, true
	}
}

// isTaskFileBasename reports whether a basename looks like a task file: one or more digits, a
// dash, then a slug — the NNN-slug.md shape every task file carries. Non-task files under tasks/
// (a README, an INDEX) are left to the literal-path check.
func isTaskFileBasename(base string) bool {
	i := 0
	for i < len(base) && base[i] >= '0' && base[i] <= '9' {
		i++
	}
	return i > 0 && i < len(base) && base[i] == '-'
}

var (
	inlineCodeSpanRe = regexp.MustCompile("`[^`]+`")
	// taskPathRe matches an absolute task path (tasks/<state>/NNN-slug.md) wherever it is written.
	taskPathRe = regexp.MustCompile(`tasks/[\w.-]+/\d+-[\w.-]+\.md`)
)

// checkCodeTaskPaths scans inline-code spans — where verify: bindings live, invisible to the link
// scan — for absolute task paths and resolves each like a moved-task link. Returns one message per
// broken or ambiguous reference. A path written in code must survive the task moving directories
// the same way a markdown link now does (TASK-143).
func checkCodeTaskPaths(from, body string, pathSet map[string]struct{}) []string {
	// Strip fenced code blocks first, the way extractLinks does: a ``` example is not an inline
	// span, but the backtick regex would otherwise match from the opening fence's third backtick
	// to the closing fence's first, leaking fenced content (e.g. a stale task path in an example)
	// into the inline scan and turning the gate red. Inline code — where verify: bindings live —
	// is preserved (TASK-143 review M1).
	body = stripFencedRegions(body)
	seen := map[string]struct{}{}
	var msgs []string
	for _, span := range inlineCodeSpanRe.FindAllString(body, -1) {
		for _, p := range taskPathRe.FindAllString(span, -1) {
			if _, dup := seen[p]; dup {
				continue
			}
			seen[p] = struct{}{}
			if _, ok := pathSet[p]; ok {
				continue // literal path still valid
			}
			_, found, ambiguous := resolveTaskLink(p, pathSet)
			switch {
			case ambiguous:
				msgs = append(msgs, fmt.Sprintf("%s: inline task path %q is ambiguous: basename matches several files under tasks/", from, p))
			case found:
				// resolves to its current state dir — valid
			default:
				msgs = append(msgs, fmt.Sprintf("%s: inline task path %q does not resolve to any tasks/ file", from, p))
			}
		}
	}
	return msgs
}

func countLines(s string) int {
	if s == "" {
		return 0
	}
	n := strings.Count(s, "\n")
	if !strings.HasSuffix(s, "\n") {
		n++
	}
	return n
}
