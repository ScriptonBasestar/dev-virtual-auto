package config

import (
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// modeConvertibleKeys are the mode fields a plan can carry with the same meaning.
//
// A mode built from these alone is a plan that has not been renamed: `stack` is the
// entries list, `compose_services` is the compose entry's `services`, and the other two
// are copied. Every other mode field (compose_profiles, environment, build, run,
// health_checks, provision, applications) spreads across sections a plan does not own,
// so a mode carrying one stays for ScaffoldModes to describe. TASK-306.
var modeConvertibleKeys = map[string]bool{
	"description":      true,
	"stack":            true,
	"compose_services": true,
	"endpoint_tags":    true,
}

// MigrateModes converts the modes that are plans in all but name into `plans:` entries.
//
// The naming question D3 refused to answer — which plan a mode becomes — has an obvious
// answer for this subclass: the mode's own name, because `dva up <mode-name>` is exactly
// what `--mode <mode-name>` selected. What still needs a person is anything a plan cannot
// express, and those modes are left in place with the reason, so a file of six modes can
// have five converted and one described rather than all six described.
//
// Only converted modes' line spans are removed; the `modes:` key itself goes when nothing
// is left under it. `default_mode: X` becomes `default_plan: X` when X converted and no
// default_plan exists; when one does, X is left unconverted so the tool never picks a
// default on the author's behalf.
func MigrateModes(src []byte) ([]byte, MigrationReport, error) {
	var report MigrationReport

	var doc yaml.Node
	if err := yaml.Unmarshal(src, &doc); err != nil {
		return nil, report, fmt.Errorf("parse: %w", err)
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return src, report, nil
	}
	root := doc.Content[0]
	modesKey, modes := mapFind(root, "modes")
	if modes == nil || modes.Kind != yaml.MappingNode || len(modes.Content) == 0 {
		return src, report, nil
	}
	stack := mapValue(root, "stack")
	plansKey, plans := mapFind(root, "plans")
	defaultModeKey, defaultMode := mapFind(root, "default_mode")
	_, defaultPlan := mapFind(root, "default_plan")

	lines := strings.Split(string(src), "\n")

	converted := &yaml.Node{Kind: yaml.MappingNode}
	var blocked []string
	var removals []lineEdit
	movedAll := true
	defaultConverted := false

	for i := 0; i+1 < len(modes.Content); i += 2 {
		nameNode, mode := modes.Content[i], modes.Content[i+1]
		name := nameNode.Value

		plan, why := migrateModeNode(name, mode, stack, plans)
		if plan != nil && defaultMode != nil && defaultMode.Value == name && defaultPlan != nil {
			plan, why = nil, fmt.Sprintf("default_mode is %q but default_plan is already %q — decide which one "+
				"'dva up' with no plan should run, then rerun", name, defaultPlan.Value)
		}
		if plan == nil {
			movedAll = false
			blocked = append(blocked, fmt.Sprintf("modes.%s: not converted — %s", name, why))
			continue
		}

		converted.Content = append(converted.Content, cloneNode(nameNode), plan)
		report.Changes = append(report.Changes, fmt.Sprintf(
			"modes.%s → plans.%s (run 'dva up %s' instead of '--mode %s')", name, name, name, name))
		if defaultMode != nil && defaultMode.Value == name {
			defaultConverted = true
		}

		_, end := blockSpan(lines, mode.Line, nameNode.Column)
		removals = append(removals, lineEdit{start: nameNode.Line, end: end})
	}

	report.Blocked = sortedBlocked(blocked)
	if len(converted.Content) == 0 {
		return src, report, nil
	}

	_, modesEnd := blockSpan(lines, modes.Line, modesKey.Column)
	edits, err := placeConvertedPlans(lines, converted, plansKey, plans, modesEnd)
	if err != nil {
		return nil, report, err
	}
	if movedAll {
		edits = append(edits, lineEdit{start: modesKey.Line, end: modesEnd})
	} else {
		edits = append(edits, removals...)
	}
	if defaultConverted {
		// Same line, same indentation and trailing comment; only the key changes.
		line := lines[defaultModeKey.Line-1]
		edits = append(edits, lineEdit{start: defaultModeKey.Line, end: defaultModeKey.Line,
			body: []string{strings.Replace(line, "default_mode", "default_plan", 1)}})
		report.Changes = append(report.Changes, fmt.Sprintf("default_mode: %s → default_plan: %s", defaultMode.Value, defaultMode.Value))
	}

	return []byte(strings.Join(applyLineEdits(lines, edits), "\n")), report, nil
}

// migrateModeNode builds the plan for one mode, or explains why it cannot.
//
// The compose_services rule is the one place a judgment is made: the list has to land on
// exactly one entry's `services`, so it attaches when the selected stack holds one compose
// entry and refuses otherwise. Guessing between two compose entries would silently start
// a different set of containers than the mode did.
func migrateModeNode(name string, mode, stack, plans *yaml.Node) (*yaml.Node, string) {
	if mode == nil || mode.Kind != yaml.MappingNode {
		return nil, "not a mapping"
	}
	if plans != nil && mapValue(plans, name) != nil {
		return nil, fmt.Sprintf("plans.%s already exists — merge the mode into it by hand", name)
	}

	var foreign []string
	for i := 0; i+1 < len(mode.Content); i += 2 {
		if !modeConvertibleKeys[mode.Content[i].Value] {
			foreign = append(foreign, mode.Content[i].Value)
		}
	}
	if len(foreign) > 0 {
		sort.Strings(foreign)
		return nil, fmt.Sprintf("%s has no plan equivalent; see the field targets below", strings.Join(foreign, ", "))
	}

	stackSel := mapValue(mode, "stack")
	services := mapValue(mode, "compose_services")
	if stackSel != nil && stackSel.Kind != yaml.SequenceNode {
		return nil, "stack is not a list"
	}
	if services != nil && services.Kind != yaml.SequenceNode {
		return nil, "compose_services is not a list"
	}

	// The entries the plan runs: the mode's stack selection, or — for a mode that only
	// narrows services — every stack entry, which is what a mode without `stack:` ran.
	var selected []string
	if stackSel != nil && len(stackSel.Content) > 0 {
		for _, n := range stackSel.Content {
			if stack == nil || mapValue(stack, n.Value) == nil {
				return nil, fmt.Sprintf("stack selects %q, which is not declared under stack:", n.Value)
			}
			selected = append(selected, n.Value)
		}
	} else if services != nil && stack != nil {
		for i := 0; i+1 < len(stack.Content); i += 2 {
			selected = append(selected, stack.Content[i].Value)
		}
	}
	if len(selected) == 0 {
		return nil, "selects nothing — neither stack nor compose_services"
	}

	servicesTarget := ""
	if services != nil && len(services.Content) > 0 {
		var composeEntries []string
		for _, entryName := range selected {
			if entryHasComposeRunner(mapValue(stack, entryName)) {
				composeEntries = append(composeEntries, entryName)
			}
		}
		switch len(composeEntries) {
		case 1:
			servicesTarget = composeEntries[0]
		case 0:
			return nil, "compose_services needs a compose entry to attach to, and the selected stack has none"
		default:
			return nil, fmt.Sprintf("compose_services could attach to any of %s — pick the entry and write "+
				"plans.%s.entries[<entry>].services by hand", strings.Join(composeEntries, ", "), name)
		}
	}

	plan := &yaml.Node{Kind: yaml.MappingNode}
	if desc := mapValue(mode, "description"); desc != nil {
		mapAppend(plan, "description", cloneNode(desc))
	}
	if tags := mapValue(mode, "endpoint_tags"); tags != nil {
		mapAppend(plan, "endpoint_tags", cloneNode(tags))
	}
	entries := &yaml.Node{Kind: yaml.SequenceNode}
	for _, entryName := range selected {
		item := &yaml.Node{Kind: yaml.MappingNode}
		mapAppend(item, "name", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: entryName})
		if entryName == servicesTarget {
			list := cloneNode(services)
			list.Style = yaml.FlowStyle
			mapAppend(item, "services", list)
		}
		entries.Content = append(entries.Content, item)
	}
	mapAppend(plan, "entries", entries)
	return plan, ""
}

