package filewriter

import (
	"strings"
	"testing"
)

// TestProvisionalSentinelConstants verifies that the provisional sentinel
// prefixes are exported, non-empty, and DISTINCT from the managed prefixes.
func TestProvisionalSentinelConstants(t *testing.T) {
	if ProvisionalBeginPrefix == "" {
		t.Fatal("ProvisionalBeginPrefix must not be empty")
	}
	if ProvisionalEndPrefix == "" {
		t.Fatal("ProvisionalEndPrefix must not be empty")
	}
	if ProvisionalBeginPrefix == BeginPrefix {
		t.Fatalf("ProvisionalBeginPrefix %q must differ from BeginPrefix %q", ProvisionalBeginPrefix, BeginPrefix)
	}
	if ProvisionalEndPrefix == EndPrefix {
		t.Fatalf("ProvisionalEndPrefix %q must differ from EndPrefix %q", ProvisionalEndPrefix, EndPrefix)
	}
}

// TestReplaceProvisionalBlock_AppendsWhenAbsent verifies that
// ReplaceProvisionalBlock appends a new provisional block after preserving all
// existing content verbatim when no provisional block for the given name exists.
func TestReplaceProvisionalBlock_AppendsWhenAbsent(t *testing.T) {
	existing := []byte("# user comment\nHost example\n\tUser me\n")
	out := ReplaceProvisionalBlock(existing, "work", "Host work\n\tUser git")

	s := string(out)
	if !strings.HasPrefix(s, "# user comment\nHost example\n\tUser me\n") {
		t.Fatalf("existing content not preserved verbatim at head:\n%q", s)
	}
	wantBlock := ProvisionalBeginPrefix + "work\nHost work\n\tUser git\n" + ProvisionalEndPrefix + "work\n"
	if !strings.Contains(s, wantBlock) {
		t.Fatalf("expected appended provisional block %q in output:\n%q", wantBlock, s)
	}
}

// TestReplaceProvisionalBlock_ReplacesExisting verifies that
// ReplaceProvisionalBlock replaces only the lines between (and including) the
// matching provisional BEGIN/END markers, leaving all other lines byte-identical.
func TestReplaceProvisionalBlock_ReplacesExisting(t *testing.T) {
	existing := []byte(
		"top line\n" +
			ProvisionalBeginPrefix + "work\n" +
			"old body\n" +
			ProvisionalEndPrefix + "work\n" +
			"bottom line\n",
	)
	out := ReplaceProvisionalBlock(existing, "work", "new body")

	want := "top line\n" +
		ProvisionalBeginPrefix + "work\n" +
		"new body\n" +
		ProvisionalEndPrefix + "work\n" +
		"bottom line\n"
	if string(out) != want {
		t.Fatalf("replace mismatch:\n got %q\nwant %q", out, want)
	}
}

// TestReplaceProvisionalBlock_Idempotent verifies that calling
// ReplaceProvisionalBlock twice with the same name and body yields
// byte-identical output (SAFE-02 idempotency, mirrored for provisional).
func TestReplaceProvisionalBlock_Idempotent(t *testing.T) {
	existing := []byte("preamble\n")
	out1 := ReplaceProvisionalBlock(existing, "work", "Host work\n\tUser git")
	out2 := ReplaceProvisionalBlock(out1, "work", "Host work\n\tUser git")

	if string(out1) != string(out2) {
		t.Fatalf("ReplaceProvisionalBlock is not idempotent:\n out1 %q\n out2 %q", out1, out2)
	}
}

// TestReplaceProvisionalBlock_PreservesManagedBlocks verifies that a managed
// block co-resident with a provisional block of the same name is NOT modified
// by ReplaceProvisionalBlock — the two sentinels are mutually exclusive.
func TestReplaceProvisionalBlock_PreservesManagedBlocks(t *testing.T) {
	existing := []byte(
		BeginPrefix + "work\n" +
			"managed body\n" +
			EndPrefix + "work\n" +
			ProvisionalBeginPrefix + "work\n" +
			"old provisional body\n" +
			ProvisionalEndPrefix + "work\n",
	)
	out := ReplaceProvisionalBlock(existing, "work", "new provisional body")
	s := string(out)

	// Managed block must be byte-identical.
	managedBlock := BeginPrefix + "work\n" + "managed body\n" + EndPrefix + "work\n"
	if !strings.Contains(s, managedBlock) {
		t.Errorf("managed block was modified by ReplaceProvisionalBlock:\n%q", s)
	}
	// Old provisional body gone, new one present.
	if strings.Contains(s, "old provisional body") {
		t.Errorf("old provisional body still present:\n%q", s)
	}
	if !strings.Contains(s, "new provisional body") {
		t.Errorf("new provisional body missing:\n%q", s)
	}
}

