package dummytui

import (
	"strings"
	"testing"
)

// wantGSSHScreens is the full 6-screen set from
// .planning/design/global-ssh/manifest.json, reused by every test below
// that walks the full screen set.
var wantGSSHScreens = []string{
	"options-list", "option-detail", "fix-preview",
	"confirm-write", "backup-notice", "result-applied",
}

// TestGlobalSSH_RegistersSixScreensAsSoleOwnerOfActivationKeyTwo asserts
// the global-ssh surface registers exactly 6 screens, claims ActivationKey
// "2", and is the SOLE surface currently claiming "2" — i.e.
// RegisterOrReplace actually replaced the 02-02 data.go placeholder rather
// than merely registering alongside it (review HIGH-2).
func TestGlobalSSH_RegistersSixScreensAsSoleOwnerOfActivationKeyTwo(t *testing.T) {
	sd, ok := lookupSurface("global-ssh")
	if !ok {
		t.Fatal("global-ssh surface not registered — surface_globalssh.go init() did not run?")
	}
	if len(sd.Screens) != 6 {
		t.Fatalf("global-ssh: %d screens registered, want 6", len(sd.Screens))
	}
	if sd.ActivationKey != "2" {
		t.Fatalf("global-ssh: ActivationKey = %q, want %q", sd.ActivationKey, "2")
	}
	if sd.Title == "" {
		t.Fatal("global-ssh: Title is empty")
	}

	var owners []string
	for _, other := range Surfaces() {
		if other.ActivationKey == "2" {
			owners = append(owners, other.ID)
		}
	}
	if len(owners) != 1 || owners[0] != "global-ssh" {
		t.Fatalf("global-ssh: ActivationKey %q owners = %v, want exactly [global-ssh] (RegisterOrReplace must replace the data.go placeholder, not coexist with it)", "2", owners)
	}
}

// TestGlobalSSH_EveryScreenRendersNonEmptyWithSignatureAndBreadcrumb walks
// every screen ID in manifest.json's screen set and asserts
// RenderScreen("global-ssh", id) is non-empty, contains that screen's
// manifest signature, and contains the "global-ssh/<id>" breadcrumb.
func TestGlobalSSH_EveryScreenRendersNonEmptyWithSignatureAndBreadcrumb(t *testing.T) {
	if len(wantGSSHScreens) != 6 {
		t.Fatalf("test setup: wantGSSHScreens has %d entries, want 6", len(wantGSSHScreens))
	}

	for _, id := range wantGSSHScreens {
		id := id
		t.Run(id, func(t *testing.T) {
			out, err := RenderScreen("global-ssh", id)
			if err != nil {
				t.Fatalf("RenderScreen(global-ssh, %s): unexpected error: %v", id, err)
			}
			if out == "" {
				t.Fatalf("RenderScreen(global-ssh, %s): empty output", id)
			}
			sig, ok := gsshSignatureByScreen[id]
			if !ok {
				t.Fatalf("test setup: no signature registered for screen %q in gsshSignatureByScreen", id)
			}
			if !strings.Contains(out, sig) {
				t.Fatalf("RenderScreen(global-ssh, %s): output missing signature %q:\n%s", id, sig, out)
			}
			breadcrumb := "global-ssh/" + id
			if !strings.Contains(out, breadcrumb) {
				t.Fatalf("RenderScreen(global-ssh, %s): output missing breadcrumb %q:\n%s", id, breadcrumb, out)
			}
		})
	}
}

// TestGlobalSSH_SignaturesAreAllUnique guards against a copy-paste
// signature collision across screens (review HIGH-3c: a signature must be a
// screen-specific marker, never a generic reused string).
func TestGlobalSSH_SignaturesAreAllUnique(t *testing.T) {
	seen := map[string]string{}
	for id, sig := range gsshSignatureByScreen {
		if prevID, dup := seen[sig]; dup {
			t.Fatalf("signature %q reused by both screen %q and screen %q", sig, prevID, id)
		}
		seen[sig] = id
	}
	if len(seen) != 6 {
		t.Fatalf("gsshSignatureByScreen has %d unique signatures, want 6", len(seen))
	}
}

