package config

import (
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// MigrateApplications converts `applications:` declarations into multi-runner stack
// entries and reports what it did.
//
// Conversion is per application, not per file. An application that cannot be converted
// mechanically stays exactly where it is and the ones around it still move, because the
// two sections coexist today: a file holding both a migrated `stack.api` and an
// unmigrated `applications.web` loads and runs. All-or-nothing would mean one
// unconvertible application blocking the other seven, and the operator would have to
// hand-migrate everything to get anything.
//
// Only the migrated applications' line spans are removed; every other byte survives,
// including the blank lines and comments around the ones left behind. Re-encoding the
// section would have been shorter and would have flattened it, for the reason
// MigrateLegacyCompose rewrites single entries rather than the document.
func MigrateApplications(src []byte) ([]byte, MigrationReport, error) {
	var report MigrationReport

	var doc yaml.Node
	if err := yaml.Unmarshal(src, &doc); err != nil {
		return nil, report, fmt.Errorf("parse: %w", err)
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return src, report, nil
	}
	root := doc.Content[0]
	appsKey, apps := mapFind(root, "applications")
	if apps == nil || apps.Kind != yaml.MappingNode || len(apps.Content) == 0 {
		return src, report, nil
	}
	stackKey, stack := mapFind(root, "stack")

	lines := strings.Split(string(src), "\n")

	converted := &yaml.Node{Kind: yaml.MappingNode}
	var blocked []string
	var removals []lineEdit
	movedAll := true

	for i := 0; i+1 < len(apps.Content); i += 2 {
		nameNode, app := apps.Content[i], apps.Content[i+1]
		name := nameNode.Value

		entry, notes, blockers := migrateApplicationNode(name, app, stack, mapValue(root, "interaction"))
		blocked = append(blocked, blockers...)
		if entry == nil {
			// No honest entry to write, so the application stays where it is. Blockers
			// alongside an entry mean the opposite — it converted, and something it
			// carried needs placing by hand.
			movedAll = false
			continue
		}

		converted.Content = append(converted.Content, cloneNode(nameNode), entry)
		report.Changes = append(report.Changes, fmt.Sprintf("applications.%s → stack.%s", name, name))
		report.Changes = append(report.Changes, notes...)

		_, end := blockSpan(lines, app.Line, nameNode.Column)
		removals = append(removals, lineEdit{start: nameNode.Line, end: end})
	}

	report.Blocked = sortedBlocked(blocked)
	if len(converted.Content) == 0 {
		// Nothing convertible, so the file is returned untouched — the blockers are the
		// whole of the result and the caller reports them.
		return src, report, nil
	}

	_, appsEnd := blockSpan(lines, apps.Line, appsKey.Column)
	edits, err := placeConvertedApplications(lines, converted, stackKey, stack, appsEnd)
	if err != nil {
		return nil, report, err
	}
	if movedAll {
		// The whole section went, so the `applications:` key goes with it rather than
		// being left over an empty block, which is a parse error and not a migration.
		edits = append(edits, lineEdit{start: appsKey.Line, end: appsEnd})
	} else {
		edits = append(edits, removals...)
	}

	return []byte(strings.Join(applyLineEdits(lines, edits), "\n")), report, nil
}

// placeConvertedApplications decides where the new stack entries are written.
//
// Appending to an existing `stack:` keeps one declaration store, which is the point of
// the restructure. With no stack section at all a whole one is opened directly after the
// applications block, so once that block is removed the entries sit where their
// declarations used to and the file's section order is preserved.
//
// Both anchors are deliberately outside every span the caller deletes. Two edits that
// shared a boundary would have to be applied in a particular order to be correct, and
// applyLineEdits sorts by position rather than intent.
func placeConvertedApplications(lines []string, converted *yaml.Node, stackKey, stack *yaml.Node, appsEnd int) ([]lineEdit, error) {
	indent, anchor, prefix := 2, appsEnd, []string{"stack:"}
	if stack != nil && stack.Kind == yaml.MappingNode && len(stack.Content) > 0 {
		indent = leadingSpaces(lines[stack.Line-1])
		_, anchor = blockSpan(lines, stack.Line, stackKey.Column)
		prefix = nil
	}

	body, err := encodeNode(converted, indent)
	if err != nil {
		return nil, fmt.Errorf("encode migrated applications: %w", err)
	}
	// A zero-width range: replacing nothing is how this list expresses insertion.
	return []lineEdit{{start: anchor + 1, end: anchor, body: append(prefix, strings.Split(body, "\n")...)}}, nil
}

// leftovers are the application fields with no mechanical target, and what a person has
// to do about each.
//
// They do not block the conversion. An application's `run.native` is a faithful, complete
// declaration of what to start whether or not the application also carried a dev command,
// and refusing the whole thing over one un-portable field would discard a conversion that
// is correct. Measured against the live corpus, refusing was the same as converting
// nothing: every application in it declares at least one of these.
//
// The split from `notes` below is by whether there is anything to do. These need hands, so
// they are reported as blocked; a field whose behaviour the restructure removes outright
// has no action attached and is reported beside the conversion instead.
var applicationLeftovers = []struct{ key, why string }{
	{"dev", "'dva app up <app> --dev' ran a second command per application; a stack entry " +
		"runs one. Declare the hot-reload variant as its own entry and select it with a plan"},
	{"variants", "variants inherited from their parent application; stack entries do not " +
		"inherit. Write each variant as its own entry"},
	{"depends_on", "ordering belongs to the plan that runs the entry, not to the declaration — " +
		"add it as plans.<plan>.entries[].depends_on"},
}

// migrateApplicationNode converts one application, or explains why it cannot be.
//
// It returns the stack entry, notes about what was deliberately not carried over, and
// blockers. A non-empty blocker list means there is no honest entry to write at all — not
// merely that something was left behind — and the caller leaves the application in place.
func migrateApplicationNode(name string, app, stack, interaction *yaml.Node) (*yaml.Node, []string, []string) {
	var notes, blockers []string

	if app.Kind != yaml.MappingNode {
		return nil, nil, []string{fmt.Sprintf("applications.%s: not a mapping", name)}
	}
	if mapValue(stack, name) != nil {
		return nil, nil, []string{fmt.Sprintf(
			"applications.%s: stack.%s already exists — merge them by hand, since migration "+
				"cannot tell which declaration is authoritative", name, name)}
	}

	for _, leftover := range applicationLeftovers {
		v := mapValue(app, leftover.key)
		if v == nil || (v.Kind == yaml.SequenceNode && len(v.Content) == 0) {
			continue
		}
		why := leftover.why
		if leftover.key == "dev" {
			// A dev command that an interaction already runs verbatim has a home: the
			// hot-reload entry would be a third copy of it (TASK-317).
			if dup := interactionRunning(interaction, appDevCommand(v)); dup != "" {
				why += fmt.Sprintf(". interaction.%s already runs this exact command — keep that "+
					"and drop the field, or make the entry and delete the interaction", dup)
			}
		}
		blockers = append(blockers, fmt.Sprintf("applications.%s.%s: %s", name, leftover.key, why))
	}

	native := &yaml.Node{Kind: yaml.MappingNode}
	if dir := mapValue(app, "dir"); dir != nil {
		mapAppend(native, "dir", cloneNode(dir))
	}
	for _, path := range []struct{ from, to string }{{"build", "build"}, {"run", "run"}} {
		cmd, docker := appExecNative(app, path.from)
		if cmd != nil {
			mapAppend(native, path.to, cloneNode(cmd))
		}
		if docker == nil {
			continue
		}
		// AppDockerRef is {service, profile, command}: 'dva app up --strategy docker' turned
		// it into `docker compose up -d <service>` run from the application's own directory
		// (app_manager.go buildDockerCommand/startDockerApp), with no -f, so it addressed
		// whichever compose file that directory happens to hold — not the project's compose
		// entry. Reproducing it needs that file's name, which the declaration does not carry.
		//
		// It is also the mode machinery rather than the application: the docker path is
		// reached only by --strategy or a mode's `applications`, and modes are split by hand
		// (D3). So it belongs with that split, and the native path converts on its own. (D2)
		if path.from == "build" {
			// resolveCommand sends the docker build path to resolveDockerCommand, which reads
			// run.docker — nothing ever read build.docker. Saying it is unreachable is worth
			// more than restating where the docker strategy went.
			notes = append(notes, fmt.Sprintf(
				"  applications.%s.build.docker: dropped — 'dva app build --strategy docker' ran "+
					"run.docker, so this was never executed", name))
			continue
		}
		blockers = append(blockers, fmt.Sprintf(
			"applications.%s.%s.docker: %s. It ran from the application's own directory against "+
				"whatever compose file is there, so give that file its own stack entry with a "+
				"compose runner and select the service from a plan", name, path.from, describeDockerRef(docker)))
	}
	if env := mapValue(app, "environment"); env != nil {
		mapAppend(native, "env", cloneNode(env))
	}

	if mapValue(native, "run") == nil {
		// Nothing to start natively, so there is no entry to write — this is the one case
		// where the leftovers above are the whole application rather than an aside.
		return nil, nil, append(blockers, fmt.Sprintf(
			"applications.%s: no run.native command, so there is nothing for a native runner to start", name))
	}

	// port fed effectivePort, which reclaimed the port from whatever was squatting on it
	// before starting an application. Its only caller is the application manager, which
	// goes with this section, so the behaviour is removed by the restructure rather than
	// dropped by this migration — but it is removed, and saying so is the difference
	// between a migration and a quiet deletion.
	if port := mapValue(app, "port"); port != nil {
		notes = append(notes, fmt.Sprintf(
			"  applications.%s.port: %s not carried over — it drove the port-reclaim check in "+
				"'dva app up', which no longer exists. If the port is user-facing, keep it as "+
				"endpoints.%s: {url: \"http://localhost:%s\"} so 'dva endpoints' still lists it",
			name, port.Value, name, port.Value))
	}

	entry := &yaml.Node{Kind: yaml.MappingNode}
	for _, carried := range []string{"description", "tags"} {
		if v := mapValue(app, carried); v != nil {
			mapAppend(entry, carried, cloneNode(v))
		}
	}
	mapAppend(entry, "default_runner", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "native"})
	runners := &yaml.Node{Kind: yaml.MappingNode}
	mapAppend(runners, "native", native)
	mapAppend(entry, "runners", runners)

	// One health check keyed by the entry it belongs to. The orchestrator reads
	// entry.HealthChecks for both `--wait` and status, and materializeResolvedEntry
	// copies it onto the plan entry, so this reaches the plan path unchanged.
	//
	// `required` is the exception and is dropped rather than copied. HealthCheckConfig no
	// longer has the field (config.go: its only reader was AppManager.startApp, deleted with
	// the section). Before TASK-182 the entry-scoped schema also lacked additionalProperties
	// bound, so a copied `required: true` validated clean and left the operator believing
	// strict readiness survived. The schema is closed now; migration still drops the key so
	// the rewrite does not depend on the user hitting validate after --write.
	if health := mapValue(app, "health"); health != nil {
		check := cloneNode(health)
		if req := mapValue(check, "required"); req != nil {
			mapDelete(check, "required")
			if req.Value == "true" {
				notes = append(notes, fmt.Sprintf(
					"  applications.%s.health.required: true dropped — strict readiness (non-zero exit "+
						"when the check never passes) has no equivalent on the plan path; entry health "+
						"checks are advisory. Gate it with a checks: entry or an interaction command", name))
			}
		}
		checks := &yaml.Node{Kind: yaml.MappingNode}
		mapAppend(checks, name, check)
		mapAppend(entry, "health_checks", checks)
	}

	return entry, notes, blockers
}