// TestRemoveProvisionalBlock_RemovesBlock verifies that
// RemoveProvisionalBlock removes the named provisional block and returns
// content without it, preserving surrounding content byte-for-byte (WR-04
// mirrored for provisional).
func TestRemoveProvisionalBlock_RemovesBlock(t *testing.T) {
	content := []byte(
		"top line\n" +
			ProvisionalBeginPrefix + "work\n" +
			"work body\n" +
			ProvisionalEndPrefix + "work\n" +
			"bottom line\n",
	)
	got := RemoveProvisionalBlock(content, "work")
	s := string(got)

	if strings.Contains(s, ProvisionalBeginPrefix+"work") {
		t.Errorf("BEGIN provisional marker still present after remove:\n%q", s)
	}
	if strings.Contains(s, ProvisionalEndPrefix+"work") {
		t.Errorf("END provisional marker still present after remove:\n%q", s)
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

// TestRemoveProvisionalBlock_AbsentBlock verifies that calling
// RemoveProvisionalBlock on content without the named block returns the input
// unchanged (idempotent).
func TestRemoveProvisionalBlock_AbsentBlock(t *testing.T) {
	original := []byte("some content\nmore content\n")
	got := RemoveProvisionalBlock(original, "nonexistent")
	if string(got) != string(original) {
		t.Fatalf("content changed when provisional block was absent:\n got %q\nwant %q", got, original)
	}
}

// TestRemoveProvisionalBlock_PreservesManagedBlock verifies that
// RemoveProvisionalBlock removes only the provisional sentinel range and
// leaves a co-resident managed block byte-for-byte (T-05.7-14-03).
func TestRemoveProvisionalBlock_PreservesManagedBlock(t *testing.T) {
	content := []byte(
		BeginPrefix + "work\n" +
			"managed body\n" +
			EndPrefix + "work\n" +
			"\n" +
			ProvisionalBeginPrefix + "work\n" +
			"provisional body\n" +
			ProvisionalEndPrefix + "work\n",
	)
	got := string(RemoveProvisionalBlock(content, "work"))

	// Provisional block gone.
	if strings.Contains(got, "provisional body") {
		t.Errorf("provisional body still present:\n%q", got)
	}
	// Managed block byte-identical.
	managedBlock := BeginPrefix + "work\n" + "managed body\n" + EndPrefix + "work\n"
	if !strings.Contains(got, managedBlock) {
		t.Errorf("managed block was damaged:\n%q", got)
	}
}

// TestRemoveProvisionalBlock_PreservesForeignTrailingBlankLine verifies that
// RemoveProvisionalBlock does NOT consume a trailing blank line placed after
// the END marker by the user (WR-04 mirrored for provisional).
func TestRemoveProvisionalBlock_PreservesForeignTrailingBlankLine(t *testing.T) {
	content := []byte(
		ProvisionalBeginPrefix + "work\n" +
			"work body\n" +
			ProvisionalEndPrefix + "work\n" +
			"\n" +
			"Host foreign.example.com\n" +
			"\tHostname foreign.example.com\n",
	)
	got := string(RemoveProvisionalBlock(content, "work"))

	if strings.Contains(got, "work body") {
		t.Errorf("provisional block not removed:\n%q", got)
	}
	want := "\nHost foreign.example.com\n\tHostname foreign.example.com\n"
	if got != want {
		t.Errorf("foreign trailing blank line was consumed (WR-04):\n got %q\nwant %q", got, want)
	}
}

// TestListProvisionalBlocks_Empty verifies that ListProvisionalBlocks on empty
// content returns nil with no panic.
func TestListProvisionalBlocks_Empty(t *testing.T) {
	got := ListProvisionalBlocks([]byte(""))
	if got != nil {
		t.Fatalf("expected nil slice for empty content, got %v", got)
	}
}

// TestListProvisionalBlocks_ReturnsProvisional verifies that
// ListProvisionalBlocks returns provisional blocks in file order.
func TestListProvisionalBlocks_ReturnsProvisional(t *testing.T) {
	content := []byte(
		ProvisionalBeginPrefix + "personal\nHost personal\n\tUser git\n" + ProvisionalEndPrefix + "personal\n" +
			ProvisionalBeginPrefix + "work\nHost work\n\tUser git\n" + ProvisionalEndPrefix + "work\n",
	)
	got := ListProvisionalBlocks(content)
	if len(got) != 2 {
		t.Fatalf("expected 2 provisional blocks, got %d: %v", len(got), got)
	}
	if got[0].Name != "personal" {
		t.Errorf("block[0] Name mismatch: got %q want %q", got[0].Name, "personal")
	}
	if got[1].Name != "work" {
		t.Errorf("block[1] Name mismatch: got %q want %q", got[1].Name, "work")
	}
}

// TestListProvisionalBlocks_MutualExclusion verifies that ListProvisionalBlocks
// does NOT return managed blocks, and that ListBlocks (managed scan) does NOT
// return provisional blocks (T-05.7-14-01: the two sentinels are mutually exclusive).
func TestListProvisionalBlocks_MutualExclusion(t *testing.T) {
	content := []byte(
		BeginPrefix + "managed-only\n" + "managed body\n" + EndPrefix + "managed-only\n" +
			ProvisionalBeginPrefix + "prov-only\n" + "provisional body\n" + ProvisionalEndPrefix + "prov-only\n",
	)

	managedBlocks := ListBlocks(content)
	provBlocks := ListProvisionalBlocks(content)

	// ListBlocks must return only the managed block.
	if len(managedBlocks) != 1 || managedBlocks[0].Name != "managed-only" {
		t.Errorf("ListBlocks returned unexpected result: %v", managedBlocks)
	}
	// ListProvisionalBlocks must return only the provisional block.
	if len(provBlocks) != 1 || provBlocks[0].Name != "prov-only" {
		t.Errorf("ListProvisionalBlocks returned unexpected result: %v", provBlocks)
	}
}

// TestProvisionalRoundTrip_WriteListRemove verifies that a provisional block
// survives a full round-trip: write → list → remove, without touching a
// co-resident managed block or foreign content.
func TestProvisionalRoundTrip_WriteListRemove(t *testing.T) {
	// Start with a managed block + foreign content.
	base := []byte(
		"# foreign preamble\n" +
			BeginPrefix + "alice\n" + "managed alice body\n" + EndPrefix + "alice\n" +
			"# foreign postamble\n",
	)

	// Write provisional.
	withProv := ReplaceProvisionalBlock(base, "alice", "provisional alice body")

	// List provisional — must see exactly the provisional block.
	blocks := ListProvisionalBlocks(withProv)
	if len(blocks) != 1 || blocks[0].Name != "alice" {
		t.Fatalf("expected 1 provisional block 'alice', got: %v", blocks)
	}

	// List managed — must still see the managed block, NOT the provisional.
	managedBlocks := ListBlocks(withProv)
	if len(managedBlocks) != 1 || managedBlocks[0].Name != "alice" {
		t.Fatalf("ListBlocks expected 1 managed block 'alice', got: %v", managedBlocks)
	}

	// Remove provisional.
	dropped := RemoveProvisionalBlock(withProv, "alice")

	// After removal, provisional must be gone but managed + foreign preserved.
	if strings.Contains(string(dropped), ProvisionalBeginPrefix) {
		t.Errorf("provisional BEGIN marker still present after remove:\n%q", string(dropped))
	}
	if string(dropped) != string(base) {
		t.Errorf("after provisional remove, content not restored to original:\n got %q\nwant %q", string(dropped), string(base))
	}
}

// TestReplaceProvisionalBlock_CRLF verifies that ReplaceProvisionalBlock
// matches an existing provisional block's markers even when the file uses CRLF
// line endings, and that foreign CRLF content is preserved byte-for-byte.
func TestReplaceProvisionalBlock_CRLF(t *testing.T) {
	existing := []byte(
		"top line\r\n" +
			ProvisionalBeginPrefix + "work\r\n" +
			"old body\r\n" +
			ProvisionalEndPrefix + "work\r\n" +
			"bottom line\r\n",
	)
	out := string(ReplaceProvisionalBlock(existing, "work", "new body"))

	// Exactly one BEGIN provisional marker.
	if got := strings.Count(out, ProvisionalBeginPrefix+"work"); got != 1 {
		t.Fatalf("expected exactly 1 work provisional BEGIN, got %d:\n%q", got, out)
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
