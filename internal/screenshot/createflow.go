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
				// 09-07-PLAN.md Task 2 discovery: this screen's script
				// auto-checks the D-01 checkbox and reaches testUpload before
				// stage 1 (see CaptureCreateFlowScreens' own D-02 comment), so
				// the upload beat's "Running: <command>" lines render on this
				// frame too — the SAME UP-4 fixture-vs-live command-path
				// divergence uploadVisualSpecs' own upload-results spec
				// classifies.
				uxRegionDifferenceScoped(RegionUploadSection, "fixture-vs-live-command-path", "UP-4",
					"the real upload command names spec.KeyPath as constructed by this wizard's own key-generation step; the dummy renders its frozen \"/usr/local/bin/gh ...\" command shape — both sides share the \"ssh-key add\" invocation and its --title/--type arguments",
					`contains:"ssh-key add"`),
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
				// 09-07-PLAN.md Task 2 discovery: same reasoning as
				// test-stage1-direct above — this screen is reached AFTER the
				// upload beat auto-ran on step 1, so its content persists here too.
				uxRegionDifferenceScoped(RegionUploadSection, "fixture-vs-live-command-path", "UP-4",
					"the real upload command names spec.KeyPath as constructed by this wizard's own key-generation step; the dummy renders its frozen \"/usr/local/bin/gh ...\" command shape — both sides share the \"ssh-key add\" invocation and its --title/--type arguments",
					`contains:"ssh-key add"`),
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
				// 09-07-PLAN.md Task 2 discovery: same reasoning as
				// test-stage1-direct above — this screen is reached AFTER the
				// upload beat auto-ran on step 1, so its content persists here too.
				uxRegionDifferenceScoped(RegionUploadSection, "fixture-vs-live-command-path", "UP-4",
					"the real upload command names spec.KeyPath as constructed by this wizard's own key-generation step; the dummy renders its frozen \"/usr/local/bin/gh ...\" command shape — both sides share the \"ssh-key add\" invocation and its --title/--type arguments",
					`contains:"ssh-key add"`),
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
	// 07-06-PLAN.md Task 1: consume the Phase 7 Global Git registry alongside
	// the existing four, without disturbing any of them (five-way merged).
	specs = append(specs, globalGitSpecs()...)
	// 08-08-PLAN.md Task 2 / 09.4-02: consume the Doctor registry
	// alongside the existing five, without disturbing any of them
	// (six-way merged).
	specs = append(specs, doctorSpecs()...)
	// 09-07-PLAN.md Task 2: consume the Phase 9 upload-surface registry
	// alongside the existing six, without disturbing any of them
	// (seven-way merged).
	specs = append(specs, uploadVisualSpecs()...)
	// 09.2-03 Task 3: consume the Global Git Ignore registry without
	// disturbing the existing surface registries.
	specs = append(specs, gitIgnoreVisualSpecs()...)
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
		strings.HasPrefix(ref, "GSSH-D-") || strings.HasPrefix(ref, "STORE-") ||
		// 07-06-PLAN.md Task 1 (Phase 7 registration): the disambiguating
		// GGIT-D- prefix for 07-CONTEXT.md decisions, the SAME pattern as
		// GSSH-D- for Phase 6.
		strings.HasPrefix(ref, "GGIT-D-") ||
		// 09-07-PLAN.md Task 2 (Phase 9 registration): UP-4 names the DLV-4-
		// shaped fixture-vs-live/dummy-vs-real comparison-mechanism class for
		// the upload surface (mirrors DLV-4/GSSH-D-/GGIT-D-'s own precedent —
		// this is a property of the comparison mechanism itself, not a
		// numbered 09-CONTEXT.md decision, so it does not collide with any
		// bare D-NN there).
		strings.HasPrefix(ref, "UP-") ||
		strings.HasPrefix(ref, "GIGN-")
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

// UploadEligibility is overridden with a deterministic, machine-independent
// answer — mirroring dummytui.FixtureBackend.UploadEligibility's OWN
// hostname branching exactly (github->Ready, gitlab->Unauth, else->Omitted).
// Before Phase 9's RegionUploadSection existed, every pre-existing
// create-flow spec's real capture ran the checkbox's REAL eligibility probe
// unmodified — resolving Ready/Unauth/Disabled based on whichever gh/glab
// state happens to be true on the machine running the gate, a pre-existing
// latent non-determinism CR-01's own intra-run check could never catch
// (both runs of a single test execute on the SAME machine). RegionUploadSection
// makes that machine-dependent content visible to region comparison for the
// FIRST time, so it must be pinned deterministically here — matching the
// dummy's own default shape means every PRE-Phase-9 create-flow spec's
// captured checkbox content becomes byte-identical between real and dummy,
// needing no new disposition at all. Phase 9's OWN specs
// (uploadVisualSpecs' upload-checkbox-unauth/disabled) reach their
// alternate states via CaptureUploadScreens' OWN, more specific
// uploadProbeBackend wrapper instead — this override never runs there,
// since mergeUploadCaptures passes newBackendForHome's raw *realBackend
// directly, not offlineCaptureBackend-wrapped.
func (o offlineCaptureBackend) UploadEligibility(hostname string) tea.Cmd {
	return func() tea.Msg {
		switch {
		case strings.Contains(hostname, "github"):
			return tuikit.UploadEligibilityMsg{Hostname: hostname, View: tuikit.UploadEligibilityView{
				State: tuikit.UploadEligibilityReady, ProviderName: "GitHub", ToolName: "gh", Hostname: "github.com",
			}}
		case strings.Contains(hostname, "gitlab"):
			return tuikit.UploadEligibilityMsg{Hostname: hostname, View: tuikit.UploadEligibilityView{
				State: tuikit.UploadEligibilityUnauth, ProviderName: "GitLab", ToolName: "glab", Hostname: "gitlab.com",
			}}
		default:
			return tuikit.UploadEligibilityMsg{Hostname: hostname, View: tuikit.UploadEligibilityView{State: tuikit.UploadEligibilityOmitted}}
		}
	}
}

