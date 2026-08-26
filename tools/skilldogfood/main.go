// Command skilldogfood verifies a selected SHA-pinned DVA executable's skill
// installer without AI runtimes.
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
	"time"
)

const allNativeRuntimes = "claude-code,codex,opencode,grok,antigravity"
const commandStderrLimit = 8 * 1024

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
type receiptRecord struct {
	Schema      int        `json:"schema"`
	Scope       string     `json:"scope"`
	Destination string     `json:"destination"`
	Runtimes    []string   `json:"runtimes"`
	Version     string     `json:"version"`
	BundleSHA   string     `json:"bundle_sha256"`
	Files       []fileHash `json:"files"`
}
type fileHash struct {
	Path string `json:"path"`
	SHA  string `json:"sha256"`
}

// treeEntry records the runtime-path facts that a dry-run must preserve.
type treeEntry struct {
	Path       string
	Type       string
	Mode       fs.FileMode
	ContentSHA string
	LinkTarget string
	ModTime    time.Time
}

// gitTreeState captures content that porcelain status alone cannot distinguish.
// Runtime destinations are snapshotted separately because they are commonly ignored.
type gitTreeState struct {
	Status           string
	WorktreeDiffSHA  string
	IndexDiffSHA     string
	UntrackedEntries []treeEntry
}

type limitedBuffer struct {
	contents []byte
	limit    int
}

func (buffer *limitedBuffer) Write(value []byte) (int, error) {
	remaining := buffer.limit - len(buffer.contents)
	if remaining > 0 {
		if len(value) < remaining {
			remaining = len(value)
		}
		buffer.contents = append(buffer.contents, value[:remaining]...)
	}
	return len(value), nil
}

func (buffer *limitedBuffer) String() string { return string(buffer.contents) }

type invocation struct {
	binary string
	env    []string
}

