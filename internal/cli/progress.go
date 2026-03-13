package cli

import (
	"fmt"
	"os"
	"sync"
	"time"
)

// spinnerFrames is the braille spinner animation sequence.
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// progress reports init progress in two modes:
// - default: in-place spinner on a single line (overwritten via \r)
// - verbose: each message printed as a new line
type progress struct {
	verbose bool
	mu      sync.Mutex
	msg     string
	done    chan struct{}
	stopped bool
}

// newProgress creates a progress reporter.
func newProgress(verbose bool) *progress {
	return &progress{
		verbose: verbose,
		done:    make(chan struct{}),
	}
}

// Start begins the spinner goroutine (no-op in verbose mode).
func (p *progress) Start(initialMsg string) {
	p.msg = initialMsg
	if p.verbose {
		fmt.Fprintf(os.Stderr, "  %s\n", initialMsg)
		return
	}
	go p.spin()
}

// Update changes the displayed message.
func (p *progress) Update(msg string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.msg = msg
	if p.verbose {
		fmt.Fprintf(os.Stderr, "  %s\n", msg)
	}
}

// Stop terminates the spinner and clears the line.
func (p *progress) Stop() {
	p.mu.Lock()
	if p.stopped {
		p.mu.Unlock()
		return
	}
	p.stopped = true
	p.mu.Unlock()

	if !p.verbose {
		close(p.done)
		// Clear spinner line
		fmt.Fprintf(os.Stderr, "\r\033[K")
	}
}

// StopWithMessage terminates and prints a final status line.
func (p *progress) StopWithMessage(msg string) {
	p.Stop()
	fmt.Fprintf(os.Stderr, "%s\n", msg)
}

func (p *progress) spin() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	frame := 0
	for {
		select {
		case <-p.done:
			return
		case <-ticker.C:
			p.mu.Lock()
			msg := p.msg
			p.mu.Unlock()
			fmt.Fprintf(os.Stderr, "\r\033[K%s %s", spinnerFrames[frame%len(spinnerFrames)], msg)
			frame++
		}
	}
}
