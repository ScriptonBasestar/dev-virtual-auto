package cli

import (
	"fmt"
	"slices"
	"strings"
)

// This file holds what outlived stack.go and app.go. Both files were deleted with the
// `dva stack` / `dva app` command families, but three of their helpers were never about
// those commands — `dva up` calls all three — and the rest went with the commands.
//
// Kept here rather than moved into compose.go so the split stays visible: these describe
// how DVA reads a leftover argument, which is the same question on every command that sets
// DisableFlagParsing, not a property of the compose passthrough.

// stackSelectorFlags are the shared flags every entry-selecting command honours, listed in
// error messages as "accepted here".
//
// appSelectorFlags used to sit beside this — the narrower subset `dva app up/restart/build`
// actually honoured, since those took only parseDvaFlags' mode and discarded env and the tag
// filters. It went with the commands; the discarding is what TASK-113 recorded, and there is
// nothing left doing it.
var stackSelectorFlags = []string{
	"--mode", "-M", "--env", "-E", "--tag", "--tags", "-T",
	"--exclude-tag", "--exclude-tags", "--dry-run", "--debug", "--json",
}

// withSelectors returns a command's own flags followed by the shared ones it honours.
func withSelectors(own []string, selectors []string) []string {
	return append(append([]string{}, own...), selectors...)
}

// rejectUnknownFlags fails on a leftover argument that still looks like a flag.
//
// up/stop/down read whatever parseDvaFlags leaves behind as NAMEs, and DisableFlagParsing
// means cobra never vets it. A mistyped flag therefore became a name, matched nothing, and
// the command exited 0 having silently dropped it: measured on the since-removed stack
// family, `dva stack up infra --nowait` started infra with `--wait` still on, and `dva stack
// up --nowait` started nothing at all and reported success (TASK-087). No entry can be named
// "--nowait", so a leading dash surviving to this point is a user error, not a name.
//
// Deliberately not applied where arguments are forwarded verbatim to an external tool —
// `dva logs <plan> <entry> --tail=5 --since=1h` reaches docker compose unchanged. There an
// unrecognised flag is docker's to interpret, and rejecting it would delete a working feature.
//
// path is the command as the user types it after `dva` ("up", "down"), and noun names what a
// bare word here would have meant ("a stack entry name") so the message can explain why the
// token was not read as one. Pass an empty noun for a command that takes no positional names
// at all — `dva up` — where that sentence would be a lie.
//
// known is the full list to advertise, supplied by the caller rather than assembled here, so
// a command cannot advertise a flag it consumes and then ignores.
//
// NOTE: known is used only to build the message. The rejection itself fires on ANY
// dash-prefixed argument, so callers must pass what is LEFT after the flags they recognise
// have been consumed, never the raw args.
func rejectUnknownFlags(path, noun string, args, known []string) error {
	for _, a := range args {
		if len(a) < 2 || !strings.HasPrefix(a, "-") {
			continue
		}
		var msg strings.Builder
		fmt.Fprintf(&msg, "unknown flag %q for \"dva %s\"", a, path)
		if noun != "" {
			fmt.Fprintf(&msg, "\n       → %s cannot start with \"-\", so this was read as one and matched nothing", noun)
		}
		msg.WriteString("\n       → accepted here: ")
		msg.WriteString(strings.Join(known, ", "))
		if s := similarTo(a, known); len(s) > 0 {
			msg.WriteString("\n\nDid you mean?")
			for _, k := range s {
				fmt.Fprintf(&msg, "\n  dva %s %s", path, k)
			}
		}
		return fmt.Errorf("%s", msg.String())
	}
	return nil
}

// dropFlagTerminator removes the first `--` from a list of positional names.
//
// parseDvaFlags deliberately KEEPS the terminator in its output, and that is right for its
// other callers: `dva up` takes no positional names, so the surviving `--` is what makes
// rejectUnknownFlags refuse a stray one. A command that does take names has to consume it
// instead, because there the token is a separator and not an argument. Only the first is
// dropped — a second `--` is an ordinary word, and `dva restart -- -- s1` should say so.
//
// Restart-local on purpose. Dropping it inside parseDvaFlags would newly ACCEPT a stray
// terminator on every other caller, which is the regression parseDvaFlags' own closing
// comment warns about. TASK-207.
func dropFlagTerminator(names []string) []string {
	i := slices.Index(names, "--")
	if i < 0 {
		return names
	}
	out := make([]string, 0, len(names)-1)
	out = append(out, names[:i]...)
	return append(out, names[i+1:]...)
}

// rejectUnknownEntryNames fails on a positional argument naming no declared stack entry.
//
// The name-shaped twin of rejectUnknownFlags, and needed because that one fires only on a
// leading dash. filterByNames (internal/lifecycle) keeps the entries whose name was asked for
// and silently drops the rest, so a typo narrowed the selection to nothing and the empty
// selection was reported as success — a warning and exit 0. `dva up` rejects an unknown
// positional outright and down/stop take none at all, which left restart as the only lifecycle
// verb still exposed to it (TASK-207).
//
// declared is every entry the config declares, NOT the post-filter set. A name that exists but
// is excluded by --tag selects nothing legitimately and stays a warning; only a name that could
// never match anything is an error here. TASK-198's open question owns the tag arm.
//
// A dash-prefixed token can still arrive: rejectUnknownFlags requires len >= 2, so a bare "-"
// slips it, and after the `--` terminator every token is a name whatever it spells. Both get a
// line saying so, because "unknown entry" alone reads as a config problem to someone who
// believed they were typing a flag.
func rejectUnknownEntryNames(path, noun string, names, declared []string) error {
	known := make(map[string]struct{}, len(declared))
	for _, d := range declared {
		known[d] = struct{}{}
	}
	for _, n := range names {
		if _, ok := known[n]; ok {
			continue
		}
		var msg strings.Builder
		fmt.Fprintf(&msg, "unknown %s %q for \"dva %s\"", noun, n, path)
		if strings.HasPrefix(n, "-") {
			fmt.Fprintf(&msg, "\n       → read as %s, not a flag: after \"--\" every argument is a name, and a bare \"-\" is too short to be a flag", noun)
		}
		if len(declared) == 0 {
			msg.WriteString("\n       → this config declares no stack entries")
		} else {
			msg.WriteString("\n       → declared here: ")
			msg.WriteString(strings.Join(declared, ", "))
		}
		if s := similarTo(n, declared); len(s) > 0 {
			msg.WriteString("\n\nDid you mean?")
			for _, k := range s {
				fmt.Fprintf(&msg, "\n  dva %s %s", path, k)
			}
		}
		return fmt.Errorf("%s", msg.String())
	}
	return nil
}

// similarTo returns the candidates within edit distance 2 of s, matching the threshold
// resolveProvisionProfile already uses for its "Did you mean?".
func similarTo(s string, candidates []string) []string {
	var out []string
	for _, c := range candidates {
		if levenshtein(s, c) <= 2 {
			out = append(out, c)
		}
	}
	return out
}
