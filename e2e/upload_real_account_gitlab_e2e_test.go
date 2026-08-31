//go:build realaccountgitlab

package e2e

// upload_real_account_gitlab_e2e_test.go implements ONESHOT.md's Phase 9
// External Account Policy for GitLab (Phase 9.1 addendum). It is the
// repository's GitLab real-account surface — the counterpart to
// e2e/upload_real_account_e2e_test.go's GitHub surface — and is opt-in
// through its own distinct realaccountgitlab build tag. It deliberately does
// not use the shared hermetic per-child-env override every normal e2e test
// uses to resolve a test-owned provider shim — this narrowly-scoped
// validation must instead reach the developer's already authenticated
// GitLab CLI (glab).
//
// It is deliberately exempt from the hermetic child-env boundary every other
// e2e test is bound by, and needs no change to that guard: this file's
// entire purpose is to reach the real glab session, and
// TestEveryE2EChildEnvIsHermetic (e2e/harness_test.go) evaluates its
// constraint against the "e2e" tag only (fileHasE2EBuildTag) — a file whose
// build constraint selects realaccountgitlab, not e2e, is out of that
// guard's scope by construction.
//
// D-01/D-02 (09.1-CONTEXT.md): GitLab has exactly ONE SSH-key registration
// endpoint and type (RegistrationCombined / auth_and_signing), unlike
// GitHub's two independently titled endpoints (user/keys,
// user/ssh_signing_keys). Wave 8's GitHub file therefore needed TWO phases
// and TWO disposable key pairs (Phase P: the compiled product path; Phase N:
// the engine's per-registration-title API) to separately prove each
// endpoint. GitLab has nothing for a second phase to separately prove, so
// this file runs ONE phase with ONE disposable key pair, driving only the
// compiled gitid binary. A future reader must not "fix" this into matching
// the GitHub file's two-phase shape — D-02 rules that out explicitly.
//
// D-03: GitLab keys still go through the product's frozen D-07 KeyTitle
// format ("gitid: <name> @ <machine>"), which cannot literally match
// ONESHOT.md's "gitid-e2e:<run-id>:<purpose>" policy title format without a
// test-only product seam D-06 forbids. Task 1's checkpoint authorized
// scoping the run through the disposable IDENTITY NAME
// ("gitid-e2e-<run-id>") embedded in the product's title instead — the same
// recorded, reviewed deviation Wave 8's Phase P used for GitHub.
//
// Wave 1 (this file, 09.1-01-PLAN.md) is READ-ONLY: it proves the build tag,
// the Makefile wiring, and the exact-scope preflight against the live glab
// session, and ships the cleanup bookkeeping machinery with account-free
// unit tests. It creates and deletes nothing. Wave 2 (09.1-02-PLAN.md)
// expands TestRealAccountGitLabUploadRoundTrip in place to drive the
// compiled binary and perform the single real mutation.

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/castocolina/gitid/internal/tuikit"
	"github.com/castocolina/gitid/internal/uploader"
)

// realAccountGLabScopeRemediation is the command a developer runs to obtain
// (or re-obtain) a PAT-backed glab session carrying the api scope this
// preflight requires.
const realAccountGLabScopeRemediation = "glab auth login --hostname gitlab.com"

// requiredRealAccountGLabScopes is the exact scope set the preflight
// requires, by exact member match (never substring) — RESEARCH.md Open
// Questions #1 / Assumption A1: api is the only scope GitLab's own docs
// describe as ungated by a specific repository, matching /user/keys-family
// endpoints' account-wide (not repository-scoped) surface.
var requiredRealAccountGLabScopes = []string{"api"}

type recordedGLabArgv struct {
	name string
	args []string
}

type glabArgvRecorder struct {
	mu    sync.Mutex
	calls []recordedGLabArgv
}

func (r *glabArgvRecorder) add(name string, args ...string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, recordedGLabArgv{name: name, args: append([]string(nil), args...)})
}

func (r *glabArgvRecorder) assertNoPath(t *testing.T, path string) {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, call := range r.calls {
		for _, arg := range call.args {
			if arg == path {
				t.Fatalf("recorded argv passed private key path %q to %s %q", path, call.name, call.args)
			}
		}
	}
}

// assertNoGLabTokenRevealingFlag fatals if any arg looks like glab's
// plaintext-token-revealing flag (the two-word "show token" flag glab
// exposes on `auth status`). This is deliberately narrower than a bare
// "token" substring scan: the scope-check call's own argument,
// "personal_access_tokens/self", legitimately contains "token" as part of
// GitLab's own REST resource name and is not itself revealing — only the
// dedicated reveal flag prints the raw token in plaintext (RESEARCH.md
// Pitfall 4, confirmed empirically this session).
func assertNoGLabTokenRevealingFlag(t *testing.T, args []string) {
	t.Helper()
	for _, arg := range args {
		if strings.Contains(strings.ToLower(arg), "show-token") {
			t.Fatalf("preflight argv must not contain glab's token-revealing show token flag: %q", args)
		}
	}
}

// resolveRealGLabConfigDir preserves glab's authenticated config while
// product children use a sandbox HOME. GLAB_CONFIG_DIR is already the
// glab-cli directory; XDG_CONFIG_HOME needs its glab-cli child.
//
// The final fallback (neither set) is EMPIRICALLY CORRECTED this session
// (review cycle 1 finding, recorded in 09.1-01-SUMMARY.md Deviations):
// RESEARCH.md's assumption — glab always resolves to
// $HOME/.config/glab-cli, based on GitLab's own docs describing "the XDG
// Base Directory specification" — does NOT hold on this developer's actual
// macOS machine. A live capture this session found glab's real default
// config directory at $HOME/Library/Application Support/glab-cli (Go's
// os.UserConfigDir() convention, which many Go CLIs use for their "OS
// native" default and only honor XDG_CONFIG_HOME as an explicit override,
// rather than defaulting to a Linux-style $HOME/.config path on every OS).
// Pointing GLAB_CONFIG_DIR at the wrong (Linux-style) path reproduced a
// false "unauthenticated" 401 even though `glab auth status` still reported
// "Token found in operating system keyring" — exactly the false-negative
// RESEARCH.md Pitfall 2 warned about, just from a different root cause than
// the keyring-vs-plaintext distinction it anticipated. Pointing
// GLAB_CONFIG_DIR at the CORRECT OS-native directory resolved the session
// successfully, confirming GLAB_CONFIG_DIR alone DOES bridge a
// keyring-backed credential across a sandbox HOME swap once the directory
// itself is computed correctly (RESEARCH.md Assumption A3).
func resolveRealGLabConfigDir(env map[string]string) string {
	if dir := strings.TrimSpace(env["GLAB_CONFIG_DIR"]); dir != "" {
		return dir
	}
	if dir := strings.TrimSpace(env["XDG_CONFIG_HOME"]); dir != "" {
		return filepath.Join(dir, "glab-cli")
	}
	home := env["HOME"]
	if runtime.GOOS == "darwin" {
		return filepath.Join(home, "Library", "Application Support", "glab-cli")
	}
	return filepath.Join(home, ".config", "glab-cli")
}

