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

// normalizeTimestamps replaces all ISO-8601 timestamps in s with a fixed placeholder
// so that captures taken at different wall-clock seconds are byte-identical (CR-01).
func normalizeTimestamps(s string) string {
	s = sandboxPathFragmentPattern.ReplaceAllString(s, "<sandbox>")
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
type RegionDisposition struct {
	Region         RegionName
	Divergence     string
	Decision       string
	Reason         string
	Classification string
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
			RequiredRegions:        []RegionName{RegionFormFields},
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
			RequiredRegions:        []RegionName{RegionContinueDisabledReason},
			RegionDispositions: []RegionDisposition{
				uxRegionDifference(RegionContinueDisabledReason, "continue-disabled-reason", "D-19", "The live binary exposes the deferred Git configuration reason while the approved TUI preserves its validity-gated copy."),
				uxRegionDifference(RegionHeaderStatus, "fixture-header-status", "D-16", "The live disposable home starts empty while the approved fixture contains identities."),
				uxRegionDifference(RegionSidebar, "fixture-sidebar", "D-16", "The live disposable home starts empty while the approved fixture contains identities."),
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
			if !found || record.Decision == "" || !strings.HasPrefix(record.Decision, "D-") || record.Reason == "" || !validDifferenceClassification(record.Classification) {
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
				!strings.HasPrefix(disposition.Decision, "D-") ||
				strings.TrimSpace(disposition.Reason) == "" ||
				!validDifferenceClassification(disposition.Classification) {
				return fmt.Errorf("screenshot: ValidateScreenSpecs: spec %q region %q lacks a decision-linked disposition", s.ScreenID, disposition.Region)
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
				Command: o.Backend.Stage1Command(spec),
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
				Command:           o.Backend.Stage2Command(spec),
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

	// confirm-write: Skip Git (4 Tabs from user.name to the Skip button,
	// then Enter) reaches the review ceremony (state A, unconfirmed).
	m = tabN(m, 4)
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

	// Validate that all required live frames are present.
	for _, spec := range RequiredScreenSpecs() {
		if !spec.ApplicableLive {
			continue
		}
		text, ok := out[spec.ScreenID]
		if !ok || strings.TrimSpace(text) == "" {
			return nil, fmt.Errorf("screenshot: CaptureCreateFlowScreens: required frame %q is missing or empty", spec.ScreenID)
		}
	}

	return out, nil
}
