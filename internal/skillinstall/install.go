// Package skillinstall installs the DVA-owned Agent Skills without requiring an AI runtime.
package skillinstall

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	pathpkg "path"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"syscall"

	bundled "github.com/ScriptonBasestar/dva/skills"
)

type Scope string

const (
	ScopeUser    Scope = "user"
	ScopeProject Scope = "project"
)

type Runtime string

const (
	RuntimeClaudeCode  Runtime = "claude-code"
	RuntimeCodex       Runtime = "codex"
	RuntimeOpenCode    Runtime = "opencode"
	RuntimeGrok        Runtime = "grok"
	RuntimeAntigravity Runtime = "antigravity"
	RuntimeAgentMesh   Runtime = "agent-mesh"
)

// Options supplies filesystem roots. Empty roots are resolved from the process environment.
type Options struct {
	Scope       Scope
	Runtimes    []Runtime
	HomeDir     string
	ProjectRoot string
	StateRoot   string
	DryRun      bool
	Version     string
}

type Result struct {
	Scope        Scope               `json:"scope"`
	Destinations []DestinationResult `json:"destinations"`
}

type DestinationResult struct {
	Destination        string          `json:"destination"`
	Runtimes           []Runtime       `json:"runtimes"`
	Skills             []string        `json:"skills"`
	Status             string          `json:"status"`
	Detail             string          `json:"detail,omitempty"`
	SourceVersion      string          `json:"source_version,omitempty"`
	SourceBundleSHA    string          `json:"source_bundle_sha256,omitempty"`
	InstalledVersion   string          `json:"installed_version,omitempty"`
	InstalledBundleSHA string          `json:"installed_bundle_sha256,omitempty"`
	RuntimeStatuses    []RuntimeStatus `json:"runtime_statuses"`
}

type RuntimeStatus struct {
	Runtime Runtime `json:"runtime"`
	Status  string  `json:"status"`
}

type receipt struct {
	Schema      int        `json:"schema"`
	Format      string     `json:"format,omitempty"`
	Scope       Scope      `json:"scope"`
	Destination string     `json:"destination"`
	Runtimes    []Runtime  `json:"runtimes"`
	Version     string     `json:"version"`
	BundleSHA   string     `json:"bundle_sha256"`
	Files       []fileHash `json:"files"`
}

const (
	receiptSchemaCurrent = 2
	receiptFormatNative  = "agent-skills-directory"
	receiptFormatFlat    = "agent-mesh-flat-markdown"
)

type fileHash struct {
	Path string `json:"path"`
	SHA  string `json:"sha256"`
}

type destination struct {
	path     string
	runtimes []Runtime
}

type destinationState struct {
	target destination
	record receipt
	found  bool
}

type skillBundle struct {
	files    []fileHash
	contents map[string][]byte
}

// DefaultRuntimes returns every supported runtime.
func DefaultRuntimes() []Runtime {
	return []Runtime{RuntimeClaudeCode, RuntimeCodex, RuntimeOpenCode, RuntimeGrok, RuntimeAntigravity, RuntimeAgentMesh}
}