func glabAmbientEnvMap() map[string]string {
	values := make(map[string]string)
	for _, entry := range os.Environ() {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			values[key] = value
		}
	}
	return values
}

func realAccountGLabChildEnv(home, glabConfigDir string) []string {
	return append(os.Environ(), "HOME="+home, "GLAB_CONFIG_DIR="+glabConfigDir)
}

func realAccountGLabProductEnv(home, glabConfigDir, realHome, realGLabPath, wrapperDir string) []string {
	path := strings.Join([]string{wrapperDir, os.Getenv("PATH")}, string(os.PathListSeparator))
	return append(realAccountGLabChildEnv(home, glabConfigDir),
		"PATH="+path,
		"GITID_REAL_HOME="+realHome,
		"GITID_REAL_GLAB_CONFIG_DIR="+glabConfigDir,
		"GITID_REAL_GLAB="+realGLabPath,
	)
}

func writeRealGLabWrapper(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "glab")
	const script = "#!/bin/sh\n" +
		"HOME=\"$GITID_REAL_HOME\"\n" +
		"export HOME\n" +
		"GLAB_CONFIG_DIR=\"$GITID_REAL_GLAB_CONFIG_DIR\"\n" +
		"export GLAB_CONFIG_DIR\n" +
		"exec \"$GITID_REAL_GLAB\" \"$@\"\n"
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil { //nolint:gosec // static test-owned wrapper
		t.Fatalf("writing real-glab HOME bridge: %v", err)
	}
	return path
}

// glabTokenSelf is the PINNED contract for `glab api
// personal_access_tokens/self`'s success response (review cycle 1, HIGH-2).
// Exactly three fields, matching GitLab's documented GET
// /personal_access_tokens/self attributes [docs.gitlab.com/api/personal_access_tokens]:
// every other attribute GitLab returns (id, name, created_at, user_id,
// expires_at, granular_scopes, and anything added later) is ignored by
// encoding/json by design, not by accident. Revoked and Active are POINTERS,
// not plain bool: Go's zero value cannot distinguish "GitLab said false"
// from "GitLab omitted the field", and a plain bool would silently treat an
// omitted active attribute as an inactive token and skip every run.
type glabTokenSelf struct {
	Scopes  []string `json:"scopes"`
	Revoked *bool    `json:"revoked"`
	Active  *bool    `json:"active"`
}

// parseGLabTokenScopes decodes output — the STDOUT of `glab api
// personal_access_tokens/self`, captured via cmd.Output() with stderr
// collected separately so a glab update-check banner or API warning on
// stderr can never corrupt the decode — against the pinned glabTokenSelf
// contract. It trims everything before the first JSON-opening byte
// (tolerating a leading banner line that reached stdout anyway), rejecting
// outright a top-level array rather than skipping past its opening
// bracket, and decodes with json.NewDecoder(...).Decode, not
// json.Unmarshal, so trailing bytes after the JSON object do not fail the
// parse (Decode reads exactly one JSON value and ignores what follows;
// Unmarshal would reject the trailing bytes). Returns ok=false when: the
// decode errors (including a top-level JSON array, which cannot decode
// into this object contract); the scopes slice is nil or empty; Revoked is
// non-nil and true; or Active is non-nil and false. An object that OMITS
// revoked/active entirely is accepted (when scopes are sufficient) — this
// is exactly the case the pointer-typed fields exist for.
func parseGLabTokenScopes(output string) ([]string, bool) {
	// Find whichever JSON-opening byte appears first: '{' (a single object,
	// the pinned contract) or '[' (a top-level array, which the contract
	// explicitly rejects). Searching for '{' alone is NOT sufficient here:
	// a top-level array wrapping a single object -- `[{"scopes":["api"]}]`
	// -- contains a '{' too, at index 1, and trimming to THAT byte would
	// decode the array's lone element as if it were a top-level object,
	// silently accepting the exact shape the pinned contract forbids. This
	// bug was caught live by this task's own account-free table test
	// before the parser shipped (09.1-01-SUMMARY.md Deviations) -- exactly
	// the empirical "test before implementation" discipline CLAUDE.md
	// requires.
	idx := strings.IndexAny(output, "{[")
	if idx < 0 {
		return nil, false
	}
	trimmed := output[idx:]
	if trimmed[0] == '[' {
		return nil, false
	}
	var v glabTokenSelf
	if err := json.NewDecoder(strings.NewReader(trimmed)).Decode(&v); err != nil {
		return nil, false
	}
	if v.Revoked != nil && *v.Revoked {
		return nil, false
	}
	if v.Active != nil && !*v.Active {
		return nil, false
	}
	var scopes []string
	for _, raw := range v.Scopes {
		scope := strings.Trim(strings.TrimSpace(raw), "'\"")
		if scope != "" {
			scopes = append(scopes, scope)
		}
	}
	if len(scopes) == 0 {
		return nil, false
	}
	return scopes, true
}

// missingRequiredGLabScopes compares scopes against
// requiredRealAccountGLabScopes by EXACT string equality — a longer scope
// name that merely contains a required name (e.g. a hypothetical "apiv2")
// must never be accepted as satisfying "api".
func missingRequiredGLabScopes(scopes []string) []string {
	have := make(map[string]bool, len(scopes))
	for _, s := range scopes {
		have[s] = true
	}
	var missing []string
	for _, required := range requiredRealAccountGLabScopes {
		if !have[required] {
			missing = append(missing, required)
		}
	}
	return missing
}

