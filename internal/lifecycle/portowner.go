package lifecycle

import (
	"context"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ScriptonBasestar/dva/internal/config"
)

// Port ownership resolution ties a tracked process to the process actually
// listening on its port. Apps are spawned with Setpgid, so the recorded PID
// (the `sh -c` wrapper) is the process-group leader and the real server is a
// child sharing that PGID. Matching a listener by PGID therefore proves the
// tracked process owns the port — and, crucially, tells a stale orphan from a
// previous run (a different group) apart from the service dva actually started.

// portOwnershipSupported reports whether port→PID resolution is available on
// this host. It currently relies on lsof; when lsof is absent, ownership-based
// checks fall back to legacy (probe-only) behavior instead of failing closed.
func portOwnershipSupported() bool {
	_, err := exec.LookPath("lsof")
	return err == nil
}

// portOwnerPIDs returns the PIDs of processes listening on the given TCP port.
// It shells out to lsof and returns nil when nothing listens or lsof is
// unavailable. Callers that must distinguish "no owner" from "cannot tell"
// should gate on portOwnershipSupported first.
func portOwnerPIDs(port int) []int {
	if port <= 0 {
		return nil
	}
	// -nP: skip name/port lookups; -iTCP:<port> -sTCP:LISTEN: listening sockets
	// on that port; -t: terse output (one PID per line). lsof exits non-zero
	// when there is no match, which Output surfaces as an error → nil.
	out, err := exec.Command("lsof", "-nP", "-iTCP:"+strconv.Itoa(port), "-sTCP:LISTEN", "-t").Output()
	if err != nil {
		return nil
	}
	var pids []int
	for f := range strings.FieldsSeq(string(out)) {
		if pid, err := strconv.Atoi(f); err == nil && pid > 0 {
			pids = append(pids, pid)
		}
	}
	return pids
}

// resolvePortOwnership reports which process currently listens on port and
// whether it belongs to the process group led by trackedPID. It returns
// (0, false) when the port is free. When the port is held by a process outside
// trackedPID's group (an orphan/foreign process), it returns (thatPID, false);
// pass trackedPID <= 0 to treat every listener as foreign.
func resolvePortOwnership(trackedPID, port int) (portPID int, owned bool) {
	owners := portOwnerPIDs(port)
	if len(owners) == 0 {
		return 0, false
	}
	for _, o := range owners {
		if trackedPID > 0 && o == trackedPID {
			return o, true
		}
		if trackedPID > 0 {
			if pgid, err := syscall.Getpgid(o); err == nil && pgid == trackedPID {
				return o, true
			}
		}
	}
	// No listener is part of our group — report the first as a foreign owner.
	return owners[0], false
}

// waitForPortOwnership polls until the process group led by pid owns port, or
// until timeout. It returns early with false once the tracked process has died
// without ever binding the port (its group can never own it afterwards).
func waitForPortOwnership(ctx context.Context, pid, port int, timeout time.Duration) bool {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	for {
		if _, owned := resolvePortOwnership(pid, port); owned {
			return true
		}
		if !IsProcessRunning(pid) {
			// Wrapper exited; give the port one last look (a fast child may
			// have bound and detached) then conclude.
			_, owned := resolvePortOwnership(pid, port)
			return owned
		}
		select {
		case <-ctx.Done():
			_, owned := resolvePortOwnership(pid, port)
			return owned
		case <-time.After(300 * time.Millisecond):
		}
	}
}

// reclaimPort terminates every process listening on port: SIGTERM first, then
// SIGKILL for survivors after a short grace period. It returns the PIDs it
// signalled. Used by `down` to free a port from orphans that outlived their
// process group. Signals are sent to the PID directly (not the group) because
// an orphan may have been reparented into a different group.
func reclaimPort(port int) []int {
	owners := portOwnerPIDs(port)
	if len(owners) == 0 {
		return nil
	}
	for _, pid := range owners {
		_ = syscall.Kill(pid, syscall.SIGTERM)
	}
	for range 20 {
		time.Sleep(100 * time.Millisecond)
		if len(portOwnerPIDs(port)) == 0 {
			return owners
		}
	}
	for _, pid := range portOwnerPIDs(port) {
		_ = syscall.Kill(pid, syscall.SIGKILL)
	}
	return owners
}

// effectivePort resolves the TCP port an app is expected to listen on for
// ownership checks. Preference: explicit `port:`, then the PORT environment
// variable, then an explicit port in the health check URL/address. Returns 0
// when none can be determined (ownership checks then fall back to probe-only).
func effectivePort(app *config.ApplicationConfig) int {
	if app == nil {
		return 0
	}
	if app.Port > 0 {
		return app.Port
	}
	if v := strings.TrimSpace(app.Environment["PORT"]); v != "" {
		if p, err := strconv.Atoi(v); err == nil && p > 0 {
			return p
		}
	}
	if app.Health != nil {
		if p := portFromURL(app.Health.URL); p > 0 {
			return p
		}
		if p := portFromHostPort(app.Health.Address); p > 0 {
			return p
		}
	}
	return 0
}

// portFromURL extracts an explicit port from a URL like http://host:PORT/path.
// It returns 0 when the URL carries no explicit port (scheme defaults such as
// 80/443 are intentionally not assumed for local dev services).
func portFromURL(raw string) int {
	if raw == "" {
		return 0
	}
	u, err := url.Parse(raw)
	if err != nil {
		return 0
	}
	if p, err := strconv.Atoi(u.Port()); err == nil && p > 0 {
		return p
	}
	return 0
}

// portFromHostPort extracts the port from a bare "host:port" address.
func portFromHostPort(addr string) int {
	if addr == "" {
		return 0
	}
	i := strings.LastIndex(addr, ":")
	if i < 0 {
		return 0
	}
	if p, err := strconv.Atoi(strings.TrimSpace(addr[i+1:])); err == nil && p > 0 {
		return p
	}
	return 0
}
