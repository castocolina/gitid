//go:build e2e || realaccount

package e2e

import (
	"errors"
	"fmt"
	"go/ast"
	"go/build/constraint"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// packageBin is the shared binary path built once for the entire test package
// via buildOnce. Using a package-level binary avoids running `go build` inside
// t.TempDir() (which would put the Go module cache there, causing permission
// errors during cleanup because go.mod files are read-only).
//
// realHome is captured at package init time — before any test can call
// t.Setenv("HOME", sandbox). This preserves the original GOPATH derivation
// even when tests change HOME to a sandbox.
var (
	buildOnce  sync.Once
	packageBin string
	buildErr   error
	realHome   = os.Getenv("HOME") // captured at init, before any t.Setenv
)

// Dummy-demo binary cache — a separate sync.Once/path/err triple from the
// ones above, so BuildDummyBinary's cached `cmd/gitid-dummy` build never
// collides with BuildBinary's cached `cmd/gitid` build.
var (
	dummyBuildOnce  sync.Once
	dummyPackageBin string
	dummyBuildErr   error
)

// SandboxHome creates a hermetic HOME directory and sets HOME to it via
// t.Setenv so it is automatically restored after the test.
// All files the binary writes (including ~/.ssh and ~/.gitconfig) land there.
func SandboxHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	return home
}

// ShortSandboxHome is SandboxHome with a SHORT, fixed-prefix path
// (os.MkdirTemp, mirroring BuildBinary's own short-path convention) instead
// of t.TempDir()'s test-name-embedding path — needed by any assertion that
// checks a REAL PTY frame for a literal, un-clipped absolute path (e.g. the
// D-13 scan-hit (file, line) identity assertions): t.TempDir()'s path
// embeds the full test function name plus a subtest counter, easily
// exceeding the ~55-column budget a bounded PreviewBlock clips long lines
// to, which would truncate the very line number the assertion needs to see.
func ShortSandboxHome(t *testing.T) string {
	t.Helper()
	// "/tmp" explicitly (never os.MkdirTemp("", ...), which honors $TMPDIR —
	// on macOS test runs that resolves to a long per-process
	// /var/folders/.../T/ path, defeating the whole point of this helper).
	// The prefix itself is kept to 2 chars — every extra byte here is a byte
	// a bounded PreviewBlock's clip has less room for.
	home, err := os.MkdirTemp("/tmp", "h")
	if err != nil {
		t.Fatalf("ShortSandboxHome: MkdirTemp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(home) })
	t.Setenv("HOME", home)
	return home
}

// BuildBinary compiles the gitid binary once per test package run and returns
// its path. Subsequent calls return the same cached binary path without
// recompiling (fast and deterministic). The binary is placed in os.MkdirTemp
// (not t.TempDir) so the Go module cache does not land inside a test-managed
// directory — this avoids cleanup permission errors (go.mod is read-only).
func BuildBinary(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "gitid-e2e-*")
		if err != nil {
			buildErr = err
			return
		}
		bin := filepath.Join(dir, "gitid")
		cmd := exec.Command("go", "build", "-o", bin, "./cmd/gitid")
		cmd.Dir = repoRoot(t)
		// Restore the original HOME so `go build` derives GOPATH from the real
		// home (not from a sandbox). This prevents go.mod files being written
		// into t.TempDir() which would cause cleanup permission errors.
		cmd.Env = append(os.Environ(), "HOME="+realHome)
		if combined, berr := cmd.CombinedOutput(); berr != nil {
			buildErr = fmt.Errorf("%w\n%s", berr, combined)
			_ = os.RemoveAll(dir)
			return
		}
		packageBin = bin
	})
	if buildErr != nil {
		t.Fatalf("BuildBinary: go build failed: %v", buildErr)
	}
	if packageBin == "" {
		t.Fatal("BuildBinary: binary path is empty after build")
	}
	return packageBin
}

// BuildDummyBinary compiles the cmd/gitid-dummy binary once per test package
// run and returns its path. It mirrors BuildBinary exactly (sync.Once-cached,
// os.MkdirTemp so the Go module cache never lands inside a test-managed
// t.TempDir(), HOME=realHome restored during the build) but targets
// ./cmd/gitid-dummy and uses its own dummyBuildOnce/dummyPackageBin pair so
// it never collides with BuildBinary's real-product binary cache.
func BuildDummyBinary(t *testing.T) string {
	t.Helper()
	dummyBuildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "gitid-dummy-e2e-*")
		if err != nil {
			dummyBuildErr = err
			return
		}
		bin := filepath.Join(dir, "gitid-dummy")
		cmd := exec.Command("go", "build", "-o", bin, "./cmd/gitid-dummy")
		cmd.Dir = repoRoot(t)
		// Restore the original HOME so `go build` derives GOPATH from the
		// real home (not a sandbox), preventing go.mod writes into
		// t.TempDir() (which would cause cleanup permission errors).
		cmd.Env = append(os.Environ(), "HOME="+realHome)
		if combined, berr := cmd.CombinedOutput(); berr != nil {
			dummyBuildErr = fmt.Errorf("%w\n%s", berr, combined)
			_ = os.RemoveAll(dir)
			return
		}
		dummyPackageBin = bin
	})
	if dummyBuildErr != nil {
		t.Fatalf("BuildDummyBinary: go build failed: %v", dummyBuildErr)
	}
	if dummyPackageBin == "" {
		t.Fatal("BuildDummyBinary: binary path is empty after build")
	}
	return dummyPackageBin
}

