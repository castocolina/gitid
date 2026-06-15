package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"github.com/castocolina/gitid/internal/doctor"
)

// familyState tracks the async loading state of a doctor check family.
type familyState int

const (
	familyLoading familyState = iota
	familyLoaded
	familyError
)

// dashboardModel is the TUI home screen. It streams seven doctor check families
// in independently as tea.Cmd goroutines (D-09) and renders findings via lipgloss.
// runID provides a stale-result guard (RESEARCH Pitfall 4).
type dashboardModel struct {
	families   [7]familyState
	findings   map[doctor.Family][]doctor.Finding
	spinners   [7]spinner.Model
	width      int
	height     int
	runID      int
	doctorDeps doctor.Deps
}

// newDashboardModel constructs a dashboardModel with all families in familyLoading
// and spinners initialized.
func newDashboardModel(d doctor.Deps) dashboardModel {
	var spins [7]spinner.Model
	for i := range spins {
		s := spinner.New()
		s.Spinner = spinner.Dot
		spins[i] = s
	}
	return dashboardModel{
		findings:   make(map[doctor.Family][]doctor.Finding),
		spinners:   spins,
		doctorDeps: d,
	}
}

// familyIndex returns the index of a Family in the Families() slice (0-based).
func familyIndex(fam doctor.Family) int {
	for i, f := range doctor.Families() {
		if f == fam {
			return i
		}
	}
	return -1
}

// init starts one tea.Cmd per doctor family (Batch of 7). Each Cmd calls the
// matching Check* field on Deps, producing a familyResultMsg when done. This
// is the D-09 async per-family streaming pattern; doctor.Run is never called
// (RESEARCH Pitfall 5). Spinner ticks are not included in the Batch — the
// spinner.Model generates its own ticks when its Update method is called.
func (m dashboardModel) init() (dashboardModel, tea.Cmd) {
	fams := doctor.Families()
	cmds := make([]tea.Cmd, len(fams))
	for i, fam := range fams {
		cmds[i] = makeFamilyCmd(m.runID, fam, m.doctorDeps)
	}
	return m, tea.Batch(cmds...)
}

// makeFamilyCmd selects the per-family Check* field from Deps and returns a
// tea.Cmd that calls it in a goroutine, producing a familyResultMsg on completion.
// The runID is embedded in the message for the stale-result guard (Pitfall 4).
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
	}
	return func() tea.Msg {
		var findings []doctor.Finding
		if fn != nil {
			findings = fn(d)
		}
		return familyResultMsg{runID: runID, family: fam, findings: findings}
	}
}

// update handles messages for the dashboard screen (screenModel interface).
func (m dashboardModel) update(msg tea.Msg) (screenModel, tea.Cmd) {
	switch msg := msg.(type) {
	case familyResultMsg:
		// Stale-result guard: drop messages from a previous refresh run (Pitfall 4).
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

	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, keys.Refresh):
			m.runID++
			// Reset all families to loading and clear findings.
			for i := range m.families {
				m.families[i] = familyLoading
			}
			m.findings = make(map[doctor.Family][]doctor.Finding)
			_, cmd := m.init()
			return m, cmd
		case key.Matches(msg, keys.Select):
			// Enter drills into the identity list (Task 2 wires newIdentityListScreen).
			return m, pushCmd(newIdentityListScreen(m.doctorDeps))
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case spinner.TickMsg:
		// Propagate spinner tick to the right spinner.
		for i := range m.spinners {
			if m.families[i] == familyLoading {
				var cmd tea.Cmd
				m.spinners[i], cmd = m.spinners[i].Update(msg)
				return m, cmd
			}
		}
		return m, nil
	}

	return m, nil
}

// view renders the dashboard as a vertical list of family panels with loading
// spinners or findings per family. Implements the UI-SPEC Screen 1 layout.
func (m dashboardModel) view() string {
	var sb strings.Builder

	// Screen title.
	sb.WriteString(StyleTitle.Render("gitid — Doctor Dashboard"))
	sb.WriteString("\n\n")

	// Render minimum-width guard.
	if m.width > 0 && m.width < 80 {
		sb.WriteString("Terminal too narrow — resize to at least 80 columns")
		return sb.String()
	}

	fams := doctor.Families()
	fixableCount := 0

	for i, fam := range fams {
		// Family header.
		header := fmt.Sprintf("=== %s ===", string(fam))
		sb.WriteString(StyleHeader.Render(header))
		sb.WriteString("\n")

		switch m.families[i] {
		case familyLoading:
			sb.WriteString("  ")
			sb.WriteString(m.spinners[i].View())
			sb.WriteString(" checking...\n")

		case familyError:
			sb.WriteString(StyleFinding.Foreground(SeverityStyle(doctor.SeverityError).GetForeground()).
				Render("  ✗ check failed"))
			sb.WriteString("\n")

		case familyLoaded:
			famFindings := m.findings[fam]
			if len(famFindings) == 0 {
				sb.WriteString(StylePass.Render("  ✓ all checks passed"))
				sb.WriteString("\n")
			} else {
				for _, f := range famFindings {
					sb.WriteString(renderFinding(f))
					if f.Fix != nil {
						fixableCount++
					}
				}
			}
		}

		if i < len(fams)-1 {
			sb.WriteString("\n")
		}
	}

	// Footer fix hint (D-11): shown when fixable findings exist.
	if fixableCount > 0 {
		sb.WriteString("\n")
		sb.WriteString(StyleFaint.Render(
			fmt.Sprintf("  %d fix(es) available — run 'gitid doctor --fix' to apply", fixableCount)))
		sb.WriteString("\n")
	}

	// Help footer.
	sb.WriteString("\n")
	sb.WriteString(StyleFaint.Render("  q quit  Enter identities  r refresh  ? help"))
	sb.WriteString("\n")

	return sb.String()
}

// renderFinding builds a styled string for one finding, translating the
// cmd/gitid/doctor.go renderFinding ANSI pattern to lipgloss styles.
// Mirrors the UI-SPEC Screen 1 finding layout: glyph+title, explanation (4-space
// indent), fix line (4-space indent, faint), [fix] badge.
func renderFinding(f doctor.Finding) string {
	glyph := "  ✗ "
	if f.Severity == doctor.SeverityInfo {
		glyph = "  ! "
	}
	severityStyle := SeverityStyle(f.Severity)
	titleLine := severityStyle.Render(glyph + f.Title)

	// Inline severity label (omit for error — ✗ implies it).
	switch f.Severity {
	case doctor.SeverityCritical:
		titleLine += " [critical]"
	case doctor.SeverityWarning:
		titleLine += " [warning]"
	case doctor.SeverityInfo:
		titleLine += " [info]"
	}

	var s string
	s += titleLine + "\n"
	if f.Explanation != "" {
		s += StyleBody.PaddingLeft(4).Render(f.Explanation) + "\n"
	}
	if f.SuggestedFix != "" {
		s += StyleFaint.PaddingLeft(4).Render("fix: "+f.SuggestedFix) + "\n"
	}
	if f.Fix != nil {
		s += "    [fix]\n"
	}
	return s
}
