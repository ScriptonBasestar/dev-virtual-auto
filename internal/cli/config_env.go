package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/ScriptonBasestar/dva/internal/config"
	"github.com/ScriptonBasestar/dva/internal/output"
	"github.com/spf13/cobra"
)

// bridgeGOOS is injectable so the unsupported-platform path is exercised on every
// platform CI runs, rather than only on the platforms where it would fire. A
// fail-closed branch that is never executed is a branch nobody has checked.
var bridgeGOOS = runtime.GOOS

// supportedBridgePlatforms is the declared support set of TASK-245 §8-6. It is a
// whitelist because the guarantees behind it — 0600 restricting other users,
// replace-over-open-file, directory fsync — were measured on these two and
// assumed nowhere.
var supportedBridgePlatforms = map[string]bool{"linux": true, "darwin": true}

var configEnvCmd = &cobra.Command{
	Use:   "env",
	Short: "Bridge encrypted secrets into the plaintext env files dva.yml declares",
	Long: `Bridge encrypted secrets into the plaintext env files dva.yml declares.

An env_file entry that declares 'sops_source' names the encrypted file that
produces it. Nothing about that declaration changes how a config loads: DVA never
decrypts on its own, and lifecycle commands never unseal implicitly. Both
subcommands here are explicit, one target at a time.

  dva config env edit     edit the encrypted source with sops
  dva config env unseal   write the decrypted target

DVA does not own age or KMS keys, does not create encrypted sources, and never
prints a decrypted value. See USAGE.md, "config env".`,
}

var configEnvUnsealCmd = &cobra.Command{
	Use:   "unseal [target]",
	Short: "Decrypt an env_file entry's sops_source into its plaintext target",
	Long: `Decrypt an env_file entry's sops_source into its plaintext target.

[target] is the entry's 'path' exactly as dva.yml spells it. It may be omitted
only when a single entry declares sops_source; with several, naming one is
required rather than guessed.

The write is atomic: a 0600 temporary file in the target's own directory receives
sops' output directly, is validated as dotenv, and only then replaces the target.
Any failure leaves an existing target byte-for-byte unchanged.

An existing target is never overwritten without --force. --force covers that case
and no other: it does not bypass the git, symlink, path or platform guards.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		target := ""
		if len(args) == 1 {
			target = args[0]
		}
		force, _ := cmd.Flags().GetBool("force")
		return runEnvUnseal(target, force)
	},
}

var configEnvEditCmd = &cobra.Command{
	Use:   "edit [target]",
	Short: "Edit an env_file entry's encrypted source with sops",
	Long: `Edit an env_file entry's encrypted source with sops.

[target] selects the entry the same way 'unseal' does — by its declared 'path'.
What is opened is the entry's sops_source, never the plaintext target: this
command reads, writes and creates no target at all, so a target left on disk is
stale until you run 'unseal'.

sops owns the editor session and the key material. DVA passes stdin, stdout and
stderr through untouched.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		target := ""
		if len(args) == 1 {
			target = args[0]
		}
		// --json is a root persistent flag, so every subcommand inherits it and
		// "do not register it" is not available as an answer. The contract
		// therefore defines what passing it does: refuse before running an
		// editor, because an interactive session and a stdout document cannot
		// share the same stream.
		if jsonOutput {
			return bridgeErr(codeJSONUnsupported, "edit is interactive and has no --json output")
		}
		if force, _ := cmd.Flags().GetBool("force"); force {
			return bridgeErr(codeForceUnsupported, "edit never writes a target, so --force does not apply")
		}
		return runEnvEdit(target)
	},
}

func init() {
	configEnvUnsealCmd.Flags().Bool("force", false, "Replace an existing plaintext target")

	// Hidden on edit, and only so that `dva config env edit --force` produces the
	// frozen force_unsupported_for_edit code instead of cobra's generic unknown-flag
	// error. It is not part of edit's interface, which is why --help does not list it.
	configEnvEditCmd.Flags().Bool("force", false, "Not applicable to edit")
	_ = configEnvEditCmd.Flags().MarkHidden("force")

	configEnvCmd.AddCommand(configEnvUnsealCmd)
	configEnvCmd.AddCommand(configEnvEditCmd)
	configCmd.AddCommand(configEnvCmd)
}

