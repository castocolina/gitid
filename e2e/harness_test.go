//go:build e2e

package e2e

import (
	"fmt"
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

// FakeGHDir writes a mode-switching fake gh script and sets GITID_FAKE_GH_MODE.
// The caller prepends the returned dir to PATH via cmd.Env.
//
// Modes:
//
//	ok        — auth status exit 0; ssh-key add prints "Added SSH key." and exits 0
//	auth-fail — auth status exits 1 (not authenticated)
//
// Script is a static literal — never constructed from user input (G204-clean).
func FakeGHDir(t *testing.T, mode string) string {
	t.Helper()
	dir := t.TempDir()
	const script = "#!/bin/sh\n" +
		"case \"$1\" in\n" +
		"  auth)\n" +
		"    case \"$GITID_FAKE_GH_MODE\" in\n" +
		"      ok) exit 0 ;;\n" +
		"      *) echo \"error: not logged into github.com\"; exit 1 ;;\n" +
		"    esac\n" +
		"    ;;\n" +
		"  ssh-key)\n" +
		"    case \"$GITID_FAKE_GH_MODE\" in\n" +
		"      ok) echo \"Added SSH key.\"; exit 0 ;;\n" +
		"      *) echo \"error: not authenticated\"; exit 1 ;;\n" +
		"    esac\n" +
		"    ;;\n" +
		"  *)\n" +
		"    exit 0\n" +
		"    ;;\n" +
		"esac\n"
	scriptPath := filepath.Join(dir, "gh")
	if err := os.WriteFile(scriptPath, []byte(script), 0o700); err != nil { //nolint:gosec // test-only static script (G306)
		t.Fatalf("FakeGHDir: writing fake gh: %v", err)
	}
	t.Setenv("GITID_FAKE_GH_MODE", mode)
	return dir
}

// FakeGLabDir writes a mode-switching fake glab script and sets GITID_FAKE_GLAB_MODE.
// The caller prepends the returned dir to PATH via cmd.Env.
//
// Modes:
//
//	ok        — auth status exit 0; ssh-key add exits 0
//	auth-fail — auth status exits 1 (not authenticated)
//
// Script is a static literal — never constructed from user input (G204-clean).
func FakeGLabDir(t *testing.T, mode string) string {
	t.Helper()
	dir := t.TempDir()
	const script = "#!/bin/sh\n" +
		"case \"$1\" in\n" +
		"  auth)\n" +
		"    case \"$GITID_FAKE_GLAB_MODE\" in\n" +
		"      ok) exit 0 ;;\n" +
		"      *) echo \"error: not authenticated to gitlab.com\"; exit 1 ;;\n" +
		"    esac\n" +
		"    ;;\n" +
		"  ssh-key)\n" +
		"    case \"$GITID_FAKE_GLAB_MODE\" in\n" +
		"      ok) echo \"Added SSH key.\"; exit 0 ;;\n" +
		"      *) echo \"error: not authenticated\"; exit 1 ;;\n" +
		"    esac\n" +
		"    ;;\n" +
		"  *)\n" +
		"    exit 0\n" +
		"    ;;\n" +
		"esac\n"
	scriptPath := filepath.Join(dir, "glab")
	if err := os.WriteFile(scriptPath, []byte(script), 0o700); err != nil { //nolint:gosec // test-only static script (G306)
		t.Fatalf("FakeGLabDir: writing fake glab: %v", err)
	}
	t.Setenv("GITID_FAKE_GLAB_MODE", mode)
	return dir
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

// setupLocalBareRepo initialises a git bare repository in a temp directory and
// returns its file:// URL and base name. The REAL system git is used here (not a
// fake) so the network-free clone target is a genuine git repository.
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
