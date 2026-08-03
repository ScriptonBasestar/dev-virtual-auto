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
	versionFieldRefRe   = regexp.MustCompile("`version`|version 필드|version field")
	runningVersionVarRe = regexp.MustCompile(`\{\{[^}]*dva_version[^}]*\}\}`)

	// A deterministic rewrite of the field. No flow has business editing a target's
	// floor, so the value being written does not matter — the rewrite itself is the
	// defect. dva-improve's fix_version step was exactly this.
	versionRewriteRe = regexp.MustCompile(`\bsed\b.*version:`)

	// The rule restated as prose. Bilingual because TASK-067's English-only sweep is
	// what let the Korean restatement through.
	runningVersionProseKoRe = regexp.MustCompile(`현재\s*DVA[^\n]{0,20}버전`)
	runningVersionProseEnRe = regexp.MustCompile(`(?i)(matches|must match|equal)[^\n]{0,20}current DVA`)
)

// versionRuleViolation names the rule a line breaks, or "" if it is clean.
//
// yamlComment lines are skipped because a YAML comment is addressed to a
// maintainer and never reaches the LLM's prompt. Markdown `#` lines are headings,
// which do, so the caller must not treat those as comments.
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
			for i, line := range strings.Split(string(body), "\n") {
				scannedLines++
				comment := yaml && strings.HasPrefix(strings.TrimSpace(line), "#")
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
	}
	for _, a := range allowed {
		comment := a.yaml && strings.HasPrefix(strings.TrimSpace(a.line), "#")
		if rule := versionRuleViolation(a.line, comment); rule != "" {
			t.Errorf("guard false-positived on %s (%s):\n\t%s", a.name, rule, a.line)
		}
	}
}
