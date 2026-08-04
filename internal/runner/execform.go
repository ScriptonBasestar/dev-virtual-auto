package runner

import "fmt"

// execForm names the one way a resolved interaction declares the work it will run.
//
// It exists because that decision used to be written out three times — once per runner, as a
// chain of ifs ending in an unguarded fall-through to the single-command form — and the three
// copies disagreed. Every disagreement presented identically: a declared form was silently
// replaced by whatever the final if produced, while `dva validate` exited 0 and `--explain`
// described the substitute. TASK-094 (`steps:` on kubectl), TASK-175 (`script:`/`script_file:`
// on kubectl) and TASK-178 (a list `command:` on kubectl and compose) are three instances of
// that one shape, found one at a time over three separate tasks.
//
// One classifier does not buy compile-time exhaustiveness: Go will not reject a switch missing
// a case, so this cannot make "a runner forgot a form" a build error. What it does buy is that
// a forgotten form reaches an explicit default and returns unhandledFormError instead of
// running something else — the failure moves from silent-and-wrong to loud-and-stopped — and
// that each runner's coverage is now readable as one switch rather than inferred from the shape
// of an if-chain and the order of its conditions.
type execForm int

const (
	formSteps execForm = iota
	formScriptFile
	formScript
	formCommandList
	formCommand
)

// String names the form the way the config declares it, so a message about an unhandled form
// points at the field its author wrote rather than at an internal constant.
func (f execForm) String() string {
	switch f {
	case formSteps:
		return "steps:"
	case formScriptFile:
		return "script_file:"
	case formScript:
		return "script:"
	case formCommandList:
		return "command: (list)"
	case formCommand:
		return "command:"
	}
	return fmt.Sprintf("execForm(%d)", int(f))
}

// classifyForm reports which execution form a resolved command runs.
//
// The order is LocalRunner's precedence — steps > script_file > script > command list > command
// — which was the only one of the three copies that covered every form, and is now the only
// copy at all. The forms are mutually exclusive here but not in the schema: a config may
// declare several, and this is what picks the winner.
//
// formCommand is terminal. A command declaring no form at all classifies as an empty
// `command:` and reaches the exec it has always reached, rather than a sixth "nothing declared"
// form — that would be a behaviour change wearing a refactor's clothes, and belongs to whoever
// decides what an empty interaction should do, not here.
func classifyForm(cmd *ResolvedCommand) execForm {
	switch {
	case len(cmd.Steps) > 0:
		return formSteps
	case cmd.ScriptFile != "":
		return formScriptFile
	case cmd.Script != "":
		return formScript
	case len(cmd.CommandLines) > 0:
		return formCommandList
	default:
		return formCommand
	}
}

// unhandledFormError is what a runner returns for a form it has no case for.
//
// It names dva rather than the config on purpose. Every form here is one the schema accepts and
// `validate` passes, so an author who reaches this wrote nothing wrong; the alternative wording
// would send them editing a dva.yml that is already correct. This is the message the three
// tasks above would have produced instead of a wrong command, had it existed.
func unhandledFormError(runner string, f execForm) error {
	return fmt.Errorf("%s runner: no handling for %s — this is a dva bug, not a config error", runner, f)
}
