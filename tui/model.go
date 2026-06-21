package tui

import (
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"

	"github.com/castocolina/gitid/internal/adopter"
	"github.com/castocolina/gitid/internal/doctor"
	"github.com/castocolina/gitid/internal/gitconfig"
	"github.com/castocolina/gitid/internal/identity"
	"github.com/castocolina/gitid/internal/repoclone"
	"github.com/castocolina/gitid/internal/uploader"
)

// viewKind identifies the currently active main-pane view.
type viewKind int

const (
	identitiesView    viewKind = iota // View 1: Identities (default)
	healthView                        // View 2: Health (doctor)
	globalOptionsView                 // View 3: Global Options
)

// modalKind identifies the currently active modal overlay.
// noModal means the persistent layout is shown without any overlay.
// Additional modal kinds are reserved for Plans 04-06; declared here so
// the type is stable across slices without import changes.
type modalKind int

const (
	noModal            modalKind = iota // no overlay — show persistent layout
	helpModal                           // ? key → help shortcut overlay
	paletteModal                        // Ctrl+P → command palette overlay
	fixConfirmModal                     // x in health view → fix confirm overlay (Plan 03)
	proveModal                          // structural inline edit → prove-before-write loop (Plan 04)
	editConfirmModal                    // non-structural inline edit → simple confirm (Plan 04)
	createWizardModal                   // a → create/add identity wizard (Plan 05)
	copyPubkeyModal                     // c → copy public key modal (Plan 05)
	deleteConfirmModal                  // d → delete identity confirm modal (Plan 06)
	rotateConfirmModal                  // R → rotate key confirm + wizard (Plan 06)
	adoptModal                          // A on kindFragment → adopt fragment modal (Plan 07)
	addRepoModal                        // ctrl+r → add repo clone modal (Plan 07)
)

// tuiDeps holds the injected dependencies for the TUI. Built once by
// buildTUIDeps and threaded through the root model and sub-models so every
// write screen receives real, filewriter-backed seams (CR-02).
type tuiDeps struct {
	doctor   doctor.Deps
	identity identity.Deps
	update   identity.UpdateDeps
	// delete holds the deletion deps wired via buildTUIDeleteDeps (Plan 06, D-16).
	// Routes all removals through filewriter.BackupAndRemove (SAFE-01, T-05.6-20).
	delete identity.DeleteDeps
	// readFragment reads a per-identity gitconfig fragment so the update path
	// can preserve the existing signing state (FIX-1).
	readFragment func(fragPath string) (gitconfig.FragmentInfo, error)

	// Phase 5.7 additions (Plan 06): adopt, repoclone, uploader Deps for the
	// TUI modals (Plans 07+). Every function field must be non-nil at runtime
	// (D-13/D-16 anti-blindspot; TestBuildTUIDepsNilGuard_Phase57).
	adopt     adopter.Deps
	repoclone repoclone.Deps
	uploader  uploader.Deps
}

// rootModel is the top-level Bubble Tea model for the Phase 5.6 two-pane
// persistent layout.
//
// Layout (above responsive breakpoint ≥ 80×24):
//
//	┌──────────────────────────────────────────────┐
//	│ HEADER: gitid · [Identities] Health Options  │
//	├──────────────┬───────────────────────────────┤
//	│ SIDEBAR 18c  │ MAIN PANE (remaining width)   │
//	├──────────────┴───────────────────────────────┤
//	│ FOOTER: q quit  tab focus  ↑↓ move  ? help   │
//	└──────────────────────────────────────────────┘
type rootModel struct {
	width, height int

	activeView  viewKind  // identitiesView | healthView | globalOptionsView
	activeModal modalKind // noModal | helpModal | paletteModal | fixConfirmModal

	sidebar          sidebarModel
	sidebarCollapsed bool // true when width < 80 (D-03)

	focused string // "sidebar" | "main" — which pane has keyboard focus

	toast      string
	toastStyle lipgloss.Style

	palette paletteModel

	// health is the Health view sub-model (Plan 03). Initialized on first switch to
	// healthView and on 'r' refresh; nil-guarded until init() is called.
	health      healthViewModel
	healthReady bool // true after the first init() call for health

	// confirm is the active confirm modal sub-model (Plan 03 for fix; Plan 06 for delete/rotate).
	confirm confirmModel

	// detail is the Identity detail pane sub-model (Plan 04).
	detail identityDetailModel

	// globalopts is the Global Options view sub-model (Plan 04).
	// Initialized on first switch to globalOptionsView and on 'r' refresh.
	globalopts      globalOptionsModel
	globaloptsReady bool // true after the first init call for globalopts

	// proveWizard is the shared prove-before-write modal sub-model (Plan 04).
	// Populated when activeModal == proveModal (structural inline edit).
	proveWizard wizardProveModel

	// wizard is the create/add identity wizard modal (Plan 05).
	// Populated when activeModal == createWizardModal.
	wizard createWizardModel

	// copyModal is the copy-public-key modal (Plan 05).
	// Populated when activeModal == copyPubkeyModal.
	copyModal copyPubkeyModel

	// adoptM is the Adopt fragment modal sub-model (Plan 07).
	// Populated when activeModal == adoptModal.
	adoptM adoptModel

	// addRepoM is the Add Repo clone modal sub-model (Plan 07).
	// Populated when activeModal == addRepoModal.
	addRepoM addRepoModel

	deps tuiDeps
}

