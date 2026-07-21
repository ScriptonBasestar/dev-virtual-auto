// Command skillgen converts canonical Agent Skills (skills/<name>/SKILL.md) into
// per-platform artifacts, driven by skills/_targets.yaml. It is the single
// converter behind `make generate`; edit the canonical skill, never a generated
// output. See skills/README.md for the format spec and field-mapping table.
//
// Shapes handled (from skills/_targets.yaml):
//   - agent-skill : symlink <output> → ../skills    (Claude Code, Antigravity, OpenCode
//                   all read SKILL.md natively; the symlink is the whole projection)
//   - mdc         : .cursor/rules/<name>.mdc         (Cursor; lazy-loaded → full body inlined)
//   - agents-md   : AGENTS.md marked section         (Codex; always-injected → pointer-only)
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// targetOverride is a per-skill, per-target override block from `x-targets:`.
type targetOverride struct {
	Globs       []string `yaml:"globs"`
	AlwaysApply *bool    `yaml:"alwaysApply"`
}

// frontmatter is the canonical SKILL.md YAML header (the superset).
type frontmatter struct {
	Name        string                    `yaml:"name"`
	Description string                    `yaml:"description"`
	XTargets    map[string]targetOverride `yaml:"x-targets"`
}

type skill struct {
	fm   frontmatter
	body string
	dir  string // repo-relative dir, e.g. "skills/dva"
}

// manifest mirrors skills/_targets.yaml.
type manifest struct {
	Defaults struct {
		Globs []string `yaml:"globs"`
	} `yaml:"defaults"`
	Targets map[string]struct {
		Shape              string `yaml:"shape"`
		Output             string `yaml:"output"`
		Ext                string `yaml:"ext"`
		Generated          bool   `yaml:"generated"`
		Marker             string `yaml:"marker"`
		DefaultAlwaysApply bool   `yaml:"default_alwaysApply"`
	} `yaml:"targets"`
}

func main() {
	root := flag.String("root", ".", "repo root (skills/ and _targets.yaml live here)")
	flag.Parse()

	man, err := loadManifest(filepath.Join(*root, "skills", "_targets.yaml"))
	if err != nil {
		fatal("load manifest: %v", err)
	}
	skills, err := loadSkills(filepath.Join(*root, "skills"))
	if err != nil {
		fatal("load skills: %v", err)
	}
	if len(skills) == 0 {
		fatal("no skills found under skills/")
	}

	// Emit each target. agents-md targets share one output file (AGENTS.md),
	// so collect them and write once.
	agentsMdDone := map[string]bool{}
	for _, tname := range sortedKeys(man.Targets) {
		t := man.Targets[tname]
		switch t.Shape {
		case "agent-skill":
			// The platform reads SKILL.md directly; the projection is a symlink
			// to canonical skills/. Idempotent: no diff when already correct,
			// self-heals a missing link (e.g. on a fresh clone where the dir is
			// gitignored). This is what makes `make generate` reproduce the tree.
			if err := ensureSymlink(*root, t.Output); err != nil {
				fatal("symlink %s: %v", tname, err)
			}
			fmt.Printf("skillgen: %-9s -> %s -> ../skills (symlink)\n", tname, t.Output)
		case "mdc":
			for _, s := range skills {
				if err := emitCursor(*root, s, man, t.Output, t.Ext, t.DefaultAlwaysApply); err != nil {
					fatal("cursor %s: %v", s.fm.Name, err)
				}
			}
			fmt.Printf("skillgen: %-9s -> %s*%s (%d skills)\n", tname, t.Output, t.Ext, len(skills))
		case "agents-md":
			if agentsMdDone[t.Output] {
				continue
			}
			if err := emitAgentsMD(filepath.Join(*root, t.Output), skills, t.Marker); err != nil {
				fatal("agents-md %s: %v", t.Output, err)
			}
			agentsMdDone[t.Output] = true
			fmt.Printf("skillgen: %-9s -> %s (marked section, %d skills)\n", tname, t.Output, len(skills))
		default:
			fmt.Printf("skillgen: %-9s -> shape %q not yet implemented (skipped)\n", tname, t.Shape)
		}
	}
}

