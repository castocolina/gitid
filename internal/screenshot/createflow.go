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

// normalizeTimestamps replaces all ISO-8601 timestamps in s with a fixed placeholder
// so that captures taken at different wall-clock seconds are byte-identical (CR-01).
func normalizeTimestamps(s string) string {
	return timestampPattern.ReplaceAllString(s, "<timestamp>")
}

// CaptureWidth/CaptureHeight are the D-04/D-24 fixed capture geometry —
// the SAME 100x30 values screenshot-tui (internal/screenshot/tui.go,
// tui_capture_test.go) already uses.
const (
	CaptureWidth  = 100
	CaptureHeight = 30
)

// CreateFlowScreenIDs is retained only by the text-only legacy capture helper.
// Visual packet completeness is derived from ScreenSpecRegistry instead.
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

// ScreenSpec is the typed capture contract for one create-flow logical screen.
type ScreenSpec struct {
	// ScreenID is the logical identifier (one of CreateFlowScreenIDs).
	ScreenID string
	// Route is the canonical HTML route for this screen (e.g. "/create-flow/ssh-form-filled").
	Route string
	// Interaction is a human-readable description of the script steps that produce this state.
	Interaction string
	// StateMarker is the unique string that MUST appear in the captured text to
	// confirm the correct state was captured. A route name alone is never sufficient.
	StateMarker string
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
	// NonApplicableReason is the explicit justification when a surface is not applicable.
	// Must be non-empty when any Applicable* flag is false.
	NonApplicableReason string
	// RequiredRegions are the semantic regions that must be present in every
	// applicable capture. They make missing evidence a validation error.
	RequiredRegions []RegionName
}