func main() {
	var binary, expectedSHA, flowRoot string
	flags := flag.NewFlagSet("skilldogfood", flag.ExitOnError)
	flags.StringVar(&binary, "dva-bin", "", "absolute path to the selected dva executable")
	flags.StringVar(&expectedSHA, "expected-sha256", os.Getenv("DVA_SHA256"), "independently recorded SHA-256 of the selected dva executable")
	flags.StringVar(&flowRoot, "flow-root", "", "absolute path to a flow Git repository root whose state will remain stable")
	if err := flags.Parse(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if err := run(binary, expectedSHA, flowRoot, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: skill installer dogfood failed: %v\n", err)
		os.Exit(1)
	}
}

func run(binaryArg, expectedSHA, flowArg string, out io.Writer) (err error) {
	binary, err := executableFile(binaryArg)
	if err != nil {
		return fmt.Errorf("DVA_BIN: %w", err)
	}
	flowRoot, err := gitRoot(flowArg)
	if err != nil {
		return fmt.Errorf("FLOW_ROOT: %w", err)
	}

	if err := validSHA256(expectedSHA); err != nil {
		return fmt.Errorf("DVA_SHA256: %w", err)
	}
	executed, sha, cleanup, err := immutableExecutableCopy(binary, expectedSHA)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, cleanup()) }()
	base := invocation{binary: executed, env: os.Environ()}
	version, err := base.output("version")
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "DVA binary (original): %s\nDVA SHA-256 (executed immutable copy): %s\ndva version:\n%s", binary, sha, version); err != nil {
		return err
	}
	if !strings.HasSuffix(version, "\n") {
		if _, err := fmt.Fprintln(out); err != nil {
			return err
		}
	}

	if err := verifyFlowDryRun(base, flowRoot); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "flow dry-run: unchanged %s\n", flowRoot); err != nil {
		return err
	}

	fixture, err := os.MkdirTemp("", "dva-skill-dogfood-")
	if err != nil {
		return fmt.Errorf("create fixture: %w", err)
	}
	defer func() {
		err = errors.Join(err, removeAll("clean fixture "+fixture, fixture))
	}()
	fixture, err = filepath.EvalSymlinks(fixture)
	if err != nil {
		return fmt.Errorf("resolve fixture path: %w", err)
	}

	project := filepath.Join(fixture, "project")
	if err := os.MkdirAll(project, 0o755); err != nil {
		return fmt.Errorf("create fixture project: %w", err)
	}
	fixtureEnv := withEnvironment(os.Environ(), map[string]string{
		"HOME":            filepath.Join(fixture, "home"),
		"XDG_STATE_HOME":  filepath.Join(fixture, "state"),
		"XDG_CONFIG_HOME": filepath.Join(fixture, "config"),
		"XDG_DATA_HOME":   filepath.Join(fixture, "data"),
		"XDG_CACHE_HOME":  filepath.Join(fixture, "cache"),
	})
	isolated := invocation{binary: executed, env: fixtureEnv}
	if err := verifyFixtureRoundTrip(isolated, project, filepath.Join(fixture, "state", "dva")); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(out, "real_target_dry_run: passed"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(out, "fixture_round_trip: passed"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(out, "shared_runtime_unlink: passed"); err != nil {
		return err
	}
	return nil
}

func validSHA256(value string) error {
	if len(value) != sha256.Size*2 {
		return fmt.Errorf("must be a %d-character SHA-256 hex string", sha256.Size*2)
	}
	if _, err := hex.DecodeString(value); err != nil {
		return fmt.Errorf("must be hexadecimal: %w", err)
	}
	return nil
}

func immutableExecutableCopy(source, expected string) (path, digest string, cleanup func() error, err error) {
	directory, err := os.MkdirTemp("", "dva-skill-dogfood-bin-")
	if err != nil {
		return "", "", nil, fmt.Errorf("create immutable executable directory: %w", err)
	}
	cleanup = func() error { return removeAll("clean immutable executable directory "+directory, directory) }
	input, err := os.Open(source)
	if err != nil {
		return "", "", nil, errors.Join(fmt.Errorf("open DVA_BIN: %w", err), cleanup())
	}
	destination := filepath.Join(directory, "dva")
	output, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o700)
	if err != nil {
		return "", "", nil, errors.Join(fmt.Errorf("create immutable DVA copy: %w", err), input.Close(), cleanup())
	}
	hash := sha256.New()
	_, copyErr := io.Copy(io.MultiWriter(output, hash), input)
	closeErr := errors.Join(input.Close(), output.Close())
	if copyErr != nil || closeErr != nil {
		return "", "", nil, errors.Join(copyErr, closeErr, cleanup())
	}
	digest = hex.EncodeToString(hash.Sum(nil))
	if digest != strings.ToLower(expected) {
		return "", "", nil, errors.Join(fmt.Errorf("DVA_BIN SHA-256 %s does not match expected %s", digest, expected), cleanup())
	}
	return destination, digest, cleanup, nil
}

func removeAll(context, path string) error {
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("%s: %w", context, err)
	}
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

