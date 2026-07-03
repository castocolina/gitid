package dummytui

import (
	"strings"
	"testing"
)

// wantFIXScreens is the full 6-screen set from
// .planning/design/fixer/manifest.json, reused by every test below that
// walks the full screen set.
var wantFIXScreens = []string{
	"fixer-list", "fix-preview", "confirm-destructive",
	"backup-notice", "result-applied", "nothing-to-fix",
}

// TestFixer_RegistersSixScreensAsSoleOwnerOfActivationKeyFive asserts the
// fixer surface registers exactly 6 screens, claims ActivationKey "5",
// and is the SOLE surface currently claiming "5" -- i.e.
// RegisterOrReplace actually replaced the 02-02 data.go placeholder
// rather than merely registering alongside it (review HIGH-2).
func TestFixer_RegistersSixScreensAsSoleOwnerOfActivationKeyFive(t *testing.T) {
	sd, ok := lookupSurface("fixer")
	if !ok {
		t.Fatal("fixer surface not registered — surface_fixer.go init() did not run?")
	}
	if len(sd.Screens) != 6 {
		t.Fatalf("fixer: %d screens registered, want 6", len(sd.Screens))
	}
	if sd.ActivationKey != "5" {
		t.Fatalf("fixer: ActivationKey = %q, want %q", sd.ActivationKey, "5")
	}
	if sd.Title == "" {
		t.Fatal("fixer: Title is empty")
	}

	var owners []string
	for _, other := range Surfaces() {
		if other.ActivationKey == "5" {
			owners = append(owners, other.ID)
		}
	}
	if len(owners) != 1 || owners[0] != "fixer" {
		t.Fatalf("fixer: ActivationKey %q owners = %v, want exactly [fixer] (RegisterOrReplace must replace the data.go placeholder, not coexist with it)", "5", owners)
	}
}

// TestFixer_EveryScreenRendersNonEmptyWithSignatureAndBreadcrumb walks
// every screen ID in manifest.json's screen set and asserts
// RenderScreen("fixer", id) is non-empty, contains that screen's
// manifest signature, and contains the "fixer/<id>" breadcrumb.
func TestFixer_EveryScreenRendersNonEmptyWithSignatureAndBreadcrumb(t *testing.T) {
	if len(wantFIXScreens) != 6 {
		t.Fatalf("test setup: wantFIXScreens has %d entries, want 6", len(wantFIXScreens))
	}

	for _, id := range wantFIXScreens {
		id := id
		t.Run(id, func(t *testing.T) {
			out, err := RenderScreen("fixer", id)
			if err != nil {
				t.Fatalf("RenderScreen(fixer, %s): unexpected error: %v", id, err)
			}
			if out == "" {
				t.Fatalf("RenderScreen(fixer, %s): empty output", id)
			}
			sig, ok := fixSignatureByScreen[id]
			if !ok {
				t.Fatalf("test setup: no signature registered for screen %q in fixSignatureByScreen", id)
			}
			if !strings.Contains(out, sig) {
				t.Fatalf("RenderScreen(fixer, %s): output missing signature %q:\n%s", id, sig, out)
			}
			breadcrumb := "fixer/" + id
			if !strings.Contains(out, breadcrumb) {
				t.Fatalf("RenderScreen(fixer, %s): output missing breadcrumb %q:\n%s", id, breadcrumb, out)
			}
		})
	}
}

// TestFixer_SignaturesAreAllUnique guards against a copy-paste signature
// collision across screens (review HIGH-3c: a signature must be a
// screen-specific marker, never a generic reused string).
func TestFixer_SignaturesAreAllUnique(t *testing.T) {
	seen := map[string]string{}
	for id, sig := range fixSignatureByScreen {
		if prevID, dup := seen[sig]; dup {
			t.Fatalf("signature %q reused by both screen %q and screen %q", sig, prevID, id)
		}
		seen[sig] = id
	}
	if len(seen) != 6 {
		t.Fatalf("fixSignatureByScreen has %d unique signatures, want 6", len(seen))
	}
}

