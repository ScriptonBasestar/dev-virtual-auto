package main

import (
	"strings"
	"testing"
)

func TestReplaceBlockUpdatesEveryMatchingMarker(t *testing.T) {
	content := strings.Join([]string{
		"instruction: |",
		"  <!-- AUTOGEN:rules:start -->",
		"  stale",
		"  <!-- AUTOGEN:rules:end -->",
		"  retry: |",
		"    <!-- AUTOGEN:rules:start -->",
		"    stale again",
		"    <!-- AUTOGEN:rules:end -->",
	}, "\n")

	rendered, err := replaceBlock(content, "rules", "first line\n\nsecond line")
	if err != nil {
		t.Fatalf("replaceBlock() error = %v", err)
	}
	if strings.Count(rendered, "first line") != 2 || strings.Count(rendered, "second line") != 2 {
		t.Fatalf("replaceBlock() did not update every marker:\n%s", rendered)
	}
	if strings.Contains(rendered, "  \n") {
		t.Fatalf("replaceBlock() left trailing whitespace on a blank line:\n%s", rendered)
	}
}

func TestReplaceBlockRequiresMarker(t *testing.T) {
	if _, err := replaceBlock("instruction: |\n  no marker", "rules", "body"); err == nil {
		t.Fatal("replaceBlock() error = nil, want missing marker error")
	}
}
