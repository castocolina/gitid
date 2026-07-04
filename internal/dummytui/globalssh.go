package dummytui

// globalssh.go is the Go mirror of
// .planning/design/mockup-src/src/demo/screens/GlobalSsh.tsx per
// 02-REDESIGN-SPEC.md §4 — sub-tabs:
//
//	[Options]           GSSH-01 master-detail with per-row apply
//	                    checkboxes; advisory, never blocking; Apply
//	                    selected → ceremony.
//	[Storage & preview] STORE-01 dual strategy: sentinel block in
//	                    ~/.ssh/config vs gitid-owned
//	                    ~/.ssh/config.d/gitid.config via ONE Include line
//	                    near the top — with the resulting config rendered
//	                    per strategy; switching layouts walks the ceremony
//	                    (STORE-03: migration is a backed-up write).

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
)

// Global SSH sub-tabs.
type gssSubTab int

const (
	gssOptions gssSubTab = iota
	gssStorage
)

// Global SSH modes.
type gssMode int

const (
	gssBrowse gssMode = iota
	gssApplyCeremony
	gssStorageCeremony
)

// Sub-tab strip composition — subTabStrip renders exactly these labels
// (with a one-space lead and a one-space gap) and handleClick hit-tests
// against the same strings, so the spans can never drift.
const (
	gssTabOptionsLabel = " Options "
	gssTabStorageLabel = " Storage & preview "
)

// gssBannerBeyond is the findingsBanner tail used on the Options sub-tab —
// shared by renderOptions and gssOptionsTopLines.
const gssBannerBeyond = "these global options"

// optionRowLines is how many lines one master-list option row renders
// (optionRow here and the inline rows in globalgit.go) — shared with click
// hit-testing.
const optionRowLines = 2

// withToggled returns a copy of set with key flipped. Copy-on-write keeps
// the value-copied models pure: maps are reference types, so toggling in
// place would mutate every previously returned copy of the model.
func withToggled(set map[string]bool, key string) map[string]bool {
	next := make(map[string]bool, len(set)+1)
	for k, v := range set {
		next[k] = v
	}
	next[key] = !next[key]
	return next
}

// globalSSHModel is the Global SSH tab child model.
type globalSSHModel struct {
	subTab        gssSubTab
	mode          gssMode
	detailKey     string
	chosen        map[string]bool
	storageChoice SSHStorageLayout
	ceremony      ceremonyModel
}

// newGlobalSSHModel mirrors GlobalSsh.tsx's initial state: IdentitiesOnly
// detail, every needs-action option pre-chosen EXCEPT ForwardAgent (the
// fixture user's deliberate decline).
func newGlobalSSHModel() globalSSHModel {
	chosen := map[string]bool{}
	for _, o := range GlobalSSHOptions {
		if o.NeedsAction && o.Key != "ForwardAgent" {
			chosen[o.Key] = true
		}
	}
	return globalSSHModel{
		detailKey:     "IdentitiesOnly",
		chosen:        chosen,
		storageChoice: StorageSentinel,
	}
}

// activate syncs the storage radio with the live state.
func (m globalSSHModel) activate(s DemoState) (screenModel, tea.Cmd) {
	m.storageChoice = s.SSHStorage
	return m, nil
}

func (m globalSSHModel) handleMsg(tea.Msg, DemoState) keyResult { return keyResult{model: m} }

// appliedOption is one option after the applied-state overlay.
type appliedOption struct {
	GlobalSSHOption
	applied bool
}

// overlaidOptions maps the fixture options through the applied overlay:
// keys the user applied render as current=recommended, needsAction=false,
// one-liner prefixed "Applied by gitid — ".
func overlaidOptions(s DemoState) []appliedOption {
	out := make([]appliedOption, 0, len(GlobalSSHOptions))
	for _, o := range GlobalSSHOptions {
		entry := appliedOption{GlobalSSHOption: o}
		for _, k := range s.SSHApplied {
			if k == o.Key {
				entry.Current = o.Recommended
				entry.NeedsAction = false
				entry.OneLiner = "Applied by gitid — " + o.OneLiner
				entry.applied = true
			}
		}
		out = append(out, entry)
	}
	return out
}

// pendingOptions filters the overlaid options still needing action.
func pendingOptions(options []appliedOption) []appliedOption {
	var out []appliedOption
	for _, o := range options {
		if o.NeedsAction {
			out = append(out, o)
		}
	}
	return out
}

