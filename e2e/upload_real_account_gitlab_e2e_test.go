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
// contract. It trims everything before the first '{' (tolerating a leading
// banner line that reached stdout anyway) and decodes with
// json.NewDecoder(...).Decode, not json.Unmarshal, so trailing bytes after
// the JSON object do not fail the parse (Decode reads exactly one JSON
// value and ignores what follows; Unmarshal would reject the trailing
// bytes). Returns ok=false when: the decode errors (including a top-level
// JSON array, which cannot decode into this object contract); the scopes
// slice is nil or empty; Revoked is non-nil and true; or Active is non-nil
// and false. An object that OMITS revoked/active entirely is accepted (when
// scopes are sufficient) — this is exactly the case the pointer-typed
// fields exist for.
func parseGLabTokenScopes(output string) ([]string, bool) {
	idx := strings.IndexByte(output, '{')
	if idx < 0 {
		return nil, false
	}
	var v glabTokenSelf
	if err := json.NewDecoder(strings.NewReader(output[idx:])).Decode(&v); err != nil {
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
// in 09.1-01-SUMMARY.md Deviations). BuildBinary is NOT duplicated here:
// wave 1 declares no compiled-binary driver — the compiled-binary driver
// plan 09.1-02 adds is reserved and out of scope for this wave — so it is
// never needed in this file.
func glabSandboxHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	return home
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
			if exitErr, ok := err.(*exec.ExitError); ok {
				return string(out), exitErr.ExitCode(), fmt.Errorf("%w: %s", err, uploader.RedactCLIOutput(string(out), realHome, 240))
			}
			return string(out), 2, err
		},
	}
}

// TestRealAccountGitLabUploadRoundTrip is this wave's READ-ONLY slice,
// which plan 09.1-02 expands in place (it is the final test, not a
// scaffold — see the file-level doc comment). It performs zero mutation:
// preflight, run-scoping, a baseline inventory read, and a read-only final
// sweep registered in the LIFO position wave 2's mutation will rely on.
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
		if baselineRecorded && countUnscopedGLab(inventory, productScope) != baselineUnscoped {
			t.Errorf("pre-existing unscoped inventory count changed: before=%d after=%d", baselineUnscoped, countUnscopedGLab(inventory, productScope))
		}
		t.Logf("real-account final sweep: remaining=%d product-scope=%q", len(remaining), productScope)
	})

	baseline, err := uploader.Inventory(uploader.ToolGLab, glabPath, deps)
	if err != nil {
		t.Fatalf("reading baseline GitLab inventory: %v", err)
	}
	baselineUnscoped = countUnscopedGLab(baseline, productScope)
	baselineRecorded = true
	t.Logf("real-account baseline: unscoped-count=%d run-scope=%q (this wave performs no mutation)", baselineUnscoped, productScope)
}