// resolvedEntry is what the first nine preflight steps produce: a selection that
// has been proved safe to act on, plus the handle every later operation uses.
type resolvedEntry struct {
	entry config.EnvFileConfig
	root  *envRoot
	// anchor is the handle on the target's own directory. Every byte of the
	// write — temp create, recovery sweep, rename, fsync — goes through it, so
	// no component of the declared path is re-resolved after preflight.
	anchor *targetAnchor
	// targetExists distinguishes "created" from "replaced" in the success
	// document. It is recorded during preflight rather than re-stat'ed after the
	// write, when the answer would always be yes.
	targetExists bool
	// outsideRepo drives the §7-2 stderr notice.
	outsideRepo bool
}

// preflight runs TASK-245 §4-2 in its frozen order and stops at the first
// failure. Nothing here decrypts: sops is reached only after every one of these
// steps passes, so a write that will be refused never causes a decryption.
//
// The order is not incidental. Each step is decidable without the information a
// later step gathers, which is what lets the fault matrix assert exactly one
// code for an input that violates several rules at once.
func preflight(c *config.Config, target string, force bool) (*resolvedEntry, error) {
	// 1. platform
	if !supportedBridgePlatforms[bridgeGOOS] {
		return nil, bridgeErr(codeUnsupportedPlatform, "this platform is not supported for env bridge writes")
	}

	// 2. origin provenance
	if err := checkWritableOrigin(c); err != nil {
		return nil, err
	}

	// 3. selector
	entry, err := selectEncryptedEntry(c, target)
	if err != nil {
		return nil, err
	}

	root, err := openEnvRoot(c.FileDir())
	if err != nil {
		return nil, err
	}
	ok := false
	defer func() {
		if !ok {
			root.Close()
		}
	}()

	// 4. path shape and component safety, source before target: the source is the
	// input, and a caller who mis-declared both should hear about the file they
	// were reading from first.
	srcState, srcInfo, err := root.checkPath(entry.SopsSource)
	if err != nil {
		return nil, err
	}
	tgtState, tgtInfo, err := root.checkPath(entry.Path)
	if err != nil {
		return nil, err
	}

	// 5. source is target
	if filepath.Clean(entry.SopsSource) == filepath.Clean(entry.Path) {
		return nil, bridgeErr(codeSourceIsTarget, "%s is both the source and the target", entry.Path)
	}

	// 6. source state
	switch srcState {
	case pathMissingLeaf, pathMissingParent:
		return nil, bridgeErr(codeSourceMissing, "encrypted source %s does not exist", entry.SopsSource)
	case pathPresent:
		if !srcInfo.Mode().IsRegular() {
			return nil, bridgeErr(codeSourceNotRegular, "encrypted source %s is not a regular file", entry.SopsSource)
		}
	}
	// Openability is state, not shape: a source that exists and is regular can
	// still be unreadable, and discovering that only after sops fails would
	// report a decryption problem for a permissions one.
	f, err := root.root.Open(entry.SopsSource)
	if err != nil {
		return nil, bridgeErr(codeSourceUnreadable, "cannot read encrypted source %s", entry.SopsSource)
	}
	_ = f.Close()

	// 7. target parent
	if tgtState == pathMissingParent {
		return nil, bridgeErr(codeTargetParentMissing, "the directory for %s does not exist", entry.Path)
	}

	// The parent is known to exist and to be free of symlinked components, so
	// this is the moment to take the handle on it. Everything after this point —
	// the git question included — is asked about the directory the bytes will
	// actually land in, not about a name that could be re-resolved later.
	anchor, err := root.openTargetAnchor(entry.Path)
	if err != nil {
		switch {
		case errors.Is(err, fs.ErrNotExist):
			return nil, bridgeErr(codeTargetParentMissing, "the directory for %s does not exist", entry.Path)
		case errors.Is(err, fs.ErrPermission):
			return nil, bridgeErr(codePermissionDenied, "permission denied writing %s", entry.Path)
		}
		return nil, err
	}
	defer func() {
		if !ok {
			anchor.Close()
		}
	}()

	// 8. target kind
	if tgtState == pathPresent && !tgtInfo.Mode().IsRegular() {
		return nil, bridgeErr(codeTargetNotRegular, "%s is not a regular file", entry.Path)
	}

	// 9. git
	outside := false
	switch classifyGitTarget(anchor.dir.dir, anchor.leaf) {
	case gitTracked:
		return nil, bridgeErr(codeTargetTracked, "%s is tracked by git; a decrypted file must never be tracked", entry.Path)
	case gitUntrackedNotIgnored:
		return nil, bridgeErr(codeTargetNotIgnored, "%s is not ignored by git; add it to .gitignore first", entry.Path)
	case gitBinaryMissing:
		return nil, bridgeErr(codeGitUnavailable, "%s is inside a git repository but git is not on PATH", entry.Path)
	case gitOutsideRepo:
		outside = true
	}

	// 10. existing target versus --force
	if tgtState == pathPresent && !force {
		return nil, bridgeErr(codeTargetExists, "%s already exists; pass --force to replace it", entry.Path)
	}

	ok = true
	return &resolvedEntry{
		entry:        entry,
		root:         root,
		anchor:       anchor,
		targetExists: tgtState == pathPresent,
		outsideRepo:  outside,
	}, nil
}