// emitCursor writes .cursor/rules/<name>.mdc. Cursor rules are lazy-loaded on
// glob match, so the full skill body is inlined (Claude-only constructs stripped,
// relative reference paths rewritten to repo-root paths).
func emitCursor(root string, s skill, man manifest, outDir, ext string, defaultAlways bool) error {
	globs := man.Defaults.Globs
	always := defaultAlways
	if ov, ok := s.fm.XTargets["cursor"]; ok {
		if len(ov.Globs) > 0 {
			globs = ov.Globs
		}
		if ov.AlwaysApply != nil {
			always = *ov.AlwaysApply
		}
	}

	body := rewriteRefPaths(stripClaudeDynamics(s.body), s.dir)
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "description: %s\n", oneLine(s.fm.Description))
	fmt.Fprintf(&b, "globs: %s\n", strings.Join(globs, ","))
	fmt.Fprintf(&b, "alwaysApply: %t\n", always)
	b.WriteString("---\n\n")
	fmt.Fprintf(&b, "<!-- GENERATED from %s/SKILL.md by tools/skillgen — do not edit; edit the canonical skill and run `make generate` -->\n\n", s.dir)
	b.WriteString(strings.TrimRight(body, "\n"))
	b.WriteString("\n")

	out := filepath.Join(root, filepath.FromSlash(outDir), s.fm.Name+ext)
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	return os.WriteFile(out, []byte(b.String()), 0o644)
}

// emitAgentsMD replaces (or appends) a marked block in AGENTS.md. The block is
// pointer-only: AGENTS.md is always-injected context, so per the degradation
// policy each skill contributes a description + link, never its full body.
func emitAgentsMD(path string, skills []skill, marker string) error {
	if marker == "" {
		marker = "skills:auto"
	}
	start := fmt.Sprintf("<!-- %s:start -->", marker)
	end := fmt.Sprintf("<!-- %s:end -->", marker)

	var sec strings.Builder
	sec.WriteString(start)
	sec.WriteString("\n## AI Skills\n\n")
	sec.WriteString("Generated from `skills/` by `tools/skillgen` — do not edit this block; edit the canonical skill and run `make generate`. Open the linked `SKILL.md` on demand for full guidance.\n\n")
	for _, s := range skills {
		fmt.Fprintf(&sec, "- **%s** — %s See `%s/SKILL.md`.\n", s.fm.Name, oneLine(s.fm.Description), s.dir)
	}
	sec.WriteString(end)

	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	content := string(existing)
	switch {
	case strings.Contains(content, start) && strings.Contains(content, end):
		pre := content[:strings.Index(content, start)]
		post := content[strings.Index(content, end)+len(end):]
		content = pre + sec.String() + post
	case content == "":
		content = sec.String() + "\n"
	default:
		content = strings.TrimRight(content, "\n") + "\n\n" + sec.String() + "\n"
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

// ensureSymlink makes `output` a relative symlink to canonical skills/. It is
// idempotent (correct link → no-op) and refuses to clobber a real directory.
func ensureSymlink(root, output string) error {
	link := filepath.Join(root, filepath.FromSlash(output))
	target, err := filepath.Rel(filepath.Dir(link), filepath.Join(root, "skills"))
	if err != nil {
		return err
	}
	if cur, err := os.Readlink(link); err == nil {
		if cur == target {
			return nil // already correct
		}
	} else if fi, err := os.Lstat(link); err == nil && fi.Mode()&os.ModeSymlink == 0 {
		return fmt.Errorf("%s exists and is not a symlink; refusing to replace", output)
	}
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		return err
	}
	_ = os.Remove(link) // clear a stale/incorrect symlink, if any
	return os.Symlink(target, link)
}

// ── parsing helpers ──────────────────────────────────────────────────────────

func loadManifest(path string) (manifest, error) {
	var m manifest
	data, err := os.ReadFile(path)
	if err != nil {
		return m, err
	}
	return m, yaml.Unmarshal(data, &m)
}

func loadSkills(dir string) ([]skill, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var skills []skill
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		p := filepath.Join(dir, e.Name(), "SKILL.md")
		if _, err := os.Stat(p); err != nil {
			continue // not a skill dir
		}
		fm, body, err := parseSkill(p)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		skills = append(skills, skill{fm: fm, body: body, dir: "skills/" + e.Name()})
	}
	sort.Slice(skills, func(i, j int) bool { return skills[i].fm.Name < skills[j].fm.Name })
	return skills, nil
}

