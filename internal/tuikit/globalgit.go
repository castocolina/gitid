package tuikit

// globalgit.go is the Go mirror of
// .planning/design/mockup-src/src/demo/screens/GlobalGit.tsx per
// 02-REDESIGN-SPEC.md §4 — GGIT-01 baseline master-detail with per-row
// apply checkboxes, the main-vs-master highlight, and a
// sentinel-preserving apply ceremony. gitid never writes user.email into
// the baseline managed block — identities own their author via includeIf
// fragments. D9 (02-DESIGN-DECISIONS-CHECKPOINT-2.md) promotes the
// user.email row from awareness-only to a first-class EDITABLE
// global-fallback field + apply checkbox (unchecked/empty by default —
// setting it is explicit opt-in), applied through its OWN dedicated
// ceremony — a DOCUMENTED, CONSCIOUS divergence from recipes/ (which leave
// it unset), with the includeIf-precedence invariant preserved: identity
// fragments always override this fallback.

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// globalGitModel is the Global Git tab child model.
// gitCeremonyKind discriminates which ceremony m.ceremony is — set once at
// open time, never re-derived from display text (WR-09).
type gitCeremonyKind int

const (
	gitCeremonyNone gitCeremonyKind = iota
	gitCeremonyBaseline
	gitCeremonyFallback
	// gitCeremonyCustomKey is the free-form custom Git key write ceremony
	// (Phase 9.5 plan 09.5-03, PROP-03) — a THIRD, independent ceremony
	// instance opened only from the "Set keys" sub-tab's "n" form, never
	// reachable from the baseline or fallback branches, and vice versa.
	gitCeremonyCustomKey
)

// Global Git sub-tabs (09.5-02, Task 1) — this screen's FIRST sub-tab strip.
// ggitOptions carries today's ENTIRE existing content unchanged; ggitSetKeys
// (PROP-02) lists every git config key actually set on the machine.
type ggitSubTab int

const (
	ggitOptions ggitSubTab = iota
	ggitSetKeys
)

// Sub-tab strip composition — renderSubTabStrip (frame.go, D-D) renders
// exactly these labels (with a one-space lead and a one-space gap) and
// handleClick hit-tests against the same strings, so the spans can never
// drift. ggitTabSetKeysLabel derives from design.go's frozen
// PropsGitSubTabLabel constant (padding added around it) — ONE source for
// the label text, never restated — the SAME discipline gssTabPropertiesLabel
// already follows for Global SSH's third label.
const (
	// ggitTabOptionsLabel derives from design.go's frozen
	// PropsOptionsSubTabLabel constant (padding added around it) — ONE
	// source for the label text, never restated (WR-06, round 3).
	ggitTabOptionsLabel = " " + PropsOptionsSubTabLabel + " "
	ggitTabSetKeysLabel = " " + PropsGitSubTabLabel + " "
)

// ggitFooterCycleLabel is the ←→ footer action's label, naming both
// sub-tabs — mirrors gssFooterCycleLabel's identical role on Global SSH.
const ggitFooterCycleLabel = "Options / Set keys"

// ggitNextSubTab returns the sub-tab the → key cycles to: Options → Set
// keys → Options.
func ggitNextSubTab(cur ggitSubTab) ggitSubTab {
	if cur == ggitOptions {
		return ggitSetKeys
	}
	return ggitOptions
}

// ggitPrevSubTab returns the sub-tab the ← key cycles to. With exactly two
// sub-tabs both directions toggle the same pair (mirrors gssPrevSubTab's own
// doc comment about its two-sub-tab era, before Global SSH grew a third) —
// kept as its own function so a future third Global Git sub-tab does not
// need to re-derive this split from gssPrevSubTab's history a second time.
func ggitPrevSubTab(cur ggitSubTab) ggitSubTab {
	return ggitNextSubTab(cur)
}

type globalGitModel struct {
	// backend is the injected options/commit seam. The model fetches the
	// option states on activation and never renders fixture data for a row
	// the backend could have answered.
	backend Backend
	// subTab is this screen's FIRST sub-tab (09.5-02, Task 1) — ggitOptions
	// (today's ENTIRE existing content, unchanged) or ggitSetKeys (PROP-02).
	subTab ggitSubTab
	// detailKey is the selected option row's key.
	detailKey string
	chosen    map[string]bool
	// options is the live Options pane row set fetched from the backend on
	// activation; optionsErr carries the fetch failure the pane renders
	// instead of a blank body (GGIT-01 advisory posture — 07-UI-SPEC.md
	// RESOLVED "error" row: optionsErr rendered as an inline warning note).
	options    []GlobalGitOptionView
	optionsErr string
	// applyCommitPending gates the apply ceremony's receipt: CommitGlobalGit is
	// dispatched only from handleMsg once GlobalGitCommitMsg arrives with an
	// empty Err, never optimistically on ceremonyConfirmed (mirrors
	// identities.go's gitCommitPending and globalssh.go's applyCommitPending).
	// appliedKeys is the EXACT key set captured at ceremonyConfirmed.
	applyCommitPending bool
	appliedKeys        []string
	// fallbackCommitPending gates the D9 ceremony's receipt the same way
	// applyCommitPending gates the baseline ceremony. The two flags are
	// mutually exclusive: confirming one ceremony never dispatches the
	// other's commit method.
	fallbackCommitPending bool
	// pendingFallbackName / pendingFallbackEmail are the EXACT values
	// submitted to CommitGitFallbackAuthor at ceremonyConfirmed (BL-04,
	// 09.4-REVIEW.md independent re-review) — handleMsg must dispatch the
	// reducer action with these, never with m.nameInput.Value()/
	// m.emailInput.Value() read fresh at message-arrival time: activate()
	// re-seeds both inputs from the (pre-write, now-stale) backend state on
	// re-entry, so a Ctrl+P-then-back round-trip while the commit is still
	// in flight would otherwise silently substitute the OLD values into the
	// dispatched action, even though the write on disk used the submitted
	// ones.
	pendingFallbackName  string
	pendingFallbackEmail string
	// commitRequestToken is incremented at every ceremony confirm (CR-01,
	// 09.4-REVIEW.md second independent re-review) and captured into the
	// dispatched command's wrapper message (gitCommitTokenMsg). BL-04's
	// fix (previous round) stopped dropping a completed write's reducer
	// action when the ceremony that started it had been abandoned, but
	// gated the ceremony-UI mutation on a bare applyCommitPending/
	// fallbackCommitPending boolean with no per-request correlation — so a
	// STALE message from an abandoned ceremony could be misattributed to a
	// DIFFERENT, newer ceremony of the same kind that opened afterward and
	// is genuinely still in flight, corrupting its receipt with the first
	// ceremony's backup path. Comparing the token in handleMsg closes that
	// gap without reopening BL-04's — the reducer action still dispatches
	// unconditionally; only the ceremony-UI mutation additionally requires
	// the token to match.
	commitRequestToken int
	// lastHeight persists the terminal height from the most recent
	// tea.WindowSizeMsg (WR-10, 09.4-REVIEW.md independent re-review) so
	// gitVisibleRowCount can measure the REAL row budget instead of the
	// canonical minFrameHeight — a taller terminal was still windowing the
	// master list to the 30-row budget and leaving blank rows under a
	// "+N more" cue. Zero until the first resize arrives; rowBudgetHeight
	// falls back to minFrameHeight in that case.
	lastHeight int
	// ceremonyOpen flags the active ceremony. ceremonyKind records WHICH
	// ceremony it is, set once at open time by the "a" handler that built
	// it (WR-09, 09.4-REVIEW.md independent re-review) — confirming a
	// ceremony must dispatch the write it was opened as, never re-derive
	// that from the ceremony's user-facing Heading string. Heading is
	// display text: baselineCeremonyFor already builds a dynamic one
	// (":424"), and the moment fallbackCeremonyFor's is made dynamic too
	// (matching every other ceremony in this file), a string match here
	// would silently fall through to the wrong commit method.
	ceremonyOpen bool
	ceremonyKind gitCeremonyKind
	ceremony     ceremonyModel
	// nameInput / emailInput are the D-04 two-field fallback pair —
	// independently editable, independently applicable. Seeded from
	// GitFallbackAuthorState on activate so "what is currently filled"
	// is always a truthful statement about the machine.
	nameInput  textinput.Model
	emailInput textinput.Model
	// fieldFocus is 0 = name, 1 = email while the fallback detail pane
	// is selected. Tab moves between them; Enter starts text-edit on
	// the focused field.
	fieldFocus int
	// fieldEditing: Enter on the selected fallback row enters text-edit
	// mode on the focused field (D1 focused rendering; every key but
	// Esc/Enter/Tab reaches the input) — this screen's single reserved-
	// letter shortcuts (space, a) would otherwise collide with typing
	// those same letters into the field; Esc/Enter exit editing back
	// to row navigation.
	fieldEditing bool
	// currentName / currentEmail are the block's contents at activate
	// time — the removal-case guard needs them independently of whatever
	// the user has since typed.
	currentName  string
	currentEmail string
	// listWindowStart is the index of the first visible ROW in the master
	// list (0-based) — the per-row analog of ExactTextViewport.LineOffset
	// (07-UI-SPEC.md RESOLVED "overflow" row). It stays 0 whenever every
	// row fits inside the computed body budget; only when the row set
	// exceeds the budget does it advance, one row at a time, in the
	// direction of travel as the selection moves outside the visible
	// window (never a jump-to-center). Shared across the ggitOptions and
	// ggitSetKeys sub-tabs (09.5-02, Task 2) — mirrors globalSSHModel's
	// identical one-field-for-every-sub-tab-list reuse.
	listWindowStart int
	// setKeys is the live "Set keys" sub-tab row set (PROP-02) fetched from
	// the backend on activation, synchronously, the SAME pattern activate()
	// already uses for options above; setKeysErr carries the fetch failure
	// the pane renders instead of a blank body, mirroring optionsErr's
	// advisory posture.
	setKeys    []GitSetKeyView
	setKeysErr string
	// filter is the "Set keys" sub-tab's type-to-filter textinput (D-B);
	// filterFocused mirrors globalSSHModel's identical properties-filter
	// keyboard-capture contract — while true, handleKey routes every key
	// but esc into the input and view() reports capturesKeys: true.
	filter        textinput.Model
	filterFocused bool
	// setKeysDetailKey is the selected row's key on the Set keys sub-tab —
	// kept SEPARATE from detailKey (the Options sub-tab's own selection)
	// because the two lists' keys live in different casing/scope domains
	// and are never the same slice, mirroring globalSSHModel's identical
	// propDetailKey/detailKey split.
	setKeysDetailKey string
	// customKeyOpen flags the "Set keys" sub-tab's 2-field custom-key entry
	// form (Phase 9.5 plan 09.5-03, PROP-03), reachable only via "n". While
	// open it captures the keyboard exactly like the D9 fallback pair's
	// fieldEditing — every key but Esc/Enter/Tab reaches the focused input.
	customKeyOpen bool
	// customKeyInput / customValueInput are the free-form key=value pair
	// textinputs — the D9 two-field pattern reused verbatim, its own
	// independent pair (never shared with nameInput/emailInput).
	customKeyInput   textinput.Model
	customValueInput textinput.Model
	// customFieldFocus is 0 = key, 1 = value — the SAME 0/1 convention
	// fieldFocus uses for the D9 pair.
	customFieldFocus int
	// customKeyFormErr carries the backend's own SplitGitKey/validateValue
	// rejection (PropsGitKeyInvalidFmt), rendered inline on the form. A
	// non-empty value means CustomGitKeyPlan failed and the ceremony was
	// NOT opened — the same "a preview that cannot be computed renders the
	// error inline" rule both Global screens already follow.
	customKeyFormErr string
	// customKeyCommitPending gates the custom-key ceremony's receipt the
	// SAME way applyCommitPending/fallbackCommitPending gate their own
	// ceremonies — mutually exclusive with both.
	customKeyCommitPending bool
	// pendingCustomKey / pendingCustomValue are the EXACT key/value
	// submitted to CommitCustomGitKey at ceremonyConfirmed (mirrors
	// pendingFallbackName/pendingFallbackEmail's staleness-safety
	// rationale above) — handleMsg must dispatch using the message's own
	// snapshot, never these fields read fresh, for the identical reason.
	pendingCustomKey   string
	pendingCustomValue string
}