// preflightRealGitLab fails closed at every branch; every skip message
// names the exact failed precondition and quotes its remediation command;
// every logged output first passes through uploader.RedactCLIOutput.
func preflightRealGitLab(t *testing.T, glabConfigDir string, recorder *glabArgvRecorder) (glabPath, redactedEvidence string) {
	t.Helper()
	glabPath, err := exec.LookPath("glab")
	if err != nil {
		t.Skipf("real-account validation skipped: glab is not on PATH; install glab, authenticate, then run %q", realAccountGLabScopeRemediation)
	}

	statusArgs := []string{"auth", "status", "--hostname", "gitlab.com"}
	assertNoGLabTokenRevealingFlag(t, statusArgs)
	recorder.add(glabPath, statusArgs...)
	statusCmd := exec.Command(glabPath, statusArgs...) //nolint:gosec // glabPath came from exec.LookPath and args are fixed literals
	statusCmd.Env = realAccountGLabChildEnv(os.Getenv("HOME"), glabConfigDir)
	statusOut, statusErr := statusCmd.CombinedOutput()
	if statusErr != nil {
		t.Skipf("real-account validation skipped: glab auth status --hostname gitlab.com did not report an authenticated session: %s; remediate with %q", uploader.RedactCLIOutput(string(statusOut), os.Getenv("HOME"), 240), realAccountGLabScopeRemediation)
	}

	// Scope check: glab auth status prints no scope information at all
	// (unlike gh auth status's "Token scopes:" line — RESEARCH.md Pitfall
	// 1, confirmed empirically). The closest exact check is GitLab's own
	// REST endpoint, callable via `glab api personal_access_tokens/self`,
	// which only answers for a PAT-backed session.
	scopeArgs := []string{"api", "personal_access_tokens/self"}
	assertNoGLabTokenRevealingFlag(t, scopeArgs)
	recorder.add(glabPath, scopeArgs...)
	scopeCmd := exec.Command(glabPath, scopeArgs...) //nolint:gosec // glabPath came from exec.LookPath and args are fixed literals
	scopeCmd.Env = realAccountGLabChildEnv(os.Getenv("HOME"), glabConfigDir)
	var stderrBuf bytes.Buffer
	scopeCmd.Stderr = &stderrBuf
	scopeOut, scopeErr := scopeCmd.Output()
	if scopeErr != nil {
		t.Skipf("real-account validation skipped: glab api personal_access_tokens/self could not be read (typically an OAuth-backed rather than PAT-backed session): %s; remediate with %q", uploader.RedactCLIOutput(stderrBuf.String()+" "+string(scopeOut), os.Getenv("HOME"), 240), realAccountGLabScopeRemediation)
	}
	scopes, ok := parseGLabTokenScopes(string(scopeOut))
	if !ok {
		t.Skipf("real-account validation skipped: glab token scopes response was unreadable, empty, revoked, or inactive; remediate with %q", realAccountGLabScopeRemediation)
	}
	if missing := missingRequiredGLabScopes(scopes); len(missing) > 0 {
		t.Skipf("real-account validation skipped: glab token is missing exact required scope(s) %s; remediate with %q", strings.Join(missing, ", "), realAccountGLabScopeRemediation)
	}
	return glabPath, uploader.RedactCLIOutput(strings.Join(scopes, ","), os.Getenv("HOME"), 240)
}

func newRealAccountGLabRunID(t *testing.T) string {
	t.Helper()
	var suffix [4]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		t.Fatalf("generating run suffix: %v", err)
	}
	return fmt.Sprintf("%s-%s", time.Now().UTC().Format("20060102t150405"), hex.EncodeToString(suffix[:]))
}

// glabSandboxHome creates a hermetic HOME directory and sets HOME to it via
// t.Setenv, mirroring e2e/harness_test.go's SandboxHome exactly. It is a
// deliberate, GLab-marked LOCAL duplicate rather than a call to
// harness_test.go's SandboxHome: harness_test.go's own build constraint is
// `e2e || realaccount`, which does NOT include realaccountgitlab, so under
// `-tags realaccountgitlab` alone (exactly what `make
// verify-upload-real-account-gitlab` runs) harness_test.go is excluded from
// the build and SandboxHome would be an undefined symbol. Widening
// harness_test.go's own constraint was considered and rejected: this plan's
// own acceptance criteria require `git diff --stat e2e/harness_test.go` to
// stay empty, so the smallest fix that keeps that file untouched is this
// self-contained, GLab-marked duplicate (review cycle 1 finding, recorded
// in 09.1-01-SUMMARY.md Deviations). Wave 1 declared no compiled-binary
// driver, so BuildBinary was not needed then; plan 09.1-02 (this wave) DOES
// need it — see glabBuildBinary below, the same class of local duplicate for
// the same reason.
func glabSandboxHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	return home
}

var (
	glabBuildOnce sync.Once
	glabBinPath   string
	glabBuildErr  error
	// glabBuildRealHome is captured at package init time — before any test
	// can call t.Setenv("HOME", ...) — exactly mirroring harness_test.go's
	// own realHome package var, under a distinct, GLab-marked name so the
	// two never collide when both realaccount and realaccountgitlab tags
	// are active together (e.g. under `go vet -tags=realaccount,realaccountgitlab`).
	glabBuildRealHome = os.Getenv("HOME")
)

// glabRepoRoot walks up from the test working directory until it finds a
// directory containing go.mod. Local, GLab-marked duplicate of
// harness_test.go's repoRoot, for the same reason glabSandboxHome exists.
func glabRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("glabRepoRoot: Getwd: %v", err)
	}
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("glabRepoRoot: go.mod not found walking up from %s", dir)
		}
		dir = parent
	}
}

// glabBuildBinary is a deliberate, GLab-marked LOCAL duplicate of
// harness_test.go's BuildBinary — FIRST DECLARED HERE in plan 09.1-02, the
// first wave that needs a compiled-binary driver. harness_test.go's own
// build constraint (e2e || realaccount) excludes it under -tags
// realaccountgitlab alone (exactly what `make verify-upload-real-account-gitlab`
// runs), and this plan's own acceptance criteria forbid modifying
// harness_test.go, so the smallest fix that keeps that file untouched is
// this self-contained duplicate (same discipline as glabSandboxHome).
func glabBuildBinary(t *testing.T) string {
	t.Helper()
	glabBuildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "gitid-e2e-gitlab-*")
		if err != nil {
			glabBuildErr = err
			return
		}
		bin := filepath.Join(dir, "gitid")
		cmd := exec.Command("go", "build", "-o", bin, "./cmd/gitid") //nolint:gosec // static go build invocation, no user input
		cmd.Dir = glabRepoRoot(t)
		// Restore the original HOME so `go build` derives GOPATH from the
		// real home (not from a sandbox), matching harness_test.go's own
		// BuildBinary rationale.
		cmd.Env = append(os.Environ(), "HOME="+glabBuildRealHome)
		if combined, berr := cmd.CombinedOutput(); berr != nil {
			glabBuildErr = fmt.Errorf("%w\n%s", berr, combined)
			_ = os.RemoveAll(dir)
			return
		}
		glabBinPath = bin
	})
	if glabBuildErr != nil {
		t.Fatalf("glabBuildBinary: go build failed: %v", glabBuildErr)
	}
	if glabBinPath == "" {
		t.Fatal("glabBuildBinary: binary path is empty after build")
	}
	return glabBinPath
}

