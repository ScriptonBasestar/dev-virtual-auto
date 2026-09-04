// Command flowgen renders the shared DVA configuration corpus into the public
// Agent Mesh flow files. Agent Mesh installs flow YAML but not arbitrary Markdown
// assets, so a published flow must not need the source checkout that generated it.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type injection struct {
	flow   string
	marker string
	source string
}

var injections = []injection{
	{"agent-mesh-flows/dva-improve.yaml", "dva_flow_mode_preserve", "agent-mesh-flows/shared/guardrails/guardrails-preserve.md"},
	{"agent-mesh-flows/dva-improve.yaml", "dva_flow_mode_rewrite", "agent-mesh-flows/shared/guardrails/guardrails-rewrite.md"},
	{"agent-mesh-flows/dva-improve.yaml", "dva_flow_guardrails", "agent-mesh-flows/shared/library/shared-guardrails.md"},
	{"agent-mesh-flows/dva-improve.yaml", "dva_flow_naming", "agent-mesh-flows/shared/library/naming-presets.md"},
	{"agent-mesh-flows/dva-improve.yaml", "dva_flow_schema", "agent-mesh-flows/shared/library/dva-schema.md"},
	{"agent-mesh-flows/dva-improve.yaml", "dva_flow_examples", "agent-mesh-flows/shared/library/reference-examples.md"},
	{"agent-mesh-flows/dva-improve.yaml", "dva_flow_checklist", "agent-mesh-flows/shared/library/shared-checklist.md"},
	{"agent-mesh-flows/dva-improve.yaml", "dva_flow_devbox_apply", "agent-mesh-flows/shared/library/devbox-apply.md"},
	{"agent-mesh-flows/dva-diagnose.yaml", "dva_flow_guardrails", "agent-mesh-flows/shared/library/shared-guardrails.md"},
	{"agent-mesh-flows/dva-improve-guided/00-analyze.yaml", "dva_flow_guardrails", "agent-mesh-flows/shared/library/shared-guardrails.md"},
	{"agent-mesh-flows/dva-improve-guided/00-analyze.yaml", "dva_flow_naming", "agent-mesh-flows/shared/library/naming-presets.md"},
	{"agent-mesh-flows/dva-improve-guided/00-analyze.yaml", "dva_flow_devbox_apply", "agent-mesh-flows/shared/library/devbox-apply.md"},
	{"agent-mesh-flows/dva-improve-guided/30-configure.yaml", "dva_flow_guardrails", "agent-mesh-flows/shared/library/shared-guardrails.md"},
	{"agent-mesh-flows/dva-improve-guided/30-configure.yaml", "dva_flow_schema", "agent-mesh-flows/shared/library/dva-schema.md"},
	{"agent-mesh-flows/dva-improve-guided/30-configure.yaml", "dva_flow_naming", "agent-mesh-flows/shared/library/naming-presets.md"},
	{"agent-mesh-flows/dva-improve-guided/30-configure.yaml", "dva_flow_examples", "agent-mesh-flows/shared/library/reference-examples.md"},
	{"agent-mesh-flows/dva-improve-guided/30-configure.yaml", "dva_flow_checklist", "agent-mesh-flows/shared/library/shared-checklist.md"},
	{"agent-mesh-flows/dva-improve-guided/30-configure.yaml", "dva_flow_devbox_apply", "agent-mesh-flows/shared/library/devbox-apply.md"},
}

func main() {
	changed := map[string]bool{}
	for _, spec := range injections {
		body, err := os.ReadFile(spec.source)
		if err != nil {
			fail(err)
		}
		flow, err := os.ReadFile(spec.flow)
		if err != nil {
			fail(err)
		}
		rendered, err := replaceBlock(string(flow), spec.marker, strings.TrimSuffix(string(body), "\n"))
		if err != nil {
			fail(fmt.Errorf("flowgen: %s: %w", spec.flow, err))
		}
		if rendered != string(flow) {
			if err := os.WriteFile(spec.flow, []byte(rendered), 0o644); err != nil {
				fail(err)
			}
			changed[spec.flow] = true
		}
	}
	for _, flow := range sortedKeys(changed) {
		fmt.Println("flowgen: updated", filepath.ToSlash(flow))
	}
	if len(changed) == 0 {
		fmt.Println("flowgen: public flows already up-to-date")
	}
}

func replaceBlock(content, marker, body string) (string, error) {
	start := "<!-- AUTOGEN:" + marker + ":start -->"
	end := "<!-- AUTOGEN:" + marker + ":end -->"
	pattern := `(?m)^([ \t]*)` + regexp.QuoteMeta(start) + `[\s\S]*?^[ \t]*` + regexp.QuoteMeta(end)
	matches := regexp.MustCompile(pattern).FindAllStringSubmatchIndex(content, -1)
	if len(matches) == 0 {
		return "", fmt.Errorf("marker %q is missing", start)
	}

	var out strings.Builder
	last := 0
	for _, match := range matches {
		indent := content[match[2]:match[3]]
		out.WriteString(content[last:match[0]])
		out.WriteString(renderBlock(indent, start, end, body))
		last = match[1]
	}
	out.WriteString(content[last:])
	return out.String(), nil
}

func renderBlock(indent, start, end, body string) string {
	var rendered strings.Builder
	rendered.WriteString(indent)
	rendered.WriteString(start)
	for line := range strings.SplitSeq(body, "\n") {
		rendered.WriteByte('\n')
		if line != "" {
			rendered.WriteString(indent)
			rendered.WriteString(line)
		}
	}
	rendered.WriteByte('\n')
	rendered.WriteString(indent)
	rendered.WriteString(end)
	return rendered.String()
}

func sortedKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	for i := range keys {
		for j := i + 1; j < len(keys); j++ {
			if keys[j] < keys[i] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	return keys
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
