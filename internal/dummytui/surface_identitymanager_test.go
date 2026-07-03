package dummytui

import (
	"strings"
	"testing"
)

// wantIMScreens is the full 8-screen set from
// .planning/design/identity-manager/manifest.json, reused by every test
// below that walks the full screen set.
var wantIMScreens = []string{
	"list-populated", "list-empty", "detail-ssh-first", "action-menu",
	"clone-name-prompt", "delete-choice", "confirm-destructive", "backup-notice",
}

// TestIdentityManager_RegistersEightScreensAsSoleOwnerOfActivationKeyOne
// asserts the identity-manager surface registers exactly 8 screens, claims
// ActivationKey "1" (the app's HOME/view-1), and is the SOLE surface
// currently claiming "1" — i.e. RegisterOrReplace actually replaced the
// 02-02 data.go placeholder rather than merely registering alongside it
// (review HIGH-2).
func TestIdentityManager_RegistersEightScreensAsSoleOwnerOfActivationKeyOne(t *testing.T) {
	sd, ok := lookupSurface("identity-manager")
	if !ok {
		t.Fatal("identity-manager surface not registered — surface_identitymanager.go init() did not run?")
	}
	if len(sd.Screens) != 8 {
		t.Fatalf("identity-manager: %d screens registered, want 8", len(sd.Screens))
	}
	if sd.ActivationKey != "1" {
		t.Fatalf("identity-manager: ActivationKey = %q, want %q", sd.ActivationKey, "1")
	}
	if sd.Title == "" {
		t.Fatal("identity-manager: Title is empty")
	}

	var owners []string
	for _, other := range Surfaces() {
		if other.ActivationKey == "1" {
			owners = append(owners, other.ID)
		}
	}
	if len(owners) != 1 || owners[0] != "identity-manager" {
		t.Fatalf("identity-manager: ActivationKey %q owners = %v, want exactly [identity-manager] (RegisterOrReplace must replace the data.go placeholder, not coexist with it)", "1", owners)
	}
}

// TestIdentityManager_EveryScreenRendersNonEmptyWithSignatureAndBreadcrumb
// walks every screen ID in manifest.json's screen set and asserts
// RenderScreen("identity-manager", id) is non-empty, contains that screen's
// manifest signature, and contains the "identity-manager/<id>" breadcrumb.
func TestIdentityManager_EveryScreenRendersNonEmptyWithSignatureAndBreadcrumb(t *testing.T) {
	if len(wantIMScreens) != 8 {
		t.Fatalf("test setup: wantIMScreens has %d entries, want 8", len(wantIMScreens))
	}

	for _, id := range wantIMScreens {
		id := id
		t.Run(id, func(t *testing.T) {
			out, err := RenderScreen("identity-manager", id)
			if err != nil {
				t.Fatalf("RenderScreen(identity-manager, %s): unexpected error: %v", id, err)
			}
			if out == "" {
				t.Fatalf("RenderScreen(identity-manager, %s): empty output", id)
			}
			sig, ok := imSignatureByScreen[id]
			if !ok {
				t.Fatalf("test setup: no signature registered for screen %q in imSignatureByScreen", id)
			}
			if !strings.Contains(out, sig) {
				t.Fatalf("RenderScreen(identity-manager, %s): output missing signature %q:\n%s", id, sig, out)
			}
			breadcrumb := "identity-manager/" + id
			if !strings.Contains(out, breadcrumb) {
				t.Fatalf("RenderScreen(identity-manager, %s): output missing breadcrumb %q:\n%s", id, breadcrumb, out)
			}
		})
	}
}

// TestIdentityManager_SignaturesAreAllUnique guards against a copy-paste
// signature collision across screens (review HIGH-3c: a signature must be a
// screen-specific marker, never a generic reused string).
func TestIdentityManager_SignaturesAreAllUnique(t *testing.T) {
	seen := map[string]string{}
	for id, sig := range imSignatureByScreen {
		if prevID, dup := seen[sig]; dup {
			t.Fatalf("signature %q reused by both screen %q and screen %q", sig, prevID, id)
		}
		seen[sig] = id
	}
	if len(seen) != 8 {
		t.Fatalf("imSignatureByScreen has %d unique signatures, want 8", len(seen))
	}
}

// TestIdentityManager_EightLabelTaxonomyAnchors asserts all 8 MGR-02 state
// labels (internal/identity/state.go's locked vocabulary) appear, as WORDS
// (not just color), somewhere across the rendered screen set — the
// NO_COLOR-legibility proof.
func TestIdentityManager_EightLabelTaxonomyAnchors(t *testing.T) {
	var all strings.Builder
	for _, id := range wantIMScreens {
		out, err := RenderScreen("identity-manager", id)
		if err != nil {
			t.Fatalf("RenderScreen(identity-manager, %s): %v", id, err)
		}
		all.WriteString(out)
		all.WriteString("\n")
	}
	combined := all.String()

	for _, want := range []string{
		"complete", "incomplete", "git-only", "key-unused",
		"key-used-ssh-only", "key-used-both", "key-missing", "fragment-path-missing",
	} {
		if !strings.Contains(combined, want) {
			t.Errorf("identity-manager: combined screen output missing MGR-02 label %q", want)
		}
	}
}