func gitRoot(path string) (string, error) {
	if path == "" {
		return "", errors.New("must be set to an absolute Git repository root path")
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
	return root, nil
}

func verifyFlowDryRun(inv invocation, flowRoot string) (err error) {
	before, err := snapshotGitTreeState(flowRoot)
	if err != nil {
		return err
	}
	beforeSkills, err := snapshotRuntimePaths(flowRoot)
	if err != nil {
		return fmt.Errorf("snapshot runtime paths before dry-run: %w", err)
	}
	stateDir, err := os.MkdirTemp("", "dva-skill-dogfood-env-")
	if err != nil {
		return fmt.Errorf("create dry-run state directory: %w", err)
	}
	defer func() { err = errors.Join(err, removeAll("clean dry-run environment "+stateDir, stateDir)) }()
	dryRoots := map[string]string{
		"HOME": filepath.Join(stateDir, "home"), "XDG_CONFIG_HOME": filepath.Join(stateDir, "config"),
		"XDG_DATA_HOME": filepath.Join(stateDir, "data"), "XDG_CACHE_HOME": filepath.Join(stateDir, "cache"), "XDG_STATE_HOME": filepath.Join(stateDir, "state"),
	}
	for _, root := range dryRoots {
		if err := os.MkdirAll(root, 0o755); err != nil {
			return err
		}
	}
	dryRun := invocation{binary: inv.binary, env: withEnvironment(inv.env, dryRoots)}
	result, err := dryRun.json(flowRoot, "skill", "install", "--scope", "project", "--runtime", allNativeRuntimes, "--dry-run")
	if err != nil {
		return fmt.Errorf("run project-scope dry-run: %w", err)
	}
	if result.Operation != "install" || !result.DryRun || result.Scope != "project" {
		return fmt.Errorf("unexpected dry-run response: operation=%q dry_run=%t scope=%q", result.Operation, result.DryRun, result.Scope)
	}
	if err := requireEnvelope(result, "install", true); err != nil {
		return fmt.Errorf("project-scope dry-run envelope: %w", err)
	}
	if err := requireDestinations(flowRoot, result, "would-install"); err != nil {
		return fmt.Errorf("project-scope dry-run result: %w", err)
	}
	after, err := snapshotGitTreeState(flowRoot)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(before, after) {
		return fmt.Errorf("project-scope dry-run changed Git-visible state:\nbefore:\n%safter:\n%s", formatGitTreeState(before), formatGitTreeState(after))
	}
	afterSkills, err := snapshotRuntimePaths(flowRoot)
	if err != nil {
		return fmt.Errorf("snapshot runtime paths after dry-run: %w", err)
	}
	if !reflect.DeepEqual(beforeSkills, afterSkills) {
		return fmt.Errorf("project-scope dry-run changed runtime paths:\nbefore:\n%safter:\n%s", formatSnapshot(beforeSkills), formatSnapshot(afterSkills))
	}
	for name, root := range dryRoots {
		if err := requireEmptyDirectory(root); err != nil {
			return fmt.Errorf("project-scope dry-run wrote %s: %w", name, err)
		}
	}
	return nil
}

func verifyFixtureRoundTrip(inv invocation, project, stateRoot string) error {
	installed, err := inv.json(project, "skill", "install", "--scope", "project", "--runtime", allNativeRuntimes)
	if err != nil {
		return fmt.Errorf("install isolated fixture: %w", err)
	}
	if err := requireEnvelope(installed, "install", false); err != nil {
		return fmt.Errorf("install envelope: %w", err)
	}
	if err := requireDestinations(project, installed, "installed"); err != nil {
		return fmt.Errorf("install result: %w", err)
	}

	status, err := inv.json(project, "skill", "status", "--scope", "project", "--runtime", allNativeRuntimes)
	if err != nil {
		return fmt.Errorf("check installed fixture status: %w", err)
	}
	if err := requireEnvelope(status, "status", false); err != nil {
		return fmt.Errorf("status envelope: %w", err)
	}
	if err := requireDestinations(project, status, "installed"); err != nil {
		return fmt.Errorf("installed status: %w", err)
	}
	if err := verifyOwnedArtifacts(project, stateRoot); err != nil {
		return fmt.Errorf("installed artifacts: %w", err)
	}

	unlinked, err := inv.json(project, "skill", "uninstall", "--scope", "project", "--runtime", "codex")
	if err != nil {
		return fmt.Errorf("uninstall Codex from shared destination: %w", err)
	}
	if err := requireEnvelope(unlinked, "uninstall", false); err != nil {
		return fmt.Errorf("codex uninstall envelope: %w", err)
	}
	if err := requireOnlyDestination(project, unlinked, ".agents/skills", "unlinked"); err != nil {
		return fmt.Errorf("codex-only uninstall result: %w", err)
	}
	if err := requireRuntimeStatuses(unlinked.Results[0], map[string]string{"codex": "unlinked"}); err != nil {
		return fmt.Errorf("codex-only uninstall membership: %w", err)
	}

	partial, err := inv.json(project, "skill", "status", "--scope", "project", "--runtime", "codex,antigravity")
	if err != nil {
		return fmt.Errorf("check shared destination after Codex uninstall: %w", err)
	}
	if err := requireEnvelope(partial, "status", false); err != nil {
		return fmt.Errorf("shared status envelope: %w", err)
	}
	if err := requireOnlyDestination(project, partial, ".agents/skills", "partial"); err != nil {
		return fmt.Errorf("shared destination status: %w", err)
	}
	if err := requireRuntimeStatuses(partial.Results[0], map[string]string{"codex": "absent", "antigravity": "installed"}); err != nil {
		return fmt.Errorf("shared destination membership: %w", err)
	}
	if err := verifySharedUnlink(project, stateRoot); err != nil {
		return fmt.Errorf("shared runtime unlink artifacts: %w", err)
	}

	removed, err := inv.json(project, "skill", "uninstall", "--scope", "project", "--runtime", allNativeRuntimes)
	if err != nil {
		return fmt.Errorf("uninstall remaining fixture skills: %w", err)
	}
	if err := requireEnvelope(removed, "uninstall", false); err != nil {
		return fmt.Errorf("remaining uninstall envelope: %w", err)
	}
	if err := requireDestinationStatusSet(project, removed, map[string]string{
		".agents/skills": "uninstalled", ".claude/skills": "uninstalled", ".grok/skills": "uninstalled", ".opencode/skills": "uninstalled",
	}); err != nil {
		return fmt.Errorf("remaining uninstall result: %w", err)
	}

	absent, err := inv.json(project, "skill", "status", "--scope", "project", "--runtime", allNativeRuntimes)
	if err != nil {
		return fmt.Errorf("check removed fixture status: %w", err)
	}
	if err := requireEnvelope(absent, "status", false); err != nil {
		return fmt.Errorf("absent status envelope: %w", err)
	}
	if err := requireDestinations(project, absent, "absent"); err != nil {
		return fmt.Errorf("final status: %w", err)
	}
	if err := verifyArtifactsAbsent(project, stateRoot); err != nil {
		return fmt.Errorf("final artifacts: %w", err)
	}
	return nil
}

func verifyOwnedArtifacts(project, stateRoot string) error {
	for suffix, runtimes := range map[string][]string{".agents/skills": {"antigravity", "codex"}, ".claude/skills": {"claude-code"}, ".grok/skills": {"grok"}, ".opencode/skills": {"opencode"}} {
		destination := filepath.Join(project, filepath.FromSlash(suffix))
		files, err := installedSkillFiles(destination)
		if err != nil {
			return err
		}
		record, err := readReceipt(stateRoot, destination)
		if err != nil {
			return err
		}
		if record.Schema != 1 || record.Scope != "project" || record.Destination != destination || record.Version == "" || !sameStrings(record.Runtimes, runtimes) || !sameFiles(record.Files, files) || record.BundleSHA != bundleSHA(files) {
			return fmt.Errorf("invalid receipt for %s", destination)
		}
	}
	return nil
}

func verifySharedUnlink(project, stateRoot string) error {
	destination := filepath.Join(project, ".agents", "skills")
	if _, err := installedSkillFiles(destination); err != nil {
		return fmt.Errorf("shared skill files not retained: %w", err)
	}
	record, err := readReceipt(stateRoot, destination)
	if err != nil {
		return err
	}
	if !sameStrings(record.Runtimes, []string{"antigravity"}) {
		return fmt.Errorf("shared receipt runtimes=%v, want [antigravity]", record.Runtimes)
	}
	return nil
}

func verifyArtifactsAbsent(project, stateRoot string) error {
	for _, suffix := range []string{".agents/skills", ".claude/skills", ".grok/skills", ".opencode/skills"} {
		for _, skill := range []string{"dva", "dva-config"} {
			if _, err := os.Lstat(filepath.Join(project, filepath.FromSlash(suffix), skill)); !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("skill artifact remains at %s", filepath.Join(suffix, skill))
			}
		}
	}
	return requireEmptyOrMissingDirectory(filepath.Join(stateRoot, "skill-installs"))
}

