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

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// Global SSH sub-tabs.
type gssSubTab int

const (
	gssOptions gssSubTab = iota
	gssStorage
	gssProperties
)

// Global SSH modes.
type gssMode int

const (
	gssBrowse gssMode = iota
	gssApplyCeremony
	gssStorageCeremony
	// gssCustomDirectiveForm is stage 1 (Phase 9.5 plan 09.5-04, PROP-04):
	// the 2-field name/value entry form, mirroring plan 09.5-03's Git
	// custom-key form.
	gssCustomDirectiveForm
	// gssCustomDirectiveValidate is stage 2: the async staged-config
	// ssh -G validate+prove dispatch and its in-flight/stopping render —
	// the ONLY genuinely new render this plan adds (D-H/D-I).
	gssCustomDirectiveValidate
	// gssCustomDirectiveCeremony is stage 3: the standard non-destructive
	// Async apply ceremony, reached ONLY from a stage-2 OK:true proof.
	gssCustomDirectiveCeremony
)

// Sub-tab strip composition — subTabStrip renders exactly these labels
// (with a one-space lead and a one-space gap) and handleClick hit-tests
// against the same strings, so the spans can never drift.
const (
	// gssTabOptionsLabel derives from design.go's frozen
	// PropsOptionsSubTabLabel constant (padding added around it) — ONE
	// source for the label text, never restated (WR-06, round 3).
	gssTabOptionsLabel = " " + PropsOptionsSubTabLabel + " "
	// gssTabStorageLabel derives from design.go's frozen
	// PropsSSHStorageSubTabLabel constant (padding added around it) — ONE
	// source for the label text, never restated (WR-11).
	gssTabStorageLabel = " " + PropsSSHStorageSubTabLabel + " "
	// gssTabPropertiesLabel derives from design.go's frozen
	// PropsSSHSubTabLabel constant (padding added around it) — ONE source
	// for the label text, never restated.
	gssTabPropertiesLabel = " " + PropsSSHSubTabLabel + " "
)

// gssFooterCycleLabel is the ←→ footer action's label, naming all three
// sub-tabs — identical across every gssBrowse-mode branch so the footer
// never implies a different cycle depending on which sub-tab is active.
const gssFooterCycleLabel = "Options / Storage / Directives"

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
	// directives is the live "All directives" sub-tab row set (PROP-01)
	// fetched from the backend on activation, synchronously, the SAME
	// pattern activate() already uses for options above; directivesErr
	// carries the fetch failure the pane renders instead of a blank body,
	// mirroring optionsErr's advisory posture.
	directives    []SSHDirectiveView
	directivesErr string
	// filter is the "All directives" sub-tab's type-to-filter textinput
	// (D-B); filterFocused mirrors globalGitModel.fieldEditing's keyboard-
	// capture contract — while true, handleKey routes every key but esc
	// into the input and view() reports capturesKeys: true.
	filter        textinput.Model
	filterFocused bool
	// propDetailKey is the selected row's key on the properties sub-tab —
	// kept SEPARATE from detailKey (the Options sub-tab's own selection)
	// because the two lists' keys live in different casing domains
	// (lowercase ssh -G spellings here vs. the Policy table's canonical
	// camelCase there) and are never the same slice.
	propDetailKey string
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
	// listWindowStart is the index of the first visible ROW in the Options
	// master list (0-based) — WR-09/09.4-REVIEW.md: this screen rendered
	// every row unconditionally with no scroll window (unlike Global Git's
	// gitComputeScrollWindow/gitRowForScreenRow pair), risking silent
	// truncation past the frame's fixed row budget and a click-row desync
	// once the policy table grows. Mirrors globalGitModel.listWindowStart
	// exactly, reusing the same generic gitScrollWindow/gitRowForScreenRow/
	// scrollWindowFor/gitCueLine machinery (that machinery was already
	// screen-agnostic despite its git* naming — nothing in it references
	// Global Git specifically).
	listWindowStart int
	// lastHeight persists the terminal height from the most recent
	// tea.WindowSizeMsg (WR-10, 09.4-REVIEW.md independent re-review),
	// mirroring globalGitModel.lastHeight — gssVisibleRowCount measures the
	// REAL row budget instead of the canonical minFrameHeight. Zero until
	// the first resize arrives; rowBudgetHeight falls back to
	// minFrameHeight in that case.
	lastHeight int
	// commitRequestToken is incremented at every ceremony confirm (CR-01,
	// 09.4-REVIEW.md second independent re-review), mirroring
	// globalGitModel.commitRequestToken — see that field's doc comment for
	// the full rationale. Captured into the dispatched command's wrapper
	// message (gitCommitTokenMsg, shared with Global Git) so handleMsg can
	// tell a stale message apart from the genuinely current ceremony's own.
	commitRequestToken int
	// customDirectiveNameInput / customDirectiveValueInput are stage 1's
	// free-form name/value pair — mirrors globalGitModel.customKeyInput /
	// customValueInput exactly (Phase 9.5 plan 09.5-04, PROP-04).
	customDirectiveNameInput  textinput.Model
	customDirectiveValueInput textinput.Model
	// customDirectiveFieldFocus is 0 = name, 1 = value — the SAME 0/1
	// convention every other 2-field form in this codebase uses.
	customDirectiveFieldFocus int
	// customDirectiveProofPending is true from the moment
	// ValidateCustomSSHDirective is dispatched until its
	// SSHCustomDirectiveProofMsg answer arrives — while true, stage 2's keys
	// are inert (mirrors every other in-flight state on this screen).
	customDirectiveProofPending bool
	// customDirectiveProof is the LAST stage-2 proof result, rendered by
	// renderCustomDirectiveValidate whenever the proof stopped short of
	// OK: true (an OK: true proof transitions mode directly to the
	// ceremony in the SAME handleMsg call, so this field is never read for
	// that outcome).
	customDirectiveProof SSHDirectiveProofView
	// customDirectiveValidateErr carries a TRANSPORT-level stage-2 failure
	// (SSHCustomDirectiveProofMsg.Err) — an unproven directive that fails
	// closed, distinct from a completed-but-rejected classification.
	customDirectiveValidateErr string
	// pendingDirectiveName / pendingDirectiveValue are the EXACT name/value
	// pair submitted to ValidateCustomSSHDirective and, on an OK: true
	// proof, to CustomSSHDirectivePlan and CommitCustomSSHDirective —
	// mirrors globalGitModel.pendingCustomKey / pendingCustomValue.
	pendingDirectiveName  string
	pendingDirectiveValue string
	// customDirectiveCommitPending gates the custom-directive ceremony's
	// receipt: CommitCustomSSHDirective is dispatched only from
	// ceremonyConfirmed, and the receipt is reachable only from
	// SSHCustomDirectiveCommitMsg's explicit success (mirrors
	// applyCommitPending/storageCommitPending above).
	customDirectiveCommitPending bool
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
		filter:        newPropertiesFilterInput(),
	}
}