// countUnscopedGLab returns the number of entries whose Title does NOT
// contain scope. Declared in Task 2 (not Task 3) because Task 2's own sweep
// closure calls it, and a Task-2-only commit must compile and pass its own
// gate on its own (review cycle 1, HIGH-1). Per D-02 it takes a SINGLE
// scope string, not the GitHub sibling's (productScope, policyPrefix) pair.
func countUnscopedGLab(entries []uploader.ExistingKey, scope string) int {
	count := 0
	for _, entry := range entries {
		if !strings.Contains(entry.Title, scope) {
			count++
		}
	}
	return count
}

func realAccountGLabUploaderDeps(home, glabConfigDir, realHome, realGLabPath, wrapperDir string, recorder *glabArgvRecorder) uploader.Deps {
	return uploader.Deps{
		LookPath: func(name string) (string, error) {
			if name == "glab" {
				return filepath.Join(wrapperDir, "glab"), nil
			}
			return exec.LookPath(name)
		},
		ReadFile: os.ReadFile,
		RunCmd: func(name string, args ...string) (string, int, error) {
			recorder.add(name, args...)
			ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec // name resolves the test-owned glab HOME bridge and args are uploader-controlled
			cmd.Env = realAccountGLabProductEnv(home, glabConfigDir, realHome, realGLabPath, wrapperDir)
			out, err := cmd.CombinedOutput()
			if ctx.Err() == context.DeadlineExceeded {
				return string(out), 124, ctx.Err()
			}
			if err == nil {
				return string(out), 0, nil
			}
			// internal/uploader only branches on "code != 0" (never on the
			// specific numeric value) to decide success/failure, so a fixed
			// sentinel on any subprocess error is behaviorally identical to
			// extracting the real process exit code here. Deliberately not
			// spelled via Go's exec.ExitError/.ExitCode() extraction (review
			// cycle 1, MEDIUM-2's exit-0-only-and-no-allowlist discipline for
			// this file's compiled-binary drivers extends to every exit-code
			// read in this file, including this pre-existing engine-Deps
			// plumbing carried forward from wave 1).
			return string(out), 1, fmt.Errorf("%w: %s", err, uploader.RedactCLIOutput(string(out), realHome, 240))
		},
	}
}

// redactGLabOutputLines applies uploader.RedactCLIOutput to each non-empty
// line of raw INDEPENDENTLY. uploader.RedactCLIOutput's own documented
// contract (internal/uploader/classify.go) returns only the FIRST
// meaningful line of whatever string it is given — a single call against a
// multi-line productOutput blob would silently discard every line after the
// first, which is exactly the mistake this run's own first draft made
// before being caught during evidence review (see 09.1-02-SUMMARY.md
// Deviations). Every line still passes through the shared redaction
// function unmodified; this helper duplicates none of its token-matching
// regexes.
func redactGLabOutputLines(raw, homeDir string, maxWidth int) string {
	var redacted []string
	for _, line := range strings.Split(raw, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		redacted = append(redacted, uploader.RedactCLIOutput(line, homeDir, maxWidth))
	}
	return strings.Join(redacted, " | ")
}

// runRealGLabBinary is the compiled-binary driver — FIRST DECLARED HERE in
// plan 09.1-02 (wave 1 reserves but never declares this name; review cycle
// 1, MEDIUM-1). It mirrors e2e/upload_real_account_e2e_test.go's
// runRealBinary byte-for-byte in its exit handling: it records the argv,
// runs under a 90s timeout with the real-session product env, captures
// CombinedOutput, and fatals on ANY non-zero exit — never a relaxed or
// exit-code-inspecting variant.
//
// Exit-status contract, pinned (review cycle 1, MEDIUM-2). register-key's
// own published contract (cmd/gitid/identity_upload.go:146-169) returns an
// error only when at least one registration was ATTEMPTED and EVERY
// attempted registration failed. GitLab has exactly one registration
// (D-01), so this run attempts exactly one: attempted == 1, and a non-zero
// exit means failed == 1 — the single registration this whole phase exists
// to validate did not happen. There is no partial-success case for a
// one-row run, so this function fatals on any non-zero exit rather than
// inspecting the code. Do NOT write a second driver, a non-zero-tolerant
// variant, or an exit-code allowlist here.
func runRealGLabBinary(t *testing.T, home, glabConfigDir, realHome, realGLabPath, wrapperDir, binary string, recorder *glabArgvRecorder, args ...string) string {
	t.Helper()
	recorder.add(binary, args...)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, args...) //nolint:gosec // binary came from BuildBinary and args are test-owned values
	cmd.Env = realAccountGLabProductEnv(home, glabConfigDir, realHome, realGLabPath, wrapperDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("compiled gitid %s failed: %v\noutput:\n%s", strings.Join(args, " "), err, redactGLabOutputLines(string(out), realHome, 2000))
	}
	return string(out)
}

// runGLabProductPhase — FIRST DECLARED HERE in plan 09.1-02 (same reservation
// as runRealGLabBinary above). It runs, in order: a headless create with
// upload suppressed (so identity creation and registration stay separately
// observable), then register-key, then reads the sandbox public key. It
// deliberately contains NO engine invocation and constructs no
// uploader.Deps — the compiled binary's own wiring, eligibility resolution,
// orchestration, and printing are precisely what this whole phase exists to
// validate (T-09.1-10).
func runGLabProductPhase(t *testing.T, binary, home, glabConfigDir, realHome, realGLabPath, wrapperDir, identityName string, recorder *glabArgvRecorder) (string, string, string) {
	t.Helper()
	runRealGLabBinary(t, home, glabConfigDir, realHome, realGLabPath, wrapperDir, binary, recorder,
		"create", "--name", identityName, "--provider", "gitlab.com",
		"--git-name", "gitid e2e", "--git-email", "gitid-e2e@example.invalid", "--yes", "--no-upload")
	output := runRealGLabBinary(t, home, glabConfigDir, realHome, realGLabPath, wrapperDir, binary, recorder, "register-key", identityName)
	pubPath := filepath.Join(home, ".ssh", "id_ed25519_"+identityName+".pub")
	pub, err := os.ReadFile(pubPath) //nolint:gosec // test-owned sandbox public key
	if err != nil {
		t.Fatalf("reading product-phase sandbox public key: %v", err)
	}
	return output, pubPath, string(pub)
}

