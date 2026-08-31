package tuikit

import (
	"strings"
	"testing"
)

// TestFrozenUploadCopy is the AUTHORITATIVE, byte-exact contract for every
// Phase 9 (Upload / Credentials Assist, D-08) frozen copy constant declared
// in design.go's "Upload / Credentials Assist" section — 09-UI-SPEC.md's
// Copywriting Contract, checker-approved 2026-08-28. `make gate-copy-freeze`
// is a SECONDARY source-presence guard (a comment or dead declaration would
// satisfy a plain grep); THIS test is what actually pins the value (R21,
// 09-01-PLAN.md cross-AI review).
func TestFrozenUploadCopy(t *testing.T) {
	cases := []struct {
		name string
		got  string
		want string
	}{
		{"UploadCheckboxLabelReadyFmt", UploadCheckboxLabelReadyFmt, "Register with %s automatically (auth + signing)"},
		{"UploadCheckboxLabelUnauthFmt", UploadCheckboxLabelUnauthFmt, "Register with %s automatically — not logged in to %s; run \"%s auth login\" first, or check anyway"},
		{"UploadCheckboxLabelDisabledFmt", UploadCheckboxLabelDisabledFmt, "Auto-registration unavailable — %s has no gh/glab match here. Manual steps are shown after create."},
		{"UploadRunningLineFmt", UploadRunningLineFmt, "Running: %s"},
		{"UploadResultOKFmt", UploadResultOKFmt, "✓ %s key registered"},
		{"UploadResultSkippedFmt", UploadResultSkippedFmt, "✓ %s key already registered (skipped)"},
		{"UploadResultFailedFmt", UploadResultFailedFmt, "✗ %s key registration failed: %s"},
		{"UploadUnconfirmedReasonFmt", UploadUnconfirmedReasonFmt, "accepted but not yet visible in %s's inventory — this can lag briefly after upload; re-run gitid's test to confirm"},
		{"UploadScopeRemediationAuthFmt", UploadScopeRemediationAuthFmt, "insufficient scope — run \"gh auth refresh -h %s -s admin:public_key\", then retry from the Identity Manager"},
		{"UploadScopeRemediationSigningFmt", UploadScopeRemediationSigningFmt, "insufficient scope — run \"gh auth refresh -h %s -s admin:ssh_signing_key\", then retry from the Identity Manager"},
		{"UploadCrossAccountConflict", UploadCrossAccountConflict, "GitLab rejected this key — it is already registered to a DIFFERENT account. If that's expected, remove it there first; otherwise check \"glab auth status\"."},
		{"UploadInventoryDegradedFmt", UploadInventoryDegradedFmt, "Could not check %s for existing keys — uploading anyway; duplicates are handled safely."},
		{"UploadDryRunNote", UploadDryRunNote, "--dry-run: the command(s) above were shown, not run."},
		{"UploadManualHeading", UploadManualHeading, "Auto-registration wasn't available. Register it yourself:"},
		{"UploadKeyTitleFmt", UploadKeyTitleFmt, "gitid: %s @ %s"},
		{"UploadSkippedByFlagNote", UploadSkippedByFlagNote, "Auto-upload skipped (--no-upload)."},
		{"UploadAlreadyCompleteFmt", UploadAlreadyCompleteFmt, "✓ Already registered with %s — nothing to do."},
		{"UploadRegistrationLabelAuth", UploadRegistrationLabelAuth, "Authentication"},
		{"UploadRegistrationLabelSigning", UploadRegistrationLabelSigning, "Signing"},
		{"UploadRegistrationLabelCombined", UploadRegistrationLabelCombined, "Key"},
		{"RotateDeleteOfferHeadingFmt", RotateDeleteOfferHeadingFmt, "Remove the old key from %s?"},
		{"RotateDeleteOfferBodyFmt", RotateDeleteOfferBodyFmt, "The old key (\"gitid: %s @ %s\") still authenticates there until you remove it. Delete it now?"},
		{"RotateDeleteOfferChoiceDeleteFmt", RotateDeleteOfferChoiceDeleteFmt, "[ Delete old key from %s ]"},
		{"RotateDeleteOfferChoiceLeave", RotateDeleteOfferChoiceLeave, "[ Leave it — I'll remove it myself ]"},
		{"RotateDeleteOfferResultRemovedFmt", RotateDeleteOfferResultRemovedFmt, "✓ Old key removed from %s."},
		{"RotateDeleteOfferResultLeftFmt", RotateDeleteOfferResultLeftFmt, "Left in place — remove it yourself: %s"},
		{"IdentityManagerActionRegisterKey", IdentityManagerActionRegisterKey, "Register key (u)"},
		{"RegisterKeyModalHeadingFmt", RegisterKeyModalHeadingFmt, "Register %s's key with %s"},
	}

	const wantCount = 28
	if len(cases) != wantCount {
		t.Fatalf("TestFrozenUploadCopy covers %d constants, want %d — a row was forgotten or double-counted", len(cases), wantCount)
	}

	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %q, want the frozen 09-UI-SPEC.md Copywriting Contract value %q", c.name, c.got, c.want)
		}
	}
}

// TestFrozenUploadCitedStringsUnchanged proves Phase 9 left the three CITED
// Copywriting Contract rows alone: stageWarningLine, keyUnusedResultMessage
// (identities.go:1483-1484), and keyCeremonyGraceHintFmt (identities.go:2434).
func TestFrozenUploadCitedStringsUnchanged(t *testing.T) {
	if stageWarningLine != "! Reachable — key not uploaded yet" {
		t.Errorf("stageWarningLine = %q, want unchanged CITED value", stageWarningLine)
	}
	if keyUnusedResultMessage != "Stored — key not uploaded yet; this identity is not proven for Git yet" {
		t.Errorf("keyUnusedResultMessage = %q, want unchanged CITED value", keyUnusedResultMessage)
	}
	if keyCeremonyGraceHintFmt != "The old key stays valid at %s during this window — upload the new key, verify it, then remove the old one there." {
		t.Errorf("keyCeremonyGraceHintFmt = %q, want unchanged CITED value", keyCeremonyGraceHintFmt)
	}
}

// TestUploadCheckboxLabelsCarryNoGlyph enforces this plan's deviation 1: the
// checkbox glyph pair is NOT baked into the label constants — the render
// composes the EXISTING glyphCheckOff/glyphCheckOn constants (theme.go) with
// the label, exactly as identities.go:4211-4215 already does for the
// demo-failure toggle. The forbidden runes are read from those existing
// constants (never re-typed) so this test cannot drift from the render.
func TestUploadCheckboxLabelsCarryNoGlyph(t *testing.T) {
	labels := map[string]string{
		"UploadCheckboxLabelReadyFmt":    UploadCheckboxLabelReadyFmt,
		"UploadCheckboxLabelUnauthFmt":   UploadCheckboxLabelUnauthFmt,
		"UploadCheckboxLabelDisabledFmt": UploadCheckboxLabelDisabledFmt,
	}
	forbidden := []string{glyphCheckOff, glyphCheckOn}

	for name, label := range labels {
		for _, glyph := range forbidden {
			if strings.Contains(label, glyph) {
				t.Errorf("%s = %q contains the checkbox glyph %q — the render must compose glyphCheckOff/glyphCheckOn with the label, not embed it in the constant", name, label, glyph)
			}
		}
	}
}