// FakeSSHDir writes a mode-switching fake ssh script to a temp dir and sets
// GITID_FAKE_SSH_MODE to mode. The caller prepends the returned dir to PATH via
// cmd.Env so the child gitid binary resolves the fake ssh instead of /usr/bin/ssh.
//
// The script is a static string literal — never constructed from user input and
// never passed to a shell interpreter from Go code (D-20, gosec G-204 safe).
//
// Modes:
//
//	pass    — emits "successfully authenticated" banner (tester.PASS outcome)
//	denied  — emits "Permission denied (publickey)" (tester.ReachableNotUploaded)
//	timeout — emits a connect-timeout line (tester.Failure)
//
// The script also handles "ssh -G <alias>" by emitting a fixture ssh -G block
// so the Resolved dep works correctly in E2E tests.
func FakeSSHDir(t *testing.T, mode string) string {
	t.Helper()
	dir := t.TempDir()

	// Static string literal — not constructed from user input, never exec'd via sh -c.
	// The -Q branch must come first (ProbeKeyTypes: `ssh -Q key`, -Q is
	// always $1), then -G (stage-2 resolution), then the
	// GITID_FAKE_SSH_MODE-dispatched connection test.
	//
	// -G detection scans ALL positional args, not just $1 (03-06 Task-1
	// fix): internal/tester.ResolvedVia's REAL stage-2 invocation is
	// `ssh -F <configPath> -G <alias>` — -G is $3, never $1 — so the
	// original `case "$1" in -G) ...` branch could never match it. That
	// left this fixture's -G reply effectively dead code for stage 2's
	// actual call shape; only `tester.Resolved`'s OTHER, -F-less `ssh -G
	// <alias>` shape (unused by the create-flow wizard) ever hit it. Found
	// via the first PTY e2e to drive the real two-stage test through this
	// fixture end to end (plan 03-06) — exactly the injected-seam-style
	// blindspot DLV-06 exists to catch, this time in the test harness
	// itself rather than production code.
	const script = "#!/bin/sh\n" +
		"if [ \"$1\" = \"-Q\" ]; then\n" +
		"  echo \"ssh-ed25519\"\n" +
		"  echo \"ssh-rsa\"\n" +
		"  echo \"ecdsa-sha2-nistp256\"\n" +
		"  exit 0\n" +
		"fi\n" +
		"config_path=\"\"\n" +
		"next_is_config=0\n" +
		"is_resolution=0\n" +
		"for arg in \"$@\"; do\n" +
		"  if [ \"$next_is_config\" = \"1\" ]; then\n" +
		"    config_path=\"$arg\"\n" +
		"    next_is_config=0\n" +
		"    continue\n" +
		"  fi\n" +
		"  if [ \"$arg\" = \"-F\" ]; then\n" +
		"    next_is_config=1\n" +
		"    continue\n" +
		"  fi\n" +
		"  if [ \"$arg\" = \"-G\" ]; then\n" +
		"    is_resolution=1\n" +
		"  fi\n" +
		"done\n" +
		"if [ \"$GITID_FAKE_SSH_MODE\" = \"globalssh\" ] || [ \"$GITID_FAKE_SSH_MODE\" = \"globalssh-inconclusive\" ] || [ \"$GITID_FAKE_SSH_MODE\" = \"globalssh-probe-unresolvable\" ]; then\n" +
		"  if [ \"$1\" = \"-V\" ]; then\n" +
		"    echo \"OpenSSH_9.9p2, LibreSSL 3.3.6\" >&2\n" +
		"    exit 0\n" +
		"  fi\n" +
		"  if [ \"$is_resolution\" != \"1\" ]; then\n" +
		"    echo \"fake ssh: unsupported global SSH probe\" >&2\n" +
		"    exit 2\n" +
		"  fi\n" +
		// PROP-01 (plan 09.5-01, Task 3): a new unconditional-failure mode for
		// the WILDCARD-ONLY probe `AllDirectives` -> `effective(deps)` runs
		// (`ssh -G <ProbeHost>` with NO `-F`). The existing
		// `globalssh-inconclusive` mode only fails the ISOLATED shadow-check
		// call `shadow.go` makes with a REAL `-F <config>` -- that call only
		// happens inside the apply-preview flow, never during `activate()`,
		// so it cannot force the top-level `directivesErr` / `optionsErr`
		// fail-open state this mode exists to prove. This mode fails EVERY
		// `-G` resolution unconditionally, failing BOTH the Options AND the
		// Properties sub-tab probes the same way a real `ssh` failure would
		// (they share the same `effective(deps)` call), matching the
		// Options sub-tab's own established `optionsErr` contract.
		"  if [ \"$GITID_FAKE_SSH_MODE\" = \"globalssh-probe-unresolvable\" ]; then\n" +
		"    echo \"fake ssh: global SSH probe unresolvable\" >&2\n" +
		"    exit 2\n" +
		"  fi\n" +
		"  if [ \"$GITID_FAKE_SSH_MODE\" = \"globalssh-inconclusive\" ] && [ -n \"$config_path\" ] && [ \"$config_path\" != \"/dev/null\" ]; then\n" +
		"    echo \"fake ssh: isolated global SSH probe failed\" >&2\n" +
		"    exit 2\n" +
		"  fi\n" +
		"  if [ -z \"$config_path\" ]; then\n" +
		"    config_path=\"$HOME/.ssh/config\"\n" +
		"  fi\n" +
		"  strict=no\n" +
		"  forward=no\n" +
		"  hash=no\n" +
		"  identities=no\n" +
		"  addkeys=no\n" +
		"  usekeychain=yes\n" +
		"  strict_set=0\n" +
		"  forward_set=0\n" +
		"  hash_set=0\n" +
		"  identities_set=0\n" +
		"  addkeys_set=0\n" +
		"  usekeychain_set=0\n" +
		// Phase 9.5 plan 09.5-04 (PROP-04), Task 3: the FIXED allow-list of
		// SSH directive names this fixture recognizes — the union of the six
		// curated Options-policy keys (above) and the ~21 extra ones the
		// fixed printf block below already answers, plus two more real
		// OpenSSH directives (serveralivecountmax, tcpkeepalive) reserved for
		// the "accepted" custom-directive test. Anything NOT in this list,
		// scanned inside a `Host *` stanza, makes the compiled binary's
		// rejection path real: a genuine exit-255 + `Bad configuration
		// option:` diagnostic — never a hardcoded acceptance in the TUI
		// itself.
		"  known_directive() {\n" +
		"    lower=$(printf '%s' \"$1\" | tr 'A-Z' 'a-z')\n" +
		"    case \"$lower\" in\n" +
		"      stricthostkeychecking|forwardagent|hashknownhosts|identitiesonly|addkeystoagent|usekeychain) return 0 ;;\n" +
		"      addressfamily|batchmode|canonicalizehostname|checkhostip|ciphers|clearallforwardings) return 0 ;;\n" +
		"      compression|connectionattempts|connecttimeout|controlmaster|dynamicforward|escapechar) return 0 ;;\n" +
		"      exitonforwardfailure|gatewayports|hostbasedauthentication|loglevel|pubkeyauthentication) return 0 ;;\n" +
		"      serveraliveinterval|streamlocalbindmask|userknownhostsfile|visualhostkey) return 0 ;;\n" +
		"      serveralivecountmax|tcpkeepalive) return 0 ;;\n" +
		// ignoreunknown is genuinely present in EVERY gitid-generated global-ssh
		// managed block (globalssh.go's managedHostStar: "IgnoreUnknown
		// UseKeychain") — it must be allow-listed or every staged-config proof
		// against an already-populated block would spuriously report a
		// PreexistingError on gitid's OWN generated line.
		"      ignoreunknown) return 0 ;;\n" +
		"      *) return 1 ;;\n" +
		"    esac\n" +
		"  }\n" +
		"  bad_directive=\"\"\n" +
		"  bad_line=0\n" +
		"  bad_path=\"\"\n" +
		"  scan_global_ssh_config() {\n" +
		"    scan_path=\"$1\"\n" +
		"    [ -r \"$scan_path\" ] || return\n" +
		"    scan_host=1\n" +
		"    scan_lineno=0\n" +
		"    while read -r scan_key scan_value scan_rest; do\n" +
		"      scan_lineno=$((scan_lineno + 1))\n" +
		"      [ -n \"$scan_key\" ] || continue\n" +
		"      case \"$scan_key\" in\n" +
		// Rule 1 bug fix (Phase 9.5 plan 09.5-04, PROP-04, discovered via
		// TestGlobalSSH_RealPTYCustomDirectiveWrite): an UNESCAPED `#*)`
		// case-pattern is itself a shell COMMENT starting at that `#` — the
		// shell tokenizer treats `#` as a comment lead-in whenever it opens
		// an unquoted word, including a case pattern, so `#*) continue ;;`
		// silently consumed the REST of that source line (the pattern AND
		// its `continue` action never existed as parsed code) and every
		// scanned comment line fell through to the default arm instead.
		// This was latent and harmless before this plan (the default arm
		// had no action), but plan 09.5-04's new unknown-directive `*)`
		// catch-all turned it into a false-positive `Bad configuration
		// option: #` rejection the first time a real comment line (e.g. a
		// `# BEGIN gitid managed: ...` sentinel) reached this scan with
		// scan_host=1. `\#*)` escapes the leading `#` so it is a quoted
		// case pattern character, not a comment lead-in.
		"        \\#*) continue ;;\n" +
		"        Host|host) [ \"$scan_value\" = \"*\" ] && scan_host=1 || scan_host=0 ;;\n" +
		"        Include|include) inc=\"$scan_value\"; inc=\"${inc#\\\"}\"; inc=\"${inc%\\\"}\"; for include_path in $inc; do scan_global_ssh_config \"$include_path\"; done ;;\n" +
		"        StrictHostKeyChecking|stricthostkeychecking) if [ \"$scan_host\" = \"1\" ] && [ \"$strict_set\" = \"0\" ]; then strict=\"$scan_value\"; strict_set=1; fi ;;\n" +
		"        ForwardAgent|forwardagent) if [ \"$scan_host\" = \"1\" ] && [ \"$forward_set\" = \"0\" ]; then forward=\"$scan_value\"; forward_set=1; fi ;;\n" +
		"        HashKnownHosts|hashknownhosts) if [ \"$scan_host\" = \"1\" ] && [ \"$hash_set\" = \"0\" ]; then hash=\"$scan_value\"; hash_set=1; fi ;;\n" +
		"        IdentitiesOnly|identitiesonly) if [ \"$scan_host\" = \"1\" ] && [ \"$identities_set\" = \"0\" ]; then identities=\"$scan_value\"; identities_set=1; fi ;;\n" +
		"        AddKeysToAgent|addkeystoagent) if [ \"$scan_host\" = \"1\" ] && [ \"$addkeys_set\" = \"0\" ]; then addkeys=\"$scan_value\"; addkeys_set=1; fi ;;\n" +
		"        UseKeychain|usekeychain) if [ \"$scan_host\" = \"1\" ] && [ \"$usekeychain_set\" = \"0\" ]; then usekeychain=\"$scan_value\"; usekeychain_set=1; fi ;;\n" +
		"        *) if [ \"$scan_host\" = \"1\" ] && [ -z \"$bad_directive\" ] && ! known_directive \"$scan_key\"; then bad_directive=$(printf '%s' \"$scan_key\" | tr 'A-Z' 'a-z'); bad_line=\"$scan_lineno\"; bad_path=\"$scan_path\"; fi ;;\n" +
		"      esac\n" +
		"    done < \"$scan_path\"\n" +
		"  }\n" +
		"  scan_global_ssh_config \"$config_path\"\n" +
		"  if [ -n \"$bad_directive\" ]; then\n" +
		"    echo \"$bad_path: line $bad_line: Bad configuration option: $bad_directive\" >&2\n" +
		"    echo \"$bad_path: terminating, 1 bad configuration options\" >&2\n" +
		"    exit 255\n" +
		"  fi\n" +
		"  printf 'stricthostkeychecking %s\\nforwardagent %s\\nhashknownhosts %s\\nidentitiesonly %s\\naddkeystoagent %s\\nusekeychain %s\\n' \"$strict\" \"$forward\" \"$hash\" \"$identities\" \"$addkeys\" \"$usekeychain\"\n" +
		// PROP-01 (plan 09.5-01): a "full directive set" test against this
		// fixture would be vacuous with only the six curated policy lines
		// above, so this fixed block of additional lowercase `<key>
		// <value>` lines (drawn from real `ssh -G` output) makes the
		// fixture answer the question PROP-01 actually asks — genuinely
		// MORE than the six-row policy set. These are FIXED (never derived
		// from config_path) — the six policy lines above remain the only
		// config-scanning-derived output, byte-identical, so the Options
		// sub-tab's own tests keep reading them unchanged.
		"  printf 'addressfamily any\\nbatchmode no\\ncanonicalizehostname false\\ncheckhostip no\\nciphers chacha20-poly1305@openssh.com,aes128-ctr\\nclearallforwardings no\\ncompression no\\nconnectionattempts 1\\nconnecttimeout none\\ncontrolmaster false\\ndynamicforward none\\nescapechar ~\\nexitonforwardfailure no\\ngatewayports no\\nhostbasedauthentication no\\nloglevel INFO\\npubkeyauthentication yes\\nserveraliveinterval 0\\nstreamlocalbindmask 0177\\nuserknownhostsfile ~/.ssh/known_hosts ~/.ssh/known_hosts2\\nvisualhostkey no\\n'\n" +
		"  exit 0\n" +
		"fi\n" +
		"if [ \"$is_resolution\" = \"1\" ]; then\n" +
		"  if [ -z \"$config_path\" ] || [ ! -r \"$config_path\" ]; then\n" +
		"    echo \"fake ssh: ssh -G requires a readable staged -F config\" >&2\n" +
		"    exit 2\n" +
		"  fi\n" +
		"  user=$(awk '$1 == \"User\" { print $2; exit }' \"$config_path\")\n" +
		"  hostname=$(awk '$1 == \"Hostname\" { print $2; exit }' \"$config_path\")\n" +
		"  port=$(awk '$1 == \"Port\" { print $2; exit }' \"$config_path\")\n" +
		"  identitiesonly=$(awk '$1 == \"IdentitiesOnly\" { print $2; exit }' \"$config_path\")\n" +
		"  identityfile=$(awk '$1 == \"IdentityFile\" { print $2; exit }' \"$config_path\")\n" +
		"  if [ -z \"$user\" ] || [ -z \"$hostname\" ] || [ -z \"$port\" ] || [ -z \"$identitiesonly\" ] || [ -z \"$identityfile\" ]; then\n" +
		"    echo \"fake ssh: staged config is missing required Host fields\" >&2\n" +
		"    exit 2\n" +
		"  fi\n" +
		"  printf 'user %s\\nhostname %s\\nport %s\\nidentitiesonly %s\\nidentityfile %s\\ngitidrawmarker proof-retained-verbatim\\n' \"$user\" \"$hostname\" \"$port\" \"$identitiesonly\" \"$identityfile\"\n" +
		"  exit 0\n" +
		"fi\n" +
		"case \"$GITID_FAKE_SSH_MODE\" in\n" +
		"  pass)\n" +
		"    echo \"Hi user! You've successfully authenticated, but GitHub does not provide shell access.\"\n" +
		"    exit 1\n" +
		"    ;;\n" +
		"  denied)\n" +
		"    echo \"git@ssh.github.com: Permission denied (publickey).\"\n" +
		"    exit 255\n" +
		"    ;;\n" +
		"  timeout)\n" +
		"    echo \"ssh: connect to host ssh.github.com port 443: Operation timed out\"\n" +
		"    exit 255\n" +
		"    ;;\n" +
		"  *)\n" +
		"    echo \"git@ssh.github.com: Permission denied (publickey).\"\n" +
		"    exit 255\n" +
		"    ;;\n" +
		"esac\n"

	scriptPath := filepath.Join(dir, "ssh")
	if err := os.WriteFile(scriptPath, []byte(script), 0o700); err != nil { //nolint:gosec // test-only static script (G306)
		t.Fatalf("FakeSSHDir: writing fake ssh: %v", err)
	}
	t.Setenv("GITID_FAKE_SSH_MODE", mode)
	return dir
}

func TestFakeSSHDirResolvesIdentityFileFromStagedConfig(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config")
	wantIdentityFile := filepath.Join(t.TempDir(), "id_ed25519_staged")
	config := "Host acme.github.com\n\tHostname ssh.github.com\n\tPort 443\n\tUser git\n\tIdentityFile " + wantIdentityFile + "\n\tIdentitiesOnly yes\n"
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatalf("writing staged config: %v", err)
	}

	ssh := filepath.Join(FakeSSHDir(t, "pass"), "ssh")
	out, err := exec.Command(ssh, "-F", configPath, "-G", "acme.github.com").CombinedOutput() //nolint:gosec // ssh is the test-owned FakeSSHDir script
	if err != nil {
		t.Fatalf("running fake ssh -G: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "identityfile "+wantIdentityFile) {
		t.Fatalf("fake ssh -G identityfile = %q, want staged config path %q", out, wantIdentityFile)
	}
}

func TestFakeSSHDirGlobalSSHProbesReadTheirConfig(t *testing.T) {
	home := SandboxHome(t)
	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatalf("creating .ssh: %v", err)
	}
	writeFileT(t, filepath.Join(sshDir, "config"), "Host *\n  StrictHostKeyChecking ask\n")

	ssh := filepath.Join(FakeSSHDir(t, "globalssh"), "ssh")
	plain, err := exec.Command(ssh, "-G", "gitid-probe.invalid").CombinedOutput() //nolint:gosec // ssh is the test-owned FakeSSHDir script
	if err != nil {
		t.Fatalf("running plain global SSH probe: %v\n%s", err, plain)
	}
	if !strings.Contains(string(plain), "stricthostkeychecking ask") {
		t.Fatalf("plain global SSH probe = %q, want HOME config value", plain)
	}

	first := filepath.Join(t.TempDir(), "first")
	second := filepath.Join(t.TempDir(), "second")
	writeFileT(t, first, "Host *\n  StrictHostKeyChecking accept-new\n")
	writeFileT(t, second, "Host *\n  StrictHostKeyChecking no\n")
	firstOut, err := exec.Command(ssh, "-G", "-F", first, "gitid-probe.invalid").CombinedOutput() //nolint:gosec // ssh is the test-owned FakeSSHDir script
	if err != nil {
		t.Fatalf("running first isolated global SSH probe: %v\n%s", err, firstOut)
	}
	secondOut, err := exec.Command(ssh, "-G", "-F", second, "gitid-probe.invalid").CombinedOutput() //nolint:gosec // ssh is the test-owned FakeSSHDir script
	if err != nil {
		t.Fatalf("running second isolated global SSH probe: %v\n%s", err, secondOut)
	}
	if !strings.Contains(string(firstOut), "stricthostkeychecking accept-new") || !strings.Contains(string(secondOut), "stricthostkeychecking no") {
		t.Fatalf("isolated global SSH probes must read their supplied config paths:\nfirst=%q\nsecond=%q", firstOut, secondOut)
	}

	version, err := exec.Command(ssh, "-V").CombinedOutput() //nolint:gosec // ssh is the test-owned FakeSSHDir script
	if err != nil {
		t.Fatalf("running global SSH version probe: %v\n%s", err, version)
	}
	if !strings.Contains(string(version), "OpenSSH_9.9p2") {
		t.Fatalf("global SSH version probe = %q, want OpenSSH version", version)
	}
}

