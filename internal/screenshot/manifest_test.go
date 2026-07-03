package screenshot

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/castocolina/gitid/internal/dummytui"
)

// writeManifest writes entries as a manifest.json under
// <dir>/<surfaceDirName>/manifest.json — surfaceDirName need not equal any
// entry's Surface field (LoadManifests only cares about the JSON content,
// not the directory name), which TestManifestSchema exploits to place two
// colliding manifests in different directories.
func writeManifest(t *testing.T, dir, surfaceDirName string, entries []ScreenManifestEntry) {
	t.Helper()
	surfaceDir := filepath.Join(dir, surfaceDirName)
	if err := os.MkdirAll(surfaceDir, 0o750); err != nil {
		t.Fatalf("writeManifest: MkdirAll: %v", err)
	}
	raw, err := json.Marshal(entries)
	if err != nil {
		t.Fatalf("writeManifest: Marshal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(surfaceDir, "manifest.json"), raw, 0o600); err != nil { //nolint:gosec // test-only fixture (G306)
		t.Fatalf("writeManifest: WriteFile: %v", err)
	}
}

func TestManifestSchema(t *testing.T) {
	t.Run("valid manifest loads", func(t *testing.T) {
		dir := t.TempDir()
		writeManifest(t, dir, "identity-manager", []ScreenManifestEntry{
			{Surface: "identity-manager", Screen: "entry", HTMLRoute: "/", KeysFromHome: []string{"1"}, Signature: "Identity Manager"},
		})

		entries, err := LoadManifests(dir)
		if err != nil {
			t.Fatalf("LoadManifests: %v", err)
		}
		if len(entries) != 1 {
			t.Fatalf("LoadManifests: got %d entries, want 1", len(entries))
		}
		if got, want := ScreenID(entries[0]), "identity-manager/entry"; got != want {
			t.Errorf("ScreenID: got %q, want %q", got, want)
		}
	})

	t.Run("no manifests is a no-op, not an error", func(t *testing.T) {
		dir := t.TempDir()
		entries, err := LoadManifests(dir)
		if err != nil {
			t.Fatalf("LoadManifests: %v", err)
		}
		if len(entries) != 0 {
			t.Fatalf("LoadManifests: got %d entries, want 0", len(entries))
		}
	})

	t.Run("missing surface rejected", func(t *testing.T) {
		dir := t.TempDir()
		writeManifest(t, dir, "s", []ScreenManifestEntry{
			{Surface: "", Screen: "entry", HTMLRoute: "/x", KeysFromHome: []string{"1"}, Signature: "sig"},
		})
		if _, err := LoadManifests(dir); err == nil || !strings.Contains(err.Error(), `"surface"`) {
			t.Fatalf("LoadManifests: expected a missing-surface error, got %v", err)
		}
	})

	t.Run("missing htmlRoute rejected", func(t *testing.T) {
		dir := t.TempDir()
		writeManifest(t, dir, "s", []ScreenManifestEntry{
			{Surface: "identity-manager", Screen: "entry", HTMLRoute: "", KeysFromHome: []string{"1"}, Signature: "sig"},
		})
		if _, err := LoadManifests(dir); err == nil || !strings.Contains(err.Error(), `"htmlRoute"`) {
			t.Fatalf("LoadManifests: expected a missing-htmlRoute error, got %v", err)
		}
	})

	t.Run("missing signature rejected", func(t *testing.T) {
		dir := t.TempDir()
		writeManifest(t, dir, "s", []ScreenManifestEntry{
			{Surface: "identity-manager", Screen: "entry", HTMLRoute: "/x", KeysFromHome: []string{"1"}, Signature: ""},
		})
		if _, err := LoadManifests(dir); err == nil || !strings.Contains(err.Error(), `"signature"`) {
			t.Fatalf("LoadManifests: expected a missing-signature error, got %v", err)
		}
	})

	t.Run("missing keysFromHome rejected", func(t *testing.T) {
		dir := t.TempDir()
		writeManifest(t, dir, "s", []ScreenManifestEntry{
			{Surface: "identity-manager", Screen: "entry", HTMLRoute: "/x", KeysFromHome: nil, Signature: "sig"},
		})
		if _, err := LoadManifests(dir); err == nil || !strings.Contains(err.Error(), `"keysFromHome"`) {
			t.Fatalf("LoadManifests: expected a missing-keysFromHome error, got %v", err)
		}
	})

	t.Run("duplicate screen rejected across two manifests", func(t *testing.T) {
		dir := t.TempDir()
		writeManifest(t, dir, "surface-a", []ScreenManifestEntry{
			{Surface: "identity-manager", Screen: "entry", HTMLRoute: "/a", KeysFromHome: []string{"1"}, Signature: "sig-a"},
		})
		writeManifest(t, dir, "surface-b", []ScreenManifestEntry{
			{Surface: "identity-manager", Screen: "entry", HTMLRoute: "/b", KeysFromHome: []string{"1"}, Signature: "sig-b"},
		})
		if _, err := LoadManifests(dir); err == nil || !strings.Contains(err.Error(), "duplicate screen") {
			t.Fatalf("LoadManifests: expected a duplicate-screen error, got %v", err)
		}
	})

	t.Run("duplicate htmlRoute rejected across two manifests", func(t *testing.T) {
		dir := t.TempDir()
		writeManifest(t, dir, "surface-a", []ScreenManifestEntry{
			{Surface: "surface-a", Screen: "entry", HTMLRoute: "/same", KeysFromHome: []string{"1"}, Signature: "sig-a"},
		})
		writeManifest(t, dir, "surface-b", []ScreenManifestEntry{
			{Surface: "surface-b", Screen: "entry", HTMLRoute: "/same", KeysFromHome: []string{"2"}, Signature: "sig-b"},
		})
		if _, err := LoadManifests(dir); err == nil || !strings.Contains(err.Error(), "duplicate htmlRoute") {
			t.Fatalf("LoadManifests: expected a duplicate-htmlRoute error, got %v", err)
		}
	})

	t.Run("duplicate signature rejected across two manifests", func(t *testing.T) {
		dir := t.TempDir()
		writeManifest(t, dir, "surface-a", []ScreenManifestEntry{
			{Surface: "surface-a", Screen: "entry", HTMLRoute: "/a", KeysFromHome: []string{"1"}, Signature: "same-sig"},
		})
		writeManifest(t, dir, "surface-b", []ScreenManifestEntry{
			{Surface: "surface-b", Screen: "entry", HTMLRoute: "/b", KeysFromHome: []string{"2"}, Signature: "same-sig"},
		})
		if _, err := LoadManifests(dir); err == nil || !strings.Contains(err.Error(), "duplicate signature") {
			t.Fatalf("LoadManifests: expected a duplicate-signature error, got %v", err)
		}
	})
}

// TestManifestCrossValidation asserts every REAL manifest entry currently
// checked in under .planning/design/*/manifest.json resolves against
// internal/dummytui.RenderScreen and that the rendered output contains the
// "<surface>/<screen>" breadcrumb (review HIGH-3d, TUI side — the HTML side
// is cross-validated at capture time by design_capture_test.go's
// RequiredText assertion plus the mockup's own App.tsx/verify-routes.mjs
// route-shape gate). With no manifests checked in yet (this plan ships the
// loader; surfaces add their own manifest.json files in later plans), this
// loop body never runs — an intentional no-op, never a silent false pass
// masquerading as coverage of screens that do not exist yet.
func TestManifestCrossValidation(t *testing.T) {
	designDir := filepath.Join("..", "..", ".planning", "design")
	entries, err := LoadManifests(designDir)
	if err != nil {
		t.Fatalf("LoadManifests(%q): %v", designDir, err)
	}
	for _, e := range entries {
		e := e
		t.Run(ScreenID(e), func(t *testing.T) {
			view, err := dummytui.RenderScreen(e.Surface, e.Screen)
			if err != nil {
				t.Fatalf("dummytui.RenderScreen(%q, %q): %v", e.Surface, e.Screen, err)
			}
			if !strings.Contains(view, ScreenID(e)) {
				t.Errorf("dummytui.RenderScreen(%q, %q) output is missing the %q breadcrumb", e.Surface, e.Screen, ScreenID(e))
			}
		})
	}
}
