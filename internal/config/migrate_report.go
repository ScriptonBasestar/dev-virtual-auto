package config

import (
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// MigrationReport records what a migration moved and what it left for a person.
//
// Blocked is not an error. A config that needs hands is a fact about the config, not a
// failure of the tool, and reporting it through the error channel would make one
// unconvertible section abort the conversions that were going to succeed.
type MigrationReport struct {
	Changes []string
	Blocked []string
}

// Empty reports whether the migration found nothing at all to say.
func (r MigrationReport) Empty() bool { return len(r.Changes) == 0 && len(r.Blocked) == 0 }

func (r *MigrationReport) merge(other MigrationReport) {
	r.Changes = append(r.Changes, other.Changes...)
	r.Blocked = append(r.Blocked, other.Blocked...)
}

// Migrate applies every conversion DVA can perform mechanically and reports the rest.
//
// The order is load-blocking first: MigrateLegacyCompose repairs the one shape Load()
// refuses outright, and the conversions after it read a document that parses. Each step
// reads the previous step's output, so a file that gains stack entries from
// `applications:` has those entries in view when stack order is moved onto plan entries.
func Migrate(src []byte) ([]byte, MigrationReport, error) {
	var report MigrationReport

	out, migrated, err := MigrateLegacyCompose(src)
	if err != nil {
		// The only failure here is an entry declaring both a legacy compose shape and
		// runners.compose. That file does not load at all, so there is nothing for the
		// later steps to read and no useful partial result to return.
		return nil, report, err
	}
	for _, name := range migrated {
		report.Changes = append(report.Changes, fmt.Sprintf("stack.%s → runners.compose", name))
	}

	for _, step := range []func([]byte) ([]byte, MigrationReport, error){
		MigrateApplications,
		MigrateStackOrder,
	} {
		next, stepReport, err := step(out)
		if err != nil {
			return nil, report, err
		}
		out = next
		report.merge(stepReport)
	}

	report.Blocked = append(report.Blocked, ScaffoldModes(out)...)
	return out, report, nil
}

// modeFieldTargets says where each field of a mode ends up once the mode is split.
//
// The list is the whole reason `modes` is not converted automatically: one mode spreads
// across plans, environments and the entries' own runners, and which plan a given mode
// becomes is a naming decision rather than a derivation. Printing the split for the
// modes actually declared is the most a tool can do without inventing the answer. (D3)
var modeFieldTargets = []struct{ key, target string }{
	{"description", "plans.<name>.description"},
	{"stack", "plans.<name>.entries[].name — the declarations this mode selected"},
	{"compose_services", "plans.<name>.entries[<compose entry>].services"},
	{"compose_profiles", "stack.<entry>.runners.compose.up_options, as --profile <name>"},
	{"health_checks", "stack.<entry>.health_checks"},
	{"endpoint_tags", "plans.<name>.endpoint_tags"},
	{"environment", "environments.<name>.vars, selected by plans.<name>.environment"},
	{"build", "the entry's runner: 'docker' means runners.compose, 'native' means runners.native.build"},
	{"run", "the entry's default_runner"},
	{"applications", "runners.native on the entry the application became"},
	{"provision", "no equivalent — a provision profile is chosen explicitly, not by mode"},
}

// ScaffoldModes describes how each declared mode splits across the new model.
//
// It converts nothing. A mode carries a stack filter, an environment, a build strategy
// and an application strategy in one name, and the split needs a decision per mode about
// which of those become which plan — so the useful output is the operator's own modes
// with each field pointed at its destination, not a guessed rewrite.
func ScaffoldModes(src []byte) []string {
	var doc yaml.Node
	if err := yaml.Unmarshal(src, &doc); err != nil {
		return nil
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return nil
	}
	modes := mapValue(doc.Content[0], "modes")
	if modes == nil || modes.Kind != yaml.MappingNode || len(modes.Content) == 0 {
		return nil
	}

	var out []string
	for i := 0; i+1 < len(modes.Content); i += 2 {
		name, mode := modes.Content[i].Value, modes.Content[i+1]
		out = append(out, fmt.Sprintf("modes.%s: split by hand —", name))
		for _, field := range modeFieldTargets {
			if mapValue(mode, field.key) == nil {
				continue
			}
			out = append(out, fmt.Sprintf("      %-17s → %s",
				field.key, strings.ReplaceAll(field.target, "<name>", name)))
		}
	}
	return out
}

// sortedBlocked returns blocked entries in a stable order for reporting.
func sortedBlocked(blocked []string) []string {
	out := append([]string(nil), blocked...)
	sort.Strings(out)
	return out
}