// entryHasComposeRunner reports whether a stack declaration runs through compose, in the
// shape MigrateLegacyCompose has already produced by the time this step runs.
func entryHasComposeRunner(entry *yaml.Node) bool {
	if entry == nil || entry.Kind != yaml.MappingNode {
		return false
	}
	if dr := mapValue(entry, "default_runner"); dr != nil && dr.Value == "compose" {
		return true
	}
	return mapValue(mapValue(entry, "runners"), "compose") != nil
}

// placeConvertedPlans appends to an existing `plans:` block, or opens one directly after
// the modes block so the plans sit where the modes were. Same anchoring rule as
// placeConvertedApplications: both anchors lie outside every span the caller deletes.
func placeConvertedPlans(lines []string, converted *yaml.Node, plansKey, plans *yaml.Node, modesEnd int) ([]lineEdit, error) {
	indent, anchor, prefix := 2, modesEnd, []string{"plans:"}
	if plans != nil && plans.Kind == yaml.MappingNode && len(plans.Content) > 0 {
		indent = leadingSpaces(lines[plans.Line-1])
		_, anchor = blockSpan(lines, plans.Line, plansKey.Column)
		prefix = nil
	}
	body, err := encodeNode(converted, indent)
	if err != nil {
		return nil, fmt.Errorf("encode migrated modes: %w", err)
	}
	return []lineEdit{{start: anchor + 1, end: anchor, body: append(prefix, strings.Split(body, "\n")...)}}, nil
}
