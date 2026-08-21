//go:build screenshot

package main

// gate_visual_regression_test.go is `make gate-visual-regression`'s runnable
// entry point (DLV-04.1/D-24.1, plan 03-09 Task 1/2): it captures the
// create-flow wizard's rendered text from BOTH binaries — the real cmd/gitid
// Backend (this package's own composition root) and cmd/gitid-dummy's
// FixtureBackend — using the SAME script (internal/screenshot.CaptureCreateFlowScreens),
// then diffs them REGION by REGION using ExtractRegion.
//
// STRICT SCHEMA (CR-10 fix):
// The allowlist (.planning/design/create-flow/visual-divergence-allowlist.txt)
// must use the exact format:
//
//   screen-id : region : predicate : decision-ref : reason
//
// where predicate is one of:
//   "differs"            — the region text may differ without constraint
//   "contains:<text>"    — region may differ only if it contains <text>
//   "absent:<text>"      — region may differ only if it lacks <text>
//
// The gate:
//   1. Validates every allowlist entry's schema (unknown screen IDs, regions,
//      blank reasons, missing D-XX, duplicates all fail).
//   2. Compares every region of every screen; only the exact region named by
//      a used allowlist entry may differ.
//   3. Requires at least one non-allowlisted region per screen to be byte-exact
//      (so a screen with all regions allowlisted fails even if individual
//      regions are within predicate).
//   4. Fails on unused allowlist entries (stale exemptions are removed).
//   5. Generates a labeled live-TUI contact sheet PNG for the review packet.

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/castocolina/gitid/internal/dummytui"
	"github.com/castocolina/gitid/internal/keygen"
	"github.com/castocolina/gitid/internal/screenshot"
)

// allowlistPath is the D-24.1 divergence allowlist this gate validates and enforces.
const allowlistPath = "../../.planning/design/create-flow/visual-divergence-allowlist.txt"

// allowlistEntry holds one parsed allowlist entry.
type allowlistEntry struct {
	ScreenID    string
	Region      screenshot.RegionName
	Predicate   string // "differs", "contains:<text>", "absent:<text>"
	DecisionRef string
	Reason      string
	used        bool
}