// appExecNative splits one AppExecPaths field into its native command and its docker ref.
//
// The string shorthand (`run: cargo run`) is the native command; only the object form
// can carry a docker path. Both spellings are live, so reading just one would silently
// drop whichever the author happened to use.
func appExecNative(app *yaml.Node, field string) (native, docker *yaml.Node) {
	v := mapValue(app, field)
	if v == nil {
		return nil, nil
	}
	if v.Kind == yaml.ScalarNode {
		return v, nil
	}
	return mapValue(v, "native"), mapValue(v, "docker")
}

// describeDockerRef states what the ref actually addressed, so the entry the operator
// writes by hand starts from the service name rather than from the field name.
func describeDockerRef(docker *yaml.Node) string {
	service, command := mapValue(docker, "service"), mapValue(docker, "command")
	// buildDockerCommand uses the raw command only when no service is named; with both,
	// AppDockerRef.Command was the compose sub-command and the service still decided what ran.
	if service == nil {
		if command != nil {
			return fmt.Sprintf("this ran %q directly", command.Value)
		}
		return "this named no service and no command, so it started nothing"
	}
	if profile := mapValue(docker, "profile"); profile != nil {
		return fmt.Sprintf("this started compose service %q under profile %q", service.Value, profile.Value)
	}
	return fmt.Sprintf("this started compose service %q", service.Value)
}

