package exec

import (
	"fmt"
	"path/filepath"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// ComposeArgv builds the compose binary and the argument prefix that every compose
// invocation shares: the subcommand word (or whatever the user's `command:` replaces it
// with), the -f file flags, and --project-name. Callers append their own tail — profiles,
// the subcommand, its options — because that part genuinely differs between them.
//
// It exists because the same fifteen lines were written out four times (TASK-115), and the
// copies had already begun to drift: three joined paths with `dir + "/" + f` and one with
// filepath.Join. Both copies of the argument-splitting bug below were in all four.
//
// baseDir resolves relative compose files. It is the config dir for most callers and the
// entry's source dir for sourced stack entries, which is why it is a parameter rather than
// something this function derives.
func ComposeArgv(env *config.Environment, cc *config.ComposePluginConfig, baseDir string) (string, []string, error) {
	cmd := "docker"
	args := []string{"compose"}

	if cc == nil {
		return cmd, args, nil
	}

	if cc.Command != "" {
		fields := SplitCommand(cc.Command)
		if len(fields) == 0 {
			// SplitCommand returns nil for a string that is all whitespace, and for "''"
			// — the quote branch consumes both characters and writes nothing. The old
			// guard was `cc.Command != ""`, which is true for all of those, so parts[0]
			// indexed into nil and DVA panicked with a stack trace where a config error
			// belonged. `dva up --dry-run` panicked too: the mode meant to be safe.
			return "", nil, fmt.Errorf("compose runner: command: %q contains no command word", cc.Command)
		}
		cmd = fields[0]
		// Unconditionally, including when there is only one field. The old code assigned
		// args only when len(parts) > 1, so a single-token command left the "compose"
		// seed in place and ran `podman-compose compose up …` — a stray word the user
		// never wrote, from a tool they had configured correctly.
		args = fields[1:]
	}

	for _, f := range cc.Files {
		f = env.Interpolate(f)
		if !filepath.IsAbs(f) {
			f = filepath.Join(baseDir, f)
		}
		args = append(args, "-f", f)
	}

	if cc.ProjectName != "" {
		args = append(args, "--project-name", env.Interpolate(cc.ProjectName))
	}

	return cmd, args, nil
}
