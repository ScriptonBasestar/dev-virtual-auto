package skillinstall

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestListTakeoverBackupsFiltersAndDeduplicatesSharedDestination(t *testing.T) {
	t.Parallel()
	options := testOptions(t, ScopeProject, RuntimeCodex, RuntimeAntigravity)
	_, destinations, err := resolve(options)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"dva", "dva-config"} {
		path := filepath.Join(destinations[0].path, name)
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(path, "foreign"), []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	options.Takeover = true
	if _, err := Install(options); err != nil {
		t.Fatal(err)
	}

	before := treeFingerprint(t, options.StateRoot)
	listed, err := ListTakeoverBackups(options)
	if err != nil {
		t.Fatal(err)
	}
	after := treeFingerprint(t, options.StateRoot)
	if before != after {
		t.Fatal("backup listing changed DVA state")
	}
	if len(listed.Backups) != 1 {
		t.Fatalf("backups = %#v, want one deduplicated shared destination", listed.Backups)
	}
	backup := listed.Backups[0]
	if !validBackupID(backup.BackupID) || backup.Status != "available" {
		t.Fatalf("backup = %#v", backup)
	}
	if !sameRuntimes(backup.Runtimes, []Runtime{RuntimeAntigravity, RuntimeCodex}) {
		t.Fatalf("runtimes = %v", backup.Runtimes)
	}
	if !sameStrings(backup.Skills, []string{"dva", "dva-config"}) {
		t.Fatalf("skills = %v", backup.Skills)
	}

	codexOnly := options
	codexOnly.Takeover = false
	codexOnly.Runtimes = []Runtime{RuntimeCodex}
	listed, err = ListTakeoverBackups(codexOnly)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed.Backups) != 1 || !sameRuntimes(listed.Backups[0].Runtimes, []Runtime{RuntimeCodex}) {
		t.Fatalf("runtime-filtered backups = %#v", listed.Backups)
	}

	user := options
	user.Takeover = false
	user.Scope = ScopeUser
	listed, err = ListTakeoverBackups(user)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed.Backups) != 0 {
		t.Fatalf("user scope listed project backups: %#v", listed.Backups)
	}
}

func TestListTakeoverBackupsReportsCorruptBackup(t *testing.T) {
	t.Parallel()
	options := testOptions(t, ScopeUser, RuntimeCodex)
	_, destinations, err := resolve(options)
	if err != nil {
		t.Fatal(err)
	}
	foreign := filepath.Join(destinations[0].path, "dva")
	if err := os.MkdirAll(foreign, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(foreign, "foreign"), []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	options.Takeover = true
	if _, err := Install(options); err != nil {
		t.Fatal(err)
	}
	record, found, err := readReceipt(receiptPath(options.StateRoot, destinations[0].path))
	if err != nil || !found {
		t.Fatalf("receipt = (%#v, %t, %v)", record, found, err)
	}
	backupPath, err := takeoverBackupPath(options.StateRoot, record.Destination, record.Takeovers[0])
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(backupPath, "foreign"), 0o644); err != nil {
		t.Fatal(err)
	}

	listed, err := ListTakeoverBackups(options)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed.Backups) != 1 || listed.Backups[0].Status != "corrupt" {
		t.Fatalf("backups = %#v", listed.Backups)
	}
}

func treeFingerprint(t *testing.T, root string) string {
	t.Helper()
	hash := sha256.New()
	var paths []string
	if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		paths = append(paths, path)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sort.Strings(paths)
	for _, path := range paths {
		info, err := os.Lstat(path)
		if err != nil {
			t.Fatal(err)
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = hash.Write([]byte(relative))
		_, _ = hash.Write([]byte{0})
		_, _ = hash.Write([]byte(info.Mode().String()))
		_, _ = hash.Write([]byte{0})
		if info.Mode().IsRegular() {
			contents, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			_, _ = hash.Write(contents)
		}
		_, _ = hash.Write([]byte{0})
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func TestListTakeoverBackupsRejectsInvalidReceipt(t *testing.T) {
	t.Parallel()
	options := testOptions(t, ScopeUser, RuntimeCodex)
	_, destinations, err := resolve(options)
	if err != nil {
		t.Fatal(err)
	}
	path := receiptPath(options.StateRoot, destinations[0].path)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("not-json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ListTakeoverBackups(options); err == nil || !strings.Contains(err.Error(), "read receipt") {
		t.Fatalf("error = %v", err)
	}
}