// ---------------------------------------------------------------------------
// The hermetic provider boundary (review R1, 09-02-PLAN.md Task 2 Part A).
//
// Wiring Phase 9's autonomous upload into the create flow means every e2e
// child capable of running a gitid binary is now capable of a REAL remote
// account mutation if it can resolve a real, authenticated `gh`/`glab` on
// PATH. Before this task, `newRealCreateFlowCmd` (and ~15 other
// environment-construction sites across this package) built PATH as
// "<caller's fake dirs>:<ambient PATH>" — the ambient PATH, and therefore
// any real gh/glab on it, stayed resolvable. e2eEnv is the ONE constructor
// every e2e child environment must now come from: it always inserts
// ProviderDenyDir ahead of the ambient PATH, so exec.LookPath("gh")/("glab")
// inside any e2e child resolves to a test-owned, fail-closed script — never
// the developer's real tool — regardless of what the caller supplies.
// ---------------------------------------------------------------------------

// e2eT is the minimal *testing.T surface ProviderDenyDir/e2eEnv need. It
// exists so TestE2EEnvRejectsAmbientPathAsPrefix can substitute a
// recordingT that captures a Fatalf call instead of aborting the test via
// runtime.Goexit, letting that guard test assert e2eEnv actually rejects an
// unsafe pathPrefix rather than merely trusting it does. *testing.T
// satisfies this interface, so every production call site is unaffected.
type e2eT interface {
	Helper()
	TempDir() string
	Fatalf(format string, args ...any)
}

// providerDenyScript is the shared body for both the gh and glab deny
// shims: log the attempt (when GITID_E2E_DENY_LOG is set), print a
// distinctive block marker naming itself, and exit non-zero. Never
// contacts a network; never reads a real provider configuration directory.
// A static string literal — never constructed from user input (G204-clean),
// mirroring every other shim in this file.
const providerDenyScript = "#!/bin/sh\n" +
	"if [ -n \"$GITID_E2E_DENY_LOG\" ]; then\n" +
	"  printf '%s %s\\n' \"$0\" \"$*\" >> \"$GITID_E2E_DENY_LOG\"\n" +
	"fi\n" +
	"echo \"gitid-e2e-hermetic-boundary: refusing $(basename \"$0\") — this is the test-owned deny shim, not a real provider CLI\" >&2\n" +
	"exit 17\n"

// ProviderDenyDir writes fail-closed `gh` and `glab` scripts into a fresh
// t.TempDir() and returns that directory plus the path of the log file
// every invocation is recorded to. Both scripts are the SAME static literal
// body (providerDenyScript) with a different filename — they never contact
// a network and never read a real provider configuration directory.
func ProviderDenyDir(t e2eT) (dir string, denyLog string) {
	t.Helper()
	dir = t.TempDir()
	denyLog = filepath.Join(t.TempDir(), "deny.log")
	for _, name := range []string{"gh", "glab"} {
		scriptPath := filepath.Join(dir, name)
		if err := os.WriteFile(scriptPath, []byte(providerDenyScript), 0o700); err != nil { //nolint:gosec // test-only static script (G306)
			t.Fatalf("ProviderDenyDir: writing deny shim %s: %v", name, err)
		}
	}
	return dir, denyLog
}

// ReadProviderDenyLog returns one entry per deny-shim invocation, in order,
// each entry the space-joined argv the shim recorded (its own path first).
// A missing log file (nothing was ever denied) returns an empty slice, not
// an error — the common, expected case for a hermetic test run.
func ReadProviderDenyLog(t *testing.T, denyLog string) []string {
	t.Helper()
	data, err := os.ReadFile(denyLog) //nolint:gosec // test-owned log path under t.TempDir() (G304)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("ReadProviderDenyLog: %v", err)
	}
	trimmed := strings.TrimRight(string(data), "\n")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\n")
}

// e2eAllowedAmbientPathSites documents the ONLY functions permitted to build
// a command environment outside e2eEnv — each entry paired with the reason
// TestEveryE2EChildEnvIsHermetic accepts it. Both listed functions invoke a
// build toolchain (`go build`/`make install`), never a gitid binary, so
// neither can reach a real gh/glab through the product's own resolution
// path. Read by the source-level guard test below; keep in sync with any
// change to BuildBinary/BuildDummyBinary/TestInstall_MakeInstallOutput.
var e2eAllowedAmbientPathSites = map[string]string{
	"BuildBinary":                                "invokes `go build`, never a gitid binary — needs the real toolchain PATH and the real HOME to resolve GOPATH",
	"BuildDummyBinary":                           "invokes `go build`, never a gitid binary — same reason as BuildBinary",
	"TestInstall_MakeInstallOutput":              "invokes `make install`, never a gitid binary — asserts the Makefile's own echoed install-path/PATH-hint text",
	"stampedArtifacts":                           "invokes `make release-snapshot`, never a gitid binary — needs the real toolchain PATH and the real HOME to resolve GOPATH",
	"TestRelease_UnstampedBuildKeepsDevDefaults": "invokes `make build`, never a gitid binary — same reason as BuildBinary",
	"runScratchGoreleaserRelease":                "invokes the goreleaser binary directly (D-18 homebrew-gate proof), never a gitid binary — needs the real toolchain PATH and the real HOME so `git tag -a` can resolve a real user.name/user.email identity; e2eEnv's sandboxed HOME would have none, and the real ambient environment cannot smuggle a gh/glab resolution into any gitid code path since no gitid binary is ever invoked here",
}

// e2eAmbientPathSubstrings are real-PATH indicators e2eEnv refuses to see
// among its caller-supplied pathPrefixes — the check that keeps a caller
// from smuggling the real, ambient PATH back in front of the deny shim by
// passing it (or an entry from it) as a "prefix". Kept short and portable:
// these are directories a real gh/glab install commonly lives in on the
// developer's machine, PLUS the literal ambient PATH value itself.
func ambientPathSentinelHit(prefix string) (hit string, found bool) {
	ambient := os.Getenv("PATH")
	if ambient != "" && prefix == ambient {
		return ambient, true
	}
	for _, entry := range filepath.SplitList(ambient) {
		if entry == "" {
			continue
		}
		for _, part := range filepath.SplitList(prefix) {
			if part == entry {
				return entry, true
			}
		}
	}
	return "", false
}

// e2eEnv is the ONE constructor every e2e child-process environment must
// come from (review R1). It builds the child environment from the ambient
// environment plus HOME=home, and sets PATH to the concatenation, in this
// order: the caller's pathPrefixes (fake ssh, fake gh, fake glab, fake git,
// whatever the test needs), then ProviderDenyDir's directory, then the
// ambient PATH. It also exports GITID_E2E_DENY_LOG so the deny shims record
// any attempt.
//
// The guarantee this ordering buys: the ONLY gh/glab any e2e child can
// resolve is a test-owned script under t.TempDir() — either the caller's
// deliberate fake (if pathPrefixes supplies one ahead of the deny dir), or
// the deny shim. A real, authenticated gh on the developer's machine
// becomes unreachable to exec.LookPath inside the child, while ssh,
// ssh-keygen, ssh-add, and git keep resolving exactly as they do today
// (os/exec keeps the LAST value for a duplicated env key, so appending the
// composed PATH after the rest of the ambient environment is sufficient —
// the same mechanism create_flow_pty_e2e_test.go's TERM= comment already
// documents).
//
// The quieter second benefit: every pre-existing PTY test now observes a
// DETERMINISTIC provider-eligibility state (the deny shim answers "tool
// present, not authenticated") instead of a state that depended on whether
// the developer happened to have gh installed and logged in.
//
// e2eEnv validates pathPrefixes: any prefix equal to the ambient PATH
// itself, or containing one of the ambient PATH's own real entries, fails
// the test loudly via t.Fatalf rather than silently composing a PATH that
// defeats the deny shim.
func e2eEnv(t e2eT, home string, pathPrefixes ...string) (env []string, denyLog string) {
	t.Helper()
	for _, prefix := range pathPrefixes {
		if prefix == "" {
			continue
		}
		if hit, found := ambientPathSentinelHit(prefix); found {
			t.Fatalf("e2eEnv: pathPrefix %q contains the ambient PATH entry %q — this would smuggle a real gh/glab back in front of the deny shim; pass only test-owned shim directories", prefix, hit)
			return nil, ""
		}
	}

	denyDir, denyLog := ProviderDenyDir(t)

	pathParts := make([]string, 0, len(pathPrefixes)+2)
	for _, prefix := range pathPrefixes {
		if prefix != "" {
			pathParts = append(pathParts, prefix)
		}
	}
	pathParts = append(pathParts, denyDir, os.Getenv("PATH"))

	env = append(os.Environ(),
		"HOME="+home,
		"PATH="+strings.Join(pathParts, string(os.PathListSeparator)),
		"GITID_E2E_DENY_LOG="+denyLog,
	)
	return env, denyLog
}

// envValue extracts the LAST value of key from an os/exec-shaped
// KEY=VALUE environment slice — mirroring os/exec's own "last duplicate
// wins" resolution, so a test inspecting an e2eEnv-built slice sees exactly
// what the child process would.
func envValue(env []string, key string) (string, bool) {
	prefix := key + "="
	value, found := "", false
	for _, kv := range env {
		if strings.HasPrefix(kv, prefix) {
			value = strings.TrimPrefix(kv, prefix)
			found = true
		}
	}
	return value, found
}

// lookPathIn resolves name under pathEnv (a colon/semicolon-joined PATH
// value, PathListSeparator-split) WITHOUT touching the current process's
// own PATH — so a test can ask "what would this child process resolve"
// without a global, restore-requiring t.Setenv("PATH", ...) side effect.
func lookPathIn(pathEnv, name string) (string, bool) {
	for _, dir := range filepath.SplitList(pathEnv) {
		if dir == "" {
			continue
		}
		candidate := filepath.Join(dir, name)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() && info.Mode()&0o111 != 0 {
			return candidate, true
		}
	}
	return "", false
}