// newRootModel constructs the root model with the full two-pane layout.
// deleteDeps wires the in-app delete/rotate paths (Plan 06). Pass a zero-value
// identity.DeleteDeps{} in tests that do not exercise delete/rotate flows.
// adoptDeps, repoCloneDeps, uploaderDeps wire the Phase 5.7 modal paths (Plan 07+).
// Pass zero-value structs in tests that do not exercise those paths.
func newRootModel(docDeps doctor.Deps, idDeps identity.Deps, upDeps identity.UpdateDeps, deleteDeps identity.DeleteDeps) rootModel {
	d := tuiDeps{
		doctor:       docDeps,
		identity:     idDeps,
		update:       upDeps,
		delete:       deleteDeps,
		readFragment: gitconfig.ReadFragment,
	}
	return rootModel{
		activeView: identitiesView,
		focused:    "sidebar",
		deps:       d,
		sidebar:    newSidebarModel(d.doctor),
		palette:    newPaletteModel(),
		health:     newHealthModel(d),
		detail:     newIdentityDetailModel(),
		globalopts: newGlobalOptionsModel(d),
	}
}

// newRootModelFull constructs the root model with all Phase 5.7 Deps populated.
// Called by tui.Run via buildTUIDeps() so the TUI has non-nil seams for the
// adopt/repoclone/uploader modals (Plans 07+). Tests that need the full wiring
// should use this constructor (or buildTUIDeps directly).
func newRootModelFull(docDeps doctor.Deps, idDeps identity.Deps, upDeps identity.UpdateDeps, deleteDeps identity.DeleteDeps, adoptDeps adopter.Deps, repoCloneDeps repoclone.Deps, uploaderDeps uploader.Deps) rootModel {
	d := tuiDeps{
		doctor:       docDeps,
		identity:     idDeps,
		update:       upDeps,
		delete:       deleteDeps,
		readFragment: gitconfig.ReadFragment,
		adopt:        adoptDeps,
		repoclone:    repoCloneDeps,
		uploader:     uploaderDeps,
	}
	return rootModel{
		activeView: identitiesView,
		focused:    "sidebar",
		deps:       d,
		sidebar:    newSidebarModel(d.doctor),
		palette:    newPaletteModel(),
		health:     newHealthModel(d),
		detail:     newIdentityDetailModel(),
		globalopts: newGlobalOptionsModel(d),
	}
}

// Init satisfies tea.Model. Seeds the sidebar refresh so the launched app
// shows real identities on first render (D-16 anti-blindspot), AND kicks off
// the doctor run so per-identity health badges show at rest — not only after
// the user visits the Health view (P1-5, D-08). The health family commands
// carry the initial runID, so their familyResultMsg results are accepted by
// the health sub-model's stale-guard and feed badgesFromFindings.
func (m rootModel) Init() tea.Cmd {
	_, healthCmd := m.health.init()
	return tea.Batch(m.sidebar.refresh(m.deps), healthCmd)
}

