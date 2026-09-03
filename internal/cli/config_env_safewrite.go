package cli

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// envRoot is the single handle every path operation of one bridge invocation
// goes through.
//
// It is opened once during preflight and held until the replace, which is the
// whole TOCTOU defense: the TASK-245 §8 spike repointed the config directory
// symlink after the handle was taken and the write still landed in the directory
// the handle had resolved. Re-deriving a path from a string between checks would
// give that guarantee up.
type envRoot struct {
	root *os.Root
	// dir is the resolved anchor, needed only for the argv handed to child
	// processes (sops, git), which take paths and not descriptors.
	dir string
}

func openEnvRoot(dir string) (*envRoot, error) {
	r, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	return &envRoot{root: r, dir: dir}, nil
}

func (r *envRoot) Close() { _ = r.root.Close() }

// abs renders a handle-relative path as an absolute one for a child process.
// Only call it for a path that already passed checkPath — a child cannot inherit
// the handle's containment guarantee, so the guarantee has to be established
// before the string is built.
func (r *envRoot) abs(p string) string { return filepath.Join(r.dir, p) }

// pathState is what a component walk found at the leaf.
type pathState int

const (
	// pathMissingLeaf: every parent exists, the final component does not.
	pathMissingLeaf pathState = iota
	// pathMissingParent: a directory above the final component is absent.
	pathMissingParent
	// pathPresent: the leaf exists; info describes it.
	pathPresent
)

// checkPath validates a declared relative path and reports what is at its leaf.
//
// The two shape rules come first because they are decidable from the string and
// must not depend on what happens to exist. Then every component is Lstat'ed
// through the handle: os.Root blocks escapes but deliberately follows symlinks
// that stay inside the root (§8-2), so containment alone would let `.env ->
// tracked-file` through. The frozen contract makes this an explicit gate rather
// than a property of os.Root.
func (r *envRoot) checkPath(declared string) (pathState, fs.FileInfo, error) {
	if filepath.IsAbs(declared) {
		return 0, nil, bridgeErr(codeAbsolutePath, "%q is absolute; env bridge writes only config-relative paths", declared)
	}
	if !filepath.IsLocal(declared) {
		return 0, nil, bridgeErr(codePathEscapes, "%q resolves outside the config directory", declared)
	}

	parts := strings.Split(filepath.ToSlash(filepath.Clean(declared)), "/")
	for i := range parts {
		prefix := strings.Join(parts[:i+1], "/")
		info, err := r.root.Lstat(prefix)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				if i == len(parts)-1 {
					return pathMissingLeaf, nil, nil
				}
				return pathMissingParent, nil, nil
			}
			if errors.Is(err, fs.ErrPermission) {
				return 0, nil, bridgeErr(codePermissionDenied, "permission denied writing %s", declared)
			}
			return 0, nil, err
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			return 0, nil, bridgeErr(codePathComponentSymlnk, "%q contains a symlinked path component", declared)
		}
		if i < len(parts)-1 && !info.IsDir() {
			// A non-directory standing where a directory must be is the same
			// user-visible condition as a missing directory: the leaf's parent
			// does not exist as a directory, and the bridge does not create one.
			return pathMissingParent, nil, nil
		}
		if i == len(parts)-1 {
			return pathPresent, info, nil
		}
	}
	return pathMissingLeaf, nil, nil
}

const envTempPrefix = ".dva-env-"
const envTempSuffix = ".tmp"

// staleTempAge is how long an owned temp must have gone untouched before
// recovery removes it. It is long enough that a concurrently running unseal's
// temp is never a candidate (§8-5).
const staleTempAge = time.Hour

// reclaimStaleTemps removes temporary files this command is certain it owns.
//
// TASK-245 §8-4 refuses to claim SIGKILL or power-loss cleanup, because no
// in-process handler can promise it. This is the honest substitute: the next run
// collects what the killed run left. Every condition is a conjunct — prefix,
// suffix, regular file by Lstat, and an hour of age — so a file that merely
// resembles a temp, or belongs to a run still in flight, is left alone. Errors
// are ignored on purpose: recovery is best-effort housekeeping and must never
// turn a valid unseal into a failure.
func (r *envRoot) reclaimStaleTemps(now time.Time) int {
	// Listed through the handle, not by name: the candidate set must come from
	// the same directory the removals land in, or a repointed symlink could
	// choose which of this directory's files get considered.
	d, err := r.root.Open(".")
	if err != nil {
		return 0
	}
	entries, err := d.ReadDir(-1)
	_ = d.Close()
	if err != nil {
		return 0
	}
	removed := 0
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, envTempPrefix) || !strings.HasSuffix(name, envTempSuffix) {
			continue
		}
		info, err := r.root.Lstat(name)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		if now.Sub(info.ModTime()) < staleTempAge {
			continue
		}
		if err := r.root.Remove(name); err == nil {
			removed++
		}
	}
	return removed
}