// isUnderTempDir reports whether p is inside a Go-test-owned temp
// directory (os.TempDir(), which t.TempDir() and os.MkdirTemp("", ...)
// both create under) — the check TestE2EEnvHidesRealProviderCLIs uses to
// prove a resolved gh/glab is test-owned rather than a real system/user
// install.
func isUnderTempDir(p string) bool {
	tmp := os.TempDir()
	resolvedTmp, err := filepath.EvalSymlinks(tmp)
	if err != nil {
		resolvedTmp = tmp
	}
	resolvedP, err := filepath.EvalSymlinks(filepath.Dir(p))
	if err != nil {
		resolvedP = filepath.Dir(p)
	}
	rel, err := filepath.Rel(resolvedTmp, resolvedP)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// TestE2EEnvRejectsAmbientPathAsPrefix proves e2eEnv fails loudly (not
// silently) when a caller passes the ambient PATH, or a PATH-joined string
// containing one of the ambient PATH's own real entries, as a pathPrefix —
// the control that keeps a caller from smuggling the real PATH back in
// front of the deny shim.
func TestE2EEnvRejectsAmbientPathAsPrefix(t *testing.T) {
	ambient := os.Getenv("PATH")
	if ambient == "" {
		t.Skip("ambient PATH is empty in this environment; nothing to smuggle")
	}
	entries := filepath.SplitList(ambient)
	if len(entries) == 0 {
		t.Skip("ambient PATH has no entries")
	}

	rt := &recordingT{T: t}
	e2eEnv(rt, t.TempDir(), ambient)
	if !rt.fatalCalled {
		t.Fatal("e2eEnv must t.Fatalf when a pathPrefix equals the ambient PATH verbatim")
	}

	rt2 := &recordingT{T: t}
	e2eEnv(rt2, t.TempDir(), entries[0]+string(os.PathListSeparator)+"/some/fake/dir")
	if !rt2.fatalCalled {
		t.Fatal("e2eEnv must t.Fatalf when a pathPrefix CONTAINS an ambient PATH entry")
	}

	// A genuinely test-owned prefix must NOT trip the guard.
	rt3 := &recordingT{T: t}
	safeDir, _ := ProviderDenyDir(t)
	e2eEnv(rt3, t.TempDir(), safeDir)
	if rt3.fatalCalled {
		t.Fatal("e2eEnv must not reject a genuinely test-owned pathPrefix")
	}
}

// recordingT wraps *testing.T so a test can assert a HELPER called
// t.Fatalf without the outer test itself failing — Fatalf normally calls
// runtime.Goexit, so this override records the call and returns instead
// (only safe because e2eEnv never does meaningful work after the Fatalf
// call site it's guarding here; used ONLY by
// TestE2EEnvRejectsAmbientPathAsPrefix, never by production code).
type recordingT struct {
	*testing.T
	fatalCalled bool
}

func (r *recordingT) Fatalf(format string, args ...any) {
	r.fatalCalled = true
	r.Logf("(expected) e2eEnv rejected an unsafe pathPrefix: "+format, args...)
}

// TestE2EEnvHidesRealProviderCLIs is the R1 runtime proof: under an
// e2eEnv-built PATH, both gh and glab resolve inside a test-owned temporary
// directory, executing either yields a non-zero exit plus the block marker,
// and the deny log records exactly one line per invocation. It also proves
// the test itself is meaningful (not vacuously true) by resolving gh under
// the AMBIENT PATH too: if a real one exists there, the two resolutions
// must differ; if none exists, that half is explicitly skipped rather than
// silently passing.
func TestE2EEnvHidesRealProviderCLIs(t *testing.T) {
	env, denyLog := e2eEnv(t, t.TempDir())
	pathEnv, ok := envValue(env, "PATH")
	if !ok {
		t.Fatal("e2eEnv-built env carries no PATH entry")
	}

	for _, name := range []string{"gh", "glab"} {
		resolved, found := lookPathIn(pathEnv, name)
		if !found {
			t.Fatalf("%s did not resolve under the e2eEnv-built PATH at all — the deny shim must always be present", name)
		}
		if !isUnderTempDir(resolved) {
			t.Errorf("%s resolved to %q, want a path inside a test-owned temp directory (%s)", name, resolved, os.TempDir())
		}

		cmd := exec.Command(resolved, "auth", "status") //nolint:gosec // resolved is the test-owned deny shim (G204)
		cmd.Env = env
		out, err := cmd.CombinedOutput()
		if err == nil {
			t.Errorf("%s (deny shim) must exit non-zero, got success; output: %s", name, out)
		}
		if !strings.Contains(string(out), "gitid-e2e-hermetic-boundary") {
			t.Errorf("%s (deny shim) output missing the block marker: %s", name, out)
		}

		if ambientResolved, ambientFound := exec.LookPath(name); ambientFound == nil {
			if ambientResolved == resolved {
				t.Errorf("%s resolved IDENTICALLY under e2eEnv and the ambient PATH — the deny shim is not actually shadowing anything", name)
			}
		} else {
			t.Logf("%s: no real install on the ambient PATH — skipping the differs-from-ambient half of this proof for %s", name, name)
		}
	}

	log := ReadProviderDenyLog(t, denyLog)
	if len(log) != 2 {
		t.Fatalf("deny log recorded %d invocations, want 2 (one gh, one glab):\n%v", len(log), log)
	}
}

// fileHasE2EBuildTag reports whether file's build constraints select the
// "e2e" tag — read from the file's own //go:build (or legacy // +build)
// line(s) via go/build/constraint, never assumed from a hard-coded
// filename list. A file whose constraint does NOT select "e2e" (a future
// opt-in surface such as plan 09-08's real-account test, gated behind its
// own different tag) is out of scope for TestEveryE2EChildEnvIsHermetic by
// construction.
func fileHasE2EBuildTag(t *testing.T, path string, src []byte) bool {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, src, parser.ParseComments|parser.PackageClauseOnly)
	if err != nil {
		t.Fatalf("fileHasE2EBuildTag: parsing %s: %v", path, err)
	}
	for _, cg := range file.Comments {
		for _, c := range cg.List {
			if !constraint.IsGoBuild(c.Text) && !constraint.IsPlusBuild(c.Text) {
				continue
			}
			expr, perr := constraint.Parse(c.Text)
			if perr != nil {
				continue
			}
			if expr.Eval(func(tag string) bool { return tag == "e2e" }) {
				return true
			}
		}
	}
	return false
}

// TestEveryE2EChildEnvIsHermetic is the R1 source-level guard: a scan over
// every "e2e"-tagged .go file in this package finds every assignment to a
// command's environment field (`*.Env = ...`) and asserts each one is
// produced by e2eEnv, failing with the offending file and line number for
// any that is not — unless its enclosing function is in
// e2eAllowedAmbientPathSites.
//
// Implemented via go/parser + go/ast (walking *ast.AssignStmt targets whose
// LHS is a SelectorExpr named "Env"), NOT a grep/string-presence check: a
// naive text search for "e2eEnv" would miss an existing
// `cmd.Env = os.Environ()` assignment that happens to also appear near the
// string "e2eEnv" in a comment, and would miss a `cmd.Env = append(...)`
// construction entirely. AST-level detection is what makes this guard
// trustworthy rather than decorative.
func TestEveryE2EChildEnvIsHermetic(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("reading e2e/: %v", err)
	}

	checkedAny := false
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") {
			continue
		}
		src, rerr := os.ReadFile(name) //nolint:gosec // package-local .go files from os.ReadDir (G304)
		if rerr != nil {
			t.Fatalf("reading %s: %v", name, rerr)
		}
		if !fileHasE2EBuildTag(t, name, src) {
			continue
		}
		checkedAny = true

		fset := token.NewFileSet()
		file, perr := parser.ParseFile(fset, name, src, 0)
		if perr != nil {
			t.Fatalf("parsing %s: %v", name, perr)
		}

		// Pass 1: which top-level functions call e2eEnv anywhere in their
		// own body — the marker every legitimate `.Env = ...` assignment's
		// enclosing function must carry (directly, e.g.
		// `env, _ := e2eEnv(t, home, shim)` followed by `cmd.Env = env`, or
		// transitively through a same-file helper that itself calls
		// e2eEnv, since that helper is scanned as its own function too).
		callsE2EEnv := map[string]bool{}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				if ident, ok := call.Fun.(*ast.Ident); ok && ident.Name == "e2eEnv" {
					callsE2EEnv[fn.Name.Name] = true
				}
				return true
			})
		}

		var currentFunc string
		ast.Inspect(file, func(n ast.Node) bool {
			if fn, ok := n.(*ast.FuncDecl); ok {
				currentFunc = fn.Name.Name
			}
			assign, ok := n.(*ast.AssignStmt)
			if !ok {
				return true
			}
			for _, lhs := range assign.Lhs {
				sel, ok := lhs.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "Env" {
					continue
				}
				if _, allowed := e2eAllowedAmbientPathSites[currentFunc]; allowed {
					continue
				}
				if callsE2EEnv[currentFunc] {
					continue
				}
				pos := fset.Position(assign.Pos())
				t.Errorf("%s:%d: %s assigns .Env without routing through e2eEnv — every e2e child environment must come from e2eEnv (review R1)", pos.Filename, pos.Line, currentFunc)
			}
			return true
		})
	}
	if !checkedAny {
		t.Fatal("TestEveryE2EChildEnvIsHermetic found no e2e-tagged .go file to scan — the guard would be vacuous")
	}
}

// ---------------------------------------------------------------------------
// The full Phase 9 provider-CLI shim mode set (09-02-PLAN.md Task 2 Part B).
//
// Both scripts stay static string literals — never interpolated from a test
// value (the existing G204-clean discipline every shim in this file
// follows). All variation goes through the GITID_FAKE_GH_MODE /
// GITID_FAKE_GLAB_MODE env switch plus GITID_FAKE_GH_LOG / GITID_FAKE_GLAB_LOG
// (the argv-recording log path) and GITID_FAKE_GH_INVENTORY_FILE /
// GITID_FAKE_GLAB_INVENTORY_FILE (a sibling data file the script `cat`s for
// the inventory modes — keeping the script itself a fixed literal while
// still letting a test point the inventory at a real .pub it generated).
// ---------------------------------------------------------------------------

