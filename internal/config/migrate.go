package config

import (
	"bytes"
	"fmt"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// composeRunnerKeys are the ComposePluginConfig fields a legacy entry carries
// flat. They move under runners.compose. `tags` is deliberately absent: it is
// the one key both LifecycleEntry and ComposePluginConfig declare, so moving it
// would silently drop one of its two live meanings (see migrateEntryNode).
var composeRunnerKeys = []string{"files", "project_name", "command", "method", "up_options", "services"}

// MigrateLegacyCompose rewrites the legacy compose declarations that
// LifecycleEntry.rejectLegacyComposeShape refuses into the runners shape, and
// reports which stack entries changed.
//
// Only the line span of each migrated entry is re-encoded; every other byte of
// src is passed through untouched. A whole-document round-trip would be simpler
// but yaml.v3 does not model blank lines, so it would strip every separator in
// the file and bury the real change in cosmetic noise.
func MigrateLegacyCompose(src []byte) ([]byte, []string, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(src, &doc); err != nil {
		return nil, nil, fmt.Errorf("parse: %w", err)
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return src, nil, nil
	}
	stack := mapValue(doc.Content[0], "stack")
	if stack == nil || stack.Kind != yaml.MappingNode {
		return src, nil, nil
	}

	lines := strings.Split(string(src), "\n")

	type edit struct {
		start, end int // 1-based, inclusive
		body       string
	}
	var edits []edit
	var migrated []string

	for i := 0; i+1 < len(stack.Content); i += 2 {
		keyNode, valNode := stack.Content[i], stack.Content[i+1]
		if valNode.Kind != yaml.MappingNode {
			continue
		}
		changed, err := migrateEntryNode(keyNode.Value, valNode)
		if err != nil {
			return nil, nil, err
		}
		if !changed {
			continue
		}

		start, end := blockSpan(lines, valNode.Line, keyNode.Column)
		indent := leadingSpaces(lines[start-1])
		body, err := encodeNode(valNode, indent)
		if err != nil {
			return nil, nil, fmt.Errorf("stack.%s: %w", keyNode.Value, err)
		}
		edits = append(edits, edit{start: start, end: end, body: body})
		migrated = append(migrated, keyNode.Value)
	}

	if len(edits) == 0 {
		return src, nil, nil
	}

	// Apply back to front so earlier spans keep their original line numbers.
	out := lines
	for _, v := range slices.Backward(edits) {
		e := v
		tail := append([]string{}, out[e.end:]...)
		out = append(out[:e.start-1], append(strings.Split(e.body, "\n"), tail...)...)
	}
	return []byte(strings.Join(out, "\n")), migrated, nil
}

// VerifyMigrated reports whether content parses and every stack entry satisfies
// the compose contract Load() enforces.
//
// It deliberately skips module and subproject resolution: migration rewrites a
// single file, so it must not fail merely because a sibling that file references
// is unavailable from where the command runs.
func VerifyMigrated(content []byte) error {
	cfg := &Config{}
	if err := yaml.Unmarshal(content, cfg); err != nil {
		return fmt.Errorf("parsing YAML: %w", err)
	}
	for name, entry := range cfg.Stack {
		if entry == nil {
			continue
		}
		entry.Name = name
		if err := entry.ResolvePluginFromName(); err != nil {
			return err
		}
	}
	return nil
}

// migrateEntryNode rewrites one stack entry in place, reporting whether it was
// a legacy compose declaration at all.
func migrateEntryNode(name string, entry *yaml.Node) (bool, error) {
	nested := mapValue(entry, "compose")
	hasNested := nested != nil && nested.Kind == yaml.MappingNode
	plugin := mapValue(entry, "plugin")
	hasPluginCompose := plugin != nil && plugin.Value == "compose"

	var flat []string
	for _, k := range composeRunnerKeys {
		if mapValue(entry, k) != nil {
			flat = append(flat, k)
		}
	}
	hasInferred := name == "compose" && len(flat) > 0

	if !hasNested && !hasPluginCompose && !hasInferred {
		return false, nil
	}
	if runners := mapValue(entry, "runners"); runners != nil && mapValue(runners, "compose") != nil {
		return false, fmt.Errorf("stack.%s: has both runners.compose and a legacy compose declaration; "+
			"merge them by hand — migration cannot tell which one is authoritative", name)
	}

	var composeMap *yaml.Node
	switch {
	case hasNested:
		composeMap = nested
		mapDelete(entry, "compose")
	default:
		composeMap = &yaml.Node{Kind: yaml.MappingNode}
		for _, k := range flat {
			k, v := mapCut(entry, k)
			composeMap.Content = append(composeMap.Content, k, v)
		}
	}

	// `tags` is copied, not moved: LifecycleEntry.Tags drives stack-entry tag
	// filtering while ComposePluginConfig.Tags supplies the compose service
	// filter defaults (tag_filter.go). A legacy flat entry fed both from one
	// key, so only duplicating it keeps the config's behaviour unchanged.
	if !hasNested {
		if tagsKey, tagsVal := mapFind(entry, "tags"); tagsVal != nil && mapValue(composeMap, "tags") == nil {
			composeMap.Content = append(composeMap.Content, cloneNode(tagsKey), cloneNode(tagsVal))
		}
	}

	if hasPluginCompose {
		// plugin: compose is what the schema rejects; default_runner replaces it.
		mapDelete(entry, "plugin")
	}
	if mapValue(entry, "default_runner") == nil {
		mapAppend(entry, "default_runner", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "compose"})
	}
	if runners := mapValue(entry, "runners"); runners != nil {
		mapAppend(runners, "compose", composeMap)
	} else {
		runners := &yaml.Node{Kind: yaml.MappingNode}
		mapAppend(runners, "compose", composeMap)
		mapAppend(entry, "runners", runners)
	}
	return true, nil
}

