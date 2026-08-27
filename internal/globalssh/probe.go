package globalssh

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/castocolina/gitid/internal/sshconfig"
)

// ProbeHost is the dummy host `ssh -G` is run against (D-05). An `.invalid`
// TLD is correct for three reasons: it matches only a `Host *` wildcard
// stanza (never a per-alias block, so the probe shows the honest GLOBAL
// value), it participates in the system configuration and the build's own
// compiled defaults like any other host, and `ssh -G` only prints the
// resolved configuration — it never opens a socket, so the name is provably
// offline and safe.
const ProbeHost = "gitid-probe.invalid"

// probeTimeout bounds every real `ssh -G` invocation in this package so a
// pathological config (e.g. a hanging `Match exec`) can never block gitid
// indefinitely (T-06-02). It is a var, not a const, so tests can shrink it to
// exercise real timeout behavior without waiting out the production default —
// mirrors internal/platform's probeTimeout.
var probeTimeout = 3 * time.Second

// systemConfigPath is the root-owned file gitid reads for corroboration ONLY
// (T-06-31: it is world-readable by design, is never written, and its content
// never leaves the process except as a file path + line number in a label).
const systemConfigPath = "/etc/ssh/ssh_config"

// Deps is every external effect the probe set needs, injected as function
// fields so Statuses is fully mockable in tests. Every field is non-nil in
// the real BuildProbeDeps wiring and in test fakes, closing the project's
// documented injected-seam wiring blindspot (a nil or fixture seam silently
// reaching production).
//
// ReadConfig and ReadSystemConfig both return the PATH alongside the content,
// because a provenance label that names a file must name the file it actually
// read — never a path it merely intended to read.
type Deps struct {
	// RunSSHG executes one `ssh -G` probe with the given argument SLICE and
	// returns its stdout. It is bounded by probeTimeout and never runs
	// through a shell (T-06-03).
	RunSSHG func(ctx context.Context, args ...string) (string, error)
	// ReadConfig reads the per-user ~/.ssh/config gitid parses. A missing
	// file is the common first-run case: it returns the path with nil content
	// and a nil error (there is simply nothing to name).
	ReadConfig func() (path string, content []byte, err error)
	// ReadSystemConfig reads the system-wide ssh_config for corroboration
	// ONLY — gitid never changes it. The real binding returns a non-nil error
	// (never a panic) when the file is absent or unreadable; the classifier
	// treats that as "cannot corroborate", never as evidence.
	ReadSystemConfig func() (path string, content []byte, err error)
	// GOOS is the platform token (darwin/linux) used for the platform-gated
	// label and recommendation facts.
	GOOS string
}

// BuildProbeDeps returns production Deps wired to the REAL `ssh` binary and
// the real filesystem. It is exported and real-wired because this project has
// a documented injected-seam wiring blindspot where a nil or fixture seam
// reaches production; it mirrors platform.BuildProbeDeps's shape.
func BuildProbeDeps(sshConfigPath string) Deps {
	return Deps{
		RunSSHG: func(ctx context.Context, args ...string) (string, error) {
			if ctx == nil {
				ctx = context.Background()
			}
			cmd := exec.CommandContext(ctx, "ssh", args...) //nolint:gosec // arg-slice form, no shell; args are fixed probe flags + the .invalid constant (G204)
			// Run ssh in its own process group and SIGKILL the whole group on
			// timeout, exactly as sshconfig.RealMigrateDeps does: a
			// pathological `Match exec` can fork a grandchild that holds the
			// stdout pipe after the direct child is killed, which would block
			// Output() past the context deadline on Linux (a forking /bin/sh
			// makes the pipe hang). Group-killing reaps the grandchild and
			// WaitDelay is a belt-and-suspenders bound so Output() can never
			// wait on held pipes past the deadline (T-06-02).
			cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
			cmd.Cancel = func() error {
				if cmd.Process != nil {
					_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) // best-effort group kill
				}
				return nil
			}
			cmd.WaitDelay = 500 * time.Millisecond
			out, err := cmd.Output()
			if ctx.Err() != nil {
				return string(out), fmt.Errorf("globalssh: ssh -G timed out after %s: %w", probeTimeout, ctx.Err())
			}
			return string(out), err
		},
		ReadConfig: func() (string, []byte, error) {
			content, err := os.ReadFile(sshConfigPath) //nolint:gosec // sshConfigPath is a trusted gitid-managed path supplied in-process (G304)
			if os.IsNotExist(err) {
				return sshConfigPath, nil, nil
			}
			if err != nil {
				return "", nil, fmt.Errorf("globalssh: reading user ssh config %s: %w", sshConfigPath, err)
			}
			return sshConfigPath, content, nil
		},
		ReadSystemConfig: func() (string, []byte, error) {
			content, err := os.ReadFile(systemConfigPath) //nolint:gosec // fixed, non-user-controlled system path (G304)
			if err != nil {
				return "", nil, fmt.Errorf("globalssh: reading system ssh config %s: %w", systemConfigPath, err)
			}
			return systemConfigPath, content, nil
		},
		GOOS: runtime.GOOS,
	}
}

