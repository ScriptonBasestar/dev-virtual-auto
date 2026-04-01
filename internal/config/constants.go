package config

const (
	// Configuration Files
	FileName      = FileName
	FileNameAlt   = FileNameAlt
	OverrideExt   = OverrideExt
	ModulesDirExt = ".yml"

	// Directories
	DotDirName  = ".sb/dva"
	PidsDirName = "pids"
	LogsDirName = "logs"

	// Environment Variables (DVA settings)
	EnvFileKey       = EnvFileKey
	EnvDebugKey      = "DVA_DEBUG"
	EnvHookDepthKey  = "DVA_HOOK_DEPTH"
	EnvFuncPrefixKey = "DVA_FUNC_PREFIX"
	EnvShellKey      = "DVA_SHELL"
	EnvPromptTextKey = "DVA_PROMPT_TEXT"

	// Runtime Environment Variables (injected into processes)
	EnvRuntimeOS             = EnvRuntimeOS
	EnvRuntimeWorkDirRelPath = EnvRuntimeWorkDirRelPath
	EnvRuntimeCurrentUser    = EnvRuntimeCurrentUser
	EnvRuntimeCurrentUID     = EnvRuntimeCurrentUID
)
