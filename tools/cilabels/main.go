// Command cilabels: keep Makefile (CI) help labels in lockstep with ci.yml make targets.
//
// TASK-154: a hand-kept suffix rot into a one-way signal (only fmt-check was labelled while
// CI ran five targets). This program extracts both sets and fails when they disagree, so a
// target added to CI without (CI) — or a label on a target CI does not run — breaks the gate.
package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

func main() {
	root := "."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	ciPath := filepath.Join(root, ".github", "workflows", "ci.yml")
	mfPath := filepath.Join(root, "Makefile")

	ci, err := makeTargetsFromCI(ciPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cilabels: ci.yml: %v\n", err)
		os.Exit(2)
	}
	mf, err := ciLabelsFromMakefile(mfPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cilabels: Makefile: %v\n", err)
		os.Exit(2)
	}

	fmt.Printf("ci.yml make targets (%d): %s\n", len(ci), strings.Join(sortedKeys(ci), ", "))
	fmt.Printf("Makefile (CI) labels (%d): %s\n", len(mf), strings.Join(sortedKeys(mf), ", "))

	var missingLabel, extraLabel []string
	for t := range ci {
		if !mf[t] {
			missingLabel = append(missingLabel, t)
		}
	}
	for t := range mf {
		if !ci[t] {
			extraLabel = append(extraLabel, t)
		}
	}
	sort.Strings(missingLabel)
	sort.Strings(extraLabel)

	if len(missingLabel) == 0 && len(extraLabel) == 0 {
		fmt.Println("cilabels: OK — (CI) labels match ci.yml make targets")
		return
	}
	if len(missingLabel) > 0 {
		fmt.Fprintf(os.Stderr, "ERROR: ci.yml runs make targets without Makefile (CI) label: %s\n", strings.Join(missingLabel, ", "))
	}
	if len(extraLabel) > 0 {
		fmt.Fprintf(os.Stderr, "ERROR: Makefile (CI) labels not run by ci.yml: %s\n", strings.Join(extraLabel, ", "))
	}
	os.Exit(1)
}

var reMakeRun = regexp.MustCompile(`^\s*run:\s*make\s+(\S+)`)
var reCIHelp = regexp.MustCompile(`^##\s+([^:]+):.*\(CI\)\s*$`)

func makeTargetsFromCI(path string) (map[string]bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	out := map[string]bool{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if m := reMakeRun.FindStringSubmatch(sc.Text()); m != nil {
			out[m[1]] = true
		}
	}
	return out, sc.Err()
}

func ciLabelsFromMakefile(path string) (map[string]bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	out := map[string]bool{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if m := reCIHelp.FindStringSubmatch(sc.Text()); m != nil {
			out[strings.TrimSpace(m[1])] = true
		}
	}
	return out, sc.Err()
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
