package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode"
)

var (
	// runFlagRe captures a -run argument exactly as written, quotes and backslashes included,
	// because unquoting is what decides which pattern go test actually receives (shellUnquote).
	// -run must start a word: this is what keeps --dry-run and the prose "re-run" out, so the
	// denominator counts bindings rather than every hyphenated word in the corpus.
	runFlagRe = regexp.MustCompile(`(?:^|\s)-run(?:=|\s+)('[^']*'|"[^"]*"|(?:\\.|[^\s'"])+)`)

	// testFuncDeclRe matches a top-level test declaration. The *testing.T requirement is doing
	// work: it excludes TestMain(m *testing.M), which -run cannot select, and fuzz targets.
	testFuncDeclRe = regexp.MustCompile(`(?m)^func (Test[A-Za-z0-9_]*)\([^)]*\*testing\.T[^)]*\)`)
)

// collectTestNames returns every top-level test in the inventory that -run could select, plus
// the count of files swept so the caller can tell "this repo has no Go tests" from "the
// declaration regex stopped matching". Source is read rather than compiled: `go test -list`
// would need a build, and would miss internal/integration behind its -tags=integration guard.
func collectTestNames(root string, inv []InventoryEntry) (names []string, filesSwept int, errs []string) {
	seen := map[string]struct{}{}
	for _, e := range inv {
		if !strings.HasSuffix(e.Path, "_test.go") || isSymlinkMode(e.Mode) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(e.Path)))
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: read: %v", e.Path, err))
			continue
		}
		filesSwept++
		for _, m := range testFuncDeclRe.FindAllStringSubmatch(string(data), -1) {
			if !isRunnableTestName(m[1]) {
				continue
			}
			if _, dup := seen[m[1]]; dup {
				continue
			}
			seen[m[1]] = struct{}{}
			names = append(names, m[1])
		}
	}
	return names, filesSwept, errs
}

// isRunnableTestName applies go test's own naming rule: the character after "Test" must not be
// lower case, so TestingHelper is a helper the flag cannot reach and TestFoo is a test it can.
func isRunnableTestName(name string) bool {
	rest := name[len("Test"):]
	if rest == "" {
		return true
	}
	return !unicode.IsLower([]rune(rest)[0])
}

// checkRunPatterns reports every `go test … -run …` binding written in inline code whose pattern
// selects no test in the repository. Such a command prints "no tests to run" and exits 0, so a
// verify: binding that names a test which never existed — or that quotes its alternation so the
// pipe reaches Go escaped — passes every gate while proving nothing (TASK-136).
//
// Scope is the whole repository rather than the packages the command names. A pattern that
// matches somewhere but not in ./internal/config/ is still a broken binding, but judging that
// needs a second parser for go test's package-argument grammar (./..., bare ./pkg, several
// packages, -tags changing which files compile) inside a documentation checker. Repo-wide
// catches the class this task was filed for; narrowing it later is additive.
func checkRunPatterns(from, body string, names []string) (checked int, msgs []string) {
	// Fenced blocks first, as checkCodeTaskPaths does: an illustrative ``` example is not a
	// binding, and the inline-span regex would otherwise read across the fence markers.
	body = stripFencedRegions(body)
	for _, loc := range inlineCodeSpanRe.FindAllStringIndex(body, -1) {
		span := body[loc[0]+1 : loc[1]-1]
		if !strings.Contains(span, "go test") {
			continue
		}
		line := lineAt(body, loc[0])
		if inTableRow(body, loc[0]) {
			// GFM processes exactly one backslash escape inside a code span, and only in a
			// table: \| is the only way to put a pipe in a cell. Judging the source bytes here
			// would flag -run 'A\|B' — which readers see as A|B and which runs correctly — while
			// unescaping outside a table would hide a real defect, since there \| survives single
			// quotes and reaches Go's regexp as a literal pipe.
			span = strings.ReplaceAll(span, `\|`, "|")
		}
		for _, m := range runFlagRe.FindAllStringSubmatch(span, -1) {
			checked++
			pattern := shellUnquote(m[1])
			ok, err := runPatternSelects(pattern, names)
			switch {
			case err != nil:
				msgs = append(msgs, fmt.Sprintf("%s:%d: -run %s is not a valid regexp: %v", from, line, m[1], err))
			case !ok:
				msgs = append(msgs, fmt.Sprintf("%s:%d: -run %s selects no test (go test would print \"no tests to run\" and exit 0)", from, line, m[1]))
			}
		}
	}
	return checked, msgs
}

// inTableRow reports whether the offset sits on a GFM table row — a line whose first non-space
// character is a pipe. That is the only context where \| inside a code span is markdown escaping
// rather than text, so it is the only context where the checker may rewrite it.
func inTableRow(body string, offset int) bool {
	start := strings.LastIndexByte(body[:offset], '\n') + 1
	return strings.HasPrefix(strings.TrimLeft(body[start:], " \t"), "|")
}

// runPatternSelects reports whether pattern selects at least one of names under go test's rule:
// the pattern is split on /, and each element is an unanchored regexp matched against the
// corresponding element of the test name. Only the first element is judged here — the rest name
// subtests, which exist at run time and cannot be read out of the source.
func runPatternSelects(pattern string, names []string) (bool, error) {
	first, _, _ := strings.Cut(pattern, "/")
	re, err := regexp.Compile(first)
	if err != nil {
		return false, err
	}
	if slices.ContainsFunc(names, re.MatchString) {
		return true, nil
	}
	return false, nil
}

// shellUnquote resolves one shell word into the argument go test receives. The distinction is
// load-bearing: inside single quotes a backslash is an ordinary character, so -run 'A\|B' reaches
// Go's regexp as A\|B — an escaped pipe, which matches the literal name "A|B" and therefore
// nothing. Unquoted, the shell eats the backslash and Go sees the alternation A|B.
// tasks/_archive/059 wrote the first form and recorded the second form's results.
func shellUnquote(raw string) string {
	switch {
	case len(raw) >= 2 && raw[0] == '\'' && raw[len(raw)-1] == '\'':
		return raw[1 : len(raw)-1]
	case len(raw) >= 2 && raw[0] == '"' && raw[len(raw)-1] == '"':
		// Inside double quotes a backslash is literal except before these four.
		return unescape(raw[1:len(raw)-1], "$`\"\\")
	default:
		return unescape(raw, "")
	}
}

// unescape drops a backslash that precedes any character in special. An empty special means
// every character, which is the unquoted-word rule.
func unescape(s, special string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) && (special == "" || strings.ContainsRune(special, rune(s[i+1]))) {
			i++
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
