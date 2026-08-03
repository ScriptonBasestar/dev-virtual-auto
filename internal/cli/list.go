package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ScriptonBasestar/dva/internal/config"
	"github.com/ScriptonBasestar/dva/internal/output"
	"github.com/ScriptonBasestar/dva/internal/runner"
)

var (
	lsFormat   string
	lsDetailed bool
)

var lsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List all available interaction scripts and commands",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := mustLoadConfig()
		tree := runner.NewInteractionTree(c.Interaction)
		commands := tree.List()

		keys := sortedKeys(commands)

		if jsonOutput {
			lsFormat = "json"
		}

		switch lsFormat {
		case "json":
			return printJSON(c, commands, keys)
		case "yaml":
			return printYAML(c, commands, keys)
		default:
			return printTable(c, commands, keys)
		}
	},
}

func init() {
	lsCmd.Flags().StringVarP(&lsFormat, "format", "f", "table", "Output format (table, json, yaml)")
	lsCmd.Flags().BoolVarP(&lsDetailed, "detailed", "d", false, "Show detailed information")
}

func printTable(c *config.Config, commands map[string]*runner.ResolvedCommand, keys []string) error {
	// Calculate max width for alignment
	maxName := 0
	for _, k := range keys {
		if len(k) > maxName {
			maxName = len(k)
		}
	}

	for _, k := range keys {
		cmd := commands[k]
		// A shadowed row is still listed — the user declared it and needs to see dva received
		// it — but it must carry the form that reaches it, or the listing reads as an offer of
		// `dva <k>`, which runs the built-in instead.
		usage, shadowedBy, unroutable := interactionUsage(c, cmd)
		mark := ""
		switch {
		case unroutable != "":
			// No "run:" clause here, unlike the shadowed mark below: there is no invocation to
			// offer. The rename is the whole answer, which is what ConflictAdvice already says
			// on every config load.
			// The rename lands on the declared key, which is path[0]. `app:build fast` is a
			// display name assembled from the tree walk — there is no such key in the file to
			// edit, and renaming the parent fixes every subcommand under it at once. Suggesting
			// the flattened form told the author to write a key containing a space.
			declared := cmd.Name
			if len(cmd.Path) > 0 {
				declared = cmd.Path[0]
			}
			mark = fmt.Sprintf("  (unreachable: '%s' is a reserved DVA command; rename to '%s')",
				unroutable, config.RenameSuggestion(declared))
		case shadowedBy != "":
			mark = fmt.Sprintf("  (built-in '%s' takes this name; run: %s)", shadowedBy, usage)
		}
		if lsDetailed {
			runnerType := runner.DetectRunnerType(cmd)
			detail := ""
			if cmd.Service != "" {
				detail = fmt.Sprintf("service:%s", cmd.Service)
			} else if cmd.Pod != "" {
				detail = fmt.Sprintf("pod:%s", cmd.Pod)
			}
			fmt.Printf("%-*s  [%-14s]  %-20s  %s%s\n", maxName, k, runnerType, detail, cmd.Command, mark)
			if cmd.Description != "" {
				fmt.Printf("%s  # %s\n", strings.Repeat(" ", maxName), cmd.Description)
			}
		} else {
			if cmd.Description != "" {
				fmt.Printf("%-*s  # %s%s\n", maxName, k, cmd.Description, mark)
			} else {
				fmt.Printf("%-*s%s\n", maxName, k, mark)
			}
		}
	}

	if len(c.Plans) > 0 {
		planKeys := sortedKeys(c.Plans)
		fmt.Println()
		fmt.Println("Plans (dva up <name>):")
		maxLen := maxKeyLen(planKeys)
		for _, name := range planKeys {
			plan := c.Plans[name]
			desc := plan.Description
			if desc == "" {
				entryNames := make([]string, 0, len(plan.Entries))
				for _, e := range plan.Entries {
					entryNames = append(entryNames, e.Name)
				}
				desc = strings.Join(entryNames, " → ")
			}
			fmt.Printf("  %-*s  # %s\n", maxLen, name, desc)
		}
	}
	return nil
}

// shellJoin renders a command path as the tokens a shell has to receive for the whole path to
// arrive as separate arguments. Only whitespace forces quoting: every other character the
// interaction key pattern admits (\w, -, ., :, /) is inert to a shell, and quoting all of them
// would make every example `dva ls` prints read as a literal. TASK-097.
func shellJoin(path []string) string {
	tokens := make([]string, len(path))
	for i, seg := range path {
		if strings.ContainsAny(seg, " \t\n") {
			seg = "'" + strings.ReplaceAll(seg, "'", `'\''`) + "'"
		}
		tokens[i] = seg
	}
	return strings.Join(tokens, " ")
}

