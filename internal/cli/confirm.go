package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// destructionWarning renders the sentence the confirmation prompt opens with.
//
// One builder for both callers so `clean` and `down --purge` cannot come to describe the
// same removal differently. The wording is the thing the operator consents to, and
// TestCleanWithoutDryRunStillPrompts asserts the "VOLUMES (data loss!)" phrasing verbatim —
// two copies would let one of them drift while that test still passed.
func destructionWarning(volumes, images bool) string {
	msg := "This will remove all containers, networks"
	if volumes {
		msg += ", and VOLUMES (data loss!)"
	}
	if images {
		msg += ", and locally built images"
	}
	return msg + "."
}

// confirmDestruction asks before an irreversible removal and reports whether to proceed.
//
// This lived inline in cleanCmd behind a note explaining why it was not a helper: it was
// "the only prompt in the codebase today", so factoring it out would abstract over one
// caller. `down --purge` is the second caller, and the one that outlives it — `clean` is
// being removed — so that reason has expired and the note goes with it.
//
// Moved rather than rewritten. The two decisions encoded below are settled answers that a
// reimplementation would quietly discard:
//
//   - The caller exempts dry runs. Consent is consent to the deletion, and a dry run
//     deletes nothing; without the exemption the one invocation whose purpose is to say
//     what would be destroyed refused to say it until you agreed to the destruction
//     (TASK-170).
//   - io.EOF is not a decline. Stdin exhausted before any token — a pipe, a CI runner,
//     </dev/null — means nobody was there to answer, so reporting a decline would be
//     inventing one; that returns an error. A person pressing Enter at the prompt yields
//     "unexpected newline" instead, so the documented default (N) still reaches the decline
//     branch and still exits 0 (TASK-171).
//
// invocation is what the operator would have to type to proceed non-interactively, named in
// the EOF error so it reports a way forward rather than a dead end.
func confirmDestruction(invocation string, volumes, images bool) (proceed bool, err error) {
	fmt.Fprintf(os.Stderr, "%s\nContinue? [y/N] ", destructionWarning(volumes, images))

	var answer string
	if _, scanErr := fmt.Scanln(&answer); errors.Is(scanErr, io.EOF) {
		return false, fmt.Errorf("cannot ask for confirmation: stdin reached EOF, so nothing answered the prompt and nothing was removed\n"+
			"       → pass --force to run '%s' non-interactively", invocation)
	}

	answer = strings.ToLower(strings.TrimSpace(answer))
	if answer != "y" && answer != "yes" {
		// stderr, with the prompt it answers. On stdout it was half of an interaction whose
		// other half was on another stream, so `2>/dev/null` showed "Aborted." with nothing
		// saying what had been aborted.
		fmt.Fprintln(os.Stderr, "Aborted.")
		return false, nil
	}
	return true, nil
}
