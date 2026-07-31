package runner

import (
	"strings"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// ResolvedCommand is the result of InteractionTree.Find().
type ResolvedCommand struct {
	Name         string
	Description  string
	Service      string
	Command      string
	CommandLines []string // non-nil when command: was a YAML list
	Script       string   // inline shell script block
	ScriptFile   string   // path to external shell script
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

// List returns all commands (including subcommands) as a flat map.
func (t *InteractionTree) List() map[string]*ResolvedCommand {
	result := make(map[string]*ResolvedCommand)
	for name, entry := range t.entries {
		t.expandInto(name, entry, result)
	}
	return result
}

// expand builds a flat map of all commands for a given entry.
func (t *InteractionTree) expand(name string, entry *config.InteractionCommand) map[string]*ResolvedCommand {
	result := make(map[string]*ResolvedCommand)
	t.expandInto(name, entry, result)
	return result
}

func (t *InteractionTree) expandInto(name string, entry *config.InteractionCommand, result map[string]*ResolvedCommand) {
	cmd := buildResolved(name, entry)
	result[name] = cmd

	for subName, subEntry := range entry.Subcommands {
		merged := mergeInteraction(entry, subEntry)
		t.expandInto(name+" "+subName, merged, result)
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
	// redeclaring the command starts the arguments clean. TASK-101.
	switch {
	case child.DefaultArgs != "":
		merged.DefaultArgs = child.DefaultArgs
	case child.Command != "" || len(child.CommandLines) > 0:
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
	for k, v := range m {
		result[k] = v
	}
	return result
}
