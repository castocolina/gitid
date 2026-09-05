package main

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// releaseWorkflowPath mirrors workflowPath(t) (release_plumbing_test.go, same
// package) but points at the NEW goreleaser-driven release.yml (Phase 10
// plan 10-04, D-06) rather than ci.yml.
func releaseWorkflowPath(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "..", ".github", "workflows", "release.yml")
}

// TestReleaseWorkflowNeverInlinesExpressionsIntoRunScripts is a security
// regression guard (gsd-code-reviewer finding, ci-cd-script-injection):
// a `${{ ... }}` GitHub Actions expression substitutes into a `run:` step's
// script TEXT before the shell ever parses it. Since this job's version/
// commit/date values ultimately derive from the pushed tag name
// (steps.relver.outputs.version, from ${GITHUB_REF_NAME#v}), inlining them
// directly into a run: script is a classic script-injection vector against
// a job carrying contents:write/id-token:write/attestations:write. Values
// MUST be routed through a step's env: block and referenced as ordinary
// shell variables ($VAR) instead.
func TestReleaseWorkflowNeverInlinesExpressionsIntoRunScripts(t *testing.T) {
	src := readRepoFile(t, releaseWorkflowPath(t))
	lines := strings.Split(src, "\n")
	inRun := false
	runIndent := -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		indent := len(line) - len(strings.TrimLeft(line, " "))
		if strings.HasPrefix(trimmed, "run:") {
			inRun = true
			runIndent = indent
			continue
		}
		if inRun {
			// A run: block ends at the first line at or below its own indent
			// that isn't blank (YAML block-scalar dedent).
			if trimmed != "" && indent <= runIndent {
				inRun = false
			} else if strings.Contains(line, "${{") {
				t.Fatalf("line %d: run: step script inlines a ${{ }} expression directly — route through env: and reference as a shell variable instead (script-injection risk): %s", i+1, trimmed)
			}
		}
	}
}

func TestReleaseWorkflowIsTagScoped(t *testing.T) {
	src := readRepoFile(t, releaseWorkflowPath(t))
	if !strings.Contains(src, "tags: ['v*']") && !strings.Contains(src, `tags: ["v*"]`) {
		t.Fatal("release.yml on: push: is missing a quoted tags glob 'v*'")
	}
}

func TestReleaseWorkflowJobHasExactlyThreeScopedPermissions(t *testing.T) {
	src := readRepoFile(t, releaseWorkflowPath(t))
	block := jobBlock(t, src, "release")
	if !regexp.MustCompile(`(?m)^\s+permissions:\s*$`).MatchString(block) {
		t.Fatal("release job missing permissions:")
	}
	for _, want := range []string{"contents: write", "id-token: write", "attestations: write"} {
		if !strings.Contains(block, want) {
			t.Fatalf("release job permissions missing %q", want)
		}
	}
}

// TestReleaseWorkflowStepSequence asserts setup-env-release, NOT setup-env,
// is wired in (REVIEW C-7: the regression guard that the narrower target is
// actually used), and that the gate (test+lint) runs before make release,
// which runs before the provenance attestation step.
func TestReleaseWorkflowStepSequence(t *testing.T) {
	src := readRepoFile(t, releaseWorkflowPath(t))
	block := jobBlock(t, src, "release")

	if strings.Contains(block, "run: make setup-env\n") {
		t.Fatal("release job calls the full make setup-env, not the narrower make setup-env-release (REVIEW C-7)")
	}

	idxSetup := strings.Index(block, "make setup-env-release")
	idxTest := strings.Index(block, "run: make test\n")
	idxLint := strings.Index(block, "run: make lint\n")
	idxRelease := strings.Index(block, "run: |\n          make release")
	idxAttest := strings.Index(block, "actions/attest-build-provenance")

	for name, idx := range map[string]int{
		"make setup-env-release":  idxSetup,
		"make test":               idxTest,
		"make lint":               idxLint,
		"make release":            idxRelease,
		"attest-build-provenance": idxAttest,
	} {
		if idx < 0 {
			t.Fatalf("release job missing step: %s", name)
		}
	}
	if idxSetup >= idxTest || idxTest >= idxLint || idxLint >= idxRelease || idxRelease >= idxAttest {
		t.Fatalf("release job steps out of order: setup-env-release=%d test=%d lint=%d release=%d attest=%d",
			idxSetup, idxTest, idxLint, idxRelease, idxAttest)
	}
}

func TestReleaseWorkflowPinsEveryActionToACommitSHA(t *testing.T) {
	src := readRepoFile(t, releaseWorkflowPath(t))
	re := regexp.MustCompile(`^uses:\s+[^@]+@[0-9a-f]{40} # v`)
	var uses int
	for i, line := range strings.Split(src, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "uses: ") {
			continue
		}
		uses++
		if !re.MatchString(trimmed) {
			t.Errorf("line %d: uses: is not pinned to a 40-hex SHA with # v comment: %s", i+1, trimmed)
		}
	}
	if uses == 0 {
		t.Fatal("no uses: lines found in release.yml")
	}
}

// TestReleaseWorkflowNeverAmendsAfterPublication checks executable `run:`
// content only (not doc comments — the D-08 --verify-tag equivalence note
// legitimately mentions `gh release create` as prose explaining why it is
// unnecessary here).
func TestReleaseWorkflowNeverAmendsAfterPublication(t *testing.T) {
	src := readRepoFile(t, releaseWorkflowPath(t))
	re := regexp.MustCompile(`(?i)gh\s+release`)
	for i, line := range strings.Split(src, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		if re.MatchString(line) {
			t.Fatalf("release.yml line %d amends the release after publication: %s", i+1, line)
		}
	}
}

// TestReleaseWorkflowAttestsArchivesAndChecksums asserts REVIEW C-8's fix:
// the provenance attestation must cover BOTH the archives and the checksums
// manifest — the exact file the installer trusts to validate an archive's
// integrity (D-08, D-15).
func TestReleaseWorkflowAttestsArchivesAndChecksums(t *testing.T) {
	src := readRepoFile(t, releaseWorkflowPath(t))
	block := jobBlock(t, src, "release")
	idx := strings.Index(block, "subject-path:")
	if idx < 0 {
		t.Fatal("release job missing subject-path:")
	}
	rest := block[idx:]
	end := strings.Index(rest, "\n\n")
	if end < 0 {
		end = len(rest)
	}
	subjectPathBlock := rest[:end]
	if !strings.Contains(subjectPathBlock, "dist/*.tar.gz") {
		t.Fatal("subject-path missing dist/*.tar.gz (REVIEW C-8)")
	}
	if !strings.Contains(subjectPathBlock, "dist/*_checksums.txt") {
		t.Fatal("subject-path missing dist/*_checksums.txt (REVIEW C-8: the checksums manifest must be attested too)")
	}
}

// TestReleaseWorkflowDocumentsVerifyTagEquivalence asserts D-08's
// `--verify-tag` clause is resolved with an explicit, tested code comment
// (10-RESEARCH.md Pitfall 5: goreleaser has no equivalent flag), not
// silently dropped.
func TestReleaseWorkflowDocumentsVerifyTagEquivalence(t *testing.T) {
	src := readRepoFile(t, releaseWorkflowPath(t))
	if !strings.Contains(strings.ToLower(src), "verify-tag") {
		t.Fatal("release.yml missing a documented --verify-tag structural-equivalence resolution (D-08)")
	}
}
