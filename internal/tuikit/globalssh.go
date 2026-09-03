package tuikit

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
	"github.com/charmbracelet/x/ansi"
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
	// backend is the injected options/commit seam. The model fetches the
	// option states on activation and never renders fixture data for a row
	// the backend could have answered.
	backend Backend
	subTab  gssSubTab
	mode    gssMode
	// detailKey is the selected option row's key.
	detailKey string
	chosen    map[string]bool
	// storageChoice is the STORE-01 radio selection (Storage & preview sub-tab).
	storageChoice SSHStorageLayout
	ceremony      ceremonyModel
	// options is the live Options-sub-tab row set fetched from the backend on
	// activation; optionsErr carries the fetch failure the pane renders
	// instead of a blank body (GSSH-01 advisory posture).
	options    []GlobalSSHOptionView
	optionsErr string
	// applyCommitPending gates the apply ceremony's receipt: ApplySSH is
	// dispatched only from handleMsg once GlobalSSHCommitMsg arrives with an
	// empty Err, never optimistically on ceremonyFinished (mirrors
	// identities.go's gitCommitPending). appliedKeys is the EXACT key set
	// captured at ceremonyConfirmed, so the receipt's action describes the
	// write that actually happened.
	applyCommitPending bool
	appliedKeys        []string
	// storageView is the live Storage-sub-tab migration preview fetched from
	// the backend on activation and after a successful migration. It carries
	// the PlanToken the ceremony passes back on confirmation — the token is
	// the only way one previewed plan reaches the commit without a backend
	// type crossing the boundary. storageViewErr carries the fetch failure
	// when the pane cannot describe the machine.
	storageView    SSHStorageMigrationView
	storageViewErr string
	// storageCommitPending gates the storage ceremony's receipt: SetSSHStorage
	// is dispatched only from handleMsg once SSHStorageCommitMsg arrives with
	// an empty Err, never optimistically on ceremonyFinished (mirrors
	// applyCommitPending above). storageTargetLayout is the layout the user
	// confirmed, captured at ceremonyConfirmed.
	storageCommitPending bool
	storageTargetLayout  SSHStorageLayout
}

// newGlobalSSHModel returns a model with an EMPTY selection set (D-15): the
// selection starts empty on every entry to the screen, and the toggle key is
// the only way to add to it. The pre-chosen fixture set was the demo's
// scripted state, not the real default.
func newGlobalSSHModel(b Backend) globalSSHModel {
	return globalSSHModel{
		backend:       b,
		chosen:        map[string]bool{},
		storageChoice: StorageSentinel,
	}
}

// activate syncs the storage radio with the live state, fetches the Options
// sub-tab rows and the Storage sub-tab migration preview from the backend.
// The selection is reset to empty on every entry so returning to the screen
// never resurrects a stale selection (D-15). A non-nil fetch error stores an
// empty slice plus an error note the pane renders instead of a blank body.
// Clearing storageView (and its token) here means a plan held from a
// previous entry to this screen cannot be committed against a new one.
func (m globalSSHModel) activate(s DemoState) (screenModel, tea.Cmd) {
	m.storageChoice = s.SSHStorage
	m.chosen = map[string]bool{}
	options, err := m.backend.GlobalSSHOptionStates()
	m.options = options
	if err != nil {
		m.options = nil
		m.optionsErr = err.Error()
	}
	// D-01 / UXP-01: focus the first fetched row on every activation.
	// A construction-time key would not reset on re-entry, and a hardcoded
	// first-row literal would re-break if the policy table is reordered.
	if len(m.options) > 0 {
		m.detailKey = m.options[0].Key
	}
	m = m.refetchStoragePlan()
	return m, nil
}

// refetchStoragePlan re-fetches the Storage sub-tab's migration preview for
// m.storageChoice from the backend, clearing any previously held plan/error
// first so a fetch failure never leaves a stale plan (and its now-invalid
// PlanToken) from a DIFFERENT layout selection visible.
//
// Shared by activate, the keyboard up/down handler, and the mouse radio-click
// handler (CR-05): before this helper existed, a mouse click on a radio row
// set m.storageChoice WITHOUT refetching, so m.storageViewErr kept holding
// whatever error activate() had stored (typically the CR-05 "already this
// layout" refusal), the Migrate button never rendered (renderStorage
// requires m.storageViewErr == ""), and the mouse-only path could never
// reach the migration ceremony.
func (m globalSSHModel) refetchStoragePlan() globalSSHModel {
	m.storageView = SSHStorageMigrationView{}
	m.storageViewErr = ""
	view, verr := m.backend.SSHStorageMigrationPlan(m.storageChoice)
	if verr != nil {
		m.storageViewErr = verr.Error()
	} else {
		m.storageView = view
	}
	return m
}

