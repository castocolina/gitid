package main

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// goreleaserGoRequirement records, for a given goreleaser/goreleaser/v2 tag,
// the `go` directive its own go.mod declares — the minimum Go toolchain that
// tag can be built with. Every entry is sourced live from
// proxy.golang.org/github.com/goreleaser/goreleaser/v2/@v/<ver>.mod and dated
// in the comment beside it, discharging the Makefile's own GORELEASER_VERSION
// pin comment's "do NOT change without fresh verification" policy
// mechanically instead of by prose alone.
type goreleaserGoRequirement struct {
	version   string
	goVersion string // e.g. "1.26.4"
}

// goreleaserGoRequirements is the table this test consults. Verified live
// against the Go module proxy on 2026-09-21 (this plan's own execution date):
//
//	curl -sS https://proxy.golang.org/github.com/goreleaser/goreleaser/v2/@v/<ver>.mod | grep '^go '
//
// A future GORELEASER_VERSION bump to a tag not listed here fails Test 2
// below (the drift gate) — the bumper must fetch that tag's own `go`
// directive from the proxy the same way and add a row, never guess.
var goreleaserGoRequirements = []goreleaserGoRequirement{
	{version: "v2.16.0", goVersion: "1.26.3"}, // verified 2026-09-21
	{version: "v2.17.0", goVersion: "1.26.4"}, // verified 2026-09-21
	{version: "v2.17.1", goVersion: "1.26.5"}, // verified 2026-09-21
	{version: "v2.18.0", goVersion: "1.27.0"}, // verified 2026-09-21
	{version: "v2.18.1", goVersion: "1.27.1"}, // verified 2026-09-21
	{version: "v2.18.2", goVersion: "1.27.1"}, // verified 2026-09-21
}

// parseGoVersion splits a "1.26.4"-shaped (or "1.27"-shaped, patch omitted)
// Go version string into major/minor/patch ints. A missing patch element
// parses as 0, so "1.27" and "1.27.0" compare equal.
func parseGoVersion(t *testing.T, raw string) (major, minor, patch int) {
	t.Helper()
	parts := strings.Split(raw, ".")
	if len(parts) < 2 {
		t.Fatalf("parseGoVersion: %q does not look like a major.minor[.patch] Go version", raw)
	}
	var err error
	major, err = strconv.Atoi(parts[0])
	if err != nil {
		t.Fatalf("parseGoVersion: %q: bad major component: %v", raw, err)
	}
	minor, err = strconv.Atoi(parts[1])
	if err != nil {
		t.Fatalf("parseGoVersion: %q: bad minor component: %v", raw, err)
	}
	if len(parts) >= 3 {
		patch, err = strconv.Atoi(parts[2])
		if err != nil {
			t.Fatalf("parseGoVersion: %q: bad patch component: %v", raw, err)
		}
	}
	return major, minor, patch
}

// goVersionLTE reports whether a <= b, compared as parsed major/minor/patch
// ints (never as strings — "1.26.10" must sort after "1.26.4").
func goVersionLTE(aMaj, aMin, aPatch, bMaj, bMin, bPatch int) bool {
	if aMaj != bMaj {
		return aMaj < bMaj
	}
	if aMin != bMin {
		return aMin < bMin
	}
	return aPatch <= bPatch
}

// extractPinnedGoreleaserVersion reads GORELEASER_VERSION out of the
// Makefile, anchored at column 0 so it matches only the real assignment line
// (never a doc-comment prose mention of a version number elsewhere in the
// file).
func extractPinnedGoreleaserVersion(t *testing.T, makefile string) string {
	t.Helper()
	re := regexp.MustCompile(`(?m)^GORELEASER_VERSION := (v[0-9]+\.[0-9]+\.[0-9]+)`)
	m := re.FindStringSubmatch(makefile)
	if m == nil {
		t.Fatal("Makefile has no `GORELEASER_VERSION := vX.Y.Z` line — the pin variable was renamed or removed")
	}
	return m[1]
}