// Install copies the embedded skills into every selected runtime directory.
func Install(options Options) (Result, error) {
	resolved, destinations, err := resolve(options)
	if err != nil {
		return Result{}, err
	}
	states := make([]destinationState, 0, len(destinations))
	for _, target := range destinations {
		bundle, err := bundleFor(target)
		if err != nil {
			return Result{}, err
		}
		receiptFile := receiptPath(resolved.StateRoot, target.path)
		existing, found, err := readReceipt(receiptFile)
		if err != nil {
			return Result{}, fmt.Errorf("read receipt for %s: %w", target.path, err)
		}
		if found {
			if err := validateReceipt(existing, resolved.Scope, target); err != nil {
				return Result{}, err
			}
			if err := verifyInstalled(target.path, existing.Files); err != nil {
				return Result{}, fmt.Errorf("refusing to update drifted DVA skill installation at %s: %w", target.path, err)
			}
		} else if err := ensureNoCollision(target.path, bundle.files); err != nil {
			return Result{}, err
		}
		states = append(states, destinationState{target: target, record: existing, found: found})
	}
	result := Result{Scope: resolved.Scope, Destinations: make([]DestinationResult, 0, len(destinations))}
	for _, state := range states {
		target, existing, found := state.target, state.record, state.found
		bundle, err := bundleFor(target)
		if err != nil {
			return Result{}, err
		}
		entry := resultEntry(target, resolved.Version, sourceBundleSHA(bundle.files))
		receiptFile := receiptPath(resolved.StateRoot, target.path)
		if found && equalFiles(existing.Files, bundle.files) && containsRuntimes(existing.Runtimes, target.runtimes) {
			entry.Status = "up-to-date"
			setAllRuntimeStatuses(&entry, "up-to-date")
			result.Destinations = append(result.Destinations, entry)
			continue
		}

		if resolved.DryRun {
			entry.Status = "would-install"
			setAllRuntimeStatuses(&entry, "would-install")
			result.Destinations = append(result.Destinations, entry)
			continue
		}
		if found && equalFiles(existing.Files, bundle.files) {
			existing.Runtimes = unionRuntimes(existing.Runtimes, target.runtimes)
			if err := writeReceipt(receiptFile, existing); err != nil {
				return Result{}, fmt.Errorf("update receipt: %w", err)
			}
			entry.Status = "installed"
			setAllRuntimeStatuses(&entry, "installed")
			result.Destinations = append(result.Destinations, entry)
			continue
		}
		if err := ensureDestination(target.path); err != nil {
			return Result{}, err
		}
		// Revalidate at the mutation boundary. The all-destination preflight prevents
		// predictable partial installs; this second check refuses changes made while
		// the preflight was examining later destinations.
		if found {
			if err := verifyInstalled(target.path, existing.Files); err != nil {
				return Result{}, fmt.Errorf("refusing to update drifted DVA skill installation at %s: %w", target.path, err)
			}
		} else if err := ensureNoCollision(target.path, bundle.files); err != nil {
			return Result{}, err
		}
		undo, finalize, err := replaceBundle(target.path, bundle, found)
		if err != nil {
			return Result{}, err
		}
		newReceipt := receipt{Schema: receiptSchemaCurrent, Format: targetReceiptFormat(target), Scope: resolved.Scope, Destination: target.path, Runtimes: target.runtimes, Version: resolved.Version, BundleSHA: sourceBundleSHA(bundle.files), Files: bundle.files}
		if found {
			newReceipt.Runtimes = unionRuntimes(existing.Runtimes, target.runtimes)
		}
		if err := writeReceipt(receiptFile, newReceipt); err != nil {
			if rollbackErr := undo(); rollbackErr != nil {
				return Result{}, fmt.Errorf("write receipt: %w (rollback also failed: %v)", err, rollbackErr)
			}
			return Result{}, fmt.Errorf("write receipt: %w", err)
		}
		if err := finalize(); err != nil {
			return Result{}, fmt.Errorf("clean up replaced DVA skill directories: %w", err)
		}
		entry.Status = "installed"
		setAllRuntimeStatuses(&entry, "installed")
		result.Destinations = append(result.Destinations, entry)
	}
	return result, nil
}

