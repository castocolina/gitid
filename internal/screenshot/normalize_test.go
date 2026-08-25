//go:build screenshot

package screenshot

import (
	"strings"
	"testing"
)

// TestNormalizeTimestampsAnchorsBackupSuffix proves the WR-09 fix: the
// unwrapped ".bak.<nanoseconds>" case is normalized via the anchored
// backupSuffixPattern, not a blanket "any 6+ digit run" match.
func TestNormalizeTimestampsAnchorsBackupSuffix(t *testing.T) {
	in := "Backed up → ~/.gitconfig.bak.1787620111536588000"
	got := normalizeTimestamps(in)
	want := "Backed up → ~/.gitconfig.bak.<digits>"
	if got != want {
		t.Errorf("normalizeTimestamps(%q) = %q, want %q", in, got, want)
	}
}

// TestNormalizeTimestampsHandlesWrappedBackupSuffix proves the wrapped-
// fragment fallback still normalizes a nanosecond suffix that split across
// a line wrap in the fixed-width pane (the exact shape captured in
// git-configuration-mouse-field-focus.txt: the ".bak.17" prefix ends one
// line, the remaining digits alone occupy the next).
func TestNormalizeTimestampsHandlesWrappedBackupSuffix(t *testing.T) {
	in := "                                    │ uration_RealPTYMouseFieldFocus3363557519/001/.gitconfig.bak.17\n" +
		"                                    │ 87620111627806000\n"
	got := normalizeTimestamps(in)
	if strings.Contains(got, "87620111627806000") {
		t.Errorf("normalizeTimestamps did not normalize the wrapped digit continuation:\n%s", got)
	}
	if !strings.Contains(got, "<digits>") {
		t.Errorf("normalizeTimestamps produced no <digits> placeholder for the wrapped fragment:\n%s", got)
	}
	// The border character on the wrapped-continuation row must survive —
	// only the digit run itself is replaced, never the row's own padding.
	if !strings.Contains(got, "│ <digits>") {
		t.Errorf("normalizeTimestamps must preserve the border/padding around a wrapped digit row:\n%s", got)
	}
}

// TestNormalizeTimestampsPreservesInlineNumericContent proves the WR-09
// fix's core guarantee: a 6+ digit number embedded ALONGSIDE other text on
// its row (a byte count, a key size, a future numeric ID) must survive
// normalization — the old blanket \d{6,} pattern erased these
// indiscriminately, silently masking a genuine real-vs-dummy divergence.
func TestNormalizeTimestampsPreservesInlineNumericContent(t *testing.T) {
	in := "                                    │ wrote 123456 bytes to ~/.gitconfig\n"
	got := normalizeTimestamps(in)
	if !strings.Contains(got, "123456") {
		t.Errorf("normalizeTimestamps erased inline numeric content it must preserve:\n%s", got)
	}
}