// handleMsg completes the asynchronous apply and storage-migration commits
// once the backend's commands have answered. The receipt is reachable ONLY
// from an explicit success; reducer actions are dispatched here, never
// optimistically.
func (m globalSSHModel) handleMsg(msg tea.Msg, _ DemoState) keyResult {
	if commit, ok := msg.(GlobalSSHCommitMsg); ok && m.mode == gssApplyCeremony && m.applyCommitPending {
		m.applyCommitPending = false
		if commit.Err != "" {
			message := commit.Err
			if len(commit.Restored) > 0 {
				message += " (restored: " + strings.Join(commit.Restored, "; ") + ")"
			}
			m.ceremony = m.ceremony.commitFailed(message)
			return keyResult{model: m}
		}
		plural := "s"
		if len(m.appliedKeys) == 1 {
			plural = ""
		}
		m.ceremony = m.ceremony.commitSucceeded(commit.Backups)
		// Append post-write shadow advisories to the receipt (D-04).
		// The ResultExtra field is the right slot: it is rendered directly
		// below ResultMessage on the receipt without requiring a new ceremony
		// field (06-UI-SPEC.md budgets this against the existing field).
		if len(commit.ShadowAdvisories) > 0 {
			m.ceremony = m.ceremony.withResultExtra(strings.Join(commit.ShadowAdvisories, "\n"))
		}
		return keyResult{
			model:   m,
			note:    fmt.Sprintf("%d global SSH option%s applied.", len(m.appliedKeys), plural),
			actions: []Action{ApplySSH{Keys: m.appliedKeys, Backup: firstBackup(commit.Backups)}},
		}
	}
	if commit, ok := msg.(SSHStorageCommitMsg); ok && m.mode == gssStorageCeremony && m.storageCommitPending {
		m.storageCommitPending = false
		if commit.Err != "" {
			message := commit.Err
			if commit.ConfigChangedSincePreview {
				message = "Configuration changed since the preview was opened — re-open the preview to migrate."
			} else if len(commit.Restored) > 0 {
				message += " (restored: " + strings.Join(commit.Restored, "; ") + ")"
			}
			m.ceremony = m.ceremony.commitFailed(message)
			return keyResult{model: m}
		}
		layout := m.storageTargetLayout
		m.ceremony = m.ceremony.commitSucceeded(commit.Backups)
		// WR-18: refetch for the CONFIRMED target layout, not s.SSHStorage —
		// s is the state captured BEFORE the SetSSHStorage reducer below runs,
		// so s.SSHStorage is still the OLD (pre-migration) layout. On disk the
		// layout is now `layout` (the new one), so planning for s.SSHStorage
		// silently computed the plan to migrate BACK — and stored it as the
		// live pending plan (SSHStorageMigrationPlan's own side effect), a
		// reverse migration nobody asked for. The stated "current-layout
		// marker" purpose was never actually served by this call either:
		// renderStorage's marker reads s.SSHStorage fresh from the render
		// argument, which the SetSSHStorage action below already keeps
		// correct. What genuinely goes stale is m.storageChoice and
		// m.storageView — both are refreshed here for the layout the
		// migration ACTUALLY produced. A fetch error is advisory — the write
		// already succeeded.
		m.storageChoice = layout
		view, verr := m.backend.SSHStorageMigrationPlan(layout)
		if verr == nil {
			m.storageView = view
			m.storageViewErr = ""
		}
		return keyResult{
			model:   m,
			note:    "SSH storage layout migrated to " + string(layout) + ".",
			actions: []Action{SetSSHStorage{Layout: layout, Backup: firstBackup(commit.Backups)}},
		}
	}
	return keyResult{model: m}
}

// appliedOption is one option after the applied-state overlay.
type appliedOption struct {
	GlobalSSHOptionView
	applied bool
}

func (o appliedOption) needsAction() bool {
	if o.applied {
		return false
	}
	return o.Selectable()
}

// needsAttention is the pinned tally rule (06-CONTEXT.md discretion):
// needs-action and set-but-differs count together; not-applicable does not.
// Plan 06-04's ceremony N-of-M must read this same predicate.
func (o appliedOption) needsAttention() bool {
	if o.applied {
		return false
	}
	return o.State == GlobalSSHNeedsAction || o.State == GlobalSSHDiffers
}

