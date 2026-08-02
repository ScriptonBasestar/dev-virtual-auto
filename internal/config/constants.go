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

	// EnvPrefix is shared by every environment variable DVA defines for itself, both the
	// settings it reads and the runtime vars it injects. Callers that forward the merged
	// environment somewhere it does not belong — into a container, for instance — filter on
	// this prefix, so a new DVA_ variable must keep it to stay excluded.
	EnvPrefix = "DVA_"

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
