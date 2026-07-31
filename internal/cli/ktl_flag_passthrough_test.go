// Package cli — regression tests for TASK-103.
//
// `dva ktl` sets DisableFlagParsing and appended its args straight into kubectl's argv on
// both exec paths, so DVA's own root flags became kubectl's. Measured on 0.1.44:
//
//	dva --debug ktl get pods   -> kubectl get pods --debug
//
// This is TASK-092's defect at a fourth site, but it needs its own machinery. The compose
// passthroughs honour the forceSubprocess global and so can be driven in-process; `ktl` calls
// dvaexec.ExecReplace directly, which is syscall.Exec. A naive test would replace the *test
// binary* with kubectl — kubectl's exit status becomes the test's and the assertions never run
// while `go test` prints ok. So each case runs in a child process and the parent asserts on
// what the child's kubectl shim recorded.
package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const (
	ktlChildCaseEnv = "DVA_KTL_CHILD_CASE"
	ktlChildDirEnv  = "DVA_KTL_CHILD_DIR"
)

// ktlCase is one end-to-end run: a config, the argv `dva ktl` is given, and what kubectl must
// and must not receive.
type ktlCase struct {
	name   string
	config string
	args   []string
	// absent: DVA's own flags, which must not survive into kubectl's argv.
	absent []string
	// present: everything that must still arrive, so a fix cannot pass by dropping too much.
	present []string
}

// Most fixtures use the deprecated top-level `kubectl:` form, which is what the entry
// resolution originally understood; the last case pins the modern `runners.kubectl` shape,
// which KubectlEntries() could not see at all until TASK-102 fixed it. Keeping both means a
// regression in either shape is caught here.
var ktlCases = []ktlCase{
	{
		// kubectl.go:30-35 — no kubectl entries, so args are appended to the
		// PrimaryKubectlConfig fallback.
		name: "zero-entry fallback strips --debug",
		config: `version: "0.1.44"
stack:
  web:
    order: 1
    process:
      command: sleep 1
`,
		args:    []string{"--debug", "get", "pods"},
		absent:  []string{"--debug"},
		present: []string{"get", "pods"},
	},
	{
		// kubectl.go:62-67 — a single resolved entry contributes --namespace.
		name: "resolved entry strips --debug and --json, keeps the namespace",
		config: `version: "0.1.44"
stack:
  cluster:
    order: 1
    kubectl:
      namespace: my-namespace
`,
		args:    []string{"--debug", "--json", "get", "pods", "-o", "wide"},
		absent:  []string{"--debug", "--json"},
		present: []string{"--namespace", "my-namespace", "get", "pods", "-o", "wide"},
	},
	{
		// The placement test. With two entries, args[0] must be an entry name
		// (kubectl.go:42-51). If the strip ran after that lookup instead of before it, `beta`
		// would never be found — "--debug" would be tried as the entry name and the command
		// would fail with "multiple kubectl entries".
		name: "entry name still resolves behind a leading root flag",
		config: `version: "0.1.44"
stack:
  alpha:
    order: 1
    kubectl:
      namespace: ns-alpha
  beta:
    order: 2
    kubectl:
      namespace: ns-beta
`,
		args:    []string{"--debug", "beta", "get", "pods"},
		absent:  []string{"--debug", "beta", "ns-alpha"},
		present: []string{"--namespace", "ns-beta", "get", "pods"},
	},
	{
		// The carve-out TASK-092 established: --dry-run belongs to the plugin, not to DVA.
		// kubectl's is a value flag (--dry-run=client), so it must arrive intact.
		name: "kubectl's own --dry-run is forwarded",
		config: `version: "0.1.44"
stack:
  cluster:
    order: 1
    kubectl:
      namespace: my-namespace
`,
		args:    []string{"apply", "-f", "pod.yaml", "--dry-run=client"},
		absent:  nil,
		present: []string{"apply", "-f", "pod.yaml", "--dry-run=client"},
	},
	{
		// The modern shape, promised by TASK-103 and delivered by TASK-102. Before that fix
		// KubectlEntries() returned nothing here, so `ktl` took its zero-entry fallback and the
		// declared namespace never reached kubectl at all — the command still ran, against
		// whatever namespace the kubeconfig happened to point at.
		name: "runners.kubectl contributes the namespace too",
		config: `version: "0.1.44"
stack:
  cluster:
    order: 1
    default_runner: kubectl
    runners:
      kubectl:
        namespace: modern-ns
`,
		args:    []string{"--debug", "get", "pods"},
		absent:  []string{"--debug"},
		present: []string{"--namespace", "modern-ns", "get", "pods"},
	},
}

