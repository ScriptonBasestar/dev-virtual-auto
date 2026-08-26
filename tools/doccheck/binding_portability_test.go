package main

import (
	"strings"
	"testing"
)

func portabilityFixture(t *testing.T, body string) Result {
	t.Helper()
	root := t.TempDir()
	writeFile(t, root, "docs/a.md", "# A\n\nSee [self](a.md).\n")
	writeFile(t, root, "tasks/todo/001-portability.md", body)
	writeFile(t, root, "pkg/thing_test.go", fixtureTests)
	inv := mustInventory(t, root, "docs/a.md", "tasks/todo/001-portability.md", "pkg/thing_test.go")
	return Check(CheckInput{Root: root, Inventory: inv})
}

func TestBindingPortabilityEscapedPipe(t *testing.T) {
	for _, body := range []string{
		"- [ ] pipe | verify: `find . \\| wc -l`\n",
		"- [ ] pipe | verify: `echo \"$(printf x \\| tr x y)\"`\n",
	} {
		res := portabilityFixture(t, body)
		if res.OK || res.EscapedPipeBindings != 1 || !containsAny(res.PortabilityDetail, "escaped shell pipe") {
			t.Fatalf("escaped pipe = %d, detail=%v, ok=%v", res.EscapedPipeBindings, res.PortabilityDetail, res.OK)
		}
		if !strings.HasPrefix(res.PortabilityDetail[0], "tasks/todo/001-portability.md:1:") {
			t.Fatalf("detail %q does not carry task file and line", res.PortabilityDetail[0])
		}
	}
}

func TestBindingPortabilityCheckout(t *testing.T) {
	res := portabilityFixture(t, "- [ ] checkout | verify: `cd /Users/archmagece/mywork/scripton/dev-virtual-auto && go test ./pkg`\n")
	if res.OK || res.AbsCheckoutBindings != 1 || !containsAny(res.PortabilityDetail, "checkout path") {
		t.Fatalf("checkout = %d, detail=%v, ok=%v", res.AbsCheckoutBindings, res.PortabilityDetail, res.OK)
	}
}

func TestBindingPortabilityExternalCorpus(t *testing.T) {
	for _, tt := range []struct {
		name string
		body string
		want int
	}{
		{"unguarded corpus", "- [ ] corpus | verify: `find ~/mydevbox -name dva.yml`\n", 1},
		{"corpus guard without tool guard", "- [ ] corpus | verify: `n=$(/usr/bin/find ~/mydevbox -name dva.yml | wc -l); [ \"$n\" -gt 0 ] || { exit 2; }; dva validate`\n", 1},
		{"060 absolute-tool guard", "- [ ] corpus | verify: `n=$(/usr/bin/find ~/mydevbox -name dva.yml | /usr/bin/wc -l); [ \"$n\" -gt 0 ] || { exit 2; }; /usr/bin/grep x ~/mydevbox/dva.yml`\n", 0},
		{"066 command-v guard", "- [ ] corpus | verify: `command -v dva >/dev/null || { exit 2; }; n=$(/usr/bin/find ~/mydevbox -name dva.yml | /usr/bin/wc -l); [ \"$n\" -gt 0 ] || { exit 2; }; dva validate`\n", 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			res := portabilityFixture(t, tt.body)
			if res.ExternalCorpusBindings != tt.want {
				t.Fatalf("external corpus = %d, want %d; detail=%v", res.ExternalCorpusBindings, tt.want, res.PortabilityDetail)
			}
		})
	}
}

func TestBindingPortability_ignoresNonBindingAndQuotedBRE(t *testing.T) {
	body := strings.Join([]string{
		"- [ ] first span only | verify: `echo ok` — annotation `find . \\| wc -l`",
		"- [ ] quoted BRE | verify: `grep 'a\\|b' file`",
		"- [ ] human binding | verify: `human — inspect ~/mydevbox`",
		"| criterion | verify: `find . \\| wc -l` |",
		"```sh",
		"- [ ] fenced | verify: `find . \\| wc -l`",
		"```",
	}, "\n")
	res := portabilityFixture(t, body)
	if res.EscapedPipeBindings != 0 || res.AbsCheckoutBindings != 0 || res.ExternalCorpusBindings != 0 {
		t.Fatalf("false portability finding: pipes=%d checkout=%d corpus=%d detail=%v",
			res.EscapedPipeBindings, res.AbsCheckoutBindings, res.ExternalCorpusBindings, res.PortabilityDetail)
	}
}
