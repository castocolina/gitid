package dummytui

import (
	"strings"
	"testing"
)

// wantHLTHScreens is the full 5-screen set from
// .planning/design/health/manifest.json, reused by every test below that
// walks the full screen set.
var wantHLTHScreens = []string{
	"health-with-findings", "health-all-green", "finding-detail",
	"per-identity-health", "parse-error",
}

// hlthWriteCeremonyMarkers is the LOW-11 negative-assertion list: none of
// these confirm/backup/apply write-ceremony marker strings (the exact
// phrases global-ssh/global-git/identity-manager's ceremony screens use)
// may EVER appear in health's rendered output — health diagnoses, it
// never mutates, and there is no write-ceremony screen anywhere on this
// surface (§4.6, §5).
var hlthWriteCeremonyMarkers = []string{
	"Confirm write",
	"Nothing has changed yet",
	"Backup created",
	"backup:",
	"Options applied",
	"result-applied",
	"confirm-write",
	"backup-notice",
}

// TestHealth_RegistersFiveScreensAsSoleOwnerOfActivationKeyFour asserts the
// health surface registers exactly 5 screens, claims ActivationKey "4",
// and is the SOLE surface currently claiming "4" — i.e. RegisterOrReplace
// actually replaced the 02-02 data.go placeholder rather than merely
// registering alongside it (review HIGH-2).
func TestHealth_RegistersFiveScreensAsSoleOwnerOfActivationKeyFour(t *testing.T) {
	sd, ok := lookupSurface("health")
	if !ok {
		t.Fatal("health surface not registered — surface_health.go init() did not run?")
	}
	if len(sd.Screens) != 5 {
		t.Fatalf("health: %d screens registered, want 5", len(sd.Screens))
	}
	if sd.ActivationKey != "4" {
		t.Fatalf("health: ActivationKey = %q, want %q", sd.ActivationKey, "4")
	}
	if sd.Title == "" {
		t.Fatal("health: Title is empty")
	}

	var owners []string
	for _, other := range Surfaces() {
		if other.ActivationKey == "4" {
			owners = append(owners, other.ID)
		}
	}
	if len(owners) != 1 || owners[0] != "health" {
		t.Fatalf("health: ActivationKey %q owners = %v, want exactly [health] (RegisterOrReplace must replace the data.go placeholder, not coexist with it)", "4", owners)
	}
}

// TestHealth_EveryScreenRendersNonEmptyWithSignatureAndBreadcrumb walks
// every screen ID in manifest.json's screen set and asserts
// RenderScreen("health", id) is non-empty, contains that screen's
// manifest signature, and contains the "health/<id>" breadcrumb.
func TestHealth_EveryScreenRendersNonEmptyWithSignatureAndBreadcrumb(t *testing.T) {
	if len(wantHLTHScreens) != 5 {
		t.Fatalf("test setup: wantHLTHScreens has %d entries, want 5", len(wantHLTHScreens))
	}

	for _, id := range wantHLTHScreens {
		id := id
		t.Run(id, func(t *testing.T) {
			out, err := RenderScreen("health", id)
			if err != nil {
				t.Fatalf("RenderScreen(health, %s): unexpected error: %v", id, err)
			}
			if out == "" {
				t.Fatalf("RenderScreen(health, %s): empty output", id)
			}
			sig, ok := hlthSignatureByScreen[id]
			if !ok {
				t.Fatalf("test setup: no signature registered for screen %q in hlthSignatureByScreen", id)
			}
			if !strings.Contains(out, sig) {
				t.Fatalf("RenderScreen(health, %s): output missing signature %q:\n%s", id, sig, out)
			}
			breadcrumb := "health/" + id
			if !strings.Contains(out, breadcrumb) {
				t.Fatalf("RenderScreen(health, %s): output missing breadcrumb %q:\n%s", id, breadcrumb, out)
			}
		})
	}
}

// TestHealth_SignaturesAreAllUnique guards against a copy-paste signature
// collision across screens (review HIGH-3c: a signature must be a
// screen-specific marker, never a generic reused string).
func TestHealth_SignaturesAreAllUnique(t *testing.T) {
	seen := map[string]string{}
	for id, sig := range hlthSignatureByScreen {
		if prevID, dup := seen[sig]; dup {
			t.Fatalf("signature %q reused by both screen %q and screen %q", sig, prevID, id)
		}
		seen[sig] = id
	}
	if len(seen) != 5 {
		t.Fatalf("hlthSignatureByScreen has %d unique signatures, want 5", len(seen))
	}
}

