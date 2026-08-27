//go:build screenshot

package screenshot

// createflow.go drives the shared internal/tuikit render stack (either
// binary's Backend injected) through a FIXED script and captures the
// rendered View().Content text at explicitly enumerated create-flow screen
// checkpoints — the "capture" half of the DLV-04.1/D-24.1 visual-regression
// gate (plan 03-06, Task 2).
//
// The SAME script drives both cmd/gitid-dummy's FixtureBackend and the real
// cmd/gitid Backend, so the two capture sets are directly diffable, screen
// by screen, modulo the documented divergence allowlist
// (.planning/design/create-flow/visual-divergence-allowlist.txt). No PTY,
// no subprocess: tuikit.App.Update/.View are driven in-process via
// synthesized tea.Msg values — the SAME technique internal/tuikit's own
// test suite already uses (see internal/tuikit/identities_test.go), safe
// here because this gate is about RENDERED TEXT equivalence, not raw
// keystroke/terminal-decoding correctness (that is DLV-06's PTY e2e job,
// plan 03-06 Task 1).
//
// CaptureWidth/CaptureHeight match screenshot-tui's own fixed geometry
// (D-04) so a future PNG-based capture of these same screens stays
// apples-to-apples; this gate itself only needs the geometry (pane-width
// wrapping affects the TEXT), not the vendored font/theme (which only
// affects PNG pixel rendering, not text content).

import (
	"fmt"
	"regexp"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/castocolina/gitid/internal/tuikit"
)

// timestampPattern matches ISO-8601-like timestamps embedded in backup file names,
// covering both colon-separated ("T20:39:15Z") and dash-separated ("T20-39-15Z")
// formats — tuikit.NewBackupPath replaces colons with dashes for filesystem
// compatibility. The full patterns matched are:
//   - YYYY-MM-DDTHH:MM:SSZ (standard ISO 8601)
//   - YYYY-MM-DDTHH-MM-SSZ (colon-replaced, tuikit.NewBackupPath format)
var timestampPattern = regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}[:\-]\d{2}[:\-]\d{2}Z`)

var sandboxPathFragmentPattern = regexp.MustCompile(`(?:gitid-evidence-)?capture-\d+|fake-ssh-\d+|(?:gi)?tid-stage-\d+`)

// backupSuffixPattern matches internal/filewriter's ".bak.<unix-nanoseconds>"
// backup-path suffix (filewriter.go's targetPath+".bak."+UnixNano()) —
// precisely, anchored to the literal ".bak." prefix (captured and
// preserved in the replacement — only the digits themselves are
// normalized) — the REAL Git-screen commit receipt's actual backup naming
// (04-04-PLAN.md Task 3), distinct from tuikit.NewBackupPath's ISO-8601
// ".backup." convention timestampPattern already normalizes. This is the
// PRIMARY match: it covers every occurrence that did not get split by a
// line wrap.
var backupSuffixPattern = regexp.MustCompile(`(\.bak\.)\d+`)

// wrappedDigitRowPattern is WR-09's narrowed fallback for the ONE case
// backupSuffixPattern cannot reach: a 19-digit nanosecond value can wrap
// mid-number across the fixed 100-column pane, splitting into a SECOND,
// independent digit run that carries no ".bak." prefix of its own (it
// continues on the next rendered row). Anchoring to "this row's entire
// content — after the pane border and padding — is a run of 6+ digits"
// (rather than matching ANY 6+ digit run anywhere in the frame) means an
// inline byte count, key size, or future numeric ID embedded alongside
// other text on the same row is NEVER matched: a genuine wrapped
// continuation, by construction, has nothing else on its row. The border
// and padding (captured, not consumed) are preserved verbatim in the
// replacement — only the digit run itself is normalized.
var wrappedDigitRowPattern = regexp.MustCompile(`(?m)^([^\S\n]*(?:[│|][^\S\n]*)?)(\d{6,})([^\S\n]*)$`)

// normalizeTimestamps replaces all ISO-8601 timestamps and nanosecond-
// suffixed backup paths (including line-wrapped fragments) in s with fixed
// placeholders so that captures taken at different wall-clock instants are
// byte-identical (CR-01).
func normalizeTimestamps(s string) string {
	s = sandboxPathFragmentPattern.ReplaceAllString(s, "<sandbox>")
	s = backupSuffixPattern.ReplaceAllString(s, "${1}<digits>")
	s = wrappedDigitRowPattern.ReplaceAllString(s, "${1}<digits>${3}")
	return timestampPattern.ReplaceAllString(s, "<timestamp>")
}

// CaptureWidth/CaptureHeight are the D-04/D-24 fixed capture geometry —
// the SAME 100x30 values screenshot-tui (internal/screenshot/tui.go,
// tui_capture_test.go) already uses.
const (
	CaptureWidth  = 100
	CaptureHeight = 30
)

// CreateFlowScreenIDs is retained for source compatibility only. It is not an
// inventory or completeness oracle; RequiredScreenSpecs is authoritative.
var CreateFlowScreenIDs = []string{
	"ssh-form-filled",       // step 0: the default-filled SSH form + live Host-block preview
	"reuse-key-vs-generate", // step 0: D-10 picker, populated (backend.ScanReusableKeys())
	"reuse-manual-path",     // step 0: D-10 picker's trailing manual-path row selected
	"mouse-focused-field",   // step 0: a field focused via a REAL synthesized mouse click
	"test-stage1-direct",    // step 1: stage 1 (TEST-01) outcome, captured at testRunning2
	"test-stage2-by-alias",  // step 1: stage 2 (TEST-02) outcome, captured at testStage2
	"git-form-demo",         // step 2: the demo'd Git-identity step (D-18/D-19)
	"confirm-write",         // step 3: the review/confirm-write ceremony (state A)
}

// ---------------------------------------------------------------------------
// ScreenSpec registry — typed per-screen capture contract (03-13 Task 2).
//
// Each ScreenSpec names a logical screen ID, its canonical HTML route, the
// interaction script that produces the state, the unique state marker that
// must appear in the captured text before the label is accepted, which
// surfaces the spec applies to, and optional same-route variant metadata.
//
// ValidateCapturedState MUST be called before saving any capture — a
// matching route without the state marker fails; a marker from another
// route/state fails even when hashes happen to be unique.
// ---------------------------------------------------------------------------

// SurfaceNonApplicability records why an approval surface cannot truthfully
// render a Phase 3 state and the decision that introduced that state.
type SurfaceNonApplicability struct {
	Surface        string
	Decision       string
	Reason         string
	Classification string
}

// RegionDisposition explicitly authorizes one comparable live/approved-TUI
// difference on one ScreenSpec. A RegionName alone never grants approval.
//
// Predicate is WR-19's fix: RequiredRegions is presence-only (it does not
// gate comparison — BuildRegionDiffs iterates AllRegionNames() regardless),
// so without a text predicate a disposition accepts ANY future divergence in
// that region, not just the one it was written to describe. Predicate is
// OPTIONAL — empty preserves today's blanket-acceptance behavior exactly —
// but when set it must be "contains:<text>" or "absent:<text>" (the SAME
// grammar the e2e git-screen allowlist already parses/enforces via
// gitScreenPredicateSatisfied in e2e/git_configuration_pty_e2e_test.go), and
// BuildRegionDiffs rejects a differing region whose live/approved text does
// not satisfy it.
type RegionDisposition struct {
	Region         RegionName
	Divergence     string
	Decision       string
	Reason         string
	Classification string
	Predicate      string
}

// ScreenSpec is the typed capture contract for one create-flow logical screen.
type ScreenSpec struct {
	// ScreenID is the registry's logical identifier.
	ScreenID string
	// Route is the canonical HTML route for this screen (e.g. "/create-flow/ssh-form-filled").
	Route string
	// Interaction is a human-readable description of the script steps that produce this state.
	Interaction string
	// StateMarker is the unique string that MUST appear in the captured text to
	// confirm the correct state was captured. A route name alone is never sufficient.
	StateMarker string
	// StateMarkers lists every exact byte sequence required in the saved frame.
	// StateMarker remains the compatibility shorthand for single-marker specs.
	StateMarkers []string
	// ApplicableLive reports whether this spec applies to the live binary capture.
	ApplicableLive bool
	// ApplicableApprovedTUI reports whether this spec applies to the approved-TUI capture.
	ApplicableApprovedTUI bool
	// ApplicableApprovedHTML reports whether this spec applies to the approved-HTML capture.
	ApplicableApprovedHTML bool
	// VariantOf is the base screen ID when this spec is a same-route interaction variant.
	// Must be non-empty together with non-empty VariantRationale to qualify for the
	// same-route hash exemption in duplicate checking.
	VariantOf string
	// VariantRationale documents why this variant legitimately shares the same HTML route.
	VariantRationale string
	// NonApplicability declares every surface that cannot truthfully render this
	// state. Each record must name the governing Phase 3 decision.
	NonApplicability []SurfaceNonApplicability
	// RequiredRegions are the semantic regions that must be present in every
	// applicable capture. They make missing evidence a validation error.
	RequiredRegions []RegionName
	// RegionDispositions declares the only comparable region differences this
	// specific screen accepts. Each entry is tied to a governing decision.
	RegionDispositions []RegionDisposition
}

func uxNonComparable(surface, decision, reason string) SurfaceNonApplicability {
	return SurfaceNonApplicability{
		Surface: surface, Decision: decision, Reason: reason, Classification: "ux-improvement",
	}
}

func uxRegionDifference(region RegionName, divergence, decision, reason string) RegionDisposition {
	return RegionDisposition{
		Region: region, Divergence: divergence, Decision: decision, Reason: reason, Classification: "ux-improvement",
	}
}

// uxRegionDifferenceScoped is uxRegionDifference plus a WR-19 Predicate —
// use this for any NEW disposition that can be scoped to specific text, so
// acceptance narrows to the divergence actually reviewed/approved rather
// than any future difference in that region.
func uxRegionDifferenceScoped(region RegionName, divergence, decision, reason, predicate string) RegionDisposition {
	d := uxRegionDifference(region, divergence, decision, reason)
	d.Predicate = predicate
	return d
}

// validRegionPredicate reports whether predicate is empty (no scoping — WR-19
// keeps this optional) or uses the "contains:"/"absent:" grammar. Mirrors the
// e2e git-screen allowlist's own predicate contract (CR-04: "differs" is
// never a valid predicate — every scoped divergence must name specific text).
func validRegionPredicate(predicate string) bool {
	return predicate == "" || strings.HasPrefix(predicate, "contains:") || strings.HasPrefix(predicate, "absent:")
}

// regionPredicateSatisfied reports whether predicate holds against the
// (live, approved) region text pair.
//
// CR-10 (iteration 4): the prior "hold on EITHER side" grammar
// (`!strings.Contains(live, needle) || !strings.Contains(approved, needle)`)
// was proven vacuous by probe — for every shipped predicate, one side
// structurally never carries the needle, so the OR made the predicate
// permanently true regardless of what the OTHER (real) side rendered. A
// probe with the shipped `absent:"gitdir:~/git/"` predicate accepted an
// arbitrary, unrelated live-side regression ("TOTALLY BROKEN GARBAGE
// OUTPUT") because the frozen dummy side never contains "gitdir:~/git/"
// either way.
//
// The fix expresses the SHAPE of the authorized divergence instead of "one
// side happens to lack the string":
//   - contains:X — the marker must survive on BOTH sides; the difference is
//     authorized to be elsewhere in the region (e.g. a differing count next
//     to a shared "ids" label).
//   - absent:X — the authorized divergence IS the presence/absence
//     asymmetry itself: exactly one side must carry X. Both-present or
//     both-absent is an unreviewed change and must be rejected.
//
// An empty predicate always matches (WR-19: Predicate is optional; empty
// preserves blanket acceptance). Mirrored verbatim in
// e2e/git_configuration_pty_e2e_test.go's gitScreenPredicateSatisfied — keep
// both in sync (see WR-43).
func regionPredicateSatisfied(predicate, live, approved string) bool {
	switch {
	case predicate == "":
		return true
	case strings.HasPrefix(predicate, "contains:"):
		needle := strings.Trim(strings.TrimPrefix(predicate, "contains:"), `"`)
		return strings.Contains(live, needle) && strings.Contains(approved, needle)
	case strings.HasPrefix(predicate, "absent:"):
		needle := strings.Trim(strings.TrimPrefix(predicate, "absent:"), `"`)
		return strings.Contains(live, needle) != strings.Contains(approved, needle)
	}
	return false
}

