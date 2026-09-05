package main

import (
	"path/filepath"
	"regexp"
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

// TestGoreleaserConfigBrewsStanzaDeferredNotDeleted asserts D-18 (10-CONTEXT.md
// addendum, 2026-09-05): the D-13 brews: publish leg is DEFERRED (gated at the
// Makefile/CLI level via --skip=homebrew), never deleted from this file. The
// day HOMEBREW_TAP_GITHUB_TOKEN exists as a real secret, this stanza must
// still be here, unchanged, ready to publish with zero config edits.
func TestGoreleaserConfigBrewsStanzaDeferredNotDeleted(t *testing.T) {
	src := readRepoFile(t, goreleaserConfigPath(t))
	if !strings.Contains(src, "brews:") {
		t.Fatal(".goreleaser.yaml is missing the brews: stanza — D-13/D-18 require it to be DEFERRED, not deleted")
	}
	if !strings.Contains(src, `token: "{{ .Env.HOMEBREW_TAP_GITHUB_TOKEN }}"`) {
		t.Fatal(".goreleaser.yaml brews: stanza is missing its HOMEBREW_TAP_GITHUB_TOKEN token reference")
	}
}

// TestGoreleaserConfigNeverAddsProOnlyNightlyBlock is a regression guard for
// D-19's empirically-verified finding: GoReleaser's native `nightly:` config
// key is GoReleaser-Pro-only and is silently meaningless (or rejected) by the
// pinned OSS v2.18.0 binary this project actually uses. This file must never
// gain a `nightly:` top-level key — the real nightly mechanism lives entirely
// in the Makefile's release-nightly target (a fresh git tag + the ORDINARY
// `release` command), not in this config file.
func TestGoreleaserConfigNeverAddsProOnlyNightlyBlock(t *testing.T) {
	src := readRepoFile(t, goreleaserConfigPath(t))
	re := regexp.MustCompile(`(?m)^nightly:`)
	if re.MatchString(src) {
		t.Fatal(".goreleaser.yaml must not declare a top-level `nightly:` key — that feature is GoReleaser-Pro-only and unavailable in the pinned OSS v2.18.0 binary (D-19); the real mechanism is the Makefile's release-nightly target")
	}
}
