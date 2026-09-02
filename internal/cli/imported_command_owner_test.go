package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// TestImportedInteractionAndProvisionOwnerIsolation is TASK-264's failure witness.
//
// Every branch below fails on the pre-TASK-264 tree, and each fails differently, which is
// why they are one test rather than one assertion:
//
//   - `run --project` / `project:interaction` built the child environment as
//     NewEnvironment(subCfg.Environment, parentEnv.WorkDir(), subCfg.FileDir()) — the base
//     vars slot held the child's top-level `environment:`, so the child's `vars:` were
//     dropped outright; the child's env_file was never opened; and the run was rooted at
//     the caller's cwd.
//   - an imported interaction was cloned with a SubprojectPath hint and nothing else, so it
//     resolved against the parent: parent vars, parent env_file, and script_file looked up
//     under the parent root.
//   - an imported provision profile was copied as a bare []ProvisionItem with neither owner
//     nor path, so its steps ran with the parent environment and the parent working
//     directory.
//
// The fixture gives parent and child the *same* variable names with different values, so a
// leak is a wrong value rather than a missing one — a missing value can be produced by a
// dozen unrelated mistakes, a swapped one cannot.
func TestImportedInteractionAndProvisionOwnerIsolation(t *testing.T) {
	t.Run("ImportedInteractionCanonicalAndAlias", func(t *testing.T) {
		for _, route := range []string{"child/greet", "child-greet"} {
			t.Run(route, func(t *testing.T) {
				fx := newOwnerFixture(t, ownerFixtureOptions{})
				fx.runInteraction(t, nil, route)
				fx.wantChildGreet(t)
			})
		}
	})

	t.Run("DirectChildInteraction", func(t *testing.T) {
		// Both spellings of the dynamic route reach runSubprojectCommand; both are listed
		// because only the flag form existed before namespace syntax was added, and a fix
		// applied to one of the two call shapes would still leave the other broken.
		t.Run("ProjectFlag", func(t *testing.T) {
			fx := newOwnerFixture(t, ownerFixtureOptions{})
			fx.runInteraction(t, func() { projectName = "child" }, "greet")
			fx.wantChildGreet(t)
		})
		t.Run("NamespaceSyntax", func(t *testing.T) {
			fx := newOwnerFixture(t, ownerFixtureOptions{})
			fx.runInteraction(t, nil, "child:greet")
			fx.wantChildGreet(t)
		})
	})

	t.Run("ImportedProvisionCanonicalAndAlias", func(t *testing.T) {
		for _, profile := range []string{"child/setup", "child-setup"} {
			t.Run(profile, func(t *testing.T) {
				fx := newOwnerFixture(t, ownerFixtureOptions{})
				fx.runProvision(t, profile)
				fx.wantChildProvision(t, profile)
			})
		}
	})

	t.Run("RootProfileAndInteractionUnchanged", func(t *testing.T) {
		// The other half of ownership: a locally declared route must keep resolving against
		// the root exactly as before, or "the child wins" has been implemented as "the child
		// always wins".
		fx := newOwnerFixture(t, ownerFixtureOptions{})
		fx.runInteraction(t, nil, "greet")
		got := fx.readOut(t, fx.parentOut, "greet.out")
		if want := "parent|parent-top|parent-file"; got != want {
			t.Fatalf("root interaction = %q, want %q", got, want)
		}
		if _, err := os.Stat(filepath.Join(fx.childOut, "greet.out")); !os.IsNotExist(err) {
			t.Fatalf("root interaction wrote into the child output dir: %v", err)
		}

		// The working directory is deliberately not asserted against the config dir here.
		// A root profile keeps running from the caller's cwd — newConfigEnvironment's
		// os.Getwd — and that is the historical behavior TASK-264 must not change; only an
		// owned route is rerooted, which is what wantChildProvision pins.
		fx.runProvision(t, "setup")
		record := fx.readOut(t, fx.parentOut, "provision.out")
		if values, _, ok := strings.Cut(record, "|"+string(filepath.Separator)); !ok || values != "parent|parent-top|parent-file" {
			t.Fatalf("root provision = %q, want parent values with a cwd-rooted working directory", record)
		}
	})

	t.Run("RootEnvFailureDoesNotBlockChildRoute", func(t *testing.T) {
		// TASK-248 turns a required env_file failure into an exit code. That is only safe
		// once owner selection happens before env loading, so a child route must not read
		// the root's env inputs at all — observable today as the absence of the warning the
		// root path still emits.
		fx := newOwnerFixture(t, ownerFixtureOptions{brokenRootEnvFile: true})

		_, stderr := captureStreams(t, func() {
			fx.runInteraction(t, nil, "child/greet")
		})
		if strings.Contains(stderr, "env_file") {
			t.Fatalf("child interaction route consulted the root env_file:\n%s", stderr)
		}
		fx.wantChildGreetNoRootEnvFile(t)

		_, stderr = captureStreams(t, func() {
			fx.runProvision(t, "child/setup")
		})
		if strings.Contains(stderr, "env_file") {
			t.Fatalf("child provision route consulted the root env_file:\n%s", stderr)
		}
	})

	t.Run("OwnerIsNotSerialized", func(t *testing.T) {
		// The owner is a live *Config holding the child's absolute local paths. It is
		// unexported precisely so no reflective encoder can reach it; this pins that, because
		// exporting the field would be an easy and completely silent way to leak a
		// developer's directory layout into machine-readable output.
		fx := newOwnerFixture(t, ownerFixtureOptions{})

		dumped, err := yaml.Marshal(fx.root)
		if err != nil {
			t.Fatalf("marshal config: %v", err)
		}
		manifest, err := json.Marshal(buildManifest(fx.root))
		if err != nil {
			t.Fatalf("marshal manifest: %v", err)
		}
		for name, out := range map[string]string{"config dump": string(dumped), "manifest": string(manifest)} {
			if strings.Contains(out, fx.childOut) {
				t.Errorf("%s exposed a child-owned absolute path (%s)", name, fx.childOut)
			}
		}
	})
}

