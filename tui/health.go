package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"github.com/castocolina/gitid/internal/doctor"
)

// familyState tracks the async loading state of a doctor check family.
// Declared here; shared with the confirm sub-model.
type familyState int

const (
	familyLoading familyState = iota
	familyLoaded
	familyError
)

// healthViewModel is the Health view sub-model for the Phase 5.6 two-pane layout.
// It streams all 8 doctor check families asynchronously as tea.Cmd goroutines and
// renders findings per family with per-family spinners. It is a sub-model, NOT a
// full-screen model: its view() renders into a bounded mainWidth column, and it
// contains no push/pop navigation (D-15).
//
// Ported from the Phase 5.5 dashboardModel with the following changes:
//   - update() returns (healthViewModel, tea.Cmd) not (screenModel, tea.Cmd)
//   - view() accepts width/height bounds (bounded main pane render)
//   - No pushCmd / newIdentityListScreen calls
//   - Severity glyph fix: warnings render "!" not "✗" (D-10, UI-SPEC Eval #2)
//   - badgesFromFindings helper exports badge map for the sidebar (D-08)
type healthViewModel struct {
	families []familyState
	findings map[doctor.Family][]doctor.Finding
	spinners []spinner.Model

	// selected is the index (into doctor.Families()) of the family the user has
	// navigated to with ↑/↓. The contextual rail and the main pane both highlight
	// it, so the "↑↓ move" footer affordance is real, not a lie (D-5).
	selected int

	// width/height are the bounds of the main pane assigned at render time.
	width, height int
	// runID is the stale-result guard counter (RESEARCH Pitfall 4 / PATTERNS Pattern B).
	// Each refresh increments runID; incoming familyResultMsg with an old runID is dropped.
	runID int

	doctorDeps doctor.Deps
	deps       tuiDeps
}

// newHealthModel constructs a healthViewModel with all 8 families in familyLoading
// and one spinner per family. The families/spinners slices are sized dynamically
// from doctor.Families() so no fixed-array drift when new families are added.
func newHealthModel(d tuiDeps) healthViewModel {
	n := len(doctor.Families())
	spins := make([]spinner.Model, n)
	for i := range spins {
		s := spinner.New()
		s.Spinner = spinner.Dot
		spins[i] = s
	}
	return healthViewModel{
		families:   make([]familyState, n),
		findings:   make(map[doctor.Family][]doctor.Finding),
		spinners:   spins,
		doctorDeps: d.doctor,
		deps:       d,
	}
}

// moveSelection moves the highlighted family by delta, clamped to the valid
// range. It is the navigation primitive behind the Health view's ↑/↓ keys (D-5).
func (m healthViewModel) moveSelection(delta int) healthViewModel {
	n := len(doctor.Families())
	if n == 0 {
		return m
	}
	m.selected += delta
	if m.selected < 0 {
		m.selected = 0
	}
	if m.selected >= n {
		m.selected = n - 1
	}
	return m
}

// selectedFamily returns the doctor.Family currently highlighted in the rail.
func (m healthViewModel) selectedFamily() doctor.Family {
	fams := doctor.Families()
	if m.selected < 0 || m.selected >= len(fams) {
		return ""
	}
	return fams[m.selected]
}

// familyIndex returns the 0-based index of fam in doctor.Families(), or -1 if not found.
func familyIndex(fam doctor.Family) int {
	for i, f := range doctor.Families() {
		if f == fam {
			return i
		}
	}
	return -1
}

// init starts one tea.Cmd per doctor family and one spinner.Tick per spinner.
// This produces a tea.Batch of 8 family cmds + 8 spinner ticks = 16 total entries.
// The Tick seed is mandatory: without it the spinner.Model never receives its first
// animation frame and the loading UI is frozen (PATTERNS Pattern C, RESEARCH Pitfall 4).
func (m healthViewModel) init() (healthViewModel, tea.Cmd) {
	fams := doctor.Families()
	cmds := make([]tea.Cmd, 0, len(fams)+len(m.spinners))
	for _, fam := range fams {
		cmds = append(cmds, makeFamilyCmd(m.runID, fam, m.doctorDeps))
	}
	for i := range m.spinners {
		cmds = append(cmds, m.spinners[i].Tick)
	}
	return m, tea.Batch(cmds...)
}