// effective resolves the value set the machine's configuration actually
// produces: `ssh -G gitid-probe.invalid` with NO `-F`, so every user + system
// config and the build's compiled defaults participate (D-01, D-05).
func effective(deps Deps) (map[string]string, error) {
	return runProbe(deps, "-G", ProbeHost)
}

// baseline resolves the value set THIS BUILD produces with no user
// configuration participating: `ssh -G -F /dev/null gitid-probe.invalid`.
//
// The isolated-config claim is deliberately scoped to exactly what this
// invocation provably establishes, and no more: ssh(1) documents that
// supplying a configuration file on the command line makes the per-user
// configuration file be replaced and the system-wide file be ignored, so the
// returned map is the value set this build resolves with no user
// configuration participating. It does NOT license a claim about which
// specific file a non-baseline value came from — a user Include, a Match
// block, an environment-supplied option or a vendor drop-in all produce the
// same "effective but not in our file and not the baseline" three-way
// signature (06-REVIEWS.md's HIGH provenance finding). isolation_contract_test.go
// is the executable proof of the user-config half of the claim, and the label
// wording rendered from a baseline value is scoped to the proven half.
func baseline(deps Deps) (map[string]string, error) {
	return runProbe(deps, "-G", "-F", "/dev/null", ProbeHost)
}

// runProbe runs one `ssh -G` invocation through the injected seam under the
// package's bounded timeout and parses its stdout. A probe error (including a
// timeout) is returned to the caller; Statuses degrades it to an advisory
// note, never a block.
func runProbe(deps Deps, args ...string) (map[string]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()
	out, err := deps.RunSSHG(ctx, args...)
	if err != nil {
		return nil, err
	}
	return parseResolvedOptions(out), nil
}

// parseResolvedOptions parses `ssh -G` output into a lowercase-key map. Keys
// are matched on a lowercase `<key> ` prefix, exactly as tester.ParseResolved
// does: camelCase lines never match, and that is a verified OpenSSH property,
// not an accident.
func parseResolvedOptions(out string) map[string]string {
	m := make(map[string]string)
	for _, line := range strings.Split(out, "\n") {
		idx := strings.IndexByte(line, ' ')
		if idx <= 0 {
			continue
		}
		key := line[:idx]
		if key != strings.ToLower(key) {
			continue // camelCase line: never match (tester.ParseResolved property)
		}
		m[key] = strings.TrimSpace(line[idx+1:])
	}
	return m
}

// policyKeys returns the canonical key spellings the probe set tracks, in
// Policy order.
func policyKeys() []string {
	keys := make([]string, len(Policy))
	for i, p := range Policy {
		keys[i] = p.Key
	}
	return keys
}

// hitsFromContent scans already-read config bytes for the six policy keys
// through sshconfig.ScanDirectives. The scan is best-effort and single-file
// (Include-unaware, exactly like its neighbours in sshconfig/reader.go): an
// empty result means "cannot name", never "does not exist".
//
// WR-15: this used to be fileHits(deps), which called deps.ReadConfig()
// itself — a SECOND, independent read from the one Statuses also performed
// for perAliasFromContent. Splitting the read from the scan lets Statuses
// derive both from the SAME deps.ReadConfig() call, so a write landing
// between two reads can no longer produce hits and the per-alias
// conformance count computed against two different file snapshots.
func hitsFromContent(content []byte, path string) map[string]sshconfig.DirectiveHit {
	hits := make(map[string]sshconfig.DirectiveHit)
	for _, hit := range sshconfig.ScanDirectives(content, path, policyKeys()) {
		hits[hit.Key] = hit
	}
	return hits
}
