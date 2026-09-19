package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var retiredFrameLiterals = []string{
	"Set keys",
}

// TestPromotedFramesHaveNoRetiredLabel keeps the retired-copy scan scoped to
// promoted evidence, unlike the source walk which excludes .planning prose.
func TestPromotedFramesHaveNoRetiredLabel(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("..", "..", ".planning", "phases", "*", "ui-frames", "*.txt"))
	if err != nil {
		t.Fatalf("glob promoted frames: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("no promoted evidence frames found")
	}

	var failures []string
	for _, path := range paths {
		data, err := os.ReadFile(path) //nolint:gosec // repository-owned evidence path
		if err != nil {
			t.Fatalf("read promoted frame %s: %v", path, err)
		}
		for lineNumber, line := range strings.Split(string(data), "\n") {
			for _, retired := range retiredFrameLiterals {
				if strings.Contains(line, retired) {
					failures = append(failures, filepath.ToSlash(path)+":"+fmt.Sprintf("%d", lineNumber+1)+": "+retired)
				}
			}
		}
	}
	if len(failures) > 0 {
		t.Fatalf("retired literals found in promoted frames:\n%s", strings.Join(failures, "\n"))
	}
}

// TestPromotionCasesExist prevents the promotion registry from silently
// retaining a renamed or deleted real-PTY test name.
func TestPromotionCasesExist(t *testing.T) {
	root := filepath.Join("..", "..", "e2e")
	testNamePattern := regexp.MustCompile(`func\s+(Test[A-Za-z0-9_]+)\s*\(`)
	known := map[string]bool{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(info.Name(), "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path) //nolint:gosec // repository-owned e2e source
		if err != nil {
			return err
		}
		for _, match := range testNamePattern.FindAllStringSubmatch(string(data), -1) {
			known[match[1]] = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk e2e tests: %v", err)
	}

	var missing []string
	for _, entry := range phase96Frames {
		if !known[entry.test] {
			missing = append(missing, entry.test)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("Phase 9.6 promotion cases missing from e2e source: %s", strings.Join(missing, ", "))
	}
}

// writeFrame writes a source frame file into dir for name.
func writeFrame(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name+".txt"), []byte(content), 0o600); err != nil {
		t.Fatalf("writeFrame(%s): %v", name, err)
	}
}

// TestPromoteFramesWritesNothingWhenAnyCaptureIsMissing is the WR-13
// regression: the old promotion loop wrote each tracked baseline as it
// went, only checking for missing captures AFTER the loop finished — a
// partial capture run left some baselines rewritten to the new run's
// content and others frozen at their previous commit's content, with the
// provenance table never regenerated at all. A partial capture set must
// leave dstDir completely untouched.
func TestPromoteFramesWritesNothingWhenAnyCaptureIsMissing(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	frames := []promotionEntry{
		{stateID: "a", frame: "frame-a", test: "TestA", shimMode: "ok", geometry: "100x30"},
		{stateID: "b", frame: "frame-b", test: "TestB", shimMode: "ok", geometry: "100x30"},
	}
	// Only "frame-a" exists; "frame-b" is missing (the partial-run shape).
	writeFrame(t, srcDir, "frame-a", "old baseline content for a\n")
	// Seed dstDir with a PRE-EXISTING baseline for "a" from a prior,
	// already-approved run, so we can prove it survives untouched.
	priorContent := "PRIOR approved content for a — must survive untouched\n"
	if err := os.WriteFile(filepath.Join(dstDir, "frame-a.txt"), []byte(priorContent), 0o600); err != nil {
		t.Fatalf("seeding dstDir: %v", err)
	}

	rows, err := promoteFrames(srcDir, dstDir, frames, "deadbeef", nil)
	if err == nil {
		t.Fatal("promoteFrames must fail when a capture is missing")
	}
	if rows != nil {
		t.Errorf("promoteFrames returned rows=%v on a failed run, want nil", rows)
	}

	got, rerr := os.ReadFile(filepath.Join(dstDir, "frame-a.txt")) //nolint:gosec // test-controlled path
	if rerr != nil {
		t.Fatalf("reading dstDir/frame-a.txt: %v", rerr)
	}
	if string(got) != priorContent {
		t.Errorf("frame-a.txt was rewritten despite frame-b being missing — dstDir must stay untouched on partial capture: got %q, want the prior content %q", got, priorContent)
	}
	if _, statErr := os.Stat(filepath.Join(dstDir, "frame-b.txt")); statErr == nil {
		t.Error("frame-b.txt should not exist — it was never a real capture, and nothing should have been written for it")
	}
}

// TestPromoteFramesWritesEveryFrameWhenAllCapturesArePresent is the
// positive control: a complete capture set promotes every frame and
// returns one provenance row per frame, sorted by stateID.
func TestPromoteFramesWritesEveryFrameWhenAllCapturesArePresent(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	frames := []promotionEntry{
		{stateID: "z-second", frame: "frame-z", test: "TestZ", shimMode: "ok", geometry: "100x30"},
		{stateID: "a-first", frame: "frame-a", test: "TestA", shimMode: "ok", geometry: "100x30"},
	}
	writeFrame(t, srcDir, "frame-a", "content a\n")
	writeFrame(t, srcDir, "frame-z", "content z\n")

	rows, err := promoteFrames(srcDir, dstDir, frames, "deadbeef", nil)
	if err != nil {
		t.Fatalf("promoteFrames: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %v, want 2", rows)
	}
	// Sorted by stateID: "a-first" before "z-second".
	if !strings.Contains(rows[0], "a-first") || !strings.Contains(rows[1], "z-second") {
		t.Errorf("rows not sorted by stateID: %v", rows)
	}
	for _, name := range []string{"frame-a", "frame-z"} {
		if _, statErr := os.Stat(filepath.Join(dstDir, name+".txt")); statErr != nil {
			t.Errorf("%s.txt was not written: %v", name, statErr)
		}
	}
}

// TestPromoteFramesRemovesStaleFramesNotInTheRegistry is the WR-05
// (09.5-REVIEW.md round 3) regression: promoteFrames never deleted a stale
// .txt file left behind in dstDir by a renamed or removed registry entry,
// contradicting writeProvenance's own claim that re-running "overwrites this
// table and every frame in this directory". A tracked, orphaned frame with
// no provenance row is exactly what the round-3 review's WR-05 fix names —
// it silently survives every future promotion undetected. The removal must
// happen only AFTER the all-present check (WR-13's own invariant), so a
// partial capture run still leaves dstDir completely untouched.
func TestPromoteFramesRemovesStaleFramesNotInTheRegistry(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	frames := []promotionEntry{
		{stateID: "a", frame: "frame-a", test: "TestA", shimMode: "ok", geometry: "100x30"},
	}
	writeFrame(t, srcDir, "frame-a", "content a\n")

	// A stale frame from a REMOVED or RENAMED registry entry, tracked in
	// dstDir from a prior run, with no corresponding entry in frames.
	stalePath := filepath.Join(dstDir, "frame-orphan.txt")
	if err := os.WriteFile(stalePath, []byte("stale content — no longer registered\n"), 0o600); err != nil {
		t.Fatalf("seeding stale frame: %v", err)
	}
	// A non-.txt file in dstDir (e.g. README.md) must never be touched —
	// promoteFrames only owns the .txt frames, writeProvenance owns README.md
	// separately.
	otherPath := filepath.Join(dstDir, "README.md")
	if err := os.WriteFile(otherPath, []byte("# not a frame\n"), 0o600); err != nil {
		t.Fatalf("seeding README.md: %v", err)
	}

	if _, err := promoteFrames(srcDir, dstDir, frames, "deadbeef", nil); err != nil {
		t.Fatalf("promoteFrames: %v", err)
	}

	if _, statErr := os.Stat(stalePath); !os.IsNotExist(statErr) {
		t.Errorf("frame-orphan.txt still exists after promoteFrames (stat err: %v) — a stale frame not in the registry must be removed", statErr)
	}
	if _, statErr := os.Stat(otherPath); statErr != nil {
		t.Errorf("README.md was removed by promoteFrames (stat err: %v) — only .txt frames not in the registry must be removed", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(dstDir, "frame-a.txt")); statErr != nil {
		t.Errorf("frame-a.txt (a REAL registered frame) was removed: %v", statErr)
	}
}

// TestPromoteFramesLeavesStaleFramesOnAPartialCaptureRun is
// TestPromoteFramesWritesNothingWhenAnyCaptureIsMissing's WR-05 sibling: the
// stale-frame removal must happen strictly AFTER the all-present check, so a
// PARTIAL capture run (WR-13's own failure mode) leaves dstDir — including
// any stale frame already there — completely untouched, exactly like every
// other write this function makes.
func TestPromoteFramesLeavesStaleFramesOnAPartialCaptureRun(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	frames := []promotionEntry{
		{stateID: "a", frame: "frame-a", test: "TestA", shimMode: "ok", geometry: "100x30"},
		{stateID: "b", frame: "frame-b", test: "TestB", shimMode: "ok", geometry: "100x30"},
	}
	// Only "frame-a" exists; "frame-b" is missing (the partial-run shape).
	writeFrame(t, srcDir, "frame-a", "content a\n")

	stalePath := filepath.Join(dstDir, "frame-orphan.txt")
	if err := os.WriteFile(stalePath, []byte("stale content\n"), 0o600); err != nil {
		t.Fatalf("seeding stale frame: %v", err)
	}

	if _, err := promoteFrames(srcDir, dstDir, frames, "deadbeef", nil); err == nil {
		t.Fatal("promoteFrames must fail when a capture is missing")
	}

	if _, statErr := os.Stat(stalePath); statErr != nil {
		t.Errorf("frame-orphan.txt was removed despite the partial run failing — dstDir must stay untouched on partial capture: %v", statErr)
	}
}

// ---------------------------------------------------------------------------
// 09.5-05-PLAN.md Task 2 (D-K): phase parameterisation. Every test below
// drives resolvePhase/promoteForPhase — the orchestration layer ABOVE
// promoteFrames — so promoteFrames's own signature and the two pre-existing
// tests above stay byte-for-byte unmodified.
// ---------------------------------------------------------------------------

// seedPhaseFrames writes every entry's frame file into srcDir with
// placeholder content unique per frame (so a cross-frame mixup would be
// detectable), for the given entries.
func seedPhaseFrames(t *testing.T, srcDir string, entries []promotionEntry) {
	t.Helper()
	for _, e := range entries {
		writeFrame(t, srcDir, e.frame, "placeholder content for "+e.frame+"\n")
	}
}

// TestPromoteFramesTargetsTheRequestedPhaseDirectory proves that promoting
// phase "09.5" writes ONLY into that phase's ui-frames directory under the
// given planning-phases root, and that Phase 9's directory is never created
// as a side effect.
func TestPromoteFramesTargetsTheRequestedPhaseDirectory(t *testing.T) {
	srcDir := t.TempDir()
	phasesRoot := t.TempDir()
	seedPhaseFrames(t, srcDir, phase95Frames)

	dstDir, rows, err := promoteForPhase(srcDir, phasesRoot, "09.5", "deadbeef", nil)
	if err != nil {
		t.Fatalf("promoteForPhase(09.5): %v", err)
	}
	wantDst := filepath.Join(phasesRoot, "09.5-full-ssh-git-properties-browser", "ui-frames")
	if dstDir != wantDst {
		t.Errorf("dstDir = %q, want %q", dstDir, wantDst)
	}
	if len(rows) != len(phase95Frames) {
		t.Fatalf("rows = %d, want %d", len(rows), len(phase95Frames))
	}
	for _, e := range phase95Frames {
		if _, statErr := os.Stat(filepath.Join(dstDir, e.frame+".txt")); statErr != nil {
			t.Errorf("%s.txt was not written under the phase 09.5 directory: %v", e.frame, statErr)
		}
	}
	// Nowhere else: Phase 9's directory must not have been created.
	phase9Dir := filepath.Join(phasesRoot, "09-upload-credentials-assist")
	if _, statErr := os.Stat(phase9Dir); statErr == nil {
		t.Errorf("phase 9 directory %q must not exist — promoting phase 09.5 must write nowhere else", phase9Dir)
	}
}

// TestPromoteFramesPhase9DefaultUnchanged proves that the default phase
// ("09", what main() resolves to with no flag override) still resolves to
// today's exact Phase 9 destination directory and today's exact phase9Frames
// list — the parameterisation changed nothing about the existing behavior.
func TestPromoteFramesPhase9DefaultUnchanged(t *testing.T) {
	dir, frames, err := resolvePhase(defaultPhase)
	if err != nil {
		t.Fatalf("resolvePhase(defaultPhase): %v", err)
	}
	if defaultPhase != "09" {
		t.Fatalf("defaultPhase = %q, want \"09\" — the no-argument invocation must still mean Phase 9", defaultPhase)
	}
	wantDir := "09-upload-credentials-assist"
	if dir != wantDir {
		t.Errorf("resolvePhase(%q) dir = %q, want %q", defaultPhase, dir, wantDir)
	}
	if len(frames) != len(phase9Frames) {
		t.Fatalf("resolvePhase(%q) returned %d frames, want %d (phase9Frames unchanged)", defaultPhase, len(frames), len(phase9Frames))
	}
	for i, e := range frames {
		if e != phase9Frames[i] {
			t.Errorf("resolvePhase(%q) frame[%d] = %+v, want %+v (phase9Frames unchanged)", defaultPhase, i, e, phase9Frames[i])
		}
	}
}

// TestPromoteFramesUnknownPhaseFailsClosed proves that an unknown phase
// identifier is refused by name and nothing is written anywhere — the tool
// never writes to a directory it was not asked for.
func TestPromoteFramesUnknownPhaseFailsClosed(t *testing.T) {
	if _, _, err := resolvePhase("99-does-not-exist"); err == nil {
		t.Fatal("resolvePhase must refuse an unknown phase identifier")
	}

	srcDir := t.TempDir()
	phasesRoot := t.TempDir()
	dstDir, rows, err := promoteForPhase(srcDir, phasesRoot, "99-does-not-exist", "deadbeef", nil)
	if err == nil {
		t.Fatal("promoteForPhase must fail for an unknown phase")
	}
	if dstDir != "" {
		t.Errorf("promoteForPhase returned dstDir=%q on failure, want empty", dstDir)
	}
	if rows != nil {
		t.Errorf("promoteForPhase returned rows=%v on failure, want nil", rows)
	}
	entries, readErr := os.ReadDir(phasesRoot)
	if readErr != nil {
		t.Fatalf("reading phasesRoot: %v", readErr)
	}
	if len(entries) != 0 {
		t.Errorf("phasesRoot must stay empty on an unknown-phase refusal, found: %v", entries)
	}
}
