package dummytui

import (
	"strings"
	"testing"
)

// TestGitScreen_RegistersSevenScreensKeylessWithLaunchBinding asserts the
// git-screen surface registers exactly 7 screens, is keyless (empty
// ActivationKey), and declares a LaunchFrom=="identity-manager" +
// non-empty LaunchKey binding (review C3, T-02-ML4).
func TestGitScreen_RegistersSevenScreensKeylessWithLaunchBinding(t *testing.T) {
	sd, ok := lookupSurface("git-screen")
	if !ok {
		t.Fatal("git-screen surface not registered — surface_gitscreen.go init() did not run?")
	}
	if len(sd.Screens) != 7 {
		t.Fatalf("git-screen: %d screens registered, want 7", len(sd.Screens))
	}
	if sd.ActivationKey != "" {
		t.Fatalf("git-screen: ActivationKey = %q, want empty (keyless modal surface)", sd.ActivationKey)
	}
	if sd.LaunchFrom != "identity-manager" {
		t.Fatalf("git-screen: LaunchFrom = %q, want identity-manager", sd.LaunchFrom)
	}
	if sd.LaunchKey == "" {
		t.Fatal("git-screen: LaunchKey is empty, want a non-empty launch key")
	}
}

// TestGitScreen_EveryScreenRendersNonEmptyWithSignatureAndBreadcrumb walks
// every screen ID in manifest.json's screen set and asserts
// RenderScreen("git-screen", id) is non-empty, contains that screen's
// manifest signature, and contains the "git-screen/<id>" breadcrumb.
func TestGitScreen_EveryScreenRendersNonEmptyWithSignatureAndBreadcrumb(t *testing.T) {
	wantScreens := []string{
		"git-form-empty", "git-form-filled", "match-strategy-select",
		"review-readonly", "confirm-write", "backup-notice", "result-success",
	}
	if len(wantScreens) != 7 {
		t.Fatalf("test setup: wantScreens has %d entries, want 7", len(wantScreens))
	}

	for _, id := range wantScreens {
		id := id
		t.Run(id, func(t *testing.T) {
			out, err := RenderScreen("git-screen", id)
			if err != nil {
				t.Fatalf("RenderScreen(git-screen, %s): unexpected error: %v", id, err)
			}
			if out == "" {
				t.Fatalf("RenderScreen(git-screen, %s): empty output", id)
			}
			sig, ok := gsSignatureByScreen[id]
			if !ok {
				t.Fatalf("test setup: no signature registered for screen %q in gsSignatureByScreen", id)
			}
			if !strings.Contains(out, sig) {
				t.Fatalf("RenderScreen(git-screen, %s): output missing signature %q:\n%s", id, sig, out)
			}
			breadcrumb := "git-screen/" + id
			if !strings.Contains(out, breadcrumb) {
				t.Fatalf("RenderScreen(git-screen, %s): output missing breadcrumb %q:\n%s", id, breadcrumb, out)
			}
		})
	}
}

// TestGitScreen_SignaturesAreAllUnique guards against a copy-paste
// signature collision across screens (review HIGH-3c: a signature must be a
// screen-specific marker, never a generic reused string).
func TestGitScreen_SignaturesAreAllUnique(t *testing.T) {
	seen := map[string]string{}
	for id, sig := range gsSignatureByScreen {
		if prevID, dup := seen[sig]; dup {
			t.Fatalf("signature %q reused by both screen %q and screen %q", sig, prevID, id)
		}
		seen[sig] = id
	}
	if len(seen) != 7 {
		t.Fatalf("gsSignatureByScreen has %d unique signatures, want 7", len(seen))
	}
}

// TestGitScreen_CopyParityAnchors asserts the recipe-critical literal
// values every screen must mirror byte-for-byte against the /mui mockup and
// recipeFixtures.ts are actually present in the rendered output somewhere
// in the flow (gpg.format=ssh, the allowed_signers line, the default
// gitdir match strategy).
func TestGitScreen_CopyParityAnchors(t *testing.T) {
	var all strings.Builder
	for _, id := range []string{
		"git-form-empty", "git-form-filled", "match-strategy-select",
		"review-readonly", "confirm-write", "backup-notice", "result-success",
	} {
		out, err := RenderScreen("git-screen", id)
		if err != nil {
			t.Fatalf("RenderScreen(git-screen, %s): %v", id, err)
		}
		all.WriteString(out)
		all.WriteString("\n")
	}
	combined := all.String()

	for _, want := range []string{"gpg.format", "allowed_signers", "gitdir", "hasconfig"} {
		if !strings.Contains(combined, want) {
			t.Errorf("git-screen: combined screen output missing recipe-accurate anchor %q", want)
		}
	}
}

// TestGitScreen_AllowedSignersEmailMatchesUserEmail is the machine-checkable
// proof of GITUI-04, the surface's highest-risk affordance: the
// allowed_signers line's email prefix must be byte-identical to
// user.email.
func TestGitScreen_AllowedSignersEmailMatchesUserEmail(t *testing.T) {
	if !strings.HasPrefix(gsAllowedSignersLine, gsUserEmail+" ") {
		t.Fatalf("git-screen: allowed_signers line %q does not start with user.email %q — GITUI-04 byte-identity violated", gsAllowedSignersLine, gsUserEmail)
	}
}

// TestGitScreen_KeysFormAConnectedPathReachingEveryScreen walks the
// surface's ScreenDef.Keys graph, starting from the entry screen
// (git-form-empty), and asserts every one of the 7 screens is reachable —
// mirroring the same transitions manifest.json's keysFromHome walk (review
// C3).
func TestGitScreen_KeysFormAConnectedPathReachingEveryScreen(t *testing.T) {
	sd, ok := lookupSurface("git-screen")
	if !ok {
		t.Fatal("git-screen surface not registered")
	}
	if entryScreenID(sd) != "git-form-empty" {
		t.Fatalf("git-screen: entry screen = %q, want git-form-empty", entryScreenID(sd))
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
			t.Fatalf("git-screen: screen %q referenced by a transition but not registered", cur)
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
		t.Fatalf("git-screen: only %d/%d screens reachable from entry %q via ScreenDef.Keys; unreachable: %v", len(visited), len(sd.Screens), entryScreenID(sd), missing)
	}
}
