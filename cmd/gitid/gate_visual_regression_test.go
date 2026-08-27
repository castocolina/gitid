//go:build screenshot

package main

// gate_visual_regression_test.go is `make gate-visual-regression`'s read-only
// entry point. RequiredScreenSpecs defines the symmetric real/dummy inventory.
// Each surface is checked independently for deterministic capture; real versus
// dummy differences are accepted only with an explicit ux-improvement/defect
// classification, and one-sided states remain non-comparable. HTML and PNG or
// text byte parity are intentionally outside this gate.

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/castocolina/gitid/internal/dummytui"
	"github.com/castocolina/gitid/internal/keygen"
	"github.com/castocolina/gitid/internal/screenshot"
	"github.com/castocolina/gitid/internal/tuikit"
)

// approvalCommitFull is the full SHA of the Phase-2 design approval commit (CR-03).
// Approved HTML and TUI sources are captured from this commit only, never from
// current HEAD or any other reference.
const approvalCommitFull = "3c3130e404329cf42baafdf63a6c22758437edc6"

// deterministicReusableKeyMaterial holds pre-generated, fixed-content key material
// for the gate fixture. This is generated once per test binary load and reused
// for all home directories, ensuring all captures see the same fingerprint/path
// and are byte-identical across runs (CR-01: no random key material per run).
type reusableKeyMat struct {
	privPEM []byte
	pubLine string
}

var deterministicReusableKeyMaterial = sync.OnceValue(func() reusableKeyMat {
	mat, err := keygen.GenerateMaterial(keygen.Params{
		Algo: "ed25519", Identity: "gate", Comment: "gate@gitid",
	})
	if err != nil {
		panic("gate-visual-regression: generating deterministic key fixture: " + err.Error())
	}
	return reusableKeyMat{privPEM: mat.PrivPEM, pubLine: mat.PubLine}
})

// deterministicReusableKeyFixture writes ONE parseable ed25519 key into home/.ssh
// using FIXED key material shared across all fixture instances within a test run
// (CR-01 fix: same fingerprint/path in all captures so two-run text hashes are equal).
func deterministicReusableKeyFixture(t *testing.T, home string) {
	t.Helper()
	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatalf("gate-visual-regression: seeding %s: %v", sshDir, err)
	}
	keyPath := filepath.Join(sshDir, "id_ed25519_gate")

	km := deterministicReusableKeyMaterial()
	if err := os.WriteFile(keyPath, km.privPEM, 0o600); err != nil {
		t.Fatalf("gate-visual-regression: writing the fixture private key: %v", err)
	}
	if err := os.WriteFile(keyPath+".pub", []byte(km.pubLine+"\n"), 0o644); err != nil { //nolint:gosec // .pub is public key material by definition; hermetic sandbox HOME (G306)
		t.Fatalf("gate-visual-regression: writing the fixture public key: %v", err)
	}
}

// deterministicGitIdentityFixture seeds home with TWO identities the git-screen
// registry (04-04-PLAN.md Task 3, screenshot.CaptureGitScreenScreens) needs —
// "gscreen" (COMPLETE: SSH host block + Git fragment + includeIf +
// allowed_signers, index 0 — drives git-form-filled/match-strategy-select/
// review-readonly/result-success) and "gscreenssh" (SSH-only, no Git side,
// index 1 — drives git-form-empty). Deliberately NOT named "acme": the
// create-flow wizard's own default alias prefix is "acme"
// (internal/screenshot/createflow.go captureSpec/ScreenSpecRegistry), and
// CaptureCreateFlowScreens' create-flow captures run against the SAME
// backend/HOME this fixture seeds — an "acme" collision would silently break
// the wizard's alias-collision-free default form, corrupting unrelated
// create-flow captures ("test-stage1-pass" etc. never populate because the
// wizard's step-0 form is invalid). Every byte here is FIXED, no
// randomness/timestamps (CR-01: two independent capture runs must produce
// byte-identical text hashes).
func deterministicGitIdentityFixture(t *testing.T, home string) {
	t.Helper()
	sshDir := filepath.Join(home, ".ssh")
	gitconfigD := filepath.Join(home, ".gitconfig.d")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatalf("gate-visual-regression: seeding %s: %v", sshDir, err)
	}
	if err := os.MkdirAll(gitconfigD, 0o755); err != nil {
		t.Fatalf("gate-visual-regression: seeding %s: %v", gitconfigD, err)
	}

	for _, name := range []string{"gscreen", "gscreenssh"} {
		priv := filepath.Join(sshDir, "id_ed25519_"+name)
		if err := os.WriteFile(priv, []byte("-----BEGIN OPENSSH PRIVATE KEY-----\nSTUB\n-----END OPENSSH PRIVATE KEY-----\n"), 0o600); err != nil {
			t.Fatalf("gate-visual-regression: writing fixture key %s: %v", priv, err)
		}
		pub := priv + ".pub"
		pubContent := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5STUB " + name + "@gitid-test\n"
		if err := os.WriteFile(pub, []byte(pubContent), 0o644); err != nil { //nolint:gosec // .pub is public key material by definition; hermetic sandbox HOME (G306)
			t.Fatalf("gate-visual-regression: writing fixture pubkey %s: %v", pub, err)
		}
	}

	// "gscreen" declared FIRST so identity.Reconstruct's file-order
	// reconstruction puts it at index 0 (the default-selected identity) and
	// "gscreenssh" at index 1.
	sshConfig := "# BEGIN gitid managed: gscreen\n" +
		"Host gscreen.github.com\n  HostName github.com\n  User git\n  IdentityFile ~/.ssh/id_ed25519_gscreen\n  IdentitiesOnly yes\n" +
		"# END gitid managed: gscreen\n\n" +
		"# BEGIN gitid managed: gscreenssh\n" +
		"Host gscreenssh.github.com\n  HostName github.com\n  User git\n  IdentityFile ~/.ssh/id_ed25519_gscreenssh\n  IdentitiesOnly yes\n" +
		"# END gitid managed: gscreenssh\n\n" +
		"Host *\n  IgnoreUnknown UseKeychain\n  AddKeysToAgent yes\n"
	if err := os.WriteFile(filepath.Join(sshDir, "config"), []byte(sshConfig), 0o600); err != nil {
		t.Fatalf("gate-visual-regression: writing fixture ssh/config: %v", err)
	}

	gitconfig := "[user]\n  name = Test User\n  email = test@example.com\n\n" +
		"# BEGIN gitid managed: gscreen\n" +
		"[includeIf \"gitdir:~/git/gscreen/\"]\n  path = ~/.gitconfig.d/gscreen\n" +
		"# END gitid managed: gscreen\n"
	if err := os.WriteFile(filepath.Join(home, ".gitconfig"), []byte(gitconfig), 0o644); err != nil {
		t.Fatalf("gate-visual-regression: writing fixture .gitconfig: %v", err)
	}

	fragment := "[user]\n  name = gscreen User\n  email = gscreen@example.com\n  signingkey = ~/.ssh/id_ed25519_gscreen.pub\n"
	if err := os.WriteFile(filepath.Join(gitconfigD, "gscreen"), []byte(fragment), 0o644); err != nil {
		t.Fatalf("gate-visual-regression: writing fixture fragment: %v", err)
	}

	signers := "# BEGIN gitid managed: gscreen\n" +
		"gscreen@example.com namespaces=\"git\" ssh-ed25519 AAAAC3NzaC1lZDI1NTE5STUB gscreen@gitid-test\n" +
		"# END gitid managed: gscreen\n"
	if err := os.WriteFile(filepath.Join(sshDir, "allowed_signers"), []byte(signers), 0o644); err != nil { //nolint:gosec // hermetic sandbox HOME fixture (G306)
		t.Fatalf("gate-visual-regression: writing fixture allowed_signers: %v", err)
	}
}

// deterministicIdentityManagerFixture seeds home with TWO identities the
// identity-manager registry (05-09-PLAN.md Task 3,
// screenshot.CaptureIdentityManagerScreens) needs — "imgr" (COMPLETE: SSH
// host block + Git fragment + includeIf + allowed_signers, index 0 — drives
// action-menu/delete-choice/confirm-destructive) and "imgrssh" (SSH-only, no
// Git side, index 1 — drives detail-ssh-first). Names deliberately avoid
// collision with the create-flow wizard's own default "acme" prefix and
// deterministicGitIdentityFixture's "gscreen"/"gscreenssh" (both fixtures
// run against SEPARATE, dedicated HOMEs, same isolation precedent as
// git-screen's own fixture — see mergeGitScreenCaptures' doc comment). Every
// byte here is FIXED, no randomness/timestamps (CR-01).
func deterministicIdentityManagerFixture(t *testing.T, home string) {
	t.Helper()
	sshDir := filepath.Join(home, ".ssh")
	gitconfigD := filepath.Join(home, ".gitconfig.d")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatalf("gate-visual-regression: seeding %s: %v", sshDir, err)
	}
	if err := os.MkdirAll(gitconfigD, 0o755); err != nil {
		t.Fatalf("gate-visual-regression: seeding %s: %v", gitconfigD, err)
	}

	for _, name := range []string{"imgr", "imgrssh"} {
		priv := filepath.Join(sshDir, "id_ed25519_"+name)
		if err := os.WriteFile(priv, []byte("-----BEGIN OPENSSH PRIVATE KEY-----\nSTUB\n-----END OPENSSH PRIVATE KEY-----\n"), 0o600); err != nil {
			t.Fatalf("gate-visual-regression: writing fixture key %s: %v", priv, err)
		}
		pub := priv + ".pub"
		pubContent := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5STUB " + name + "@gitid-test\n"
		if err := os.WriteFile(pub, []byte(pubContent), 0o644); err != nil { //nolint:gosec // .pub is public key material by definition; hermetic sandbox HOME fixture (G306)
			t.Fatalf("gate-visual-regression: writing fixture pubkey %s: %v", pub, err)
		}
	}

	// "imgr" declared FIRST so identity.Reconstruct's file-order
	// reconstruction puts it at index 0 (the default-selected identity) and
	// "imgrssh" at index 1.
	sshConfig := "# BEGIN gitid managed: imgr\n" +
		"Host imgr.github.com\n  HostName github.com\n  User git\n  IdentityFile ~/.ssh/id_ed25519_imgr\n  IdentitiesOnly yes\n" +
		"# END gitid managed: imgr\n\n" +
		"# BEGIN gitid managed: imgrssh\n" +
		"Host imgrssh.github.com\n  HostName github.com\n  User git\n  IdentityFile ~/.ssh/id_ed25519_imgrssh\n  IdentitiesOnly yes\n" +
		"# END gitid managed: imgrssh\n\n" +
		"Host *\n  IgnoreUnknown UseKeychain\n  AddKeysToAgent yes\n"
	if err := os.WriteFile(filepath.Join(sshDir, "config"), []byte(sshConfig), 0o600); err != nil {
		t.Fatalf("gate-visual-regression: writing fixture ssh/config: %v", err)
	}

	gitconfig := "[user]\n  name = Test User\n  email = test@example.com\n\n" +
		"# BEGIN gitid managed: imgr\n" +
		"[includeIf \"gitdir:~/git/imgr/\"]\n  path = ~/.gitconfig.d/imgr\n" +
		"# END gitid managed: imgr\n"
	if err := os.WriteFile(filepath.Join(home, ".gitconfig"), []byte(gitconfig), 0o644); err != nil {
		t.Fatalf("gate-visual-regression: writing fixture .gitconfig: %v", err)
	}

	fragment := "[user]\n  name = imgr User\n  email = imgr@example.com\n  signingkey = ~/.ssh/id_ed25519_imgr.pub\n"
	if err := os.WriteFile(filepath.Join(gitconfigD, "imgr"), []byte(fragment), 0o644); err != nil {
		t.Fatalf("gate-visual-regression: writing fixture fragment: %v", err)
	}

	signers := "# BEGIN gitid managed: imgr\n" +
		"imgr@example.com namespaces=\"git\" ssh-ed25519 AAAAC3NzaC1lZDI1NTE5STUB imgr@gitid-test\n" +
		"# END gitid managed: imgr\n"
	if err := os.WriteFile(filepath.Join(sshDir, "allowed_signers"), []byte(signers), 0o644); err != nil { //nolint:gosec // hermetic sandbox HOME fixture (G306)
		t.Fatalf("gate-visual-regression: writing fixture allowed_signers: %v", err)
	}
}

