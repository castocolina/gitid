package dummytui

import (
	"strings"
	"testing"
)

// wantGGITScreens is the full 6-screen set from
// .planning/design/global-git/manifest.json, reused by every test below
// that walks the full screen set.
var wantGGITScreens = []string{
	"options-list", "option-detail", "fix-preview",
	"confirm-write", "backup-notice", "result-applied",
}

// TestGlobalGit_RegistersSixScreensAsSoleOwnerOfActivationKeyThree asserts
// the global-git surface registers exactly 6 screens, claims ActivationKey
// "3", and is the SOLE surface currently claiming "3" -- i.e.
// RegisterOrReplace actually replaced the 02-02 data.go placeholder rather
// than merely registering alongside it (review HIGH-2).
func TestGlobalGit_RegistersSixScreensAsSoleOwnerOfActivationKeyThree(t *testing.T) {
	sd, ok := lookupSurface("global-git")
	if !ok {
		t.Fatal("global-git surface not registered — surface_globalgit.go init() did not run?")
	}
	if len(sd.Screens) != 6 {
		t.Fatalf("global-git: %d screens registered, want 6", len(sd.Screens))
	}
	if sd.ActivationKey != "3" {
		t.Fatalf("global-git: ActivationKey = %q, want %q", sd.ActivationKey, "3")
	}
	if sd.Title == "" {
		t.Fatal("global-git: Title is empty")
	}

	var owners []string
	for _, other := range Surfaces() {
		if other.ActivationKey == "3" {
			owners = append(owners, other.ID)
		}
	}
	if len(owners) != 1 || owners[0] != "global-git" {
		t.Fatalf("global-git: ActivationKey %q owners = %v, want exactly [global-git] (RegisterOrReplace must replace the data.go placeholder, not coexist with it)", "3", owners)
	}
}

// TestGlobalGit_EveryScreenRendersNonEmptyWithSignatureAndBreadcrumb walks
// every screen ID in manifest.json's screen set and asserts
// RenderScreen("global-git", id) is non-empty, contains that screen's
// manifest signature, and contains the "global-git/<id>" breadcrumb.
func TestGlobalGit_EveryScreenRendersNonEmptyWithSignatureAndBreadcrumb(t *testing.T) {
	if len(wantGGITScreens) != 6 {
		t.Fatalf("test setup: wantGGITScreens has %d entries, want 6", len(wantGGITScreens))
	}

	for _, id := range wantGGITScreens {
		id := id
		t.Run(id, func(t *testing.T) {
			out, err := RenderScreen("global-git", id)
			if err != nil {
				t.Fatalf("RenderScreen(global-git, %s): unexpected error: %v", id, err)
			}
			if out == "" {
				t.Fatalf("RenderScreen(global-git, %s): empty output", id)
			}
			sig, ok := ggitSignatureByScreen[id]
			if !ok {
				t.Fatalf("test setup: no signature registered for screen %q in ggitSignatureByScreen", id)
			}
			if !strings.Contains(out, sig) {
				t.Fatalf("RenderScreen(global-git, %s): output missing signature %q:\n%s", id, sig, out)
			}
			breadcrumb := "global-git/" + id
			if !strings.Contains(out, breadcrumb) {
				t.Fatalf("RenderScreen(global-git, %s): output missing breadcrumb %q:\n%s", id, breadcrumb, out)
			}
		})
	}
}

// TestGlobalGit_SignaturesAreAllUnique guards against a copy-paste
// signature collision across screens (review HIGH-3c: a signature must be a
// screen-specific marker, never a generic reused string).
func TestGlobalGit_SignaturesAreAllUnique(t *testing.T) {
	seen := map[string]string{}
	for id, sig := range ggitSignatureByScreen {
		if prevID, dup := seen[sig]; dup {
			t.Fatalf("signature %q reused by both screen %q and screen %q", sig, prevID, id)
		}
		seen[sig] = id
	}
	if len(seen) != 6 {
		t.Fatalf("ggitSignatureByScreen has %d unique signatures, want 6", len(seen))
	}
}

// TestGlobalGit_BaselineOptionSetAnchors asserts all 11 GGIT-01 baseline
// option keys appear, as WORDS, somewhere across the rendered screen set --
// the NO_COLOR-legibility proof and the pinned option-set contract.
func TestGlobalGit_BaselineOptionSetAnchors(t *testing.T) {
	var all strings.Builder
	for _, id := range wantGGITScreens {
		out, err := RenderScreen("global-git", id)
		if err != nil {
			t.Fatalf("RenderScreen(global-git, %s): %v", id, err)
		}
		all.WriteString(out)
		all.WriteString("\n")
	}
	combined := all.String()

	for _, want := range []string{
		"init.defaultBranch", "core.ignorecase", "autocrlf", "user.email",
		"push.autoSetupRemote", "pull.rebase", "fetch.prune",
		"alias", "color", "merge.conflictstyle", "diff.colorMoved",
	} {
		if !strings.Contains(combined, want) {
			t.Errorf("global-git: combined screen output missing GGIT-01 option %q", want)
		}
	}
}

// TestGlobalGit_MainVsMasterHighlighted asserts the init.defaultBranch
// option carries its dedicated main-vs-master highlight (GGIT-01) on both
// options-list and option-detail, and that "master" (the option being
// moved away from) is named, not just "main".
func TestGlobalGit_MainVsMasterHighlighted(t *testing.T) {
	for _, id := range []string{"options-list", "option-detail"} {
		out, err := RenderScreen("global-git", id)
		if err != nil {
			t.Fatalf("RenderScreen(global-git, %s): %v", id, err)
		}
		if !strings.Contains(out, "main vs master") {
			t.Errorf("RenderScreen(global-git, %s): missing the main-vs-master highlight", id)
		}
		if !strings.Contains(out, "master") {
			t.Errorf("RenderScreen(global-git, %s): missing \"master\" (the branch name being moved away from)", id)
		}
		if !strings.Contains(out, "main") {
			t.Errorf("RenderScreen(global-git, %s): missing \"main\" (the recommended branch name)", id)
		}
	}
}

