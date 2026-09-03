package cli

import (
	"bytes"
	"os"
	"os/exec"
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
}

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
