package config

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// MigrateStackOrder moves `stack.*.order` onto the plan entries that reference the
// declaration.
//
// This is a repair, not a tidy-up. ResolvePlan builds each entry with
// `Order: planEntry.Order` and no fallback to the declaration, and
// materializeResolvedEntry then overwrites the declaration's own field with that value —
// so on the plan path a `stack.*.order` is not merely deprecated, it is never read. A
// config that declares an order there and runs through a plan whose entries carry none
// has every entry tied at zero and executes in alphabetical order instead.
//
// An order with no plan entry to move to stays where it is: the plan-less path still
// sorts stack declarations by it, so deleting it there would break the ordering rather
// than relocate it.
func MigrateStackOrder(src []byte) ([]byte, MigrationReport, error) {
	var report MigrationReport

	var doc yaml.Node
	if err := yaml.Unmarshal(src, &doc); err != nil {
		return nil, report, fmt.Errorf("parse: %w", err)
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return src, report, nil
	}
	root := doc.Content[0]
	stack := mapValue(root, "stack")
	if stack == nil || stack.Kind != yaml.MappingNode {
		return src, report, nil
	}
	plans := mapValue(root, "plans")

	lines := strings.Split(string(src), "\n")

	var edits []lineEdit
	var blocked []string

	for i := 0; i+1 < len(stack.Content); i += 2 {
		nameNode, entry := stack.Content[i], stack.Content[i+1]
		if entry.Kind != yaml.MappingNode {
			continue
		}
		orderKey, orderVal := mapFind(entry, "order")
		if orderVal == nil {
			continue
		}
		// A zero order is the field's own default, which warnLegacyStackOrder does not
		// report either. Moving it would add noise to every plan that referenced the
		// entry without changing a single execution.
		order, err := strconv.Atoi(strings.TrimSpace(orderVal.Value))
		if err != nil || order <= 0 {
			continue
		}

		refs := planEntriesNamed(plans, nameNode.Value)
		if len(refs) == 0 {
			blocked = append(blocked, fmt.Sprintf(
				"stack.%s.order: %d has no plan entry to move to — add %q to a plan's entries[] "+
					"first, since deleting the order here would drop the ordering rather than relocate it",
				nameNode.Value, order, nameNode.Value))
			continue
		}

		for _, ref := range refs {
			if mapValue(ref.item, "order") != nil {
				report.Changes = append(report.Changes, fmt.Sprintf(
					"  stack.%s.order: %d dropped — plans.%s already orders this entry, and the plan's "+
						"value is the one that ran", nameNode.Value, order, ref.plan))
				continue
			}
			indent := ref.item.Column - 1
			// The item's continuation keys sit one column left of where the mapping starts,
			// because the "- " opening the item is not indentation. Passing the mapping's own
			// column would end the block at its first continuation key.
			_, end := blockSpan(lines, ref.item.Line, indent)
			edits = append(edits, lineEdit{
				start: end + 1, end: end,
				body: []string{fmt.Sprintf("%sorder: %d", strings.Repeat(" ", indent), order)},
			})
			report.Changes = append(report.Changes, fmt.Sprintf("stack.%s.order: %d → plans.%s.entries[%s].order",
				nameNode.Value, order, ref.plan, nameNode.Value))
		}

		_, orderEnd := blockSpan(lines, orderVal.Line, orderKey.Column)
		edits = append(edits, lineEdit{start: orderKey.Line, end: orderEnd})
	}

	report.Blocked = stackOrderBlocked(plans, blocked)
	if len(edits) == 0 {
		return src, report, nil
	}
	return []byte(strings.Join(applyLineEdits(lines, edits), "\n")), report, nil
}

// planEntryRef locates one plan entry that references a stack declaration.
type planEntryRef struct {
	plan string
	item *yaml.Node
}

// planEntriesNamed finds every plan entry referencing name, in plan-name order so the
// report and the edits do not depend on YAML map iteration.
func planEntriesNamed(plans *yaml.Node, name string) []planEntryRef {
	if plans == nil || plans.Kind != yaml.MappingNode {
		return nil
	}
	var refs []planEntryRef
	for i := 0; i+1 < len(plans.Content); i += 2 {
		planName, plan := plans.Content[i].Value, plans.Content[i+1]
		entries := mapValue(plan, "entries")
		if entries == nil || entries.Kind != yaml.SequenceNode {
			continue
		}
		for _, item := range entries.Content {
			if item.Kind != yaml.MappingNode {
				continue
			}
			if n := mapValue(item, "name"); n != nil && n.Value == name {
				refs = append(refs, planEntryRef{plan: planName, item: item})
			}
		}
	}
	sort.SliceStable(refs, func(i, j int) bool { return refs[i].plan < refs[j].plan })
	return refs
}

// stackOrderBlocked explains why orders that clearly needed moving did not move.
//
// A config with no plans at all gets a line of its own ahead of the entries. Telling
// someone to add a declaration to a plan's entries[] is useless advice when there is no
// plan to add it to, and writing that plan is the larger of the two jobs.
func stackOrderBlocked(plans *yaml.Node, blocked []string) []string {
	if len(blocked) == 0 {
		return nil
	}
	out := sortedBlocked(blocked)
	if plans == nil || plans.Kind != yaml.MappingNode || len(plans.Content) == 0 {
		return append([]string{"stack.*.order: this config declares no plans, so there is nowhere to " +
			"move the ordering to — declare a plan whose entries[] name these declarations, then re-run"}, out...)
	}
	return out
}
