package cli

import (
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

func TestPrintServiceTable_Normal(t *testing.T) {
	services := []ServiceInfo{
		{
			Service: "postgres",
			State:   "running",
			Health:  "healthy",
			Publishers: []Publisher{
				{PublishedPort: 5432, TargetPort: 5432, Protocol: "tcp"},
			},
		},
		{
			Service: "redis",
			State:   "running",
			Health:  "",
		},
	}
	output := captureStdout(t, func() {
		printServiceTable(services, "myproject", false, nil)
	})
	if !strings.Contains(output, "myproject") {
		t.Error("should contain project name")
	}
	if !strings.Contains(output, "postgres") {
		t.Error("should contain postgres service")
	}
	if !strings.Contains(output, "redis") {
		t.Error("should contain redis service")
	}
	if !strings.Contains(output, "SERVICE") {
		t.Error("should contain header")
	}
}

func TestPrintServiceTable_AlreadyRunning(t *testing.T) {
	services := []ServiceInfo{
		{Service: "web", State: "running", Health: "healthy"},
	}
	output := captureStdout(t, func() {
		printServiceTable(services, "proj", true, nil)
	})
	if !strings.Contains(output, "already running") {
		t.Error("should indicate already running")
	}
}

func TestPrintServiceTable_Empty(t *testing.T) {
	output := captureStdout(t, func() {
		printServiceTable(nil, "proj", false, nil)
	})
	if !strings.Contains(output, "started") {
		t.Error("should contain started header even with empty services")
	}
}

func TestPrintServiceTable_WithPortConfig(t *testing.T) {
	services := []ServiceInfo{
		{
			Service: "api",
			State:   "running",
			Health:  "healthy",
			Publishers: []Publisher{
				{PublishedPort: 8080, TargetPort: 8080, Protocol: "tcp"},
			},
		},
	}
	svcConfigs := map[string]config.ServiceTagConfig{
		"api": {
			Ports: map[int]config.PortConfig{
				8080: {Label: "REST API"},
			},
		},
	}
	output := captureStdout(t, func() {
		printServiceTable(services, "proj", false, svcConfigs)
	})
	if !strings.Contains(output, "REST API") {
		t.Error("should contain port label from config")
	}
}

func TestPrintServiceJSON_Basic(t *testing.T) {
	services := []ServiceInfo{
		{Service: "web", State: "running", Health: "healthy"},
	}
	output := captureStdout(t, func() {
		printServiceJSON(services, "myproject", false, nil)
	})
	if !strings.Contains(output, "myproject") {
		t.Error("JSON should contain project name")
	}
	if !strings.Contains(output, "web") {
		t.Error("JSON should contain service name")
	}
}

func TestPrintServiceJSON_WithHealthChecks(t *testing.T) {
	services := []ServiceInfo{
		{Service: "db", State: "running", Health: "healthy"},
	}
	hcResults := []HealthCheckResult{
		{Name: "pg", Ready: true},
	}
	output := captureStdout(t, func() {
		printServiceJSON(services, "proj", true, hcResults)
	})
	if !strings.Contains(output, "health_checks") {
		t.Error("JSON should contain health_checks when provided")
	}
	if !strings.Contains(output, "already_running") {
		t.Error("JSON should contain already_running field")
	}
}

func TestPrintServiceJSON_NilServices(t *testing.T) {
	output := captureStdout(t, func() {
		printServiceJSON(nil, "proj", false, nil)
	})
	if !strings.Contains(output, "proj") {
		t.Error("JSON should contain project name even with nil services")
	}
}