// newGlobalGitModel returns a model with an EMPTY selection set (D-15, R-1):
// the selection starts empty on every entry to the screen, and the toggle key
// is the only way to add to it. The pre-chosen fixture set was the demo's
// scripted state, not the real default. Mirror newGlobalSSHModel's comment
// verbatim — this is 6/D-15, carried forward by 07-CONTEXT.md's <domain>
// block, and it is what makes an apply on the real binary a statement of
// intent rather than a fixture replay.
func newGlobalGitModel(b Backend) globalGitModel {
	return globalGitModel{
		backend:          b,
		chosen:           map[string]bool{},
		nameInput:        newTextInput(""),
		emailInput:       newTextInput(""),
		filter:           newGitSetKeysFilterInput(),
		customKeyInput:   newTextInput(""),
		customValueInput: newTextInput(""),
	}
}

// newGitSetKeysFilterInput builds the "Set keys" filter textinput with its
// frozen prompt/placeholder (09.5-UI-SPEC.md) — a small helper so both the
// constructor and activate()'s per-entry reset build the SAME shape,
// mirroring globalssh.go's newPropertiesFilterInput.
func newGitSetKeysFilterInput() textinput.Model {
	ti := newTextInput("")
	ti.Prompt = "/ "
	ti.Placeholder = PropsFilterPlaceholder
	// WR-03: bubbles/v2's textinput clips its placeholder to the model's
	// width; the zero value (no SetWidth call) clips to a single rune, so
	// the frozen placeholder never actually rendered — the row showed "/ T".
	ti.SetWidth(lipgloss.Width(PropsFilterPlaceholder) + 1)
	return ti
}

// activate syncs the Options pane rows from the backend and resets the
// selection to empty on every entry so returning to the screen never
// resurrects a stale selection (D-15, R-1). A non-nil fetch error stores an
// empty slice plus an error note the pane renders instead of a blank body.
//
// This is SYNCHRONOUS — 07-UI-SPEC.md resolved the "loading" row explicitly:
// activate() calls GlobalGitOptionStates(), stores the result, and returns
// (m, nil) with no tea.Cmd, no spinner, and no intermediate state — exactly
// as globalSSHModel.activate() already behaves (cited as the precedent for
// every other Global-* tab).
func (m globalGitModel) activate(DemoState) (screenModel, tea.Cmd) {
	// D-01/UXP-01 precedent: every entry to this screen resets to the first
	// sub-tab, alongside every other per-entry reset below — a screen
	// re-entered from scratch must not resume wherever a previous visit left
	// off (09.5-02, Task 1 — this screen's first sub-tab reset).
	m.subTab = ggitOptions
	m.chosen = map[string]bool{}
	m.listWindowStart = 0
	m.optionsErr = ""
	// CR-02: a screen re-entered from scratch must not resume a ceremony
	// whose selection this same call just cleared above. The keyboard
	// cannot reach activate() while a ceremony is open (its handleKey
	// returns handled:true, short-circuiting the globals), but a mouse
	// click on the header tab bar bypasses that guard, so this reset is the
	// state-machine half of the fix — App.handleMouse's capturesKeys guard
	// is the other half.
	m.ceremonyOpen = false
	m.applyCommitPending = false
	m.fallbackCommitPending = false
	m.ceremony = ceremonyModel{}
	// PROP-03 (09.5-03): the custom-key form and its pending state reset
	// per-entry alongside every other reset above — a screen re-entered
	// from scratch must not resurrect a stale in-progress custom-key entry.
	m.customKeyOpen = false
	m.customKeyInput = newTextInput("")
	m.customValueInput = newTextInput("")
	m.customFieldFocus = 0
	m.customKeyFormErr = ""
	m.customKeyCommitPending = false
	m.pendingCustomKey = ""
	m.pendingCustomValue = ""
	options, err := m.backend.GlobalGitOptionStates()
	m.options = options
	if err != nil {
		m.options = nil
		m.optionsErr = err.Error()
	}
	// D-01 / UXP-01: same derived first-row rule as Global SSH. detailKey
	// must reset alongside listWindowStart: screens are persistent model
	// instances re-activated in place on every tab switch (app.go), so a
	// deep-scrolled selection would otherwise survive a tab switch while
	// listWindowStart does not — leaving the selected row off-window with
	// no ▸ marker anywhere until several more keypresses let the window
	// catch up (found in code review: the selection cursor genuinely
	// desyncs from the visible scroll window after leaving and returning).
	if len(m.options) > 0 {
		m.detailKey = m.options[0].Key
	}
	state, stateErr := m.backend.GitFallbackAuthorState()
	if stateErr == nil {
		m.currentName = state.Name
		m.currentEmail = state.Email
		m.nameInput = newTextInput(state.Name)
		m.emailInput = newTextInput(state.Email)
	}
	// PROP-02 (09.5-02, Task 2): the "Set keys" sub-tab's row set, fetched
	// synchronously alongside the Options fetch above — the SAME per-entry
	// pattern activate() already uses for every other seam on this screen.
	setKeys, setKeysErr := m.backend.AllGitSetKeys()
	m.setKeys = setKeys
	m.setKeysErr = ""
	if setKeysErr != nil {
		m.setKeys = nil
		m.setKeysErr = setKeysErr.Error()
	}
	// D-B: the filter resets per-entry, alongside every other per-entry
	// reset above — a stale filter from a previous visit must never survive
	// re-entering the screen.
	m.filter = newGitSetKeysFilterInput()
	m.filterFocused = false
	if len(m.setKeys) > 0 {
		m.setKeysDetailKey = m.setKeys[0].Key
	} else {
		m.setKeysDetailKey = ""
	}
	return m, nil
}

// handleMsg completes the asynchronous apply commit once the backend's command
// has answered. The receipt is reachable ONLY from an explicit success;
// reducer actions are dispatched here, never optimistically.
// rowBudgetHeight returns the real terminal height once known (WR-10),
// falling back to the canonical minFrameHeight before the first
// tea.WindowSizeMsg arrives — matching every existing test that drives this
// model directly without ever sending one.
func (m globalGitModel) rowBudgetHeight() int {
	// WR-02 (09.4-REVIEW.md second independent re-review): floor at
	// minFrameHeight, not just > 0. tea.WindowSizeMsg reaches every
	// screen's handleMsg (app.go) before App's own too-small guard
	// (a.width < minFrameWidth || a.height < minFrameHeight) is ever
	// consulted, so a resize transient (or a host briefly reporting a tiny
	// height mid-resize) could persist a below-canonical m.lastHeight and
	// transiently collapse the scroll budget independent of what view()'s
	// own render arguments are doing.
	if m.lastHeight >= minFrameHeight {
		return m.lastHeight
	}
	return minFrameHeight
}

// gitCommitTokenMsg pairs a dispatched commit's real tea.Msg with the token
// captured at confirm time (CR-01, 09.4-REVIEW.md second independent
// re-review) AND a snapshot of exactly what was submitted at THAT confirm
// (CR-01, third independent re-review). wrapGitCommitToken wraps every
// Commit*'s returned tea.Cmd with one of these so handleMsg can tell a
// STALE message (from a ceremony that was abandoned and superseded by a
// newer one of the same kind before its own message arrived) apart from
// the genuinely current one, AND so the unconditionally-dispatched reducer
// Action (BL-04) reads the STALE message's own submitted values —
// keys/name/email/layout — never m.appliedKeys/m.pendingFallbackName/
// m.pendingFallbackEmail/m.storageTargetLayout, which are plain model
// fields that get silently overwritten by a newer same-kind ceremony's own
// confirm before the stale message arrives. Without the snapshot, the
// token comparison alone only protected the ceremony UI from
// misattribution — the reducer action could still commit a NEWER
// ceremony's values into App.state attributed to an OLDER commit's
// success.
type gitCommitTokenMsg struct {
	token  int
	msg    tea.Msg
	keys   []string
	name   string
	email  string
	layout SSHStorageLayout
	// customKey / customValue snapshot the EXACT pair submitted to
	// CommitCustomGitKey at ceremonyConfirmed (PROP-03) — the custom-key
	// ceremony's own mirror of keys/name/email above.
	customKey   string
	customValue string
	// sshDirectiveName / sshDirectiveValue snapshot the EXACT pair submitted
	// to CommitCustomSSHDirective at ceremonyConfirmed (Phase 9.5 plan
	// 09.5-04, PROP-04) — the custom-SSH-directive ceremony's own mirror of
	// customKey/customValue above, sharing this SAME wrapper type since
	// globalssh.go's handleMsg unwraps it identically.
	sshDirectiveName  string
	sshDirectiveValue string
}

func wrapGitCommitToken(token int, cmd tea.Cmd, snapshot gitCommitTokenMsg) tea.Cmd {
	if cmd == nil {
		return nil
	}
	return func() tea.Msg {
		snapshot.token = token
		snapshot.msg = cmd()
		return snapshot
	}
}