// TestHealth_ListAndSummaryScreensHaveBothSSHAndGitSections asserts
// HLTH-01 on the three screens that present a FULL health snapshot
// (health-with-findings, health-all-green, per-identity-health): each
// carries BOTH an "SSH" and a "Git" section heading, never merged, never
// omitted. finding-detail and parse-error are deliberate single-finding
// DEEP-DIVES (the same "detail screen shows one target, not the whole
// list" pattern surface_globalssh.go's option-detail and
// surface_globalgit.go's option-detail already establish) — each still
// NAMES which section its one finding belongs to (asserted below), but
// does not repeat the other, empty section.
func TestHealth_ListAndSummaryScreensHaveBothSSHAndGitSections(t *testing.T) {
	for _, id := range []string{"health-with-findings", "health-all-green", "per-identity-health"} {
		id := id
		t.Run(id, func(t *testing.T) {
			out, err := RenderScreen("health", id)
			if err != nil {
				t.Fatalf("RenderScreen(health, %s): %v", id, err)
			}
			if !strings.Contains(out, "SSH") {
				t.Errorf("RenderScreen(health, %s): missing the SSH section (HLTH-01):\n%s", id, out)
			}
			if !strings.Contains(out, "Git") {
				t.Errorf("RenderScreen(health, %s): missing the Git section (HLTH-01):\n%s", id, out)
			}
		})
	}
}

// TestHealth_DetailScreensNameTheirOwnSection asserts finding-detail and
// parse-error — the two single-finding deep-dive screens — each still
// names the ONE section (SSH or Git) their finding belongs to.
func TestHealth_DetailScreensNameTheirOwnSection(t *testing.T) {
	fd, err := RenderScreen("health", "finding-detail")
	if err != nil {
		t.Fatalf("RenderScreen(health, finding-detail): %v", err)
	}
	if !strings.Contains(fd, "SSH") {
		t.Errorf("finding-detail: expected its own section (SSH) to be named:\n%s", fd)
	}

	pe, err := RenderScreen("health", "parse-error")
	if err != nil {
		t.Fatalf("RenderScreen(health, parse-error): %v", err)
	}
	if !strings.Contains(pe, "Git") {
		t.Errorf("parse-error: expected its own section (Git) to be named:\n%s", pe)
	}
}

// TestHealth_FourSeverityLevelsPresentWithLockedGlyphContract asserts all
// four internal/doctor/doctor.go Severity words (info/warning/error/
// critical) appear as WORDS somewhere across the rendered screen set (the
// NO_COLOR-legibility proof), and that the LOCKED glyph contract holds:
// warning is paired with `!`, error AND critical are paired with `✗`
// (never with each other's glyph), info is paired with `~` — `✗` is NEVER
// paired with the word "warning" anywhere in the combined output.
func TestHealth_FourSeverityLevelsPresentWithLockedGlyphContract(t *testing.T) {
	var all strings.Builder
	for _, id := range wantHLTHScreens {
		out, err := RenderScreen("health", id)
		if err != nil {
			t.Fatalf("RenderScreen(health, %s): %v", id, err)
		}
		all.WriteString(out)
		all.WriteString("\n")
	}
	combined := all.String()

	for _, want := range []string{"info", "warning", "error", "critical"} {
		if !strings.Contains(combined, want) {
			t.Errorf("health: combined screen output missing severity word %q", want)
		}
	}

	if !strings.Contains(combined, "! warning") {
		t.Error(`health: expected "! warning" (warning paired with "!") somewhere in the combined output`)
	}
	if !strings.Contains(combined, "✗ error") {
		t.Error(`health: expected "✗ error" (error paired with "✗") somewhere in the combined output`)
	}
	if !strings.Contains(combined, "✗ critical") {
		t.Error(`health: expected "✗ critical" (critical paired with "✗") somewhere in the combined output`)
	}
	if !strings.Contains(combined, "~ info") {
		t.Error(`health: expected "~ info" (info paired with "~") somewhere in the combined output`)
	}
	if strings.Contains(combined, "✗ warning") {
		t.Error(`health: LOCKED glyph contract violation — "✗ warning" must never appear (warning is always "!", never "✗")`)
	}
}

// TestHealth_ContradictionAndRedundancyExamplesPresent asserts the three
// HLTH-03/HLTH-04 concrete examples the plan pins are all present
// somewhere across the surface: a duplicate "Host *" stanza, an
// "IdentitiesOnly" contradiction, and an "includeIf" targeting a missing
// fragment.
func TestHealth_ContradictionAndRedundancyExamplesPresent(t *testing.T) {
	var all strings.Builder
	for _, id := range wantHLTHScreens {
		out, err := RenderScreen("health", id)
		if err != nil {
			t.Fatalf("RenderScreen(health, %s): %v", id, err)
		}
		all.WriteString(out)
		all.WriteString("\n")
	}
	combined := all.String()

	if !strings.Contains(combined, "Host *") {
		t.Error(`health: missing the "Host *" duplicate-stanza example (HLTH-03)`)
	}
	if !strings.Contains(combined, "IdentitiesOnly") {
		t.Error(`health: missing an "IdentitiesOnly" contradiction example (HLTH-04)`)
	}
	if !strings.Contains(combined, "includeIf") {
		t.Error(`health: missing an "includeIf" targeting-a-missing-fragment example (HLTH-04)`)
	}
}