// applyChosen is the chosen ∩ pending key set, in fixture order.
func (m globalSSHModel) applyChosen(options []appliedOption) []string {
	var keys []string
	for _, o := range options {
		if o.NeedsAction && m.chosen[o.Key] {
			keys = append(keys, o.Key)
		}
	}
	return keys
}

// detailIndex resolves the selected option row index.
func (m globalSSHModel) detailIndex(options []appliedOption) int {
	for i, o := range options {
		if o.Key == m.detailKey {
			return i
		}
	}
	return 0
}

// managedHostStar renders the Host * managed block reflecting the applied
// option keys — extending the recipe's own Host * shape.
func managedHostStar(applied []string) string {
	begin, end := ManagedBlockSentinels("global-ssh")
	var lines []string
	for _, key := range applied {
		for _, o := range GlobalSSHOptions {
			if o.Key == key {
				lines = append(lines, "    "+o.Key+" "+o.Recommended)
			}
		}
	}
	body := ""
	if len(lines) > 0 {
		body = strings.Join(lines, "\n") + "\n"
	}
	return begin + "\nIgnoreUnknown UseKeychain\n\nHost *\n" + body + "    UseKeychain yes\n    AddKeysToAgent yes\n" + end
}

// The STORE-01 resulting-config previews (GlobalSsh.tsx mirror strings).
func sentinelPreview(s DemoState) string {
	return "# ~/.ssh/config — gitid blocks live in place, sentinel-delimited\n\nHost personal.github.com\n    Hostname ssh.github.com\n    Port 443\n    User git\n    IdentityFile ~/.ssh/id_ed25519_personal\n    IdentitiesOnly yes\n\n" + managedHostStar(s.SSHApplied)
}

const includePreviewMain = "# ~/.ssh/config (top of file)\nInclude ~/.ssh/config.d/gitid.config\n\n# …everything else in your config, untouched…"

func includePreviewOwned(s DemoState) string {
	return "# ~/.ssh/config.d/gitid.config (gitid-owned file)\nHost personal.github.com\n    Hostname ssh.github.com\n    Port 443\n    User git\n    IdentityFile ~/.ssh/id_ed25519_personal\n    IdentitiesOnly yes\n\n" + managedHostStar(s.SSHApplied)
}

// applyCeremonyFor builds the Apply-selected ceremony: `+` per chosen key,
// context for already-set options, and an explicit declined line per
// pending-but-unchecked option (advisory, never required).
func (m globalSSHModel) applyCeremonyFor(s DemoState) ceremonyModel {
	options := overlaidOptions(s)
	pending := pendingOptions(options)
	chosen := m.applyChosen(options)
	var lines []string
	for _, k := range chosen {
		for _, o := range GlobalSSHOptions {
			if o.Key == k {
				lines = append(lines, "+ "+o.Key+" "+o.Recommended)
			}
		}
	}
	for _, o := range options {
		if !o.NeedsAction {
			lines = append(lines, "  "+o.Key+" "+o.Recommended+" (already set)")
		}
	}
	for _, o := range pending {
		if !m.chosen[o.Key] {
			lines = append(lines, "  "+o.Key+" — left unchanged (declined; advisory)")
		}
	}
	target := "~/.ssh/config"
	if s.SSHStorage == StorageInclude {
		target = "~/.ssh/config.d/gitid.config"
	}
	rest := ""
	if len(pending)-len(chosen) > 0 {
		rest = " The rest were left unchanged, as chosen."
	}
	return newCeremony(ceremonyConfig{
		Heading:       "Write Host * managed block to ~/.ssh/config",
		Targets:       []string{target},
		Backups:       []string{NewBackupPath("~/.ssh/config")},
		Preview:       strings.Join(lines, "\n"),
		PreviewDiff:   true,
		ResultMessage: fmt.Sprintf("%d of %d recommended options applied to Host *.%s", len(chosen), len(pending), rest),
		ConfirmLabel:  "Apply selected",
	})
}