// TestImportedCommandOwnerSharedAcrossRegistrations pins the property that makes canonical
// and alias routes interchangeable: they are two names for one import, so they must hand back
// the same *Config, not two configs that happen to agree today.
func TestImportedCommandOwnerSharedAcrossRegistrations(t *testing.T) {
	fx := newOwnerFixture(t, ownerFixtureOptions{})
	root := fx.root

	canonical := root.Interaction["child/greet"].OwnerConfig(root)
	alias := root.Interaction["child-greet"].OwnerConfig(root)
	if canonical == nil || canonical != alias {
		t.Fatalf("interaction owners differ: canonical=%p alias=%p", canonical, alias)
	}
	if canonical == root {
		t.Fatal("imported interaction owner is the root config")
	}

	pCanonical := root.Provision.ProfileOwner("child/setup", root)
	pAlias := root.Provision.ProfileOwner("child-setup", root)
	if pCanonical == nil || pCanonical != pAlias {
		t.Fatalf("provision owners differ: canonical=%p alias=%p", pCanonical, pAlias)
	}
	if pCanonical != canonical {
		t.Fatal("interaction and provision imported from one subproject resolved to different configs")
	}
	if got := root.Provision.ProfileOwner("setup", root); got != root {
		t.Fatalf("root profile owner = %p, want root %p", got, root)
	}
	if got := root.Interaction["greet"].OwnerConfig(root); got != root {
		t.Fatalf("root interaction owner = %p, want root %p", got, root)
	}
}

type ownerFixtureOptions struct {
	// brokenRootEnvFile makes the parent declare a required env_file that does not exist,
	// so any path that loads the root environment reports it.
	brokenRootEnvFile bool
}

type ownerFixture struct {
	root      *config.Config
	parent    string
	child     string
	parentOut string
	childOut  string
}

// runInteraction drives the real `dva run` entry point, including its namespace parsing and
// its owner selection, rather than calling the helpers underneath it.
func (f *ownerFixture) runInteraction(t *testing.T, setup func(), args ...string) {
	t.Helper()
	f.withCLIState(t, setup)
	if err := runCmd.RunE(runCmd, args); err != nil {
		t.Fatalf("run %v: %v", args, err)
	}
}

func (f *ownerFixture) runProvision(t *testing.T, profile string) {
	t.Helper()
	f.withCLIState(t, nil)
	if err := provisionCmd.RunE(provisionCmd, []string{profile}); err != nil {
		t.Fatalf("provision %s: %v", profile, err)
	}
}

// withCLIState installs the fixture as the loaded config and clears the package globals the
// two commands read, restoring all of them afterwards. env is cleared as well as cfg: it is
// loadEnv's cache, and a value left over from a sibling subtest would answer for the root
// environment this one is trying to observe.
func (f *ownerFixture) withCLIState(t *testing.T, setup func()) {
	t.Helper()
	oldCfg, oldEnv := cfg, env
	oldDryRun, oldJSON, oldProject, oldPublish, oldList := dryRun, jsonOutput, projectName, publishPorts, provisionList
	t.Cleanup(func() {
		cfg, env = oldCfg, oldEnv
		dryRun, jsonOutput, projectName, publishPorts, provisionList = oldDryRun, oldJSON, oldProject, oldPublish, oldList
	})
	cfg, env = f.root, nil
	dryRun, jsonOutput, projectName, publishPorts, provisionList = false, false, "", nil, false
	if setup != nil {
		setup()
	}
}

func (f *ownerFixture) readOut(t *testing.T, dir, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("read %s: %v", filepath.Join(dir, name), err)
	}
	return strings.TrimSpace(string(data))
}