// ScreenSpecRegistry returns the canonical typed ScreenSpec registry consumed
// by capture, validation, region generation, and publication. Same-route interaction
// variants carry explicit VariantOf + VariantRationale metadata.
func ScreenSpecRegistry() []ScreenSpec {
	specs := []ScreenSpec{
		{
			ScreenID:               "ssh-form-filled",
			Route:                  "/create-flow/ssh-form-filled",
			Interaction:            "Open the create wizard with default prefix 'acme'; form is pre-filled.",
			StateMarker:            "IdentitiesOnly yes",
			ApplicableLive:         true,
			ApplicableApprovedTUI:  true,
			ApplicableApprovedHTML: true,
			RequiredRegions:        []RegionName{RegionFormFields, RegionHostPreview},
			RegionDispositions: []RegionDisposition{
				uxRegionDifference(RegionFormFields, "form-defaults", "D-16", "The live backend uses current provider defaults while the approved TUI preserves frozen demo defaults."),
				uxRegionDifference(RegionKeySection, "key-catalog", "D-16", "The live backend uses its probed key catalog while the approved TUI preserves the frozen fixture order."),
				uxRegionDifference(RegionHostPreview, "host-preview", "D-16", "The live Host block uses the current checked renderer while the approved TUI preserves the frozen fixture rendering."),
				uxRegionDifference(RegionHeaderStatus, "fixture-header-status", "D-16", "The live disposable home starts empty while the approved fixture contains identities."),
				uxRegionDifference(RegionSidebar, "fixture-sidebar", "D-16", "The live disposable home starts empty while the approved fixture contains identities."),
			},
		},
		{
			ScreenID:               "reuse-key-vs-generate",
			Route:                  "/create-flow/reuse-key-vs-generate",
			Interaction:            "Tab to the key-source toggle (4 Tabs), then press Right to select Reuse.",
			StateMarker:            "Reuse an existing key",
			ApplicableLive:         true,
			ApplicableApprovedHTML: true,
			NonApplicability: []SurfaceNonApplicability{uxNonComparable(
				"approved-tui", "D-10", "The approved TUI predates the Phase 3 reusable-key picker.",
			)},
			RequiredRegions: []RegionName{RegionKeySection},
		},
		{
			ScreenID:               "reuse-manual-path",
			Route:                  "/create-flow/reuse-key-vs-generate",
			Interaction:            "From reuse mode, press Left to select the manual-path row; the manual-path text input becomes active.",
			StateMarker:            "Enter a path manually",
			ApplicableLive:         true,
			ApplicableApprovedHTML: true,
			RequiredRegions:        []RegionName{RegionKeySection},
			VariantOf:              "reuse-key-vs-generate",
			VariantRationale:       "No separate HTML route exists for the manual-path interaction variant; it shares /create-flow/reuse-key-vs-generate.",
			NonApplicability: []SurfaceNonApplicability{uxNonComparable(
				"approved-tui", "D-10", "The approved TUI predates the Phase 3 reusable-key picker.",
			)},
		},
		{
			ScreenID:               "mouse-focused-field",
			Route:                  "/create-flow/ssh-form-filled",
			Interaction:            "Click the Port field row with a real mouse event; verify focus moves to Port.",
			StateMarker:            "▸ Port",
			ApplicableLive:         true,
			ApplicableApprovedTUI:  true,
			ApplicableApprovedHTML: true,
			// WR-08: this VariantOf ssh-form-filled captures the SAME pane —
			// only the focus state differs — so it must be gated on the same
			// RegionFormFields/RegionHostPreview the base screen requires,
			// not downgraded to RegionKeybar alone. The RegionDispositions
			// below already declare every accepted divergence (D-16) for
			// these regions; without RequiredRegions listing them, those
			// dispositions were dead metadata and the gate silently stopped
			// failing when the real binary's SSH form fields drifted from
			// the approved design.
			RequiredRegions: []RegionName{RegionKeybar, RegionFormFields, RegionHostPreview},
			RegionDispositions: []RegionDisposition{
				uxRegionDifference(RegionFormFields, "form-defaults", "D-16", "The live backend uses current provider defaults while the approved TUI preserves frozen demo defaults."),
				uxRegionDifference(RegionKeySection, "key-catalog", "D-16", "The live backend uses its probed key catalog while the approved TUI preserves the frozen fixture order."),
				uxRegionDifference(RegionHostPreview, "host-preview", "D-16", "The live Host block uses the current checked renderer while the approved TUI preserves the frozen fixture rendering."),
				uxRegionDifference(RegionHeaderStatus, "fixture-header-status", "D-16", "The live disposable home starts empty while the approved fixture contains identities."),
				uxRegionDifference(RegionSidebar, "fixture-sidebar", "D-16", "The live disposable home starts empty while the approved fixture contains identities."),
			},
			VariantOf:        "ssh-form-filled",
			VariantRationale: "No separate HTML route exists for mouse-focused-field; it shares /create-flow/ssh-form-filled with a different focus state.",
		},
		{
			ScreenID:               "test-stage1-direct",
			Route:                  "/create-flow/test-stage1-direct",
			Interaction:            "Advance to step 1 (Test connection); press Enter to run stage 1. Capture at testRunning2 (stage-1 result visible, stage-2 pending).",
			StateMarker:            "… running ssh…",
			ApplicableLive:         true,
			ApplicableApprovedTUI:  true,
			ApplicableApprovedHTML: true,
			RequiredRegions:        []RegionName{RegionConnectivityOutput},
			RegionDispositions: []RegionDisposition{
				uxRegionDifference(RegionConnectivityOutput, "connectivity-output", "D-02", "The live capture uses the current backend outcome while the approved TUI uses a frozen fixture."),
				uxRegionDifference(RegionHeaderStatus, "fixture-header-status", "D-16", "The live disposable home starts empty while the approved fixture contains identities."),
				uxRegionDifference(RegionSidebar, "fixture-sidebar", "D-16", "The live disposable home starts empty while the approved fixture contains identities."),
			},
		},
		{
			ScreenID:               "test-stage2-by-alias",
			Route:                  "/create-flow/test-stage2-by-alias",
			Interaction:            "After stage-1 completes (D-04 auto-chain), wait for stage-2 result. Capture at testStage2 (both stages done).",
			StateMarker:            "Next: Git identity",
			ApplicableLive:         true,
			ApplicableApprovedHTML: true,
			NonApplicability: []SurfaceNonApplicability{uxNonComparable(
				"approved-tui", "D-04", "The approved TUI predates Phase 3 completed stage-2 auto-chain affordance.",
			)},
			RequiredRegions: []RegionName{RegionConnectivityOutput},
		},
		{
			ScreenID:               "git-form-demo",
			Route:                  "/git-screen/git-form-filled",
			Interaction:            "Advance past test stages to step 2 (Git identity); demo'd with D-16 banner.",
			StateMarker:            "Step 3/4",
			ApplicableLive:         true,
			ApplicableApprovedTUI:  true,
			ApplicableApprovedHTML: true,
			// WR-08 investigation found a RegionContinueDisabledReason
			// extractor (extractContinueDisabledReason) that was defined but
			// never wired into any RequiredRegions or AllRegionNames() —
			// dead code no gate ever exercised. WR-36 (04-REVIEW.md
			// iteration 4) found that WR-08's OWN follow-up comment here
			// claimed it was "now re-wired in createflow_regions.go", which
			// was false: AllRegionNames() still omitted it, so it remained
			// exactly as unreachable as before, just now documented as live
			// (worse than plain dead code). Both the constant and its
			// extractor were DELETED (not re-wired) — git-form-demo captures
			// the wizard's Git step with valid, filled fields, so
			// [ Continue ] is enabled and no disabled-reason line is
			// rendered at all; this screen genuinely has nothing for that
			// region to extract. A future screen spec that captures the
			// DISABLED-Continue state (e.g. an invalid-email variant) is the
			// correct place to reintroduce a region for it, wired into both
			// RequiredRegions and AllRegionNames() from the start.
			RequiredRegions: []RegionName{RegionKeybar},
			RegionDispositions: []RegionDisposition{
				uxRegionDifference(RegionHeaderStatus, "fixture-header-status", "D-16", "The live disposable home starts empty while the approved fixture contains identities."),
				uxRegionDifference(RegionSidebar, "fixture-sidebar", "D-16", "The live disposable home starts empty while the approved fixture contains identities."),
				// 04-04-PLAN.md Task 3 discovery: git-form-demo renders through
				// the SAME shared gitForm.view() code the git-screen registry's
				// RegionGitPreview/RegionGitFormFields regions extract from — the
				// SAME CTX-D-02/CTX-D-01 gitdir-default/author-name-template
				// divergences the git-screen registry classifies apply here too,
				// since this screen's wizard-default identity is "acme" (create-flow's
				// hardcoded default prefix), not the git-screen registry's own
				// "gscreen"/"gscreenssh" fixture identities.
				uxRegionDifference(RegionGitPreview, "gitdir-default", "CTX-D-02",
					"the real binary derives the gitdir default as \"~/git/<identity>/\" per D-02; the dummy's frozen includeIf preview fixture predates this derivation and shows the pre-Phase-4 \"~/<identity>/\" sample path"),
			},
		},
		{
			ScreenID:               "confirm-write",
			Route:                  "/create-flow/confirm-write",
			Interaction:            "Skip Git (4 Tabs from user.name, Enter on Skip button); ceremony state A.",
			StateMarker:            "BEGIN gitid managed:",
			ApplicableLive:         true,
			ApplicableApprovedTUI:  true,
			ApplicableApprovedHTML: true,
			RequiredRegions:        []RegionName{RegionConfirmationPreview},
			RegionDispositions: []RegionDisposition{
				uxRegionDifference(RegionConfirmationPreview, "confirmation-preview", "D-05", "The Phase 3 pre-write ceremony differs from the approved fixture preview."),
				uxRegionDifference(RegionHeaderStatus, "fixture-header-status", "D-16", "The live disposable home starts empty while the approved fixture contains identities."),
				uxRegionDifference(RegionSidebar, "fixture-sidebar", "D-16", "The live disposable home starts empty while the approved fixture contains identities."),
			},
		},
		{
			ScreenID:       "reuse-manual-resolved",
			Interaction:    "Select Reuse, focus the manual path input, type a sandbox key path, and wait for its algorithm and fingerprint.",
			StateMarker:    "ssh-ed25519",
			ApplicableLive: true,
			NonApplicability: []SurfaceNonApplicability{
				uxNonComparable("approved-tui", "D-10", "The approved TUI predates the resolved manual-key state."),
				uxNonComparable("approved-html", "D-10", "The approved HTML has no resolved manual-key state."),
			},
			RequiredRegions: []RegionName{RegionKeySection},
		},
		{
			ScreenID:       "test-stage1-pass",
			Interaction:    "Run stage 1 through the pass fake SSH and capture its completed PASS state while stage 2 is pending.",
			StateMarker:    "Hi user!",
			ApplicableLive: true,
			NonApplicability: []SurfaceNonApplicability{
				uxNonComparable("approved-tui", "D-04", "The approved TUI does not execute Phase 3 auto-chained SSH tests."),
				uxNonComparable("approved-html", "D-04", "The approved HTML has no completed Phase 3 test state."),
			},
			RequiredRegions: []RegionName{RegionConnectivityOutput},
		},
		{
			ScreenID:       "test-stage1-command-output",
			Interaction:    "Complete stage one, focus the proof viewport with raw v, and use PgUp/PgDn plus Left/Right until its exact command and output frame is visible.",
			StateMarker:    "Stage 1 output:",
			StateMarkers:   []string{"Stage 1 output:"},
			ApplicableLive: true,
			NonApplicability: []SurfaceNonApplicability{
				uxNonComparable("approved-tui", "D-04", "The approved TUI does not expose the Phase 3 proof viewport."),
				uxNonComparable("approved-html", "D-04", "The approved HTML has no completed Phase 3 test state."),
			},
			RequiredRegions: []RegionName{RegionConnectivityOutput},
		},
		{
			ScreenID:       "test-stage2-command-output",
			Interaction:    "Complete both stages, focus the proof viewport with raw v, and use PgUp/PgDn plus Left/Right until the exact stage-two command and output frame is visible.",
			StateMarker:    "Stage 2 output:",
			StateMarkers:   []string{"Stage 2 output:"},
			ApplicableLive: true,
			NonApplicability: []SurfaceNonApplicability{
				uxNonComparable("approved-tui", "D-04", "The approved TUI does not expose the Phase 3 proof viewport."),
				uxNonComparable("approved-html", "D-04", "The approved HTML has no completed Phase 3 test state."),
			},
			RequiredRegions: []RegionName{RegionConnectivityOutput},
		},
		{
			ScreenID:       "test-stage2-resolution-user-host-port",
			Interaction:    "From the focused proof viewport, use raw PgUp/PgDn and Left/Right until ssh -G User, Hostname, and Port are visible together.",
			StateMarker:    "user git",
			StateMarkers:   []string{"user git", "hostname ssh.github.com", "port 443"},
			ApplicableLive: true,
			NonApplicability: []SurfaceNonApplicability{
				uxNonComparable("approved-tui", "D-04", "The approved TUI does not expose the Phase 3 proof viewport."),
				uxNonComparable("approved-html", "D-04", "The approved HTML has no completed Phase 3 test state."),
			},
			RequiredRegions:  []RegionName{RegionConnectivityOutput},
			VariantOf:        "test-stage2-by-alias",
			VariantRationale: "Interaction variant of test-stage2-by-alias; same stage-2 terminal state scrolled to User/Hostname/Port fields.",
		},
		{
			ScreenID:       "test-stage2-resolution-identities-key",
			Interaction:    "From the focused proof viewport, use raw PgUp/PgDn and Left/Right until ssh -G IdentitiesOnly and the first IdentityFile bytes are visible together.",
			StateMarker:    "identitiesonly yes",
			StateMarkers:   []string{"identitiesonly yes", "identityfile"},
			ApplicableLive: true,
			NonApplicability: []SurfaceNonApplicability{
				uxNonComparable("approved-tui", "D-04", "The approved TUI does not expose the Phase 3 proof viewport."),
				uxNonComparable("approved-html", "D-04", "The approved HTML has no completed Phase 3 test state."),
			},
			RequiredRegions:  []RegionName{RegionConnectivityOutput},
			VariantOf:        "test-stage2-by-alias",
			VariantRationale: "Interaction variant of test-stage2-by-alias; same stage-2 terminal state scrolled to IdentitiesOnly/IdentityFile fields.",
		},
		{
			ScreenID:       "test-reachable-not-uploaded",
			Interaction:    "Run both stages through the denied fake SSH and capture the completed copy-public-key warning state.",
			StateMarker:    "key not uploaded yet",
			ApplicableLive: true,
			NonApplicability: []SurfaceNonApplicability{
				uxNonComparable("approved-tui", "D-02", "The approved TUI has no Phase 3 reachable-not-uploaded outcome."),
				uxNonComparable("approved-html", "D-02", "The approved TUI has no Phase 3 reachable-not-uploaded outcome."),
			},
			RequiredRegions: []RegionName{RegionConnectivityOutput},
		},
		{
			ScreenID:       "test-hard-failure-retry",
			Interaction:    "Run stage 1 through the timeout fake SSH and capture the completed hard-failure retry state.",
			StateMarker:    "Retry (Enter)",
			ApplicableLive: true,
			NonApplicability: []SurfaceNonApplicability{
				uxNonComparable("approved-tui", "D-01", "The approved TUI has no Phase 3 hard-failure retry state."),
				uxNonComparable("approved-html", "D-01", "The approved HTML has no Phase 3 hard-failure retry state."),
			},
			RequiredRegions: []RegionName{RegionConnectivityOutput},
		},
		{
			ScreenID:       "confirm-summary-key-path",
			Interaction:    "Navigate to the confirmation ceremony, focus its viewport with raw v, then use PgUp/PgDn and Left/Right until the complete sandbox key path is visible before write.",
			StateMarker:    "~/.ssh/id_ed25519_acme",
			StateMarkers:   []string{"~/.ssh/id_ed25519_acme"},
			ApplicableLive: true,
			NonApplicability: []SurfaceNonApplicability{
				uxNonComparable("approved-tui", "D-05", "The approved TUI does not expose the Phase 3 confirmation viewport."),
				uxNonComparable("approved-html", "D-05", "The approved HTML has no Phase 3 confirmation viewport."),
			},
			RequiredRegions: []RegionName{RegionConfirmationPreview},
		},
		{
			ScreenID:       "confirm-managed-block",
			Interaction:    "From the focused confirmation viewport, use raw PgUp/PgDn and Left/Right until the complete BEGIN-to-END managed block is visible before write.",
			StateMarker:    "# END gitid managed:",
			StateMarkers:   []string{"# BEGIN gitid managed:", "# END gitid managed:"},
			ApplicableLive: true,
			NonApplicability: []SurfaceNonApplicability{
				uxNonComparable("approved-tui", "D-05", "The approved TUI does not expose the Phase 3 confirmation viewport."),
				uxNonComparable("approved-html", "D-05", "The approved HTML has no Phase 3 confirmation viewport."),
			},
			RequiredRegions: []RegionName{RegionConfirmationPreview},
		},
	}
	// 04-04-PLAN.md Task 3: consume the git-screen registry alongside the
	// create-flow registry above — ONE combined ScreenSpec inventory drives
	// every consumer (the routine gate, RequiredVisualPanelCount, and the
	// evidence packet publisher).
	specs = append(specs, gitScreenSpecs()...)
	// 05-09-PLAN.md Task 3: consume the identity-manager registry alongside
	// the existing two, without disturbing either.
	specs = append(specs, identityManagerSpecs()...)
	// 06-07-PLAN.md Task 1: consume the Phase 6 Global SSH registry alongside
	// the existing three, without disturbing any of them (four-way merged).
	specs = append(specs, globalSSHSpecs()...)
	return specs
}

