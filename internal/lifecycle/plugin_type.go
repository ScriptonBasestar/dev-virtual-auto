package lifecycle

import "slices"

// PluginType identifies a lifecycle plugin backend.
type PluginType string

// --- Tier 1: Core (implemented) ---

const (
	// PluginCompose orchestrates via docker compose.
	PluginCompose PluginType = "compose"
	// PluginProcess manages local background processes.
	PluginProcess PluginType = "process"
	// PluginScript runs shell script hooks.
	PluginScript PluginType = "script"
	// PluginDocker runs standalone docker containers (docker run).
	PluginDocker PluginType = "docker"
	// PluginKubectl applies Kubernetes manifests via kubectl.
	PluginKubectl PluginType = "kubectl"
	// PluginHelm deploys Helm charts.
	PluginHelm PluginType = "helm"
)

// --- Tier 2: Extended ---

const (
	// PluginKustomize manages Kubernetes manifests via kustomize overlays.
	PluginKustomize PluginType = "kustomize"
	// PluginTilt runs Tilt for local Kubernetes hot-reload development.
	PluginTilt PluginType = "tilt"
	// PluginSkaffold runs Skaffold build-push-deploy pipelines.
	PluginSkaffold PluginType = "skaffold"
	// PluginPodmanCompose orchestrates via podman-compose (rootless).
	PluginPodmanCompose PluginType = "podman-compose"
	// PluginVagrant provisions VMs via Vagrant.
	PluginVagrant PluginType = "vagrant"
)

// --- Tier 3: Niche ---

const (
	// PluginSAM runs AWS SAM local for Lambda emulation.
	PluginSAM PluginType = "sam"
	// PluginServerless runs serverless-offline for local FaaS emulation.
	PluginServerless PluginType = "serverless"
	// PluginMultipass provisions lightweight Ubuntu VMs via Multipass.
	PluginMultipass PluginType = "multipass"
)

// AllPluginTypes returns every known plugin type.
func AllPluginTypes() []PluginType {
	return []PluginType{
		// Tier 1
		PluginCompose, PluginProcess, PluginScript,
		PluginDocker, PluginKubectl, PluginHelm,
		// Tier 2
		PluginKustomize, PluginTilt, PluginSkaffold,
		PluginPodmanCompose, PluginVagrant,
		// Tier 3
		PluginSAM, PluginServerless, PluginMultipass,
	}
}

// IsKnown returns true if the plugin type is defined (even if not yet implemented).
func (pt PluginType) IsKnown() bool {
	return slices.Contains(AllPluginTypes(), pt)
}

func (pt PluginType) String() string {
	return string(pt)
}
