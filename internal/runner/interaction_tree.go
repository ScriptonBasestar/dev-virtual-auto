package runner

import (
	"fmt"
	"maps"
	"sort"
	"strings"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// ResolvedCommand is the result of InteractionTree.Find().
type ResolvedCommand struct {
	// Path is the segments this command was reached through — ["rails"] for a top-level entry,
	// ["rails", "db", "migrate"] for a subcommand. Name is these joined by a space, and that join
	// is lossy: a segment may itself contain a space (schema.json admits \s in an interaction
	// key), so splitting Name back apart guesses. Read Path instead. TASK-097.
	Path         []string
	Name         string
	Description  string
	Service      string
	Command      string
	CommandLines []string               // non-nil when command: was a YAML list
	Script       string                 // inline shell script block
	ScriptFile   string                 // path to external shell script
	Steps        []config.ProvisionItem // named steps (sequential)
	Workdir      string
	User         string
	DefaultArgs  string
	Environment  map[string]string
	Shell        bool
	Entrypoint   string
	RunnerName   string
	Pod          string
	Compose      ComposeOpts
	Argv         []string
}

// ComposeOpts holds normalized compose options for a command.
type ComposeOpts struct {
	Method     string
	Profiles   []string
	RunOptions []string
}

// HasExecutionTarget reports whether the resolved node has anything Execute would run.
//
// This is the post-merge twin of config.InteractionCommand.hasExecutionTarget (TASK-165 /
// TASK-173). It lives on ResolvedCommand because inheritance is already applied by
// InteractionTree.Find — the CLI resolve step should not re-walk parents. Fields that only
// exist on InteractionCommand (hooks) are intentionally absent: interaction hooks are not
// executed by the runner path, so they must not keep an otherwise empty node green at run
// time while validate already treats them as a target for warning suppression.
//
// Drift guard: every field here maps to a field hasExecutionTarget reads (or Argv, which is
// the post-resolve form of invocation args / default_args). A new execution form must update
// both predicates; classifyForm is the third place that names forms for the runners.
func (cmd *ResolvedCommand) HasExecutionTarget() bool {
	if cmd == nil {
		return false
	}
	return strings.TrimSpace(cmd.Command) != "" ||
		len(cmd.CommandLines) > 0 ||
		cmd.Script != "" ||
		cmd.ScriptFile != "" ||
		len(cmd.Steps) > 0 ||
		cmd.Service != "" ||
		cmd.Pod != "" ||
		cmd.RunnerName != "" ||
		strings.TrimSpace(cmd.DefaultArgs) != "" ||
		len(cmd.Argv) > 0
}

// ErrNothingToRun is returned when dva run resolves a node with no execution target.
func ErrNothingToRun(cmd *ResolvedCommand) error {
	name := "this interaction"
	if cmd != nil && cmd.Name != "" {
		name = cmd.Name
	}
	// The trailing colon belongs to the `default_args:` YAML key, not to the sentence, and
	// the same key list is printed verbatim by runner.go's --explain path. Rewriting the
	// list to satisfy ST1005 would leave the two copies spelling the keys differently.
	return fmt.Errorf( //nolint:staticcheck // ST1005: last token is a YAML key, not punctuation
		"%s has nothing to run — add command:, script:, script_file:, steps:, service:, pod:, or default_args:",
		name,
	)
}

// InteractionTree resolves dva.yml interaction commands with subcommand support.
type InteractionTree struct {
	entries map[string]*config.InteractionCommand
}

// NewInteractionTree creates an InteractionTree from config interaction map.
func NewInteractionTree(entries map[string]*config.InteractionCommand) *InteractionTree {
	return &InteractionTree{entries: entries}
}

// Find resolves a command name (with optional sub-args) to a ResolvedCommand.
func (t *InteractionTree) Find(name string, argv ...string) *ResolvedCommand {
	entry, ok := t.entries[name]
	if !ok {
		return nil
	}

	// Expand into flat command map
	commands := t.expand(name, entry)

	// Try progressively shorter key combos (name + args)
	keys := append([]string{name}, argv...)
	var rest []string

	for i := len(keys); i > 0; i-- {
		key := strings.Join(keys[:i], " ")
		if cmd, ok := commands[key]; ok {
			// Remaining args become argv
			rest = keys[i:]
			cmd.Argv = rest
			return cmd
		}
	}

	return nil
}

// Collision records two declarations that resolve to the same command name.
//
// Winner is the path that keeps the name; Loser is the one dropped. Which is which is decided by
// the sorted walk — that is, by the config — and not by Go's map seed, so it is reproducible.
type Collision struct {
	Key    string
	Winner []string
	Loser  []string
}

// List returns all commands (including subcommands) as a flat map.
func (t *InteractionTree) List() map[string]*ResolvedCommand {
	commands, _ := t.ListWithCollisions()
	return commands
}

// ListWithCollisions returns the same map as List plus every command name that more than one
// declaration produces.
//
// It is one walk rather than a second traversal beside List, because a collision detector that
// re-derives what the expansion already knows is a second source of truth and the two drift —
// the lesson TASK-101 and TASK-097 both landed on.
func (t *InteractionTree) ListWithCollisions() (map[string]*ResolvedCommand, []Collision) {
	result := make(map[string]*ResolvedCommand)
	var collisions []Collision
	for _, name := range sortedNames(t.entries) {
		t.expandInto([]string{name}, t.entries[name], result, &collisions)
	}
	return result, collisions
}

// expand builds a flat map of all commands for a given entry.
func (t *InteractionTree) expand(name string, entry *config.InteractionCommand) map[string]*ResolvedCommand {
	result := make(map[string]*ResolvedCommand)
	t.expandInto([]string{name}, entry, result, nil)
	return result
}

// sortedNames returns an interaction map's keys in a fixed order.
//
// Go randomizes map iteration, and expandInto writes into one flat key space where two different
// declarations can spell the same key. Ranging at random meant the survivor changed between runs
// of the same binary on the same file — including inside Find, so `dva run a b c` executed
// different commands on consecutive invocations. Sorting does not resolve the collision; it makes
// the resolution a property of the config, which is what SOUL.md 신념 2 requires. TASK-104.
func sortedNames(m map[string]*config.InteractionCommand) []string {
	names := make([]string, 0, len(m))
	for name := range m {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// expandInto walks entry and its subcommands into result, keyed by the space-joined path.
//
// It takes the path rather than the joined name so that ResolvedCommand can carry the segments
// it was built from: the join is one-way once any segment contains a space, and a consumer that
// re-splits the key gets the boundary wrong. TASK-097.
//
// collisions may be nil, for callers that only want the map.
func (t *InteractionTree) expandInto(path []string, entry *config.InteractionCommand, result map[string]*ResolvedCommand, collisions *[]Collision) {
	name := strings.Join(path, " ")

	// First writer wins, and the walk above is sorted, so the winner is stable. Overwriting was
	// what made the survivor depend on the map seed; refusing to overwrite is what makes it not.
	// Reporting the loser is Collisions' job — dropping it silently is the defect. TASK-104.
	if existing, taken := result[name]; taken {
		if collisions != nil {
			*collisions = append(*collisions, Collision{Key: name, Winner: existing.Path, Loser: path})
		}
	} else {
		cmd := buildResolved(name, entry)
		cmd.Path = path
		result[name] = cmd
	}

	// Recursion continues even when the name above was refused: the losing declaration's own
	// children sit at longer paths that need not collide with anything.
	for _, subName := range sortedNames(entry.Subcommands) {
		merged := mergeInteraction(entry, entry.Subcommands[subName])
		// A fresh backing array per child. append(path, subName) would let two siblings share
		// one array and overwrite each other's last segment, so the paths would come out
		// correct only for whichever child ran last.
		child := append(append(make([]string, 0, len(path)+1), path...), subName)
		t.expandInto(child, merged, result, collisions)
	}
}

// buildResolved converts InteractionCommand to ResolvedCommand.
func buildResolved(name string, entry *config.InteractionCommand) *ResolvedCommand {
	cmd := &ResolvedCommand{
		Name:         name,
		Description:  entry.Description,
		Service:      entry.Service,
		Command:      strings.TrimSpace(entry.Command),
		CommandLines: entry.CommandLines,
		Script:       entry.Script,
		ScriptFile:   entry.ScriptFile,
		Steps:        entry.Steps,
		Workdir:      entry.Workdir,
		User:         entry.User,
		DefaultArgs:  strings.TrimSpace(entry.DefaultArgs),
		Environment:  entry.Environment,
		Shell:        entry.ShellEnabled(),
		Entrypoint:   entry.Entrypoint,
		RunnerName:   entry.Runner,
		Pod:          entry.Pod,
	}

	if cmd.Environment == nil {
		cmd.Environment = make(map[string]string)
	}

	// Normalize compose options
	cmd.Compose = normalizeCompose(entry)

	return cmd
}

func normalizeCompose(entry *config.InteractionCommand) ComposeOpts {
	opts := ComposeOpts{
		Method: "run",
	}

	if entry.Compose != nil {
		if entry.Compose.Method != "" {
			opts.Method = entry.Compose.Method
		}
		opts.Profiles = entry.Compose.Profiles
		opts.RunOptions = normalizeRunOptions(entry.Compose.RunOptions)
	}

	return opts
}

// normalizeRunOptions ensures each option has a dash prefix.
func normalizeRunOptions(options []string) []string {
	if len(options) == 0 {
		return nil
	}
	result := make([]string, 0, len(options))
	for _, o := range options {
		if strings.HasPrefix(o, "-") {
			result = append(result, o)
		} else {
			result = append(result, "--"+o)
		}
	}
	return result
}

// mergeInteraction merges a parent interaction with a subcommand entry.
func mergeInteraction(parent, child *config.InteractionCommand) *config.InteractionCommand {
	merged := &config.InteractionCommand{
		Description:  parent.Description,
		Service:      parent.Service,
		Command:      parent.Command,
		CommandLines: parent.CommandLines,
		Script:       parent.Script,
		ScriptFile:   parent.ScriptFile,
		Steps:        parent.Steps,
		Workdir:      parent.Workdir,
		User:         parent.User,
		DefaultArgs:  parent.DefaultArgs,
		Environment:  copyMap(parent.Environment),
		Shell:        parent.Shell,
		Entrypoint:   parent.Entrypoint,
		Runner:       parent.Runner,
		Pod:          parent.Pod,
		Compose:      parent.Compose,
	}

	// The one field taken from the child rather than inherited from the parent, and the
	// reason it is not in the literal above: expandInto recurses on merged.Subcommands, so
	// carrying the parent's map through would re-expand the parent's own children under
	// every child name and never terminate. Leaving it nil — which is what this did before
	// TASK-095 — terminated instead, one level too early: anything nested three deep was
	// silently dropped, including `rails db migrate` and `rails db seed` in
	// examples/full-stack.yml.
	merged.Subcommands = child.Subcommands

	// Override with child values
	if child.Description != "" {
		merged.Description = child.Description
	}
	if child.Service != "" {
		merged.Service = child.Service
	}
	if child.Command != "" {
		merged.Command = child.Command
		merged.CommandLines = nil // scalar overrides list
	}
	if child.CommandLines != nil {
		merged.CommandLines = child.CommandLines
		if len(child.CommandLines) > 0 {
			merged.Command = child.CommandLines[0]
		}
	}
	if child.Script != "" {
		merged.Script = child.Script
	}
	if child.ScriptFile != "" {
		merged.ScriptFile = child.ScriptFile
	}
	if child.Steps != nil {
		merged.Steps = child.Steps
	}
	if child.Workdir != "" {
		merged.Workdir = child.Workdir
	}
	if child.User != "" {
		merged.User = child.User
	}
	// default_args belongs to the command it was written for. A child's command: *replaces* the
	// parent's outright — see the child.Command / child.CommandLines blocks above, which assign
	// rather than append — so inheriting the parent's arguments across that replacement attaches
	// them to a command that never asked for them: in examples/full-stack.yml `dva run rails
	// console` ran `console server -p 3000 -b 0.0.0.0`, and so did `rails db migrate` and
	// `rails db seed` with their own command names.
	//
	// A pure container child — description: only, like `rails db` — still inherits both the
	// command and its arguments, which is what lets a group share one argument list. Only
	// redeclaring what runs starts the arguments clean. TASK-101.
	//
	// script:, script_file: and steps: are redeclarations too, and TASK-149 added them here.
	// The blocks just above assign them onto merged, so such a child does not run the parent's
	// command — but it kept the parent's arguments, and `--explain --json` reports arguments
	// through commandArgs unconditionally. A `script:` child of `rails` therefore published
	// "arguments": ["-e","development"] for an execution that consumes no arguments at all.
	// Runtime is unaffected on the local and compose paths, where Execute returns at the
	// steps/script branches before commandArgs is reached; this closes the reporting gap and
	// keeps one rule instead of a command-shaped exception. See TASK-174 for the sibling
	// defect this does *not* fix: merged.Command still carries the parent's command, so the
	// same plan names a command the child will not run.
	switch {
	case child.DefaultArgs != "":
		merged.DefaultArgs = child.DefaultArgs
	case child.Command != "" || len(child.CommandLines) > 0,
		child.Script != "" || child.ScriptFile != "" || len(child.Steps) > 0:
		merged.DefaultArgs = ""
	}
	if child.Shell != nil {
		merged.Shell = child.Shell
	}
	if child.Entrypoint != "" {
		merged.Entrypoint = child.Entrypoint
	}
	if child.Runner != "" {
		merged.Runner = child.Runner
	}
	if child.Pod != "" {
		merged.Pod = child.Pod
	}
	if child.Compose != nil {
		merged.Compose = child.Compose
	}

	// Merge environment
	for k, v := range child.Environment {
		if merged.Environment == nil {
			merged.Environment = make(map[string]string)
		}
		merged.Environment[k] = v
	}

	return merged
}

func copyMap(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	result := make(map[string]string, len(m))
	maps.Copy(result, m)
	return result
}