// TestFixer_ListAndNothingToFixHaveBothSSHAndGitSections asserts FIX-02
// (§4.7 "two sections") on the two screens that present a FULL
// snapshot (fixer-list, nothing-to-fix): each carries BOTH an "SSH" and
// a "Git" section heading, never merged.
func TestFixer_ListAndNothingToFixHaveBothSSHAndGitSections(t *testing.T) {
	for _, id := range []string{"fixer-list", "nothing-to-fix"} {
		id := id
		t.Run(id, func(t *testing.T) {
			out, err := RenderScreen("fixer", id)
			if err != nil {
				t.Fatalf("RenderScreen(fixer, %s): %v", id, err)
			}
			if !strings.Contains(out, "SSH") {
				t.Errorf("RenderScreen(fixer, %s): missing the SSH section (FIX-02):\n%s", id, out)
			}
			if !strings.Contains(out, "Git") {
				t.Errorf("RenderScreen(fixer, %s): missing the Git section (FIX-02):\n%s", id, out)
			}
		})
	}
}

// TestFixer_SeverityExplanationSuggestedFixPresent asserts FIX-01: every
// fixer-list problem row carries a severity glyph+word AND its suggested
// fix text.
func TestFixer_SeverityExplanationSuggestedFixPresent(t *testing.T) {
	out, err := RenderScreen("fixer", "fixer-list")
	if err != nil {
		t.Fatalf("RenderScreen(fixer, fixer-list): %v", err)
	}
	for _, f := range fixFindings {
		if !strings.Contains(out, f.title) {
			t.Errorf("fixer-list: missing finding title %q", f.title)
		}
		if !strings.Contains(out, f.suggestedFix) {
			t.Errorf("fixer-list: missing finding suggestedFix %q", f.suggestedFix)
		}
	}
	if !strings.Contains(out, "! warning") && !strings.Contains(out, "✗ error") && !strings.Contains(out, "✗ critical") {
		t.Errorf("fixer-list: expected at least one severity glyph+word pairing:\n%s", out)
	}
}

// TestFixer_FixInPlaceDiffShowsRewriteNotAddition asserts the §4.7
// highest-risk affordance (T-02-FIX): fix-preview shows a TRUE
// before/after `-`/`+` rewrite diff of an EXISTING directive, not an
// additions-only `+` list (unlike global-ssh's/global-git's fix-preview).
func TestFixer_FixInPlaceDiffShowsRewriteNotAddition(t *testing.T) {
	out, err := RenderScreen("fixer", "fix-preview")
	if err != nil {
		t.Fatalf("RenderScreen(fixer, fix-preview): %v", err)
	}
	if !strings.Contains(out, "-     IdentitiesOnly no") {
		t.Errorf("fix-preview: missing the `-` (removed) line of the rewrite diff:\n%s", out)
	}
	if !strings.Contains(out, "+     IdentitiesOnly yes") {
		t.Errorf("fix-preview: missing the `+` (added) line of the rewrite diff:\n%s", out)
	}
	if !strings.Contains(out, "REWRITES") {
		t.Errorf("fix-preview: missing the explicit rewrite-not-addition framing:\n%s", out)
	}
	if !strings.Contains(out, fixTargetFile) {
		t.Errorf("fix-preview: missing the target file %q:\n%s", fixTargetFile, out)
	}
}

// TestFixer_ConfirmDestructiveNeverDefaultsToYes asserts §5's destructive-
// confirm rule: confirm-destructive states the default focus is "No,
// cancel" and never states a default-focused "yes".
func TestFixer_ConfirmDestructiveNeverDefaultsToYes(t *testing.T) {
	out, err := RenderScreen("fixer", "confirm-destructive")
	if err != nil {
		t.Fatalf("RenderScreen(fixer, confirm-destructive): %v", err)
	}
	if !strings.Contains(out, "Default-focused: No, cancel") {
		t.Errorf("confirm-destructive: missing the default-focused-No statement (§5):\n%s", out)
	}
	if strings.Contains(out, "Default-focused: Yes") {
		t.Errorf("confirm-destructive: destructive actions must never default-focus yes (§5):\n%s", out)
	}
}

