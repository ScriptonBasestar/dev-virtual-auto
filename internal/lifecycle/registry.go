package lifecycle

import "fmt"

// registry maps implemented plugin types to their factory functions.
// Plugins are registered at compile time — no dynamic loading.
var registry = map[PluginType]func() LifecyclePlugin{
	PluginCompose:       func() LifecyclePlugin { return &ComposePlugin{} },
	PluginProcess:       func() LifecyclePlugin { return &ProcessPlugin{} },
	PluginScript:        func() LifecyclePlugin { return &ScriptPlugin{} },
	PluginDocker:        func() LifecyclePlugin { return &DockerPlugin{} },
	PluginPodmanCompose: func() LifecyclePlugin { return &PodmanComposePlugin{} },
	PluginKubectl:       func() LifecyclePlugin { return &KubectlPlugin{} },
	PluginHelm:          func() LifecyclePlugin { return &HelmPlugin{} },
	PluginKustomize:     func() LifecyclePlugin { return &KustomizePlugin{} },
	PluginVagrant:       func() LifecyclePlugin { return &VagrantPlugin{} },
	PluginTilt:          func() LifecyclePlugin { return &TiltPlugin{} },
	PluginSkaffold:      func() LifecyclePlugin { return &SkaffoldPlugin{} },
	PluginSAM:           func() LifecyclePlugin { return &SAMPlugin{} },
	PluginServerless:    func() LifecyclePlugin { return &ServerlessPlugin{} },
	PluginMultipass:     func() LifecyclePlugin { return &MultipassPlugin{} },
}

// NewPlugin creates a new plugin instance by type name.
// Returns a distinct error for known-but-unimplemented types vs unknown types.
func NewPlugin(name string) (LifecyclePlugin, error) {
	pt := PluginType(name)

	factory, ok := registry[pt]
	if ok {
		return factory(), nil
	}

	if pt.IsKnown() {
		return nil, fmt.Errorf("lifecycle plugin %q is not yet implemented", name)
	}

	implemented := make([]string, 0, len(registry))
	for k := range registry {
		implemented = append(implemented, k.String())
	}
	all := AllPluginTypes()
	planned := make([]string, 0)
	for _, t := range all {
		if _, exists := registry[t]; !exists {
			planned = append(planned, t.String())
		}
	}
	return nil, fmt.Errorf("unknown lifecycle plugin %q (implemented: %v, planned: %v)", name, implemented, planned)
}

// ImplementedPlugins returns plugin types that have a working implementation.
func ImplementedPlugins() []PluginType {
	result := make([]PluginType, 0, len(registry))
	for pt := range registry {
		result = append(result, pt)
	}
	return result
}

// PlannedPlugins returns plugin types that are defined but not yet implemented.
func PlannedPlugins() []PluginType {
	var result []PluginType
	for _, pt := range AllPluginTypes() {
		if _, ok := registry[pt]; !ok {
			result = append(result, pt)
		}
	}
	return result
}
