package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/ScriptonBasestar/dva/internal/config"
	dvaexec "github.com/ScriptonBasestar/dva/internal/exec"
	"github.com/ScriptonBasestar/dva/internal/lifecycle"
	"github.com/ScriptonBasestar/dva/internal/output"
)

// DoctorResult holds the outcome of a single doctor check.
// DoctorResult is one check's outcome.
//
// Name and Finding are deliberately different jobs, because one string cannot do both.
// Name is the check's stable identity — it must read the same whether the check passed or
// failed, so that a --json consumer can correlate runs and so that [pass] rows read as
// English. That forces Name into the shape of the assertion the check makes
// ("Docker daemon accessible"), which is exactly why it cannot also be the failure line:
// printed after [FAIL] it states the desired state as though it had been observed, and the
// only thing carrying the negation is a four-character tag that a grep, a copied line or a
// summary drops (TASK-139).
//
// Finding is what was actually observed, phrased so it cannot read as its own opposite. It
// is set only on rows that can fail, rendered only on the failure path, and omitted from
// JSON when empty. A check whose Name is already finding-shaped — those emitted only on
// failure, one per offending item — needs no Finding.
type DoctorResult struct {
	Name        string `json:"name"`
	Finding     string `json:"finding,omitempty"`
	Passed      bool   `json:"passed"`
	FixHint     string `json:"fix_hint,omitempty"`
	Fixable     bool   `json:"fixable,omitempty"`
	Fixed       bool   `json:"fixed,omitempty"`
	UserDefined bool   `json:"user_defined,omitempty"`
	fixFunc     func() error
}

var doctorFix bool

// doctorStrict inverts the advisory contract: by default a built-in check that fails is reported
// on screen but does not reach the exit code (TestDoctorExitError_BuiltinFailedOnly_Advisory
// pins this), because most built-ins are advice — a closed Docker Desktop, a missing gitignore
// line. --strict counts every failing check, so `dva doctor --strict && dva up` stops before up
// walks into a failure doctor already diagnosed, such as compose files that do not parse. The
// decision to keep advice out of the default exit code is TASK-122's to confirm; the switch
// itself lands now because it changes no caller's behaviour when off.
var doctorStrict bool

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check environment prerequisites and diagnose common setup issues",
	Long: `Run environment checks defined in the 'checks' section of dva.yml.
Also runs built-in checks for Docker availability and compose file existence.

Useful for diagnosing setup problems before running 'dva up' or 'dva provision'.
Use --fix to automatically resolve fixable issues.

By default a built-in check that fails is advisory: it prints [FAIL] but the exit
code stays 0, because most built-ins diagnose transient or environmental state
(Docker not running) rather than the configuration. Pass --strict to make every
failing check count toward the exit code, so CI fails when, for example, the
compose files do not parse.`,
	Example: `  dva doctor          # Check environment prerequisites (Docker, compose files, .env, ...)
  dva doctor --fix    # Automatically resolve fixable issues
  dva doctor --json   # JSON output
  dva doctor --strict # Count every failing check toward the exit code`,
	RunE: func(cmd *cobra.Command, args []string) error {
		c := mustLoadConfig()

		results := runDoctorChecks(c)

		if doctorFix {
			applyDoctorFixes(results)
		}

		if jsonOutput {
			if err := output.PrintJSON(map[string]any{"checks": results}); err != nil {
				return err
			}
			return doctorExitError(results, doctorStrict)
		}

		printDoctorResults(results)
		return doctorExitError(results, doctorStrict)
	},
}

func init() {
	doctorCmd.Flags().BoolVar(&doctorFix, "fix", false, "Automatically fix issues that can be resolved")
	doctorCmd.Flags().BoolVar(&doctorStrict, "strict", false, "Count every failing check toward the exit code (built-in checks are advisory by default)")
	doctorCmd.GroupID = "project"
	rootCmd.AddCommand(doctorCmd)
}

