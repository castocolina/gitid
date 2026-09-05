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

// findInlinedRunExpression scans workflow YAML source for a `${{ }}`
// GitHub Actions expression inlined directly into a `run:` step's script
// text — the ci-cd-script-injection vulnerability class fixed in b08256c.
// It reports the FIRST offending line (1-indexed) it finds, covering both
// shapes a `run:` step can take:
//   - `run: |` block-scalar form: every continuation line at the block's
//     indent is scanned (this was already correct pre-WR-02-fix).
//   - single-line `run: <script>` form: the run: line ITSELF must also be
//     scanned — round-1 review WR-01 found the pre-fix loop `continue`d
//     the instant it matched the `run:` prefix, never scanning that same
//     line's text for an inlined expression.
func findInlinedRunExpression(src string) (lineNo int, line string, found bool) {
	lines := strings.Split(src, "\n")
	inRun := false
	runIndent := -1
	for i, l := range lines {
		trimmed := strings.TrimSpace(l)
		indent := len(l) - len(strings.TrimLeft(l, " "))
		if strings.HasPrefix(trimmed, "run:") {
			inRun = true
			runIndent = indent
			// WR-01 fix: check the run: line itself too — a single-line
			// `run: make release VERSION=${{ ... }}` step never enters the
			// `inRun` continuation-scanning branch below (there are no
			// continuation lines to scan), so the pre-fix loop's `continue`
			// here was a blind spot that would report PASS on exactly the
			// vulnerability class this guard exists to catch.
			if strings.Contains(l, "${{") {
				return i + 1, l, true
			}
			continue
		}
		if inRun {
			// A run: block ends at the first line at or below its own indent
			// that isn't blank (YAML block-scalar dedent).
			if trimmed != "" && indent <= runIndent {
				inRun = false
			} else if strings.Contains(l, "${{") {
				return i + 1, l, true
			}
		}
	}
	return 0, "", false
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
	if lineNo, line, found := findInlinedRunExpression(src); found {
		t.Fatalf("line %d: run: step script inlines a ${{ }} expression directly — route through env: and reference as a shell variable instead (script-injection risk): %s", lineNo, strings.TrimSpace(line))
	}
}

// TestFindInlinedRunExpressionCatchesSingleLineRunSteps (round-1 review
// WR-01, regression-guard-blind-spot): proves findInlinedRunExpression
// (and, transitively, TestReleaseWorkflowNeverInlinesExpressionsIntoRunScripts)
// actually detects the vulnerability class it exists to guard against —
// including the single-line `run:` shape the pre-fix loop never scanned.
// Before the WR-01 fix, the single-line-inlined-expression case here
// reported found=false (a false PASS on a live script-injection shape);
// after the fix it reports found=true.
func TestFindInlinedRunExpressionCatchesSingleLineRunSteps(t *testing.T) {
	cases := []struct {
		name      string
		src       string
		wantFound bool
	}{
		{
			name: "single-line run: step inlines an expression directly (WR-01's exact reproduction)",
			src: "jobs:\n" +
				"  release:\n" +
				"    steps:\n" +
				"      - name: make release\n" +
				"        run: make release VERSION=${{ steps.relver.outputs.version }}\n",
			wantFound: true,
		},
		{
			name: "single-line run: step with no expression is clean",
			src: "jobs:\n" +
				"  release:\n" +
				"    steps:\n" +
				"      - name: make test\n" +
				"        run: make test\n",
			wantFound: false,
		},
		{
			name: "run: | block-scalar continuation line inlines an expression",
			src: "jobs:\n" +
				"  release:\n" +
				"    steps:\n" +
				"      - name: make release\n" +
				"        run: |\n" +
				"          make release VERSION=${{ steps.relver.outputs.version }}\n",
			wantFound: true,
		},
		{
			name: "run: | block-scalar continuation lines are clean (values routed through env:)",
			src: "jobs:\n" +
				"  release:\n" +
				"    steps:\n" +
				"      - name: make release\n" +
				"        run: |\n" +
				"          make release VERSION=\"$VERSION\"\n" +
				"        env:\n" +
				"          VERSION: ${{ steps.relver.outputs.version }}\n",
			wantFound: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, found := findInlinedRunExpression(tc.src)
			if found != tc.wantFound {
				t.Fatalf("findInlinedRunExpression(%q) found = %v, want %v", tc.src, found, tc.wantFound)
			}
		})
	}
}

func TestReleaseWorkflowIsTagScoped(t *testing.T) {
	src := readRepoFile(t, releaseWorkflowPath(t))
	if !strings.Contains(src, "tags: ['v*']") && !strings.Contains(src, `tags: ["v*"]`) {
		t.Fatal("release.yml on: push: is missing a quoted tags glob 'v*'")
	}
}