// TestFixer_BackupNoticeNamesPathBeforeApplying asserts §4.7's backup
// affordance: backup-notice names the timestamped backup path.
func TestFixer_BackupNoticeNamesPathBeforeApplying(t *testing.T) {
	out, err := RenderScreen("fixer", "backup-notice")
	if err != nil {
		t.Fatalf("RenderScreen(fixer, backup-notice): %v", err)
	}
	if !strings.Contains(out, fixBackupPath) {
		t.Errorf("backup-notice: missing the timestamped backup path %q:\n%s", fixBackupPath, out)
	}
	if !strings.Contains(out, fixTargetFile) {
		t.Errorf("backup-notice: missing the target file %q:\n%s", fixTargetFile, out)
	}
}

// TestFixer_ResultAppliedNamesRestorePath asserts §5 beat 4: result-applied
// states what changed and restates the backup path for restore.
func TestFixer_ResultAppliedNamesRestorePath(t *testing.T) {
	out, err := RenderScreen("fixer", "result-applied")
	if err != nil {
		t.Fatalf("RenderScreen(fixer, result-applied): %v", err)
	}
	if !strings.Contains(out, fixResultMessage) {
		t.Errorf("result-applied: missing the result message %q:\n%s", fixResultMessage, out)
	}
	if !strings.Contains(out, fixBackupPath) {
		t.Errorf("result-applied: missing the restore backup path %q:\n%s", fixBackupPath, out)
	}
	if !strings.Contains(out, "preserved verbatim") {
		t.Errorf("result-applied: missing the preserved-verbatim containment note:\n%s", out)
	}
}

// TestFixer_NothingToFixIsTheHealthyEmptyState asserts §4.7's healthy
// empty state: nothing-to-fix reports zero fixable problems for both
// sections, with a success (green ✓) glyph.
func TestFixer_NothingToFixIsTheHealthyEmptyState(t *testing.T) {
	out, err := RenderScreen("fixer", "nothing-to-fix")
	if err != nil {
		t.Fatalf("RenderScreen(fixer, nothing-to-fix): %v", err)
	}
	if !strings.Contains(out, "0 fixable problems") {
		t.Errorf("nothing-to-fix: missing the zero-findings statement:\n%s", out)
	}
	if !strings.Contains(out, "✓") {
		t.Errorf("nothing-to-fix: missing the success glyph:\n%s", out)
	}
}

// TestFixer_SafetyNoteOnEveryScreen asserts the fix-in-place safety
// affordance statement (§4.7, §5) appears on ALL 6 screens.
func TestFixer_SafetyNoteOnEveryScreen(t *testing.T) {
	for _, id := range wantFIXScreens {
		id := id
		t.Run(id, func(t *testing.T) {
			out, err := RenderScreen("fixer", id)
			if err != nil {
				t.Fatalf("RenderScreen(fixer, %s): %v", id, err)
			}
			if !strings.Contains(out, fixSafetyNote) {
				t.Fatalf("RenderScreen(fixer, %s): missing the safety banner %q:\n%s", id, fixSafetyNote, out)
			}
		})
	}
}

// TestFixer_BatchFixNotePreviewsEveryChange asserts §4.7's "batch-fix
// must still preview every change; no silent multi-file mutation" rule
// is stated on fixer-list.
func TestFixer_BatchFixNotePreviewsEveryChange(t *testing.T) {
	out, err := RenderScreen("fixer", "fixer-list")
	if err != nil {
		t.Fatalf("RenderScreen(fixer, fixer-list): %v", err)
	}
	if !strings.Contains(out, "each one still previews its own diff") {
		t.Errorf("fixer-list: missing the batch-fix-still-previews note (§4.7):\n%s", out)
	}
	if strings.Contains(strings.ToLower(out), "silently") && !strings.Contains(out, "nothing is applied silently") {
		t.Errorf("fixer-list: unexpected 'silently' language not matching the no-silent-mutation note:\n%s", out)
	}
}