// mergeIdentityManagerCaptures captures the four Phase 5 identity-manager
// checkpoints (05-09-PLAN.md Task 3, screenshot.CaptureIdentityManagerScreens)
// for both the real backend (seeded from imgrHome via
// deterministicIdentityManagerFixture) and the dummy backend, merging each
// into the caller's realCaptures/dummyCaptures maps — mirrors
// mergeGitScreenCaptures exactly, including its temporary $HOME override.
func mergeIdentityManagerCaptures(t *testing.T, realCaptures, dummyCaptures map[string]string, imgrHome string) {
	t.Helper()
	restoreHome := os.Getenv("HOME")
	t.Setenv("HOME", imgrHome)
	imgrRealBackend := newBackendForHome(imgrHome)
	rawImgrReal, err := screenshot.CaptureIdentityManagerScreens(imgrRealBackend)
	if err != nil {
		t.Fatalf("gate-visual-regression: capturing identity-manager real backend: %v", err)
	}
	t.Setenv("HOME", restoreHome)
	imgrReal := normalizeDisposableHome(rawImgrReal, imgrHome)
	for id, text := range imgrReal {
		realCaptures[id] = text
	}

	imgrDummy, err := screenshot.CaptureIdentityManagerScreens(dummytui.NewFixtureBackend())
	if err != nil {
		t.Fatalf("gate-visual-regression: capturing identity-manager dummy backend: %v", err)
	}
	for id, text := range imgrDummy {
		dummyCaptures[id] = text
	}
}

// predicateMatches returns true when the allowlist predicate permits the
// difference between real and dummy for this region. The "differs" predicate
// is never a valid input here — it is rejected at parse time (CR-04).
func predicateMatches(predicate, regionText string) bool {
	switch {
	case strings.HasPrefix(predicate, "contains:"):
		needle := strings.TrimPrefix(predicate, "contains:")
		// strip surrounding quotes if present
		needle = strings.Trim(needle, `"`)
		return strings.Contains(screenshot.StripANSIExported(regionText), needle)
	case strings.HasPrefix(predicate, "absent:"):
		needle := strings.TrimPrefix(predicate, "absent:")
		needle = strings.Trim(needle, `"`)
		return !strings.Contains(screenshot.StripANSIExported(regionText), needle)
	}
	return false
}

// textHash returns the hex SHA-256 of s.
func textHash(s string) string {
	h := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", h)
}

func normalizeDisposableHome(captures map[string]string, home string) map[string]string {
	normalized := make(map[string]string, len(captures))
	for id, capture := range captures {
		capture = strings.ReplaceAll(capture, home, "<home>")
		// macOS resolves /private/var to /var while scanning key paths.
		capture = strings.ReplaceAll(capture, strings.TrimPrefix(home, "/private"), "<home>")
		// Long paths wrap before the complete HOME can be matched. The t.TempDir
		// run component remains intact and is the only semantic difference.
		capture = strings.ReplaceAll(capture, "/"+filepath.Base(home)+"/", "/<run>/")
		normalized[id] = capture
	}
	return normalized
}

// mergeGitScreenCaptures captures the five Phase 4 git-screen checkpoints
// (04-04-PLAN.md Task 3, screenshot.CaptureGitScreenScreens) for both the
// real backend (seeded from gitHome via deterministicGitIdentityFixture) and
// the dummy backend, and merges each into the caller's realCaptures/
// dummyCaptures maps — the explicit two-registry merge
// internal/screenshot/createflow.go's ScreenSpecRegistry() doc comment
// describes. Temporarily overrides $HOME for the real capture (the identity
// inventory reader tilde-expands against it, the same pattern
// TestGateVisualRegression already relies on for home1/home2) and restores
// it to restoreHome afterward so subsequent create-flow-dependent code is
// unaffected.
func mergeGitScreenCaptures(t *testing.T, realCaptures, dummyCaptures map[string]string, gitHome string) {
	t.Helper()
	restoreHome := os.Getenv("HOME")
	t.Setenv("HOME", gitHome)
	gitRealBackend := newBackendForHome(gitHome)
	rawGitReal, err := screenshot.CaptureGitScreenScreens(gitRealBackend)
	if err != nil {
		t.Fatalf("gate-visual-regression: capturing git-screen real backend: %v", err)
	}
	t.Setenv("HOME", restoreHome)
	gitReal := normalizeDisposableHome(rawGitReal, gitHome)
	for id, text := range gitReal {
		realCaptures[id] = text
	}

	gitDummy, err := screenshot.CaptureGitScreenScreens(dummytui.NewFixtureBackend())
	if err != nil {
		t.Fatalf("gate-visual-regression: capturing git-screen dummy backend: %v", err)
	}
	for id, text := range gitDummy {
		dummyCaptures[id] = text
	}
}

// TestGateVisualRegression is `make gate-visual-regression`'s entry point.
//
// CR-01 (read-only): writes ONLY to t.TempDir() — never to .planning/ or any
// tracked path. Runs TWO captures per backend, compares text hashes per screen
// for determinism.
//
// CR-04 (applicable regions): every named region that is nonempty on either
// side is gated. RequiredRegions remains the mandatory-presence subset, not the
// comparison inventory.
//
// CR-05 (fail closed): fatal on any missing screen, failed capture, or
// schema error.
func TestGateVisualRegression(t *testing.T) {
	// Two fresh temp homes — each capture run gets its own isolated HOME so
	// filesystem state cannot bleed between runs (CR-01 determinism).
	home1 := t.TempDir()
	home2 := t.TempDir()
	stageDir := filepath.Join(t.TempDir(), "stage")
	t.Setenv("GITID_STAGE_DIR", stageDir)

	// Seed deterministic (path-stable) fixtures in both homes.
	deterministicReusableKeyFixture(t, home1)
	deterministicReusableKeyFixture(t, home2)

	// 04-04-PLAN.md Task 3: git-screen checkpoints (screenshot.
	// CaptureGitScreenScreens) are captured against SEPARATE, dedicated
	// HOMEs — seeding the create-flow home with extra identities was tried
	// and reverted: it shifted the wizard's sidebar layout and broke an
	// UNRELATED create-flow screen's connectivity-output region (discovered
	// empirically). Isolating the git-screen fixture keeps the two
	// registries' captures independent.
	gitHome1 := t.TempDir()
	gitHome2 := t.TempDir()
	deterministicGitIdentityFixture(t, gitHome1)
	deterministicGitIdentityFixture(t, gitHome2)

	// 05-09-PLAN.md Task 3: identity-manager checkpoints, isolated the SAME
	// way git-screen's own fixture is (see comment above).
	imgrHome1 := t.TempDir()
	imgrHome2 := t.TempDir()
	deterministicIdentityManagerFixture(t, imgrHome1)
	deterministicIdentityManagerFixture(t, imgrHome2)

	// 06-07-PLAN.md Task 1: Global SSH checkpoints, isolated the SAME way
	// (deterministicGlobalSSHFixture seeds its OWN Include-layout home so the
	// four option states are produced without perturbing any prior surface).
	gssHome1 := t.TempDir()
	gssHome2 := t.TempDir()
	deterministicGlobalSSHFixture(t, gssHome1)
	deterministicGlobalSSHFixture(t, gssHome2)

	// CR-01: run TWO independent captures and compare text hashes.
	t.Setenv("HOME", home1)
	realBackend1 := newBackendForHome(home1)
	dummyBackend1 := dummytui.NewFixtureBackend()
	rawRealCaptures1, err := screenshot.CaptureCreateFlowScreens(realBackend1)
	if err != nil {
		t.Fatalf("gate-visual-regression: capturing real backend (run 1): %v", err)
	}
	realCaptures1 := normalizeDisposableHome(rawRealCaptures1, home1)
	dummyCaptures1, err := screenshot.CaptureCreateFlowScreens(dummyBackend1)
	if err != nil {
		t.Fatalf("gate-visual-regression: capturing dummy backend (run 1): %v", err)
	}
	mergeGitScreenCaptures(t, realCaptures1, dummyCaptures1, gitHome1)
	mergeIdentityManagerCaptures(t, realCaptures1, dummyCaptures1, imgrHome1)
	mergeGlobalSSHCaptures(t, realCaptures1, dummyCaptures1, gssHome1)

	t.Setenv("HOME", home2)
	realBackend2 := newBackendForHome(home2)
	dummyBackend2 := dummytui.NewFixtureBackend()
	rawRealCaptures2, err := screenshot.CaptureCreateFlowScreens(realBackend2)
	if err != nil {
		t.Fatalf("gate-visual-regression: capturing real backend (run 2): %v", err)
	}
	realCaptures2 := normalizeDisposableHome(rawRealCaptures2, home2)
	dummyCaptures2, err := screenshot.CaptureCreateFlowScreens(dummyBackend2)
	if err != nil {
		t.Fatalf("gate-visual-regression: capturing dummy backend (run 2): %v", err)
	}
	mergeGitScreenCaptures(t, realCaptures2, dummyCaptures2, gitHome2)
	mergeIdentityManagerCaptures(t, realCaptures2, dummyCaptures2, imgrHome2)
	mergeGlobalSSHCaptures(t, realCaptures2, dummyCaptures2, gssHome2)

	specs := screenshot.RequiredScreenSpecs()
	// Determinism is checked within each surface. Real and dummy are not byte,
	// pixel, or HTML parity targets for one another.
	for _, spec := range specs {
		id := spec.ScreenID
		if spec.ApplicableLive && realCaptures1[id] != realCaptures2[id] {
			t.Errorf("gate-visual-regression: CR-01 FAIL — screen %q real-backend capture is NOT deterministic across two runs (hash1=%s hash2=%s)\n--- first ---\n%s\n--- second ---\n%s",
				id, textHash(realCaptures1[id]), textHash(realCaptures2[id]), screenshot.StripANSIExported(realCaptures1[id]), screenshot.StripANSIExported(realCaptures2[id]))
		}
		if spec.ApplicableApprovedTUI && dummyCaptures1[id] != dummyCaptures2[id] {
			t.Errorf("gate-visual-regression: CR-01 FAIL — screen %q dummy-backend capture is NOT deterministic across two runs (hash1=%s hash2=%s)",
				id, textHash(dummyCaptures1[id]), textHash(dummyCaptures2[id]))
		}
	}

	if t.Failed() {
		t.FailNow() // determinism failure invalidates the gate
	}

	records, err := screenshot.BuildRegionDiffs("gate-local", realCaptures1, dummyCaptures1, specs)
	if err != nil {
		t.Fatalf("gate-visual-regression: building symmetric registry region evidence: %v", err)
	}
	data := screenshot.BuildRegionDiffsJSON("gate-local", records)
	if err := screenshot.ValidateRegionDiffs(data, "gate-local", specs); err != nil {
		t.Fatalf("gate-visual-regression: validating classified region evidence: %v", err)
	}
	for _, record := range records {
		for _, region := range record.Regions {
			if !region.Comparable || !region.Equal {
				if region.Classification != "ux-improvement" && region.Classification != "defect" {
					t.Errorf("gate-visual-regression: %s/%s has no valid classification", record.ScreenID, region.Name)
				}
			}
		}
	}
	t.Logf("gate-visual-regression: OK — %d RequiredScreenSpecs frames checked as a classified real/dummy symmetric union", len(specs))

	// CR-01: the gate intentionally writes NOTHING to tracked paths.
	// Any PNG/evidence generation goes via the explicit make generate-visual-review-packet
	// target (Task 3 publication step), not the routine gate.
}

