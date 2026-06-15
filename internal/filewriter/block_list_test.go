package filewriter

import (
	"strings"
	"testing"
)

// TestListBlocks_Empty verifies that ListBlocks on empty content returns nil
// with no panic.
func TestListBlocks_Empty(t *testing.T) {
	got := ListBlocks([]byte(""))
	if got != nil {
		t.Fatalf("expected nil slice for empty content, got %v", got)
	}
}

// TestListBlocks_NoBlocks verifies that content with no sentinel markers
// returns a nil slice.
func TestListBlocks_NoBlocks(t *testing.T) {
	content := []byte("# user comment\nHost example\n\tUser me\n")
	got := ListBlocks(content)
	if len(got) != 0 {
		t.Fatalf("expected 0 blocks for content with no markers, got %d", len(got))
	}
}

// TestListBlocks_OneBlock verifies that one complete block is returned with
// the correct Name and Body.
func TestListBlocks_OneBlock(t *testing.T) {
	content := []byte(BeginPrefix + "work\nHost work\n\tUser git\n" + EndPrefix + "work\n")
	got := ListBlocks(content)
	if len(got) != 1 {
		t.Fatalf("expected 1 block, got %d: %v", len(got), got)
	}
	if got[0].Name != "work" {
		t.Errorf("Name mismatch: got %q want %q", got[0].Name, "work")
	}
	want := "Host work\n\tUser git"
	if got[0].Body != want {
		t.Errorf("Body mismatch:\n got %q\nwant %q", got[0].Body, want)
	}
}

// TestListBlocks_TwoBlocks verifies that two complete blocks are returned in
// file order.
func TestListBlocks_TwoBlocks(t *testing.T) {
	content := []byte(
		BeginPrefix + "personal\nHost personal\n\tUser git\n" + EndPrefix + "personal\n" +
			BeginPrefix + "work\nHost work\n\tUser git\n" + EndPrefix + "work\n",
	)
	got := ListBlocks(content)
	if len(got) != 2 {
		t.Fatalf("expected 2 blocks, got %d: %v", len(got), got)
	}
	if got[0].Name != "personal" {
		t.Errorf("block[0] Name mismatch: got %q want %q", got[0].Name, "personal")
	}
	if got[1].Name != "work" {
		t.Errorf("block[1] Name mismatch: got %q want %q", got[1].Name, "work")
	}
}

// TestListBlocks_IncompleteBeginWithoutEnd verifies that a BEGIN with no
// matching END is silently skipped (not returned).
func TestListBlocks_IncompleteBeginWithoutEnd(t *testing.T) {
	content := []byte(BeginPrefix + "orphan\nHost orphan\n\tUser git\n")
	got := ListBlocks(content)
	if len(got) != 0 {
		t.Fatalf("expected 0 blocks for incomplete block, got %d: %v", len(got), got)
	}
}

// TestListBlocks_ForeignContentPreservedInOutput verifies that foreign content
// between/around blocks never appears in any block's Body.
func TestListBlocks_ForeignContentPreservedInOutput(t *testing.T) {
	content := []byte(
		"# user-written header\n" +
			BeginPrefix + "work\nHost work\n\tUser git\n" + EndPrefix + "work\n" +
			"# user-written footer\n",
	)
	got := ListBlocks(content)
	if len(got) != 1 {
		t.Fatalf("expected 1 block, got %d", len(got))
	}
	if strings.Contains(got[0].Body, "user-written") {
		t.Errorf("foreign content leaked into Body: %q", got[0].Body)
	}
}

// TestListBlocks_CRLFNormalized verifies that CRLF line endings are
// normalised to LF before scanning so the block is still matched.
func TestListBlocks_CRLFNormalized(t *testing.T) {
	// Build with explicit \r\n bytes.
	raw := BeginPrefix + "work\r\nHost work\r\n\tUser git\r\n" + EndPrefix + "work\r\n"
	got := ListBlocks([]byte(raw))
	if len(got) != 1 {
		t.Fatalf("expected 1 block after CRLF normalisation, got %d: %v", len(got), got)
	}
	if got[0].Name != "work" {
		t.Errorf("Name mismatch: got %q want %q", got[0].Name, "work")
	}
}

// TestListBlocks_RoundTripWithReplaceBlock verifies that ListBlocks on the
// output of ReplaceBlock finds a block whose Body equals the trimmed body.
func TestListBlocks_RoundTripWithReplaceBlock(t *testing.T) {
	body := "Host work\n\tUser git\n"
	out := ReplaceBlock([]byte(""), "work", body)
	blocks := ListBlocks(out)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block after round-trip, got %d", len(blocks))
	}
	want := strings.TrimRight(body, "\n")
	if blocks[0].Body != want {
		t.Errorf("round-trip Body mismatch:\n got %q\nwant %q", blocks[0].Body, want)
	}
}