// RequiredScreenSpecs returns the registry-backed visual packet inventory.
// No panel count or parallel screen list is authoritative.
func RequiredScreenSpecs() []ScreenSpec { return ScreenSpecRegistry() }

// ScreenAppliesToSurface reports whether a registry frame is required for a
// capture surface.
func ScreenAppliesToSurface(spec ScreenSpec, surface string) bool {
	switch surface {
	case "live":
		return spec.ApplicableLive
	case "approved-tui":
		return spec.ApplicableApprovedTUI
	case "approved-html":
		return spec.ApplicableApprovedHTML
	default:
		return false
	}
}

// NonApplicabilityForSurface returns the explicit record for a surface that
// cannot truthfully render a screen's declared state.
func NonApplicabilityForSurface(spec ScreenSpec, surface string) (SurfaceNonApplicability, bool) {
	for _, record := range spec.NonApplicability {
		if record.Surface == surface {
			return record, true
		}
	}
	return SurfaceNonApplicability{}, false
}

// ValidateCapturedState reports whether the captured text satisfies the spec's
// state marker contract. Returns nil if all markers are present, an error if
// any marker is absent.
//
// Region-bound enforcement (03-16 Task 2): each marker must appear at least
// once within a focused pane line (prefixed with │ or after stripping the │
// border from a line where only whitespace precedes it). Markers that appear
// only outside the focused pane — i.e., in plain unreceived lines without any
// │ — do not authorize the capture. This prevents a complete out-of-viewport
// duplicate from impersonating a focused-region marker.
func ValidateCapturedState(spec ScreenSpec, capturedText string) error {
	markers := spec.StateMarkers
	if len(markers) == 0 && spec.StateMarker != "" {
		markers = []string{spec.StateMarker}
	}
	if len(markers) == 0 {
		return fmt.Errorf("screenshot: ValidateCapturedState: spec %q has no state marker", spec.ScreenID)
	}
	for _, marker := range markers {
		if !markerInPane(capturedText, marker) {
			return fmt.Errorf("screenshot: ValidateCapturedState: spec %q state marker %q absent from captured text or not within a focused viewport pane (│)", spec.ScreenID, marker)
		}
	}
	return nil
}

// markerInPane reports whether marker appears within the right-pane content
// of text — i.e., in lines that contain the "│" pane separator.
//
// The separator may have sidebar content (or ANSI styling) to its left, so
// only the portion to the RIGHT of the first "│" contributes to the
// authorizing context. Markers that appear only in lines without "│" (header,
// breadcrumb, keybar, or out-of-viewport duplicates) do not authorize.
func markerInPane(text, marker string) bool {
	normalizedMarker := normalizeCapturedStateText(marker)

	// Collect content from all pane lines (lines with a "│" separator).
	var paneContent strings.Builder
	for _, line := range strings.Split(text, "\n") {
		if !strings.Contains(stripANSI(line), "│") {
			continue
		}
		// rightPane maps through ANSI codes to the raw offset of the separator
		// and returns only the content to its right.
		rp := rightPane(line)
		if paneContent.Len() > 0 {
			paneContent.WriteByte(' ')
		}
		paneContent.WriteString(rp)
	}
	if paneContent.Len() == 0 {
		// No pane lines found — marker cannot be in the pane.
		return false
	}
	// Normalize the accumulated pane content (collapse whitespace) and check.
	normalizedPane := strings.Join(strings.Fields(paneContent.String()), " ")
	return strings.Contains(normalizedPane, normalizedMarker)
}

// normalizeCapturedStateText removes the detail-pane border from wrapped rows
// and collapses whitespace so semantic markers survive physical PTY wrapping.
func normalizeCapturedStateText(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if border := strings.IndexRune(line, '│'); border >= 0 && strings.TrimSpace(line[:border]) == "" {
			lines[i] = line[border+len("│"):]
		}
	}
	return strings.Join(strings.Fields(strings.Join(lines, " ")), " ")
}

// validDecisionRef reports whether ref is a recognized decision-reference
// vocabulary entry: the create-flow registry's bare D-NN/T-NN identifiers
// (03-CONTEXT.md/03-06-SUMMARY.md), or the Phase 4 git-screen registry's
// scoped CTX-D-NN (04-CONTEXT.md) / UI-D-NN (04-UI-SPEC.md) identifiers
// (04-04-PLAN.md Task 3 — generalized decision-ref validation). Scoped
// prefixes exist because 04-CONTEXT.md and 04-UI-SPEC.md's own D-NN
// numbering collides (both start at D-01) — CTX-D-/UI-D- disambiguate which
// document a reference resolves against.
func validDecisionRef(ref string) bool {
	// 05-09-PLAN.md Task 3: two additions to the existing (D-/T-/CTX-D-/
	// UI-D-) vocabulary, both scoped the SAME way "CTX-D-" already
	// disambiguates Phase 4's git-screen decisions from Phase 3's bare
	// D-NN/T-NN namespace in this SAME shared registry:
	//   - "DLV-" for the one divergence class that is a property of the
	//     real-vs-dummy comparison MECHANISM itself (fixture-set size)
	//     rather than of any single numbered design decision — mirrors
	//     e2e/identity_manager_pty_e2e_test.go's own
	//     identManagerDecisionRefPattern, which accepts the same DLV-NN
	//     form for the identical reason.
	//   - "MGR-D-" for a genuine Phase 5 05-CONTEXT.md decision cited in
	//     THIS shared registry: 05-CONTEXT.md's OWN D-NN numbering starts
	//     at D-01, same as 03-CONTEXT.md's — a bare "D-11" here would be
	//     genuinely ambiguous (Phase 3's D-11 is "Validation: parse +
	//     derive missing .pub..."; Phase 5's D-11 is "Delete everything
	//     backup-copies the key pair..."), the EXACT cross-registry
	//     collision "CTX-D-" was introduced to prevent for Phase 4.
	return strings.HasPrefix(ref, "D-") || strings.HasPrefix(ref, "T-") ||
		strings.HasPrefix(ref, "CTX-D-") || strings.HasPrefix(ref, "UI-D-") ||
		strings.HasPrefix(ref, "DLV-") || strings.HasPrefix(ref, "MGR-D-") ||
		// 06-07-PLAN.md Task 1 (Phase 6 registration): the scoped vocabulary
		// for the Global SSH registries in this SHARED registry. 06-CONTEXT.md
		// numbers its own decisions D-01..D-16, colliding with 03-CONTEXT.md's
		// bare D-NN namespace exactly the way 04-CONTEXT.md's did — so the
		// Phase 6 refs use the disambiguating GSSH-D- prefix (the git-screen
		// CTX-D-/UI-D- precedent) for 06-CONTEXT decisions and STORE- for the
		// STORE-01/STORE-03 storage-layout decisions. DLV- remains a valid
		// shared requirements literal for the DLV-04 real-vs-frozen-dummy
		// comparison class (the same literal Phase 5's registry already uses).
		strings.HasPrefix(ref, "GSSH-D-") || strings.HasPrefix(ref, "STORE-")
}

// ValidateScreenSpecs checks the registry for structural correctness:
// - No duplicate screen IDs without valid VariantOf metadata
// - Every VariantOf references an existing base screen ID
// - State markers are unique across non-variant specs (variants may share routes)
func ValidateScreenSpecs(specs []ScreenSpec) error {
	seenIDs := make(map[string]int) // screen_id → first occurrence index
	seenMarkers := make(map[string]string)
	for i, s := range specs {
		if s.ScreenID == "" {
			return fmt.Errorf("screenshot: ValidateScreenSpecs: spec[%d] has empty ScreenID", i)
		}
		markers := s.StateMarkers
		if len(markers) == 0 && s.StateMarker != "" {
			markers = []string{s.StateMarker}
		}
		if len(markers) == 0 {
			return fmt.Errorf("screenshot: ValidateScreenSpecs: spec %q has empty state marker", s.ScreenID)
		}
		for _, marker := range markers {
			if strings.TrimSpace(marker) == "" {
				return fmt.Errorf("screenshot: ValidateScreenSpecs: spec %q has an empty required marker", s.ScreenID)
			}
			if prior, exists := seenMarkers[marker]; exists && prior != s.ScreenID {
				return fmt.Errorf("screenshot: ValidateScreenSpecs: marker %q is shared by %q and %q", marker, prior, s.ScreenID)
			}
			seenMarkers[marker] = s.ScreenID
		}
		if s.ApplicableApprovedHTML && s.Route == "" {
			return fmt.Errorf("screenshot: ValidateScreenSpecs: HTML-applicable spec %q has no route", s.ScreenID)
		}
		for _, surface := range []string{"live", "approved-tui", "approved-html"} {
			record, found := NonApplicabilityForSurface(s, surface)
			if ScreenAppliesToSurface(s, surface) {
				if found {
					return fmt.Errorf("screenshot: ValidateScreenSpecs: applicable %s spec %q has a non-applicability record", surface, s.ScreenID)
				}
				continue
			}
			if !found || record.Decision == "" || !validDecisionRef(record.Decision) || record.Reason == "" || !validDifferenceClassification(record.Classification) {
				return fmt.Errorf("screenshot: ValidateScreenSpecs: non-applicable %s spec %q lacks a decision-linked record", surface, s.ScreenID)
			}
		}
		nonApplicableSurfaces := make(map[string]bool, len(s.NonApplicability))
		for _, record := range s.NonApplicability {
			if !validPacketSurface(record.Surface) || nonApplicableSurfaces[record.Surface] {
				return fmt.Errorf("screenshot: ValidateScreenSpecs: spec %q has invalid non-applicability surface %q", s.ScreenID, record.Surface)
			}
			nonApplicableSurfaces[record.Surface] = true
		}
		if len(s.RequiredRegions) == 0 {
			return fmt.Errorf("screenshot: ValidateScreenSpecs: spec %q has no required regions", s.ScreenID)
		}
		regions := make(map[RegionName]bool, len(s.RequiredRegions))
		for _, region := range s.RequiredRegions {
			if region == "" || regions[region] {
				return fmt.Errorf("screenshot: ValidateScreenSpecs: spec %q has invalid required region %q", s.ScreenID, region)
			}
			regions[region] = true
		}
		knownRegions := make(map[RegionName]bool, len(AllRegionNames()))
		for _, region := range AllRegionNames() {
			knownRegions[region] = true
		}
		dispositions := make(map[RegionName]bool, len(s.RegionDispositions))
		for _, disposition := range s.RegionDispositions {
			if !knownRegions[disposition.Region] || dispositions[disposition.Region] {
				return fmt.Errorf("screenshot: ValidateScreenSpecs: spec %q has invalid region disposition %q", s.ScreenID, disposition.Region)
			}
			if strings.TrimSpace(disposition.Divergence) == "" ||
				!validDecisionRef(disposition.Decision) ||
				strings.TrimSpace(disposition.Reason) == "" ||
				!validDifferenceClassification(disposition.Classification) {
				return fmt.Errorf("screenshot: ValidateScreenSpecs: spec %q region %q lacks a decision-linked disposition", s.ScreenID, disposition.Region)
			}
			if !validRegionPredicate(disposition.Predicate) {
				return fmt.Errorf("screenshot: ValidateScreenSpecs: spec %q region %q has invalid predicate %q (must be empty, \"contains:<text>\", or \"absent:<text>\" — WR-19)", s.ScreenID, disposition.Region, disposition.Predicate)
			}
			dispositions[disposition.Region] = true
		}
		if prior, exists := seenIDs[s.ScreenID]; exists {
			// A duplicate is allowed ONLY when this spec declares a same-route variant.
			priorSpec := specs[prior]
			// For a valid same-route variant: VariantOf must reference the prior spec's
			// ScreenID (or vice versa), and VariantRationale must be non-empty.
			isVariant := (s.VariantOf == priorSpec.ScreenID || priorSpec.VariantOf == s.ScreenID) &&
				(s.VariantRationale != "" || priorSpec.VariantRationale != "")
			if !isVariant {
				return fmt.Errorf("screenshot: ValidateScreenSpecs: duplicate screen ID %q (index %d and %d) without valid VariantOf declaration", s.ScreenID, prior, i)
			}
		}
		seenIDs[s.ScreenID] = i
	}
	return nil
}