// TestRealAccountGitLabUploadRoundTrip is the final test (not a scaffold —
// see the file-level doc comment). Wave 1 (09.1-01) shipped its read-only
// slice: preflight, run-scoping, a baseline inventory read, and the
// read-only final sweep registered in the LIFO position this wave's
// mutation relies on. Plan 09.1-02 (this wave) expands it in place with the
// single real mutation: one disposable key registered through the compiled
// binary, its resource ID resolved by run-scoped inventory lookup, an
// explicit delete after scope re-confirmation, then the wave-1 sweep fires
// last (LIFO) to confirm a clean account state.
func TestRealAccountGitLabUploadRoundTrip(t *testing.T) {
	ambient := glabAmbientEnvMap()
	realHome := ambient["HOME"]
	glabConfigDir := resolveRealGLabConfigDir(ambient)
	if strings.TrimSpace(realHome) == "" || strings.TrimSpace(glabConfigDir) == "" {
		t.Skipf("real-account validation skipped: unable to resolve the ambient HOME or GLAB_CONFIG_DIR; remediate with %q", realAccountGLabScopeRemediation)
	}
	recorder := &glabArgvRecorder{}
	glabPath, redactedEvidence := preflightRealGitLab(t, glabConfigDir, recorder)
	realGLabPath := glabPath
	t.Logf("real-account preflight: glab=%s evidence=%s", glabPath, redactedEvidence)

	runID := newRealAccountGLabRunID(t)
	productScope := "gitid-e2e-" + runID
	if productScope == "" {
		t.Fatal("real-account run scoping string must be non-empty")
	}

	wrapperPath := writeRealGLabWrapper(t)
	wrapperDir := filepath.Dir(wrapperPath)
	home := glabSandboxHome(t)
	deps := realAccountGLabUploaderDeps(home, glabConfigDir, realHome, realGLabPath, wrapperDir, recorder)
	glabPath = wrapperPath
	baselineRecorded := false
	baselineUnscoped := 0

	// Register the read-only sweep first so LIFO cleanup runs outstanding-ID
	// deletion before the sweep, including when a later Fatal aborts the
	// body. Wave 2 (09.1-02) registers the outstanding-ID drain SECOND, the
	// moment its first ID is resolved — this wave has nothing to record
	// into that map yet, so only the sweep is registered here.
	t.Cleanup(func() {
		inventory, err := uploader.Inventory(uploader.ToolGLab, glabPath, deps)
		if err != nil {
			t.Errorf("real-account final read-only sweep could not read inventory: %v", err)
			return
		}
		var remaining []uploader.ExistingKey
		for _, entry := range inventory {
			if strings.Contains(entry.Title, productScope) {
				remaining = append(remaining, entry)
			}
		}
		if len(remaining) != 0 {
			t.Errorf("real-account final read-only sweep found %d remaining run-scoped entries: %#v", len(remaining), remaining)
		}
		if unscopedNow := countUnscopedGLab(inventory, productScope); baselineRecorded && unscopedNow != baselineUnscoped {
			t.Errorf("pre-existing unscoped inventory count changed: before=%d after=%d", baselineUnscoped, unscopedNow)
		}
		t.Logf("real-account final sweep: remaining=%d product-scope=%q", len(remaining), productScope)
	})

	baseline, err := uploader.Inventory(uploader.ToolGLab, glabPath, deps)
	if err != nil {
		t.Fatalf("reading baseline GitLab inventory: %v", err)
	}
	baselineUnscoped = countUnscopedGLab(baseline, productScope)
	baselineRecorded = true
	t.Logf("real-account baseline: unscoped-count=%d run-scope=%q", baselineUnscoped, productScope)

	// The single real mutation this phase exists to perform. Register the
	// outstanding-ID drain's t.Cleanup SECOND, the moment the first ID is
	// resolved, so LIFO runs it before the wave-1 sweep registered above.
	outstanding := &outstandingGLabRemoteKeys{}
	cleanupRegistered := false
	record := func(entry uploader.ExistingKey, scope string) {
		if !strings.Contains(entry.Title, scope) {
			t.Fatalf("refusing to record inventory entry ID %s whose title %q lacks run scope %q", entry.ID, entry.Title, scope)
		}
		outstanding.add(entry, scope)
		t.Logf("resolved resource ID by run-scoped inventory lookup: registration=%s id=%s title=%q", glabRegistrationLabel(entry.Registration), entry.ID, entry.Title)
		if !cleanupRegistered {
			cleanupRegistered = true
			t.Cleanup(func() {
				drainOutstandingGLabRemoteKeys(uploader.ToolGLab, glabPath, deps, outstanding, t.Errorf)
			})
		}
	}

	binary := glabBuildBinary(t)
	productOutput, productPubPath, productPub := runGLabProductPhase(t, binary, home, glabConfigDir, realHome, realGLabPath, wrapperDir, productScope, recorder)
	t.Logf("real-account register-key output (redacted, per-line): %s", redactGLabOutputLines(productOutput, realHome, 2000))
	if !strings.Contains(productOutput, "Running:") {
		t.Fatalf("compiled register-key output did not announce its command: %s", redactGLabOutputLines(productOutput, realHome, 2000))
	}
	// The success row is composed from the product's own constants, never a
	// hand-typed literal — a future change to either constant fails this
	// test instead of silently drifting. register-key's exit status is
	// already the exit-0-only contract runRealGLabBinary enforced above:
	// runRealGLabBinary would have fataled if it were anything else, so
	// reaching this line IS the assertion that it exited 0.
	wantSuccessRow := fmt.Sprintf(tuikit.UploadResultOKFmt, tuikit.UploadRegistrationLabelCombined)
	if !strings.Contains(productOutput, wantSuccessRow) {
		t.Fatalf("compiled register-key output did not contain the GitLab combined success row %q: %s", wantSuccessRow, redactGLabOutputLines(productOutput, realHome, 2000))
	}
	productPrivatePath := strings.TrimSuffix(productPubPath, ".pub")
	recorder.assertNoPath(t, productPrivatePath)

	// Capture the resource ID by a run-scoped inventory lookup — glab ssh-key
	// add prints a human confirmation, not a machine-readable ID; the lookup
	// is how the ID is obtained, used only to learn the ID.
	inventory, err := uploader.Inventory(uploader.ToolGLab, glabPath, deps)
	if err != nil {
		t.Fatalf("reading product-phase inventory for run-scoped ID lookup: %v", err)
	}
	productEntry := glabExactScopedEntry(t, inventory, uploader.RegistrationCombined, productScope)
	record(productEntry, productScope)
	if !uploader.HasRegistration(inventory, productPub, uploader.RegistrationCombined) {
		t.Fatal("product-phase inventory did not confirm the combined registration for the compiled product key")
	}

	// Explicit deletion by recorded ID only, re-confirming scope immediately
	// before each delete. This loop already handles any number of
	// outstanding entries; here it simply iterates once.
	for _, recorded := range outstanding.snapshot() {
		if err := deleteRecordedGLabRemoteKey(uploader.ToolGLab, glabPath, deps, outstanding, recorded); err != nil {
			t.Fatalf("explicit deletion of recorded ID %s failed: %v", recorded.entry.ID, err)
		}
		t.Logf("deleted recorded resource ID after scope re-confirmation: registration=%s id=%s", glabRegistrationLabel(recorded.entry.Registration), recorded.entry.ID)
	}
	// The drain (registered second) fires first and finds an empty map; the
	// wave-1 sweep (registered first) fires last and asserts zero run-scoped
	// entries remain plus an unchanged unscoped count.
}