// newPropertiesFilterInput builds the "All directives" filter textinput
// with its frozen prompt/placeholder (09.5-UI-SPEC.md) — a small helper so
// both the constructor and activate()'s per-entry reset build the SAME
// shape.
func newPropertiesFilterInput() textinput.Model {
	ti := newTextInput("")
	ti.Prompt = "/ "
	ti.Placeholder = PropsFilterPlaceholder
	// WR-03: bubbles/v2's textinput clips its placeholder to the model's
	// width; the zero value (no SetWidth call) clips to a single rune, so
	// the frozen placeholder never actually rendered — the row showed "/ T".
	ti.SetWidth(lipgloss.Width(PropsFilterPlaceholder) + 1)
	return ti
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
	// CR-02: a screen re-entered from scratch must not resume a ceremony
	// whose selection this same call just cleared above. The keyboard
	// cannot reach activate() while a ceremony is open (its handleKey
	// returns handled:true, short-circuiting the globals), but a mouse
	// click on the header tab bar bypasses that guard, so this reset is the
	// state-machine half of the fix — App.handleMouse's capturesKeys guard
	// is the other half.
	m.mode = gssBrowse
	m.applyCommitPending = false
	m.storageCommitPending = false
	m.ceremony = ceremonyModel{}
	m.listWindowStart = 0
	// Phase 9.5 plan 09.5-04 (PROP-04): every custom-directive form/proof/
	// commit field resets on entry, mirroring the CR-02 reset above — a
	// screen re-entered from scratch must never resume a stale stage 1/2/3
	// state a previous visit left behind.
	m.customDirectiveNameInput = newTextInput("")
	m.customDirectiveValueInput = newTextInput("")
	m.customDirectiveFieldFocus = 0
	m.customDirectiveProofPending = false
	m.customDirectiveProof = SSHDirectiveProofView{}
	m.customDirectiveValidateErr = ""
	m.pendingDirectiveName = ""
	m.pendingDirectiveValue = ""
	m.customDirectiveCommitPending = false
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
	directives, derr := m.backend.AllSSHDirectives()
	m.directives = directives
	if derr != nil {
		m.directives = nil
		m.directivesErr = derr.Error()
	}
	// D-B: the filter resets per-entry, alongside every other per-entry
	// reset above — a stale filter from a previous visit must never survive
	// re-entering the screen.
	m.filter = newPropertiesFilterInput()
	m.filterFocused = false
	if len(m.directives) > 0 {
		m.propDetailKey = m.directives[0].Key
	} else {
		m.propDetailKey = ""
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
// rowBudgetHeight returns the real terminal height once known (WR-10),
// falling back to the canonical minFrameHeight before the first
// tea.WindowSizeMsg arrives — matching every existing test that drives this
// model directly without ever sending one.
func (m globalSSHModel) rowBudgetHeight() int {
	// WR-02 (09.4-REVIEW.md second independent re-review): floor at
	// minFrameHeight, mirroring globalGitModel.rowBudgetHeight — see that
	// function's doc comment for the full rationale.
	if m.lastHeight >= minFrameHeight {
		return m.lastHeight
	}
	return minFrameHeight
}

func (m globalSSHModel) handleMsg(msg tea.Msg, _ DemoState) keyResult {
	if sz, ok := msg.(tea.WindowSizeMsg); ok {
		m.lastHeight = sz.Height
		return keyResult{model: m}
	}
	token := -1
	var snapshot gitCommitTokenMsg
	if wrapped, ok := msg.(gitCommitTokenMsg); ok {
		token = wrapped.token
		snapshot = wrapped
		msg = wrapped.msg
	}
	// BL-04 (09.4-REVIEW.md independent re-review): this branch used to gate
	// EVERYTHING — including dispatching the ApplySSH reducer action — on
	// m.mode/m.applyCommitPending. Ctrl+P bypasses the screen's own
	// pending-ceremony guard (it is intercepted by App.handleKey before the
	// screen ever sees a key), and re-entering this screen from the palette
	// runs activate(), which CR-02 made reset those very flags. So a
	// still-in-flight commit's message arrived after the flags were already
	// cleared, and the whole branch — including the reducer action — was
	// silently skipped, even though the write had already landed on disk.
	// ceremonyOpen distinguishes "the ceremony UI is still here to receive
	// the receipt" from "the write itself succeeded and App.state must
	// still refresh" — only the former gates ceremony/UI mutation; the
	// reducer action and note fire on every genuine success regardless.
	// CR-01 (second independent re-review): additionally require the
	// message's token to match m.commitRequestToken — a message whose
	// ceremony was abandoned and then superseded by a NEWER ceremony of the
	// same kind must not be treated as belonging to that newer one either
	// (see globalGitModel.commitRequestToken's doc comment for the full
	// rationale; this is the identical mechanism, shared wrapper type).
	if commit, ok := msg.(GlobalSSHCommitMsg); ok {
		// ceremonyOpen is only whether the ceremony UI is still around to
		// receive the receipt — it must NOT gate the reducer action below.
		// m.appliedKeys survives activate() (CR-02 resets mode/pending/
		// ceremony, never appliedKeys), so it is still valid here even after
		// abandonment.
		ceremonyOpen := m.mode == gssApplyCeremony && m.applyCommitPending && token == m.commitRequestToken
		if ceremonyOpen {
			m.applyCommitPending = false
		}
		if commit.Err != "" {
			if ceremonyOpen {
				message := commit.Err
				if len(commit.Restored) > 0 {
					message += " (restored: " + strings.Join(commit.Restored, "; ") + ")"
				}
				m.ceremony = m.ceremony.commitFailed(message)
				return keyResult{model: m}
			}
			// WR-01 (second independent re-review): a background write that
			// fails after its ceremony is gone must still surface somewhere.
			return keyResult{model: m, note: "Background write failed: " + commit.Err}
		}
		// CR-01 (third independent re-review): read the SUBMITTED keys from
		// the message's own snapshot, never m.appliedKeys — a newer
		// same-kind ceremony's confirm overwrites m.appliedKeys before this
		// (possibly stale) message arrives, which would otherwise commit
		// the WRONG key set to App.state attributed to THIS commit's
		// success.
		keys := snapshot.keys
		plural := "s"
		if len(keys) == 1 {
			plural = ""
		}
		if ceremonyOpen {
			m.ceremony = m.ceremony.commitSucceeded(commit.Backups)
			// Append post-write shadow advisories to the receipt (D-04).
			// The ResultExtra field is the right slot: it is rendered directly
			// below ResultMessage on the receipt without requiring a new
			// ceremony field (06-UI-SPEC.md budgets this against the existing
			// field).
			if len(commit.ShadowAdvisories) > 0 {
				m.ceremony = m.ceremony.withResultExtra(strings.Join(commit.ShadowAdvisories, "\n"))
			}
		}
		return keyResult{
			model:   m,
			note:    fmt.Sprintf("%d global SSH option%s applied.", len(keys), plural),
			actions: []Action{ApplySSH{Keys: keys, Backup: firstBackup(commit.Backups)}},
		}
	}
	if commit, ok := msg.(SSHStorageCommitMsg); ok {
		ceremonyOpen := m.mode == gssStorageCeremony && m.storageCommitPending && token == m.commitRequestToken
		if ceremonyOpen {
			m.storageCommitPending = false
		}
		if commit.Err != "" {
			if ceremonyOpen {
				message := commit.Err
				if commit.ConfigChangedSincePreview {
					message = "Configuration changed since the preview was opened — re-open the preview to migrate."
				} else if len(commit.Restored) > 0 {
					message += " (restored: " + strings.Join(commit.Restored, "; ") + ")"
				}
				m.ceremony = m.ceremony.commitFailed(message)
				return keyResult{model: m}
			}
			return keyResult{model: m, note: "Background write failed: " + commit.Err}
		}
		// CR-01 (third independent re-review): read the SUBMITTED layout from
		// the message's own snapshot, never m.storageTargetLayout — a newer
		// storage ceremony's confirm overwrites m.storageTargetLayout before
		// this (possibly stale) message arrives, which would otherwise
		// migrate to the WRONG layout attributed to THIS commit's success.
		layout := snapshot.layout
		if ceremonyOpen {
			m.ceremony = m.ceremony.commitSucceeded(commit.Backups)
		}
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
	// Phase 9.5 plan 09.5-04 (PROP-04): stage 2's async validate+prove
	// answer. Guarded by mode AND the in-flight flag together — the mode
	// check alone would still accept a message after the user has already
	// left stage 2 back to the form or the browser via Esc, and the
	// in-flight flag alone would still accept a message after this SAME
	// stage 2 has already consumed its one answer (there is no token here:
	// unlike the ceremony's commit dispatch, keys are fully inert while
	// customDirectiveProofPending is true, so a second dispatch from the
	// SAME stage 2 visit is structurally impossible).
	if proof, ok := msg.(SSHCustomDirectiveProofMsg); ok {
		if m.mode != gssCustomDirectiveValidate || !m.customDirectiveProofPending {
			return keyResult{model: m}
		}
		m.customDirectiveProofPending = false
		if proof.Err != "" {
			// D-H: an unproven directive fails CLOSED — a transport-level
			// failure renders the failure and does NOT open the ceremony.
			m.customDirectiveValidateErr = proof.Err
			return keyResult{model: m}
		}
		m.customDirectiveProof = proof.Proof
		if !proof.Proof.OK {
			// UnknownName / PreexistingError / a known-name value rejection
			// (D-I) — every one of these STOPS here. renderCustomDirectiveValidate
			// renders the frozen sentence for whichever outcome this is; the
			// ceremony is never opened.
			return keyResult{model: m}
		}
		// OK: true is the ONLY transition that reaches the ceremony (D-H).
		// The plan is fetched synchronously in this SAME handleMsg call — if
		// it errors, the error renders inline on stage 2 and the ceremony is
		// STILL not opened.
		plan, planErr := m.backend.CustomSSHDirectivePlan(m.pendingDirectiveName, m.pendingDirectiveValue)
		if planErr != nil {
			m.customDirectiveValidateErr = planErr.Error()
			return keyResult{model: m}
		}
		m.ceremony = m.customDirectiveCeremonyFor(m.pendingDirectiveName, m.pendingDirectiveValue, plan)
		m.mode = gssCustomDirectiveCeremony
		return keyResult{model: m}
	}
	// Phase 9.5 plan 09.5-04 (PROP-04): the custom-directive write commit —
	// mirrors GlobalSSHCommitMsg's handling immediately above, including
	// the CR-01/CR-02 ceremonyOpen + token guard and the WR-01 background-
	// failure note.
	if commit, ok := msg.(SSHCustomDirectiveCommitMsg); ok {
		ceremonyOpen := m.mode == gssCustomDirectiveCeremony && m.customDirectiveCommitPending && token == m.commitRequestToken
		if ceremonyOpen {
			m.customDirectiveCommitPending = false
		}
		if commit.Err != "" {
			if ceremonyOpen {
				message := commit.Err
				if len(commit.Restored) > 0 {
					message += " (restored: " + strings.Join(commit.Restored, "; ") + ")"
				}
				m.ceremony = m.ceremony.commitFailed(message)
				return keyResult{model: m}
			}
			return keyResult{model: m, note: "Background write failed: " + commit.Err}
		}
		// CR-01 (third independent re-review): read the SUBMITTED name/value
		// from the message's own snapshot, never m.pendingDirectiveName/
		// m.pendingDirectiveValue — a newer same-kind ceremony's confirm
		// overwrites those fields before this (possibly stale) message
		// arrives.
		name, value := snapshot.sshDirectiveName, snapshot.sshDirectiveValue
		if ceremonyOpen {
			m.ceremony = m.ceremony.commitSucceeded(commit.Backups)
			if len(commit.Advisories) > 0 {
				m.ceremony = m.ceremony.withResultExtra(strings.Join(commit.Advisories, "\n"))
			}
		}
		return keyResult{
			model: m,
			note:  fmt.Sprintf(PropsSSHCustomReceiptFmt, name, value),
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

// gssFilteredDirectives returns the "All directives" rows matching the
// current filter value, case-insensitively, against BOTH the key and the
// value (a user filtering for a path fragment expects the value to be
// searched too). Every consumer — render, the up/down handler, and the
// click hit-test — reads this SAME function, so the scroll window and the
// click row index can never disagree about which row is where.
func (m globalSSHModel) gssFilteredDirectives() []SSHDirectiveView {
	q := strings.ToLower(m.filter.Value())
	out := make([]SSHDirectiveView, 0, len(m.directives))
	for _, d := range m.directives {
		if strings.Contains(strings.ToLower(d.Key), q) || strings.Contains(strings.ToLower(d.Value), q) {
			out = append(out, d)
		}
	}
	return out
}

// gssPropertiesDetailIndex resolves the selected directive row's index
// within filtered — the properties-sub-tab mirror of detailIndex above.
func (m globalSSHModel) gssPropertiesDetailIndex(filtered []SSHDirectiveView) int {
	for i, d := range filtered {
		if d.Key == m.propDetailKey {
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

// customDirectiveCeremonyFor builds the custom-directive write ceremony
// (Phase 9.5 plan 09.5-04, PROP-04) from an ALREADY-VALIDATED plan view —
// the caller (handleMsg's SSHCustomDirectiveProofMsg branch) has already
// received an OK: true staged-config proof AND called
// m.backend.CustomSSHDirectivePlan(name, value) successfully, so this
// helper never itself errors. Reuses ceremonyModel UNMODIFIED, mirroring
// globalgit.go's customKeyCeremonyFor exactly: the SAME Async: true,
// non-destructive apply shape GSSH-01/GGIT-01 already use — never the
// typed-confirm escalation reserved for FIX-01's existing-value rewrite.
func (m globalSSHModel) customDirectiveCeremonyFor(name, value string, plan SSHCustomDirectivePlanView) ceremonyModel {
	targets := plan.Targets
	if len(targets) == 0 {
		// The backend's plan always resolves the storage target; this
		// fallback keeps the ceremony buildable for a stub that answers an
		// empty plan.
		targets = []string{"~/.ssh/config"}
	}
	backups := plan.Backups
	resolvedTarget := targets[0]
	preview := plan.Diff
	if preview == "" {
		preview = "+ " + name + " " + value
	}
	return newCeremony(ceremonyConfig{
		Heading:       fmt.Sprintf(PropsSSHCustomCeremonyHeadingFmt, resolvedTarget),
		Targets:       targets,
		Backups:       backups,
		Preview:       preview,
		PreviewDiff:   true,
		ConfirmLabel:  "Write",
		ResultMessage: fmt.Sprintf(PropsSSHCustomReceiptFmt, name, value),
		Async:         true,
	})
}

// gssNextSubTab returns the sub-tab the → key cycles to: Options → Storage &
// preview → All directives → Options (09.5-UI-SPEC.md).
func gssNextSubTab(cur gssSubTab) gssSubTab {
	switch cur {
	case gssOptions:
		return gssStorage
	case gssStorage:
		return gssProperties
	default: // gssProperties
		return gssOptions
	}
}

// gssPrevSubTab returns the sub-tab the ← key cycles to — the OPPOSITE
// direction of gssNextSubTab, not an alias of it. With only two sub-tabs the
// two directions were indistinguishable (both toggled the same pair); with
// three they are not, and treating them as aliases was the exact bug this
// function fixes.
func gssPrevSubTab(cur gssSubTab) gssSubTab {
	switch cur {
	case gssOptions:
		return gssProperties
	case gssProperties:
		return gssStorage
	default: // gssStorage
		return gssOptions
	}
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
			m.commitRequestToken++
			token := m.commitRequestToken
			keys := m.applyChosen(m.overlaidOptions(s))
			m.appliedKeys = keys
			m.applyCommitPending = true
			cmd := m.backend.CommitGlobalSSH(keys)
			snapshot := gitCommitTokenMsg{keys: keys}
			return keyResult{model: m, handled: true, cmd: wrapGitCommitToken(token, cmd, snapshot)}
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
			m.commitRequestToken++
			token := m.commitRequestToken
			m.storageTargetLayout = m.storageChoice
			m.storageCommitPending = true
			cmd := m.backend.CommitSSHStorage(m.storageChoice, m.storageView.PlanToken)
			snapshot := gitCommitTokenMsg{layout: m.storageChoice}
			return keyResult{model: m, handled: true, cmd: wrapGitCommitToken(token, cmd, snapshot)}
		case ceremonyFinished:
			m.mode = gssBrowse
		case ceremonyNone:
		}
		return keyResult{model: m, handled: true}
	}

	// Phase 9.5 plan 09.5-04 (PROP-04) stage 3: the custom-directive
	// ceremony's key routing — mirrors the two ceremony blocks above
	// exactly, dispatching CommitCustomSSHDirective through the SAME
	// commitRequestToken/wrapGitCommitToken machinery (no parallel
	// completion path).
	if m.mode == gssCustomDirectiveCeremony {
		if m.customDirectiveCommitPending {
			// In flight: keys are inert until the commit result arrives.
			return keyResult{model: m, handled: true}
		}
		var outcome ceremonyOutcome
		m.ceremony, outcome = m.ceremony.handleKey(msg)
		switch outcome {
		case ceremonyCancelled:
			m.mode = gssBrowse
		case ceremonyConfirmed:
			m.commitRequestToken++
			token := m.commitRequestToken
			name := m.pendingDirectiveName
			value := m.pendingDirectiveValue
			m.customDirectiveCommitPending = true
			cmd := m.backend.CommitCustomSSHDirective(name, value)
			snapshot := gitCommitTokenMsg{sshDirectiveName: name, sshDirectiveValue: value}
			return keyResult{model: m, handled: true, cmd: wrapGitCommitToken(token, cmd, snapshot)}
		case ceremonyFinished:
			m.mode = gssBrowse
		case ceremonyNone:
		}
		return keyResult{model: m, handled: true}
	}

	// Phase 9.5 plan 09.5-04 (PROP-04) stage 1: while the custom-directive
	// form is open, it owns the keyboard exactly like plan 09.5-03's Git
	// custom-key form (globalgit.go) — Tab moves focus between the two
	// fields, Enter on the VALUE field (focus index 1) submits, Esc closes
	// the form without submitting, and every other key routes through
	// updateInput to the focused field. Enter on the NAME field (focus
	// index 0) does NOT submit — it only exists as a Tab target.
	if m.mode == gssCustomDirectiveForm {
		switch key {
		case "esc":
			m.mode = gssBrowse
			m.customDirectiveNameInput.Blur()
			m.customDirectiveValueInput.Blur()
			return keyResult{model: m, handled: true}
		case "tab":
			m.customDirectiveFieldFocus = 1 - m.customDirectiveFieldFocus
			m.focusCustomDirectiveField()
			return keyResult{model: m, handled: true}
		case "enter":
			if m.customDirectiveFieldFocus != 1 {
				m.customDirectiveFieldFocus = 1
				m.focusCustomDirectiveField()
				return keyResult{model: m, handled: true}
			}
			name := m.customDirectiveNameInput.Value()
			value := m.customDirectiveValueInput.Value()
			m.pendingDirectiveName = name
			m.pendingDirectiveValue = value
			m.customDirectiveProofPending = true
			m.customDirectiveValidateErr = ""
			m.customDirectiveProof = SSHDirectiveProofView{}
			m.mode = gssCustomDirectiveValidate
			m.customDirectiveNameInput.Blur()
			m.customDirectiveValueInput.Blur()
			cmd := m.backend.ValidateCustomSSHDirective(name, value)
			return keyResult{model: m, handled: true, cmd: cmd}
		default:
			if m.customDirectiveFieldFocus == 0 {
				m.customDirectiveNameInput, _ = updateInput(m.customDirectiveNameInput, msg)
			} else {
				m.customDirectiveValueInput, _ = updateInput(m.customDirectiveValueInput, msg)
			}
			return keyResult{model: m, handled: true}
		}
	}

	// Phase 9.5 plan 09.5-04 (PROP-04) stage 2: while validating/proving,
	// every key is inert during the in-flight probe (matching every other
	// in-flight state on this screen); once stopped at one of D-I's
	// stopping outcomes, Esc returns to the browser — there is NO key that
	// reaches the ceremony from here (D-H's un-skippable gate).
	if m.mode == gssCustomDirectiveValidate {
		if m.customDirectiveProofPending {
			return keyResult{model: m, handled: true}
		}
		if key == "esc" {
			m.mode = gssBrowse
			return keyResult{model: m, handled: true}
		}
		return keyResult{model: m, handled: true}
	}

	// D-B: while the properties filter is focused, it owns the keyboard —
	// every key but esc routes into the input and is reported handled,
	// mirroring globalGitModel.fieldEditing's identical capture contract
	// (esc blurs WITHOUT clearing; clearing is a separate, explicit action
	// no other gitid text field conflates with blur either).
	if m.subTab == gssProperties && m.filterFocused {
		if key == "esc" {
			m.filterFocused = false
			m.filter.Blur()
			return keyResult{model: m, handled: true}
		}
		before := m.filter.Value()
		m.filter, _ = updateInput(m.filter, msg)
		if m.filter.Value() != before {
			// D-C: on filter-text change, reset the selection to the FIRST
			// row of the newly filtered set — a selected-but-invisible row
			// must be structurally impossible, not defended against.
			filtered := m.gssFilteredDirectives()
			m.propDetailKey = ""
			if len(filtered) > 0 {
				m.propDetailKey = filtered[0].Key
			}
			m.listWindowStart = 0
		}
		return keyResult{model: m, handled: true}
	}

	options := m.overlaidOptions(s)
	if m.subTab == gssOptions && m.optionsErr != "" {
		// WR-12 (09.4-REVIEW.md independent re-review): align with Global
		// Git's error-state key contract — a failed probe is fully
		// fail-open (no ←/→ special case either), never consuming
		// navigation keys on a screen that is least able to help.
		return keyResult{model: m}
	}
	if m.subTab == gssProperties && m.directivesErr != "" {
		// Fail-open, mirroring the Options sub-tab's own probe-failure
		// contract immediately above: every navigation key still reaches
		// the app globals on the screen least able to help.
		return keyResult{model: m}
	}
	if m.subTab == gssOptions && len(options) == 0 {
		// Advisory / fail-open: no rows to act on, but navigation must still
		// reach the globals (tabs, ?, q) — never trap the user on this
		// screen, matching Global Git's fail-open contract for the same
		// class of failed/empty probe (07-UI-SPEC.md RESOLVED "error" row).
		switch key {
		case "left", "right":
			if key == "right" {
				m.subTab = gssNextSubTab(m.subTab)
			} else {
				m.subTab = gssPrevSubTab(m.subTab)
			}
			if m.subTab == gssStorage {
				m.storageChoice = s.SSHStorage
				m = m.refetchStoragePlan()
			}
			return keyResult{model: m, handled: true}
		}
		return keyResult{model: m}
	}
	switch key {
	case "left", "right":
		if key == "right" {
			m.subTab = gssNextSubTab(m.subTab)
		} else {
			m.subTab = gssPrevSubTab(m.subTab)
		}
		if m.subTab == gssStorage {
			m.storageChoice = s.SSHStorage
			m = m.refetchStoragePlan()
		}
		return keyResult{model: m, handled: true}
	case "up", "down":
		switch m.subTab {
		case gssOptions:
			idx := m.detailIndex(options)
			if key == "down" && idx < len(options)-1 {
				idx++
			}
			if key == "up" && idx > 0 {
				idx--
			}
			m.detailKey = options[idx].Key
			m.listWindowStart = scrollWindowFor(m.listWindowStart, idx, gssVisibleRowCount(len(options), m.rowBudgetHeight(), s))
		case gssStorage:
			if m.storageChoice == StorageSentinel {
				m.storageChoice = StorageInclude
			} else {
				m.storageChoice = StorageSentinel
			}
			// Refetch the storage view for the newly selected layout.
			m = m.refetchStoragePlan()
		case gssProperties:
			filtered := m.gssFilteredDirectives()
			if len(filtered) > 0 {
				idx := m.gssPropertiesDetailIndex(filtered)
				if key == "down" && idx < len(filtered)-1 {
					idx++
				}
				if key == "up" && idx > 0 {
					idx--
				}
				m.propDetailKey = filtered[idx].Key
				m.listWindowStart = scrollWindowFor(m.listWindowStart, idx, gssPropertiesVisibleRowCount(len(filtered), m.rowBudgetHeight()))
			}
		}
		return keyResult{model: m, handled: true}
	case "/":
		if m.subTab != gssProperties || m.directivesErr != "" {
			return keyResult{model: m}
		}
		m.filterFocused = true
		m.filter.Focus()
		return keyResult{model: m, handled: true}
	case "n":
		// Phase 9.5 plan 09.5-04 (PROP-04): opens stage 1's custom-directive
		// form — mirrors the "/" filter case immediately above, gated to
		// the SAME sub-tab and fail-closed on a failed directives probe.
		if m.subTab != gssProperties || m.directivesErr != "" {
			return keyResult{model: m}
		}
		m.mode = gssCustomDirectiveForm
		m.customDirectiveFieldFocus = 0
		m.customDirectiveNameInput = newTextInput("")
		m.customDirectiveValueInput = newTextInput("")
		m.focusCustomDirectiveField()
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

// gssSubTabStripRows is a one-line alias delegating to the shared
// subTabStripRows() (frame.go, D-D, 09.5-02 extraction) — kept so Phase
// 9.4's existing TestSubTabStrip* contract's call surface never changes.
// This is the single source of truth for the strip's height; all consumers
// (renderOptions, renderStorage, gssOptionsTopLines, handleClick) must derive
// from this.
func gssSubTabStripRows() int {
	return subTabStripRows()
}

// subTabStrip renders the [Options] [Storage & preview] [All directives]
// strip with a border, via the shared renderSubTabStrip (frame.go, D-D,
// 09.5-02 extraction) — Global SSH supplies its three labels and its active
// index; the border/style/composition logic lives in exactly one place.
func (m globalSSHModel) subTabStrip() string {
	return renderSubTabStrip([]string{gssTabOptionsLabel, gssTabStorageLabel, gssTabPropertiesLabel}, int(m.subTab))
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

// gssVisibleRowCount computes how many option rows fit inside the body
// budget WITHOUT overflowing — the Global SSH mirror of gitVisibleRowCount
// (globalgit.go), using gssOptionsTopLines instead of gitTopLines for the
// screen's own chrome (strip + optional findings banner). height is the
// caller's rowBudgetHeight() — see gitVisibleRowCount's doc comment (WR-10,
// 09.4-REVIEW.md independent re-review) for the full rationale: a MEASURED
// budget off the REAL terminal height (falling back to the canonical fixed
// frame height before the first resize), one line reserved for the scroll
// cue only when the row set does not already fit.
func gssVisibleRowCount(totalRows, height int, s DemoState) int {
	budget := frameBodyRows(height) - gssOptionsTopLines(s)
	if budget < 1 {
		budget = 1
	}
	if totalRows*optionRowLines <= budget {
		return totalRows
	}
	reserved := budget - 1
	if reserved < 0 {
		reserved = 0
	}
	visible := reserved / optionRowLines
	if visible < 1 {
		visible = 1
	}
	if visible > totalRows {
		visible = totalRows
	}
	return visible
}

// gssComputeScrollWindow derives the current scroll window from the model's
// listWindowStart and the live option count — the Global SSH mirror of
// gitComputeScrollWindow (globalgit.go), sharing the SAME gitScrollWindow
// type/gitCueDirection/gitCueLine machinery so the two screens' scrolling
// behavior can never silently drift apart.
func (m globalSSHModel) gssComputeScrollWindow(totalRows int, s DemoState) gitScrollWindow {
	visible := gssVisibleRowCount(totalRows, m.rowBudgetHeight(), s)
	needsScroll := visible < totalRows
	windowStart := m.listWindowStart
	if windowStart < 0 {
		windowStart = 0
	}
	maxStart := totalRows - visible
	if maxStart < 0 {
		maxStart = 0
	}
	if windowStart > maxStart {
		windowStart = maxStart
	}
	w := gitScrollWindow{needsScroll: needsScroll, windowStart: windowStart, visibleRows: visible}
	if !needsScroll {
		return w
	}
	hiddenBelow := windowStart+visible < totalRows
	if hiddenBelow {
		w.cue = gitCueDown
		w.hiddenCount = totalRows - (windowStart + visible)
		return w
	}
	w.cue = gitCueUp
	w.hiddenCount = windowStart
	return w
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
		// WR-01: the label row is derived from gssSubTabStripRows (not a
		// hardcoded 1) and the label spans are read from the ACTUAL rendered
		// line via hitNeedle/ansi.StringWidth (not `len(...)` byte counts and
		// a hand-computed "┊ " prefix offset), so the click zones can never
		// drift from the border layout subTabStrip() actually draws — the
		// same guarantee every other click zone in this file already has.
		// Border rows (0 and stripRows-1) stay inert.
		if y == stripRows/2 {
			body := m.view(s, width, height).body
			switch {
			case hitNeedle(body, x, y, gssTabOptionsLabel):
				m.subTab = gssOptions
				return keyResult{model: m, handled: true}
			case hitNeedle(body, x, y, gssTabStorageLabel):
				m.subTab = gssStorage
				m.storageChoice = s.SSHStorage
				m = m.refetchStoragePlan()
				return keyResult{model: m, handled: true}
			case hitNeedle(body, x, y, gssTabPropertiesLabel):
				m.subTab = gssProperties
				return keyResult{model: m, handled: true}
			}
		}
		// Border rows (0 and stripRows-1) and any other click in the strip area are inert.
		return keyResult{model: m}
	}
	if m.subTab == gssStorage {
		return m.handleStorageClick(x, y, width, height, s)
	}
	if m.subTab == gssProperties {
		return m.handlePropertiesClick(x, y, width, height)
	}
	if x >= masterListWidth(width) || y < gssOptionsTopLines(s) {
		return keyResult{model: m}
	}
	options := m.overlaidOptions(s)
	// WR-09: the click's row index is computed from the WINDOW START via
	// the scroll window (not from the screen position alone), mirroring
	// Global Git's own handleClick — a scroll offset the click handler
	// doesn't know about would otherwise silently toggle the wrong row.
	w := m.gssComputeScrollWindow(len(options), s)
	row, ok := gitRowForScreenRow(w, y-gssOptionsTopLines(s))
	if !ok || row >= len(options) {
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

// handlePropertiesClick resolves "All directives" clicks: a click on the
// filter row focuses it; a click on a master-list row selects it — reading
// the SAME gssFilteredDirectives() slice the render and the up/down handler
// use, so the click row index can never disagree about which row is where
// (the WR-09 defect class this codebase has already been bitten by).
func (m globalSSHModel) handlePropertiesClick(x, y, width, height int) keyResult {
	if m.directivesErr != "" {
		return keyResult{model: m}
	}
	if y == gssSubTabStripRows() {
		m.filterFocused = true
		m.filter.Focus()
		return keyResult{model: m, handled: true}
	}
	if x >= masterListWidth(width) || y < gssPropertiesTopLines() {
		return keyResult{model: m}
	}
	filtered := m.gssFilteredDirectives()
	w := m.gssPropertiesComputeScrollWindow(len(filtered), height)
	row, ok := gssPropertiesRowForScreenRow(w, y-gssPropertiesTopLines())
	if !ok || row >= len(filtered) {
		return keyResult{model: m}
	}
	m.propDetailKey = filtered[row].Key
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
	case GlobalSSHNeedsAction:
		// Only needs-action renders the warning glyph (D-05: the `!` is triggered
		// ONLY by the needs-action state). A differs row renders plain — the
		// second line carries the explanation (D-06).
		tone = styleWarning.Render("!")
	case GlobalSSHDiffers:
		// Differs renders without the warning glyph (D-06). Use the faint
		// neutral marker already established by the not-applicable branch,
		// keeping both renders speaking the same visual language for
		// "inert, not alarming" (D-05/D-06/D-07).
		tone = styleFaint.Render("·")
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
	switch m.subTab {
	case gssStorage:
		crumb = PropsSSHStorageSubTabLabel
	case gssProperties:
		crumb = PropsSSHSubTabLabel
	}

	var body string
	var actions []FooterAction
	capturesKeys := false
	switch m.mode {
	case gssApplyCeremony, gssStorageCeremony, gssCustomDirectiveCeremony:
		// WR-02: the crumb line above the body already reads "Options" or
		// "Storage & preview" (crumb, set above regardless of mode), so
		// re-rendering the 3-row bordered strip inside the ceremony body
		// would be redundant chrome eating into the ceremony's already-tight
		// row budget (minFrameHeight leaves ~5 spare rows for the apply/
		// storage ceremony; the strip alone consumed 3 of them). The
		// custom-directive ceremony (Phase 9.5 plan 09.5-04, PROP-04)
		// shares this SAME branch — m.subTab stays gssProperties throughout
		// stages 1-3, so crumb (set above, unconditionally on m.subTab)
		// already reads "All directives" correctly here too.
		body = m.ceremony.view(width - 2)
		actions = ceremonyFooterActions()
		capturesKeys = true // the ceremony consumes every plain key
	case gssCustomDirectiveForm:
		// Phase 9.5 plan 09.5-04 (PROP-04) stage 1.
		body = m.renderCustomDirectiveForm(width)
		actions = []FooterAction{
			{Key: "Tab", Label: "next field"},
			{Key: "Enter", Label: "check & write"},
			{Key: "Esc", Label: "cancel"},
		}
		capturesKeys = true
	case gssCustomDirectiveValidate:
		// Phase 9.5 plan 09.5-04 (PROP-04) stage 2 — the ONLY genuinely new
		// render this plan adds.
		body = m.renderCustomDirectiveValidate(width)
		actions = []FooterAction{{Key: "Esc", Label: "back"}}
		capturesKeys = true
	case gssBrowse:
		switch m.subTab {
		case gssOptions:
			if m.optionsErr != "" {
				// Advisory posture extends to the detection layer: the pane
				// renders the error note, never a blank body. WR-12
				// (09.4-REVIEW.md independent re-review): mirror Global
				// Git's error branch, which keeps the doctor findings
				// banner above the warning — this screen's probe-failure
				// branch previously dropped it exactly when the user is
				// least able to help themselves.
				body = m.subTabStrip() + "\n"
				if banner := findingsBanner(s, "SSH", gssBannerBeyond); banner != "" {
					body += " " + banner + "\n"
				}
				body += " " + styleWarning.Render("! "+m.optionsErr) + "\n\n " +
					styleFaint.Render("The option states could not be read from this machine.")
				actions = []FooterAction{{Key: "←→", Label: gssFooterCycleLabel}}
			} else {
				body = m.renderOptions(s, options, width, height)
				actions = []FooterAction{
					{Key: "↑↓", Label: "select option"},
					{Key: "←→", Label: gssFooterCycleLabel},
					{Key: "space", Label: "toggle"},
				}
				if len(chosen) > 0 {
					actions = append(actions, FooterAction{Key: "a", Label: fmt.Sprintf("apply %d selected", len(chosen))})
				}
			}
		case gssStorage:
			body = m.renderStorage(s, width, height)
			actions = []FooterAction{
				{Key: "←→", Label: gssFooterCycleLabel},
				{Key: "↑↓", Label: "layout"},
			}
			if m.storageChoice != s.SSHStorage && m.storageViewErr == "" {
				actions = append(actions, FooterAction{Key: "Enter", Label: "migrate layout…"})
			}
		case gssProperties:
			body = m.renderProperties(width, height)
			actions = []FooterAction{
				{Key: "←→", Label: gssFooterCycleLabel},
			}
			if m.directivesErr == "" {
				actions = append(actions,
					FooterAction{Key: "↑↓", Label: "select"},
					FooterAction{Key: "/", Label: "filter"},
					FooterAction{Key: "n", Label: PropsAddCustomDirectiveLabel},
				)
			}
			capturesKeys = m.filterFocused
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

	selIdx := m.detailIndex(options)
	// WR-09: the master list becomes a scrolling window over the row set,
	// mirroring Global Git's own view (globalgit.go) — when every row fits
	// inside the computed budget, scrollWin.needsScroll is false and the
	// loop below covers every row exactly as before (byte-identical, zero
	// regression); when it does not fit, exactly [windowStart,
	// windowStart+visibleRows) renders plus one reserved cue line.
	scrollWin := m.gssComputeScrollWindow(len(options), s)
	visibleOptions := options
	if scrollWin.needsScroll {
		visibleOptions = options[scrollWin.windowStart : scrollWin.windowStart+scrollWin.visibleRows]
	}
	var listRows []string
	if scrollWin.cue == gitCueUp {
		listRows = append(listRows, gitCueLine(scrollWin))
	}
	for i, o := range visibleOptions {
		absoluteIdx := scrollWin.windowStart + i
		listRows = append(listRows, optionRow(o.GlobalSSHOptionView, m.chosen[o.Key], absoluteIdx == selIdx, o.applied, listWidth))
	}
	if scrollWin.cue == gitCueDown {
		listRows = append(listRows, gitCueLine(scrollWin))
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

// gssPropertiesTopLines counts the body lines rendered above the first
// directive row on the "All directives" sub-tab: the sub-tab strip plus the
// ONE filter row. Task 2 decided NOT to render the SSH findings banner here
// (unlike gssOptionsTopLines) — this sub-tab already shows the FULL resolved
// directive set, so a "doctor found N findings beyond these options" framing
// would be self-contradictory (recorded in 09.5-01-SUMMARY.md).
func gssPropertiesTopLines() int {
	return gssSubTabStripRows() + 1
}

// propKeyColumnWidth is the master-list row's key-column display width —
// pubkeyacceptedalgorithms-class long keys simply push the value right of
// this column rather than being clipped themselves; truncLine's visible cue
// on the WHOLE line is what protects the fixed row width.
const propKeyColumnWidth = 26

// gssPropertiesVisibleRowCount computes how many directive rows fit inside
// the body budget WITHOUT overflowing — the properties-sub-tab mirror of
// gssVisibleRowCount, but dividing by ONE line per row (this list has no
// toggle/apply affordance and needs no optionRow-style 2-line sub-budget).
func gssPropertiesVisibleRowCount(totalRows, height int) int {
	budget := frameBodyRows(height) - gssPropertiesTopLines()
	if budget < 1 {
		budget = 1
	}
	if totalRows <= budget {
		return totalRows
	}
	reserved := budget - 1 // one line reserved for the scroll cue
	if reserved < 1 {
		reserved = 1
	}
	if reserved > totalRows {
		reserved = totalRows
	}
	return reserved
}

// gssPropertiesComputeScrollWindow derives the current scroll window from
// the model's listWindowStart and the FILTERED row count — the properties
// mirror of gssComputeScrollWindow, sharing the SAME gitScrollWindow type so
// this screen's THIRD list can never silently drift from the other two's
// scrolling behavior.
func (m globalSSHModel) gssPropertiesComputeScrollWindow(totalRows, height int) gitScrollWindow {
	visible := gssPropertiesVisibleRowCount(totalRows, height)
	needsScroll := visible < totalRows
	windowStart := m.listWindowStart
	if windowStart < 0 {
		windowStart = 0
	}
	maxStart := totalRows - visible
	if maxStart < 0 {
		maxStart = 0
	}
	if windowStart > maxStart {
		windowStart = maxStart
	}
	w := gitScrollWindow{needsScroll: needsScroll, windowStart: windowStart, visibleRows: visible}
	if !needsScroll {
		return w
	}
	hiddenBelow := windowStart+visible < totalRows
	if hiddenBelow {
		w.cue = gitCueDown
		w.hiddenCount = totalRows - (windowStart + visible)
		return w
	}
	w.cue = gitCueUp
	w.hiddenCount = windowStart
	return w
}

// gssPropertiesRowForScreenRow maps a body-relative screen row (y -
// gssPropertiesTopLines()) to the directive-row index the user is pointing
// at, honoring the scroll window and treating the reserved cue line as
// inert — the ONE-line-per-row mirror of gitRowForScreenRow, which assumes
// optionRow's 2-line shape and therefore cannot be reused verbatim here.
func gssPropertiesRowForScreenRow(w gitScrollWindow, y int) (idx int, ok bool) {
	if !w.needsScroll {
		return w.windowStart + y, y >= 0 && y < w.visibleRows
	}
	switch w.cue {
	case gitCueUp:
		if y == 0 {
			return 0, false // the cue line itself
		}
		row := y - 1
		return w.windowStart + row, row >= 0 && row < w.visibleRows
	default: // gitCueDown
		return w.windowStart + y, y >= 0 && y < w.visibleRows
	}
}

// propertiesFilterRow renders the filter input's live value on the left and
// the right-aligned match count on the SAME line (09.5-UI-SPEC.md: exactly
// ONE row — no separate result-count line).
func (m globalSSHModel) propertiesFilterRow(width, matchCount int) string {
	left := " " + m.filter.View()
	right := fmt.Sprintf(PropsMatchCountFmt, matchCount, len(m.directives))
	pad := width - ansi.StringWidth(left) - ansi.StringWidth(right) - 1
	if pad < 1 {
		pad = 1
	}
	return ansi.Truncate(left+strings.Repeat(" ", pad)+styleFaint.Render(right), width, "")
}

// propertyRow renders one master-list "All directives" row: a leading
// space, the key (padded for column alignment), then the value — ONE line
// (not optionRow's 2-line shape: this list has no toggle/apply affordance
// and needs no sub-line budget), truncated with a visible cue.
func propertyRow(d SSHDirectiveView, selected bool, width int) string {
	key := styleBold.Render(d.Key)
	if selected {
		key = styleSelected.Render(d.Key)
	}
	marker := "  "
	if selected {
		marker = styleBold.Render("▸ ")
	}
	line := " " + marker + padDisplay(key, propKeyColumnWidth) + d.Value
	return truncLine(line, width)
}

// renderProperties renders the "All directives" sub-tab's flat, filterable
// master-detail body (PROP-01): the filter row with its live match count,
// the one-line-per-directive master list (reusing gssPropertiesComputeScrollWindow/
// gitCueLine), the detail pane (full key, full unclipped value, the frozen
// source line, and — only when PolicyBacked — the cross-reference note),
// and the probe-failure / filter-zero-match / zero-rows empty states.
func (m globalSSHModel) renderProperties(width, height int) string {
	if m.directivesErr != "" {
		return m.subTabStrip() + "\n " +
			styleWarning.Render(PropsSSHProbeFailedHeading) + "\n\n " +
			styleFaint.Render(PropsSSHProbeFailedBody)
	}

	filtered := m.gssFilteredDirectives()
	body := m.subTabStrip() + "\n" + m.propertiesFilterRow(width, len(filtered)) + "\n"

	// WR-17-class guard (mirrors renderOptions): a Backend implementation
	// may legitimately return (nil, nil) — zero rows, no error. This is a
	// DIFFERENT, non-alarming state from a filter matching nothing below.
	if len(m.directives) == 0 {
		return body + " " + styleFaint.Render("No SSH directives to show.")
	}
	if len(filtered) == 0 {
		return body + " " + styleFaint.Render(fmt.Sprintf(PropsSSHNoFilterMatchFmt, m.filter.Value()))
	}

	listWidth := masterListWidth(width)
	detailWidth := width - listWidth - masterDetailGutter
	rows := frameBodyRows(height) - gssPropertiesTopLines()

	selIdx := m.gssPropertiesDetailIndex(filtered)
	w := m.gssPropertiesComputeScrollWindow(len(filtered), height)
	visible := filtered
	if w.needsScroll {
		visible = filtered[w.windowStart : w.windowStart+w.visibleRows]
	}
	var listRows []string
	if w.cue == gitCueUp {
		listRows = append(listRows, gitCueLine(w))
	}
	for i, d := range visible {
		absoluteIdx := w.windowStart + i
		listRows = append(listRows, propertyRow(d, absoluteIdx == selIdx, listWidth))
	}
	if w.cue == gitCueDown {
		listRows = append(listRows, gitCueLine(w))
	}
	list := strings.Join(listRows, "\n")

	detail := filtered[selIdx]
	var d strings.Builder
	d.WriteString(" " + styleBold.Render(detail.Key) + "\n\n")
	d.WriteString(lipgloss.NewStyle().Width(detailWidth).Render(" "+detail.Value) + "\n\n")
	d.WriteString(" " + styleFaint.Render(PropsSSHSourceLine) + "\n")
	if detail.PolicyBacked {
		// Informational only: never a second interactive affordance, and the
		// value is never rendered twice side-by-side (09.5-CONTEXT.md).
		d.WriteString(" " + styleFaint.Render(PropsCrossReferenceNote) + "\n")
	}
	detailPane := fitPane(lipgloss.NewStyle().Width(detailWidth).Render(d.String()), rows)

	return body + joinMasterDetail(list, listWidth, detailPane, rows)
}

// renderCustomDirectiveForm renders stage 1's 2-field name/value entry form
// (Phase 9.5 plan 09.5-04, PROP-04) — mirrors globalgit.go's
// renderCustomKeyForm exactly, reusing the SAME gitFallbackFieldLine
// row renderer.
func (m globalSSHModel) renderCustomDirectiveForm(width int) string {
	var d strings.Builder
	d.WriteString(m.subTabStrip() + "\n")
	d.WriteString(" " + styleBold.Render(PropsAddCustomDirectiveLabel) + "\n\n")
	d.WriteString(gitFallbackFieldLine("Directive", m.customDirectiveNameInput, m.customDirectiveFieldFocus == 0, false) + "\n")
	d.WriteString(gitFallbackFieldLine("Value", m.customDirectiveValueInput, m.customDirectiveFieldFocus == 1, false) + "\n")
	return lipgloss.NewStyle().Width(width).Render(d.String())
}

// customDirectiveOutputMaxLines bounds proof.Output's rendered height so a
// pathological multi-line ssh -G diagnostic can never overrun the canonical
// 30-row frame the whole visual-regression gate is pinned to (WR-14).
const customDirectiveOutputMaxLines = 8

// sanitizeProofOutput strips every C0 control byte (and DEL) OTHER THAN
// newline from a verbatim ssh subprocess output, then bounds its line count
// through fitPane's existing clip-with-cue contract (WR-14). proof.Output is
// untrusted terminal content — unlike PreviewBlock (used for the command
// line one row above), it has no line budget and no control-character
// filtering of its own, so both are applied here, at the ONE call site that
// renders it, before it ever reaches the frame.
func sanitizeProofOutput(output string) string {
	var b strings.Builder
	for _, r := range output {
		if r == '\n' || (r >= 0x20 && r != 0x7f) {
			b.WriteRune(r)
		}
	}
	return fitPane(b.String(), customDirectiveOutputMaxLines)
}

// SanitizeDisplayValue strips control bytes/runes from an externally
// sourced value (a raw `git config` or `ssh -G` resolved value) before it is
// interpolated into a single terminal row (WR-02, 09.5-REVIEW.md round 3).
// The two new properties browsers (setKeyRow/propertyRow, and their detail
// panes) render machine-config values that gitid never wrote and never
// validated — unlike a custom key/directive VALUE, which is rejected at
// write time for control characters, an EXISTING value already set on the
// machine (by another tool, or by hand) can legitimately carry raw ANSI
// escapes or embedded newlines, and `git config --show-origin --list -z`
// faithfully returns them verbatim.
//
// Unlike sanitizeProofOutput — which deliberately PRESERVES '\n' because the
// SSH custom-directive proof pane is genuinely multi-line — every consumer
// of this function renders exactly ONE terminal row per list item; the
// master-list row-budgeting arithmetic
// (ggitSetKeysComputeScrollWindow/gssPropertiesRowForScreenRow et al.)
// assumes one screen row per entry, so a raw '\n' surviving into that row
// shifts every subsequent row's hit-test/scroll math and can push the tail
// of the list past RenderFrame's bodyHeight truncation; a raw ANSI escape
// (CSI, OSC, …) reaches the real terminal verbatim. This strips every C0
// control byte (0x00-0x1F, including '\n' and TAB), DEL (0x7F), AND the C1
// control range (U+0080-U+009F) — a strictly tighter filter than
// sanitizeProofOutput's (IN-02 in the same review round noted
// sanitizeProofOutput lets C1 through; this function has no established
// multi-line contract to preserve, so it starts tighter).
func SanitizeDisplayValue(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r < 0x20 || r == 0x7f || (r >= 0x80 && r <= 0x9f) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// renderCustomDirectiveValidate renders stage 2 — the ONLY genuinely new
// render this plan adds (Phase 9.5 plan 09.5-04, PROP-04). Renders TWO
// sequential beats from the ONE staged-config proof (D-I): the name-check
// outcome first, then — only when the name passed — the ssh -G proof
// showing the exact command and its real output, through the SAME "shown ==
// run" render shape identities.go's create-flow test stages use
// (PreviewBlock("$ "+cmd) followed by the real output as faint evidence).
// An OK: true proof never reaches this function — handleMsg transitions
// m.mode to gssCustomDirectiveCeremony in the SAME call that receives it, so
// this render only ever needs to cover the in-flight state and D-I's four
// stopping outcomes (transport error, UnknownName, PreexistingError, a
// known-name value rejection).
func (m globalSSHModel) renderCustomDirectiveValidate(width int) string {
	var d strings.Builder
	d.WriteString(m.subTabStrip() + "\n")
	d.WriteString(" " + styleBold.Render(PropsAddCustomDirectiveLabel) + "\n\n")
	if m.customDirectiveProofPending {
		// WR-14 round 2: this beat renders BEFORE ValidateCustomSSHDirective's
		// proof comes back — i.e. before ValidateDirectiveName has run on
		// m.pendingDirectiveName — so it is the ONE interpolation in this
		// render that is reached with UNVALIDATED, raw user-typed text. Every
		// other use of m.pendingDirectiveName below is reached only after a
		// proof already validated the name; this one is not, so it goes
		// through the same sanitizer WR-14 gave proof.Output.
		d.WriteString(" " + fmt.Sprintf(PropsSSHNameCheckFmt, sanitizeProofOutput(m.pendingDirectiveName)) + "\n")
		return lipgloss.NewStyle().Width(width).Render(d.String())
	}
	if m.customDirectiveValidateErr != "" {
		// A transport-level failure — the probe itself could not run. D-H:
		// an unproven directive fails CLOSED, never written.
		d.WriteString(" " + styleError.Render("✗ "+m.customDirectiveValidateErr) + "\n")
		d.WriteString(" " + styleFaint.Render(PropsSSHNothingWrittenNote) + "\n")
		return lipgloss.NewStyle().Width(width).Render(d.String())
	}
	proof := m.customDirectiveProof
	if proof.UnknownName {
		// Beat 1 only — the name itself failed, so there is no ssh -G value
		// proof to show (D-I: "only if the name passed").
		d.WriteString(" " + styleWarning.Render(fmt.Sprintf(PropsSSHUnknownDirectiveFmt, m.pendingDirectiveName)) + "\n")
		return lipgloss.NewStyle().Width(width).Render(d.String())
	}
	// Beat 1: the name check passed.
	d.WriteString(" " + styleHealthy.Render(fmt.Sprintf(PropsSSHRecognizedDirectiveFmt, m.pendingDirectiveName)) + "\n")
	// Beat 2: the ssh -G proof against the staged throwaway config —
	// TEST-01's "shown == run" contract, reused verbatim rather than
	// inventing a new render.
	d.WriteString(PreviewBlock("ssh -G proof (staged, throwaway config)", "$ "+proof.Command, false, width, 2) + "\n")
	// WR-14: proof.Output is the verbatim combined output of a real ssh
	// subprocess — untrusted terminal content. Unlike PreviewBlock above,
	// this block has no line budget of its own, so it is bounded and
	// control-character-sanitized HERE before it ever reaches the frame.
	sanitizedOutput := sanitizeProofOutput(proof.Output)
	d.WriteString(lipgloss.NewStyle().Width(width).Render(" "+styleFaint.Render(sanitizedOutput)) + "\n")
	switch {
	case proof.PreexistingError:
		// D-I's third outcome: a DIFFERENT directive already had a problem
		// in the current global block — never blamed on the entry just
		// submitted.
		d.WriteString(" " + styleWarning.Render(fmt.Sprintf(PropsSSHPreexistingConfigErrorFmt, proof.OffendingName)) + "\n")
	default:
		// D-I's fourth outcome: the name is recognized but the VALUE was
		// rejected. PropsSSHProofRejectedFmt itself carries the verbatim
		// output (TEST-01's exact-output discipline) — sanitized the SAME
		// way as the block above (WR-14).
		d.WriteString(" " + styleWarning.Render(fmt.Sprintf(PropsSSHProofRejectedFmt, sanitizedOutput)) + "\n")
	}
	return lipgloss.NewStyle().Width(width).Render(d.String())
}

// focusCustomDirectiveField mirrors focusCustomKeyField's exact shape
// (globalgit.go) for the PROP-04 (09.5-04) custom-directive name/value pair
// — customDirectiveFieldFocus uses the SAME 0/1 convention.
func (m *globalSSHModel) focusCustomDirectiveField() {
	if m.customDirectiveFieldFocus == 0 {
		m.customDirectiveNameInput.Focus()
		m.customDirectiveValueInput.Blur()
		return
	}
	m.customDirectiveNameInput.Blur()
	m.customDirectiveValueInput.Focus()
}