// ValidateScreenSpecRegistry validates the built-in registry.
func ValidateScreenSpecRegistry() error {
	return ValidateScreenSpecs(ScreenSpecRegistry())
}

// ApprovedHTMLRoutes returns the canonical map from logical screen ID to the
// approved reference route in the HTML mockup. Each route is the full
// fragment path (e.g. "/create-flow/ssh-form-filled") so callers can use it
// directly in a URLFragment without further transformation.
//
// The routes are specified by the plan's interface contract and the approved
// Phase-2 mockup route registry:
//   - reuse-manual-path: an interaction variant of /create-flow/reuse-key-vs-generate
//     (NOT ssh-form-blank-prefix — that is a different form state)
//   - mouse-focused-field: an interaction variant of /create-flow/ssh-form-filled
//     (NOT ssh-form-empty — the mouse captures the FILLED form with focus moved)
//   - git-form-demo: uses the /git-screen/git-form-filled route
//     (NOT create-flow/backup-notice — git-form-demo is the git-screen surface)
func ApprovedHTMLRoutes() map[string]string {
	return approvedHTMLRoutesInternal()
}

// step drives model with msg, then synchronously drains any cmd chain the
// Update call returns — recursively, so a multi-hop chain (e.g. a stage
// test's tea.Cmd delivering a WizardStageMsg) resolves fully before the
// next scripted step runs. Bubble Tea's own runtime does the same thing
// across render frames; here it happens inline since there is no real
// event loop driving this capture.
func step(model tea.Model, msg tea.Msg) tea.Model {
	m, cmd := model.Update(msg)
	if cmd != nil {
		if msg2 := cmd(); msg2 != nil {
			return step(m, msg2)
		}
	}
	return m
}

// stepAndPendingCmd drives model with msg and returns the updated model plus
// the pending tea.Cmd (if any) WITHOUT executing the cmd. This allows callers
// to capture the model state after one message is processed but BEFORE the
// auto-chained command fires — enabling capture of intermediate stage states
// (CR-02: stage-1 state before stage-2 auto-chain fires).
//
// Usage: the caller receives (m, pendingCmd); captures anyView(m); then calls
// step(m, pendingCmd()) to advance to the next stage.
func stepAndPendingCmd(model tea.Model, msg tea.Msg) (tea.Model, tea.Cmd) {
	m, cmd := model.Update(msg)
	return m, cmd
}

// keyRune sends a single printable-character key press (e.g. "n" to open
// the wizard, "y" — none needed here, but mirrors the e2e harness's
// keystroke-by-keystroke discipline).
func keyRune(model tea.Model, r rune) tea.Model {
	return step(model, tea.KeyPressMsg{Code: r, Text: string(r)})
}

func keyEnter(model tea.Model) tea.Model { return step(model, tea.KeyPressMsg{Code: tea.KeyEnter}) }
func keyTab(model tea.Model) tea.Model   { return step(model, tea.KeyPressMsg{Code: tea.KeyTab}) }
func keyRight(model tea.Model) tea.Model { return step(model, tea.KeyPressMsg{Code: tea.KeyRight}) }
func keyLeft(model tea.Model) tea.Model  { return step(model, tea.KeyPressMsg{Code: tea.KeyLeft}) }
func keyPgDown(model tea.Model) tea.Model {
	return step(model, tea.KeyPressMsg{Code: tea.KeyPgDown})
}
func keyPgUp(model tea.Model) tea.Model { return step(model, tea.KeyPressMsg{Code: tea.KeyPgUp}) }

func captureViewportMarkers(model tea.Model, markers []string, allowHorizontal bool) (tea.Model, bool) {
	containsAll := func(candidate tea.Model) bool {
		view := anyView(candidate)
		for _, marker := range markers {
			if !strings.Contains(view, marker) {
				return false
			}
		}
		return true
	}

	model = keyRune(model, 'v')
	// Exercise both advertised axes, then establish a deterministic origin.
	model = keyPgDown(model)
	model = keyPgUp(model)
	model = keyRight(model)
	for range 32 {
		model = keyLeft(model)
	}
	for range 12 {
		if containsAll(model) {
			return model, true
		}
		if allowHorizontal {
			for range 32 {
				model = keyRight(model)
				if containsAll(model) {
					return model, true
				}
			}
			for range 32 {
				model = keyLeft(model)
			}
		}
		model = keyPgDown(model)
	}
	return model, false
}

// tabN sends n Tab keypresses in sequence.
func tabN(model tea.Model, n int) tea.Model {
	for i := 0; i < n; i++ {
		model = keyTab(model)
	}
	return model
}

// freshWizard boots a new App around backend at the fixed capture geometry
// and opens the create wizard from the identities pane (the SAME 'n'
// keystroke a real user presses) — the common starting point every
// captured screen scripts forward from, so no screen's capture can leak
// state from a PRIOR screen's navigation.
func freshWizard(backend tuikit.Backend) tea.Model {
	var model tea.Model = tuikit.NewApp(backend)
	model = step(model, tea.WindowSizeMsg{Width: CaptureWidth, Height: CaptureHeight})
	model = keyRune(model, 'n')
	return model
}

// anyView extracts the plain rendered text from model's current View() —
// the tea.Model interface's View() already returns a concrete
// tea.View{Content string, ...} (charm.land/bubbletea/v2).
func anyView(model tea.Model) string {
	return model.View().Content
}

// clickField locates label's row in text (the SAME "search the decoded
// frame for a label, click its row" technique e2e/create_flow_pty_e2e_test.go's
// clickLabelRow uses for real xterm SGR sequences) and sends a REAL
// tea.MouseClickMsg at that position — proving the mouse-click code path
// itself, not merely re-deriving the same result a keyboard Tab would.
func clickField(model tea.Model, label string) tea.Model {
	text := anyView(model)
	x, y, ok := locateLabel(text, label)
	if !ok {
		return model
	}
	return step(model, tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
}

// locateLabel finds label's first occurrence in text and returns its
// (column, row) — 0-based, matching tea.Mouse's coordinate space.
func locateLabel(text, label string) (x, y int, ok bool) {
	lines := splitLines(text)
	for row, line := range lines {
		if idx := indexOf(line, label); idx >= 0 {
			return idx + 1, row, true
		}
	}
	return 0, 0, false
}

// splitLines and indexOf avoid importing "strings" twice across this small
// file's helpers — trivial local implementations, no external behavior.
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	lines = append(lines, s[start:])
	return lines
}

