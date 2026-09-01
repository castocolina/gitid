package tuikit

import "testing"

// TestFrozenGitIgnoreCopy is the AUTHORITATIVE, byte-exact contract for
// every Global Git Ignore (09.2-01 / 09.2-UI-SPEC.md Copywriting Contract)
// frozen copy constant declared in design.go's "Global Git Ignore screen
// copy" section. `make gate-copy-freeze` is a SECONDARY source-presence
// guard (a comment or dead declaration would satisfy a plain grep) — before
// this test existed, the gate registered NO Global Git Ignore copy at all,
// and the only tests exercising these constants built their "want" from the
// SAME function under test (tautological with respect to wording, not just
// plumbing). THIS test is what actually pins the value (09.2-REVIEW.md
// WR-07, matching upload_copy_test.go's TestFrozenUploadCopy precedent).
func TestFrozenGitIgnoreCopy(t *testing.T) {
	cases := []struct {
		name string
		got  string
		want string
	}{
		{"GitIgnoreHeading", GitIgnoreHeading, "Global Git Ignore"},
		{"GitIgnoreWiringWired", GitIgnoreWiringWired, "✓ Wired — core.excludesfile points at this file; Git reads it."},
		{"GitIgnoreWiringKeyUnset", GitIgnoreWiringKeyUnset, "! core.excludesfile is not set — Git does not read any global ignore file yet. Confirming here will set it."},
		{"GitIgnoreWiringNoBaseline", GitIgnoreWiringNoBaseline, "! No gitid-managed Git baseline configuration was found on this machine — open the Fixer to set that up before this screen can wire core.excludesfile."},
		{"GitIgnoreNoManagedBlock", GitIgnoreNoManagedBlock, "No managed block found yet in ~/.gitignore_global — showing the curated defaults below. Nothing has been written."},
		{"GitIgnoreTwoTargetNote", GitIgnoreTwoTargetNote, "This write will also set core.excludesfile in your Git baseline, since it is not set yet."},
		{"GitIgnoreReceiptSingleTarget", GitIgnoreReceiptSingleTarget, "Global gitignore written to ~/.gitignore_global."},
		{"GitIgnoreReceiptTwoTarget", GitIgnoreReceiptTwoTarget, "Global gitignore written to ~/.gitignore_global. core.excludesfile was also set to point at this file."},
		{"GitIgnoreReceiptNoBackup", GitIgnoreReceiptNoBackup, "No backup was needed — the content was unchanged or the file is new."},
		{"GitIgnoreReceiptChangedSincePreview", GitIgnoreReceiptChangedSincePreview, "This file changed since you last reviewed it — press a to review the current content again before writing."},
		{"GitIgnoreCeremonyHeading", GitIgnoreCeremonyHeading, "Review your global gitignore before writing."},
		{"GitIgnoreEditLabel", GitIgnoreEditLabel, "Edit"},
		{"GitIgnoreResetLabel", GitIgnoreResetLabel, "Reset to defaults"},
		{"GitIgnoreApplyLabel", GitIgnoreApplyLabel, "Review & write"},
		{"GitIgnoreDoneEditingLabel", GitIgnoreDoneEditingLabel, "Done editing"},
		{"GitIgnoreDiscardedEditsStatus", GitIgnoreDiscardedEditsStatus, "Leaving this screen discards unsaved edits."},
		{"GitIgnoreMalformedReasonUnclosedMarker", GitIgnoreMalformedReasonUnclosedMarker, "an opening marker with no matching closing marker"},
		{"GitIgnoreMalformedReasonMismatchedMarker", GitIgnoreMalformedReasonMismatchedMarker, "a closing marker with no valid matching opening marker"},
		{"GitIgnoreMalformedReasonDuplicateBlock", GitIgnoreMalformedReasonDuplicateBlock, "two complete gitid blocks in one file"},
		{"GitIgnoreMalformedReasonUnspecified", GitIgnoreMalformedReasonUnspecified, "a malformed gitid marker"},
		{"GitIgnoreWiringPointsElsewhere(...)", GitIgnoreWiringPointsElsewhere("~/other-file"), "! core.excludesfile points at ~/other-file instead of this file — that choice is left alone; writing here only affects the file below."},
		{"GitIgnoreMalformedFileMessage(...)", GitIgnoreMalformedFileMessage("~/.gitignore_global", 3, GitIgnoreMalformedReasonUnclosedMarker), "~/.gitignore_global has a broken gitid marker at line 3 (an opening marker with no matching closing marker) — repair the file by hand before this screen can read or write it."},
		{"GitIgnoreSentinelRejectedMessage(...)", GitIgnoreSentinelRejectedMessage(2), "✗ Line 2 looks like a gitid managed-block marker and can't be part of your content — edit or remove that line before applying."},
		{"GitIgnoreReceiptWrongTarget(...)", GitIgnoreReceiptWrongTarget("~/my-own-ignore"), "Global gitignore written to ~/.gitignore_global. core.excludesfile still points at ~/my-own-ignore, so Git is not reading this file — that setting was left as you configured it."},
	}

	const wantCount = 24
	if len(cases) != wantCount {
		t.Fatalf("TestFrozenGitIgnoreCopy covers %d constants, want %d — a row was forgotten or double-counted", len(cases), wantCount)
	}

	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %q, want the frozen 09.2-UI-SPEC.md Copywriting Contract value %q", c.name, c.got, c.want)
		}
	}
}
