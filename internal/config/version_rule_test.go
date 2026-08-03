package config

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// versionRuleCorpus is every file that tells an AI what to put in a config's
// `version:` field: the agent-mesh flows that write and edit real dva.yml files,
// the portable skills, and the reference text make generate embeds.
//
// MinScaffoldVersion's doc comment states the rule — `version:` is what a config
// requires of its *reader*, so scaffolding the running version ratchets every
// target's floor upward. Nothing compiles the prose that teaches it. TASK-067 fixed
// the rule statement in the library and pinned it with a grep, but scoped that grep
// to agent-mesh-flows/shared/library/, so dva-improve.yaml — the flow that actually
// rewrites configs — kept instructing the opposite for another eight tasks
// (TASK-135). The scope, not the pattern, is what let it survive.
func versionRuleCorpus() []string {
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(file), "..", "..")
	return []string{
		filepath.Join(root, "agent-mesh-flows"),
		filepath.Join(root, "skills"),
		filepath.Join(root, "internal", "cli", "library_reference.txt"),
	}
}

var (
	// A write instruction: a reference to the `version` field on the same line as an
	// interpolation of the running binary's version. This is the form prose-only greps
	// miss — three of the four dva-improve.yaml offenders named no version at all,
	// just the template variable. The reference is matched unbackticked too, because
	// 30-configure.yaml's offender wrote a bare `version 필드` and a backtick-only
	// pattern let it through.
	//
	// The interpolation half matches any {{…version…}}, not just the `dva_version` key
	// the known offenders happened to use: the defect is a version value flowing into
	// the field, and a flow that renamed its context key to `cli_version` would commit
	// the same one. Widening is safe because the field-reference half carries the
	// specificity — a line that merely reports a version ("DVA 버전: {{…}}") names no
	// field and is not matched.
	versionFieldRefRe   = regexp.MustCompile("`version`|version 필드|version field")
	runningVersionVarRe = regexp.MustCompile(`\{\{[^}]*version[^}]*\}\}`)

	// A deterministic rewrite of the field. No flow has business editing a target's
	// floor, so the value being written does not matter — the rewrite itself is the
	// defect. dva-improve's fix_version step was exactly this, with sed.
	//
	// The tool list is not the rule; it is how far a line-based guard can see. The rule
	// is "no flow rewrites a target's `version:`", and yq/perl/awk reach it as readily as
	// sed did. `version` is left-anchored on a non-word, non-dash character so a
	// legitimate `sed … schema_version:` or `api-version:` does not trip a message that
	// would name the wrong field.
	versionRewriteRe = regexp.MustCompile(`\b(?:sed|perl|awk|yq)\b.*(?:[^\w-]|\A)version\s*[:=]`)

	// The rule restated as prose. Bilingual because TASK-067's English-only sweep is
	// what let the Korean restatement through.
	runningVersionProseKoRe = regexp.MustCompile(`현재\s*DVA[^\n]{0,20}버전`)
	runningVersionProseEnRe = regexp.MustCompile(`(?i)(matches|must match|equal)[^\n]{0,20}current DVA`)
)

// versionRuleViolation names the rule a line breaks, or "" if it is clean.
//
// yamlComment lines are skipped because a YAML comment is addressed to a maintainer
// and never reaches the LLM's prompt. Deciding that is the caller's job and it is not
// a prefix test — see yamlCommentScanner.
func versionRuleViolation(line string, yamlComment bool) string {
	if yamlComment {
		return ""
	}
	switch {
	case versionFieldRefRe.MatchString(line) && runningVersionVarRe.MatchString(line):
		return "instructs writing the running binary's version into `version:`"
	case versionRewriteRe.MatchString(line):
		return "rewrites a target config's `version:`"
	case runningVersionProseKoRe.MatchString(line):
		return "states the version rule as \"current DVA version\" (ko)"
	case runningVersionProseEnRe.MatchString(line):
		return "states the version rule as \"current DVA version\" (en)"
	}
	return ""
}

func isYamlPath(path string) bool {
	return strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml")
}

// blockScalarOpenRe matches a mapping key whose value is a `|` or `>` block scalar,
// with the optional chomping/indentation indicators and a trailing comment.
var blockScalarOpenRe = regexp.MustCompile(`^\s*(?:-\s+)?[^#\s][^:]*:\s*[|>][-+0-9]*\s*(?:#.*)?$`)

