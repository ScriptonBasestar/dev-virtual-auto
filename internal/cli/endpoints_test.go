package cli

import (
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

func TestFilterEndpoints_NoTags(t *testing.T) {
	endpoints := map[string]config.EndpointConfig{
		"api":  {URL: "http://localhost:8080", Label: "API", Tags: []string{"app"}},
		"db":   {URL: "localhost:5432", Label: "DB", Tags: []string{"infra"}},
	}
	result := filterEndpoints(endpoints, nil)
	if len(result) != 2 {
		t.Errorf("expected 2, got %d", len(result))
	}
}

func TestFilterEndpoints_WithTags(t *testing.T) {
	endpoints := map[string]config.EndpointConfig{
		"api":      {URL: "http://localhost:8080", Label: "API", Tags: []string{"app"}},
		"db":       {URL: "localhost:5432", Label: "DB", Tags: []string{"infra"}},
		"frontend": {URL: "http://localhost:3000", Label: "UI", Tags: []string{"app", "ui"}},
	}
	result := filterEndpoints(endpoints, []string{"app"})
	if len(result) != 2 {
		t.Errorf("expected 2 (api + frontend), got %d", len(result))
	}
	if _, ok := result["db"]; ok {
		t.Error("db should be filtered out")
	}
}

func TestFilterEndpoints_NoMatch(t *testing.T) {
	endpoints := map[string]config.EndpointConfig{
		"api": {URL: "http://localhost:8080", Label: "API", Tags: []string{"app"}},
	}
	result := filterEndpoints(endpoints, []string{"monitoring"})
	if len(result) != 0 {
		t.Errorf("expected 0, got %d", len(result))
	}
}

func TestPrintEndpointTable_Empty(t *testing.T) {
	out := captureStdout(t, func() {
		printEndpointTable(nil, nil, nil)
	})
	if out != "" {
		t.Errorf("expected empty output for nil endpoints, got: %s", out)
	}
}

func TestPrintEndpointTable_WithEndpoints(t *testing.T) {
	endpoints := map[string]config.EndpointConfig{
		"api": {
			URL:   "http://localhost:8080",
			Label: "API Server",
			Tags:  []string{"app"},
			Paths: map[string]string{"/health": "Health check"},
		},
		"ssh": {
			URL:   "ssh://git@localhost:2222",
			Label: "Git SSH",
			Tags:  []string{"app"},
		},
	}
	out := captureStdout(t, func() {
		printEndpointTable(endpoints, nil, nil)
	})
	if !strings.Contains(out, "Endpoints:") {
		t.Error("expected Endpoints: header")
	}
	if !strings.Contains(out, "API Server") {
		t.Error("expected API Server label")
	}
	if !strings.Contains(out, "http://localhost:8080") {
		t.Error("expected API URL")
	}
	if !strings.Contains(out, "Git SSH") {
		t.Error("expected Git SSH label")
	}
	if !strings.Contains(out, "/health") {
		t.Error("expected sub-path /health")
	}
}

func TestPrintEndpointTable_WithHealthStatus(t *testing.T) {
	endpoints := map[string]config.EndpointConfig{
		"api": {URL: "http://localhost:8080", Label: "API", Tags: []string{"app"}},
		"ssh": {URL: "ssh://localhost:2222", Label: "SSH", Tags: []string{"app"}},
	}
	hcResults := []HealthCheckResult{
		{Name: "api", Ready: true},
	}
	out := captureStdout(t, func() {
		printEndpointTable(endpoints, nil, hcResults)
	})
	if !strings.Contains(out, "🟢") {
		t.Error("expected green circle status icon for healthy api endpoint")
	}
}

func TestPrintEndpointTable_TagFiltering(t *testing.T) {
	endpoints := map[string]config.EndpointConfig{
		"api": {URL: "http://localhost:8080", Label: "API", Tags: []string{"app"}},
		"db":  {URL: "localhost:5432", Label: "DB", Tags: []string{"infra"}},
	}
	out := captureStdout(t, func() {
		printEndpointTable(endpoints, []string{"app"}, nil)
	})
	if !strings.Contains(out, "API") {
		t.Error("expected API to be shown")
	}
	if strings.Contains(out, "DB") {
		t.Error("DB should be filtered out by tag")
	}
}

func TestShowText_WithEndpoints(t *testing.T) {
	c := &config.Config{
		Endpoints: map[string]config.EndpointConfig{
			"api": {URL: "http://localhost:8080", Label: "API Server"},
			"ssh": {URL: "ssh://localhost:2222", Label: "Git SSH"},
		},
	}
	out := captureStdout(t, func() {
		showText(c)
	})
	if !strings.Contains(out, "Endpoints:") {
		t.Error("showText should display Endpoints section")
	}
	if !strings.Contains(out, "API Server") {
		t.Error("showText should display endpoint label")
	}
}

func TestShowJSON_WithEndpoints(t *testing.T) {
	c := &config.Config{
		Endpoints: map[string]config.EndpointConfig{
			"api": {URL: "http://localhost:8080", Label: "API Server"},
		},
	}
	out := captureStdout(t, func() {
		showJSON(c)
	})
	if !strings.Contains(out, `"endpoints"`) {
		t.Error("showJSON should contain endpoints key")
	}
	if !strings.Contains(out, "API Server") {
		t.Error("showJSON should contain endpoint label")
	}
}
