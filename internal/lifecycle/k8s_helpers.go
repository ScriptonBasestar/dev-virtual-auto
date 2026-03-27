package lifecycle

import (
	"encoding/json"
	"strings"
)

// buildK8sNamespaceArgs returns ["-n", namespace] if namespace is non-empty.
func buildK8sNamespaceArgs(namespace string) []string {
	if namespace == "" {
		return nil
	}
	return []string{"-n", namespace}
}

// buildKubectlContextArgs returns ["--context", ctx] if ctx is non-empty.
func buildKubectlContextArgs(ctx string) []string {
	if ctx == "" {
		return nil
	}
	return []string{"--context", ctx}
}

// buildHelmContextArgs returns ["--kube-context", ctx] if ctx is non-empty.
// Note: Helm uses --kube-context, not --context.
func buildHelmContextArgs(ctx string) []string {
	if ctx == "" {
		return nil
	}
	return []string{"--kube-context", ctx}
}

// k8sResourceStatus mirrors kubectl get -o json output for parsing.
type k8sResourceStatus struct {
	Items []struct {
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
		Status struct {
			Phase string `json:"phase"`
		} `json:"status"`
	} `json:"items"`
}

// parseK8sResourceStatus parses kubectl get -o json output and converts
// each item into a ServiceStatus.
func parseK8sResourceStatus(jsonData []byte) ([]ServiceStatus, error) {
	var res k8sResourceStatus
	if err := json.Unmarshal(jsonData, &res); err != nil {
		return nil, err
	}

	services := make([]ServiceStatus, 0, len(res.Items))
	for _, item := range res.Items {
		services = append(services, ServiceStatus{
			Name:   item.Metadata.Name,
			State:  strings.ToLower(item.Status.Phase),
			Health: "unknown",
		})
	}

	return services, nil
}