// yamlCommentScanner decides, line by line, whether a `#`-prefixed line in a YAML file
// is inert.
//
// A prefix test alone gets this wrong, and wrong in the direction that disables the
// guard. Every prompt in agent-mesh-flows/ is written as markdown inside an
// `instruction: |` block scalar, so its headings — `# Role & Objective`,
// `## CRITICAL: version 필드` — are `#`-prefixed lines that reach the LLM verbatim.
// dva-improve.yaml:741 is one today. Treating those as comments would let a restatement
// of exactly the anti-pattern TASK-135 removed pass the guard, and the current fix sits
// one edit away from that: `## 0. MANDATORY: version 필드는 그대로 둔다` is a heading
// whose body could be folded up into it.
//
// So a `#` line counts as a comment only outside a block scalar. The heuristic errs
// toward strict: a misread that keeps the scanner "inside" a block only causes more
// lines to be examined, never fewer.
type yamlCommentScanner struct {
	// blockIndent is the leading-whitespace width of the line that opened the current
	// block scalar, or -1 when no block is open. Content is always indented past it.
	blockIndent int
}

func newYamlCommentScanner() *yamlCommentScanner {
	return &yamlCommentScanner{blockIndent: -1}
}

func (s *yamlCommentScanner) isComment(line string) bool {
	trimmed := strings.TrimSpace(line)
	indent := len(line) - len(strings.TrimLeft(line, " \t"))

	if s.blockIndent >= 0 {
		// A blank line inside a block scalar is a paragraph break, not the end of it.
		if trimmed == "" || indent > s.blockIndent {
			return false
		}
		s.blockIndent = -1 // dedented back out; fall through and read this line as YAML
	}
	if blockScalarOpenRe.MatchString(line) {
		s.blockIndent = indent
		return false
	}
	return strings.HasPrefix(trimmed, "#")
}

