package skillinstall

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
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

	roots := []string{
		options.StateRoot,
		destinations[0].path,
		filepath.Join(filepath.Dir(options.StateRoot), "agent-skills"),
	}
	before := treeFingerprints(t, roots...)
	listed, err := ListTakeoverBackups(options)
	if err != nil {
		t.Fatal(err)
	}
	after := treeFingerprints(t, roots...)
	if !sameStrings(before, after) {
		t.Fatal("backup listing changed an installation, claim, or DVA state root")
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

	options.Takeover = false
	if _, err := Uninstall(options); err != nil {
		t.Fatal(err)
	}
	before = treeFingerprints(t, roots...)
	listed, err = ListTakeoverBackups(options)
	if err != nil {
		t.Fatal(err)
	}
	after = treeFingerprints(t, roots...)
	if !sameStrings(before, after) {
		t.Fatal("backup-only listing changed an installation, claim, or DVA state root")
	}
	if len(listed.Backups) != 1 || listed.Backups[0].Status != "available" || !sameRuntimes(listed.Backups[0].Runtimes, []Runtime{RuntimeAntigravity, RuntimeCodex}) {
		t.Fatalf("backup-only listing = %#v", listed.Backups)
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

func TestListTakeoverBackupsReportsEachBackupIDIndependently(t *testing.T) {
	t.Parallel()
	options := testOptions(t, ScopeUser, RuntimeCodex)
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
	record, found, err := readReceipt(receiptPath(options.StateRoot, destinations[0].path))
	if err != nil || !found || len(record.Takeovers) != 2 {
		t.Fatalf("receipt = (%#v, %t, %v)", record, found, err)
	}
	firstID, secondID := strings.Repeat("1", 32), strings.Repeat("2", 32)
	base := takeoverDestinationRoot(options.StateRoot, record.Destination)
	oldRoot := filepath.Join(base, record.Takeovers[0].BackupID)
	firstRoot, secondRoot := filepath.Join(base, firstID), filepath.Join(base, secondID)
	if err := os.Rename(oldRoot, firstRoot); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(secondRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, _, err := copyBackupTree(filepath.Join(firstRoot, "dva-config"), filepath.Join(secondRoot, "dva-config")); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(firstRoot, "dva-config")); err != nil {
		t.Fatal(err)
	}
	for index := range record.Takeovers {
		switch record.Takeovers[index].Skill {
		case "dva":
			record.Takeovers[index].BackupID = firstID
		case "dva-config":
			record.Takeovers[index].BackupID = secondID
		default:
			t.Fatalf("unexpected takeover skill %q", record.Takeovers[index].Skill)
		}
	}
	if err := writeReceipt(receiptPath(options.StateRoot, destinations[0].path), record); err != nil {
		t.Fatal(err)
	}

	listed, err := ListTakeoverBackups(options)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed.Backups) != 2 || listed.Backups[0].BackupID != firstID || listed.Backups[1].BackupID != secondID || listed.Backups[0].Status != "available" || listed.Backups[1].Status != "available" {
		t.Fatalf("multi-ID listing = %#v", listed.Backups)
	}
	if err := os.Chmod(filepath.Join(secondRoot, "dva-config", "foreign"), 0o644); err != nil {
		t.Fatal(err)
	}
	listed, err = ListTakeoverBackups(options)
	if err != nil {
		t.Fatal(err)
	}
	if listed.Backups[0].Status != "available" || listed.Backups[1].Status != "corrupt" {
		t.Fatalf("partially corrupt listing = %#v", listed.Backups)
	}
}

func TestVerifyTakeoverBackupsRejectsUnsafeBackupAncestors(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink fixture is platform-specific")
	}
	for _, test := range []struct {
		name   string
		mutate func(t *testing.T, base, backupRoot string)
	}{
		{
			name: "backup destination symlink",
			mutate: func(t *testing.T, base, backupRoot string) {
				t.Helper()
				hold := base + ".held"
				if err := os.Rename(base, hold); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(hold, base); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "backup destination non-directory",
			mutate: func(t *testing.T, base, backupRoot string) {
				t.Helper()
				if err := os.RemoveAll(base); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(base, []byte("not a directory"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "backup ID symlink",
			mutate: func(t *testing.T, base, backupRoot string) {
				t.Helper()
				hold := backupRoot + ".held"
				if err := os.Rename(backupRoot, hold); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(hold, backupRoot); err != nil {
					t.Fatal(err)
				}
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
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
			base := takeoverDestinationRoot(options.StateRoot, record.Destination)
			backupRoot := filepath.Join(base, record.Takeovers[0].BackupID)
			test.mutate(t, base, backupRoot)
			if status, _ := verifyTakeoverBackups(options.StateRoot, record); status != "corrupt" {
				t.Fatalf("verification status = %q", status)
			}
		})
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

func treeFingerprints(t *testing.T, roots ...string) []string {
	t.Helper()
	result := make([]string, len(roots))
	for index, root := range roots {
		result[index] = treeFingerprint(t, root)
	}
	return result
}

func TestListTakeoverBackupsValidatesEveryFoundReceipt(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name   string
		mutate func(t *testing.T, options Options, target destination, record receipt)
		want   string
	}{
		{
			name: "empty object",
			mutate: func(t *testing.T, options Options, target destination, record receipt) {
				t.Helper()
				path := receiptPath(options.StateRoot, target.path)
				if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte("{}"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
			want: "validate receipt",
		},
		{
			name: "wrong destination",
			mutate: func(t *testing.T, options Options, target destination, record receipt) {
				t.Helper()
				record.Destination = filepath.Join(target.path, "wrong")
				if err := writeReceipt(receiptPath(options.StateRoot, target.path), record); err != nil {
					t.Fatal(err)
				}
			},
			want: "validate receipt",
		},
		{
			name: "wrong scope",
			mutate: func(t *testing.T, options Options, target destination, record receipt) {
				t.Helper()
				record.Scope = ScopeProject
				if err := writeReceipt(receiptPath(options.StateRoot, target.path), record); err != nil {
					t.Fatal(err)
				}
			},
			want: "validate receipt",
		},
		{
			name:   "valid non-takeover",
			mutate: func(t *testing.T, options Options, target destination, record receipt) {},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			options := testOptions(t, ScopeUser, RuntimeCodex)
			if _, err := Install(options); err != nil {
				t.Fatal(err)
			}
			_, destinations, err := resolve(options)
			if err != nil {
				t.Fatal(err)
			}
			record, found, err := readReceipt(receiptPath(options.StateRoot, destinations[0].path))
			if err != nil || !found {
				t.Fatalf("receipt = (%#v, %t, %v)", record, found, err)
			}
			test.mutate(t, options, destinations[0], record)
			listed, err := ListTakeoverBackups(options)
			if test.want == "" {
				if err != nil || len(listed.Backups) != 0 {
					t.Fatalf("valid non-takeover list = (%#v, %v)", listed, err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}
