package config

import (
	"fmt"
	"log/slog"
	"maps"
	"sort"
	"strings"
)

// reservedCommands is the canonical set of built-in DVA command names.
// Use IsReservedCommand or ReservedCommandNames for read-only access.
var reservedCommands = map[string]bool{
	"help": true, "version": true, "ls": true, "compose": true,
	"up": true, "stop": true, "down": true, "build": true,
	"run": true, "provision": true, "validate": true, "manifest": true,
	"ktl": true, "ssh": true, "console": true,
	"completion": true, "init": true, "status": true, "config": true,
	"logs": true, "restart": true, "show": true, "doctor": true,
}

// hookableCommands is the subset of reserved commands that support
// before/replace/after hooks via the interaction section.
//
// `clean` left this set with the command: teardown is `dva down <plan> --purge`, and a flag
// has no interaction key to hang a hook on. It is the one removal that could go quiet —
// `stack`/`app`/`infra` were never hookable, so nobody had a working hook under those names
// to lose — so validateHookPlacement carries a message naming the removal by name.
var hookableCommands = map[string]bool{
	"up": true, "down": true, "stop": true,
	"restart": true, "build": true,
	"logs": true,
}

// IsHookableCommand reports whether name is a built-in command that
// supports before/replace/after hooks.
func IsHookableCommand(name string) bool {
	return hookableCommands[name]
}

