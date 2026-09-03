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

// envRoot is a directory handle every path operation of one bridge invocation
// goes through.
//
// It is opened once during preflight and held until the replace, which is the
// whole TOCTOU defense: the TASK-245 §8 spike repointed the config directory
// symlink after the handle was taken and the write still landed in the directory
// the handle had resolved. Re-deriving a path from a string between checks would
// give that guarantee up.
//
// One invocation opens two of these. The first is the config root, which owns
// the shape checks and the argv handed to sops. The second is the target's own
// directory (see targetAnchor), because a handle on the config root only anchors
// a target that sits directly in it — for `sub/.env` the `sub` component would
// still be re-resolved by name at rename time, which is exactly the moment the
// preflight's guarantees are needed.
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

// openAt opens a nested handle on a subdirectory of r, inheriting r's
// containment: the new handle cannot reach anything r could not.
func (r *envRoot) openAt(rel string) (*envRoot, error) {
	sub, err := r.root.OpenRoot(rel)
	if err != nil {
		return nil, err
	}
	return &envRoot{root: sub, dir: filepath.Join(r.dir, rel)}, nil
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

// targetAnchor is the directory that will receive the write, held open from
// preflight through the rename.
//
// Anchoring on the target's own directory rather than on the config root is what
// makes the rename a same-directory operation: after the anchor is taken there is
// no path component left for the kernel to re-resolve by name, so a `sub` that is
// swapped or repointed while sops runs cannot redirect where the plaintext lands.
// The temp is created here for the same reason, and it is also where the user's
// ignore rules for the env file already point.
type targetAnchor struct {
	// dir is the handle on the target's own directory. Every write, rename,
	// recovery sweep and fsync of this invocation goes through it.
	dir *envRoot
	// cfg is the config root handle the anchor was opened through, kept so the
	// pre-rename re-validation can ask what the declared path names *now*.
	cfg *envRoot
	// rel is the anchor's path relative to the config root, "." when the target
	// sits directly in it.
	rel string
	// leaf is the target's final component — the name the temp is renamed onto.
	leaf string
	// declared is the path as dva.yml spells it, for messages only.
	declared string
}

// openTargetAnchor takes the handle on the directory declared holds its leaf in.
// Call it only after checkPath has approved declared and its parent is known to
// exist: this opens a directory, it does not vet one.
func (r *envRoot) openTargetAnchor(declared string) (*targetAnchor, error) {
	clean := filepath.Clean(declared)
	rel, leaf := filepath.Dir(clean), filepath.Base(clean)
	d, err := r.openAt(rel)
	if err != nil {
		return nil, err
	}
	return &targetAnchor{dir: d, cfg: r, rel: rel, leaf: leaf, declared: declared}, nil
}

func (a *targetAnchor) Close() { a.dir.Close() }

// stillAnchored re-establishes, immediately before the rename, that the declared
// path still names the directory the preflight approved.
//
// The handle alone already prevents the plaintext from landing anywhere else —
// the rename follows the inode, not the name. What it cannot do on its own is
// notice that the name has moved on, and committing there would satisfy neither
// the §5-3 symlink gate nor the §5-4 git guard, both of which were answered for
// the path as it read at preflight. So the answer is refusal, not a write into a
// directory whose git status nothing has asked about.
//
// A target at the config root has no component below the held handle and skips
// the check entirely; there is nothing left to re-resolve.
func (a *targetAnchor) stillAnchored() error {
	if a.rel == "." {
		return nil
	}
	state, info, err := a.cfg.checkPath(a.rel)
	if err != nil {
		return err
	}
	if state != pathPresent || !info.IsDir() {
		return a.movedErr()
	}
	held, err := a.dir.root.Stat(".")
	if err != nil {
		return err
	}
	if !os.SameFile(info, held) {
		return a.movedErr()
	}
	return nil
}

// movedErr carries no frozen code on purpose. TASK-245 §7-1 closed the code set,
// and this condition has no member in it; inventing one would be a contract
// change, and reusing a member that names a different cause would be worse than
// a codeless envelope.
func (a *targetAnchor) movedErr() error {
	return fmt.Errorf("env bridge: the directory for %s is no longer the one the preflight approved; "+
		"nothing was written", a.declared)
}

// envTempInfix and envTempSuffix bracket the random part of a temp name. Both
// have to be present for recovery to claim a file (§8-5).
const envTempInfix = ".dva-env-"
const envTempSuffix = ".tmp"

// staleTempAge is how long an owned temp must have gone untouched before
// recovery removes it. It is long enough that a concurrently running unseal's
// temp is never a candidate (§8-5).
const staleTempAge = time.Hour

// isOwnedTemp recognizes the namespace recovery is allowed to remove from.
//
// §8-5 spelled this as a `.dva-env-` prefix, from when every temp was created at
// the config root under a name of its own. Now that a temp is created beside its
// target and carries that target's name (see tempName), the marker sits in the
// middle instead — the conjunct is unchanged in strength, only in position.
func isOwnedTemp(name string) bool {
	return strings.Contains(name, envTempInfix) && strings.HasSuffix(name, envTempSuffix)
}

// reclaimStaleTemps removes temporary files this command is certain it owns.
//
// TASK-245 §8-4 refuses to claim SIGKILL or power-loss cleanup, because no
// in-process handler can promise it. This is the honest substitute: the next run
// collects what the killed run left. Every condition is a conjunct — the owned
// name shape, regular file by Lstat, and an hour of age — so a file that merely
// resembles a temp, or belongs to a run still in flight, is left alone. Errors
// are ignored on purpose: recovery is best-effort housekeeping and must never
// turn a valid unseal into a failure.
func (r *envRoot) reclaimStaleTemps(now time.Time) {
	// Listed through the handle, not by name: the candidate set must come from
	// the same directory the removals land in, or a repointed symlink could
	// choose which of this directory's files get considered.
	d, err := r.root.Open(".")
	if err != nil {
		return
	}
	entries, err := d.ReadDir(-1)
	_ = d.Close()
	if err != nil {
		return
	}
	for _, e := range entries {
		name := e.Name()
		if !isOwnedTemp(name) {
			continue
		}
		info, err := r.root.Lstat(name)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		if now.Sub(info.ModTime()) < staleTempAge {
			continue
		}
		_ = r.root.Remove(name)
	}
}

// tempName is derived from the target's own file name, the process and a random
// token. Nothing in it comes from the file's *content*, so a temp left on disk
// names no secret (§7-4) — the leaf is a path the user wrote in dva.yml and that
// git is already being asked about.
//
// The leaf is there so that a stray temp falls under the same ignore rule as the
// target for the glob forms projects actually write (`.env*`, `.env.*`, `*.env*`
// all cover `.env.dva-env-…tmp`). This is a new rule, not one §8-5 stated: that
// section named a `.dva-env-` prefix, from when the temp was created at the config
// root and matched no ignore pattern anywhere. A project that ignores its env file
// by exact name still will not cover the temp, and §8-4's refusal to promise
// SIGKILL cleanup is unchanged; the residue is now bounded to the target's own
// directory instead of the config root.
//
// The token is random rather than a timestamp. A clock-derived name looks unique
// and is not: darwin reports UnixNano at microsecond resolution, so two writers
// starting in the same microsecond produce the same string, and the O_EXCL create
// then fails with a bare "file exists" that carries none of the frozen codes. The
// uniqueness of a temporary name must not depend on how finely the host happens
// to report time.
func tempName(leaf string, pid int, token string) string {
	return fmt.Sprintf("%s%s%d-%s%s", leaf, envTempInfix, pid, token, envTempSuffix)
}

// tempCreateAttempts bounds the retry so that a directory which refuses every
// create for some other reason cannot spin. Two collisions on 64 random bits do
// not happen; the loop exists so that if they somehow did, the command would
// still succeed rather than report a name clash to a user who cannot act on it.
const tempCreateAttempts = 5

// newTemp creates a uniquely named temporary in the anchor — that is, in the
// target's own directory.
//
// Collision is retried rather than reported: an O_EXCL refusal means the name was
// taken, which is this function's problem to solve and not a condition the caller
// or the user can do anything about. Every other create error — permission,
// read-only filesystem, no space — is returned unchanged so the caller can map it
// to the code that names the real cause.
func (a *targetAnchor) newTemp() (*safeWriter, error) {
	pid := os.Getpid()
	var err error
	for range tempCreateAttempts {
		var token string
		if token, err = randomToken(); err != nil {
			return nil, err
		}
		var w *safeWriter
		if w, err = a.newSafeWriter(tempName(a.leaf, pid, token)); err == nil {
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
	anchor *targetAnchor
	name   string
	file   *os.File
	done   bool
}

// newSafeWriter creates the temp with O_EXCL and mode 0600 through the anchor
// handle, which is the same directory the target's leaf lives in. That is what
// makes the later rename an atomic same-filesystem replace rather than a copy,
// and it is why the name carries no directory component: there is none left.
func (a *targetAnchor) newSafeWriter(name string) (*safeWriter, error) {
	f, err := a.dir.root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	return &safeWriter{anchor: a, name: name, file: f}, nil
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
	_ = w.file.Close()
	w.discard()
}

// discard is Abort for a writer whose descriptor is already closed.
func (w *safeWriter) discard() {
	w.done = true
	_ = w.anchor.dir.root.Remove(w.name)
}

// Commit syncs the payload, syncs the anchor directory, re-checks that the
// anchor is still what the declared path names, then renames.
//
// The directory sync is the step that is easy to omit and hard to notice
// missing: without it the rename can be lost on crash even though the file's own
// contents were durable, leaving the target absent after a run that reported
// success. Both supported platforms accepted it in the §8-1 spike. It flushes the
// anchor, which is the directory the rename actually touches — for `sub/.env`
// that is `sub`, and flushing the config root instead would leave the entry the
// rename created unflushed.
func (w *safeWriter) Commit() error {
	if w.done {
		return errors.New("safe writer already finished")
	}
	if err := w.file.Sync(); err != nil {
		w.Abort()
		return err
	}
	if err := w.file.Close(); err != nil {
		w.discard()
		return err
	}
	if err := w.anchor.dir.syncDir(); err != nil {
		w.discard()
		return err
	}
	// Last thing before the rename, so the window it leaves open is a few
	// syscalls wide rather than the length of an unbounded sops run.
	if err := w.anchor.stillAnchored(); err != nil {
		w.discard()
		return err
	}
	if err := w.anchor.dir.root.Rename(w.name, w.anchor.leaf); err != nil {
		w.discard()
		return err
	}
	w.done = true
	if err := w.anchor.dir.syncDir(); err != nil {
		return &postRenameError{target: w.anchor.declared, err: err}
	}
	return nil
}

// postRenameError reports a failure that happened after the target was already
// replaced.
//
// It exists because unseal's help text promises that "any failure leaves an
// existing target byte-for-byte unchanged", and the final directory fsync is the
// one step that runs when that promise no longer holds. Returning a bare error
// there would make the command contradict its own documentation; this one says
// what actually happened.
type postRenameError struct {
	target string
	err    error
}

func (e *postRenameError) Error() string {
	return fmt.Sprintf("env bridge: %s was replaced, but flushing its directory to disk failed: %v; "+
		"the new content is in place and may not survive a crash", e.target, e.err)
}

func (e *postRenameError) Unwrap() error { return e.err }

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