// tempName is derived from the process and a random token only. Nothing in it
// comes from the file being written, so a temp left on disk names no secret
// (§7-4).
//
// The token is random rather than a timestamp. A clock-derived name looks unique
// and is not: darwin reports UnixNano at microsecond resolution, so two writers
// starting in the same microsecond produce the same string, and the O_EXCL create
// then fails with a bare "file exists" that carries none of the frozen codes. The
// uniqueness of a temporary name must not depend on how finely the host happens
// to report time.
func tempName(pid int, token string) string {
	return fmt.Sprintf("%s%d-%s%s", envTempPrefix, pid, token, envTempSuffix)
}

// tempCreateAttempts bounds the retry so that a directory which refuses every
// create for some other reason cannot spin. Two collisions on 64 random bits do
// not happen; the loop exists so that if they somehow did, the command would
// still succeed rather than report a name clash to a user who cannot act on it.
const tempCreateAttempts = 5

// newTemp creates a uniquely named temporary in the target's directory.
//
// Collision is retried rather than reported: an O_EXCL refusal means the name was
// taken, which is this function's problem to solve and not a condition the caller
// or the user can do anything about. Every other create error — permission,
// read-only filesystem, no space — is returned unchanged so the caller can map it
// to the code that names the real cause.
func (r *envRoot) newTemp() (*safeWriter, error) {
	pid := os.Getpid()
	var err error
	for range tempCreateAttempts {
		var token string
		if token, err = randomToken(); err != nil {
			return nil, err
		}
		var w *safeWriter
		if w, err = r.newSafeWriter(tempName(pid, token)); err == nil {
			return w, nil
		}
		if !errors.Is(err, fs.ErrExist) {
			return nil, err
		}
	}
	return nil, err
}

func randomToken() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

// safeWriter owns one temp file from creation to replacement.
type safeWriter struct {
	root *envRoot
	name string
	file *os.File
	done bool
}

// newSafeWriter creates the temp with O_EXCL and mode 0600 in the same directory
// as the target, which is what makes the later rename an atomic same-filesystem
// replace rather than a copy.
func (r *envRoot) newSafeWriter(name string) (*safeWriter, error) {
	f, err := r.root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	return &safeWriter{root: r, name: name, file: f}, nil
}

// File is the destination handed to the sops child as its stdout. The decrypted
// bytes travel kernel-side from the child into this descriptor and never exist
// as a string inside DVA (§7-4).
func (w *safeWriter) File() *os.File { return w.file }

// Abort removes the temp and leaves the target untouched. Safe to defer: it is a
// no-op once Commit has run.
func (w *safeWriter) Abort() {
	if w.done {
		return
	}
	w.done = true
	_ = w.file.Close()
	_ = w.root.root.Remove(w.name)
}

// Commit syncs the payload, syncs the parent directory, then renames.
//
// The directory sync is the step that is easy to omit and hard to notice
// missing: without it the rename can be lost on crash even though the file's own
// contents were durable, leaving the target absent after a run that reported
// success. Both supported platforms accepted it in the §8-1 spike.
func (w *safeWriter) Commit(target string) error {
	if w.done {
		return errors.New("safe writer already finished")
	}
	if err := w.file.Sync(); err != nil {
		w.Abort()
		return err
	}
	if err := w.file.Close(); err != nil {
		w.done = true
		_ = w.root.root.Remove(w.name)
		return err
	}
	if err := w.root.syncDir(); err != nil {
		w.done = true
		_ = w.root.root.Remove(w.name)
		return err
	}
	if err := w.root.root.Rename(w.name, target); err != nil {
		w.done = true
		_ = w.root.root.Remove(w.name)
		return err
	}
	w.done = true
	return w.root.syncDir()
}

// syncDir flushes the directory entry through the held handle rather than by
// reopening r.dir by name.
//
// Reopening would re-resolve the path, and TestConfigEnvRejectsPathSwap shows why
// that matters: when the config directory is reached through a symlink that is
// repointed mid-run, the rename still lands in the originally resolved directory
// (it is handle-relative) while a by-name reopen would fsync a different
// directory entirely — flushing metadata for a directory the write never touched
// and leaving the real one unflushed. The durability step has to observe the same
// directory the rename did.
func (r *envRoot) syncDir() error {
	d, err := r.root.Open(".")
	if err != nil {
		return err
	}
	defer func() { _ = d.Close() }()
	return d.Sync()
}
