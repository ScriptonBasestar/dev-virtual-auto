// Command skilldogfood verifies an installed DVA binary's skill installer without AI runtimes.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
)

const allNativeRuntimes = "claude-code,codex,opencode,grok,antigravity"

type commandResult struct {
	Operation string              `json:"operation"`
	DryRun    bool                `json:"dry_run"`
	Scope     string              `json:"scope"`
	Results   []destinationResult `json:"results"`
}

type destinationResult struct {
	Destination     string          `json:"destination"`
	Runtimes        []string        `json:"runtimes"`
	Status          string          `json:"status"`
	RuntimeStatuses []runtimeStatus `json:"runtime_statuses"`
}

type runtimeStatus struct {
	Runtime string `json:"runtime"`
	Status  string `json:"status"`
}

// treeEntry records all stable facts that a dry-run is allowed to preserve.
// It intentionally excludes timestamps, which a read-only directory traversal may change
// on filesystems that update access metadata.
type treeEntry struct {
	Path       string
	Type       string
	Mode       fs.FileMode
	ContentSHA string
	LinkTarget string
}

type invocation struct {
	binary string
	env    []string
}

func main() {
	var binary, flowRoot string
	flags := flag.NewFlagSet("skilldogfood", flag.ExitOnError)
	flags.StringVar(&binary, "dva-bin", "", "absolute path to the installed dva binary")
	flags.StringVar(&flowRoot, "flow-root", "", "absolute path to a clean flow Git repository")
	flags.Parse(os.Args[1:])
	if err := run(binary, flowRoot, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: skill installer dogfood failed: %v\n", err)
		os.Exit(1)
	}
}

func run(binaryArg, flowArg string, out io.Writer) (err error) {
	binary, err := executableFile(binaryArg)
	if err != nil {
		return fmt.Errorf("DVA_BIN: %w", err)
	}
	flowRoot, err := cleanGitRoot(flowArg)
	if err != nil {
		return fmt.Errorf("FLOW_ROOT: %w", err)
	}

	sha, err := fileSHA256(binary)
	if err != nil {
		return fmt.Errorf("hash DVA_BIN: %w", err)
	}
	base := invocation{binary: binary, env: os.Environ()}
	version, err := base.output("version")
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "DVA binary: %s\nSHA-256: %s\ndva version:\n%s", binary, sha, version)
	if !strings.HasSuffix(version, "\n") {
		fmt.Fprintln(out)
	}

	if err := verifyFlowDryRun(base, flowRoot); err != nil {
		return err
	}
	fmt.Fprintf(out, "flow dry-run: unchanged %s\n", flowRoot)

	fixture, err := os.MkdirTemp("", "dva-skill-dogfood-")
	if err != nil {
		return fmt.Errorf("create fixture: %w", err)
	}
	defer func() {
		if cleanupErr := os.RemoveAll(fixture); cleanupErr != nil && err == nil {
			err = fmt.Errorf("clean fixture %s: %w", fixture, cleanupErr)
		}
	}()

	project := filepath.Join(fixture, "project")
	if err := os.MkdirAll(project, 0o755); err != nil {
		return fmt.Errorf("create fixture project: %w", err)
	}
	fixtureEnv := withEnvironment(os.Environ(), map[string]string{
		"HOME":           filepath.Join(fixture, "home"),
		"XDG_STATE_HOME": filepath.Join(fixture, "state"),
	})
	isolated := invocation{binary: binary, env: fixtureEnv}
	if err := verifyFixtureRoundTrip(isolated, project); err != nil {
		return err
	}
	fmt.Fprintln(out, "isolated install/status/uninstall round-trip: passed")
	return nil
}

func executableFile(path string) (string, error) {
	if path == "" {
		return "", errors.New("must be set to an absolute executable path")
	}
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("must be absolute, got %q", path)
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return "", fmt.Errorf("is not an executable regular file: %s", path)
	}
	return filepath.EvalSymlinks(path)
}