// runDoctorChecks runs built-in checks plus user-defined checks from dva.yml.
func runDoctorChecks(c *config.Config) []DoctorResult {
	var results []DoctorResult

	// Built-in: portable daemon check, with a Linux-only socket diagnostic on failure.
	results = append(results, runDockerDoctorChecks(checkDocker, checkDockerSocketPermissions, runtime.GOOS)...)

	// Built-in: Compose project name alignment
	results = append(results, checkComposeProjectNameAlignment(c))

	// Built-in: a subproject claiming the parent's compose project name
	results = append(results, checkSubprojectComposeProjectNames(c)...)

	// Built-in: environment inputs. This is the same report the execution routes fail
	// closed on, inspected rather than applied, so doctor and `up` can never disagree
	// about which declarations are loadable (TASK-247 §5).
	envReport := config.InspectEnvFiles(c.EnvFile, c.FileDir())
	results = append(results, envInputResults(envReport)...)

	// Built-in: non-Compose stack entry files exist
	results = append(results, checkStackFiles(c)...)

	if envReport.Incomplete() {
		// Both compose checks below interpolate their paths with env-file-derived
		// values, and the second starts a compose child. Running them on an
		// environment DVA has already refused would report a MISSING file whose name
		// was resolved against inputs that were never applied — a failure against a
		// config that is fine. So they are skipped, and the skip is a row rather than
		// a silence, because a check that quietly does not run reads as a check that
		// passed. passed:true: not running a check is not evidence of a broken file,
		// and the env failure row above already carries the real finding.
		//
		// The guards keep parity with the checks themselves — neither emits anything
		// for a config with no compose files, so neither may emit a skip notice.
		if len(c.AllComposeFiles()) > 0 {
			results = append(results, DoctorResult{
				Name:   "Compose file existence (skipped: environment input unavailable)",
				Passed: true,
			})
		}
		if cc := c.PrimaryComposeConfig(); cc != nil && len(cc.Files) > 0 {
			results = append(results, DoctorResult{
				Name:   "Compose config resolves (skipped: environment input unavailable)",
				Passed: true,
			})
		}
	} else {
		// Built-in: Compose files exist
		results = append(results, checkComposeFiles(c)...)

		// Built-in: Compose config resolves (catches include:/-f targets that the
		// per-file existence check above misses — e.g. compose.yaml includes a file
		// that was renamed/removed). Runs the configured compose command's `config`,
		// which needs no daemon (TASK-119 made the check binary match the config).
		results = append(results, checkComposeConfigResolves(c)...)
	}

	for _, check := range c.DoctorChecks {
		r := runSingleCheck(check, c.FileDir())
		r.UserDefined = true
		results = append(results, r)
	}

	// Built-in: devcontainer.json exists (when devcontainer section is enabled)
	if len(c.Devcontainer) > 0 && isDevcontainerEnabled(c.Devcontainer) {
		results = append(results, runSingleCheck(config.DoctorCheck{
			Name:    "devcontainer.json exists",
			Type:    "file_exists",
			Path:    ".devcontainer/devcontainer.json",
			FixHint: "Run: dva config validate --fix",
		}, c.FileDir()))
	}

	// Built-in: Check if .sb/dva is ignored in .gitignore
	results = append(results, checkGitignoreStatus(c.FileDir()))

	// The application port-ownership check used to run here. It read
	// `applications.<app>.port` (and the port implied by health.url/address) and asked
	// lsof whether the process holding it was one dva had started — the condition that
	// lets a dead app look healthy because a stale orphan is answering. Both its input
	// and its implementation went with `applications:` (docs/43); stack entries declare
	// no port for it to check, so it is not portable to the plan path as it stood.

	return results
}

func runDockerDoctorChecks(
	daemonCheck func() DoctorResult,
	socketCheck func() DoctorResult,
	goos string,
) []DoctorResult {
	daemon := daemonCheck()
	results := []DoctorResult{daemon}
	if !daemon.Passed && goos == "linux" {
		results = append(results, socketCheck())
	}
	return results
}