func runEnvUnseal(target string, force bool) error {
	c, err := loadConfig()
	if err != nil {
		return err
	}

	resolved, err := preflight(c, target, force)
	if err != nil {
		return err
	}
	defer resolved.root.Close()
	defer resolved.anchor.Close()

	// Recovery runs after preflight, not before: a command that is going to
	// refuse should not also mutate the directory it refused to write in. It
	// sweeps the anchor, which is where this command's temps live.
	resolved.anchor.dir.reclaimStaleTemps(time.Now())

	// 11. sops presence, checked separately from a failed run so "not installed"
	// never reads as "your key is wrong".
	if !bridgeSops.Available() {
		return bridgeErr(codeSopsNotFound, "sops is not installed or not on PATH")
	}

	// 12. decrypt into a temp we own, validate, then replace.
	w, err := resolved.anchor.newTemp()
	if err != nil {
		if errors.Is(err, fs.ErrPermission) {
			return bridgeErr(codePermissionDenied, "permission denied writing %s", resolved.entry.Path)
		}
		return err
	}
	defer w.Abort()

	if err := bridgeSops.Decrypt(resolved.root.abs(resolved.entry.SopsSource), w.File()); err != nil {
		// The child's exit status is deliberately not propagated: sops exits 128
		// on a key failure, and letting that through would make DVA's exit
		// contract a restatement of sops' (§7-3). Its stderr is not echoed
		// either, because it can quote file content.
		return bridgeErr(codeDecryptFailed, "decryption failed for %s", resolved.entry.SopsSource)
	}

	if err := validateDecrypted(resolved, w); err != nil {
		return err
	}

	if err := w.Commit(); err != nil {
		// A post-rename failure is passed through verbatim. Mapping it onto
		// permission_denied would hand the user a code whose whole meaning is
		// "nothing was written", when the target has in fact been replaced.
		if _, isPostRename := errors.AsType[*postRenameError](err); isPostRename {
			return err
		}
		if errors.Is(err, fs.ErrPermission) {
			return bridgeErr(codePermissionDenied, "permission denied writing %s", resolved.entry.Path)
		}
		return err
	}

	result := "created"
	if resolved.targetExists {
		result = "replaced"
	}
	if resolved.outsideRepo {
		// stderr, and not a key in the JSON document: the success shape is frozen
		// and a consumer diffing it must not see a field appear because of where
		// the user happens to keep their project (§7-2).
		fmt.Fprintf(os.Stderr, "note: %s is not inside a git repository; nothing verifies that it stays untracked\n", resolved.entry.Path)
	}
	if jsonOutput {
		return output.PrintJSON(map[string]any{
			"action": "unseal",
			"target": resolved.entry.Path,
			"source": resolved.entry.SopsSource,
			"result": result,
		})
	}
	fmt.Printf("unsealed %s from %s (%s)\n", resolved.entry.Path, resolved.entry.SopsSource, result)
	return nil
}