// TestHealth_ReadOnlyIntegrityBannerOnEveryScreen asserts hlthReadOnlyNote
// (the explicit "Health only diagnoses..." statement) appears on ALL 5
// screens — the read-only-integrity affordance stated positively,
// complementing the negative check below (§4.6, §5).
func TestHealth_ReadOnlyIntegrityBannerOnEveryScreen(t *testing.T) {
	for _, id := range wantHLTHScreens {
		id := id
		t.Run(id, func(t *testing.T) {
			out, err := RenderScreen("health", id)
			if err != nil {
				t.Fatalf("RenderScreen(health, %s): %v", id, err)
			}
			if !strings.Contains(out, hlthReadOnlyNote) {
				t.Fatalf("RenderScreen(health, %s): missing the read-only banner %q:\n%s", id, hlthReadOnlyNote, out)
			}
		})
	}
}

// TestHealth_NoWriteCeremonyMarkerAnywhere is the LOW-11 NEGATIVE
// assertion: none of health's 5 screens may EVER contain a confirm/
// backup/apply write-ceremony marker string — this surface diagnoses, it
// never mutates, and unlike every other primary surface built so far it
// has NO write-ceremony screen at all (§4.6, §5's read-only-integrity
// highest-risk affordance).
func TestHealth_NoWriteCeremonyMarkerAnywhere(t *testing.T) {
	var all strings.Builder
	for _, id := range wantHLTHScreens {
		out, err := RenderScreen("health", id)
		if err != nil {
			t.Fatalf("RenderScreen(health, %s): %v", id, err)
		}
		all.WriteString(out)
		all.WriteString("\n")
	}
	combined := all.String()

	for _, marker := range hlthWriteCeremonyMarkers {
		if strings.Contains(combined, marker) {
			t.Errorf("health: read-only-integrity violation — found write-ceremony marker %q in the combined health screen output (§4.6, §5, review LOW-11)", marker)
		}
	}
}

// TestHealth_KeysFormAConnectedPathReachingEveryScreen walks the
// surface's ScreenDef.Keys graph, starting from the entry screen
// (health-with-findings), and asserts every one of the 5 screens is
// reachable — mirroring the same transitions manifest.json's
// keysFromHome walk (review C3).
func TestHealth_KeysFormAConnectedPathReachingEveryScreen(t *testing.T) {
	sd, ok := lookupSurface("health")
	if !ok {
		t.Fatal("health surface not registered")
	}
	if entryScreenID(sd) != "health-with-findings" {
		t.Fatalf("health: entry screen = %q, want health-with-findings", entryScreenID(sd))
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
			t.Fatalf("health: screen %q referenced by a transition but not registered", cur)
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
		t.Fatalf("health: only %d/%d screens reachable from entry %q via ScreenDef.Keys; unreachable: %v", len(visited), len(sd.Screens), entryScreenID(sd), missing)
	}
}

// TestHealth_IntraSurfaceKeysNeverReuseLaunchKeysNOrG asserts no screen's
// ScreenDef.Keys claims "n" or "g" — create-flow's and git-screen's own
// LaunchKeys (02-UX-DIRECTION.md §2 key-allocation table) — which would
// have already failed loudly at RegisterOrReplace time via registry.go's
// collision guard; this test documents and guards the invariant directly
// against the live registry too.
func TestHealth_IntraSurfaceKeysNeverReuseLaunchKeysNOrG(t *testing.T) {
	sd, ok := lookupSurface("health")
	if !ok {
		t.Fatal("health surface not registered")
	}
	for _, scr := range sd.Screens {
		for k := range scr.Keys {
			if k == "n" || k == "g" {
				t.Fatalf("health: screen %q claims key %q, which collides with create-flow/git-screen's own LaunchKey", scr.ID, k)
			}
		}
	}
}

// TestHealth_PerIdentitySliceTracesTheSameFindingAsTheListView asserts
// HLTH-05's traceability claim directly: per-identity-health's Git
// finding is the SAME hlthFinding value (by id) as
// git-includeif-missing-fragment in the full findings list, not a
// re-derived duplicate.
func TestHealth_PerIdentitySliceTracesTheSameFindingAsTheListView(t *testing.T) {
	if hlthPerIdentityGitFinding.id != "git-includeif-missing-fragment" {
		t.Fatalf("hlthPerIdentityGitFinding.id = %q, want %q", hlthPerIdentityGitFinding.id, "git-includeif-missing-fragment")
	}
	listFinding := hlthFindingByID("git-includeif-missing-fragment")
	if hlthPerIdentityGitFinding != listFinding {
		t.Fatalf("hlthPerIdentityGitFinding diverges from the list-view finding of the same id:\n%+v\nvs\n%+v", hlthPerIdentityGitFinding, listFinding)
	}
}