func cleanGitRoot(path string) (string, error) {
	if path == "" {
		return "", errors.New("must be set to an absolute clean Git repository path")
	}
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("must be absolute, got %q", path)
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("is not a directory: %s", resolved)
	}
	rootOutput, err := commandOutput(nil, "git", "-C", resolved, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("is not a Git repository root: %w", err)
	}
	root, err := filepath.EvalSymlinks(strings.TrimSpace(rootOutput))
	if err != nil {
		return "", err
	}
	if root != resolved {
		return "", fmt.Errorf("must name the repository root, not %s", resolved)
	}
	status, err := gitStatus(root)
	if err != nil {
		return "", err
	}
	if status != "" {
		return "", fmt.Errorf("repository is not clean:\n%s", status)
	}
	return root, nil
}

func verifyFlowDryRun(inv invocation, flowRoot string) (err error) {
	before, err := gitStatus(flowRoot)
	if err != nil {
		return err
	}
	beforeSkills, err := snapshotRuntimePaths(flowRoot)
	if err != nil {
		return fmt.Errorf("snapshot runtime paths before dry-run: %w", err)
	}
	stateDir, err := os.MkdirTemp("", "dva-skill-dogfood-state-")
	if err != nil {
		return fmt.Errorf("create dry-run state directory: %w", err)
	}
	defer func() {
		if cleanupErr := os.RemoveAll(stateDir); cleanupErr != nil && err == nil {
			err = fmt.Errorf("clean dry-run state directory %s: %w", stateDir, cleanupErr)
		}
	}()
	dryRun := invocation{binary: inv.binary, env: withEnvironment(inv.env, map[string]string{"XDG_STATE_HOME": stateDir})}
	result, err := dryRun.json(flowRoot, "skill", "install", "--scope", "project", "--runtime", allNativeRuntimes, "--dry-run")
	if err != nil {
		return fmt.Errorf("run project-scope dry-run: %w", err)
	}
	if result.Operation != "install" || !result.DryRun || result.Scope != "project" {
		return fmt.Errorf("unexpected dry-run response: operation=%q dry_run=%t scope=%q", result.Operation, result.DryRun, result.Scope)
	}
	if err := requireDestinations(result, "would-install", map[string]string{}); err != nil {
		return fmt.Errorf("project-scope dry-run result: %w", err)
	}
	after, err := gitStatus(flowRoot)
	if err != nil {
		return err
	}
	if before != after {
		return fmt.Errorf("project-scope dry-run changed Git status:\nbefore:\n%safter:\n%s", before, after)
	}
	afterSkills, err := snapshotRuntimePaths(flowRoot)
	if err != nil {
		return fmt.Errorf("snapshot runtime paths after dry-run: %w", err)
	}
	if !reflect.DeepEqual(beforeSkills, afterSkills) {
		return fmt.Errorf("project-scope dry-run changed runtime paths:\nbefore:\n%safter:\n%s", formatSnapshot(beforeSkills), formatSnapshot(afterSkills))
	}
	if err := requireEmptyDirectory(stateDir); err != nil {
		return fmt.Errorf("project-scope dry-run wrote XDG_STATE_HOME: %w", err)
	}
	return nil
}

