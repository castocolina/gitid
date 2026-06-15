package filewriter

import (
	"strings"
	"testing"
)

// TestReplaceBlockAppendsWhenAbsent verifies that ReplaceBlock appends a new
// sentinel-delimited managed block after preserving all existing content
// verbatim when no block for the given name exists yet.
func TestReplaceBlockAppendsWhenAbsent(t *testing.T) {
	existing := []byte("# user comment\nHost example\n\tUser me\n")
	out := ReplaceBlock(existing, "work", "Host work\n\tUser git")

	s := string(out)
	if !strings.HasPrefix(s, "# user comment\nHost example\n\tUser me\n") {
		t.Fatalf("existing content not preserved verbatim at head:\n%q", s)
	}
	wantBlock := BeginPrefix + "work\nHost work\n\tUser git\n" + EndPrefix + "work\n"
	if !strings.Contains(s, wantBlock) {
		t.Fatalf("expected appended block %q in output:\n%q", wantBlock, s)
	}
}

// TestReplaceBlockReplacesExisting verifies that ReplaceBlock replaces only the
// lines between (and including) the matching BEGIN/END markers, leaving all
// other lines byte-identical.
func TestReplaceBlockReplacesExisting(t *testing.T) {
	existing := []byte(
		"top line\n" +
			BeginPrefix + "work\n" +
			"old body\n" +
			EndPrefix + "work\n" +
			"bottom line\n",
	)
	out := ReplaceBlock(existing, "work", "new body")

	want := "top line\n" +
		BeginPrefix + "work\n" +
		"new body\n" +
		EndPrefix + "work\n" +
		"bottom line\n"
	if string(out) != want {
		t.Fatalf("replace mismatch:\n got %q\nwant %q", out, want)
	}
}

// TestReplaceBlockIdempotent verifies SAFE-02: calling ReplaceBlock twice with
// the same name and body yields byte-identical output (an empty diff).
func TestReplaceBlockIdempotent(t *testing.T) {
	existing := []byte("preamble\n")
	out1 := ReplaceBlock(existing, "work", "Host work\n\tUser git")
	out2 := ReplaceBlock(out1, "work", "Host work\n\tUser git")

	if string(out1) != string(out2) {
		t.Fatalf("ReplaceBlock is not idempotent:\n out1 %q\n out2 %q", out1, out2)
	}
}

// TestReplaceBlockPreservesForeignContent verifies that lines outside any gitid
// block, and blocks owned by OTHER names, are preserved byte-for-byte while the
// targeted block is updated.
func TestReplaceBlockPreservesForeignContent(t *testing.T) {
	existing := []byte(
		"hand-written top\n" +
			BeginPrefix + "personal\n" +
			"personal body\n" +
			EndPrefix + "personal\n" +
			"hand-written middle\n" +
			BeginPrefix + "work\n" +
			"old work body\n" +
			EndPrefix + "work\n" +
			"hand-written bottom\n",
	)
	out := ReplaceBlock(existing, "work", "new work body")
	s := string(out)

	// Foreign block and surrounding hand-written lines are untouched.
	personalBlock := BeginPrefix + "personal\n" + "personal body\n" + EndPrefix + "personal\n"
	for _, frag := range []string{
		"hand-written top\n",
		personalBlock,
		"hand-written middle\n",
		"hand-written bottom\n",
	} {
		if !strings.Contains(s, frag) {
			t.Fatalf("foreign content %q was not preserved:\n%q", frag, s)
		}
	}
	// The targeted block now carries the new body and no longer the old one.
	if !strings.Contains(s, BeginPrefix+"work\n"+"new work body\n"+EndPrefix+"work\n") {
		t.Fatalf("work block was not updated:\n%q", s)
	}
	if strings.Contains(s, "old work body") {
		t.Fatalf("old work body still present:\n%q", s)
	}
}

// TestReplaceBlock_CRLF verifies that ReplaceBlock matches an EXISTING block's
// BEGIN/END markers even when the file uses CRLF line endings, so the block is
// replaced in place rather than a duplicate being appended. The foreign CRLF
// content surrounding the block is preserved byte-for-byte.
func TestReplaceBlock_CRLF(t *testing.T) {
	existing := []byte(
		"top line\r\n" +
			BeginPrefix + "work\r\n" +
			"old body\r\n" +
			EndPrefix + "work\r\n" +
			"bottom line\r\n",
	)
	out := string(ReplaceBlock(existing, "work", "new body"))

	// Exactly one BEGIN marker — the existing block was replaced, not duplicated.
	if got := strings.Count(out, BeginPrefix+"work"); got != 1 {
		t.Fatalf("expected exactly 1 work BEGIN marker (no duplicate), got %d:\n%q", got, out)
	}
	if strings.Contains(out, "old body") {
		t.Errorf("old body still present after CRLF replace:\n%q", out)
	}
	if !strings.Contains(out, "new body") {
		t.Errorf("new body missing after CRLF replace:\n%q", out)
	}
	// Foreign CRLF lines must survive byte-for-byte.
	if !strings.Contains(out, "top line\r\n") || !strings.Contains(out, "bottom line\r\n") {
		t.Errorf("foreign CRLF content not preserved byte-for-byte:\n%q", out)
	}
}

// TestReplaceBlockAddsSecondDistinctBlock verifies that a different name adds a
// second distinct block without disturbing the first.
func TestReplaceBlockAddsSecondDistinctBlock(t *testing.T) {
	out := ReplaceBlock([]byte(""), "work", "work body")
	out = ReplaceBlock(out, "personal", "personal body")
	s := string(out)

	if !strings.Contains(s, BeginPrefix+"work\n"+"work body\n"+EndPrefix+"work\n") {
		t.Fatalf("first block missing after adding second:\n%q", s)
	}
	if !strings.Contains(s, BeginPrefix+"personal\n"+"personal body\n"+EndPrefix+"personal\n") {
		t.Fatalf("second block missing:\n%q", s)
	}
}
