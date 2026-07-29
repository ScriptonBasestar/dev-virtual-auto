package cli

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

//go:embed dva_guide_template.txt
var dvaGuideTemplate string

const dvaGuideFilename = "dva-guide.md"

// agentConfigSnippet is the section appended to CLAUDE.md / AGENTS.md.
const agentConfigSnippet = `
## DVA (Dev Virtual Auto)

This project uses DVA for all build, test, and run operations.
Always use DVA commands instead of raw docker compose or language-specific tools.

- Usage guide: [%s](%s)
- Available commands: run ` + "`dva ls`" + ` or ` + "`dva manifest -f json`" + `
`

// generateAIDocs creates the DVA guide and updates CLAUDE.md/AGENTS.md.
// It returns the path to the generated guide file.
func generateAIDocs() (string, error) {
	docsDir := detectDocsDir()

	guidePath := filepath.Join(docsDir, dvaGuideFilename)

	if err := os.WriteFile(guidePath, []byte(dvaGuideTemplate), 0644); err != nil {
		return "", fmt.Errorf("failed to write %s: %w", guidePath, err)
	}

	// Update CLAUDE.md and AGENTS.md with reference to the guide
	updated := false
	for _, agentFile := range []string{"CLAUDE.md", "AGENTS.md"} {
		if _, err := os.Stat(agentFile); os.IsNotExist(err) {
			continue
		}
		if err := upsertDVASection(agentFile, guidePath); err != nil {
			fmt.Fprintf(os.Stderr, "⚠️  Could not update %s: %v\n", agentFile, err)
		} else {
			updated = true
		}
	}

	if !updated {
		fmt.Println("💡 CLAUDE.md / AGENTS.md not found. Create AGENTS.md to let AI agents discover DVA automatically:")
		fmt.Printf("   echo '# AGENTS.md' > AGENTS.md && dva config docs\n")
	}

	return guidePath, nil
}

// detectDocsDir finds the existing docs directory or creates one.
// Priority: docs/ → doc/ → create docs/
func detectDocsDir() string {
	candidates := []string{"docs", "doc"}

	for _, dir := range candidates {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir
		}
	}

	// No docs directory found — create docs/
	if err := os.MkdirAll("docs", 0755); err != nil {
		// Fallback: write to current directory
		fmt.Fprintf(os.Stderr, "⚠️  Could not create docs/ directory: %v (writing to current directory)\n", err)
		return "."
	}

	fmt.Println("📁 Created docs/ directory for DVA guide")
	return "docs"
}

// upsertDVASection adds or updates the DVA section in an agent config file.
func upsertDVASection(filename string, guidePath string) error {
	snippet := fmt.Sprintf(agentConfigSnippet, guidePath, guidePath)

	data, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist — skip (don't create agent config files unprompted)
			return nil
		}
		return err
	}

	content := string(data)

	// Check if DVA section already exists
	if strings.Contains(content, "## DVA (Dev Virtual Auto)") {
		// Replace existing section
		return replaceDVASection(filename, content, snippet)
	}

	// Append new section
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += snippet

	return os.WriteFile(filename, []byte(content), 0644)
}

// replaceDVASection replaces an existing DVA section in the file content.
func replaceDVASection(filename, content, newSnippet string) error {
	startMarker := "## DVA (Dev Virtual Auto)"
	startIdx := strings.Index(content, startMarker)
	if startIdx < 0 {
		return nil
	}

	// Find the end of the DVA section (next ## heading or EOF)
	rest := content[startIdx+len(startMarker):]
	endIdx := strings.Index(rest, "\n## ")
	if endIdx < 0 {
		// DVA section goes to EOF
		content = content[:startIdx] + strings.TrimLeft(newSnippet, "\n")
	} else {
		endIdx = keepLeadingComments(rest, endIdx)
		content = content[:startIdx] + strings.TrimLeft(newSnippet, "\n") + rest[endIdx:]
	}

	return os.WriteFile(filename, []byte(content), 0644)
}

// keepLeadingComments moves a section boundary back over the HTML comment lines
// that immediately precede a heading.
//
// Another generator marks the block it owns with an opening comment on the line
// above its heading (tools/skillgen writes `<!-- skills:auto:start -->` before
// `## AI Skills`). Those comments introduce the block below, so cutting at the
// heading alone would delete the opening marker of a block DVA does not own and
// leave its closing marker orphaned. headingIdx points at the newline before the
// heading; the returned index is at or before it.
func keepLeadingComments(rest string, headingIdx int) int {
	for headingIdx > 0 {
		lineStart := strings.LastIndexByte(rest[:headingIdx], '\n')
		if lineStart < 0 {
			break
		}
		line := strings.TrimSpace(rest[lineStart+1 : headingIdx])
		if !strings.HasPrefix(line, "<!--") || !strings.HasSuffix(line, "-->") {
			break
		}
		headingIdx = lineStart
	}
	return headingIdx
}