// TestFixer_KeysFormAConnectedPathReachingEveryScreen walks the
// surface's ScreenDef.Keys graph, starting from the entry screen
// (fixer-list), and asserts every one of the 6 screens is reachable --
// mirroring the same transitions manifest.json's keysFromHome walk
// (review C3).
func TestFixer_KeysFormAConnectedPathReachingEveryScreen(t *testing.T) {
	sd, ok := lookupSurface("fixer")
	if !ok {
		t.Fatal("fixer surface not registered")
	}
	if entryScreenID(sd) != "fixer-list" {
		t.Fatalf("fixer: entry screen = %q, want fixer-list", entryScreenID(sd))
	}

	byID := map[string]ScreenDef{}
	for _, scr := range sd.Screens {
		byID[scr.ID] = scr
	}

	visited := map[string]bool{}
	queue := []string{entryScreenID(sd)}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if visited[cur] {
			continue
		}
		visited[cur] = true
		scr, ok := byID[cur]
		if !ok {
			t.Fatalf("fixer: screen %q referenced by a transition but not registered", cur)
		}
		for _, target := range scr.Keys {
			if !visited[target] {
				queue = append(queue, target)
			}
		}
	}

	if len(visited) != len(sd.Screens) {
		var missing []string
		for _, scr := range sd.Screens {
			if !visited[scr.ID] {
				missing = append(missing, scr.ID)
			}
		}
		t.Fatalf("fixer: only %d/%d screens reachable from entry %q via ScreenDef.Keys; unreachable: %v", len(visited), len(sd.Screens), entryScreenID(sd), missing)
	}
}

// TestFixer_IntraSurfaceKeysNeverReuseLaunchKeysNOrG asserts no screen's
// ScreenDef.Keys claims "n" or "g" -- create-flow's and git-screen's own
// LaunchKeys (02-UX-DIRECTION.md §2 key-allocation table) -- which would
// have already failed loudly at RegisterOrReplace time via registry.go's
// collision guard; this test documents and guards the invariant directly
// against the live registry too.
func TestFixer_IntraSurfaceKeysNeverReuseLaunchKeysNOrG(t *testing.T) {
	sd, ok := lookupSurface("fixer")
	if !ok {
		t.Fatal("fixer surface not registered")
	}
	for _, scr := range sd.Screens {
		for k := range scr.Keys {
			if k == "n" || k == "g" {
				t.Fatalf("fixer: screen %q claims key %q, which collides with create-flow/git-screen's own LaunchKey", scr.ID, k)
			}
		}
	}
}

// TestFixer_TargetFindingTracesTheSameFindingAsHealth asserts the
// traceability claim directly: fixer's flagship walk-through target
// (fixTarget) is byte-identical (by id) to health's own
// hlthFindingDetailTarget -- the SAME finding, not a re-derived
// duplicate (HLTH-04's "available on the Fixer screen" hand-off, honored
// concretely).
func TestFixer_TargetFindingTracesTheSameFindingAsHealth(t *testing.T) {
	if fixTarget.id != "ssh-identitiesonly-contradiction" {
		t.Fatalf("fixTarget.id = %q, want %q", fixTarget.id, "ssh-identitiesonly-contradiction")
	}
	if fixTarget.id != hlthFindingDetailTarget.id {
		t.Fatalf("fixTarget.id = %q diverges from hlthFindingDetailTarget.id = %q -- the fixer's flagship target must trace the SAME finding health/finding-detail deep-dives", fixTarget.id, hlthFindingDetailTarget.id)
	}
	if fixTarget.title != hlthFindingDetailTarget.title || fixTarget.explanation != hlthFindingDetailTarget.explanation {
		t.Fatalf("fixTarget diverges from hlthFindingDetailTarget on title/explanation copy:\n%+v\nvs\n%+v", fixTarget, hlthFindingDetailTarget)
	}
}