// applyDoctorFixes attempts to fix all failed checks that have a fix function or command.
func applyDoctorFixes(results []DoctorResult) {
	for i := range results {
		r := &results[i]
		if r.Passed || !r.Fixable {
			continue
		}
		if r.fixFunc != nil {
			if err := r.fixFunc(); err != nil {
				fmt.Fprintf(os.Stderr, "  [fix-err] %s: %v\n", r.Name, err)
			} else {
				r.Fixed = true
				r.Passed = true
				// The finding described the state before the fix ran, so it is no longer
				// true. Leaving it would ship a row that is passed:true with a finding
				// saying otherwise — the same contradiction this field exists to remove.
				r.Finding = ""
			}
		}
	}
}

func checkGitignoreStatus(configDir string) DoctorResult {
	r := DoctorResult{Name: fmt.Sprintf("%s/ is ignored in .gitignore", config.DotDirName)}

	gitignorePath := filepath.Join(configDir, ".gitignore")
	data, err := os.ReadFile(gitignorePath)
	if err != nil {
		if os.IsNotExist(err) {
			r.Passed = false
			r.Finding = fmt.Sprintf("no .gitignore here, so %s/ is not ignored", config.DotDirName)
			r.Fixable = true
			r.FixHint = fmt.Sprintf("Create .gitignore and add '%s/' to avoid committing transient state", config.DotDirName)
			r.fixFunc = func() error {
				_, err := ensureGitignore(configDir)
				return err
			}
			return r
		}
		r.Passed = false
		r.Finding = fmt.Sprintf(".gitignore could not be read, so %s/ cannot be confirmed ignored: %v", config.DotDirName, err)
		return r
	}

	if isDvaIgnored(string(data)) {
		r.Passed = true
	} else {
		r.Passed = false
		r.Finding = fmt.Sprintf("%s/ is NOT ignored in .gitignore", config.DotDirName)
		r.Fixable = true
		r.FixHint = fmt.Sprintf("Add '%s/' to .gitignore to avoid committing transient state", config.DotDirName)
		r.fixFunc = func() error {
			_, err := ensureGitignore(configDir)
			return err
		}
	}

	return r
}

func runSingleCheck(check config.DoctorCheck, configDir string) DoctorResult {
	r := DoctorResult{
		Name:    check.Name,
		FixHint: check.FixHint,
	}

	// check.Name comes from dva.yml, so it is whatever the author wrote — and authors write
	// assertions ("devcontainer.json exists", and the built-in devcontainer check below is
	// itself routed through here). The finding is derived from the check's own inputs rather
	// than from that name, so a user-defined row cannot state the opposite of what happened
	// either. Same convention as the built-ins, applied where the wording is not ours.
	switch check.Type {
	case "file_exists":
		path := check.Path
		if !filepath.IsAbs(path) {
			path = filepath.Join(configDir, path)
		}
		r.Passed = fileExists(path)
		r.Finding = condStr(!r.Passed, fmt.Sprintf("no file at %s", check.Path))

	case "command":
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		r.Passed = exec.CommandContext(ctx, "sh", "-c", check.Command).Run() == nil
		r.Finding = condStr(!r.Passed, fmt.Sprintf("command exited non-zero: %s", check.Command))

	case "docker_socket":
		// Full result, not only Passed: checkDockerSocketPermissions names the path it
		// measured (or that docker info failed), and collapsing it to a boolean used to
		// replace that with the generic "Docker socket is NOT accessible" line — which is
		// how a missing default path looked identical to a permissions problem (TASK-180).
		sock := checkDockerSocketPermissions()
		r.Passed = sock.Passed
		if !sock.Passed {
			r.Finding = sock.Finding
			if r.FixHint == "" {
				r.FixHint = sock.FixHint
			}
		}

	default:
		r.Passed = false
		r.Finding = fmt.Sprintf("check %q declares unknown type %q, so nothing was verified", check.Name, check.Type)
		r.FixHint = fmt.Sprintf("Unknown check type: %s", check.Type)
	}

	// Clear fix hint and finding on success: a finding describes something observed to be
	// wrong, and leaving one on a passing row would put it in --json for consumers to trip on.
	if r.Passed {
		r.FixHint = ""
		r.Finding = ""
	}

	// If check has a fix command, mark as fixable
	if !r.Passed && check.Fix != "" {
		r.Fixable = true
		fixCmd := check.Fix
		r.fixFunc = func() error {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, "sh", "-c", fixCmd)
			cmd.Dir = configDir
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			return cmd.Run()
		}
	}

	return r
}