// validateDecrypted re-reads the temp through DVA's own dotenv parser.
//
// It has to be DVA's parser and not sops' notion of valid dotenv: the file is
// about to be loaded by this tool, so the only question that matters is whether
// this tool can read it. The parser returns a count and a line number and
// nothing else, so validation cannot become a leak.
func validateDecrypted(r *resolvedEntry, w *safeWriter) error {
	// Reopened by name through the same handle rather than rewound, because the
	// write descriptor is open write-only.
	f, err := r.anchor.dir.root.Open(w.name)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	info, err := f.Stat()
	if err != nil {
		return err
	}
	if info.Size() == 0 {
		// sops can exit 0 having produced nothing. Replacing a working target
		// with an empty file would be a silent outage, so an empty payload is a
		// failure even though the child succeeded.
		return bridgeErr(codeEmptyOutput, "sops produced no output for %s", r.entry.SopsSource)
	}
	if _, line, err := config.ValidateDotenvStream(f); err != nil {
		if errors.Is(err, config.ErrDotenvLineTooLong) {
			// Named separately from a syntax failure because the file is not
			// malformed: the line is well formed and larger than DVA reads, and
			// telling the user to go hunting for a typo on it would be wrong.
			return bridgeErr(codeInvalidDotenv,
				"decrypted output has a line longer than %d bytes at line %d", config.MaxDotenvLineBytes, line)
		}
		if line > 0 {
			return bridgeErr(codeInvalidDotenv, "decrypted output is not valid dotenv at line %d", line)
		}
		return bridgeErr(codeInvalidDotenv, "decrypted output is not valid dotenv")
	}
	return nil
}

func runEnvEdit(target string) error {
	c, err := loadConfig()
	if err != nil {
		return err
	}

	if !supportedBridgePlatforms[bridgeGOOS] {
		return bridgeErr(codeUnsupportedPlatform, "this platform is not supported for env bridge writes")
	}
	if err := checkWritableOrigin(c); err != nil {
		return err
	}
	entry, err := selectEncryptedEntry(c, target)
	if err != nil {
		return err
	}

	root, err := openEnvRoot(c.FileDir())
	if err != nil {
		return err
	}
	defer root.Close()

	// edit touches only the source, so only the source's shape and state are
	// checked. The target's git status is irrelevant to a command that will not
	// write it, and refusing on it would block editing a secret because of a file
	// this command never opens.
	state, info, err := root.checkPath(entry.SopsSource)
	if err != nil {
		return err
	}
	switch state {
	case pathMissingLeaf, pathMissingParent:
		return bridgeErr(codeSourceMissing, "encrypted source %s does not exist", entry.SopsSource)
	case pathPresent:
		if !info.Mode().IsRegular() {
			return bridgeErr(codeSourceNotRegular, "encrypted source %s is not a regular file", entry.SopsSource)
		}
	}
	if !bridgeSops.Available() {
		return bridgeErr(codeSopsNotFound, "sops is not installed or not on PATH")
	}

	// One code covers three outcomes here — a key failure, a non-zero editor, and
	// a session the user left unchanged — because the frozen code set has no name
	// for a cancelled edit and inventing one would be a contract change. The
	// message is worded for all three: real sops exits 200 on "File has not
	// changed", which is a cancellation and not a decryption failure, so calling
	// it one would tell the user to go looking at their keys.
	if err := bridgeSops.Edit(root.abs(entry.SopsSource)); err != nil {
		return bridgeErr(codeDecryptFailed, "sops did not complete the editing session for %s; "+
			"the source is unchanged", entry.SopsSource)
	}

	// stderr, so the hint cannot contaminate whatever the editor left on stdout.
	// It names a command that exists in this release, with the selector the user
	// would actually have to type.
	fmt.Fprintf(os.Stderr, "note: %s is now stale; run `dva config env unseal %s` to refresh it\n", entry.Path, entry.Path)
	return nil
}
