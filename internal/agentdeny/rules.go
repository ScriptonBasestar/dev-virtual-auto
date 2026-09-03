// Package agentdeny is the canonical single source for the agent-runtime deny-rule
// policy — the commands an AI coding agent must never be allowed to invoke because they
// can expose decrypted secrets (TASK-281 §3-6, TASK-286).
//
// DVA cannot reliably learn that its own caller is an LLM (TASK-281 §3-6): every signal
// available at the CLI layer is advisory and caller-supplied, and none of it is treated
// as a security boundary. The one layer that does know its caller's nature is the agent
// runtime itself, and most runtimes already refuse to invoke a tool call before it ever
// reaches a human approval step when a project or user settings file says to. This
// package holds the list DVA deploys into that layer — it is policy the runtime enforces
// on trust, not a boundary DVA itself enforces. See docs/agent-deny-rules.md for the
// full coverage table and honest-limits statement.
package agentdeny

// GatedCommand is one DVA CLI invocation an agent runtime must refuse to run directly.
type GatedCommand struct {
	// ID is a stable identifier used in receipts and generated output; it never changes
	// once shipped, even if Argv's wording does.
	ID string
	// Argv is the canonical, space-separated invocation as a human would type it.
	Argv string
	// Reason is the one-line justification shown in generated documentation.
	Reason string
	// Source names the frozen decision this entry backstops.
	Source string
}

// GatedCommands is the canonical, single-source list. Adding a command here is the only
// way it is ever projected into a runtime's deny rules — nothing else reads the gate
// state at generation time. tools/agentdenygen renders it into docs/agent-deny-rules.md,
// and 'make check-generate' fails if that render has drifted from this list.
//
// This is the complete set of TASK-281 §3-7 gated commands as of TASK-286. If a future
// card adds another command behind an env_bridge-shaped gate, add it here in the same
// change — see docs/agent-deny-rules.md "Binding this list to the CLI" for why this
// cannot yet be a compile-time or test-time check against the registered cobra command
// tree.
var GatedCommands = []GatedCommand{
	{
		ID:     "config-env-seal",
		Argv:   "dva config env seal",
		Reason: "Reads a plaintext env target and writes an encrypted sops source; disabled by default behind env_bridge.allow_seal.",
		Source: "TASK-281 §3-3, §3-7 (seal_not_enabled)",
	},
	{
		ID:     "config-env-show",
		Argv:   "dva config env show",
		Reason: "Prints decrypted secret values to the controlling terminal; disabled by default behind env_bridge.allow_show. This is the exact command TASK-281 §3-6 hands to this package: its advisory agent-environment detection stops a compliant agent invoking dva directly, but a pty-hijacking or similarly compliant-but-uninformed runtime can defeat it — the deny rule is the actual mitigation for that residual gap.",
		Source: "TASK-281 §3-4, §3-6, §3-7 (show_not_enabled, agent_environment_detected)",
	},
}

// Patterns returns the deny-pattern strings for g, in the literal form written into a
// runtime's settings file. For Claude Code (this package's only implemented target,
// see docs/agent-deny-rules.md's coverage table) an entry is parsed as a tool-scoped
// rule: a bare string with no parentheses names a TOOL, not a command, so the argv must
// be wrapped as "Bash(<argv> *)" to name the Bash tool and give it a command pattern —
// an earlier version of this function emitted the bare, unwrapped argv, which matched
// no real tool and denied nothing. The space before the trailing "*" is also
// significant, not cosmetic: Claude Code documents it as part of the rule, so
// "Bash(dva config env show *)" matches the bare command AND any trailing
// arguments/flags, while a same form with no space (dropped here) would also match an
// unrelated command sharing the same character prefix (e.g. a future "show-diff"
// subcommand) — the same over-matching bug shape TASK-286's own pattern design was
// supposed to avoid for the "unseal"/"seal" neighbor case. Only one pattern per gated
// command is emitted: earlier per-token doubled-space variants are dropped, because
// Claude Code parses and normalizes the command text rather than doing a raw string
// compare, so they added noise to a file the user owns without covering anything the
// wrapped-and-spaced form doesn't already cover.
//
// This does NOT cover a path-qualified invocation (e.g. "./bin/dva config env show" —
// this repo's own `make build` produces bin/dva), environment-runner wrappers Claude
// Code's own stripped-wrapper list excludes (mise exec, devbox run, direnv exec, docker
// exec), shell quoting/splicing tricks, or a project-authored dva.yml interaction that
// internally shells out to a gated command under a different name (e.g. `dva run
// my-interaction`). Those fall in the same adversarial-obfuscation bucket as the pty
// hijack TASK-281 §3-6 already recorded and handed to this layer generally — a text
// prefix pattern cannot close them, and TASK-281's "compliant agent, not adversarial"
// threat model does not ask it to. See docs/agent-deny-rules.md "Honest limits".
func (g GatedCommand) Patterns() []string {
	return []string{"Bash(" + g.Argv + " *)"}
}

// ByID returns the gated command with the given ID, or false if none matches.
func ByID(id string) (GatedCommand, bool) {
	for _, command := range GatedCommands {
		if command.ID == id {
			return command, true
		}
	}
	return GatedCommand{}, false
}