// TestGateVisualRegressionReadOnly proves that running TestGateVisualRegression
// does NOT modify any committed/tracked file. It is a structural invariant test
// that runs the gate twice in isolated temp environments and asserts that the
// contents of the committed 03-09 packet dir are unchanged (CR-01).
func TestGateVisualRegressionReadOnly(t *testing.T) {
	// Snapshot the hashes of all tracked files in the committed packet directory.
	// We snapshot via os.ReadDir + sha256 to catch ANY byte-level change.
	packetDir := "../../.planning/phases/03-create-flow-backend/03-09-review-packet"

	before := snapshotDir(t, packetDir)

	// Run the gate in a temp home (same as routine gate does).
	home := t.TempDir()
	deterministicReusableKeyFixture(t, home)
	deterministicGitIdentityFixture(t, home)
	t.Setenv("HOME", home)
	realB := newBackendForHome(home)
	dummyB := dummytui.NewFixtureBackend()
	if _, err := screenshot.CaptureCreateFlowScreens(realB); err != nil {
		t.Logf("gate-visual-regression: capturing real backend (read-only check): %v", err)
	}
	if _, err := screenshot.CaptureCreateFlowScreens(dummyB); err != nil {
		t.Logf("gate-visual-regression: capturing dummy backend (read-only check): %v", err)
	}
	// 04-04-PLAN.md Task 3: git-screen captures must be equally read-only.
	if _, err := screenshot.CaptureGitScreenScreens(realB); err != nil {
		t.Logf("gate-visual-regression: capturing git-screen real backend (read-only check): %v", err)
	}
	if _, err := screenshot.CaptureGitScreenScreens(dummyB); err != nil {
		t.Logf("gate-visual-regression: capturing git-screen dummy backend (read-only check): %v", err)
	}
	// 05-09-PLAN.md Task 3: identity-manager captures must be equally read-only.
	deterministicIdentityManagerFixture(t, home)
	if _, err := screenshot.CaptureIdentityManagerScreens(realB); err != nil {
		t.Logf("gate-visual-regression: capturing identity-manager real backend (read-only check): %v", err)
	}
	if _, err := screenshot.CaptureIdentityManagerScreens(dummyB); err != nil {
		t.Logf("gate-visual-regression: capturing identity-manager dummy backend (read-only check): %v", err)
	}
	// 06-07-PLAN.md Task 1: Global SSH captures must be equally read-only.
	deterministicGlobalSSHFixture(t, home)
	if _, err := screenshot.CaptureGlobalSSHScreens(realB); err != nil {
		t.Logf("gate-visual-regression: capturing global-ssh real backend (read-only check): %v", err)
	}
	if _, err := screenshot.CaptureGlobalSSHScreens(dummyB); err != nil {
		t.Logf("gate-visual-regression: capturing global-ssh dummy backend (read-only check): %v", err)
	}

	after := snapshotDir(t, packetDir)

	// Assert no files changed.
	for path, h := range before {
		if after[path] != h {
			t.Errorf("gate-visual-regression: CR-01 FAIL — routine gate modified tracked file %q (before: %s, after: %s)", path, h, after[path])
		}
	}
	for path := range after {
		if _, ok := before[path]; !ok {
			t.Errorf("gate-visual-regression: CR-01 FAIL — routine gate CREATED new tracked file %q", path)
		}
	}
}

// snapshotDir returns a map of relative path → sha256-hex for all regular files
// under dir (non-recursive, one level only for the packet dir structure).
func snapshotDir(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := make(map[string]string)
	if err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if info.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path) //nolint:gosec // fixed packet-dir path (G304)
		if err != nil {
			return nil
		}
		h := sha256.Sum256(data)
		rel, _ := filepath.Rel(dir, path)
		out[rel] = fmt.Sprintf("%x", h)
		return nil
	}); err != nil {
		t.Fatalf("snapshotDir %s: %v", dir, err)
	}
	return out
}

// TestApprovalCommitRecorded proves that the approval commit constant in this
// file matches the actual approval commit recorded in .planning/design/APPROVAL.md
// (CR-03: the gate must reference the correct, full, unambiguous approval SHA).
func TestApprovalCommitRecorded(t *testing.T) {
	if len(approvalCommitFull) != 40 {
		t.Errorf("approvalCommitFull is not a full 40-hex SHA: %q", approvalCommitFull)
	}
	// Verify the commit exists in the repo.
	gitDir := "../../.git"
	if _, err := os.Stat(gitDir); err != nil {
		t.Skipf("not in a git repo (no .git at %s): %v", gitDir, err)
	}
	// Read the HEAD to verify git is accessible.
	headFile := filepath.Join(gitDir, "HEAD")
	if _, err := os.ReadFile(headFile); err != nil { //nolint:gosec // fixed repo path (G304)
		t.Skipf("cannot read .git/HEAD: %v", err)
	}
	// We can't exec git in a test without introducing external dependency,
	// but we verify the constant is non-empty, full-length, and hex-only.
	for _, r := range approvalCommitFull {
		if !strings.ContainsRune("0123456789abcdef", r) {
			t.Errorf("approvalCommitFull contains non-hex character %q", r)
		}
	}
	t.Logf("gate-visual-regression: approval commit = %s (CR-03)", approvalCommitFull)
}

// TestAllScreensCapturedAndNonEmpty validates every registry-required live
// capture and every applicable dummy comparison frame.
func TestAllScreensCapturedAndNonEmpty(t *testing.T) {
	home := t.TempDir()
	deterministicReusableKeyFixture(t, home)
	t.Setenv("HOME", home)
	realB := newBackendForHome(home)
	dummyB := dummytui.NewFixtureBackend()
	realCaptures, err := screenshot.CaptureCreateFlowScreens(realB)
	if err != nil {
		t.Fatalf("capturing real backend: %v", err)
	}
	dummyCaptures, err := screenshot.CaptureCreateFlowScreens(dummyB)
	if err != nil {
		t.Fatalf("capturing dummy backend: %v", err)
	}
	gitHome := t.TempDir()
	deterministicGitIdentityFixture(t, gitHome)
	mergeGitScreenCaptures(t, realCaptures, dummyCaptures, gitHome)
	imgrHome := t.TempDir()
	deterministicIdentityManagerFixture(t, imgrHome)
	mergeIdentityManagerCaptures(t, realCaptures, dummyCaptures, imgrHome)
	// 06-07-PLAN.md Task 1: the Global SSH registry shares this spec set, so
	// its frames must be present for BuildRegionDiffs regardless of which
	// registry this control scopes its own mutation to.
	gssHome := t.TempDir()
	deterministicGlobalSSHFixture(t, gssHome)
	mergeGlobalSSHCaptures(t, realCaptures, dummyCaptures, gssHome)
	t.Setenv("HOME", home) // restore for any later HOME-dependent assertions

	for _, spec := range screenshot.RequiredScreenSpecs() {
		if spec.ApplicableLive && strings.TrimSpace(realCaptures[spec.ScreenID]) == "" {
			t.Errorf("real backend: required screen %q is empty (CR-05)", spec.ScreenID)
		}
		if spec.ApplicableApprovedTUI && strings.TrimSpace(dummyCaptures[spec.ScreenID]) == "" {
			t.Errorf("dummy backend: applicable screen %q is empty (CR-05)", spec.ScreenID)
		}
	}
}

// TestNegativeControl_UnclassifiedDifferenceRejected proves that a region
// diverging without a ux-improvement/defect classification is rejected by
// ValidateRegionDiffs.
//
// CR-11 (iteration 4) renamed this from
// TestNegativeControls_AllProtectedRegionsDetectMutation: the old name and
// doc comment claimed it proved "every protected (non-allowlisted) region on
// every screen is sensitive to mutations" — the body did none of that. Both
// loops `break` on the first hit, so exactly ONE NamedRegionDiff on ONE
// screen was ever exercised (order-dependent on
// RequiredScreenSpecs()/AllRegionNames()); it targeted `!region.Equal`
// regions — i.e. the ALLOWLISTED ones — not "protected (non-allowlisted)"
// regions; and it only ever cleared a metadata field
// (Classification = ""), never mutated rendered TEXT, so it proved
// ValidateRegionDiffs requires a classification field and nothing about
// whether a changed rendering is actually detected.
//
// This narrower, honestly-named test keeps exactly that one property (the
// classification-requirement check). The property CR-11 actually cared
// about — "is every silent (equal), allowlist-eligible region really
// TEXT-sensitive, not just metadata-sensitive" — is now proven exhaustively,
// across every screen, by
// TestNegativeControl_AllComparableEqualRegionsAreMutationSensitive below.
func TestNegativeControl_UnclassifiedDifferenceRejected(t *testing.T) {
	home := t.TempDir()
	deterministicReusableKeyFixture(t, home)
	t.Setenv("HOME", home)
	realB := newBackendForHome(home)
	realCaptures, err := screenshot.CaptureCreateFlowScreens(realB)
	if err != nil {
		t.Fatalf("capturing real backend: %v", err)
	}
	dummyB := dummytui.NewFixtureBackend()
	dummyCaptures, err := screenshot.CaptureCreateFlowScreens(dummyB)
	if err != nil {
		t.Fatalf("capturing dummy backend: %v", err)
	}
	gitHome := t.TempDir()
	deterministicGitIdentityFixture(t, gitHome)
	mergeGitScreenCaptures(t, realCaptures, dummyCaptures, gitHome)
	imgrHome := t.TempDir()
	deterministicIdentityManagerFixture(t, imgrHome)
	mergeIdentityManagerCaptures(t, realCaptures, dummyCaptures, imgrHome)
	// 06-07-PLAN.md Task 1: the Global SSH registry shares this spec set, so
	// its frames must be present for BuildRegionDiffs regardless of which
	// registry this control scopes its own mutation to.
	gssHome := t.TempDir()
	deterministicGlobalSSHFixture(t, gssHome)
	mergeGlobalSSHCaptures(t, realCaptures, dummyCaptures, gssHome)
	t.Setenv("HOME", home) // restore for any later HOME-dependent assertions

	specs := screenshot.RequiredScreenSpecs()
	records, err := screenshot.BuildRegionDiffs("negative-control", realCaptures, dummyCaptures, specs)
	if err != nil {
		t.Fatalf("building classified region evidence: %v", err)
	}
	mutated := false
	for i := range records {
		for j := range records[i].Regions {
			region := &records[i].Regions[j]
			if !region.Comparable || !region.Equal {
				region.Classification = ""
				mutated = true
				break
			}
		}
		if mutated {
			break
		}
	}
	if !mutated {
		t.Fatal("negative-control: no classified difference was available to mutate")
	}
	data, err := json.Marshal(screenshot.RegionDiffs{Version: "test", SourceCommit: "negative-control", GeneratedAt: "test", Screens: records})
	if err != nil {
		t.Fatalf("marshaling mutated region evidence: %v", err)
	}
	if err := screenshot.ValidateRegionDiffs(data, "negative-control", specs); err == nil {
		t.Fatal("negative-control: validator accepted a difference without ux-improvement/defect classification")
	}
}