// Update satisfies tea.Model. Dispatches messages to the correct sub-model
// or handles global keys at the root level.
func (m rootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.sidebarCollapsed = msg.Width < 80
		return m, nil

	case refreshSidebarMsg:
		m.sidebar.accounts = msg.accounts
		m.sidebar.unmanaged = msg.unmanaged
		// Real initial selection: select the first identity when none is selected
		// so the detail pane populates on launch without a keystroke (P1-5).
		if m.sidebar.selected < 0 && len(msg.accounts) > 0 {
			m.sidebar.selected = 0
		}
		return m, nil

	case setToastMsg:
		m.toast = msg.text
		m.toastStyle = msg.style
		return m, clearToastAfter(3 * time.Second)

	case clearToastMsg:
		m.toast = ""
		return m, nil

	case clearModalMsg:
		m.activeModal = noModal
		return m, nil

	case familyResultMsg:
		// Propagate to the health sub-model.
		var cmd tea.Cmd
		m.health, cmd = m.health.update(msg)
		// After a family completes, update sidebar badges from the current findings.
		m.sidebar.badges = badgesFromFindings(m.health.findings)
		// The doctor run was seeded at launch (Init); mark it ready so opening the
		// Health view does not redundantly re-run the families (P1-5).
		m.healthReady = true
		return m, cmd

	case fixResultMsg:
		// Propagate to the confirm sub-model.
		var cmd tea.Cmd
		m.confirm, cmd = m.confirm.update(msg)
		// On successful fix, trigger a health re-run for the affected family so
		// findings and badges update live. The confirm sub-model emits both
		// clearModalCmd and a re-run cmd; we bundle those here.
		return m, cmd

	case deleteResultMsg:
		// Propagate to the confirm sub-model; on success dismiss + refresh sidebar
		// AND re-run health (D-4): a sidebar-only refresh left the Coherence section
		// listing includeIf/orphan findings for the just-deleted identity.
		var cmd tea.Cmd
		m.confirm, cmd = m.confirm.update(msg)
		if msg.err == nil {
			m.activeModal = noModal
			var healthCmd tea.Cmd
			m.health, healthCmd = m.health.refresh()
			m.healthReady = true
			return m, tea.Batch(cmd, m.sidebar.refresh(m.deps), healthCmd)
		}
		return m, cmd

	case rotateResultMsg:
		// Propagate to the confirm sub-model; on success dismiss + refresh sidebar
		// AND re-run health (D-4) so health findings reflect the rotated key state.
		var cmd tea.Cmd
		m.confirm, cmd = m.confirm.update(msg)
		if msg.err == nil {
			m.activeModal = noModal
			var healthCmd tea.Cmd
			m.health, healthCmd = m.health.refresh()
			m.healthReady = true
			return m, tea.Batch(cmd, m.sidebar.refresh(m.deps), healthCmd)
		}
		return m, cmd

	case preWriteResultMsg:
		// Route to the active modal's prove sub-model.
		if m.activeModal == createWizardModal {
			var cmd tea.Cmd
			m.wizard, cmd = m.wizard.update(msg)
			return m, cmd
		}
		var cmd tea.Cmd
		m.proveWizard, cmd = m.proveWizard.update(msg)
		return m, cmd

	case resolvedResultMsg:
		// Route to the active modal's prove sub-model.
		if m.activeModal == createWizardModal {
			var cmd tea.Cmd
			m.wizard, cmd = m.wizard.update(msg)
			return m, cmd
		}
		var cmd tea.Cmd
		m.proveWizard, cmd = m.proveWizard.update(msg)
		return m, cmd

	case writeResultMsg:
		// Propagate to the prove wizard sub-model; dismiss modal on success.
		var cmd tea.Cmd
		m.proveWizard, cmd = m.proveWizard.update(msg)
		if msg.err == nil {
			// Successful write: dismiss the modal and reset the detail pane.
			m.activeModal = noModal
			m.detail.inlineEditMode = false
			// Refresh sidebar so the updated identity shows.
			return m, tea.Batch(cmd, m.sidebar.refresh(m.deps))
		}
		return m, cmd

	case keygenResultMsg:
		// Propagate to the create wizard.
		if m.activeModal == createWizardModal {
			var cmd tea.Cmd
			m.wizard, cmd = m.wizard.update(msg)
			return m, cmd
		}
		return m, nil

	case wizardCreateResultMsg:
		// Propagate to the create wizard; dismiss and refresh sidebar on success.
		if m.activeModal == createWizardModal {
			var cmd tea.Cmd
			m.wizard, cmd = m.wizard.update(msg)
			if msg.err == nil {
				// Successful create: dismiss modal, refresh sidebar.
				m.activeModal = noModal
				return m, tea.Batch(cmd, m.sidebar.refresh(m.deps))
			}
			return m, cmd
		}
		return m, nil

	case clipboardResultMsg:
		// Propagate to the copy modal.
		if m.activeModal == copyPubkeyModal {
			var cmd tea.Cmd
			m.copyModal, cmd = m.copyModal.update(msg)
			return m, cmd
		}
		// Also propagate to wizard (upload step clipboard copy).
		if m.activeModal == createWizardModal {
			var cmd tea.Cmd
			m.wizard, cmd = m.wizard.update(msg)
			return m, cmd
		}
		return m, nil

	case uploadKeyResultMsg:
		// Propagate to the copy modal (Plan 07 upload-assist, AUTOUP-01).
		if m.activeModal == copyPubkeyModal {
			var cmd tea.Cmd
			m.copyModal, cmd = m.copyModal.update(msg)
			return m, cmd
		}
		return m, nil

	case adoptResultMsg:
		// Propagate to the adopt modal.
		if m.activeModal == adoptModal {
			var cmd tea.Cmd
			m.adoptM, cmd = m.adoptM.update(msg)
			return m, cmd
		}
		return m, nil

	case adoptCancelMsg:
		// Adopt was cancelled or completed — close the modal and refresh sidebar.
		m.activeModal = noModal
		return m, m.sidebar.refresh(m.deps)

	case removeOriginalResultMsg:
		// Optional remove-original step completed — always close modal and refresh.
		m.activeModal = noModal
		return m, tea.Batch(m.sidebar.refresh(m.deps), setToastCmd(func() string {
			if msg.err != nil {
				return "remove failed: " + msg.err.Error()
			}
			return "original removed (backed up: " + msg.backupPath + ")"
		}(), func() lipgloss.Style {
			if msg.err != nil {
				return SeverityStyle(doctor.SeverityError)
			}
			return StylePass
		}()))

	case detectResultMsg:
		// Propagate to the add repo modal.
		if m.activeModal == addRepoModal {
			var cmd tea.Cmd
			m.addRepoM, cmd = m.addRepoM.update(msg)
			return m, cmd
		}
		return m, nil

	case cloneResultMsg:
		// Propagate to the add repo modal.
		if m.activeModal == addRepoModal {
			var cmd tea.Cmd
			m.addRepoM, cmd = m.addRepoM.update(msg)
			return m, cmd
		}
		return m, nil

	case pullResultMsg:
		// Propagate to the add repo modal.
		if m.activeModal == addRepoModal {
			var cmd tea.Cmd
			m.addRepoM, cmd = m.addRepoM.update(msg)
			return m, cmd
		}
		return m, nil

	case baselineLoadedMsg:
		// Propagate to the global options sub-model.
		var cmd tea.Cmd
		m.globalopts, cmd = m.globalopts.update(msg)
		return m, cmd

	case spinner.TickMsg:
		// Propagate to the health sub-model (spinner animation in loading families).
		var cmd tea.Cmd
		m.health, cmd = m.health.update(msg)
		return m, cmd

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	// Delegate to active sub-model when palette is open.
	if m.activeModal == paletteModal {
		var cmd tea.Cmd
		m.palette, cmd = m.palette.update(msg)
		return m, cmd
	}

	return m, nil
}

