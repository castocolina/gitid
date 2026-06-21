package tui

import (
	"os"
	"os/exec"
	"runtime"
	"strings"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"

	"github.com/castocolina/gitid/internal/doctor"
	"github.com/castocolina/gitid/internal/gitconfig"
	"github.com/castocolina/gitid/internal/identity"
)

// unmanagedEntry represents a hand-written SSH Host block or orphan key that
// is not tracked by any gitid-managed identity. These are surfaced in the
// read-only "Unmanaged" sidebar section (D-12, D-13).
//
// Available affordances (read-only, D-13): reveal-path, copy-pubkey, open-location.
// NO edit/health/management actions are exposed for unmanaged entries.
// "Adopt into gitid" is explicitly deferred (CONTEXT Deferred Ideas).
type unmanagedEntry struct {
	// shortName is the Host alias or key filename, truncated to 12 chars.
	shortName string
	// keyPath is the full path to the private key file (for revealPath and openLocation).
	keyPath string
	// pubLine is the SSH public key line from the .pub file (for copyUnmanagedPubkey).
	// This is the ONLY value passed to clipboard.Copy — the private key is never copied (D-13).
	pubLine string
}

// sidebarModel is the left-pane sub-model for the persistent two-pane layout.
// It renders:
//   - "Identities" section: managed identity rows (name · provider · (N) + badge)
//   - "Unmanaged" section: orphan SSH keys and hand-written Host blocks (read-only)
//
// The sidebar is fixed at 18 columns and does NOT use bubbles/v2 list.Model
// because 18 cols is too narrow for the default list delegate (PATTERNS § tui/sidebar.go).
type sidebarModel struct {
	accounts  []identity.Account
	unmanaged []unmanagedEntry
	selected  int // index into accounts; -1 = none selected
	// selectedUnmanaged is the index into unmanaged; -1 means no unmanaged entry is
	// selected. When selectedUnmanaged >= 0, the managed selected is irrelevant for
	// affordance dispatch (D-13).
	selectedUnmanaged int

	badges map[string]doctor.Severity // keyed by identity name; Plan 03 fills this

	width, height int
}

// newSidebarModel constructs an empty sidebar. The caller seeds it via
// refresh() which emits a refreshSidebarMsg that replaces the zero-value slices.
func newSidebarModel(_ doctor.Deps) sidebarModel {
	return sidebarModel{
		selected:          -1,
		selectedUnmanaged: -1,
		badges:            make(map[string]doctor.Severity),
	}
}

// selectedAccount returns a pointer to the currently selected Account or nil
// when nothing is selected (empty list or selected == -1).
func (m sidebarModel) selectedAccount() *identity.Account {
	if m.selected < 0 || m.selected >= len(m.accounts) {
		return nil
	}
	return &m.accounts[m.selected]
}

// selectedUnmanagedEntry returns a pointer to the currently selected unmanaged
// entry, or nil when no unmanaged entry is selected.
func (m sidebarModel) selectedUnmanagedEntry() *unmanagedEntry {
	if m.selectedUnmanaged < 0 || m.selectedUnmanaged >= len(m.unmanaged) {
		return nil
	}
	return &m.unmanaged[m.selectedUnmanaged]
}

// unmanagedAffordances returns the list of affordances for the currently
// selected unmanaged entry. D-13: reveal-path, copy-pubkey, open-location ONLY.
// No edit/health/management actions.
func unmanagedAffordances() []string {
	return []string{"revealPath", "copyUnmanagedPubkey", "openLocation"}
}

// openLocationCmd returns a tea.Cmd that reveals the directory of the given
// file path in the OS file manager, dispatched via os/exec WITHOUT a shell
// (no sh -c, no interpolation — T-05.6-23). Best-effort: errors are silently
// dropped so the TUI does not crash on a failed open.
//
// Security invariant (T-05.6-23): the directory path is computed via
// filepath.Dir and passed directly as an arg-slice — never through sh -c or
// any shell interpolation.
func openLocationCmd(keyPath string) tea.Cmd {
	return func() tea.Msg {
		dir := ""
		if keyPath != "" {
			// Use os/exec without shell — arg-slice form (no sh -c, G204-clean).
			dir = fileDirOf(keyPath)
		}
		if dir == "" {
			return setToastMsg{text: "no path to reveal", style: StyleFaint}
		}
		openPathNoShell(dir) // best-effort; errors dropped
		return setToastMsg{text: "opened: " + dir, style: StyleFaint}
	}
}

// fileDirOf returns the directory component of path using strings splitting.
// Falls back to an empty string when path is empty.
func fileDirOf(path string) string {
	if path == "" {
		return ""
	}
	idx := strings.LastIndexByte(path, '/')
	if idx <= 0 {
		return "."
	}
	return path[:idx]
}

