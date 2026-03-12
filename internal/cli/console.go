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

var consoleInjectCmd = &cobra.Command{
	Use:   "inject",
	Short: "Output shell function definitions for current project",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := loadConfig()
		if err != nil {
			// No config → no aliases
			fmt.Println("function dva_clear() { true; }")
			return nil
		}

		var aliases []string
		var out []string

		// Add interaction commands as shell functions
		for name := range c.Interaction {
			aliases = append(aliases, name)
		}

		// Add built-in shortcuts
		builtins := []string{"compose", "up", "stop", "down", "provision", "build"}
		aliases = append(aliases, builtins...)

		binPath, _ := os.Executable()
		for _, name := range aliases {
			out = append(out, fmt.Sprintf("function %s() { %s %s \"$@\"; }", name, binPath, name))
		}

		// dva_clear function
		var unsets []string
		for _, name := range aliases {
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
