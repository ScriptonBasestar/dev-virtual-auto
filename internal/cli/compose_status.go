package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/ScriptonBasestar/dva/internal/config"
	"github.com/ScriptonBasestar/dva/internal/output"
)

// ServiceInfo represents a service from docker compose ps JSON output.
type ServiceInfo struct {
	Name       string      `json:"Name"`
	Service    string      `json:"Service"`
	State      string      `json:"State"`
	Health     string      `json:"Health"`
	Publishers []Publisher `json:"Publishers"`
}

// Publisher represents a port mapping from docker compose ps JSON output.
type Publisher struct {
	URL           string `json:"URL"`
	TargetPort    int    `json:"TargetPort"`
	PublishedPort int    `json:"PublishedPort"`
	Protocol      string `json:"Protocol"`
}

// queryComposeServices runs docker compose ps --format json and parses the result.
func queryComposeServices(e *config.Environment, c *config.Config) ([]ServiceInfo, error) {
	composeCmd, composeArgs := buildComposeArgs(e, c, []string{"ps", "--format", "json"})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, composeCmd, composeArgs...).Output()
	if err != nil {
		return nil, err
	}
	return parseServiceInfo(out)
}

// parseServiceInfo parses docker compose ps JSON output.
// Handles both JSON array and JSON lines formats.
func parseServiceInfo(data []byte) ([]ServiceInfo, error) {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return nil, nil
	}

	// Try JSON array first
	var services []ServiceInfo
	if err := json.Unmarshal(data, &services); err == nil {
		return services, nil
	}

	// Fallback: JSON lines format
	lines := strings.Split(trimmed, "\n")
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var svc ServiceInfo
		if err := json.Unmarshal([]byte(line), &svc); err != nil {
			return nil, fmt.Errorf("line %d: failed to parse service info: %w", i+1, err)
		}
		services = append(services, svc)
	}
	return services, nil
}

// allServicesHealthy checks if all requested services are running and healthy.
// If requestedServices is empty, checks all services.
// Services without a healthcheck (Health=="") pass if State is "running".
func allServicesHealthy(services []ServiceInfo, requestedServices []string) bool {
	if len(services) == 0 {
		return false
	}

	if len(requestedServices) == 0 {
		for _, s := range services {
			if !isServiceHealthy(s) {
				return false
			}
		}
		return true
	}

	// Build lookup map
	svcMap := make(map[string]ServiceInfo, len(services))
	for _, s := range services {
		svcMap[s.Service] = s
	}

	for _, name := range requestedServices {
		s, ok := svcMap[name]
		if !ok {
			return false
		}
		if !isServiceHealthy(s) {
			return false
		}
	}
	return true
}

func isServiceHealthy(s ServiceInfo) bool {
	if s.State != "running" {
		return false
	}
	// No healthcheck defined → running is enough
	if s.Health == "" {
		return true
	}
	return s.Health == "healthy"
}

// formatPorts formats publisher list into a human-readable string.
func formatPorts(publishers []Publisher) string {
	if len(publishers) == 0 {
		return ""
	}
	seen := make(map[string]bool)
	var parts []string
	for _, p := range publishers {
		if p.PublishedPort == 0 {
			continue
		}
		key := fmt.Sprintf("%d:%d/%s", p.PublishedPort, p.TargetPort, p.Protocol)
		if seen[key] {
			continue
		}
		seen[key] = true
		parts = append(parts, fmt.Sprintf("%d → %d/%s", p.PublishedPort, p.TargetPort, p.Protocol))
	}
	return strings.Join(parts, ", ")
}

// portHint describes a well-known container port.
type portHint struct {
	Label string // human-readable role, e.g. "Web UI", "SMTP"
	HTTP  bool   // true if the port speaks HTTP(S)
}

// wellKnownPorts maps container-internal (target) ports to their hints.
// Treat as read-only; Go does not support const maps.
var wellKnownPorts = map[int]portHint{
	80:    {"HTTP", true},
	443:   {"HTTPS", true},
	1025:  {"SMTP", false},
	2222:  {"SSH", false},
	3000:  {"HTTP", true},
	3306:  {"MySQL", false},
	4222:  {"NATS", false},
	5432:  {"PostgreSQL", false},
	5672:  {"AMQP", false},
	6379:  {"Redis", false},
	8025:  {"Web UI", true},
	8080:  {"HTTP", true},
	8200:  {"API", true},
	8222:  {"Monitor", true},
	9090:  {"HTTP", true},
	9092:  {"Kafka", false},
	15672: {"Management", true},
	27017: {"MongoDB", false},
}

