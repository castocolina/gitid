package main

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// nightlyWorkflowPath mirrors releaseWorkflowPath (release_yml_test.go, same
// package) but points at the new nightly.yml (D-19, 10-CONTEXT.md addendum
// 2026-09-05).
func nightlyWorkflowPath(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "..", ".github", "workflows", "nightly.yml")
}

// TestNightlyWorkflowExistsAndTriggers asserts nightly.yml exists and fires
// on BOTH schedule and workflow_dispatch (D-19).
func TestNightlyWorkflowExistsAndTriggers(t *testing.T) {
	src := readRepoFile(t, nightlyWorkflowPath(t))
	if !strings.Contains(src, "schedule:") {
		t.Fatal("nightly.yml missing a schedule: trigger")
	}
	if !strings.Contains(src, "cron:") {
		t.Fatal("nightly.yml missing a cron: expression under schedule:")
	}
	if !strings.Contains(src, "workflow_dispatch:") {
		t.Fatal("nightly.yml missing a workflow_dispatch: trigger")
	}
}

// TestNightlyWorkflowJobHasExactlyContentsWritePermission asserts the
// nightly job carries ONLY contents: write — narrower than release.yml's
// job (no id-token/attestations, since nightly skips provenance attestation
// to keep this workflow narrow, D-19).
func TestNightlyWorkflowJobHasExactlyContentsWritePermission(t *testing.T) {
	src := readRepoFile(t, nightlyWorkflowPath(t))
	block := jobBlock(t, src, "nightly")
	entries := permissionsBlockEntries(t, block)
	want := []string{"contents: write"}
	if len(entries) != len(want) || entries[0] != want[0] {
		t.Fatalf("nightly job permissions = %v, want exactly %v (no id-token/attestations)", entries, want)
	}
}

// TestNightlyWorkflowPinsEveryActionToACommitSHA mirrors
// TestReleaseWorkflowPinsEveryActionToACommitSHA / TestWorkflowPinsEveryActionToACommitSHA.
func TestNightlyWorkflowPinsEveryActionToACommitSHA(t *testing.T) {
	src := readRepoFile(t, nightlyWorkflowPath(t))
	var uses int
	for i, line := range strings.Split(src, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "uses: ") {
			continue
		}
		uses++
		if !strings.Contains(trimmed, "@") || !strings.Contains(trimmed, "# v") {
			t.Errorf("line %d: uses: is not pinned to a commit SHA with a # v comment: %s", i+1, trimmed)
		}
	}
	if uses == 0 {
		t.Fatal("no uses: lines found in nightly.yml")
	}
}

// TestNightlyWorkflowStepSequence asserts the step order: setup-env-release
// -> test -> lint -> release-nightly (mirroring release.yml's own sequence
// test, but ending in release-nightly instead of release/attest).
func TestNightlyWorkflowStepSequence(t *testing.T) {
	src := readRepoFile(t, nightlyWorkflowPath(t))
	block := jobBlock(t, src, "nightly")

	idxSetup := strings.Index(block, "make setup-env-release")
	idxTest := strings.Index(block, "run: make test\n")
	idxLint := strings.Index(block, "run: make lint\n")
	idxNightly := strings.Index(block, "make release-nightly")

	for name, idx := range map[string]int{
		"make setup-env-release": idxSetup,
		"make test":              idxTest,
		"make lint":              idxLint,
		"make release-nightly":   idxNightly,
	} {
		if idx < 0 {
			t.Fatalf("nightly job missing step: %s", name)
		}
	}
	if idxSetup >= idxTest || idxTest >= idxLint || idxLint >= idxNightly {
		t.Fatalf("nightly job steps out of order: setup-env-release=%d test=%d lint=%d release-nightly=%d",
			idxSetup, idxTest, idxLint, idxNightly)
	}
}

// TestMakefileReleaseNightlyTargetNeverPassesProOnlyNightlyFlag is a
// regression guard for D-19's empirical finding: the pinned OSS goreleaser
// v2.18.0 binary has no --nightly flag (GoReleaser-Pro-only). The
// release-nightly Makefile target must always pass --skip=homebrew and must
// NEVER pass a literal --nightly flag to goreleaser.
func TestMakefileReleaseNightlyTargetNeverPassesProOnlyNightlyFlag(t *testing.T) {
	src := readRepoFile(t, makefilePath(t))
	// Anchored at column 0 so this matches the REAL target line, not the
	// "## release-nightly: ..." doc-comment heading above it (which
	// legitimately discusses --nightly in prose, explaining why it is NOT
	// used) — a naive strings.Index found that comment first and produced
	// a false failure here.
	re := regexp.MustCompile(`(?m)^release-nightly:`)
	loc := re.FindStringIndex(src)
	if loc == nil {
		t.Fatal("Makefile missing release-nightly: target")
	}
	// Look at the target's recipe body only (up to the next unindented line
	// or blank-then-non-tab line), a simple heuristic sufficient for this
	// Makefile's own formatting style.
	rest := src[loc[0]:]
	end := strings.Index(rest, "\n\n")
	if end < 0 {
		end = len(rest)
	}
	body := rest[:end]
	if !strings.Contains(body, "--skip=homebrew") {
		t.Fatal("Makefile release-nightly target does not unconditionally pass --skip=homebrew")
	}
	if strings.Contains(body, "--nightly") {
		t.Fatal("Makefile release-nightly target passes a literal --nightly flag — this flag does not exist in the pinned OSS goreleaser v2.18.0 binary (GoReleaser-Pro-only, D-19)")
	}
}
