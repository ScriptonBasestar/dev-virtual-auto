// Package skillinstall installs the DVA-owned Agent Skills without requiring an AI runtime.
package skillinstall

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	pathpkg "path"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"syscall"

	"github.com/ScriptonBasestar/dva/internal/skillclaim"
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
	Scope                 Scope
	Runtimes              []Runtime
	HomeDir               string
	ProjectRoot           string
	StateRoot             string
	ClaimRoot             string
	DryRun                bool
	Takeover              bool
	RestoreTakeoverBackup bool
	Version               string
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
	TakeoverBackup     string          `json:"takeover_backup,omitempty"`
	BackupStatus       string          `json:"backup_status,omitempty"`
}

type RuntimeStatus struct {
	Runtime Runtime `json:"runtime"`
	Status  string  `json:"status"`
}

type receipt struct {
	Schema       int              `json:"schema"`
	Installation string           `json:"installation,omitempty"`
	Format       string           `json:"format,omitempty"`
	Scope        Scope            `json:"scope"`
	Destination  string           `json:"destination"`
	Runtimes     []Runtime        `json:"runtimes"`
	Version      string           `json:"version"`
	BundleSHA    string           `json:"bundle_sha256"`
	Files        []fileHash       `json:"files"`
	Takeovers    []takeoverBackup `json:"takeovers,omitempty"`
}

const (
	receiptSchemaCurrent = 3
	receiptFormatNative  = "agent-skills-directory"
	receiptFormatFlat    = "agent-mesh-flat-markdown"
)

type takeoverBackup struct {
	Skill          string        `json:"skill"`
	BackupID       string        `json:"backup_id"`
	Kind           string        `json:"kind"`
	ManifestDigest string        `json:"manifest_digest"`
	Entries        []backupEntry `json:"entries"`
}

type backupEntry struct {
	Path string `json:"path"`
	Kind string `json:"kind"`
	Mode uint32 `json:"mode"`
	SHA  string `json:"sha256,omitempty"`
}

type fileHash struct {
	Path string `json:"path"`
	SHA  string `json:"sha256"`
}

type destination struct {
	path     string
	runtimes []Runtime
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
	// Inspect every destination before the first mutation. Claim locks repeat these
	// checks at the mutation boundary, but this pass avoids predictable partial work.
	for _, target := range destinations {
		if err := preflightInstall(resolved, target); err != nil {
			return Result{}, err
		}
	}
	result := Result{Scope: resolved.Scope, Destinations: make([]DestinationResult, 0, len(destinations))}
	for _, target := range destinations {
		entry, err := installDestination(resolved, target)
		if err != nil {
			return Result{}, err
		}
		result.Destinations = append(result.Destinations, entry)
	}
	return result, nil
}

func preflightInstall(options Options, target destination) error {
	bundle, err := bundleFor(target)
	if err != nil {
		return err
	}
	record, found, err := readReceipt(receiptPath(options.StateRoot, target.path))
	if err != nil {
		return fmt.Errorf("read receipt for %s: %w", target.path, err)
	}
	operationID := "preflight"
	projection, err := projectedClaims(target, options.Scope, target.runtimes, bundle, skillclaim.StateActive, operationID)
	if err != nil {
		return err
	}
	if !found || (record.Schema == receiptSchemaCurrent && record.Installation == "absent") {
		if err := ensureClaimsAbsentUnlocked(options.ClaimRoot, projection); err != nil {
			return err
		}
		if found && hasForeignCollision(target.path, bundle.files) {
			return fmt.Errorf("refusing collision at %s while a takeover backup is retained", target.path)
		}
		if !found {
			if err := ensureNoCollision(target.path, bundle.files); err != nil && !options.Takeover {
				return err
			}
			if options.Takeover {
				return validateTakeover(target, bundle)
			}
		}
		return nil
	}
	if err := validateReceipt(record, options.Scope, target); err != nil {
		return err
	}
	if err := verifyInstalled(target.path, record.Files); err != nil {
		return fmt.Errorf("refusing to update drifted DVA skill installation at %s: %w", target.path, err)
	}
	oldProjection, err := projectedClaims(target, options.Scope, record.Runtimes, skillBundle{files: record.Files}, skillclaim.StateActive, operationID)
	if err != nil {
		return err
	}
	if record.Schema < receiptSchemaCurrent {
		return ensureClaimsAbsentUnlocked(options.ClaimRoot, oldProjection)
	}
	return verifyClaimsUnlocked(options.ClaimRoot, oldProjection)
}

func installDestination(options Options, target destination) (DestinationResult, error) {
	bundle, err := bundleFor(target)
	if err != nil {
		return DestinationResult{}, err
	}
	entry := resultEntry(target, options.Version, sourceBundleSHA(bundle.files))
	record, found, err := readReceipt(receiptPath(options.StateRoot, target.path))
	if err != nil {
		return DestinationResult{}, err
	}
	if options.DryRun {
		entry.Status = "would-install"
		setAllRuntimeStatuses(&entry, "would-install")
		if !found && options.Takeover && hasForeignCollision(target.path, bundle.files) {
			entry.Detail = "would back up foreign DVA-name skill before takeover"
			entry.BackupStatus = "would-backup"
		}
		return entry, nil
	}
	if err := ensureDestination(target.path); err != nil {
		return DestinationResult{}, err
	}
	destinations, err := claimDestinations(target, bundle)
	if err != nil {
		return DestinationResult{}, err
	}
	store, err := skillclaim.Begin(options.ClaimRoot, destinations)
	if err != nil {
		return DestinationResult{}, err
	}
	defer func() { _ = store.Close() }()

	// Repeat receipt and file checks while the per-skill claim locks are held.
	record, found, err = readReceipt(receiptPath(options.StateRoot, target.path))
	if err != nil {
		return DestinationResult{}, err
	}
	if found && record.Schema == receiptSchemaCurrent && record.Installation == "active" && equalFiles(record.Files, bundle.files) && containsRuntimes(record.Runtimes, target.runtimes) {
		if err := verifyInstalled(target.path, record.Files); err != nil {
			return DestinationResult{}, err
		}
		projection, err := projectedClaims(target, options.Scope, record.Runtimes, bundle, skillclaim.StateActive, "up-to-date-check")
		if err != nil {
			return DestinationResult{}, err
		}
		current, err := readLockedClaims(store, projection)
		if err != nil {
			return DestinationResult{}, err
		}
		if err := verifyActiveClaims(current, projection); err != nil {
			return DestinationResult{}, err
		}
		entry.Status = "up-to-date"
		setAllRuntimeStatuses(&entry, "up-to-date")
		if len(record.Takeovers) > 0 {
			entry.BackupStatus, entry.TakeoverBackup = verifyTakeoverBackups(options.StateRoot, record)
		}
		return entry, nil
	}
	if !found || record.Schema < receiptSchemaCurrent || record.Installation == "absent" {
		return installWithReservations(options, target, bundle, store, record, found, entry)
	}
	return updateClaimedInstall(options, target, bundle, store, record, entry)
}

