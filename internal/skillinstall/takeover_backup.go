package skillinstall

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
)

const (
	backupKindDirectory = "directory"
	backupKindFile      = "file"
)

func createTakeoverBackups(stateRoot string, target destination, bundle skillBundle) ([]takeoverBackup, func() error, error) {
	var names []string
	for _, name := range skillNames(bundle) {
		if _, err := os.Lstat(claimDestination(target, name)); err == nil {
			names = append(names, name)
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, nil, err
		}
	}
	if len(names) == 0 {
		return nil, func() error { return nil }, nil
	}

	backupID, err := newBackupID()
	if err != nil {
		return nil, nil, err
	}
	base := takeoverDestinationRoot(stateRoot, target.path)
	if err := os.MkdirAll(base, 0o700); err != nil {
		return nil, nil, err
	}
	stage, err := os.MkdirTemp(base, ".stage-")
	if err != nil {
		return nil, nil, err
	}
	cleanupStage := true
	defer func() {
		if cleanupStage {
			_ = os.RemoveAll(stage)
		}
	}()

	records := make([]takeoverBackup, 0, len(names))
	for _, name := range names {
		kind, entries, err := copyBackupTree(claimDestination(target, name), filepath.Join(stage, name))
		if err != nil {
			return nil, nil, fmt.Errorf("back up takeover skill %s: %w", name, err)
		}
		records = append(records, takeoverBackup{
			Skill: name, BackupID: backupID, Kind: kind,
			ManifestDigest: backupManifestDigest(entries), Entries: entries,
		})
	}
	final := filepath.Join(base, backupID)
	if err := os.Rename(stage, final); err != nil {
		return nil, nil, err
	}
	cleanupStage = false
	if err := syncDirectory(base); err != nil {
		return nil, nil, err
	}
	cleanup := func() error {
		if err := os.RemoveAll(final); err != nil {
			return err
		}
		return syncDirectory(base)
	}
	return records, cleanup, nil
}

func newBackupID() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

func takeoverDestinationRoot(stateRoot, destination string) string {
	digest := sha256.Sum256([]byte(destination))
	return filepath.Join(stateRoot, "skill-takeovers", hex.EncodeToString(digest[:]))
}

func takeoverBackupPath(stateRoot, destination string, record takeoverBackup) (string, error) {
	if !validBackupID(record.BackupID) || !validTakeoverSkill(record.Skill) {
		return "", errors.New("takeover receipt contains an invalid backup identity")
	}
	return filepath.Join(takeoverDestinationRoot(stateRoot, destination), record.BackupID, record.Skill), nil
}

func validBackupID(value string) bool {
	raw, err := hex.DecodeString(value)
	return err == nil && len(raw) == 16 && value == strings.ToLower(value)
}

func validTakeoverSkill(value string) bool {
	return value == "dva" || value == "dva-config" || value == "dva.md" || value == "dva-config.md"
}