// assertAllComparableEqualRegionsAreMutationSensitive is CR-11's exhaustive
// negative control: for EVERY comparable, currently-Equal region across
// EVERY record (optionally scoped by include), it mutates the region's
// rendered TEXT — not a metadata field — and proves ValidateRegionDiffs
// rejects the mutated evidence. A region that passes here would silently
// accept a real rendering regression forever, since it is currently equal
// (no disposition/allowlist entry required) and therefore has nothing else
// gating it. include may be nil to check every screen.
func assertAllComparableEqualRegionsAreMutationSensitive(t *testing.T, specs []screenshot.ScreenSpec, records []screenshot.RegionDiffRecord, include func(screenID string) bool) {
	t.Helper()
	checked := 0
	for i := range records {
		if include != nil && !include(records[i].ScreenID) {
			continue
		}
		for j := range records[i].Regions {
			region := records[i].Regions[j]
			if !region.Comparable || !region.Equal {
				continue // covered by TestNegativeControl_UnclassifiedDifferenceRejected instead
			}
			checked++

			// Deep-copy via JSON round-trip so mutating this one region
			// cannot alias another iteration's fixture.
			raw, err := json.Marshal(records)
			if err != nil {
				t.Fatalf("deep-copying region evidence for %s/%s: %v", records[i].ScreenID, region.Name, err)
			}
			var mutated []screenshot.RegionDiffRecord
			if err := json.Unmarshal(raw, &mutated); err != nil {
				t.Fatalf("deep-copying region evidence for %s/%s: %v", records[i].ScreenID, region.Name, err)
			}
			m := &mutated[i].Regions[j]
			m.LiveText += "\nGATE-CANARY"
			m.LiveHash = textHash(m.LiveText)

			data, err := json.Marshal(screenshot.RegionDiffs{Version: "test", SourceCommit: "negative-control", GeneratedAt: "test", Screens: mutated})
			if err != nil {
				t.Fatalf("marshaling mutated region evidence for %s/%s: %v", records[i].ScreenID, region.Name, err)
			}
			if err := screenshot.ValidateRegionDiffs(data, "negative-control", specs); err == nil {
				t.Errorf("negative-control: region %q on screen %q is NOT mutation-sensitive — the gate would silently accept a real rendering regression here", region.Name, records[i].ScreenID)
			}
		}
	}
	if checked == 0 {
		t.Fatal("negative-control: no comparable, currently-equal region was available to mutate — fixture regressed to all-divergent, or the scope filter matched nothing")
	}
}

// TestNegativeControl_AllComparableEqualRegionsAreMutationSensitive is CR-11's
// required fix: proves, across EVERY RequiredScreenSpecs() screen and EVERY
// comparable currently-equal region on it, that a real TEXT mutation (not a
// metadata field) is caught. See assertAllComparableEqualRegionsAreMutationSensitive.
func TestNegativeControl_AllComparableEqualRegionsAreMutationSensitive(t *testing.T) {
	home := t.TempDir()
	deterministicReusableKeyFixture(t, home)
	t.Setenv("HOME", home)
	realB := newBackendForHome(home)
	realCaptures, err := screenshot.CaptureCreateFlowScreens(realB)
	if err != nil {
		t.Fatalf("capturing real backend: %v", err)
	}
	dummyB := dummytui.NewFixtureBackend()
	dummyCaptures, err := screenshot.CaptureCreateFlowScreens(dummyB)
	if err != nil {
		t.Fatalf("capturing dummy backend: %v", err)
	}
	gitHome := t.TempDir()
	deterministicGitIdentityFixture(t, gitHome)
	mergeGitScreenCaptures(t, realCaptures, dummyCaptures, gitHome)
	imgrHome := t.TempDir()
	deterministicIdentityManagerFixture(t, imgrHome)
	mergeIdentityManagerCaptures(t, realCaptures, dummyCaptures, imgrHome)
	// 06-07-PLAN.md Task 1: the Global SSH registry shares this spec set, so
	// its frames must be present for BuildRegionDiffs regardless of which
	// registry this control scopes its own mutation to.
	gssHome := t.TempDir()
	deterministicGlobalSSHFixture(t, gssHome)
	mergeGlobalSSHCaptures(t, realCaptures, dummyCaptures, gssHome)
	t.Setenv("HOME", home) // restore for any later HOME-dependent assertions

	specs := screenshot.RequiredScreenSpecs()
	records, err := screenshot.BuildRegionDiffs("negative-control", realCaptures, dummyCaptures, specs)
	if err != nil {
		t.Fatalf("building classified region evidence: %v", err)
	}
	assertAllComparableEqualRegionsAreMutationSensitive(t, specs, records, nil)
}

// ---------------------------------------------------------------------------
// 04-04-PLAN.md Task 3 negative controls: missing state, stale/removed
// classification, and cross-registry leakage — each proving the gate
// actually fails when it should, scoped explicitly to the Phase 4 git-screen
// registry (not merely inherited, possibly vacuously, from the generic
// create-flow controls above). Same discipline as the 03-06 precedent
// (03-06-SUMMARY.md "fail-path hand-verified by removing the git-form-demo
// allowlist entry, observing a red FAIL, then reverting").
// ---------------------------------------------------------------------------

// gitScreenScreenIDs is the known Phase 4 git-screen checkpoint vocabulary,
// duplicated here (internal/screenshot's gitScreenSpecs is unexported) so
// these negative controls can scope themselves to Phase 4 without a new
// export surface — deliberately narrow, matching the deterministic fixture's
// own precedent (deterministicGitIdentityFixture's identity-name comment).
var gitScreenScreenIDs = map[string]bool{
	"git-form-filled": true, "git-form-empty": true, "match-strategy-select": true,
	"review-readonly": true, "result-success": true,
}

// TestNegativeControl_MissingGitScreenState proves the gate fails closed
// when a required Phase 4 git-screen capture is missing — CR-05's
// fail-closed contract is not vacuous for the git-screen registry
// specifically (as opposed to only ever being exercised by create-flow's
// own screens).
func TestNegativeControl_MissingGitScreenState(t *testing.T) {
	dummyCaptures, err := screenshot.CaptureGitScreenScreens(dummytui.NewFixtureBackend())
	if err != nil {
		t.Fatalf("capturing git-screen dummy backend: %v", err)
	}
	broken := make(map[string]string, len(dummyCaptures))
	for id, text := range dummyCaptures {
		if id == "result-success" {
			continue // deliberately drop a required git-screen state
		}
		broken[id] = text
	}
	if _, ok := broken["result-success"]; ok {
		t.Fatal("negative-control setup bug: \"result-success\" was not actually dropped")
	}
	specs := screenshot.RequiredScreenSpecs()
	if _, err := screenshot.BuildRegionDiffs("negative-control", broken, broken, specs); err == nil {
		t.Fatal("negative-control: BuildRegionDiffs accepted a capture set missing the required \"result-success\" git-screen frame — CR-05 fail-closed is NOT enforced for Phase 4 states")
	}
}

// TestNegativeControl_GitScreenUnclassifiedDifferenceRejected proves that
// mutating away a Phase 4 git-screen RegionDisposition's classification is
// caught — the SAME protection TestNegativeControl_UnclassifiedDifferenceRejected
// proves generically, scoped explicitly to a git-screen record so Phase 4
// coverage can never be silently exempt from the mutation the generic
// control happens to find first.
//
// CR-11 (iteration 4) renamed this from TestNegativeControl_StaleGitScreenClassification
// — it is a copy-paste of the pre-fix generic control and inherited the SAME
// three defects (break-after-first, targets allowlisted `!Equal` regions
// only, metadata-only mutation). See the sibling rename's comment for the
// full account. The exhaustive, TEXT-mutating property is now proven,
// scoped to git-screen records, by
// TestNegativeControl_AllGitScreenComparableEqualRegionsAreMutationSensitive
// below.
func TestNegativeControl_GitScreenUnclassifiedDifferenceRejected(t *testing.T) {
	home := t.TempDir()
	deterministicReusableKeyFixture(t, home)
	t.Setenv("HOME", home)
	realB := newBackendForHome(home)
	realCaptures, err := screenshot.CaptureCreateFlowScreens(realB)
	if err != nil {
		t.Fatalf("capturing real backend: %v", err)
	}
	dummyB := dummytui.NewFixtureBackend()
	dummyCaptures, err := screenshot.CaptureCreateFlowScreens(dummyB)
	if err != nil {
		t.Fatalf("capturing dummy backend: %v", err)
	}
	gitHome := t.TempDir()
	deterministicGitIdentityFixture(t, gitHome)
	mergeGitScreenCaptures(t, realCaptures, dummyCaptures, gitHome)
	// 05-09-PLAN.md Task 3: RequiredScreenSpecs() is a THREE-way merged
	// registry now (create-flow + git-screen + identity-manager) —
	// BuildRegionDiffs below requires every ApplicableLive frame present
	// regardless of which registry this control scopes ITS OWN mutation to,
	// so the identity-manager captures must still be merged in.
	imgrHome := t.TempDir()
	deterministicIdentityManagerFixture(t, imgrHome)
	mergeIdentityManagerCaptures(t, realCaptures, dummyCaptures, imgrHome)
	// 06-07-PLAN.md Task 1: the Global SSH registry shares this spec set, so
	// its frames must be present for BuildRegionDiffs regardless of which
	// registry this control scopes its own mutation to.
	gssHome := t.TempDir()
	deterministicGlobalSSHFixture(t, gssHome)
	mergeGlobalSSHCaptures(t, realCaptures, dummyCaptures, gssHome)
	t.Setenv("HOME", home)

	specs := screenshot.RequiredScreenSpecs()
	records, err := screenshot.BuildRegionDiffs("negative-control", realCaptures, dummyCaptures, specs)
	if err != nil {
		t.Fatalf("building classified region evidence: %v", err)
	}
	mutated := false
	for i := range records {
		if !gitScreenScreenIDs[records[i].ScreenID] {
			continue // scope this control to Phase 4 git-screen records only
		}
		for j := range records[i].Regions {
			region := &records[i].Regions[j]
			if !region.Comparable || !region.Equal {
				region.Classification = ""
				mutated = true
				break
			}
		}
		if mutated {
			break
		}
	}
	if !mutated {
		t.Fatal("negative-control: no classified git-screen difference was available to mutate")
	}
	data, err := json.Marshal(screenshot.RegionDiffs{Version: "test", SourceCommit: "negative-control", GeneratedAt: "test", Screens: records})
	if err != nil {
		t.Fatalf("marshaling mutated region evidence: %v", err)
	}
	if err := screenshot.ValidateRegionDiffs(data, "negative-control", specs); err == nil {
		t.Fatal("negative-control: validator accepted a Phase 4 git-screen difference without ux-improvement/defect classification")
	}
}

// TestNegativeControl_AllGitScreenComparableEqualRegionsAreMutationSensitive
// is CR-11's required fix, scoped to the Phase 4 git-screen registry: proves
// that every comparable, currently-equal region on every git-screen
// checkpoint (git-form-filled, git-form-empty, match-strategy-select,
// review-readonly, result-success) is sensitive to a real TEXT mutation, not
// just a metadata field. See assertAllComparableEqualRegionsAreMutationSensitive.
func TestNegativeControl_AllGitScreenComparableEqualRegionsAreMutationSensitive(t *testing.T) {
	home := t.TempDir()
	deterministicReusableKeyFixture(t, home)
	t.Setenv("HOME", home)
	realB := newBackendForHome(home)
	realCaptures, err := screenshot.CaptureCreateFlowScreens(realB)
	if err != nil {
		t.Fatalf("capturing real backend: %v", err)
	}
	dummyB := dummytui.NewFixtureBackend()
	dummyCaptures, err := screenshot.CaptureCreateFlowScreens(dummyB)
	if err != nil {
		t.Fatalf("capturing dummy backend: %v", err)
	}
	gitHome := t.TempDir()
	deterministicGitIdentityFixture(t, gitHome)
	mergeGitScreenCaptures(t, realCaptures, dummyCaptures, gitHome)
	// 05-09-PLAN.md Task 3: same reason as the sibling negative control
	// above — RequiredScreenSpecs() now needs every registry's captures
	// present regardless of which one this control scopes its own check to.
	imgrHome := t.TempDir()
	deterministicIdentityManagerFixture(t, imgrHome)
	mergeIdentityManagerCaptures(t, realCaptures, dummyCaptures, imgrHome)
	// 06-07-PLAN.md Task 1: the Global SSH registry shares this spec set, so
	// its frames must be present for BuildRegionDiffs regardless of which
	// registry this control scopes its own mutation to.
	gssHome := t.TempDir()
	deterministicGlobalSSHFixture(t, gssHome)
	mergeGlobalSSHCaptures(t, realCaptures, dummyCaptures, gssHome)
	t.Setenv("HOME", home)

	specs := screenshot.RequiredScreenSpecs()
	records, err := screenshot.BuildRegionDiffs("negative-control", realCaptures, dummyCaptures, specs)
	if err != nil {
		t.Fatalf("building classified region evidence: %v", err)
	}
	assertAllComparableEqualRegionsAreMutationSensitive(t, specs, records, func(screenID string) bool {
		return gitScreenScreenIDs[screenID]
	})
}