// openPathNoShell reveals dir in the OS file manager using the platform's
// canonical open command. Dispatched via arg-slice exec (no sh -c, no
// interpolation — T-05.6-23). Best-effort: the error is discarded so the
// TUI does not crash on an unavailable tool.
//
// Platform dispatch (T-05.6-23, G204-clean):
//   - macOS: `open <dir>`
//   - Linux/other: `xdg-open <dir>`
func openPathNoShell(dir string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", dir) //nolint:gosec // arg-slice; dir is filepath.Dir of trusted key path (T-05.6-23)
	default:
		cmd = exec.Command("xdg-open", dir) //nolint:gosec // arg-slice; dir is filepath.Dir of trusted key path (T-05.6-23)
	}
	_ = cmd.Run() // best-effort; error intentionally discarded
}

// refresh returns a tea.Cmd that re-reads ~/.ssh/config and ~/.gitconfig via
// the ReadFile seam, calls identity.Reconstruct, and emits refreshSidebarMsg
// with the updated accounts and unmanaged entries.
//
// This is the live data path that satisfies D-16 (anti-blindspot): Init() calls
// refresh() so the real gitid entry shows actual identities on launch.
func (m sidebarModel) refresh(deps tuiDeps) tea.Cmd {
	readFile := deps.doctor.ReadFile
	return func() tea.Msg {
		var sshBytes, gcBytes []byte
		if readFile != nil {
			home, err := os.UserHomeDir()
			if err == nil {
				sshBytes, _ = readFile(home + "/.ssh/config")
				gcBytes, _ = readFile(home + "/.gitconfig")
			}
		}

		accounts, _ := identity.Reconstruct(sshBytes, gcBytes, gitconfig.ReadFragment)

		// Fall back to deps.doctor.Identities ONLY in pure test mode (no live read
		// seam). In live mode (readFile != nil) an empty Reconstruct result is
		// authoritative — otherwise deleting every identity would resurrect the
		// stale launch-time snapshot as ✗ "zombie" rows (D-2 regression).
		if len(accounts) == 0 && readFile == nil {
			accounts = deps.doctor.Identities
		}

		// Build unmanaged entries from orphan findings in the last health run.
		// Since Plan 03 drives the health run, we have no orphan data yet.
		// Accept an empty slice now; Plan 03 will fill m.unmanaged via a
		// separate message after the health check completes.
		unmanaged := buildUnmanaged(deps)

		return refreshSidebarMsg{accounts: accounts, unmanaged: unmanaged}
	}
}

// buildUnmanaged extracts orphan/unlinked SSH key entries from the doctor Deps.
// The doctor.Deps fields SSHManagedBlockNames and GitconfigManagedBlockNames are
// populated at wiring time (buildTUIDoctorDeps) with the block names from the
// parsed configs. Cross-referencing the two sets identifies class-1 and class-2
// orphans (per CheckOrphans logic). Class-3 (unused key files) are identified via
// KeyPaths vs AllSSHHostIdentityFiles.
//
// RESEARCH Open Q #3 resolution: the "unlinked SSH key" finding (class 3) uses
// the doctor.Deps.KeyPaths field and the AllSSHHostIdentityFiles set. We surface
// class-3 orphans here; class-1/class-2 are managed blocks, not "hand-written",
// so they are omitted from the unmanaged display.
func buildUnmanaged(deps tuiDeps) []unmanagedEntry {
	d := deps.doctor
	if len(d.AllSSHHostIdentityFiles) == 0 {
		return nil
	}

	// Build a set of referenced key paths.
	referenced := make(map[string]bool, len(d.AllSSHHostIdentityFiles))
	for _, p := range d.AllSSHHostIdentityFiles {
		referenced[p] = true
	}

	var result []unmanagedEntry
	for _, kp := range d.KeyPaths {
		if !referenced[kp] {
			// Orphan key: not claimed by any Host block.
			result = append(result, unmanagedEntry{
				shortName: shortName(kp, 12),
			})
		}
	}
	return result
}

// shortName returns the last segment of path, truncated to maxLen chars.
func shortName(path string, maxLen int) string {
	// Use last slash-separated component.
	parts := strings.Split(path, "/")
	name := parts[len(parts)-1]
	if len([]rune(name)) > maxLen {
		runes := []rune(name)
		return string(runes[:maxLen])
	}
	return name
}

