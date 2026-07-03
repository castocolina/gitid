package dummytui

import (
	"strings"
	"testing"
)

// TestCreateFlow_RegistersTwelveScreensKeylessWithLaunchBinding asserts the
// create-flow surface registers exactly 12 screens, is keyless (empty
// ActivationKey), and declares a LaunchFrom=="identity-manager" +
// non-empty LaunchKey binding (review C3, T-02-ML3).
func TestCreateFlow_RegistersTwelveScreensKeylessWithLaunchBinding(t *testing.T) {
	sd, ok := lookupSurface("create-flow")
	if !ok {
		t.Fatal("create-flow surface not registered — surface_createflow.go init() did not run?")
	}
	if len(sd.Screens) != 12 {
		t.Fatalf("create-flow: %d screens registered, want 12", len(sd.Screens))
	}
	if sd.ActivationKey != "" {
		t.Fatalf("create-flow: ActivationKey = %q, want empty (keyless modal surface)", sd.ActivationKey)
	}
	if sd.LaunchFrom != "identity-manager" {
		t.Fatalf("create-flow: LaunchFrom = %q, want identity-manager", sd.LaunchFrom)
	}
	if sd.LaunchKey == "" {
		t.Fatal("create-flow: LaunchKey is empty, want a non-empty launch key")
	}
}

// TestCreateFlow_EveryScreenRendersNonEmptyWithSignatureAndBreadcrumb walks
// every screen ID in manifest.json's screen set and asserts
// RenderScreen("create-flow", id) is non-empty, contains that screen's
// manifest signature, and contains the "create-flow/<id>" breadcrumb.
func TestCreateFlow_EveryScreenRendersNonEmptyWithSignatureAndBreadcrumb(t *testing.T) {
	wantScreens := []string{
		"algo-catalog", "ssh-form-empty", "ssh-form-filled", "ssh-form-blank-prefix",
		"reuse-key-vs-generate", "macos-globals-block", "test-stage1-direct",
		"test-stage2-by-alias", "test-fail", "confirm-write", "backup-notice",
		"result-success",
	}
	if len(wantScreens) != 12 {
		t.Fatalf("test setup: wantScreens has %d entries, want 12", len(wantScreens))
	}

	for _, id := range wantScreens {
		id := id
		t.Run(id, func(t *testing.T) {
			out, err := RenderScreen("create-flow", id)
			if err != nil {
				t.Fatalf("RenderScreen(create-flow, %s): unexpected error: %v", id, err)
			}
			if out == "" {
				t.Fatalf("RenderScreen(create-flow, %s): empty output", id)
			}
			sig, ok := cfSignatureByScreen[id]
			if !ok {
				t.Fatalf("test setup: no signature registered for screen %q in cfSignatureByScreen", id)
			}
			if !strings.Contains(out, sig) {
				t.Fatalf("RenderScreen(create-flow, %s): output missing signature %q:\n%s", id, sig, out)
			}
			breadcrumb := "create-flow/" + id
			if !strings.Contains(out, breadcrumb) {
				t.Fatalf("RenderScreen(create-flow, %s): output missing breadcrumb %q:\n%s", id, breadcrumb, out)
			}
		})
	}
}

// TestCreateFlow_SignaturesAreAllUnique guards against a copy-paste
// signature collision across screens (review HIGH-3c: a signature must be a
// screen-specific marker, never a generic reused string).
func TestCreateFlow_SignaturesAreAllUnique(t *testing.T) {
	seen := map[string]string{}
	for id, sig := range cfSignatureByScreen {
		if prevID, dup := seen[sig]; dup {
			t.Fatalf("signature %q reused by both screen %q and screen %q", sig, prevID, id)
		}
		seen[sig] = id
	}
	if len(seen) != 12 {
		t.Fatalf("cfSignatureByScreen has %d unique signatures, want 12", len(seen))
	}
}

// TestCreateFlow_CopyParityAnchors asserts the recipe-critical literal
// values every screen must mirror byte-for-byte against the /mui mockup and
// recipeFixtures.ts are actually present in the rendered output somewhere
// in the flow (Port 443, IdentitiesOnly yes, the ssh -G IdentityFile proof).
func TestCreateFlow_CopyParityAnchors(t *testing.T) {
	var all strings.Builder
	for _, id := range []string{
		"algo-catalog", "ssh-form-empty", "ssh-form-filled", "ssh-form-blank-prefix",
		"reuse-key-vs-generate", "macos-globals-block", "test-stage1-direct",
		"test-stage2-by-alias", "test-fail", "confirm-write", "backup-notice",
		"result-success",
	} {
		out, err := RenderScreen("create-flow", id)
		if err != nil {
			t.Fatalf("RenderScreen(create-flow, %s): %v", id, err)
		}
		all.WriteString(out)
		all.WriteString("\n")
	}
	combined := all.String()

	for _, want := range []string{"IdentitiesOnly yes", "Port 443", "ssh -G"} {
		if !strings.Contains(combined, want) {
			t.Errorf("create-flow: combined screen output missing recipe-accurate anchor %q", want)
		}
	}
}

// TestCreateFlow_KeysFormAConnectedPathReachingEveryScreen walks the
// surface's ScreenDef.Keys graph, starting from the entry screen
// (algo-catalog), and asserts every one of the 12 screens is reachable —
// mirroring the same transitions manifest.json's keysFromHome walk (review
// C3).
func TestCreateFlow_KeysFormAConnectedPathReachingEveryScreen(t *testing.T) {
	sd, ok := lookupSurface("create-flow")
	if !ok {
		t.Fatal("create-flow surface not registered")
	}
	if entryScreenID(sd) != "algo-catalog" {
		t.Fatalf("create-flow: entry screen = %q, want algo-catalog", entryScreenID(sd))
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
			t.Fatalf("create-flow: screen %q referenced by a transition but not registered", cur)
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
		t.Fatalf("create-flow: only %d/%d screens reachable from entry %q via ScreenDef.Keys; unreachable: %v", len(visited), len(sd.Screens), entryScreenID(sd), missing)
	}
}