func checkDocker() DoctorResult {
	r := DoctorResult{Name: "Docker daemon accessible"}

	// Shared with the compose lifecycle path, which consults the same probe after a
	// failed command so the daemon diagnosis a failing `dva up` prints and the one
	// doctor reports here cannot drift apart. nil env: doctor probes the ambient
	// environment, as it always has.
	r.Passed = lifecycle.DockerDaemonReachable(nil)

	if !r.Passed {
		r.Finding = "Docker daemon is NOT accessible ('docker info' failed)"
		r.FixHint = "Start Docker Desktop or ensure dockerd is running"
	}
	return r
}

// resolveDockerSocketPath maps DOCKER_HOST to a local Unix socket filesystem path.
//
// Split out of checkDockerSocketPermissions so the mapping is assertable without a
// daemon (TASK-180). Empty dockerHost uses the historical default; unix:// yields that
// path; any other scheme (tcp, ssh, npipe, fd) yields an empty path because there is no
// local socket file to open — the check then falls through to the daemon probe.
func resolveDockerSocketPath(dockerHost string) (path string, fromEnv bool) {
	dockerHost = strings.TrimSpace(dockerHost)
	if dockerHost == "" {
		return "/var/run/docker.sock", false
	}
	const unix = "unix://"
	if after, ok := strings.CutPrefix(dockerHost, unix); ok {
		return after, true
	}
	return "", true
}

func checkDockerSocketPermissions() DoctorResult {
	return evaluateDockerSocket(os.Getenv("DOCKER_HOST"), func() bool {
		return lifecycle.DockerDaemonReachable(nil)
	})
}

// evaluateDockerSocket is the pure body of type: docker_socket / the Linux built-in
// socket diagnostic.
//
// Decision (TASK-180): honour DOCKER_HOST's unix socket when one is configured, and when
// there is no local socket path to measure — default path missing, or a non-unix
// DOCKER_HOST — fall through to the same daemon probe checkDocker uses. That is the
// choice that keeps the two verdicts from disagreeing on a healthy Colima/Podman/Desktop
// host: a path check alone would still hard-fail when the default layout is absent, and
// a daemon-only check would drop the permission finding the type name still implies.
// When an explicit unix:// DOCKER_HOST points at a missing file, that is reported as the
// failure (docker info would fail the same way).
func evaluateDockerSocket(dockerHost string, daemonOK func() bool) DoctorResult {
	path, fromEnv := resolveDockerSocketPath(dockerHost)

	if path != "" {
		if _, err := os.Stat(path); err == nil {
			f, err := os.Open(path)
			if err != nil {
				return DoctorResult{
					Name:    "Docker socket permissions",
					Finding: fmt.Sprintf("%s exists but this user cannot open it", path),
					Passed:  false,
					FixHint: "Add user to docker group or use sudo",
				}
			}
			_ = f.Close()
			return DoctorResult{
				Name:   "Docker socket permissions",
				Passed: true,
			}
		}
		if fromEnv {
			return DoctorResult{
				Name:    "Docker socket accessible",
				Finding: fmt.Sprintf("no Docker socket at %s (from DOCKER_HOST)", path),
				Passed:  false,
				FixHint: "Start the Docker daemon or set DOCKER_HOST to a live unix socket",
			}
		}
	}

	// Default path absent, or DOCKER_HOST is not a local Unix socket: align with the
	// portable probe so this check cannot pass while checkDocker fails, or the reverse.
	if daemonOK != nil && daemonOK() {
		return DoctorResult{
			Name:   "Docker socket permissions",
			Passed: true,
		}
	}

	finding := "Docker daemon is not reachable ('docker info' failed)"
	if path != "" {
		finding = fmt.Sprintf("no Docker socket at %s and 'docker info' failed", path)
	} else if strings.TrimSpace(dockerHost) != "" {
		finding = fmt.Sprintf("DOCKER_HOST=%s is not a local Unix socket and 'docker info' failed", dockerHost)
	}
	return DoctorResult{
		Name:    "Docker socket accessible",
		Finding: finding,
		Passed:  false,
		FixHint: "Start Docker Desktop, colima, or dockerd; if DOCKER_HOST is set, check it",
	}
}

