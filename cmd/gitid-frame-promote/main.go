// gitid-frame-promote is the ONE reproducible command (09-07-PLAN.md Task 1,
// review R14; parameterised by phase per 09.5-05-PLAN.md Task 1's D-K) that
// promotes a phase's PTY frames from the gitignored capture directory
// (tmp/ui-frames/, e2e/ui_pty_e2e_test.go's saveFrame) into that phase's
// tracked approved-baseline directory
// (.planning/phases/<phase-dir>/ui-frames/), recording machine-checked
// provenance for each promoted frame so a later frame from a DIFFERENT run
// can never be mistaken for the reviewed one.
//
// Run the PTY suite first (captures land in tmp/ui-frames/), then:
//
//	go run ./cmd/gitid-frame-promote               # phase 09 (default, today's Phase 9 behavior)
//	go run ./cmd/gitid-frame-promote -phase 09.5    # Phase 9.5
//
// The -phase flag selects the phase identifier to promote (see the
// phaseFrames registry below for known identifiers); omitting it resolves to
// defaultPhase ("09"), byte-identical to this tool's pre-parameterisation
// behavior. An unknown phase identifier is refused by name — nothing is
// written to any directory for it.
//
// Every promoted frame's row in ui-frames/README.md records: the state
// ID, the frame filename, the producing test function, the shim mode it was
// captured under, the fixed capture geometry, the source commit
// (`git rev-parse HEAD`), and the SHA-256 of the frame's content.
// TestUploadFrameProvenanceMatches (cmd/gitid/gate_visual_regression_test.go)
// verifies every row against the committed frame's real hash in both
// directions.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// promotionEntry pairs one captured frame with the provenance facts the
// promoted PROVENANCE.md row must record — the registry-to-frame mapping
// the plan requires stays explicit and reviewable here, rather than
// inferred from filenames.
type promotionEntry struct {
	stateID  string
	frame    string // filename in tmp/ui-frames/, without the .txt suffix
	test     string
	shimMode string
	geometry string
}

// phase9Frames is the fixed set of Phase 9 upload-surface frames this tool
// promotes — 09-07-PLAN.md Task 1's twelve new PTY tests plus the two
// pre-existing plan-09-02 tracer tests that already cover an Approved Base
// State row.
var phase9Frames = []promotionEntry{
	{"upload-checkbox-ready", "create-flow-upload-autonomous-github", "TestCreateFlow_UploadAutonomousGitHubTracer", "gh ok", "100x30"},
	{"upload-checkbox-unauth", "create-flow-upload-checkbox-unauth", "TestCreateFlow_UploadCheckboxUnauthState", "gh auth-fail", "100x30"},
	{"upload-checkbox-disabled", "create-flow-upload-checkbox-disabled", "TestCreateFlow_UploadCheckboxDisabledState", "deny shim (no gh)", "100x30"},
	{"upload-checkbox-tab-and-click", "create-flow-upload-checkbox-tab-and-click", "TestCreateFlow_UploadCheckboxTabAndClickReachable", "gh ok", "100x30"},
	{"upload-manual-fallback", "create-flow-upload-partial-scope", "TestCreateFlow_UploadPartialScopeShowsBothRows", "gh scope-fail-signing", "100x30"},
	{"upload-already-complete", "create-flow-upload-already-complete", "TestCreateFlow_UploadAlreadyCompleteCollapsesToOneLine", "gh inventory-both", "100x30"},
	{"upload-omitted", "create-flow-reachable-not-uploaded-evidence", "TestCreateFlow_ReachableNotUploadedEvidence", "fake ssh denied, no gh", "100x30"},
	{"register-key-modal", "identity-manager-register-key-modal-runs", "TestIdentityManager_RegisterKeyModalRuns", "gh ok", "100x30"},
	{"register-key-modal-manual-fallback", "identity-manager-register-key-modal-manual-fallback", "TestIdentityManager_RegisterKeyModalManualFallback", "gh auth-fail", "100x30"},
	{"register-key-modal-u-key", "identity-manager-register-key-modal-u-key", "TestIdentityManager_RegisterKeyModalOpensWithU", "gh ok", "100x30"},
	{"rotate-delete-offer-default", "identity-manager-rotate-delete-offer-default", "TestIdentityManager_RotateDeleteOfferDefaultsToLeave", "gh delete-ok + inventory", "100x30"},
	{"rotate-delete-offer-delete", "identity-manager-rotate-delete-offer-delete", "TestIdentityManager_RotateDeleteOfferDeletesOnExplicitChoice", "gh delete-ok + inventory", "100x30"},
	{"rotate-delete-offer-absent", "identity-manager-rotate-delete-offer-absent", "TestIdentityManager_RotateDeleteOfferAbsentWhenInventoryFails", "gh inventory-fail", "100x30"},
}