// HookableCommandList renders the hookable set as the sentence fragment the two hook
// placement errors both end in.
//
// Derived rather than written out. The literal list said "up, down, stop, restart, build,
// clean, logs" in both messages, so removing `clean` from the set above would have left two
// messages advertising a command the same file no longer accepts — the reader is told to
// use a name that fails the check that printed the advice.
func HookableCommandList() string {
	names := make([]string, 0, len(hookableCommands))
	for name := range hookableCommands {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

// HookableCommands returns a copy of the hookable command set.
func HookableCommands() map[string]bool {
	cp := make(map[string]bool, len(hookableCommands))
	maps.Copy(cp, hookableCommands)
	return cp
}

// ReservedCommands returns a copy of the built-in DVA command set.
// A custom interaction declared under one of these names is rejected by Validate; nothing about
// it is silent. See ShadowedByBuiltin for what the bare form actually runs, and ConflictAdvice
// for the way out, which differs by the kind of collision.
func ReservedCommands() map[string]bool {
	cp := make(map[string]bool, len(reservedCommands))
	maps.Copy(cp, reservedCommands)
	return cp
}

// IsReservedCommand reports whether name is a built-in DVA command.
func IsReservedCommand(name string) bool {
	return reservedCommands[name]
}

// ShadowedByBuiltin reports whether the bare `dva <name>` form runs a built-in command
// instead of the interaction declared under name.
//
// cli.Execute only rewrites `dva <name>` to `dva run <name>` when name is not a built-in, so a
// reserved name never reaches its interaction that way. It stays reachable as `dva run <name>`,
// which is why the condition is shadowing and not the "ignored" this once claimed to be.
//
// A hookable built-in that declares before/replace/after is not shadowed: the built-in runs the
// hook, so the bare form does reach what the author declared. A namespaced name like `compose:ps`
// is not shadowed either — the colon keeps it out of the built-in set, so dynamic routing sends
// it to `run` — even though ValidateReservedCommands still rejects the prefix.
func ShadowedByBuiltin(name string, cmd *InteractionCommand) bool {
	if !IsReservedCommand(name) {
		return false
	}
	if IsHookableCommand(name) && cmd != nil && cmd.HasHooks() {
		return false
	}
	return true
}

// UnroutableNamespacePrefix returns the reserved built-in named by a namespaced
// interaction key's prefix, or "" when the key carries no such prefix.
//
// This is a third state, distinct from ShadowedByBuiltin. A shadowed key still runs —
// `dva run <name>` reaches it — so the surfaces can name a working invocation. A key
// like `compose:ps` is reached by nothing: the colon keeps it out of the built-in set,
// and the run form reads `compose:` as a subproject reference. ConflictAdvice carries the
// full reason and the way out; this reports only whether the condition holds, so the
// machine-readable surfaces and the validator cannot disagree about which keys it
// covers.
//
// Scope is deliberately narrow: a *reserved* prefix, and since TASK-167 that is the whole
// unroutable class rather than a subset of it. A free prefix (`mytool:fast`) used to fail
// the same way — run.go split every key on ':' — and TASK-167 answered that as the routing
// question it was: LiteralKeyWins now routes such a key to the command the author declared.
// The reserved prefix is the one case it excepts, so this predicate still describes exactly
// the keys nothing can reach.
func UnroutableNamespacePrefix(name string) string {
	idx := strings.Index(name, ":")
	if idx <= 0 {
		return ""
	}
	if prefix := name[:idx]; IsReservedCommand(prefix) {
		return prefix
	}
	return ""
}

// LiteralKeyWins reports whether a command name should be looked up as an interaction key
// as written, rather than split on ':' and read as `<subproject>:<command>`.
//
// run.go used to split unconditionally, before asking whether the literal key existed. That
// made every colon key whose prefix was not a declared subproject reachable by nothing:
// `dva mytool:fast` exited 1 with "subproject `mytool` not found" while `dva config validate`
// said the file was valid and `dva manifest` advertised `usage_example: dva mytool:fast`.
// The colon was never reserved for subprojects by the schema, the docs, or the config model
// — only by that one SplitN — so this makes routing agree with the four surfaces that
// already treated the key as an ordinary command.
//
// An exact match beats an inference: a declared key is what the author wrote, a subproject
// reference is what the shape suggests. A real subproject is unaffected, because a parent
// declaring `subprojects: {engine: ...}` has no literal `engine:test` key of its own — that
// command lives in the child's dva.yml.
//
// The reserved prefix is excepted so `compose:ps` keeps failing. Validate rejects such a
// config outright (rc 1, reserved conflict), and routing it here would ship a file that one
// surface calls a hard error while another runs it happily — the disagreement TASK-137 was
// about. It also keeps `unroutable_reason` ("prefix is a reserved DVA command") a true and
// complete account of what is left unroutable. TASK-167.
//
// The exception is keyed off the live reserved set, so removing a built-in moves keys out of
// it: `app:build` was unroutable while `app` was a command and is an ordinary interaction
// now. That is the intended consequence — the prefix names nothing DVA owns any more — and
// it is pinned by a test rather than left to be discovered.
func LiteralKeyWins(c *Config, name string) bool {
	if !strings.Contains(name, ":") {
		return false
	}
	if UnroutableNamespacePrefix(name) != "" {
		return false
	}
	_, declared := c.Interaction[name]
	return declared
}

// RenameSuggestion returns the name a colon-carrying key should be renamed to.
//
// Every colon goes, not just the first. `compose:sub:cmd` → `compose-sub:cmd` still carries
// a colon, so run.go still splits it and it still fails — and because `compose-sub` is not a
// reserved command, UnroutableNamespacePrefix no longer covers it, so validate reports
// the config clean and ls shows the entry unmarked. Following the advice literally turned
// a loud error into the silent one TASK-167 describes; measured before the fix on `dva
// app-sub:cmd` (back when `app` was a built-in), which exited 1 with subproject `app-sub`
// not found while validate said valid.
//
// One function because the advice string and the ls mark both need it and had drifted
// into two copies of the same expression.
func RenameSuggestion(name string) string {
	return strings.ReplaceAll(name, ":", "-")
}

// ReservedCommandConflict represents a conflict between an interaction
// command name and a reserved built-in command.
type ReservedCommandConflict struct {
	Name   string
	Source string // "interaction", "module:<name>", "override"
}

// ValidateReservedCommands checks if any interaction command names
// conflict with reserved built-in command names.
// Commands that define hook fields (before/replace/after) on hookable
// built-in commands are NOT treated as conflicts.
// Namespaced names like "compose:ps" are also rejected when the prefix
// before ':' is a reserved command (e.g., "compose", "run").
func ValidateReservedCommands(interaction map[string]*InteractionCommand) []ReservedCommandConflict {
	var conflicts []ReservedCommandConflict
	for name, cmd := range interaction {
		if IsReservedCommand(name) {
			// ShadowedByBuiltin carries the hook exemption, so the surfaces that mark a
			// conflict and the validator that rejects one cannot disagree about which
			// names are conflicts.
			if ShadowedByBuiltin(name, cmd) {
				conflicts = append(conflicts, ReservedCommandConflict{
					Name:   name,
					Source: "interaction",
				})
			}
			continue
		}
		// Check namespace prefix: "compose:ps" conflicts if "compose" is reserved. Shared with
		// the manifest/ls mark, so a key the surfaces call unroutable is exactly a key the
		// validator rejects.
		if UnroutableNamespacePrefix(name) != "" {
			conflicts = append(conflicts, ReservedCommandConflict{
				Name:   name,
				Source: "interaction",
			})
		}
	}
	return conflicts
}

// ConflictAdvice returns what happens to an interaction declared under name, and the way out.
//
// One function because the warning on every config load and the error from validate describe the
// same condition to the same reader. They disagreed before: the warning said the interaction was
// discarded while the error said the config was fatal, and neither named the invocation that works.
//
// Every invocation named here was executed against the binary. That is the bar, because advice
// that names a command which refuses is worse than no advice — the reader trusts it and stops.
func ConflictAdvice(name string) string {
	// Namespaced keys first: this is the one case reachable by no invocation at all, so it is
	// also the one case where "rename" is the whole answer rather than a preference.
	if idx := strings.Index(name, ":"); idx > 0 {
		// The cause is spelled out but no failing invocation is written in full: this text is
		// read by machines (it reaches the reader through validate's stderr and the load-time
		// warning), and a consumer scanning for a `dva run …` form would lift it out of the
		// negation and execute the one command that cannot work.
		return fmt.Sprintf(
			"namespace prefix '%s' is a reserved DVA command — no invocation reaches this key: "+
				"the bare form is not a built-in, and the run form reads '%s:' as a subproject "+
				"reference, so it fails with subproject '%s' not found. Use a different "+
				"separator (e.g., '%s')",
			name[:idx], name[:idx], name[:idx], RenameSuggestion(name),
		)
	}
	if IsHookableCommand(name) {
		// The hook route is listed first because it is the only one that gets the short form
		// working; `dva run` is the fallback for an author who wants a separate command.
		return fmt.Sprintf(
			"'%s' is a reserved DVA command — declare before/replace/after to extend the "+
				"built-in, or reach this as 'dva run %s'",
			name, name,
		)
	}
	// The interaction is not discarded: measured, `dva run status` still executes it. Only the
	// bare `dva status` form is lost, to the built-in. Telling the reader it was ignored sends
	// them looking for a command that never ran, when what they need is the form that reaches it.
	//
	// The old wording is deliberately not quoted anywhere in this file, comments included: a
	// grep for it is one of TASK-076's checks, and a comment containing the phrase would satisfy
	// the grep while the message regressed — the check has to fail for exactly one reason.
	return fmt.Sprintf(
		"'%s' is a reserved DVA command — the built-in runs for 'dva %s', so this is reachable "+
			"only as 'dva run %s'. Rename to get the short form back (e.g., 'my-%s')",
		name, name, name, name,
	)
}

// FormatConflictWarnings formats conflict list as warning messages.
func FormatConflictWarnings(conflicts []ReservedCommandConflict) string {
	if len(conflicts) == 0 {
		return ""
	}

	// Sorted, and one clause per conflict: conflicts arrive in map iteration order, so a message
	// that detailed only conflicts[0] named a different command on every run — and the three
	// kinds of conflict do not share advice, so one clause cannot stand in for the others.
	names := make([]string, 0, len(conflicts))
	for _, c := range conflicts {
		names = append(names, c.Name)
	}
	sort.Strings(names)

	clauses := make([]string, 0, len(names))
	for _, name := range names {
		clauses = append(clauses, fmt.Sprintf("interaction command %s", ConflictAdvice(name)))
	}
	return strings.Join(clauses, "; ")
}

// WarnReservedCommandConflicts logs warnings for any conflicts found.
func WarnReservedCommandConflicts(interaction map[string]*InteractionCommand) []ReservedCommandConflict {
	conflicts := ValidateReservedCommands(interaction)
	if len(conflicts) > 0 {
		slog.Warn(FormatConflictWarnings(conflicts))
	}
	return conflicts
}