func checkComposeProjectNameAlignment(c *config.Config) DoctorResult {
	warnings := c.ValidateComposeProjectNames()
	if len(warnings) == 0 {
		return DoctorResult{
			Name:   "Compose project name alignment",
			Passed: true,
		}
	}

	w := warnings[0] // show first warning
	msg := fmt.Sprintf("Compose file %s has %s", w.File,
		condStr(w.ComposeName == "", "missing project name", fmt.Sprintf("name '%s'", w.ComposeName)))

	// Name is the same string the passing branch above returns. It used to be msg, so this
	// one check reported itself under two different JSON names depending on the outcome, and
	// a consumer keying on "name" could not correlate a failing run with a passing one. The
	// observation moved to Finding, which is where it belonged: the human line is unchanged.
	return DoctorResult{
		Name:    "Compose project name alignment",
		Finding: msg,
		Passed:  false,
		FixHint: fmt.Sprintf("Set 'name: %s' in %s", w.DvaName, w.File),
		Fixable: true,
		fixFunc: func() error { return c.FixComposeProjectName(w) },
	}
}

// checkSubprojectComposeProjectNames reports a subproject whose compose project name equals
// the parent's while the two point at different compose files.
//
// Compose project identity is what `docker compose down` scopes to, so two configs sharing a
// name while describing different stacks means `dva down` in the child reaps the parent's
// containers. Every single-file view looks correct: checkComposeProjectNameAlignment passes
// on both, because it checks a different axis — that one dva.yml's project_name matches its
// own compose file's top-level name.
//
// This lives in doctor rather than validate because validate never sees the child. A
// subproject is only loaded when it declares an import: block (config.resolveSubprojectImports),
// and the collision arises in configs that declare just a path — so detecting it means opening
// a file validate has no reason to open. doctor is diagnostic and already reads the filesystem.
//
// Comparing file sets rather than service sets is deliberate: DVA has no compose service
// parser, and file identity is what decides whether two configs hand docker the same stack.
// Identical files mean an overlay-style split, which is legitimate and stays silent.
func checkSubprojectComposeProjectNames(c *config.Config) []DoctorResult {
	parentName := c.ComposeProjectName()
	if parentName == "" || len(c.Subprojects) == 0 {
		return nil
	}
	parentFiles := absComposeFiles(c)

	// Sorted so the output is stable; map iteration order would reshuffle results per run.
	names := make([]string, 0, len(c.Subprojects))
	for name := range c.Subprojects {
		names = append(names, name)
	}
	sort.Strings(names)

	var results []DoctorResult
	for _, name := range names {
		sub := c.Subprojects[name]
		// One subproject at a time: LoadSubprojects returns nil, err on any single failure,
		// so passing the whole map would let one missing child hide every other result.
		subs, err := config.LoadSubprojects(c.FileDir(), map[string]config.SubprojectConfig{name: sub},
			config.SkipVersionCheck())
		if err != nil {
			// Unloadable child is not this check's business — checkStackFiles and the
			// loader's own errors report it. Staying silent here avoids a second complaint
			// about the same file.
			continue
		}
		subCfg := subs[name]
		if subCfg == nil || subCfg.ComposeProjectName() != parentName {
			continue
		}
		if sameStringSet(parentFiles, absComposeFiles(subCfg)) {
			continue // same stack under one project name — an overlay split, not a collision
		}

		results = append(results, DoctorResult{
			Name: fmt.Sprintf("Subproject %q shares compose project name %q with the parent but references different compose files",
				name, parentName),
			Passed: false,
			FixHint: fmt.Sprintf("Give %s its own project_name; otherwise 'dva down' in %s removes the parent's containers too",
				name, name),
		})
	}
	return results
}

