package cli

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"strings"
)

// sopsRunner is the injectable child-process seam. The fault matrix drives every
// failure branch through a fake; TestConfigEnvRealSOPS drives the same code
// through the real binary so the fake cannot drift from what sops does.
type sopsRunner interface {
	// Available reports whether the binary can be found at all.
	Available() bool
	// Decrypt writes plaintext dotenv into out and returns an error on any
	// non-zero exit. The caller owns out; the runner never creates a file.
	Decrypt(source string, out *os.File) error
	// Edit runs an interactive editing session on the encrypted source.
	Edit(source string) error
	// Encrypt reads plaintext dotenv from in and writes encrypted dotenv to out.
	// destination is the source path as dva.yml declares it — never opened by
	// this call — passed only so sops' `.sops.yaml` creation-rule matching
	// evaluates against the file being created rather than /dev/stdin. Returns
	// errSopsCreationRuleMismatch when sops' own diagnostic identifies a missing
	// or non-matching creation rule (seal §3-3 folds this into the same code as
	// a missing `.sops.yaml`, since DVA does not reimplement rule matching).
	Encrypt(destination string, in *os.File, out *os.File) error
}

// errSopsCreationRuleMismatch is a sentinel a sopsRunner.Encrypt implementation
// returns (wrapped or bare) when sops refused because no creation rule matches
// the destination path, distinct from every other encryption failure.
var errSopsCreationRuleMismatch = errors.New("sops: no matching creation rule")

// sopsStderrLimit caps what is read from the child. sops stderr is never echoed
// (§7-4), but an unbounded read of a misbehaving child is still a way to spend
// memory, and the buffer exists only so a debug log can record that something
// was said, not what.
const sopsStderrLimit = 8 << 10

type realSops struct{}

func (realSops) Available() bool {
	_, err := exec.LookPath("sops")
	return err == nil
}

// Decrypt runs sops by argv with no shell, and states the dotenv format on both
// sides rather than letting sops infer it from the file extension — an
// `.env.enc` name tells sops nothing, and a wrong guess would silently produce
// a differently-shaped file.
//
// out is handed to the child as its stdout, so decrypted bytes go from the
// child straight into the 0600 temp descriptor without passing through this
// process. That is what makes "no secret in DVA output" a property of the
// data path and not of careful logging.
func (realSops) Decrypt(source string, out *os.File) error {
	cmd := exec.Command("sops", "decrypt", "--input-type", "dotenv", "--output-type", "dotenv", source)
	cmd.Stdout = out
	var stderr bytes.Buffer
	cmd.Stderr = &limitedWriter{w: &stderr, remaining: sopsStderrLimit}
	cmd.Stdin = nil
	return cmd.Run()
}

// Edit is pure passthrough: sops owns the editor session, so stdin, stdout and
// stderr are the terminal's. DVA neither reads the plaintext nor writes a target
// here — `unseal` is the only writer.
func (realSops) Edit(source string) error {
	cmd := exec.Command("sops", "edit", "--input-type", "dotenv", "--output-type", "dotenv", source)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Encrypt runs sops by argv with no shell, reading plaintext from /dev/stdin
// rather than a real path so the only file sops is ever told to touch is the
// destination it is about to create. `--filename-override` is what makes
// `.sops.yaml`'s creation_rules match against that destination instead of the
// literal "/dev/stdin" — seal never has a plaintext path that already carries
// the destination's name, since the plaintext is the *target*, not the source.
//
// sops' stderr is captured, capped, and never echoed (§7-4): DVA does not
// reimplement `.sops.yaml` rule matching, so a missing-rule failure is
// recognized only by pattern-matching sops' own diagnostic text — a heuristic,
// not a re-derivation, and the reason a false negative here still fails closed
// as codeEncryptFailed rather than silently succeeding.
func (realSops) Encrypt(destination string, in *os.File, out *os.File) error {
	cmd := exec.Command("sops", "encrypt", "--input-type", "dotenv", "--output-type", "dotenv",
		"--filename-override", destination, "/dev/stdin")
	cmd.Stdin = in
	cmd.Stdout = out
	var stderr bytes.Buffer
	cmd.Stderr = &limitedWriter{w: &stderr, remaining: sopsStderrLimit}
	err := cmd.Run()
	if err != nil && strings.Contains(strings.ToLower(stderr.String()), "creation rule") {
		return errSopsCreationRuleMismatch
	}
	return err
}

// limitedWriter discards past a cap instead of failing, so a chatty child cannot
// turn its own verbosity into a write error the parent would report as the cause.
type limitedWriter struct {
	w         *bytes.Buffer
	remaining int
}

func (l *limitedWriter) Write(p []byte) (int, error) {
	if l.remaining > 0 {
		n := min(len(p), l.remaining)
		l.w.Write(p[:n])
		l.remaining -= n
	}
	return len(p), nil
}

var bridgeSops sopsRunner = realSops{}