// formatPortURLs formats publisher list as multi-line URL strings with role labels and sub-paths.
// portConfigs provides user-defined labels/paths keyed by published (host) port; may be nil.
func formatPortURLs(publishers []Publisher, portConfigs map[int]config.PortConfig) []string {
	if len(publishers) == 0 {
		return nil
	}
	seen := make(map[int]bool)
	var lines []string
	for _, p := range publishers {
		if p.PublishedPort == 0 || seen[p.PublishedPort] {
			continue
		}
		seen[p.PublishedPort] = true

		// Determine label and HTTP scheme: user config takes priority over wellKnownPorts
		var label string
		isHTTP := true
		pc, hasConfig := portConfigs[p.PublishedPort]
		if hasConfig && pc.Label != "" {
			label = pc.Label
			if pc.HTTP != nil {
				isHTTP = *pc.HTTP
			}
			// else: isHTTP stays true (HTTP default for labeled ports)
		} else if hint, known := wellKnownPorts[p.TargetPort]; known {
			label = hint.Label
			isHTTP = hint.HTTP
		} else {
			// Unknown port, no config — assume HTTP, no label
			isHTTP = true
		}

		// Build URL line
		var urlLine string
		switch {
		case label != "" && isHTTP:
			urlLine = fmt.Sprintf("http://localhost:%d (%s)", p.PublishedPort, label)
		case label != "":
			urlLine = fmt.Sprintf("localhost:%d (%s)", p.PublishedPort, label)
		default:
			urlLine = fmt.Sprintf("http://localhost:%d", p.PublishedPort)
		}
		lines = append(lines, urlLine)

		// Append sub-path lines if configured
		if hasConfig && len(pc.Paths) > 0 {
			// Sort paths for stable output
			paths := make([]string, 0, len(pc.Paths))
			for path := range pc.Paths {
				paths = append(paths, path)
			}
			sort.Strings(paths)

			// Calculate max path length for alignment
			maxLen := 0
			for _, path := range paths {
				if len(path) > maxLen {
					maxLen = len(path)
				}
			}
			for _, path := range paths {
				lines = append(lines, fmt.Sprintf("  %-*s  %s", maxLen, path, pc.Paths[path]))
			}
		}
	}
	return lines
}

// printServiceTable prints a formatted table of services with ports.
// svcConfigs provides per-service port labels/paths from dva.yml; may be nil.
func printServiceTable(services []ServiceInfo, projectName string, alreadyRunning bool, svcConfigs map[string]config.ServiceTagConfig) {
	if alreadyRunning {
		fmt.Printf("\n[ok] Services already running (project: %s)\n\n", projectName)
	} else {
		fmt.Printf("\n[+] Services started (project: %s)\n\n", projectName)
	}

	if len(services) == 0 {
		return
	}

	var buf strings.Builder
	tw := tabwriter.NewWriter(&buf, 2, 0, 3, ' ', 0)
	fmt.Fprintf(tw, "  SERVICE\tSTATUS\tURL\n")
	for _, s := range services {
		status := s.State
		if s.Health != "" {
			status = s.Health
		}
		var portConfigs map[int]config.PortConfig
		if sc, ok := svcConfigs[s.Service]; ok {
			portConfigs = sc.Ports
		}
		lines := formatPortURLs(s.Publishers, portConfigs)
		if len(lines) == 0 {
			fmt.Fprintf(tw, "  %s\t%s\t\n", s.Service, status)
		} else {
			fmt.Fprintf(tw, "  %s\t%s\t%s\n", s.Service, status, lines[0])
			for _, line := range lines[1:] {
				fmt.Fprintf(tw, "  \t\t%s\n", line)
			}
		}
	}
	tw.Flush()
	fmt.Print(buf.String())
	fmt.Println()
}

// printServiceJSON outputs services in JSON format.
func printServiceJSON(services []ServiceInfo, projectName string, alreadyRunning bool, healthChecks []HealthCheckResult) error {
	data := map[string]any{
		"project":         projectName,
		"already_running": alreadyRunning,
		"services":        services,
	}
	if len(healthChecks) > 0 {
		data["health_checks"] = healthChecks
	}
	return output.PrintJSON(data)
}

// printRelatedServiceHints checks running services against their "related" config
// and prints warnings for related services that are not currently running.
func printRelatedServiceHints(services []ServiceInfo, svcConfigs map[string]config.ServiceTagConfig) {
	if len(svcConfigs) == 0 {
		return
	}

	// Build set of running service names
	running := make(map[string]bool, len(services))
	for _, s := range services {
		running[s.Service] = true
	}

	// Check each running service for missing related services
	var hints []string
	seen := make(map[string]bool)
	for _, s := range services {
		sc, ok := svcConfigs[s.Service]
		if !ok || len(sc.Related) == 0 {
			continue
		}
		for _, rel := range sc.Related {
			if running[rel] || seen[rel] {
				continue
			}
			seen[rel] = true
			hint := fmt.Sprintf("  %s -> related service '%s' is not running", s.Service, rel)
			if sc.Hint != "" {
				hint += fmt.Sprintf("\n    Hint: %s", sc.Hint)
			}
			hint += fmt.Sprintf("\n    Run:  dva up %s", rel)
			hints = append(hints, hint)
		}
	}

	if len(hints) > 0 {
		fmt.Fprintf(os.Stderr, "Related Services:\n")
		for _, h := range hints {
			fmt.Fprintln(os.Stderr, h)
		}
		fmt.Fprintln(os.Stderr)
	}
}

// extractServiceNames extracts positional service names from args,
// skipping flags (args starting with '-') and their values.
func extractServiceNames(args []string) []string {
	var names []string
	skipNext := false
	for _, a := range args {
		if skipNext {
			skipNext = false
			continue
		}
		if strings.HasPrefix(a, "-") {
			// Flags that take a space-separated value (e.g., "-t 30", "--scale redis=2").
			// Equals-sign forms (e.g., "--timeout=30") are single tokens and handled implicitly.
			if a == "-t" || a == "--timeout" || a == "--scale" {
				skipNext = true
			}
			continue
		}
		names = append(names, a)
	}
	return names
}