func TestKtlDoesNotForwardRootFlags(t *testing.T) {
	if name := os.Getenv(ktlChildCaseEnv); name != "" {
		runKtlChild(t, name)
		return
	}

	for _, tc := range ktlCases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "dva.yml"), []byte(tc.config), 0o644); err != nil {
				t.Fatal(err)
			}
			shimDir := filepath.Join(dir, "shim")
			if err := os.Mkdir(shimDir, 0o755); err != nil {
				t.Fatal(err)
			}
			shim := "#!/bin/sh\nprintf 'KTL-ARGV: %s\\n' \"$*\"\n"
			if err := os.WriteFile(filepath.Join(shimDir, "kubectl"), []byte(shim), 0o755); err != nil {
				t.Fatal(err)
			}

			out := runKtlParent(t, tc.name, dir)
			argv, ok := ktlArgvFrom(out)
			if !ok {
				t.Fatalf("child never reached the kubectl shim; output was:\n%s", out)
			}
			t.Logf("kubectl argv: %q", argv)

			for _, a := range tc.absent {
				if hasArg(argv, a) {
					t.Errorf("%s reached kubectl in %q — DVA's flag became kubectl's", a, argv)
				}
			}
			for _, p := range tc.present {
				if !hasArg(argv, p) {
					t.Errorf("%s did not survive into %q — the passthrough is broken", p, argv)
				}
			}
		})
	}
}

// runKtlParent re-executes this test binary for one case and returns everything it printed.
func runKtlParent(t *testing.T, name, dir string) string {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestKtlDoesNotForwardRootFlags$")
	cmd.Env = append(os.Environ(), ktlChildCaseEnv+"="+name, ktlChildDirEnv+"="+dir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("child %q exited with %v; output:\n%s", name, err, out)
	}
	return string(out)
}

// runKtlChild is the other side: it becomes kubectl. Nothing after ktlCmd.RunE runs on the
// success path, because syscall.Exec has replaced this process with the shim.
func runKtlChild(t *testing.T, name string) {
	t.Helper()
	dir := os.Getenv(ktlChildDirEnv)
	if dir == "" {
		t.Fatalf("%s set without %s", ktlChildCaseEnv, ktlChildDirEnv)
	}

	var tc *ktlCase
	for i := range ktlCases {
		if ktlCases[i].name == name {
			tc = &ktlCases[i]
			break
		}
	}
	if tc == nil {
		t.Fatalf("unknown case %q", name)
	}

	// PATH is rebuilt rather than prepended to: a real kubectl lives at
	// /opt/homebrew/bin/kubectl on this machine, and this process is about to exec whatever
	// `kubectl` resolves to. The guard below is the thing standing between this test and a
	// real cluster, so it aborts rather than falling back.
	shimPath := filepath.Join(dir, "shim", "kubectl")
	if err := os.Setenv("PATH", filepath.Join(dir, "shim")+":/bin:/usr/bin"); err != nil {
		t.Fatal(err)
	}
	resolved, err := exec.LookPath("kubectl")
	if err != nil {
		t.Fatalf("kubectl does not resolve under the test PATH: %v", err)
	}
	if resolved != shimPath {
		t.Fatalf("kubectl resolves to %q, not the shim %q — refusing to exec a real kubectl", resolved, shimPath)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	if err := ktlCmd.RunE(ktlCmd, append([]string(nil), tc.args...)); err != nil {
		t.Fatalf("ktl returned %v instead of exec'ing kubectl", err)
	}
	t.Fatal("ktl returned without exec'ing kubectl")
}

// ktlArgvFrom extracts the shim's line from the child's output. The child also prints Go's
// own test chatter before exec'ing, so this cannot just take the whole stream.
func ktlArgvFrom(out string) (string, bool) {
	const marker = "KTL-ARGV: "
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, marker) {
			return strings.TrimPrefix(line, marker), true
		}
	}
	return "", false
}