// handleKey dispatches key presses, routing global keys at root and delegating
// modal-specific keys to the active modal sub-model (Pitfall 9 tab routing).
func (m rootModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Route to active modal first.
	switch m.activeModal {
	case helpModal:
		if msg.String() == "esc" || msg.String() == "?" {
			m.activeModal = noModal
		}
		return m, nil

	case paletteModal:
		if msg.String() == "esc" {
			m.activeModal = noModal
			m.palette = newPaletteModel()
			return m, nil
		}
		var cmd tea.Cmd
		m.palette, cmd = m.palette.update(msg)
		if m.palette.activated {
			item := m.palette.selectedItem()
			m.palette = newPaletteModel()
			m.activeModal = noModal
			return m.applyPaletteAction(item)
		}
		return m, cmd

	case fixConfirmModal, deleteConfirmModal, rotateConfirmModal:
		// Route key to the confirm sub-model.
		var cmd tea.Cmd
		m.confirm, cmd = m.confirm.update(msg)
		return m, cmd

	case proveModal:
		// Route key to the prove wizard sub-model.
		var cmd tea.Cmd
		m.proveWizard, cmd = m.proveWizard.update(msg)
		return m, cmd

	case editConfirmModal:
		// Simple confirm modal for non-structural edits.
		switch msg.String() {
		case "enter":
			// Dispatch the non-structural update via identity.Update.
			if m.detail.account != nil {
				existing := *m.detail.account
				edited := m.detail.editedAccountFromFields()
				m.activeModal = noModal
				m.detail.inlineEditMode = false
				return m, runProveWriteCmd(existing, edited, m.detail.signed, m.deps)
			}
			m.activeModal = noModal
		case "esc":
			m.activeModal = noModal
			m.detail.inlineEditMode = false
		}
		return m, nil

	case createWizardModal:
		// Route key to the create wizard. Form step keys are handled by handleKey;
		// other steps by update(). Return the sub-model's cmd to the runtime AS-IS
		// — do NOT execute it inline to peek at it. The textinput returns a
		// cursor-blink tea.Tick on every keystroke; running it synchronously here
		// slept the event loop ~150 ms per character (the reported typing lag).
		// Modal dismissal flows through the normal `clearModalMsg` handler above.
		var cmd tea.Cmd
		if m.wizard.step == wizardStepForm {
			m.wizard, cmd = m.wizard.handleKey(msg)
		} else {
			m.wizard, cmd = m.wizard.update(msg)
		}
		return m, cmd

	case copyPubkeyModal:
		// Route key to the copy modal; return its cmd directly (see createWizardModal
		// note — no inline cmd() peek, dismissal via clearModalMsg).
		var cmd tea.Cmd
		m.copyModal, cmd = m.copyModal.update(msg)
		return m, cmd

	case adoptModal:
		// Route key to the adopt modal (Plan 07, ADOPT-01).
		var cmd tea.Cmd
		m.adoptM, cmd = m.adoptM.update(msg)
		return m, cmd

	case addRepoModal:
		// Route key to the add repo modal (Plan 07, REPO-01).
		var cmd tea.Cmd
		m.addRepoM, cmd = m.addRepoM.update(msg)
		return m, cmd
	}

	// When inline editing is active in the identities view, route enter/esc to the
	// detail pane before global key dispatch (Pitfall 9: Tab routing extended).
	if m.detail.inlineEditMode && m.activeView == identitiesView {
		key := msg.String()
		if key == "enter" || key == "esc" {
			var cmd tea.Cmd
			m.detail, cmd = m.detail.handleKey(key)
			// After the detail pane processes Enter, check if it signals prove or confirm.
			if m.detail.proveModalPending {
				m.detail.proveModalPending = false
				if m.detail.account != nil {
					existing := *m.detail.account
					edited := m.detail.editedAccountFromFields()
					m.proveWizard = newWizardProveModel(existing, edited, m.detail.signed, m.deps)
					var initCmd tea.Cmd
					m.proveWizard, initCmd = m.proveWizard.init()
					m.activeModal = proveModal
					return m, tea.Batch(cmd, initCmd)
				}
			}
			if m.detail.editConfirmPending {
				m.detail.editConfirmPending = false
				m.activeModal = editConfirmModal
				return m, cmd
			}
			return m, cmd
		}
	}

	// Global keys (no modal open).
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "enter":
		// Enter on a selected sidebar row drills into the detail pane (P1-6):
		// previously Enter was a dead key, breaking the universal list convention.
		if m.activeView == identitiesView && m.focused == "sidebar" {
			if m.sidebar.selectedAccount() != nil || m.sidebar.selectedUnmanagedEntry() != nil {
				m.focused = "main"
			}
		}
		return m, nil

	case "?":
		m.activeModal = helpModal
		return m, nil

	case "ctrl+p":
		m.activeModal = paletteModal
		m.palette = newPaletteModel()
		return m, nil

	case "1":
		return m.switchTo(identitiesView)
	case "2":
		return m.switchTo(healthView)
	case "3":
		return m.switchTo(globalOptionsView)
	case "left", "right":
		// ←/→ cycle through the three views — an additive third switch path
		// alongside 1/2/3 + Ctrl+P (P2-9, D-04 extended). Tab owns pane focus,
		// so there is no conflict.
		if msg.String() == "left" {
			return m.switchTo(prevView(m.activeView))
		}
		return m.switchTo(nextView(m.activeView))

	case "\\":
		// SidebarToggle: when below breakpoint, toggle visibility.
		if m.sidebarCollapsed {
			m.sidebarCollapsed = false
		} else {
			m.sidebarCollapsed = m.width < 80
		}
		return m, nil

	case "tab":
		// Pitfall 9: Tab routing — when inline edit is active, Tab cycles detail fields.
		// Otherwise Tab cycles pane focus.
		if m.detail.inlineEditMode && m.activeView == identitiesView {
			var cmd tea.Cmd
			m.detail, cmd = m.detail.handleKey("tab")
			return m, cmd
		}
		// Pane focus cycle when no modal open.
		if m.focused == "sidebar" {
			m.focused = "main"
		} else {
			m.focused = "sidebar"
		}
		return m, nil

	case "up", "k":
		// In the Health view, ↑/↓ navigate the contextual family rail (D-5) — this
		// makes the "↑↓ move" footer affordance real instead of a no-op.
		if m.activeView == healthView {
			m.health = m.health.moveSelection(-1)
			return m, nil
		}
		if m.focused == "sidebar" {
			var cmd tea.Cmd
			m.sidebar, cmd = m.sidebar.updateKey("up")
			return m, cmd
		}
		return m, nil

	case "down", "j":
		if m.activeView == healthView {
			m.health = m.health.moveSelection(1)
			return m, nil
		}
		if m.focused == "sidebar" {
			var cmd tea.Cmd
			m.sidebar, cmd = m.sidebar.updateKey("down")
			return m, cmd
		}
		return m, nil

	case "r":
		// Refresh: re-stream health families when in health view;
		// re-read baseline when in global options view.
		switch m.activeView {
		case healthView:
			var cmd tea.Cmd
			m.health, cmd = m.health.refresh()
			m.healthReady = true
			return m, cmd
		case globalOptionsView:
			var cmd tea.Cmd
			m.globalopts, cmd = m.globalopts.refresh()
			m.globaloptsReady = true
			return m, cmd
		}
		return m, nil

	case "x":
		// In-app fix: only meaningful in health view when a fixable finding is focused.
		if m.activeView == healthView {
			// Find the currently focused fixable finding.
			finding, ok := m.focusedFixableFinding()
			if ok {
				m.confirm = newConfirmModel(fixConfirm, finding, m.deps)
				m.activeModal = fixConfirmModal
				return m, nil
			}
		}
		return m, setToastCmd("coming next", StyleFaint)

	case "e":
		// Route 'e' to inline edit in the appropriate view.
		switch m.activeView {
		case identitiesView:
			// Wire up the detail pane's deps and selected account.
			m.detail.account = m.sidebar.selectedAccount()
			m.detail.deps = m.deps
			var cmd tea.Cmd
			m.detail, cmd = m.detail.handleKey("e")
			// Check if the detail pane opened proveModal or editConfirmModal.
			if m.detail.proveModalPending {
				m.detail.proveModalPending = false
				if m.detail.account != nil {
					existing := *m.detail.account
					edited := m.detail.editedAccountFromFields()
					m.proveWizard = newWizardProveModel(existing, edited, m.detail.signed, m.deps)
					var initCmd tea.Cmd
					m.proveWizard, initCmd = m.proveWizard.init()
					m.activeModal = proveModal
					return m, tea.Batch(cmd, initCmd)
				}
			}
			return m, cmd
		case globalOptionsView:
			var cmd tea.Cmd
			m.globalopts, cmd = m.globalopts.handleKey("e")
			return m, cmd
		default:
			return m, setToastCmd("coming next", StyleFaint)
		}

	case "A":
		// Open the Adopt modal for the focused kindFragment unmanaged entry (Plan 07, ADOPT-01).
		// Distinct from lowercase 'a' (add identity wizard).
		if m.activeView == identitiesView {
			if ue := m.sidebar.selectedUnmanagedEntry(); ue != nil && ue.kind == kindFragment {
				m.adoptM = newAdoptModel(ue.fragmentPath, ue.shortName, nil, m.deps)
				m.activeModal = adoptModal
				return m, nil
			}
			return m, setToastCmd("select a fragment in the Unmanaged section to adopt", StyleFaint)
		}
		return m, setToastCmd("coming next", StyleFaint)

	case "a":
		// Open the create/add wizard modal (Plan 05).
		if m.activeView == identitiesView {
			m.wizard = newCreateWizardModel("", m.deps)
			m.activeModal = createWizardModal
			return m, nil
		}
		return m, setToastCmd("coming next", StyleFaint)

	case "c":
		// Open the copy-public-key modal for the selected identity (Plan 05),
		// or for the selected unmanaged entry (Plan 06, D-13 — pubkey only).
		if m.activeView == identitiesView {
			// Unmanaged entries take precedence for affordance dispatch (D-13):
			// copy the .pub line ONLY — the private key is never copied.
			if ue := m.sidebar.selectedUnmanagedEntry(); ue != nil {
				m.copyModal = newCopyPubkeyModel(ue.pubLine, ue.keyPath, "github.com", m.deps)
				var initCmd tea.Cmd
				m.copyModal, initCmd = m.copyModal.init()
				m.activeModal = copyPubkeyModal
				return m, initCmd
			}
			acct := m.sidebar.selectedAccount()
			if acct != nil {
				// Guard: an Incomplete identity whose SSH/key side is gone has no
				// public key to copy (D-3). Refuse rather than open a modal on a
				// dead row or copy a bare path string.
				if acct.PubPath == "" {
					return m, setToastCmd("no public key for this identity", StyleFaint)
				}
				// Read the real .pub line; never fall back to copying the path.
				pubLine := ""
				if m.deps.update.ReadPub != nil {
					if line, err := m.deps.update.ReadPub(acct.PubPath); err == nil && line != "" {
						pubLine = line
					}
				}
				if pubLine == "" {
					return m, setToastCmd("public key file not found — nothing to copy", StyleFaint)
				}
				provider := acct.Provider
				if provider == "" {
					provider = "github.com"
				}
				m.copyModal = newCopyPubkeyModel(pubLine, acct.KeyPath, provider, m.deps)
				var initCmd tea.Cmd
				m.copyModal, initCmd = m.copyModal.init()
				m.activeModal = copyPubkeyModal
				return m, initCmd
			}
			return m, setToastCmd("no identity selected", StyleFaint)
		}
		return m, setToastCmd("coming next", StyleFaint)

	case "d":
		// Open the delete confirm modal for the selected identity (Plan 06, TUI-06).
		if m.activeView == identitiesView {
			acct := m.sidebar.selectedAccount()
			if acct != nil {
				m.confirm = newConfirmModel(deleteConfirm, doctor.Finding{}, m.deps)
				m.confirm.deleteAcct = acct
				m.activeModal = deleteConfirmModal
				return m, nil
			}
			return m, setToastCmd("no identity selected", StyleFaint)
		}
		return m, setToastCmd("coming next", StyleFaint)

	case "R":
		// Open the rotate confirm modal for the selected identity (Plan 06, TUI-06).
		if m.activeView == identitiesView {
			acct := m.sidebar.selectedAccount()
			if acct != nil {
				m.confirm = newConfirmModel(rotateConfirm, doctor.Finding{}, m.deps)
				m.confirm.rotateAcct = acct
				m.activeModal = rotateConfirmModal
				return m, nil
			}
			return m, setToastCmd("no identity selected", StyleFaint)
		}
		return m, setToastCmd("coming next", StyleFaint)

	case "o":
		// Open-location affordance for unmanaged entries (Plan 06, D-13):
		// reveal the key's directory in the OS file manager, dispatched via
		// os/exec WITHOUT a shell (no interpolation — T-05.6-23). Read-only.
		if m.activeView == identitiesView {
			if ue := m.sidebar.selectedUnmanagedEntry(); ue != nil {
				return m, openLocationCmd(ue.keyPath)
			}
		}
		return m, setToastCmd("coming next", StyleFaint)

	case "p":
		// Reveal-path affordance for unmanaged entries (Plan 06, D-13): show the
		// full private-key path in the toast area. Read-only — nothing is copied.
		if m.activeView == identitiesView {
			if ue := m.sidebar.selectedUnmanagedEntry(); ue != nil {
				return m, setToastCmd("path: "+ue.keyPath, StyleFaint)
			}
		}
		return m, setToastCmd("coming next", StyleFaint)

	case "ctrl+r":
		// Open the Add Repo modal from the Identities view (Plan 07, REPO-01).
		if m.activeView == identitiesView {
			m.addRepoM = newAddRepoModel(m.deps)
			m.activeModal = addRepoModal
			return m, nil
		}
		return m, setToastCmd("coming next", StyleFaint)
	}

	return m, nil
}

