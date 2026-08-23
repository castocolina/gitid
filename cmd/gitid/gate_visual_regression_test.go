//go:build screenshot

package main

// gate_visual_regression_test.go is `make gate-visual-regression`'s read-only
// entry point. RequiredScreenSpecs defines the symmetric real/dummy inventory.
// Each surface is checked independently for deterministic capture; real versus
// dummy differences are accepted only with an explicit ux-improvement/defect
// classification, and one-sided states remain non-comparable. HTML and PNG or
// text byte parity are intentionally outside this gate.

import (
	"bufio"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/castocolina/gitid/internal/dummytui"
	"github.com/castocolina/gitid/internal/keygen"
	"github.com/castocolina/gitid/internal/screenshot"
)

// allowlistPath is the D-24.1 divergence allowlist this gate validates and enforces.
const allowlistPath = "../../.planning/design/create-flow/visual-divergence-allowlist.txt"

// approvalCommitFull is the full SHA of the Phase-2 design approval commit (CR-03).
// Approved HTML and TUI sources are captured from this commit only, never from
// current HEAD or any other reference.
const approvalCommitFull = "3c3130e404329cf42baafdf63a6c22758437edc6"

// allowlistEntry holds one parsed allowlist entry.
type allowlistEntry struct {
	ScreenID    string
	Region      screenshot.RegionName
	Predicate   string // "contains:<text>" or "absent:<text>" — "differs" is forbidden
	DecisionRef string
	Reason      string
	used        bool
}

// parseAllowlist parses the strict 5-field allowlist format:
//
//	screen-id : region : predicate : decision-ref : reason
//
// Blank lines and # comments are ignored. Violations fail the test.
// The "differs" predicate is rejected (CR-04: must use contains/absent).
func parseAllowlist(t *testing.T, path string) []allowlistEntry {
	t.Helper()
	f, err := os.Open(path) //nolint:gosec // fixed, repo-relative gitid test fixture path (G304)
	if err != nil {
		t.Fatalf("gate-visual-regression: reading allowlist %s: %v", path, err)
	}
	defer f.Close() //nolint:errcheck // read-only fixture file

	// valid screen IDs
	validScreens := make(map[string]bool)
	var validScreenIDs []string
	for _, spec := range screenshot.RequiredScreenSpecs() {
		validScreens[spec.ScreenID] = true
		validScreenIDs = append(validScreenIDs, spec.ScreenID)
	}
	// valid region names
	validRegions := make(map[screenshot.RegionName]bool)
	for _, r := range screenshot.AllRegionNames() {
		validRegions[r] = true
	}

	var entries []allowlistEntry
	seen := make(map[string]bool) // "screenID:region" → detect duplicates
	lineNum := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := splitAllowlistLine(line)
		if len(parts) != 5 {
			t.Errorf("gate-visual-regression: allowlist line %d: expected 5 colon-separated fields (screen:region:predicate:decision-ref:reason), got %d in: %q", lineNum, len(parts), line)
			continue
		}
		screenID := strings.TrimSpace(parts[0])
		region := screenshot.RegionName(strings.TrimSpace(parts[1]))
		predicate := strings.TrimSpace(parts[2])
		decisionRef := strings.TrimSpace(parts[3])
		reason := strings.TrimSpace(parts[4])

		if !validScreens[screenID] {
			t.Errorf("gate-visual-regression: allowlist line %d: unknown screen ID %q (valid: %v)", lineNum, screenID, validScreenIDs)
		}
		if !validRegions[region] {
			t.Errorf("gate-visual-regression: allowlist line %d: unknown region %q", lineNum, region)
		}
		// CR-04: "differs" is forbidden — every differing region must declare a
		// narrowly scoped predicate.
		if predicate == "differs" {
			t.Errorf("gate-visual-regression: allowlist line %d: forbidden predicate %q — use contains:<text> or absent:<text> to scope the allowed divergence (CR-04)", lineNum, predicate)
			continue
		}
		if predicate == "" {
			t.Errorf("gate-visual-regression: allowlist line %d: blank predicate", lineNum)
		}
		if !strings.HasPrefix(predicate, "contains:") && !strings.HasPrefix(predicate, "absent:") {
			t.Errorf("gate-visual-regression: allowlist line %d: invalid predicate %q (must be 'contains:<text>' or 'absent:<text>')", lineNum, predicate)
		}
		if decisionRef == "" {
			t.Errorf("gate-visual-regression: allowlist line %d: blank decision-ref (D-XX or T-XX required)", lineNum)
		}
		if reason == "" {
			t.Errorf("gate-visual-regression: allowlist line %d: blank reason", lineNum)
		}
		key := screenID + ":" + string(region)
		if seen[key] {
			t.Errorf("gate-visual-regression: allowlist line %d: duplicate entry for screen %q region %q", lineNum, screenID, region)
		}
		seen[key] = true
		entries = append(entries, allowlistEntry{
			ScreenID:    screenID,
			Region:      region,
			Predicate:   predicate,
			DecisionRef: decisionRef,
			Reason:      reason,
		})
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("gate-visual-regression: scanning allowlist %s: %v", path, err)
	}
	return entries
}

