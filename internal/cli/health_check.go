package cli

// HealthCheckResult holds the outcome of a single health check.
//
// The actual probes (HTTP/TCP) live in the lifecycle package as
// lifecycle.CheckHTTP / lifecycle.CheckTCP; this package reuses them so the
// endpoint table and the app table cannot diverge on how "reachable" is judged.
type HealthCheckResult struct {
	Name      string `json:"name"`
	Ready     bool   `json:"ready"`
	Started   bool   `json:"started,omitempty"`
	StartHint string `json:"start_hint,omitempty"`
}