// absComposeFiles returns c's compose files as absolute paths, so two configs in different
// directories are compared by the file they actually resolve to rather than by the relative
// spelling each happens to use.
func absComposeFiles(c *config.Config) []string {
	dir := c.FileDir()
	files := c.AllComposeFiles()
	out := make([]string, 0, len(files))
	for _, f := range files {
		if !filepath.IsAbs(f) {
			f = filepath.Join(dir, f)
		}
		out = append(out, filepath.Clean(f))
	}
	return out
}

// sameStringSet reports whether a and b contain the same elements, ignoring order and
// duplicates. Order is irrelevant here: `docker compose -f x -f y` and `-f y -f x` merge to
// the same project.
func sameStringSet(a, b []string) bool {
	if len(a) == 0 || len(b) == 0 {
		return false // nothing to compare — do not claim the stacks match
	}
	set := make(map[string]bool, len(a))
	for _, s := range a {
		set[s] = true
	}
	seen := make(map[string]bool, len(b))
	for _, s := range b {
		if !set[s] {
			return false
		}
		seen[s] = true
	}
	return len(seen) == len(set)
}

// checkEnvFiles reports one row per env-file declaration.
//
// It replaces the former existence check, and the difference is not cosmetic: existence
// was never the question a user needed answered. A file that exists but cannot be read,
// or whose line 12 is not an assignment, used to pass doctor and then stop `dva up`, so
// the command whose whole job is explaining why things do not work was the one surface
// that could not see the reason. Asking the loader instead of os.Stat means doctor
// reports exactly what the runtime will decide.
func checkEnvFiles(c *config.Config) []DoctorResult {
	return envInputResults(config.InspectEnvFiles(c.EnvFile, c.FileDir()))
}

// envInputResults renders one report into doctor rows, in declaration order.
//
// An optional file that is simply absent produces no row at all. That is the author
// having said "use this if it is here", and reporting a row for it would turn a
// deliberate choice into something that looks like a finding.
func envInputResults(r *config.EnvInputReport) []DoctorResult {
	var results []DoctorResult
	for _, entry := range r.Entries {
		if entry.Status == config.EnvInputSkipped {
			continue
		}
		// One Name across both outcomes: a --json consumer correlates a failing run
		// with a passing one by this string, so it may not encode the verdict.
		result := DoctorResult{
			Name:   fmt.Sprintf("Environment input loads: %s", entry.File),
			Passed: entry.Status == config.EnvInputLoaded,
		}
		if !result.Passed {
			// entry.Reason() is the frozen, content-free explanation. Nothing here can
			// widen it to a key, a value or a merge count, because the report does not
			// carry those in the first place.
			result.Finding = fmt.Sprintf("Environment input is UNAVAILABLE: %s", entry.Reason())
			result.FixHint = fmt.Sprintf("Fix env_file entry: %s", entry.File)
		}
		results = append(results, result)
	}
	return results
}

func checkStackFiles(c *config.Config) []DoctorResult {
	var results []DoctorResult
	cfgDir := c.FileDir()

	for name, entry := range c.Stack {
		if entry.ComposeConfig() != nil {
			continue
		}

		// KubectlConfig(), not the typed field: runners.kubectl is the supported form and
		// reading .Kubectl alone skipped every modern entry (TASK-150).
		var files []string
		if kc := entry.KubectlConfig(); kc != nil {
			files = []string{kc.Kubeconfig}
		}

		for _, f := range files {
			if f == "" {
				continue
			}
			path := f
			if !filepath.IsAbs(path) {
				path = filepath.Join(cfgDir, f)
			}

			passed := fileExists(path)
			results = append(results, DoctorResult{
				Name:    fmt.Sprintf("Stack file exists: %s (%s)", name, f),
				Finding: condStr(!passed, fmt.Sprintf("Stack entry %q references a MISSING file: %s", name, f)),
				Passed:  passed,
				FixHint: condStr(!passed, fmt.Sprintf("Create or check path: %s", f)),
			})
		}
	}

	return results
}