// FakeGHDir writes the full-mode-set fake gh script and sets
// GITID_FAKE_GH_MODE. The caller prepends the returned dir to PATH via
// e2eEnv's pathPrefixes. Returns the shim directory and the log file path
// every invocation's argv is recorded to (ReadFakeCLILog reads it back).
//
// Dispatches on $1 (auth|ssh-key|api) and, for ssh-key, on $2 (add|list|
// delete). FIRST action on every invocation: log the argv (when
// GITID_FAKE_GH_LOG is set). LAST resort: an unrecognized verb records its
// argv, prints a distinctive unhandled marker, and exits non-zero (fail
// CLOSED, so a half-wired product call surfaces as a visible test failure
// instead of a silent success).
//
// Modes: ok, auth-fail, scope-fail-signing, scope-fail-all, duplicate,
// inventory-both, inventory-auth-only, inventory-fail, delete-ok — see the
// script's own case statement below for the exact behavior of each.
//
// delete-ok's `api` verb (09-06/09-07's D-04 delete-offer tests) also
// serves FakeGHInventoryFile's fixture, exactly like inventory-both — the
// D-04 offer needs BOTH a matching inventory read (to resolve the old
// key's provider ID via uploader.FindByTitle) AND a successful delete in
// the SAME session, and no other single mode combines both.
//
// Script is a static literal — never constructed from user input (G204-clean).
func FakeGHDir(t *testing.T, mode string) (dir string, logPath string) {
	t.Helper()
	dir = t.TempDir()
	logPath = filepath.Join(t.TempDir(), "gh.log")
	const script = "#!/bin/sh\n" +
		"if [ -n \"$GITID_FAKE_GH_LOG\" ]; then printf '%s\\n' \"$*\" >> \"$GITID_FAKE_GH_LOG\"; fi\n" +
		"case \"$1\" in\n" +
		"  auth)\n" +
		"    case \"$GITID_FAKE_GH_MODE\" in\n" +
		"      ok|scope-fail-signing|scope-fail-all|duplicate|inventory-both|inventory-auth-only|inventory-fail|delete-ok) exit 0 ;;\n" +
		"      *) echo \"error: not logged into github.com\"; exit 1 ;;\n" +
		"    esac\n" +
		"    ;;\n" +
		"  ssh-key)\n" +
		"    case \"$2\" in\n" +
		"      add)\n" +
		"        case \"$GITID_FAKE_GH_MODE\" in\n" +
		"          ok|inventory-both|inventory-auth-only|inventory-fail|delete-ok)\n" +
		// FakeGHTrackAddedKeys (opt-in, env unset by default so every other
		// test's behavior is byte-identical to before): record the REAL
		// pubkey blob this "add" call was given, keyed by --type, so a
		// LATER inventory read (below) can report it as genuinely present —
		// mirroring real GitHub, where an added key becomes visible on the
		// next read. Without this, CR-02's belt-and-braces check in
		// rotateDeleteOfferFor (which reads FRESH inventory and requires
		// the CURRENT key's blob to already be registered) can never be
		// satisfied by a static fixture that never reflects an "add".
		// WR-02 (iteration 3): record the REAL --title ($5 — argv is
		// always ["ssh-key","add",pubPath,"--title",title,"--type",type],
		// see uploader.go's deleteArgs sibling), never a synthetic one. A
		// rotate's new key is registered under the SAME D-07 title as the
		// old key — that collision is the entire premise of CR-01's blob
		// exclusion; a fixture that gives the new key a different title
		// would let the title filter (not the exclusion) disambiguate
		// them, silently proving nothing about CR-01's actual logic.
		"            if [ -n \"$GITID_FAKE_GH_KEYS_AUTH_FILE\" ] && echo \"$*\" | grep -q -- '--type authentication'; then\n" +
		"              printf '{\"id\":9001,\"title\":\"%s\",\"key\":\"%s\"}\\n' \"$5\" \"$(cat \"$3\" 2>/dev/null)\" >> \"$GITID_FAKE_GH_KEYS_AUTH_FILE\"\n" +
		"            fi\n" +
		"            if [ -n \"$GITID_FAKE_GH_KEYS_SIGNING_FILE\" ] && echo \"$*\" | grep -q -- '--type signing'; then\n" +
		"              printf '{\"id\":9002,\"title\":\"%s\",\"key\":\"%s\"}\\n' \"$5\" \"$(cat \"$3\" 2>/dev/null)\" >> \"$GITID_FAKE_GH_KEYS_SIGNING_FILE\"\n" +
		"            fi\n" +
		"            echo \"Added SSH key.\"; exit 0 ;;\n" +
		"          scope-fail-signing)\n" +
		"            if echo \"$*\" | grep -q -- '--type authentication'; then\n" +
		"              echo \"Added SSH key.\"; exit 0\n" +
		"            fi\n" +
		"            echo \"error: insufficient scope (missing admin:ssh_signing_key)\" >&2; exit 1 ;;\n" +
		"          scope-fail-all)\n" +
		"            echo \"error: insufficient scope (missing admin:public_key)\" >&2; exit 1 ;;\n" +
		"          duplicate)\n" +
		"            echo \"! Key already exists on your account\"; exit 0 ;;\n" +
		"          *) echo \"error: not authenticated\" >&2; exit 1 ;;\n" +
		"        esac\n" +
		"        ;;\n" +
		"      delete)\n" +
		"        case \"$GITID_FAKE_GH_MODE\" in\n" +
		"          delete-ok) exit 0 ;;\n" +
		"          *) echo \"error: not authenticated\" >&2; exit 1 ;;\n" +
		"        esac\n" +
		"        ;;\n" +
		"      list)\n" +
		"        exit 0\n" +
		"        ;;\n" +
		"      *)\n" +
		"        echo \"gitid-e2e-unhandled-verb: gh ssh-key $2\" >&2; exit 3 ;;\n" +
		"    esac\n" +
		"    ;;\n" +
		"  api)\n" +
		"    case \"$GITID_FAKE_GH_MODE\" in\n" +
		"      inventory-fail)\n" +
		"        echo \"error: could not read inventory\" >&2; exit 1 ;;\n" +
		"      inventory-both|delete-ok)\n" +
		"        if [ -n \"$GITID_FAKE_GH_INVENTORY_FILE\" ] && [ -r \"$GITID_FAKE_GH_INVENTORY_FILE\" ]; then\n" +
		"          base=$(cat \"$GITID_FAKE_GH_INVENTORY_FILE\")\n" +
		"        else\n" +
		"          base='[]'\n" +
		"        fi\n" +
		// FakeGHTrackAddedKeys' read side: merge whichever added-keys file
		// matches the endpoint being queried into the static base fixture.
		// extra_file stays empty (no-op, byte-identical output to before)
		// unless the test opted in via FakeGHTrackAddedKeys.
		"        case \"$*\" in\n" +
		"          *user/ssh_signing_keys*) extra_file=\"$GITID_FAKE_GH_KEYS_SIGNING_FILE\" ;;\n" +
		"          *) extra_file=\"$GITID_FAKE_GH_KEYS_AUTH_FILE\" ;;\n" +
		"        esac\n" +
		"        extra=\"\"\n" +
		"        if [ -n \"$extra_file\" ] && [ -s \"$extra_file\" ]; then\n" +
		"          extra=$(paste -sd, \"$extra_file\")\n" +
		"        fi\n" +
		"        base_inner=$(printf '%s' \"$base\" | sed -e 's/^\\[//' -e 's/\\]$//')\n" +
		"        if [ -n \"$base_inner\" ] && [ -n \"$extra\" ]; then\n" +
		"          printf '[%s,%s]\\n' \"$base_inner\" \"$extra\"\n" +
		"        elif [ -n \"$extra\" ]; then\n" +
		"          printf '[%s]\\n' \"$extra\"\n" +
		"        else\n" +
		"          printf '%s\\n' \"$base\"\n" +
		"        fi\n" +
		"        exit 0 ;;\n" +
		"      inventory-auth-only)\n" +
		// WR-01: uploader.Inventory now issues `gh api --paginate user/keys`,
		// so the endpoint token is no longer positionally $2 — match it
		// anywhere in "$*" instead (still never matches user/ssh_signing_keys,
		// since that path does not contain the literal substring "user/keys").
		"        case \"$*\" in\n" +
		"          *user/keys*)\n" +
		"            if [ -n \"$GITID_FAKE_GH_INVENTORY_FILE\" ] && [ -r \"$GITID_FAKE_GH_INVENTORY_FILE\" ]; then\n" +
		"              cat \"$GITID_FAKE_GH_INVENTORY_FILE\"\n" +
		"            else\n" +
		"              echo '[]'\n" +
		"            fi\n" +
		"            ;;\n" +
		"          *) echo '[]' ;;\n" +
		"        esac\n" +
		"        exit 0 ;;\n" +
		"      *)\n" +
		"        echo '[]'; exit 0 ;;\n" +
		"    esac\n" +
		"    ;;\n" +
		"  *)\n" +
		"    echo \"gitid-e2e-unhandled-verb: gh $1\" >&2\n" +
		"    exit 3\n" +
		"    ;;\n" +
		"esac\n"
	scriptPath := filepath.Join(dir, "gh")
	if err := os.WriteFile(scriptPath, []byte(script), 0o700); err != nil { //nolint:gosec // test-only static script (G306)
		t.Fatalf("FakeGHDir: writing fake gh: %v", err)
	}
	t.Setenv("GITID_FAKE_GH_MODE", mode)
	t.Setenv("GITID_FAKE_GH_LOG", logPath)
	return dir, logPath
}

// FakeGHInventoryFile points the "inventory-both"/"inventory-auth-only"
// fake-gh modes' `api` verb at a fixed JSON fixture file — the mechanism
// that lets a test drive the inventory response with a REAL .pub-derived
// key blob it generated, while fakeGHScript's own body stays a static
// literal (it only ever `cat`s a path named by an env var).
func FakeGHInventoryFile(t *testing.T, jsonBody string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "gh-inventory.json")
	if err := os.WriteFile(path, []byte(jsonBody), 0o600); err != nil {
		t.Fatalf("FakeGHInventoryFile: %v", err)
	}
	t.Setenv("GITID_FAKE_GH_INVENTORY_FILE", path)
	return path
}

// FakeGHTrackAddedKeys makes FakeGHDir's "ssh-key add" verb (inventory-both/
// delete-ok modes) append each added key's REAL blob to a per-registration-
// type state file, and makes the "api" inventory-read verb merge those
// additions into its response — so a test that drives a REAL key rotation
// through the fake gh shim can observe the NEW key as genuinely present in
// a SUBSEQUENT inventory read, matching real GitHub's behavior (an added
// key becomes visible on the next list/read). Without this, CR-02's
// belt-and-braces check in cmd/gitid's rotateDeleteOfferFor — which reads
// FRESH inventory and refuses the D-04 delete offer unless the CURRENT
// key's blob is already registered for every desired registration — can
// never be satisfied by a static inventory fixture that never reflects an
// "add", by design (the check exists specifically to catch a registration
// that never actually completed; a stateless fixture looks identical to
// that failure mode unless it is made stateful here instead).
//
// Opt-in and env-unset by default: a test that never calls this sees
// byte-identical fake-gh behavior to before this helper existed.
func FakeGHTrackAddedKeys(t *testing.T) {
	t.Helper()
	authFile := filepath.Join(t.TempDir(), "gh-added-auth.ndjson")
	signingFile := filepath.Join(t.TempDir(), "gh-added-signing.ndjson")
	t.Setenv("GITID_FAKE_GH_KEYS_AUTH_FILE", authFile)
	t.Setenv("GITID_FAKE_GH_KEYS_SIGNING_FILE", signingFile)
}

// FakeGLabDir writes the full-mode-set fake glab script and sets
// GITID_FAKE_GLAB_MODE. The caller prepends the returned dir to PATH via
// e2eEnv's pathPrefixes. Returns the shim directory and the log file path
// every invocation's argv is recorded to (ReadFakeCLILog reads it back).
//
// Mirrors FakeGHDir's structure and fail-closed unknown-verb branch.
//
// Modes: ok, auth-fail, taken, inventory-present, inventory-empty,
// inventory-fail, delete-ok — see the script's own case statement below for
// the exact behavior of each.
//
// Script is a static literal — never constructed from user input (G204-clean).
func FakeGLabDir(t *testing.T, mode string) (dir string, logPath string) {
	t.Helper()
	dir = t.TempDir()
	logPath = filepath.Join(t.TempDir(), "glab.log")
	const script = "#!/bin/sh\n" +
		"if [ -n \"$GITID_FAKE_GLAB_LOG\" ]; then printf '%s\\n' \"$*\" >> \"$GITID_FAKE_GLAB_LOG\"; fi\n" +
		"case \"$1\" in\n" +
		"  auth)\n" +
		"    case \"$GITID_FAKE_GLAB_MODE\" in\n" +
		"      ok|taken|inventory-present|inventory-empty|inventory-fail|delete-ok) exit 0 ;;\n" +
		"      *) echo \"error: not authenticated to gitlab.com\"; exit 1 ;;\n" +
		"    esac\n" +
		"    ;;\n" +
		"  ssh-key)\n" +
		"    case \"$2\" in\n" +
		"      add)\n" +
		"        case \"$GITID_FAKE_GLAB_MODE\" in\n" +
		"          ok|inventory-present|inventory-empty|inventory-fail|delete-ok)\n" +
		"            echo \"Added SSH key.\"; exit 0 ;;\n" +
		"          taken)\n" +
		"            echo \"error: Fingerprint has already been taken\" >&2; exit 1 ;;\n" +
		"          *) echo \"error: not authenticated\" >&2; exit 1 ;;\n" +
		"        esac\n" +
		"        ;;\n" +
		"      delete)\n" +
		"        case \"$GITID_FAKE_GLAB_MODE\" in\n" +
		"          delete-ok) exit 0 ;;\n" +
		"          *) echo \"error: not authenticated\" >&2; exit 1 ;;\n" +
		"        esac\n" +
		"        ;;\n" +
		"      list)\n" +
		"        case \"$GITID_FAKE_GLAB_MODE\" in\n" +
		"          inventory-present)\n" +
		"            if [ -n \"$GITID_FAKE_GLAB_INVENTORY_FILE\" ] && [ -r \"$GITID_FAKE_GLAB_INVENTORY_FILE\" ]; then\n" +
		"              cat \"$GITID_FAKE_GLAB_INVENTORY_FILE\"\n" +
		"            else\n" +
		"              echo '[]'\n" +
		"            fi\n" +
		"            exit 0 ;;\n" +
		"          inventory-fail)\n" +
		"            echo \"error: could not read inventory\" >&2; exit 1 ;;\n" +
		"          *)\n" +
		"            echo '[]'; exit 0 ;;\n" +
		"        esac\n" +
		"        ;;\n" +
		"      *)\n" +
		"        echo \"gitid-e2e-unhandled-verb: glab ssh-key $2\" >&2; exit 3 ;;\n" +
		"    esac\n" +
		"    ;;\n" +
		"  *)\n" +
		"    echo \"gitid-e2e-unhandled-verb: glab $1\" >&2\n" +
		"    exit 3\n" +
		"    ;;\n" +
		"esac\n"
	scriptPath := filepath.Join(dir, "glab")
	if err := os.WriteFile(scriptPath, []byte(script), 0o700); err != nil { //nolint:gosec // test-only static script (G306)
		t.Fatalf("FakeGLabDir: writing fake glab: %v", err)
	}
	t.Setenv("GITID_FAKE_GLAB_MODE", mode)
	t.Setenv("GITID_FAKE_GLAB_LOG", logPath)
	return dir, logPath
}