// storageCeremonyFor builds the STORE-03 migration ceremony for the
// selected layout.
func (m globalSSHModel) storageCeremonyFor() ceremonyModel {
	toInclude := m.storageChoice == StorageInclude
	headingTail := "sentinel blocks in ~/.ssh/config"
	diff := "+ gitid blocks written back, sentinel-delimited, into ~/.ssh/config\n- Include ~/.ssh/config.d/gitid.config (line removed)\n- ~/.ssh/config.d/gitid.config (file retired)\n  everything outside gitid blocks: untouched"
	result := "SSH storage layout migrated to in-place sentinel blocks — reversible via this same screen."
	if toInclude {
		headingTail = "Include’d gitid.config"
		diff = "+ Include ~/.ssh/config.d/gitid.config   (near the top of ~/.ssh/config)\n+ ~/.ssh/config.d/gitid.config (all gitid blocks move here)\n- # BEGIN/END gitid managed blocks removed from ~/.ssh/config\n  everything outside gitid blocks: untouched"
		result = "SSH storage layout migrated to the Include’d gitid-owned file — reversible via this same screen."
	}
	return newCeremony(ceremonyConfig{
		Heading:       "Migrate SSH storage layout → " + headingTail,
		Targets:       []string{"~/.ssh/config", "~/.ssh/config.d/gitid.config"},
		Backups:       []string{NewBackupPath("~/.ssh/config")},
		Preview:       diff,
		PreviewDiff:   true,
		ResultMessage: result,
		ConfirmLabel:  "Migrate",
	})
}

// handleKey implements the Global SSH key model.
func (m globalSSHModel) handleKey(msg tea.KeyMsg, s DemoState) keyResult {
	key := msg.String()

	if m.mode == gssApplyCeremony {
		var outcome ceremonyOutcome
		m.ceremony, outcome = m.ceremony.handleKey(msg)
		switch outcome {
		case ceremonyCancelled:
			m.mode = gssBrowse
		case ceremonyFinished:
			keys := m.applyChosen(overlaidOptions(s))
			m.mode = gssBrowse
			plural := "s"
			if len(keys) == 1 {
				plural = ""
			}
			return keyResult{model: m, handled: true,
				note:    fmt.Sprintf("%d global SSH option%s applied.", len(keys), plural),
				actions: []Action{ApplySSH{Keys: keys, Backup: NewBackupPath("~/.ssh/config")}}}
		case ceremonyNone, ceremonyConfirmed:
		}
		return keyResult{model: m, handled: true}
	}
	if m.mode == gssStorageCeremony {
		var outcome ceremonyOutcome
		m.ceremony, outcome = m.ceremony.handleKey(msg)
		switch outcome {
		case ceremonyCancelled:
			m.mode = gssBrowse
		case ceremonyFinished:
			m.mode = gssBrowse
			return keyResult{model: m, handled: true,
				note:    "SSH storage layout: " + string(m.storageChoice) + ".",
				actions: []Action{SetSSHStorage{Layout: m.storageChoice, Backup: NewBackupPath("~/.ssh/config")}}}
		case ceremonyNone, ceremonyConfirmed:
		}
		return keyResult{model: m, handled: true}
	}

	options := overlaidOptions(s)
	switch key {
	case "left", "right":
		if m.subTab == gssOptions {
			m.subTab = gssStorage
			m.storageChoice = s.SSHStorage
		} else {
			m.subTab = gssOptions
		}
		return keyResult{model: m, handled: true}
	case "up", "down":
		if m.subTab == gssOptions {
			idx := m.detailIndex(options)
			if key == "down" && idx < len(options)-1 {
				idx++
			}
			if key == "up" && idx > 0 {
				idx--
			}
			m.detailKey = options[idx].Key
		} else {
			if m.storageChoice == StorageSentinel {
				m.storageChoice = StorageInclude
			} else {
				m.storageChoice = StorageSentinel
			}
		}
		return keyResult{model: m, handled: true}
	case "space":
		if m.subTab == gssOptions {
			o := options[m.detailIndex(options)]
			if o.NeedsAction {
				m.chosen = withToggled(m.chosen, o.Key)
			}
		}
		return keyResult{model: m, handled: true}
	case "a":
		if m.subTab == gssOptions && len(m.applyChosen(options)) > 0 {
			m.ceremony = m.applyCeremonyFor(s)
			m.mode = gssApplyCeremony
		}
		return keyResult{model: m, handled: true}
	case "enter":
		if m.subTab == gssStorage && m.storageChoice != s.SSHStorage {
			m.ceremony = m.storageCeremonyFor()
			m.mode = gssStorageCeremony
			return keyResult{model: m, handled: true}
		}
		return keyResult{model: m}
	}
	return keyResult{model: m}
}

// subTabStrip renders the [Options] [Storage & preview] strip.
func (m globalSSHModel) subTabStrip() string {
	options := gssTabOptionsLabel
	storage := gssTabStorageLabel
	if m.subTab == gssOptions {
		options = styleReverse.Render(options)
	} else {
		storage = styleReverse.Render(storage)
	}
	return " " + options + " " + storage
}