func verifyFixtureRoundTrip(inv invocation, project string) error {
	installed, err := inv.json(project, "skill", "install", "--scope", "project", "--runtime", allNativeRuntimes)
	if err != nil {
		return fmt.Errorf("install isolated fixture: %w", err)
	}
	if err := requireDestinations(installed, "installed", map[string]string{}); err != nil {
		return fmt.Errorf("install result: %w", err)
	}

	status, err := inv.json(project, "skill", "status", "--scope", "project", "--runtime", allNativeRuntimes)
	if err != nil {
		return fmt.Errorf("check installed fixture status: %w", err)
	}
	if err := requireDestinations(status, "installed", map[string]string{}); err != nil {
		return fmt.Errorf("installed status: %w", err)
	}

	unlinked, err := inv.json(project, "skill", "uninstall", "--scope", "project", "--runtime", "codex")
	if err != nil {
		return fmt.Errorf("uninstall Codex from shared destination: %w", err)
	}
	if err := requireOnlyDestination(unlinked, ".agents/skills", "unlinked"); err != nil {
		return fmt.Errorf("Codex-only uninstall result: %w", err)
	}

	partial, err := inv.json(project, "skill", "status", "--scope", "project", "--runtime", "codex,antigravity")
	if err != nil {
		return fmt.Errorf("check shared destination after Codex uninstall: %w", err)
	}
	if err := requireOnlyDestination(partial, ".agents/skills", "partial"); err != nil {
		return fmt.Errorf("shared destination status: %w", err)
	}
	if err := requireRuntimeStatuses(partial.Results[0], map[string]string{"codex": "absent", "antigravity": "installed"}); err != nil {
		return fmt.Errorf("shared destination membership: %w", err)
	}

	removed, err := inv.json(project, "skill", "uninstall", "--scope", "project", "--runtime", allNativeRuntimes)
	if err != nil {
		return fmt.Errorf("uninstall remaining fixture skills: %w", err)
	}
	if err := requireDestinationStatusSet(removed, map[string]string{
		".agents/skills": "uninstalled", ".claude/skills": "uninstalled", ".grok/skills": "uninstalled", ".opencode/skills": "uninstalled",
	}); err != nil {
		return fmt.Errorf("remaining uninstall result: %w", err)
	}

	absent, err := inv.json(project, "skill", "status", "--scope", "project", "--runtime", allNativeRuntimes)
	if err != nil {
		return fmt.Errorf("check removed fixture status: %w", err)
	}
	if err := requireDestinations(absent, "absent", map[string]string{}); err != nil {
		return fmt.Errorf("final status: %w", err)
	}
	return nil
}

func (inv invocation) output(args ...string) (string, error) {
	return commandOutput(inv.env, inv.binary, args...)
}

func (inv invocation) json(directory string, args ...string) (commandResult, error) {
	args = append([]string{"--json"}, args...)
	output, err := commandOutputInDir(inv.env, directory, inv.binary, args...)
	if err != nil {
		return commandResult{}, err
	}
	var result commandResult
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		return commandResult{}, fmt.Errorf("decode JSON response %q: %w", output, err)
	}
	return result, nil
}

func commandOutput(env []string, command string, args ...string) (string, error) {
	return commandOutputInDir(env, "", command, args...)
}

func commandOutputInDir(env []string, directory, command string, args ...string) (string, error) {
	cmd := exec.Command(command, args...)
	cmd.Dir = directory
	cmd.Env = env
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s: %w\n%s", strings.Join(append([]string{command}, args...), " "), err, output)
	}
	return string(output), nil
}

func gitStatus(root string) (string, error) {
	return commandOutput(nil, "git", "-C", root, "status", "--porcelain=v1", "--untracked-files=all")
}

