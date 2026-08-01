// Command doccheck enforces the repository documentation gate (TASK-090 option B):
//
//   - links: every non-symlink inventory .md is scanned; relative targets and
//     heading anchors resolve against the git inventory (tracked + non-ignored
//     untracked); git symlink aliases (mode 120000) are skipped once
//   - size: every .md under docs/ and workflows/ is ≤500 lines and ≤10240 bytes,
//     except workflows/dva-dogfood/METHODOLOGY.md (size-only exemption)
//
// Exit 1 on vacuous runs (zero candidates or zero links), broken links, or
// oversized size-enforced docs. Stdlib only — no third-party packages.
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
	for _, d := range res.OversizedDetail {
		fmt.Printf("  OVERSIZE %s\n", d)
	}
	for _, d := range res.BrokenDetail {
		fmt.Printf("  BROKEN   %s\n", d)
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