// TestNegativeControl_CrossRegistryLeakage proves every Phase 4 git-screen
// spec's decision references use ONLY the scoped CTX-D-NN/UI-D-NN
// vocabulary — never a bare D-NN/T-NN ref that would silently resolve
// against create-flow's OWN 03-CONTEXT.md numbering instead of
// 04-CONTEXT.md's (04-04-PLAN.md Task 3: "resolve the CONTEXT/UI-SPEC
// namespace collision"). validDecisionRef's generalized acceptance of bare
// D-NN/T-NN (kept for create-flow backward compatibility) would otherwise
// silently mask a bare ref accidentally leaking into the git-screen
// registry — a real cross-registry ambiguity, since 04-CONTEXT.md and
// 03-CONTEXT.md both number their own decisions starting at D-01.
func TestNegativeControl_CrossRegistryLeakage(t *testing.T) {
	found := false
	for _, spec := range screenshot.RequiredScreenSpecs() {
		if !gitScreenScreenIDs[spec.ScreenID] {
			continue
		}
		found = true
		for _, d := range spec.RegionDispositions {
			if !strings.HasPrefix(d.Decision, "CTX-D-") && !strings.HasPrefix(d.Decision, "UI-D-") {
				t.Errorf("cross-registry leakage: git-screen spec %q region %q disposition uses bare decision ref %q — must be scoped CTX-D-NN/UI-D-NN", spec.ScreenID, d.Region, d.Decision)
			}
		}
		for _, na := range spec.NonApplicability {
			if !strings.HasPrefix(na.Decision, "CTX-D-") && !strings.HasPrefix(na.Decision, "UI-D-") {
				t.Errorf("cross-registry leakage: git-screen spec %q surface %q non-applicability uses bare decision ref %q — must be scoped CTX-D-NN/UI-D-NN", spec.ScreenID, na.Surface, na.Decision)
			}
		}
	}
	if !found {
		t.Fatal("cross-registry leakage: no git-screen specs found in RequiredScreenSpecs() — the registry merge broke")
	}
}

// ---------------------------------------------------------------------------
// 05-09-PLAN.md Task 3 negative controls: missing state, unclassified
// difference, mutation-sensitivity, and cross-registry leakage — scoped to
// the Phase 5 identity-manager registry, mirroring the Phase 4 git-screen
// negative controls above exactly.
// ---------------------------------------------------------------------------

// identityManagerScreenIDs is the known Phase 5 identity-manager checkpoint
// vocabulary, duplicated here (internal/screenshot's identityManagerSpecs is
// unexported) so these negative controls can scope themselves to Phase 5
// without a new export surface — mirrors gitScreenScreenIDs' own precedent.
var identityManagerScreenIDs = map[string]bool{
	"action-menu": true, "delete-choice": true, "confirm-destructive": true, "detail-ssh-first": true,
}

// TestNegativeControl_MissingIdentityManagerState proves the gate fails
// closed when a required Phase 5 identity-manager capture is missing.
func TestNegativeControl_MissingIdentityManagerState(t *testing.T) {
	dummyCaptures, err := screenshot.CaptureIdentityManagerScreens(dummytui.NewFixtureBackend())
	if err != nil {
		t.Fatalf("capturing identity-manager dummy backend: %v", err)
	}
	broken := make(map[string]string, len(dummyCaptures))
	for id, text := range dummyCaptures {
		if id == "confirm-destructive" {
			continue // deliberately drop a required identity-manager state
		}
		broken[id] = text
	}
	if _, ok := broken["confirm-destructive"]; ok {
		t.Fatal("negative-control setup bug: \"confirm-destructive\" was not actually dropped")
	}
	specs := screenshot.RequiredScreenSpecs()
	if _, err := screenshot.BuildRegionDiffs("negative-control", broken, broken, specs); err == nil {
		t.Fatal("negative-control: BuildRegionDiffs accepted a capture set missing the required \"confirm-destructive\" identity-manager frame — CR-05 fail-closed is NOT enforced for Phase 5 states")
	}
}

// TestNegativeControl_IdentityManagerUnclassifiedDifferenceRejected proves
// that mutating away a Phase 5 identity-manager RegionDisposition's
// classification is caught — mirrors
// TestNegativeControl_GitScreenUnclassifiedDifferenceRejected exactly,
// scoped to identityManagerScreenIDs.
func TestNegativeControl_IdentityManagerUnclassifiedDifferenceRejected(t *testing.T) {
	home := t.TempDir()
	deterministicReusableKeyFixture(t, home)
	t.Setenv("HOME", home)
	realB := newBackendForHome(home)
	realCaptures, err := screenshot.CaptureCreateFlowScreens(realB)
	if err != nil {
		t.Fatalf("capturing real backend: %v", err)
	}
	dummyB := dummytui.NewFixtureBackend()
	dummyCaptures, err := screenshot.CaptureCreateFlowScreens(dummyB)
	if err != nil {
		t.Fatalf("capturing dummy backend: %v", err)
	}
	gitHome := t.TempDir()
	deterministicGitIdentityFixture(t, gitHome)
	mergeGitScreenCaptures(t, realCaptures, dummyCaptures, gitHome)
	imgrHome := t.TempDir()
	deterministicIdentityManagerFixture(t, imgrHome)
	mergeIdentityManagerCaptures(t, realCaptures, dummyCaptures, imgrHome)
	// 06-07-PLAN.md Task 1: the Global SSH registry shares this spec set, so
	// its frames must be present for BuildRegionDiffs regardless of which
	// registry this control scopes its own mutation to.
	gssHome := t.TempDir()
	deterministicGlobalSSHFixture(t, gssHome)
	mergeGlobalSSHCaptures(t, realCaptures, dummyCaptures, gssHome)
	t.Setenv("HOME", home)

	specs := screenshot.RequiredScreenSpecs()
	records, err := screenshot.BuildRegionDiffs("negative-control", realCaptures, dummyCaptures, specs)
	if err != nil {
		t.Fatalf("building classified region evidence: %v", err)
	}
	mutated := false
	for i := range records {
		if !identityManagerScreenIDs[records[i].ScreenID] {
			continue // scope this control to Phase 5 identity-manager records only
		}
		for j := range records[i].Regions {
			region := &records[i].Regions[j]
			if !region.Comparable || !region.Equal {
				region.Classification = ""
				mutated = true
				break
			}
		}
		if mutated {
			break
		}
	}
	if !mutated {
		t.Fatal("negative-control: no classified identity-manager difference was available to mutate")
	}
	data, err := json.Marshal(screenshot.RegionDiffs{Version: "test", SourceCommit: "negative-control", GeneratedAt: "test", Screens: records})
	if err != nil {
		t.Fatalf("marshaling mutated region evidence: %v", err)
	}
	if err := screenshot.ValidateRegionDiffs(data, "negative-control", specs); err == nil {
		t.Fatal("negative-control: validator accepted a Phase 5 identity-manager difference without ux-improvement/defect classification")
	}
}

// TestNegativeControl_AllIdentityManagerComparableEqualRegionsAreMutationSensitive
// proves that every comparable, currently-equal region on every identity-
// manager checkpoint (action-menu, delete-choice, confirm-destructive,
// detail-ssh-first) is sensitive to a real TEXT mutation, not just a
// metadata field — mirrors the git-screen sibling exactly.
func TestNegativeControl_AllIdentityManagerComparableEqualRegionsAreMutationSensitive(t *testing.T) {
	home := t.TempDir()
	deterministicReusableKeyFixture(t, home)
	t.Setenv("HOME", home)
	realB := newBackendForHome(home)
	realCaptures, err := screenshot.CaptureCreateFlowScreens(realB)
	if err != nil {
		t.Fatalf("capturing real backend: %v", err)
	}
	dummyB := dummytui.NewFixtureBackend()
	dummyCaptures, err := screenshot.CaptureCreateFlowScreens(dummyB)
	if err != nil {
		t.Fatalf("capturing dummy backend: %v", err)
	}
	gitHome := t.TempDir()
	deterministicGitIdentityFixture(t, gitHome)
	mergeGitScreenCaptures(t, realCaptures, dummyCaptures, gitHome)
	imgrHome := t.TempDir()
	deterministicIdentityManagerFixture(t, imgrHome)
	mergeIdentityManagerCaptures(t, realCaptures, dummyCaptures, imgrHome)
	// 06-07-PLAN.md Task 1: the Global SSH registry shares this spec set, so
	// its frames must be present for BuildRegionDiffs regardless of which
	// registry this control scopes its own mutation to.
	gssHome := t.TempDir()
	deterministicGlobalSSHFixture(t, gssHome)
	mergeGlobalSSHCaptures(t, realCaptures, dummyCaptures, gssHome)
	t.Setenv("HOME", home)

	specs := screenshot.RequiredScreenSpecs()
	records, err := screenshot.BuildRegionDiffs("negative-control", realCaptures, dummyCaptures, specs)
	if err != nil {
		t.Fatalf("building classified region evidence: %v", err)
	}
	assertAllComparableEqualRegionsAreMutationSensitive(t, specs, records, func(screenID string) bool {
		return identityManagerScreenIDs[screenID]
	})
}

// TestNegativeControl_IdentityManagerCrossRegistryLeakage proves every Phase
// 5 identity-manager spec's decision references use ONLY the scoped
// DLV-NN/MGR-D-NN vocabulary — never a bare D-NN/T-NN ref that would
// silently resolve against create-flow's own 03-CONTEXT.md numbering
// instead of 05-CONTEXT.md's — mirrors TestNegativeControl_CrossRegistryLeakage
// exactly, scoped to identityManagerScreenIDs.
func TestNegativeControl_IdentityManagerCrossRegistryLeakage(t *testing.T) {
	found := false
	for _, spec := range screenshot.RequiredScreenSpecs() {
		if !identityManagerScreenIDs[spec.ScreenID] {
			continue
		}
		found = true
		for _, d := range spec.RegionDispositions {
			if !strings.HasPrefix(d.Decision, "DLV-") && !strings.HasPrefix(d.Decision, "MGR-D-") {
				t.Errorf("cross-registry leakage: identity-manager spec %q region %q disposition uses decision ref %q — must be scoped DLV-NN/MGR-D-NN", spec.ScreenID, d.Region, d.Decision)
			}
		}
		for _, na := range spec.NonApplicability {
			if !strings.HasPrefix(na.Decision, "DLV-") && !strings.HasPrefix(na.Decision, "MGR-D-") {
				t.Errorf("cross-registry leakage: identity-manager spec %q surface %q non-applicability uses decision ref %q — must be scoped DLV-NN/MGR-D-NN", spec.ScreenID, na.Surface, na.Decision)
			}
		}
	}
	if !found {
		t.Fatal("cross-registry leakage: no identity-manager specs found in RequiredScreenSpecs() — the registry merge broke")
	}
}