// parseAllowlist parses the strict 5-field allowlist format:
//
//	screen-id : region : predicate : decision-ref : reason
//
// Blank lines and # comments are ignored. Violations fail the test.
func parseAllowlist(t *testing.T, path string) []allowlistEntry {
	t.Helper()
	f, err := os.Open(path) //nolint:gosec // fixed, repo-relative gitid test fixture path (G304)
	if err != nil {
		t.Fatalf("gate-visual-regression: reading allowlist %s: %v", path, err)
	}
	defer f.Close() //nolint:errcheck // read-only fixture file

	// valid screen IDs
	validScreens := make(map[string]bool)
	for _, id := range screenshot.CreateFlowScreenIDs {
		validScreens[id] = true
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
		// split on " : " (space-colon-space) or ":" — tolerate both
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
			t.Errorf("gate-visual-regression: allowlist line %d: unknown screen ID %q (valid: %v)", lineNum, screenID, screenshot.CreateFlowScreenIDs)
		}
		if !validRegions[region] {
			t.Errorf("gate-visual-regression: allowlist line %d: unknown region %q", lineNum, region)
		}
		if predicate == "" {
			t.Errorf("gate-visual-regression: allowlist line %d: blank predicate", lineNum)
		}
		if predicate != "differs" && !strings.HasPrefix(predicate, "contains:") && !strings.HasPrefix(predicate, "absent:") {
			t.Errorf("gate-visual-regression: allowlist line %d: invalid predicate %q (must be 'differs', 'contains:<text>', or 'absent:<text>')", lineNum, predicate)
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
// so splitting naively on the 3rd colon fails. Instead:
//   - split field 0 (screen-id) and field 1 (region) on the first two colons
//   - detect whether field 2 starts with "contains:" or "absent:" (these have
//     an embedded colon) and consume accordingly
//   - split field 3 (decision-ref) and field 4 (reason) on the next two colons
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
		// Find the next colon that is NOT inside a quoted string.
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

// predicateMatches returns true when the allowlist predicate permits the
// difference between real and dummy for this region.
func predicateMatches(predicate, regionText string) bool {
	switch {
	case predicate == "differs":
		return true
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

// eviPath is where the EVIDENCE.json and contact-sheet PNG are written.
const reviewPacketDir = "../../.planning/phases/03-create-flow-backend/03-09-review-packet"

// TestGateVisualRegression is `make gate-visual-regression`'s entry point.
// It is region-exact per screen, modulo the strict allowlist schema.
func TestGateVisualRegression(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedReusableKeyFixture(t, home)

	allowlist := parseAllowlist(t, allowlistPath)
	if t.Failed() {
		t.FailNow() // schema errors prevent a meaningful gate run
	}

	// build per-screen lookup: screenID → []allowlistEntry
	allowed := make(map[string][]allowlistEntry)
	for i := range allowlist {
		id := allowlist[i].ScreenID
		allowed[id] = append(allowed[id], allowlist[i])
	}

	realBackend := newBackendForHome(home)
	dummyBackend := dummytui.NewFixtureBackend()

	realCaptures := screenshot.CaptureCreateFlowScreens(realBackend)
	dummyCaptures := screenshot.CaptureCreateFlowScreens(dummyBackend)

	allRegions := screenshot.AllRegionNames()
	var unallowlistedFailures int
	var predicateFailures int

	for _, id := range screenshot.CreateFlowScreenIDs {
		r, rok := realCaptures[id]
		d, dok := dummyCaptures[id]
		if !rok || !dok {
			t.Errorf("gate-visual-regression: screen %q missing from a capture set (real ok=%v, dummy ok=%v)", id, rok, dok)
			continue
		}

		// track which regions on this screen are allowlisted
		allowedRegions := make(map[screenshot.RegionName]int) // region → index in allowlist
		for i, e := range allowlist {
			if e.ScreenID == id {
				allowedRegions[e.Region] = i
			}
		}

		var nonExemptIdenticalCount int

		for _, region := range allRegions {
			realRegion := screenshot.ExtractRegion(r, region)
			dummyRegion := screenshot.ExtractRegion(d, region)

			if realRegion == dummyRegion {
				// identical: fine regardless of allowlist status
				if _, isAllowed := allowedRegions[region]; !isAllowed {
					nonExemptIdenticalCount++
				}
				continue
			}

			// regions differ — check allowlist
			entryIdx, isAllowed := allowedRegions[region]
			if !isAllowed {
				unallowlistedFailures++
				t.Errorf("gate-visual-regression: FAILED — screen %q region %q differs with NO allowlist entry:\n--- dummy (%s/%s) ---\n%s\n--- real (cmd/gitid) ---\n%s",
					id, region, id, region,
					screenshot.StripANSIExported(dummyRegion),
					screenshot.StripANSIExported(realRegion))
				continue
			}

			// check predicate
			entry := &allowlist[entryIdx]
			if !predicateMatches(entry.Predicate, realRegion) {
				predicateFailures++
				t.Errorf("gate-visual-regression: FAILED — screen %q region %q predicate %q not satisfied by real region text:\n%s",
					id, region, entry.Predicate, screenshot.StripANSIExported(realRegion))
				continue
			}
			entry.used = true
			t.Logf("gate-visual-regression: screen %q region %q differs — allowlisted (%s: %s)", id, region, entry.DecisionRef, entry.Reason)
		}

		if nonExemptIdenticalCount == 0 {
			t.Errorf("gate-visual-regression: screen %q has NO non-allowlisted region that is byte-identical — every region is either allowlisted or differs; ensure at least one structural region (header, breadcrumb, stepper, keybar) is non-exempt and byte-exact", id)
		}
	}

	// stale-entry check: every allowlist entry must have been used
	for _, e := range allowlist {
		if !e.used {
			t.Errorf("gate-visual-regression: STALE allowlist entry — screen %q region %q was not exercised (the region did not differ or was never captured); remove the stale entry", e.ScreenID, e.Region)
		}
	}

	if unallowlistedFailures == 0 && predicateFailures == 0 {
		t.Logf("gate-visual-regression: OK — %d screens, %d regions checked, all differences allowlisted and within predicate", len(screenshot.CreateFlowScreenIDs), len(allRegions)*len(screenshot.CreateFlowScreenIDs))
	}

	// Generate the live-TUI contact sheet PNG and EVIDENCE.json for the review packet
	if !t.Failed() {
		generateLiveTUIContactSheet(t, realCaptures)
	}
}

// generateLiveTUIContactSheet renders each captured real-backend screen to a
// labeled PNG via CaptureTUI and assembles them into a contact sheet.
// The resulting PNG and EVIDENCE.json are written to the review packet dir.
func generateLiveTUIContactSheet(t *testing.T, realCaptures map[string]string) {
	t.Helper()

	fontFile := os.Getenv("SCREENSHOT_FONT")
	if fontFile == "" {
		// resolve from repo root relative to cmd/gitid
		fontFile = "../../.planning/design/fonts/JetBrainsMono-Regular.ttf"
	}
	theme := os.Getenv("SCREENSHOT_THEME")
	if theme == "" {
		theme = "dracula"
	}

	// skip PNG generation if freeze is not installed (test env may lack it)
	if _, err := os.Stat(fontFile); err != nil {
		t.Logf("gate-visual-regression: contact sheet skipped — font file not found at %s (run make setup-env)", fontFile)
		return
	}

	pngDir := filepath.Join(reviewPacketDir, "panel-pngs")
	var hashes []string
	evidence := map[string]interface{}{
		"generated":     time.Now().UTC().Format(time.RFC3339),
		"source_commit": currentGitCommit(),
		"geometry":      fmt.Sprintf("%dx%d", screenshot.CaptureWidth, screenshot.CaptureHeight),
		"font":          filepath.Base(fontFile),
		"theme":         theme,
		"screens":       map[string]string{},
	}
	screensMap := evidence["screens"].(map[string]string)

	for _, id := range screenshot.CreateFlowScreenIDs {
		golden, ok := realCaptures[id]
		if !ok {
			continue
		}
		res, err := screenshot.CaptureTUI(golden, screenshot.TUIOptions{
			FontFile: fontFile,
			Theme:    theme,
			OutDir:   pngDir,
			Name:     id,
			Width:    screenshot.CaptureWidth,
			Height:   screenshot.CaptureHeight,
		})
		if err != nil {
			t.Logf("gate-visual-regression: contact sheet: skipping %q PNG: %v", id, err)
			continue
		}
		hashes = append(hashes, res.SHA256)
		screensMap[id] = res.SHA256
		t.Logf("gate-visual-regression: contact sheet: %s → %s (sha256:%s)", id, res.PNGPath, res.SHA256)
	}

	// write EVIDENCE.json
	evidenceBytes, err := json.MarshalIndent(evidence, "", "  ")
	if err == nil {
		evidencePath := filepath.Join(reviewPacketDir, "EVIDENCE.json")
		if writeErr := os.WriteFile(evidencePath, evidenceBytes, 0o600); writeErr != nil { //nolint:gosec // controlled artifact path (G306)
			t.Logf("gate-visual-regression: EVIDENCE.json write error: %v", writeErr)
		} else {
			t.Logf("gate-visual-regression: EVIDENCE.json written at %s", evidencePath)
		}
	}

	if len(hashes) > 0 {
		t.Logf("gate-visual-regression: live-TUI contact sheet: %d panels rendered to %s", len(hashes), pngDir)
	}
}

// TestGenerateApprovedTUIContactSheet renders the approved Phase-2 TUI
// design (cmd/gitid-dummy's FixtureBackend) to a labeled contact sheet
// for the 03-09 review packet. This is the "approved TUI reference"
// the live gate output is reviewed against.
//
// The dummy backend represents the Phase-2 approved design contract
// (DLV-08, 2026-07-06 by Pepe). Each panel is labeled with its screen ID.
func TestGenerateApprovedTUIContactSheet(t *testing.T) {
	fontFile := os.Getenv("SCREENSHOT_FONT")
	if fontFile == "" {
		fontFile = "../../.planning/design/fonts/JetBrainsMono-Regular.ttf"
	}
	theme := os.Getenv("SCREENSHOT_THEME")
	if theme == "" {
		theme = "dracula"
	}
	if _, err := os.Stat(fontFile); err != nil {
		t.Skipf("TestGenerateApprovedTUIContactSheet: font file not found at %s (run make setup-env)", fontFile)
	}

	dummyBackend := dummytui.NewFixtureBackend()
	dummyCaptures := screenshot.CaptureCreateFlowScreens(dummyBackend)

	pngDir := filepath.Join(reviewPacketDir, "approved-tui-panels")
	if err := os.MkdirAll(pngDir, 0o750); err != nil { //nolint:gosec // controlled output dir (G301)
		t.Fatalf("TestGenerateApprovedTUIContactSheet: creating output dir: %v", err)
	}

	var rendered int
	for _, id := range screenshot.CreateFlowScreenIDs {
		golden, ok := dummyCaptures[id]
		if !ok {
			continue
		}
		res, err := screenshot.CaptureTUI(golden, screenshot.TUIOptions{
			FontFile: fontFile,
			Theme:    theme,
			OutDir:   pngDir,
			Name:     "approved-tui-" + id,
			Width:    screenshot.CaptureWidth,
			Height:   screenshot.CaptureHeight,
		})
		if err != nil {
			t.Logf("TestGenerateApprovedTUIContactSheet: skipping %q: %v", id, err)
			continue
		}
		rendered++
		t.Logf("approved-tui: %s → %s (sha256:%s)", id, res.PNGPath, res.SHA256)
	}
	if rendered == 0 {
		t.Error("TestGenerateApprovedTUIContactSheet: no panels rendered")
	} else {
		t.Logf("TestGenerateApprovedTUIContactSheet: %d approved-TUI panels rendered to %s", rendered, pngDir)
	}
}

// currentGitCommit returns the current HEAD short hash or "unknown".
func currentGitCommit() string {
	// best-effort; failure returns placeholder
	data, err := os.ReadFile("../../.git/HEAD")
	if err != nil {
		return "unknown"
	}
	ref := strings.TrimSpace(string(data))
	if strings.HasPrefix(ref, "ref: ") {
		refPath := strings.TrimPrefix(ref, "ref: ")
		hash, err := os.ReadFile(filepath.Join("../../.git", refPath))
		if err == nil {
			return strings.TrimSpace(string(hash))[:7]
		}
	}
	if len(ref) >= 7 {
		return ref[:7]
	}
	return "unknown"
}
