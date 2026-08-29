package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