// Status reports whether every requested destination is installed and unmodified.
func Status(options Options) (Result, error) {
	resolved, destinations, err := resolve(options)
	if err != nil {
		return Result{}, err
	}
	result := Result{Scope: resolved.Scope, Destinations: make([]DestinationResult, 0, len(destinations))}
	for _, target := range destinations {
		bundle, err := bundleFor(target)
		if err != nil {
			return Result{}, err
		}
		entry := resultEntry(target, resolved.Version, sourceBundleSHA(bundle.files))
		record, found, err := readReceipt(receiptPath(resolved.StateRoot, target.path))
		if err != nil {
			entry.Status, entry.Detail = "invalid-receipt", err.Error()
			setAllRuntimeStatuses(&entry, "invalid-receipt")
		} else if !found {
			if hasForeignCollision(target.path, bundle.files) {
				entry.Status = "foreign-conflict"
			} else {
				entry.Status = "absent"
			}
			setAllRuntimeStatuses(&entry, entry.Status)
		} else if err := validateReceipt(record, resolved.Scope, target); err != nil {
			entry.Status, entry.Detail = "invalid-receipt", err.Error()
			setAllRuntimeStatuses(&entry, "invalid-receipt")
		} else if err := verifyInstalled(target.path, record.Files); err != nil {
			entry.InstalledVersion, entry.InstalledBundleSHA = record.Version, record.BundleSHA
			entry.Status, entry.Detail = "drifted", err.Error()
			setAllRuntimeStatuses(&entry, "drifted")
		} else {
			entry.InstalledVersion, entry.InstalledBundleSHA = record.Version, record.BundleSHA
			entry.Status = setMembershipStatuses(&entry, record.Runtimes, "installed", "absent")
		}
		result.Destinations = append(result.Destinations, entry)
	}
	return result, nil
}

// Uninstall removes only a verified DVA-owned installation.
func Uninstall(options Options) (Result, error) {
	resolved, destinations, err := resolve(options)
	if err != nil {
		return Result{}, err
	}
	states := make([]destinationState, 0, len(destinations))
	for _, target := range destinations {
		receiptFile := receiptPath(resolved.StateRoot, target.path)
		record, found, err := readReceipt(receiptFile)
		if err != nil {
			return Result{}, fmt.Errorf("read receipt for %s: %w", target.path, err)
		}
		if found {
			if err := validateReceipt(record, resolved.Scope, target); err != nil {
				return Result{}, err
			}
			if installedRequested := intersectRuntimes(record.Runtimes, target.runtimes); len(installedRequested) > 0 {
				if err := verifyInstalled(target.path, record.Files); err != nil {
					return Result{}, fmt.Errorf("refusing to uninstall drifted DVA skill installation at %s: %w", target.path, err)
				}
			}
		}
		states = append(states, destinationState{target: target, record: record, found: found})
	}
	result := Result{Scope: resolved.Scope, Destinations: make([]DestinationResult, 0, len(destinations))}
	for _, state := range states {
		target, record, found := state.target, state.record, state.found
		entry := resultEntry(target, "", "")
		receiptFile := receiptPath(resolved.StateRoot, target.path)
		if !found {
			entry.Status = "not-installed"
			setAllRuntimeStatuses(&entry, "not-installed")
			result.Destinations = append(result.Destinations, entry)
			continue
		}
		installedRequested := intersectRuntimes(record.Runtimes, target.runtimes)
		if len(installedRequested) == 0 {
			entry.Status = "not-installed"
			setAllRuntimeStatuses(&entry, "not-installed")
			result.Destinations = append(result.Destinations, entry)
			continue
		}
		if resolved.DryRun {
			entry.Status = setMembershipStatuses(&entry, record.Runtimes, "would-uninstall", "not-installed")
			result.Destinations = append(result.Destinations, entry)
			continue
		}
		remaining := removeRuntimes(record.Runtimes, target.runtimes)
		if len(remaining) > 0 {
			record.Runtimes = remaining
			if err := writeReceipt(receiptFile, record); err != nil {
				return Result{}, fmt.Errorf("update receipt: %w", err)
			}
			entry.Status = "unlinked"
			setMembershipStatuses(&entry, installedRequested, "unlinked", "not-installed")
			result.Destinations = append(result.Destinations, entry)
			continue
		}
		if hasRuntime(target.runtimes, RuntimeAgentMesh) {
			for _, file := range record.Files {
				if err := os.Remove(filepath.Join(target.path, filepath.FromSlash(file.Path))); err != nil && !errors.Is(err, os.ErrNotExist) {
					return Result{}, fmt.Errorf("remove DVA skill %s: %w", file.Path, err)
				}
			}
			if err := os.Remove(target.path); err != nil && !errors.Is(err, os.ErrNotExist) && !errors.Is(err, syscall.ENOTEMPTY) {
				return Result{}, fmt.Errorf("remove empty Agent Mesh DVA namespace: %w", err)
			}
		} else {
			for _, name := range bundled.Names {
				if err := os.RemoveAll(filepath.Join(target.path, name)); err != nil {
					return Result{}, fmt.Errorf("remove DVA skill %s: %w", name, err)
				}
			}
		}
		if err := os.Remove(receiptFile); err != nil && !errors.Is(err, os.ErrNotExist) {
			return Result{}, fmt.Errorf("remove receipt: %w", err)
		}
		entry.Status = "uninstalled"
		setMembershipStatuses(&entry, installedRequested, "uninstalled", "not-installed")
		result.Destinations = append(result.Destinations, entry)
	}
	return result, nil
}