// focusedFixableFinding returns the first fixable finding from the health view's
// findings, prioritising the finding at the current cursor position. Returns
// (finding, true) when a fixable finding is available, (zero, false) otherwise.
func (m rootModel) focusedFixableFinding() (doctor.Finding, bool) {
	// Prefer a fixable finding in the family the user navigated to with ↑/↓ (D-5),
	// so 'x' acts on what the rail cursor points at.
	if fam := m.health.selectedFamily(); fam != "" {
		for _, f := range m.health.findings[fam] {
			if f.Fix != nil {
				return f, true
			}
		}
	}
	// Fall back to the first fixable finding across all families.
	for _, fam := range doctor.Families() {
		for _, f := range m.health.findings[fam] {
			if f.Fix != nil {
				return f, true
			}
		}
	}
	return doctor.Finding{}, false
}

// applyPaletteAction handles a palette item activation.
func (m rootModel) applyPaletteAction(item paletteItem) (tea.Model, tea.Cmd) {
	switch item.action {
	case "view:identities":
		return m.switchTo(identitiesView)
	case "view:health":
		return m.switchTo(healthView)
	case "view:global":
		return m.switchTo(globalOptionsView)

	case "action:addrepo":
		// Open the Add Repo modal (Plan 07, REPO-01).
		m.addRepoM = newAddRepoModel(m.deps)
		m.activeModal = addRepoModal
		return m, nil

	case "action:adopt":
		// Open the Adopt modal for the focused fragment (Plan 07, ADOPT-01).
		if ue := m.sidebar.selectedUnmanagedEntry(); ue != nil && ue.kind == kindFragment {
			m.adoptM = newAdoptModel(ue.fragmentPath, ue.shortName, nil, m.deps)
			m.activeModal = adoptModal
			return m, nil
		}
		return m, setToastCmd("select a fragment in the Unmanaged section first", StyleFaint)

	default:
		return m, setToastCmd("coming next", StyleFaint)
	}
}