// ---------------------------------------------------------------------------
// 06-07-PLAN.md Task 1 (Phase 6 registration): the Global SSH checkpoints'
// deterministic fixture, merge, acceptance tests, and four negative controls —
// mirroring the Phase 4 and Phase 5 sets by name and by shape.
// ---------------------------------------------------------------------------

// globalSSHScreenIDs is the known Phase 6 Global SSH checkpoint vocabulary,
// duplicated here (internal/screenshot's globalSSHSpecs is unexported) so the
// negative controls and the count assertion can scope themselves to Phase 6
// without a new export surface — mirrors gitScreenScreenIDs/
// identityManagerScreenIDs' own precedent.
var globalSSHScreenIDs = map[string]bool{
	"gss-options-list": true, "gss-storage-current": true, "gss-storage-other": true,
	"gss-apply-preview": true, "gss-apply-receipt": true,
	"gss-storage-migrate-preview": true, "gss-storage-migrate-receipt": true,
}

// preGlobalSSHScreenIDs is the complete pre-Phase-6 registry vocabulary the
// four-way merge must leave untouched: 18 create-flow + 5 git-screen + 4
// identity-manager checkpoint IDs. Hardcoded here so
// TestGlobalSSHRegistryFrameCountIncrease proves the frame count rose by
// EXACTLY the number of Global SSH specs registered (06-07-PLAN.md Task 1
// acceptance criterion) rather than by an unexamined drift.
var preGlobalSSHScreenIDs = []string{
	"ssh-form-filled", "reuse-key-vs-generate", "reuse-manual-path", "mouse-focused-field",
	"test-stage1-direct", "test-stage2-by-alias", "git-form-demo", "confirm-write",
	"reuse-manual-resolved", "test-stage1-pass", "test-stage1-command-output",
	"test-stage2-command-output", "test-stage2-resolution-user-host-port",
	"test-stage2-resolution-identities-key", "test-reachable-not-uploaded",
	"test-hard-failure-retry", "confirm-summary-key-path", "confirm-managed-block",
	"git-form-filled", "git-form-empty", "match-strategy-select", "review-readonly", "result-success",
	"action-menu", "delete-choice", "confirm-destructive", "detail-ssh-first",
}

// globalSSHPTYFrameDir is the committed PTY evidence directory the receipt
// states' non-applicability records must point at — resolved from the repo
// root so the assertion works from any working directory.
func globalSSHPTYFrameDir() string {
	return filepath.Join("..", "..", ".planning", "phases", "06-global-ssh-options", "ui-frames")
}

// deterministicGlobalSSHFixture seeds home with the Include-layout config
// graph the Phase 6 Global SSH registry (06-07-PLAN.md Task 1,
// screenshot.CaptureGlobalSSHScreens) needs: an Include line (floored from
// the fixture's own SSH dir), the Include'd gitid-owned file, and a bare
// `Host *` block carrying TWO directives that make the D-01 probe classify
// four distinct row states WITHOUT depending on this machine's real ssh
// defaults — ForwardAgent yes (recommended no -> StateDiffers,
// SourceGitidParsed) and AddKeysToAgent yes (recommended yes ->
// StateAlreadySet). StrictHostKeyChecking/HashKnownHosts/UseKeychain stay
// unset (StateNeedsAction via the isolated baseline) and IdentitiesOnly has
// zero gitid-managed aliases to verify (StateNotApplicable/ReasonNothingToVerify).
// Names deliberately avoid collision with the create-flow "acme" default and
// the git-screen/identity-manager fixtures (all run against SEPARATE,
// dedicated HOMEs, the same isolation precedent as mergeGitScreenCaptures).
func deterministicGlobalSSHFixture(t *testing.T, home string) {
	t.Helper()
	sshDir := filepath.Join(home, ".ssh")
	includeDir := filepath.Join(sshDir, "config.d")
	if err := os.MkdirAll(includeDir, 0o700); err != nil {
		t.Fatalf("gate-visual-regression: seeding global-ssh %s: %v", includeDir, err)
	}

	main := "Include " + filepath.Join(includeDir, "*.config") + "\n\n" +
		"Host *\n" +
		"  ForwardAgent yes\n" +
		"  AddKeysToAgent yes\n"
	if err := os.WriteFile(filepath.Join(sshDir, "config"), []byte(main), 0o600); err != nil {
		t.Fatalf("gate-visual-regression: writing global-ssh fixture ssh/config: %v", err)
	}
	owned := "Host placeholder.invalid\n  User git\n"
	if err := os.WriteFile(filepath.Join(includeDir, "gitid.config"), []byte(owned), 0o600); err != nil {
		t.Fatalf("gate-visual-regression: writing global-ssh fixture owned file: %v", err)
	}
}

// mergeGlobalSSHCaptures captures the Phase 6 Global SSH checkpoints
// (06-07-PLAN.md Task 1, screenshot.CaptureGlobalSSHScreens) for both the
// real backend (seeded from gssHome via deterministicGlobalSSHFixture) and
// the dummy backend, merging each into the caller's realCaptures/
// dummyCaptures maps — mirrors mergeGitScreenCaptures exactly, including its
// temporary $HOME override, so the fixture's extra identities/config never
// perturb any previously registered surface's capture (T-06-46).
func mergeGlobalSSHCaptures(t *testing.T, realCaptures, dummyCaptures map[string]string, gssHome string) {
	t.Helper()
	restoreHome := os.Getenv("HOME")
	t.Setenv("HOME", gssHome)
	gssRealBackend := newBackendForHome(gssHome)
	rawGssReal, err := screenshot.CaptureGlobalSSHScreens(gssRealBackend)
	if err != nil {
		t.Fatalf("gate-visual-regression: capturing global-ssh real backend: %v", err)
	}
	t.Setenv("HOME", restoreHome)
	gssReal := normalizeDisposableHome(rawGssReal, gssHome)
	for id, text := range gssReal {
		realCaptures[id] = text
	}

	gssDummy, err := screenshot.CaptureGlobalSSHScreens(dummytui.NewFixtureBackend())
	if err != nil {
		t.Fatalf("gate-visual-regression: capturing global-ssh dummy backend: %v", err)
	}
	for id, text := range gssDummy {
		dummyCaptures[id] = text
	}
}

// TestGlobalSSHRegistryFrameCountIncrease proves the registry frame count
// rose by EXACTLY the number of Global SSH specs registered (Task 1
// acceptance criterion: "its output reports a frame count that increased by
// exactly the number of Global SSH specs registered") and that every
// pre-Phase-6 screen survived the four-way merge untouched.
func TestGlobalSSHRegistryFrameCountIncrease(t *testing.T) {
	specs := screenshot.RequiredScreenSpecs()
	byID := make(map[string]screenshot.ScreenSpec, len(specs))
	for _, s := range specs {
		byID[s.ScreenID] = s
	}
	for _, id := range preGlobalSSHScreenIDs {
		if _, ok := byID[id]; !ok {
			t.Errorf("registry frame-count: pre-existing screen %q missing from the four-way registry", id)
		}
	}
	if len(byID) != len(preGlobalSSHScreenIDs)+len(globalSSHScreenIDs) {
		t.Errorf("registry frame-count: got %d frames, want %d (%d pre-existing + %d Global SSH registered)",
			len(byID), len(preGlobalSSHScreenIDs)+len(globalSSHScreenIDs), len(preGlobalSSHScreenIDs), len(globalSSHScreenIDs))
	}
	for id := range globalSSHScreenIDs {
		if _, ok := byID[id]; !ok {
			t.Errorf("registry frame-count: Global SSH screen %q missing from the registry", id)
		}
	}
	t.Logf("gate-visual-regression: frame count preamble — %d pre-existing + %d Global SSH = %d (06-07 adds exactly the Phase 6 specs)",
		len(preGlobalSSHScreenIDs), len(globalSSHScreenIDs), len(byID))
}

// TestGlobalSSHHTMLNonApplicabilityPerSpec proves EVERY Global SSH spec
// records the approved-HTML surface as explicitly non-applicable with a
// non-empty reason naming the standing UI-reference rule, and that the
// approved-TUI surface is applicable on the capturable states and declared
// non-applicable on the two receipt states (T-06-45).
func TestGlobalSSHHTMLNonApplicabilityPerSpec(t *testing.T) {
	found := 0
	for _, spec := range screenshot.RequiredScreenSpecs() {
		if !globalSSHScreenIDs[spec.ScreenID] {
			continue
		}
		found++
		record, ok := screenshot.NonApplicabilityForSurface(spec, "approved-html")
		if !ok {
			t.Errorf("Global SSH spec %q has no approved-html non-applicability record", spec.ScreenID)
			continue
		}
		if strings.TrimSpace(record.Reason) == "" {
			t.Errorf("Global SSH spec %q approved-html non-applicability has an empty reason", spec.ScreenID)
		}
		if !strings.Contains(record.Reason, "UI Reference") && !strings.Contains(record.Reason, "Bubble Tea dummy") && !strings.Contains(record.Reason, "HTML/MUI") {
			t.Errorf("Global SSH spec %q approved-html reason does not name the standing UI-reference rule: %q", spec.ScreenID, record.Reason)
		}
	}
	if found != len(globalSSHScreenIDs) {
		t.Fatalf("Global SSH HTML audit found %d specs in the registry, want %d", found, len(globalSSHScreenIDs))
	}
}

// TestGlobalSSHNonApplicabilityNamesExistingPTYFrame proves every Global SSH
// spec that declares any OTHER surface non-applicable (live or approved-tui;
// i.e. the two receipt states) carries a reason naming a SPECIFIC PTY frame
// file, and that the named file exists under the phase's ui-frames/
// directory — the T-06-44 anti-hollow-capture contract (a pointer to "the
// PTY suite" is not evidence).
func TestGlobalSSHNonApplicabilityNamesExistingPTYFrame(t *testing.T) {
	frameDir := globalSSHPTYFrameDir()
	if _, err := os.Stat(frameDir); err != nil {
		t.Fatalf("Global SSH ui-frames dir %s is not readable: %v", frameDir, err)
	}
	specs := screenshot.RequiredScreenSpecs()
	for _, spec := range specs {
		if !globalSSHScreenIDs[spec.ScreenID] {
			continue
		}
		for surface, skipped := range map[string]bool{"live": spec.ApplicableLive, "approved-tui": spec.ApplicableApprovedTUI} {
			if skipped {
				continue
			}
			record, ok := screenshot.NonApplicabilityForSurface(spec, surface)
			if !ok {
				t.Errorf("Global SSH spec %q surface %q is non-applicable but has no record", spec.ScreenID, surface)
				continue
			}
			frame := namedPTYFrame(t, record.Reason, spec.ScreenID)
			if frame == "" {
				t.Errorf("Global SSH spec %q surface %q non-applicability names no ui-frames/ file: %q", spec.ScreenID, surface, record.Reason)
				continue
			}
			if _, err := os.Stat(filepath.Join(frameDir, frame)); err != nil {
				t.Errorf("Global SSH spec %q surface %q names PTY frame %s which does not exist under %s", spec.ScreenID, surface, frame, frameDir)
			}
		}
	}
}

// namedPTYFrame extracts the ui-frames/<name>.txt file a reason string names.
func namedPTYFrame(t *testing.T, reason, screenID string) string {
	t.Helper()
	m := regexp.MustCompile(`ui-frames/([A-Za-z0-9._-]+\.txt)`).FindStringSubmatch(reason)
	if len(m) == 2 {
		return m[1]
	}
	return ""
}