// parseSkill splits YAML frontmatter (delimited by --- lines) from the body.
func parseSkill(path string) (frontmatter, string, error) {
	var fm frontmatter
	data, err := os.ReadFile(path)
	if err != nil {
		return fm, "", err
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return fm, "", fmt.Errorf("missing frontmatter opening ---")
	}
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end == -1 {
		return fm, "", fmt.Errorf("missing frontmatter closing ---")
	}
	if err := yaml.Unmarshal([]byte(strings.Join(lines[1:end], "\n")), &fm); err != nil {
		return fm, "", err
	}
	if fm.Name == "" || fm.Description == "" {
		return fm, "", fmt.Errorf("frontmatter requires name and description")
	}
	body := strings.TrimLeft(strings.Join(lines[end+1:], "\n"), "\n")
	return fm, body, nil
}

// ── body transforms ──────────────────────────────────────────────────────────

var claudeDynamicLine = regexp.MustCompile("(?m)^!`.*`\\s*$")
var relRefPath = regexp.MustCompile("`(references/|assets/)")

// stripClaudeDynamics removes Claude-Code `!`cmd“ dynamic-injection lines (which
// render as literal text elsewhere) and then prunes any section heading those
// lines left empty (e.g. "## Project Context" whose only content was `!`dva show“).
func stripClaudeDynamics(body string) string {
	out := claudeDynamicLine.ReplaceAllString(body, "")
	out = removeEmptyHeadings(out)
	return regexp.MustCompile(`\n{3,}`).ReplaceAllString(out, "\n\n")
}

// removeEmptyHeadings drops a heading whose section has no content — detected by
// the next non-blank line being a heading of equal-or-shallower level, or EOF. A
// heading followed by a deeper heading (a real subsection) is kept.
func removeEmptyHeadings(body string) string {
	level := func(s string) int {
		s = strings.TrimSpace(s)
		n := 0
		for n < len(s) && s[n] == '#' {
			n++
		}
		if n > 0 && (n == len(s) || s[n] == ' ') {
			return n
		}
		return 0
	}
	lines := strings.Split(body, "\n")
	drop := make(map[int]bool)
	for i, ln := range lines {
		lvl := level(ln)
		if lvl == 0 {
			continue
		}
		j := i + 1
		for j < len(lines) && strings.TrimSpace(lines[j]) == "" {
			j++
		}
		if j >= len(lines) || (level(lines[j]) != 0 && level(lines[j]) <= lvl) {
			drop[i] = true
		}
	}
	out := make([]string, 0, len(lines))
	for i, ln := range lines {
		if !drop[i] {
			out = append(out, ln)
		}
	}
	return strings.Join(out, "\n")
}

// rewriteRefPaths turns skill-relative `references/…`/`assets/…` links into
// repo-root paths so they resolve from a generated file elsewhere in the tree.
func rewriteRefPaths(body, skillDir string) string {
	return relRefPath.ReplaceAllString(body, "`"+skillDir+"/$1")
}

// oneLine collapses any whitespace (including newlines from YAML folded scalars)
// into single spaces — required for single-line frontmatter/list fields.
func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }

func sortedKeys[V any](m map[string]V) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

func fatal(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "skillgen: "+format+"\n", a...)
	os.Exit(1)
}