// gssOptionsTopLines counts the body lines rendered above the first option
// row on the Options sub-tab (the sub-tab strip plus the optional findings
// banner) — shared by renderOptions and handleClick.
func gssOptionsTopLines(s DemoState) int {
	lines := 1 // sub-tab strip
	if findingsBanner(s, "SSH", gssBannerBeyond) != "" {
		lines++
	}
	return lines
}

// handleClick implements mouseTarget (browse mode only — ceremonies stay
// keyboard-driven): body line 0 is the sub-tab strip, where a click on
// either label switches sub-tabs; on the Options sub-tab a click on an
// option row (either of its two lines) selects it.
func (m globalSSHModel) handleClick(x, y, width int, s DemoState) keyResult {
	if m.mode != gssBrowse {
		return keyResult{model: m}
	}
	if y == 0 { // the sub-tab strip: " " + options label + " " + storage label
		optStart := 1
		optEnd := optStart + len(gssTabOptionsLabel)
		stoStart := optEnd + 1
		stoEnd := stoStart + len(gssTabStorageLabel)
		switch {
		case x >= optStart && x < optEnd:
			m.subTab = gssOptions
			return keyResult{model: m, handled: true}
		case x >= stoStart && x < stoEnd:
			m.subTab = gssStorage
			m.storageChoice = s.SSHStorage
			return keyResult{model: m, handled: true}
		}
		return keyResult{model: m}
	}
	if m.subTab != gssOptions || x >= masterListWidth(width) || y < gssOptionsTopLines(s) {
		return keyResult{model: m}
	}
	options := overlaidOptions(s)
	row := (y - gssOptionsTopLines(s)) / optionRowLines
	if row >= len(options) {
		return keyResult{model: m}
	}
	m.detailKey = options[row].Key
	return keyResult{model: m, handled: true}
}

// findingsBanner renders the "doctor found N findings beyond…" banner for
// a section, or "" when there are none.
func findingsBanner(s DemoState, section, beyond string) string {
	n := 0
	for _, f := range s.Findings {
		if f.Section == section {
			n++
		}
	}
	if n == 0 {
		return ""
	}
	plural := "s"
	if n == 1 {
		plural = ""
	}
	return " " + styleWarning.Render(fmt.Sprintf("! The doctor found %d %s finding%s beyond %s.", n, section, plural, beyond)) +
		"  " + styleFocusLink.Render("Open Doctor (4)")
}

// optionRow renders one master-list option row (2 lines).
func optionRow(key, current, recommended, risk string, needsAction, chosen, selected, applied bool, width int) string {
	marker := "  "
	if selected {
		marker = styleBold.Render("▸ ")
	}
	box := "   "
	if needsAction {
		box = "☐ "
		if chosen {
			box = "☑ "
		}
	} else if applied {
		box = "✓ "
	}
	tone := styleHealthy.Render("✓")
	if needsAction {
		tone = styleWarning.Render("!")
	}
	name := styleBold.Render(key)
	if selected {
		name = styleSelected.Render(key)
	}
	chip := ""
	if risk != "" {
		chip = "  " + styleFaint.Render("["+risk+"]")
	}
	line1 := " " + marker + box + tone + " " + name + chip
	line2 := "      " + styleFaint.Render("now: "+current+" → "+recommended)
	return truncLine(line1, width) + "\n" + truncLine(line2, width)
}

// truncLine hard-truncates a styled line to width cells.
func truncLine(line string, width int) string {
	return lipgloss.NewStyle().MaxWidth(width).Render(line)
}

// view implements screenModel.
func (m globalSSHModel) view(s DemoState, width, height int) screenView {
	options := overlaidOptions(s)
	pending := pendingOptions(options)
	chosen := m.applyChosen(options)

	status := "All recommendations applied or already set. Advisory, never a compliance gate."
	tone := "info"
	if len(pending) > 0 {
		status = fmt.Sprintf("%d of %d options need action — %s", len(pending), len(options), GlobalSSHAdvisoryNote)
		tone = "warning"
	}

	crumb := "Options"
	if m.subTab == gssStorage {
		crumb = "Storage & preview"
	}

	var body string
	var actions []FooterAction
	switch m.mode {
	case gssApplyCeremony, gssStorageCeremony:
		body = m.subTabStrip() + "\n" + m.ceremony.view(width-2)
		actions = []FooterAction{{Key: "Esc", Label: "cancel"}}
	case gssBrowse:
		if m.subTab == gssOptions {
			body = m.renderOptions(s, options, width, height)
			actions = []FooterAction{
				{Key: "↑↓", Label: "select option"},
				{Key: "←→", Label: "Options / Storage"},
				{Key: "space", Label: "choose"},
			}
			if len(chosen) > 0 {
				actions = append(actions, FooterAction{Key: "a", Label: fmt.Sprintf("apply %d selected", len(chosen))})
			}
		} else {
			body = m.renderStorage(s, width, height)
			actions = []FooterAction{
				{Key: "←→", Label: "Options / Storage"},
				{Key: "↑↓", Label: "choose layout"},
			}
			if m.storageChoice != s.SSHStorage {
				actions = append(actions, FooterAction{Key: "Enter", Label: "migrate layout…"})
			}
		}
	}
	return screenView{body: body, crumbs: []string{crumb}, status: status, statusTone: tone, actions: actions}
}