// phase95Frames is the fixed set of Phase 9.5 Global SSH/Git properties-
// browser frames this tool promotes — the seventeen PTY captures the four
// 09.5-0{1,2,3,4} plans' real-terminal tests produce (PROP-01..04), read from
// each plan's own SUMMARY.md rather than guessed. Two tests each capture two
// frames (a before/after or narrowed/digit-captured pair); every other test
// captures one.
var phase95Frames = []promotionEntry{
	// 09.5-01 (PROP-01): the "All directives" browser tracer + its filter,
	// probe-failure, and mouse-click states.
	{"global-ssh-all-directives-browse", "global-ssh-all-directives-browse", "TestGlobalSSH_RealPTYAllDirectivesBrowse", "fake ssh: globalssh", "100x30"},
	{"global-ssh-all-directives-filter-narrowed", "global-ssh-all-directives-filter-narrowed", "TestGlobalSSH_RealPTYAllDirectivesFilter", "fake ssh: globalssh", "100x30"},
	{"global-ssh-all-directives-filter-digit-captured", "global-ssh-all-directives-filter-digit-captured", "TestGlobalSSH_RealPTYAllDirectivesFilter", "fake ssh: globalssh", "100x30"},
	{"global-ssh-all-directives-probe-failure", "global-ssh-all-directives-probe-failure", "TestGlobalSSH_RealPTYAllDirectivesProbeFailure", "fake ssh: globalssh-probe-unresolvable", "100x30"},
	{"global-ssh-all-directives-label-click", "global-ssh-all-directives-label-click", "TestGlobalSSH_RealPTYAllDirectivesLabelMouseClick", "fake ssh: globalssh", "100x30"},
	// 09.5-02 (PROP-02): the "Set keys" browser, its filter, its sub-tab
	// strip mouse click, and its probe-failure state.
	{"global-git-set-keys-browse", "global-git-set-keys-browse", "TestGlobalGit_RealPTYSetKeysBrowse", "real git, no shim", "100x30"},
	{"global-git-set-keys-back-to-options", "global-git-set-keys-back-to-options", "TestGlobalGit_RealPTYSetKeysBrowse", "real git, no shim", "100x30"},
	{"global-git-set-keys-filter-narrowed", "global-git-set-keys-filter-narrowed", "TestGlobalGit_RealPTYSetKeysFilter", "real git, no shim", "100x30"},
	{"global-git-set-keys-filter-digit-captured", "global-git-set-keys-filter-digit-captured", "TestGlobalGit_RealPTYSetKeysFilter", "real git, no shim", "100x30"},
	{"global-git-strip-click-to-set-keys", "global-git-strip-click-to-set-keys", "TestGlobalGit_RealPTYSubTabStripClick", "real git, no shim", "100x30"},
	{"global-git-strip-click-to-options", "global-git-strip-click-to-options", "TestGlobalGit_RealPTYSubTabStripClick", "real git, no shim", "100x30"},
	{"global-git-set-keys-probe-failure", "global-git-set-keys-probe-failure", "TestGlobalGit_RealPTYSetKeysProbeFailure", "fake git 2.50.0 (config probe broken)", "100x30"},
	// 09.5-03 (PROP-03): the free-form custom Git key entry flow.
	{"global-git-custom-key-write", "global-git-custom-key-write", "TestGlobalGit_RealPTYCustomKeyWrite", "real git, no shim", "100x30"},
	{"global-git-custom-key-malformed", "global-git-custom-key-malformed", "TestGlobalGit_RealPTYCustomKeyRejectsMalformedKey", "real git, no shim", "100x30"},
	// 09.5-04 (PROP-04): the custom SSH directive entry flow (stage 2's
	// un-skippable rejection is the phase's focal point).
	{"global-ssh-custom-directive-rejected-name", "global-ssh-custom-directive-rejected-name", "TestGlobalSSH_RealPTYCustomDirectiveRejectedNameNeverWrites", "fake ssh: globalssh", "100x30"},
	{"global-ssh-custom-directive-ceremony", "global-ssh-custom-directive-ceremony", "TestGlobalSSH_RealPTYCustomDirectiveWrite", "fake ssh: globalssh", "100x30"},
	{"global-ssh-custom-directive-write-receipt", "global-ssh-custom-directive-write-receipt", "TestGlobalSSH_RealPTYCustomDirectiveWrite", "fake ssh: globalssh", "100x30"},
}

