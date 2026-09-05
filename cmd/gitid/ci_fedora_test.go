package main

import (
	"regexp"
	"strings"
	"testing"
)

// TestCIWorkflowNeverInlinesExpressionsIntoRunScripts (round-1 review
// IN-01): the ci-cd-script-injection regression guard
// (TestReleaseWorkflowNeverInlinesExpressionsIntoRunScripts,
// release_yml_test.go) existed only for release.yml. ci.yml's fedora job
// runs several run: steps too (dnf install, git config --global --add
// safe.directory, make test, make lint, make test-e2e) and none inline a
// ${{ }} expression today — the blast radius here is smaller than
// release.yml's (ci.yml's top-level permissions stay contents: read, no
// secret referenced anywhere in the file), which is why this was filed as
// Info rather than Warning — but the guard is a one-line generalization
// of an already-written one. Reuses findInlinedRunExpression
// (release_yml_test.go, same package) rather than duplicating the scan
// logic.
func TestCIWorkflowNeverInlinesExpressionsIntoRunScripts(t *testing.T) {
	src := readRepoFile(t, workflowPath(t))
	if lineNo, line, found := findInlinedRunExpression(src); found {
		t.Fatalf("line %d: run: step script inlines a ${{ }} expression directly — route through env: and reference as a shell variable instead (script-injection risk): %s", lineNo, strings.TrimSpace(line))
	}
}

// TestFedoraJobExists asserts ci.yml's PLAT-03 fedora container job block is
// present. workflowPath/jobBlock/readRepoFile are defined in
// release_plumbing_test.go (same package, no import needed).
func TestFedoraJobExists(t *testing.T) {
	src := readRepoFile(t, workflowPath(t))
	block := jobBlock(t, src, "fedora")
	if block == "" {
		t.Fatal("fedora job block is empty")
	}
}

// TestFedoraJobRunsOnlyOnPush asserts D-02's cost-tier cadence: the fedora
// container job runs on push-to-main and release tags only, never on every
// pull_request.
func TestFedoraJobRunsOnlyOnPush(t *testing.T) {
	src := readRepoFile(t, workflowPath(t))
	block := jobBlock(t, src, "fedora")
	re := regexp.MustCompile(`(?m)^\s+if:\s+github\.event_name == 'push'\s*$`)
	if !re.MatchString(block) {
		t.Fatal(`fedora job missing exact if: github.event_name == 'push'`)
	}
}

// TestFedoraJobUsesFedoraLatestContainer asserts the job runs inside a
// fedora:latest container (D-01) rather than a nonexistent
// runs-on: fedora-latest runner label.
func TestFedoraJobUsesFedoraLatestContainer(t *testing.T) {
	src := readRepoFile(t, workflowPath(t))
	block := jobBlock(t, src, "fedora")
	if !regexp.MustCompile(`(?m)^\s+image:\s+fedora:latest\s*$`).MatchString(block) {
		t.Fatal("fedora job container.image is not fedora:latest")
	}
}

// TestFedoraJobDnfInstallIsFirstStep asserts the dnf install step is the
// VERY FIRST step in the job, before any `uses:` action — fedora:latest ships
// no Node.js, and every first-party GitHub Action (including
// actions/checkout itself) needs Node.js to execute at all inside the
// container (10-RESEARCH.md Pattern 2 / Pitfall 3).
func TestFedoraJobDnfInstallIsFirstStep(t *testing.T) {
	src := readRepoFile(t, workflowPath(t))
	block := jobBlock(t, src, "fedora")
	idx := strings.Index(block, "steps:")
	if idx < 0 {
		t.Fatal("fedora job missing steps:")
	}
	rest := block[idx+len("steps:"):]
	var firstAction string
	for _, line := range strings.Split(rest, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "run:") || strings.HasPrefix(trimmed, "uses:") {
			firstAction = trimmed
			break
		}
	}
	if firstAction == "" {
		t.Fatal("fedora job has no run:/uses: steps")
	}
	if !strings.HasPrefix(firstAction, "run:") || !strings.Contains(firstAction, "dnf install") {
		t.Fatalf("fedora job's first step must be the dnf install run: step, got: %s", firstAction)
	}
}

// TestFedoraJobSafeDirectoryBeforeCheckout asserts REVIEW C-6's fix: the
// container runs as root against a checkout directory the HOST runner's
// user owns (a UID mismatch that trips git's "detected dubious ownership"
// refusal), so `git config --global --add safe.directory` must land BEFORE
// actions/checkout runs.
func TestFedoraJobSafeDirectoryBeforeCheckout(t *testing.T) {
	src := readRepoFile(t, workflowPath(t))
	block := jobBlock(t, src, "fedora")
	safeDirIdx := strings.Index(block, "git config --global --add safe.directory")
	checkoutIdx := strings.Index(block, "actions/checkout@")
	if safeDirIdx < 0 {
		t.Fatal("fedora job missing git config --global --add safe.directory step")
	}
	if checkoutIdx < 0 {
		t.Fatal("fedora job missing actions/checkout step")
	}
	if safeDirIdx > checkoutIdx {
		t.Fatal("fedora job's safe.directory step must come BEFORE actions/checkout (REVIEW C-6)")
	}
}

// TestFedoraJobCheckoutFetchesFullTagHistory asserts REVIEW C-6's second
// fix: the default shallow fetch-depth: 1 fetches no tags, so `git describe
// --tags` silently falls to a bare-SHA form. fetch-depth: 0 keeps the job's
// own VERSION stamp meaningful CI signal, matching D-10's guardrail.
func TestFedoraJobCheckoutFetchesFullTagHistory(t *testing.T) {
	src := readRepoFile(t, workflowPath(t))
	block := jobBlock(t, src, "fedora")
	checkoutIdx := strings.Index(block, "actions/checkout@")
	if checkoutIdx < 0 {
		t.Fatal("fedora job missing actions/checkout step")
	}
	rest := block[checkoutIdx:]
	// Bound the search to this step's own body: stop at the next "- name:"
	// step marker so a later, unrelated fetch-depth (there is none today,
	// but this keeps the assertion scoped to the checkout step specifically).
	if end := strings.Index(rest[1:], "\n      - name:"); end >= 0 {
		rest = rest[:end+1]
	}
	if !regexp.MustCompile(`fetch-depth:\s*0`).MatchString(rest) {
		t.Fatal("fedora job's checkout step is missing fetch-depth: 0 (REVIEW C-6)")
	}
}

// TestFedoraJobDeclaresNoPermissions asserts the fedora job needs no
// permissions: key of its own, inheriting the workflow's least-privilege
// top-level contents: read — matching the undeclared-permissions precedent
// build-cross/check already establish.
func TestFedoraJobDeclaresNoPermissions(t *testing.T) {
	src := readRepoFile(t, workflowPath(t))
	block := jobBlock(t, src, "fedora")
	if strings.Contains(block, "permissions:") {
		t.Fatal("fedora job must not declare a permissions: key")
	}
}

// TestFedoraJobRunsTheFullAutomatedSuite asserts the job invokes the exact
// same make test/make lint/make test-e2e targets every other job invokes —
// no divergent CI invocation path (must_haves key_links).
func TestFedoraJobRunsTheFullAutomatedSuite(t *testing.T) {
	src := readRepoFile(t, workflowPath(t))
	block := jobBlock(t, src, "fedora")
	for _, target := range []string{"run: make test", "run: make lint", "run: make test-e2e"} {
		if !strings.Contains(block, target) {
			t.Fatalf("fedora job missing %q as a separate run step", target)
		}
	}
}