// RunUpload is overridden with a deterministic, machine-independent result
// — mirroring dummytui.FixtureBackend.RunUpload's OWN fixed command shape
// and hostname-driven outcome exactly (github -> both registrations
// uploaded, gitlab -> one combined row, "partial" -> the scope-remediation
// partial-failure pair), for the SAME reason UploadEligibility above is
// overridden: without this, any pre-existing create-flow spec whose script
// reaches testUpload (the checkbox auto-checks Ready via the override
// above) would invoke the REAL backend's uploaderDeps (real exec.LookPath),
// resolving differently depending on whether gh/glab happens to be
// installed on the machine running the gate.
func (o offlineCaptureBackend) RunUpload(spec tuikit.CreateSpec) tea.Cmd {
	title := fmt.Sprintf(tuikit.UploadKeyTitleFmt, spec.Identity, "demo-machine")
	// WR-05: quote the title exactly like dummytui.FixtureBackend.RunUpload
	// now does (see its doc comment) — this method's own doc comment
	// requires mirroring that fixture's command shape EXACTLY, and an
	// unquoted D-07 title (always contains spaces) would desync the two.
	authCmd := fmt.Sprintf("/usr/local/bin/gh ssh-key add %s.pub --title '%s' --type authentication", spec.KeyPath, title)
	signCmd := fmt.Sprintf("/usr/local/bin/gh ssh-key add %s.pub --title '%s' --type signing", spec.KeyPath, title)
	glabCmd := fmt.Sprintf("/usr/local/bin/glab ssh-key add %s.pub -t '%s' --usage-type auth_and_signing", spec.KeyPath, title)

	var view tuikit.UploadRunView
	switch {
	case strings.Contains(spec.Hostname, "partial"):
		view = tuikit.UploadRunView{Rows: []tuikit.UploadResultRow{
			{Registration: tuikit.UploadRegistrationAuthentication, Label: tuikit.UploadRegistrationLabelAuth, Command: authCmd, Outcome: tuikit.UploadRowUploaded},
			{Registration: tuikit.UploadRegistrationSigning, Label: tuikit.UploadRegistrationLabelSigning, Command: signCmd, Outcome: tuikit.UploadRowFailed,
				Reason: fmt.Sprintf(tuikit.UploadScopeRemediationSigningFmt, "github.com")},
		}}
	case strings.Contains(spec.Hostname, "gitlab"):
		view = tuikit.UploadRunView{Rows: []tuikit.UploadResultRow{
			{Registration: tuikit.UploadRegistrationCombined, Label: tuikit.UploadRegistrationLabelCombined, Command: glabCmd, Outcome: tuikit.UploadRowUploaded},
		}}
	default:
		view = tuikit.UploadRunView{Rows: []tuikit.UploadResultRow{
			{Registration: tuikit.UploadRegistrationAuthentication, Label: tuikit.UploadRegistrationLabelAuth, Command: authCmd, Outcome: tuikit.UploadRowUploaded},
			{Registration: tuikit.UploadRegistrationSigning, Label: tuikit.UploadRegistrationLabelSigning, Command: signCmd, Outcome: tuikit.UploadRowUploaded},
		}}
	}
	return func() tea.Msg { return tuikit.UploadRunMsg{Name: spec.Identity, View: view} }
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

	// Enter from testIdle → testRunning1 + TestStage1 cmd returned — UNLESS
	// the D-01 checkbox auto-checked (FixtureBackend.UploadEligibility
	// answers Ready for any github.com-implying host, which this fresh
	// wizard's default hostname is), in which case Enter instead enters
	// testUpload and returns the RunUpload cmd (Phase 9 UP-02/UP-03). Detect
	// that one extra hop and deliver its UploadRunMsg first — the same
	// no-user-keystroke auto-advance into testRunning1 the real wizard does
	// (D-02) — before falling through to the stage-1 protocol below.
	var stage1Cmd tea.Cmd
	m, stage1Cmd = stepAndPendingCmd(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if stage1Cmd != nil {
		if uploadMsg := stage1Cmd(); uploadMsg != nil {
			if _, isUploadRun := uploadMsg.(tuikit.UploadRunMsg); isUploadRun {
				m, stage1Cmd = stepAndPendingCmd(m, uploadMsg)
			} else {
				// Not an upload hop — restore the original cmd so the
				// stage-1 delivery below still sees it (stage1Cmd() was
				// called once above purely to inspect the message type; a
				// tea.Cmd is a plain closure, safe to invoke more than
				// once here since offlineCaptureBackend's stage cmds are
				// pure and side-effect-free).
				stage1Cmd = func() tea.Msg { return uploadMsg }
			}
		}
	}
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
	var failureStage1Cmd tea.Cmd
	failure, failureStage1Cmd = stepAndPendingCmd(failure, tea.KeyPressMsg{Code: tea.KeyEnter})
	// Same D-01 upload auto-advance hop as the stage-1/stage-2 capture above
	// (Enter from testIdle enters testUpload, not testRunning1, whenever the
	// checkbox auto-checked on a Ready eligibility answer) — deliver it
	// first so the model actually reaches testRunning1 before the injected
	// WizardStageMsg below, or the message is a no-op against a model still
	// parked in testUpload and "connectivity-output" never renders.
	if failureStage1Cmd != nil {
		if uploadMsg := failureStage1Cmd(); uploadMsg != nil {
			if _, isUploadRun := uploadMsg.(tuikit.UploadRunMsg); isUploadRun {
				failure, _ = stepAndPendingCmd(failure, uploadMsg)
			}
		}
	}
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
		// 07-06-PLAN.md Task 1: Global Git specs (CaptureGlobalGitScreens) are
		// likewise captured separately against their own seeded HOME.
		// 08-08-PLAN.md Task 2 / 09.4-02: Doctor specs (CaptureDoctorScreens)
		// are likewise captured separately against their own seeded HOME.
		if !spec.ApplicableLive || isGitScreenID(spec.ScreenID) || isIdentityManagerScreenID(spec.ScreenID) || isGlobalSSHScreenID(spec.ScreenID) || isGlobalGitScreenID(spec.ScreenID) || isDoctorScreenID(spec.ScreenID) || isUploadScreenID(spec.ScreenID) || isGitIgnoreScreenID(spec.ScreenID) {
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

// CaptureGlobalGitScreens drives the Global Git Options and apply ceremony
// through their approval states and captures the rendered text at each one,
// indexed by ScreenID. This is a second in-process capture pass (alongside
// CaptureGlobalSSHScreens, both called independently by
// createflow_packet.go) — the real backend's live probe results against a
// sandbox-seeded ~/.gitconfig; the dummy's frozen GlobalGitOptions fixture.
//
// The states captured (per 07-06-PLAN.md Task 1):
//   - ggit-options-list — the Options sub-tab in browse mode (default entry).
//   - ggit-options-scrolled — the Options list scrolled, rendering the cue.
//   - ggit-options-with-selection — a row toggled on (init.defaultBranch).
//   - ggit-apply-preview — the apply ceremony at its pre-write state A.
//
// Three states are NON-APPLICABLE in this in-process gate, evidenced instead
// by a real PTY frame from plan 07-04:
//   - ggit-options-differs-row — needs a sandbox HOME whose git config was
//     seeded with a value that CONFLICTS with the recommended one, a
//     different seed than the single fixture HOME this capture pass shares
//     for every other state. Evidence: ui-frames/global-git-differs.txt.
//   - ggit-options-probe-error — needs a broken/missing git binary; the
//     dummy's fixture backend structurally cannot fail a probe it never
//     runs (the live-non-applicability case). Evidence:
//     ui-frames/global-git-probe-failure.txt.
//   - ggit-apply-receipt — the real journal-backed write (runGlobalGitApply,
//     plan 07-01) has completed — NOT capturable in-process. Evidence:
//     ui-frames/global-git-apply-confirm.txt.
//
// backend must satisfy tuikit.GlobalGitPlanner (both real cmd/gitid and
// dummytui.NewFixtureBackend do). The real backend's option states run the
// D-03 git-config probe against the seeded fixture home; the dummy returns
// its frozen fixture rows.
// ---------------------------------------------------------------------------

// globalGitApp boots a fresh tuikit.App around backend at the fixed capture
// geometry and activates the Global Git tab — the SAME '3' ActivationKey a
// real user presses (cmd/gitid/app.go's setTab(TabGlobalGit)).
func globalGitApp(backend tuikit.Backend) tea.Model {
	var model tea.Model = tuikit.NewApp(backend)
	model = step(model, tea.WindowSizeMsg{Width: CaptureWidth, Height: CaptureHeight})
	return keyRune(model, '3')
}

// CaptureGlobalGitScreens implements the doc comment above.
func CaptureGlobalGitScreens(backend tuikit.Backend) (map[string]string, error) {
	out := make(map[string]string, 6)
	capture := func(m tea.Model) string { return normalizeTimestamps(anyView(m)) }

	// ggit-options-list: the Options sub-tab in browse mode (default entry).
	browse := globalGitApp(backend)
	out["ggit-options-list"] = capture(browse)

	// ggit-options-scrolled: scroll down to render the scroll cue below.
	scrolled := browse
	for i := 0; i < 8; i++ {
		scrolled = keyDown(scrolled)
	}
	out["ggit-options-scrolled"] = capture(scrolled)

	// ggit-options-with-selection: toggle init.defaultBranch (it's the first
	// row, already focused on entry).
	selected := globalGitApp(backend)
	selected = keyRune(selected, ' ')
	out["ggit-options-with-selection"] = capture(selected)

	// ggit-options-differs-row and ggit-options-probe-error are NOT captured
	// here: both require a differently-seeded sandbox HOME (a conflicting
	// git-config value, or a broken/missing git binary respectively) than the
	// single fixture HOME every other in-process Global Git capture in this
	// function shares. Both specs are registered non-applicable on BOTH
	// surfaces (ApplicableLive: false, ApplicableApprovedTUI: false) —
	// evidence for each lives in a real PTY frame from plan 07-04 instead of
	// a hollow in-process frame (see globalGitSpecs' NonApplicability records).

	// ggit-apply-preview: toggle init.defaultBranch then press 'a' to open
	// the apply ceremony at its pre-write state A.
	apply := globalGitApp(backend)
	apply = keyRune(apply, ' ')
	apply = keyRune(apply, 'a')
	out["ggit-apply-preview"] = capture(apply)

	for _, spec := range globalGitSpecs() {
		// Specs non-applicable on BOTH surfaces (the receipt state, the
		// differs-row state, and the probe-error state) need conditions this
		// in-process gate never produces; nothing is captured for them, by
		// design — never a hollow frame.
		if !spec.ApplicableLive && !spec.ApplicableApprovedTUI {
			continue
		}
		text, ok := out[spec.ScreenID]
		if !ok || strings.TrimSpace(text) == "" {
			return nil, fmt.Errorf("screenshot: CaptureGlobalGitScreens: required frame %q is missing or empty", spec.ScreenID)
		}
	}

	// A fixture/home bug that leaves the apply ceremony stuck in browse mode
	// would pass the non-emptiness loop above; assert the ceremony actually
	// opened.
	if out["ggit-apply-preview"] == out["ggit-options-list"] {
		return nil, fmt.Errorf("screenshot: CaptureGlobalGitScreens: ggit-apply-preview captured the same frame as ggit-options-list — the apply ceremony never opened")
	}

	return out, nil
}

// ---------------------------------------------------------------------------
// Doctor regions (08-08-PLAN.md Task 2 / 09.4-02, DLV-04). CaptureDoctorScreens
// mirrors CaptureGlobalGitScreens' self-contained pattern exactly: a
// dedicated capture function driving the shared tuikit render stack over
// Doctor's own seeded home, merged into the shared registry by
// mergeDoctorCaptures in cmd/gitid/gate_visual_regression_test.go —
// never threaded into CaptureCreateFlowScreens' own home (isDoctorScreenID
// excludes these IDs from that completeness check, the same reason every
// other later-phase surface is excluded there).
// ---------------------------------------------------------------------------

// doctorApp boots a fresh tuikit.App around backend at the fixed capture
// geometry and activates tabKey — '4' for the merged Doctor tab (the SAME
// key a real user presses; app.go's setTab dispatch).
func doctorApp(backend tuikit.Backend, tabKey rune) tea.Model {
	var model tea.Model = tuikit.NewApp(backend)
	model = step(model, tea.WindowSizeMsg{Width: CaptureWidth, Height: CaptureHeight})
	return keyRune(model, tabKey)
}

// CaptureDoctorScreens captures the merged Doctor tab's screen states over
// backend's own seeded home. Covers: doctor-findings (the unfiltered
// findings list with its always-visible inline detail pane after the real
// scan), doctor-selected (the same list with a fixable finding selected),
// and doctor-ceremony-preview (state A of the compressed 2-state fix
// ceremony, reached by pressing f in place).
//
// The all-green / nothing-to-fix and batch-walk states are NOT captured
// here: both require either a genuinely-clean real scan (this package's
// fixture deliberately seeds real findings so doctor-findings has content
// to compare) or a real Persist write mid-walk that this no-subprocess,
// in-process capture path never performs — the SAME class of gap
// CaptureGlobalGitScreens' own doc comment records for its
// probe-error/differs-row states. Evidence for the full state set lives in
// e2e/doctor_pty_e2e_test.go's real-PTY suite instead.
func CaptureDoctorScreens(backend tuikit.Backend) (map[string]string, error) {
	out := make(map[string]string, 3)
	capture := func(m tea.Model) string { return normalizeTimestamps(anyView(m)) }

	// doctor-findings: merged Doctor tab after its real scan.
	list := doctorApp(backend, '4')
	out["doctor-findings"] = capture(list)

	// doctor-selected: the same list with the second finding selected (one
	// Down, safely clamped/no-op when only one row exists — the real
	// side's single-finding fixture) rather than the first: the dummy's
	// frozen fixture set's first-row finding (a long key path) makes the
	// ceremony's "Backup → …" line wrap across two rendered lines, and
	// timestampPattern (createflow.go) cannot normalize a timestamp split by
	// a hard line-wrap — a genuine CR-01 non-determinism, not a real
	// divergence. The second row's shorter target path fits on one line.
	selected := keyDown(list)
	out["doctor-selected"] = capture(selected)

	// doctor-ceremony-preview: press 'f' in place to open that finding's
	// ceremony at its pre-write state A.
	ceremony := keyRune(selected, 'f')
	out["doctor-ceremony-preview"] = capture(ceremony)

	for _, spec := range doctorSpecs() {
		if !spec.ApplicableLive && !spec.ApplicableApprovedTUI {
			continue
		}
		text, ok := out[spec.ScreenID]
		if !ok || strings.TrimSpace(text) == "" {
			return nil, fmt.Errorf("screenshot: CaptureDoctorScreens: required frame %q is missing or empty", spec.ScreenID)
		}
	}

	// A fixture/home bug that leaves the fix ceremony stuck on the list
	// would pass the non-emptiness loop above; assert it actually opened.
	if out["doctor-ceremony-preview"] == out["doctor-selected"] {
		return nil, fmt.Errorf("screenshot: CaptureDoctorScreens: doctor-ceremony-preview captured the same frame as doctor-selected — the fix ceremony never opened")
	}

	return out, nil
}

// isDoctorScreenID reports whether id is one of the Doctor checkpoint IDs
// registered here — used to exclude them from CaptureCreateFlowScreens'
// completeness check (they are captured separately; see
// CaptureDoctorScreens' doc comment).
func isDoctorScreenID(id string) bool {
	for _, spec := range doctorSpecs() {
		if spec.ScreenID == id {
			return true
		}
	}
	return false
}

// doctorSpecs returns the merged Doctor checkpoint specs. ApplicableApprovedHTML
// is false throughout, for the SAME reason every later-phase surface's specs
// record it explicitly (T-06-45): Phase 2's approved Bubble Tea dummy is the
// SOLE UI/UX parity target for Phases 3-10, and the historical HTML/MUI
// artifacts are Phase-2 design history.
//
// RegionDispositions mirror
// .planning/design/health-fixer/visual-divergence-allowlist.txt's classified
// entries verbatim (kept in sync by TestDoctorAllowlistMatchesRegistry
// in cmd/gitid/gate_visual_regression_test.go).
func doctorSpecs() []ScreenSpec {
	// hfFixtureClass is the DLV-04 fixture-vs-live comparison class every
	// other later-phase surface's registry uses for the SAME class of
	// divergence: the real backend renders real doctor.Run(deps) findings
	// against a seeded fixture home; the dummy renders its frozen
	// DemoFinding fixture set (internal/dummytui/data.go).
	hfFixtureClass := "DLV-4"

	noHTML := []SurfaceNonApplicability{uxNonComparable("approved-html", hfFixtureClass,
		"AGENTS.md's BINDING UI Reference rule: Phase 2's approved Bubble Tea dummy is the SOLE Phase 8 UI/UX parity target for Phases 3-10; the historical HTML/MUI artifacts are Phase-2 design history and are recorded explicitly NON-APPLICABLE for every Phase 8 comparison")}

	doctorBodyDisposition := uxRegionDifferenceScoped(RegionDoctorBody, "fixture-vs-live-findings", hfFixtureClass,
		"the real Doctor body renders the live doctor.Run(deps) findings against the seeded fixture home; the dummy renders its frozen DemoFinding fixture set (internal/dummytui/data.go) — the whole findings list + detail pane is the classified fixture-vs-live divergence, and the shared 'Suggested fix:' label survives on both sides for any finding carrying one",
		`contains:"Suggested fix:"`)
	doctorCeremonyDisposition := uxRegionDifferenceScoped(RegionDoctorCeremony, "fixture-vs-live-diff", hfFixtureClass,
		"the real fix ceremony's diff previews the first REAL fixable finding's actual before/after lines against the seeded fixture home; the dummy previews its frozen fixture finding's diff — the shared ceremony chrome (heading prefix, confirm/cancel buttons) survives on both sides",
		`contains:"Fix: "`)
	fixtureHeaderStatusDispositionHF := uxRegionDifferenceScoped(RegionHeaderStatus, "identity-count", hfFixtureClass,
		"header status shows the identity count, which differs (real's single-identity seeded Doctor fixture home vs the dummy's 8-identity IdentityManagerRows fixture set) — both sides carry the shared 'ids' marker",
		`contains:"ids"`)
	keybarDispositionHF := uxRegionDifferenceScoped(RegionKeybar, "finding-count-and-list", hfFixtureClass,
		"the keybar region's last three lines include the status/count line naming how many findings were scanned, which differs (real's single seeded fixture finding vs the dummy's frozen multi-finding fixture set) — both sides share the 'Esc' key hint",
		`contains:"Esc"`)
	sidebarDispositionHF := uxRegionDifferenceScoped(RegionSidebar, "fixture-vs-live-rows", hfFixtureClass,
		"on this surface RegionSidebar's left-of-│ extraction captures the FINDINGS LIST rows, not an identity sidebar — the real row set describes the live doctor.Run(deps) scan against the seeded fixture home while the dummy rows carry the frozen DemoFinding fixture set; the list content is the fixture-vs-live divergence and 'fixable' survives on both sides",
		`contains:"fixable"`)
	breadcrumbDispositionHF := uxRegionDifferenceScoped(RegionBreadcrumb, "finding-title", hfFixtureClass,
		"the ceremony's breadcrumb names the selected finding's title (\"Doctor › Fix › <title>\"), which differs between the real seeded fixture finding and the dummy's frozen fixture finding — the shared 'Doctor › Fix ›' prefix anchors both sides",
		`contains:"Doctor › Fix"`)

	return []ScreenSpec{
		{
			ScreenID:              "doctor-findings",
			Interaction:           "Boot the Doctor tab (view 4) after its real scan.",
			StateMarker:           "Suggested fix:",
			ApplicableLive:        true,
			ApplicableApprovedTUI: true,
			NonApplicability:      noHTML,
			RequiredRegions:       []RegionName{RegionDoctorBody},
			RegionDispositions:    []RegionDisposition{doctorBodyDisposition, fixtureHeaderStatusDispositionHF, keybarDispositionHF, sidebarDispositionHF},
		},
		{
			ScreenID:              "doctor-selected",
			Interaction:           "From the Doctor list (view 4), press Down to select the second finding (the second row avoids a fixture-specific backup-path line-wrap on the dummy's first row — see CaptureDoctorScreens' doc comment).",
			StateMarker:           "f · Fix this…",
			ApplicableLive:        true,
			ApplicableApprovedTUI: true,
			NonApplicability:      noHTML,
			RequiredRegions:       []RegionName{RegionDoctorBody},
			RegionDispositions:    []RegionDisposition{doctorBodyDisposition, fixtureHeaderStatusDispositionHF, keybarDispositionHF, sidebarDispositionHF},
		},
		{
			ScreenID:              "doctor-ceremony-preview",
			Interaction:           "From the Doctor list (view 4), press Down then f to open the second fixable finding's fix ceremony at its pre-write state A (the second row avoids a fixture-specific backup-path line-wrap on the dummy's first row — see CaptureDoctorScreens' doc comment).",
			StateMarker:           "Fix: ",
			ApplicableLive:        true,
			ApplicableApprovedTUI: true,
			NonApplicability:      noHTML,
			RequiredRegions:       []RegionName{RegionDoctorCeremony},
			RegionDispositions:    []RegionDisposition{doctorCeremonyDisposition, fixtureHeaderStatusDispositionHF, keybarDispositionHF, sidebarDispositionHF, breadcrumbDispositionHF},
		},
	}
}

// gitIgnoreVisualSpecs returns the six Global Git Ignore states exercised by
// the live and approved Bubble Tea applications. The historical HTML mockup
// has no corresponding screen and is explicitly non-applicable.
func gitIgnoreVisualSpecs() []ScreenSpec {
	noHTML := []SurfaceNonApplicability{uxNonComparable("approved-html", "GIGN-01", "Global Git Ignore was introduced after the approved Phase-2 HTML mockup; the approved Bubble Tea dummy is the binding UI reference.")}
	// fixtureHeaderStatusDispositionGIGN mirrors every other later-phase
	// registry's own header-status disposition (see doctorSpecs'
	// fixtureHeaderStatusDispositionHF for the identical class of
	// divergence): the live capture's disposable HOME and the dummy's
	// frozen 8-identity fixture never carry the same identity count, so the
	// header's "N ids" segment always differs — both sides still share the
	// "ids" marker.
	fixtureHeaderStatusDispositionGIGN := uxRegionDifferenceScoped(RegionHeaderStatus, "identity-count", "GIGN-01",
		"header status shows the identity count, which differs (the live capture's disposable HOME vs the dummy's 8-identity IdentityManagerRows fixture set) — both sides carry the shared 'ids' marker",
		`contains:"ids"`)
	// gignBodyDisposition mirrors doctorSpecs' doctorBodyDisposition
	// (the same class of divergence): the live body renders the real
	// gitignore/baseline content seeded by deterministicGitIgnoreFixture,
	// while the dummy renders FixtureBackend's frozen fixtureGitIgnoreContent
	// — different curated pattern subsets. The per-line editor content is
	// rendered character-cell-styled (each rune carries its own ANSI escape
	// codes), so a content needle like ".DS_Store" never survives as a
	// contiguous substring in the captured text; the footer's discard-edits
	// status line is a single uninterrupted styled span present in every
	// browse/editor-body state on both sides, so it is the shared anchor.
	gignBodyDisposition := uxRegionDifferenceScoped(RegionGIGNBody, "fixture-vs-live-content", "GIGN-01",
		"the real body renders the live gitignore/baseline content seeded for this capture; the dummy renders FixtureBackend's frozen fixture content — the pattern SET is the classified fixture-vs-live divergence, and the shared discard-edits footer status survives on both sides",
		`contains:"Leaving this screen discards unsaved edits."`)
	// gignCeremonyDisposition mirrors gignBodyDisposition for the ceremony
	// region: the diff preview and receipt both echo the differing
	// live/dummy content, so the shared confirm-button footer hint (a
	// single uninterrupted styled span present in both the preview and
	// receipt states) is the anchor instead.
	gignCeremonyDisposition := uxRegionDifferenceScoped(RegionGIGNCeremony, "fixture-vs-live-content", "GIGN-01",
		"the real ceremony previews/writes the live gitignore/baseline content seeded for this capture; the dummy previews/writes FixtureBackend's frozen fixture content — the pattern SET is the classified fixture-vs-live divergence, and the shared 'Tab/←→' footer hint survives on both sides",
		`contains:"Tab/←→"`)
	dispositions := []RegionDisposition{fixtureHeaderStatusDispositionGIGN, gignBodyDisposition, gignCeremonyDisposition}
	return []ScreenSpec{
		{ScreenID: "gign-existing-block", Interaction: "Open Global Git Ignore with an existing managed block.", StateMarker: "Wired — core.excludesfile", ApplicableLive: true, ApplicableApprovedTUI: true, NonApplicability: noHTML, RequiredRegions: []RegionName{RegionGIGNBody}, RegionDispositions: dispositions},
		{ScreenID: "gign-seeded-defaults", Interaction: "Open Global Git Ignore without a managed block and view the curated defaults.", StateMarker: "No managed block found", ApplicableLive: true, ApplicableApprovedTUI: true, NonApplicability: noHTML, RequiredRegions: []RegionName{RegionGIGNBody}, RegionDispositions: dispositions},
		{ScreenID: "gign-editing", Interaction: "Enter the editor and capture the focused editable text area.", StateMarker: "Leaving this screen discards unsaved edits.", ApplicableLive: true, ApplicableApprovedTUI: true, NonApplicability: noHTML, RequiredRegions: []RegionName{RegionGIGNBody}, RegionDispositions: dispositions},
		{ScreenID: "gign-after-reset", Interaction: "Reset the editor to curated defaults.", StateMarker: "Reset to defaults", ApplicableLive: true, ApplicableApprovedTUI: true, NonApplicability: noHTML, RequiredRegions: []RegionName{RegionGIGNBody}, RegionDispositions: dispositions},
		{ScreenID: "gign-review-ceremony", Interaction: "Review the edited global gitignore before writing.", StateMarker: "Review your global gitignore before writing.", ApplicableLive: true, ApplicableApprovedTUI: true, NonApplicability: noHTML, RequiredRegions: []RegionName{RegionGIGNCeremony}, RegionDispositions: dispositions},
		{ScreenID: "gign-receipt", Interaction: "Confirm the write and capture the receipt.", StateMarker: "Global gitignore written", ApplicableLive: true, ApplicableApprovedTUI: true, NonApplicability: noHTML, RequiredRegions: []RegionName{RegionGIGNCeremony}, RegionDispositions: dispositions},
	}
}

// CaptureGitIgnoreScreens captures the six registered Global Git Ignore states.
func CaptureGitIgnoreScreens(backend tuikit.Backend) (map[string]string, error) {
	out := make(map[string]string, 6)
	capture := func(m tea.Model) string { return normalizeTimestamps(anyView(m)) }
	app := func() tea.Model {
		var m tea.Model = tuikit.NewApp(backend)
		m = step(m, tea.WindowSizeMsg{Width: CaptureWidth, Height: CaptureHeight})
		return keyRune(m, '5')
	}
	browse := app()
	out["gign-existing-block"] = capture(browse)
	out["gign-seeded-defaults"] = capture(browse)
	// Entering edit mode calls textarea.Focus(), which returns a repeating
	// cursor-blink tea.Cmd (Blink -> blinkMsg -> another Blink, forever by
	// design). step()'s recursive cmd-drain is unbounded and blows the
	// stack on that chain, so this ONE keystroke is applied with a single,
	// non-recursive Update call instead of keyEnter/step. The textarea's
	// Focus() call already flips its internal focused flag synchronously —
	// a static capture needs nothing from the blink cmd itself.
	editingModel, _ := browse.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	editing := editingModel
	out["gign-editing"] = capture(editing)
	// The 'r' reset shortcut is only wired in browse mode (gitIgnoreModel's
	// handleKey routes 'r' to the editor's Update, i.e. types the letter,
	// while m.editing is true) — reset from browse, not from the focused
	// editing model.
	reset := keyRune(browse, 'r')
	out["gign-after-reset"] = capture(reset)
	review := keyRune(browse, 'a')
	out["gign-review-ceremony"] = capture(review)
	// One Enter confirms: the host sets commitPending and dispatches the
	// async commit cmd, and step()'s recursive drain resolves that cmd's
	// one-shot tick synchronously, delivering GlobalGitIgnoreCommitMsg and
	// landing the ceremony in its receipt (done=true) state. A SECOND Enter
	// here would be misread as ceremonyModel.handleKey's own "Enter on the
	// receipt finishes" branch (c.done -> ceremonyFinished), which closes
	// the ceremony and reverts the capture to the browse view instead of
	// the receipt — so, unlike the double-Enter pattern other capture
	// flows use for their own (differently shaped) ceremonies, this one
	// stops at a single confirm.
	confirmed := keyEnter(keyTab(review))
	out["gign-receipt"] = capture(confirmed)
	for _, spec := range gitIgnoreVisualSpecs() {
		if strings.TrimSpace(out[spec.ScreenID]) == "" {
			return nil, fmt.Errorf("screenshot: CaptureGitIgnoreScreens: required frame %q is missing or empty", spec.ScreenID)
		}
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

// isGlobalGitScreenID reports whether id is one of the Phase 7 Global Git
// checkpoint IDs registered here — used to exclude them from
// CaptureCreateFlowScreens' completeness check (they are captured separately;
// see CaptureGlobalGitScreens' doc comment).
func isGlobalGitScreenID(id string) bool {
	for _, spec := range globalGitSpecs() {
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
	// `absent:` on the dummy's "fixture value" marker — the real side renders
	// actual provenance (three-tier labels including "not set (OpenSSH default: ...)"
	// for baseline cases), while the fixture always shows "fixture value — the demo
	// does not probe this machine". After Task 1's first-row-focus fix, the
	// Provenance field is now visible by default (StrictHostKeyChecking is the first
	// row), and the "not set (OpenSSH default: ask)" string legitimately appears in
	// the real-side detail pane when the option is unfilled. The fixture-vs-real
	// discriminator is the absence of "fixture value" on the real side.
	gssOptionsFixtureDisposition := uxRegionDifferenceScoped(RegionGSSOptionsBrowse, "provenance-state-and-rows", gssFixtureClass,
		"the real Options body renders the live D-01/D-03 provenance labels (including baseline 'not set (OpenSSH default: ...)' when unprovided) and the D-11/D-12 four-state rows; the fixture renders its frozen GlobalSSHOptions and 'fixture value — the demo does not probe this machine' provenance. Task 1's first-row-focus fix surfaces the Provenance field in the detail pane by default; the real baseline provenance legitimately includes the 'not set (OpenSSH default: ask)' catalog reference string. The divergence discriminator is the absence of 'fixture value' marker on the real side — the whole master-detail body is the classified fixture-vs-live divergence (T-06-PROVENANCE)",
		`absent:"fixture value"`)
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
			ScreenID: "gss-storage-current",
			Interaction: "From the Options sub-tab, press Right to open the Storage & preview sub-tab in browse mode " +
				"with the radio on the CURRENT layout.",
			StateMarker:           "STORE-01 — where gitid-managed SSH config lives",
			ApplicableLive:        true,
			ApplicableApprovedTUI: true,
			NonApplicability:      noHTML,
			RequiredRegions:       []RegionName{RegionGSSStorageBrowse},
			RegionDispositions: []RegionDisposition{
				fixtureHeaderStatusDisposition, gssStorageFixtureDisposition, gssStoragePreviewDisposition,
			},
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

// globalGitSpecs returns the Phase 7 Global Git checkpoint specs — the SAME
// vocabulary 07-UI-SPEC.md's Approved Base States table names and 07-04's
// real-PTY suites drive over real PTYs. ApplicableApprovedHTML is false
// throughout: per AGENTS.md's BINDING UI Reference rule (recorded in the
// authority of 07-06-PLAN.md), Phase 2's approved Bubble Tea dummy is the SOLE
// Phase 7 UI/UX parity target (Phases 3-10), and the historical HTML/MUI
// artifacts are Phase-2 design history — made EXPLICIT on every spec rather
// than left to the registry's default (T-06-45).
//
// RegionDispositions mirror
// .planning/design/global-git/visual-divergence-allowlist.txt's classified
// entries verbatim (kept in sync by
// TestGlobalGitAllowlistMatchesRegistry in cmd/gitid/gate_visual_regression_test.go).
func globalGitSpecs() []ScreenSpec {
	// DLV-4 (never a numbered D-NN): the fixture-vs-live comparison-class
	// divergence — the real backend reads a seeded fixture home, the dummy's
	// FixtureBackend always renders its frozen GlobalGitOptions fixture set.
	// Governed directly by the DLV-04 requirement; the SAME literal the Phase
	// 6 Global SSH registry uses for the identical class.
	ggitFixtureClass := "DLV-4"
	fixtureHeaderStatusDispositionGit := uxRegionDifferenceScoped(RegionHeaderStatus, "identity-count", ggitFixtureClass,
		"header status shows the identity count, which differs (real's zero-identity seeded Global Git fixture home vs the dummy's 8-identity IdentityManagerRows fixture set)",
		`contains:"ids"`)
	// ggitOptionsFixtureDisposition authorizes the whole Options master-detail
	// body as the D-01/D-03-provenance + D-11/D-12-row-state fixture-vs-live
	// divergence: the real body carries the live three-tier provenance and
	// the machine's four-state row renderings while the dummy body carries
	// its frozen GlobalGitOptions fixture values and its "the demo does not
	// probe this machine" provenance label (T-06-PROVENANCE). The predicate is
	// `absent:"not set ("` — the dummy's frozen fixture wraps its explanatory
	// default in a "not set (<explanation>)" formulation on every row (e.g.
	// "not set (git's built-in default: …", "not set (OS-dependent: …"); the
	// real side's live probe against an unconfigured seeded fixture home
	// renders a bare empty current-value ("now:  → main") instead, so this
	// substring never appears there — an empirically verified real-vs-dummy
	// text sample confirmed the ORIGINAL predicate ("(setting or default)")
	// never appears on EITHER side for a policy-backed row, making it
	// vacuous; "not set (" is the actually-observed, side-specific marker.
	ggitOptionsFixtureDisposition := uxRegionDifferenceScoped(RegionGGitOptionsBrowse, "provenance-state-and-rows", ggitFixtureClass,
		"the real Options body renders the live D-01/D-03 provenance labels and the D-11/D-12 four-state rows against the seeded fixture home; the dummy renders its frozen GlobalGitOptions fixture values and its 'the demo does not probe this machine' provenance — the whole master-detail body is the classified fixture-vs-live divergence (T-06-PROVENANCE)",
		`absent:"not set ("`)
	// ggitListFixtureDisposition covers RegionSidebar's extraction on this
	// surface: its 'content before │' rule captures the OPTION-LIST rows
	// (this surface has no identity sidebar), which differ exactly as the
	// ggit-options-browse rows above do.
	ggitListFixtureDisposition := uxRegionDifferenceScoped(RegionSidebar, "fixture-vs-probe-rows", ggitFixtureClass,
		"on this surface RegionSidebar's left-of-│ extraction captures the OPTION LIST rows, not an identity sidebar — the real row set describes the live probe while the dummy rows carry its frozen fixture now-values; the list text is what the gate compares, and 'now:' survives on both sides",
		`contains:"now:"`)
	// ggitApplyCeremonyDisposition authorizes the baseline apply-ceremony diff
	// body: the real apply-ceremony diff renders the EnsureGlobalGit
	// managed block against the seeded home's baseline bytes; the dummy's
	// frozen GlobalGitApplyPlan diff renders the fixture baseline block.
	ggitApplyCeremonyDisposition := uxRegionDifferenceScoped(RegionGGitApplyCeremony, "managed-baseline-block-diff", "GGIT-D-06",
		"the real apply-ceremony diff is the EnsureGlobalGit write target against the seeded fixture home's resolved baseline file; the dummy ceremony renders its frozen GlobalGitApplyPlan fixture diff — the shared 'Write global-git managed block to' prefix anchors both sides",
		`contains:"Write global-git managed block to"`)
	// ggitApplyHeadingDisposition is the D-07 resolved-target ceremony heading:
	// the real side resolves the actual baseline path for the fixture home,
	// the dummy falls back to its frozen fixture baseline-path string.
	ggitApplyHeadingDisposition := uxRegionDifferenceScoped(RegionGGitApplyHeading, "resolved-target-file", "GGIT-D-07",
		"the apply ceremony's heading names the RESOLVED baseline target (D-07) — the real side resolves the fixture home's actual baseline path, the dummy renders its frozen fixture baseline path; the shared 'Write global-git managed block to ' prefix anchors both sides",
		`contains:"Write global-git managed block to "`)

	// noHTML is the explicit approved-HTML non-applicability record EVERY
	// Global Git spec carries, stating the standing UI-reference rule by name.
	noHTML := []SurfaceNonApplicability{uxNonComparable("approved-html", "DLV-4",
		"AGENTS.md's BINDING UI Reference rule: Phase 2's approved Bubble Tea dummy is the SOLE Phase 7 UI/UX parity target for Phases 3-10; the historical HTML/MUI artifacts are Phase-2 design history and are recorded explicitly NON-APPLICABLE for every Phase 7 comparison")}

	// probeErrorNA records the probe-error state as a live non-applicability:
	// the dummy's fixture backend structurally cannot fail a probe it never
	// runs. Evidence lives in the PTY frame from 07-04's test suite.
	probeErrorNA := []SurfaceNonApplicability{
		uxNonComparable("live", "GGIT-D-04", "the probe-error state requires a real git probe failure; this in-process, no-subprocess capture path never performs it. Evidence lives in the PTY frame .planning/phases/07-global-git-options/ui-frames/global-git-probe-failure.txt (TestGlobalGit_RealPTYProbeFailureStaysNavigable)"),
		uxNonComparable("approved-tui", "GGIT-D-04", "the dummy's fixture backend structurally cannot fail a probe it never runs — the probe-error state is reachable on the real binary only"),
		uxNonComparable("approved-html", "DLV-4", "AGENTS.md's BINDING UI Reference rule: Phase 2's approved Bubble Tea dummy is the SOLE Phase 7 UI/UX parity target for Phases 3-10; the historical HTML/MUI artifacts are Phase-2 design history"),
	}

	// applyReceiptReason records that the apply receipt state is not capturable
	// in-process; evidence lives in the PTY frame.
	applyReceiptReason := "the apply receipt requires the real write (runGlobalGitApply, plan 07-01) to have completed; this in-process, no-subprocess capture path never performs it. Evidence lives in the PTY frame .planning/phases/07-global-git-options/ui-frames/global-git-apply-confirm.txt (TestGlobalGit_RealPTYApplyConfirm)"

	// receiptNA builds the non-applicability records for receipt states:
	// they need a real write neither side performs in-process.
	receiptNA := func(decision, reason string) []SurfaceNonApplicability {
		return append(noHTML,
			uxNonComparable("live", decision, reason),
			uxNonComparable("approved-tui", decision, reason),
		)
	}

	return []ScreenSpec{
		{
			ScreenID:              "ggit-options-list",
			Interaction:           "Boot the Global Git tab (view 3) on the Options sub-tab in browse mode.",
			StateMarker:           "init.defaultBranch",
			ApplicableLive:        true,
			ApplicableApprovedTUI: true,
			NonApplicability:      noHTML,
			RequiredRegions:       []RegionName{RegionGGitOptionsBrowse},
			RegionDispositions: []RegionDisposition{
				fixtureHeaderStatusDispositionGit, ggitListFixtureDisposition, ggitOptionsFixtureDisposition,
			},
		},
		{
			ScreenID:              "ggit-options-scrolled",
			Interaction:           "From the Options list, press Down repeatedly to scroll the list and render the scroll cue below the last visible row.",
			StateMarker:           "↓ (+",
			ApplicableLive:        true,
			ApplicableApprovedTUI: true,
			NonApplicability:      noHTML,
			RequiredRegions:       []RegionName{RegionGGitOptionsBrowse},
			RegionDispositions: []RegionDisposition{
				fixtureHeaderStatusDispositionGit, ggitListFixtureDisposition, ggitOptionsFixtureDisposition,
			},
		},
		{
			ScreenID:              "ggit-options-with-selection",
			Interaction:           "From the Options list, press Space to toggle init.defaultBranch (select it) then render the apply action in the footer.",
			StateMarker:           "☑",
			ApplicableLive:        true,
			ApplicableApprovedTUI: true,
			NonApplicability:      noHTML,
			RequiredRegions:       []RegionName{RegionGGitOptionsBrowse},
			RegionDispositions: []RegionDisposition{
				fixtureHeaderStatusDispositionGit, ggitListFixtureDisposition, ggitOptionsFixtureDisposition,
			},
		},
		{
			ScreenID:              "ggit-options-differs-row",
			Interaction:           "Seed a sandbox home whose git config differs from the recommended value, then open the Global Git tab to show the differs wording on that row. NOT capturable by this generic in-process capture pass (which uses one fixed sandbox seed shared by every other state) without plumbing a second, differently-seeded HOME through the same no-subprocess technique — see the live non-applicability reason for the PTY frame that carries this evidence instead.",
			StateMarker:           "differs from",
			ApplicableLive:        false,
			ApplicableApprovedTUI: false,
			NonApplicability: []SurfaceNonApplicability{
				uxNonComparable("live", "GGIT-D-04", "this state requires a sandbox HOME whose git config was pre-seeded with a conflicting value, distinct from the shared fixture HOME every other in-process Global Git capture uses; this generic capture pass does not plumb a second seed through the same no-subprocess technique. Evidence lives in the PTY frame .planning/phases/07-global-git-options/ui-frames/global-git-differs.txt (TestGlobalGit_RealPTYDiffersRow)"),
				uxNonComparable("approved-tui", "GGIT-D-04", "the dummy's frozen fixture values never differ from the Policy's recommended values — this state is reachable on the real binary only"),
				uxNonComparable("approved-html", "DLV-4", "AGENTS.md's BINDING UI Reference rule: Phase 2's approved Bubble Tea dummy is the SOLE Phase 7 UI/UX parity target for Phases 3-10"),
			},
			RequiredRegions: []RegionName{RegionGGitOptionsBrowse},
		},
		{
			ScreenID:              "ggit-options-probe-error",
			Interaction:           "Seed a sandbox home with a malformed git config or missing git binary, then open the Global Git tab to show the probe-error warning note. This is the LIVE NON-APPLICABILITY case (07-06-PLAN.md authority): the dummy's fixture backend structurally cannot fail a probe it never runs.",
			StateMarker:           "git probe failed",
			ApplicableLive:        false,
			ApplicableApprovedTUI: false,
			NonApplicability:      probeErrorNA,
			RequiredRegions:       []RegionName{RegionGGitOptionsBrowse},
		},
		{
			ScreenID:              "ggit-apply-preview",
			Interaction:           "From the Options list, toggle init.defaultBranch, then press 'a' to open the apply ceremony at its pre-write preview.",
			StateMarker:           "Write global-git managed block to",
			ApplicableLive:        true,
			ApplicableApprovedTUI: true,
			NonApplicability:      noHTML,
			RequiredRegions:       []RegionName{RegionGGitApplyCeremony, RegionGGitApplyHeading},
			RegionDispositions: []RegionDisposition{
				fixtureHeaderStatusDispositionGit, ggitApplyCeremonyDisposition, ggitApplyHeadingDisposition,
				// internal/tuikit/ceremony.go's "Exact change: …" hint is a
				// STATIC line always rendered on the review pane — the SAME
				// shared ceremony code the git-screen ceremony uses (see that
				// spec's own RegionConfirmationPreview disposition above),
				// so RegionConfirmationPreview (a create-flow-scoped region)
				// also picks up THIS Global Git apply ceremony's content.
				// Same reasoning, intentionally BLANKET for the same reason:
				// "confirmation-preview" is not a valid region name in
				// .planning/design/global-git/visual-divergence-allowlist.txt's
				// own schema (only ggit-options-browse, ggit-apply-ceremony,
				// and ggit-apply-heading are), so there is no allowlist-
				// sourced predicate to port here.
				uxRegionDifference(RegionConfirmationPreview, "sentinel-wrapped-preview", "GGIT-D-06",
					"internal/tuikit/ceremony.go's shared \"Exact change\" hint triggers RegionConfirmationPreview's extraction on this Global Git apply ceremony too; the real preview's production EnsureGlobalGit-composed managed block vs the dummy's frozen fixture diff is the SAME divergence RegionGGitApplyCeremony already classifies"),
			},
		},
		{
			ScreenID:              "ggit-apply-receipt",
			Interaction:           "Apply ceremony state B (result receipt): the real write has completed — NOT capturable in-process; see the live/approved-tui non-applicability reasons for the PTY frame that carries this evidence.",
			StateMarker:           "global git option",
			ApplicableLive:        false,
			ApplicableApprovedTUI: false,
			NonApplicability:      receiptNA("GGIT-D-04", applyReceiptReason),
			RequiredRegions:       []RegionName{RegionGGitApplyCeremony},
		},
	}
}

// ---------------------------------------------------------------------------
// Upload / D-08 register-key checkpoints (09-07-PLAN.md Task 2, UP-01/UP-02/
// UP-03). CaptureUploadScreens drives the create-flow wizard's step-2 "Test
// connection" pane and the identity-manager's register-key pane in-process —
// EIGHT Phase 9 states, mirroring the Phase 8 doctorSpecs() pattern:
//
//   - upload-checkbox-ready    Step 1 (Test connection), Enter from testIdle
//                              enters testUpload and RunUpload resolves —
//                              the checked D-01 checkbox successfully
//                              triggering the upload beat. RunUpload
//                              resolves synchronously in this architecture
//                              (its tea.Cmd computes command text AND
//                              outcome together — captured directly with
//                              step() rather than stepAndPendingCmd/
//                              CaptureCreateFlowScreens' own CR-02
//                              intermediate-capture technique: there is no
//                              observable "announcing, not yet resolved"
//                              transient a PTY session (or this in-process
//                              capture) can catch mid-flight, confirmed by
//                              inspecting the pending-cmd state directly —
//                              it renders no announce content at all until
//                              the cmd resolves). "upload-results" is the
//                              SAME resolved state, registered as its own
//                              screen ID with its own disposition entry.
//   - upload-results           The identical resolved state as
//                              upload-checkbox-ready — the ✓/✗ result rows.
//   - upload-checkbox-unauth   Live-only: the real backend's uploaderDeps
//                              answers an unauthenticated `gh` (LookPath ok,
//                              `auth status` non-zero). The dummy
//                              FixtureBackend has no equivalent seam (it
//                              branches by HOSTNAME only, never by an
//                              auth-status concept dummytui's no-backend
//                              ALLOWLIST forbids importing) — NOT applicable
//                              to the approved-tui surface; D-09's approval
//                              for this state lives in Task 1's committed PTY
//                              frame instead
//                              (create-flow-upload-checkbox-unauth.txt).
//   - upload-checkbox-disabled Live-only, same reasoning: uploaderDeps'
//                              LookPath fails for BOTH gh and glab (the
//                              genuinely tool-absent DISABLED shape Task 1's
//                              own PTY suite doc comment notes a real PTY
//                              harness cannot safely produce — this
//                              in-process capture CAN, since it controls the
//                              seam directly).
//   - upload-manual-fallback   The D-01 checkbox declined (toggled off) on
//                              step 0, then advanced — renders the manual
//                              fallback instructions instead of running any
//                              provider command. Reachable identically on
//                              both surfaces (no hostname/auth-deps
//                              divergence involved).
//   - register-key-modal       Identity-manager surface: the default-
//                              selected identity, 'u' opens the D-08
//                              register-key pane, which auto-runs.
//   - rotate-delete-offer      NON-APPLICABLE in this in-process gate — the
//                              D-04 offer requires a completed key-rotation
//                              commit (a real cryptographic write) this
//                              no-subprocess capture path never performs,
//                              mirroring Phase 6's own gss-apply-receipt/
//                              Phase 7's ggit-apply-receipt precedent (and
//                              Phase 5's original rotate-result/repair-result
//                              exclusion). Evidence lives in the PTY frame
//                              .planning/phases/09-upload-credentials-assist/
//                              ui-frames/identity-manager-rotate-delete-
//                              offer-default.txt
//                              (TestIdentityManager_RotateDeleteOfferDefaultsToLeave).
// ---------------------------------------------------------------------------

// uploadWizardApp boots a fresh create-flow wizard around backend at the
// fixed capture geometry, wrapped with offlineCaptureBackend — the SAME
// wrapping CaptureCreateFlowScreens applies (deterministic TestStage1/
// TestStage2 AND, per Phase 9's own additions above, deterministic
// UploadEligibility/RunUpload) so this capture never blocks on or varies
// with a real SSH/gh/glab probe.
func uploadWizardApp(backend tuikit.Backend) tea.Model {
	return freshWizard(offlineCaptureBackend{backend})
}

// uploadIdentityManagerApp boots the identity-manager root around backend at
// the fixed capture geometry, mirroring identityManagerApp exactly.
func uploadIdentityManagerApp(backend tuikit.Backend) tea.Model {
	var model tea.Model = tuikit.NewApp(backend)
	model = step(model, tea.WindowSizeMsg{Width: CaptureWidth, Height: CaptureHeight})
	return model
}

// uploadVisualSpecs returns the Phase 9 upload-surface checkpoint specs.
// RegionDispositions mirror
// .planning/design/create-flow/visual-divergence-allowlist.txt and
// .planning/design/identity-manager/visual-divergence-allowlist.txt's Phase 9
// rows verbatim (kept in sync by TestUploadVisualAllowlistMatchesRegistry).
func uploadVisualSpecs() []ScreenSpec {
	upFixtureClass := "UP-4"

	noHTML := []SurfaceNonApplicability{uxNonComparable("approved-html", upFixtureClass,
		"AGENTS.md's BINDING UI Reference rule: Phase 2's approved Bubble Tea dummy is the SOLE Phase 9 UI/UX parity target for Phases 3-10; the historical HTML/MUI artifacts are Phase-2 design history and are recorded explicitly NON-APPLICABLE for every Phase 9 comparison")}

	// uploadCommandDisposition covers the "Running: <command>" announce
	// lines: the real command names a real generated key's temp-directory
	// path and gh's PATH-resolved location; the dummy names its frozen
	// "/usr/local/bin/gh ...~/.ssh/id_ed25519_<name>.pub" shape. Both sides
	// share the literal "ssh-key add" fragment and the --type/--title
	// argument shape.
	uploadCommandDisposition := uxRegionDifferenceScoped(RegionUploadSection, "fixture-vs-live-command-path", upFixtureClass,
		"the real upload command names a real generated key's temp-directory path and the PATH-resolved gh/glab binary; the dummy renders its frozen \"/usr/local/bin/gh ...\" command shape — both sides share the \"ssh-key add\" invocation and its --title/--type arguments",
		`contains:"ssh-key add"`)
	// extractConnectivityOutput's own generic "Running"/"ssh -" triggers
	// (createflow_regions.go) also fire on the upload beat's "Running:
	// <gh/glab command>" announce line — the SAME fixture-vs-live command
	// path divergence, just visible through a second, pre-existing region.
	connectivityOverlapDisposition := uxRegionDifferenceScoped(RegionConnectivityOutput, "fixture-vs-live-command-path", upFixtureClass,
		"RegionConnectivityOutput's own generic \"Running\" trigger also matches the upload beat's announce line on this screen — the SAME command-path divergence RegionUploadSection's own disposition classifies",
		`contains:"ssh-key add"`)
	// registerKeyConnectivityOverlapDisposition is the identity-manager
	// pane's own variant of the overlap above.
	registerKeyConnectivityOverlapDisposition := uxRegionDifferenceScoped(RegionConnectivityOutput, "fixture-vs-live-command-path", upFixtureClass,
		"RegionConnectivityOutput's own generic \"Running\" trigger also matches the upload beat's announce line on the register-key pane — the SAME command-path divergence RegionUploadSection's own disposition classifies",
		`contains:"ssh-key add"`)
	// rotateDeleteOfferUploadDisposition/rotateDeleteOfferConnectivityDisposition
	// are the rotate-delete-offer-specific variants: CR-01's new KeyDetail
	// line (09-REVIEW-FIX.md) pushes a realistic rotate's tail past
	// frameBodyRows(30), so renderKeyCeremony's overflow backstop
	// deterministically trims the upload beat's own already-redundant
	// announce lines on the REAL side, while the dummy's frozen fixture
	// never models that overflow — the real pane shows no "ssh-key add"
	// text at all. Predicate flipped from contains: to absent: to match
	// this observed (and reproducible) real/dummy asymmetry; kept as
	// separate dispositions from uploadCommandDisposition/
	// registerKeyConnectivityOverlapDisposition above because
	// register-key-modal's own screens are unaffected (no overflow there)
	// and still correctly use contains:.
	rotateDeleteOfferUploadDisposition := uxRegionDifferenceScoped(RegionUploadSection, "fixture-vs-live-command-path", upFixtureClass,
		"CR-01's KeyDetail line pushes a realistic rotate's tail past the frame budget, so the overflow backstop trims the upload beat's announce lines on the real side first (by design); the dummy's frozen fixture never models that overflow",
		`absent:"ssh-key add"`)
	rotateDeleteOfferConnectivityDisposition := uxRegionDifferenceScoped(RegionConnectivityOutput, "fixture-vs-live-command-path", upFixtureClass,
		"RegionConnectivityOutput's own generic \"Running\" trigger also matches the upload beat's announce line on the rotate-delete-offer screen — the SAME overflow-driven absence rotateDeleteOfferUploadDisposition classifies",
		`absent:"ssh-key add"`)

	dispIM := []RegionDisposition{
		uxRegionDifferenceScoped(RegionSidebar, "sidebar-state", upFixtureClass,
			"real sidebar carries only the checkpoint's own seeded identities; dummy sidebar lists the full 8-identity IdentityManagerRows fixture set",
			`absent:"clientB"`),
		uxRegionDifferenceScoped(RegionHeaderStatus, "identity-count", upFixtureClass,
			"header status shows the identity count, which differs (real's small seeded set vs dummy's 8 fixtures)",
			`contains:"ids"`),
		uxRegionDifferenceScoped(RegionBreadcrumb, "identity-name", upFixtureClass,
			"the breadcrumb (\"Identities › <identity> › Register key\") embeds the selected identity's name, which differs between the real fixture (\"imgr\") and the dummy fixture (\"personal\") by construction",
			`contains:"Register key"`),
	}

	// dispCF covers the SAME identity-count fixture-vs-live divergence on
	// the create-flow wizard surface: deterministicUploadFixture seeds a
	// small real identity set (via deterministicIdentityManagerFixture,
	// reused for a real ~/.ssh) while the dummy carries its frozen
	// 8-identity fixture — the wizard's own breadcrumb ("New identity") is
	// a generic literal with no embedded identity name, so it needs no
	// disposition of its own.
	dispCF := []RegionDisposition{
		uxRegionDifferenceScoped(RegionSidebar, "sidebar-state", upFixtureClass,
			"real sidebar carries only the checkpoint's own seeded identities; dummy sidebar lists the full 8-identity IdentityManagerRows fixture set",
			`absent:"clientB"`),
		uxRegionDifferenceScoped(RegionHeaderStatus, "identity-count", upFixtureClass,
			"header status shows the identity count, which differs (real's small seeded set vs dummy's 8 fixtures)",
			`contains:"ids"`),
	}

	return []ScreenSpec{
		{
			ScreenID:              "upload-checkbox-ready",
			Interaction:           "The checked D-01 checkbox successfully triggers the upload beat: same script as upload-results (see CaptureUploadScreens' doc comment for why they share one capture).",
			StateMarker:           "Authentication key registered",
			ApplicableLive:        true,
			ApplicableApprovedTUI: true,
			NonApplicability:      noHTML,
			RequiredRegions:       []RegionName{RegionUploadSection},
			RegionDispositions:    append(append([]RegionDisposition{}, dispCF...), uploadCommandDisposition, connectivityOverlapDisposition),
		},
		{
			ScreenID:              "upload-results",
			Interaction:           "The identical resolved state as upload-checkbox-ready (see CaptureUploadScreens' doc comment).",
			StateMarker:           "key registered",
			ApplicableLive:        true,
			ApplicableApprovedTUI: true,
			NonApplicability:      noHTML,
			RequiredRegions:       []RegionName{RegionUploadSection},
			RegionDispositions:    append(append([]RegionDisposition{}, dispCF...), uploadCommandDisposition, connectivityOverlapDisposition),
		},
		{
			ScreenID:              "upload-checkbox-unauth",
			Interaction:           "Live-only: eligibility forced Unauth via uploadProbeBackend — captured for BOTH backends (so it's always present in the merged output), but held non-comparable since the surrounding fixture-vs-live layout differences would otherwise need a growing, brittle set of dispositions unrelated to this state's own purpose.",
			StateMarker:           "not logged in to",
			ApplicableLive:        true,
			ApplicableApprovedTUI: false,
			NonApplicability: append(noHTML, uxNonComparable("approved-tui", upFixtureClass,
				"captured for both backends via uploadProbeBackend, but held non-comparable: the checkbox label survives identically, but the surrounding fixture-vs-live layout (identity count, key catalog, host preview) would otherwise require dispositions unrelated to this state's own purpose — D-09's approval lives in Task 1's committed PTY frame create-flow-upload-checkbox-unauth.txt (TestCreateFlow_UploadCheckboxUnauthState)")),
			// WR-12: this spec used to require RegionUploadSection, satisfied
			// only because extractUploadSection's marker list included "not
			// logged in to" — a substring of the D-01 checkbox's OWN label,
			// which caused RegionUploadSection to falsely match on ORDINARY
			// step-0 screens that merely render the checkbox in the Unauth
			// state (never having run, or been declined). Per
			// extractUploadSection's own doc comment, RegionUploadSection is
			// scoped to "the announce/result/fallback text that appears only
			// once the beat has actually run or been explicitly declined" —
			// an offered-but-undecided checkbox is neither. The checkbox row
			// IS still covered: renderUploadCheckboxRow's own doc comment
			// places it "as the last row of the SSH form, immediately after
			// Port", inside extractFormFields' [Shift+→ hint, Key toggle)
			// window — RegionFormFields is the semantically correct region
			// for this screen's own purpose (verifying the checkbox renders)
			// and still requires the StateMarker text to be present.
			RequiredRegions: []RegionName{RegionFormFields},
		},
		{
			ScreenID:              "upload-checkbox-disabled",
			Interaction:           "Live-only: eligibility forced Disabled via uploadProbeBackend — captured for BOTH backends, held non-comparable for the same reason as upload-checkbox-unauth.",
			StateMarker:           "Auto-registration unavailable",
			ApplicableLive:        true,
			ApplicableApprovedTUI: false,
			NonApplicability: append(noHTML, uxNonComparable("approved-tui", upFixtureClass,
				"captured for both backends via uploadProbeBackend, but held non-comparable — see upload-checkbox-unauth's own reasoning. D-09's approval lives in this in-process capture alone; Task 1's PTY suite documents why a real PTY harness cannot safely produce the genuinely tool-absent DISABLED shape (TestCreateFlow_UploadCheckboxDisabledState's own doc comment)")),
			// WR-03: this used to require RegionUploadSection, satisfied only
			// because extractUploadSection's marker list included
			// "Auto-registration" — the first word of the D-01 checkbox's OWN
			// DISABLED label, which caused RegionUploadSection to falsely
			// match on ORDINARY step-0 screens that merely render the
			// checkbox in the Disabled state (never having run — there is
			// nothing TO run here). Same fix, same reasoning as
			// upload-checkbox-unauth's own RequiredRegions comment just
			// above: the checkbox row is covered by RegionFormFields, and
			// this spec's StateMarker still requires the label text present.
			RequiredRegions: []RegionName{RegionFormFields},
		},
		{
			ScreenID:              "upload-manual-fallback",
			Interaction:           "Live-only: identity-manager register-key pane with eligibility forced Unauth via uploadProbeBackend — captured for BOTH backends, held non-comparable for the same reason as upload-checkbox-unauth.",
			StateMarker:           "manually",
			ApplicableLive:        true,
			ApplicableApprovedTUI: false,
			NonApplicability: append(noHTML, uxNonComparable("approved-tui", upFixtureClass,
				"captured for both backends via uploadProbeBackend, but held non-comparable — see upload-checkbox-unauth's own reasoning. D-09's approval lives in Task 1's committed PTY frame create-flow-upload-partial-scope.txt (a different concrete scenario reaching the same manual-guidance concept — see uploadScreenToFrames' own doc comment in gate_visual_regression_test.go)")),
			RequiredRegions: []RegionName{RegionUploadSection},
		},
		{
			ScreenID:              "register-key-modal",
			Interaction:           "From the identity-manager's default-selected identity, press 'u' to open the D-08 register-key pane, which auto-runs.",
			StateMarker:           "Register",
			ApplicableLive:        true,
			ApplicableApprovedTUI: true,
			NonApplicability:      noHTML,
			RequiredRegions:       []RegionName{RegionUploadSection},
			RegionDispositions:    append(append([]RegionDisposition{}, dispIM...), uploadCommandDisposition, registerKeyConnectivityOverlapDisposition),
		},
		{
			ScreenID:              "rotate-delete-offer",
			Interaction:           "NOT capturable in-process: the D-04 offer requires a completed key-rotation commit (a real cryptographic write). See uploadVisualSpecs' doc comment.",
			StateMarker:           "Remove the old key from",
			ApplicableLive:        false,
			ApplicableApprovedTUI: false,
			NonApplicability: append(noHTML,
				uxNonComparable("live", upFixtureClass,
					"the D-04 offer requires a completed key-rotation commit (CommitRotate's real cryptographic write) this no-subprocess in-process gate never performs. Evidence lives in the PTY frame .planning/phases/09-upload-credentials-assist/ui-frames/identity-manager-rotate-delete-offer-default.txt (TestIdentityManager_RotateDeleteOfferDefaultsToLeave)"),
				uxNonComparable("approved-tui", upFixtureClass,
					"same reasoning as the live surface above: the dummy's own RotateDeleteOffer fixture is reachable only after its own KeyCeremonyPlan/CommitRotate sequence completes, which this in-process gate does not drive"),
			),
			RequiredRegions: []RegionName{RegionUploadSection},
			// RegionDispositions here are NEVER exercised by THIS in-process
			// gate (ApplicableLive/ApplicableApprovedTUI are both false
			// above) — they exist solely so TestUploadVisualAllowlistMatchesRegistry
			// can byte-sync against the Phase 9 rotate-delete-offer rows
			// e2e/identity_manager_pty_e2e_test.go's
			// TestRegisterKeyModal_CompiledRealVsLiveDummyPTY registers in
			// .planning/design/identity-manager/visual-divergence-allowlist.txt
			// (09-07-PLAN.md Task 3) — that REAL, compiled-binary PTY
			// comparison is the actual enforcement mechanism for this
			// checkpoint; this in-process gate structurally cannot drive
			// it (see NonApplicability above).
			RegionDispositions: []RegionDisposition{
				uxRegionDifferenceScoped(RegionSidebar, "sidebar-state", upFixtureClass,
					"real sidebar carries only the checkpoint's own seeded identity; dummy sidebar lists the full 8-identity IdentityManagerRows fixture set",
					`absent:"clientB"`),
				uxRegionDifferenceScoped(RegionHeaderStatus, "identity-count", upFixtureClass,
					"header status shows the identity count, which differs (real's small seeded set vs dummy's 8 fixtures)",
					`contains:"ids"`),
				rotateDeleteOfferUploadDisposition,
				rotateDeleteOfferConnectivityDisposition,
			},
		},
	}
}

// isUploadScreenID reports whether id is one of the eight Phase 9
// upload-surface checkpoint IDs registered here — used to exclude them from
// CaptureCreateFlowScreens' completeness check (they are captured
// separately; see CaptureUploadScreens' doc comment).
func isGitIgnoreScreenID(id string) bool {
	for _, spec := range gitIgnoreVisualSpecs() {
		if spec.ScreenID == id {
			return true
		}
	}
	return false
}

func isUploadScreenID(id string) bool {
	for _, spec := range uploadVisualSpecs() {
		if spec.ScreenID == id {
			return true
		}
	}
	return false
}

// CaptureUploadScreens captures the Phase 9 upload-surface checkpoints
// described in uploadVisualSpecs' doc comment. backend must answer
// eligibility Ready for its default fixture host (both newBackendForHome
// with a "gh ok"-shaped uploaderDeps and dummytui.NewFixtureBackend satisfy
// this for their respective default identities).
func CaptureUploadScreens(backend tuikit.Backend) (map[string]string, error) {
	out := make(map[string]string, 8)
	capture := func(m tea.Model) string { return normalizeTimestamps(anyView(m)) }

	// upload-results / upload-checkbox-ready: step 0 -> step 1, then Enter
	// from testIdle enters testUpload and returns RunUpload's cmd. D-02
	// auto-chains UploadRunMsg's own handler straight into TestStage1 with
	// no further keystroke — step()'s full recursive chase would run that
	// ENTIRE chain (upload -> stage1 -> stage2) synchronously in one call,
	// settling on the FINAL post-stage2 state with no intermediate render
	// (unlike a real PTY session, where each real subprocess call takes
	// real wall-clock time and the screen re-renders at each intermediate
	// Update — confirmed empirically: a naive step()-only capture landed on
	// stage-2's completed content, upload's own "Running:"/result rows
	// nowhere in it). stepAndPendingCmd (the SAME CR-02 intermediate-
	// capture technique CaptureCreateFlowScreens' own test-stage1-direct
	// uses) delivers exactly ONE message at a time, so capturing right
	// after UploadRunMsg is delivered — but BEFORE its own auto-chained
	// TestStage1 cmd fires — reaches the genuine "upload resolved" state.
	// "upload-checkbox-ready" reuses the SAME resolved capture as
	// "upload-results" (mirroring CaptureCreateFlowScreens' own
	// "test-stage1-direct"/"test-stage1-pass" precedent of assigning one
	// capture to two keys): the checked checkbox's OWN pre-run state
	// renders no distinguishable RegionUploadSection content of its own
	// (its label text is ordinary step-0 form body shared with every
	// pre-Phase-9 create-flow spec, not upload-beat content — see
	// extractUploadSection's doc comment), so "ready" here means what Task
	// 1's own promoted PTY frame for this State ID actually shows: the
	// checked checkbox having successfully triggered the upload dispatch.
	m := uploadWizardApp(backend)
	m = keyEnter(m) // step 0 -> step 1
	var uploadCmd tea.Cmd
	m, uploadCmd = stepAndPendingCmd(m, tea.KeyPressMsg{Code: tea.KeyEnter}) // testIdle -> testUpload
	if uploadCmd == nil {
		return nil, fmt.Errorf("screenshot: CaptureUploadScreens: expected a pending upload cmd from testIdle -> testUpload, got nil")
	}
	uploadMsg := uploadCmd()
	if uploadMsg == nil {
		return nil, fmt.Errorf("screenshot: CaptureUploadScreens: the pending upload cmd resolved to a nil message")
	}
	m, _ = stepAndPendingCmd(m, uploadMsg) // deliver UploadRunMsg; do NOT fire its auto-chained TestStage1 cmd
	out["upload-results"] = capture(m)
	out["upload-checkbox-ready"] = out["upload-results"]

	// upload-checkbox-unauth / upload-checkbox-disabled: eligibility forced
	// via a canned UploadEligibility answer (uploadProbeBackend, below) —
	// the in-process gate's way to reach these states without a real
	// gh/glab on the machine running the gate. Applied IDENTICALLY to
	// whichever backend is passed in (real or dummy): the override fully
	// replaces UploadEligibility's answer regardless of the underlying
	// backend, so both surfaces render through the SAME shared tuikit code
	// against the SAME forced state — genuinely comparable, not merely
	// live-only.
	//
	// uploadProbeBackend must be the OUTERMOST wrapper here (embedding
	// offlineCaptureBackend, not the reverse) so ITS UploadEligibility
	// override — not offlineCaptureBackend's own deterministic-Ready
	// override — wins Go's embedded-method resolution.
	probeApp := func(state tuikit.UploadEligibilityState) tea.Model {
		return freshWizard(uploadProbeBackend{Backend: offlineCaptureBackend{backend}, state: state})
	}
	out["upload-checkbox-unauth"] = capture(probeApp(tuikit.UploadEligibilityUnauth))
	out["upload-checkbox-disabled"] = capture(probeApp(tuikit.UploadEligibilityDisabled))

	// upload-manual-fallback: identity-manager surface, register-key pane,
	// eligibility forced Unauth the SAME way, applied identically to
	// whichever backend is passed in. renderRegisterKey shows the frozen
	// UploadManualHeading whenever the plan's State is not Ready.
	if n := len(backend.InitialState().Identities); n > 0 {
		fallback := uploadIdentityManagerApp(uploadProbeBackend{Backend: backend, state: tuikit.UploadEligibilityUnauth})
		fallback = keyRune(fallback, 'u')
		out["upload-manual-fallback"] = capture(fallback)
	}

	// register-key-modal: identity-manager surface, default-selected
	// identity, 'u' opens the D-08 pane, which auto-runs on open.
	if n := len(backend.InitialState().Identities); n > 0 {
		reg := uploadIdentityManagerApp(backend)
		reg = keyRune(reg, 'u')
		out["register-key-modal"] = capture(reg)
	}

	for _, spec := range uploadVisualSpecs() {
		if !spec.ApplicableLive && !spec.ApplicableApprovedTUI {
			continue
		}
		text, ok := out[spec.ScreenID]
		if !ok || strings.TrimSpace(text) == "" {
			return nil, fmt.Errorf("screenshot: CaptureUploadScreens: required frame %q is missing or empty", spec.ScreenID)
		}
	}
	return out, nil
}

// uploadProbeBackend wraps a tuikit.Backend and replaces UploadEligibility
// with a canned answer — mirroring offlineCaptureBackend's own
// wrap-and-override pattern (above) for TestStage1/TestStage2. This is the
// in-process gate's ONLY way to reach the UNAUTH/DISABLED checkbox states
// without a real gh/glab on the machine running the gate: it swaps the
// ANSWER directly, at the tuikit.Backend interface boundary, rather than
// reaching into a specific backend's own exec-detection seam (which would
// require an import cmd/gitid cannot offer to internal/screenshot, or a
// cross-package interface only the real backend could satisfy).
type uploadProbeBackend struct {
	tuikit.Backend
	state tuikit.UploadEligibilityState
}

func (u uploadProbeBackend) UploadEligibility(hostname string) tea.Cmd {
	return func() tea.Msg {
		return tuikit.UploadEligibilityMsg{Hostname: hostname, View: tuikit.UploadEligibilityView{
			State: u.state, ProviderName: "GitHub", ToolName: "gh", Hostname: "github.com",
		}}
	}
}

// RegisterKeyPlan is the identity-manager register-key pane's own
// eligibility probe (D-08) — overridden the same way UploadEligibility is
// above, for upload-manual-fallback: renderRegisterKey shows the frozen
// UploadManualHeading whenever the plan's State is not Ready, which is
// otherwise unreachable in-process without a real gh/glab.
func (u uploadProbeBackend) RegisterKeyPlan(name string) tea.Cmd {
	return func() tea.Msg {
		return tuikit.RegisterKeyPlanMsg{Name: name, View: tuikit.UploadEligibilityView{
			State: u.state, ProviderName: "GitHub", ToolName: "gh", Hostname: "github.com",
		}}
	}
}
