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
// parseDvaFlags deliberately KEEPS the terminator in its output so that each caller can rule
// on it for itself. A command that takes names has to consume it, because there the token is a
// separator and not an argument. Only the first is dropped — a second `--` is an ordinary word,
// and `dva restart -- -- s1` should say so.
//
// This helper stays restart's alone, and that is now a statement about WHERE the drop happens
// rather than about which verbs get the identity. TASK-207 ruled `dva restart --` ≡ `dva
// restart` and called the identity restart-local, reasoning that a verb taking no positional
// names has nothing to separate, so a `--` written there is a mistake worth reporting.
// TASK-216 overturned that half, having measured what keeping it cost: `--` is what a wrapper
// writes when its own argument list may be empty, `dva down -- "$@"` with an empty "$@" is the
// ordinary use of it, and it was refused in exactly the config shape where `dva down` is most
// often run bare — 12 of 18 verb x fixture pairs disagreed with the bare form, 9 of them by
// refusing where the bare form ran the whole stack. So up/down/stop consume a leading
// terminator too, via dropLeadingTerminator on the whole-stack path (compose.go), not via this
// helper: they have no positional name list for the "first `--` anywhere" contract to scan, and
// only their args[0] is ever a separator.
//
// What has NOT changed is where the drop must not go. Moving it into parseDvaFlags would newly
// ACCEPT a stray terminator on every caller at once, including ones no card has ruled on —
// `build` is one, and TASK-217 owns it — which is the regression parseDvaFlags' own closing
// comment warns about. The three RunE bodies opt in one at a time and say so where they do;
// parseDvaFlags would opt them all in with nothing left to read to find out which.
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
// A dash-prefixed token can still arrive, by three different routes, and they get three
// different explanations because "unknown stack entry" alone reads as a config problem to
// someone who believed they were typing a flag: a bare "-" is below rejectUnknownFlags'
// len >= 2 floor, a SECOND "--" is an ordinary word once the first has been consumed, and
// anything else after the terminator is a name whatever it spells.
//
// The second "--" earns its own line because the generic one tells the caller to "move it
// before the --", which is meaningless advice for a token that IS "--". A review caught the
// message giving it; nothing in the flow made it wrong, only the wording.
//
// noun is bare ("stack entry"), not the article-prefixed form rejectUnknownFlags takes — that
// one only ever drops its noun into a sentence, while this one also uses it as a headline, and
// sharing the string produced "unknown a stack entry name".
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
		switch {
		case n == "-":
			fmt.Fprintf(&msg, "\n       → read as a %s name: a lone \"-\" is too short to be a flag", noun)
		case n == "--":
			fmt.Fprintf(&msg, "\n       → read as a %s name: only the first \"--\" separates flags from names", noun)
		case strings.HasPrefix(n, "-"):
			fmt.Fprintf(&msg, "\n       → read as a %s name, not a flag: after \"--\" every argument is a name. Move it before the \"--\"", noun)
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
//
// The distance must also be shorter than the shorter of the two names, or a short token
// matches everything: `dva restart -` suggested BOTH s1 and s2, since one character is two
// edits from any two-character name. Measured against the built binary, not reasoned about.
// The extra condition is unreachable for the flag caller — its inputs are dash-prefixed and
// its candidates are two characters at the shortest, so every match there is already distance
// 1 — and the ordinary name caller keeps its suggestions: `s3` still offers s1 and s2 at
// distance 1. It is not free, though; see the note at the comparison.
func similarTo(s string, candidates []string) []string {
	var out []string
	for _, c := range candidates {
		// The distance must be small relative to BOTH strings, not just the input.
		// Keying on len(s) alone suppressed "-" matching s1/s2 (d=2, len 1) as intended
		// but left the mirror open: "wob" suggested the one-character entry "b" (d=2,
		// len 3), which is the same noise seen from the other end. Neither direction
		// helps a caller. Note the consequence: with one-character entry names nothing
		// can satisfy d < 1, so those configs get no suggestions at all — the declared
		// list is printed above regardless, which is the fallback that matters.
		if d := levenshtein(s, c); d <= 2 && d < min(len(s), len(c)) {
			out = append(out, c)
		}
	}
	return out
}