// blockSpan returns the 1-based inclusive line range of a block whose parent key
// sits at keyCol, starting at startLine. Indentation decides the end, so block
// scalars and nested comments are covered without inspecting node kinds.
func blockSpan(lines []string, startLine, keyCol int) (int, int) {
	end := startLine
	for i := startLine; i < len(lines); i++ {
		line := lines[i]
		if strings.TrimSpace(line) == "" {
			continue // a blank line alone never ends a block
		}
		if leadingSpaces(line) < keyCol {
			break
		}
		end = i + 1
	}
	return startLine, end
}

func leadingSpaces(s string) int { return len(s) - len(strings.TrimLeft(s, " ")) }

func encodeNode(node *yaml.Node, indent int) (string, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(node); err != nil {
		return "", err
	}
	if err := enc.Close(); err != nil {
		return "", err
	}
	pad := strings.Repeat(" ", indent)
	var b strings.Builder
	for i, line := range strings.Split(strings.TrimRight(buf.String(), "\n"), "\n") {
		if i > 0 {
			b.WriteByte('\n')
		}
		if line != "" {
			b.WriteString(pad)
		}
		b.WriteString(line)
	}
	return b.String(), nil
}

func mapFind(m *yaml.Node, key string) (*yaml.Node, *yaml.Node) {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil, nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i], m.Content[i+1]
		}
	}
	return nil, nil
}

func mapValue(m *yaml.Node, key string) *yaml.Node {
	_, v := mapFind(m, key)
	return v
}

func mapDelete(m *yaml.Node, key string) {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			m.Content = append(m.Content[:i], m.Content[i+2:]...)
			return
		}
	}
}

// mapCut removes key from m and returns its key/value nodes, comments intact.
func mapCut(m *yaml.Node, key string) (*yaml.Node, *yaml.Node) {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			k, v := m.Content[i], m.Content[i+1]
			m.Content = append(m.Content[:i], m.Content[i+2:]...)
			return k, v
		}
	}
	return nil, nil
}

func mapAppend(m *yaml.Node, key string, val *yaml.Node) {
	m.Content = append(m.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, val)
}

func cloneNode(n *yaml.Node) *yaml.Node {
	if n == nil {
		return nil
	}
	c := *n
	c.Content = nil
	for _, child := range n.Content {
		c.Content = append(c.Content, cloneNode(child))
	}
	return &c
}
