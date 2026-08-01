package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// InventoryEntry is one path from the git-visible worktree inventory.
type InventoryEntry struct {
	Path string
	Mode int // git ls-files mode (e.g. 100644, 120000); untracked defaults to regular
}

// LoadInventory lists tracked files that still exist in the worktree plus
// non-ignored untracked files. Tracked deletions are excluded so mid-move
// paths cannot mask broken links via the index. Ignored filesystem paths are
// never included (broken links cannot be masked by ignored tmp files).
func LoadInventory(root string) ([]InventoryEntry, error) {
	tracked, err := gitLSFiles(root, true)
	if err != nil {
		return nil, err
	}
	others, err := gitLSFiles(root, false)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]struct{}, len(tracked)+len(others))
	out := make([]InventoryEntry, 0, len(tracked)+len(others))
	for _, e := range tracked {
		if _, ok := seen[e.Path]; ok {
			continue
		}
		exists, err := worktreeExists(root, e.Path)
		if err != nil {
			return nil, err
		}
		if !exists {
			continue // tracked but deleted from worktree
		}
		seen[e.Path] = struct{}{}
		out = append(out, e)
	}
	for _, e := range others {
		if _, ok := seen[e.Path]; ok {
			continue
		}
		exists, err := worktreeExists(root, e.Path)
		if err != nil {
			return nil, err
		}
		if !exists {
			continue
		}
		seen[e.Path] = struct{}{}
		out = append(out, e)
	}
	return out, nil
}

// worktreeExists reports whether rel is present in the worktree (file or
// symlink). Uses Lstat so git symlink aliases remain visible for mode checks.
func worktreeExists(root, rel string) (bool, error) {
	full := filepath.Join(root, filepath.FromSlash(rel))
	_, err := os.Lstat(full)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, fmt.Errorf("lstat %s: %w", rel, err)
}

func gitLSFiles(root string, staged bool) ([]InventoryEntry, error) {
	args := []string{"-C", root, "ls-files", "-z"}
	if staged {
		args = append(args, "-c", "-s")
	} else {
		args = append(args, "-o", "--exclude-standard")
	}
	cmd := exec.Command("git", args...)
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("git ls-files: %w: %s", err, bytes.TrimSpace(ee.Stderr))
		}
		return nil, fmt.Errorf("git ls-files: %w", err)
	}
	if staged {
		return parseLSFilesStage(out)
	}
	return parseLSFilesPaths(out)
}

// parseLSFilesStage parses `git ls-files -s -z`:
//
//	<mode> <sha> <stage>\t<path>\0
func parseLSFilesStage(data []byte) ([]InventoryEntry, error) {
	if len(data) == 0 {
		return nil, nil
	}
	parts := bytes.Split(data, []byte{0})
	out := make([]InventoryEntry, 0, len(parts))
	for _, p := range parts {
		if len(p) == 0 {
			continue
		}
		metaBytes, pathBytes, ok := bytes.Cut(p, []byte{'\t'})
		if !ok {
			return nil, fmt.Errorf("git ls-files -s: missing tab in %q", truncate(string(p), 80))
		}
		meta := string(metaBytes)
		path := normalizePath(string(pathBytes))
		fields := strings.Fields(meta)
		if len(fields) < 1 {
			return nil, fmt.Errorf("git ls-files -s: bad meta %q", meta)
		}
		mode, err := strconv.ParseInt(fields[0], 8, 32)
		if err != nil {
			return nil, fmt.Errorf("git ls-files -s: bad mode %q: %w", fields[0], err)
		}
		out = append(out, InventoryEntry{Path: path, Mode: int(mode)})
	}
	return out, nil
}

// parseLSFilesPaths parses plain `git ls-files -z` path lists (untracked).
func parseLSFilesPaths(data []byte) ([]InventoryEntry, error) {
	if len(data) == 0 {
		return nil, nil
	}
	parts := bytes.Split(data, []byte{0})
	out := make([]InventoryEntry, 0, len(parts))
	for _, p := range parts {
		if len(p) == 0 {
			continue
		}
		path := normalizePath(string(p))
		out = append(out, InventoryEntry{Path: path, Mode: modeRegular})
	}
	return out, nil
}

func normalizePath(path string) string {
	return strings.ReplaceAll(path, "\\", "/")
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