// TestRemoveBlock_RemovesBlock verifies that RemoveBlock removes the named block
// and returns content without it.
func TestRemoveBlock_RemovesBlock(t *testing.T) {
	content := []byte(
		"top line\n" +
			BeginPrefix + "work\n" +
			"work body\n" +
			EndPrefix + "work\n" +
			"bottom line\n",
	)
	got := RemoveBlock(content, "work")
	s := string(got)
	if strings.Contains(s, BeginPrefix+"work") {
		t.Errorf("BEGIN marker still present after remove:\n%q", s)
	}
	if strings.Contains(s, EndPrefix+"work") {
		t.Errorf("END marker still present after remove:\n%q", s)
	}
	if strings.Contains(s, "work body") {
		t.Errorf("block body still present after remove:\n%q", s)
	}
	if !strings.Contains(s, "top line\n") {
		t.Errorf("top foreign line missing:\n%q", s)
	}
	if !strings.Contains(s, "bottom line\n") {
		t.Errorf("bottom foreign line missing:\n%q", s)
	}
}

// TestRemoveBlock_AbsentBlock verifies that calling RemoveBlock on content
// without the named block returns the input unchanged (idempotent).
func TestRemoveBlock_AbsentBlock(t *testing.T) {
	original := []byte("some content\nmore content\n")
	got := RemoveBlock(original, "nonexistent")
	if string(got) != string(original) {
		t.Fatalf("content changed when block was absent:\n got %q\nwant %q", got, original)
	}
}

// TestRemoveBlock_Idempotent verifies that calling RemoveBlock twice with the
// same name produces the same output as calling it once.
func TestRemoveBlock_Idempotent(t *testing.T) {
	content := []byte(
		"top\n" +
			BeginPrefix + "work\n" + "body\n" + EndPrefix + "work\n" +
			"bottom\n",
	)
	once := RemoveBlock(content, "work")
	twice := RemoveBlock(once, "work")
	if string(once) != string(twice) {
		t.Fatalf("RemoveBlock is not idempotent:\n once %q\n twice %q", once, twice)
	}
}

// TestRemoveBlock_NoBlankAccumulation verifies that the trailing blank line
// after the END marker is consumed (Pitfall B: no blank-line accumulation on
// repeated add→delete cycles).
func TestRemoveBlock_NoBlankAccumulation(t *testing.T) {
	// Simulate the content a ReplaceBlock produces: block followed by a blank line.
	base := []byte("top\n")
	withBlock := ReplaceBlock(base, "work", "body")
	// The block is appended; add a blank line as separator (typical file shape).
	withBlock = append(withBlock, '\n')

	removed := RemoveBlock(withBlock, "work")
	// The blank line that followed the END marker should be consumed.
	// The only blank line remaining should be from "top\n" → no double blank.
	count := strings.Count(string(removed), "\n\n")
	if count > 0 {
		t.Errorf("trailing blank line accumulated after remove: %d double-newlines in %q", count, string(removed))
	}
}

// TestRemoveBlock_ForeignBlockPreserved verifies that RemoveBlock removes only
// the named block and leaves any other block byte-identical.
func TestRemoveBlock_ForeignBlockPreserved(t *testing.T) {
	content := []byte(
		BeginPrefix + "personal\n" + "personal body\n" + EndPrefix + "personal\n" +
			"\n" +
			BeginPrefix + "work\n" + "work body\n" + EndPrefix + "work\n",
	)
	got := RemoveBlock(content, "work")
	s := string(got)
	if strings.Contains(s, "work body") {
		t.Errorf("work body still present:\n%q", s)
	}
	personalBlock := BeginPrefix + "personal\n" + "personal body\n" + EndPrefix + "personal\n"
	if !strings.Contains(s, personalBlock) {
		t.Errorf("personal block was damaged:\n%q", s)
	}
}

// TestRoundTrip_ReplaceRemoveReplace verifies that the composition of
// ReplaceBlock → RemoveBlock → ReplaceBlock is stable: the block can be
// removed and re-inserted without corrupting the file.
func TestRoundTrip_ReplaceRemoveReplace(t *testing.T) {
	base := []byte("preamble\n")
	body := "Host work\n\tUser git"

	added := ReplaceBlock(base, "work", body)
	removed := RemoveBlock(added, "work")
	readded := ReplaceBlock(removed, "work", body)

	// The re-added result should contain exactly the block and the preamble.
	s := string(readded)
	if !strings.Contains(s, "preamble\n") {
		t.Errorf("preamble missing after round-trip:\n%q", s)
	}
	wantBlock := BeginPrefix + "work\n" + body + "\n" + EndPrefix + "work\n"
	if !strings.Contains(s, wantBlock) {
		t.Errorf("block missing or malformed after round-trip:\n got %q\nwant block %q", s, wantBlock)
	}
}
