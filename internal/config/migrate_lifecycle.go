package config

// migrateComposeToLifecycle converts legacy compose: config to lifecycle entries
// when no explicit lifecycle: section is defined.
func migrateComposeToLifecycle(cfg *Config) {
	if len(cfg.Lifecycle) > 0 {
		return // explicit lifecycle takes priority
	}
	if len(cfg.Compose.Files) == 0 {
		return // no compose config to migrate
	}

	entry := LifecycleEntry{
		Name:   "compose",
		Plugin: "compose",
		Order:  10,
		Compose: &ComposePluginConfig{
			Files:       cfg.Compose.Files,
			ProjectName: cfg.Compose.ProjectName,
			Command:     cfg.Compose.Command,
			UpOptions:   cfg.Compose.UpOptions,
		},
		HealthChecks: cfg.HealthChecks,
	}

	cfg.Lifecycle = []LifecycleEntry{entry}
}
