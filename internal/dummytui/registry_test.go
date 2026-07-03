package dummytui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// snapshotRegistry saves the current package-level registry state and
// registers a t.Cleanup that restores it verbatim after the test finishes.
// EVERY test in this package that calls Register/RegisterOrReplace with a
// test-scoped surface ID MUST call this first (review MED — Codex proof:
// `go test -shuffle=on -count=10 ./internal/dummytui` previously failed,
// because the package-level `registry` map is never reset between test
// iterations, so a surface registered by one run of a test collides with
// the SAME test's own registration on the next run under -count=N/-shuffle).
func snapshotRegistry(t *testing.T) {
	t.Helper()
	orig := make(map[string]SurfaceDef, len(registry))
	for k, v := range registry {
		orig[k] = v
	}
	t.Cleanup(func() {
		registry = orig
	})
}

// key builds a tea.KeyMsg whose String() returns the given text, mirroring
// how bubbletea decodes a single printable rune or a named key ("esc").
func key(s string) tea.KeyMsg {
	switch s {
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	default:
		return tea.KeyPressMsg{Code: rune(s[0])}
	}
}

func TestRoute_NumberKeysReachFinalSurfaces(t *testing.T) {
	cases := []struct {
		k    string
		want string
	}{
		{"1", "identity-manager"},
		{"2", "global-ssh"},
		{"3", "global-git"},
		{"4", "health"},
		{"5", "fixer"},
	}
	for _, tc := range cases {
		st := navState{view: "identity-manager", activeScreen: entryScreenID(mustSurface(t, "identity-manager"))}
		got := route(st, key(tc.k))
		if got.view != tc.want {
			t.Errorf("route on key %q: view = %q, want %q", tc.k, got.view, tc.want)
		}
	}
}

func TestRoute_UnknownKeyNoop(t *testing.T) {
	st := navState{view: "identity-manager", activeScreen: entryScreenID(mustSurface(t, "identity-manager"))}
	got := route(st, key("9"))
	if got.view != st.view || got.activeScreen != st.activeScreen {
		t.Errorf("route on unknown key: state changed, got %+v want unchanged %+v", got, st)
	}
}

func TestRoute_IntraSurfaceTransition(t *testing.T) {
	snapshotRegistry(t)
	const src = "test-route-intra-src"
	Register(SurfaceDef{
		ID:            src,
		ActivationKey: "",
		Screens: []ScreenDef{
			{ID: "entry", Keys: map[string]string{"x": "next"}, Render: func() string { return "entry" }},
			{ID: "next", Render: func() string { return "next" }},
		},
	})

	st := navState{view: src, activeScreen: "entry"}
	got := route(st, key("x"))
	if got.activeScreen != "next" {
		t.Fatalf("intra-surface transition: activeScreen = %q, want %q", got.activeScreen, "next")
	}
}

func TestRoute_EscReturnsToEntry(t *testing.T) {
	snapshotRegistry(t)
	const src = "test-route-esc-src"
	Register(SurfaceDef{
		ID: src,
		Screens: []ScreenDef{
			{ID: "entry", Keys: map[string]string{"x": "next"}, Render: func() string { return "entry" }},
			{ID: "next", Render: func() string { return "next" }},
		},
	})

	st := navState{view: src, activeScreen: "next"}
	got := route(st, key("esc"))
	if got.activeScreen != "entry" {
		t.Fatalf("esc on empty modalStack: activeScreen = %q, want %q", got.activeScreen, "entry")
	}
}

func TestRegister_DuplicateActivationKeyRejected(t *testing.T) {
	snapshotRegistry(t)
	defer func() {
		if recover() == nil {
			t.Fatal("Register: expected panic on duplicate non-empty ActivationKey, got none")
		}
	}()
	Register(SurfaceDef{ID: "test-dup-a", ActivationKey: "test-dup-key", Screens: []ScreenDef{{ID: "e", Render: func() string { return "" }}}})
	Register(SurfaceDef{ID: "test-dup-b", ActivationKey: "test-dup-key", Screens: []ScreenDef{{ID: "e", Render: func() string { return "" }}}})
}

