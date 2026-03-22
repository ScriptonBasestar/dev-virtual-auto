package cli

import (
	"testing"
)

func TestParseServiceInfo_JSONArray(t *testing.T) {
	input := `[
		{"Name":"proj-postgres-1","Service":"postgres","State":"running","Health":"healthy","Publishers":[{"URL":"0.0.0.0","TargetPort":5432,"PublishedPort":11310,"Protocol":"tcp"}]},
		{"Name":"proj-redis-1","Service":"redis","State":"running","Health":"healthy","Publishers":[{"URL":"0.0.0.0","TargetPort":6379,"PublishedPort":11320,"Protocol":"tcp"}]}
	]`

	services, err := parseServiceInfo([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(services))
	}
	if services[0].Service != "postgres" {
		t.Errorf("expected service 'postgres', got '%s'", services[0].Service)
	}
	if services[1].Publishers[0].PublishedPort != 11320 {
		t.Errorf("expected published port 11320, got %d", services[1].Publishers[0].PublishedPort)
	}
}

func TestParseServiceInfo_JSONLines(t *testing.T) {
	input := `{"Name":"proj-postgres-1","Service":"postgres","State":"running","Health":"healthy","Publishers":[]}
{"Name":"proj-redis-1","Service":"redis","State":"running","Health":"","Publishers":[]}`

	services, err := parseServiceInfo([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(services))
	}
	if services[1].Health != "" {
		t.Errorf("expected empty health, got '%s'", services[1].Health)
	}
}

func TestParseServiceInfo_Empty(t *testing.T) {
	services, err := parseServiceInfo([]byte(""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if services != nil {
		t.Errorf("expected nil, got %v", services)
	}
}

func TestAllServicesHealthy_AllHealthy(t *testing.T) {
	services := []ServiceInfo{
		{Service: "postgres", State: "running", Health: "healthy"},
		{Service: "redis", State: "running", Health: "healthy"},
	}
	if !allServicesHealthy(services, nil) {
		t.Error("expected all healthy")
	}
}

func TestAllServicesHealthy_OneUnhealthy(t *testing.T) {
	services := []ServiceInfo{
		{Service: "postgres", State: "running", Health: "healthy"},
		{Service: "redis", State: "running", Health: "unhealthy"},
	}
	if allServicesHealthy(services, nil) {
		t.Error("expected not all healthy")
	}
}

func TestAllServicesHealthy_NoHealthcheck(t *testing.T) {
	services := []ServiceInfo{
		{Service: "postgres", State: "running", Health: ""},
	}
	if !allServicesHealthy(services, nil) {
		t.Error("running without healthcheck should be considered healthy")
	}
}

func TestAllServicesHealthy_NotRunning(t *testing.T) {
	services := []ServiceInfo{
		{Service: "postgres", State: "exited", Health: ""},
	}
	if allServicesHealthy(services, nil) {
		t.Error("exited service should not be healthy")
	}
}

func TestAllServicesHealthy_Empty(t *testing.T) {
	if allServicesHealthy(nil, nil) {
		t.Error("empty services should return false")
	}
}

func TestAllServicesHealthy_Subset(t *testing.T) {
	services := []ServiceInfo{
		{Service: "postgres", State: "running", Health: "healthy"},
		{Service: "redis", State: "running", Health: "unhealthy"},
		{Service: "vault", State: "running", Health: "healthy"},
	}
	// Only requesting postgres and vault (both healthy)
	if !allServicesHealthy(services, []string{"postgres", "vault"}) {
		t.Error("requested subset should be healthy")
	}
	// Requesting redis (unhealthy)
	if allServicesHealthy(services, []string{"redis"}) {
		t.Error("redis is unhealthy")
	}
}

func TestAllServicesHealthy_MissingService(t *testing.T) {
	services := []ServiceInfo{
		{Service: "postgres", State: "running", Health: "healthy"},
	}
	if allServicesHealthy(services, []string{"postgres", "nonexistent"}) {
		t.Error("missing service should return false")
	}
}

func TestExtractServiceNames(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected []string
	}{
		{
			name:     "services only",
			args:     []string{"postgres", "redis"},
			expected: []string{"postgres", "redis"},
		},
		{
			name:     "with boolean flags",
			args:     []string{"-d", "--wait", "postgres"},
			expected: []string{"postgres"},
		},
		{
			name:     "with value flags",
			args:     []string{"-t", "30", "postgres", "--scale", "redis=2", "redis"},
			expected: []string{"postgres", "redis"},
		},
		{
			name:     "flags only",
			args:     []string{"-d", "--wait", "--remove-orphans"},
			expected: nil,
		},
		{
			name:     "empty",
			args:     nil,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractServiceNames(tt.args)
			if len(got) != len(tt.expected) {
				t.Fatalf("expected %v, got %v", tt.expected, got)
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("index %d: expected %q, got %q", i, tt.expected[i], got[i])
				}
			}
		})
	}
}