// renderOptions renders the Options master-detail.
func (m globalSSHModel) renderOptions(s DemoState, options []appliedOption, width, height int) string {
	listWidth := masterListWidth(width)
	detailWidth := width - listWidth - 1
	rows := frameBodyRows(height) - gssOptionsTopLines(s)

	var listRows []string
	selIdx := m.detailIndex(options)
	for i, o := range options {
		listRows = append(listRows, optionRow(o.Key, o.Current, o.Recommended, o.Risk, o.NeedsAction, m.chosen[o.Key], i == selIdx, o.applied, listWidth))
	}
	list := strings.Join(listRows, "\n")

	detail := options[selIdx]
	explanation := detail.OneLiner
	if detail.Key == "IdentitiesOnly" {
		explanation = GlobalSSHDetailExplanation
	}
	var d strings.Builder
	d.WriteString(" " + styleBold.Render(detail.Key) + "\n")
	d.WriteString(" " + styleInfo.Render("~ "+GlobalSSHAdvisoryNote) + "\n\n")
	d.WriteString(" " + explanation + "\n")
	// Wrap to the pane width, then clip with a VISIBLE cue — long option
	// explanations must never be silently cut mid-sentence (H3).
	detailPane := fitPane(lipgloss.NewStyle().Width(detailWidth).Render(d.String()), rows)

	banner := findingsBanner(s, "SSH", gssBannerBeyond)
	body := m.subTabStrip() + "\n"
	if banner != "" {
		body += banner + "\n"
	}
	return body + joinMasterDetail(list, listWidth, detailPane, rows)
}

// renderStorage renders the STORE-01 Storage & preview sub-tab.
func (m globalSSHModel) renderStorage(s DemoState, width, height int) string {
	leftWidth := masterListWidth(width)
	rightWidth := width - leftWidth - 1
	rows := frameBodyRows(height) - 1 // the sub-tab strip line

	current := func(layout SSHStorageLayout) string {
		if s.SSHStorage == layout {
			return " — current"
		}
		return ""
	}
	radio := func(layout SSHStorageLayout) string {
		if m.storageChoice == layout {
			return "● "
		}
		return "○ "
	}
	var l strings.Builder
	l.WriteString(" " + styleFaint.Render("STORE-01 — where gitid-managed SSH config lives") + "\n")
	l.WriteString(" " + radio(StorageSentinel) + "Sentinel blocks in ~/.ssh/config (default)" + current(StorageSentinel) + "\n")
	l.WriteString(" " + radio(StorageInclude) + "gitid-owned ~/.ssh/config.d/gitid.config via one Include line" + current(StorageInclude) + "\n\n")
	l.WriteString(" " + styleFaint.Render("Include paths must be absolute or ~/.ssh-relative; the Include line goes NEAR THE TOP of ~/.ssh/config. Migration between layouts is backed-up and reversible (STORE-03).") + "\n")
	if m.storageChoice != s.SSHStorage {
		l.WriteString("\n " + styleSelected.Render(" Migrate layout… (Enter) ") + "\n")
	}
	left := l.String()

	var r strings.Builder
	if m.storageChoice == StorageSentinel {
		r.WriteString(" " + PreviewLabel("Resulting config — sentinel blocks in place") + "\n")
		r.WriteString(previewBlockClipped(sentinelPreview(s), false, rightWidth, 18) + "\n")
	} else {
		r.WriteString(" " + PreviewLabel("Resulting config — Include + owned file") + "\n")
		r.WriteString(previewBlockClipped(includePreviewMain, false, rightWidth, 4) + "\n")
		r.WriteString(previewBlockClipped(includePreviewOwned(s), false, rightWidth, 10) + "\n")
	}
	right := lipgloss.NewStyle().Width(rightWidth).Render(r.String())

	return m.subTabStrip() + "\n" + joinMasterDetail(left, leftWidth, right, rows)
}
