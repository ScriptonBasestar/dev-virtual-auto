package lifecycle

import (
	"testing"
)

func TestBuildK8sNamespaceArgs_Empty(t *testing.T) {
	got := buildK8sNamespaceArgs("")
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestBuildK8sNamespaceArgs_Set(t *testing.T) {
	got := buildK8sNamespaceArgs("kube-system")
	if len(got) != 2 || got[0] != "-n" || got[1] != "kube-system" {
		t.Errorf("expected [-n kube-system], got %v", got)
	}
}

func TestBuildKubectlContextArgs_Empty(t *testing.T) {
	got := buildKubectlContextArgs("")
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestBuildKubectlContextArgs_Set(t *testing.T) {
	got := buildKubectlContextArgs("minikube")
	if len(got) != 2 || got[0] != "--context" || got[1] != "minikube" {
		t.Errorf("expected [--context minikube], got %v", got)
	}
}

func TestBuildHelmContextArgs_Empty(t *testing.T) {
	got := buildHelmContextArgs("")
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestBuildHelmContextArgs_Set(t *testing.T) {
	got := buildHelmContextArgs("prod-cluster")
	if len(got) != 2 || got[0] != "--kube-context" || got[1] != "prod-cluster" {
		t.Errorf("expected [--kube-context prod-cluster], got %v", got)
	}
}

func TestParseK8sResourceStatus_Valid(t *testing.T) {
	jsonData := []byte(`{
		"items": [
			{
				"metadata": {"name": "nginx-pod"},
				"status": {"phase": "Running"}
			},
			{
				"metadata": {"name": "redis-pod"},
				"status": {"phase": "Pending"}
			}
		]
	}`)

	services, err := parseK8sResourceStatus(jsonData)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(services))
	}

	if services[0].Name != "nginx-pod" {
		t.Errorf("expected name 'nginx-pod', got %q", services[0].Name)
	}
	if services[0].State != "running" {
		t.Errorf("expected state 'running', got %q", services[0].State)
	}
	if services[0].Health != "unknown" {
		t.Errorf("expected health 'unknown', got %q", services[0].Health)
	}

	if services[1].Name != "redis-pod" {
		t.Errorf("expected name 'redis-pod', got %q", services[1].Name)
	}
	if services[1].State != "pending" {
		t.Errorf("expected state 'pending', got %q", services[1].State)
	}
}

func TestParseK8sResourceStatus_Empty(t *testing.T) {
	jsonData := []byte(`{"items": []}`)

	services, err := parseK8sResourceStatus(jsonData)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(services) != 0 {
		t.Errorf("expected 0 services, got %d", len(services))
	}
}

func TestParseK8sResourceStatus_Invalid(t *testing.T) {
	jsonData := []byte(`{not valid json`)

	_, err := parseK8sResourceStatus(jsonData)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}