func (m globalGitModel) handleMsg(msg tea.Msg, _ DemoState) keyResult {
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
	// BL-04 (09.4-REVIEW.md independent re-review): ceremonyOpen must gate
	// only the ceremony UI mutation, never the reducer action — Ctrl+P
	// bypasses this screen's own pending-ceremony guard (intercepted by
	// App.handleKey before the screen sees the key) and re-entering via
	// activate() (CR-02) resets ceremonyOpen/applyCommitPending/
	// fallbackCommitPending. A still-in-flight commit's success message must
	// still refresh App.state even when the ceremony that started it is
	// gone by the time it arrives — the write already happened on disk.
	// CR-01 (second independent re-review): additionally require the
	// message's token to match m.commitRequestToken — a message whose
	// ceremony was abandoned and then superseded by a NEWER ceremony of the
	// same kind must not be treated as belonging to that newer one either.
	if commit, ok := msg.(GlobalGitCommitMsg); ok {
		ceremonyOpen := m.ceremonyOpen && m.applyCommitPending && token == m.commitRequestToken
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
			// fails after its ceremony is gone (abandoned, or superseded by
			// a newer one) must still surface SOMEWHERE — the ceremony's own
			// commitFailed above already covers the still-open case.
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
			if len(commit.Advisories) > 0 {
				m.ceremony = m.ceremony.withResultExtra(strings.Join(commit.Advisories, "\n"))
			}
		}
		return keyResult{
			model:   m,
			note:    fmt.Sprintf("%d global git option%s applied.", len(keys), plural),
			actions: []Action{ApplyGitBaseline{Backup: firstBackup(commit.Backups)}},
		}
	}
	if commit, ok := msg.(GitFallbackAuthorCommitMsg); ok {
		ceremonyOpen := m.ceremonyOpen && m.fallbackCommitPending && token == m.commitRequestToken
		if ceremonyOpen {
			m.fallbackCommitPending = false
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
		if ceremonyOpen {
			m.ceremony = m.ceremony.commitSucceeded(commit.Backups)
			if len(commit.Advisories) > 0 {
				m.ceremony = m.ceremony.withResultExtra(strings.Join(commit.Advisories, "\n"))
			}
		}
		// CR-01 (third independent re-review): read the SUBMITTED name/email
		// from the message's own snapshot, never m.pendingFallbackName/
		// m.pendingFallbackEmail — a newer fallback ceremony's confirm
		// overwrites those fields before this (possibly stale) message
		// arrives, which would otherwise commit the WRONG name/email pair
		// to App.state attributed to THIS commit's success. (This
		// supersedes the older "use the CAPTURED submission, not
		// m.nameInput/m.emailInput" fix — the pending fields have the same
		// staleness problem one level up.)
		name, email := snapshot.name, snapshot.email
		if ceremonyOpen {
			m.currentName = name
			m.currentEmail = email
		}
		return keyResult{
			model:   m,
			note:    GlobalGitEmailResultMessage,
			actions: []Action{ApplyGitGlobalEmail{Email: email, Name: name, Backup: firstBackup(commit.Backups)}},
		}
	}
	if commit, ok := msg.(GitCustomKeyCommitMsg); ok {
		// PROP-03 (09.5-03): the custom-key ceremony's completion — routed
		// through the SAME token-correlated stale-message guard every other
		// ceremony on this screen uses (CR-01). No parallel completion path.
		ceremonyOpen := m.ceremonyOpen && m.customKeyCommitPending && token == m.commitRequestToken
		if ceremonyOpen {
			m.customKeyCommitPending = false
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
		// Read the SUBMITTED key/value from the message's own snapshot,
		// never m.pendingCustomKey/m.pendingCustomValue — mirrors the
		// staleness-safety rationale on the fallback-author branch above.
		key, value := snapshot.customKey, snapshot.customValue
		if ceremonyOpen {
			m.ceremony = m.ceremony.commitSucceeded(commit.Backups)
			if len(commit.Advisories) > 0 {
				// WR-06: an entry that could not be re-rendered was dropped
				// rather than failing the whole write — name it, mirroring
				// the SSH custom-directive ceremony's own advisory render.
				m.ceremony = m.ceremony.withResultExtra(strings.Join(commit.Advisories, "\n"))
			}
		}
		return keyResult{
			model: m,
			note:  fmt.Sprintf(PropsGitCustomReceiptFmt, key, value),
		}
	}
	return keyResult{model: m}
}

// overlaidGitOptions projects the model's LIVE backend views through the
// applied overlay: keys the user applied render as current=recommended,
// one-liner prefixed "Applied by gitid — ".
//
// Rows come from the fetched GlobalGitOptionView slice — they MUST travel
// through the seam, never around it. The fixture path lives only in
// internal/dummytui/fixturebackend.go's GlobalGitOptionStates, which is what
// the dummy binary serves through this same seam. Any value that looks like a
// fixture but is not fetched from the backend is a fixture leak: a comment
// naming the source of truth must be visible at every such site.
func (m globalGitModel) overlaidGitOptions(s DemoState) []GlobalGitOptionView {
	out := make([]GlobalGitOptionView, 0, len(m.options))
	for _, o := range m.options {
		entry := o
		if o.Key == GlobalGitEmailFallbackKey {
			// D9: the fallback row has its OWN dedicated apply ceremony
			// and must NEVER join the generic baseline overlay.
			if s.GitGlobalName != "" || s.GitGlobalEmail != "" {
				entry.CurrentValue = fallbackCurrentLabel(s.GitGlobalName, s.GitGlobalEmail)
			}
			out = append(out, entry)
			continue
		}
		// review-findings F4 precedent (carried from the original demo):
		// the overlay sweeps every still-NeedsAction row to applied once the
		// single GitBaselineApplied flag flips, mirroring the coarse
		// single-shot semantics this screen has always had — there is no
		// per-key applied list on DemoState (unlike SSHApplied), because
		// the real backend re-probes on the next activate() and this
		// overlay only needs to simulate "just applied" for the STUB/dummy
		// path between a commit and the next activation.
		if s.GitBaselineApplied && o.State == GlobalGitNeedsAction {
			entry.CurrentValue = o.Recommended
			entry.State = GlobalGitAlreadySet
			entry.OneLiner = "Applied by gitid — " + o.OneLiner
		}
		out = append(out, entry)
	}
	return out
}

// gitNeedsAttention is the ONE tally predicate for the Global Git options
// pane (D-10, pinned in 07-03-PLAN.md's <authority> block). It counts a ROW
// at most once, and counts only rows in the needs-action state — a row whose
// State is set-but-differs is EXCLUDED (D-02: under the floor + last-wins
// model, a write into a key the user set later is provably a no-op, so the
// row has no action to offer), not-applicable rows are excluded, and a bundle
// row counts once when at least one member key is unset, whatever the other
// members do. The fallback-author row is excluded because it carries no
// writable member keys — it has its own dedicated ceremony (D-05).
//
// The list's status line and the apply ceremony's count BOTH read this one
// predicate; nothing re-derives a second count.
//
// This DIVERGES from Phase 6's own tally (06-03-SUMMARY.md), which counts
// set-but-differs rows as needing attention. The divergence is deliberate and
// asymmetric because the precedence arrow is reversed: on the SSH side gitid's
// `Host *` block can genuinely change a differing value, on the git side it
// provably cannot. Cite both D-02 and 06-03's own rule so a future reader
// sees the asymmetry is intended.
func gitNeedsAttention(o GlobalGitOptionView) bool {
	return o.Selectable()
}

// gitApplyChosen is the chosen ∩ selectable key set, in row order. The
// selectable predicate (needs-action + writable member keys + no probe error)
// is what excludes the differs, not-applicable and fallback-author rows — there
// is no per-key exclusion list and no second counting loop here.
func (m globalGitModel) gitApplyChosen(options []GlobalGitOptionView) []string {
	var keys []string
	for _, o := range options {
		if o.Selectable() && m.chosen[o.Key] {
			keys = append(keys, o.Key)
		}
	}
	return keys
}

// globalGitRowLine2 renders the option row's second line: "now: <current> →
// <recommended>" plus the state WORD. The D-02 differs vocabulary deliberately
// reuses Phase 6's own frozen sentences (GlobalSSHWordDiffersUser/Outside) —
// one vocabulary across both Global-* screens, already inside the copy-freeze
// gate. Not-applicable rows replace the now-line with their reason sentence.
func globalGitRowLine2(o GlobalGitOptionView) string {
	if o.State == GlobalGitNotApplicable {
		return globalGitNotApplicableSentence(o.NotApplicableReason)
	}
	now := "now: " + o.CurrentValue + " → " + o.Recommended
	switch o.State {
	case GlobalGitSetButDiffers:
		if o.AttributedToUser {
			return now + "  " + GlobalSSHWordDiffersUser
		}
		return now + "  " + GlobalSSHWordDiffersOutside
	case GlobalGitAlreadySet:
		if o.CurrentValue != "" {
			return now + "  " + GlobalSSHWordAlreadySet
		}
	}
	return now
}

// globalGitNotApplicableSentence renders the per-reason not-applicable line.
func globalGitNotApplicableSentence(r GlobalGitNotApplicableReason) string {
	switch r {
	case GlobalGitReasonProbeFailed:
		// Byte-identical to Phase 6's own probe-failed sentence (already in
		// the freeze gate) — one vocabulary across both Global-* screens.
		return GlobalSSHNAProbeFailed
	default:
		return GlobalSSHNAProbeFailed
	}
}

// gitDetailIndex resolves the selected option row index.
func (m globalGitModel) gitDetailIndex(options []GlobalGitOptionView) int {
	for i, o := range options {
		if o.Key == m.detailKey {
			return i
		}
	}
	return 0
}

func fallbackCurrentLabel(name, email string) string {
	switch {
	case name != "" && email != "":
		return name + " <" + email + ">"
	case name != "":
		return name
	case email != "":
		return email
	default:
		return "unset (recipes default)"
	}
}

// emailValid reports whether the D9 email field is empty (unset, valid) or
// holds a plausible email — reusing the wizard git-form's own contains-@
// check rather than inventing a second rule. A non-empty malformed email
// is the only invalid state.
func (m globalGitModel) emailValid() bool {
	email := m.emailInput.Value()
	return email == "" || strings.Contains(email, "@")
}

// fallbackApplyOffered is the D-04 / 07-UI-SPEC.md resolved "partial" row
// apply guard. Pressing `a` applies a PAIR SNAPSHOT of whatever the two
// seeded fields currently hold, in ONE ceremony instance. Clearing one
// field and applying also re-asserts the other's seeded value — that is
// correct for a TUI whose fields are visible and pre-filled. Plan 07-05's
// CLI uses explicit set/clear flags because a CLI has no screen, so an
// omitted flag must mean "leave it alone" rather than "clear it". Both
// call the SAME EnsureGitFallbackAuthor with a resolved pair; do not
// "fix" the TUI into per-field flags or the CLI into a snapshot.
//
// Offer the action when the email is empty or valid, AND at least one of
// these is true: a field is non-empty, or the machine's current block is
// non-empty (the removal case — the one place a naive "both empty means
// nothing to do" guard is wrong, and exactly how the user clears a
// fallback they previously set). Do not offer it when the email is
// non-empty and malformed, or when both fields are empty and the current
// block is empty.
func (m globalGitModel) fallbackApplyOffered() bool {
	if !m.emailValid() {
		return false
	}
	name := m.nameInput.Value()
	email := m.emailInput.Value()
	if name != "" || email != "" {
		return true
	}
	return m.currentName != "" || m.currentEmail != ""
}

// baselineCeremonyFor builds the apply ceremony using the plan view returned
// by the backend: heading, targets, backups, and preview all come from
// GlobalGitApplyPlan. The heading is the frozen prefix plus Targets[0] —
// the resolved include'd baseline path — which is the D-01 allowlisted
// divergence from the fixture's hardcoded main-config path. The ceremony is
// ASYNC: confirmation dispatches the backend commit and the receipt is
// reachable only from that commit's explicit success (ceremony.go's Async
// contract), exactly like the global-SSH apply ceremony.
func (m globalGitModel) baselineCeremonyFor(keys []string, pending int) (ceremonyModel, error) {
	plan, planErr := m.backend.GlobalGitApplyPlan(keys)
	if planErr != nil {
		return ceremonyModel{}, planErr
	}
	targets := plan.Targets
	if len(targets) == 0 {
		// The backend's plan always resolves the baseline target (D-01); this
		// fallback keeps the ceremony buildable for a stub that answers an empty plan.
		targets = []string{"~/.gitconfig.d/00-baseline"}
	}
	backups := plan.Backups
	if len(backups) == 0 {
		backups = []string{NewBackupPath(targets[0])}
	}
	preview := plan.Diff
	if preview == "" {
		preview = GlobalGitFullManagedBlockText
	}
	note := ""
	if containsString(keys, "user.useConfigOnly") {
		name, email := strings.TrimSpace(m.nameInput.Value()), strings.TrimSpace(m.emailInput.Value())
		switch {
		case email != "" && name == "":
			note = GlobalGitCrossWarningNameMissing
		case name != "" && email == "":
			note = GlobalGitCrossWarningEmailMissing
		}
	}
	return newCeremony(ceremonyConfig{
		Heading: "Write global-git managed block to " + targets[0],
		Targets: targets,
		Backups: backups,
		Preview: preview,
		// The counts are real — len(keys) selected/applied against pending
		// (needs-action rows at the moment "a" was pressed, via the SAME
		// gitNeedsAttention/D-10 tally predicate the status line reads, mirroring
		// globalssh.go's chosen/pending shape) — only the tail sentence about
		// the baseline apply leaving the global author alone is frozen
		// (GlobalGitResultTail, registered in the copy-freeze gate).
		ResultMessage:   fmt.Sprintf("%d of %d baseline options applied to %s. %s", len(keys), pending, targets[0], GlobalGitResultTail),
		ConfirmLabel:    "Apply selected",
		Hint:            note,
		PreviewMaxLines: len(strings.Split(preview, "\n")),
		Async:           true,
	}), nil
}

// fallbackCeremonyFor builds the D9 dedicated apply ceremony from the
// planner's plan view — heading, targets, backups, and preview all come
// from GitFallbackAuthorPlan so the ceremony names the resolved file and
// the real promised backup. It is a SECOND, independent ceremony instance:
// the baseline ceremony branch is untouched and neither branch can reach
// the other's commit method.
func (m globalGitModel) fallbackCeremonyFor(name, email string) (ceremonyModel, error) {
	plan, planErr := m.backend.GitFallbackAuthorPlan(name, email)
	if planErr != nil {
		return ceremonyModel{}, planErr
	}
	targets := plan.Targets
	if len(targets) == 0 {
		targets = []string{"~/.gitconfig"}
	}
	backups := plan.Backups
	if len(backups) == 0 && !plan.Removal {
		backups = []string{NewBackupPath(targets[0])}
	}
	preview := plan.Diff
	if preview == "" {
		preview = fallbackPreview(name, email)
	}
	heading := GlobalGitEmailCeremonyHeading
	if plan.Removal {
		heading = "Remove global fallback author"
	}
	return newCeremony(ceremonyConfig{
		Heading:       heading,
		Targets:       targets,
		Backups:       backups,
		Preview:       preview,
		ResultMessage: GlobalGitEmailResultMessage,
		ConfirmLabel:  "Apply",
		Async:         true,
	}), nil
}

func fallbackPreview(name, email string) string {
	var b strings.Builder
	b.WriteString("+ [user]\n")
	if name != "" {
		b.WriteString("+     name = " + name + "  " + GlobalGitEmailDiffAnnotation + "\n")
	}
	if email != "" {
		b.WriteString("+     email = " + email + "  " + GlobalGitEmailDiffAnnotation + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// customKeyCeremonyFor builds the custom-key write ceremony (Phase 9.5 plan
// 09.5-03, PROP-03) from an ALREADY-VALIDATED plan view — the caller (the
// form's Enter-on-value-field submit handler) has already called
// m.backend.CustomGitKeyPlan(key, value) and only reaches here on success,
// so this helper never itself errors. Reuses ceremonyModel UNMODIFIED
// (09.5-UI-SPEC.md / CLAUDE.md: no fast path) — this is the SAME
// Async:true, non-destructive apply shape GGIT-01 uses, never the
// typed-confirm escalation reserved for rewriting an existing value.
func (m globalGitModel) customKeyCeremonyFor(key, value string, plan GitCustomKeyPlanView) ceremonyModel {
	targets := plan.Targets
	if len(targets) == 0 {
		// The backend's plan always resolves both targets; this fallback
		// keeps the ceremony buildable for a stub that answers an empty plan.
		targets = []string{"~/.gitconfig.d/00-baseline"}
	}
	backups := plan.Backups
	// The "resolved target" the heading names is the LAST target — the
	// baseline file the custom-git-keys block actually lands in — mirroring
	// the Preview's own "against the resolved managed block" framing.
	resolvedTarget := targets[len(targets)-1]
	preview := plan.Diff
	if preview == "" {
		preview = "+ " + key + " = " + value
	}
	return newCeremony(ceremonyConfig{
		Heading:         fmt.Sprintf(PropsGitCustomCeremonyHeadingFmt, resolvedTarget),
		Targets:         targets,
		Backups:         backups,
		Preview:         preview,
		PreviewDiff:     true,
		ConfirmLabel:    "Write",
		ResultMessage:   fmt.Sprintf(PropsGitCustomReceiptFmt, key, value),
		PreviewMaxLines: len(strings.Split(preview, "\n")),
		Async:           true,
	})
}

// handleKey implements the Global Git key model.
func (m globalGitModel) handleKey(msg tea.KeyMsg, s DemoState) keyResult {
	key := msg.String()

	if m.ceremonyOpen {
		if m.applyCommitPending || m.fallbackCommitPending || m.customKeyCommitPending {
			// In flight: keys are inert until the commit result arrives.
			return keyResult{model: m, handled: true}
		}
		var outcome ceremonyOutcome
		m.ceremony, outcome = m.ceremony.handleKey(msg)
		switch outcome {
		case ceremonyCancelled:
			m.ceremonyOpen = false
		case ceremonyConfirmed:
			m.commitRequestToken++
			token := m.commitRequestToken
			if m.ceremonyKind == gitCeremonyFallback {
				m.fallbackCommitPending = true
				m.pendingFallbackName = m.nameInput.Value()
				m.pendingFallbackEmail = m.emailInput.Value()
				cmd := m.backend.CommitGitFallbackAuthor(m.pendingFallbackName, m.pendingFallbackEmail)
				snapshot := gitCommitTokenMsg{name: m.pendingFallbackName, email: m.pendingFallbackEmail}
				return keyResult{model: m, handled: true, cmd: wrapGitCommitToken(token, cmd, snapshot)}
			}
			if m.ceremonyKind == gitCeremonyCustomKey {
				m.customKeyCommitPending = true
				// WR-12: m.pendingCustomKey/Value were already captured at
				// FORM-SUBMIT time (the customKeyOpen "enter" handler above)
				// — read ONLY that snapshot here, never
				// m.customKeyInput.Value()/m.customValueInput.Value(), which
				// are the model's LIVE fields and may have since diverged
				// from what the user actually confirmed.
				cmd := m.backend.CommitCustomGitKey(m.pendingCustomKey, m.pendingCustomValue)
				snapshot := gitCommitTokenMsg{customKey: m.pendingCustomKey, customValue: m.pendingCustomValue}
				return keyResult{model: m, handled: true, cmd: wrapGitCommitToken(token, cmd, snapshot)}
			}
			keys := m.gitApplyChosen(m.overlaidGitOptions(s))
			m.appliedKeys = keys
			m.applyCommitPending = true
			cmd := m.backend.CommitGlobalGit(keys)
			snapshot := gitCommitTokenMsg{keys: keys}
			return keyResult{model: m, handled: true, cmd: wrapGitCommitToken(token, cmd, snapshot)}
		case ceremonyFinished:
			m.ceremonyOpen = false
		case ceremonyNone:
		}
		return keyResult{model: m, handled: true}
	}

	// D9: while text-editing a fallback field, every key but Esc/Enter/Tab
	// reaches the input — this screen's single-letter shortcuts (space, a)
	// would otherwise collide with typing those same letters.
	if m.fieldEditing {
		switch key {
		case "esc", "enter":
			m.fieldEditing = false
			m.nameInput.Blur()
			m.emailInput.Blur()
			return keyResult{model: m, handled: true}
		case "tab":
			m.fieldFocus = 1 - m.fieldFocus
			m.focusFallbackField()
			return keyResult{model: m, handled: true}
		default:
			if m.fieldFocus == 0 {
				m.nameInput, _ = updateInput(m.nameInput, msg)
			} else {
				m.emailInput, _ = updateInput(m.emailInput, msg)
			}
			return keyResult{model: m, handled: true}
		}
	}

	// PROP-03 (09.5-03): while the custom-key form is open, it owns the
	// keyboard exactly like the D9 fallback pair's fieldEditing block above
	// — Tab moves focus between the two fields, Enter on the VALUE field
	// (focus index 1) submits, Esc closes the form without submitting, and
	// every other key routes through updateInput to the focused field.
	// Enter on the KEY field (focus index 0) does NOT submit — it only
	// exists as a Tab target, mirroring how the D9 pair's Enter is only
	// reachable via the row-level "start editing" gesture, never as a
	// mid-field submit shortcut on the first of two fields.
	if m.customKeyOpen {
		switch key {
		case "esc":
			m.customKeyOpen = false
			m.customKeyInput.Blur()
			m.customValueInput.Blur()
			m.customKeyFormErr = ""
			return keyResult{model: m, handled: true}
		case "tab":
			m.customFieldFocus = 1 - m.customFieldFocus
			m.focusCustomKeyField()
			return keyResult{model: m, handled: true}
		case "enter":
			if m.customFieldFocus != 1 {
				m.customFieldFocus = 1
				m.focusCustomKeyField()
				return keyResult{model: m, handled: true}
			}
			gitKey := m.customKeyInput.Value()
			gitValue := m.customValueInput.Value()
			// WR-12: capture the submitted snapshot HERE, at form-submit
			// time — alongside m.ceremonyKind below — so ceremonyConfirmed
			// reads ONLY m.pendingCustomKey/Value, never the model's live
			// customKeyInput/customValueInput fields. This file's own CR-01
			// doctrine is "read the SUBMITTED values from the message's own
			// snapshot, never the live model fields"; the SSH sibling
			// (globalssh.go) already does this correctly for
			// pendingDirectiveName/Value.
			m.pendingCustomKey = gitKey
			m.pendingCustomValue = gitValue
			plan, planErr := m.backend.CustomGitKeyPlan(gitKey, gitValue)
			if planErr != nil {
				// A preview that cannot be computed renders the error
				// inline — the SAME rule both Global screens already
				// follow (baselineCeremonyFor/fallbackCeremonyFor's own
				// error handling above) — and the ceremony is NOT opened.
				m.customKeyFormErr = fmt.Sprintf(PropsGitKeyInvalidFmt, planErr.Error())
				return keyResult{model: m, handled: true}
			}
			m.customKeyFormErr = ""
			m.ceremony = m.customKeyCeremonyFor(gitKey, gitValue, plan)
			m.ceremonyOpen = true
			m.ceremonyKind = gitCeremonyCustomKey
			m.customKeyOpen = false
			m.customKeyInput.Blur()
			m.customValueInput.Blur()
			return keyResult{model: m, handled: true}
		default:
			if m.customFieldFocus == 0 {
				m.customKeyInput, _ = updateInput(m.customKeyInput, msg)
			} else {
				m.customValueInput, _ = updateInput(m.customValueInput, msg)
			}
			return keyResult{model: m, handled: true}
		}
	}

	// D-B (09.5-02, Task 2): while the Set keys filter is focused, it owns
	// the keyboard — every key but esc routes into the input and is
	// reported handled, mirroring Global SSH's identical gssProperties
	// filter-capture contract (and this screen's own fieldEditing capture
	// above): esc blurs WITHOUT clearing; clearing is a separate, explicit
	// action no other gitid text field conflates with blur either.
	if m.subTab == ggitSetKeys && m.filterFocused {
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
			filtered := m.ggitFilteredKeys()
			m.setKeysDetailKey = ""
			if len(filtered) > 0 {
				m.setKeysDetailKey = filtered[0].Key
			}
			m.listWindowStart = 0
		}
		return keyResult{model: m, handled: true}
	}

	options := m.overlaidGitOptions(s)
	if m.subTab == ggitOptions && m.optionsErr != "" {
		// A failed probe is advisory/fail-open: no rows or apply action are
		// available, but this screen must not consume navigation keys and
		// trap the user here (07-UI-SPEC.md RESOLVED "error" row) — INCLUDING
		// the new sub-tab left/right (09.5-02, Task 1): mirrors Global SSH's
		// identical gssOptions+optionsErr fail-open contract, so a probe
		// failure on this sub-tab escapes to the app's top-level tab
		// switcher exactly as it did before this screen had sub-tabs.
		return keyResult{model: m}
	}
	if m.subTab == ggitSetKeys && m.setKeysErr != "" {
		// Fail-open, mirroring the Options sub-tab's own probe-failure
		// contract immediately above: every navigation key still reaches
		// the app globals on the screen least able to help.
		return keyResult{model: m}
	}
	if m.subTab == ggitOptions && len(options) == 0 {
		// Advisory/fail-open: no rows to act on, but the sub-tab switch must
		// still work — mirrors Global SSH's identical zero-options branch.
		switch key {
		case "left", "right":
			if key == "right" {
				m.subTab = ggitNextSubTab(m.subTab)
			} else {
				m.subTab = ggitPrevSubTab(m.subTab)
			}
			return keyResult{model: m, handled: true}
		}
		return keyResult{model: m, handled: true}
	}
	switch key {
	case "left", "right":
		if key == "right" {
			m.subTab = ggitNextSubTab(m.subTab)
		} else {
			m.subTab = ggitPrevSubTab(m.subTab)
		}
		return keyResult{model: m, handled: true}
	case "up", "down":
		switch m.subTab {
		case ggitOptions:
			idx := m.gitDetailIndex(options)
			if key == "down" && idx < len(options)-1 {
				idx++
			}
			if key == "up" && idx > 0 {
				idx--
			}
			m.detailKey = options[idx].Key
			m.listWindowStart = scrollWindowFor(m.listWindowStart, idx, gitVisibleRowCount(len(options), m.rowBudgetHeight(), s))
		case ggitSetKeys:
			filtered := m.ggitFilteredKeys()
			if len(filtered) > 0 {
				idx := m.ggitSetKeysDetailIndex(filtered)
				if key == "down" && idx < len(filtered)-1 {
					idx++
				}
				if key == "up" && idx > 0 {
					idx--
				}
				m.setKeysDetailKey = filtered[idx].Key
				m.listWindowStart = scrollWindowFor(m.listWindowStart, idx, ggitSetKeysVisibleRowCount(len(filtered), m.rowBudgetHeight()))
			}
		}
		return keyResult{model: m, handled: true}
	case "/":
		if m.subTab != ggitSetKeys || m.setKeysErr != "" {
			return keyResult{model: m}
		}
		m.filterFocused = true
		m.filter.Focus()
		return keyResult{model: m, handled: true}
	case "n":
		// PROP-03 (09.5-03): "n" opens the free-form custom-key entry form,
		// reachable only from the Set keys sub-tab (mirrors "/"'s identical
		// sub-tab + fail-open guard immediately above).
		if m.subTab != ggitSetKeys || m.setKeysErr != "" {
			return keyResult{model: m}
		}
		m.customKeyOpen = true
		m.customFieldFocus = 0
		m.customKeyFormErr = ""
		m.customKeyInput = newTextInput("")
		m.customValueInput = newTextInput("")
		m.focusCustomKeyField()
		return keyResult{model: m, handled: true}
	case "space":
		if m.subTab != ggitOptions {
			return keyResult{model: m, handled: true}
		}
		o := options[m.gitDetailIndex(options)]
		// Selectable() is the ONE predicate: a row that cannot be toggled
		// (including the fallback-author row, which carries no writable member
		// keys under D-04/D-05) is refused here, renders no checkbox glyph, and
		// clicks at its checkbox position do nothing — there is no path by
		// which the three can disagree.
		if o.Selectable() {
			m.chosen = withToggled(m.chosen, o.Key)
		}
		return keyResult{model: m, handled: true}
	case "tab":
		if m.subTab != ggitOptions {
			return keyResult{model: m}
		}
		if m.detailKey == GlobalGitEmailFallbackKey {
			// WR-15: keep the textinput's own Focus() state in sync with
			// fieldFocus even outside edit mode, so it is already correct
			// the moment Enter starts editing — not one Tab behind it.
			m.fieldFocus = 1 - m.fieldFocus
			m.focusFallbackField()
			return keyResult{model: m, handled: true}
		}
		return keyResult{model: m}
	case "enter":
		if m.subTab != ggitOptions {
			return keyResult{model: m}
		}
		// D9/D8: Enter on the selected fallback row starts text-editing
		// the focused field.
		if m.detailKey == GlobalGitEmailFallbackKey {
			m.fieldEditing = true
			m.focusFallbackField()
			return keyResult{model: m, handled: true}
		}
		return keyResult{model: m}
	case "a":
		if m.subTab != ggitOptions {
			return keyResult{model: m, handled: true}
		}
		if m.detailKey == GlobalGitEmailFallbackKey && m.fallbackApplyOffered() {
			cer, cerErr := m.fallbackCeremonyFor(m.nameInput.Value(), m.emailInput.Value())
			if cerErr != nil {
				m.optionsErr = cerErr.Error()
				return keyResult{model: m, handled: true}
			}
			m.ceremony = cer
			m.ceremonyOpen = true
			m.ceremonyKind = gitCeremonyFallback
			return keyResult{model: m, handled: true}
		}
		chosen := m.gitApplyChosen(options)
		if len(chosen) > 0 {
			pending := 0
			for _, o := range options {
				if gitNeedsAttention(o) {
					pending++
				}
			}
			cer, cerErr := m.baselineCeremonyFor(chosen, pending)
			if cerErr != nil {
				m.optionsErr = cerErr.Error()
				return keyResult{model: m, handled: true}
			}
			m.ceremony = cer
			m.ceremonyOpen = true
			m.ceremonyKind = gitCeremonyBaseline
		}
		return keyResult{model: m, handled: true}
	}
	return keyResult{model: m}
}

// gitBannerBeyond is the findingsBanner tail for this screen — shared by
// view and gitTopLines.
const gitBannerBeyond = "this baseline"

// gitTopLines counts the body lines rendered above the first option row on
// the Options sub-tab: the sub-tab strip (net-new, 09.5-02, Task 1 — this
// screen's first) plus the optional findings banner. Shared by view,
// handleClick, and gitVisibleRowCount — moving all three together is the
// single highest-risk edit in 09.5-02's Task 1 (a partial update desyncs the
// click hit-test from the render). Consumers reading this function are
// scoped to the ggitOptions sub-tab only; the ggitSetKeys sub-tab gets its
// own top-lines function (Task 2), mirroring how gssOptionsTopLines and
// gssPropertiesTopLines are two separate functions on Global SSH.
func gitTopLines(s DemoState) int {
	lines := subTabStripRows()
	if findingsBanner(s, "Git", gitBannerBeyond) != "" {
		lines++
	}
	return lines
}

// gitCueDownFmt / gitCueUpFmt are the master list's own "+N more" scroll
// cues (07-UI-SPEC.md RESOLVED "overflow" row) — byte-identical style/format
// convention to renderReceiptList's "… (+%d more lines)" cue (ceremony.go),
// reworded from lines to rows with a direction arrow. Registered in
// gate-copy-freeze — the receipt list's own entry covers only its own
// wording, not these.
const (
	gitCueDownFmt = "↓ (+%d more options)"
	gitCueUpFmt   = "↑ (+%d more options)"
)

// gitVisibleRowCount computes how many option rows fit inside the body
// budget WITHOUT overflowing — the SAME frameBodyRows helper the rest of
// the screen uses, minus the master list's existing chrome (the findings
// banner via gitTopLines), divided by the row height. This is a MEASURED
// budget, never a hardcoded row count (plan 02-15's standing lesson —
// "re-measure the row budget, don't assume") — it calls frameBodyRows
// rather than embedding a literal number.
//
// height is the caller's rowBudgetHeight() (WR-10, 09.4-REVIEW.md
// independent re-review) — the REAL terminal height persisted from the last
// tea.WindowSizeMsg, falling back to the canonical minFrameHeight before
// the first resize arrives. This keeps handleKey (which has no
// width/height parameter of its own, but reads m.lastHeight via
// rowBudgetHeight()) and view/handleClick computing the identical budget
// for the SAME model, so the window position handleKey advances can never
// disagree with what view renders — while still growing the budget on a
// taller terminal instead of pinning it to the 100x30 canonical frame.
//
// When every row already fits inside the raw budget, the full row count is
// returned and no line is reserved for a cue — byte-identical to the
// pre-scrolling behavior. Only when the row set does NOT fit does one line
// get reserved for the cue, shrinking the visible count by the row height's
// worth of budget.
func gitVisibleRowCount(totalRows, height int, s DemoState) int {
	budget := frameBodyRows(height) - gitTopLines(s)
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

// scrollWindowFor computes the new window start given the CURRENT window
// start, the newly selected row index, and the visible row count — moving
// the window by exactly ONE row in the direction of travel whenever the
// selection falls outside it, never jumping to center (07-UI-SPEC.md
// RESOLVED "overflow" row; the per-row analog of
// ExactTextViewport.ScrollDown(1)/ScrollUp(1)'s existing one-step model).
// Selection moves WITHIN the window leave the window start unchanged.
func scrollWindowFor(windowStart, selectedIdx, visibleRowCount int) int {
	if selectedIdx >= windowStart+visibleRowCount {
		windowStart++
	}
	if selectedIdx < windowStart {
		windowStart--
	}
	if windowStart < 0 {
		windowStart = 0
	}
	return windowStart
}

// gitCueNone / gitCueDown / gitCueUp name which (if any) scroll cue the one
// reserved line renders — shared by view (which renders the cue text) and
// handleClick (which must treat that same line as inert).
type gitCueDirection int

const (
	gitCueNone gitCueDirection = iota
	gitCueDown
	gitCueUp
)

// gitScrollWindow bundles every measurement view and handleClick both need
// to agree on: whether the list needs to scroll at all, the effective
// (clamped) window start, the visible row count, and which cue (if any) the
// one reserved line shows. Computing this ONCE from the model + option
// count is what keeps the render and the click hit-test from silently
// disagreeing about where a row is — the exact defect class 07-04-PLAN.md's
// threat register names twice.
type gitScrollWindow struct {
	needsScroll bool
	windowStart int
	visibleRows int
	cue         gitCueDirection
	hiddenCount int
}

// gitComputeScrollWindow derives the current scroll window from the model's
// listWindowStart and the live option count. windowStart is clamped
// defensively (never negative, never past the last valid window) so a
// model driven directly in a test (bypassing handleKey's own incremental
// clamp) still renders and click-hit-tests consistently.
func (m globalGitModel) gitComputeScrollWindow(totalRows int, s DemoState) gitScrollWindow {
	visible := gitVisibleRowCount(totalRows, m.rowBudgetHeight(), s)
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
	// Not hidden below but needsScroll is true: the window must be
	// scrolled past the top (windowStart > 0), so rows are hidden above.
	w.cue = gitCueUp
	w.hiddenCount = windowStart
	return w
}

// gitCueLine renders the one reserved cue line — byte-identical style/format
// convention to renderReceiptList's own "+N more" cue (ceremony.go), reworded
// from lines to rows with a direction arrow (07-UI-SPEC.md RESOLVED
// "overflow" row).
func gitCueLine(w gitScrollWindow) string {
	switch w.cue {
	case gitCueDown:
		return " " + styleFaint.Render(fmt.Sprintf(gitCueDownFmt, w.hiddenCount))
	case gitCueUp:
		return " " + styleFaint.Render(fmt.Sprintf(gitCueUpFmt, w.hiddenCount))
	default:
		return ""
	}
}

// gitRowForScreenRow maps a body-relative screen row (y - gitTopLines(s)) to
// the option-row index the user is pointing at, honoring the scroll window
// and treating the reserved cue line as inert. ok is false for the cue line
// or any row past the rendered window — handleClick must never toggle or
// select based on a screen position that isn't a real, visible row.
func gitRowForScreenRow(w gitScrollWindow, y int) (idx int, ok bool) {
	if !w.needsScroll {
		row := y / optionRowLines
		return w.windowStart + row, row >= 0 && row < w.visibleRows
	}
	switch w.cue {
	case gitCueUp:
		if y == 0 {
			return 0, false // the cue line itself
		}
		yy := y - 1
		row := yy / optionRowLines
		return w.windowStart + row, row >= 0 && row < w.visibleRows
	default: // gitCueDown (the both-edges-hidden tie-break also resolves here)
		row := y / optionRowLines
		return w.windowStart + row, row >= 0 && row < w.visibleRows
	}
}

// ---------------------------------------------------------------------------
// 09.5-02, Task 2: the "Set keys" sub-tab's flat filterable master-detail
// body (PROP-02) — mirrors globalssh.go's gssProperties* family verbatim,
// the same shape plan 09.5-01 proved for the SSH side.
// ---------------------------------------------------------------------------

// ggitSetKeysTopLines counts the body lines rendered above the first row on
// the "Set keys" sub-tab: the sub-tab strip plus the ONE filter row. No
// findings banner here — mirrors gssPropertiesTopLines' identical decision
// (09.5-01-SUMMARY.md): this sub-tab already shows the FULL set of keys
// actually set, so a "doctor found N findings beyond these options" framing
// would be self-contradictory.
func ggitSetKeysTopLines() int {
	return subTabStripRows() + 1
}

// ggitFilteredKeys returns the "Set keys" rows matching the current filter
// value, case-insensitively, against BOTH the key and the value (a user
// filtering for a path fragment expects the value to be searched too).
// Every consumer — render, the up/down handler, and the click hit-test —
// reads this SAME function, so the scroll window and the click row index
// can never disagree about which row is where.
func (m globalGitModel) ggitFilteredKeys() []GitSetKeyView {
	q := strings.ToLower(m.filter.Value())
	out := make([]GitSetKeyView, 0, len(m.setKeys))
	for _, k := range m.setKeys {
		if strings.Contains(strings.ToLower(k.Key), q) || strings.Contains(strings.ToLower(k.Value), q) {
			out = append(out, k)
		}
	}
	return out
}

// ggitSetKeysDetailIndex resolves the selected row's index within filtered —
// the Set-keys-sub-tab mirror of gitDetailIndex above.
func (m globalGitModel) ggitSetKeysDetailIndex(filtered []GitSetKeyView) int {
	for i, k := range filtered {
		if k.Key == m.setKeysDetailKey {
			return i
		}
	}
	return 0
}

// ggitSetKeysVisibleRowCount computes how many key rows fit inside the body
// budget WITHOUT overflowing — the Set-keys mirror of gitVisibleRowCount,
// but dividing by ONE line per row (this list has no toggle/apply
// affordance and needs no optionRow-style 2-line sub-budget), mirroring
// gssPropertiesVisibleRowCount.
func ggitSetKeysVisibleRowCount(totalRows, height int) int {
	budget := frameBodyRows(height) - ggitSetKeysTopLines()
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

// ggitSetKeysComputeScrollWindow derives the current scroll window from the
// model's listWindowStart and the FILTERED row count — the Set-keys mirror
// of gitComputeScrollWindow, sharing the SAME gitScrollWindow type so this
// screen's second list can never silently drift from the Options sub-tab's
// scrolling behavior.
func (m globalGitModel) ggitSetKeysComputeScrollWindow(totalRows, height int) gitScrollWindow {
	visible := ggitSetKeysVisibleRowCount(totalRows, height)
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

// ggitSetKeysRowForScreenRow maps a body-relative screen row (y -
// ggitSetKeysTopLines()) to the key-row index the user is pointing at,
// honoring the scroll window and treating the reserved cue line as inert —
// the ONE-line-per-row mirror of gitRowForScreenRow (which assumes
// optionRow's 2-line shape and therefore cannot be reused verbatim here),
// mirroring gssPropertiesRowForScreenRow.
func ggitSetKeysRowForScreenRow(w gitScrollWindow, y int) (idx int, ok bool) {
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

// setKeysFilterRow renders the filter input's live value on the left and
// the right-aligned match count on the SAME line (09.5-UI-SPEC.md: exactly
// ONE row — no separate result-count line), mirroring propertiesFilterRow.
func (m globalGitModel) setKeysFilterRow(width, matchCount int) string {
	left := " " + m.filter.View()
	right := fmt.Sprintf(PropsMatchCountFmt, matchCount, len(m.setKeys))
	pad := width - ansi.StringWidth(left) - ansi.StringWidth(right) - 1
	if pad < 1 {
		pad = 1
	}
	return ansi.Truncate(left+strings.Repeat(" ", pad)+styleFaint.Render(right), width, "")
}

// handleSetKeysClick routes clicks on the "Set keys" sub-tab body (below
// the shared strip, already dispatched by handleClick) — the Set-keys
// mirror of Global SSH's handlePropertiesClick verbatim: a click on the
// filter row focuses the filter, a click on a master-list row selects it,
// everything else (the error state, the detail pane, out-of-bounds clicks)
// is inert.
func (m globalGitModel) handleSetKeysClick(x, y, width, height int) keyResult {
	if m.setKeysErr != "" {
		return keyResult{model: m}
	}
	if y == subTabStripRows() {
		m.filterFocused = true
		m.filter.Focus()
		return keyResult{model: m, handled: true}
	}
	if x >= masterListWidth(width) || y < ggitSetKeysTopLines() {
		return keyResult{model: m}
	}
	filtered := m.ggitFilteredKeys()
	w := m.ggitSetKeysComputeScrollWindow(len(filtered), height)
	row, ok := ggitSetKeysRowForScreenRow(w, y-ggitSetKeysTopLines())
	if !ok || row >= len(filtered) {
		return keyResult{model: m}
	}
	m.setKeysDetailKey = filtered[row].Key
	return keyResult{model: m, handled: true}
}

// setKeyRow renders one master-list "Set keys" row: a leading space, the
// key (padded for column alignment), then the value — ONE line (not
// optionRow's 2-line shape: this list has no toggle/apply affordance and
// needs no sub-line budget), truncated with a visible cue. Mirrors
// propertyRow verbatim, reusing propKeyColumnWidth (screen-agnostic despite
// its prop* naming — the SAME column width both browsers use).
func setKeyRow(k GitSetKeyView, selected bool, width int) string {
	key := styleBold.Render(k.Key)
	if selected {
		key = styleSelected.Render(k.Key)
	}
	marker := "  "
	if selected {
		marker = styleBold.Render("▸ ")
	}
	line := " " + marker + padDisplay(key, propKeyColumnWidth) + k.Value
	return truncLine(line, width)
}

// renderSetKeys renders the "Set keys" sub-tab's flat, filterable
// master-detail body (PROP-02): the shared strip, the filter row with its
// live match count, the one-line-per-key master list (reusing
// ggitSetKeysComputeScrollWindow/gitCueLine), the detail pane (full key,
// full unclipped value, the origin path, the scope word, and — only when
// PolicyBacked — the frozen cross-reference note, reusing plan 09.5-01's
// PropsCrossReferenceNote constant rather than declaring a Git-specific
// twin), and the two DISTINCT empty states (probe failure vs. genuinely
// zero keys set).
func (m globalGitModel) renderSetKeys(strip string, width, height int) string {
	if m.setKeysErr != "" {
		return strip + "\n " +
			styleWarning.Render(PropsGitProbeFailedHeading) + "\n\n " +
			styleFaint.Render(PropsGitProbeFailedBody)
	}

	filtered := m.ggitFilteredKeys()
	body := strip + "\n" + m.setKeysFilterRow(width, len(filtered)) + "\n"

	// WR-17-class guard (mirrors renderOptions/renderProperties): a Backend
	// implementation may legitimately return (nil, nil) — zero rows, no
	// error. This is a DIFFERENT, non-alarming state from a filter matching
	// nothing below — no `!` prefix, styleFaint not styleWarning.
	if len(m.setKeys) == 0 {
		return body + " " + styleFaint.Render(PropsGitNoKeysSet)
	}
	if len(filtered) == 0 {
		return body + " " + styleFaint.Render(fmt.Sprintf(PropsGitNoFilterMatchFmt, m.filter.Value()))
	}

	listWidth := masterListWidth(width)
	detailWidth := width - listWidth - masterDetailGutter
	rows := frameBodyRows(height) - ggitSetKeysTopLines()

	selIdx := m.ggitSetKeysDetailIndex(filtered)
	w := m.ggitSetKeysComputeScrollWindow(len(filtered), height)
	visible := filtered
	if w.needsScroll {
		visible = filtered[w.windowStart : w.windowStart+w.visibleRows]
	}
	var listRows []string
	if w.cue == gitCueUp {
		listRows = append(listRows, gitCueLine(w))
	}
	for i, k := range visible {
		absoluteIdx := w.windowStart + i
		listRows = append(listRows, setKeyRow(k, absoluteIdx == selIdx, listWidth))
	}
	if w.cue == gitCueDown {
		listRows = append(listRows, gitCueLine(w))
	}
	list := strings.Join(listRows, "\n")

	detail := filtered[selIdx]
	var d strings.Builder
	d.WriteString(" " + styleBold.Render(detail.Key) + "\n\n")
	d.WriteString(lipgloss.NewStyle().Width(detailWidth).Render(" "+detail.Value) + "\n\n")
	if detail.Origin != "" {
		d.WriteString(" " + styleFaint.Render(detail.Origin) + "\n")
	}
	if detail.Scope != "" {
		d.WriteString(" " + styleFaint.Render(detail.Scope) + "\n")
	}
	if detail.ValueCount > 1 {
		// WR-04: git config is legitimately multi-valued (a stacked
		// credential.helper, an --add-built list); Value/Scope/Origin above
		// carry only the LAST occurrence (git's own last-wins resolution).
		// This is the honesty disclosure so the screen never implies a
		// stacked key is single-valued.
		d.WriteString(" " + styleWarning.Render(fmt.Sprintf(PropsGitMultiValuedNoteFmt, detail.ValueCount)) + "\n")
	}
	if detail.PolicyBacked {
		// Informational only: never a second interactive affordance, and the
		// value is never rendered twice side-by-side (09.5-CONTEXT.md).
		// Reuses plan 09.5-01's PropsCrossReferenceNote constant verbatim —
		// no Git-specific twin.
		d.WriteString(" " + styleFaint.Render(PropsCrossReferenceNote) + "\n")
	}
	detailPane := fitPane(lipgloss.NewStyle().Width(detailWidth).Render(d.String()), rows)

	return body + joinMasterDetail(list, listWidth, detailPane, rows)
}

// handleClick implements mouseTarget: a click on an option row's checkbox
// glyph TOGGLES it like space (GlobalGit.tsx:127 — Checkbox onClick stops
// propagation), a click elsewhere in the row selects it, and the ceremony's
// buttons click through the shared ceremony zones. The banner and the
// detail pane are inert.
func (m globalGitModel) handleClick(x, y, width, height int, s DemoState) keyResult {
	if m.ceremonyOpen {
		body := m.view(s, width, height).body
		if next, key, ok := ceremonyClickKey(m.ceremony, body, x, y); ok {
			m.ceremony = next
			return m.handleKey(key, s)
		}
		return keyResult{model: m}
	}

	// Sub-tab strip click routing (09.5-02, Task 1 — this screen's FIRST
	// strip): rows 0..subTabStripRows()-1 are the strip, mirroring Global
	// SSH's own handleClick strip branch verbatim. The label row is derived
	// from subTabStripRows()/2 (never a hardcoded 1) and spans are read from
	// the ACTUAL rendered line via hitNeedle, so the click zones can never
	// drift from what renderSubTabStrip actually draws. Border rows stay
	// inert.
	stripRows := subTabStripRows()
	if y < stripRows {
		if y == stripRows/2 {
			body := m.view(s, width, height).body
			switch {
			case hitNeedle(body, x, y, ggitTabOptionsLabel):
				m.subTab = ggitOptions
				return keyResult{model: m, handled: true}
			case hitNeedle(body, x, y, ggitTabSetKeysLabel):
				m.subTab = ggitSetKeys
				return keyResult{model: m, handled: true}
			}
		}
		return keyResult{model: m}
	}
	if m.subTab == ggitSetKeys {
		return m.handleSetKeysClick(x, y, width, height)
	}

	// BL-03 (09.4-REVIEW.md independent re-review): handleKey guards the
	// text-edit state first (m.fieldEditing), but handleClick had no
	// equivalent guard — a click on another master-list row moved
	// m.detailKey while m.fieldEditing stayed true, so the fallback
	// name/email inputs disappeared from view() while every subsequent
	// keystroke kept routing into them. The pane owns the keys while
	// editing; a stray body click must not silently move the selection out
	// from under the focused input (the exact "mouse-driven field focus"
	// desync class this project has hit twice before, T-07-22).
	if m.fieldEditing {
		return keyResult{model: m}
	}
	if x >= masterListWidth(width) || y < gitTopLines(s) {
		return keyResult{model: m}
	}
	options := m.overlaidGitOptions(s)
	// The click's row index is computed from the WINDOW START, not from the
	// screen position alone — a scroll offset the click handler does not
	// know about silently toggles the wrong row, the exact defect class
	// this project has hit twice before on mouse-driven field focus
	// (07-04-PLAN.md T-07-22). gitRowForScreenRow bound-checks against the
	// VISIBLE window (not the full row count) and reports the cue line as
	// inert.
	w := m.gitComputeScrollWindow(len(options), s)
	row, ok := gitRowForScreenRow(w, y-gitTopLines(s))
	if !ok || row >= len(options) {
		return keyResult{model: m}
	}
	o := options[row]
	// Checkbox hit-test: only selectable rows carry a checkbox — the same ONE
	// predicate that gates the toggle key and the checkbox glyph (D-02, D-05)
	// — routed through Selectable() rather than re-checking a state field,
	// so the click, the toggle key, and the rendered glyph stay one decision.
	if o.Selectable() {
		body := m.view(s, width, height).body
		if hitNeedle(body, x, y, glyphToggleOff) ||
			hitNeedle(body, x, y, glyphToggleOn) {
			m.chosen = withToggled(m.chosen, o.Key)
			return keyResult{model: m, handled: true}
		}
	}
	m.detailKey = o.Key
	return keyResult{model: m, handled: true}
}

// view implements screenModel.
func (m globalGitModel) view(s DemoState, width, height int) screenView {
	options := m.overlaidGitOptions(s)

	var pending int
	for _, o := range options {
		if gitNeedsAttention(o) {
			pending++
		}
	}

	status := "Baseline applied. user.email stays untouched — identities own their author."
	tone := "info"
	if pending > 0 {
		status = fmt.Sprintf("%d baseline options not set — %s", pending, GlobalGitAdvisoryNote)
		tone = "warning"
	}

	if m.ceremonyOpen {
		// WR-02 precedent (Global SSH, 09.4-REVIEW.md): the crumb line
		// already names the pane ("Options"), so the ceremony body does NOT
		// re-render the 3-row bordered strip — redundant chrome eating into
		// the ceremony's already-tight row budget, the tightest on this
		// screen. Reachable from the ggitOptions sub-tab's own "a" key, OR
		// (PROP-03, 09.5-03) from the ggitSetKeys sub-tab's custom-key form
		// — ceremonyKind (set once at open time, WR-09) picks the correct
		// crumb rather than re-deriving it from m.subTab, which the form's
		// own close-before-open transition already left on ggitSetKeys
		// either way.
		ceremonyCrumb := "Options"
		if m.ceremonyKind == gitCeremonyCustomKey {
			ceremonyCrumb = strings.TrimSpace(ggitTabSetKeysLabel)
		}
		return screenView{
			body:         m.ceremony.view(width - 2),
			crumbs:       []string{ceremonyCrumb},
			status:       status,
			actions:      ceremonyFooterActions(),
			capturesKeys: true,
		}
	}

	// strip is Global Git's FIRST sub-tab strip (09.5-02, Task 1), drawn by
	// the SAME shared renderer Global SSH uses (D-D) — rendered at the top
	// of every non-ceremony body below (populated, optionsErr, zero-options,
	// and the Set keys placeholder), never omitted from one of them: a
	// missing strip on any state silently shifts the click hit-test's y
	// origin out from under gitTopLines' accounting.
	strip := renderSubTabStrip([]string{ggitTabOptionsLabel, ggitTabSetKeysLabel}, int(m.subTab))
	crumb := "Options"
	if m.subTab == ggitSetKeys {
		crumb = strings.TrimSpace(ggitTabSetKeysLabel)
	}

	if m.subTab == ggitSetKeys {
		// PROP-03 (09.5-03): the custom-key form takes over the ENTIRE body
		// while open — mirrors how m.fieldEditing's footer branch below
		// (Options sub-tab) replaces the normal action set wholesale rather
		// than layering the form on top of the master-detail body.
		if m.customKeyOpen {
			body := m.renderCustomKeyForm(strip, width)
			return screenView{
				body:   body,
				crumbs: []string{crumb},
				actions: []FooterAction{
					{Key: "Tab", Label: "next field"},
					{Key: "Enter", Label: "review & write"},
					{Key: "Esc", Label: "cancel"},
				},
				capturesKeys: true,
			}
		}
		// PROP-02 (09.5-02, Task 2): the real "Set keys" body — flat,
		// filterable, master-detail, mirroring plan 09.5-01's proven SSH
		// shape.
		body := m.renderSetKeys(strip, width, height)
		actions := []FooterAction{{Key: "←→", Label: ggitFooterCycleLabel}}
		if m.setKeysErr == "" {
			actions = append(actions,
				FooterAction{Key: "↑↓", Label: "select"},
				FooterAction{Key: "/", Label: "filter"},
				// PROP-03 (09.5-03): "n" opens the free-form custom-key
				// entry form — advertised here, mirroring "/"'s identical
				// setKeysErr-gated advertisement immediately above.
				FooterAction{Key: "n", Label: PropsAddCustomKeyLabel},
			)
		}
		return screenView{
			body:         body,
			crumbs:       []string{crumb},
			actions:      actions,
			capturesKeys: m.filterFocused,
		}
	}

	if m.optionsErr != "" {
		body := strip + "\n"
		if banner := findingsBanner(s, "Git", gitBannerBeyond); banner != "" {
			body += banner + "\n"
		}
		body += " " + styleWarning.Render("! "+m.optionsErr) + "\n\n " +
			styleFaint.Render("The option states could not be read from this machine.")
		return screenView{
			body: body,
			// status is deliberately NOT the outer `status` var here (UI
			// review finding, confirmed via the captured
			// global-git-probe-failure.txt PTY frame): `pending` above is
			// computed from `options`, which activate()'s error path
			// leaves empty — so `pending == 0` VACUOUSLY on a probe
			// failure, and the outer default ("Baseline applied...")
			// rendered as the status line even though nothing was applied
			// and the real state is unknown. A probe failure has nothing
			// honest to report as a baseline status; leave it blank.
			crumbs:  []string{crumb},
			actions: []FooterAction{{Key: "↑↓", Label: "select option"}, {Key: "←→", Label: ggitFooterCycleLabel}},
		}
	}

	// WR-17: a Backend implementation may legitimately return (nil, nil) —
	// zero rows, no error. handleKey guards len(options)==0 for key routing
	// and the optionsErr branch above covers fetch failure, but a
	// zero-row/no-error answer reached here and panicked on
	// options[selIdx] below (gitDetailIndex returns 0 on no match, and 0
	// is out of range for an empty slice). globalgit.Statuses always
	// returns len(Policy) rows today, but NoopGlobalGitPlanner exists
	// precisely to be substituted.
	if len(options) == 0 {
		body := strip + "\n"
		if banner := findingsBanner(s, "Git", gitBannerBeyond); banner != "" {
			body += banner + "\n"
		}
		body += " " + styleFaint.Render("No global Git options to show.")
		return screenView{
			body:    body,
			crumbs:  []string{crumb},
			actions: []FooterAction{{Key: "↑↓", Label: "select option"}, {Key: "←→", Label: ggitFooterCycleLabel}},
		}
	}

	listWidth := masterListWidth(width)
	detailWidth := width - listWidth - masterDetailGutter
	selIdx := m.gitDetailIndex(options)
	bodyRows := frameBodyRows(height) - gitTopLines(s)

	// The master list becomes a scrolling window over the row set
	// (07-UI-SPEC.md RESOLVED "overflow" row): when every row fits inside
	// the computed budget, scrollWin.needsScroll is false, the loop below
	// covers every row exactly as before this plan (byte-identical, zero
	// regression), and no cue line is emitted. When it does not fit,
	// exactly [windowStart, windowStart+visibleRows) renders plus one
	// reserved cue line.
	scrollWin := m.gitComputeScrollWindow(len(options), s)
	visible := options
	if scrollWin.needsScroll {
		visible = options[scrollWin.windowStart : scrollWin.windowStart+scrollWin.visibleRows]
	}

	var rows []string
	if scrollWin.cue == gitCueUp {
		rows = append(rows, gitCueLine(scrollWin))
	}
	for i, o := range visible {
		absoluteIdx := scrollWin.windowStart + i
		marker := "  "
		if absoluteIdx == selIdx {
			marker = styleBold.Render("▸ ")
		}
		// Checkbox: only selectable rows get a bracket toggle — driven by the
		// ONE Selectable predicate (needs-action + writable member keys + no
		// probe error), so a does/render mismatch cannot exist (D-02, D-05).
		// A non-selectable row still gets a NEUTRAL marker, never a blank
		// cell — the exact same reasoning already applied to toneGlyph's
		// NotApplicable case just below: every other row carries a visible
		// glyph in this column, so leaving it blank reads as a missing or
		// broken row rather than a deliberately non-interactive one.
		box := padDisplay(styleFaint.Render("·"), optionBoxWidth)
		if o.Selectable() {
			box = padDisplay(styleFaint.Render(glyphToggleOff), optionBoxWidth)
			if m.chosen[o.Key] {
				box = padDisplay(styleHealthy.Bold(true).Render(glyphToggleOn), optionBoxWidth)
			}
		}
		toneGlyph := styleHealthy.Render("✓")
		switch o.State {
		case GlobalGitNeedsAction, GlobalGitSetButDiffers:
			toneGlyph = styleWarning.Render("!")
		case GlobalGitNotApplicable:
			// A neutral marker, not a health-tone glyph (mirrors globalssh):
			// every other row carries a visible tone glyph, so a blank cell
			// here would read as a missing/broken row.
			toneGlyph = styleFaint.Render("·")
		}
		name := styleBold.Render(o.Key)
		if absoluteIdx == selIdx {
			name = styleSelected.Render(o.Key)
		}
		chip := ""
		if o.Key == "init.defaultBranch" {
			chip = "  " + styleWarning.Render("[main vs master]")
		}
		rows = append(rows, truncLine(" "+marker+box+toneGlyph+" "+name+chip, listWidth))
		rows = append(rows, truncLine("      "+styleFaint.Render(globalGitRowLine2(o)), listWidth))
	}
	if scrollWin.cue == gitCueDown {
		rows = append(rows, gitCueLine(scrollWin))
	}
	list := strings.Join(rows, "\n")

	detail := options[selIdx]
	var d strings.Builder
	if detail.Key == GlobalGitEmailFallbackKey {
		nameFocused := m.fieldFocus == 0 && m.fieldEditing
		emailFocused := m.fieldFocus == 1 && m.fieldEditing
		nameSelected := m.fieldFocus == 0 && !m.fieldEditing
		emailSelected := m.fieldFocus == 1 && !m.fieldEditing
		d.WriteString(gitFallbackFieldLine(GlobalGitNameFallbackKey, m.nameInput, nameFocused, nameSelected) + "\n")
		d.WriteString(gitFallbackFieldLine(GlobalGitEmailFallbackKey, m.emailInput, emailFocused, emailSelected))
		if !m.emailValid() {
			d.WriteString("  " + styleError.Render("needs @"))
		}
		d.WriteString("\n")
		d.WriteString(helperLine(GlobalGitEmailFallbackHelper, false) + "\n")
		d.WriteString(helperLine(GlobalGitEmailFallbackAdvisory, false) + "\n")
		// The guessed-name warning fires independently of user.useConfigOnly's
		// selection — it names a real, always-live half-works-by-construction
		// problem (D-04): git guesses the author NAME from the OS account
		// whenever a fallback email is set but the fallback name is not.
		if strings.TrimSpace(m.emailInput.Value()) != "" && strings.TrimSpace(m.nameInput.Value()) == "" {
			d.WriteString(" " + styleWarning.Render(GlobalGitGuessedNameWarning) + "\n")
		}
	} else {
		explanation := detail.OneLiner
		switch detail.Key {
		case "init.defaultBranch":
			explanation = GlobalGitDetailExplanation
		case "core.ignorecase":
			// The case-sensitivity caveat is appended to the existing
			// explanation, not baked into the frozen fixture OneLiner
			// (07-03-PLAN.md Task 3) — recommending false without saying git
			// can defeat it per-repository would be dishonest.
			explanation += " " + GlobalGitCaseSensitivityCaveat
		}
		d.WriteString(" " + styleBold.Render(detail.Key) + "\n")
		d.WriteString(" " + styleInfo.Render("~ "+GlobalGitAdvisoryNote) + "\n\n")
		d.WriteString(" " + explanation + "\n")
		for _, note := range detail.BundlePerKeyNotes {
			// Per-key "yours differs — yours wins" notes for a bundle row's
			// members the user set differently (D-09).
			d.WriteString(" " + styleFaint.Render(note) + "\n")
		}
		if detail.GateNotMet {
			// The STATIC half of the hard-gate explanation — shown only when
			// the gate is not met, kept separate from the dynamic VersionNote
			// line below so this sentence stays freezable.
			d.WriteString(" " + styleWarning.Render(GlobalGitConflictStyleGateNote) + "\n")
		}
		if detail.Key == "user.useConfigOnly" && m.chosen["user.useConfigOnly"] {
			// D-07's mandatory cross-warning: selecting the fail-loud row
			// while the fallback pair has exactly one half set means an
			// unmatched commit will hard-fail — the user is told before
			// confirming. Rendered on this row's own detail pane, since
			// that's where the checkbox the warning is ABOUT lives.
			name, email := strings.TrimSpace(m.nameInput.Value()), strings.TrimSpace(m.emailInput.Value())
			switch {
			case email != "" && name == "":
				d.WriteString(" " + styleWarning.Render(GlobalGitCrossWarningNameMissing) + "\n")
			case name != "" && email == "":
				d.WriteString(" " + styleWarning.Render(GlobalGitCrossWarningEmailMissing) + "\n")
			}
		}
		if detail.VersionNote != "" {
			// The NON-contractual dynamic version line (D-13 precedent) —
			// excluded from the copy-freeze gate.
			d.WriteString(" " + styleFaint.Render(" "+detail.VersionNote) + "\n")
		}
		if detail.Provenance != "" {
			d.WriteString(" " + styleFaint.Render(detail.Provenance) + "\n")
		}
		if detail.ProbeError != "" {
			d.WriteString(" " + styleWarning.Render("! "+detail.ProbeError) + "\n")
		}
	}
	// Wrap to the pane width, then clip with a VISIBLE cue — long option
	// explanations must never be silently cut mid-sentence (H3).
	detailPane := fitPane(lipgloss.NewStyle().Width(detailWidth).Render(d.String()), bodyRows)

	body := strip + "\n"
	if banner := findingsBanner(s, "Git", gitBannerBeyond); banner != "" {
		body += banner + "\n"
	}
	body += joinMasterDetail(list, listWidth, detailPane, bodyRows)

	if m.fieldEditing {
		return screenView{body: body, crumbs: []string{crumb}, status: status, statusTone: tone,
			actions:      []FooterAction{{Key: "Esc/Enter", Label: "done editing"}, {Key: "Tab", Label: "next field"}},
			capturesKeys: true}
	}
	chosen := m.gitApplyChosen(options)
	actions := []FooterAction{{Key: "↑↓", Label: "select option"}, {Key: "space", Label: "toggle"}, {Key: "←→", Label: ggitFooterCycleLabel}}
	if m.detailKey == GlobalGitEmailFallbackKey {
		actions = []FooterAction{{Key: "↑↓", Label: "select option"}, {Key: "Tab", Label: "next field"}, {Key: "Enter", Label: "edit"}, {Key: "←→", Label: ggitFooterCycleLabel}}
	}
	switch {
	case len(chosen) > 0:
		actions = append(actions, FooterAction{Key: "a", Label: fmt.Sprintf("apply %d selected", len(chosen))})
	case m.detailKey == GlobalGitEmailFallbackKey && m.fallbackApplyOffered():
		actions = append(actions, FooterAction{Key: "a", Label: "set global fallback author"})
	}
	return screenView{body: body, crumbs: []string{crumb}, status: status, statusTone: tone, actions: actions}
}

// renderCustomKeyForm renders the PROP-03 (09.5-03) free-form custom-key
// entry form — reachable via "n" on the Set keys sub-tab. Reuses
// gitFallbackFieldLine VERBATIM (the SAME two-field visual contract the D9
// fallback pair already uses) with its OWN independent key/value pair,
// never sharing rendering state with nameInput/emailInput. Unlike the D9
// pane (which distinguishes a Tab-selected-but-not-editing row from an
// Enter-activated editing row), this form has no separate pre-edit
// selection state — opening it via "n" focuses the key field immediately —
// so the focused field always renders in the "editing" (bright) box, never
// the intermediate "selected" one.
func (m globalGitModel) renderCustomKeyForm(strip string, width int) string {
	var d strings.Builder
	d.WriteString(strip + "\n")
	d.WriteString(" " + styleBold.Render(PropsAddCustomKeyLabel) + "\n\n")
	d.WriteString(gitFallbackFieldLine("Key", m.customKeyInput, m.customFieldFocus == 0, false) + "\n")
	d.WriteString(gitFallbackFieldLine("Value", m.customValueInput, m.customFieldFocus == 1, false) + "\n")
	if m.customKeyFormErr != "" {
		d.WriteString("\n " + styleError.Render(m.customKeyFormErr) + "\n")
	}
	return lipgloss.NewStyle().Width(width).Render(d.String())
}

// gitFallbackFieldLine renders one Global Git fallback name/email row with
// two distinct focus states (WR-15, 09.4-REVIEW.md independent re-review):
// "editing" is formFieldLine's existing bright focused box; "selected" is
// Tab having moved fieldFocus here without Enter starting edit mode yet —
// previously rendered identically to the unselected row, so pressing Tab
// produced no visible change at all.
func gitFallbackFieldLine(label string, input textinput.Model, editing, selected bool) string {
	if editing {
		return formFieldLine(label, input, true, false)
	}
	if selected {
		name := styleBold.Render(padRight(label, 16))
		return " " + styleFaint.Render("▸ ") + name + DefaultTheme.FieldBlurred.Render("["+input.Value()+"]")
	}
	return formFieldLine(label, input, false, false)
}

func (m *globalGitModel) focusFallbackField() {
	if m.fieldFocus == 0 {
		m.nameInput.Focus()
		m.emailInput.Blur()
		return
	}
	m.nameInput.Blur()
	m.emailInput.Focus()
}

// focusCustomKeyField mirrors focusFallbackField's exact shape for the
// PROP-03 (09.5-03) custom-key pair — customFieldFocus uses the SAME 0/1
// convention as fieldFocus.
func (m *globalGitModel) focusCustomKeyField() {
	if m.customFieldFocus == 0 {
		m.customKeyInput.Focus()
		m.customValueInput.Blur()
		return
	}
	m.customKeyInput.Blur()
	m.customValueInput.Focus()
}
