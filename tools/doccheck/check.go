package main

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
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
	Errors             []string
	BrokenDetail       []string
	OversizedDetail    []string
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

	if res.LinksChecked == 0 {
		res.Errors = append(res.Errors, "vacuous: zero links checked")
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