// checkComposeFiles reports whether each compose file named in the config is on disk.
//
// The name is interpolated before the lookup, as every runner does. Without that, a
// `files:` entry of compose.${STAGE}.yml was checked under that literal name, no such file
// existed, and doctor reported a failure against a config that runs perfectly — measured on
// a fixture whose compose.dev.yml was present the whole time. The result still reports the
// written form rather than the expanded one, because that is the line the user has to go
// and edit. TASK-119.
func checkComposeFiles(c *config.Config) []DoctorResult {
	var results []DoctorResult
	seen := make(map[string]struct{})
	e, _ := loadEnv(c)

	for _, f := range c.AllComposeFiles() {
		if f == "" {
			continue
		}

		path := e.Interpolate(f)
		if !filepath.IsAbs(path) {
			path = filepath.Join(c.FileDir(), path)
		}
		path = filepath.Clean(path)
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}

		passed := fileExists(path)
		results = append(results, DoctorResult{
			Name:    fmt.Sprintf("Compose file exists: %s", f),
			Finding: condStr(!passed, fmt.Sprintf("Compose file is MISSING: %s", f)),
			Passed:  passed,
			FixHint: condStr(!passed, fmt.Sprintf("Create or check path: %s", f)),
		})
	}

	return results
}

// checkComposeConfigResolves runs `compose config -q` for the primary compose file set so a
// compose.yaml whose -f or include: target does not resolve is caught here (before `dva up`)
// rather than only failing at up time. `compose config` only parses and merges files and
// needs no daemon.
//
// The argv comes from dvaexec.ComposeArgv, the same builder every runner uses, and that is
// the point rather than an implementation detail. This function used to hardcode `docker`
// and never read `command:`, so a config saying `command: podman-compose` was checked by
// running docker — the check passed or failed on a tool the user was not running. Measured
// with a PATH shim: `docker compose -f … config --quiet` was executed and podman-compose was
// never invoked at all. Going through ComposeArgv also brings the interpolation the four
// runners already had, so a `files:` entry of compose.${STAGE}.yml is no longer checked
// under that literal name. TASK-119.
func checkComposeConfigResolves(c *config.Config) []DoctorResult {
	cc := c.PrimaryComposeConfig()
	if cc == nil || len(cc.Files) == 0 {
		return nil
	}

	e, _ := loadEnv(c)
	composeCmd, args, err := dvaexec.ComposeArgv(e, cc, c.FileDir())
	if err != nil {
		// A command: that splits to no words. `dva doctor` is the command people run to
		// find out what is wrong, so this is the one place it should certainly not be
		// silent about.
		return []DoctorResult{{
			Name:    "Compose config resolves",
			Finding: "compose config could NOT be run: the configured command splits to no words",
			Passed:  false,
			FixHint: err.Error(),
		}}
	}

	if _, lookErr := exec.LookPath(composeCmd); lookErr != nil {
		// Reported, not skipped. This used to `return nil`, which meant a podman-only
		// machine got no compose check and no note that one had been dropped — and the
		// old comment justified it by pointing at the daemon check, which reports the
		// absence of docker, not the absence of this check. A check that silently does
		// not run is the defect shape this repo keeps producing.
		//
		// The name has to carry the whole fact because printDoctorResults has two states,
		// pass and FAIL, and prints FixHint only on FAIL. Passed is true because a missing
		// binary is not evidence that the user's compose files are wrong, which is what
		// this check claims to be about; the hint below therefore reaches --json consumers
		// only, and the name is what a human reads.
		return []DoctorResult{{
			Name:    fmt.Sprintf("Compose config resolves (skipped: %s is not on PATH)", composeCmd),
			Passed:  true,
			FixHint: fmt.Sprintf("install %s, or point stack.<entry>.runners.compose.command at a binary on PATH", composeCmd),
		}}
	}

	args = append(args, "config", "--quiet")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, composeCmd, args...)
	cmd.Dir = c.FileDir()
	out, err := cmd.CombinedOutput()
	if err == nil {
		return []DoctorResult{{Name: "Compose config resolves", Passed: true}}
	}

	detail := firstNonEmptyLine(string(out))
	// composeCmd is the binary name only ("docker"), not the full invocation prefix — the
	// `compose` subcommand lives in args. The hint must be a command the user can run, so show what
	// they configured (cc.Command) or the "docker compose" default, not the bare binary (TASK-156).
	composePrefix := cc.Command
	if composePrefix == "" {
		composePrefix = "docker compose"
	}
	hint := fmt.Sprintf("check compose.files in dva.yml and any include: paths, then run: %s config", composePrefix)
	if detail != "" {
		hint = detail + " — " + hint
	}
	return []DoctorResult{{
		Name:    "Compose config resolves",
		Finding: fmt.Sprintf("compose config does NOT resolve (%s config exited non-zero)", composePrefix),
		Passed:  false,
		FixHint: hint,
	}}
}

