package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
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

// formatPortURLs formats publisher list as clickable URLs with role labels.
func formatPortURLs(publishers []Publisher) string {
	if len(publishers) == 0 {
		return ""
	}
	seen := make(map[int]bool)
	var parts []string
	for _, p := range publishers {
		if p.PublishedPort == 0 || seen[p.PublishedPort] {
			continue
		}
		seen[p.PublishedPort] = true

		hint, known := wellKnownPorts[p.TargetPort]
		switch {
		case known && hint.HTTP:
			parts = append(parts, fmt.Sprintf("http://localhost:%d (%s)", p.PublishedPort, hint.Label))
		case known:
			parts = append(parts, fmt.Sprintf("localhost:%d (%s)", p.PublishedPort, hint.Label))
		default:
			parts = append(parts, fmt.Sprintf("http://localhost:%d", p.PublishedPort))
		}
	}
	return strings.Join(parts, "  ")
}

// printServiceTable prints a formatted table of services with ports.
func printServiceTable(services []ServiceInfo, projectName string, alreadyRunning bool) {
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
		ports := formatPortURLs(s.Publishers)
		fmt.Fprintf(tw, "  %s\t%s\t%s\n", s.Service, status, ports)
	}
	tw.Flush()
	fmt.Print(buf.String())
	fmt.Println()
}

// printServiceJSON outputs services in JSON format.
func printServiceJSON(services []ServiceInfo, projectName string, alreadyRunning bool) error {
	data := map[string]any{
		"project":         projectName,
		"already_running": alreadyRunning,
		"services":        services,
	}
	return output.PrintJSON(data)
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