func TestFormatPortURLs(t *testing.T) {
	tests := []struct {
		name     string
		pubs     []Publisher
		expected string
	}{
		{
			name:     "empty",
			pubs:     nil,
			expected: "",
		},
		{
			name: "unknown target port shows http URL",
			pubs: []Publisher{
				{PublishedPort: 11300, TargetPort: 11300, Protocol: "tcp"},
			},
			expected: "http://localhost:11300",
		},
		{
			name: "well-known HTTP port with label",
			pubs: []Publisher{
				{PublishedPort: 11330, TargetPort: 8200, Protocol: "tcp"},
			},
			expected: "http://localhost:11330 (API)",
		},
		{
			name: "well-known non-HTTP port without http scheme",
			pubs: []Publisher{
				{PublishedPort: 11310, TargetPort: 5432, Protocol: "tcp"},
			},
			expected: "localhost:11310 (PostgreSQL)",
		},
		{
			name: "mixed SMTP and Web UI",
			pubs: []Publisher{
				{PublishedPort: 11350, TargetPort: 1025, Protocol: "tcp"},
				{PublishedPort: 11351, TargetPort: 8025, Protocol: "tcp"},
			},
			expected: "localhost:11350 (SMTP)  http://localhost:11351 (Web UI)",
		},
		{
			name: "skip zero and deduplicate",
			pubs: []Publisher{
				{PublishedPort: 0, TargetPort: 5432, Protocol: "tcp"},
				{PublishedPort: 11310, TargetPort: 5432, Protocol: "tcp"},
				{PublishedPort: 11310, TargetPort: 5432, Protocol: "tcp"},
			},
			expected: "localhost:11310 (PostgreSQL)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatPortURLs(tt.pubs)
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestFormatPorts(t *testing.T) {
	tests := []struct {
		name     string
		pubs     []Publisher
		expected string
	}{
		{
			name:     "empty",
			pubs:     nil,
			expected: "",
		},
		{
			name: "single port",
			pubs: []Publisher{
				{PublishedPort: 11310, TargetPort: 5432, Protocol: "tcp"},
			},
			expected: "11310 → 5432/tcp",
		},
		{
			name: "multiple ports",
			pubs: []Publisher{
				{PublishedPort: 11350, TargetPort: 1025, Protocol: "tcp"},
				{PublishedPort: 11351, TargetPort: 8025, Protocol: "tcp"},
			},
			expected: "11350 → 1025/tcp, 11351 → 8025/tcp",
		},
		{
			name: "skip zero published port",
			pubs: []Publisher{
				{PublishedPort: 0, TargetPort: 5432, Protocol: "tcp"},
				{PublishedPort: 11310, TargetPort: 5432, Protocol: "tcp"},
			},
			expected: "11310 → 5432/tcp",
		},
		{
			name: "deduplicate",
			pubs: []Publisher{
				{PublishedPort: 11310, TargetPort: 5432, Protocol: "tcp"},
				{PublishedPort: 11310, TargetPort: 5432, Protocol: "tcp"},
			},
			expected: "11310 → 5432/tcp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatPorts(tt.pubs)
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}
