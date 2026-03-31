package config

var (
	// Version is the current DVA version (bump manually for releases).
	Version = "0.1.44"
	// Commit is the git commit hash, injected at build time via ldflags.
	Commit = "dev"
	// BuildDate is the build timestamp, injected at build time via ldflags.
	BuildDate = "unknown"
)
