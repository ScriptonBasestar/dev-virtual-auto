package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"time"

	"github.com/ScriptonBasestar/dva/internal/config"
	"github.com/spf13/cobra"
)

var configEnvSealCmd = &cobra.Command{
	Use:   "seal [target]",
	Short: "Encrypt a plaintext env_file target into a new sops_source (disabled by default)",
	Long: `Encrypt a plaintext env_file target into a new sops_source.

Off by default. Set env_bridge.allow_seal: true in dva.yml to enable it — see
USAGE.md, "config env".

seal is create-only: it refuses with source_exists when the entry's
sops_source already exists, and has no --force to override that. Use 'edit' to
change an encrypted source that already exists. seal never deletes the
plaintext target it read.

seal takes no key or provider arguments; recipients come entirely from
.sops.yaml's creation_rules, which must have a rule reachable from the source's
own directory up to the config root.

Before writing, seal lists the key *names* the plaintext target declares — never
their values — and asks for confirmation on the controlling terminal. --yes
skips that prompt and is required when there is no controlling terminal.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		target := ""
		if len(args) == 1 {
			target = args[0]
		}
		yes, _ := cmd.Flags().GetBool("yes")
		return runEnvSeal(target, yes)
	},
}

func init() {
	configEnvSealCmd.Flags().Bool("yes", false, "Confirm the listed key names without a controlling terminal prompt")
	configEnvCmd.AddCommand(configEnvSealCmd)
}

// sealResolvedEntry is what sealPreflight produces once every row up to and
// including the .sops.yaml ancestor check has passed.
type sealResolvedEntry struct {
	entry    config.EnvFileConfig
	root     *envRoot
	anchor   *targetAnchor // anchored on the source's own directory, not the target's
	keyNames []string
}

// sealPreflight runs TASK-281 §3-3-1 rows 6-23 in order and stops at the first
// failure. Rows 1-5 (gate, --json, platform, origin, version) run in
// runEnvSeal before this is ever called, exactly as unseal's own preflight
// keeps platform as its own row 1 rather than folding it in here.
func sealPreflight(c *config.Config, target string) (*sealResolvedEntry, error) {
	// 6-7: env_file origin provenance.
	if err := checkWritableOrigin(c); err != nil {
		return nil, err
	}
	// 8-11: selector.
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

	// 12-14: path shape, source before target — the source is what this command
	// creates, and a caller who mis-declared both should hear about it first.
	srcState, _, err := root.checkPath(entry.SopsSource)
	if err != nil {
		return nil, err
	}
	tgtState, tgtInfo, err := root.checkPath(entry.Path)
	if err != nil {
		return nil, err
	}

	// 15: source == target.
	if filepath.Clean(entry.SopsSource) == filepath.Clean(entry.Path) {
		return nil, bridgeErr(codeSourceIsTarget, "%s is both the source and the target", entry.Path)
	}

	// 16: source already exists — seal is create-only, no --force.
	if srcState == pathPresent {
		return nil, bridgeErr(codeSourceExists,
			"encrypted source %s already exists; use `edit` to change it", entry.SopsSource)
	}
	// 17: source's parent directory missing.
	if srcState == pathMissingParent {
		return nil, bridgeErr(codeSourceParentMissing, "the directory for %s does not exist", entry.SopsSource)
	}

	// 18: plaintext target missing.
	if tgtState != pathPresent {
		return nil, bridgeErr(codeSealTargetMissing, "plaintext target %s does not exist", entry.Path)
	}
	// 19: plaintext target not a regular file.
	if !tgtInfo.Mode().IsRegular() {
		return nil, bridgeErr(codeTargetNotRegular, "%s is not a regular file", entry.Path)
	}

	// 20: plaintext target unreadable. 21-22: parse and extract key names —
	// never values — for the confirmation prompt.
	tf, err := root.root.Open(entry.Path)
	if err != nil {
		if errors.Is(err, fs.ErrPermission) {
			return nil, bridgeErr(codePermissionDenied, "permission denied reading %s", entry.Path)
		}
		return nil, err
	}
	names, err := config.DotenvKeyNames(tf)
	_ = tf.Close()
	if err != nil {
		return nil, bridgeErr(codeInvalidDotenvInput, "plaintext target %s is not valid dotenv", entry.Path)
	}
	if len(names) == 0 {
		return nil, bridgeErr(codeEmptyPlaintextInput, "plaintext target %s declares no assignments", entry.Path)
	}

	// Anchor on the source's own directory, not the target's — this is the
	// write side of the operation, and reusing openTargetAnchor here (rather
	// than a second writer) is what TASK-282's implementation notes require.
	// Safe to open now: srcState == pathMissingLeaf is the only state that
	// reaches this line, so the parent is already known to exist.
	anchor, err := root.openTargetAnchor(entry.SopsSource)
	if err != nil {
		switch {
		case errors.Is(err, fs.ErrNotExist):
			return nil, bridgeErr(codeSourceParentMissing, "the directory for %s does not exist", entry.SopsSource)
		case errors.Is(err, fs.ErrPermission):
			return nil, bridgeErr(codePermissionDenied, "permission denied writing %s", entry.SopsSource)
		}
		return nil, err
	}
	defer func() {
		if !ok {
			anchor.Close()
		}
	}()

	// 23: .sops.yaml creation-rule fail-closed check, before sops is ever run.
	if !hasSopsCreationRuleAncestor(root, anchor.rel) {
		return nil, bridgeErr(codeSopsCreationRuleMissing,
			"no .sops.yaml reachable from %s declares a creation rule", entry.SopsSource)
	}

	ok = true
	return &sealResolvedEntry{entry: entry, root: root, anchor: anchor, keyNames: names}, nil
}

// hasSopsCreationRuleAncestor walks from dir (the source's own directory,
// relative to the config root) up through its ancestors to "." inclusive,
// looking for a .sops.yaml. It never walks above the config root: that
// boundary is already what os.Root structurally enforces, and TASK-281 §3-3
// draws the search exactly there ("config root부터 containment 경계까지").
//
// This only checks existence. It does not parse .sops.yaml or evaluate its
// creation_rules against the destination path — DVA does not reimplement
// sops' rule matching (§3-3); that half of row 27 is sops' own failure,
// translated by Encrypt via errSopsCreationRuleMismatch.
func hasSopsCreationRuleAncestor(root *envRoot, dir string) bool {
	for {
		candidate := ".sops.yaml"
		if dir != "." {
			candidate = filepath.Join(dir, ".sops.yaml")
		}
		if info, err := root.root.Lstat(candidate); err == nil && !info.IsDir() {
			return true
		}
		if dir == "." {
			return false
		}
		dir = filepath.Dir(dir)
	}
}

func runEnvSeal(target string, yes bool) error {
	c, err := loadConfig()
	if err != nil {
		return err
	}

	// 1: gate.
	if err := checkSealEnabled(c); err != nil {
		return err
	}
	// 2: --json has no meaning for seal — its confirmation prompt and result
	// are not a document.
	if jsonOutput {
		return bridgeErr(codeJSONUnsupportedForSeal, "seal has no --json output")
	}
	// 3: platform.
	if !supportedBridgePlatforms[bridgeGOOS] {
		return bridgeErr(codeUnsupportedPlatform, "this platform is not supported for env bridge writes")
	}
	// 4-5: env_bridge origin and version.
	if err := checkEnvBridgeOriginAndVersion(c); err != nil {
		return err
	}

	resolved, err := sealPreflight(c, target)
	if err != nil {
		return err
	}
	defer resolved.root.Close()
	defer resolved.anchor.Close()

	resolved.anchor.dir.reclaimStaleTemps(time.Now())

	// 24: sops presence, checked apart from a failed run so "not installed"
	// never reads as a rejected creation rule.
	if !bridgeSops.Available() {
		return bridgeErr(codeSopsNotFound, "sops is not installed or not on PATH")
	}

	// 25-26: confirmation.
	if err := confirmSealKeys(resolved.keyNames, yes); err != nil {
		return err
	}

	w, err := resolved.anchor.newTemp()
	if err != nil {
		// 28: temp create/rename/fsync failures all map to permission_denied —
		// seal's matrix does not carry unseal's postRenameError distinction.
		return bridgeErr(codePermissionDenied, "permission denied writing %s", resolved.entry.SopsSource)
	}
	defer w.Abort()

	plaintext, err := resolved.root.root.Open(resolved.entry.Path)
	if err != nil {
		return bridgeErr(codePermissionDenied, "permission denied reading %s", resolved.entry.Path)
	}
	defer func() { _ = plaintext.Close() }()

	// 27: encrypt. destination is the declared source path, never opened here —
	// it exists only so .sops.yaml's creation_rules match against it.
	if err := bridgeSops.Encrypt(resolved.root.abs(resolved.entry.SopsSource), plaintext, w.File()); err != nil {
		if errors.Is(err, errSopsCreationRuleMismatch) {
			return bridgeErr(codeSopsCreationRuleMissing,
				"no .sops.yaml creation rule matches %s", resolved.entry.SopsSource)
		}
		return bridgeErr(codeEncryptFailed, "encryption failed for %s", resolved.entry.Path)
	}

	if err := w.Commit(); err != nil {
		return bridgeErr(codePermissionDenied, "permission denied writing %s", resolved.entry.SopsSource)
	}

	// 29: success. The plaintext target is deliberately left in place — seal
	// never deletes it (§3-3-1 row 29's note); cleanup is the user's decision.
	fmt.Printf("sealed %s into %s\n", resolved.entry.Path, resolved.entry.SopsSource)
	return nil
}

// confirmSealKeys implements rows 25-26. --yes is a deterministic declaration
// that the caller already reviewed the key names, independent of whether a
// terminal is even present — it is not a bypass flag (§3-3): it discloses
// nothing and opens no permission the gate did not already grant.
func confirmSealKeys(names []string, yes bool) error {
	if yes {
		return nil
	}
	tty, err := bridgeOpenTTY()
	if err != nil {
		return bridgeErr(codeConfirmationRequired,
			"no controlling terminal to confirm sealing keys; rerun with --yes after reviewing them")
	}
	defer func() { _ = tty.Close() }()

	_, _ = fmt.Fprintf(tty, "about to seal %d key(s): %s\n", len(names), strings.Join(names, ", "))
	_, _ = fmt.Fprint(tty, "proceed? [y/N] ")
	reply, _ := bufio.NewReader(tty).ReadString('\n')
	reply = strings.TrimSpace(strings.ToLower(reply))
	if reply != "y" && reply != "yes" {
		// Codeless on purpose, the same way targetAnchor.movedErr is: a
		// declined confirmation is not a member of the frozen code set
		// (§3-3-1 row 26), and reusing an unrelated code would misname it.
		return errors.New("env bridge: sealing cancelled; nothing was written")
	}
	return nil
}