// switchTo sets the active view and performs the view's lazy initialization the
// first time it is shown (Health streams the doctor families; Global Options
// loads the baseline). Shared by the 1/2/3 keys, the ←/→ keys, and the command
// palette so every switch path inits consistently.
func (m rootModel) switchTo(v viewKind) (tea.Model, tea.Cmd) {
	m.activeView = v
	switch v {
	case healthView:
		if !m.healthReady {
			m.healthReady = true
			var cmd tea.Cmd
			m.health, cmd = m.health.init()
			return m, cmd
		}
	case globalOptionsView:
		if !m.globaloptsReady {
			m.globaloptsReady = true
			var cmd tea.Cmd
			m.globalopts, cmd = m.globalopts.refresh()
			return m, cmd
		}
	}
	return m, nil
}

// nextView / prevView cycle the three views in order for ←/→ navigation.
func nextView(v viewKind) viewKind {
	switch v {
	case identitiesView:
		return healthView
	case healthView:
		return globalOptionsView
	default:
		return identitiesView
	}
}

func prevView(v viewKind) viewKind {
	switch v {
	case identitiesView:
		return globalOptionsView
	case healthView:
		return identitiesView
	default:
		return healthView
	}
}

// View satisfies tea.Model. Returns the rendered content with AltScreen enabled.
// Alt-screen via View.AltScreen = true (tea.WithAltScreen() does not exist in v2 — Pitfall 1).
func (m rootModel) View() tea.View {
	v := tea.NewView(m.renderContent())
	v.AltScreen = true
	return v
}

// renderContent returns the full display string. If the terminal is too small
// it returns the plain-text minimum-size guard (no lipgloss). Otherwise renders
// the persistent two-pane layout, compositing any active modal overlay on top.
func (m rootModel) renderContent() string {
	if m.width < 80 || m.height < 24 {
		return "Terminal too small — resize to at least 80x24"
	}

	layout := m.renderPersistentLayout()

	if m.activeModal == noModal {
		return layout
	}

	// Dim the persistent layout before compositing the modal overlay (D-02).
	dimmed := StyleDimmed.Render(layout)

	var modalContent string
	switch m.activeModal {
	case helpModal:
		modalContent = renderHelpModal(m.width)
	case paletteModal:
		modalContent = m.palette.view(m.width)
	case fixConfirmModal, deleteConfirmModal, rotateConfirmModal:
		modalContent = m.confirm.view(m.width)
	case proveModal:
		modalContent = m.proveWizard.view(m.width)
	case editConfirmModal:
		modalContent = renderEditConfirmModal(m.width)
	case createWizardModal:
		modalContent = m.wizard.view(m.width)
	case copyPubkeyModal:
		modalContent = m.copyModal.view(m.width)
	case adoptModal:
		modalContent = m.adoptM.view(m.width)
	case addRepoModal:
		modalContent = m.addRepoM.view(m.width)
	default:
		return dimmed
	}

	mw := modalWidth(m.width)
	mh := lipgloss.Height(modalContent)
	x, y := modalOrigin(m.width, m.height, mw, mh)
	return placeOverlay(x, y, modalContent, dimmed)
}

