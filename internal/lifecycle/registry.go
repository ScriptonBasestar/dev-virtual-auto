package lifecycle

import "fmt"

// registry maps plugin type names to their factory functions.
// Plugins are registered at compile time — no dynamic loading.
var registry = map[string]func() LifecyclePlugin{
	"compose": func() LifecyclePlugin { return &ComposePlugin{} },
	"process": func() LifecyclePlugin { return &ProcessPlugin{} },
	"script":  func() LifecyclePlugin { return &ScriptPlugin{} },
}

// NewPlugin creates a new plugin instance by type name.
func NewPlugin(name string) (LifecyclePlugin, error) {
	factory, ok := registry[name]
	if !ok {
		available := make([]string, 0, len(registry))
		for k := range registry {
			available = append(available, k)
		}
		return nil, fmt.Errorf("unknown lifecycle plugin %q (available: %v)", name, available)
	}
	return factory(), nil
}
