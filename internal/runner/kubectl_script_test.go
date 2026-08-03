package runner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// TestKubectlExecArgsCoversEveryOneShotForm covers TASK-175. KubectlRunner.Execute branched on
// Steps and nothing else — Script and ScriptFile appeared nowhere in kubectl.go — so a `pod:`
// interaction declaring either fell through to the command form and produced
// `kubectl exec <pod> --` with nothing after it. Measured on b59ab6d, before the fix:
//
//	dva run podscript   -> kubectl exec --tty --stdin web --
//	dva run podfile     -> kubectl exec --tty --stdin web --
//
// while `dva validate` exited 0 on the same config. That is TASK-094's defect in the same
// function, two execution forms short of done.
//
// These assert execArgs rather than Execute because Execute ends in syscall.Exec: it does not
// return, and under `go test` the TASK-144 guard panics rather than let it replace the test
// binary. execArgs is the seam that makes the argv observable without a cluster, the same one
// DockerComposeRunner.executeArgs opened for compose in TASK-132.
func TestKubectlExecArgsCoversEveryOneShotForm(t *testing.T) {
	cases := []struct {
		name string
		cmd  *ResolvedCommand
		want []string
	}{
		{
			name: "script runs in the pod, not on the host",
			cmd:  &ResolvedCommand{Pod: "web", Script: "echo hello\n"},
			want: []string{"exec", "--tty", "--stdin", "web", "--", "sh", "-c", "echo hello\n"},
		},
		{
			name: "a container qualifier still addresses the container",
			cmd:  &ResolvedCommand{Pod: "web:sidecar", Script: "echo hello\n"},
			want: []string{"exec", "--tty", "--stdin", "--container", "sidecar", "web", "--", "sh", "-c", "echo hello\n"},
		},
		{
			name: "a script declared alongside a command does not run the command",
			cmd:  &ResolvedCommand{Pod: "web", Command: "bundle exec rails", Script: "echo hello\n"},
			want: []string{"exec", "--tty", "--stdin", "web", "--", "sh", "-c", "echo hello\n"},
		},
		{
			// LocalRunner's precedence is steps > script_file > script > command; the kubectl
			// path has to agree, or the same config would run different things by runner.
			name: "script_file wins over script, as it does locally",
			cmd:  &ResolvedCommand{Pod: "web", Script: "echo inline\n", ScriptFile: "/dev/null"},
			want: []string{"exec", "--tty", "--stdin", "web", "--", "sh", "-c", ""},
		},
		{
			// The control. If this row changed, the fix would have moved the command form
			// rather than added a branch beside it.
			name: "the command form is untouched",
			cmd:  &ResolvedCommand{Pod: "web", Command: "bundle exec rails", DefaultArgs: "-e development"},
			want: []string{"exec", "--tty", "--stdin", "web", "--", "bundle", "exec", "rails", "-e", "development"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := &KubectlRunner{Cmd: tc.cmd}
			got, err := r.execArgs()
			if err != nil {
				t.Fatalf("execArgs returned an error: %v", err)
			}
			if !equalArgs(got, tc.want) {
				t.Errorf("argv = %q, want %q", got, tc.want)
			}
		})
	}
}

// A script_file lives on the host, so its path means nothing on the other side of `kubectl exec`
// and the contents have to travel instead. Relative paths resolve against the directory holding
// dva.yml, as they do on the local path.
func TestKubectlExecArgsSendsScriptFileContentsResolvedAgainstDvaYml(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, config.FileName), []byte("interaction:\n  noop:\n    command: \"true\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	const body = "#!/bin/sh\necho from the file\n"
	if err := os.WriteFile(filepath.Join(dir, "hello.sh"), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}

	r := &KubectlRunner{
		Cmd:  &ResolvedCommand{Pod: "web", ScriptFile: "./hello.sh"},
		Opts: RunOptions{Config: cfg},
	}
	got, err := r.execArgs()
	if err != nil {
		t.Fatalf("execArgs returned an error: %v", err)
	}

	want := []string{"exec", "--tty", "--stdin", "web", "--", "sh", "-c", body}
	if !equalArgs(got, want) {
		t.Errorf("argv = %q, want %q", got, want)
	}
}

// An unreadable script_file must say so. The alternative is an empty body, which `sh -c ""`
// accepts and exits 0 on — a missing script reported as a successful run, which is the shape of
// failure this whole task is about.
func TestKubectlExecArgsReportsAnUnreadableScriptFile(t *testing.T) {
	r := &KubectlRunner{Cmd: &ResolvedCommand{Pod: "web", ScriptFile: "/nonexistent/dva-t175.sh"}}

	_, err := r.execArgs()
	if err == nil {
		t.Fatal("execArgs accepted a script_file that does not exist")
	}
	if !strings.Contains(err.Error(), "/nonexistent/dva-t175.sh") {
		t.Errorf("error %q does not name the path it could not read", err)
	}
}

// The subcommand half of the defect, through the real resolution path rather than a hand-built
// literal — the inheritance is what produces it, so the test has to include the inheritance.
//
// mergeInteraction copies the parent's Command onto every child (TASK-174), so `scripted` reaches
// the runner carrying `bundle exec rails` as well as its own script. Before this fix the kubectl
// runner ran that inherited command and discarded the script: not "nothing happened", but the
// wrong command reported as success.
func TestKubectlSubcommandScriptDoesNotRunTheParentsCommand(t *testing.T) {
	tree := NewInteractionTree(map[string]*config.InteractionCommand{
		"rails": {
			Pod:     "web",
			Command: "bundle exec rails",
			Subcommands: map[string]*config.InteractionCommand{
				"scripted": {Script: "echo scripted child ran\n"},
			},
		},
	})

	cmd := tree.Find("rails", "scripted")
	if cmd == nil {
		t.Fatal("rails scripted did not resolve")
	}
	if cmd.Command == "" {
		t.Fatal("fixture no longer inherits the parent command, so it cannot show the defect")
	}

	got, err := (&KubectlRunner{Cmd: cmd}).execArgs()
	if err != nil {
		t.Fatalf("execArgs returned an error: %v", err)
	}

	want := []string{"exec", "--tty", "--stdin", "web", "--", "sh", "-c", "echo scripted child ran\n"}
	if !equalArgs(got, want) {
		t.Errorf("argv = %q, want %q", got, want)
	}
}

func equalArgs(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
