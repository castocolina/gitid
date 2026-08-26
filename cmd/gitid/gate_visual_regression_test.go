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
	"strings"
	"sync"
	"testing"

	"github.com/castocolina/gitid/internal/dummytui"
	"github.com/castocolina/gitid/internal/keygen"
	"github.com/castocolina/gitid/internal/screenshot"
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