func copyBackupTree(source, destination string) (string, []backupEntry, error) {
	info, err := os.Lstat(source)
	if err != nil {
		return "", nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || (!info.Mode().IsRegular() && !info.IsDir()) {
		return "", nil, fmt.Errorf("%s is not a regular file or directory", source)
	}
	if info.Mode().IsRegular() {
		entry, err := copyBackupFile(source, destination, ".", info.Mode().Perm())
		return backupKindFile, []backupEntry{entry}, err
	}

	if err := os.MkdirAll(destination, 0o700); err != nil {
		return "", nil, err
	}
	var entries []backupEntry
	var directories []struct {
		path string
		mode fs.FileMode
	}
	err = filepath.WalkDir(source, func(path string, item fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := item.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || (!info.IsDir() && !info.Mode().IsRegular()) {
			return fmt.Errorf("%s is not a regular file or directory", path)
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		target := filepath.Join(destination, filepath.FromSlash(relative))
		if info.IsDir() {
			if err := os.MkdirAll(target, 0o700); err != nil {
				return err
			}
			entries = append(entries, backupEntry{Path: relative, Kind: backupKindDirectory, Mode: uint32(info.Mode().Perm())})
			directories = append(directories, struct {
				path string
				mode fs.FileMode
			}{target, info.Mode().Perm()})
			return nil
		}
		entry, err := copyBackupFile(path, target, relative, info.Mode().Perm())
		if err == nil {
			entries = append(entries, entry)
		}
		return err
	})
	if err != nil {
		return "", nil, err
	}
	for _, directory := range slices.Backward(directories) {
		if err := os.Chmod(directory.path, directory.mode); err != nil {
			return "", nil, err
		}
		if err := syncDirectory(directory.path); err != nil {
			return "", nil, err
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return backupKindDirectory, entries, nil
}

func copyBackupFile(source, destination, relative string, mode fs.FileMode) (backupEntry, error) {
	contents, err := os.ReadFile(source)
	if err != nil {
		return backupEntry{}, err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return backupEntry{}, err
	}
	file, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return backupEntry{}, err
	}
	_, writeErr := file.Write(contents)
	chmodErr := file.Chmod(mode)
	syncErr := file.Sync()
	closeErr := file.Close()
	for _, candidate := range []error{writeErr, chmodErr, syncErr, closeErr} {
		if candidate != nil {
			return backupEntry{}, candidate
		}
	}
	digest := sha256.Sum256(contents)
	return backupEntry{Path: relative, Kind: backupKindFile, Mode: uint32(mode), SHA: hex.EncodeToString(digest[:])}, nil
}

func backupManifestDigest(entries []backupEntry) string {
	hash := sha256.New()
	for _, entry := range entries {
		_, _ = hash.Write([]byte(entry.Path))
		_, _ = hash.Write([]byte{0})
		_, _ = hash.Write([]byte(entry.Kind))
		_, _ = hash.Write([]byte{0})
		_, _ = hash.Write([]byte(strconv.FormatUint(uint64(entry.Mode), 8)))
		_, _ = hash.Write([]byte{0})
		_, _ = hash.Write([]byte(entry.SHA))
		_, _ = hash.Write([]byte{0})
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func verifyTakeoverBackups(stateRoot string, record receipt) (string, string) {
	if len(record.Takeovers) == 0 {
		return "", ""
	}
	first := ""
	for _, takeover := range record.Takeovers {
		path, err := takeoverBackupPath(stateRoot, record.Destination, takeover)
		if err != nil {
			return "corrupt", ""
		}
		if first == "" {
			first = path
		}
		kind, entries, err := inspectBackupTree(path)
		if err != nil || kind != takeover.Kind || backupManifestDigest(entries) != takeover.ManifestDigest || !equalBackupEntries(entries, takeover.Entries) {
			return "corrupt", first
		}
	}
	return "available", first
}

func inspectBackupTree(root string) (string, []backupEntry, error) {
	info, err := os.Lstat(root)
	if err != nil {
		return "", nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || (!info.IsDir() && !info.Mode().IsRegular()) {
		return "", nil, errors.New("backup is not regular")
	}
	if info.Mode().IsRegular() {
		contents, err := os.ReadFile(root)
		if err != nil {
			return "", nil, err
		}
		digest := sha256.Sum256(contents)
		return backupKindFile, []backupEntry{{Path: ".", Kind: backupKindFile, Mode: uint32(info.Mode().Perm()), SHA: hex.EncodeToString(digest[:])}}, nil
	}
	var entries []backupEntry
	err = filepath.WalkDir(root, func(path string, item fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := item.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || (!info.IsDir() && !info.Mode().IsRegular()) {
			return errors.New("backup contains a non-regular entry")
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		entry := backupEntry{Path: relative, Mode: uint32(info.Mode().Perm())}
		if info.IsDir() {
			entry.Kind = backupKindDirectory
		} else {
			entry.Kind = backupKindFile
			contents, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			digest := sha256.Sum256(contents)
			entry.SHA = hex.EncodeToString(digest[:])
		}
		entries = append(entries, entry)
		return nil
	})
	if err != nil {
		return "", nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return backupKindDirectory, entries, nil
}

func replaceWithTakeoverBackups(stateRoot string, record receipt) (func() error, func() error, error) {
	stage, err := os.MkdirTemp(record.Destination, ".dva-takeover-restore-")
	if err != nil {
		return nil, nil, err
	}
	backups := make(map[string]takeoverBackup, len(record.Takeovers))
	for _, takeover := range record.Takeovers {
		path, err := takeoverBackupPath(stateRoot, record.Destination, takeover)
		if err != nil {
			_ = os.RemoveAll(stage)
			return nil, nil, err
		}
		kind, entries, err := copyBackupTree(path, filepath.Join(stage, takeover.Skill+".original"))
		if err != nil || kind != takeover.Kind || !equalBackupEntries(entries, takeover.Entries) {
			_ = os.RemoveAll(stage)
			if err == nil {
				err = errors.New("staged takeover backup differs from receipt")
			}
			return nil, nil, err
		}
		backups[takeover.Skill] = takeover
	}

	type move struct {
		final    string
		dva      string
		restored bool
	}
	var moves []move
	rollback := func() error {
		var first error
		for _, move := range slices.Backward(moves) {
			if move.restored {
				if err := os.RemoveAll(move.final); err != nil && first == nil {
					first = err
				}
			}
			if move.dva != "" {
				if err := os.Rename(move.dva, move.final); err != nil && first == nil {
					first = err
				}
			}
		}
		if err := os.RemoveAll(stage); err != nil && first == nil {
			first = err
		}
		return first
	}
	fail := func(cause error) (func() error, func() error, error) {
		if rollbackErr := rollback(); rollbackErr != nil {
			return nil, nil, fmt.Errorf("%w (restore rollback also failed: %v; recovery stage: %s)", cause, rollbackErr, stage)
		}
		return nil, nil, cause
	}
	for _, name := range skillNames(skillBundle{files: record.Files}) {
		final := filepath.Join(record.Destination, name)
		dva := filepath.Join(stage, name+".dva")
		if _, err := os.Lstat(final); err == nil {
			if err := os.Rename(final, dva); err != nil {
				return fail(err)
			}
			moves = append(moves, move{final: final, dva: dva})
		} else if !errors.Is(err, os.ErrNotExist) {
			return fail(err)
		} else {
			moves = append(moves, move{final: final})
		}
		if _, ok := backups[name]; ok {
			if err := os.Rename(filepath.Join(stage, name+".original"), final); err != nil {
				return fail(err)
			}
			moves[len(moves)-1].restored = true
		}
	}
	finalize := func() error {
		if err := os.RemoveAll(stage); err != nil {
			return err
		}
		seen := map[string]bool{}
		for _, takeover := range record.Takeovers {
			if seen[takeover.BackupID] {
				continue
			}
			seen[takeover.BackupID] = true
			root := filepath.Join(takeoverDestinationRoot(stateRoot, record.Destination), takeover.BackupID)
			if err := os.RemoveAll(root); err != nil {
				return err
			}
		}
		return syncDirectory(takeoverDestinationRoot(stateRoot, record.Destination))
	}
	return rollback, finalize, nil
}

func equalBackupEntries(left, right []backupEntry) bool {
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

func syncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	syncErr := directory.Sync()
	closeErr := directory.Close()
	if syncErr != nil {
		return syncErr
	}
	return closeErr
}