// defaultPhase is the phase identifier main() resolves to when -phase is not
// given — "09", so the no-argument invocation stays byte-identical to this
// tool's pre-parameterisation behavior (D-K's explicit requirement).
const defaultPhase = "09"

// phaseFrames maps a phase identifier to its tracked ui-frames destination
// (a directory name directly under .planning/phases/) and the frame list to
// promote for it. Add a new phase's entry here rather than writing a second
// promoter — the tool's whole point is one fail-closed, reproducible path.
var phaseFrames = map[string]struct {
	dir     string
	title   string // README.md heading — names THIS phase's surface, not a generic one
	summary string // README.md intro paragraph — must not mislead a reader into another phase's scope
	frames  []promotionEntry
}{
	defaultPhase: {
		dir:   "09-upload-credentials-assist",
		title: "Phase 9 upload-surface approved PTY frames",
		summary: "These are D-09's fresh approved captures for exactly the amended upload\n" +
			"screens — the Phase 9 visual-regression baseline every gate-visual-regression\n" +
			"run compares the real binary against.",
		frames: phase9Frames,
	},
	"09.5": {
		dir:   "09.5-full-ssh-git-properties-browser",
		title: "Phase 9.5 Global SSH/Git properties-browser approved PTY frames",
		summary: "These are PROP-01..04's approved captures for the \"All directives\" browser,\n" +
			"the custom-directive entry flow, the \"Set keys\" browser (with its net-new\n" +
			"sub-tab strip), and the custom Git key entry flow.",
		frames: phase95Frames,
	},
}

// resolvePhase looks up phase's destination directory name and frame list.
// An unknown phase is refused BY NAME — the caller never gets a directory or
// frame list to write with, so nothing downstream can write anywhere.
func resolvePhase(phase string) (dir string, frames []promotionEntry, err error) {
	entry, ok := phaseFrames[phase]
	if !ok {
		known := make([]string, 0, len(phaseFrames))
		for id := range phaseFrames {
			known = append(known, id)
		}
		sort.Strings(known)
		return "", nil, fmt.Errorf("unknown phase %q (known phases: %s)", phase, strings.Join(known, ", "))
	}
	return entry.dir, entry.frames, nil
}

// resolveProvenanceText looks up phase's README.md title and intro summary
// — a small companion to resolvePhase (kept separate so resolvePhase's
// signature, already covered by TestPromoteFramesPhase9DefaultUnchanged,
// never has to change again just because a phase's prose changes).
func resolveProvenanceText(phase string) (title, summary string, err error) {
	entry, ok := phaseFrames[phase]
	if !ok {
		return "", "", fmt.Errorf("unknown phase %q", phase)
	}
	return entry.title, entry.summary, nil
}

