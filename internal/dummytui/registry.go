package dummytui

import (
	"fmt"
	"sort"

	tea "charm.land/bubbletea/v2"
)

// ScreenDef defines a single screen within a surface: its ID, the intra-surface
// key transitions available while it is active (ScreenDef.Keys maps a key
// string to the target screen ID within the SAME surface), and a Render
// function returning the screen's BODY content (not the full shell — the
// shell wraps it with header/status/keybar, see RenderScreen).
type ScreenDef struct {
	ID     string
	Keys   map[string]string
	Render func() string
}

// SurfaceDef defines a top-level or modal surface.
//
// ActivationKey is set ONLY on the five number-key primary surfaces
// (identity-manager/global-ssh/global-git/health/fixer). LaunchFrom and
// LaunchKey are set ONLY on keyless modal surfaces (create-flow,
// git-screen): TARGET-OWNED, the modal surface names the source surface ID
// (LaunchFrom) and the key (LaunchKey) that, while LaunchFrom is the active
// top-level surface, pushes this surface as a modal frame. See doc.go for
// the full modal-launch contract and the key-allocation table.
type SurfaceDef struct {
	ID            string
	Title         string
	ActivationKey string
	LaunchFrom    string
	LaunchKey     string
	Screens       []ScreenDef
}

// modalFrame identifies one entry on navState.modalStack: a keyless surface
// and its currently active screen.
type modalFrame struct {
	Surface string
	Screen  string
}

// navState is the dummy's pure navigation state: the active top-level
// surface (view), its active screen, and a stack of launched modal frames.
// An empty modalStack means no modal is open.
type navState struct {
	view         string
	activeScreen string
	modalStack   []modalFrame
}

// registry is the package-level surface store. Populated via Register /
// RegisterOrReplace, normally called from each surface file's init().
var registry = map[string]SurfaceDef{}

// reservedOrNumberKeys are keys that can never be claimed as a LaunchKey or
// an intra-surface ScreenDef.Keys transition: the five ActivationKey number
// keys plus the globally reserved keys (doc.go key-allocation table).
var reservedOrNumberKeys = map[string]bool{
	"1": true, "2": true, "3": true, "4": true, "5": true,
	"esc": true, "q": true, "?": true, "/": true, "enter": true,
	"up": true, "down": true, "left": true, "right": true, "j": true, "k": true,
}

// Register adds a new surface to the registry. It panics if sd.ActivationKey
// is non-empty and already claimed by a DIFFERENT surface — an empty/unset
// ActivationKey is EXEMPT from this uniqueness check, so keyless modal
// surfaces (create-flow, git-screen) both register via plain Register
// without colliding (review H2). It also panics on any LaunchKey collision
// (see collisionCheck) — a test-detectable failure at registration time
// rather than a silent 02-11 e2e failure (review iter-3).
func Register(sd SurfaceDef) {
	registerSurface(sd, false)
}

// RegisterOrReplace adds sd to the registry, replacing any existing surface
// that currently owns sd.ActivationKey (if ActivationKey is non-empty) so
// exactly one surface ends up owning that key. This is what lets a fan-out
// surface (Wave 4) replace a 02-02 placeholder without ever editing
// model.go or data.go. Runs the same LaunchKey collision guard as Register.
func RegisterOrReplace(sd SurfaceDef) {
	registerSurface(sd, true)
}

func registerSurface(sd SurfaceDef, replace bool) {
	var oldOwnerID string
	if sd.ActivationKey != "" {
		for id, existing := range registry {
			if id == sd.ID {
				continue
			}
			if existing.ActivationKey == sd.ActivationKey {
				if !replace {
					panic(fmt.Sprintf("dummytui: Register(%q): activation key %q already claimed by surface %q — use RegisterOrReplace to replace it", sd.ID, sd.ActivationKey, id))
				}
				oldOwnerID = id
			}
		}
	}

	if err := collisionCheck(sd); err != nil {
		panic(err.Error())
	}

	if oldOwnerID != "" {
		delete(registry, oldOwnerID)
	}
	registry[sd.ID] = sd
}

// collisionCheck runs the LaunchKey collision guard (review iter-3):
//
//   - Direction A (sd is a keyless modal surface): sd.LaunchKey must not be a
//     reserved/number key, must not already be claimed by any ScreenDef.Keys
//     transition of the sd.LaunchFrom surface, and must not already be
//     claimed by another keyless surface with the same LaunchFrom.
//   - Direction B (sd introduces/replaces ScreenDef.Keys transitions): none
//     of sd's screens may define a key that is a reserved/number key, or
//     that is already claimed as a LaunchKey by an existing keyless surface
//     whose LaunchFrom == sd.ID.
func collisionCheck(sd SurfaceDef) error {
	if sd.LaunchKey != "" {
		if reservedOrNumberKeys[sd.LaunchKey] {
			return fmt.Errorf("dummytui: register %q: LaunchKey %q is a reserved or number key", sd.ID, sd.LaunchKey)
		}
		if src, ok := registry[sd.LaunchFrom]; ok {
			for _, scr := range src.Screens {
				if _, exists := scr.Keys[sd.LaunchKey]; exists {
					return fmt.Errorf("dummytui: register %q: LaunchKey %q collides with an intra-surface ScreenDef.Keys transition on surface %q screen %q", sd.ID, sd.LaunchKey, sd.LaunchFrom, scr.ID)
				}
			}
		}
		for id, other := range registry {
			if id == sd.ID {
				continue
			}
			if other.LaunchFrom == sd.LaunchFrom && other.LaunchKey == sd.LaunchKey {
				return fmt.Errorf("dummytui: register %q: LaunchKey %q on LaunchFrom %q already claimed by keyless surface %q", sd.ID, sd.LaunchKey, sd.LaunchFrom, id)
			}
		}
	}

	for _, scr := range sd.Screens {
		for k := range scr.Keys {
			if reservedOrNumberKeys[k] {
				return fmt.Errorf("dummytui: register %q: screen %q key %q is a reserved or number key", sd.ID, scr.ID, k)
			}
			for id, other := range registry {
				if id == sd.ID {
					continue
				}
				if other.LaunchFrom == sd.ID && other.LaunchKey == k {
					return fmt.Errorf("dummytui: register %q: screen %q key %q collides with keyless surface %q's LaunchKey targeting %q", sd.ID, scr.ID, k, id, sd.ID)
				}
			}
		}
	}
	return nil
}