func installWithReservations(options Options, target destination, bundle skillBundle, store *skillclaim.LockedStore, previousReceipt receipt, receiptFound bool, entry DestinationResult) (DestinationResult, error) {
	if receiptFound {
		if err := validateReceipt(previousReceipt, options.Scope, target); err != nil {
			return DestinationResult{}, err
		}
		if previousReceipt.Schema < receiptSchemaCurrent {
			if err := verifyInstalled(target.path, previousReceipt.Files); err != nil {
				return DestinationResult{}, err
			}
		} else if hasForeignCollision(target.path, bundle.files) {
			return DestinationResult{}, errors.New("refusing reinstall because a backup-only destination is no longer empty")
		}
	}
	operationID, err := newClaimOperationID()
	if err != nil {
		return DestinationResult{}, err
	}
	runtimes := append([]Runtime(nil), target.runtimes...)
	if receiptFound && previousReceipt.Installation != "absent" {
		runtimes = unionRuntimes(previousReceipt.Runtimes, runtimes)
	}
	claims, err := projectedClaims(target, options.Scope, runtimes, bundle, skillclaim.StateReserved, operationID)
	if err != nil {
		return DestinationResult{}, err
	}
	if err := ensureClaimsAbsent(store, claims); err != nil {
		return DestinationResult{}, err
	}
	if !receiptFound {
		if err := ensureNoCollision(target.path, bundle.files); err != nil && !options.Takeover {
			return DestinationResult{}, err
		}
		if options.Takeover {
			if err := validateTakeover(target, bundle); err != nil {
				return DestinationResult{}, err
			}
		}
	}
	reserved, err := reserveClaims(store, claims)
	if err != nil {
		_ = rollbackClaimsToAbsent(store, reserved)
		return DestinationResult{}, err
	}
	takeovers := append([]takeoverBackup(nil), previousReceipt.Takeovers...)
	rollbackTakeover := func() error { return nil }
	finalizeTakeover := func() error { return nil }
	cleanupTakeover := func() error { return nil }
	takeoverRecovery := ""
	if !receiptFound && options.Takeover {
		takeovers, rollbackTakeover, finalizeTakeover, cleanupTakeover, takeoverRecovery, err = createTakeoverBackups(options.StateRoot, target, bundle)
		if err != nil {
			_ = rollbackClaimsToAbsent(store, reserved)
			return DestinationResult{}, err
		}
	}
	replaceExisting := receiptFound && previousReceipt.Installation != "absent"
	undo, finalize, err := replaceBundle(target.path, bundle, replaceExisting)
	if err != nil {
		originalErr := rollbackTakeover()
		cleanupErr := error(nil)
		if originalErr == nil {
			cleanupErr = cleanupTakeover()
		}
		claimErr := rollbackClaimsToAbsent(store, reserved)
		return DestinationResult{}, fmt.Errorf("install captured takeover: %w (original rollback: %v; backup cleanup: %v; claim rollback: %v; recovery artifact: %s)", err, originalErr, cleanupErr, claimErr, takeoverRecovery)
	}
	newReceipt := receipt{
		Schema: receiptSchemaCurrent, Installation: "active", Format: targetReceiptFormat(target),
		Scope: options.Scope, Destination: target.path, Runtimes: runtimes, Version: options.Version,
		BundleSHA: sourceBundleSHA(bundle.files), Files: bundle.files, Takeovers: takeovers,
	}
	receiptFile := receiptPath(options.StateRoot, target.path)
	if err := writeReceipt(receiptFile, newReceipt); err != nil {
		rollbackErr := undo()
		originalErr := error(nil)
		cleanupErr := error(nil)
		if rollbackErr == nil {
			originalErr = rollbackTakeover()
		}
		if rollbackErr == nil && originalErr == nil {
			cleanupErr = cleanupTakeover()
		}
		claimErr := rollbackClaimsToAbsent(store, reserved)
		return DestinationResult{}, fmt.Errorf("write receipt: %w (file rollback: %v; original rollback: %v; backup cleanup: %v; claim rollback: %v; recovery artifact: %s)", err, rollbackErr, originalErr, cleanupErr, claimErr, takeoverRecovery)
	}
	if _, err := activateReservedClaims(store, reserved); err != nil {
		rollbackErr := undo()
		originalErr := error(nil)
		cleanupErr := error(nil)
		if rollbackErr == nil {
			originalErr = rollbackTakeover()
		}
		if rollbackErr == nil && originalErr == nil {
			cleanupErr = cleanupTakeover()
		}
		receiptErr := restoreReceipt(receiptFile, previousReceipt, receiptFound)
		claimErr := rollbackClaimsToAbsent(store, claims)
		return DestinationResult{}, fmt.Errorf("activate DVA claims: %w (file rollback: %v; original rollback: %v; receipt rollback: %v; backup cleanup: %v; claim rollback: %v; recovery artifact: %s)", err, rollbackErr, originalErr, receiptErr, cleanupErr, claimErr, takeoverRecovery)
	}
	if err := finalize(); err != nil {
		entry.Detail = fmt.Sprintf("installed; cleanup retained a temporary artifact: %v", err)
	}
	if err := finalizeTakeover(); err != nil {
		entry.Detail = fmt.Sprintf("installed; cleanup retained captured originals at %s: %v", takeoverRecovery, err)
	}
	entry.Status = "installed"
	setAllRuntimeStatuses(&entry, "installed")
	if len(takeovers) > 0 {
		entry.BackupStatus, entry.TakeoverBackup = verifyTakeoverBackups(options.StateRoot, newReceipt)
	}
	return entry, nil
}

