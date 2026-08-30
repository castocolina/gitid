//go:build realaccount

package e2e

// upload_real_account_e2e_test.go implements ONESHOT.md's Phase 9 External
// Account Policy. It is the repository's only real-account surface and is
// opt-in through the distinct realaccount build tag. It deliberately does not
// use e2eEnv: every normal e2e child must resolve a test-owned provider shim,
// while this narrowly-scoped validation must reach the developer's already
// authenticated GitHub CLI. Task 1 authorized the Phase P naming deviation:
// Phase P scopes the compiled product path through its disposable identity
// name because the shipped D-07 title format is frozen; Phase N uses the
// policy's literal per-registration titles.

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/castocolina/gitid/internal/keygen"
	"github.com/castocolina/gitid/internal/uploader"
)

const realAccountScopeRemediation = "gh auth refresh -h github.com -s admin:public_key -s admin:ssh_signing_key"

var requiredRealAccountScopes = []string{"admin:public_key", "admin:ssh_signing_key"}

type recordedArgv struct {
	name string
	args []string
}

type argvRecorder struct {
	mu    sync.Mutex
	calls []recordedArgv
}

func (r *argvRecorder) add(name string, args ...string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, recordedArgv{name: name, args: append([]string(nil), args...)})
}

func (r *argvRecorder) assertNoPath(t *testing.T, path string) {
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

// resolveRealGHConfigDir preserves gh's authenticated config while product
// children use a sandbox HOME. GH_CONFIG_DIR is already the gh directory;
// XDG_CONFIG_HOME needs its gh child.
func resolveRealGHConfigDir(env map[string]string) string {
	if dir := strings.TrimSpace(env["GH_CONFIG_DIR"]); dir != "" {
		return dir
	}
	if dir := strings.TrimSpace(env["XDG_CONFIG_HOME"]); dir != "" {
		return filepath.Join(dir, "gh")
	}
	return filepath.Join(env["HOME"], ".config", "gh")
}

func ambientEnvMap() map[string]string {
	values := make(map[string]string)
	for _, entry := range os.Environ() {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			values[key] = value
		}
	}
	return values
}

func realAccountChildEnv(home, ghConfigDir string) []string {
	return append(os.Environ(), "HOME="+home, "GH_CONFIG_DIR="+ghConfigDir)
}

func realAccountProductEnv(home, ghConfigDir, realHome, realGHPath, wrapperDir string) []string {
	path := strings.Join([]string{wrapperDir, os.Getenv("PATH")}, string(os.PathListSeparator))
	return append(realAccountChildEnv(home, ghConfigDir),
		"PATH="+path,
		"GITID_REAL_HOME="+realHome,
		"GITID_REAL_GH_CONFIG_DIR="+ghConfigDir,
		"GITID_REAL_GH="+realGHPath,
	)
}

func writeRealGHWrapper(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "gh")
	const script = "#!/bin/sh\n" +
		"HOME=\"$GITID_REAL_HOME\"\n" +
		"export HOME\n" +
		"GH_CONFIG_DIR=\"$GITID_REAL_GH_CONFIG_DIR\"\n" +
		"export GH_CONFIG_DIR\n" +
		"exec \"$GITID_REAL_GH\" \"$@\"\n"
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil { //nolint:gosec // static test-owned wrapper
		t.Fatalf("writing real-gh HOME bridge: %v", err)
	}
	return path
}

func parseGitHubScopeLine(output string) (map[string]bool, string, bool) {
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		idx := strings.Index(trimmed, "Token scopes:")
		if idx < 0 {
			continue
		}
		scopeLine := strings.TrimSpace(trimmed[idx:])
		scopes := make(map[string]bool)
		for _, raw := range strings.Split(strings.TrimPrefix(scopeLine, "Token scopes:"), ",") {
			scope := strings.Trim(strings.TrimSpace(raw), "'\"")
			if scope != "" {
				scopes[scope] = true
			}
		}
		return scopes, scopeLine, true
	}
	return nil, "", false
}

func missingRequiredScopes(scopes map[string]bool) []string {
	var missing []string
	for _, required := range requiredRealAccountScopes {
		if !scopes[required] {
			missing = append(missing, required)
		}
	}
	return missing
}

