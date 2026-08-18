// Comment scanning. am does not ignore a `#` comment the way /bin/sh does, so the prose
// explaining a step is checked as carefully as the step: it can carry a substitution am
// will run, and it can change the quote state of the code beneath it.
package main

import (
	"regexp"
	"strings"
)

// reCommentSubstitution finds what am reads as code inside a comment: a backtick pair, or
// `$(`. Measured against am cb8b4ce -- a comment carrying `backend` blocked the step on
// "backend not in allowlist", and one carrying $(nosuchcmd) blocked it on "nosuchcmd",
// while the same words without the wrapper ran fine. POSIX sh executes neither.
var reCommentSubstitution = regexp.MustCompile("`[^`]*`|\\$\\([^)]*\\)?")

// commentSubstitutions returns, for every comment in text, the offset and source text of
// each substitution span. Whole-line comments and trailing ones are both reported: am
// does not care where the `#` sits, and neither does the block.
//
// A `#` inside single or double quotes is not a comment, so the scan reuses the same
// quote-skipping the bare-word tokenizer does rather than matching `#` in raw text.
func commentSubstitutions(text string) []commentSpan {
	var out []commentSpan
	for _, r := range shellCommentRanges(text) {
		for _, m := range reCommentSubstitution.FindAllStringIndex(text[r[0]:r[1]], -1) {
			out = append(out, commentSpan{offset: r[0] + m[0], text: text[r[0]+m[0] : r[0]+m[1]]})
		}
	}
	return out
}

// shellCommentRanges returns the half-open byte range of every comment in text.
//
// A `#` inside single or double quotes is not a comment, so the scan reuses the same
// quote-skipping the bare-word tokenizer does rather than matching `#` in raw text. Note
// that this is /bin/sh semantics, not am semantics: am is the one that lets a comment
// change quote state, which is what `comment-apostrophe` below is about.
func shellCommentRanges(text string) [][2]int {
	var out [][2]int
	for i := 0; i < len(text); {
		switch text[i] {
		case '\'', '"':
			i = skipQuoted(text, i+1, text[i])
		case '\\':
			i += 2
		case '#':
			end := strings.IndexByte(text[i:], '\n')
			if end < 0 {
				end = len(text)
			} else {
				end += i
			}
			out = append(out, [2]int{i, end})
			i = end
		default:
			i++
		}
	}
	return out
}

// commentQuotes returns the offset of every quote character sitting inside a comment.
//
// am carries quote state across a comment: `#` does not end a string, and a string is not
// confined to a line. An odd number of quotes in prose therefore inverts the parity of
// everything after it, and the next legitimate quoted argument opens where the analyzer
// believes one closes -- exposing its contents as shell. Measured against am cb8b4ce on
// otherwise identical steps: a comment carrying one apostrophe blocked the multi-line
// `awk '...'` below it on `command "BEGIN" not in allowlist`, a second apostrophe in the
// same comment made the step run, and the same pair of measurements holds for a double
// quote and an `awk "..."`.
//
// am tracks the two characters separately, so an apostrophe cannot close a double quote.
// The rule does not care: both are reported, because a field is correct only by parity
// and parity is decided by prose somebody else is editing.
func commentQuotes(text string) []int {
	var out []int
	for _, r := range shellCommentRanges(text) {
		for i := r[0]; i < r[1]; i++ {
			if text[i] == '\'' || text[i] == '"' {
				out = append(out, i)
			}
		}
	}
	return out
}

// commentSpan is one substitution found inside a comment.
type commentSpan struct {
	offset int
	text   string
}