// updateKey handles up/down navigation within the sidebar.
func (m sidebarModel) updateKey(key string) (sidebarModel, tea.Cmd) {
	if len(m.accounts) == 0 {
		return m, nil
	}
	switch key {
	case "up", "k":
		if m.selected > 0 {
			m.selected--
		} else {
			m.selected = 0
		}
	case "down", "j":
		if m.selected < len(m.accounts)-1 {
			m.selected++
		}
		// Auto-select first on first down press when nothing is selected.
		if m.selected < 0 {
			m.selected = 0
		}
	}
	return m, nil
}

// view renders the sidebar at the given dimensions.
// focused controls whether the sidebar border is in the accent color.
func (m sidebarModel) view(width, height int, focused bool) string {
	m.width = width
	m.height = height

	var sb strings.Builder

	// "Identities" section header.
	sb.WriteString(StyleSidebarSection.Render("Identities"))
	sb.WriteString("\n")

	if len(m.accounts) == 0 {
		sb.WriteString(StyleFaint.Render("  No identities."))
		sb.WriteString("\n")
	} else {
		// Auto-select first account if nothing is selected.
		if m.selected < 0 && len(m.accounts) > 0 {
			m.selected = 0
		}
		for i, acct := range m.accounts {
			sb.WriteString(m.renderAccountRow(i, acct, width))
			sb.WriteString("\n")
		}
	}

	// Unmanaged section — only rendered when non-empty.
	if len(m.unmanaged) > 0 {
		sb.WriteString("\n")
		sb.WriteString(StyleSidebarSection.Faint(true).Render("Unmanaged"))
		sb.WriteString("\n")
		for _, u := range m.unmanaged {
			row := StyleSidebarUnmanaged.Render("○ " + u.shortName + " ~")
			sb.WriteString(row)
			sb.WriteString("\n")
		}
	}

	content := sb.String()
	// Trim trailing newline so lipgloss height calculation is accurate.
	content = strings.TrimRight(content, "\n")

	// Pad to height so the sidebar fills the content area.
	lines := strings.Split(content, "\n")
	for len(lines) < height {
		lines = append(lines, "")
	}
	// Clamp to height.
	if len(lines) > height {
		lines = lines[:height]
	}
	result := strings.Join(lines, "\n")

	// Apply a focused border highlight (subtle: bold the leading column).
	_ = focused // reserved for Plan 03+ where focus state changes the border color
	return result
}

// renderAccountRow renders a single managed identity row.
// Format: "› name  provider  (N)" with badge at rightmost cell.
// Selected row uses StyleSelected; others use StyleSidebarItem.
func (m sidebarModel) renderAccountRow(idx int, acct identity.Account, width int) string {
	// Badge glyph for this identity (Plan 03 fills badges; default = pass ✓).
	sev, hasBadge := m.badges[acct.Name]
	badgeGlyph := "✓"
	if hasBadge {
		badgeGlyph = SeverityGlyph(sev, asciiMode())
	}
	// Badge occupies 1 visible cell at the right edge.
	badgeW := 1

	// Count alias/sites for the (N) slot.
	siteCount := len(acct.Matches)
	if siteCount == 0 && acct.Alias != "" {
		siteCount = 1
	}
	countStr := ""
	if siteCount > 0 {
		countStr = "(" + itoa(siteCount) + ")"
	}

	// Provider shortname: use first token before "."
	provider := acct.Provider
	if idx2 := strings.IndexByte(provider, '.'); idx2 > 0 {
		provider = provider[:idx2]
	}

	// Name column: truncate at width - 10 (leave room for provider + count + badge).
	nameMaxW := width - 10
	if nameMaxW < 1 {
		nameMaxW = 1
	}
	name := truncateRunes(acct.Name, nameMaxW)

	// Selected-row indicator.
	prefix := "  "
	if idx == m.selected {
		prefix = "› "
	}

	row := prefix + name + "  " + provider + "  " + countStr

	// Right-pad to width - badgeW then append badge.
	rowW := lipgloss.Width(row)
	padNeeded := width - badgeW - rowW
	if padNeeded < 0 {
		padNeeded = 0
	}
	row = row + strings.Repeat(" ", padNeeded) + badgeGlyph

	if idx == m.selected {
		return StyleSelected.Render(row)
	}
	return StyleSidebarItem.Render(row)
}

// truncateRunes truncates s to at most n runes, appending "…" if truncated.
func truncateRunes(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	if n <= 0 {
		return ""
	}
	return string(runes[:n-1]) + "…"
}

// itoa converts a non-negative integer to its decimal string representation
// without importing "strconv" (avoids an extra import for a trivial helper).
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := make([]byte, 0, 10)
	for n > 0 {
		buf = append([]byte{byte('0' + n%10)}, buf...)
		n /= 10
	}
	return string(buf)
}