// makeFamilyCmd selects the per-family CheckFn from d and returns a tea.Cmd that
// runs it in a goroutine, producing a familyResultMsg on completion.
//
// Critical invariants (ported verbatim from Phase 5.5 dashboard.go):
//   - recover() wrap: a panicking check converts to familyResultMsg{err} instead
//     of crashing the program (T-05.6-08, PATTERNS Pattern D, Pitfall 7).
//   - nil-guard: a nil CheckFn returns an explicit error (never a silent pass).
//     This is the D-16 anti-blindspot mitigation: an unwired check must error,
//     not silently report "✓ all checks passed" (T-05.6-09).
func makeFamilyCmd(runID int, fam doctor.Family, d doctor.Deps) tea.Cmd {
	var fn doctor.CheckFn
	switch fam {
	case doctor.FamilyDeps:
		fn = d.CheckDeps
	case doctor.FamilyPerms:
		fn = d.CheckPerms
	case doctor.FamilyCoherence:
		fn = d.CheckCoherence
	case doctor.FamilyOrphans:
		fn = d.CheckOrphans
	case doctor.FamilySigning:
		fn = d.CheckSigning
	case doctor.FamilyAgent:
		fn = d.CheckAgent
	case doctor.FamilyBaseline:
		fn = d.CheckBaseline
	case doctor.FamilyOverlap:
		fn = d.CheckOverlap
	case doctor.FamilyRedundancy:
		fn = d.CheckRedundancy
	}
	return func() (msg tea.Msg) {
		// recover() wrap: converts any panic inside the doctor check goroutine into
		// an error result so the UI renders "✗ check failed" instead of crashing.
		defer func() {
			if r := recover(); r != nil {
				msg = familyResultMsg{
					runID:  runID,
					family: fam,
					err:    fmt.Errorf("doctor check %q panicked: %v", string(fam), r),
				}
			}
		}()
		// nil-guard: a nil fn means the CheckFn field was never wired.
		// Surface as an error (not a silent pass) — the recurring injected-seam
		// blindspot (D-16) that hid the Overlap check in Phase 5.5 (CR-02).
		if fn == nil {
			return familyResultMsg{
				runID:  runID,
				family: fam,
				err:    fmt.Errorf("doctor check %q is not wired in the TUI", string(fam)),
			}
		}
		return familyResultMsg{runID: runID, family: fam, findings: fn(d)}
	}
}

// update handles messages for the health view sub-model and returns the updated
// model and any resulting tea.Cmd. Called from rootModel.Update when the active
// view is healthView.
func (m healthViewModel) update(msg tea.Msg) (healthViewModel, tea.Cmd) {
	switch msg := msg.(type) {

	case familyResultMsg:
		// Stale-result guard: drop results from a previous refresh runID (RESEARCH Pitfall 4).
		// This prevents a slow old run from overwriting a fast new run's results.
		if msg.runID != m.runID {
			return m, nil
		}
		idx := familyIndex(msg.family)
		if idx < 0 {
			return m, nil
		}
		if msg.err != nil {
			m.families[idx] = familyError
		} else {
			m.families[idx] = familyLoaded
		}
		m.findings[msg.family] = msg.findings
		return m, nil

	case spinner.TickMsg:
		// Propagate spinner tick to every loading family's spinner.
		for i := range m.spinners {
			if m.families[i] == familyLoading {
				var cmd tea.Cmd
				m.spinners[i], cmd = m.spinners[i].Update(msg)
				return m, cmd
			}
		}
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	}

	return m, nil
}

// refresh increments the runID and re-streams all 8 families. Called when the
// user presses 'r' in the health view. The stale-guard in update() ensures any
// in-flight results from the old run are silently dropped.
func (m healthViewModel) refresh() (healthViewModel, tea.Cmd) {
	m.runID++
	for i := range m.families {
		m.families[i] = familyLoading
	}
	m.findings = make(map[doctor.Family][]doctor.Finding)
	m, cmd := m.init()
	return m, cmd
}