func resolve(options Options) (Options, []destination, error) {
	if options.Scope != ScopeUser && options.Scope != ScopeProject {

		return Options{}, nil, fmt.Errorf("skill install scope must be %q or %q", ScopeUser, ScopeProject)
	}
	if options.HomeDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return Options{}, nil, fmt.Errorf("resolve home directory: %w", err)
		}
		options.HomeDir = home
	}
	home, err := filepath.Abs(options.HomeDir)
	if err != nil {
		return Options{}, nil, err
	}
	options.HomeDir = home
	if options.ProjectRoot == "" {
		project, err := os.Getwd()
		if err != nil {
			return Options{}, nil, err
		}
		options.ProjectRoot = project
	}
	project, err := filepath.Abs(options.ProjectRoot)
	if err != nil {
		return Options{}, nil, err
	}
	options.ProjectRoot = project
	if options.StateRoot == "" {
		if xdg := os.Getenv("XDG_STATE_HOME"); xdg != "" {
			options.StateRoot = filepath.Join(xdg, "dva")
		} else {
			options.StateRoot = filepath.Join(home, ".local", "state", "dva")
		}
	}
	state, err := filepath.Abs(options.StateRoot)
	if err != nil {
		return Options{}, nil, err
	}
	options.StateRoot = state
	if options.Version == "" {
		options.Version = "unknown"
	}
	if len(options.Runtimes) == 0 {
		options.Runtimes = DefaultRuntimes()
	}
	seen := map[Runtime]bool{}
	groups := map[string][]Runtime{}
	for _, runtime := range options.Runtimes {
		if seen[runtime] {
			continue
		}
		seen[runtime] = true
		path, err := runtimePath(runtime, options.Scope, home, project)
		if err != nil {
			return Options{}, nil, err
		}
		groups[path] = append(groups[path], runtime)
	}
	paths := make([]string, 0, len(groups))
	for path := range groups {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	destinations := make([]destination, 0, len(paths))
	for _, path := range paths {
		runtimes := groups[path]
		slices.Sort(runtimes)
		destinations = append(destinations, destination{path: path, runtimes: runtimes})
	}
	return options, destinations, nil
}

func runtimePath(runtime Runtime, scope Scope, home, project string) (string, error) {
	var relative string
	switch scope {
	case ScopeUser:
		switch runtime {
		case RuntimeClaudeCode:
			relative = ".claude/skills"
		case RuntimeCodex:
			relative = ".agents/skills"
		case RuntimeOpenCode:
			relative = ".config/opencode/skills"
		case RuntimeGrok:
			relative = ".grok/skills"
		case RuntimeAntigravity:
			relative = ".gemini/config/skills"
		case RuntimeAgentMesh:
			relative = ".config/agent-mesh/skills/dva"
		default:
			return "", fmt.Errorf("unsupported skill runtime %q", runtime)
		}
		return filepath.Join(home, relative), nil
	case ScopeProject:
		switch runtime {
		case RuntimeClaudeCode:
			relative = ".claude/skills"
		case RuntimeCodex, RuntimeAntigravity:
			relative = ".agents/skills"
		case RuntimeOpenCode:
			relative = ".opencode/skills"
		case RuntimeGrok:
			relative = ".grok/skills"
		case RuntimeAgentMesh:
			relative = ".agent-mesh/skills/dva"
		default:
			return "", fmt.Errorf("unsupported skill runtime %q", runtime)
		}
		return filepath.Join(project, relative), nil
	default:
		return "", fmt.Errorf("unsupported skill scope %q", scope)
	}
}

