package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// goreleaserConfigPath returns the path to the repo-root .goreleaser.yaml,
// mirroring workflowPath/makefilePath's own repo-root-relative pattern in
// release_plumbing_test.go (same package).
func goreleaserConfigPath(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "..", ".goreleaser.yaml")
}

// TestGoreleaserConfigDistIsNeverBin asserts .goreleaser.yaml points its
// output directory at goreleaser's own default `dist`, never at `bin` — the
// Makefile's own build/build-cross targets write into bin/, and both `make
// release`/`make release-snapshot` invoke goreleaser with --clean, which
// deletes the dist: directory before every build (REVIEW C-4).
func TestGoreleaserConfigDistIsNeverBin(t *testing.T) {
	src := readRepoFile(t, goreleaserConfigPath(t))
	if !strings.Contains(src, "dist: dist") {
		t.Fatal(".goreleaser.yaml missing literal `dist: dist`")
	}
	if strings.Contains(src, "dist: bin") {
		t.Fatal(".goreleaser.yaml points dist: at bin — this would collide with build-cross output (REVIEW C-4)")
	}
}

// TestGoreleaserConfigPrereleaseAuto asserts the release: pipe classifies
// prerelease status from the tag suffix (D-08's "curated header" +
// REVIEW C-3's make_latest fix both depend on this being wired).
func TestGoreleaserConfigPrereleaseAuto(t *testing.T) {
	src := readRepoFile(t, goreleaserConfigPath(t))
	if !strings.Contains(src, "prerelease: auto") {
		t.Fatal(".goreleaser.yaml missing `prerelease: auto`")
	}
}

// TestGoreleaserConfigMakeLatestTiedToPrerelease asserts the literal
// REVIEW C-3 fix: make_latest must never default to true unconditionally,
// or a prerelease could be briefly presented as the repository's latest
// stable download.
func TestGoreleaserConfigMakeLatestTiedToPrerelease(t *testing.T) {
	src := readRepoFile(t, goreleaserConfigPath(t))
	if !strings.Contains(src, `make_latest: "{{ not .Prerelease }}"`) {
		t.Fatal(`.goreleaser.yaml missing literal make_latest: "{{ not .Prerelease }}" (REVIEW C-3)`)
	}
}

// TestGoreleaserConfigLdflagsTargetVersionPackage asserts the build's
// ldflags -X path targets internal/version's exact symbol path — proving
// this is the SAME path Plan 10-01 wired the Makefile's own LDFLAGS to, not
// a drifted duplicate (D-05: no drift between two independent version
// descriptions).
func TestGoreleaserConfigLdflagsTargetVersionPackage(t *testing.T) {
	src := readRepoFile(t, goreleaserConfigPath(t))
	const wantPath = "github.com/castocolina/gitid/internal/version."
	if !strings.Contains(src, wantPath) {
		t.Fatalf(".goreleaser.yaml ldflags missing -X path %q", wantPath)
	}
	for _, symbol := range []string{"version=", "commit=", "buildDate="} {
		if !strings.Contains(src, wantPath+symbol) {
			t.Errorf("ldflags missing -X %s%s", wantPath, symbol)
		}
	}
	if !strings.Contains(src, "{{.Env.VERSION}}") || !strings.Contains(src, "{{.Env.COMMIT}}") || !strings.Contains(src, "{{.Env.DATE}}") {
		t.Fatal(".goreleaser.yaml ldflags must reference {{.Env.VERSION}}/{{.Env.COMMIT}}/{{.Env.DATE}} — the SAME env vars the Makefile computes, not goreleaser's own internal git describe (Pitfall 2)")
	}
}