func preflightRealGitHub(t *testing.T, ghConfigDir string, recorder *argvRecorder) (string, string) {
	t.Helper()
	ghPath, err := exec.LookPath("gh")
	if err != nil {
		t.Skipf("real-account validation skipped: gh is not on PATH; install gh, authenticate, then run %q", realAccountScopeRemediation)
	}

	args := []string{"auth", "status", "--hostname", "github.com"}
	for _, arg := range args {
		if strings.Contains(strings.ToLower(arg), "token") {
			t.Fatalf("preflight argv must not contain a token-revealing flag: %q", args)
		}
	}
	recorder.add(ghPath, args...)
	cmd := exec.Command(ghPath, args...) //nolint:gosec // ghPath came from exec.LookPath and args are fixed literals
	cmd.Env = realAccountChildEnv(os.Getenv("HOME"), ghConfigDir)
	out, runErr := cmd.CombinedOutput()
	if runErr != nil {
		t.Skipf("real-account validation skipped: gh auth status --hostname github.com did not report an authenticated session: %s; remediate with %q", uploader.RedactCLIOutput(string(out), os.Getenv("HOME"), 240), realAccountScopeRemediation)
	}

	scopes, scopeLine, found := parseGitHubScopeLine(string(out))
	if !found {
		t.Skipf("real-account validation skipped: gh auth status --hostname github.com did not report a Token scopes line; remediate with %q", realAccountScopeRemediation)
	}
	if missing := missingRequiredScopes(scopes); len(missing) > 0 {
		t.Skipf("real-account validation skipped: gh token is missing exact required scope(s) %s; remediate with %q", strings.Join(missing, ", "), realAccountScopeRemediation)
	}
	return ghPath, uploader.RedactCLIOutput(scopeLine, os.Getenv("HOME"), 240)
}

func newRealAccountRunID(t *testing.T) string {
	t.Helper()
	var suffix [4]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		t.Fatalf("generating run suffix: %v", err)
	}
	return fmt.Sprintf("%s-%s", time.Now().UTC().Format("20060102t150405"), hex.EncodeToString(suffix[:]))
}

type recordedRemoteKey struct {
	entry uploader.ExistingKey
	scope string
}

type outstandingRemoteKeys struct {
	mu      sync.Mutex
	entries map[string]recordedRemoteKey
}

func remoteKeyMapID(entry uploader.ExistingKey) string {
	return fmt.Sprintf("%d:%s", entry.Registration, entry.ID)
}

func (o *outstandingRemoteKeys) add(entry uploader.ExistingKey, scope string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.entries == nil {
		o.entries = make(map[string]recordedRemoteKey)
	}
	o.entries[remoteKeyMapID(entry)] = recordedRemoteKey{entry: entry, scope: scope}
}

func (o *outstandingRemoteKeys) remove(entry uploader.ExistingKey) {
	o.mu.Lock()
	defer o.mu.Unlock()
	delete(o.entries, remoteKeyMapID(entry))
}

func (o *outstandingRemoteKeys) snapshot() []recordedRemoteKey {
	o.mu.Lock()
	defer o.mu.Unlock()
	out := make([]recordedRemoteKey, 0, len(o.entries))
	for _, entry := range o.entries {
		out = append(out, entry)
	}
	return out
}

func findRecordedInventoryEntry(entries []uploader.ExistingKey, recorded recordedRemoteKey) (uploader.ExistingKey, bool) {
	for _, entry := range entries {
		if entry.ID == recorded.entry.ID && entry.Registration == recorded.entry.Registration {
			return entry, true
		}
	}
	return uploader.ExistingKey{}, false
}

func isAlreadyAbsent(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "404") || strings.Contains(text, "not found")
}