func TestRegister_EmptyActivationKeyExemption(t *testing.T) {
	snapshotRegistry(t)
	// Two keyless (empty ActivationKey) surfaces registered together must both
	// succeed — no duplicate-key rejection for empty keys (review H2).
	Register(SurfaceDef{ID: "test-keyless-a", Screens: []ScreenDef{{ID: "e", Render: func() string { return "" }}}})
	Register(SurfaceDef{ID: "test-keyless-b", Screens: []ScreenDef{{ID: "e", Render: func() string { return "" }}}})

	if _, ok := lookupSurface("test-keyless-a"); !ok {
		t.Fatal("test-keyless-a not registered")
	}
	if _, ok := lookupSurface("test-keyless-b"); !ok {
		t.Fatal("test-keyless-b not registered")
	}
}

func TestRegisterOrReplace_SingleOwner(t *testing.T) {
	snapshotRegistry(t)
	const testKey = "test-replace-key"
	Register(SurfaceDef{ID: "test-replace-placeholder", ActivationKey: testKey, Screens: []ScreenDef{{ID: "e", Render: func() string { return "placeholder" }}}})
	RegisterOrReplace(SurfaceDef{ID: "test-replace-real", ActivationKey: testKey, Screens: []ScreenDef{{ID: "e", Render: func() string { return "real" }}}})

	if _, ok := lookupSurface("test-replace-placeholder"); ok {
		t.Fatal("RegisterOrReplace: stale placeholder still present in Surfaces()")
	}
	owner, ok := lookupSurface("test-replace-real")
	if !ok {
		t.Fatal("RegisterOrReplace: replacement surface not registered")
	}
	if owner.ID != "test-replace-real" {
		t.Fatalf("RegisterOrReplace: owner ID = %q, want test-replace-real", owner.ID)
	}

	count := 0
	for _, sd := range Surfaces() {
		if sd.ActivationKey == testKey {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("RegisterOrReplace: %d surfaces own activation key %q, want exactly 1", count, testKey)
	}
}

func TestModalLaunch_PushPopAndNoNumberKeyReaches(t *testing.T) {
	snapshotRegistry(t)
	const (
		source  = "test-modal-source"
		keyless = "test-modal-keyless"
	)
	Register(SurfaceDef{
		ID:            source,
		ActivationKey: "",
		Screens:       []ScreenDef{{ID: "entry", Render: func() string { return "source entry" }}},
	})
	Register(SurfaceDef{
		ID:         keyless,
		LaunchFrom: source,
		LaunchKey:  "z",
		Screens:    []ScreenDef{{ID: "modal-entry", Render: func() string { return "modal entry" }}},
	})

	st := navState{view: source, activeScreen: "entry"}
	launched := route(st, key("z"))
	if len(launched.modalStack) != 1 {
		t.Fatalf("modal launch: modalStack len = %d, want 1", len(launched.modalStack))
	}
	top := launched.modalStack[0]
	if top.Surface != keyless || top.Screen != "modal-entry" {
		t.Fatalf("modal launch: top frame = %+v, want {%s modal-entry}", top, keyless)
	}

	// No number key ever reaches the keyless surface directly.
	unrelated := navState{view: source, activeScreen: "entry"}
	afterNum := route(unrelated, key("1"))
	if afterNum.view == keyless {
		t.Fatal("modal launch: a number key must never switch the top-level view to a keyless surface")
	}

	popped := route(launched, key("esc"))
	if len(popped.modalStack) != 0 {
		t.Fatalf("esc pop: modalStack len = %d, want 0", len(popped.modalStack))
	}
	if popped.view != source || popped.activeScreen != "entry" {
		t.Fatalf("esc pop: state = %+v, want parent {%s entry}", popped, source)
	}
}

func TestModalLaunch_NumberKeysIgnoredWhileModalActive(t *testing.T) {
	snapshotRegistry(t)
	const source = "test-modal-numnoop-source"
	const keyless = "test-modal-numnoop-keyless"
	Register(SurfaceDef{ID: source, Screens: []ScreenDef{{ID: "entry", Render: func() string { return "" }}}})
	Register(SurfaceDef{ID: keyless, LaunchFrom: source, LaunchKey: "y", Screens: []ScreenDef{{ID: "modal-entry", Render: func() string { return "" }}}})

	st := navState{view: source, activeScreen: "entry", modalStack: []modalFrame{{Surface: keyless, Screen: "modal-entry"}}}
	got := route(st, key("2"))
	if got.view != source {
		t.Fatalf("number key while modal active: top-level view changed to %q, want unchanged %q", got.view, source)
	}
	if len(got.modalStack) != 1 {
		t.Fatalf("number key while modal active: modalStack mutated, len = %d", len(got.modalStack))
	}
}

func TestLaunchKeyCollisionGuard(t *testing.T) {
	t.Run("keyless LaunchKey colliding with LaunchFrom ScreenDef.Keys is rejected", func(t *testing.T) {
		snapshotRegistry(t)
		const source = "test-collision-src-1"
		Register(SurfaceDef{
			ID: source,
			Screens: []ScreenDef{
				{ID: "entry", Keys: map[string]string{"w": "next"}, Render: func() string { return "" }},
				{ID: "next", Render: func() string { return "" }},
			},
		})

		defer func() {
			if recover() == nil {
				t.Fatal("Register: expected panic on LaunchKey colliding with LaunchFrom ScreenDef.Keys")
			}
		}()
		Register(SurfaceDef{ID: "test-collision-keyless-1", LaunchFrom: source, LaunchKey: "w", Screens: []ScreenDef{{ID: "e", Render: func() string { return "" }}}})
	})

	t.Run("RegisterOrReplace introducing a colliding ScreenDef.Keys is rejected", func(t *testing.T) {
		snapshotRegistry(t)
		const source = "test-collision-src-2"
		Register(SurfaceDef{ID: source, Screens: []ScreenDef{{ID: "entry", Render: func() string { return "" }}}})
		Register(SurfaceDef{ID: "test-collision-keyless-2", LaunchFrom: source, LaunchKey: "v", Screens: []ScreenDef{{ID: "e", Render: func() string { return "" }}}})

		defer func() {
			if recover() == nil {
				t.Fatal("RegisterOrReplace: expected panic on ScreenDef.Keys colliding with an existing keyless LaunchKey")
			}
		}()
		RegisterOrReplace(SurfaceDef{
			ID: source,
			Screens: []ScreenDef{
				{ID: "entry", Keys: map[string]string{"v": "next"}, Render: func() string { return "" }},
				{ID: "next", Render: func() string { return "" }},
			},
		})
	})

	t.Run("non-colliding LaunchKey registers cleanly", func(t *testing.T) {
		snapshotRegistry(t)
		const source = "test-collision-src-3"
		Register(SurfaceDef{ID: source, Screens: []ScreenDef{{ID: "entry", Keys: map[string]string{"w": "next"}, Render: func() string { return "" }}, {ID: "next", Render: func() string { return "" }}}})
		Register(SurfaceDef{ID: "test-collision-keyless-3", LaunchFrom: source, LaunchKey: "u", Screens: []ScreenDef{{ID: "e", Render: func() string { return "" }}}})

		if _, ok := lookupSurface("test-collision-keyless-3"); !ok {
			t.Fatal("non-colliding LaunchKey: surface not registered")
		}
	})

	t.Run("LaunchKey equal to a number key is rejected", func(t *testing.T) {
		snapshotRegistry(t)
		const source = "test-collision-src-4"
		Register(SurfaceDef{ID: source, Screens: []ScreenDef{{ID: "entry", Render: func() string { return "" }}}})
		defer func() {
			if recover() == nil {
				t.Fatal("Register: expected panic on LaunchKey equal to a number key")
			}
		}()
		Register(SurfaceDef{ID: "test-collision-keyless-4", LaunchFrom: source, LaunchKey: "3", Screens: []ScreenDef{{ID: "e", Render: func() string { return "" }}}})
	})

	t.Run("LaunchKey equal to a reserved key is rejected", func(t *testing.T) {
		snapshotRegistry(t)
		const source = "test-collision-src-5"
		Register(SurfaceDef{ID: source, Screens: []ScreenDef{{ID: "entry", Render: func() string { return "" }}}})
		defer func() {
			if recover() == nil {
				t.Fatal("Register: expected panic on LaunchKey equal to a reserved key")
			}
		}()
		Register(SurfaceDef{ID: "test-collision-keyless-5", LaunchFrom: source, LaunchKey: "esc", Screens: []ScreenDef{{ID: "e", Render: func() string { return "" }}}})
	})
}

func TestRoutePrecedence_ScreenKeysBeforeLaunchKey(t *testing.T) {
	snapshotRegistry(t)
	const source = "test-precedence-src"
	Register(SurfaceDef{
		ID: source,
		Screens: []ScreenDef{
			{ID: "entry", Keys: map[string]string{"t": "intra-target"}, Render: func() string { return "" }},
			{ID: "intra-target", Render: func() string { return "" }},
		},
	})
	// A launch key "t" targeting a DIFFERENT keyless surface would collide at
	// registration (guarded above), so precedence is proven directly: routing
	// "t" from the entry screen (which claims "t" via ScreenDef.Keys) must
	// resolve the intra-surface transition, not fall through to a view switch.
	st := navState{view: source, activeScreen: "entry"}
	got := route(st, key("t"))
	if got.activeScreen != "intra-target" {
		t.Fatalf("precedence: activeScreen = %q, want intra-target (ScreenDef.Keys must win)", got.activeScreen)
	}
	if got.view != source {
		t.Fatalf("precedence: view changed to %q, want unchanged %q", got.view, source)
	}
}

func TestRenderScreen_DeterministicAndBreadcrumb(t *testing.T) {
	first, err := RenderScreen("identity-manager", entryScreenID(mustSurface(t, "identity-manager")))
	if err != nil {
		t.Fatalf("RenderScreen: unexpected error: %v", err)
	}
	second, err := RenderScreen("identity-manager", entryScreenID(mustSurface(t, "identity-manager")))
	if err != nil {
		t.Fatalf("RenderScreen: unexpected error on second call: %v", err)
	}
	if first != second {
		t.Fatalf("RenderScreen: not deterministic:\n%q\nvs\n%q", first, second)
	}
	wantBreadcrumb := "identity-manager/" + entryScreenID(mustSurface(t, "identity-manager"))
	if !strings.Contains(first, wantBreadcrumb) {
		t.Fatalf("RenderScreen: output missing breadcrumb %q:\n%s", wantBreadcrumb, first)
	}
}

func TestRenderScreen_UnknownScreenErrors(t *testing.T) {
	if _, err := RenderScreen("identity-manager", "no-such-screen"); err == nil {
		t.Fatal("RenderScreen: expected error for unknown screen, got nil")
	}
	if _, err := RenderScreen("no-such-surface", "no-such-screen"); err == nil {
		t.Fatal("RenderScreen: expected error for unknown surface, got nil")
	}
}

// mustSurface is a test helper that fetches a registered surface or fails
// the test immediately (used only to derive entryScreenID for the five
// FINAL placeholder surfaces seeded by data.go's init()).
func mustSurface(t *testing.T, id string) SurfaceDef {
	t.Helper()
	sd, ok := lookupSurface(id)
	if !ok {
		t.Fatalf("surface %q not registered — data.go init() did not run?", id)
	}
	return sd
}
