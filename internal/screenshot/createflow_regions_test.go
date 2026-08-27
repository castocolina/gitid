//go:build screenshot

package screenshot

import (
	"strings"
	"testing"
)

// TestExtractGSSApplyHeadingAbsorbsWrappedContinuationRow is the WR-12
// regression: extractGSSApplyHeading's doc comment promises it "absorbs an
// immediately-following wrapped continuation row if the resolved target
// wraps past the ceremony's width" — before the fix, the wrapped
// continuation line was never appended to out; the function always returned
// after the FIRST matching line (or, when the very next row was the
// "Touches" row, broke out and returned that first line too, with no
// continuation content either way). A resolved target long enough to wrap
// (the common case for a sandbox HOME like
// /tmp/h4042748673/.ssh/config.d/gitid.config) silently dropped its tail
// from the T-06-CEREMONYTARGET comparison.
func TestExtractGSSApplyHeadingAbsorbsWrappedContinuationRow(t *testing.T) {
	lines := []string{
		"some unrelated preceding line",
		"Write Host * managed block to /tmp/h4042748673/.ssh/config.d/",
		"gitid.config",
		"Touches: personal, work",
		"some unrelated trailing line",
	}
	got := extractGSSApplyHeading(lines)
	if !strings.Contains(got, "Write Host * managed block to") {
		t.Fatalf("extracted heading missing the anchor line: %q", got)
	}
	if !strings.Contains(got, "gitid.config") {
		t.Errorf("extracted heading dropped the wrapped continuation row; WR-12 regressed: %q", got)
	}
	if strings.Contains(got, "Touches") {
		t.Errorf("extracted heading leaked the Touches row, which is not part of the heading: %q", got)
	}
	if strings.Contains(got, "unrelated") {
		t.Errorf("extracted heading leaked an unrelated line: %q", got)
	}
}

// TestExtractGSSApplyHeadingSingleLineNoWrap proves the non-wrapped case
// (heading immediately followed by "Touches") still returns just the one
// heading line — the fix must not over-absorb when there is no wrap.
func TestExtractGSSApplyHeadingSingleLineNoWrap(t *testing.T) {
	lines := []string{
		"Write Host * managed block to ~/.ssh/config",
		"Touches: personal",
	}
	got := extractGSSApplyHeading(lines)
	want := "Write Host * managed block to ~/.ssh/config"
	if got != want {
		t.Errorf("extractGSSApplyHeading() = %q, want %q", got, want)
	}
}