// --- Cleanup bookkeeping machinery (Task 3) ---
//
// Nothing below this point contacts GitLab — every test in this section is
// account-free and exercises the mechanism against a fake uploader.Deps.
// This machinery is what makes wave 2's single real mutation safe to
// authorize at all: the outstanding-ID map is the single source of truth,
// deletion is by recorded ID after a scope re-confirmation and only through
// uploader.DeleteRecordedKey's guarded entry point, and the drain is
// idempotent. Reproduced from e2e/upload_real_account_e2e_test.go's
// reviewed (Wave 8 R19/R20), tool-agnostic mechanism, under the GLab-marked
// names Task 2 established.

type recordedGLabRemoteKey struct {
	entry uploader.ExistingKey
	scope string
}

// outstandingGLabRemoteKeys is a mutex-protected map — the SINGLE SOURCE OF
// TRUTH for what still needs deleting — keyed by glabRemoteKeyMapID. This is
// what makes the explicit-deletion path and the t.Cleanup drain path
// compose instead of double-deleting (Wave 8's R19 finding).
type outstandingGLabRemoteKeys struct {
	mu      sync.Mutex
	entries map[string]recordedGLabRemoteKey
}

func glabRemoteKeyMapID(entry uploader.ExistingKey) string {
	return fmt.Sprintf("%d:%s", entry.Registration, entry.ID)
}

func (o *outstandingGLabRemoteKeys) add(entry uploader.ExistingKey, scope string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.entries == nil {
		o.entries = make(map[string]recordedGLabRemoteKey)
	}
	o.entries[glabRemoteKeyMapID(entry)] = recordedGLabRemoteKey{entry: entry, scope: scope}
}

func (o *outstandingGLabRemoteKeys) remove(entry uploader.ExistingKey) {
	o.mu.Lock()
	defer o.mu.Unlock()
	delete(o.entries, glabRemoteKeyMapID(entry))
}

func (o *outstandingGLabRemoteKeys) snapshot() []recordedGLabRemoteKey {
	o.mu.Lock()
	defer o.mu.Unlock()
	out := make([]recordedGLabRemoteKey, 0, len(o.entries))
	for _, entry := range o.entries {
		out = append(out, entry)
	}
	return out
}

func findRecordedGLabInventoryEntry(entries []uploader.ExistingKey, recorded recordedGLabRemoteKey) (uploader.ExistingKey, bool) {
	for _, entry := range entries {
		if entry.ID == recorded.entry.ID && entry.Registration == recorded.entry.Registration {
			return entry, true
		}
	}
	return uploader.ExistingKey{}, false
}

// isGLabAlreadyAbsent reports an already-absent delete response as success,
// not failure.
//
// KNOWN UNVERIFIED GAP (review cycle 1, LOW). These two markers were ported
// from the GitHub sibling (e2e/upload_real_account_e2e_test.go's
// isAlreadyAbsent), which chose them against GitHub's own error wording.
// Nobody has ever seen GitLab's actual wording for "delete a stale/missing
// SSH key ID" — this gap is NOT closed by this plan. FAIL-CLOSED DIRECTION:
// a GitLab wording this predicate does not recognise is classified as a
// REAL deletion failure, surfaced by the drain's reporter and by the final
// read-only sweep, never silently swallowed. Do not widen the matching by
// guessing additional GitLab-specific phrasings here; 09.1-02 Task 2
// captures the real wording if a live run ever produces one, and closing
// this gap is a follow-up with evidence, not a plan-time guess.
func isGLabAlreadyAbsent(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "404") || strings.Contains(text, "not found")
}

// deleteRecordedGLabRemoteKey re-reads the inventory, re-confirms the
// recorded ID still carries its recorded scope, and only then deletes
// through uploader.DeleteRecordedKey — never by title alone, never by a
// broad query.
func deleteRecordedGLabRemoteKey(tool uploader.Tool, toolPath string, deps uploader.Deps, outstanding *outstandingGLabRemoteKeys, recorded recordedGLabRemoteKey) error {
	inventory, err := uploader.Inventory(tool, toolPath, deps)
	if err != nil {
		return fmt.Errorf("re-reading inventory before deleting recorded ID %s: %w", recorded.entry.ID, err)
	}
	current, found := findRecordedGLabInventoryEntry(inventory, recorded)
	if !found {
		outstanding.remove(recorded.entry)
		return nil
	}
	if !strings.Contains(current.Title, recorded.scope) {
		return fmt.Errorf("refusing to delete recorded ID %s: current title %q no longer carries run scope %q", current.ID, current.Title, recorded.scope)
	}
	if _, err := uploader.DeleteRecordedKey(tool, toolPath, current, deps); err != nil {
		if isGLabAlreadyAbsent(err) {
			outstanding.remove(recorded.entry)
			return nil
		}
		return fmt.Errorf("deleting recorded ID %s after scope re-confirmation: %w", current.ID, err)
	}
	outstanding.remove(recorded.entry)
	return nil
}

// drainOutstandingGLabRemoteKeys iterates ONLY over IDs still present in
// the map, reporting failures through report rather than fataling.
func drainOutstandingGLabRemoteKeys(tool uploader.Tool, toolPath string, deps uploader.Deps, outstanding *outstandingGLabRemoteKeys, report func(string, ...any)) {
	for _, recorded := range outstanding.snapshot() {
		if err := deleteRecordedGLabRemoteKey(tool, toolPath, deps, outstanding, recorded); err != nil {
			report("real-account cleanup failed for recorded ID %s: %v", recorded.entry.ID, err)
		}
	}
}

func glabEntriesWithScope(entries []uploader.ExistingKey, registration uploader.Registration, scope string) []uploader.ExistingKey {
	var matches []uploader.ExistingKey
	for _, entry := range entries {
		if entry.Registration == registration && strings.Contains(entry.Title, scope) {
			matches = append(matches, entry)
		}
	}
	return matches
}

// glabExactScopedEntry fatals unless EXACTLY one entry matches — the guard
// against acting on a same-titled stranger.
func glabExactScopedEntry(t *testing.T, entries []uploader.ExistingKey, registration uploader.Registration, scope string) uploader.ExistingKey {
	t.Helper()
	matches := glabEntriesWithScope(entries, registration, scope)
	if len(matches) != 1 {
		t.Fatalf("run-scoped inventory lookup for %s scope %q found %d entries, want exactly 1: %#v", glabRegistrationLabel(registration), scope, len(matches), matches)
	}
	return matches[0]
}

