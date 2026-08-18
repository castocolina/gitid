//go:build screenshot

package main

// gate_visual_regression_test.go is `make gate-visual-regression`'s runnable
// entry point (DLV-04.1/D-24.1, plan 03-06 Task 2): it captures the create-
// flow wizard's rendered text from BOTH binaries — the real cmd/gitid
// Backend (this package's own composition root) and cmd/gitid-dummy's
// FixtureBackend — using the SAME script (internal/screenshot.
// CaptureCreateFlowScreens), then diffs them screen by screen. A screen is
// byte-exact unless its ID is listed in
// .planning/design/create-flow/visual-divergence-allowlist.txt, in which
// case a difference is expected and logged, not failed.

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/castocolina/gitid/internal/dummytui"
	"github.com/castocolina/gitid/internal/keygen"
	"github.com/castocolina/gitid/internal/screenshot"
)

// allowlistPath is the D-24.1 divergence allowlist this gate honors.
const allowlistPath = "../../.planning/design/create-flow/visual-divergence-allowlist.txt"

// loadAllowlist parses path into a set of allowlisted screen IDs — see
// visual-divergence-allowlist.txt's own header comment for the exact
// "<screen-id>: <reason>" format. Blank lines and lines starting with '#'
// (including commented-out entries) are ignored, so a screen ID can be
// documented without being exempted (the D-16 blanket note is deliberately
// commented out — it never applies to this gate's create-flow-only set).
func loadAllowlist(t *testing.T, path string) map[string]string {
	t.Helper()
	f, err := os.Open(path) //nolint:gosec // fixed, repo-relative gitid test fixture path (G304)
	if err != nil {
		t.Fatalf("gate-visual-regression: reading the allowlist %s: %v", path, err)
	}
	defer f.Close() //nolint:errcheck // read-only fixture file

	out := make(map[string]string)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.Index(line, ":")
		if idx < 0 {
			continue
		}
		id := strings.TrimSpace(line[:idx])
		reason := strings.TrimSpace(line[idx+1:])
		out[id] = reason
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("gate-visual-regression: scanning the allowlist %s: %v", path, err)
	}
	return out
}

// seedReusableKeyFixture writes ONE parseable ed25519 key into home/.ssh so
// the real backend's D-10 picker (ScanReusableKeys) renders a POPULATED
// list for the reuse-key-vs-generate/reuse-manual-path screens — an empty
// picker would only ever show "No parseable keys found...", never
// exercising the candidate-row render path DLV-04.1 must gate.
func seedReusableKeyFixture(t *testing.T, home string) {
	t.Helper()
	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatalf("gate-visual-regression: seeding %s: %v", sshDir, err)
	}
	keyPath := filepath.Join(sshDir, "id_ed25519_gate")
	mat, err := keygen.GenerateMaterial(keygen.Params{Algo: "ed25519", Identity: "gate", Comment: "gate@gitid"})
	if err != nil {
		t.Fatalf("gate-visual-regression: generating the reuse-picker fixture key: %v", err)
	}
	if err := os.WriteFile(keyPath, mat.PrivPEM, 0o600); err != nil {
		t.Fatalf("gate-visual-regression: writing the fixture private key: %v", err)
	}
	if err := os.WriteFile(keyPath+".pub", []byte(mat.PubLine+"\n"), 0o644); err != nil { //nolint:gosec // .pub is public key material by definition; hermetic sandbox HOME (G306)
		t.Fatalf("gate-visual-regression: writing the fixture public key: %v", err)
	}
}

// TestGateVisualRegression is `make gate-visual-regression`'s entry point.
// It is byte-exact per screen, modulo the allowlist — no fuzzy/approximate
// diff.
func TestGateVisualRegression(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedReusableKeyFixture(t, home)

	allowlist := loadAllowlist(t, allowlistPath)

	realBackend := newBackendForHome(home)
	dummyBackend := dummytui.NewFixtureBackend()

	realCaptures := screenshot.CaptureCreateFlowScreens(realBackend)
	dummyCaptures := screenshot.CaptureCreateFlowScreens(dummyBackend)

	var unallowlistedFailures int
	for _, id := range screenshot.CreateFlowScreenIDs {
		r, rok := realCaptures[id]
		d, dok := dummyCaptures[id]
		if !rok || !dok {
			t.Errorf("gate-visual-regression: screen %q missing from a capture set (real ok=%v, dummy ok=%v)", id, rok, dok)
			continue
		}
		if r == d {
			continue // byte-identical: always fine, allowlisted or not
		}
		reason, allowed := allowlist[id]
		if !allowed {
			unallowlistedFailures++
			t.Errorf("gate-visual-regression: FAILED — screen %q differs from the dummy golden with NO allowlist entry:\n--- dummy (%s) ---\n%s\n--- real (cmd/gitid) ---\n%s",
				id, id, d, r)
			continue
		}
		t.Logf("gate-visual-regression: screen %q differs — allowlisted (%s)", id, reason)
	}

	if unallowlistedFailures == 0 {
		t.Logf("gate-visual-regression: OK — %d screens captured, all differences allowlisted or byte-identical", len(screenshot.CreateFlowScreenIDs))
	}
}