// TestGlobalGit_ManagedBlockContainmentShown asserts confirm-write renders
// the sentinel-visible managed block (GGIT-01's own highest-risk
// affordance: writes must preserve content outside the managed block
// verbatim), and that the "preserved verbatim" note is stated.
func TestGlobalGit_ManagedBlockContainmentShown(t *testing.T) {
	out, err := RenderScreen("global-git", "confirm-write")
	if err != nil {
		t.Fatalf("RenderScreen(global-git, confirm-write): %v", err)
	}
	for _, want := range []string{
		"# BEGIN gitid managed: global-git",
		"# END gitid managed: global-git",
		ggitTargetFile,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("RenderScreen(global-git, confirm-write): missing %q", want)
		}
	}

	fixOut, err := RenderScreen("global-git", "fix-preview")
	if err != nil {
		t.Fatalf("RenderScreen(global-git, fix-preview): %v", err)
	}
	if !strings.Contains(fixOut, "preserved verbatim") {
		t.Error("RenderScreen(global-git, fix-preview): missing the \"preserved verbatim\" managed-block-containment note")
	}

	resultOut, err := RenderScreen("global-git", "result-applied")
	if err != nil {
		t.Fatalf("RenderScreen(global-git, result-applied): %v", err)
	}
	if !strings.Contains(resultOut, "preserved verbatim") {
		t.Error("RenderScreen(global-git, result-applied): missing the \"preserved verbatim\" managed-block-containment note")
	}
}

// TestGlobalGit_AdvisoryNeverBlocking is the machine-checkable proof of
// §4.5/§5's highest-risk affordance shared with global-ssh: recommendations
// render as an advisory (word present), and the surface never uses
// blocking/compliance-gate language anywhere in its rendered output.
func TestGlobalGit_AdvisoryNeverBlocking(t *testing.T) {
	var all strings.Builder
	for _, id := range wantGGITScreens {
		out, err := RenderScreen("global-git", id)
		if err != nil {
			t.Fatalf("RenderScreen(global-git, %s): %v", id, err)
		}
		all.WriteString(out)
		all.WriteString("\n")
	}
	combined := strings.ToLower(all.String())

	if !strings.Contains(combined, "advisory") {
		t.Error("global-git: combined screen output missing the word \"advisory\"")
	}
	if !strings.Contains(combined, "recommended") {
		t.Error("global-git: combined screen output missing the word \"recommended\"")
	}
	for _, blocked := range []string{"blocked", "must fix", "required to proceed"} {
		if strings.Contains(combined, blocked) {
			t.Errorf("global-git: combined screen output contains blocking-language %q — recommendations must be advisory, never blocking (§4.5)", blocked)
		}
	}
	// The informational user.email row must stay visible through to the end
	// of the ceremony (fix-preview, confirm-write, result-applied) --
	// concrete proof that gitid never silently drops the explanation for
	// why it is left alone.
	for _, id := range []string{"fix-preview", "confirm-write", "result-applied"} {
		out, err := RenderScreen("global-git", id)
		if err != nil {
			t.Fatalf("RenderScreen(global-git, %s): %v", id, err)
		}
		if !strings.Contains(out, "user.email") {
			t.Errorf("RenderScreen(global-git, %s): missing the user.email note — the structural rule must stay visible through the whole ceremony", id)
		}
	}
}

// TestGlobalGit_KeysFormAConnectedPathReachingEveryScreen walks the
// surface's ScreenDef.Keys graph, starting from the entry screen
// (options-list), and asserts every one of the 6 screens is reachable --
// mirroring the same transitions manifest.json's keysFromHome walk (review
// C3).
func TestGlobalGit_KeysFormAConnectedPathReachingEveryScreen(t *testing.T) {
	sd, ok := lookupSurface("global-git")
	if !ok {
		t.Fatal("global-git surface not registered")
	}
	if entryScreenID(sd) != "options-list" {
		t.Fatalf("global-git: entry screen = %q, want options-list", entryScreenID(sd))
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
			t.Fatalf("global-git: screen %q referenced by a transition but not registered", cur)
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
		t.Fatalf("global-git: only %d/%d screens reachable from entry %q via ScreenDef.Keys; unreachable: %v", len(visited), len(sd.Screens), entryScreenID(sd), missing)
	}
}

// TestGlobalGit_IntraSurfaceKeysNeverReuseLaunchKeysNOrG asserts no screen's
// ScreenDef.Keys claims "n" or "g" -- create-flow's and git-screen's own
// LaunchKeys (02-UX-DIRECTION.md §2 key-allocation table) -- which would
// have already failed loudly at RegisterOrReplace time via registry.go's
// collision guard; this test documents and guards the invariant directly
// against the live registry too.
func TestGlobalGit_IntraSurfaceKeysNeverReuseLaunchKeysNOrG(t *testing.T) {
	sd, ok := lookupSurface("global-git")
	if !ok {
		t.Fatal("global-git surface not registered")
	}
	for _, scr := range sd.Screens {
		for k := range scr.Keys {
			if k == "n" || k == "g" {
				t.Fatalf("global-git: screen %q claims key %q, which collides with create-flow/git-screen's own LaunchKey", scr.ID, k)
			}
		}
	}
}
