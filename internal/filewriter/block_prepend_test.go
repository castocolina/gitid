package filewriter

import (
	"strings"
	"testing"
)

// TestPrependBlockIfNotFound verifies the first-write (prepend) and empty-input
// branches of PrependBlockIfNotFound.
func TestPrependBlockIfNotFound(t *testing.T) {
	t.Run("prepends before non-empty existing content", func(t *testing.T) {
		existing := []byte("[core]\n\tignorecase = true\n")
		out := PrependBlockIfNotFound(existing, "baseline-include", "[include]\n\tpath = ~/.gitconfig.d/00-baseline")

		got := string(out)

		// The output must START with the canonical managed block.
		beginMarker := BeginPrefix + "baseline-include"
		endMarker := EndPrefix + "baseline-include"
		wantBlock := beginMarker + "\n[include]\n\tpath = ~/.gitconfig.d/00-baseline\n" + endMarker + "\n"
		if !strings.HasPrefix(got, wantBlock) {
			t.Fatalf("output does not start with canonical block:\n got: %q\nwant prefix: %q", got, wantBlock)
		}

		// The original existing content must follow verbatim (no extra blank line between).
		wantSuffix := "[core]\n\tignorecase = true\n"
		if !strings.HasSuffix(got, wantSuffix) {
			t.Fatalf("original content not preserved verbatim at tail:\n got: %q\nwant suffix: %q", got, wantSuffix)
		}
	})

	t.Run("empty input returns only the canonical block", func(t *testing.T) {
		out := PrependBlockIfNotFound(nil, "baseline-include", "[include]\n\tpath = ~/.gitconfig.d/00-baseline")
		got := string(out)

		beginMarker := BeginPrefix + "baseline-include"
		endMarker := EndPrefix + "baseline-include"
		want := beginMarker + "\n[include]\n\tpath = ~/.gitconfig.d/00-baseline\n" + endMarker + "\n"
		if got != want {
			t.Fatalf("empty input: got %q, want %q", got, want)
		}
	})

	t.Run("nil same as empty input", func(t *testing.T) {
		outNil := PrependBlockIfNotFound(nil, "baseline-include", "[include]\n\tpath = x")
		outEmpty := PrependBlockIfNotFound([]byte{}, "baseline-include", "[include]\n\tpath = x")
		if string(outNil) != string(outEmpty) {
			t.Fatalf("nil and empty should produce identical output:\n nil:   %q\n empty: %q", outNil, outEmpty)
		}
	})
}

// TestPrependBlockIfNotFound_UpdateInPlace verifies that when a managed block
// already exists at any position (e.g. mid-file with foreign content on both
// sides), a second call updates only the block lines in place — the floor
// position is preserved and foreign content before/after is byte-identical.
func TestPrependBlockIfNotFound_UpdateInPlace(t *testing.T) {
	beginMarker := BeginPrefix + "baseline-include"
	endMarker := EndPrefix + "baseline-include"

	// Build a file where the block appears in the MIDDLE (not at the top).
	//   foreign-top
	//   BEGIN baseline-include
	//   old body
	//   END baseline-include
	//   foreign-bottom
	existing := []byte(
		"[user]\n\tname = Alice\n" +
			beginMarker + "\n" +
			"[include]\n\tpath = ~/.gitconfig.d/old\n" +
			endMarker + "\n" +
			"[alias]\n\tco = checkout\n",
	)

	newBody := "[include]\n\tpath = ~/.gitconfig.d/00-baseline"
	out := PrependBlockIfNotFound(existing, "baseline-include", newBody)
	got := string(out)

	wantBlock := beginMarker + "\n" + newBody + "\n" + endMarker + "\n"

	// Block must be updated with the new body.
	if !strings.Contains(got, wantBlock) {
		t.Fatalf("updated block not found:\n got: %q\nwant block: %q", got, wantBlock)
	}
	// Old body must be gone.
	if strings.Contains(got, "old") {
		t.Fatalf("old body still present after update:\n%q", got)
	}
	// Foreign content BEFORE the block must be preserved.
	if !strings.HasPrefix(got, "[user]\n\tname = Alice\n") {
		t.Fatalf("foreign content before block was moved or removed:\n%q", got)
	}
	// Foreign content AFTER the block must be preserved.
	if !strings.HasSuffix(got, "[alias]\n\tco = checkout\n") {
		t.Fatalf("foreign content after block was moved or removed:\n%q", got)
	}
	// Block must NOT be at the top (floor position preserved = mid-file).
	if strings.HasPrefix(got, beginMarker) {
		t.Fatalf("block moved to top on update — should stay in place:\n%q", got)
	}

	// SC-1 idempotency: calling twice with same name + body yields byte-identical output.
	out2 := PrependBlockIfNotFound(out, "baseline-include", newBody)
	if string(out2) != got {
		t.Fatalf("idempotency failure — second call produced different output:\n first: %q\nsecond: %q", got, string(out2))
	}
}