func updateClaimedInstall(options Options, target destination, bundle skillBundle, store *skillclaim.LockedStore, record receipt, entry DestinationResult) (DestinationResult, error) {
	if err := validateReceipt(record, options.Scope, target); err != nil {
		return DestinationResult{}, err
	}
	if record.Installation != "active" {
		return DestinationResult{}, errors.New("recovery-required: claimed receipt is not active")
	}
	if err := verifyInstalled(target.path, record.Files); err != nil {
		return DestinationResult{}, err
	}
	operationID, err := newClaimOperationID()
	if err != nil {
		return DestinationResult{}, err
	}
	oldProjection, err := projectedClaims(target, options.Scope, record.Runtimes, skillBundle{files: record.Files}, skillclaim.StateActive, operationID)
	if err != nil {
		return DestinationResult{}, err
	}
	current, err := readLockedClaims(store, oldProjection)
	if err != nil {
		return DestinationResult{}, err
	}
	if err := verifyActiveClaims(current, oldProjection); err != nil {
		return DestinationResult{}, err
	}
	runtimes := unionRuntimes(record.Runtimes, target.runtimes)
	desired, err := projectedClaims(target, options.Scope, runtimes, bundle, skillclaim.StateActive, operationID)
	if err != nil {
		return DestinationResult{}, err
	}
	updating, err := transitionActiveClaims(store, current, desired, skillclaim.StateUpdating, operationID)
	if err != nil {
		return DestinationResult{}, err
	}
	undo, finalize := func() error { return nil }, func() error { return nil }
	if !equalFiles(record.Files, bundle.files) {
		undo, finalize, err = replaceBundle(target.path, bundle, true)
		if err != nil {
			claimErr := rollbackClaimsToActive(store, current)
			return DestinationResult{}, fmt.Errorf("replace managed skills after claim reservation: %w (claim rollback: %v)", err, claimErr)
		}
	}
	updated := receipt{
		Schema: receiptSchemaCurrent, Installation: "active", Format: targetReceiptFormat(target),
		Scope: options.Scope, Destination: target.path, Runtimes: runtimes, Version: options.Version,
		BundleSHA: sourceBundleSHA(bundle.files), Files: bundle.files, Takeovers: record.Takeovers,
	}
	receiptFile := receiptPath(options.StateRoot, target.path)
	if err := writeReceipt(receiptFile, updated); err != nil {
		fileErr := undo()
		claimErr := rollbackClaimsToActive(store, current)
		return DestinationResult{}, fmt.Errorf("update receipt: %w (file rollback: %v; claim rollback: %v)", err, fileErr, claimErr)
	}
	if err := activateUpdatedClaims(store, updating); err != nil {
		fileErr := undo()
		receiptErr := writeReceipt(receiptFile, record)
		claimErr := rollbackClaimsToActive(store, current)
		return DestinationResult{}, fmt.Errorf("activate updated claims: %w (file rollback: %v; receipt rollback: %v; claim rollback: %v)", err, fileErr, receiptErr, claimErr)
	}
	if err := finalize(); err != nil {
		return DestinationResult{}, err
	}
	entry.Status = "installed"
	setAllRuntimeStatuses(&entry, "installed")
	if len(updated.Takeovers) > 0 {
		entry.BackupStatus, entry.TakeoverBackup = verifyTakeoverBackups(options.StateRoot, updated)
	}
	return entry, nil
}

