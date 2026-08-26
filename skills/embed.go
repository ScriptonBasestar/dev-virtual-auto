// Package skills exposes the canonical DVA Agent Skills bundled into the CLI.
package skills

import "embed"

// Files contains the portable skill directories shipped by DVA.
//
//go:embed dva dva-config
var Files embed.FS

// Names is the deterministic installation order for bundled skills.
var Names = []string{"dva", "dva-config"}
