package cli

import (
	"path/filepath"

	"github.com/spf13/cobra"
)

var configEnvShowCmd = &cobra.Command{
	Use:   "show [target]",
	Short: "Decrypt an env_file entry's sops_source to the controlling terminal (disabled by default)",
	Long: `Decrypt an env_file entry's sops_source and print it to the controlling
terminal only.

Off by default. Set env_bridge.allow_show: true in dva.yml to enable it — see
USAGE.md, "config env".

Output goes to /dev/tty and nowhere else: not stdout, so '>', '|' and '$(...)'
cannot capture it; not a --json document, which show does not support; not a
log, an error message, or a temporary filename. Gating this command open opens
none of those other paths — TASK-245's redaction rules stay absolute
everywhere else.

show refuses in an environment that advertises itself as an automated agent
(CLAUDECODE, CLAUDE_CODE_ENTRYPOINT or AI_AGENT set). This is advisory, not a
security boundary — no caller identity is authenticated here — and it ships
without a bypass flag.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		target := ""
		if len(args) == 1 {
			target = args[0]
		}
		return runEnvShow(target)
	},
}

func init() {
	configEnvCmd.AddCommand(configEnvShowCmd)
}

// runEnvShow implements TASK-281 §3-4-1's ordering. show's own checks (gate,
// --json, agent detection, tty) come before the shared preflight subset —
// deliberately the reverse of seal/unseal, which run platform first — because
// §3-6 wants the output-safety questions answered before any config detail is
// even inspected.
func runEnvShow(target string) error {
	c, err := loadConfig()
	if err != nil {
		return err
	}

	// 1: gate.
	if err := checkShowEnabled(c); err != nil {
		return err
	}
	// 2: --json.
	if jsonOutput {
		return bridgeErr(codeJSONUnsupportedForShow, "show writes to the controlling terminal and has no --json output")
	}
	// 3: advisory agent-environment detection.
	if detectAgentEnvironment() {
		return bridgeErr(codeAgentEnvironmentDetect, "show is not available in an automated agent environment")
	}
	// 4: controlling terminal.
	tty, err := bridgeOpenTTY()
	if err != nil {
		return bridgeErr(codeNoControllingTerminal, "no controlling terminal available for show's output")
	}
	defer func() { _ = tty.Close() }()

	// 5: preflight subset — platform, env_bridge origin/version, env_file
	// origin provenance, selector, source path shape, source==target, source
	// state. No target-parent, target-kind or git check: show creates nothing,
	// so there is no write-side condition to ask about.
	if !supportedBridgePlatforms[bridgeGOOS] {
		return bridgeErr(codeUnsupportedPlatform, "this platform is not supported for env bridge writes")
	}
	if err := checkEnvBridgeOriginAndVersion(c); err != nil {
		return err
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

	srcState, srcInfo, err := root.checkPath(entry.SopsSource)
	if err != nil {
		return err
	}
	if filepath.Clean(entry.SopsSource) == filepath.Clean(entry.Path) {
		return bridgeErr(codeSourceIsTarget, "%s is both the source and the target", entry.Path)
	}
	switch srcState {
	case pathMissingLeaf, pathMissingParent:
		return bridgeErr(codeSourceMissing, "encrypted source %s does not exist", entry.SopsSource)
	case pathPresent:
		if !srcInfo.Mode().IsRegular() {
			return bridgeErr(codeSourceNotRegular, "encrypted source %s is not a regular file", entry.SopsSource)
		}
	}
	f, err := root.root.Open(entry.SopsSource)
	if err != nil {
		return bridgeErr(codeSourceUnreadable, "cannot read encrypted source %s", entry.SopsSource)
	}
	_ = f.Close()

	if !bridgeSops.Available() {
		return bridgeErr(codeSopsNotFound, "sops is not installed or not on PATH")
	}

	// 6: decrypt straight to the terminal descriptor. Same shape as unseal's
	// write — the child's stdout is the destination file, so the plaintext
	// never exists as a string inside this process.
	if err := bridgeSops.Decrypt(root.abs(entry.SopsSource), tty); err != nil {
		return bridgeErr(codeDecryptFailed, "decryption failed for %s", entry.SopsSource)
	}
	return nil
}