func restoreReceipt(path string, previous receipt, found bool) error {
	if found {
		return writeReceipt(path, previous)
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
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
			result.Destinations = append(result.Destinations, entry)
			continue
		}
		if found {
			if err := validateReceipt(record, resolved.Scope, target); err != nil {
				entry.Status, entry.Detail = "invalid-receipt", err.Error()
				setAllRuntimeStatuses(&entry, "invalid-receipt")
				result.Destinations = append(result.Destinations, entry)
				continue
			}
		}
		projectionRuntimes := target.runtimes
		projectionFiles := bundle.files
		if found {
			projectionRuntimes = record.Runtimes
			projectionFiles = record.Files
			if record.Schema == receiptSchemaCurrent && record.Installation == "absent" {
				projectionRuntimes = target.runtimes
			}
		}
		projection, projectionErr := projectedClaims(target, resolved.Scope, projectionRuntimes, skillBundle{files: projectionFiles}, skillclaim.StateActive, "status")
		if projectionErr != nil {
			entry.Status, entry.Detail = "recovery-required", projectionErr.Error()
			setAllRuntimeStatuses(&entry, entry.Status)
			result.Destinations = append(result.Destinations, entry)
			continue
		}
		if !found {
			if err := ensureClaimsAbsentUnlocked(resolved.ClaimRoot, projection); err != nil {
				entry.Status, entry.Detail = "recovery-required", err.Error()
			} else if hasForeignCollision(target.path, bundle.files) {
				entry.Status = "foreign-conflict"
			} else {
				entry.Status = "absent"
			}
			setAllRuntimeStatuses(&entry, entry.Status)
		} else if err := validateReceipt(record, resolved.Scope, target); err != nil {
			entry.Status, entry.Detail = "invalid-receipt", err.Error()
			setAllRuntimeStatuses(&entry, "invalid-receipt")
		} else if record.Schema < receiptSchemaCurrent {
			entry.InstalledVersion, entry.InstalledBundleSHA = record.Version, record.BundleSHA
			if err := verifyInstalled(target.path, record.Files); err != nil {
				entry.Status, entry.Detail = "drifted", err.Error()
			} else if err := ensureClaimsAbsentUnlocked(resolved.ClaimRoot, projection); err != nil {
				entry.Status, entry.Detail = "recovery-required", err.Error()
			} else {
				entry.Status = "legacy-unclaimed"
			}
			setAllRuntimeStatuses(&entry, entry.Status)
		} else if record.Installation == "absent" {
			entry.BackupStatus, entry.TakeoverBackup = verifyTakeoverBackups(resolved.StateRoot, record)
			if err := ensureClaimsAbsentUnlocked(resolved.ClaimRoot, projection); err != nil {
				entry.Status, entry.Detail = "recovery-required", err.Error()
			} else if hasForeignCollision(target.path, record.Files) {
				entry.Status, entry.Detail = "recovery-required", "backup-only destination contains a DVA-name collision"
			} else {
				entry.Status = "backup-only"
			}
			if entry.BackupStatus == "corrupt" {
				entry.Detail = "takeover backup is missing or differs from its receipt"
			}
			setAllRuntimeStatuses(&entry, "absent")
		} else if err := verifyInstalled(target.path, record.Files); err != nil {
			entry.InstalledVersion, entry.InstalledBundleSHA = record.Version, record.BundleSHA
			entry.Status, entry.Detail = "drifted", err.Error()
			setAllRuntimeStatuses(&entry, "drifted")
		} else if err := verifyClaimsUnlocked(resolved.ClaimRoot, projection); err != nil {
			entry.InstalledVersion, entry.InstalledBundleSHA = record.Version, record.BundleSHA
			entry.Status, entry.Detail = "recovery-required", err.Error()
			setAllRuntimeStatuses(&entry, entry.Status)
		} else {
			entry.InstalledVersion, entry.InstalledBundleSHA = record.Version, record.BundleSHA
			entry.Status = setMembershipStatuses(&entry, record.Runtimes, "installed", "absent")
			if len(record.Takeovers) > 0 {
				entry.BackupStatus, entry.TakeoverBackup = verifyTakeoverBackups(resolved.StateRoot, record)
				if entry.BackupStatus == "corrupt" {
					entry.Detail = "takeover backup is missing or differs from its receipt"
				}
			}
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
	for _, target := range destinations {
		if err := preflightUninstall(resolved, target); err != nil {
			return Result{}, err
		}
	}
	result := Result{Scope: resolved.Scope, Destinations: make([]DestinationResult, 0, len(destinations))}
	for _, target := range destinations {
		entry, err := uninstallDestination(resolved, target)
		if err != nil {
			return Result{}, err
		}
		result.Destinations = append(result.Destinations, entry)
	}
	return result, nil
}

func preflightUninstall(options Options, target destination) error {
	record, found, err := readReceipt(receiptPath(options.StateRoot, target.path))
	if err != nil {
		return err
	}
	if !found {
		bundle, err := bundleFor(target)
		if err != nil {
			return err
		}
		projection, err := projectedClaims(target, options.Scope, target.runtimes, bundle, skillclaim.StateActive, "preflight")
		if err != nil {
			return err
		}
		return ensureClaimsAbsentUnlocked(options.ClaimRoot, projection)
	}
	if err := validateReceipt(record, options.Scope, target); err != nil {
		return err
	}
	if options.RestoreTakeoverBackup {
		if len(record.Takeovers) == 0 {
			return fmt.Errorf("no takeover backup exists for %s", target.path)
		}
		if status, _ := verifyTakeoverBackups(options.StateRoot, record); status != "available" {
			return fmt.Errorf("takeover backup for %s is %s", target.path, status)
		}
		if record.Installation == "active" && len(removeRuntimes(record.Runtimes, target.runtimes)) > 0 {
			return errors.New("restore requires selecting every consumer of the shared skill destination")
		}
	}
	if record.Schema == receiptSchemaCurrent && record.Installation == "absent" {
		bundle := skillBundle{files: record.Files}
		projection, err := projectedClaims(target, options.Scope, target.runtimes, bundle, skillclaim.StateActive, "preflight")
		if err != nil {
			return err
		}
		if err := ensureClaimsAbsentUnlocked(options.ClaimRoot, projection); err != nil {
			return err
		}
		if hasForeignCollision(target.path, record.Files) {
			return errors.New("backup-only destination contains a DVA-name collision")
		}
		return nil
	}
	if err := verifyInstalled(target.path, record.Files); err != nil {
		return fmt.Errorf("refusing to uninstall drifted DVA skill installation at %s: %w", target.path, err)
	}
	projection, err := projectedClaims(target, options.Scope, record.Runtimes, skillBundle{files: record.Files}, skillclaim.StateActive, "preflight")
	if err != nil {
		return err
	}
	if record.Schema < receiptSchemaCurrent {
		return ensureClaimsAbsentUnlocked(options.ClaimRoot, projection)
	}
	return verifyClaimsUnlocked(options.ClaimRoot, projection)
}

func uninstallDestination(options Options, target destination) (DestinationResult, error) {
	entry := resultEntry(target, "", "")
	receiptFile := receiptPath(options.StateRoot, target.path)
	record, found, err := readReceipt(receiptFile)
	if err != nil {
		return DestinationResult{}, err
	}
	if !found {
		bundle, bundleErr := bundleFor(target)
		if bundleErr != nil {
			return DestinationResult{}, bundleErr
		}
		if options.DryRun {
			entry.Status = "not-installed"
			setAllRuntimeStatuses(&entry, entry.Status)
			return entry, nil
		}
		claimPaths, claimErr := claimDestinations(target, bundle)
		if claimErr != nil {
			return DestinationResult{}, claimErr
		}
		store, claimErr := skillclaim.Begin(options.ClaimRoot, claimPaths)
		if claimErr != nil {
			return DestinationResult{}, claimErr
		}
		defer func() { _ = store.Close() }()
		projection, claimErr := projectedClaims(target, options.Scope, target.runtimes, bundle, skillclaim.StateActive, "uninstall-check")
		if claimErr != nil {
			return DestinationResult{}, claimErr
		}
		if claimErr := ensureClaimsAbsent(store, projection); claimErr != nil {
			return DestinationResult{}, claimErr
		}
		entry.Status = "not-installed"
		setAllRuntimeStatuses(&entry, entry.Status)
		return entry, nil
	}
	if err := validateReceipt(record, options.Scope, target); err != nil {
		return DestinationResult{}, err
	}
	if options.DryRun {
		if options.RestoreTakeoverBackup {
			entry.Status, entry.BackupStatus = "would-restore-takeover", "would-restore"
		} else if record.Installation == "absent" {
			entry.Status = "not-installed"
		} else {
			entry.Status = "would-uninstall"
		}
		setAllRuntimeStatuses(&entry, entry.Status)
		return entry, nil
	}
	if err := ensureDestination(target.path); err != nil {
		return DestinationResult{}, err
	}
	bundle := skillBundle{files: record.Files}
	destinations, err := claimDestinations(target, bundle)
	if err != nil {
		return DestinationResult{}, err
	}
	store, err := skillclaim.Begin(options.ClaimRoot, destinations)
	if err != nil {
		return DestinationResult{}, err
	}
	defer func() { _ = store.Close() }()
	record, found, err = readReceipt(receiptFile)
	if err != nil || !found {
		return DestinationResult{}, fmt.Errorf("receipt changed before uninstall: %w", err)
	}
	if record.Installation == "absent" {
		if !options.RestoreTakeoverBackup {
			entry.Status = "not-installed"
			setAllRuntimeStatuses(&entry, entry.Status)
			return entry, nil
		}
		return restoreBackupOnly(options, target, record, store, entry)
	}
	if err := verifyInstalled(target.path, record.Files); err != nil {
		return DestinationResult{}, err
	}
	operationID, err := newClaimOperationID()
	if err != nil {
		return DestinationResult{}, err
	}
	projection, err := projectedClaims(target, options.Scope, record.Runtimes, bundle, skillclaim.StateActive, operationID)
	if err != nil {
		return DestinationResult{}, err
	}
	installedRequested := intersectRuntimes(record.Runtimes, target.runtimes)
	if len(installedRequested) == 0 && record.Schema < receiptSchemaCurrent {
		entry.Status = "not-installed"
		setAllRuntimeStatuses(&entry, entry.Status)
		return entry, nil
	}
	var current []skillclaim.Claim
	if record.Schema < receiptSchemaCurrent {
		if err := ensureClaimsAbsent(store, projection); err != nil {
			return DestinationResult{}, err
		}
		reserved, err := reserveClaims(store, projection)
		if err != nil {
			_ = rollbackClaimsToAbsent(store, reserved)
			return DestinationResult{}, err
		}
		current, err = activateReservedClaims(store, reserved)
		if err != nil {
			return DestinationResult{}, fmt.Errorf("migrate legacy receipt claims: %w; recovery-required", err)
		}
	} else {
		current, err = readLockedClaims(store, projection)
		if err != nil {
			return DestinationResult{}, err
		}
		if err := verifyActiveClaims(current, projection); err != nil {
			return DestinationResult{}, err
		}
	}
	if len(installedRequested) == 0 {
		entry.Status = "not-installed"
		setAllRuntimeStatuses(&entry, entry.Status)
		return entry, nil
	}
	remaining := removeRuntimes(record.Runtimes, target.runtimes)
	if options.RestoreTakeoverBackup && len(remaining) > 0 {
		return DestinationResult{}, errors.New("restore requires selecting every consumer")
	}
	if len(remaining) > 0 {
		return unlinkConsumers(options, target, record, store, current, remaining, operationID, entry)
	}
	if options.RestoreTakeoverBackup {
		return restoreActiveTakeover(options, target, record, store, current, operationID, entry)
	}
	return removeLastConsumer(options, target, record, store, current, operationID, entry)
}

func unlinkConsumers(options Options, target destination, record receipt, store *skillclaim.LockedStore, current []skillclaim.Claim, remaining []Runtime, operationID string, entry DestinationResult) (DestinationResult, error) {
	desired, err := projectedClaims(target, options.Scope, remaining, skillBundle{files: record.Files}, skillclaim.StateActive, operationID)
	if err != nil {
		return DestinationResult{}, err
	}
	updating, err := transitionActiveClaims(store, current, desired, skillclaim.StateUpdating, operationID)
	if err != nil {
		return DestinationResult{}, err
	}
	updated := record
	updated.Schema = receiptSchemaCurrent
	updated.Installation = "active"
	updated.Format = targetReceiptFormat(target)
	updated.Runtimes = remaining
	if err := writeReceipt(receiptPath(options.StateRoot, target.path), updated); err != nil {
		claimErr := rollbackClaimsToActive(store, current)
		return DestinationResult{}, fmt.Errorf("update receipt: %w (claim rollback: %v)", err, claimErr)
	}
	if err := activateUpdatedClaims(store, updating); err != nil {
		receiptErr := writeReceipt(receiptPath(options.StateRoot, target.path), record)
		claimErr := rollbackClaimsToActive(store, current)
		return DestinationResult{}, fmt.Errorf("activate consumer update: %w (receipt rollback: %v; claim rollback: %v)", err, receiptErr, claimErr)
	}
	entry.Status = "unlinked"
	setMembershipStatuses(&entry, intersectRuntimes(record.Runtimes, target.runtimes), "unlinked", "not-installed")
	return entry, nil
}

func removeLastConsumer(options Options, target destination, record receipt, store *skillclaim.LockedStore, current []skillclaim.Claim, operationID string, entry DestinationResult) (DestinationResult, error) {
	releasing, err := transitionActiveClaims(store, current, current, skillclaim.StateReleasing, operationID)
	if err != nil {
		return DestinationResult{}, err
	}
	rollbackFiles, finalizeFiles, recoveryStage, err := stageManagedRemoval(target.path, record.Files)
	if err != nil {
		claimErr := rollbackClaimsToActive(store, current)
		return DestinationResult{}, fmt.Errorf("stage DVA skill removal: %w (claim rollback: %v)", err, claimErr)
	}
	if err := removeTransitionedClaims(store, releasing); err != nil {
		fileErr := rollbackFiles()
		claimErr := rollbackClaimsToActive(store, current)
		if fileErr != nil || claimErr != nil {
			return DestinationResult{}, fmt.Errorf("remove DVA claim: %w (file rollback: %v; claim rollback: %v; recovery stage: %s)", err, fileErr, claimErr, recoveryStage)
		}
		return DestinationResult{}, fmt.Errorf("remove DVA claim: %w", err)
	}
	receiptFile := receiptPath(options.StateRoot, target.path)
	if len(record.Takeovers) > 0 {
		tombstone := record
		tombstone.Schema = receiptSchemaCurrent
		tombstone.Installation = "absent"
		tombstone.Runtimes = nil
		if err := writeReceipt(receiptFile, tombstone); err != nil {
			fileErr := rollbackFiles()
			claimErr := rollbackClaimsToActive(store, current)
			return DestinationResult{}, fmt.Errorf("write backup-only receipt: %w (file rollback: %v; claim rollback: %v; recovery stage: %s)", err, fileErr, claimErr, recoveryStage)
		}
	} else if err := os.Remove(receiptFile); err != nil && !errors.Is(err, os.ErrNotExist) {
		fileErr := rollbackFiles()
		claimErr := rollbackClaimsToActive(store, current)
		return DestinationResult{}, fmt.Errorf("remove receipt: %w (file rollback: %v; claim rollback: %v; recovery stage: %s)", err, fileErr, claimErr, recoveryStage)
	}
	if err := finalizeFiles(); err != nil {
		entry.Detail = fmt.Sprintf("uninstalled; cleanup retained a recovery artifact at %s: %v", recoveryStage, err)
	}
	if hasRuntime(target.runtimes, RuntimeAgentMesh) {
		if err := os.Remove(target.path); err != nil && !errors.Is(err, os.ErrNotExist) && !errors.Is(err, syscall.ENOTEMPTY) {
			return DestinationResult{}, err
		}
	}
	entry.Status = "uninstalled"
	setMembershipStatuses(&entry, intersectRuntimes(record.Runtimes, target.runtimes), "uninstalled", "not-installed")
	if len(record.Takeovers) > 0 {
		entry.BackupStatus, entry.TakeoverBackup = verifyTakeoverBackups(options.StateRoot, receipt{Destination: record.Destination, Takeovers: record.Takeovers})
	}
	return entry, nil
}

func restoreActiveTakeover(options Options, target destination, record receipt, store *skillclaim.LockedStore, current []skillclaim.Claim, operationID string, entry DestinationResult) (DestinationResult, error) {
	if status, _ := verifyTakeoverBackups(options.StateRoot, record); status != "available" {
		return DestinationResult{}, fmt.Errorf("takeover backup is %s", status)
	}
	restoring, err := transitionActiveClaims(store, current, current, skillclaim.StateRestoring, operationID)
	if err != nil {
		return DestinationResult{}, err
	}
	rollbackRestore, finalizeRestore, err := replaceWithTakeoverBackups(options.StateRoot, record)
	if err != nil {
		claimErr := rollbackClaimsToActive(store, current)
		return DestinationResult{}, fmt.Errorf("restore takeover backup: %w (claim rollback: %v)", err, claimErr)
	}
	if err := removeTransitionedClaims(store, restoring); err != nil {
		fileErr := rollbackRestore()
		claimErr := rollbackClaimsToActive(store, current)
		return DestinationResult{}, fmt.Errorf("release restored claims: %w (file rollback: %v; claim rollback: %v)", err, fileErr, claimErr)
	}
	if err := os.Remove(receiptPath(options.StateRoot, target.path)); err != nil && !errors.Is(err, os.ErrNotExist) {
		fileErr := rollbackRestore()
		claimErr := rollbackClaimsToActive(store, current)
		return DestinationResult{}, fmt.Errorf("remove restored receipt: %w (file rollback: %v; claim rollback: %v)", err, fileErr, claimErr)
	}
	if err := finalizeRestore(); err != nil {
		entry.Detail = fmt.Sprintf("restored; cleanup retained a recovery artifact: %v", err)
	}
	entry.Status, entry.BackupStatus = "restored-takeover", "restored"
	setAllRuntimeStatuses(&entry, "restored-takeover")
	return entry, nil
}

func restoreBackupOnly(options Options, target destination, record receipt, store *skillclaim.LockedStore, entry DestinationResult) (DestinationResult, error) {
	if status, _ := verifyTakeoverBackups(options.StateRoot, record); status != "available" {
		return DestinationResult{}, fmt.Errorf("takeover backup is %s", status)
	}
	if hasForeignCollision(target.path, record.Files) {
		return DestinationResult{}, errors.New("refusing restore because a DVA-name destination reappeared")
	}
	operationID, err := newClaimOperationID()
	if err != nil {
		return DestinationResult{}, err
	}
	projection, err := projectedClaims(target, options.Scope, target.runtimes, skillBundle{files: record.Files}, skillclaim.StateReserved, operationID)
	if err != nil {
		return DestinationResult{}, err
	}
	if err := ensureClaimsAbsent(store, projection); err != nil {
		return DestinationResult{}, err
	}
	reserved, err := reserveClaims(store, projection)
	if err != nil {
		_ = rollbackClaimsToAbsent(store, reserved)
		return DestinationResult{}, err
	}
	active, err := activateReservedClaims(store, reserved)
	if err != nil {
		return DestinationResult{}, fmt.Errorf("reserve restore claims: %w; recovery-required", err)
	}
	restoring, err := transitionActiveClaims(store, active, active, skillclaim.StateRestoring, operationID+"-restore")
	if err != nil {
		return DestinationResult{}, err
	}
	rollbackRestore, finalizeRestore, err := replaceWithTakeoverBackups(options.StateRoot, record)
	if err != nil {
		claimErr := rollbackClaimsToAbsent(store, restoring)
		return DestinationResult{}, fmt.Errorf("restore takeover backup: %w (claim rollback: %v)", err, claimErr)
	}
	if err := removeTransitionedClaims(store, restoring); err != nil {
		fileErr := rollbackRestore()
		claimErr := rollbackClaimsToAbsent(store, restoring)
		return DestinationResult{}, fmt.Errorf("release restore claims: %w (file rollback: %v; claim rollback: %v)", err, fileErr, claimErr)
	}
	if err := os.Remove(receiptPath(options.StateRoot, target.path)); err != nil && !errors.Is(err, os.ErrNotExist) {
		fileErr := rollbackRestore()
		claimErr := rollbackClaimsToAbsent(store, restoring)
		return DestinationResult{}, fmt.Errorf("remove backup-only receipt: %w (file rollback: %v; claim rollback: %v)", err, fileErr, claimErr)
	}
	if err := finalizeRestore(); err != nil {
		entry.Detail = fmt.Sprintf("restored; cleanup retained a recovery artifact: %v", err)
	}
	entry.Status, entry.BackupStatus = "restored-takeover", "restored"
	setAllRuntimeStatuses(&entry, "restored-takeover")
	return entry, nil
}

func resolve(options Options) (Options, []destination, error) {
	if options.Scope != ScopeUser && options.Scope != ScopeProject {

		return Options{}, nil, fmt.Errorf("skill install scope must be %q or %q", ScopeUser, ScopeProject)
	}
	if (options.Takeover || options.RestoreTakeoverBackup) && len(options.Runtimes) == 0 {
		return Options{}, nil, errors.New("--takeover and --restore-takeover-backup require at least one explicit --runtime")
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
	if options.ClaimRoot == "" {
		options.ClaimRoot = filepath.Dir(state)
	}
	claimRoot, err := filepath.Abs(options.ClaimRoot)
	if err != nil {
		return Options{}, nil, err
	}
	options.ClaimRoot = claimRoot
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

func skillNames(bundle skillBundle) []string {
	if len(bundle.files) > 0 && !strings.Contains(bundle.files[0].Path, "/") {
		return []string{"dva.md", "dva-config.md"}
	}
	return append([]string(nil), bundled.Names...)
}

func claimDestination(target destination, name string) string {
	return filepath.Join(target.path, name)
}

func validateTakeover(target destination, bundle skillBundle) error {
	for _, name := range skillNames(bundle) {
		path := claimDestination(target, name)
		info, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || (!info.Mode().IsRegular() && !info.IsDir()) {
			return fmt.Errorf("refusing takeover of symlink or special skill %s", path)
		}
		if info.IsDir() {
			err := filepath.WalkDir(path, func(p string, entry fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if entry.Type()&os.ModeSymlink != 0 || (!entry.IsDir() && !entry.Type().IsRegular()) {
					return fmt.Errorf("refusing takeover of symlink or special skill %s", p)
				}
				return nil
			})
			if err != nil {
				return err
			}
		}
	}
	return nil
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
		if err := writeBytesSynced(filepath.Join(stage, filepath.FromSlash(file.Path)), contents, 0o644); err != nil {
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
		if rollbackErr == nil {
			if err := os.RemoveAll(stage); err != nil {
				rollbackErr = err
			}
		}
		if err := syncDirectory(destination); err != nil && rollbackErr == nil {
			rollbackErr = err
		}
		return rollbackErr
	}
	fail := func(cause error) (func() error, func() error, error) {
		if rollbackErr := rollback(); rollbackErr != nil {
			return nil, nil, fmt.Errorf("%w (rollback also failed: %v; recovery stage: %s)", cause, rollbackErr, stage)
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
	if err := syncDirectory(destination); err != nil {
		return fail(err)
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
		if rollbackErr == nil {
			if err := os.RemoveAll(stage); err != nil {
				rollbackErr = err
			}
		}
		if err := syncDirectory(destination); err != nil && rollbackErr == nil {
			rollbackErr = err
		}
		return rollbackErr
	}
	fail := func(cause error) (func() error, func() error, error) {
		if rollbackErr := rollback(); rollbackErr != nil {
			return nil, nil, fmt.Errorf("%w (rollback also failed: %v; recovery stage: %s)", cause, rollbackErr, stage)
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
	if err := syncDirectory(destination); err != nil {
		return fail(err)
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
	return writeBytesSynced(destination, contents, 0o644)
}

func writeBytesSynced(destination string, contents []byte, mode fs.FileMode) error {
	file, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(contents)
	syncErr := file.Sync()
	closeErr := file.Close()
	for _, candidate := range []error{writeErr, syncErr, closeErr} {
		if candidate != nil {
			return candidate
		}
	}
	return nil
}

func receiptPath(stateRoot, destination string) string {
	digest := sha256.Sum256([]byte(destination))
	return filepath.Join(stateRoot, "skill-installs", hex.EncodeToString(digest[:])+".json")
}

func readReceipt(path string) (receipt, bool, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return receipt{}, false, nil
	}
	if err != nil {
		return receipt{}, false, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 {
		return receipt{}, false, fmt.Errorf("receipt %s is not a regular file", path)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return receipt{}, false, err
	}
	if err := skillclaim.RejectDuplicateKeys(contents); err != nil {
		return receipt{}, false, err
	}
	var record receipt
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&record); err != nil {
		return receipt{}, false, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return receipt{}, false, errors.New("receipt has trailing JSON value")
		}
		return receipt{}, false, err
	}
	return record, true, nil
}

func validateReceipt(record receipt, scope Scope, target destination) error {
	if (record.Schema != 1 && record.Schema != 2 && record.Schema != receiptSchemaCurrent) || record.Scope != scope || record.Destination != target.path {
		return fmt.Errorf("receipt does not belong to %s", target.path)
	}
	if len(record.Files) == 0 || record.Version == "" || !validSHA(record.BundleSHA) {
		return errors.New("receipt has invalid source metadata")
	}
	if !sort.SliceIsSorted(record.Files, func(i, j int) bool { return record.Files[i].Path < record.Files[j].Path }) {
		return errors.New("receipt files are not sorted")
	}
	if !slices.IsSorted(record.Runtimes) {
		return errors.New("receipt runtimes are not sorted")
	}
	for index := range record.Runtimes {
		if !validReceiptRuntime(record.Runtimes[index]) || (index > 0 && record.Runtimes[index-1] == record.Runtimes[index]) {
			return errors.New("receipt runtimes are invalid or not unique")
		}
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
	for index, file := range record.Files {
		if !validReceiptPath(file.Path) || !validSHA(file.SHA) || (index > 0 && record.Files[index-1].Path == file.Path) {
			return errors.New("receipt contains an invalid file record")
		}
	}
	if sourceBundleSHA(record.Files) != record.BundleSHA {
		return errors.New("receipt bundle digest differs from its files")
	}
	if record.Schema == 3 {
		if record.Installation != "active" && record.Installation != "absent" {
			return errors.New("receipt has invalid installation state")
		}
		if record.Installation == "active" && len(record.Runtimes) == 0 {
			return errors.New("active receipt has no runtime consumers")
		}
		if record.Installation == "absent" && (len(record.Runtimes) != 0 || len(record.Takeovers) == 0) {
			return errors.New("backup-only receipt has invalid runtime state")
		}
		if !sort.SliceIsSorted(record.Takeovers, func(i, j int) bool { return record.Takeovers[i].Skill < record.Takeovers[j].Skill }) {
			return errors.New("receipt takeover records are not sorted")
		}
		for takeoverIndex, takeover := range record.Takeovers {
			if !validTakeoverSkill(takeover.Skill) || !validBackupID(takeover.BackupID) || (takeover.Kind != backupKindFile && takeover.Kind != backupKindDirectory) || !validSHA(takeover.ManifestDigest) || len(takeover.Entries) == 0 || !sort.SliceIsSorted(takeover.Entries, func(i, j int) bool { return takeover.Entries[i].Path < takeover.Entries[j].Path }) {
				return errors.New("receipt has invalid takeover metadata")
			}
			if takeoverIndex > 0 && record.Takeovers[takeoverIndex-1].Skill == takeover.Skill {
				return errors.New("receipt takeover records are not unique")
			}
			if (format == receiptFormatFlat) != strings.HasSuffix(takeover.Skill, ".md") || (takeover.Kind == backupKindFile && len(takeover.Entries) != 1) {
				return errors.New("receipt takeover kind does not match its destination")
			}
			for entryIndex, entry := range takeover.Entries {
				if !validBackupEntry(entry, takeover.Kind) || (entryIndex > 0 && takeover.Entries[entryIndex-1].Path == entry.Path) {
					return errors.New("receipt takeover has invalid file record")
				}
			}
			if backupManifestDigest(takeover.Entries) != takeover.ManifestDigest {
				return errors.New("receipt takeover manifest digest differs from its entries")
			}
		}
	} else if len(record.Takeovers) > 0 {
		return errors.New("legacy receipt has takeover metadata")
	}
	return nil
}

func validReceiptRuntime(runtime Runtime) bool {
	switch runtime {
	case RuntimeClaudeCode, RuntimeCodex, RuntimeOpenCode, RuntimeGrok, RuntimeAntigravity, RuntimeAgentMesh:
		return true
	default:
		return false
	}
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
	return err == nil && len(decoded) == sha256.Size && value == strings.ToLower(value)
}

func validBackupEntry(entry backupEntry, rootKind string) bool {
	if entry.Kind != backupKindFile && entry.Kind != backupKindDirectory || entry.Mode > 0o777 {
		return false
	}
	if rootKind == backupKindFile {
		return entry.Path == "." && entry.Kind == backupKindFile && validSHA(entry.SHA)
	}
	if entry.Path == "." {
		return entry.Kind == backupKindDirectory && entry.SHA == ""
	}
	if strings.HasPrefix(entry.Path, "/") || strings.Contains(entry.Path, `\`) || strings.HasSuffix(entry.Path, "/") || pathpkg.Clean(entry.Path) != entry.Path || strings.HasPrefix(entry.Path, "../") || entry.Path == ".." {
		return false
	}
	return (entry.Kind == backupKindFile && validSHA(entry.SHA)) || (entry.Kind == backupKindDirectory && entry.SHA == "")
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
	if err := os.Rename(tempName, path); err != nil {
		return err
	}
	return syncDirectory(filepath.Dir(path))
}
