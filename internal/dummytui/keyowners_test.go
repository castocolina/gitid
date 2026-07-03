package dummytui

import "testing"

// TestKeyOwners_FinalFiveOwnNumberKeys asserts (review HIGH-2) that
// Surfaces() maps ActivationKey "1".."5" to exactly the five FINAL
// (non-placeholder) real surfaces: identity-manager, global-ssh,
// global-git, health, fixer. Each fan-out plan RegisterOrReplace'd its own
// real surface onto data.go's placeholder ActivationKey (Wave 4), so this
// test also proves no placeholder from data.go survived into the frozen
// reference set, and no view competes for a number key.
func TestKeyOwners_FinalFiveOwnNumberKeys(t *testing.T) {
	want := map[string]string{
		"1": "identity-manager",
		"2": "global-ssh",
		"3": "global-git",
		"4": "health",
		"5": "fixer",
	}

	got := map[string]string{}
	for _, sd := range Surfaces() {
		if sd.ActivationKey == "" {
			continue
		}
		got[sd.ActivationKey] = sd.ID
	}

	for key, wantID := range want {
		gotID, ok := got[key]
		if !ok {
			t.Errorf("key %q: no surface owns this ActivationKey, want %q", key, wantID)
			continue
		}
		if gotID != wantID {
			t.Errorf("key %q: owned by %q, want %q", key, gotID, wantID)
		}
	}

	if len(got) != len(want) {
		t.Errorf("ActivationKey owners: %d distinct number keys claimed, want exactly %d (%v)", len(got), len(want), got)
	}
}

// TestKeyOwners_ModalSurfacesAreKeylessWithLaunchBinding asserts (review C3)
// that create-flow and git-screen are registered as KEYLESS modal surfaces
// (empty ActivationKey — neither competes for a number key) and each
// declares a non-empty LaunchFrom/LaunchKey launch binding, so both remain
// reachable from Identities via the 02-02 launch mechanism (the running
// binary's key routing, not a direct RenderScreen call).
func TestKeyOwners_ModalSurfacesAreKeylessWithLaunchBinding(t *testing.T) {
	want := map[string]string{
		"create-flow": "identity-manager",
		"git-screen":  "identity-manager",
	}

	surfaces := map[string]SurfaceDef{}
	for _, sd := range Surfaces() {
		surfaces[sd.ID] = sd
	}

	for id, wantLaunchFrom := range want {
		sd, ok := surfaces[id]
		if !ok {
			t.Fatalf("surface %q not registered", id)
		}
		if sd.ActivationKey != "" {
			t.Errorf("surface %q: ActivationKey = %q, want empty (keyless modal surface)", id, sd.ActivationKey)
		}
		if sd.LaunchFrom != wantLaunchFrom {
			t.Errorf("surface %q: LaunchFrom = %q, want %q", id, sd.LaunchFrom, wantLaunchFrom)
		}
		if sd.LaunchKey == "" {
			t.Errorf("surface %q: LaunchKey is empty, want a non-empty launch binding", id)
		}
	}
}