func snapshotRuntimePaths(root string) ([]treeEntry, error) {
	var snapshot []treeEntry
	for _, relative := range []string{".agents/skills", ".claude/skills", ".grok/skills", ".opencode/skills"} {
		path := filepath.Join(root, relative)
		info, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			snapshot = append(snapshot, treeEntry{Path: filepath.ToSlash(relative), Type: "missing"})
			continue
		}
		if err != nil {
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err != nil {
				return nil, err
			}
			snapshot = append(snapshot, treeEntry{Path: filepath.ToSlash(relative), Type: "symlink", Mode: info.Mode(), LinkTarget: target})
			continue
		}
		err = filepath.WalkDir(path, func(current string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			relativePath, err := filepath.Rel(root, current)
			if err != nil {
				return err
			}
			record := treeEntry{Path: filepath.ToSlash(relativePath), Mode: info.Mode()}
			switch {
			case info.Mode()&os.ModeSymlink != 0:
				target, err := os.Readlink(current)
				if err != nil {
					return err
				}
				record.Type, record.LinkTarget = "symlink", target
			case info.IsDir():
				record.Type = "directory"
			case info.Mode().IsRegular():
				digest, err := fileSHA256(current)
				if err != nil {
					return err
				}
				record.Type, record.ContentSHA = "file", digest
			default:
				record.Type = "other"
			}
			snapshot = append(snapshot, record)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Slice(snapshot, func(i, j int) bool { return snapshot[i].Path < snapshot[j].Path })
	return snapshot, nil
}

func formatSnapshot(snapshot []treeEntry) string {
	var lines []string
	for _, entry := range snapshot {
		line := fmt.Sprintf("%s type=%s mode=%#o", entry.Path, entry.Type, entry.Mode)
		if entry.ContentSHA != "" {
			line += " sha256=" + entry.ContentSHA
		}
		if entry.LinkTarget != "" {
			line += " target=" + entry.LinkTarget
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n") + "\n"
}

func requireEmptyDirectory(path string) error {
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	if len(entries) != 0 {
		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			names = append(names, entry.Name())
		}
		sort.Strings(names)
		return fmt.Errorf("contains %s", strings.Join(names, ", "))
	}
	return nil
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func withEnvironment(base []string, replacements map[string]string) []string {
	keys := make([]string, 0, len(replacements))
	for key := range replacements {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]string, 0, len(base)+len(keys))
	for _, entry := range base {
		key, _, found := strings.Cut(entry, "=")
		if !found {
			result = append(result, entry)
			continue
		}
		if _, replaced := replacements[key]; !replaced {
			result = append(result, entry)
		}
	}
	for _, key := range keys {
		result = append(result, key+"="+replacements[key])
	}
	return result
}

func requireDestinations(result commandResult, status string, runtimeStatuses map[string]string) error {
	expected := map[string]string{
		".agents/skills": "", ".claude/skills": "", ".grok/skills": "", ".opencode/skills": "",
	}
	for suffix := range expected {
		expected[suffix] = status
	}
	if err := requireDestinationStatusSet(result, expected); err != nil {
		return err
	}
	for _, entry := range result.Results {
		if len(runtimeStatuses) > 0 {
			if err := requireRuntimeStatuses(entry, runtimeStatuses); err != nil {
				return err
			}
		}
	}
	return nil
}

func requireDestinationStatusSet(result commandResult, expected map[string]string) error {
	if len(result.Results) != len(expected) {
		return fmt.Errorf("got %d destinations, want %d", len(result.Results), len(expected))
	}
	seen := make(map[string]bool, len(expected))
	for _, entry := range result.Results {
		suffix, ok := destinationSuffix(entry.Destination)
		if !ok {
			return fmt.Errorf("unexpected destination %q", entry.Destination)
		}
		want, ok := expected[suffix]
		if !ok {
			return fmt.Errorf("unexpected destination suffix %q", suffix)
		}
		if seen[suffix] {
			return fmt.Errorf("duplicate destination suffix %q", suffix)
		}
		seen[suffix] = true
		if entry.Status != want {
			return fmt.Errorf("%s has status %q, want %q", suffix, entry.Status, want)
		}
	}
	return nil
}

func requireOnlyDestination(result commandResult, suffix, status string) error {
	if len(result.Results) != 1 {
		return fmt.Errorf("got %d destinations, want 1", len(result.Results))
	}
	entry := result.Results[0]
	actual, ok := destinationSuffix(entry.Destination)
	if !ok || actual != suffix {
		return fmt.Errorf("got destination %q, want suffix %q", entry.Destination, suffix)
	}
	if entry.Status != status {
		return fmt.Errorf("%s has status %q, want %q", suffix, entry.Status, status)
	}
	return nil
}

func destinationSuffix(path string) (string, bool) {
	for _, suffix := range []string{".agents/skills", ".claude/skills", ".grok/skills", ".opencode/skills"} {
		if strings.HasSuffix(filepath.ToSlash(path), suffix) {
			return suffix, true
		}
	}
	return "", false
}

func requireRuntimeStatuses(entry destinationResult, expected map[string]string) error {
	actual := make(map[string]string, len(entry.RuntimeStatuses))
	for _, status := range entry.RuntimeStatuses {
		actual[status.Runtime] = status.Status
	}
	for runtime, want := range expected {
		if actual[runtime] != want {
			return fmt.Errorf("%s runtime %q has status %q, want %q", entry.Destination, runtime, actual[runtime], want)
		}
	}
	return nil
}
