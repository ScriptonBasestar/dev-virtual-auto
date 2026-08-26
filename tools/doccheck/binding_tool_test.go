package main

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestBindingToolBareGrep(t *testing.T) {
	res := portabilityFixture(t, "- [ ] lookup | verify: `n=$(grep -c x file); [ \"$n\" -eq 1 ]`\n")
	if res.OK || res.WrappedToolBindings != 1 || res.BareToolBindings != 1 || !containsAny(res.PortabilityDetail, "bare wrapped tool(s): grep") {
		t.Fatalf("wrapped/bare tools = %d/%d, detail=%v, ok=%v", res.WrappedToolBindings, res.BareToolBindings, res.PortabilityDetail, res.OK)
	}
}

func TestBindingToolBareFind(t *testing.T) {
	res := portabilityFixture(t, "- [ ] lookup | verify: `gopls check $(find cmd -name '*.go')`\n")
	if res.OK || res.WrappedToolBindings != 1 || res.BareToolBindings != 1 || !containsAny(res.PortabilityDetail, "bare wrapped tool(s): find") {
		t.Fatalf("wrapped/bare tools = %d/%d, detail=%v, ok=%v", res.WrappedToolBindings, res.BareToolBindings, res.PortabilityDetail, res.OK)
	}
}

func TestBindingToolAbsoluteForms(t *testing.T) {
	res := portabilityFixture(t, "- [ ] lookup | verify: `/usr/bin/find . -type f | /usr/bin/grep -q x`\n")
	if res.WrappedToolBindings != 1 || res.BareToolBindings != 0 {
		t.Fatalf("absolute tools produced wrapped/bare=%d/%d: %v", res.WrappedToolBindings, res.BareToolBindings, res.PortabilityDetail)
	}
}

func TestBindingToolPopulation(t *testing.T) {
	body := strings.Join([]string{
		"```sh",
		"- [ ] fenced | verify: `grep x file`",
		"```",
		"- [ ] prose before span | verify: human preface `grep x file` — measured",
		"- [ ] annotation | verify: `/usr/bin/grep x file` — original `grep x file`",
		"- [ ] error annotation | verify: `/usr/bin/find .` — output `find: |: unknown primary or operator`",
	}, "\n")
	res := portabilityFixture(t, body)
	if res.WrappedToolBindings != 3 || res.BareToolBindings != 1 {
		t.Fatalf("wrapped/bare tools = %d/%d, want 3/1; detail=%v", res.WrappedToolBindings, res.BareToolBindings, res.PortabilityDetail)
	}
}

func TestBindingToolShellCommandWords(t *testing.T) {
	for _, tt := range []struct {
		name string
		span string
		want int
	}{
		{"pipeline", `printf x | grep x file`, 1},
		{"double quoted substitution", `printf '%s' "$(grep x file)"`, 1},
		{"double quoted command", `"grep" x file`, 1},
		{"single quoted command", `'grep' x file`, 1},
		{"partly quoted command", `g"re"p x file`, 1},
		{"escaped command", `\grep x file`, 1},
		{"combined leading redirection", `</dev/null grep x file`, 1},
		{"separate leading redirection", `< /dev/null grep x file`, 1},
		{"zsh noglob prefix", `noglob grep x file`, 1},
		{"plain argv", `printf '%s' grep find`, 0},
		{"quoted pattern", `/usr/bin/grep 'grep find' file`, 0},
		{"quoted absolute command", `"/usr/bin/grep" x file`, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			res := portabilityFixture(t, "- [ ] shell | verify: `"+tt.span+"`\n")
			if res.BareToolBindings != tt.want {
				t.Fatalf("bare tools = %d, want %d; wrapped=%d detail=%v", res.BareToolBindings, tt.want, res.WrappedToolBindings, res.PortabilityDetail)
			}
			if tt.want == 0 && strings.Contains(tt.span, "/usr/bin/grep") && res.WrappedToolBindings != 1 {
				t.Fatalf("quoted/argument absolute form wrapped=%d, want 1", res.WrappedToolBindings)
			}
		})
	}
}

func TestBindingToolHumanForms(t *testing.T) {
	res := portabilityFixture(t, strings.Join([]string{
		"- [ ] normal human | verify: human — inspect `grep x file`",
		"- [ ] malformed human | verify: `human — inspect $(grep x file)`",
	}, "\n"))
	if res.WrappedToolBindings != 1 || res.BareToolBindings != 1 {
		t.Fatalf("wrapped/bare tools = %d/%d, want 1/1; detail=%v", res.WrappedToolBindings, res.BareToolBindings, res.PortabilityDetail)
	}
}

func TestBindingToolSweepsRealCorpus(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(file), "..", "..")
	inv, err := LoadInventory(root)
	if err != nil {
		t.Fatalf("inventory: %v", err)
	}
	res := Check(CheckInput{Root: root, Inventory: inv})
	if res.WrappedToolBindings < 100 {
		t.Errorf("wrapped_tool_bindings=%d, want at least 100 — binding sweep no longer reaches the task corpus", res.WrappedToolBindings)
	}
	if res.BareToolBindings > 0 {
		t.Errorf("bare_tool_bindings=%d:\n  %s", res.BareToolBindings, strings.Join(res.PortabilityDetail, "\n  "))
	}
}