func deleteRecordedRemoteKey(tool uploader.Tool, toolPath string, deps uploader.Deps, outstanding *outstandingRemoteKeys, recorded recordedRemoteKey) error {
	inventory, err := uploader.Inventory(tool, toolPath, deps)
	if err != nil {
		return fmt.Errorf("re-reading inventory before deleting recorded ID %s: %w", recorded.entry.ID, err)
	}
	current, found := findRecordedInventoryEntry(inventory, recorded)
	if !found {
		outstanding.remove(recorded.entry)
		return nil
	}
	if !strings.Contains(current.Title, recorded.scope) {
		return fmt.Errorf("refusing to delete recorded ID %s: current title %q no longer carries run scope %q", current.ID, current.Title, recorded.scope)
	}
	if _, err := uploader.DeleteRecordedKey(tool, toolPath, current, deps); err != nil {
		if isAlreadyAbsent(err) {
			outstanding.remove(recorded.entry)
			return nil
		}
		return fmt.Errorf("deleting recorded ID %s after scope re-confirmation: %w", current.ID, err)
	}
	outstanding.remove(recorded.entry)
	return nil
}

func drainOutstandingRemoteKeys(tool uploader.Tool, toolPath string, deps uploader.Deps, outstanding *outstandingRemoteKeys, report func(string, ...any)) {
	for _, recorded := range outstanding.snapshot() {
		if err := deleteRecordedRemoteKey(tool, toolPath, deps, outstanding, recorded); err != nil {
			report("real-account cleanup failed for recorded ID %s: %v", recorded.entry.ID, err)
		}
	}
}

func countUnscoped(entries []uploader.ExistingKey, scopes ...string) int {
	count := 0
	for _, entry := range entries {
		matched := false
		for _, scope := range scopes {
			if strings.Contains(entry.Title, scope) {
				matched = true
				break
			}
		}
		if !matched {
			count++
		}
	}
	return count
}

func entriesWithScope(entries []uploader.ExistingKey, registration uploader.Registration, scope string) []uploader.ExistingKey {
	var matches []uploader.ExistingKey
	for _, entry := range entries {
		if entry.Registration == registration && strings.Contains(entry.Title, scope) {
			matches = append(matches, entry)
		}
	}
	return matches
}

func exactScopedEntry(t *testing.T, entries []uploader.ExistingKey, registration uploader.Registration, scope string) uploader.ExistingKey {
	t.Helper()
	matches := entriesWithScope(entries, registration, scope)
	if len(matches) != 1 {
		t.Fatalf("run-scoped inventory lookup for %s scope %q found %d entries, want exactly 1: %#v", registrationLabel(registration), scope, len(matches), matches)
	}
	return matches[0]
}

func registrationLabel(registration uploader.Registration) string {
	if registration == uploader.RegistrationSigning {
		return "signing"
	}
	return "authentication"
}

