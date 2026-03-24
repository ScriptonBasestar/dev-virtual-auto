package cli

import (
	"testing"
)

func TestNewProgress(t *testing.T) {
	p := newProgress(true)
	if p == nil {
		t.Fatal("newProgress returned nil")
	}
	if !p.verbose {
		t.Error("verbose should be true")
	}
	if p.stopped {
		t.Error("stopped should be false initially")
	}
}

func TestProgress_VerboseMode(t *testing.T) {
	p := newProgress(true)
	// In verbose mode, Start/Update/Stop should not panic
	p.Start("initializing")
	p.Update("processing")
	p.Stop()

	// Double stop should not panic
	p.Stop()
}

func TestProgress_NonVerboseMode(t *testing.T) {
	p := newProgress(false)
	p.Start("working")
	p.Update("processing")
	// Give spinner goroutine a moment to start
	p.Stop()

	if !p.stopped {
		t.Error("should be stopped")
	}

	// Double stop should not panic
	p.Stop()
}

func TestProgress_StopWithMessage(t *testing.T) {
	p := newProgress(true)
	p.Start("working")
	p.StopWithMessage("done")

	if !p.stopped {
		t.Error("should be stopped after StopWithMessage")
	}
}