// TestGlobalSSHFixtureCoversFourOptionStates proves the deterministic fixture
// produces at least one row in each of the four option states (Task 1
// acceptance criterion) — so the comparison never exercises a single state.
func TestGlobalSSHFixtureCoversFourOptionStates(t *testing.T) {
	home := t.TempDir()
	deterministicGlobalSSHFixture(t, home)
	restore := os.Getenv("HOME")
	t.Setenv("HOME", home)
	defer t.Setenv("HOME", restore)
	states, err := newBackendForHome(home).GlobalSSHOptionStates()
	if err != nil {
		t.Fatalf("reading fixture option states: %v", err)
	}
	labels := map[tuikit.GlobalSSHOptionState]string{
		tuikit.GlobalSSHNeedsAction: "needs-action", tuikit.GlobalSSHAlreadySet: "already-set",
		tuikit.GlobalSSHDiffers: "set-differs", tuikit.GlobalSSHNotApplicable: "not-applicable",
	}
	for _, o := range states {
		if _, ok := labels[o.State]; !ok {
			t.Errorf("fixture option %q reports unknown state %d", o.Key, o.State)
		}
	}
	for state, want := range labels {
		n := 0
		for _, o := range states {
			if o.State == state {
				n++
			}
		}
		if n == 0 {
			t.Errorf("deterministicGlobalSSHFixture produced no %s row (all states: %v)", want, stateLabels(states, labels))
		}
	}
}

// stateLabels renders every fixture row's state name for a failure message.
func stateLabels(states []tuikit.GlobalSSHOptionView, labels map[tuikit.GlobalSSHOptionState]string) string {
	var out []string
	for _, o := range states {
		out = append(out, o.Key+"="+labels[o.State])
	}
	return strings.Join(out, " ")
}

// TestGlobalSSHAllowlistMatchesRegistry parses
// .planning/design/global-ssh/visual-divergence-allowlist.txt and proves a
// byte 1:1 correspondence with the Global SSH specs' code dispositions —
// every allowlist entry maps to a registry disposition and vice versa — and
// that the three REQUIRED entries exist by name (T-06-GLOBALBLOCK,
// T-06-CEREMONYTARGET, T-06-PROVENANCE). THE FILE IS THE GATE'S CHECKED-IN
// CLASSIFICATION: removing a disposition (or widening a registry predicate)
// fails this test before the gate can ever pass silently.
func TestGlobalSSHAllowlistMatchesRegistry(t *testing.T) {
	entries, err := readDivergenceAllowlist(filepath.Join("..", "..", ".planning", "design", "global-ssh", "visual-divergence-allowlist.txt"))
	if err != nil {
		t.Fatalf("reading the Global SSH allowlist: %v", err)
	}
	ids := make(map[string]bool, len(entries))
	for _, e := range entries {
		if ids[e.Name] {
			t.Errorf("allowlist duplicate entry name %q", e.Name)
		}
		ids[e.Name] = true
	}
	for _, want := range []string{"T-06-GLOBALBLOCK", "T-06-CEREMONYTARGET", "T-06-PROVENANCE"} {
		if !ids[want] {
			t.Errorf("allowlist is missing the REQUIRED entry %q — a known divergence would fail as an unexplained gate failure", want)
		}
	}
	// T-06-GLOBALBLOCK's entry must pin ALL THREE aspects of that recorded
	// divergence (06-01): the guard-line placement, the two-vs-four-space
	// indent, and the Policy-ordered key sequence — removing any one of the
	// needles from the file is a gate failure.
	byName := make(map[string]allowlistEntry, len(entries))
	for _, e := range entries {
		byName[e.Name] = e
	}
	block, ok := byName["T-06-GLOBALBLOCK"]
	if !ok {
		t.Fatal("T-06-GLOBALBLOCK entry missing after name audit")
	}
	for _, needle := range []string{"IgnoreUnknown UseKeychain", "two-space", "Policy"} {
		if !strings.Contains(block.Reason, needle) {
			t.Errorf("T-06-GLOBALBLOCK reason does not pin needle %q — that aspect of the divergence is unclassified", needle)
		}
	}
	// Byte 1:1 with the registry dispositions.
	registry := make(map[string]screenshot.RegionDisposition)
	for _, spec := range screenshot.RequiredScreenSpecs() {
		if !globalSSHScreenIDs[spec.ScreenID] {
			continue
		}
		for _, d := range spec.RegionDispositions {
			registry[spec.ScreenID+":"+string(d.Region)] = d
		}
	}
	entryKey := func(e allowlistEntry) string { return e.ScreenID + ":" + e.Region }
	entryMap := make(map[string]allowlistEntry, len(entries))
	for _, e := range entries {
		entryMap[entryKey(e)] = e
	}
	for key, d := range registry {
		e, ok := entryMap[key]
		if !ok {
			t.Errorf("registry disposition %s has no allowlist entry — the classification is not recorded", key)
			continue
		}
		if e.Predicate != d.Predicate {
			t.Errorf("allowlist entry %s predicate %q != registry predicate %q", key, e.Predicate, d.Predicate)
		}
		if e.Decision != d.Decision {
			t.Errorf("allowlist entry %s decision %q != registry decision %q", key, e.Decision, d.Decision)
		}
	}
	for key := range entryMap {
		if _, ok := registry[key]; !ok {
			t.Errorf("allowlist entry %s has no matching registry disposition — stale entry (a removed allowlisted divergence) must be deleted", key)
		}
	}
}

// TestGlobalSSHAllowlistFormat proves the allowlist file's schema: every
// entry states a classification and no predicate is broader than a specific
// needle (each begins contains:/absent: with a non-empty quoted needle).
func TestGlobalSSHAllowlistFormat(t *testing.T) {
	entries, err := readDivergenceAllowlist(filepath.Join("..", "..", ".planning", "design", "global-ssh", "visual-divergence-allowlist.txt"))
	if err != nil {
		t.Fatalf("reading the Global SSH allowlist: %v", err)
	}
	for _, e := range entries {
		if !strings.HasPrefix(e.Classification, "improvement") && !strings.HasPrefix(e.Classification, "defect") {
			t.Errorf("allowlist entry %q has no improvement/defect classification: %q", e.Name, e.Classification)
		}
		if !strings.HasPrefix(e.Predicate, `contains:"`) && !strings.HasPrefix(e.Predicate, `absent:"`) {
			t.Errorf("allowlist entry %q uses a non-needle predicate %q — a broad predicate would match unrelated text", e.Name, e.Predicate)
		} else {
			needle := strings.Trim(e.Predicate[strings.Index(e.Predicate, ":"):], `" `)
			if strings.TrimSpace(needle) == "" {
				t.Errorf("allowlist entry %q predicate %q has an empty needle", e.Name, e.Predicate)
			}
		}
	}
}

// TestGlobalSSHMakefileFilterSelectsControls asserts the Makefile's
// gate-visual-regression test-name filter selects all four Global SSH
// negative controls (by name-prefix comparison), so a rename cannot silently
// orphan a control.
func TestGlobalSSHMakefileFilterSelectsControls(t *testing.T) {
	makefile, err := os.ReadFile(filepath.Join("..", "..", "Makefile"))
	if err != nil {
		t.Skipf("cannot read Makefile: %v", err)
	}
	lines := strings.Split(string(makefile), "\n")
	targetIdx := -1
	for i, line := range lines {
		if strings.HasPrefix(line, "gate-visual-regression:") {
			targetIdx = i
			break
		}
	}
	if targetIdx < 0 {
		t.Fatal("Makefile has no gate-visual-regression target to inspect")
	}
	filter := ""
	for _, line := range lines[targetIdx+1:] {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(line, "\t") || strings.HasPrefix(line, " ") {
			if strings.HasPrefix(trimmed, "go test -tags screenshot -run") {
				filter = trimmed
				break
			}
			if trimmed != "" {
				// Still inside the recipe body but not the -run line yet.
				continue
			}
		}
		// A non-indented, non-blank line ends the target's recipe.
		break
	}
	if filter == "" {
		t.Fatal("gate-visual-regression target has no -run filter line to inspect")
	}
	start := strings.Index(filter, "'Test(")
	if start < 0 {
		t.Fatalf("gate filter %q carries no 'Test( pattern to assert against", filter)
	}
	end := strings.LastIndex(filter, ")")
	if end < 0 || end <= start {
		t.Fatalf("gate filter %q has an unbalanced pattern", filter)
	}
	pattern := filter[start+1 : end+1]
	re, err := regexp.Compile(pattern)
	if err != nil {
		t.Fatalf("gate filter %q is not a valid Go regexp: %v", pattern, err)
	}
	controls := []string{
		"TestNegativeControl_MissingGlobalSSHState",
		"TestNegativeControl_GlobalSSHUnclassifiedDifferenceRejected",
		"TestNegativeControl_AllGlobalSSHComparableEqualRegionsAreMutationSensitive",
		"TestNegativeControl_GlobalSSHCrossRegistryLeakage",
	}
	for _, c := range controls {
		if !re.MatchString(c) {
			t.Errorf("Makefile gate filter %q does not select control %q — a rename has silently orphaned it", pattern, c)
		}
	}
	for _, c := range controls {
		if !strings.HasPrefix(c, "TestNegativeControl_") {
			t.Errorf("control %q does not carry the named-prefix contract %q", c, "TestNegativeControl_")
		}
	}
}

// TestNegativeControl_MissingGlobalSSHState proves the gate fails closed when
// a required Phase 6 Global SSH live capture is missing — CR-05's fail-closed
// contract is not vacuous for the Global SSH registry specifically.
func TestNegativeControl_MissingGlobalSSHState(t *testing.T) {
	home := t.TempDir()
	deterministicGlobalSSHFixture(t, home)
	restore := os.Getenv("HOME")
	t.Setenv("HOME", home)
	realB := newBackendForHome(home)
	captures, err := screenshot.CaptureGlobalSSHScreens(realB)
	if err != nil {
		t.Fatalf("capturing global-ssh real backend: %v", err)
	}
	t.Setenv("HOME", restore)
	broken := make(map[string]string, len(captures))
	for id, text := range captures {
		if id == "gss-options-list" {
			continue // deliberately drop a required Global SSH state
		}
		broken[id] = text
	}
	if _, ok := broken["gss-options-list"]; ok {
		t.Fatal("negative-control setup bug: \"gss-options-list\" was not actually dropped")
	}
	specs := screenshot.RequiredScreenSpecs()
	if _, err := screenshot.BuildRegionDiffs("negative-control", broken, broken, specs); err == nil {
		t.Fatal("negative-control: BuildRegionDiffs accepted a capture set missing the required \"gss-options-list\" frame — CR-05 fail-closed is NOT enforced for Phase 6 states")
	}
}

// TestNegativeControl_GlobalSSHUnclassifiedDifferenceRejected proves that
// mutating away a Phase 6 Global SSH RegionDisposition's classification is
// caught — the same protection the generic control proves, scoped explicitly
// to a Global SSH record.
func TestNegativeControl_GlobalSSHUnclassifiedDifferenceRejected(t *testing.T) {
	home := t.TempDir()
	deterministicReusableKeyFixture(t, home)
	t.Setenv("HOME", home)
	realB := newBackendForHome(home)
	realCaptures, err := screenshot.CaptureCreateFlowScreens(realB)
	if err != nil {
		t.Fatalf("capturing real backend: %v", err)
	}
	dummyB := dummytui.NewFixtureBackend()
	dummyCaptures, err := screenshot.CaptureCreateFlowScreens(dummyB)
	if err != nil {
		t.Fatalf("capturing dummy backend: %v", err)
	}
	gitHome := t.TempDir()
	deterministicGitIdentityFixture(t, gitHome)
	mergeGitScreenCaptures(t, realCaptures, dummyCaptures, gitHome)
	imgrHome := t.TempDir()
	deterministicIdentityManagerFixture(t, imgrHome)
	mergeIdentityManagerCaptures(t, realCaptures, dummyCaptures, imgrHome)
	gssHome := t.TempDir()
	deterministicGlobalSSHFixture(t, gssHome)
	mergeGlobalSSHCaptures(t, realCaptures, dummyCaptures, gssHome)
	t.Setenv("HOME", home)

	specs := screenshot.RequiredScreenSpecs()
	records, err := screenshot.BuildRegionDiffs("negative-control", realCaptures, dummyCaptures, specs)
	if err != nil {
		t.Fatalf("building classified region evidence: %v", err)
	}
	mutated := false
	for i := range records {
		if !globalSSHScreenIDs[records[i].ScreenID] {
			continue // scope this control to Phase 6 Global SSH records only
		}
		for j := range records[i].Regions {
			region := &records[i].Regions[j]
			if !region.Comparable || !region.Equal {
				region.Classification = ""
				mutated = true
				break
			}
		}
		if mutated {
			break
		}
	}
	if !mutated {
		t.Fatal("negative-control: no classified Global SSH difference was available to mutate")
	}
	data, err := json.Marshal(screenshot.RegionDiffs{Version: "test", SourceCommit: "negative-control", GeneratedAt: "test", Screens: records})
	if err != nil {
		t.Fatalf("marshaling mutated region evidence: %v", err)
	}
	if err := screenshot.ValidateRegionDiffs(data, "negative-control", specs); err == nil {
		t.Fatal("negative-control: validator accepted a Phase 6 Global SSH difference without ux-improvement/defect classification")
	}
}

