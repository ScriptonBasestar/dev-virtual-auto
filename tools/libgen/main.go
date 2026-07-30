// Command libgen regenerates Go-sourced fact blocks inside
// agent-mesh-flows/shared/library/shared-guardrails.md so the markdown stays in
// sync with the Go source of truth (internal/config). Run via `make generate`.
//
// Facts sourced from Go:
//   - reserved + hookable commands → internal/config/reserved.go
//   - canonical section order       → internal/config/validate_warnings.go
//   - the `version:` rule           → internal/config/version.go
//
// Facts that live only in markdown (naming presets, forbidden ports, schema doc)
// are intentionally NOT touched here — see shared/library/README.md.
package main

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/ScriptonBasestar/dva/internal/config"
)

const guardrailsPath = "agent-mesh-flows/shared/library/shared-guardrails.md"

func main() {
	content, err := os.ReadFile(guardrailsPath)
	if err != nil {
		fail(err)
	}

	reserved := sortedKeys(config.ReservedCommands())
	hookable := sortedKeys(config.HookableCommands())
	section := config.CanonicalSectionOrder()

	out := string(content)
	out, err = replaceBlock(out, "reserved_commands", renderReserved(reserved, hookable))
	if err != nil {
		fail(err)
	}
	out, err = replaceBlock(out, "section_order", renderSection(section))
	if err != nil {
		fail(err)
	}
	out, err = replaceBlock(out, "version_rule", renderVersion(config.MinScaffoldVersion))
	if err != nil {
		fail(err)
	}

	if out == string(content) {
		fmt.Println("libgen:", guardrailsPath, "already up-to-date")
		return
	}
	if err := os.WriteFile(guardrailsPath, []byte(out), 0o644); err != nil {
		fail(err)
	}
	fmt.Println("libgen: updated", guardrailsPath)
}

// renderReserved formats the reserved and hookable command lists. Sorted for
// stable output (Go maps are unordered).
func renderReserved(reserved, hookable []string) string {
	return fmt.Sprintf("Reserved (%d, must not be interaction keys): %s\nHookable (use `replace:` hooks, %d): %s",
		len(reserved), backtickList(reserved),
		len(hookable), backtickList(hookable))
}

// renderSection formats the canonical section order joined by arrows.
func renderSection(order []string) string {
	return backtickListArrow(order)
}

// renderVersion formats the `version:` rule. The floor is the only version fact prose
// should carry: MinScaffoldVersion is a const that changes when init's output stops
// loading on older DVA, whereas Version changes every release — so pinning generated
// configs to Version is exactly what version.go exists to prevent.
func renderVersion(floor string) string {
	return fmt.Sprintf("Omit `version:` for no compatibility gate. When declaring it, use `%s` — the floor `dva init` writes. "+
		"Never scaffold the running CLI version: that makes every generated config refuse to load on an older DVA, "+
		"ratcheting the floor up on each release. Subproject `version:` is checked against the running DVA "+
		"independently; DVA never compares a subproject's version to root, so do not require them to agree.", floor)
}

// replaceBlock swaps the content between a named AUTOGEN marker pair. The leading
// indentation of the start marker is captured ($1) and re-applied to every
// generated line, so a block indented inside a list item stays indented.
func replaceBlock(content, marker, body string) (string, error) {
	startTok := fmt.Sprintf("<!-- AUTOGEN:%s:start -->", marker)
	endTok := fmt.Sprintf("<!-- AUTOGEN:%s:end -->", marker)
	pattern := `([ \t]*)` + regexp.QuoteMeta(startTok) + `[\s\S]*?` + regexp.QuoteMeta(endTok)
	re := regexp.MustCompile(pattern)
	if !re.MatchString(content) {
		return "", fmt.Errorf("libgen: marker %q not found in %s — install markers first", marker, guardrailsPath)
	}

	lines := strings.Split(body, "\n")
	var b strings.Builder
	// ${1} (not $1) delimits the capture group from a following letter/digit —
	// otherwise Go's regexp treats "$1Reserved" as an unnamed group "1Reserved"
	// and expands it to empty, eating the "Reserved" prefix.
	b.WriteString("${1}")
	b.WriteString(startTok)
	for _, line := range lines {
		b.WriteString("\n${1}")
		b.WriteString(line)
	}
	b.WriteString("\n${1}")
	b.WriteString(endTok)
	return re.ReplaceAllString(content, b.String()), nil
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func backtickList(items []string) string {
	quoted := make([]string, len(items))
	for i, v := range items {
		quoted[i] = "`" + v + "`"
	}
	return strings.Join(quoted, ", ")
}

func backtickListArrow(items []string) string {
	quoted := make([]string, len(items))
	for i, v := range items {
		quoted[i] = "`" + v + "`"
	}
	return strings.Join(quoted, " → ")
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