// lineEdit replaces the 1-based inclusive line range [start, end] with body. An empty
// body deletes the range; end < start inserts before start without replacing anything.
type lineEdit struct {
	start, end int
	body       []string
}

// applyLineEdits walks the original lines once, emitting each edit at its anchor.
//
// Not the back-to-front rewrite MigrateLegacyCompose uses. That one is correct for its
// own edits because they are disjoint entry bodies, but a zero-width insertion occupies
// the end of one span and the start of the next at the same time: appending a stack
// entry after a `stack:` block that is immediately followed by `applications:` anchors
// the insertion on the very line the deletion begins. Rewriting in place then makes the
// second edit's line numbers describe text the first edit already moved. Reading the
// original and writing a new slice means no edit ever sees another's output.
//
// Ties keep the caller's order, which is what puts an insertion before the deletion that
// starts on the same line.
func applyLineEdits(lines []string, edits []lineEdit) []string {
	sort.SliceStable(edits, func(i, j int) bool { return edits[i].start < edits[j].start })

	out := make([]string, 0, len(lines))
	pos := 1 // the 1-based line about to be copied
	for _, e := range edits {
		for pos < e.start && pos <= len(lines) {
			out = append(out, lines[pos-1])
			pos++
		}
		out = append(out, e.body...)
		if e.end >= e.start && e.end >= pos {
			pos = e.end + 1 // the replaced range is dropped
		}
	}
	for ; pos <= len(lines); pos++ {
		out = append(out, lines[pos-1])
	}
	return out
}

// appDevCommand returns the shell line an application's `dev:` ran — the scalar form, or
// the `native:` member of the object form.
func appDevCommand(dev *yaml.Node) string {
	if dev == nil {
		return ""
	}
	if dev.Kind == yaml.ScalarNode {
		return strings.TrimSpace(dev.Value)
	}
	if native := mapValue(dev, "native"); native != nil && native.Kind == yaml.ScalarNode {
		return strings.TrimSpace(native.Value)
	}
	return ""
}

// interactionRunning returns the name of the first top-level interaction whose scalar
// `command:` is exactly cmd, or "" when none is.
func interactionRunning(interaction *yaml.Node, cmd string) string {
	if cmd == "" || interaction == nil || interaction.Kind != yaml.MappingNode {
		return ""
	}
	for i := 0; i+1 < len(interaction.Content); i += 2 {
		c := mapValue(interaction.Content[i+1], "command")
		if c != nil && c.Kind == yaml.ScalarNode && strings.TrimSpace(c.Value) == cmd {
			return interaction.Content[i].Value
		}
	}
	return ""
}