// TestGlobalSSH_DangerousOptionSetAnchors asserts all 6 GSSH-01
// dangerous-by-default option names appear, as WORDS, somewhere across the
// rendered screen set — the NO_COLOR-legibility proof and the pinned
// option-set contract.
func TestGlobalSSH_DangerousOptionSetAnchors(t *testing.T) {
	var all strings.Builder
	for _, id := range wantGSSHScreens {
		out, err := RenderScreen("global-ssh", id)
		if err != nil {
			t.Fatalf("RenderScreen(global-ssh, %s): %v", id, err)
		}
		all.WriteString(out)
		all.WriteString("\n")
	}
	combined := all.String()

	for _, want := range []string{
		"StrictHostKeyChecking", "ForwardAgent", "HashKnownHosts",
		"IdentitiesOnly", "AddKeysToAgent", "UseKeychain",
	} {
		if !strings.Contains(combined, want) {
			t.Errorf("global-ssh: combined screen output missing GSSH-01 option %q", want)
		}
	}
}

// TestGlobalSSH_AdvisoryNeverBlocking is the machine-checkable proof of
// §4.4/§5's highest-risk affordance: recommendations render as an advisory
// (word present), and the surface never uses blocking/compliance-gate
// language anywhere in its rendered output.
func TestGlobalSSH_AdvisoryNeverBlocking(t *testing.T) {
	var all strings.Builder
	for _, id := range wantGSSHScreens {
		out, err := RenderScreen("global-ssh", id)
		if err != nil {
			t.Fatalf("RenderScreen(global-ssh, %s): %v", id, err)
		}
		all.WriteString(out)
		all.WriteString("\n")
	}
	combined := strings.ToLower(all.String())

	if !strings.Contains(combined, "advisory") {
		t.Error("global-ssh: combined screen output missing the word \"advisory\"")
	}
	if !strings.Contains(combined, "recommended") {
		t.Error("global-ssh: combined screen output missing the word \"recommended\"")
	}
	for _, blocked := range []string{"blocked", "must fix", "required to proceed"} {
		if strings.Contains(combined, blocked) {
			t.Errorf("global-ssh: combined screen output contains blocking-language %q — recommendations must be advisory, never blocking (§4.4)", blocked)
		}
	}
	// The declined ForwardAgent recommendation must stay visible through to
	// the end of the ceremony (fix-preview, confirm-write, result-applied)
	// — concrete proof the user's choice to leave an option unchanged is
	// respected and shown, not silently dropped.
	for _, id := range []string{"fix-preview", "confirm-write", "result-applied"} {
		out, err := RenderScreen("global-ssh", id)
		if err != nil {
			t.Fatalf("RenderScreen(global-ssh, %s): %v", id, err)
		}
		if !strings.Contains(out, "ForwardAgent") {
			t.Errorf("RenderScreen(global-ssh, %s): missing the declined ForwardAgent option — the advisory choice must stay visible through the whole ceremony", id)
		}
	}
}

// TestGlobalSSH_KeysFormAConnectedPathReachingEveryScreen walks the
// surface's ScreenDef.Keys graph, starting from the entry screen
// (options-list), and asserts every one of the 6 screens is reachable —
// mirroring the same transitions manifest.json's keysFromHome walk (review
// C3).
func TestGlobalSSH_KeysFormAConnectedPathReachingEveryScreen(t *testing.T) {
	sd, ok := lookupSurface("global-ssh")
	if !ok {
		t.Fatal("global-ssh surface not registered")
	}
	if entryScreenID(sd) != "options-list" {
		t.Fatalf("global-ssh: entry screen = %q, want options-list", entryScreenID(sd))
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
			t.Fatalf("global-ssh: screen %q referenced by a transition but not registered", cur)
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
		t.Fatalf("global-ssh: only %d/%d screens reachable from entry %q via ScreenDef.Keys; unreachable: %v", len(visited), len(sd.Screens), entryScreenID(sd), missing)
	}
}

// TestGlobalSSH_IntraSurfaceKeysNeverReuseLaunchKeysNOrG asserts no screen's
// ScreenDef.Keys claims "n" or "g" — create-flow's and git-screen's own
// LaunchKeys (02-UX-DIRECTION.md §2 key-allocation table) — which would
// have already failed loudly at RegisterOrReplace time via registry.go's
// collision guard; this test documents and guards the invariant directly
// against the live registry too.
func TestGlobalSSH_IntraSurfaceKeysNeverReuseLaunchKeysNOrG(t *testing.T) {
	sd, ok := lookupSurface("global-ssh")
	if !ok {
		t.Fatal("global-ssh surface not registered")
	}
	for _, scr := range sd.Screens {
		for k := range scr.Keys {
			if k == "n" || k == "g" {
				t.Fatalf("global-ssh: screen %q claims key %q, which collides with create-flow/git-screen's own LaunchKey", scr.ID, k)
			}
		}
	}
}