func indexOf(s, substr string) int {
	if substr == "" {
		return -1
	}
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// offlineCaptureBackend wraps any tuikit.Backend and replaces TestStage1 and
// TestStage2 with deterministic, immediately-resolved offline results.
// This ensures the in-process capture script can advance the wizard through
// all stages without blocking on a real SSH call (D-22) or waiting for a
// tick timer (which would never fire inside step()).
type offlineCaptureBackend struct {
	tuikit.Backend
}

func (o offlineCaptureBackend) TestStage1(spec tuikit.CreateSpec) tea.Cmd {
	return func() tea.Msg {
		return tuikit.WizardStageMsg{
			Stage: 1,
			Result: tuikit.TestResultView{
				Outcome: tuikit.TestOutcomePass,
				Command: o.Stage1Command(spec),
				Detail:  "Hi user! You've successfully authenticated, but GitHub does not provide shell access.",
			},
		}
	}
}

func (o offlineCaptureBackend) TestStage2(spec tuikit.CreateSpec) tea.Cmd {
	return func() tea.Msg {
		return tuikit.WizardStageMsg{
			Stage: 2,
			Result: tuikit.TestResultView{
				Outcome:           tuikit.TestOutcomeReachableNotUploaded,
				Command:           o.Stage2Command(spec),
				Detail:            "identityfile " + spec.KeyPath,
				ResolutionCommand: "ssh -F /tmp/gitid-stage/config -G " + spec.Alias,
				ResolutionOutput: strings.Join([]string{
					"user git",
					"hostname " + spec.Hostname,
					"port " + spec.Port,
					"identitiesonly yes",
					"identityfile " + spec.KeyPath,
				}, "\n"),
			},
		}
	}
}

// captureSpec returns a representative CreateSpec for the offline capture —
// the default form values (acme prefix, github.com provider, standard endpoint).
// This is used to build the command strings for the injected stage results.
func captureSpec(backend tuikit.Backend) tuikit.CreateSpec {
	hostname, port := backend.ProviderDefaults("github.com")
	return tuikit.CreateSpec{
		Identity:  "acme",
		Provider:  "github.com",
		Alias:     "acme.github.com",
		Hostname:  hostname,
		Port:      port,
		KeyPath:   "~/.ssh/id_ed25519_acme",
		Algorithm: "ed25519",
	}
}

// CaptureCreateFlowScreens drives backend's create-flow wizard through the
// fixed script above and returns the rendered text for every live-applicable
// RequiredScreenSpecs checkpoint, keyed by screen ID.
// The backend is wrapped with offlineCaptureBackend to ensure TestStage1 and
// TestStage2 resolve immediately without network calls or tick timers (D-22).
//
// All captures are normalized: timestamps in backup file names are replaced
// with "<timestamp>" so the output is byte-identical across wall-clock seconds
// (CR-01 determinism contract).
//
// Returns an error if any required frame is missing or fails marker validation.
func CaptureCreateFlowScreens(backend tuikit.Backend) (map[string]string, error) {
	backend = offlineCaptureBackend{backend}
	out := make(map[string]string, len(RequiredScreenSpecs()))
	capture := func(m tea.Model) string {
		return normalizeTimestamps(anyView(m))
	}

	// ssh-form-filled: the wizard's default-filled step 0.
	m := freshWizard(backend)
	out["ssh-form-filled"] = capture(m)

	// reuse-key-vs-generate: Tab from Alias prefix (focus 1) to the
	// Generate/Reuse toggle (focus 5) — 4 Tabs — then flip to reuse.
	m = freshWizard(backend)
	m = tabN(m, 4)
	m = keyRight(m)
	out["reuse-key-vs-generate"] = capture(m)

	// reuse-manual-path: move from the source toggle into the reuse picker,
	// then a single Left wraps from its first key to the trailing manual row.
	m = keyTab(m)
	m = keyLeft(m)
	m = keyTab(m)
	out["reuse-manual-path"] = capture(m)
	if keys := backend.ScanReusableKeys(); len(keys) > 0 {
		for _, r := range keys[0].Path {
			m = keyRune(m, r)
		}
		out["reuse-manual-resolved"] = capture(m)
	}

	// mouse-focused-field: a REAL synthesized mouse click on the Port row,
	// from a fresh generate-mode wizard (keeps this screen's SSH-field
	// layout directly comparable to ssh-form-filled).
	m = freshWizard(backend)
	m = clickField(m, "Port")
	out["mouse-focused-field"] = capture(m)

	// test-stage1-direct: drive a valid state transition through the real
	// wizard state machine (CR-02: inject through testRunning1, not directly).
	//
	// Protocol:
	//   1. keyEnter from step 1 (test-idle) sets testPhase = testRunning1 and
	//      fires TestStage1 as a tea.Cmd.
	//   2. We use stepAndPendingCmd to process the Enter and get the pending
	//      TestStage1 cmd WITHOUT executing it yet.
	//   3. Execute the stage-1 cmd: it delivers WizardStageMsg{Stage:1}.
	//      handleMsg sees testPhase=testRunning1, records stage-1 result, sets
	//      testPhase=testRunning2, and returns TestStage2 cmd.
	//   4. stepAndPendingCmd stops here — stage-2 cmd is pending but NOT fired.
	//      The model is in testRunning2 state; anyView shows the stage-1 result.
	//   5. Capture "test-stage1-direct" at this intermediate state.
	//   6. Execute the pending TestStage2 cmd to advance to testStage2 state.
	//   7. Capture "test-stage2-by-alias" with the complete two-stage result.
	m = freshWizard(backend)
	m = keyEnter(m) // step 0 -> step 1 (test connection screen shown, testPhase=testIdle)

	// Enter from testIdle → testRunning1 + TestStage1 cmd returned.
	var stage1Cmd tea.Cmd
	m, stage1Cmd = stepAndPendingCmd(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if stage1Cmd != nil {
		// Deliver the stage-1 result (fires TestStage2 cmd as side effect).
		var stage2Cmd tea.Cmd
		stage1Msg := stage1Cmd()
		if stage1Msg != nil {
			m, stage2Cmd = stepAndPendingCmd(m, stage1Msg)
		}
		out["test-stage1-direct"] = capture(m)
		out["test-stage1-pass"] = capture(m)
		if exact, ok := captureViewportMarkers(m, []string{"Stage 1 output:"}, false); ok {
			out["test-stage1-command-output"] = capture(exact)
		}
		// Deliver the stage-2 result.
		if stage2Cmd != nil {
			stage2Msg := stage2Cmd()
			if stage2Msg != nil {
				m = step(m, stage2Msg)
			}
		}
	} else {
		// Fallback (should not happen with offline backend): capture current view.
		out["test-stage1-direct"] = capture(m)
	}
	out["test-stage2-by-alias"] = capture(m)
	out["test-reachable-not-uploaded"] = capture(m)
	for id, markers := range map[string][]string{
		"test-stage2-command-output":            {"Stage 2 output:"},
		"test-stage2-resolution-user-host-port": {"user git", "hostname ssh.github.com", "port 443"},
		"test-stage2-resolution-identities-key": {"identitiesonly yes", "identityfile"},
	} {
		if exact, ok := captureViewportMarkers(m, markers, false); ok {
			out[id] = capture(exact)
		}
	}

	// git-form-demo: advance to step 3 (Git identity) from the test screen.
	// After both stage results are complete, the test screen shows
	// "Next: Git identity (Enter)". A single Enter advances the wizard.
	m = keyEnter(m)
	// If the model is still on the test screen (not yet on the git form),
	// try one more Enter — the wizard may need two keystrokes to advance
	// from the test-stage2-result to the git-form step in some backends.
	{
		view := anyView(m)
		if strings.Contains(view, "Stage 2") || strings.Contains(view, "Stage 1") {
			m = keyEnter(m)
		}
	}
	out["git-form-demo"] = capture(m)

	// confirm-write: Skip Git (5 Tabs from user.name to the Skip button —
	// name → email → strategy → Force SSH → Back → Skip, CR-06: Force SSH is
	// now a wizard-ring member — then Enter) reaches the review ceremony
	// (state A, unconfirmed).
	m = tabN(m, 5)
	m = keyEnter(m)
	out["confirm-write"] = capture(m)
	if exact, ok := captureViewportMarkers(m, []string{"~/.ssh/id_ed25519_acme"}, true); ok {
		out["confirm-summary-key-path"] = capture(exact)
	}
	if exact, ok := captureViewportMarkers(m, []string{"# BEGIN gitid managed:", "# END gitid managed:"}, false); ok {
		out["confirm-managed-block"] = capture(exact)
	}

	// Capture the hard-failure retry state through the same exported stage
	// message path the real backend uses, without invoking SSH.
	failure := freshWizard(backend)
	failure = keyEnter(failure)
	failure, _ = stepAndPendingCmd(failure, tea.KeyPressMsg{Code: tea.KeyEnter})
	failure = step(failure, tuikit.WizardStageMsg{
		Stage: 1,
		Result: tuikit.TestResultView{
			Outcome: tuikit.TestOutcomeFailure,
			Command: backend.Stage1Command(captureSpec(backend)),
			Detail:  "ssh: connect to host ssh.github.com port 443: Operation timed out",
		},
	})
	out["test-hard-failure-retry"] = capture(failure)

	// Validate that all required live frames are present. Git-screen specs
	// (04-04-PLAN.md Task 3) are captured SEPARATELY by
	// CaptureGitScreenScreens against their OWN seeded HOME — merging them
	// into this same backend/HOME would add real identities to the wizard's
	// sidebar and shift every create-flow screen's layout (a regression
	// discovered empirically: it broke "git-form-demo"'s connectivity-output
	// region, which has no identity-count dependency of its own). Callers
	// that need the combined inventory (the routine gate, the packet
	// publisher) merge both capture maps explicitly — see
	// cmd/gitid/gate_visual_regression_test.go.
	for _, spec := range RequiredScreenSpecs() {
		// 05-09-PLAN.md Task 3: identity-manager specs (CaptureIdentityManagerScreens)
		// are ALSO captured separately, against their own seeded HOME, for the
		// SAME reason git-screen specs are excluded here (see comment above).
		// 06-07-PLAN.md Task 1: Global SSH specs (CaptureGlobalSSHScreens) are
		// likewise captured separately against their own seeded HOME.
		if !spec.ApplicableLive || isGitScreenID(spec.ScreenID) || isIdentityManagerScreenID(spec.ScreenID) || isGlobalSSHScreenID(spec.ScreenID) {
			continue
		}
		text, ok := out[spec.ScreenID]
		if !ok || strings.TrimSpace(text) == "" {
			return nil, fmt.Errorf("screenshot: CaptureCreateFlowScreens: required frame %q is missing or empty", spec.ScreenID)
		}
	}

	return out, nil
}

// ---------------------------------------------------------------------------
// Git-screen checkpoints (04-04-PLAN.md Task 3, Phase 4 registration).
//
// CaptureGitScreenScreens drives backend's Identities/Configure-Git pane
// in-process through the SAME semantic checkpoint script
// e2e/git_configuration_pty_e2e_test.go's TestGitConfiguration_
// CompiledRealVsLiveDummyPTY (Task 2) drives over real PTYs — five named
// checkpoints (git-form-filled, git-form-empty, match-strategy-select,
// review-readonly, result-success), the SAME vocabulary both gates classify
// divergences against in
// .planning/design/git-screen/visual-divergence-allowlist.txt.
//
// backend must expose at least two identities: the default-selected
// identity (index 0, a COMPLETE identity — Git already configured) drives
// every checkpoint except git-form-empty; the SECOND identity (index 1, an
// SSH-only identity with no Git fragment yet) drives git-form-empty.
// dummytui.NewFixtureBackend() already satisfies this (its "personal"/"work"
// fixtures); a real Backend needs its HOME seeded accordingly before this is
// called (see cmd/gitid/gate_visual_regression_test.go's
// deterministicGitIdentityFixture).
// ---------------------------------------------------------------------------

// gitScreenIdentitiesApp boots a fresh tuikit.App around backend at the
// fixed capture geometry, landing on the Identities detail pane — the SAME
// entry point the "g" keystroke launches Configure Git from.
func gitScreenIdentitiesApp(backend tuikit.Backend) tea.Model {
	var model tea.Model = tuikit.NewApp(backend)
	model = step(model, tea.WindowSizeMsg{Width: CaptureWidth, Height: CaptureHeight})
	return model
}

func keyDown(model tea.Model) tea.Model { return step(model, tea.KeyPressMsg{Code: tea.KeyDown}) }

func keyUp(model tea.Model) tea.Model { return step(model, tea.KeyPressMsg{Code: tea.KeyUp}) }

// CaptureGitScreenScreens implements the doc comment above.
func CaptureGitScreenScreens(backend tuikit.Backend) (map[string]string, error) {
	// WR-11: the doc comment above states the >= 2 identities precondition
	// but nothing checked it. With a single identity, keyDown (used to reach
	// git-form-empty below) is a no-op, so git-form-empty silently captured
	// the SAME identity as git-form-filled — the completeness loop at the
	// bottom of this function only asserts non-emptiness, so the gate PASSED
	// while silently losing the SSH-only checkpoint's actual coverage.
	if n := len(backend.InitialState().Identities); n < 2 {
		return nil, fmt.Errorf("screenshot: CaptureGitScreenScreens requires >= 2 seeded identities, got %d", n)
	}
	out := make(map[string]string, 5)
	capture := func(m tea.Model) string { return normalizeTimestamps(anyView(m)) }

	// git-form-filled: default-selected identity (edit mode — Git already
	// configured).
	m := gitScreenIdentitiesApp(backend)
	m = keyRune(m, 'g')
	out["git-form-filled"] = capture(m)

	// match-strategy-select: focus the strategy field (name -> email -> strategy).
	strategyFocused := keyTab(m)
	strategyFocused = keyTab(strategyFocused)
	out["match-strategy-select"] = capture(strategyFocused)

	// review-readonly / result-success: the write ceremony, confirmed twice
	// (Enter reaches the preview, a second Enter commits — matching the real
	// PTY suite's confirmed sequence).
	review := keyEnter(m)
	out["review-readonly"] = capture(review)
	result := keyEnter(review)
	out["result-success"] = capture(result)

	// git-form-empty: the second identity (SSH-only completion).
	empty := gitScreenIdentitiesApp(backend)
	empty = keyDown(empty)
	empty = keyRune(empty, 'g')
	out["git-form-empty"] = capture(empty)

	for _, spec := range gitScreenSpecs() {
		text, ok := out[spec.ScreenID]
		if !ok || strings.TrimSpace(text) == "" {
			return nil, fmt.Errorf("screenshot: CaptureGitScreenScreens: required frame %q is missing or empty", spec.ScreenID)
		}
	}
	// WR-11: even with >= 2 identities confirmed above, a fixture whose
	// second identity happens to render identically to the first (or a
	// future script change that stops advancing to it) would still pass
	// the non-emptiness loop above while silently losing the SSH-only
	// checkpoint. Assert the two frames actually differ.
	if out["git-form-empty"] == out["git-form-filled"] {
		return nil, fmt.Errorf("screenshot: CaptureGitScreenScreens: git-form-empty captured the same frame as git-form-filled — the second identity was never reached")
	}
	return out, nil
}

// gitScreenSpecs returns the five Phase 4 git-screen checkpoint specs — the
// SAME checkpoint vocabulary e2e/git_configuration_pty_e2e_test.go's
// TestGitConfiguration_CompiledRealVsLiveDummyPTY (Task 2) uses over real
// PTYs. ApplicableApprovedHTML is false throughout: D-12 makes
// cmd/gitid-dummy the sole Phase 4 UI/UX reference — no HTML/MUI/browser
// capture participates.
//
// RegionDispositions mirror
// .planning/design/git-screen/visual-divergence-allowlist.txt's classified
// entries verbatim (kept in sync by
// TestGitScreenAllowlistMatchesRegistry in cmd/gitid/gate_visual_regression_test.go) —
// this is the mechanism BuildRegionDiffs/ValidateRegionDiffs actually
// consult (the same pattern the create-flow registry above already uses),
// generalized to accept CTX-D-NN decision refs (04-CONTEXT.md) alongside the
// create-flow registry's bare D-NN/T-NN vocabulary.
// isGitScreenID reports whether id is one of the five Phase 4 git-screen
// checkpoint IDs — used to exclude them from CaptureCreateFlowScreens'
// completeness check (they are captured separately; see gitScreenSpecs'
// doc comment).
func isGitScreenID(id string) bool {
	for _, spec := range gitScreenSpecs() {
		if spec.ScreenID == id {
			return true
		}
	}
	return false
}

func gitScreenSpecs() []ScreenSpec {
	// WR-27: these dispositions gain a real Predicate, ported verbatim from
	// .planning/design/git-screen/visual-divergence-allowlist.txt — the
	// SAME strings/grammar the e2e gate (gitScreenPredicateSatisfied)
	// already enforces for these exact checkpoint/region pairs, so this
	// in-process gate now narrows to the identical divergence rather than
	// accepting any future difference in the region. Before this fix, all 32
	// dispositions across this file used blanket uxRegionDifference with an
	// empty Predicate — uxRegionDifferenceScoped had zero production callers,
	// so WR-19's rejection branch in BuildRegionDiffs never actually fired.
	// CR-14 (iteration 4) subsequently re-scoped gitPreviewDisposition's
	// Predicate from `absent:"gitdir:~/git/"` to `contains:"gitdir:~/git/"`
	// — see its own comment below for why the divergence it authorizes
	// changed shape.
	fixtureSidebarDisposition := uxRegionDifferenceScoped(RegionSidebar, "sidebar-state", "CTX-D-12",
		"real sidebar carries only the checkpoint's own seeded identities; dummy sidebar lists the full 8-identity IdentityManagerRows fixture set",
		`absent:"clientB"`)
	fixtureHeaderStatusDisposition := uxRegionDifferenceScoped(RegionHeaderStatus, "identity-count", "CTX-D-12",
		"header status shows the identity count, which differs (real's small seeded set vs dummy's 8 fixtures)",
		`contains:"ids"`)
	// CR-14 (iteration 4): FixtureBackend.IncludeIfPreview now substitutes
	// spec.GitDir (the real "~/git/<identity>/" D-02 derivation) instead of
	// a frozen "~/<identity>/" literal, so the CTX-D-02 gitdir-default
	// divergence this disposition used to authorize no longer occurs — real
	// and dummy now agree on the SAME derivation. What remains is purely the
	// identity-name substitution (fragment path + includeIf condition embed
	// the selected identity), the SAME CTX-D-12 class breadcrumbDisposition/
	// gitStrategyDisposition already cover. The `absent:"gitdir:~/git/"`
	// predicate is replaced with `contains:"gitdir:~/git/"` rather than
	// dropped to bare/blanket: under CR-10's fixed grammar, contains:
	// requires the marker on BOTH sides, so this actively guards against
	// CR-14's exact regression recurring — if the dummy's derivation ever
	// regresses back to a frozen "~/<identity>/" literal, only the real side
	// would carry "gitdir:~/git/" and this predicate correctly rejects it.
	gitPreviewDisposition := uxRegionDifferenceScoped(RegionGitPreview, "identity-name", "CTX-D-12",
		"the includeIf preview's fragment path and gitdir condition (\"gitdir:~/git/<identity>/\") embed the selected identity's name, which differs between the real fixture (\"gscreen\"/\"gscreenssh\") and the dummy fixture (\"personal\"/\"work\") by construction — CR-14 fixed the dummy's frozen preview to derive the same \"~/git/<identity>/\" shape D-02 requires, so no other content differs",
		`contains:"gitdir:~/git/"`)
	formFieldsDisposition := uxRegionDifferenceScoped(RegionGitFormFields, "author-name-template", "CTX-D-01",
		"the real fixture's seeded author name (\"<identity> User\") and the dummy's frozen fixture (\"<identity> identity\") use different literal text from two independently authored test fixtures; field structure/order is identical",
		`absent:"User"`)
	emptyFormFieldsDisposition := uxRegionDifference(RegionGitFormFields, "identity-name", "CTX-D-12",
		"the empty form's compact metadata line (\"signingkey=~/.ssh/id_ed25519_<identity>.pub\") embeds the selected identity's name, which differs between the real and dummy fixtures by construction — field structure/order is identical")
	breadcrumbDisposition := uxRegionDifference(RegionBreadcrumb, "identity-name", "CTX-D-12",
		"the breadcrumb (\"Identities › <identity> › Configure Git\") embeds the selected identity's name, which differs between the real fixture (\"gscreen\"/\"gscreenssh\") and the dummy fixture (\"personal\"/\"work\") by construction")
	gitStrategyDisposition := uxRegionDifference(RegionGitStrategy, "identity-name", "CTX-D-12",
		"the gitdir strategy option's label (\"gitdir (default) — applies inside ~/<identity>/\") embeds the selected identity's name, which differs between the real and dummy fixtures by construction — same label text/structure otherwise")
	noHTML := []SurfaceNonApplicability{uxNonComparable("approved-html", "CTX-D-12",
		"D-12: cmd/gitid-dummy is the sole Phase 4 UI/UX reference — no HTML/MUI/browser capture participates in Phase 4 acceptance")}

	return []ScreenSpec{
		{
			ScreenID:              "git-form-filled",
			Interaction:           "Boot the Identities pane on the default-selected (complete) identity and press 'g' to open Configure Git in edit mode.",
			StateMarker:           "editing existing fragment",
			ApplicableLive:        true,
			ApplicableApprovedTUI: true,
			NonApplicability:      noHTML,
			RequiredRegions:       []RegionName{RegionGitFormFields, RegionGitPreview},
			RegionDispositions:    []RegionDisposition{fixtureSidebarDisposition, fixtureHeaderStatusDisposition, gitPreviewDisposition, formFieldsDisposition, breadcrumbDisposition, gitStrategyDisposition},
		},
		{
			ScreenID:              "git-form-empty",
			Interaction:           "From the Identities pane, select the second (SSH-only) identity and press 'g' to open Configure Git's SSH-only completion path.",
			StateMarker:           "completes this identity",
			ApplicableLive:        true,
			ApplicableApprovedTUI: true,
			NonApplicability:      noHTML,
			RequiredRegions:       []RegionName{RegionGitPreview},
			RegionDispositions:    []RegionDisposition{fixtureSidebarDisposition, fixtureHeaderStatusDisposition, gitPreviewDisposition, breadcrumbDisposition, gitStrategyDisposition, emptyFormFieldsDisposition},
		},
		{
			ScreenID:              "match-strategy-select",
			Interaction:           "From git-form-filled, Tab twice (name -> email -> strategy) to focus the match-strategy field at its default (gitdir).",
			StateMarker:           "gitdir (default)",
			ApplicableLive:        true,
			ApplicableApprovedTUI: true,
			NonApplicability:      noHTML,
			RequiredRegions:       []RegionName{RegionGitStrategy, RegionGitPreview},
			RegionDispositions:    []RegionDisposition{fixtureSidebarDisposition, fixtureHeaderStatusDisposition, gitPreviewDisposition, formFieldsDisposition, breadcrumbDisposition, gitStrategyDisposition},
		},
		{
			ScreenID:              "review-readonly",
			Interaction:           "From git-form-filled, press Enter to reach the read-only write-ceremony preview.",
			StateMarker:           "Write Git identity for",
			ApplicableLive:        true,
			ApplicableApprovedTUI: true,
			NonApplicability:      noHTML,
			RequiredRegions:       []RegionName{RegionGitCeremony},
			RegionDispositions: []RegionDisposition{
				fixtureSidebarDisposition, fixtureHeaderStatusDisposition, breadcrumbDisposition,
				// WR-27: predicate ported verbatim from the allowlist's
				// review-readonly:git-ceremony entry.
				uxRegionDifferenceScoped(RegionGitCeremony, "sentinel-wrapped-preview", "CTX-D-12",
					"the real ceremony preview renders the production sentinel-wrapped includeIf block (gitconfig.RenderIncludeIf); the dummy's frozen sample has no sentinels and predates the production renderer",
					`absent:"BEGIN gitid managed"`),
				// internal/tuikit/ceremony.go's "Exact change: …" hint is a
				// STATIC line always rendered on the review pane — the SAME
				// shared ceremony code create-flow's own confirm-write screen
				// uses, so RegionConfirmationPreview (a create-flow-scoped
				// region) also picks up this git-screen ceremony's content.
				// Same divergence, same reason as RegionGitCeremony above.
				//
				// WR-27: intentionally left BLANKET (no Predicate) — the
				// allowlist file's own schema does not list "confirmation-
				// preview" as a valid region at all (only "git-ceremony" is),
				// so there is no allowlist-sourced predicate string to port
				// here without independently deriving/verifying one. Scoping
				// this one is a genuine follow-up, not silently declared done.
				uxRegionDifference(RegionConfirmationPreview, "sentinel-wrapped-preview", "CTX-D-12",
					"internal/tuikit/ceremony.go's shared \"Exact change\" hint triggers RegionConfirmationPreview's extraction on this git-screen ceremony too; the real preview's production sentinels vs the dummy's sentinel-less frozen sample is the SAME divergence RegionGitCeremony already classifies"),
			},
		},
		{
			ScreenID:              "result-success",
			Interaction:           "From review-readonly, press Enter to confirm the write and reach the result receipt.",
			StateMarker:           "configured",
			ApplicableLive:        true,
			ApplicableApprovedTUI: true,
			NonApplicability:      noHTML,
			RequiredRegions:       []RegionName{RegionGitCeremony},
			RegionDispositions: []RegionDisposition{
				fixtureSidebarDisposition, fixtureHeaderStatusDisposition, breadcrumbDisposition,
				// WR-27: predicate ported verbatim from the allowlist's
				// result-success:git-ceremony entry.
				uxRegionDifferenceScoped(RegionGitCeremony, "backup-receipt-completeness", "CTX-D-12",
					"the real commit receipt lists every backup actually taken (including the pre-existing fragment file's own \".bak.\"-suffixed backup); the dummy's ceremony echoes its static 2-entry declared backup list, which never names a fragment backup",
					`absent:".bak."`),
				// extractKeybar's "last 3 non-empty lines" heuristic includes
				// the receipt's "Git identity "<identity>" configured." echo
				// line (below the ceremony body), which also embeds the
				// identity name.
				uxRegionDifference(RegionKeybar, "identity-name", "CTX-D-12",
					"the receipt echo line (\"Git identity \\\"<identity>\\\" configured.\") embeds the selected identity's name, which differs between the real and dummy fixtures by construction — the keybar chrome itself (Tab/Esc hints) is byte-identical"),
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Identity-manager checkpoints (05-09-PLAN.md Task 3, Phase 5 registration).
//
// CaptureIdentityManagerScreens drives backend's Identities pane in-process
// through FOUR of the six checkpoints
// e2e/identity_manager_pty_e2e_test.go's
// TestIdentityManager_CompiledRealVsLiveDummyPTY (05-09-PLAN.md Task 2)
// drives over real PTYs: action-menu, delete-choice, confirm-destructive,
// and detail-ssh-first. rotate-result and repair-result are DELIBERATELY
// NOT registered here: both require a real (or FakeSSHDir-substituted) SSH
// connectivity probe (cmd/gitid/lifecycle.go's preWriteGate), which needs a
// subprocess PATH override this in-process, no-subprocess gate has no way
// to inject — forcing a real network-dependent probe into this FAST,
// deterministic supplementary check would defeat its own purpose (and this
// plan's own CR-01-style determinism requirement). The e2e PTY suite
// (Task 1's TestIdentityManager_KeyCeremonyRotate/Repair and Task 2's own
// rotate-result/repair-result checkpoints) already carries the real DLV-06/
// DLV-04 evidence for both ceremonies with FakeSSHDir substituted.
//
// backend must expose at least two identities: the default-selected
// identity (index 0, a COMPLETE identity) drives action-menu/delete-choice/
// confirm-destructive; the SECOND identity (index 1, SSH-only) drives
// detail-ssh-first.
// ---------------------------------------------------------------------------

// identityManagerApp boots a fresh tuikit.App around backend at the fixed
// capture geometry, landing on the Identities detail pane.
func identityManagerApp(backend tuikit.Backend) tea.Model {
	var model tea.Model = tuikit.NewApp(backend)
	model = step(model, tea.WindowSizeMsg{Width: CaptureWidth, Height: CaptureHeight})
	return model
}

// CaptureIdentityManagerScreens implements the doc comment above.
func CaptureIdentityManagerScreens(backend tuikit.Backend) (map[string]string, error) {
	if n := len(backend.InitialState().Identities); n < 2 {
		return nil, fmt.Errorf("screenshot: CaptureIdentityManagerScreens requires >= 2 seeded identities, got %d", n)
	}
	out := make(map[string]string, 4)
	capture := func(m tea.Model) string { return normalizeTimestamps(anyView(m)) }

	// action-menu: default-selected (complete) identity, 'a' opens the menu.
	m := identityManagerApp(backend)
	actions := keyRune(m, 'a')
	out["action-menu"] = capture(actions)

	// delete-choice: 'd' opens the git-only-default scope chooser.
	del := keyRune(m, 'd')
	out["delete-choice"] = capture(del)

	// confirm-destructive: Down selects "everything", Enter opens the
	// ceremony's confirm state (still pre-confirm — the preview/backup
	// promise, never the actual write).
	everything := keyDown(del)
	confirm := keyEnter(everything)
	out["confirm-destructive"] = capture(confirm)

	// detail-ssh-first: the second (SSH-only) identity's live detail pane.
	detail := identityManagerApp(backend)
	detail = keyDown(detail)
	out["detail-ssh-first"] = capture(detail)

	for _, spec := range identityManagerSpecs() {
		text, ok := out[spec.ScreenID]
		if !ok || strings.TrimSpace(text) == "" {
			return nil, fmt.Errorf("screenshot: CaptureIdentityManagerScreens: required frame %q is missing or empty", spec.ScreenID)
		}
	}
	if out["detail-ssh-first"] == out["action-menu"] {
		return nil, fmt.Errorf("screenshot: CaptureIdentityManagerScreens: detail-ssh-first captured the same frame as action-menu — the second identity was never reached")
	}
	return out, nil
}

// isIdentityManagerScreenID reports whether id is one of the four Phase 5
// identity-manager checkpoint IDs registered here — used to exclude them
// from CaptureCreateFlowScreens' completeness check (they are captured
// separately; see CaptureIdentityManagerScreens' doc comment).
func isIdentityManagerScreenID(id string) bool {
	for _, spec := range identityManagerSpecs() {
		if spec.ScreenID == id {
			return true
		}
	}
	return false
}

func identityManagerSpecs() []ScreenSpec {
	// DLV-4 (never a numbered D-NN): this class of divergence is a property
	// of the comparison MECHANISM itself (the dummy's frozen, always-8-
	// identity fixture set vs. the real backend's minimal seeded set),
	// governed directly by the DLV-04 requirement — mirrors
	// e2e/identity_manager_pty_e2e_test.go's own allowlist citation for the
	// SAME divergence class.
	fixtureSidebarDisposition := uxRegionDifferenceScoped(RegionSidebar, "sidebar-state", "DLV-4",
		"real sidebar carries only the checkpoint's own seeded identities; dummy sidebar lists the full 8-identity IdentityManagerRows fixture set",
		`absent:"clientB"`)
	fixtureHeaderStatusDisposition := uxRegionDifferenceScoped(RegionHeaderStatus, "identity-count", "DLV-4",
		"header status shows the identity count, which differs (real's small seeded set vs dummy's 8 fixtures)",
		`contains:"ids"`)
	// breadcrumbDisposition mirrors gitScreenSpecs' own breadcrumbDisposition:
	// the breadcrumb ("Identities › <identity> › …") embeds the selected
	// identity's name, which differs between the real fixture ("imgr"/
	// "imgrssh") and the dummy fixture ("personal"/"work") by construction.
	breadcrumbDisposition := uxRegionDifference(RegionBreadcrumb, "identity-name", "DLV-4",
		"the breadcrumb (\"Identities › <identity> › …\") embeds the selected identity's name, which differs between the real fixture (\"imgr\"/\"imgrssh\") and the dummy fixture (\"personal\"/\"work\") by construction")
	noHTML := []SurfaceNonApplicability{uxNonComparable("approved-html", "DLV-4",
		"cmd/gitid-dummy is the sole Phase 5 UI/UX reference for this in-process gate, matching the e2e PTY suite's own DLV-04 comparison — no HTML/MUI/browser capture participates in Phase 5 acceptance")}

	return []ScreenSpec{
		{
			ScreenID:              "action-menu",
			Interaction:           "Boot the Identities pane on the default-selected (complete) identity and press 'a' to open the action menu.",
			StateMarker:           "Actions — ",
			ApplicableLive:        true,
			ApplicableApprovedTUI: true,
			NonApplicability:      noHTML,
			RequiredRegions:       []RegionName{RegionActionMenuRows},
			RegionDispositions: []RegionDisposition{
				fixtureSidebarDisposition, fixtureHeaderStatusDisposition, breadcrumbDisposition,
				uxRegionDifferenceScoped(RegionActionMenuRows, "identity-name", "DLV-4",
					"the action-menu heading (\"Actions — <identity>\") embeds the selected identity's name, which differs between the real fixture (\"imgr\") and the dummy fixture (\"personal\") by construction — the four row labels below it are identical",
					`contains:"Actions — "`),
			},
		},
		{
			ScreenID:              "delete-choice",
			Interaction:           "From the Identities pane, press 'd' to open the delete-scope chooser.",
			StateMarker:           "choose scope",
			ApplicableLive:        true,
			ApplicableApprovedTUI: true,
			NonApplicability:      noHTML,
			RequiredRegions:       []RegionName{RegionDeleteChoiceOptions},
			RegionDispositions: []RegionDisposition{
				fixtureSidebarDisposition, fixtureHeaderStatusDisposition, breadcrumbDisposition,
				uxRegionDifferenceScoped(RegionDeleteChoiceOptions, "identity-name", "DLV-4",
					"the delete-choice heading (\"Delete \\\"<identity>\\\" — choose scope\") embeds the selected identity's name, which differs between the real fixture (\"imgr\") and the dummy fixture (\"personal\") by construction — the two scope options below it are identical",
					`contains:"choose scope"`),
			},
		},
		{
			ScreenID:              "confirm-destructive",
			Interaction:           "From delete-choice, press Down to select everything scope then Enter to reach the confirm-destructive ceremony's pre-confirm preview.",
			StateMarker:           "This action is irreversible",
			ApplicableLive:        true,
			ApplicableApprovedTUI: true,
			NonApplicability:      noHTML,
			RequiredRegions:       []RegionName{RegionConfirmWarningBlock},
			RegionDispositions: []RegionDisposition{
				fixtureSidebarDisposition, fixtureHeaderStatusDisposition, breadcrumbDisposition,
				uxRegionDifferenceScoped(RegionConfirmWarningBlock, "key-copy-completeness", "MGR-D-11",
					"the real confirm pane names the D-11 key-copy sentence and the real archive-dir backup line (DeletePlan.KeyCopyPath, sshconfig.ArchiveDir); the dummy's FixtureBackend.DeletePlan never sets KeyCopyPath and echoes its own static declared backup list instead — the dummy fixture predates the D-11 requirement",
					`contains:"This action is irreversible"`),
				uxRegionDifferenceScoped(RegionBackupPathList, "key-copy-completeness", "MGR-D-11",
					"same divergence as RegionConfirmWarningBlock above: the real backup-promise line names the D-06 archive directory; the dummy's static declared backup list names file-level paths instead",
					`contains:"Backup"`),
				// git-screen's own RegionGitPreview extractor (extractGitPreview)
				// starts on ANY line containing "fragment file" — a marker this
				// ceremony's OWN target-list line ("- ~/.gitconfig.d/<identity>
				// (Git fragment file)") also contains, so BuildRegionDiffs (which
				// checks every named region against every screen regardless of
				// RequiredRegions) picks it up here too. Same identity-name
				// divergence class as RegionConfirmWarningBlock above.
				uxRegionDifferenceScoped(RegionGitPreview, "extraction-boundary-asymmetry", "DLV-4",
					"git-screen's RegionGitPreview extractor (bounded by a \"Write it\" end marker this delete ceremony never renders — its confirm button reads \"Delete (Enter)\") extracts this ceremony's target-list content asymmetrically between the two ANSI-styled renders; both sides carry the SAME target-list content verbatim (verified directly against RegionConfirmWarningBlock above), this is purely an extraction-boundary artifact of a region built for a different ceremony shape",
					`absent:"fragment file"`),
			},
		},
		{
			ScreenID:              "detail-ssh-first",
			Interaction:           "From the Identities pane, select the second (SSH-only) identity — its detail pane renders the SSH-first section with no fabricated Git fields.",
			StateMarker:           "SSH — shown first, always",
			ApplicableLive:        true,
			ApplicableApprovedTUI: true,
			NonApplicability:      noHTML,
			RequiredRegions:       []RegionName{RegionDetailSSHSection, RegionDetailGitSection, RegionDetailFindingsSection},
			RegionDispositions: []RegionDisposition{
				fixtureSidebarDisposition, fixtureHeaderStatusDisposition, breadcrumbDisposition,
				uxRegionDifferenceScoped(RegionDetailSSHSection, "fixture-literal-values", "DLV-6",
					"the real fixture (no explicit Port) renders the honest absence marker per observedPort; the dummy's frozen fixture carries a recipe-shape Hostname/Port pair — different fixture literal values, same field structure/order/labels",
					// "Hostname:" (a PLAIN label, not the styled/underlined
					// section heading) — the heading itself is rendered with
					// PER-CHARACTER ANSI styling, so a multi-character needle
					// spanning it never survives as a contiguous substring.
					`contains:"Hostname:"`),
				uxRegionDifferenceScoped(RegionDetailFindingsSection, "real-vs-frozen-findings", "DLV-6",
					"MGR-07 findings are real per-identity health on the real side (one real \"no-gitconfig-includeif-block\" warning for the SSH-only fixture) vs. the dummy's frozen zero-findings demo data — the section heading's own count differs too, but its underlined styling renders per-character (a multi-character needle spanning it can never survive as a contiguous plain substring), so the predicate below is scoped to the body content instead",
					`absent:"No findings for"`),
				uxRegionDifferenceScoped(RegionKeybar, "identity-count", "DLV-4",
					"the status line's identity count (\"N identities — selection renders…\") differs (real's 2-identity seeded set vs dummy's 8 fixtures) — the keybar chrome itself (Esc/Ctrl+P hints) is byte-identical",
					`contains:"identities"`),
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Global SSH checkpoints (06-07-PLAN.md Task 1, Phase 6 registration).
//
// CaptureGlobalSSHScreens drives backend's Global SSH tab (view 2) in-process
// through the SAME semantic checkpoints
// e2e/global_ssh_pty_e2e_test.go's TestGlobalSSH_RealPTY* suite (Task 3 of
// 06-04) and e2e/global_ssh_storage_pty_e2e_test.go's TestGlobalSSHStorage_*
// suite (06-05 Task 3) drive over real PTYs — six named checkpoints derived
// from 06-UI-SPEC.md's Approved Base States table:
//
//   - gss-options-list           Options sub-tab in browse mode (master-detail).
//   - gss-storage-current        Storage & preview sub-tab in browse mode with
//                                the radio on the CURRENT layout (real fixture
//                                home is Include-layout, so the real side holds
//                                the Include radio; the dummy fixture home is
//                                Sentinel-layout, so the dummy side holds the
//                                Sentinel radio — the layout-selection state
//                                itself is part of the fixture-vs-live DLV-4
//                                class, see gssStorageFixtureDisposition).
//   - gss-storage-other          Storage & preview sub-tab in browse mode with
//                                the radio on the OTHER (non-current) layout.
//   - gss-apply-preview          The apply ceremony's state A (pre-write diff).
//   - gss-apply-receipt          NON-APPLICABLE in this in-process gate — the
//                                receipt requires the real journal-backed write
//                                (runGlobalSSHApply, plan 06-04) to have
//                                happened, which this no-subprocess capture path
//                                never performs. Evidence lives in the PTY
//                                frame .planning/phases/06-global-ssh-options/
//                                ui-frames/global-ssh-apply-confirm.txt
//                                (TestGlobalSSH_RealPTYApplyConfirm).
//   - gss-storage-migrate-preview The storage-migration ceremony's state A
//                                (STORE-03 pre-write diff).
//   - gss-storage-migrate-receipt NON-APPLICABLE in this in-process gate — the
//                                receipt requires the STORE-03 two-file write
//                                (runSSHStorageMigrate, plan 06-05) to have
//                                happened. Evidence lives in the PTY frame
//                                .planning/phases/06-global-ssh-options/
//                                ui-frames/storage-migrate-confirm-post.txt.
//
// backend must satisfy tuikit.GlobalSSHPlanner and tuikit.SSHStoragePlanner
// (both real cmd/gitid and dummytui.NewFixtureBackend do). The real backend's
// option states run the D-01 ssh -G/ssh -V probes against the seeded fixture
// home; the dummy returns its frozen fixture rows.
// ---------------------------------------------------------------------------

// globalSSHApp boots a fresh tuikit.App around backend at the fixed capture
// geometry and activates the Global SSH tab — the SAME '2' ActivationKey a
// real user presses (cmd/gitid/app.go's setTab(TabGlobalSSH)).
func globalSSHApp(backend tuikit.Backend) tea.Model {
	var model tea.Model = tuikit.NewApp(backend)
	model = step(model, tea.WindowSizeMsg{Width: CaptureWidth, Height: CaptureHeight})
	return keyRune(model, '2')
}

// CaptureGlobalSSHScreens implements the doc comment above.
func CaptureGlobalSSHScreens(backend tuikit.Backend) (map[string]string, error) {
	out := make(map[string]string, 6)
	capture := func(m tea.Model) string { return normalizeTimestamps(anyView(m)) }

	// gss-options-list: the Options sub-tab in browse mode (default entry).
	browse := globalSSHApp(backend)
	out["gss-options-list"] = capture(browse)

	// gss-storage-current / gss-storage-other: the Storage & preview sub-tab
	// in browse mode under each layout. '→' switches to Storage at the CD
	// current layout; '↓' flips the radio to the other layout and refetches
	// its STORE-03 plan.
	current := keyRight(browse)
	out["gss-storage-current"] = capture(current)
	other := keyDown(current)
	out["gss-storage-other"] = capture(other)

	// gss-apply-preview: move UP from the default-selected IdentitiesOnly row
	// (verify-only, not selectable) to StrictHostKeyChecking — the SAME
	// navigation e2e/global_ssh_pty_e2e_test.go's openGlobalSSHPreview uses
	// (3 up-arrows) — toggle it, and open the apply ceremony at its pre-write
	// state A. The real ceremony renders the EnsureGlobals block diff
	// (T-06-GLOBALBLOCK); the dummy's empty GlobalSSHApplyPlan falls back to
	// its flat "+ Key Recommended" list.
	apply := globalSSHApp(backend)
	apply = keyUp(apply)
	apply = keyUp(apply)
	apply = keyUp(apply)
	apply = keyRune(apply, ' ')
	apply = keyRune(apply, 'a')
	out["gss-apply-preview"] = capture(apply)

	// gss-storage-migrate-preview: Storage sub-tab, flip the layout radio
	// (choice != current is what arms the migrate action), then Enter opens
	// the STORE-03 migration ceremony at its pre-write state A.
	migrate := globalSSHApp(backend)
	migrate = keyRight(migrate)
	migrate = keyDown(migrate)
	migrate = keyEnter(migrate)
	out["gss-storage-migrate-preview"] = capture(migrate)

	for _, spec := range globalSSHSpecs() {
		// The two receipt states are registered non-applicable on BOTH
		// surfaces (they need a real write neither side performs in-process);
		// nothing is captured for them, by design — never a hollow frame.
		if !spec.ApplicableLive && !spec.ApplicableApprovedTUI {
			continue
		}
		text, ok := out[spec.ScreenID]
		if !ok || strings.TrimSpace(text) == "" {
			return nil, fmt.Errorf("screenshot: CaptureGlobalSSHScreens: required frame %q is missing or empty", spec.ScreenID)
		}
	}
	// A fixture/home bug that leaves the two storage browse frames identical
	// (or the apply ceremony stuck in browse mode) would pass the non-emptiness
	// loop above; assert the states actually differ.
	if out["gss-storage-current"] == out["gss-storage-other"] {
		return nil, fmt.Errorf("screenshot: CaptureGlobalSSHScreens: the two storage-layout browse frames are identical — the layout radio never moved")
	}
	if out["gss-apply-preview"] == out["gss-options-list"] {
		return nil, fmt.Errorf("screenshot: CaptureGlobalSSHScreens: gss-apply-preview captured the same frame as gss-options-list — the apply ceremony never opened")
	}
	return out, nil
}

// isGlobalSSHScreenID reports whether id is one of the Phase 6 Global SSH
// checkpoint IDs registered here — used to exclude them from
// CaptureCreateFlowScreens' completeness check (they are captured separately;
// see CaptureGlobalSSHScreens' doc comment).
func isGlobalSSHScreenID(id string) bool {
	for _, spec := range globalSSHSpecs() {
		if spec.ScreenID == id {
			return true
		}
	}
	return false
}

// globalSSHSpecs returns the Phase 6 Global SSH checkpoint specs — the SAME
// vocabulary 06-UI-SPEC.md's Approved Base States table names and 06-04/06-05's
// real-PTY suites drive over real PTYs. ApplicableApprovedHTML is false
// throughout: per AGENTS.md's BINDING UI Reference rule (recorded in the
// authority of 06-07-PLAN.md), Phase 2's approved Bubble Tea dummy is the SOLE
// Phase 6 UI/UX parity target (Phases 3-10), and the historical HTML/MUI
// artifacts are Phase-2 design history — made EXPLICIT on every spec rather
// than left to the registry's default (T-06-45).
//
// RegionDispositions mirror
// .planning/design/global-ssh/visual-divergence-allowlist.txt's classified
// entries verbatim (kept in sync by
// TestGlobalSSHAllowlistMatchesRegistry in cmd/gitid/gate_visual_regression_test.go).
func globalSSHSpecs() []ScreenSpec {
	// DLV-4 (never a numbered D-NN): the fixture-vs-live comparison-class
	// divergence — the real backend reads a seeded fixture home, the dummy's
	// FixtureBackend always renders its frozen 8-identity/static fixture set.
	// Governed directly by the DLV-04 requirement; the SAME literal the Phase
	// 5 identity-manager registry uses for the identical class.
	gssFixtureClass := "DLV-4"
	fixtureHeaderStatusDisposition := uxRegionDifferenceScoped(RegionHeaderStatus, "identity-count", gssFixtureClass,
		"header status shows the identity count, which differs (real's zero-identity seeded Global SSH fixture home vs the dummy's 8-identity IdentityManagerRows fixture set)",
		`contains:"ids"`)
	// gssOptionsFixtureDisposition authorizes the whole Options master-detail
	// body as the D-01/D-03-provenance + D-11/D-12-row-state fixture-vs-live
	// divergence: the real body carries the live three-tier provenance and
	// the machine's four-state row renderings while the dummy body carries
	// its frozen fixture values and its "the demo does not probe this
	// machine" provenance label (T-06-PROVENANCE). The predicate is
	// `absent:` on the dummy's frozen row formulation "not set (OpenSSH
	// default: ask)" (StrictHostKeyChecking's fixture Current) — present in
	// the visible pane on the dummy side, never rendered by the real side,
	// whose bare baseline already says "None" as "now: ask → …" without the
	// "OpenSSH default: " prefix.
	gssOptionsFixtureDisposition := uxRegionDifferenceScoped(RegionGSSOptionsBrowse, "provenance-state-and-rows", gssFixtureClass,
		"the real Options body renders the live D-01/D-03 provenance labels and the D-11/D-12 four-state rows against the seeded fixture home; the dummy renders its frozen GlobalSSHOptions Current values (its \"not set (OpenSSH default: ask)\" StrictHostKeyChecking formulation is the visible-pane needle) and its 'the demo does not probe this machine' provenance — the whole master-detail body is the classified fixture-vs-live divergence (T-06-PROVENANCE)",
		`absent:"not set (OpenSSH default: ask)"`)
	// gssListFixtureDisposition covers RegionSidebar's extraction on this
	// surface: its 'content before │' rule captures the OPTION-LIST rows
	// (this surface has no identity sidebar), which differ exactly as the
	// gss-options-browse rows above do.
	gssListFixtureDisposition := uxRegionDifferenceScoped(RegionSidebar, "fixture-vs-probe-rows", gssFixtureClass,
		"on this surface RegionSidebar's left-of-│ extraction captures the OPTION LIST rows, not an identity sidebar — the real row set describes the live probe while the dummy rows carry its frozen fixture now-values; the list text is what the gate compares, and 'now:' survives on both sides",
		`contains:"now:"`)
	// gssStorageFixtureDisposition covers RegionSidebar on the Storage
	// sub-tab: the left pane's radio/current markers differ because the real
	// fixture home is Include-layout and the dummy fixture home is always
	// Sentinel-layout (STORE-01 label is the shared anchor).
	gssStorageFixtureDisposition := uxRegionDifferenceScoped(RegionSidebar, "storage-layout-radio-state", gssFixtureClass,
		"the Storage left pane's STORE-01 label, radios and current-layout marker render the backend's live layout choice (real fixture home = Include; dummy fixture = Sentinel) — identical STORE-01 chrome, radio/marker state differs by construction",
		`contains:"STORE-01"`)
	// gssStoragePreviewDisposition authorizes the Storage right-pane resulting-
	// config previews: the real side plans the migration from the seeded
	// machine bytes while the dummy renders its frozen personal/work + managed
	// globals-block fixture previews (T-06-GLOBALBLOCK's frozen-side fixture
	// is here). "Host personal.github.com" is the dummy-only fixture identity
	// needle.
	gssStoragePreviewDisposition := uxRegionDifferenceScoped(RegionGSSStorageBrowse, "fixture-preview-vs-planned", gssFixtureClass,
		"the real resulting-config preview is PlanMigration's compose of the seeded machine bytes (Include line, zero managed identities); the dummy renders its frozen personal/work IncludePreviewOwned/SentinelPreview fixtures, including the managedHostStar globals block's four-space fixture rendering (06-01's recorded T-06-GLOBALBLOCK fixture side)",
		`absent:"Host personal.github.com"`)
	// gssApplyCeremonyDisposition is the T-06-GLOBALBLOCK required entry's
	// enforcement: the real apply-ceremony diff renders the EnsureGlobals
	// managed block (IgnoreUnknown UseKeychain guard FIRST, two-space body
	// indents, Policy-ordered keys — 06-01's recorded real side); the dummy's
	// empty GlobalSSHApplyPlan falls back to a flat "+ Key Recommended" list
	// that never renders the block at all, so the guard needle is present on
	// the real side ONLY.
	gssApplyCeremonyDisposition := uxRegionDifferenceScoped(RegionGSSApplyCeremony, "managed-globals-block-diff", "GSSH-D-06",
		"the real apply-ceremony diff is the EnsureGlobals write target: 'IgnoreUnknown UseKeychain' as the guard line BEFORE 'Host *', two-space hostIndent body, Policy-ordered keys (T-06-GLOBALBLOCK, 06-01); the dummy ceremony (empty GlobalSSHApplyPlan) lists '+ Key Recommended' flat lines and never renders the block — removing either the guard-line placement, the two-vs-four-space indent, or the Policy-vs-selection key order from the real renderer would break this needle",
		`absent:"IgnoreUnknown UseKeychain"`)
	// gssApplyHeadingDisposition is the T-06-CEREMONYTARGET required entry's
	// enforcement: the D-07 resolved-target ceremony heading shares the frozen
	// "Write Host * managed block to " prefix on both sides while its target
	// tail is the resolved file (real: Include'd gitid.config; dummy: the
	// ~/.ssh/config fallback).
	gssApplyHeadingDisposition := uxRegionDifferenceScoped(RegionGSSApplyHeading, "resolved-target-file", "GSSH-D-07",
		"the apply ceremony's heading names the RESOLVED storage target (D-07, T-06-CEREMONYTARGET) — the real side resolves ~/.ssh/config.d/gitid.config (Include'd layout), the dummy falls back to ~/.ssh/config; the shared 'Write Host * managed block to ' prefix anchors both sides",
		`contains:"Write Host * managed block to "`)
	// gssStorageCeremonyDisposition authorizes the STORE-03 migration
	// ceremony's diff body: the real plan diff vs the dummy's frozen fixture
	// diff, sharing the "Migrate SSH storage layout" heading on both sides.
	gssStorageCeremonyDisposition := uxRegionDifferenceScoped(RegionGSSStorageCeremony, "store-03-plan-diff", "STORE-03",
		"the real storage-migration ceremony diff is PlanMigration's actual STORE-03 plan for the seeded machine; the dummy renders its frozen fixture diff — the shared heading anchors the authorized divergence",
		`contains:"Migrate SSH storage layout"`)

	// noHTML is the explicit approved-HTML non-applicability record EVERY
	// Global SSH spec carries, stating the standing UI-reference rule by name
	// rather than leaving HTML parity to the registry's default (T-06-45).
	noHTML := []SurfaceNonApplicability{uxNonComparable("approved-html", "DLV-4",
		"AGENTS.md's BINDING UI Reference rule: Phase 2's approved Bubble Tea dummy is the SOLE Phase 6 UI/UX parity target for Phases 3-10; the historical HTML/MUI artifacts are Phase-2 design history and are recorded explicitly NON-APPLICABLE for every Phase 6 comparison")}

	// receiptNA builds the non-applicability records for the two receipt
	// states: they need a real write this no-subprocess in-process gate never
	// performs, so each names the specific PTY frame file that carries that
	// evidence instead (a hollow in-process frame would be worse than an
	// accurate non-applicability record — Phase 5's rotate-result/repair-result
	// precedent).
	receiptNA := func(decision, reason string) []SurfaceNonApplicability {
		return append(noHTML,
			uxNonComparable("live", decision, reason),
			uxNonComparable("approved-tui", decision, reason),
		)
	}

	applyReceiptReason := "the apply receipt requires the real journal-backed write (runGlobalSSHApply, plan 06-04) to have completed; this in-process, no-subprocess capture path never performs it. Evidence lives in the PTY frame .planning/phases/06-global-ssh-options/ui-frames/global-ssh-apply-confirm.txt (TestGlobalSSH_RealPTYApplyConfirm)"
	storageReceiptReason := "the storage-migration receipt requires the STORE-03 two-file write (runSSHStorageMigrate, plan 06-05) to have completed; this in-process, no-subprocess capture path never performs it. Evidence lives in the PTY frame .planning/phases/06-global-ssh-options/ui-frames/storage-migrate-confirm-post.txt"

	return []ScreenSpec{
		{
			ScreenID:              "gss-options-list",
			Interaction:           "Boot the Global SSH tab (view 2) on the Options sub-tab in browse mode.",
			StateMarker:           "StrictHostKeyChecking",
			ApplicableLive:        true,
			ApplicableApprovedTUI: true,
			NonApplicability:      noHTML,
			RequiredRegions:       []RegionName{RegionGSSOptionsBrowse},
			RegionDispositions: []RegionDisposition{
				fixtureHeaderStatusDisposition, gssListFixtureDisposition, gssOptionsFixtureDisposition,
			},
		},
		{
			ScreenID:              "gss-storage-current",
			Interaction:           "From the Options sub-tab, press Right to open the Storage & preview sub-tab in browse mode with the radio on the CURRENT layout.",
			StateMarker:           "STORE-01 — where gitid-managed SSH config lives",
			ApplicableLive:        false,
			ApplicableApprovedTUI: true,
			NonApplicability: append(noHTML,
				// The live backend CANNOT truthfully render this frame: on the
				// seeded fixture home the current layout is Include, and
				// planning a migration TO the current layout is a no-op the
				// real backend honestly reports ("layout is already include —
				// nothing to plan") instead of fabricating a resulting-config
				// preview. The real-machine browse evidence lives in the PTY
				// frame (06-05) instead.
				uxNonComparable("live", "DLV-4",
					"the real backend renders the honest \"nothing to plan\" hint for the CURRENT layout (SSHStorageMigrationPlan refuses a no-op migration to the layout that already is current on the seeded Include-layout fixture home) and never a resulting-config preview browse; the Include-layout browse evidence lives in the PTY frame .planning/phases/06-global-ssh-options/ui-frames/storage-browse.txt"),
			),
			RequiredRegions: []RegionName{RegionGSSStorageBrowse},
		},
		{
			ScreenID:              "gss-storage-other",
			Interaction:           "From the Storage & preview sub-tab, press Down to flip the layout radio to the OTHER (non-current) layout in browse mode — arming the migrate action. On the seeded fixture home the non-current layout is Sentinel; on the dummy it is Include.",
			StateMarker:           "Resulting config",
			ApplicableLive:        true,
			ApplicableApprovedTUI: true,
			NonApplicability:      noHTML,
			RequiredRegions:       []RegionName{RegionGSSStorageBrowse},
			RegionDispositions: []RegionDisposition{
				fixtureHeaderStatusDisposition, gssStorageFixtureDisposition, gssStoragePreviewDisposition,
			},
		},
		{
			ScreenID:              "gss-apply-preview",
			Interaction:           "From the Options sub-tab, move Up three rows from the default-selected IdentitiesOnly row to StrictHostKeyChecking (the same navigation the real PTY suite's openGlobalSSHPreview uses), Space to toggle it, then 'a' to open the apply ceremony at its pre-write preview.",
			StateMarker:           "Write Host * managed block to",
			ApplicableLive:        true,
			ApplicableApprovedTUI: true,
			NonApplicability:      noHTML,
			RequiredRegions:       []RegionName{RegionGSSApplyCeremony, RegionGSSApplyHeading},
			RegionDispositions: []RegionDisposition{
				fixtureHeaderStatusDisposition, gssApplyCeremonyDisposition, gssApplyHeadingDisposition,
			},
		},
		{
			ScreenID:              "gss-apply-receipt",
			Interaction:           "Apply ceremony state B (result receipt): the real journal-backed write has completed — NOT capturable in-process; see the live/approved-tui non-applicability reasons for the PTY frame that carries this evidence.",
			StateMarker:           "Wrote →",
			ApplicableLive:        false,
			ApplicableApprovedTUI: false,
			NonApplicability:      receiptNA("GSSH-D-04", applyReceiptReason),
			RequiredRegions:       []RegionName{RegionGSSApplyCeremony},
		},
		{
			ScreenID:              "gss-storage-migrate-preview",
			Interaction:           "From the Storage & preview sub-tab, flip the layout radio (Down) then press Enter to open the STORE-03 migration ceremony at its pre-write preview.",
			StateMarker:           "Migrate SSH storage layout",
			ApplicableLive:        true,
			ApplicableApprovedTUI: true,
			NonApplicability:      noHTML,
			RequiredRegions:       []RegionName{RegionGSSStorageCeremony},
			RegionDispositions: []RegionDisposition{
				fixtureHeaderStatusDisposition, gssStorageCeremonyDisposition,
			},
		},
		{
			ScreenID:              "gss-storage-migrate-receipt",
			Interaction:           "Storage-migration ceremony state B (result receipt): the STORE-03 two-file write has completed — NOT capturable in-process; see the live/approved-tui non-applicability reasons for the PTY frame that carries this evidence.",
			StateMarker:           "SSH storage layout migrated to",
			ApplicableLive:        false,
			ApplicableApprovedTUI: false,
			NonApplicability:      receiptNA("STORE-03", storageReceiptReason),
			RequiredRegions:       []RegionName{RegionGSSStorageCeremony},
		},
	}
}
