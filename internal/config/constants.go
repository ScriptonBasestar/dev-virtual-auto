package config

const (
	// Configuration Files
	FileName      = "dva.yml"
	FileNameAlt   = "dva.yaml"
	OverrideExt   = ".override.yml"
	ModulesDirExt = ".yml"

	// Directories
	DotDirName  = ".sb/dva"
	PidsDirName = "pids"
	LogsDirName = "logs"

	// Environment Variables (DVA settings)
	EnvFileKey       = "DVA_FILE"
	EnvDebugKey      = "DVA_DEBUG"
	EnvHookDepthKey  = "DVA_HOOK_DEPTH"
	EnvFuncPrefixKey = "DVA_FUNC_PREFIX"
	EnvShellKey      = "DVA_SHELL"
	EnvPromptTextKey = "DVA_PROMPT_TEXT"

	// Runtime Environment Variables (injected into processes)
	EnvRuntimeOS             = "DVA_OS"
	EnvRuntimeWorkDirRelPath = "DVA_WORK_DIR_REL_PATH"
	EnvRuntimeCurrentUser    = "DVA_CURRENT_USER"
	EnvRuntimeCurrentUID     = "DVA_CURRENT_UID"
)