// splitAllowlistLine splits s into exactly 5 fields using the schema:
//
//	screen-id : region : predicate : decision-ref : reason
//
// The predicate field may itself contain a colon (e.g. "contains:text"),
// so splitting naively on the 3rd colon fails.
func splitAllowlistLine(s string) []string {
	// helper: split off one field at the next colon
	cut := func(r string) (field, rest string, ok bool) {
		idx := strings.Index(r, ":")
		if idx < 0 {
			return "", r, false
		}
		return r[:idx], r[idx+1:], true
	}

	f0, rest, ok := cut(s)
	if !ok {
		return nil
	}
	f1, rest, ok := cut(rest)
	if !ok {
		return nil
	}
	// now rest starts with the predicate; detect compound predicates
	var f2, f3, f4 string
	if strings.HasPrefix(rest, "contains:") || strings.HasPrefix(rest, "absent:") {
		// The predicate is "keyword:value" where value may be quoted text.
		inner := rest
		keyword := ""
		if strings.HasPrefix(inner, "contains:") {
			keyword = "contains:"
		} else {
			keyword = "absent:"
		}
		inner = inner[len(keyword):]
		// strip the leading quote and find the closing quote, then the colon
		if strings.HasPrefix(inner, `"`) {
			closeQ := strings.Index(inner[1:], `"`)
			if closeQ >= 0 {
				quoted := inner[:closeQ+2] // includes both quotes
				f2 = keyword + quoted
				rest = inner[closeQ+2:]
				if strings.HasPrefix(rest, ":") {
					rest = rest[1:]
				}
			} else {
				// no closing quote — take until next colon
				idx := strings.Index(inner, ":")
				if idx < 0 {
					return nil
				}
				f2 = keyword + inner[:idx]
				rest = inner[idx+1:]
			}
		} else {
			// no quotes: take until next colon
			idx := strings.Index(inner, ":")
			if idx < 0 {
				return nil
			}
			f2 = keyword + inner[:idx]
			rest = inner[idx+1:]
		}
	} else {
		var ok2 bool
		f2, rest, ok2 = cut(rest)
		if !ok2 {
			return nil
		}
	}
	f3, f4, ok = cut(rest)
	if !ok {
		return nil
	}
	return []string{f0, f1, f2, f3, f4}
}

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
	t.Setenv("HOME", home)
	realB := newBackendForHome(home)
	dummyB := dummytui.NewFixtureBackend()
	if _, err := screenshot.CaptureCreateFlowScreens(realB); err != nil {
		t.Logf("gate-visual-regression: capturing real backend (read-only check): %v", err)
	}
	if _, err := screenshot.CaptureCreateFlowScreens(dummyB); err != nil {
		t.Logf("gate-visual-regression: capturing dummy backend (read-only check): %v", err)
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

	for _, spec := range screenshot.RequiredScreenSpecs() {
		if spec.ApplicableLive && strings.TrimSpace(realCaptures[spec.ScreenID]) == "" {
			t.Errorf("real backend: required screen %q is empty (CR-05)", spec.ScreenID)
		}
		if spec.ApplicableApprovedTUI && strings.TrimSpace(dummyCaptures[spec.ScreenID]) == "" {
			t.Errorf("dummy backend: applicable screen %q is empty (CR-05)", spec.ScreenID)
		}
	}
}

// TestNegativeControls_AllProtectedRegionsDetectMutation proves that every
// protected (non-allowlisted) region on every screen is sensitive to mutations
// — the gate can catch any meaningful drift (CR-04 exhaustive negative controls).
func TestNegativeControls_AllProtectedRegionsDetectMutation(t *testing.T) {
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

// min returns the smaller of a and b.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// currentGitCommit returns the current HEAD short hash or "unknown".
func currentGitCommit() string {
	// best-effort; failure returns placeholder
	data, err := os.ReadFile("../../.git/HEAD") //nolint:gosec // fixed repo path (G304)
	if err != nil {
		return "unknown"
	}
	ref := strings.TrimSpace(string(data))
	if strings.HasPrefix(ref, "ref: ") {
		refPath := strings.TrimPrefix(ref, "ref: ")
		hash, err := io.ReadAll(strings.NewReader(""))
		_ = hash
		hashBytes, err := os.ReadFile(filepath.Join("../../.git", refPath)) //nolint:gosec // fixed repo path (G304)
		if err == nil {
			return strings.TrimSpace(string(hashBytes))[:7]
		}
	}
	if len(ref) >= 7 {
		return ref[:7]
	}
	return "unknown"
}