// FakeGLabInventoryFile is FakeGHInventoryFile's glab-mode sibling, for the
// "inventory-present" fake-glab mode's `ssh-key list -F json` output.
func FakeGLabInventoryFile(t *testing.T, jsonBody string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "glab-inventory.json")
	if err := os.WriteFile(path, []byte(jsonBody), 0o600); err != nil {
		t.Fatalf("FakeGLabInventoryFile: %v", err)
	}
	t.Setenv("GITID_FAKE_GLAB_INVENTORY_FILE", path)
	return path
}

// ReadFakeCLILog returns one entry per shim invocation, in order — each
// entry the space-joined argv (the shim's own $* — its OWN path is never
// included, unlike ReadProviderDenyLog's $0 $* shape, since the fake-gh/glab
// log's consumers assert on the uploader-package argv shape directly).
// A missing log file returns an empty slice, not an error.
func ReadFakeCLILog(t *testing.T, logPath string) []string {
	t.Helper()
	data, err := os.ReadFile(logPath) //nolint:gosec // test-owned log path under t.TempDir() (G304)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("ReadFakeCLILog: %v", err)
	}
	trimmed := strings.TrimRight(string(data), "\n")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\n")
}

// FakeGitDir writes a mode-switching fake git script and sets GITID_FAKE_GIT_MODE.
// The caller prepends the returned dir to PATH via cmd.Env.
//
// Modes:
//
//	clone-ok   — mkdir dest and exits 0 (simulates a successful clone)
//	clone-fail — exits 128 (simulates a failed clone)
//
// Script is a static literal — never constructed from user input (G204-clean).
func FakeGitDir(t *testing.T, mode string) string {
	t.Helper()
	dir := t.TempDir()
	const script = "#!/bin/sh\n" +
		"case \"$GITID_FAKE_GIT_MODE\" in\n" +
		"  clone-ok)\n" +
		"    # mkdir the last argument (destination path) to simulate a successful clone\n" +
		"    dest=\"${@: -1}\"\n" +
		"    mkdir -p \"$dest\"\n" +
		"    exit 0\n" +
		"    ;;\n" +
		"  clone-fail)\n" +
		"    echo \"fatal: repository not found\"\n" +
		"    exit 128\n" +
		"    ;;\n" +
		"  *)\n" +
		"    exit 0\n" +
		"    ;;\n" +
		"esac\n"
	scriptPath := filepath.Join(dir, "git")
	if err := os.WriteFile(scriptPath, []byte(script), 0o700); err != nil { //nolint:gosec // test-only static script (G306)
		t.Fatalf("FakeGitDir: writing fake git: %v", err)
	}
	t.Setenv("GITID_FAKE_GIT_MODE", mode)
	return dir
}

func FakeGitShimDir(t *testing.T, version, failSubcommand string) string {
	t.Helper()
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("FakeGitShimDir: locating real git: %v", err)
	}
	dir := t.TempDir()
	const script = "#!/bin/sh\n" +
		"if [ \"$1\" = \"--version\" ] && [ \"$#\" -eq 1 ]; then\n" +
		"  printf 'git version %s\\n' \"$GITID_FAKE_GIT_VERSION\"\n" +
		"  exit 0\n" +
		"fi\n" +
		"if [ \"$1\" = \"$GITID_FAKE_GIT_FAIL_SUBCOMMAND\" ]; then\n" +
		"  printf 'fake git: %s failed\\n' \"$1\" >&2\n" +
		"  exit 1\n" +
		"fi\n" +
		"exec \"$GITID_REAL_GIT\" \"$@\"\n"
	scriptPath := filepath.Join(dir, "git")
	if err := os.WriteFile(scriptPath, []byte(script), 0o700); err != nil { //nolint:gosec // test-only static script (G306)
		t.Fatalf("FakeGitShimDir: writing fake git: %v", err)
	}
	t.Setenv("GITID_REAL_GIT", realGit)
	t.Setenv("GITID_FAKE_GIT_VERSION", version)
	t.Setenv("GITID_FAKE_GIT_FAIL_SUBCOMMAND", failSubcommand)
	return dir
}

// setupLocalBareRepo initialises a git bare repository in a temp directory and
// returns its file:// URL and base name. The REAL system git is used here (not a
// fake) so the network-free clone target is a genuine git repository.
func TestFakeGitShimDelegatesOverridesVersionAndFailsSubcommand(t *testing.T) {
	seeded := filepath.Join(t.TempDir(), "seeded.gitconfig")
	writeFileT(t, seeded, "[user]\n\tname = Shim User\n\temail = shim@example.com\n")

	git := filepath.Join(FakeGitShimDir(t, "2.34.1", "rev-parse"), "git")
	version, err := exec.Command(git, "--version").CombinedOutput() //nolint:gosec // git is the test-owned FakeGitShimDir script
	if err != nil {
		t.Fatalf("running shim version: %v\n%s", err, version)
	}
	if got, want := strings.TrimSpace(string(version)), "git version 2.34.1"; got != want {
		t.Fatalf("shim version = %q, want %q", got, want)
	}

	delegated, err := exec.Command(git, "config", "--file", seeded, "--list").CombinedOutput() //nolint:gosec // git is the test-owned FakeGitShimDir script
	if err != nil {
		t.Fatalf("running delegated config: %v\n%s", err, delegated)
	}
	for _, want := range []string{"user.name=Shim User", "user.email=shim@example.com"} {
		if !strings.Contains(string(delegated), want) {
			t.Fatalf("delegated config = %q, want %q", delegated, want)
		}
	}

	failed, err := exec.Command(git, "rev-parse", "--git-dir").CombinedOutput() //nolint:gosec // git is the test-owned FakeGitShimDir script
	if err == nil {
		t.Fatalf("configured failing subcommand succeeded: %s", failed)
	}
	if !strings.Contains(string(failed), "fake git: rev-parse failed") {
		t.Fatalf("configured failing subcommand output = %q", failed)
	}
}

// runFakeCLI runs the shim at binPath with e2eEnv's hermetic base plus
// extraEnv appended (the mode/log/inventory-file overrides each test case
// supplies), returning combined output and the exit code (-1 on a
// non-ExitError failure). Routing even a test-owned shim invocation through
// e2eEnv keeps the package's "every .Env assignment traces to e2eEnv" rule
// exceptionless rather than growing the documented-allowlist surface for a
// callee that could easily just call it directly.
func runFakeCLI(t *testing.T, binPath string, extraEnv []string, args ...string) (out string, code int) {
	t.Helper()
	cmd := exec.Command(binPath, args...) //nolint:gosec // binPath is the test-owned fake-CLI script (G204)
	env, _ := e2eEnv(t, t.TempDir())
	cmd.Env = append(env, extraEnv...)
	outBytes, err := cmd.CombinedOutput()
	if err == nil {
		return string(outBytes), 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return string(outBytes), exitErr.ExitCode()
	}
	t.Fatalf("runFakeCLI: %v", err)
	return "", -1
}

// TestFakeGHShimModesBehaveAsDocumented exercises every fake-gh mode
// FakeGHDir documents, asserting the exit code, an output substring, and
// that the invocation was recorded to the log — including the fail-closed
// unknown-verb branch.
func TestFakeGHShimModesBehaveAsDocumented(t *testing.T) {
	cases := []struct {
		name       string
		mode       string
		args       []string
		wantCode   int
		wantOutput string
	}{
		{"ok/auth", "ok", []string{"auth", "status", "--hostname", "github.com"}, 0, ""},
		{"ok/ssh-key-add", "ok", []string{"ssh-key", "add", "k.pub", "--title", "t", "--type", "authentication"}, 0, "Added SSH key."},
		{"auth-fail/auth", "auth-fail", []string{"auth", "status", "--hostname", "github.com"}, 1, "not logged into github.com"},
		{"scope-fail-signing/auth-ok", "scope-fail-signing", []string{"ssh-key", "add", "k.pub", "--title", "t", "--type", "authentication"}, 0, "Added SSH key."},
		{"scope-fail-signing/signing-fails", "scope-fail-signing", []string{"ssh-key", "add", "k.pub", "--title", "t", "--type", "signing"}, 1, "admin:ssh_signing_key"},
		{"scope-fail-all", "scope-fail-all", []string{"ssh-key", "add", "k.pub", "--title", "t", "--type", "authentication"}, 1, "admin:public_key"},
		{"duplicate", "duplicate", []string{"ssh-key", "add", "k.pub", "--title", "t", "--type", "authentication"}, 0, "already exists"},
		{"inventory-both/keys", "inventory-both", []string{"api", "user/keys"}, 0, ""},
		{"inventory-both/signing-keys", "inventory-both", []string{"api", "user/ssh_signing_keys"}, 0, ""},
		{"inventory-auth-only/signing-empty", "inventory-auth-only", []string{"api", "user/ssh_signing_keys"}, 0, "[]"},
		{"inventory-fail", "inventory-fail", []string{"api", "user/keys"}, 1, "could not read inventory"},
		{"delete-ok", "delete-ok", []string{"ssh-key", "delete", "12345"}, 0, ""},
		{"unknown-verb", "ok", []string{"totally-unrecognized"}, 3, "gitid-e2e-unhandled-verb: gh totally-unrecognized"},
		{"unknown-ssh-key-verb", "ok", []string{"ssh-key", "frobnicate"}, 3, "gitid-e2e-unhandled-verb: gh ssh-key frobnicate"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir, logPath := FakeGHDir(t, c.mode)
			bin := filepath.Join(dir, "gh")
			extraEnv := []string{
				"GITID_FAKE_GH_MODE=" + c.mode,
				"GITID_FAKE_GH_LOG=" + logPath,
				"GITID_FAKE_GH_INVENTORY_FILE=" + os.Getenv("GITID_FAKE_GH_INVENTORY_FILE"),
			}
			out, code := runFakeCLI(t, bin, extraEnv, c.args...)
			if code != c.wantCode {
				t.Errorf("exit code = %d, want %d (output: %q)", code, c.wantCode, out)
			}
			if c.wantOutput != "" && !strings.Contains(out, c.wantOutput) {
				t.Errorf("output = %q, want substring %q", out, c.wantOutput)
			}
			log := ReadFakeCLILog(t, logPath)
			if len(log) != 1 {
				t.Fatalf("log recorded %d invocations, want 1: %v", len(log), log)
			}
			if log[0] != strings.Join(c.args, " ") {
				t.Errorf("logged argv = %q, want %q", log[0], strings.Join(c.args, " "))
			}
		})
	}
}

