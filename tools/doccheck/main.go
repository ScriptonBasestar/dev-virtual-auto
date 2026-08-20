// Command doccheck enforces the repository documentation gate (TASK-090 option B):
//
//   - links: every non-symlink inventory .md is scanned; relative targets and
//     heading anchors resolve against the git inventory (tracked + non-ignored
//     untracked); git symlink aliases (mode 120000) are skipped once
//   - size: every .md under docs/ and workflows/ is ≤500 lines and ≤10240 bytes,
//     with no per-file exemption
//   - verify bindings: every `go test … -run …` written in inline code selects at
//     least one test declared in the tree, so a binding cannot name a test that
//     does not exist and still exit 0 (TASK-136)
//   - archive frontmatter: every card under tasks/_archive/ carries `id:` or
//     `type:`. Older ce tested those fields before testing whether the file was
//     archived, so a card missing both was audited against a format that
//     postdates it; fixed ce skips the archive before reading it and reports
//     nothing there. Neither build asserts this property — only this does, and
//     the upgrade widens its remit rather than retiring it (TASK-206)
//
// Exit 1 on vacuous runs (zero candidates, zero links, _test.go files that yield
// no test names, or archive files that yield no cards), broken links, oversized
// size-enforced docs, -run patterns selecting nothing, or archived cards missing
// both detection fields. Stdlib only — no third-party packages.
package main

import (
	"fmt"
	"os"
)

func main() {
	root, err := os.Getwd()
	if err != nil {
		fail(err)
	}
	if len(os.Args) > 1 {
		root = os.Args[1]
	}

	inv, err := LoadInventory(root)
	if err != nil {
		fail(err)
	}

	res := Check(CheckInput{Root: root, Inventory: inv})
	printReport(res)
	if !res.OK {
		os.Exit(1)
	}
}

func printReport(res Result) {
	fmt.Printf("markdown_candidates: %d\n", res.MarkdownCandidates)
	fmt.Printf("markdown_checked:    %d\n", res.MarkdownChecked)
	fmt.Printf("links_checked:       %d\n", res.LinksChecked)
	fmt.Printf("symlinks_skipped:    %d\n", res.SymlinksSkipped)
	fmt.Printf("broken_links:        %d\n", res.BrokenLinks)
	fmt.Printf("oversized_docs:      %d\n", res.OversizedDocs)
	fmt.Printf("test_funcs_found:    %d (from %d _test.go files)\n", res.TestFuncsFound, res.TestFilesSwept)
	fmt.Printf("run_patterns:        %d\n", res.RunPatternsChecked)
	fmt.Printf("unmatched_run:       %d\n", res.UnmatchedRunFlags)
	fmt.Printf("archive_cards:       %d (from %d file(s) under %s)\n", res.ArchiveCards, res.ArchiveFilesSeen, archivePrefix)
	fmt.Printf("archive_missing:     %d\n", res.ArchiveMissing)
	for _, d := range res.OversizedDetail {
		fmt.Printf("  OVERSIZE %s\n", d)
	}
	for _, d := range res.BrokenDetail {
		fmt.Printf("  BROKEN   %s\n", d)
	}
	for _, d := range res.UnmatchedRunDetail {
		fmt.Printf("  NO-TESTS %s\n", d)
	}
	for _, d := range res.ArchiveDetail {
		fmt.Printf("  ARCHIVE  %s\n", d)
	}
	for _, e := range res.Errors {
		fmt.Printf("  ERROR    %s\n", e)
	}
	if res.OK {
		fmt.Println("doc-check: OK")
	} else {
		fmt.Println("doc-check: FAIL")
	}
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "doccheck: %v\n", err)
	os.Exit(2)
}
