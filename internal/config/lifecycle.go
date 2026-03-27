package config

// LifecycleEntry defines a single entry in the lifecycle pipeline.
type LifecycleEntry struct {
	Name         string                       `yaml:"name"`
	Plugin       string                       `yaml:"plugin"`
	Order        int                          `yaml:"order"`
	Tags         []string                     `yaml:"tags"`
	Compose      *ComposePluginConfig         `yaml:"compose,omitempty"`
	Process      *ProcessPluginConfig         `yaml:"process,omitempty"`
	K8s          *K8sPluginConfig             `yaml:"k8s,omitempty"`
	Script       *ScriptPluginConfig          `yaml:"script,omitempty"`
	Exports      map[string]string            `yaml:"exports"`
	HealthChecks map[string]HealthCheckConfig `yaml:"health_checks"`
}

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

// K8sPluginConfig holds Kubernetes plugin settings.
type K8sPluginConfig struct {
	Namespace string   `yaml:"namespace"`
	Manifests []string `yaml:"manifests"`
	Context   string   `yaml:"context"`
}

// ScriptPluginConfig holds shell script plugin settings.
type ScriptPluginConfig struct {
	Up   string `yaml:"up"`
	Down string `yaml:"down"`
	Stop string `yaml:"stop"`
}