// TestFakeGLabShimModesBehaveAsDocumented is TestFakeGHShimModesBehaveAsDocumented's
// glab-mode sibling.
func TestFakeGLabShimModesBehaveAsDocumented(t *testing.T) {
	cases := []struct {
		name       string
		mode       string
		args       []string
		wantCode   int
		wantOutput string
	}{
		{"ok/auth", "ok", []string{"auth", "status", "--hostname", "gitlab.com"}, 0, ""},
		{"ok/ssh-key-add", "ok", []string{"ssh-key", "add", "k.pub", "-t", "t", "--usage-type", "auth"}, 0, "Added SSH key."},
		{"auth-fail/auth", "auth-fail", []string{"auth", "status", "--hostname", "gitlab.com"}, 1, "not authenticated to gitlab.com"},
		{"taken", "taken", []string{"ssh-key", "add", "k.pub", "-t", "t", "--usage-type", "auth"}, 1, "already been taken"},
		{"inventory-present", "inventory-present", []string{"ssh-key", "list", "-F", "json"}, 0, ""},
		{"inventory-empty", "inventory-empty", []string{"ssh-key", "list", "-F", "json"}, 0, "[]"},
		{"inventory-fail", "inventory-fail", []string{"ssh-key", "list", "-F", "json"}, 1, "could not read inventory"},
		{"delete-ok", "delete-ok", []string{"ssh-key", "delete", "12345"}, 0, ""},
		{"unknown-verb", "ok", []string{"totally-unrecognized"}, 3, "gitid-e2e-unhandled-verb: glab totally-unrecognized"},
		{"unknown-ssh-key-verb", "ok", []string{"ssh-key", "frobnicate"}, 3, "gitid-e2e-unhandled-verb: glab ssh-key frobnicate"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir, logPath := FakeGLabDir(t, c.mode)
			bin := filepath.Join(dir, "glab")
			extraEnv := []string{
				"GITID_FAKE_GLAB_MODE=" + c.mode,
				"GITID_FAKE_GLAB_LOG=" + logPath,
				"GITID_FAKE_GLAB_INVENTORY_FILE=" + os.Getenv("GITID_FAKE_GLAB_INVENTORY_FILE"),
			}
			out, code := runFakeCLI(t, bin, extraEnv, c.args...)
			if code != c.wantCode {
				t.Errorf("exit code = %d, want %d (output: %q)", code, c.wantCode, out)
			}
			if c.wantOutput != "" && !strings.Contains(out, c.wantOutput) {
				t.Errorf("output = %q, want substring %q", out, c.wantOutput)
			}
			log := ReadFakeCLILog(t, logPath)
			if len(log) != 1 {
				t.Fatalf("log recorded %d invocations, want 1: %v", len(log), log)
			}
			if log[0] != strings.Join(c.args, " ") {
				t.Errorf("logged argv = %q, want %q", log[0], strings.Join(c.args, " "))
			}
		})
	}
}

// TestFakeGHInventoryFileFeedsAPIVerb proves the inventory-file mechanism:
// FakeGHInventoryFile's content is what the "inventory-both" mode's `api
// user/keys` verb echoes back verbatim.
func TestFakeGHInventoryFileFeedsAPIVerb(t *testing.T) {
	dir, logPath := FakeGHDir(t, "inventory-both")
	bin := filepath.Join(dir, "gh")
	invPath := FakeGHInventoryFile(t, `[{"key":"ssh-ed25519 AAAAFAKE fixture@gitid"}]`)
	extraEnv := []string{
		"GITID_FAKE_GH_MODE=inventory-both",
		"GITID_FAKE_GH_LOG=" + logPath,
		"GITID_FAKE_GH_INVENTORY_FILE=" + invPath,
	}
	out, code := runFakeCLI(t, bin, extraEnv, "api", "user/keys")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0: %s", code, out)
	}
	if !strings.Contains(out, "ssh-ed25519 AAAAFAKE fixture@gitid") {
		t.Errorf("output = %q, want the inventory fixture content echoed back", out)
	}
}

func setupLocalBareRepo(t *testing.T) (repoURL, repoName string) {
	t.Helper()
	bare := t.TempDir()
	if err := exec.Command("git", "init", "--bare", bare).Run(); err != nil { //nolint:gosec // arg-slice; no shell (G204)
		t.Fatalf("setupLocalBareRepo: git init --bare: %v", err)
	}
	return "file://" + bare, filepath.Base(bare)
}

// ---------------------------------------------------------------------------
// 05-09-PLAN.md Task 1 sandbox seeding helpers — the identity-manager
// PTY suite's fixtures. Each helper is a single-purpose, raw-disk-write
// seeder (never a shared "big fixture" function): one identity shape per
// helper call, composed together by the caller into one sandbox home,
// mirroring the pattern already established by seedMinimalIdentity
// (ui_pty_e2e_test.go), seedGitPTYIdentity/seedRecipeShapeSSHConfig
// (git_configuration_pty_e2e_test.go / identity_manager_pty_e2e_test.go),
// and cmd/gitid/wiring_test.go's seedDeleteFixture/
// seedTwoIdentitiesSameProvider/seedCloneSourceFixture.
// ---------------------------------------------------------------------------

// writeStubKeyPair writes a non-cryptographic stub ed25519 private/public
// key pair at home's ~/.ssh/id_ed25519_<name> (+ .pub) — sufficient for
// every identity-manager fixture in this file, none of which perform a real
// SSH handshake (mirrors seedMinimalIdentity's own stub-key convention).
func writeStubKeyPair(t *testing.T, home, name string) {
	t.Helper()
	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatalf("writeStubKeyPair: MkdirAll .ssh: %v", err)
	}
	priv := filepath.Join(sshDir, "id_ed25519_"+name)
	if err := os.WriteFile(priv, []byte("-----BEGIN OPENSSH PRIVATE KEY-----\nSTUB-"+name+"\n-----END OPENSSH PRIVATE KEY-----\n"), 0o600); err != nil {
		t.Fatalf("writeStubKeyPair: WriteFile priv: %v", err)
	}
	pub := fmt.Sprintf("ssh-ed25519 AAAAC3NzaC1lZDI1NTE5STUB%s %s@gitid-test\n", name, name)
	if err := os.WriteFile(priv+".pub", []byte(pub), 0o644); err != nil { //nolint:gosec // hermetic sandbox HOME fixture (G306)
		t.Fatalf("writeStubKeyPair: WriteFile pub: %v", err)
	}
}

// taxonomyIdentity names one identity seedEightTaxonomyIdentities plants,
// plus the MGR-02 axis pair it is built to prove reachable and the
// ClassifyState collapsed word its OWN detail pane must render.
type taxonomyIdentity struct {
	Name              string
	WantIdentityState string
	WantKeyState      string
	WantRowWord       string
}

// seedEightTaxonomyIdentities seeds a sandbox home with SEVEN identities
// whose (IdentityState, KeyState) axis pairs union to all EIGHT locked
// MGR-02 labels, and whose ClassifyState collapse results are the SEVEN
// reachable row words (05-01-SUMMARY.md's frozen taxonomy resolution,
// review R-06 — key-used-both is observable only on the KeyState axis,
// never as a collapsed row word). Also plants one ORPHAN key file that no
// Host block anywhere references, for the Inventory.UnusedKeys assertion.
//
// "kun" (complete / key-unused) is the one non-obvious fixture: a
// per-identity KeyState of key-unused is structurally unreachable for any
// identity with its OWN SSH Host block under NORMAL circumstances, because
// BuildInventory's keyUsedInSSH fact is a literal string membership check
// of acct.KeyPath against sshconfig.ParseAllHostIdentityFiles' parsed set —
// and an identity's own managed Host block trivially satisfies that check
// against itself. The one real-world condition that breaks it is a
// leftover/duplicate HAND-WRITTEN Host stanza for the SAME alias, placed
// BEFORE gitid's own managed block, naming a DIFFERENT IdentityFile:
// OpenSSH's (and kevinburke/ssh_config's) "first obtained value wins"
// resolution order means the hand-written stanza's key path is what ends up
// in the referenced set, not gitid's own managed block's key — so the
// managed identity's real key is never found "in use" over SSH, even though
// the identity itself is structurally complete. Verified empirically against
// a scratch identity.BuildInventory call before being written into this
// fixture (see this plan's SUMMARY for the finding).
func seedEightTaxonomyIdentities(t *testing.T, home string) []taxonomyIdentity {
	t.Helper()

	writeStubKeyPair(t, home, "cpl")
	writeStubKeyPair(t, home, "kun")
	writeStubKeyPair(t, home, "kun_other")
	writeStubKeyPair(t, home, "inc")
	writeStubKeyPair(t, home, "fpm")
	// "kms" deliberately has NO key file on disk — key-missing.
	writeStubKeyPair(t, home, "sso")
	writeStubKeyPair(t, home, "orphan") // referenced by no Host block anywhere

	sshConfig := "" +
		"# BEGIN gitid managed: _global\n" +
		"Host *\n" +
		"  IdentitiesOnly yes\n" +
		"# END gitid managed: _global\n\n" +
		// Hand-written duplicate stanza (kun's key-unused fixture) — MUST
		// precede the gitid-managed "kun" block below.
		"Host kun.github.com\n" +
		"  IdentityFile ~/.ssh/id_ed25519_kun_other\n\n" +
		sshHostBlock("cpl") +
		sshHostBlock("kun") +
		sshHostBlock("inc") +
		sshHostBlock("fpm") +
		sshHostBlock("kms") +
		sshHostBlock("sso")
	writeFileT(t, filepath.Join(home, ".ssh", "config"), sshConfig)

	gitconfig := "[user]\n  name = Global User\n\n" +
		gitconfigIncludeIfBlock("cpl") +
		gitconfigIncludeIfBlock("kun") +
		gitconfigIncludeIfBlock("gto") +
		gitconfigIncludeIfBlock("fpm") +
		gitconfigIncludeIfBlock("kms") +
		gitconfigIncludeIfBlock("sso")
	writeFileT(t, filepath.Join(home, ".gitconfig"), gitconfig)

	gitconfigD := filepath.Join(home, ".gitconfig.d")
	if err := os.MkdirAll(gitconfigD, 0o700); err != nil {
		t.Fatalf("seedEightTaxonomyIdentities: MkdirAll .gitconfig.d: %v", err)
	}
	// cpl: full SSH-signing fragment -> keyUsedInGit=true, keyUsedInSSH=true -> key-used-both.
	writeFileT(t, filepath.Join(gitconfigD, "cpl"), signingFragment("cpl"))
	// kun: no signing config -> keyUsedInGit=false; keyUsedInSSH=false (the
	// duplicate-stanza trick above) -> key-unused.
	writeFileT(t, filepath.Join(gitconfigD, "kun"), plainFragment("kun"))
	// gto: git-only (NO SSH Host block seeded above) -> always key-missing
	// through the real pipeline (acct.KeyPath stays empty with no Host
	// block), regardless of fragment content.
	writeFileT(t, filepath.Join(gitconfigD, "gto"), plainFragment("gto"))
	// fpm: gitconfig includeIf block present, but the FRAGMENT FILE itself
	// is never written -> fragment-path-missing.
	// kms: full signing fragment, but its key file was never written above
	// -> key-missing wins regardless of signing config.
	writeFileT(t, filepath.Join(gitconfigD, "kms"), signingFragment("kms"))
	// sso: no signing config, key exists and is SSH-referenced -> key-used-ssh-only.
	writeFileT(t, filepath.Join(gitconfigD, "sso"), plainFragment("sso"))

	return []taxonomyIdentity{
		{Name: "cpl", WantIdentityState: "complete", WantKeyState: "key-used-both", WantRowWord: "complete"},
		{Name: "kun", WantIdentityState: "complete", WantKeyState: "key-unused", WantRowWord: "key-unused"},
		{Name: "inc", WantIdentityState: "incomplete", WantKeyState: "key-used-ssh-only", WantRowWord: "incomplete"},
		{Name: "gto", WantIdentityState: "git-only", WantKeyState: "key-missing", WantRowWord: "git-only"},
		{Name: "fpm", WantIdentityState: "fragment-path-missing", WantKeyState: "key-used-ssh-only", WantRowWord: "fragment-path-missing"},
		{Name: "kms", WantIdentityState: "complete", WantKeyState: "key-missing", WantRowWord: "key-missing"},
		{Name: "sso", WantIdentityState: "complete", WantKeyState: "key-used-ssh-only", WantRowWord: "key-used-ssh-only"},
	}
}