// interactionUsage returns the invocation that reaches cmd, and the built-in that runs instead
// when the bare form is typed ("" when nothing does).
//
// One function because `dva ls` and `dva manifest` describe the same key to the same reader and
// must not disagree about which form works — the manifest used to print `dva build` for an
// interaction that `dva build` provably could not reach.
//
// It reads cmd.Path, not cmd.Name. The two carry the same information only while no segment
// contains a space, and one legally can: schema.json's interaction key pattern includes \s and
// nothing Go-side rejects it, so `interaction: {"my task": ...}` parses. Splitting the joined
// name on its first space then reports the parent as `my`, and the emitted `dva my task` is the
// one form that provably does not run — bare routing looks up `args[0]`, and `my` is not a key.
// Measured: `dva my task` exits 1, `dva 'my task'` runs it.
//
// A subcommand is reached through its parent, so it is the parent's name the built-in set is
// asked about. The parent's hook exemption does not carry over: measured, a `build` declaring
// replace: dispatches to that hook for `dva build fast` and ignores the argument, so the
// subcommand is reachable only through `dva run` even though the parent itself is reachable
// bare. That is why the exemption lives in ShadowedByBuiltin, consulted here for top-level
// paths only.
// interactionUsage reports how an interaction is actually invoked, and which of the two
// conflict states it is in. It is the single source for `dva manifest` and `dva ls --json`
// so the two machine-readable surfaces cannot describe the same key differently.
//
// unroutable is returned for a key no invocation reaches (TASK-137). usage is empty in
// that case: every candidate string exits non-zero, and usage_example carries an implicit
// promise that running it works. Naming the rename suggestion instead would be a second
// lie — `dva app-build` does not exist until the author performs the rename.
func interactionUsage(c *config.Config, cmd *runner.ResolvedCommand) (usage, shadowedBy, unroutable string) {
	path := cmd.Path
	if len(path) == 0 {
		// A ResolvedCommand that did not come from the tree walk. Treating the whole name as one
		// segment is the honest reading — it is what the caller named — and it is what this
		// function did for every key before the path existed.
		path = []string{cmd.Name}
	}
	root := path[0]
	form := shellJoin(path)

	switch {
	// Unroutable first: a namespaced key reaches neither branch below, and both of those
	// describe keys that still run.
	//
	// No length guard. A declared `app:build` with a subcommands: block flattens into two
	// entries — `app:build` and `app:build fast` — and the dead prefix kills both: the
	// longer form fails identically, because run.go splits args[0] and never looks at the
	// rest. Guarding on len(path)==1 marked the parent and advertised the child as
	// `dva app:build fast`, which is the exact promise this mark exists to retract.
	case config.UnroutableNamespacePrefix(root) != "":
		return "", "", config.UnroutableNamespacePrefix(root)
	case len(path) > 1 && config.IsReservedCommand(root):
		return fmt.Sprintf("dva run %s", form), root, ""
	case len(path) == 1 && config.ShadowedByBuiltin(root, c.Interaction[root]):
		// c.Interaction, not the tree: hooks live on the declared command, and a single-segment
		// path is always a declared key, so this lookup is exact.
		return fmt.Sprintf("dva run %s", form), root, ""
	}
	return fmt.Sprintf("dva %s", form), "", ""
}

func buildCommandEntries(c *config.Config, commands map[string]*runner.ResolvedCommand, keys []string) map[string]any {
	entries := make(map[string]any, len(keys))
	for _, k := range keys {
		cmd := commands[k]
		entry := map[string]any{
			"command": cmd.Command,
			"runner":  runner.DetectRunnerType(cmd),
			"shell":   cmd.Shell,
		}
		if cmd.Description != "" {
			entry["description"] = cmd.Description
		}
		if cmd.Service != "" {
			entry["service"] = cmd.Service
			entry["compose_method"] = cmd.Compose.Method
		}
		if cmd.Pod != "" {
			entry["pod"] = cmd.Pod
		}
		// Only on the conflicted entries, so the field's presence is the signal and a consumer
		// never has to parse the description to learn the short form will not reach this.
		_, shadowedBy, unroutable := interactionUsage(c, cmd)
		if shadowedBy != "" {
			entry["shadowed_by_builtin"] = shadowedBy
		}
		if unroutable != "" {
			entry["unroutable"] = unroutable
			entry["unroutable_reason"] = config.ConflictAdvice(cmd.Name)
		}
		entries[k] = entry
	}
	return entries
}

func printJSON(c *config.Config, commands map[string]*runner.ResolvedCommand, keys []string) error {
	entries := buildCommandEntries(c, commands, keys)
	if len(c.Plans) == 0 {
		return output.PrintJSON(entries)
	}

	plans := make(map[string]any, len(c.Plans))
	for _, name := range sortedKeys(c.Plans) {
		p := c.Plans[name]
		if p == nil {
			continue
		}
		entryNames := make([]string, 0, len(p.Entries))
		for _, e := range p.Entries {
			entryNames = append(entryNames, e.Name)
		}
		plans[name] = map[string]any{
			"description": p.Description,
			"environment": p.Environment,
			"site":        p.Site,
			"entries":     entryNames,
		}
	}

	return output.PrintJSON(map[string]any{
		"interaction_commands": entries,
		"plans":                plans,
	})
}

func printYAML(c *config.Config, commands map[string]*runner.ResolvedCommand, keys []string) error {
	entries := buildCommandEntries(c, commands, keys)
	if len(c.Plans) == 0 {
		return output.PrintYAML(entries)
	}

	plans := make(map[string]any, len(c.Plans))
	for _, name := range sortedKeys(c.Plans) {
		p := c.Plans[name]
		if p == nil {
			continue
		}
		entryNames := make([]string, 0, len(p.Entries))
		for _, e := range p.Entries {
			entryNames = append(entryNames, e.Name)
		}
		plans[name] = map[string]any{
			"description": p.Description,
			"environment": p.Environment,
			"site":        p.Site,
			"entries":     entryNames,
		}
	}

	return output.PrintYAML(map[string]any{
		"interaction_commands": entries,
		"plans":                plans,
	})
}
