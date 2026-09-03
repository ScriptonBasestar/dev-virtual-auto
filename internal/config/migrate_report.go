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

	report.Blocked = append(report.Blocked, ReportInteractionEnvFile(out)...)
	report.Blocked = append(report.Blocked, ScaffoldModes(out)...)
	report.Blocked = append(report.Blocked, ReportModuleScope(out)...)
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

// ReportInteractionEnvFile lists every `env_file:` declared on an interaction command or
// subcommand, as a blocked entry.
//
// Blocked rather than converted, and deliberately so: the field is inert, so there is no
// value to preserve and nothing to move it to that a tool could derive. The two honest
// destinations are the top-level `env_file:`, which changes the scope from one command to
// the whole config, and the command's own `environment:`, which needs the values a file was
// supposed to supply. Both are decisions about intent, and a migration that guessed either
// would silently widen a config's environment or drop inputs. TASK-265 §4.
//
// It reads the document rather than a loaded Config because that is what every other step
// here does — the report describes the file the author will edit, including the sections a
// later load-time merge would have flattened away.
//
// Scope: `src` is the one document `Migrate` was handed — the config file itself, never a
// module. A declaration written inside a file named under `modules:` lives in a document
// this function never opens, so it is invisible here, including to the "Left for you" list
// `dva config migrate` prints: nothing walks the merged view this check's raw-node approach
// deliberately avoids. `dva config validate` is the only surface that sees it, because it
// runs on the loaded, merged `Config` — and even there the warning names the merged dotted
// path, not the module file the author has to open. ReportModuleScope names that gap instead
// of leaving it silent. TASK-285 §1.
func ReportInteractionEnvFile(src []byte) []string {
	var doc yaml.Node
	if err := yaml.Unmarshal(src, &doc); err != nil {
		return nil
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return nil
	}
	interaction := mapValue(doc.Content[0], "interaction")
	if interaction == nil || interaction.Kind != yaml.MappingNode {
		return nil
	}

	var out []string
	// Recursive for the reason the validate-time check is: `subcommands` nests without a
	// depth limit, so a walk that stopped at the first level would report the shallow
	// declaration and stay silent on the identical nested one.
	var walk func(path string, cmd *yaml.Node)
	walk = func(path string, cmd *yaml.Node) {
		if cmd == nil || cmd.Kind != yaml.MappingNode {
			return
		}
		if mapValue(cmd, "env_file") != nil {
			out = append(out, fmt.Sprintf("%s.env_file: %s", path, InteractionEnvFileBlockedMessage))
		}
		subs := mapValue(cmd, "subcommands")
		if subs == nil || subs.Kind != yaml.MappingNode {
			return
		}
		for i := 0; i+1 < len(subs.Content); i += 2 {
			walk(path+".subcommands."+subs.Content[i].Value, subs.Content[i+1])
		}
	}
	for i := 0; i+1 < len(interaction.Content); i += 2 {
		walk("interaction."+interaction.Content[i].Value, interaction.Content[i+1])
	}
	return out
}

// InteractionEnvFileBlockedMessage is the migrate-side wording of
// InteractionEnvFileMessage. It is a separate string because the two surfaces answer
// different questions: `validate` explains why the declaration is a problem, while
// `migrate` lists an edit the author has to make by hand, so it opens with the action and
// leaves the release number to the validate channel and the CHANGELOG.
const InteractionEnvFileBlockedMessage = "remove it — declare shared inputs in the top-level 'env_file:', or inline command-local values under 'environment:'"

// ReportModuleScope names the modules a config declares, so a report built only from this
// document does not read as though it covered them too.
//
// It does not open the module files: doing so would mean re-deriving which of their
// declarations are load-bearing on this side of the merge, which is the same "which file
// does the edit land in" question TASK-285's direction refuses to answer as a side effect of
// a coverage-gap repair. Naming the gap is the honest minimum — the reader is pointed at
// `dva config validate`, which does load the merge, for what this command cannot see.
func ReportModuleScope(src []byte) []string {
	var doc yaml.Node
	if err := yaml.Unmarshal(src, &doc); err != nil {
		return nil
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return nil
	}
	modules := mapValue(doc.Content[0], "modules")
	if modules == nil || modules.Kind != yaml.SequenceNode || len(modules.Content) == 0 {
		return nil
	}

	names := make([]string, 0, len(modules.Content))
	for _, n := range modules.Content {
		names = append(names, n.Value)
	}
	return []string{fmt.Sprintf(
		"modules: %s — declared here but not read by this command; a deprecation inside one "+
			"of them (an interaction env_file among them) is invisible to this report. Run "+
			"'dva config validate', which loads the merged config, to see it — it will name the "+
			"merged dotted path rather than the module file to edit",
		strings.Join(names, ", "),
	)}
}

// sortedBlocked returns blocked entries in a stable order for reporting.
func sortedBlocked(blocked []string) []string {
	out := append([]string(nil), blocked...)
	sort.Strings(out)
	return out
}
