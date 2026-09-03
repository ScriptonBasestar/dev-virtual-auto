//go:build !windows

package cli

import (
	"fmt"
	"os"
	"syscall"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// bridgeEnabledYAML is simpleBridgeYAML's counterpart for seal/show: a version
// satisfying EnvBridgeIntroducedVersion and a root env_bridge: block. Both
// booleans are explicit rather than defaulted so a row that means to leave one
// off (e.g. allow_seal only) is legible at the call site. The target is always
// ".env" — no row needs a different one, unlike simpleBridgeYAML's callers.
func bridgeEnabledYAML(source string, allowSeal, allowShow bool) string {
	return fmt.Sprintf(
		"version: \"%s\"\nenv_bridge:\n  allow_seal: %t\n  allow_show: %t\nenv_file:\n  - {path: .env, sops_source: %s}\n",
		config.EnvBridgeIntroducedVersion, allowSeal, allowShow, source)
}

// sopsCreationRuleYAML is a minimal .sops.yaml with one matching rule. seal's
// preflight only checks that a file exists on the ancestor walk (§3-3); it
// never parses this content, so the exact recipient is not load-bearing.
const sopsCreationRuleYAML = "creation_rules:\n  - path_regex: \\.enc\\.env$\n    age: age1exampleexampleexampleexampleexampleexampleexampleexamplex\n"

// ttyPipe returns two ends of one full-duplex descriptor standing in for
// /dev/tty: dvaSide is what a test hands to withTTY, testSide is what the test
// itself reads and writes. A plain os.Pipe cannot play this role because
// confirmSealKeys both writes a prompt and reads a reply through the *same*
// *os.File bridgeOpenTTY returns, and a pipe's two ends are one-directional
// each; a socketpair's fd is bidirectional like a real terminal's.
//
// syscall.Socketpair has no windows equivalent, so this lives in its own
// build-tagged file rather than in config_env_fixture_test.go. seal and show
// are both linux/darwin-only (supportedBridgePlatforms in config_env.go), and
// every test that calls ttyPipe (config_env_seal_test.go, config_env_show_test.go)
// carries the same //go:build !windows tag, so nothing outside this platform
// pair ever references it.
func ttyPipe(t *testing.T) (dvaSide, testSide *os.File) {
	t.Helper()
	fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		t.Fatalf("socketpair: %v", err)
	}
	dvaSide = os.NewFile(uintptr(fds[0]), "tty-dva")
	testSide = os.NewFile(uintptr(fds[1]), "tty-test")
	t.Cleanup(func() {
		_ = dvaSide.Close()
		_ = testSide.Close()
	})
	return dvaSide, testSide
}

// withTTY makes bridgeOpenTTY succeed with side for the rest of the test — the
// row means to exercise the "a controlling terminal is present" branch. Pair
// it with ttyPipe to also observe what DVA wrote there.
func (f *bridgeFixture) withTTY(side *os.File) {
	f.t.Helper()
	bridgeOpenTTY = func() (*os.File, error) { return side, nil }
}

// defaultSealFixture is seal's happy path: env_bridge on for seal, a plaintext
// target ready to encrypt, no source yet, and a .sops.yaml reachable from the
// source's own directory.
func defaultSealFixture(t *testing.T) *bridgeFixture {
	t.Helper()
	return newBridgeFixture(t, bridgeEnabledYAML("secrets.env.enc", true, false),
		map[string]string{".env": bridgePayload(), ".sops.yaml": sopsCreationRuleYAML})
}

// defaultShowFixture is show's happy path: env_bridge on for show, an
// encrypted source already on disk, no plaintext target (show never creates
// one).
func defaultShowFixture(t *testing.T) *bridgeFixture {
	t.Helper()
	return newBridgeFixture(t, bridgeEnabledYAML("secrets.env.enc", false, true),
		map[string]string{"secrets.env.enc": "ENC"})
}