// ScreenSpecRegistry returns the canonical typed ScreenSpec registry consumed
// by capture, validation, region generation, and publication. Every logical
// screen ID in CreateFlowScreenIDs has exactly one spec. Same-route interaction
// variants carry explicit VariantOf + VariantRationale metadata.
func ScreenSpecRegistry() []ScreenSpec {
	return []ScreenSpec{
		{
			ScreenID:               "ssh-form-filled",
			Route:                  "/create-flow/ssh-form-filled",
			Interaction:            "Open the create wizard with default prefix 'acme'; form is pre-filled.",
			StateMarker:            "IdentitiesOnly yes",
			ApplicableLive:         true,
			ApplicableApprovedTUI:  true,
			ApplicableApprovedHTML: true,
			RequiredRegions:        []RegionName{RegionFormFields, RegionHostPreview},
		},
		{
			ScreenID:               "reuse-key-vs-generate",
			Route:                  "/create-flow/reuse-key-vs-generate",
			Interaction:            "Tab to the key-source toggle (4 Tabs), then press Right to select Reuse.",
			StateMarker:            "Reuse an existing key",
			ApplicableLive:         true,
			ApplicableApprovedTUI:  true,
			ApplicableApprovedHTML: true,
			RequiredRegions:        []RegionName{RegionKeySection},
		},
		{
			ScreenID:               "reuse-manual-path",
			Route:                  "/create-flow/reuse-key-vs-generate",
			Interaction:            "From reuse mode, press Left to select the manual-path row; the manual-path text input becomes active.",
			StateMarker:            "Enter a path manually",
			ApplicableLive:         true,
			ApplicableApprovedTUI:  true,
			ApplicableApprovedHTML: true,
			RequiredRegions:        []RegionName{RegionKeySection},
			VariantOf:              "reuse-key-vs-generate",
			VariantRationale:       "No separate HTML route exists for the manual-path interaction variant; it shares /create-flow/reuse-key-vs-generate.",
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
			VariantOf:              "ssh-form-filled",
			VariantRationale:       "No separate HTML route exists for mouse-focused-field; it shares /create-flow/ssh-form-filled with a different focus state.",
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
		},
		{
			ScreenID:               "test-stage2-by-alias",
			Route:                  "/create-flow/test-stage2-by-alias",
			Interaction:            "After stage-1 completes (D-04 auto-chain), wait for stage-2 result. Capture at testStage2 (both stages done).",
			StateMarker:            "Next: Git identity",
			ApplicableLive:         true,
			ApplicableApprovedTUI:  true,
			ApplicableApprovedHTML: true,
			RequiredRegions:        []RegionName{RegionConnectivityOutput},
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
		},
		{
			ScreenID:            "reuse-manual-resolved",
			Interaction:         "Select Reuse, focus the manual path input, type a sandbox key path, and wait for its algorithm and fingerprint.",
			StateMarker:         "ssh-ed25519",
			ApplicableLive:      true,
			NonApplicableReason: "The approved sources have no resolved sandbox-key state.",
			RequiredRegions:     []RegionName{RegionKeySection},
		},
		{
			ScreenID:            "test-stage1-pass",
			Interaction:         "Run stage 1 through the pass fake SSH and capture its completed PASS state while stage 2 is pending.",
			StateMarker:         "Hi user!",
			ApplicableLive:      true,
			NonApplicableReason: "The approved sources do not execute the real external SSH process.",
			RequiredRegions:     []RegionName{RegionConnectivityOutput},
		},
		{
			ScreenID:            "test-stage2-proof-top",
			Interaction:         "Complete both pass stages, focus the proof viewport with raw v, and capture the initial proof frame.",
			StateMarker:         "Proof viewport focused",
			ApplicableLive:      true,
			NonApplicableReason: "The approved sources do not execute the real external SSH process.",
			RequiredRegions:     []RegionName{RegionConnectivityOutput},
		},
		{
			ScreenID:            "test-stage2-proof-bottom",
			Interaction:         "From the focused proof viewport, send raw PgDn until the ssh -G identityfile output is visible.",
			StateMarker:         "identityfile",
			ApplicableLive:      true,
			NonApplicableReason: "The approved sources do not execute the real external SSH process.",
			RequiredRegions:     []RegionName{RegionConnectivityOutput},
		},
		{
			ScreenID:            "test-stage2-proof-right",
			Interaction:         "From the focused proof viewport, send raw Right until a long command suffix is visible.",
			StateMarker:         "→ cols",
			ApplicableLive:      true,
			NonApplicableReason: "The approved sources do not execute the real external SSH process.",
			RequiredRegions:     []RegionName{RegionConnectivityOutput},
		},
		{
			ScreenID:            "test-reachable-not-uploaded",
			Interaction:         "Run both stages through the denied fake SSH and capture the completed copy-public-key warning state.",
			StateMarker:         "copy public key",
			ApplicableLive:      true,
			NonApplicableReason: "The approved sources do not execute the real external SSH process.",
			RequiredRegions:     []RegionName{RegionConnectivityOutput},
		},
		{
			ScreenID:            "test-hard-failure-retry",
			Interaction:         "Run stage 1 through the timeout fake SSH and capture the completed hard-failure retry state.",
			StateMarker:         "Retry (Enter)",
			ApplicableLive:      true,
			NonApplicableReason: "The approved sources do not execute the real external SSH process.",
			RequiredRegions:     []RegionName{RegionConnectivityOutput},
		},
		{
			ScreenID:            "confirm-summary",
			Interaction:         "Navigate to the confirmation ceremony, focus its viewport with raw v, and capture the summary/key-path frame before write.",
			StateMarker:         "Exact change focused",
			ApplicableLive:      true,
			NonApplicableReason: "The approved sources do not expose the production ceremony viewport.",
			RequiredRegions:     []RegionName{RegionConfirmationPreview},
		},
		{
			ScreenID:            "confirm-managed-block",
			Interaction:         "From the focused confirmation viewport, send raw PgDn until the managed-block END sentinel is visible before write.",
			StateMarker:         "# END gitid managed:",
			ApplicableLive:      true,
			NonApplicableReason: "The approved sources do not expose the production ceremony viewport.",
			RequiredRegions:     []RegionName{RegionConfirmationPreview},
		},
	}
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

// ValidateCapturedState reports whether the captured text satisfies the spec's
// state marker contract. Returns nil if the marker is present, an error if
// absent or if the marker belongs to a different screen's spec.
func ValidateCapturedState(spec ScreenSpec, capturedText string) error {
	if spec.StateMarker == "" {
		return fmt.Errorf("screenshot: ValidateCapturedState: spec %q has no state marker", spec.ScreenID)
	}
	if !strings.Contains(capturedText, spec.StateMarker) {
		return fmt.Errorf("screenshot: ValidateCapturedState: spec %q state marker %q absent from captured text", spec.ScreenID, spec.StateMarker)
	}
	return nil
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
		if s.StateMarker == "" {
			return fmt.Errorf("screenshot: ValidateScreenSpecs: spec %q has empty state marker", s.ScreenID)
		}
		if prior, exists := seenMarkers[s.StateMarker]; exists && prior != s.ScreenID {
			return fmt.Errorf("screenshot: ValidateScreenSpecs: marker %q is shared by %q and %q", s.StateMarker, prior, s.ScreenID)
		}
		seenMarkers[s.StateMarker] = s.ScreenID
		if s.ApplicableApprovedHTML && s.Route == "" {
			return fmt.Errorf("screenshot: ValidateScreenSpecs: HTML-applicable spec %q has no route", s.ScreenID)
		}
		if (!s.ApplicableLive || !s.ApplicableApprovedTUI || !s.ApplicableApprovedHTML) && s.NonApplicableReason == "" {
			return fmt.Errorf("screenshot: ValidateScreenSpecs: spec %q lacks a non-applicability reason", s.ScreenID)
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
				Outcome: tuikit.TestOutcomeReachableNotUploaded,
				Command: o.Backend.Stage1Command(spec),
				Detail:  "git@" + spec.Hostname + ": Permission denied (publickey).",
			},
		}
	}
}

func (o offlineCaptureBackend) TestStage2(spec tuikit.CreateSpec) tea.Cmd {
	return func() tea.Msg {
		return tuikit.WizardStageMsg{
			Stage: 2,
			Result: tuikit.TestResultView{
				Outcome: tuikit.TestOutcomeReachableNotUploaded,
				Command: o.Backend.Stage2Command(spec),
				Detail:  "identityfile " + spec.KeyPath,
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
// fixed script above and returns the rendered text at every
// CreateFlowScreenIDs checkpoint, keyed by screen ID.
// The backend is wrapped with offlineCaptureBackend to ensure TestStage1 and
// TestStage2 resolve immediately without network calls or tick timers (D-22).
//
// All captures are normalized: timestamps in backup file names are replaced
// with "<timestamp>" so the output is byte-identical across wall-clock seconds
// (CR-01 determinism contract).
func CaptureCreateFlowScreens(backend tuikit.Backend) map[string]string {
	backend = offlineCaptureBackend{backend}
	out := make(map[string]string, len(CreateFlowScreenIDs))
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

	// reuse-manual-path: from reuseIdx=0, a single Left wraps DIRECTLY to
	// the trailing manual-path row (D-10's own wraparound arithmetic),
	// regardless of how many candidates the backend's scan returns.
	m = keyLeft(m)
	out["reuse-manual-path"] = capture(m)

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

	return out
}