func readReceipt(stateRoot, destination string) (receiptRecord, error) {
	digest := sha256.Sum256([]byte(destination))
	path := filepath.Join(stateRoot, "skill-installs", hex.EncodeToString(digest[:])+".json")
	contents, err := os.ReadFile(path)
	if err != nil {
		return receiptRecord{}, err
	}
	var record receiptRecord
	if err := json.Unmarshal(contents, &record); err != nil {
		return receiptRecord{}, err
	}
	return record, nil
}

func installedSkillFiles(destination string) ([]fileHash, error) {
	var files []fileHash
	for _, skill := range []string{"dva", "dva-config"} {
		root := filepath.Join(destination, skill)
		info, err := os.Lstat(root)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("invalid skill root %s", root)
		}
		err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			if entry.Type()&os.ModeSymlink != 0 || !entry.Type().IsRegular() {
				return fmt.Errorf("invalid skill file %s", path)
			}
			digest, err := fileSHA256(path)
			if err != nil {
				return err
			}
			relative, err := filepath.Rel(destination, path)
			if err != nil {
				return err
			}
			files = append(files, fileHash{Path: filepath.ToSlash(relative), SHA: digest})
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, nil
}
func sameFiles(left, right []fileHash) bool { return reflect.DeepEqual(left, right) }
func bundleSHA(files []fileHash) string {
	hash := sha256.New()
	for _, file := range files {
		_, _ = hash.Write([]byte(file.Path))
		_, _ = hash.Write([]byte{0})
		_, _ = hash.Write([]byte(file.SHA))
		_, _ = hash.Write([]byte{0})
	}
	return hex.EncodeToString(hash.Sum(nil))
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

func snapshotGitTreeState(root string) (gitTreeState, error) {
	status, err := gitStatus(root)
	if err != nil {
		return gitTreeState{}, err
	}
	worktreeDiffSHA, err := commandOutputSHA256("", "git", "-C", root, "diff", "--no-ext-diff", "--no-textconv", "--binary", "HEAD", "--")
	if err != nil {
		return gitTreeState{}, fmt.Errorf("snapshot worktree diff: %w", err)
	}
	indexDiffSHA, err := commandOutputSHA256("", "git", "-C", root, "diff", "--cached", "--no-ext-diff", "--no-textconv", "--binary", "HEAD", "--")
	if err != nil {
		return gitTreeState{}, fmt.Errorf("snapshot index diff: %w", err)
	}
	untrackedOutput, err := commandOutput(nil, "git", "-C", root, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return gitTreeState{}, fmt.Errorf("list untracked files: %w", err)
	}
	untracked, err := snapshotListedFiles(root, strings.Split(strings.TrimSuffix(untrackedOutput, "\x00"), "\x00"))
	if err != nil {
		return gitTreeState{}, fmt.Errorf("snapshot untracked files: %w", err)
	}
	return gitTreeState{
		Status:           status,
		WorktreeDiffSHA:  worktreeDiffSHA,
		IndexDiffSHA:     indexDiffSHA,
		UntrackedEntries: untracked,
	}, nil
}

func snapshotListedFiles(root string, paths []string) ([]treeEntry, error) {
	entries := make([]treeEntry, 0, len(paths))
	for _, relative := range paths {
		if relative == "" {
			continue
		}
		path := filepath.Join(root, filepath.FromSlash(relative))
		info, err := os.Lstat(path)
		if err != nil {
			return nil, err
		}
		entry := treeEntry{Path: filepath.ToSlash(relative), Mode: info.Mode(), ModTime: info.ModTime()}
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			target, err := os.Readlink(path)
			if err != nil {
				return nil, err
			}
			entry.Type, entry.LinkTarget = "symlink", target
		case info.Mode().IsRegular():
			digest, err := fileSHA256(path)
			if err != nil {
				return nil, err
			}
			entry.Type, entry.ContentSHA = "file", digest
		default:
			entry.Type = "other"
		}
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return entries, nil
}

func commandOutputSHA256(directory, command string, args ...string) (string, error) {
	cmd := exec.Command(command, args...)
	cmd.Dir = directory
	hash := sha256.New()
	stderr := limitedBuffer{limit: commandStderrLimit}
	cmd.Stdout = hash
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s: %w\n%s", strings.Join(append([]string{command}, args...), " "), err, stderr.String())
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func formatGitTreeState(state gitTreeState) string {
	return fmt.Sprintf("status:\n%sworktree_diff_sha256=%s\nindex_diff_sha256=%s\nuntracked:\n%s", state.Status, state.WorktreeDiffSHA, state.IndexDiffSHA, formatSnapshot(state.UntrackedEntries))
}

func snapshotRuntimePaths(root string) ([]treeEntry, error) {
	var snapshot []treeEntry
	seen := make(map[string]bool)
	for _, relative := range []string{".agents/skills", ".claude/skills", ".grok/skills", ".opencode/skills"} {
		if err := rejectSymlinkComponents(root, relative); err != nil {
			return nil, err
		}
		components := strings.Split(filepath.ToSlash(relative), "/")
		for index := 1; index < len(components); index++ {
			ancestor := strings.Join(components[:index], "/")
			if seen[ancestor] {
				continue
			}
			seen[ancestor] = true
			entry, err := snapshotSinglePath(root, ancestor)
			if err != nil {
				return nil, err
			}
			snapshot = append(snapshot, entry)
		}
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
			return nil, fmt.Errorf("refusing symlink runtime path %s", path)
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
			record := treeEntry{Path: filepath.ToSlash(relativePath), Mode: info.Mode(), ModTime: info.ModTime()}
			switch {
			case info.Mode()&os.ModeSymlink != 0:
				relativeToRuntime, err := filepath.Rel(path, current)
				if err != nil {
					return err
				}
				managed, _, _ := strings.Cut(filepath.ToSlash(relativeToRuntime), "/")
				if managed == "dva" || managed == "dva-config" {
					return fmt.Errorf("refusing symlink managed skill target %s", current)
				}
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

func snapshotSinglePath(root, relative string) (treeEntry, error) {
	path := filepath.Join(root, filepath.FromSlash(relative))
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return treeEntry{Path: filepath.ToSlash(relative), Type: "missing"}, nil
	}
	if err != nil {
		return treeEntry{}, err
	}
	entry := treeEntry{Path: filepath.ToSlash(relative), Mode: info.Mode(), ModTime: info.ModTime()}
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		target, err := os.Readlink(path)
		if err != nil {
			return treeEntry{}, err
		}
		entry.Type, entry.LinkTarget = "symlink", target
	case info.IsDir():
		entry.Type = "directory"
	case info.Mode().IsRegular():
		digest, err := fileSHA256(path)
		if err != nil {
			return treeEntry{}, err
		}
		entry.Type, entry.ContentSHA = "file", digest
	default:
		entry.Type = "other"
	}
	return entry, nil
}

func rejectSymlinkComponents(root, relative string) error {
	current := root
	for component := range strings.SplitSeq(filepath.FromSlash(relative), string(filepath.Separator)) {
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing symlink runtime path component %s", current)
		}
	}
	return nil
}

func formatSnapshot(snapshot []treeEntry) string {
	var lines []string
	for _, entry := range snapshot {
		line := fmt.Sprintf("%s type=%s mode=%#o modtime=%s", entry.Path, entry.Type, entry.Mode, entry.ModTime.UTC().Format(time.RFC3339Nano))
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

func requireEmptyOrMissingDirectory(path string) error {
	err := requireEmptyDirectory(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func fileSHA256(path string) (digest string, err error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { err = errors.Join(err, file.Close()) }()
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

func requireEnvelope(result commandResult, operation string, dryRun bool) error {
	if result.Operation != operation || result.DryRun != dryRun || result.Scope != "project" {
		return fmt.Errorf("got operation=%q dry_run=%t scope=%q", result.Operation, result.DryRun, result.Scope)
	}
	return nil
}

func requireDestinations(project string, result commandResult, status string) error {
	expected := map[string]string{
		".agents/skills": "", ".claude/skills": "", ".grok/skills": "", ".opencode/skills": "",
	}
	for suffix := range expected {
		expected[suffix] = status
	}
	return requireDestinationStatusSet(project, result, expected)
}

func requireDestinationStatusSet(project string, result commandResult, expected map[string]string) error {
	if len(result.Results) != len(expected) {
		return fmt.Errorf("got %d destinations, want %d", len(result.Results), len(expected))
	}
	seen := make(map[string]bool, len(expected))
	for _, entry := range result.Results {
		suffix, ok := destinationSuffix(entry.Destination)
		if !ok {
			return fmt.Errorf("unexpected destination %q", entry.Destination)
		}
		if entry.Destination != filepath.Join(project, filepath.FromSlash(suffix)) {
			return fmt.Errorf("destination %q is not exact project path", entry.Destination)
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
		wantRuntimes := map[string][]string{".agents/skills": {"antigravity", "codex"}, ".claude/skills": {"claude-code"}, ".grok/skills": {"grok"}, ".opencode/skills": {"opencode"}}[suffix]
		if !sameStrings(entry.Runtimes, wantRuntimes) {
			return fmt.Errorf("%s has runtimes=%v statuses=%d, want runtimes=%v", suffix, entry.Runtimes, len(entry.RuntimeStatuses), wantRuntimes)
		}
		wantStatuses := make(map[string]string, len(wantRuntimes))
		for _, runtime := range wantRuntimes {
			wantStatuses[runtime] = entry.Status
		}
		if result.Operation == "uninstall" && suffix == ".agents/skills" && entry.Status == "uninstalled" {
			wantStatuses["codex"] = "not-installed"
		}
		if err := requireRuntimeStatuses(entry, wantStatuses); err != nil {
			return err
		}
	}
	return nil
}

func sameStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	left, right = append([]string(nil), left...), append([]string(nil), right...)
	sort.Strings(left)
	sort.Strings(right)
	return reflect.DeepEqual(left, right)
}

func requireOnlyDestination(project string, result commandResult, suffix, status string) error {
	if len(result.Results) != 1 {
		return fmt.Errorf("got %d destinations, want 1", len(result.Results))
	}
	entry := result.Results[0]
	actual, ok := destinationSuffix(entry.Destination)
	if !ok || actual != suffix {
		return fmt.Errorf("got destination %q, want suffix %q", entry.Destination, suffix)
	}
	if entry.Destination != filepath.Join(project, filepath.FromSlash(suffix)) {
		return fmt.Errorf("destination %q is not exact project path", entry.Destination)
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
	if len(entry.RuntimeStatuses) != len(expected) {
		return fmt.Errorf("%s has %d runtime statuses, want %d", entry.Destination, len(entry.RuntimeStatuses), len(expected))
	}
	actual := make(map[string]string, len(entry.RuntimeStatuses))
	for _, status := range entry.RuntimeStatuses {
		if _, duplicate := actual[status.Runtime]; duplicate {
			return fmt.Errorf("%s repeats runtime status %q", entry.Destination, status.Runtime)
		}
		actual[status.Runtime] = status.Status
	}
	for runtime, want := range expected {
		if actual[runtime] != want {
			return fmt.Errorf("%s runtime %q has status %q, want %q", entry.Destination, runtime, actual[runtime], want)
		}
	}
	return nil
}
