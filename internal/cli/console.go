package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var consoleCmd = &cobra.Command{
	Use:   "console",
	Short: "Launch or inject into a DVA-integrated shell",
}

var consoleStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Output shell integration script (eval in .zshrc/.bashrc)",
	Run: func(cmd *cobra.Command, args []string) {
		binPath, _ := os.Executable()
		fmt.Print(consoleStartScript(binPath))
	},
}

// shellBuiltins are names that should not be overridden without a prefix.
var shellBuiltins = map[string]bool{
	"test": true, "echo": true, "printf": true, "cd": true,
	"export": true, "source": true, "type": true, "command": true,
	"eval": true, "exec": true, "exit": true, "read": true, "set": true,
}

var consoleInjectCmd = &cobra.Command{
	Use:   "inject",
	Short: "Output shell function definitions for current project",
	Long:  "Set DVA_FUNC_PREFIX=dva_ to prefix all generated functions (avoids shell builtin conflicts).",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := loadConfig()
		if err != nil {
			// No config → no aliases
			fmt.Println("function dva_clear() { true; }")
			return nil
		}

		prefix := os.Getenv(config.EnvFuncPrefixKey)

		var aliases []string
		var out []string

		// Add interaction commands as shell functions
		for name := range c.Interaction {
			aliases = append(aliases, name)
		}

		// Add built-in shortcuts
		builtins := []string{"compose", "up", "stop", "down", "provision", "build", config.LogsDirName, "restart"}
		aliases = append(aliases, builtins...)

		binPath, _ := os.Executable()
		var funcNames []string
		for _, name := range aliases {
			funcName := prefix + name
			// Warn about shell builtin conflicts (only when no prefix)
			if prefix == "" && shellBuiltins[name] {
				out = append(out, fmt.Sprintf("# [warn] '%s' shadows a shell builtin. Set DVA_FUNC_PREFIX=dva_ to avoid.", name))
			}
			out = append(out, fmt.Sprintf("function %s() { %s %s \"$@\"; }", funcName, binPath, name))
			funcNames = append(funcNames, funcName)
		}

		// dva_clear function
		var unsets []string
		for _, name := range funcNames {
			unsets = append(unsets, "  unset -f "+name)
		}
		clearBody := "true"
		if len(unsets) > 0 {
			clearBody = strings.Join(unsets, "\n")
		}
		out = append(out, fmt.Sprintf("function dva_clear() {\n%s\n}", clearBody))

		fmt.Println(strings.Join(out, "\n\n"))
		return nil
	},
}

func init() {
	consoleCmd.AddCommand(consoleStartCmd, consoleInjectCmd)
}

func consoleStartScript(binPath string) string {
	return fmt.Sprintf(`export DVA_SHELL=1
export DVA_PROMPT_TEXT="⦒"

function dva_clear() {
  true
}

function dva_inject() {
  eval "$(%s console inject)"
}

function dva_reload() {
  dva_clear
  dva_inject
}

function __zsh_like_cd() {
  \typeset __zsh_like_cd_hook
  if
    builtin "$@"
  then
    for __zsh_like_cd_hook in chpwd "${chpwd_functions[@]}"
    do
      if \typeset -f "$__zsh_like_cd_hook" >/dev/null 2>&1
      then "$__zsh_like_cd_hook" || break
      fi
    done
    true
  else
    return $?
  fi
}

[[ -n "${ZSH_VERSION:-}" ]] ||
{
  function cd()    { __zsh_like_cd cd    "$@" ; }
  function popd()  { __zsh_like_cd popd  "$@" ; }
  function pushd() { __zsh_like_cd pushd "$@" ; }
}

export -a chpwd_functions
[[ " ${chpwd_functions[*]} " == *" dva_reload "* ]] || chpwd_functions+=(dva_reload)

dva_reload
`, binPath)
}