// firstNonEmptyLine returns the first non-blank line of s, trimmed.
func firstNonEmptyLine(s string) string {
	for line := range strings.SplitSeq(s, "\n") {
		if t := strings.TrimSpace(line); t != "" {
			return t
		}
	}
	return ""
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// failureLine is what a reader sees after [FAIL]: the finding when the check recorded one,
// and otherwise the name, which the fail-only checks already phrase as a finding. Falling
// back rather than requiring Finding everywhere keeps rows like
// `App "web" port 3000 held by a process dva did not start` unchanged — they never render
// on the pass path, so their name is already the observation.
func (r DoctorResult) failureLine() string {
	if r.Finding != "" {
		return r.Finding
	}
	return r.Name
}

func printDoctorResults(results []DoctorResult) {
	fmt.Println("Environment Checks:")
	fmt.Println()

	passed := 0
	failed := 0
	fixed := 0
	for _, r := range results {
		if r.Fixed {
			fmt.Printf("  [fixed] %s\n", r.Name)
			fixed++
			passed++
		} else if r.Passed {
			fmt.Printf("  [pass] %s\n", r.Name)
			passed++
		} else {
			fmt.Printf("  [FAIL] %s\n", r.failureLine())
			if r.FixHint != "" {
				fmt.Printf("         -> %s\n", r.FixHint)
			}
			if r.Fixable && !strings.Contains(r.FixHint, "--fix") {
				fmt.Printf("         -> Run 'dva doctor --fix' to auto-fix\n")
			}
			failed++
		}
	}

	summary := fmt.Sprintf("\n  %d passed, %d failed", passed, failed)
	if fixed > 0 {
		summary += fmt.Sprintf(" (%d auto-fixed)", fixed)
	}
	fmt.Println(summary)
}

// doctorExitError decides whether failing checks reach the exit code.
//
// The default contract is advisory: only user-defined checks (the dva.yml 'checks' section)
// count, so a built-in [FAIL] prints but leaves exit 0. strict inverts that and counts every
// failing check, so a compose config that does not resolve fails the run under --strict. The
// advisory default is pinned by TestDoctorExitError_BuiltinFailedOnly_Advisory; strict is pinned
// by TestDoctorExitError_StrictCountsBuiltins. TASK-122.
func doctorExitError(results []DoctorResult, strict bool) error {
	failed := 0
	for _, r := range results {
		if (r.UserDefined || strict) && !r.Passed {
			failed++
		}
	}
	if failed == 0 {
		return nil
	}
	if strict {
		return fmt.Errorf("%d check(s) failed (--strict)", failed)
	}
	return fmt.Errorf("%d user check(s) failed", failed)
}

func condStr(cond bool, s string, fallback ...string) string {
	if cond {
		return s
	}
	if len(fallback) > 0 {
		return fallback[0]
	}
	return ""
}
