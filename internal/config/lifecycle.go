package config

// LifecycleEntry defines a single entry in the lifecycle pipeline.
type LifecycleEntry struct {
	Name         string                       `yaml:"name"`
	Plugin       string                       `yaml:"plugin"`
	Order        int                          `yaml:"order"`
	Tags         []string                     `yaml:"tags"`
	Exports      map[string]string            `yaml:"exports"`
	HealthChecks map[string]HealthCheckConfig `yaml:"health_checks"`

	// --- Tier 1: Core ---
	Compose *ComposePluginConfig `yaml:"compose,omitempty"`
	Process *ProcessPluginConfig `yaml:"process,omitempty"`
	Script  *ScriptPluginConfig  `yaml:"script,omitempty"`
	Docker  *DockerPluginConfig  `yaml:"docker,omitempty"`
	Kubectl *KubectlPluginConfig `yaml:"kubectl,omitempty"`
	Helm    *HelmPluginConfig    `yaml:"helm,omitempty"`

	// --- Tier 2: Extended ---
	Kustomize    *KustomizePluginConfig    `yaml:"kustomize,omitempty"`
	Tilt         *TiltPluginConfig         `yaml:"tilt,omitempty"`
	Skaffold     *SkaffoldPluginConfig     `yaml:"skaffold,omitempty"`
	PodmanCompose *PodmanComposePluginConfig `yaml:"podman_compose,omitempty"`
	Vagrant      *VagrantPluginConfig      `yaml:"vagrant,omitempty"`

	// --- Tier 3: Niche ---
	SAM        *SAMPluginConfig        `yaml:"sam,omitempty"`
	Serverless *ServerlessPluginConfig `yaml:"serverless,omitempty"`
	Multipass  *MultipassPluginConfig  `yaml:"multipass,omitempty"`
}

// ===== Tier 1: Core =====

// ComposePluginConfig holds Docker Compose plugin settings.
type ComposePluginConfig struct {
	Files       []string `yaml:"files"`
	ProjectName string   `yaml:"project_name"`
	Command     string   `yaml:"command"`
	UpOptions   []string `yaml:"up_options"`
}

// ProcessPluginConfig holds local process plugin settings.
type ProcessPluginConfig struct {
	Command      string `yaml:"command"`
	Dir          string `yaml:"dir"`
	ReadyTimeout int    `yaml:"ready_timeout"`
}

// ScriptPluginConfig holds shell script plugin settings.
type ScriptPluginConfig struct {
	Up   string `yaml:"up"`
	Down string `yaml:"down"`
	Stop string `yaml:"stop"`
}

// DockerPluginConfig holds standalone docker container settings.
type DockerPluginConfig struct {
	Image   string            `yaml:"image"`
	Name    string            `yaml:"name"`
	Ports   []string          `yaml:"ports"`
	Volumes []string          `yaml:"volumes"`
	Env     map[string]string `yaml:"env"`
	Options []string          `yaml:"options"`
}

// KubectlPluginConfig holds kubectl apply settings.
type KubectlPluginConfig struct {
	Manifests []string `yaml:"manifests"`
	Namespace string   `yaml:"namespace"`
	Context   string   `yaml:"context"`
}

// HelmPluginConfig holds Helm chart deployment settings.
type HelmPluginConfig struct {
	Chart     string            `yaml:"chart"`
	Release   string            `yaml:"release"`
	Namespace string            `yaml:"namespace"`
	Context   string            `yaml:"context"`
	Values    []string          `yaml:"values"`
	Set       map[string]string `yaml:"set"`
}

// ===== Tier 2: Extended =====

// KustomizePluginConfig holds kustomize overlay settings.
type KustomizePluginConfig struct {
	Dir       string `yaml:"dir"`
	Namespace string `yaml:"namespace"`
	Context   string `yaml:"context"`
}

// TiltPluginConfig holds Tilt local dev settings.
type TiltPluginConfig struct {
	Dir  string   `yaml:"dir"`
	Args []string `yaml:"args"`
}

// SkaffoldPluginConfig holds Skaffold pipeline settings.
type SkaffoldPluginConfig struct {
	Config  string   `yaml:"config"`
	Profile string   `yaml:"profile"`
	Args    []string `yaml:"args"`
}

// PodmanComposePluginConfig holds podman-compose settings.
type PodmanComposePluginConfig struct {
	Files       []string `yaml:"files"`
	ProjectName string   `yaml:"project_name"`
}

// VagrantPluginConfig holds Vagrant VM settings.
type VagrantPluginConfig struct {
	Dir     string `yaml:"dir"`
	Machine string `yaml:"machine"`
}

// ===== Tier 3: Niche =====

// SAMPluginConfig holds AWS SAM local settings.
type SAMPluginConfig struct {
	Template string `yaml:"template"`
	Port     int    `yaml:"port"`
	Args     []string `yaml:"args"`
}

// ServerlessPluginConfig holds serverless-offline settings.
type ServerlessPluginConfig struct {
	Dir  string   `yaml:"dir"`
	Port int      `yaml:"port"`
	Args []string `yaml:"args"`
}

// MultipassPluginConfig holds Multipass VM settings.
type MultipassPluginConfig struct {
	Name      string `yaml:"name"`
	Image     string `yaml:"image"`
	CPUs      int    `yaml:"cpus"`
	Memory    string `yaml:"memory"`
	Disk      string `yaml:"disk"`
	CloudInit string `yaml:"cloud_init"`
}