// Surfaces returns every registered surface, sorted by ID for deterministic
// iteration order.
func Surfaces() []SurfaceDef {
	out := make([]SurfaceDef, 0, len(registry))
	for _, sd := range registry {
		out = append(out, sd)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// lookupSurface returns the registered surface for id, if any.
func lookupSurface(id string) (SurfaceDef, bool) {
	sd, ok := registry[id]
	return sd, ok
}

// findScreen returns the ScreenDef with the given ID within sd, if any.
func findScreen(sd SurfaceDef, screenID string) (ScreenDef, bool) {
	for _, scr := range sd.Screens {
		if scr.ID == screenID {
			return scr, true
		}
	}
	return ScreenDef{}, false
}

// entryScreenID returns the first screen's ID for a surface — its default,
// entry-point screen.
func entryScreenID(sd SurfaceDef) string {
	if len(sd.Screens) == 0 {
		return ""
	}
	return sd.Screens[0].ID
}

// activationKeyOwner returns the surface ID whose ActivationKey equals key,
// if any.
func activationKeyOwner(key string) (string, bool) {
	for id, sd := range registry {
		if sd.ActivationKey != "" && sd.ActivationKey == key {
			return id, true
		}
	}
	return "", false
}

// keylessByLaunch returns the keyless surface whose LaunchFrom == source and
// LaunchKey == key, if any.
func keylessByLaunch(source, key string) (SurfaceDef, bool) {
	for _, sd := range registry {
		if sd.ActivationKey == "" && sd.LaunchFrom == source && sd.LaunchKey == key {
			return sd, true
		}
	}
	return SurfaceDef{}, false
}

// route is the pure navigation reducer (registry.go, review C3). Given the
// current navState and a key press, it returns the next navState. See
// doc.go for the full precedence/modal-launch contract.
func route(st navState, msg tea.KeyMsg) navState {
	k := msg.String()

	if len(st.modalStack) > 0 {
		return routeModal(st, k)
	}
	return routeTopLevel(st, k)
}

func routeModal(st navState, k string) navState {
	top := st.modalStack[len(st.modalStack)-1]

	if k == "esc" {
		st.modalStack = st.modalStack[:len(st.modalStack)-1]
		return st
	}

	sd, ok := registry[top.Surface]
	if !ok {
		return st
	}
	scr, ok := findScreen(sd, top.Screen)
	if ok {
		if target, exists := scr.Keys[k]; exists {
			frame := modalFrame{Surface: top.Surface, Screen: target}
			st.modalStack = append(append([]modalFrame(nil), st.modalStack[:len(st.modalStack)-1]...), frame)
			return st
		}
	}

	if nested, ok := keylessByLaunch(top.Surface, k); ok {
		frame := modalFrame{Surface: nested.ID, Screen: entryScreenID(nested)}
		st.modalStack = append(append([]modalFrame(nil), st.modalStack...), frame)
		return st
	}

	// Number keys (and any other unclaimed key) are no-ops while a modal is
	// active — they never reach the top-level view switch.
	return st
}

func routeTopLevel(st navState, k string) navState {
	sd, ok := registry[st.view]
	if ok {
		scr, ok := findScreen(sd, st.activeScreen)
		if ok {
			if target, exists := scr.Keys[k]; exists {
				st.activeScreen = target
				return st
			}
		}
	}

	if k == "esc" {
		if ok {
			st.activeScreen = entryScreenID(sd)
		}
		return st
	}

	if launched, ok := keylessByLaunch(st.view, k); ok {
		st.modalStack = append(st.modalStack, modalFrame{Surface: launched.ID, Screen: entryScreenID(launched)})
		return st
	}

	if targetID, ok := activationKeyOwner(k); ok {
		st.view = targetID
		st.activeScreen = entryScreenID(registry[targetID])
		return st
	}

	return st
}

// RenderScreen returns the deterministic FULL-SHELL View() string for
// (surfaceID, screenID), including the "<surface>/<screen>" breadcrumb
// screen-ID. It errors for an unknown surface or screen.
//
// Delegates to shell.go's renderShell, which composes the full four-region
// shell (header/body/status/keybar) — review C2: Task 1 shipped a
// self-contained minimal inline composition here so the package compiled and
// this function's tests passed standalone before shell.go existed; Task 2
// rewired this body to shell.go's full composition, which every capture and
// live model.go View() call now shares.
func RenderScreen(surfaceID, screenID string) (string, error) {
	sd, ok := lookupSurface(surfaceID)
	if !ok {
		return "", fmt.Errorf("dummytui: RenderScreen: unknown surface %q", surfaceID)
	}
	scr, ok := findScreen(sd, screenID)
	if !ok {
		return "", fmt.Errorf("dummytui: RenderScreen: unknown screen %q on surface %q", screenID, surfaceID)
	}

	return renderShell(sd, scr), nil
}