// glabRegistrationLabel returns "combined" for GitLab's only registration
// shape. Unlike the GitHub sibling's registrationLabel, this deliberately
// has no signing/authentication branch — GitLab has exactly one
// registration type (D-01/D-02).
func glabRegistrationLabel(registration uploader.Registration) string {
	if registration == uploader.RegistrationCombined {
		return "combined"
	}
	return "unknown"
}

// --- Account-free test doubles ---

// fakeGLabInventoryRecord mirrors internal/uploader/inventory.go's
// unexported providerKey wire shape (id/title/key) so fakeGLabCleanupDeps
// can produce JSON the production glabInventory/decodeProviderKeyPages path
// decodes exactly as it would decode real `glab ssh-key list -F json`
// output. ID is json.Number (not string) because GitLab's real API returns
// numeric IDs as unquoted JSON numbers, matching providerKey's own field
// type — confirmed this session that json.Marshal emits json.Number
// unquoted, exactly the shape decodeProviderKeyPages expects.
type fakeGLabInventoryRecord struct {
	ID    json.Number `json:"id"`
	Title string      `json:"title"`
	Key   string      `json:"key"`
}

func fakeGLabInventoryEntries(entries []uploader.ExistingKey) []fakeGLabInventoryRecord {
	records := make([]fakeGLabInventoryRecord, 0, len(entries))
	for _, entry := range entries {
		records = append(records, fakeGLabInventoryRecord{ID: json.Number(entry.ID), Title: entry.Title, Key: entry.Key})
	}
	return records
}

// fakeGLabCleanupDeps returns an account-free uploader.Deps whose RunCmd
// answers `ssh-key list` with entries (always the same fixed set — this
// double tests bookkeeping ordering/idempotency, not inventory mutation)
// and `ssh-key delete <id>` by incrementing deleteCalls[id]. If id equals
// alreadyAbsentID, the delete reports a 404-shaped error so
// isGLabAlreadyAbsent's branch is exercised through the real drain path,
// not asserted in isolation.
func fakeGLabCleanupDeps(t *testing.T, entries []uploader.ExistingKey, deleteCalls map[string]int, alreadyAbsentID string) uploader.Deps {
	t.Helper()
	var mu sync.Mutex
	return uploader.Deps{
		LookPath: func(name string) (string, error) { return "/fake/glab", nil },
		ReadFile: os.ReadFile,
		RunCmd: func(_ string, args ...string) (string, int, error) {
			if len(args) >= 2 && args[0] == "ssh-key" && args[1] == "list" {
				payload, err := json.Marshal(fakeGLabInventoryEntries(entries))
				if err != nil {
					t.Fatalf("marshaling fake GitLab inventory: %v", err)
				}
				return string(payload), 0, nil
			}
			if len(args) >= 3 && args[0] == "ssh-key" && args[1] == "delete" {
				id := args[2]
				mu.Lock()
				deleteCalls[id]++
				mu.Unlock()
				if alreadyAbsentID != "" && id == alreadyAbsentID {
					return "", 1, fmt.Errorf("glab: 404 Not Found")
				}
				return "", 0, nil
			}
			return "", 1, fmt.Errorf("fake glab: unexpected argv %v", args)
		},
	}
}

// TestRealAccountGitLabResolvesRealGLabConfigDir table-tests
// resolveRealGLabConfigDir's three-way priority order plus the empty-value
// fall-through, confirmed against this session's live macOS capture
// (09.1-01-SUMMARY.md Deviations).
func TestRealAccountGitLabResolvesRealGLabConfigDir(t *testing.T) {
	wantDefault := filepath.Join("/home/test", ".config", "glab-cli")
	if runtime.GOOS == "darwin" {
		wantDefault = filepath.Join("/home/test", "Library", "Application Support", "glab-cli")
	}
	tests := []struct {
		name string
		env  map[string]string
		want string
	}{
		{name: "GLAB_CONFIG_DIR wins", env: map[string]string{"GLAB_CONFIG_DIR": "/credential/glab", "XDG_CONFIG_HOME": "/xdg", "HOME": "/home/test"}, want: "/credential/glab"},
		{name: "XDG_CONFIG_HOME second", env: map[string]string{"XDG_CONFIG_HOME": "/xdg", "HOME": "/home/test"}, want: filepath.Join("/xdg", "glab-cli")},
		{name: "HOME fallback uses the OS-native default", env: map[string]string{"HOME": "/home/test"}, want: wantDefault},
		{name: "empty GLAB_CONFIG_DIR falls through to XDG_CONFIG_HOME", env: map[string]string{"GLAB_CONFIG_DIR": "  ", "XDG_CONFIG_HOME": "/xdg", "HOME": "/home/test"}, want: filepath.Join("/xdg", "glab-cli")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := resolveRealGLabConfigDir(test.env); got != test.want {
				t.Fatalf("resolveRealGLabConfigDir() = %q, want %q", got, test.want)
			}
		})
	}
}

// TestRealAccountGitLabScopeCheckRequiresExactMember drives the SAME
// parseGLabTokenScopes/missingRequiredGLabScopes pair the live preflight
// calls (review cycle 1, HIGH-2's noise/liveness cases), against synthetic
// personal_access_tokens/self JSON responses.
func TestRealAccountGitLabScopeCheckRequiresExactMember(t *testing.T) {
	tests := []struct {
		name   string
		output string
		accept bool
	}{
		{name: "exact api member alone", output: `{"scopes": ["api"]}`, accept: true},
		{name: "exact api member alongside others", output: `{"scopes": ["read_user", "api", "read_repository"]}`, accept: true},
		{name: "unknown extra attributes are ignored by the three-field struct", output: `{"id": 42, "name": "gitid-e2e", "scopes": ["api"], "created_at": "2024-01-01T00:00:00Z", "user_id": 7, "expires_at": null, "granular_scopes": null}`, accept: true},
		{name: "leading banner line before the JSON object", output: "A new version of glab is available!\n" + `{"scopes": ["api"]}`, accept: true},
		{name: "trailing bytes after the JSON object", output: `{"scopes": ["api"]}` + "\nsome trailing banner text", accept: true},
		{name: "omitted revoked and active attributes accepted when scopes are sufficient", output: `{"scopes": ["api"]}`, accept: true},
		{name: "substring-only scope is rejected, not accepted", output: `{"scopes": ["apiv2"]}`, accept: false},
		{name: "missing api entirely", output: `{"scopes": ["read_api"]}`, accept: false},
		{name: "empty scopes array", output: `{"scopes": []}`, accept: false},
		{name: "no scopes attribute at all", output: `{"revoked": false, "active": true}`, accept: false},
		{name: "top-level JSON array instead of an object", output: `[{"scopes": ["api"]}]`, accept: false},
		{name: "output is not valid JSON at all", output: `not json at all`, accept: false},
		{name: "revoked token is rejected", output: `{"scopes": ["api"], "revoked": true}`, accept: false},
		{name: "inactive token is rejected", output: `{"scopes": ["api"], "active": false}`, accept: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			scopes, ok := parseGLabTokenScopes(test.output)
			accepted := ok && len(missingRequiredGLabScopes(scopes)) == 0
			if accepted != test.accept {
				t.Fatalf("parseGLabTokenScopes(%q) accepted=%t, want %t (scopes=%v ok=%t)", test.output, accepted, test.accept, scopes, ok)
			}
		})
	}
}