// sshHostBlock renders one gitid-managed SSH Host block for name at the
// recipe-shape geometry (alt-SSH Hostname/Port) every other fixture in this
// package uses.
func sshHostBlock(name string) string {
	return "# BEGIN gitid managed: " + name + "\n" +
		"Host " + name + ".github.com\n" +
		"  Hostname ssh.github.com\n" +
		"  Port 443\n" +
		"  User git\n" +
		"  IdentityFile ~/.ssh/id_ed25519_" + name + "\n" +
		"  IdentitiesOnly yes\n" +
		"# END gitid managed: " + name + "\n\n"
}

// gitconfigIncludeIfBlock renders one gitid-managed gitconfig includeIf
// block for name, pointing at ~/.gitconfig.d/<name>.
func gitconfigIncludeIfBlock(name string) string {
	return "# BEGIN gitid managed: " + name + "\n" +
		"[includeIf \"gitdir:~/git/" + name + "/\"]\n" +
		"  path = ~/.gitconfig.d/" + name + "\n" +
		"# END gitid managed: " + name + "\n\n"
}

// signingFragment renders a gitconfig fragment wired for ssh commit signing
// with name's own key (drives keyUsedInGit=true when the key is readable).
func signingFragment(name string) string {
	return "[user]\n  name = " + name + " User\n  email = " + name + "@example.com\n" +
		"  signingkey = ~/.ssh/id_ed25519_" + name + ".pub\n\n" +
		"[gpg]\n  format = ssh\n\n[commit]\n  gpgsign = true\n"
}

// plainFragment renders a gitconfig fragment with no signing configuration
// at all (drives keyUsedInGit=false).
func plainFragment(name string) string {
	return "[user]\n  name = " + name + " User\n  email = " + name + "@example.com\n"
}

// writeFileT is os.WriteFile with the shared 0o644 mode and a t.Fatalf on
// error — the common tail every helper in this section shares.
func writeFileT(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil { //nolint:gosec // hermetic sandbox HOME fixture (G306)
		t.Fatalf("writeFileT: WriteFile %s: %v", path, err)
	}
}

// seedSharedKeyIdentities seeds TWO identities whose SSH Host blocks
// reference the literal SAME key path — the D-12 shared-key fixture: a
// delete-everything or rotate/repair ceremony against either identity must
// name the other as a sibling owner and never destroy the shared key.
// Returns the two identity names.
func seedSharedKeyIdentities(t *testing.T, home string) (first, second string) {
	t.Helper()
	first, second = "shareA", "shareB"
	writeStubKeyPair(t, home, "shared")

	sshConfig := "" +
		"# BEGIN gitid managed: _global\n" +
		"Host *\n" +
		"  IdentitiesOnly yes\n" +
		"# END gitid managed: _global\n\n" +
		"# BEGIN gitid managed: " + first + "\n" +
		"Host " + first + ".github.com\n" +
		"  Hostname ssh.github.com\n" +
		"  Port 443\n" +
		"  User git\n" +
		"  IdentityFile ~/.ssh/id_ed25519_shared\n" +
		"  IdentitiesOnly yes\n" +
		"# END gitid managed: " + first + "\n\n" +
		"# BEGIN gitid managed: " + second + "\n" +
		"Host " + second + ".github.com\n" +
		"  Hostname ssh.github.com\n" +
		"  Port 443\n" +
		"  User git\n" +
		"  IdentityFile ~/.ssh/id_ed25519_shared\n" +
		"  IdentitiesOnly yes\n" +
		"# END gitid managed: " + second + "\n\n"
	writeFileT(t, filepath.Join(home, ".ssh", "config"), sshConfig)

	gitconfig := "[user]\n  name = Global User\n\n" +
		gitconfigIncludeIfBlock(first) + gitconfigIncludeIfBlock(second)
	writeFileT(t, filepath.Join(home, ".gitconfig"), gitconfig)

	if err := os.MkdirAll(filepath.Join(home, ".gitconfig.d"), 0o700); err != nil {
		t.Fatalf("seedSharedKeyIdentities: MkdirAll .gitconfig.d: %v", err)
	}
	writeFileT(t, filepath.Join(home, ".gitconfig.d", first), plainFragment(first))
	writeFileT(t, filepath.Join(home, ".gitconfig.d", second), plainFragment(second))

	line := mustAllowedSignersLineE2E(t, first+"@example.com", "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5STUBshared shared@gitid-test")
	writeFileT(t, filepath.Join(home, ".ssh", "allowed_signers"),
		"# BEGIN gitid managed: "+first+"\n"+line+"# END gitid managed: "+first+"\n")

	return first, second
}

// mustAllowedSignersLineE2E renders one allowed_signers line for email/
// pubLine — the same shape mustAllowedSignersLine produces in
// cmd/gitid/wiring_test.go, duplicated here (a separate build-tagged test
// binary cannot import an unexported helper across packages).
func mustAllowedSignersLineE2E(t *testing.T, email, pubLine string) string {
	t.Helper()
	fields := strings.Fields(pubLine)
	if len(fields) < 2 {
		t.Fatalf("mustAllowedSignersLineE2E: malformed pub line %q", pubLine)
	}
	return email + ` namespaces="git" ` + fields[0] + " " + fields[1] + "\n"
}

// seedTwoIdentitiesSameProviderE2E seeds two identities on the SAME
// provider (github.com) — the D-09 fixture: deleting one (everything scope)
// must leave the shared per-provider insteadOf rewrite block untouched
// because the sibling still references it. Returns the two identity names.
func seedTwoIdentitiesSameProviderE2E(t *testing.T, home string) (first, second string) {
	t.Helper()
	first, second = "provA", "provB"
	writeStubKeyPair(t, home, first)
	writeStubKeyPair(t, home, second)

	sshConfig := "" +
		"# BEGIN gitid managed: _global\n" +
		"Host *\n" +
		"  IdentitiesOnly yes\n" +
		"# END gitid managed: _global\n\n" +
		sshHostBlock(first) + sshHostBlock(second)
	writeFileT(t, filepath.Join(home, ".ssh", "config"), sshConfig)

	gitconfig := "[user]\n  name = Global User\n\n" +
		gitconfigIncludeIfBlock(first) + gitconfigIncludeIfBlock(second) +
		"# BEGIN gitid managed: rewrite-github.com\n" +
		"[url \"git@github.com:\"]\n" +
		"  insteadOf = https://github.com/\n" +
		"# END gitid managed: rewrite-github.com\n"
	writeFileT(t, filepath.Join(home, ".gitconfig"), gitconfig)

	if err := os.MkdirAll(filepath.Join(home, ".gitconfig.d"), 0o700); err != nil {
		t.Fatalf("seedTwoIdentitiesSameProviderE2E: MkdirAll .gitconfig.d: %v", err)
	}
	writeFileT(t, filepath.Join(home, ".gitconfig.d", first), plainFragment(first))
	writeFileT(t, filepath.Join(home, ".gitconfig.d", second), plainFragment(second))

	return first, second
}

// plantedHit is one D-13 unmanaged-reference scan hit this fixture plants
// at a KNOWN, asserted (file, line) — never an approximate count (review
// R-27).
type plantedHit struct {
	File string
	Line int
}

// seedDeleteEverythingTargetWithPlantedHits seeds a complete, deletable
// identity named target PLUS exactly two D-13 scan hits at KNOWN lines: one
// inside a SIBLING identity's own managed Host block (region
// "other-managed" — a PermitOpen directive naming target's alias) and one
// in a hand-written, unmanaged trailing Host stanza (region "unmanaged").
// Mirrors cmd/gitid/wiring_test.go's TestDeletePlanScanReportsUnmanagedAndSiblingHits
// fixture exactly, so the two mechanisms can never disagree about what a
// planted hit looks like. Returns the two expected hits, file-relative to
// home, in the order the scan sources are read (SSH config first).
func seedDeleteEverythingTargetWithPlantedHits(t *testing.T, home, target string) []plantedHit {
	t.Helper()
	writeStubKeyPair(t, home, target)

	sshConfig := "" +
		"# BEGIN gitid managed: _global\n" + // line 1
		"Host *\n" + // 2
		"  IdentitiesOnly yes\n" + // 3
		"# END gitid managed: _global\n" + // 4
		"\n" + // 5
		"# BEGIN gitid managed: " + target + "\n" + // 6
		"Host " + target + ".github.com\n" + // 7
		"  Hostname ssh.github.com\n" + // 8
		"  Port 443\n" + // 9
		"  User git\n" + // 10
		"  IdentityFile ~/.ssh/id_ed25519_" + target + "\n" + // 11
		"  IdentitiesOnly yes\n" + // 12
		"# END gitid managed: " + target + "\n" + // 13
		"\n" + // 14
		"# BEGIN gitid managed: sibling\n" + // 15
		"Host sibling.github.com\n" + // 16
		"  Hostname ssh.github.com\n" + // 17
		"  Port 443\n" + // 18
		"  User git\n" + // 19
		"  IdentityFile ~/.ssh/id_ed25519_sibling\n" + // 20
		"  PermitOpen " + target + ".github.com:443\n" + // 21 -- other-managed hit
		"# END gitid managed: sibling\n" + // 22
		"\n" + // 23
		"Host " + target + ".github.com\n" + // 24 -- unmanaged hit (hand-written duplicate)
		"  Hostname ssh.github.com\n" + // 25
		"  IdentityFile ~/.ssh/id_ed25519_" + target + "\n" // 26
	writeFileT(t, filepath.Join(home, ".ssh", "config"), sshConfig)
	writeStubKeyPair(t, home, "sibling")

	gitconfig := "[user]\n  name = Global User\n\n" +
		gitconfigIncludeIfBlock(target) + gitconfigIncludeIfBlock("sibling")
	writeFileT(t, filepath.Join(home, ".gitconfig"), gitconfig)

	if err := os.MkdirAll(filepath.Join(home, ".gitconfig.d"), 0o700); err != nil {
		t.Fatalf("seedDeleteEverythingTargetWithPlantedHits: MkdirAll .gitconfig.d: %v", err)
	}
	writeFileT(t, filepath.Join(home, ".gitconfig.d", target), plainFragment(target))
	writeFileT(t, filepath.Join(home, ".gitconfig.d", "sibling"), plainFragment("sibling"))

	sshConfigPath := filepath.Join(home, ".ssh", "config")
	return []plantedHit{
		{File: sshConfigPath, Line: 21},
		{File: sshConfigPath, Line: 24},
	}
}

// seedDeleteEverythingTargetClean seeds a complete, deletable identity named
// target with NO other identity and NO hand-written stanza referencing its
// alias anywhere — the D-13 zero-hits control: the unmanaged-reference
// warning block must not render at all.
func seedDeleteEverythingTargetClean(t *testing.T, home, target string) {
	t.Helper()
	writeStubKeyPair(t, home, target)

	sshConfig := "" +
		"# BEGIN gitid managed: _global\n" +
		"Host *\n" +
		"  IdentitiesOnly yes\n" +
		"# END gitid managed: _global\n\n" +
		sshHostBlock(target)
	writeFileT(t, filepath.Join(home, ".ssh", "config"), sshConfig)

	gitconfig := "[user]\n  name = Global User\n\n" + gitconfigIncludeIfBlock(target)
	writeFileT(t, filepath.Join(home, ".gitconfig"), gitconfig)

	if err := os.MkdirAll(filepath.Join(home, ".gitconfig.d"), 0o700); err != nil {
		t.Fatalf("seedDeleteEverythingTargetClean: MkdirAll .gitconfig.d: %v", err)
	}
	writeFileT(t, filepath.Join(home, ".gitconfig.d", target), plainFragment(target))
}

// repoRoot walks up from the test working directory until it finds a directory
// containing go.mod, which marks the repository root.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("repoRoot: Getwd: %v", err)
	}
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("repoRoot: go.mod not found walking up from %s", dir)
		}
		dir = parent
	}
}