// TestIdentityManager_DeleteChoiceSafeDefaultNeverDefaultsToEverything
// asserts delete-choice's safer option ("Delete Git identity only") is
// marked as the default, and the irreversible "everything" option is
// present but never marked default (MGR-06, §5).
func TestIdentityManager_DeleteChoiceSafeDefaultNeverDefaultsToEverything(t *testing.T) {
	out, err := RenderScreen("identity-manager", "delete-choice")
	if err != nil {
		t.Fatalf("RenderScreen(identity-manager, delete-choice): %v", err)
	}
	if !strings.Contains(out, imDeleteChoiceGitOnly) {
		t.Fatalf("delete-choice: missing the safer option %q:\n%s", imDeleteChoiceGitOnly, out)
	}
	if !strings.Contains(out, imDeleteChoiceEverything) {
		t.Fatalf("delete-choice: missing the irreversible option %q:\n%s", imDeleteChoiceEverything, out)
	}
	if !strings.Contains(out, imDeleteChoiceGitOnly+" ✓ default") {
		t.Fatalf("delete-choice: the safer option %q is not marked default:\n%s", imDeleteChoiceGitOnly, out)
	}
	if strings.Contains(out, imDeleteChoiceEverything+" ✓ default") {
		t.Fatalf("delete-choice: the irreversible option %q must NEVER be marked default (§5):\n%s", imDeleteChoiceEverything, out)
	}
}

// TestIdentityManager_ModalScreensUsePlaceOverlayWithoutPanic exercises
// every placeOverlay-composited screen (action-menu, clone-name-prompt,
// delete-choice, confirm-destructive, backup-notice) and asserts each
// renders without panicking and produces non-empty output containing its
// own signature — the modal-compositing no-panic proof.
func TestIdentityManager_ModalScreensUsePlaceOverlayWithoutPanic(t *testing.T) {
	modalScreens := []string{"action-menu", "clone-name-prompt", "delete-choice", "confirm-destructive", "backup-notice"}
	for _, id := range modalScreens {
		id := id
		t.Run(id, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("RenderScreen(identity-manager, %s) panicked: %v", id, r)
				}
			}()
			out, err := RenderScreen("identity-manager", id)
			if err != nil {
				t.Fatalf("RenderScreen(identity-manager, %s): %v", id, err)
			}
			sig := imSignatureByScreen[id]
			if !strings.Contains(out, sig) {
				t.Fatalf("RenderScreen(identity-manager, %s): missing signature %q:\n%s", id, sig, out)
			}
		})
	}
}

// TestIdentityManager_KeysFormAConnectedPathReachingEveryScreen walks the
// surface's ScreenDef.Keys graph, starting from the entry screen
// (list-populated), and asserts every one of the 8 screens is reachable —
// mirroring the same transitions manifest.json's keysFromHome walk (review
// C3).
func TestIdentityManager_KeysFormAConnectedPathReachingEveryScreen(t *testing.T) {
	sd, ok := lookupSurface("identity-manager")
	if !ok {
		t.Fatal("identity-manager surface not registered")
	}
	if entryScreenID(sd) != "list-populated" {
		t.Fatalf("identity-manager: entry screen = %q, want list-populated", entryScreenID(sd))
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
			t.Fatalf("identity-manager: screen %q referenced by a transition but not registered", cur)
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
		t.Fatalf("identity-manager: only %d/%d screens reachable from entry %q via ScreenDef.Keys; unreachable: %v", len(visited), len(sd.Screens), entryScreenID(sd), missing)
	}
}

// TestIdentityManager_IntraSurfaceKeysNeverReuseLaunchKeysNOrG asserts no
// screen's ScreenDef.Keys claims "n" or "g" — create-flow's and
// git-screen's own LaunchKeys (02-UX-DIRECTION.md §2 key-allocation table)
// — which would have already failed loudly at RegisterOrReplace time via
// registry.go's collision guard; this test documents and guards the
// invariant directly against the live registry too.
func TestIdentityManager_IntraSurfaceKeysNeverReuseLaunchKeysNOrG(t *testing.T) {
	sd, ok := lookupSurface("identity-manager")
	if !ok {
		t.Fatal("identity-manager surface not registered")
	}
	for _, scr := range sd.Screens {
		for k := range scr.Keys {
			if k == "n" || k == "g" {
				t.Fatalf("identity-manager: screen %q claims key %q, which collides with create-flow/git-screen's own LaunchKey", scr.ID, k)
			}
		}
	}
}