// TestRealAccountGitLabCleanupBookkeepingIsIdempotent proves, with a fake
// deleter counting calls per ID, that a fully-explicit-deletion path leaves
// the outstanding map empty (the drain performs ZERO deletions), an
// early-failure path leaves exactly the undeleted IDs for the drain (each
// deleted exactly once), and a deleter reporting the entry already absent
// removes the ID without being treated as an error.
func TestRealAccountGitLabCleanupBookkeepingIsIdempotent(t *testing.T) {
	first := uploader.ExistingKey{ID: "101", Registration: uploader.RegistrationCombined, Title: "gitid: gitid-e2e-test @ host"}
	second := uploader.ExistingKey{ID: "202", Registration: uploader.RegistrationCombined, Title: "gitid: gitid-e2e-test @ host"}
	entries := []uploader.ExistingKey{first, second}

	for _, test := range []struct {
		name            string
		explicitFirst   bool
		explicitSecond  bool
		alreadyAbsentID string
		wantDrainCalls  map[string]int
	}{
		{
			name:           "fully explicit deletion path performs zero drain deletions",
			explicitFirst:  true,
			explicitSecond: true,
			wantDrainCalls: map[string]int{},
		},
		{
			name:           "early-failure path leaves remaining IDs, each deleted exactly once by the drain",
			explicitFirst:  true,
			wantDrainCalls: map[string]int{"202": 1},
		},
		{
			name:            "already-absent report during the drain removes the ID and is not an error",
			explicitFirst:   true,
			alreadyAbsentID: "202",
			wantDrainCalls:  map[string]int{"202": 1},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			outstanding := &outstandingGLabRemoteKeys{}
			outstanding.add(first, "gitid-e2e-test")
			outstanding.add(second, "gitid-e2e-test")

			explicitCalls := map[string]int{}
			explicitDeps := fakeGLabCleanupDeps(t, entries, explicitCalls, "")
			if test.explicitFirst {
				if err := deleteRecordedGLabRemoteKey(uploader.ToolGLab, "/fake/glab", explicitDeps, outstanding, recordedGLabRemoteKey{entry: first, scope: "gitid-e2e-test"}); err != nil {
					t.Fatalf("explicit delete of first failed: %v", err)
				}
			}
			if test.explicitSecond {
				if err := deleteRecordedGLabRemoteKey(uploader.ToolGLab, "/fake/glab", explicitDeps, outstanding, recordedGLabRemoteKey{entry: second, scope: "gitid-e2e-test"}); err != nil {
					t.Fatalf("explicit delete of second failed: %v", err)
				}
			}

			drainCalls := map[string]int{}
			reportedErrors := 0
			drainDeps := fakeGLabCleanupDeps(t, entries, drainCalls, test.alreadyAbsentID)
			drainOutstandingGLabRemoteKeys(uploader.ToolGLab, "/fake/glab", drainDeps, outstanding, func(string, ...any) { reportedErrors++ })

			if reportedErrors != 0 {
				t.Fatalf("drain reported %d unexpected error(s)", reportedErrors)
			}
			if len(drainCalls) != len(test.wantDrainCalls) {
				t.Fatalf("drain delete call set = %v, want %v", drainCalls, test.wantDrainCalls)
			}
			for id, want := range test.wantDrainCalls {
				if got := drainCalls[id]; got != want {
					t.Fatalf("drain delete calls for id %s = %d, want %d", id, got, want)
				}
			}
			if got := len(outstanding.snapshot()); got != 0 {
				t.Fatalf("outstanding map not empty after drain: %d entries remain", got)
			}
		})
	}
}

// TestRealAccountGitLabCleanupOrderingIsLIFOSweepLast asserts the LIFO
// t.Cleanup ordering directly (registration order, not prose): the
// outstanding-ID closure's marker must appear before the sweep's marker in
// the recorded execution slice.
func TestRealAccountGitLabCleanupOrderingIsLIFOSweepLast(t *testing.T) {
	var order []string
	passed := t.Run("ordering", func(t *testing.T) {
		t.Cleanup(func() { order = append(order, "sweep") })
		t.Cleanup(func() { order = append(order, "outstanding") })
	})
	if !passed {
		t.Fatal("cleanup ordering subtest failed")
	}
	if got, want := strings.Join(order, ","), "outstanding,sweep"; got != want {
		t.Fatalf("cleanup execution order = %q, want %q", got, want)
	}
}

// TestRedactGLabOutputLinesRedactsEveryLine proves the fix for the logging
// defect this task's own evidence review caught: uploader.RedactCLIOutput
// alone returns only the FIRST non-empty line of whatever string it is
// given, so a single call against a multi-line register-key output would
// silently drop everything after the "Running: ..." announcement line —
// including the actual success row. This test drives the same real,
// two-line shape the real run produced (redacted here, not the real
// account's title/path), confirming every line survives and is
// independently redacted.
func TestRedactGLabOutputLinesRedactsEveryLine(t *testing.T) {
	raw := "Running: /home/dev/bin/glab ssh-key add /home/dev/.ssh/id_ed25519_gitid-e2e-x.pub -t 'gitid: gitid-e2e-x @ host' --usage-type auth_and_signing\n\n✓ Key key registered\n"
	got := redactGLabOutputLines(raw, "/home/dev", 240)
	if !strings.Contains(got, "~/bin/glab") {
		t.Fatalf("redactGLabOutputLines did not redact the home path in the Running line: %q", got)
	}
	if !strings.Contains(got, "✓ Key key registered") {
		t.Fatalf("redactGLabOutputLines dropped the success row (the exact defect this test guards against): %q", got)
	}
	if strings.Contains(got, "\n") {
		t.Fatalf("redactGLabOutputLines must join lines with a separator, never a raw newline: %q", got)
	}
}