// TestGeneratorCorpusDoesNotScaffoldTheRunningVersion is the guard TASK-067's grep
// should have been: same class, wider scope, both languages.
func TestGeneratorCorpusDoesNotScaffoldTheRunningVersion(t *testing.T) {
	var scannedFiles, scannedLines int

	for _, target := range versionRuleCorpus() {
		err := filepath.Walk(target, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			if !isYamlPath(path) && !strings.HasSuffix(path, ".md") && !strings.HasSuffix(path, ".txt") {
				return nil
			}
			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			scannedFiles++
			yaml := isYamlPath(path)
			scanner := newYamlCommentScanner()
			for i, line := range strings.Split(string(body), "\n") {
				scannedLines++
				comment := yaml && scanner.isComment(line)
				if rule := versionRuleViolation(line, comment); rule != "" {
					t.Errorf("%s:%d %s\n\t%s\n\t`version:` is the minimum DVA a config requires of its reader, "+
						"not a stamp of the binary that wrote it. Leave an existing value alone; omit it when absent. "+
						"See config.MinScaffoldVersion.", path, i+1, rule, strings.TrimSpace(line))
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walking %s: %v", target, err)
		}
	}

	// A guard that silently scans nothing reports the same green as a clean corpus.
	if scannedFiles == 0 || scannedLines == 0 {
		t.Fatalf("scanned %d files / %d lines — the corpus paths are wrong, so this test proves nothing",
			scannedFiles, scannedLines)
	}
	t.Logf("scanned %d files / %d lines", scannedFiles, scannedLines)
}

// TestVersionRuleGuardCatchesTheKnownOffenders feeds the guard the exact lines it
// was written for. Without it the guard above is unfalsifiable: it passes on a
// clean tree whether or not its patterns match anything at all.
func TestVersionRuleGuardCatchesTheKnownOffenders(t *testing.T) {
	// Verbatim from dva-improve.yaml and 30-configure.yaml before TASK-135.
	offenders := []struct {
		name string
		line string
	}{
		{"contract expected_results", `    - "버전이 현재 DVA CLI 버전과 일치한다"`},
		{"phase 3 instruction", "      - `version` 필드는 반드시 **현재 DVA 버전 `{{check_prerequisites.dva_version | trim}}`**로 설정하세요."},
		{"phase 4 instruction", "      `version` 필드를 반드시 `{{check_prerequisites.dva_version | trim}}`로 설정하세요."},
		{"self-review checklist", "      - [ ] `version` 필드가 `{{check_prerequisites.dva_version | trim}}`인가?"},
		{"guided write path", "      version 필드는 반드시 `{{detect_track.dva_version | trim}}`로 설정."},
		{"fix_version sed", `      sed -i "s/^version: *\".*\"/version: \"$EXPECTED\"/" "$DVA_FILE"`},
		{"english restatement", "**`version:` field** — Must match the current DVA CLI version."},

		// Near-misses of the two forms above. Neither was ever in the tree; both are one
		// plausible edit away from the lines that were, and the patterns they defeat are
		// the ones this guard shipped with. TASK-135 review.
		{"renamed context key", "      `version` 필드는 `{{detect_track.cli_version | trim}}`로 설정하세요."},
		{"rewrite via yq", `      yq -i ".version = \"$EXPECTED\"" "$DVA_FILE"`},
		{"rewrite via perl", `      perl -pi -e "s/^version:.*/version: $EXPECTED/" "$DVA_FILE"`},
	}
	for _, o := range offenders {
		if versionRuleViolation(o.line, false) == "" {
			t.Errorf("guard missed the %s offender:\n\t%s", o.name, o.line)
		}
	}

	// Lines that name the running version for a legitimate reason must survive, or
	// the guard gets disabled the first time it cries wolf.
	allowed := []struct {
		name string
		line string
		yaml bool
	}{
		{"schema-compat instruction", "      4. DVA schema {{check_prerequisites.dva_version | trim}} 호환 필수.", true},
		{"context key definition", `      dva_version: "dva version --json 2>/dev/null | jq -r '.version // \"unknown\"'"`, true},
		{"report line", "      - DVA 버전: {{check_dva.dva_version}}", true},
		{"corrected rule statement", "- `version` is optional or set to the reader floor — never the running CLI version", false},
		{"yaml comment describing the removed step", "  # default. The step that used to live here sed-rewrote `^version:` to the running", true},

		// The widened rewrite pattern has to stay off neighbouring field names, or it
		// reports "rewrites a target config's `version:`" about a line that does no such
		// thing — and a guard that names the wrong field gets deleted.
		{"sed on a different version-suffixed key", `      sed -i "s/^schema_version:.*/schema_version: 2/" "$f"`, true},
		{"sed on a hyphenated key", `      sed -i "s/api-version:.*/api-version: v2/" "$f"`, true},
	}
	for _, a := range allowed {
		comment := a.yaml && newYamlCommentScanner().isComment(a.line)
		if rule := versionRuleViolation(a.line, comment); rule != "" {
			t.Errorf("guard false-positived on %s (%s):\n\t%s", a.name, rule, a.line)
		}
	}
}

// TestYamlCommentScannerDoesNotSwallowPromptHeadings is the falsification for the
// comment classifier. The corpus scan cannot catch a mistake here: misclassifying a
// heading makes the guard skip lines, which reads as the same green as a clean tree.
func TestYamlCommentScannerDoesNotSwallowPromptHeadings(t *testing.T) {
	// Shaped like every prompt-bearing step in agent-mesh-flows/: a `|` block scalar
	// whose content is markdown.
	doc := []struct {
		line    string
		comment bool
	}{
		{"steps:", false},
		{"  # a real comment, addressed to a maintainer", true},
		{"  - id: rewrite", false},
		{"    instruction: |", false},
		{"      # Phase 6.5: Semantic Warning Triage", false}, // heading; dva-improve.yaml:741
		{"", false}, // paragraph break, still inside
		{"      ## MANDATORY: version 필드", false},          // heading the guard must read
		{"      Set it to {{detect.dva_version}}.", false}, // ordinary prose
		{"    action: shell", false},                       // dedent ends the block
		{"    # a comment again, outside the scalar", true},
	}

	scanner := newYamlCommentScanner()
	for i, want := range doc {
		if got := scanner.isComment(want.line); got != want.comment {
			t.Errorf("line %d: isComment(%q) = %v, want %v", i+1, want.line, got, want.comment)
		}
	}

	// The point of all of the above, end to end: a write instruction folded into a
	// heading must still be caught. Before TASK-135's review it was not.
	inBlock := newYamlCommentScanner()
	inBlock.isComment("    instruction: |")
	offender := "      ## Set `version` to {{check.dva_version | trim}}"
	if versionRuleViolation(offender, inBlock.isComment(offender)) == "" {
		t.Errorf("a markdown heading inside a block scalar carried a write instruction and the guard skipped it:\n\t%s", offender)
	}
}