// modalWidth returns the clamped modal width: min(width-8, 72) per UI-SPEC.
func modalWidth(w int) int {
	mw := w - 8
	if mw > 72 {
		mw = 72
	}
	if mw < 20 {
		mw = 20
	}
	return mw
}

// renderPersistentLayout builds the full header + (sidebar | main) + footer layout.
func (m rootModel) renderPersistentLayout() string {
	header := m.renderHeader()
	footer := m.renderFooter()

	// Reserve 2 rows for header and footer; remainder is content height.
	contentH := m.height - 2
	if contentH < 1 {
		contentH = 1
	}

	var body string
	if m.sidebarCollapsed {
		// Single-pane mode: main fills the full width.
		body = m.renderMainPane(m.width, contentH)
	} else {
		// Two-pane mode: sidebar 18c + separator + main (RESEARCH Pattern 3,
		// Pitfall 8: sidebar 18 + 1 separator = 19 cols subtracted from mainW).
		const sidebarW = 18
		mainW := m.width - sidebarW - 1 // 1 for separator column

		sb := m.renderRail(sidebarW, contentH, m.focused == "sidebar")
		main := m.renderMainPane(mainW, contentH)

		// Build a separator column of "│" characters, one per content row.
		sepLines := make([]string, contentH)
		for i := range sepLines {
			sepLines[i] = "│"
		}
		sep := strings.Join(sepLines, "\n")

		body = lipgloss.JoinHorizontal(lipgloss.Top, sb, sep, main)
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
}

// renderHeader renders the 1-row header: app name, view tabs, health badge slot.
func (m rootModel) renderHeader() string {
	appName := StyleTitle.Render("gitid")

	tabs := []string{
		m.renderTab("Identities", identitiesView),
		m.renderTab("Health", healthView),
		m.renderTab("Global Options", globalOptionsView),
	}
	tabStr := "  " + strings.Join(tabs, "  ")

	// Toast overlays the health badge slot when active.
	badge := "  "
	if m.toast != "" {
		badge = m.toastStyle.Render(m.toast)
	}

	// Right-align the badge.
	center := appName + tabStr
	padW := m.width - lipgloss.Width(center) - lipgloss.Width(badge)
	if padW < 1 {
		padW = 1
	}
	return center + strings.Repeat(" ", padW) + badge
}

// renderTab formats a single view tab, prefixed with its switch key so the
// tabs read as a visible keymap (recognition over recall): "1 Identities",
// "2 Health", "3 Global Options". viewKind is iota-ordered, so the key is
// int(v)+1. The active tab is bracketed + bold-underlined.
func (m rootModel) renderTab(label string, v viewKind) string {
	keyed := itoa(int(v)+1) + " " + label
	if m.activeView == v {
		return "[" + StyleTabActive.Render(keyed) + "]"
	}
	return StyleTabInactive.Render(keyed)
}

// renderFooter renders the 1-row footer with bold key hints for the current
// view and modal state. Always rendered (UI-SPEC Footer contract; closes G-02/G-04).
func (m rootModel) renderFooter() string {
	hint := func(k, d string) string {
		return StyleHelpKey.Render(k) + " " + StyleHelpDesc.Render(d)
	}

	var hints []string
	if m.activeModal != noModal {
		hints = []string{hint("esc", "close"), hint("?", "help")}
	} else {
		// Priority order: the highest-value, load-bearing affordances come first
		// so they survive when the line is collapsed (P0-2/P1-4). View switching,
		// add, select, and help must never be the first dropped.
		// "views" + "quit" are the always-on essentials (navigation + exit) and
		// lead so they survive collapse; per-view actions follow by value.
		hints = append(hints, hint("1·2·3/←→", "views"), hint("q", "quit"))
		// In collapsed mode the sidebar toggle is essential navigation — keep it
		// high-priority so it is not dropped before the action hints.
		if m.sidebarCollapsed {
			hints = append(hints, hint("\\", "toggle sidebar"))
		}
		switch m.activeView {
		case identitiesView:
			hints = append(hints,
				hint("a", "add"),
				hint("enter", "select"),
				hint("e", "edit"),
			)
			// Only advertise copy when the selected identity actually has a public
			// key — a dead/incomplete row has nothing to copy (D-3).
			if acct := m.sidebar.selectedAccount(); acct != nil && acct.PubPath != "" {
				hints = append(hints, hint("c", "copy"))
			}
			// Adopt hint: only shown when a kindFragment unmanaged row is focused (ADOPT-01).
			if ue := m.sidebar.selectedUnmanagedEntry(); ue != nil && ue.kind == kindFragment {
				hints = append(hints, hint("A", "adopt"))
			}
			hints = append(hints,
				hint("ctrl+r", "repo"),
				hint("↑↓", "move"),
				hint("d", "delete"),
				hint("R", "new key"),
			)
		case healthView:
			hints = append(hints,
				hint("x", "fix"),
				hint("↑↓", "move"),
				hint("r", "refresh"),
			)
		case globalOptionsView:
			hints = append(hints,
				hint("e", "edit"),
				hint("r", "refresh"),
			)
		}
		hints = append(hints, hint("?", "help"), hint("ctrl+p", "palette"))
	}

	maxW := m.width - 2
	if maxW < 1 {
		maxW = 1
	}
	return fitHints(hints, maxW)
}

// fitHints joins as many priority-ordered hints as fit within maxW visible
// columns. When any hint is dropped, it appends a faint "· more in ?" pointer
// so the full keymap is one keypress away (the help overlay) — instead of
// mid-string truncation that silently hid primary actions (P0-2).
func fitHints(hints []string, maxW int) string {
	const sep = "  "
	more := StyleFaint.Render("· more in ?")
	kept := make([]string, 0, len(hints))
	truncated := false
	for i, h := range hints {
		candidate := strings.Join(append(kept, h), sep)
		reserve := 0
		if i < len(hints)-1 {
			reserve = lipgloss.Width(sep) + lipgloss.Width(more)
		}
		if lipgloss.Width(candidate)+reserve > maxW {
			truncated = true
			break
		}
		kept = append(kept, h)
	}
	line := strings.Join(kept, sep)
	if truncated && len(kept) < len(hints) {
		if line != "" {
			line += sep
		}
		line += more
	}
	if lipgloss.Width(line) > maxW {
		line = truncateString(line, maxW)
	}
	return line
}

// renderRail renders the left rail's content for the ACTIVE view (D-01 reopened,
// WP-5: the rail is now contextual). It stays 18 cols and Tab-focusable in every
// view — only its content changes, so the rail no longer "lies" by showing the
// identity list while the main pane is about doctor families or global config:
//   - Identities view → managed identities + the unmanaged section
//   - Health view     → the 8 doctor families with a live status glyph
//   - Global Options  → the config-section index
func (m rootModel) renderRail(w, h int, focused bool) string {
	switch m.activeView {
	case healthView:
		return m.renderHealthRail(w)
	case globalOptionsView:
		return renderGlobalRail(w)
	default:
		return m.sidebar.view(w, h, focused)
	}
}

// renderHealthRail lists the 8 doctor families with a status glyph derived from
// the live health run: ✓ clear, ! has findings, ✗ check errored, · pending.
func (m rootModel) renderHealthRail(w int) string {
	ascii := asciiMode()
	var sb strings.Builder
	sb.WriteString(StyleSidebarSection.Render("Health") + "\n")
	for _, fam := range doctor.Families() {
		idx := familyIndex(fam)
		var glyph string
		switch m.health.families[idx] {
		case familyError:
			glyph = SeverityStyle(doctor.SeverityError).Render(SeverityGlyph(doctor.SeverityError, ascii))
		case familyLoaded:
			if len(m.health.findings[fam]) > 0 {
				glyph = SeverityStyle(doctor.SeverityWarning).Render(SeverityGlyph(doctor.SeverityWarning, ascii))
			} else {
				passGlyph := "✓"
				if ascii {
					passGlyph = "OK"
				}
				glyph = StylePass.Render(passGlyph)
			}
		default: // familyLoading / pending
			glyph = StyleFaint.Render("·")
		}
		// Cursor marks the family the user has navigated to with ↑/↓ (D-5).
		cursor := "  "
		label := StyleSidebarItem.Render(truncateString(string(fam), w-4))
		if idx == m.health.selected {
			cursor = "▸ "
			label = StyleTabActive.Render(truncateString(string(fam), w-4))
		}
		sb.WriteString(cursor + glyph + " " + label + "\n")
	}
	return sb.String()
}

// globalSections names the sections the Global Options view renders, in order.
var globalSections = []string{"Core", "Push/Pull/Fetch", "Color", "Aliases", "Gitignore", "URL Rewrites"}

// renderGlobalRail lists the Global Options config sections as an anchor index.
func renderGlobalRail(w int) string {
	var sb strings.Builder
	sb.WriteString(StyleSidebarSection.Render("Global") + "\n")
	for _, s := range globalSections {
		sb.WriteString("  " + StyleSidebarItem.Render(truncateString(s, w-2)) + "\n")
	}
	return sb.String()
}

// renderMainPane renders the active view's content at the given dimensions.
func (m rootModel) renderMainPane(w, h int) string {
	switch m.activeView {
	case identitiesView:
		return m.renderIdentitiesMainPane(w)
	case healthView:
		return m.health.view(w, h)
	case globalOptionsView:
		return m.globalopts.view(w, h)
	}
	return ""
}

// renderIdentitiesMainPane renders View 1 main pane content using the detail sub-model.
// The detail pane is kept in sync with the sidebar selection on every render.
func (m rootModel) renderIdentitiesMainPane(w int) string {
	// Keep the detail model's account in sync with the sidebar selection.
	// This is a pure render path (no mutation of model state).
	detail := m.detail
	detail.account = m.sidebar.selectedAccount()
	return detail.view(w)
}

// renderEditConfirmModal renders the simple confirm modal for non-structural edits.
// Shows "Write changes? [Enter / Esc]" per UI-SPEC D-05.
func renderEditConfirmModal(w int) string {
	mw := modalWidth(w)
	var sb strings.Builder
	sb.WriteString(StyleModalTitle.Render("Write Changes"))
	sb.WriteString("\n\n")
	sb.WriteString(StyleBody.Render("Apply the edited field values?"))
	sb.WriteString("\n\n")
	sb.WriteString(StyleFaint.Render("[Enter to write · Esc to cancel]"))
	return StyleModal.Width(mw).Render(sb.String())
}

// truncateString truncates s to at most maxCols visible columns, appending "…".
// ANSI escape sequences are counted as zero visible width so styled strings
// are truncated at the correct visible boundary. We use a rune-by-rune loop
// but check lipgloss.Width on each prefix slice to get ANSI-aware measurement.
// This is a best-effort approach — for the footer use case it is accurate enough.
func truncateString(s string, maxCols int) string {
	if maxCols <= 0 {
		return ""
	}
	// Use lipgloss.Width which strips ANSI sequences to measure visible width.
	if lipgloss.Width(s) <= maxCols {
		return s
	}
	// Binary search or linear scan by rune index to find the cut point.
	// We add the ellipsis so the effective budget is maxCols - 1 visible cols.
	budget := maxCols - 1
	runes := []rune(s)
	// Scan from left: build prefix and measure visible width.
	for i := len(runes); i > 0; i-- {
		prefix := string(runes[:i])
		if lipgloss.Width(prefix) <= budget {
			return prefix + "…"
		}
	}
	return "…"
}
