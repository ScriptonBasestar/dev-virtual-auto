package config

// MinScaffoldVersion is the compatibility floor `dva init` writes into a new
// dva.yml: the oldest DVA that understands the config init emits (the multi-runner
// stack model, 2e25daf). It is deliberately not Version. `version:` states what a
// config requires of its reader, not which binary produced it — scaffolding the
// running version would make every new config refuse to load on any older DVA,
// ratcheting the floor upward on each release. Raise this only when init starts
// emitting something an older DVA cannot parse.
//
// Unlike Version it is a const: no build may inject a different floor.
const MinScaffoldVersion = "0.1.44"

var (
	// Version is the current DVA version (bump manually for releases).
	Version = "0.1.45"
	// Commit is the git commit hash, injected at build time via ldflags.
	Commit = "dev"
	// BuildDate is the build timestamp, injected at build time via ldflags.
	BuildDate = "unknown"
)