func main() {
	phase := flag.String("phase", defaultPhase, "phase identifier to promote frames for (see phaseFrames in main.go for known identifiers)")
	flag.Parse()
	if err := run(*phase); err != nil {
		fmt.Fprintf(os.Stderr, "gitid-frame-promote: %v\n", err)
		os.Exit(1)
	}
}

func run(phase string) error {
	root, err := repoRoot()
	if err != nil {
		return fmt.Errorf("resolving repo root: %w", err)
	}
	commit, err := gitHead(root)
	if err != nil {
		return fmt.Errorf("resolving source commit: %w", err)
	}
	srcDir := filepath.Join(root, "tmp", "ui-frames")
	planningPhasesRoot := filepath.Join(root, ".planning", "phases")

	dstDir, rows, err := promoteForPhase(srcDir, planningPhasesRoot, phase, commit, func(format string, a ...any) { fmt.Printf(format, a...) })
	if err != nil {
		return err
	}

	title, summary, err := resolveProvenanceText(phase)
	if err != nil {
		return err
	}
	if err := writeProvenance(filepath.Join(dstDir, "README.md"), title, summary, rows); err != nil {
		return fmt.Errorf("writing PROVENANCE table: %w", err)
	}
	fmt.Printf("promoted %d frame(s) for phase %s; provenance written to %s\n", len(rows), phase, filepath.Join(dstDir, "README.md"))
	return nil
}

// promoteForPhase resolves phase to its destination directory (under
// planningPhasesRoot) and frame list, then promotes via promoteFrames. It is
// the same orchestration run() uses against a real repo, factored out so
// tests can drive it against a scratch directory tree instead. On any error
// (including an unknown phase) it returns "" / nil / the error, having
// written nothing.
func promoteForPhase(srcDir, planningPhasesRoot, phase, commit string, progress func(format string, a ...any)) (dstDir string, rows []string, err error) {
	dir, frames, err := resolvePhase(phase)
	if err != nil {
		return "", nil, err
	}
	dstDir = filepath.Join(planningPhasesRoot, dir, "ui-frames")
	if err := os.MkdirAll(dstDir, 0o750); err != nil {
		return "", nil, fmt.Errorf("creating %s: %w", dstDir, err)
	}
	rows, err = promoteFrames(srcDir, dstDir, frames, commit, progress)
	if err != nil {
		return "", nil, err
	}
	return dstDir, rows, nil
}