func runRealBinary(t *testing.T, home, ghConfigDir, realHome, realGHPath, wrapperDir, binary string, recorder *argvRecorder, args ...string) string {
	t.Helper()
	recorder.add(binary, args...)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, args...) //nolint:gosec // binary came from BuildBinary and args are test-owned values
	cmd.Env = realAccountProductEnv(home, ghConfigDir, realHome, realGHPath, wrapperDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("compiled gitid %s failed: %v\noutput:\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

// runProductPhase deliberately contains no uploader engine invocation or deps
// construction: it drives the compiled gitid binary that a user invokes.
func runProductPhase(t *testing.T, binary, home, ghConfigDir, realHome, realGHPath, wrapperDir, identityName string, recorder *argvRecorder) (string, string, string) {
	t.Helper()
	runRealBinary(t, home, ghConfigDir, realHome, realGHPath, wrapperDir, binary, recorder,
		"create", "--name", identityName, "--provider", "github.com",
		"--git-name", "gitid e2e", "--git-email", "gitid-e2e@example.invalid", "--yes", "--no-upload")
	output := runRealBinary(t, home, ghConfigDir, realHome, realGHPath, wrapperDir, binary, recorder, "register-key", identityName)
	pubPath := filepath.Join(home, ".ssh", "id_ed25519_"+identityName+".pub")
	pub, err := os.ReadFile(pubPath) //nolint:gosec // test-owned sandbox public key
	if err != nil {
		t.Fatalf("reading Phase P sandbox public key: %v", err)
	}
	return output, pubPath, string(pub)
}

func realAccountUploaderDeps(home, ghConfigDir, realHome, realGHPath, wrapperDir string, recorder *argvRecorder) uploader.Deps {
	return uploader.Deps{
		LookPath: func(name string) (string, error) {
			if name == "gh" {
				return filepath.Join(wrapperDir, "gh"), nil
			}
			return exec.LookPath(name)
		},
		ReadFile: os.ReadFile,
		RunCmd: func(name string, args ...string) (string, int, error) {
			recorder.add(name, args...)
			ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec // name resolves the test-owned gh HOME bridge and args are uploader-controlled
			cmd.Env = realAccountProductEnv(home, ghConfigDir, realHome, realGHPath, wrapperDir)
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

func TestRealAccountGitHubUploadRoundTrip(t *testing.T) {
	ambient := ambientEnvMap()
	realHome := ambient["HOME"]
	ghConfigDir := resolveRealGHConfigDir(ambient)
	if strings.TrimSpace(realHome) == "" || strings.TrimSpace(ghConfigDir) == "" {
		t.Skipf("real-account validation skipped: unable to resolve the ambient HOME or GH_CONFIG_DIR; remediate with %q", realAccountScopeRemediation)
	}
	recorder := &argvRecorder{}
	ghPath, redactedScopeLine := preflightRealGitHub(t, ghConfigDir, recorder)
	realGHPath := ghPath
	wrapperPath := writeRealGHWrapper(t)
	wrapperDir := filepath.Dir(wrapperPath)
	t.Logf("real-account preflight: gh=%s scope-line=%s", ghPath, redactedScopeLine)

	runID := newRealAccountRunID(t)
	productScope := "gitid-e2e-" + runID
	policyPrefix := "gitid-e2e:" + runID + ":"
	if productScope == "" || policyPrefix == "" {
		t.Fatal("real-account run scoping strings must be non-empty")
	}

	home := SandboxHome(t)
	deps := realAccountUploaderDeps(home, ghConfigDir, realHome, realGHPath, wrapperDir, recorder)
	ghPath = wrapperPath
	outstanding := &outstandingRemoteKeys{}
	baselineRecorded := false
	baselineUnscoped := 0

	// Register the read-only sweep first so LIFO cleanup runs outstanding-ID
	// deletion before the sweep, including when a later Fatal aborts the body.
	t.Cleanup(func() {
		inventory, err := uploader.Inventory(uploader.ToolGH, ghPath, deps)
		if err != nil {
			t.Errorf("real-account final read-only sweep could not read inventory: %v", err)
			return
		}
		var remaining []uploader.ExistingKey
		for _, entry := range inventory {
			if strings.Contains(entry.Title, productScope) || strings.Contains(entry.Title, policyPrefix) {
				remaining = append(remaining, entry)
			}
		}
		if len(remaining) != 0 {
			t.Errorf("real-account final read-only sweep found %d remaining run-scoped entries: %#v", len(remaining), remaining)
		}
		if baselineRecorded && countUnscoped(inventory, productScope, policyPrefix) != baselineUnscoped {
			t.Errorf("pre-existing unscoped inventory count changed: before=%d after=%d", baselineUnscoped, countUnscoped(inventory, productScope, policyPrefix))
		}
		t.Logf("real-account final sweep: remaining=%d product-scope=%q policy-prefix=%q", len(remaining), productScope, policyPrefix)
	})

	baseline, err := uploader.Inventory(uploader.ToolGH, ghPath, deps)
	if err != nil {
		t.Fatalf("reading baseline GitHub inventory: %v", err)
	}
	baselineUnscoped = countUnscoped(baseline, productScope, policyPrefix)
	baselineRecorded = true

	cleanupRegistered := false
	record := func(entry uploader.ExistingKey, scope string) {
		if !strings.Contains(entry.Title, scope) {
			t.Fatalf("refusing to record inventory entry ID %s whose title %q lacks run scope %q", entry.ID, entry.Title, scope)
		}
		outstanding.add(entry, scope)
		t.Logf("resolved resource ID by run-scoped inventory lookup: registration=%s id=%s title=%q", registrationLabel(entry.Registration), entry.ID, entry.Title)
		if !cleanupRegistered {
			cleanupRegistered = true
			t.Cleanup(func() {
				drainOutstandingRemoteKeys(uploader.ToolGH, ghPath, deps, outstanding, t.Errorf)
			})
		}
	}

	binary := BuildBinary(t)
	productOutput, productPubPath, productPub := runProductPhase(t, binary, home, ghConfigDir, realHome, realGHPath, wrapperDir, productScope, recorder)
	if !strings.Contains(productOutput, "Running:") {
		t.Fatalf("compiled register-key output did not announce its command: %s", productOutput)
	}
	if !strings.Contains(productOutput, "Authentication key registered") || !strings.Contains(productOutput, "Signing key registered") {
		t.Fatalf("compiled register-key output did not contain a successful result row for each registration: %s", productOutput)
	}
	productPrivatePath := strings.TrimSuffix(productPubPath, ".pub")
	recorder.assertNoPath(t, productPrivatePath)

	inventory, err := uploader.Inventory(uploader.ToolGH, ghPath, deps)
	if err != nil {
		t.Fatalf("reading Phase P inventory for run-scoped ID lookup: %v", err)
	}
	productAuth := exactScopedEntry(t, inventory, uploader.RegistrationAuthentication, productScope)
	record(productAuth, productScope)
	productSigning := exactScopedEntry(t, inventory, uploader.RegistrationSigning, productScope)
	record(productSigning, productScope)
	if !uploader.HasRegistration(inventory, productPub, uploader.RegistrationAuthentication) || !uploader.HasRegistration(inventory, productPub, uploader.RegistrationSigning) {
		t.Fatal("Phase P inventory did not confirm both authentication and signing registrations for the compiled product key")
	}

	phaseNDir := t.TempDir()
	phaseNPrivatePath := filepath.Join(phaseNDir, "id_ed25519_"+runID)
	phaseNPublicPath := phaseNPrivatePath + ".pub"
	material, err := keygen.GenerateMaterial(keygen.Params{Algo: "ed25519", Identity: "gitid-e2e-" + runID, Comment: "gitid-e2e@local"})
	if err != nil {
		t.Fatalf("generating Phase N disposable ed25519 key: %v", err)
	}
	if err := os.WriteFile(phaseNPrivatePath, material.PrivPEM, 0o600); err != nil { //nolint:gosec // test-owned temporary private key
		t.Fatalf("writing Phase N disposable private key: %v", err)
	}
	if err := os.WriteFile(phaseNPublicPath, []byte(material.PubLine), 0o644); err != nil { //nolint:gosec // test-owned temporary public key
		t.Fatalf("writing Phase N disposable public key: %v", err)
	}
	policyAuthTitle := policyPrefix + "authentication"
	policySigningTitle := policyPrefix + "signing"
	if policyAuthTitle == policySigningTitle || policyAuthTitle != "gitid-e2e:"+runID+":authentication" || policySigningTitle != "gitid-e2e:"+runID+":signing" {
		t.Fatalf("Phase N policy titles are not exact and distinct: auth=%q signing=%q", policyAuthTitle, policySigningTitle)
	}

	// Phase N separately proves the per-registration-title engine API. Phase P
	// above remains the authoritative compiled-product evidence for UP-03.
	phaseNResults := uploader.UploadKeys(uploader.ToolGH, ghPath, phaseNPublicPath, []uploader.RegistrationRequest{
		{Registration: uploader.RegistrationAuthentication, Title: policyAuthTitle},
		{Registration: uploader.RegistrationSigning, Title: policySigningTitle},
	}, deps)
	if len(phaseNResults) != 2 || phaseNResults[0].Outcome != uploader.OutcomeUploaded || phaseNResults[1].Outcome != uploader.OutcomeUploaded {
		t.Fatalf("Phase N per-title registrations failed: %#v", phaseNResults)
	}
	recorder.assertNoPath(t, phaseNPrivatePath)

	inventory, err = uploader.Inventory(uploader.ToolGH, ghPath, deps)
	if err != nil {
		t.Fatalf("reading Phase N inventory for exact-title ID lookup: %v", err)
	}
	policyAuth, found := uploader.FindByTitle(inventory, policyAuthTitle)
	if !found || policyAuth.Registration != uploader.RegistrationAuthentication {
		t.Fatalf("run-scoped inventory lookup did not resolve exact Phase N authentication title %q", policyAuthTitle)
	}
	record(policyAuth, policyPrefix)
	policySigning, found := uploader.FindByTitle(inventory, policySigningTitle)
	if !found || policySigning.Registration != uploader.RegistrationSigning {
		t.Fatalf("run-scoped inventory lookup did not resolve exact Phase N signing title %q", policySigningTitle)
	}
	record(policySigning, policyPrefix)
	if !uploader.HasRegistration(inventory, material.PubLine, uploader.RegistrationAuthentication) || !uploader.HasRegistration(inventory, material.PubLine, uploader.RegistrationSigning) {
		t.Fatal("Phase N inventory did not confirm both authentication and signing registrations")
	}

	for _, recorded := range outstanding.snapshot() {
		if err := deleteRecordedRemoteKey(uploader.ToolGH, ghPath, deps, outstanding, recorded); err != nil {
			t.Fatalf("explicit deletion of recorded ID %s failed: %v", recorded.entry.ID, err)
		}
		t.Logf("deleted recorded resource ID after scope re-confirmation: registration=%s id=%s", registrationLabel(recorded.entry.Registration), recorded.entry.ID)
	}
}

func TestRealAccountScopeParserRequiresExactMembers(t *testing.T) {
	tests := []struct {
		name   string
		line   string
		accept bool
	}{
		{name: "exact pair", line: "  - Token scopes: 'admin:public_key', 'admin:ssh_signing_key', 'repo'", accept: true},
		{name: "authentication substring only", line: "  - Token scopes: 'xadmin:public_key', 'admin:ssh_signing_key'", accept: false},
		{name: "signing substring only", line: "  - Token scopes: 'admin:public_key', 'admin:ssh_signing_key:extra'", accept: false},
		{name: "one required scope", line: "  - Token scopes: 'admin:public_key', 'repo'", accept: false},
		{name: "missing line", line: "  - Token: gho_************************************", accept: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			scopes, _, found := parseGitHubScopeLine(test.line)
			accepted := found && len(missingRequiredScopes(scopes)) == 0
			if accepted != test.accept {
				t.Fatalf("parseGitHubScopeLine(%q) accepted=%t, want %t", test.line, accepted, test.accept)
			}
		})
	}
}

func TestRealAccountResolvesRealGHConfigDir(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want string
	}{
		{name: "GH_CONFIG_DIR wins", env: map[string]string{"GH_CONFIG_DIR": "/credential/gh", "XDG_CONFIG_HOME": "/xdg", "HOME": "/home/test"}, want: "/credential/gh"},
		{name: "XDG_CONFIG_HOME second", env: map[string]string{"XDG_CONFIG_HOME": "/xdg", "HOME": "/home/test"}, want: "/xdg/gh"},
		{name: "HOME fallback", env: map[string]string{"HOME": "/home/test"}, want: "/home/test/.config/gh"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := resolveRealGHConfigDir(test.env); got != test.want {
				t.Fatalf("resolveRealGHConfigDir() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestRealAccountCleanupBookkeepingIsIdempotent(t *testing.T) {
	first := uploader.ExistingKey{ID: "1", Registration: uploader.RegistrationAuthentication, Title: "gitid-e2e:test"}
	second := uploader.ExistingKey{ID: "2", Registration: uploader.RegistrationSigning, Title: "gitid-e2e:test"}
	for _, test := range []struct {
		name           string
		explicitFirst  bool
		explicitSecond bool
		wantCleanup    int
	}{
		{name: "all explicitly deleted", explicitFirst: true, explicitSecond: true, wantCleanup: 0},
		{name: "early failure retains remaining", explicitFirst: true, wantCleanup: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			outstanding := &outstandingRemoteKeys{}
			outstanding.add(first, "gitid-e2e:test")
			outstanding.add(second, "gitid-e2e:test")
			if test.explicitFirst {
				outstanding.remove(first)
			}
			if test.explicitSecond {
				outstanding.remove(second)
			}
			if got := len(outstanding.snapshot()); got != test.wantCleanup {
				t.Fatalf("cleanup outstanding count = %d, want %d", got, test.wantCleanup)
			}
		})
	}
}

func TestRealAccountCleanupOrderingIsLIFOSweepLast(t *testing.T) {
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