// wantChildGreet asserts on the whole record at once. The output dir is itself an assertion:
// OUTDIR is declared in both configs with different values, so a file appearing under the
// child's dir already proves the child's vars were the ones in effect.
func (f *ownerFixture) wantChildGreet(t *testing.T) {
	t.Helper()
	if got, want := f.readOut(t, f.childOut, "greet.out"), "child|child-top|child-file"; got != want {
		t.Fatalf("imported interaction env = %q, want %q", got, want)
	}
	if _, err := os.Stat(filepath.Join(f.parentOut, "greet.out")); !os.IsNotExist(err) {
		t.Fatalf("child interaction wrote into the parent output dir: %v", err)
	}
}

func (f *ownerFixture) wantChildGreetNoRootEnvFile(t *testing.T) {
	t.Helper()
	if got, want := f.readOut(t, f.childOut, "greet.out"), "child|child-top|child-file"; got != want {
		t.Fatalf("imported interaction env = %q, want %q", got, want)
	}
}

// wantChildProvision checks the value trio plus the working directory, which only the
// provision path can witness: its steps run through runShellCommand, the one executor that
// reads Environment.WorkDir.
func (f *ownerFixture) wantChildProvision(t *testing.T, profile string) {
	t.Helper()
	got := f.readOut(t, f.childOut, "provision.out")
	if want := "child|child-top|child-file|" + f.child; got != want {
		t.Fatalf("imported provision record = %q, want %q", got, want)
	}
	if _, err := os.Stat(filepath.Join(f.parentOut, "provision.out")); !os.IsNotExist(err) {
		t.Fatalf("child provision wrote into the parent output dir: %v", err)
	}
	// The marker belongs to the project the command was run against, and its name must
	// survive the "/" in a canonical import name — filepath.Join used to read that as a
	// directory component and the write failed with ENOENT.
	marker := filepath.Join(f.parent, config.DotDirName, provisionMarkerName(profile))
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("provision marker %s: %v", marker, err)
	}
}

func newOwnerFixture(t *testing.T, opts ownerFixtureOptions) *ownerFixture {
	t.Helper()

	// EvalSymlinks because $TMPDIR is a symlink on macOS: the fixture compares a recorded
	// $PWD against these paths, and the shell reports the resolved one.
	parent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	child := filepath.Join(parent, "child")
	parentOut := filepath.Join(parent, "out")
	childOut := filepath.Join(child, "out")
	for _, dir := range []string{childOut, parentOut} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	rootEnvFile := "env_file: .env"
	if opts.brokenRootEnvFile {
		rootEnvFile = "env_file:\n  files: missing.env\n  required: true"
	}

	// The record is OWNER|TOP_VALUE|ENV_FILE_VALUE — one value from each precedence layer
	// (vars, top-level environment:, env_file) so a fix that repairs only one layer fails
	// here. OUTDIR selects which side's directory the record lands in.
	writeFile(t, filepath.Join(parent, config.FileName), fmt.Sprintf(`version: "0.1.0"
vars:
  OWNER: parent
  OUTDIR: %s
environment:
  TOP_VALUE: parent-top
%s
interaction:
  greet:
    script_file: greet.sh
provision:
  setup:
    - step: record
      run: 'printf "%%s|%%s|%%s|%%s" "$OWNER" "$TOP_VALUE" "$ENV_FILE_VALUE" "$PWD" > "$OUTDIR/provision.out"'
subprojects:
  child:
    path: child
    import:
      interactions:
        - name: greet
          as: child-greet
      provision:
        - name: setup
          as: child-setup
`, parentOut, rootEnvFile))

	writeFile(t, filepath.Join(child, config.FileName), fmt.Sprintf(`version: "0.1.0"
vars:
  OWNER: child
  OUTDIR: %s
environment:
  TOP_VALUE: child-top
env_file: .env
interaction:
  greet:
    script_file: greet.sh
provision:
  setup:
    - step: record
      run: 'printf "%%s|%%s|%%s|%%s" "$OWNER" "$TOP_VALUE" "$ENV_FILE_VALUE" "$PWD" > "$OUTDIR/provision.out"'
`, childOut))

	writeFile(t, filepath.Join(parent, ".env"), "ENV_FILE_VALUE=parent-file\n")
	writeFile(t, filepath.Join(child, ".env"), "ENV_FILE_VALUE=child-file\n")

	// Same relative name on both sides. script_file resolves against the owning config's
	// directory, so the name alone cannot say which one runs — only the owner can.
	script := "#!/bin/sh\nprintf '%s|%s|%s' \"$OWNER\" \"$TOP_VALUE\" \"$ENV_FILE_VALUE\" > \"$OUTDIR/greet.out\"\n"
	for _, dir := range []string{parent, child} {
		path := filepath.Join(dir, "greet.sh")
		writeFile(t, path, script)
		if err := os.Chmod(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	root, err := config.Load(parent)
	if err != nil {
		t.Fatalf("load owner fixture: %v", err)
	}
	return &ownerFixture{root: root, parent: parent, child: child, parentOut: parentOut, childOut: childOut}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