func resultEntry(target destination, version, bundleSHA string) DestinationResult {
	return DestinationResult{
		Destination: target.path, Runtimes: append([]Runtime(nil), target.runtimes...),
		Skills: append([]string(nil), bundled.Names...), SourceVersion: version, SourceBundleSHA: bundleSHA,
		RuntimeStatuses: make([]RuntimeStatus, 0, len(target.runtimes)),
	}
}

func setAllRuntimeStatuses(entry *DestinationResult, status string) {
	entry.RuntimeStatuses = entry.RuntimeStatuses[:0]
	for _, runtime := range entry.Runtimes {
		entry.RuntimeStatuses = append(entry.RuntimeStatuses, RuntimeStatus{Runtime: runtime, Status: status})
	}
}

func setMembershipStatuses(entry *DestinationResult, present []Runtime, presentStatus, absentStatus string) string {
	set := make(map[Runtime]bool, len(present))
	for _, runtime := range present {
		set[runtime] = true
	}
	entry.RuntimeStatuses = entry.RuntimeStatuses[:0]
	allPresent, allAbsent := true, true
	for _, runtime := range entry.Runtimes {
		status := absentStatus
		if set[runtime] {
			status = presentStatus
			allAbsent = false
		} else {
			allPresent = false
		}
		entry.RuntimeStatuses = append(entry.RuntimeStatuses, RuntimeStatus{Runtime: runtime, Status: status})
	}
	if allPresent {
		return presentStatus
	}
	if allAbsent {
		return absentStatus
	}
	return "partial"
}

func bundledBundle() (skillBundle, error) {
	bundle := skillBundle{contents: make(map[string][]byte)}
	for _, name := range bundled.Names {
		err := fs.WalkDir(bundled.Files, name, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			contents, err := bundled.Files.ReadFile(path)
			if err != nil {
				return err
			}
			digest := sha256.Sum256(contents)
			path = filepath.ToSlash(path)
			bundle.files = append(bundle.files, fileHash{Path: path, SHA: hex.EncodeToString(digest[:])})
			bundle.contents[path] = contents
			return nil
		})
		if err != nil {
			return skillBundle{}, err
		}
	}
	sort.Slice(bundle.files, func(i, j int) bool { return bundle.files[i].Path < bundle.files[j].Path })
	return bundle, nil
}

func bundledFiles() ([]fileHash, error) {
	bundle, err := bundledBundle()
	return bundle.files, err
}

func bundleFor(target destination) (skillBundle, error) {
	if hasRuntime(target.runtimes, RuntimeAgentMesh) {
		return agentMeshBundle()
	}
	return bundledBundle()
}

func hasRuntime(runtimes []Runtime, wanted Runtime) bool {
	return slices.Contains(runtimes, wanted)
}

func ensureDestination(path string) error {
	if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing symlink skill destination %s", path)
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.MkdirAll(path, 0o755)
}

