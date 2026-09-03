package agentdeny

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/ScriptonBasestar/dva/internal/skillinstall"
)

const receiptSchema = 1

// receipt records exactly which deny-pattern strings DVA added to one destination file,
// so a later Status/Uninstall can tell a DVA-owned entry from one the user added
// independently — the same local-modification-detection contract
// internal/skillinstall's receipts provide for skill installs, applied to a single JSON
// array instead of a set of files.
type receipt struct {
	Schema      int                `json:"schema"`
	Scope       skillinstall.Scope `json:"scope"`
	Runtime     string             `json:"runtime"`
	Destination string             `json:"destination"`
	// Patterns is sorted and holds every pattern DVA has ever added at this
	// destination, so a later GatedCommands addition is still recognized as DVA-owned
	// once a subsequent install picks it up.
	Patterns []string `json:"patterns"`
	Version  string   `json:"version"`
}

func receiptPath(stateRoot, destination string) string {
	digest := sha256.Sum256([]byte(destination))
	return filepath.Join(stateRoot, "agent-deny-installs", hex.EncodeToString(digest[:])+".json")
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
	if record.Schema != receiptSchema {
		return receipt{}, false, fmt.Errorf("receipt %s has unsupported schema %d", path, record.Schema)
	}
	return record, true, nil
}

// writeReceipt writes record atomically: temp file in the same directory (so the
// rename is on one filesystem), fsync before close, rename over the destination, then
// fsync the parent directory — a crash at any point leaves the previous receipt (or
// none) intact, never a half-written one. Mirrors internal/skillinstall's writeReceipt.
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

func removeReceipt(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// writeSettingsFile writes an ordinary (non-secret) shared config file atomically,
// preserving mode if the file already existed, defaulting to 0o644 for a new one — the
// same temp-file+fsync+rename discipline as writeReceipt, applied to a file other tools
// and humans also read and edit.
func writeSettingsFile(path string, contents []byte) error {
	mode := os.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".settings-")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	defer func() { _ = os.Remove(tempName) }()
	if err := temp.Chmod(mode); err != nil {
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

func syncDirectory(dir string) error {
	f, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	return f.Sync()
}