// TestNegativeControl_AllGlobalSSHComparableEqualRegionsAreMutationSensitive
// proves every comparable, currently-equal region on every Global SSH
// checkpoint is sensitive to a real TEXT mutation, not just a metadata field.
func TestNegativeControl_AllGlobalSSHComparableEqualRegionsAreMutationSensitive(t *testing.T) {
	home := t.TempDir()
	deterministicReusableKeyFixture(t, home)
	t.Setenv("HOME", home)
	realB := newBackendForHome(home)
	realCaptures, err := screenshot.CaptureCreateFlowScreens(realB)
	if err != nil {
		t.Fatalf("capturing real backend: %v", err)
	}
	dummyB := dummytui.NewFixtureBackend()
	dummyCaptures, err := screenshot.CaptureCreateFlowScreens(dummyB)
	if err != nil {
		t.Fatalf("capturing dummy backend: %v", err)
	}
	gitHome := t.TempDir()
	deterministicGitIdentityFixture(t, gitHome)
	mergeGitScreenCaptures(t, realCaptures, dummyCaptures, gitHome)
	imgrHome := t.TempDir()
	deterministicIdentityManagerFixture(t, imgrHome)
	mergeIdentityManagerCaptures(t, realCaptures, dummyCaptures, imgrHome)
	gssHome := t.TempDir()
	deterministicGlobalSSHFixture(t, gssHome)
	mergeGlobalSSHCaptures(t, realCaptures, dummyCaptures, gssHome)
	t.Setenv("HOME", home)

	specs := screenshot.RequiredScreenSpecs()
	records, err := screenshot.BuildRegionDiffs("negative-control", realCaptures, dummyCaptures, specs)
	if err != nil {
		t.Fatalf("building classified region evidence: %v", err)
	}
	assertAllComparableEqualRegionsAreMutationSensitive(t, specs, records, func(screenID string) bool {
		return globalSSHScreenIDs[screenID]
	})
}

// TestNegativeControl_GlobalSSHCrossRegistryLeakage proves every Phase 6
// Global SSH spec's decision references use ONLY the scoped
// GSSH-D-NN/STORE-NN/DLV-NN vocabulary — never a bare D-NN/T-NN ref
// (03-CONTEXT.md's namespace), never CTX-D-/UI-D- (04-CONTEXT.md's), never
// MGR-D- (05-CONTEXT.md's) — mirroring the Phase 4 and Phase 5 leakage
// controls exactly. DLV- remains valid because it is a shared REQUIREMENTS
// literal (DLV-04), the same exception Phase 5's own control grants.
func TestNegativeControl_GlobalSSHCrossRegistryLeakage(t *testing.T) {
	found := false
	for _, spec := range screenshot.RequiredScreenSpecs() {
		if !globalSSHScreenIDs[spec.ScreenID] {
			continue
		}
		found = true
		for _, d := range spec.RegionDispositions {
			if !isGlobalSSHScopedRef(d.Decision) {
				t.Errorf("cross-registry leakage: Global SSH spec %q region %q disposition uses decision ref %q — must be scoped GSSH-D-NN/STORE-NN/DLV-NN (never bare D-/T-, CTX-D-/UI-D-, or MGR-D-)", spec.ScreenID, d.Region, d.Decision)
			}
		}
		for _, na := range spec.NonApplicability {
			if !isGlobalSSHScopedRef(na.Decision) {
				t.Errorf("cross-registry leakage: Global SSH spec %q surface %q non-applicability uses decision ref %q — must be scoped GSSH-D-NN/STORE-NN/DLV-NN", spec.ScreenID, na.Surface, na.Decision)
			}
		}
	}
	if !found {
		t.Fatal("cross-registry leakage: no Global SSH specs found in RequiredScreenSpecs() — the registry merge broke")
	}
}

// isGlobalSSHScopedRef reports whether ref uses the Phase-6 scoped vocabulary.
func isGlobalSSHScopedRef(ref string) bool {
	return strings.HasPrefix(ref, "GSSH-D-") || strings.HasPrefix(ref, "STORE-") || strings.HasPrefix(ref, "DLV-")
}

// TestGlobalSSHFixtureDeterminismAcrossRuns proves the Global SSH fixture's
// captures are byte-identical across two independent runs in the same test
// process (CR-01's across-runs determinism contract, scoped to Phase 6).
func TestGlobalSSHFixtureDeterminismAcrossRuns(t *testing.T) {
	home1 := t.TempDir()
	home2 := t.TempDir()
	deterministicGlobalSSHFixture(t, home1)
	deterministicGlobalSSHFixture(t, home2)

	restore := os.Getenv("HOME")
	t.Setenv("HOME", home1)
	r1, err := screenshot.CaptureGlobalSSHScreens(newBackendForHome(home1))
	if err != nil {
		t.Fatalf("run 1: %v", err)
	}
	t.Setenv("HOME", home2)
	r2, err := screenshot.CaptureGlobalSSHScreens(newBackendForHome(home2))
	if err != nil {
		t.Fatalf("run 2: %v", err)
	}
	t.Setenv("HOME", restore)

	n1 := normalizeDisposableHome(r1, home1)
	n2 := normalizeDisposableHome(r2, home2)
	for id := range globalSSHScreenIDs {
		if a, b := n1[id], n2[id]; a != b {
			t.Errorf("Global SSH screen %q is NOT deterministic across two independent fixtured runs:\n--- run 1 ---\n%s\n--- run 2 ---\n%s", id, screenshot.StripANSIExported(a), screenshot.StripANSIExported(b))
		}
	}
}

// TestGlobalSSHPriorSurfacesUnchanged proves adding the Global SSH fixture
// does not perturb any previously registered surface's capture: the
// create-flow/git-screen/identity-manager frames captured WITH the Global SSH
// merge are byte-identical to the same frames captured WITHOUT it (T-06-46).
func TestGlobalSSHPriorSurfacesUnchanged(t *testing.T) {
	home := t.TempDir()
	deterministicReusableKeyFixture(t, home)
	t.Setenv("HOME", home)
	realB := newBackendForHome(home)
	dummyB := dummytui.NewFixtureBackend()

	baselineReal, err := screenshot.CaptureCreateFlowScreens(realB)
	if err != nil {
		t.Fatalf("baseline real: %v", err)
	}
	baselineDummy, err := screenshot.CaptureCreateFlowScreens(dummyB)
	if err != nil {
		t.Fatalf("baseline dummy: %v", err)
	}
	gitHome := t.TempDir()
	deterministicGitIdentityFixture(t, gitHome)
	mergeGitScreenCaptures(t, baselineReal, baselineDummy, gitHome)
	imgrHome := t.TempDir()
	deterministicIdentityManagerFixture(t, imgrHome)
	mergeIdentityManagerCaptures(t, baselineReal, baselineDummy, imgrHome)

	// Re-capture the prior surfaces with the Global SSH merge in play.
	real2, err := screenshot.CaptureCreateFlowScreens(realB)
	if err != nil {
		t.Fatalf("post-gss real: %v", err)
	}
	dummy2, err := screenshot.CaptureCreateFlowScreens(dummyB)
	if err != nil {
		t.Fatalf("post-gss dummy: %v", err)
	}
	gitHome2 := t.TempDir()
	deterministicGitIdentityFixture(t, gitHome2)
	mergeGitScreenCaptures(t, real2, dummy2, gitHome2)
	imgrHome2 := t.TempDir()
	deterministicIdentityManagerFixture(t, imgrHome2)
	mergeIdentityManagerCaptures(t, real2, dummy2, imgrHome2)
	gssHome := t.TempDir()
	deterministicGlobalSSHFixture(t, gssHome)
	mergeGlobalSSHCaptures(t, real2, dummy2, gssHome)
	t.Setenv("HOME", home)

	for _, id := range preGlobalSSHScreenIDs {
		if baselineReal[id] != real2[id] {
			t.Errorf("prior surface %q real capture changed after the Global SSH fixture was added", id)
		}
		if baselineDummy[id] != dummy2[id] {
			t.Errorf("prior surface %q dummy capture changed after the Global SSH fixture was added", id)
		}
	}
}

// ---------------------------------------------------------------------------
// Global SSH allowlist file primitives
// ---------------------------------------------------------------------------

// allowlistEntry is one parsed row of the checked-in divergence allowlist.
type allowlistEntry struct {
	Name           string
	ScreenID       string
	Region         string
	Predicate      string
	Decision       string
	Reason         string
	Classification string
}

// allowlistFrameNamePattern matches the ui-frames/<file>.txt pointer syntax.
var allowlistFrameNamePattern = regexp.MustCompile(`ui-frames/([A-Za-z0-9._-]+\.txt)`)

// allowlistLinePattern parses one strict-schema allowlist row:
//
//	<T-06-NAME> <screen-id>:<region>:<predicate>:<decision>:<classification>:<reason>
//
// where predicate is the quoted-needle form contains:"..." / absent:"..."
// (the quoted needle may itself contain colons), the <T-06-NAME> marker is
// optional (defaulting to <screen-id>:<region>), and the reason is the rest
// of the line (it may contain colons). The quoted needle is what makes a
// colon-split parser wrong — a bare strings.SplitN shifts every field after
// `contains:"now:"`.
var allowlistLinePattern = regexp.MustCompile(
	`^(?:<([^>]+)>)?\s*(gss-[A-Za-z0-9-]+):([a-z0-9-]+):(contains:"[^"]*"|absent:"[^"]*"):(GSSH-D-[0-9]+|STORE-[0-9]+|DLV-[0-9]+):(improvement|defect):(.*)$`)

// readDivergenceAllowlist parses the strict 7-field allowlist schema shared
// by every phase's visual-divergence-allowlist.txt (see allowlistLinePattern).
// Blank lines and #-comments are ignored.
func readDivergenceAllowlist(path string) ([]allowlistEntry, error) {
	data, err := os.ReadFile(path) //nolint:gosec // fixed repo-relative path (G304)
	if err != nil {
		return nil, err
	}
	var out []allowlistEntry
	for idx, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		m := allowlistLinePattern.FindStringSubmatch(line)
		if m == nil {
			return nil, fmt.Errorf("allowlist line %d does not match the strict schema: %q", idx+1, line)
		}
		name := ""
		if m[1] != "" {
			name = strings.TrimSpace(m[1])
		}
		entry := allowlistEntry{
			ScreenID:       m[2],
			Region:         m[3],
			Predicate:      m[4],
			Decision:       m[5],
			Classification: m[6],
			Reason:         strings.TrimSpace(m[7]),
		}
		if name == "" {
			name = entry.ScreenID + ":" + entry.Region
		}
		entry.Name = name
		out = append(out, entry)
	}
	return out, nil
}
