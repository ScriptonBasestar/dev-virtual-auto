package cli

import (
	"fmt"
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