// view renders the health view into a bounded pane of width w and height h.
// Each family is rendered as a StylePanel bordered block. Loading families show
// a spinner + "checking...". Loaded families show their findings or "✓ all clear".
// Error families show "✗ check failed".
//
// Glyph contract (D-10, UI-SPEC Eval #2, LOCKED):
//   - Warning → "!" yellow (NEVER "✗")
//   - Error/Critical → "✗" red
//   - Info → "~" cyan
func (m healthViewModel) view(w, _ int) string {
	_ = w // reserved for potential per-panel width clamping
	var sb strings.Builder

	fams := doctor.Families()
	for i, fam := range fams {
		// Family header. The selected family (rail ↑/↓) is marked so the content
		// pane visibly tracks the rail cursor (D-5).
		marker := "  "
		if i == m.selected {
			marker = "▸ "
		}
		header := fmt.Sprintf("%s=== %s ===", marker, string(fam))
		if i == m.selected {
			sb.WriteString(StyleTabActive.Render(header))
		} else {
			sb.WriteString(StyleHeader.Render(header))
		}
		sb.WriteString("\n")

		switch m.families[i] {
		case familyLoading:
			sb.WriteString("  ")
			sb.WriteString(m.spinners[i].View())
			sb.WriteString(" checking...\n")

		case familyError:
			sb.WriteString(SeverityStyle(doctor.SeverityError).Render("  ✗ check failed"))
			sb.WriteString("\n")

		case familyLoaded:
			famFindings := m.findings[fam]
			if len(famFindings) == 0 {
				sb.WriteString(StylePass.Render("  ✓ all clear"))
				sb.WriteString("\n")
			} else {
				for _, f := range famFindings {
					sb.WriteString(renderFinding(f))
				}
			}
		}

		if i < len(fams)-1 {
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// renderFinding returns a styled multi-line string for one doctor.Finding.
//
// Glyph contract (LOCKED by D-10 / UI-SPEC Eval #2):
//   - SeverityWarning → glyph "!" (yellow), labeled "[warning]"
//   - SeverityError / SeverityCritical → glyph "✗" (red), labeled per level
//   - SeverityInfo → glyph "~" (cyan), labeled "[info]"
//
// asciiMode() is consulted for terminals that cannot render UTF-8 glyphs.
//
// Format:
//
//	{glyph} {Title} [{severity}]
//	  {Explanation}
//	  fix: {SuggestedFix}
//	  [fix]  ← only when f.Fix != nil
func renderFinding(f doctor.Finding) string {
	glyph := SeverityGlyph(f.Severity, asciiMode())
	severityStyle := SeverityStyle(f.Severity)
	titleLine := severityStyle.Render("  " + glyph + " " + f.Title)

	// Inline severity label.
	switch f.Severity {
	case doctor.SeverityCritical:
		titleLine += " [critical]"
	case doctor.SeverityError:
		titleLine += " [error]"
	case doctor.SeverityWarning:
		titleLine += " [warning]"
	case doctor.SeverityInfo:
		titleLine += " [info]"
	}

	var s strings.Builder
	s.WriteString(titleLine)
	s.WriteString("\n")
	if f.Explanation != "" {
		s.WriteString(StyleBody.PaddingLeft(4).Render(f.Explanation))
		s.WriteString("\n")
	}
	if f.SuggestedFix != "" {
		s.WriteString(StyleFaint.PaddingLeft(4).Render("fix: " + f.SuggestedFix))
		s.WriteString("\n")
	}
	if f.Fix != nil {
		if f.Fix.Summary != "" {
			s.WriteString(StyleFaint.PaddingLeft(4).Render("Fix: " + f.Fix.Summary))
			s.WriteString("\n")
		}
		s.WriteString("    [fix]\n")
	}
	return s.String()
}

// badgesFromFindings derives a per-identity severity map from the full findings
// map. For each finding that has a non-empty IdentityName, the identity's badge
// is updated to the worst (highest) severity seen across all findings.
//
// Findings with an empty IdentityName are global (e.g. tool-not-found,
// ssh-agent-missing) and are not attributed to any specific identity.
//
// This map is handed to sidebarModel.badges so sidebar rows show a live health
// badge reflecting the real doctor run (D-08).
func badgesFromFindings(findings map[doctor.Family][]doctor.Finding) map[string]doctor.Severity {
	badges := make(map[string]doctor.Severity)
	for _, famFindings := range findings {
		for _, f := range famFindings {
			if f.IdentityName == "" {
				// Global finding — not identity-scoped; skip.
				continue
			}
			current, exists := badges[f.IdentityName]
			if !exists || f.Severity > current {
				badges[f.IdentityName] = f.Severity
			}
		}
	}
	return badges
}