// permissionsBlockEntries extracts a job block's `permissions:` mapping
// entries — the lines strictly indented deeper than the `permissions:` key
// itself, up to (not including) the first line at or below that key's own
// indent (the next same-or-lower-indent key, e.g. `steps:`). Returns the
// trimmed entry lines in file order. Used to assert the block contains
// EXACTLY the expected permission set, not merely that the expected keys
// are present among possibly more (round-1 review WR-03).
func permissionsBlockEntries(t *testing.T, block string) []string {
	t.Helper()
	lines := strings.Split(block, "\n")
	permRe := regexp.MustCompile(`^(\s*)permissions:\s*$`)
	start := -1
	permIndent := -1
	for i, l := range lines {
		if m := permRe.FindStringSubmatch(l); m != nil {
			start = i
			permIndent = len(m[1])
			break
		}
	}
	if start < 0 {
		t.Fatal("permissions: key not found in job block")
	}
	entryRe := regexp.MustCompile(`^\s+\w[\w-]*:\s`)
	var entries []string
	for i := start + 1; i < len(lines); i++ {
		l := lines[i]
		trimmed := strings.TrimSpace(l)
		if trimmed == "" {
			continue
		}
		indent := len(l) - len(strings.TrimLeft(l, " "))
		if indent <= permIndent {
			break
		}
		if entryRe.MatchString(l) {
			entries = append(entries, trimmed)
		}
	}
	return entries
}

// TestPermissionsBlockEntriesDetectsExtraPermission (round-1 review WR-03,
// test-name-overstates-what-it-checks): the pre-fix
// TestReleaseWorkflowJobHasExactlyThreeScopedPermissions only asserted the
// 3 expected permission strings were PRESENT in the job block — a
// substring `strings.Contains` check does not notice a 4th, unexpected
// permission key coexisting alongside the 3 expected ones. This proves
// permissionsBlockEntries (and therefore the real test, once it asserts
// len(entries) == 3) actually notices the extra key: a synthetic block
// with a 4th permission (`actions: write`) added on top of the 3 expected
// ones must report 4 entries, not silently pass as "the 3 expected keys
// are all here".
func TestPermissionsBlockEntriesDetectsExtraPermission(t *testing.T) {
	cases := []struct {
		name        string
		block       string
		wantEntries []string
	}{
		{
			name: "exactly the 3 expected permissions",
			block: "  release:\n" +
				"    permissions:\n" +
				"      contents: write\n" +
				"      id-token: write\n" +
				"      attestations: write\n" +
				"    steps:\n" +
				"      - name: noop\n",
			wantEntries: []string{"contents: write", "id-token: write", "attestations: write"},
		},
		{
			name: "a 4th unexpected permission is added on top of the 3 expected ones",
			block: "  release:\n" +
				"    permissions:\n" +
				"      contents: write\n" +
				"      id-token: write\n" +
				"      attestations: write\n" +
				"      actions: write\n" +
				"    steps:\n" +
				"      - name: noop\n",
			wantEntries: []string{"contents: write", "id-token: write", "attestations: write", "actions: write"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := permissionsBlockEntries(t, tc.block)
			if len(got) != len(tc.wantEntries) {
				t.Fatalf("permissionsBlockEntries = %v (%d entries), want %v (%d entries)", got, len(got), tc.wantEntries, len(tc.wantEntries))
			}
			for i, want := range tc.wantEntries {
				if got[i] != want {
					t.Fatalf("permissionsBlockEntries[%d] = %q, want %q (full: %v)", i, got[i], want, got)
				}
			}
			// Reproduces the PRE-FIX test's own presence-only logic to
			// demonstrate the blind spot directly: even the 4-permission
			// case satisfies a "these 3 substrings are present" check,
			// which is exactly why WR-03 flagged the test's name
			// ("exactly three") as overstating what it verifies.
			want3 := []string{"contents: write", "id-token: write", "attestations: write"}
			presenceOnlyOK := true
			for _, w := range want3 {
				if !strings.Contains(tc.block, w) {
					presenceOnlyOK = false
				}
			}
			if !presenceOnlyOK {
				t.Fatalf("presence-only check unexpectedly failed for %q", tc.name)
			}
		})
	}
}

// TestReleaseWorkflowJobHasExactlyThreeScopedPermissions is a security
// regression guard: the release job carries elevated, secret-bearing
// permissions (contents:write, id-token:write, attestations:write). Its
// name promises EXACTLY three scoped permissions — WR-03 (round-1 review)
// found the body only asserted the 3 expected keys were present, never
// that there were no MORE than 3. A future PR silently widening this
// job's blast radius (e.g. adding actions: write) would have passed this
// guard. Assert both presence of the 3 expected keys AND that the
// permissions: mapping has exactly 3 entries — nothing else.
func TestReleaseWorkflowJobHasExactlyThreeScopedPermissions(t *testing.T) {
	src := readRepoFile(t, releaseWorkflowPath(t))
	block := jobBlock(t, src, "release")
	if !regexp.MustCompile(`(?m)^\s+permissions:\s*$`).MatchString(block) {
		t.Fatal("release job missing permissions:")
	}
	want := []string{"contents: write", "id-token: write", "attestations: write"}
	for _, w := range want {
		if !strings.Contains(block, w) {
			t.Fatalf("release job permissions missing %q", w)
		}
	}
	entries := permissionsBlockEntries(t, block)
	if len(entries) != len(want) {
		t.Fatalf("release job permissions: has %d entries %v, want exactly %d %v (WR-03: a name promising \"exactly three\" must assert absence of extras too)", len(entries), entries, len(want), want)
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