// extractExportedGoToolchain reads the GOTOOLCHAIN Makefile exports for
// every recipe (including setup-env/setup-env-release's `go install
// goreleaser@$(GORELEASER_VERSION)` lines), anchored at column 0 for the
// same reason as extractPinnedGoreleaserVersion above.
func extractExportedGoToolchain(t *testing.T, makefile string) string {
	t.Helper()
	re := regexp.MustCompile(`(?m)^export GOTOOLCHAIN := go([0-9]+\.[0-9]+(?:\.[0-9]+)?)`)
	m := re.FindStringSubmatch(makefile)
	if m == nil {
		t.Fatal("Makefile has no `export GOTOOLCHAIN := goX.Y[.Z]` line — the exported toolchain pin was renamed or removed")
	}
	return m[1]
}

// TestGoreleaserPinIsBuildableByPinnedToolchain is a network-free regression
// guard: the pinned GORELEASER_VERSION must be installable under the
// Makefile's own exported GOTOOLCHAIN, or every workflow calling `make
// setup-env`/`make setup-env-release` dies at the bootstrap step exactly as
// it did from 2026-09-05 onward (goreleaser v2.18.0 requires go >= 1.27.0,
// while GOTOOLCHAIN pins go1.26.4 — this is the real CI failure this test
// exists to catch and prevent from recurring silently).
func TestGoreleaserPinIsBuildableByPinnedToolchain(t *testing.T) {
	makefile := readRepoFile(t, makefilePath(t))
	pinnedVersion := extractPinnedGoreleaserVersion(t, makefile)
	toolchain := extractExportedGoToolchain(t, makefile)
	toolchainMaj, toolchainMin, toolchainPatch := parseGoVersion(t, toolchain)

	t.Run("pinned version builds under the pinned toolchain", func(t *testing.T) {
		var required *goreleaserGoRequirement
		for i := range goreleaserGoRequirements {
			if goreleaserGoRequirements[i].version == pinnedVersion {
				required = &goreleaserGoRequirements[i]
				break
			}
		}
		if required == nil {
			t.Fatalf("Makefile pins GORELEASER_VERSION := %s, which has no entry in goreleaserGoRequirements — cannot check buildability; see Test 2 below", pinnedVersion)
		}
		reqMaj, reqMin, reqPatch := parseGoVersion(t, required.goVersion)
		if !goVersionLTE(reqMaj, reqMin, reqPatch, toolchainMaj, toolchainMin, toolchainPatch) {
			t.Fatalf(
				"Makefile GORELEASER_VERSION := %s requires go >= %s, but Makefile's `export GOTOOLCHAIN := go%s` pins an OLDER toolchain — "+
					"`go install github.com/goreleaser/goreleaser/v2@%s` will fail with \"requires go >= %s (running go %s)\" in every CI job that runs "+
					"`make setup-env`/`make setup-env-release`. Either downgrade GORELEASER_VERSION to a tag whose go.mod requirement is <= %s, "+
					"or raise GOTOOLCHAIN to >= %s in the same commit.",
				pinnedVersion, required.goVersion, toolchain, pinnedVersion, required.goVersion, toolchain, toolchain, required.goVersion,
			)
		}
	})

	t.Run("pinned version has a table entry (drift gate)", func(t *testing.T) {
		for _, entry := range goreleaserGoRequirements {
			if entry.version == pinnedVersion {
				return
			}
		}
		t.Fatalf(
			"Makefile pins GORELEASER_VERSION := %s, which has no entry in goreleaserGoRequirements (cmd/gitid/goreleaser_pin_test.go). "+
				"Fetch its `go` directive live from the module proxy before bumping the pin: "+
				"curl -sS https://proxy.golang.org/github.com/goreleaser/goreleaser/v2/@v/%s.mod | grep '^go ' -- then add a dated row to the table.",
			pinnedVersion, pinnedVersion,
		)
	})
}