// promoteFrames copies every entry's captured frame from srcDir into dstDir
// and returns each promoted row for the PROVENANCE table, or an error
// naming every missing capture. progress is called once per promoted frame
// (nil is fine — tests pass nil to stay silent).
//
// WR-13: read and validate EVERY source frame before writing anything. The
// old version wrote each tracked baseline as it went and only checked for
// missing captures after the loop finished — so a partial capture run (one
// PTY test skipped, one source frame absent) rewrote SOME approved
// baselines while leaving the rest at their previous commit's content, and
// the caller never regenerated README.md at all: a baseline directory whose
// frames and provenance table disagree, and whose non-rewritten frames can
// be mistaken for having come from THIS run. Nothing is written to dstDir
// until every entry has a real source file in srcDir.
func promoteFrames(srcDir, dstDir string, frames []promotionEntry, commit string, progress func(format string, a ...any)) ([]string, error) {
	entries := append([]promotionEntry(nil), frames...)
	sort.Slice(entries, func(i, j int) bool { return entries[i].stateID < entries[j].stateID })

	contents := make(map[string][]byte, len(entries))
	var missing []string
	for _, e := range entries {
		srcPath := filepath.Join(srcDir, e.frame+".txt")
		content, err := os.ReadFile(srcPath) //nolint:gosec // fixed, tool-owned path
		if err != nil {
			missing = append(missing, e.frame)
			continue
		}
		contents[e.frame] = content
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing captures in %s (run the PTY suite first): %v", srcDir, missing)
	}

	var rows []string
	for _, e := range entries {
		content := contents[e.frame]
		dstPath := filepath.Join(dstDir, e.frame+".txt")
		if err := os.WriteFile(dstPath, content, 0o644); err != nil { //nolint:gosec // tracked repo file, not a secret
			return nil, fmt.Errorf("writing %s: %w", dstPath, err)
		}
		sum := sha256.Sum256(content)
		rows = append(rows, fmt.Sprintf("| %s | %s.txt | %s | %s | %s | %s | %s |",
			e.stateID, e.frame, e.test, e.shimMode, e.geometry, commit, hex.EncodeToString(sum[:])))
		if progress != nil {
			progress("promoted %-45s <- tmp/ui-frames/%s.txt\n", e.frame+".txt", e.frame)
		}
	}

	// WR-05 (09.5-REVIEW.md round 3): remove any .txt in dstDir that is NOT
	// in this run's frame registry — a renamed or removed registry entry
	// otherwise leaves an orphan tracked frame behind, with no provenance
	// row naming it, contradicting writeProvenance's own claim that
	// re-running "overwrites this table and every frame in this directory".
	// Deliberately placed AFTER the all-present check and every write above
	// (WR-13's own invariant): a partial capture run must still leave dstDir
	// — including any pre-existing stale frame — completely untouched.
	wanted := make(map[string]bool, len(entries))
	for _, e := range entries {
		wanted[e.frame+".txt"] = true
	}
	dstEntries, err := os.ReadDir(dstDir)
	if err != nil {
		return nil, fmt.Errorf("reading %s to prune stale frames: %w", dstDir, err)
	}
	for _, de := range dstEntries {
		name := de.Name()
		if de.IsDir() || !strings.HasSuffix(name, ".txt") || wanted[name] {
			continue
		}
		stalePath := filepath.Join(dstDir, name)
		if err := os.Remove(stalePath); err != nil {
			return nil, fmt.Errorf("removing stale frame %s: %w", stalePath, err)
		}
		if progress != nil {
			progress("removed  %-45s (no longer in the registry)\n", name)
		}
	}

	return rows, nil
}

// writeProvenance renders a phase's README.md: title and summary come from
// that phase's phaseFrames registry entry (resolveProvenanceText) — never a
// hardcoded Phase 9 string — so a later phase's promoted directory is never
// mislabeled with Phase 9's own prose (found while reviewing 09.5-05-PLAN.md
// Task 2's promoted output: the pre-parameterisation version of this
// function always wrote the Phase 9 heading regardless of which phase was
// actually promoted).
func writeProvenance(path, title, summary string, rows []string) error {
	var b strings.Builder
	b.WriteString("# " + title + "\n\n")
	b.WriteString(summary + " Every frame is captured at the fixed\n")
	b.WriteString("100x30 geometry, driven by a real PTY session with raw keystrokes against the\n")
	b.WriteString("compiled `gitid` binary (never a unit/wiring-test substitute — ONESHOT.md rule 8).\n\n")
	b.WriteString("Promoted by `go run ./cmd/gitid-frame-promote` — never hand-copied. Re-run that\n")
	b.WriteString("command after any PTY test change; it overwrites this table and every frame in\n")
	b.WriteString("this directory from a fresh `tmp/ui-frames/` capture.\n\n")
	b.WriteString("## Provenance\n\n")
	b.WriteString("| State ID | Frame | Producing test | Shim mode | Geometry | Source commit | SHA-256 |\n")
	b.WriteString("|---|---|---|---|---|---|---|\n")
	for _, r := range rows {
		b.WriteString(r + "\n")
	}
	return os.WriteFile(path, []byte(b.String()), 0o644) //nolint:gosec // tracked repo file, not a secret
}

func repoRoot() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func gitHead(root string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