// overlaidOptions maps the model's LIVE backend views through the applied
// overlay: keys the user applied render as current=recommended, applied=true,
// one-liner prefixed "Applied by gitid — ".
func (m globalSSHModel) overlaidOptions(s DemoState) []appliedOption {
	out := make([]appliedOption, 0, len(m.options))
	for _, o := range m.options {
		entry := appliedOption{GlobalSSHOptionView: o}
		for _, k := range s.SSHApplied {
			if k == o.Key {
				entry.CurrentValue = o.Recommended
				entry.State = GlobalSSHAlreadySet
				entry.OneLiner = "Applied by gitid — " + o.OneLiner
				entry.Explanation = entry.OneLiner
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
		if o.needsAction() {
			out = append(out, o)
		}
	}
	return out
}

// applyChosen is the chosen ∩ pending key set, in row order.
func (m globalSSHModel) applyChosen(options []appliedOption) []string {
	var keys []string
	for _, o := range options {
		if o.needsAction() && m.chosen[o.Key] {
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

// SentinelPreview is the STORE-01 resulting-config preview for the in-place
// sentinel layout (GlobalSsh.tsx mirror string). The dummy backend returns
// this verbatim; the real backend returns PlanMigration's bytes instead.
func SentinelPreview(s DemoState) string {
	return "# ~/.ssh/config — gitid blocks live in place, sentinel-delimited\n\nHost personal.github.com\n    Hostname ssh.github.com\n    Port 443\n    User git\n    IdentityFile ~/.ssh/id_ed25519_personal\n    IdentitiesOnly yes\n\n" + managedHostStar(s.SSHApplied)
}

// IncludePreviewMain is the STORE-01 preview of ~/.ssh/config after an
// Include-layout migration (the floored Include line; gitid blocks have moved).
const IncludePreviewMain = "# ~/.ssh/config (top of file)\nInclude ~/.ssh/config.d/gitid.config\n\n# …everything else in your config, untouched…"

// IncludePreviewOwned is the STORE-01 preview of the gitid-owned Include'd
// file (GlobalSsh.tsx mirror string). The dummy backend returns this
// verbatim; the real backend returns PlanMigration's bytes instead.
func IncludePreviewOwned(s DemoState) string {
	return "# ~/.ssh/config.d/gitid.config (gitid-owned file)\nHost personal.github.com\n    Hostname ssh.github.com\n    Port 443\n    User git\n    IdentityFile ~/.ssh/id_ed25519_personal\n    IdentitiesOnly yes\n\n" + managedHostStar(s.SSHApplied)
}

// applyCeremonyFor builds the Apply-selected ceremony: `+` per chosen key,
// context for already-set options, an explicit declined line per
// pending-but-unchecked option (advisory, never required), and — from the
// backend's plan — the resolved TARGET FILE the write will actually touch and
// its diff. The ceremony is ASYNC: confirmation dispatches the backend commit
// and the receipt is reachable only from that commit's explicit success
// (ceremony.go's Async contract), exactly like the standalone Git ceremony.
func (m globalSSHModel) applyCeremonyFor(s DemoState) (ceremonyModel, error) {
	options := m.overlaidOptions(s)
	pending := pendingOptions(options)
	chosen := m.applyChosen(options)
	var lines []string
	for _, k := range chosen {
		for _, o := range options {
			if o.Key == k {
				lines = append(lines, "+ "+o.Key+" "+o.Recommended)
			}
		}
	}
	for _, o := range options {
		if o.State == GlobalSSHAlreadySet {
			lines = append(lines, "  "+o.Key+" "+o.Recommended+" (already set)")
		}
	}
	for _, o := range pending {
		if !m.chosen[o.Key] {
			lines = append(lines, "  "+o.Key+" — left unchanged (declined; advisory)")
		}
	}

	plan, planErr := m.backend.GlobalSSHApplyPlan(chosen)
	if planErr != nil {
		return ceremonyModel{}, planErr
	}
	targets := plan.Targets
	if len(targets) == 0 {
		// The backend's plan always resolves the storage target (D-07); this
		// fallback keeps the ceremony buildable for a stub that answers an
		// empty plan.
		fallback := "~/.ssh/config"
		if s.SSHStorage == StorageInclude {
			fallback = "~/.ssh/config.d/gitid.config"
		}
		targets = []string{fallback}
	}
	backups := plan.Backups
	if len(backups) == 0 {
		backups = []string{NewBackupPath(targets[0])}
	}
	preview := plan.Diff
	if preview == "" {
		preview = strings.Join(lines, "\n")
	}
	// Append shadow warnings or the inconclusive note (D-04).
	// With zero warnings and a conclusive simulation, nothing extra renders.
	// With an inconclusive simulation, the note renders and no shadow warnings
	// are shown (an unproven claim is never printed as a fact).
	if plan.SimulationInconclusive {
		if plan.SimulationNote != "" {
			preview += "\n" + plan.SimulationNote
		}
	} else {
		for _, w := range plan.ShadowWarnings {
			preview += "\n" + w
		}
	}
	rest := ""
	if len(pending)-len(chosen) > 0 {
		rest = " The rest were left unchanged, as chosen."
	}
	return newCeremony(ceremonyConfig{
		Heading:       "Write Host * managed block to " + targets[0],
		Targets:       targets,
		Backups:       backups,
		Preview:       preview,
		PreviewDiff:   true,
		ResultMessage: fmt.Sprintf("%d of %d recommended options applied to Host *.%s", len(chosen), len(pending), rest),
		ConfirmLabel:  "Apply selected",
		Async:         true,
	}), nil
}

// storageCeremonyFor builds the STORE-03 migration ceremony for the selected
// layout using the plan view the backend already computed (which carries the
// PlanToken the confirmation will send back). The ceremony is ASYNC:
// confirmation dispatches the backend commit and the receipt is reachable
// only from that commit's explicit success (ceremony.go's Async contract),
// exactly like the apply ceremony.
func (m globalSSHModel) storageCeremonyFor(view SSHStorageMigrationView) ceremonyModel {
	toInclude := m.storageChoice == StorageInclude
	result := "SSH storage layout migrated to in-place sentinel blocks — reversible via this same screen."
	if toInclude {
		result = "SSH storage layout migrated to the Include’d gitid-owned file — reversible via this same screen."
	}
	heading := view.Heading
	if heading == "" {
		headingTail := "sentinel blocks in ~/.ssh/config"
		if toInclude {
			headingTail = "Include’d gitid.config"
		}
		heading = "Migrate SSH storage layout → " + headingTail
	}
	targets := view.Targets
	if len(targets) == 0 {
		targets = []string{"~/.ssh/config", "~/.ssh/config.d/gitid.config"}
	}
	backups := view.Backups
	if len(backups) == 0 {
		backups = []string{NewBackupPath("~/.ssh/config")}
	}
	diff := view.Diff
	if diff == "" {
		if toInclude {
			diff = "+ Include ~/.ssh/config.d/gitid.config   (near the top of ~/.ssh/config)\n+ ~/.ssh/config.d/gitid.config (all gitid blocks move here)\n- # BEGIN/END gitid managed blocks removed from ~/.ssh/config\n  everything outside gitid blocks: untouched"
		} else {
			diff = "+ gitid blocks written back, sentinel-delimited, into ~/.ssh/config\n- Include ~/.ssh/config.d/gitid.config (line removed)\n- ~/.ssh/config.d/gitid.config (file retired)\n  everything outside gitid blocks: untouched"
		}
	}
	return newCeremony(ceremonyConfig{
		Heading:       heading,
		Targets:       targets,
		Backups:       backups,
		Preview:       diff,
		PreviewDiff:   true,
		ResultMessage: result,
		ConfirmLabel:  "Migrate",
		Async:         true,
	})
}

// handleKey implements the Global SSH key model.
func (m globalSSHModel) handleKey(msg tea.KeyMsg, s DemoState) keyResult {
	key := msg.String()

	if m.mode == gssApplyCeremony {
		if m.applyCommitPending {
			// In flight: keys are inert until the commit result arrives.
			return keyResult{model: m, handled: true}
		}
		var outcome ceremonyOutcome
		m.ceremony, outcome = m.ceremony.handleKey(msg)
		switch outcome {
		case ceremonyCancelled:
			m.mode = gssBrowse
		case ceremonyConfirmed:
			// Dispatch the async commit; the receipt is reached only from the
			// commit's explicit success (handleMsg), never optimistically.
			keys := m.applyChosen(m.overlaidOptions(s))
			m.appliedKeys = keys
			m.applyCommitPending = true
			return keyResult{model: m, handled: true, cmd: m.backend.CommitGlobalSSH(keys)}
		case ceremonyFinished:
			m.mode = gssBrowse
		case ceremonyNone:
		}
		return keyResult{model: m, handled: true}
	}
	if m.mode == gssStorageCeremony {
		if m.storageCommitPending {
			// In flight: keys are inert until the commit result arrives.
			return keyResult{model: m, handled: true}
		}
		var outcome ceremonyOutcome
		m.ceremony, outcome = m.ceremony.handleKey(msg)
		switch outcome {
		case ceremonyCancelled:
			m.mode = gssBrowse
		case ceremonyConfirmed:
			// Dispatch the async commit; the receipt is reached only from the
			// commit's explicit success (handleMsg), never optimistically.
			// Pass the token from the view the ceremony was OPENED with —
			// never a freshly fetched one.
			m.storageTargetLayout = m.storageChoice
			m.storageCommitPending = true
			return keyResult{model: m, handled: true,
				cmd: m.backend.CommitSSHStorage(m.storageChoice, m.storageView.PlanToken)}
		case ceremonyFinished:
			m.mode = gssBrowse
		case ceremonyNone:
		}
		return keyResult{model: m, handled: true}
	}

	options := m.overlaidOptions(s)
	if m.subTab == gssOptions && len(options) == 0 {
		// No rows (the backend could not answer): every row key is inert; the
		// pane renders the error note.
		return keyResult{model: m, handled: true}
	}
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
			// Refetch the storage view for the newly selected layout.
			m = m.refetchStoragePlan()
		}
		return keyResult{model: m, handled: true}
	case "space":
		if m.subTab == gssOptions {
			o := options[m.detailIndex(options)]
			if o.Selectable() {
				m.chosen = withToggled(m.chosen, o.Key)
			}
		}
		return keyResult{model: m, handled: true}
	case "a":
		if m.subTab == gssOptions && len(m.applyChosen(options)) > 0 {
			cer, cerErr := m.applyCeremonyFor(s)
			if cerErr != nil {
				// A preview that cannot be computed renders the error inline
				// and does NOT open the ceremony.
				m.optionsErr = cerErr.Error()
				return keyResult{model: m, handled: true}
			}
			m.ceremony = cer
			m.mode = gssApplyCeremony
		}
		return keyResult{model: m, handled: true}
	case "enter":
		if m.subTab == gssStorage && m.storageChoice != s.SSHStorage {
			if m.storageViewErr != "" {
				// A preview that cannot be computed renders the error inline
				// and does NOT open the ceremony.
				return keyResult{model: m, handled: true}
			}
			m.ceremony = m.storageCeremonyFor(m.storageView)
			m.mode = gssStorageCeremony
			return keyResult{model: m, handled: true}
		}
		return keyResult{model: m}
	}
	return keyResult{model: m}
}

// gssSubTabStripRows returns the number of rows the sub-tab strip occupies.
// This is the single source of truth for the strip's height; all consumers
// (renderOptions, renderStorage, gssOptionsTopLines, handleClick) must derive
// from this.
func gssSubTabStripRows() int {
	return 3 // top border + labels + bottom border
}

// subTabStrip renders the [Options] [Storage & preview] strip with a border.
func (m globalSSHModel) subTabStrip() string {
	options := gssTabOptionsLabel
	storage := gssTabStorageLabel
	if m.subTab == gssOptions {
		options = styleReverse.Render(options)
	} else {
		storage = styleReverse.Render(storage)
	}
	label := " " + options + " " + storage

	// Build a bordered box around the labels using dashed border runes in accent color.
	accentBorder := lipgloss.NewStyle().Foreground(DefaultTheme.Accent)
	width := lipgloss.Width(label) + 2 // label + 2 for the side borders
	topBorder := "╭" + strings.Repeat("╌", width) + "╮"
	labelLine := "┊ " + label + " ┊"
	bottomBorder := "╰" + strings.Repeat("╌", width) + "╯"

	return accentBorder.Render(topBorder) + "\n" + accentBorder.Render(labelLine) + "\n" + accentBorder.Render(bottomBorder)
}

// gssOptionsTopLines counts the body lines rendered above the first option
// row on the Options sub-tab (the sub-tab strip plus the optional findings
// banner) — shared by renderOptions and handleClick.
func gssOptionsTopLines(s DemoState) int {
	lines := gssSubTabStripRows() // sub-tab strip (single source)
	if findingsBanner(s, "SSH", gssBannerBeyond) != "" {
		lines++
	}
	return lines
}

// handleClick implements mouseTarget: the sub-tab strip occupies the first
// gssSubTabStripRows rows, where a click on either label switches sub-tabs
// (border rows are inert); on the Options sub-tab a click on an option row's
// checkbox glyph TOGGLES it like space (web Checkbox onClick stopPropagation)
// while a click elsewhere in the row selects it; on the Storage sub-tab a
// click on a radio row selects that layout and the Migrate button dispatches
// Enter. Ceremony buttons click through the shared ceremony zones.
func (m globalSSHModel) handleClick(x, y, width, height int, s DemoState) keyResult {
	if m.mode != gssBrowse {
		body := m.view(s, width, height).body
		if next, key, ok := ceremonyClickKey(m.ceremony, body, x, y); ok {
			m.ceremony = next
			return m.handleKey(key, s)
		}
		return keyResult{model: m}
	}

	// Check if the click is on the sub-tab strip (rows 0..gssSubTabStripRows-1).
	stripRows := gssSubTabStripRows()
	if y < stripRows {
		// The strip occupies multiple rows. The labels are on row 1 (the middle row).
		// Rows 0 (top border) and stripRows-1 (bottom border) are inert.
		if y == 1 {
			// Middle row with labels. Calculate label positions.
			// The strip renders as: "╭─────╮\n┊ Options Storage ┊\n╰─────╯"
			// The labels start after "┊ " (2 chars) and need to account for the column offset.
			labelStartX := 2 // "┊ " prefix
			optStart := labelStartX
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
		}
		// Border rows (0 and stripRows-1) and any other click in the strip area are inert.
		return keyResult{model: m}
	}
	if m.subTab == gssStorage {
		return m.handleStorageClick(x, y, width, height, s)
	}
	if x >= masterListWidth(width) || y < gssOptionsTopLines(s) {
		return keyResult{model: m}
	}
	options := m.overlaidOptions(s)
	row := (y - gssOptionsTopLines(s)) / optionRowLines
	if row >= len(options) {
		return keyResult{model: m}
	}
	if options[row].Selectable() && m.clickOnCheckbox(x, y, width, height, s) {
		// The checkbox cell toggles THAT row without moving the selection
		// (GlobalSsh.tsx:185 — Checkbox onClick stops propagation).
		m.chosen = withToggled(m.chosen, options[row].Key)
		return keyResult{model: m, handled: true}
	}
	m.detailKey = options[row].Key
	return keyResult{model: m, handled: true}
}

// clickOnCheckbox reports whether the click falls on the [ ]/[✓] toggle cell
// of the rendered body line — the glyph span is derived from the drawn row,
// never from column math.
func (m globalSSHModel) clickOnCheckbox(x, y, width, height int, s DemoState) bool {
	body := m.view(s, width, height).body
	return hitNeedle(body, x, y, glyphToggleOff) ||
		hitNeedle(body, x, y, glyphToggleOn)
}

// handleStorageClick resolves Storage & preview clicks: radio rows select a
// layout; the Migrate button walks the STORE-03 ceremony via its key.
func (m globalSSHModel) handleStorageClick(x, y, width, height int, s DemoState) keyResult {
	body := m.view(s, width, height).body
	if hitNeedle(body, x, y, " Migrate layout… (Enter) ") {
		return m.handleKey(mustKey("Enter"), s)
	}
	if x >= masterListWidth(width) {
		return keyResult{model: m}
	}
	line, ok := blockLine(body, y)
	if !ok {
		return keyResult{model: m}
	}
	switch {
	case strings.Contains(line, "Sentinel blocks in ~/.ssh/config"):
		m.storageChoice = StorageSentinel
		// CR-05: refetch, mirroring the keyboard up/down handler — a mouse
		// click that only set the radio without refetching left
		// m.storageViewErr holding a stale error, hiding the Migrate button
		// and making the ceremony unreachable from the mouse path.
		return keyResult{model: m.refetchStoragePlan(), handled: true}
	case strings.Contains(line, "gitid-owned ~/.ssh/config.d"):
		m.storageChoice = StorageInclude
		return keyResult{model: m.refetchStoragePlan(), handled: true}
	}
	return keyResult{model: m}
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
func optionRow(o GlobalSSHOptionView, chosen, selected, applied bool, width int) string {
	marker := "  "
	if selected {
		marker = styleBold.Render("▸ ")
	}
	selectable := o.Selectable()
	box := padDisplay(styleFaint.Render("·"), optionBoxWidth)
	if selectable {
		box = padDisplay(styleFaint.Render(glyphToggleOff), optionBoxWidth)
		if chosen {
			box = padDisplay(styleHealthy.Bold(true).Render(glyphToggleOn), optionBoxWidth)
		}
	} else if applied {
		box = padDisplay("✓", optionBoxWidth)
	}
	tone := " "
	switch o.State {
	case GlobalSSHAlreadySet:
		tone = styleHealthy.Render("✓")
	case GlobalSSHNeedsAction, GlobalSSHDiffers:
		tone = styleWarning.Render("!")
	case GlobalSSHNotApplicable:
		// A neutral marker, not a health-tone glyph (D-12 forbids introducing
		// a new health state): every other row carries a visible tone glyph,
		// so a blank cell here reads as a missing/broken row rather than a
		// deliberately inert one.
		tone = styleFaint.Render("·")
	}
	name := styleBold.Render(o.Key)
	if selected {
		name = styleSelected.Render(o.Key)
	}
	chip := ""
	if o.Risk != "" {
		chip = "  " + styleFaint.Render("["+o.Risk+"]")
	}
	line1 := " " + marker + box + tone + " " + name + chip
	line2 := "      " + styleFaint.Render(optionRowLine2(o))
	return truncLine(line1, width) + "\n" + truncLine(line2, width)
}

func optionRowLine2(o GlobalSSHOptionView) string {
	if o.State == GlobalSSHNotApplicable {
		return notApplicableSentence(o.NotApplicableReason)
	}
	now := "now: " + o.CurrentValue + " → " + o.Recommended
	switch o.State {
	case GlobalSSHDiffers:
		if o.AttributedToUser {
			return now + "  " + GlobalSSHWordDiffersUser
		}
		return now + "  " + GlobalSSHWordDiffersOutside
	case GlobalSSHAlreadySet:
		if strings.HasPrefix(o.Provenance, "not set") {
			return now + "  " + GlobalSSHWordSafeByDefault
		}
		return now + "  " + GlobalSSHWordAlreadySet
	default:
		return now
	}
}

func notApplicableSentence(r GlobalSSHNotApplicableReason) string {
	switch r {
	case GlobalSSHReasonPlatform:
		return GlobalSSHNAPlatform
	case GlobalSSHReasonVersionTooOld:
		return GlobalSSHNAVersionTooOld
	case GlobalSSHReasonVersionUnverified:
		return GlobalSSHNAVersionUnverified
	case GlobalSSHReasonNothingToVerify:
		return GlobalSSHNANothingToVerify
	case GlobalSSHReasonProbeFailed:
		return GlobalSSHNAProbeFailed
	default:
		return GlobalSSHNAPlatform
	}
}

// truncLine truncates a styled line to width cells with a visible `…` cue
// when it clips — master-list values (the `now: …` lines) must never
// hard-clip silently (UX re-verification R2).
func truncLine(line string, width int) string {
	return ansi.Truncate(line, width, "…")
}

// view implements screenModel.
func (m globalSSHModel) view(s DemoState, width, height int) screenView {
	options := m.overlaidOptions(s)
	chosen := m.applyChosen(options)

	// Status tally = needs-action + set-but-differs; skip not-applicable
	// (06-CONTEXT.md discretion, pinned here so 06-04 cannot re-derive it).
	attention := 0
	for _, o := range options {
		if o.needsAttention() {
			attention++
		}
	}
	status := "All recommendations applied or already set. Advisory, never a compliance gate."
	tone := "info"
	if attention > 0 {
		status = fmt.Sprintf("%d of %d options need action — %s", attention, len(options), GlobalSSHAdvisoryNote)
		tone = "warning"
	}

	crumb := "Options"
	if m.subTab == gssStorage {
		crumb = "Storage & preview"
	}

	var body string
	var actions []FooterAction
	capturesKeys := false
	switch m.mode {
	case gssApplyCeremony, gssStorageCeremony:
		body = m.subTabStrip() + "\n" + m.ceremony.view(width-2)
		actions = ceremonyFooterActions()
		capturesKeys = true // the ceremony consumes every plain key
	case gssBrowse:
		if m.subTab == gssOptions {
			if m.optionsErr != "" {
				// Advisory posture extends to the detection layer: the pane
				// renders the error note, never a blank body.
				body = m.subTabStrip() + "\n " + styleWarning.Render("! "+m.optionsErr) + "\n\n " +
					styleFaint.Render("The option states could not be read from this machine.")
				actions = []FooterAction{{Key: "←→", Label: "Options / Storage"}}
			} else {
				body = m.renderOptions(s, options, width, height)
				actions = []FooterAction{
					{Key: "↑↓", Label: "select option"},
					{Key: "←→", Label: "Options / Storage"},
					{Key: "space", Label: "toggle"},
				}
				if len(chosen) > 0 {
					actions = append(actions, FooterAction{Key: "a", Label: fmt.Sprintf("apply %d selected", len(chosen))})
				}
			}
		} else {
			body = m.renderStorage(s, width, height)
			actions = []FooterAction{
				{Key: "←→", Label: "Options / Storage"},
				{Key: "↑↓", Label: "layout"},
			}
			if m.storageChoice != s.SSHStorage && m.storageViewErr == "" {
				actions = append(actions, FooterAction{Key: "Enter", Label: "migrate layout…"})
			}
		}
	}
	return screenView{body: body, crumbs: []string{crumb}, status: status, statusTone: tone,
		actions: actions, capturesKeys: capturesKeys}
}

// renderOptions renders the Options master-detail.
func (m globalSSHModel) renderOptions(s DemoState, options []appliedOption, width, height int) string {
	// WR-17: a Backend implementation may legitimately return (nil, nil) —
	// zero rows, no error. handleKey guards len(options)==0 for key routing
	// and view guards m.optionsErr != "" for the fetch-error case, but a
	// zero-row/no-error answer reached here and panicked on options[selIdx]
	// below (detailIndex returns 0 on no match, and 0 is out of range for an
	// empty slice). globalssh.Statuses always returns len(Policy) rows
	// today, but NoopGlobalSSHPlanner exists precisely to be substituted.
	if len(options) == 0 {
		return m.subTabStrip() + "\n " + styleFaint.Render("No global SSH options to show.")
	}

	listWidth := masterListWidth(width)
	detailWidth := width - listWidth - masterDetailGutter
	rows := frameBodyRows(height) - gssOptionsTopLines(s)

	var listRows []string
	selIdx := m.detailIndex(options)
	for i, o := range options {
		listRows = append(listRows, optionRow(o.GlobalSSHOptionView, m.chosen[o.Key], i == selIdx, o.applied, listWidth))
	}
	list := strings.Join(listRows, "\n")

	detail := options[selIdx]
	explanation := detail.Explanation
	if explanation == "" {
		explanation = detail.OneLiner
	}
	var d strings.Builder
	d.WriteString(" " + styleBold.Render(detail.Key) + "\n")
	d.WriteString(" " + styleInfo.Render("~ "+GlobalSSHAdvisoryNote) + "\n\n")
	d.WriteString(" " + explanation + "\n")
	// D-03: provenance renders in the detail block — longer label text in an
	// existing slot, never a new row.
	if detail.Provenance != "" {
		d.WriteString(" " + styleFaint.Render(detail.Provenance) + "\n")
	}
	if detail.VersionNote != "" {
		d.WriteString(" " + styleFaint.Render(detail.VersionNote) + "\n")
	}
	if detail.ProbeError != "" {
		d.WriteString(" " + styleWarning.Render("! "+detail.ProbeError) + "\n")
	}
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
	rightWidth := width - leftWidth - masterDetailGutter
	rows := frameBodyRows(height) - gssSubTabStripRows() // account for the sub-tab strip

	current := func(layout SSHStorageLayout) string {
		if s.SSHStorage == layout {
			return " — current"
		}
		return ""
	}
	radio := func(layout SSHStorageLayout) string {
		if m.storageChoice == layout {
			return glyphRadioOn + " "
		}
		return glyphRadioOff + " "
	}
	var l strings.Builder
	l.WriteString(" " + styleFaint.Render("STORE-01 — where gitid-managed SSH config lives") + "\n")
	l.WriteString(" " + radio(StorageSentinel) + "Sentinel blocks in ~/.ssh/config (default)" + current(StorageSentinel) + "\n")
	l.WriteString(" " + radio(StorageInclude) + "gitid-owned ~/.ssh/config.d/gitid.config via one Include line" + current(StorageInclude) + "\n\n")
	l.WriteString(" " + styleFaint.Render("Include paths must be absolute or ~/.ssh-relative; the Include line goes NEAR THE TOP of ~/.ssh/config. Migration between layouts is backed-up and reversible (STORE-03).") + "\n")
	if m.storageChoice != s.SSHStorage && m.storageViewErr == "" {
		l.WriteString("\n " + styleSelected.Render(" Migrate layout… (Enter) ") + "\n")
	}
	left := l.String()

	var r strings.Builder
	if m.storageViewErr != "" {
		// When the preview cannot be computed, render the error in the right
		// pane and suppress the migrate action — a pane that cannot describe
		// the machine must not offer to change it.
		r.WriteString(" " + styleWarning.Render("! "+m.storageViewErr) + "\n")
		r.WriteString(" " + styleFaint.Render("Re-enter the screen to retry.") + "\n")
	} else if m.storageChoice == StorageSentinel {
		r.WriteString(" " + PreviewLabel("Resulting config — sentinel blocks in place") + "\n")
		r.WriteString(previewBlockClipped(m.storageView.SentinelPreview, false, rightWidth, 18) + "\n")
	} else {
		r.WriteString(" " + PreviewLabel("Resulting config — Include + owned file") + "\n")
		r.WriteString(previewBlockClipped(m.storageView.MainPreview, false, rightWidth, 4) + "\n")
		r.WriteString(previewBlockClipped(m.storageView.OwnedPreview, false, rightWidth, 10) + "\n")
	}
	right := lipgloss.NewStyle().Width(rightWidth).Render(r.String())

	return m.subTabStrip() + "\n" + joinMasterDetail(left, leftWidth, right, rows)
}