func ensureNoCollision(destination string, files []fileHash) error {
	if info, err := os.Lstat(destination); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing symlink skill destination %s", destination)
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	for _, path := range collisionPaths(destination, files) {
		if _, err := os.Lstat(path); err == nil {
			return fmt.Errorf("refusing collision at %s; no DVA receipt exists", path)
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func hasForeignCollision(destination string, files []fileHash) bool {
	for _, path := range collisionPaths(destination, files) {
		if _, err := os.Lstat(path); err == nil {
			return true
		}
	}
	return false
}

func collisionPaths(destination string, files []fileHash) []string {
	paths := make(map[string]bool, len(files))
	for _, file := range files {
		name := file.Path
		if first, _, found := strings.Cut(name, "/"); found {
			name = first
		}
		paths[filepath.Join(destination, filepath.FromSlash(name))] = true
	}
	result := make([]string, 0, len(paths))
	for path := range paths {
		result = append(result, path)
	}
	sort.Strings(result)
	return result
}

func verifyInstalled(destination string, expected []fileHash) error {
	actual, err := installedFiles(destination, expected)
	if err != nil {
		return err
	}
	if !equalFiles(expected, actual) {
		return errors.New("installed files differ from DVA receipt")
	}
	return nil
}

func installedFiles(destination string, expected []fileHash) ([]fileHash, error) {
	if len(expected) > 0 && !strings.Contains(expected[0].Path, "/") {
		return installedFlatFiles(destination, expected)
	}
	var files []fileHash
	for _, name := range bundled.Names {
		root := filepath.Join(destination, name)
		info, err := os.Lstat(root)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("%s is not a regular skill directory", root)
		}
		err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.Type()&os.ModeSymlink != 0 {
				return fmt.Errorf("skill file %s is a symlink", path)
			}
			if entry.IsDir() {
				return nil
			}
			if !entry.Type().IsRegular() {
				return fmt.Errorf("skill file %s is not regular", path)
			}
			contents, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			relative, err := filepath.Rel(destination, path)
			if err != nil {
				return err
			}
			digest := sha256.Sum256(contents)
			files = append(files, fileHash{Path: filepath.ToSlash(relative), SHA: hex.EncodeToString(digest[:])})
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, nil
}

func installedFlatFiles(destination string, expected []fileHash) ([]fileHash, error) {
	files := make([]fileHash, 0, len(expected))
	for _, expectedFile := range expected {
		path := filepath.Join(destination, filepath.FromSlash(expectedFile.Path))
		info, err := os.Lstat(path)
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("skill file %s is not regular", path)
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		digest := sha256.Sum256(contents)
		files = append(files, fileHash{Path: expectedFile.Path, SHA: hex.EncodeToString(digest[:])})
	}
	return files, nil
}

func equalFiles(left, right []fileHash) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func sourceBundleSHA(files []fileHash) string {
	hash := sha256.New()
	for _, file := range files {
		_, _ = hash.Write([]byte(file.Path))
		_, _ = hash.Write([]byte{0})
		_, _ = hash.Write([]byte(file.SHA))
		_, _ = hash.Write([]byte{0})
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func containsRuntimes(have, wanted []Runtime) bool {
	set := make(map[Runtime]bool, len(have))
	for _, runtime := range have {
		set[runtime] = true
	}
	for _, runtime := range wanted {
		if !set[runtime] {
			return false
		}
	}
	return true
}

func unionRuntimes(left, right []Runtime) []Runtime {
	set := make(map[Runtime]bool, len(left)+len(right))
	for _, runtime := range append(append([]Runtime(nil), left...), right...) {
		set[runtime] = true
	}
	result := make([]Runtime, 0, len(set))
	for runtime := range set {
		result = append(result, runtime)
	}
	slices.Sort(result)
	return result
}

func removeRuntimes(have, removed []Runtime) []Runtime {
	remove := make(map[Runtime]bool, len(removed))
	for _, runtime := range removed {
		remove[runtime] = true
	}
	var result []Runtime
	for _, runtime := range have {
		if !remove[runtime] {
			result = append(result, runtime)
		}
	}
	slices.Sort(result)
	return result
}

func intersectRuntimes(left, right []Runtime) []Runtime {
	wanted := make(map[Runtime]bool, len(right))
	for _, runtime := range right {
		wanted[runtime] = true
	}
	var result []Runtime
	for _, runtime := range left {
		if wanted[runtime] {
			result = append(result, runtime)
		}
	}
	slices.Sort(result)
	return result
}

func replaceSkillDirectories(destination string, files []fileHash, replaceExisting bool) (func() error, func() error, error) {
	return replaceSkillDirectoriesWithRename(destination, files, replaceExisting, os.Rename)
}

func replaceBundle(destination string, bundle skillBundle, replaceExisting bool) (func() error, func() error, error) {
	if len(bundle.files) > 0 && !strings.Contains(bundle.files[0].Path, "/") {
		return replaceFlatFiles(destination, bundle, replaceExisting)
	}
	return replaceSkillDirectories(destination, bundle.files, replaceExisting)
}

func replaceFlatFiles(destination string, bundle skillBundle, replaceExisting bool) (func() error, func() error, error) {
	stage, err := os.MkdirTemp(destination, ".dva-skill-stage-")
	if err != nil {
		return nil, nil, err
	}
	for _, file := range bundle.files {
		contents := bundle.contents[file.Path]
		if err := os.WriteFile(filepath.Join(stage, filepath.FromSlash(file.Path)), contents, 0o644); err != nil {
			_ = os.RemoveAll(stage)
			return nil, nil, err
		}
	}
	type move struct{ final, backup string }
	moves := make([]move, 0, len(bundle.files))
	rollback := func() error {
		var rollbackErr error
		for index := range slices.Backward(moves) {
			if err := os.Remove(moves[index].final); err != nil && !errors.Is(err, os.ErrNotExist) && rollbackErr == nil {
				rollbackErr = err
			}
			if moves[index].backup != "" {
				if err := os.Rename(moves[index].backup, moves[index].final); err != nil && rollbackErr == nil {
					rollbackErr = err
				}
			}
		}
		if err := os.RemoveAll(stage); err != nil && rollbackErr == nil {
			rollbackErr = err
		}
		return rollbackErr
	}
	fail := func(cause error) (func() error, func() error, error) {
		if rollbackErr := rollback(); rollbackErr != nil {
			return nil, nil, fmt.Errorf("%w (rollback also failed: %v)", cause, rollbackErr)
		}
		return nil, nil, cause
	}
	for _, file := range bundle.files {
		final := filepath.Join(destination, filepath.FromSlash(file.Path))
		backup := filepath.Join(stage, filepath.FromSlash(file.Path)+".backup")
		if _, err := os.Lstat(final); err == nil {
			if !replaceExisting {
				return fail(fmt.Errorf("refusing collision at %s; no DVA receipt exists", final))
			}
			if err := os.Rename(final, backup); err != nil {
				return fail(err)
			}
			moves = append(moves, move{final: final, backup: backup})
		} else if !errors.Is(err, os.ErrNotExist) {
			return fail(err)
		} else {
			moves = append(moves, move{final: final})
		}
		if err := os.Rename(filepath.Join(stage, filepath.FromSlash(file.Path)), final); err != nil {
			return fail(err)
		}
	}
	return rollback, func() error { return os.RemoveAll(stage) }, nil
}

func replaceSkillDirectoriesWithRename(destination string, files []fileHash, replaceExisting bool, rename func(string, string) error) (func() error, func() error, error) {
	stage, err := os.MkdirTemp(destination, ".dva-skill-stage-")
	if err != nil {
		return nil, nil, err
	}
	for _, file := range files {
		if err := writeEmbedded(filepath.Join(stage, filepath.FromSlash(file.Path)), file.Path); err != nil {
			_ = os.RemoveAll(stage)
			return nil, nil, err
		}
	}
	type move struct{ final, backup string }
	moves := make([]move, 0, len(bundled.Names))
	rollback := func() error {
		var rollbackErr error
		for index := range slices.Backward(moves) {
			if err := os.RemoveAll(moves[index].final); err != nil && rollbackErr == nil {
				rollbackErr = err
			}
			if moves[index].backup != "" {
				if err := os.Rename(moves[index].backup, moves[index].final); err != nil && rollbackErr == nil {
					rollbackErr = err
				}
			}
		}
		if err := os.RemoveAll(stage); err != nil && rollbackErr == nil {
			rollbackErr = err
		}
		return rollbackErr
	}
	fail := func(cause error) (func() error, func() error, error) {
		if rollbackErr := rollback(); rollbackErr != nil {
			return nil, nil, fmt.Errorf("%w (rollback also failed: %v)", cause, rollbackErr)
		}
		return nil, nil, cause
	}
	for _, name := range bundled.Names {
		final := filepath.Join(destination, name)
		backup := filepath.Join(stage, name+".backup")
		if _, err := os.Lstat(final); err == nil {
			if !replaceExisting {
				return fail(fmt.Errorf("refusing collision at %s; no DVA receipt exists", final))
			}
			if err := rename(final, backup); err != nil {
				return fail(err)
			}
			moves = append(moves, move{final: final, backup: backup})
		} else if !errors.Is(err, os.ErrNotExist) {
			return fail(err)
		} else {
			moves = append(moves, move{final: final})
		}
		if err := rename(filepath.Join(stage, name), final); err != nil {
			return fail(err)
		}
	}
	finalize := func() error { return os.RemoveAll(stage) }
	return rollback, finalize, nil
}

func writeEmbedded(destination, embeddedPath string) error {
	contents, err := bundled.Files.ReadFile(embeddedPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	return os.WriteFile(destination, contents, 0o644)
}

func receiptPath(stateRoot, destination string) string {
	digest := sha256.Sum256([]byte(destination))
	return filepath.Join(stateRoot, "skill-installs", hex.EncodeToString(digest[:])+".json")
}

func readReceipt(path string) (receipt, bool, error) {
	contents, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return receipt{}, false, nil
	}
	if err != nil {
		return receipt{}, false, err
	}
	var record receipt
	if err := json.Unmarshal(contents, &record); err != nil {
		return receipt{}, false, err
	}
	return record, true, nil
}

func validateReceipt(record receipt, scope Scope, target destination) error {
	if (record.Schema != 1 && record.Schema != receiptSchemaCurrent) || record.Scope != scope || record.Destination != target.path {
		return fmt.Errorf("receipt does not belong to %s", target.path)
	}
	if len(record.Files) == 0 || record.Version == "" || !validSHA(record.BundleSHA) {
		return errors.New("receipt has invalid source metadata")
	}
	if !sort.SliceIsSorted(record.Files, func(i, j int) bool { return record.Files[i].Path < record.Files[j].Path }) {
		return errors.New("receipt files are not sorted")
	}
	format := receiptFormatForFiles(record.Files)
	if format == "" || format != targetReceiptFormat(target) {
		return errors.New("receipt file format does not match destination runtime")
	}
	if record.Schema == 1 {
		if format != receiptFormatNative {
			return errors.New("legacy receipt cannot describe a flat skill installation")
		}
	} else if record.Format != format {
		return errors.New("receipt format does not match its files")
	}
	for _, file := range record.Files {
		if !validSHA(file.SHA) {
			return errors.New("receipt contains an invalid file record")
		}
	}
	return nil
}

func targetReceiptFormat(target destination) string {
	if hasRuntime(target.runtimes, RuntimeAgentMesh) {
		return receiptFormatFlat
	}
	return receiptFormatNative
}

func receiptFormatForFiles(files []fileHash) string {
	if len(files) == 0 {
		return ""
	}
	flat := true
	native := true
	for _, file := range files {
		flat = flat && (file.Path == "dva.md" || file.Path == "dva-config.md")
		native = native && validNativeReceiptPath(file.Path)
	}
	switch {
	case flat && !native:
		return receiptFormatFlat
	case native && !flat:
		return receiptFormatNative
	default:
		return ""
	}
}

func validReceiptPath(value string) bool {
	return (value == "dva.md" || value == "dva-config.md") || validNativeReceiptPath(value)
}

func validNativeReceiptPath(value string) bool {
	if value == "" || strings.HasPrefix(value, "/") || strings.Contains(value, `\`) || pathpkg.Clean(value) != value {
		return false
	}
	parts := strings.Split(value, "/")
	if len(parts) < 2 || (parts[0] != "dva" && parts[0] != "dva-config") {
		return false
	}
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return true
}

func validSHA(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size
}

func writeReceipt(path string, record receipt) error {
	contents, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	contents = append(contents, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".receipt-")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	defer func() { _ = os.Remove(tempName) }()
	if err := temp.Chmod(0o600); err != nil {
		_ = temp.Close()
		return err
	}
	if _, err := temp.Write(contents); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempName, path)
}
