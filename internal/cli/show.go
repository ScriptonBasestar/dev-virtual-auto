package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ScriptonBasestar/dva/internal/config"
	"github.com/ScriptonBasestar/dva/internal/output"
)

var showCmd = &cobra.Command{
	Use:   "show",
	Short: "Show registered configuration summary (stack entries, plans, commands)",
	Long: `Display a human-readable summary of the current dva.yml configuration.

One section per declared area — stack entries and the runners each declares, plans,
modes (--mode), environments (--env), interaction commands, provision profiles,
health checks, subprojects — and areas the config does not declare are omitted.

Stack rows name the entry, which is the argument 'dva stack up <name>' takes.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		c := mustLoadConfig()

		if jsonOutput {
			return showJSON(c)
		}
		return showText(c)
	},
}

// stackEntryView is the declaration-level answer to "what did dva.yml register?": the entry's own
// name, the runners it declares, and where it sits in the sequence.
//
// `dva status` answers the runtime question instead — it asks the orchestrator what is currently
// up — so its `[infra] script` labels cannot be reused here. `show` has to work with nothing
// running, which is when a reader most needs to know what the names are.
type stackEntryView struct {
	Name        string
	Description string
	Runners     []string
	Default     string
	Order       int
}

// stackViews returns one view per declared stack entry in declaration order — by `order`, then by
// name — which is what the entries themselves declare, not a prediction of any command's sequence.
// `dva stack up` reads the same field but currently rotates equal orders (TASK-084), and
// `dva up <plan>` ignores stack order entirely: NewPlanOrchestrator walks the plan's own entries,
// each carrying its own order and runner.
//
// SortedStack() is the source for both the order and for populating Name from the map key. It sorts
// on Order alone with an unstable sort, so equal orders arrive here in map-iteration order; the
// name tiebreak below is local until TASK-084 gives SortedStack the same tiebreak
// PrimaryComposeEntry already documents, at which point this sort can go.
func stackViews(c *config.Config) []stackEntryView {
	entries := c.SortedStack()
	views := make([]stackEntryView, 0, len(entries))
	for _, e := range entries {
		views = append(views, stackEntryView{
			Name:        e.Name,
			Description: e.Description,
			Runners:     e.RunnerNames(),
			// Canonicalized, so it is comparable with Runners below and so a JSON consumer can
			// match it against an element of `runners` as the schema promises.
			Default: e.DefaultRunnerName(),
			Order:   e.Order,
		})
	}
	// Not SliceStable: Name comes from a map key, so (Order, Name) is already a strict total order
	// and stability would add nothing. Saying Slice makes it clear the comparator supplies the
	// determinism, not the incoming sequence.
	sort.Slice(views, func(i, j int) bool {
		if views[i].Order != views[j].Order {
			return views[i].Order < views[j].Order
		}
		return views[i].Name < views[j].Name
	})
	return views
}

// stackEntryDetail renders one entry's declared runners and sequence position.
//
// Runner names are echoed as the author spelled them — RunnerNames() canonicalizes only the
// podman_compose spelling — so the output greps back to a key in the reader's own dva.yml rather
// than to a plugin name they never typed.
func stackEntryDetail(v stackEntryView) string {
	var parts []string
	switch {
	case len(v.Runners) == 0:
		// A live shape, not a test artifact: a stack entry has no required fields in schema.json,
		// so `void: {description: ...}` loads and validates. Plugin inference does not rescue it
		// either — DetectPlugin only fires when the entry name is itself a known plugin name.
		// Saying "no runner declared" beats an empty bracket that reads like a rendering bug.
		parts = append(parts, "no runner declared")
	case len(v.Runners) == 1:
		parts = append(parts, "runner:"+v.Runners[0])
	default:
		parts = append(parts, "runners:"+strings.Join(v.Runners, ","))
	}
	// Named unless it is already the single runner printed above: with two or more runners it is
	// the one that runs, and a default naming a runner the entry does not declare is worth
	// seeing rather than hiding behind the list. Both sides are canonicalized, so a
	// `podman_compose` default against a `podman-compose` runner counts as the same runner.
	//
	// showJSON deliberately keeps the default even when this hides it: a consumer reconstructing
	// the file needs to know the key was written, while a human does not need the word twice.
	if v.Default != "" && (len(v.Runners) != 1 || v.Runners[0] != v.Default) {
		parts = append(parts, "default:"+v.Default)
	}
	if v.Order != 0 {
		parts = append(parts, fmt.Sprintf("order:%d", v.Order))
	}
	detail := "[" + strings.Join(parts, ", ") + "]"
	if v.Description != "" {
		return v.Description + " " + detail
	}
	return detail
}

func showText(c *config.Config) error {
	// Header
	fmt.Printf("DVA v%s\n", config.Version)
	fmt.Printf("Config: %s\n", c.FilePath())
	if c.Version != "" {
		fmt.Printf("  Required version: %s\n", c.Version)
	}

	// Stack entries, before Compose: the Compose block reports one *runner*'s settings, so on its
	// own it never names the entry those settings belong to — and the entry name is what
	// `dva stack up <name>` and the tag filters take.
	if views := stackViews(c); len(views) > 0 {
		fmt.Println()
		fmt.Println("Stack (dva stack up <name>):")
		names := make([]string, 0, len(views))
		for _, v := range views {
			names = append(names, v.Name)
		}
		maxLen := maxKeyLen(names)
		for _, v := range views {
			fmt.Printf("  %-*s  %s\n", maxLen, v.Name, stackEntryDetail(v))
		}
	}

	// Lifecycle / Compose
	if cc := c.PrimaryComposeConfig(); cc != nil {
		if cc.ProjectName != "" || len(cc.Files) > 0 {
			fmt.Println()
			fmt.Println("Compose:")
			if cc.ProjectName != "" {
				fmt.Printf("  Project: %s\n", cc.ProjectName)
			}
			if len(cc.Files) > 0 {
				fmt.Printf("  Files:   %s\n", strings.Join(cc.Files, ", "))
			}
		}
	}

	// Modes (--mode)
	if len(c.Modes) > 0 {
		fmt.Println()
		fmt.Println("Modes (--mode/-M):")
		names := sortedKeys(c.Modes)
		maxLen := maxKeyLen(names)
		for _, name := range names {
			m := c.Modes[name]
			fmt.Printf("  %-*s  %s\n", maxLen, name, m.Description)
		}
	}

	// Environments (--env)
	if len(c.Environments) > 0 {
		fmt.Println()
		fmt.Println("Environments (--env/-E):")
		names := sortedKeys(c.Environments)
		maxLen := maxKeyLen(names)
		for _, name := range names {
			ep := c.Environments[name]
			desc := ep.Description
			var parts []string
			if len(ep.Environment) > 0 {
				parts = append(parts, fmt.Sprintf("%d vars", len(ep.Environment)))
			}
			if len(ep.Stack) > 0 {
				parts = append(parts, fmt.Sprintf("%d stack filters", len(ep.Stack)))
			}
			if len(parts) > 0 {
				if desc != "" {
					desc += " "
				}
				desc += "(" + strings.Join(parts, ", ") + ")"
			}
			fmt.Printf("  %-*s  %s\n", maxLen, name, desc)
		}
	}

	if len(c.Plans) > 0 {
		fmt.Println()
		fmt.Println("Plans (dva up <name>):")
		names := sortedKeys(c.Plans)
		maxLen := maxKeyLen(names)
		for _, name := range names {
			plan := c.Plans[name]
			if plan == nil {
				continue
			}
			var parts []string
			if plan.Environment != "" {
				parts = append(parts, "env:"+plan.Environment)
			}
			if plan.Site != "" {
				parts = append(parts, "site:"+plan.Site)
			}
			if len(plan.Entries) > 0 {
				parts = append(parts, fmt.Sprintf("%d entries", len(plan.Entries)))
			}
			desc := plan.Description
			if len(parts) > 0 {
				if desc == "" {
					desc = "[" + strings.Join(parts, ", ") + "]"
				} else {
					desc += " [" + strings.Join(parts, ", ") + "]"
				}
			}
			fmt.Printf("  %-*s  %s\n", maxLen, name, desc)
		}
	}

	if len(c.Sites) > 0 {
		fmt.Println()
		fmt.Println("Sites:")
		names := sortedKeys(c.Sites)
		maxLen := maxKeyLen(names)
		for _, name := range names {
			site := c.Sites[name]
			if site == nil {
				continue
			}
			desc := site.Description
			if len(site.Vars) > 0 {
				if desc == "" {
					desc = fmt.Sprintf("(%d vars)", len(site.Vars))
				} else {
					desc += fmt.Sprintf(" (%d vars)", len(site.Vars))
				}
			}
			fmt.Printf("  %-*s  %s\n", maxLen, name, desc)
		}
	}

	// Applications
	if len(c.Applications) > 0 {
		fmt.Println()
		fmt.Printf("Applications: %d defined\n", len(c.Applications))
		names := sortedKeys(c.Applications)
		maxLen := maxKeyLen(names)
		for _, name := range names {
			app := c.Applications[name]
			strategies := []string{}
			if app.Run.HasNative() || app.Dev.HasNative() {
				strategies = append(strategies, "native")
			}
			if app.Run.HasDocker() || app.Dev.HasDocker() {
				strategies = append(strategies, "docker")
			}
			desc := app.Description
			if len(strategies) > 0 {
				desc += fmt.Sprintf(" [%s]", strings.Join(strategies, "/"))
			}
			fmt.Printf("  %-*s  %s\n", maxLen, name, desc)
		}
	}

	// Interaction commands
	if len(c.Interaction) > 0 {
		fmt.Println()
		fmt.Printf("Interaction Commands: %d defined\n", len(c.Interaction))
		names := sortedKeys(c.Interaction)
		maxLen := maxKeyLen(names)
		for _, name := range names {
			ic := c.Interaction[name]
			desc := ic.Description
			subCount := countSubcommands(ic)
			if subCount > 0 {
				desc += fmt.Sprintf(" (+%d sub)", subCount)
			}
			fmt.Printf("  %-*s  %s\n", maxLen, name, desc)
		}
	}

	// Provision profiles
	if len(c.Provision.Profiles) > 0 {
		fmt.Println()
		names := sortedKeys(c.Provision.Profiles)
		if c.Provision.DefaultProfile != "" {
			for i, n := range names {
				if n == c.Provision.DefaultProfile {
					names[i] = n + " (default)"
				}
			}
		}
		fmt.Printf("Provision Profiles: %s\n", strings.Join(names, ", "))
	}

	// Health checks
	if len(c.HealthChecks) > 0 {
		fmt.Println()
		fmt.Printf("Health Checks: %s\n", strings.Join(sortedKeys(c.HealthChecks), ", "))
	}

	// Endpoints
	if len(c.Endpoints) > 0 {
		fmt.Println()
		fmt.Printf("Endpoints:\n")
		names := sortedKeys(c.Endpoints)
		maxLen := maxKeyLen(names)
		for _, name := range names {
			ep := c.Endpoints[name]
			fmt.Printf("  %-*s  %s  %s\n", maxLen, name, ep.Label, ep.URL)
		}
	}

	// Subprojects
	if len(c.Subprojects) > 0 {
		fmt.Println()
		fmt.Printf("Subprojects:\n")
		for _, name := range sortedKeys(c.Subprojects) {
			sub := c.Subprojects[name]
			tags := ""
			if len(sub.ExcludeTags) > 0 {
				tags = fmt.Sprintf(" (exclude: %s)", strings.Join(sub.ExcludeTags, ", "))
			}
			fmt.Printf("  %s -> %s%s\n", name, sub.Path, tags)
		}
	}

	// Infra
	if len(c.Infra) > 0 {
		fmt.Println()
		fmt.Printf("Infra: %s\n", strings.Join(sortedKeys(c.Infra), ", "))
	}

	// Environment variables count
	if len(c.Environment) > 0 {
		fmt.Println()
		fmt.Printf("Environment Variables: %d defined\n", len(c.Environment))
	}

	return nil
}

func showJSON(c *config.Config) error {
	data := map[string]any{
		"dva_version":    config.Version,
		"config_path":    c.FilePath(),
		"config_version": c.Version,
	}

	// Same source as the text view: a consumer that sees `compose` but no `stack` cannot tell
	// which entry the compose settings came from.
	if views := stackViews(c); len(views) > 0 {
		stack := make(map[string]any, len(views))
		for _, v := range views {
			entry := map[string]any{
				"description": v.Description,
				"runners":     v.Runners,
				"order":       v.Order,
			}
			if v.Default != "" {
				entry["default_runner"] = v.Default
			}
			stack[v.Name] = entry
		}
		data["stack"] = stack
	}

	if cc := c.PrimaryComposeConfig(); cc != nil {
		if cc.ProjectName != "" || len(cc.Files) > 0 {
			compose := map[string]any{}
			if cc.ProjectName != "" {
				compose["project_name"] = cc.ProjectName
			}
			if len(cc.Files) > 0 {
				compose["files"] = cc.Files
			}
			data["compose"] = compose
		}
	}

	if len(c.Modes) > 0 {
		modes := make(map[string]string, len(c.Modes))
		for k, v := range c.Modes {
			modes[k] = v.Description
		}
		data["modes"] = modes
	}

	if len(c.Environments) > 0 {
		envs := make(map[string]any, len(c.Environments))
		for k, v := range c.Environments {
			envs[k] = map[string]any{
				"description": v.Description,
				"vars_count":  len(v.Environment),
				"stack_count": len(v.Stack),
			}
		}
		data["environments"] = envs
	}

	if len(c.Plans) > 0 {
		plans := make(map[string]any, len(c.Plans))
		for k, v := range c.Plans {
			if v == nil {
				continue
			}
			entryNames := make([]string, 0, len(v.Entries))
			for _, e := range v.Entries {
				entryNames = append(entryNames, e.Name)
			}
			plans[k] = map[string]any{
				"description": v.Description,
				"environment": v.Environment,
				"site":        v.Site,
				"entries":     entryNames,
			}
		}
		data["plans"] = plans
	}

	if len(c.Sites) > 0 {
		sites := make(map[string]any, len(c.Sites))
		for k, v := range c.Sites {
			if v == nil {
				continue
			}
			sites[k] = map[string]any{
				"description": v.Description,
				"vars_count":  len(v.Vars),
			}
		}
		data["sites"] = sites
	}

	if len(c.Applications) > 0 {
		apps := make(map[string]any, len(c.Applications))
		for k, v := range c.Applications {
			entry := map[string]any{"description": v.Description}
			if v.Run.HasNative() || v.Dev.HasNative() {
				entry["native"] = true
			}
			if v.Run.HasDocker() || v.Dev.HasDocker() {
				entry["docker"] = true
			}
			if len(v.Tags) > 0 {
				entry["tags"] = v.Tags
			}
			apps[k] = entry
		}
		data["applications"] = apps
	}

	if len(c.Interaction) > 0 {
		cmds := make(map[string]any, len(c.Interaction))
		for k, v := range c.Interaction {
			entry := map[string]any{"description": v.Description}
			if sub := countSubcommands(v); sub > 0 {
				entry["subcommands"] = sub
			}
			cmds[k] = entry
		}
		data["interaction_commands"] = cmds
	}

	if len(c.Provision.Profiles) > 0 {
		provData := map[string]any{
			"profiles": sortedKeys(c.Provision.Profiles),
		}
		if c.Provision.DefaultProfile != "" {
			provData["default_profile"] = c.Provision.DefaultProfile
		}
		data["provision"] = provData
	}

	if len(c.HealthChecks) > 0 {
		data["health_checks"] = sortedKeys(c.HealthChecks)
	}

	if len(c.Endpoints) > 0 {
		eps := make(map[string]any, len(c.Endpoints))
		for k, v := range c.Endpoints {
			eps[k] = map[string]any{"label": v.Label, "url": v.URL}
		}
		data["endpoints"] = eps
	}

	if len(c.Subprojects) > 0 {
		subs := make(map[string]string, len(c.Subprojects))
		for k, v := range c.Subprojects {
			subs[k] = v.Path
		}
		data["subprojects"] = subs
	}

	if len(c.Infra) > 0 {
		data["infra"] = sortedKeys(c.Infra)
	}

	data["environment_variables_count"] = len(c.Environment)

	return output.PrintJSON(data)
}

func countSubcommands(ic *config.InteractionCommand) int {
	if ic.Subcommands == nil {
		return 0
	}
	count := len(ic.Subcommands)
	for _, sub := range ic.Subcommands {
		count += countSubcommands(sub)
	}
	return count
}

// sortedKeys returns sorted keys from a map. Uses type parameter for flexibility.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func maxKeyLen(keys []string) int {
	max := 0
	for _, k := range keys {
		if len(k) > max {
			max = len(k)
		}
	}
	return max
}
