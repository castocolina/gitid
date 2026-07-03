package screenshot

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// ScreenManifestEntry describes one capturable screen: which product
// surface it belongs to, its screen ID, the MUI mockup's HashRouter route,
// the ABSOLUTE-from-startup TUI keystroke sequence that reaches it on the
// real cmd/gitid-dummy binary, and a screen-specific text signature. The
// signature (plus the "<surface>/<screen>" breadcrumb built from
// Surface/Screen via ScreenID) is what proves a capture/e2e assertion
// landed on the RIGHT screen — never a false positive (review HIGH-3c,
// T-02-FP).
//
// For a KEYLESS modal surface (create-flow, git-screen — added by later
// fan-out plans), KeysFromHome MUST include the target-owned launch key
// (SurfaceDef.LaunchKey, see internal/dummytui/doc.go's modal-launch
// contract) that opens the modal from its LaunchFrom surface — the modal
// is reached the way a real user reaches it on the running binary, never
// via a direct dummytui.RenderScreen call in the e2e (review C3).
type ScreenManifestEntry struct {
	Surface      string   `json:"surface"`
	Screen       string   `json:"screen"`
	HTMLRoute    string   `json:"htmlRoute"`
	KeysFromHome []string `json:"keysFromHome"`
	Signature    string   `json:"signature"`
}

// ScreenID returns the "<surface>/<screen>" breadcrumb identifier shared
// across the MUI mockup's Header (route module `title`), internal/dummytui's
// RenderScreen, and every capture/e2e assertion (review HIGH-3b).
func ScreenID(e ScreenManifestEntry) string {
	return e.Surface + "/" + e.Screen
}

// LoadManifests globs designDir/*/manifest.json, unmarshals each surface's
// entry array, and validates the hardened schema (review HIGH-3c):
//
//   - every entry has a non-empty Surface, Screen, HTMLRoute, and
//     Signature, and a non-empty KeysFromHome
//   - the "<surface>/<screen>" ScreenID is globally unique across every
//     manifest found
//   - HTMLRoute is globally unique across every manifest found
//   - Signature is globally unique across every manifest found
//
// It returns an error naming the first violation found (in manifest-glob
// order, which is sorted for determinism). A designDir with no
// manifest.json files yet returns an empty slice and a nil error —
// surfaces add manifests incrementally as later plans land; no shared
// driver code changes when they do (fan-out-safe).
func LoadManifests(designDir string) ([]ScreenManifestEntry, error) {
	pattern := filepath.Join(designDir, "*", "manifest.json")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("screenshot: LoadManifests: globbing %q: %w", pattern, err)
	}
	sort.Strings(matches) // deterministic load/validation order

	var all []ScreenManifestEntry
	seenScreenID := map[string]string{}
	seenRoute := map[string]string{}
	seenSignature := map[string]string{}

	for _, path := range matches {
		raw, readErr := os.ReadFile(path) //nolint:gosec // path from filepath.Glob over the repo-owned designDir, never external input (G304)
		if readErr != nil {
			return nil, fmt.Errorf("screenshot: LoadManifests: reading %q: %w", path, readErr)
		}
		var entries []ScreenManifestEntry
		if err := json.Unmarshal(raw, &entries); err != nil {
			return nil, fmt.Errorf("screenshot: LoadManifests: parsing %q: %w", path, err)
		}
		for _, e := range entries {
			if err := validateEntry(e, path); err != nil {
				return nil, err
			}

			id := ScreenID(e)
			if prev, dup := seenScreenID[id]; dup {
				return nil, fmt.Errorf("screenshot: LoadManifests: duplicate screen %q declared by both %q and %q", id, prev, path)
			}
			seenScreenID[id] = path

			if prev, dup := seenRoute[e.HTMLRoute]; dup {
				return nil, fmt.Errorf("screenshot: LoadManifests: duplicate htmlRoute %q declared by both %q and %q", e.HTMLRoute, prev, path)
			}
			seenRoute[e.HTMLRoute] = path

			if prev, dup := seenSignature[e.Signature]; dup {
				return nil, fmt.Errorf("screenshot: LoadManifests: duplicate signature %q declared by both %q and %q", e.Signature, prev, path)
			}
			seenSignature[e.Signature] = path

			all = append(all, e)
		}
	}
	return all, nil
}

// validateEntry checks the hardened per-entry schema (review HIGH-3c):
// every required field is non-empty and KeysFromHome is non-empty.
func validateEntry(e ScreenManifestEntry, path string) error {
	switch {
	case e.Surface == "":
		return fmt.Errorf("screenshot: LoadManifests: %q: entry is missing required field \"surface\"", path)
	case e.Screen == "":
		return fmt.Errorf("screenshot: LoadManifests: %q: entry for surface %q is missing required field \"screen\"", path, e.Surface)
	case e.HTMLRoute == "":
		return fmt.Errorf("screenshot: LoadManifests: %q: entry %q is missing required field \"htmlRoute\"", path, ScreenID(e))
	case e.Signature == "":
		return fmt.Errorf("screenshot: LoadManifests: %q: entry %q is missing required field \"signature\"", path, ScreenID(e))
	case len(e.KeysFromHome) == 0:
		return fmt.Errorf("screenshot: LoadManifests: %q: entry %q is missing required field \"keysFromHome\" (must be the absolute keystroke sequence from startup, including the launch key for a keyless modal surface)", path, ScreenID(e))
	}
	return nil
}

// SurfacesByEntries groups entries by Surface, preserving each surface's
// entries in the order LoadManifests returned them. Used by both the
// screenshot-tagged capture driver and the e2e-tagged PTY walker so
// per-surface subtests share one grouping implementation.
func SurfacesByEntries(entries []ScreenManifestEntry) map[string][]ScreenManifestEntry {
	bySurface := make(map[string][]ScreenManifestEntry)
	for _, e := range entries {
		bySurface[e.Surface] = append(bySurface[e.Surface], e)
	}
	return bySurface
}

// SortedSurfaceNames returns the keys of a surface->entries grouping (as
// produced by SurfacesByEntries), sorted for deterministic t.Run iteration
// order.
func SortedSurfaceNames(bySurface map[string][]ScreenManifestEntry) []string {
	names := make([]string, 0, len(bySurface))
	for name := range bySurface {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
