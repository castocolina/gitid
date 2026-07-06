package dummytui

// app.go is the root Bubble Tea v2 model of the live gitid-dummy demo —
// the Go mirror of .planning/design/mockup-src/src/demo/DemoApp.tsx:
// four primary views in the persistent header nav (1 Identities ·
// 2 Global SSH · 3 Global Git · 4 Doctor — the Fixer is a consequence
// inside Doctor, FIX-02), contextual-only footer, live master-detail
// everywhere, no vim keys, `?` help with the full 8-state legend,
// `Ctrl+P` palette, and a real `q` quit prompt (unlike the browser demo,
// q here actually exits). All data is dummy and in-memory (DemoState).
//
// Key-routing precedence mirrors DemoApp.tsx: open overlay consumes keys
// first → the active screen's local handler (forms/ceremonies own their
// keys) → globals (1..4 tabs, ? help, ctrl+p palette, q quit prompt).

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

// screenView is what a screen contributes to the frame each render.
type screenView struct {
	body       string
	crumbs     []string
	status     string
	statusTone string
	actions    []FooterAction
	// capturesKeys: the active pane state consumes plain keys (text inputs,
	// selects, test/ceremony states, choosers) so `q` and `?` never reach
	// the globals — the frame renders the honest reserved footer
	// (Esc/Ctrl+P only; review batch 2 L1, batch 3 follow-up).
	capturesKeys bool
}

// keyResult is what a screen's key/message handler returns: the updated
// screen, reducer actions to dispatch, a command, whether the key was
// consumed, and an optional transient status note.
type keyResult struct {
	model   screenModel
	actions []Action
	cmd     tea.Cmd
	handled bool
	note    string
}

// screenModel is the contract every tab's child model implements. Handlers
// are pure over (model, state) so unit tests drive them without a
// terminal; reducer actions flow back to the App, the single Reduce caller.
type screenModel interface {
	handleKey(msg tea.KeyMsg, s DemoState) keyResult
	handleMsg(msg tea.Msg, s DemoState) keyResult
	view(s DemoState, width, height int) screenView
	// activate runs when the tab becomes active (e.g. Doctor's auto-scan).
	activate(s DemoState) (screenModel, tea.Cmd)
}

// mouseTarget is the optional click contract a screen implements: the App
// forwards left clicks with body-relative coordinates (x = frame column,
// y = 0 at the first body row) plus the frame geometry, so each screen
// hit-tests against the same layout its view renders (most re-render their
// own view and locate the clicked control's span in it — batch-1 pattern).
type mouseTarget interface {
	handleClick(x, y, width, height int, s DemoState) keyResult
}

// overlayKind is which app-level overlay (if any) owns the keys.
type overlayKind int

const (
	overlayNone overlayKind = iota
	overlayHelp
	overlayPalette
	overlayQuit
)

// helpKeys is the `?` overlay's key table — DemoApp.tsx's HELP_KEYS
// adapted to the terminal (q really quits; the palette lists views and
// actions — there are no browser reference routes here).
var helpKeys = [][2]string{
	{"1 · 2 · 3 · 4", "Switch view: Identities / Global SSH / Global Git / Doctor"},
	{"↑ ↓", "Move the selection — the detail pane updates live"},
	{"← →", "Switch sub-tabs (e.g. Options / Storage on Global SSH)"},
	{"Enter", "Activate the focused control / primary action of the pane"},
	{"Esc", "Back out one level (form → detail, modal → cancel). Never destructive"},
	{"Tab / Shift+Tab", "Move between fields and buttons in a form"},
	{"n · e · g · c · d", "Identities: new / edit SSH / configure Git / clone / delete"},
	{"f · F", "Doctor: fix the selected finding / fix all (each still previews)"},
	{"Ctrl+P", "Command palette — views and actions"},
	{"?", "This help"},
	{"q", "Quit gitid (asks first)"},
}

// legendRow is one row of the full MGR-02 state legend (spec §2: tone =
// health, pips = capability) — DemoApp.tsx's LEGEND copied verbatim.
type legendRow struct {
	state   string
	s, g    string
	meaning string
}

// helpLegend is the full 8-state legend shown in the `?` overlay.
var helpLegend = []legendRow{
	{"complete", "✓", "✓", "SSH Host block + Git fragment both present"},
	{"key-used-both", "✓", "✓", "Key wired for SSH auth AND commit signing"},
	{"key-used-ssh-only", "✓", "–", "Key wired for SSH; not for Git signing"},
	{"incomplete", "✓", "–", "SSH present; no Git identity yet"},
	{"git-only", "–", "✓", "Git identity relies on the global SSH config"},
	{"key-unused", "–", "–", "Key file exists; nothing references it"},
	{"key-missing", "✗", "–", "Host block references an absent key file"},
	{"fragment-path-missing", "✓", "✗", "includeIf points at a missing fragment"},
}

// paletteEntry is one Ctrl+P palette row.
type paletteEntry struct {
	label string
	tab   tabID
	help  bool
}

// paletteEntries mirrors DemoApp.tsx's palette views & actions (the static
// reference-mockup routes do not exist in the terminal demo).
var paletteEntries = []paletteEntry{
	{label: "1 · Identities", tab: tabIdentities},
	{label: "2 · Global SSH options", tab: tabGlobalSSH},
	{label: "3 · Global Git options", tab: tabGlobalGit},
	{label: "4 · Doctor", tab: tabDoctor},
	{label: "? · Help / key map / state legend", help: true},
}

// App is the root tea.Model: it owns DemoState (the single Reduce caller),
// the active tab, the per-tab child models, the overlays, and a transient
// status note.
type App struct {
	state   DemoState
	tab     tabID
	width   int
	height  int
	overlay overlayKind
	palette textinput.Model
	note    string
	screens [4]screenModel
	// initCmd is the initial tab's activation command — the activation
	// itself already ran in NewApp (Init's value receiver cannot retain
	// the activated screen model, so activating there would lose it).
	initCmd tea.Cmd
}

// NewApp builds the live demo app seeded from data.go's fixtures and runs
// the initial tab's activation hook, retaining the activated screen model.
func NewApp() App {
	ti := textinput.New()
	ti.Prompt = "> "
	ti.Placeholder = "Type to filter — Enter opens the first match"
	a := App{
		state:   Seed(),
		tab:     tabIdentities,
		width:   minFrameWidth,
		height:  minFrameHeight,
		palette: ti,
		screens: newScreens(),
	}
	screen, cmd := a.screens[a.tab].activate(a.state)
	a.screens[a.tab] = screen
	a.initCmd = cmd
	return a
}

// Init satisfies tea.Model — the first activation already happened in
// NewApp; Init only surfaces its command to the runtime.
func (a App) Init() tea.Cmd {
	return a.initCmd
}

// apply reduces every dispatched action into the app state.
func (a *App) apply(actions []Action) {
	for _, action := range actions {
		a.state = Reduce(a.state, action)
	}
}

// Update satisfies tea.Model — window sizing, key routing, and forwarding
// screen-owned messages (ticks) to every screen (each ignores what it does
// not own).
func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		return a, nil
	case tea.KeyMsg:
		return a.handleKey(msg)
	case tea.MouseClickMsg:
		return a.handleMouse(msg)
	default:
		var cmds []tea.Cmd
		for i := range a.screens {
			res := a.screens[i].handleMsg(msg, a.state)
			a.screens[i] = res.model
			a.apply(res.actions)
			if res.note != "" {
				a.note = res.note
			}
			if res.cmd != nil {
				cmds = append(cmds, res.cmd)
			}
		}
		return a, tea.Batch(cmds...)
	}
}

// setTab switches the active view and runs its activation hook.
func (a App) setTab(t tabID) (App, tea.Cmd) {
	a.tab = t
	a.note = ""
	screen, cmd := a.screens[t].activate(a.state)
	a.screens[t] = screen
	return a, cmd
}

// handleKey implements the DemoApp.tsx routing precedence: overlays first,
// then the active screen's local handler stack, then the globals.
func (a App) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Ctrl+P opens the palette from anywhere (mirroring the web handler).
	if key == "ctrl+p" && a.overlay != overlayPalette {
		a.overlay = overlayPalette
		a.palette.SetValue("")
		a.palette.Focus()
		return a, nil
	}

	switch a.overlay {
	case overlayHelp:
		if key == "esc" || key == "?" {
			a.overlay = overlayNone
		}
		return a, nil
	case overlayQuit:
		switch key {
		case "enter", "y":
			return a, tea.Quit
		case "esc":
			a.overlay = overlayNone
		}
		return a, nil
	case overlayPalette:
		switch key {
		case "esc":
			a.overlay = overlayNone
			return a, nil
		case "enter":
			matches := a.paletteMatches()
			a.overlay = overlayNone
			if len(matches) == 0 {
				return a, nil
			}
			if matches[0].help {
				a.overlay = overlayHelp
				return a, nil
			}
			next, cmd := a.setTab(matches[0].tab)
			return next, cmd
		default:
			a.palette, _ = a.palette.Update(msg)
			return a, nil
		}
	case overlayNone:
	}

	// Screen-local handler next: forms and ceremonies own their keys.
	a.note = ""
	res := a.screens[a.tab].handleKey(msg, a.state)
	a.screens[a.tab] = res.model
	a.apply(res.actions)
	if res.note != "" {
		a.note = res.note
	}
	if res.handled {
		return a, res.cmd
	}

	// Globals last.
	switch key {
	case "1", "2", "3", "4":
		next, cmd := a.setTab(tabID(int(key[0] - '1')))
		return next, cmd
	case "left":
		// D4 (checkpoint-2 contract): plain ←/→ switch views 1..4 at the
		// TOP LEVEL ONLY — reached here exactly because the active screen's
		// own handler returned unhandled (capturing panes and Global SSH's
		// ←/→ sub-tabs already consumed the key above and never reach this
		// branch). Clamped at the ends — no wraparound.
		if a.tab > tabIdentities {
			next, cmd := a.setTab(a.tab - 1)
			return next, cmd
		}
		return a, nil
	case "right":
		if a.tab < tabDoctor {
			next, cmd := a.setTab(a.tab + 1)
			return next, cmd
		}
		return a, nil
	case "?":
		a.overlay = overlayHelp
	case "q":
		a.overlay = overlayQuit
	}
	return a, nil
}

// handleMouse routes left clicks (spec §7 — every action is also a real
// button): header tab labels switch views, the header health chip opens the
// Doctor, contextual footer hints dispatch their advertised key, and body
// clicks go to the active screen's mouseTarget handler (rows select;
// buttons/checkboxes/radios dispatch the same path as their key). Overlays
// stay keyboard-driven; other buttons/mouse events are ignored.
func (a App) handleMouse(msg tea.MouseClickMsg) (tea.Model, tea.Cmd) {
	if msg.Button != tea.MouseLeft || a.overlay != overlayNone {
		return a, nil
	}
	if a.width < minFrameWidth || a.height < minFrameHeight {
		return a, nil // the too-small guard screen has no click targets
	}
	if msg.Y == 0 { // header row
		if t, ok := headerTabAt(msg.X); ok {
			return a.setTab(t)
		}
		if headerChipAt(a.width, a.state, msg.X) {
			return a.setTab(tabDoctor)
		}
		return a, nil
	}
	// The CONTEXTUAL footer line (row height-2: status · contextual ·
	// reserved): clicking a `<key> <label>` hint dispatches the very key it
	// advertises (Frame.tsx renders these as buttons with onActivate). The
	// reserved line stays keyboard-only — its keys are global anyway.
	if msg.Y == a.height-frameChromeBelow+1 {
		sv := a.screens[a.tab].view(a.state, a.width, a.height)
		if action, ok := footerActionAt(sv.actions, msg.X); ok {
			if key, ok := synthKey(action.Key); ok {
				return a.handleKey(key)
			}
		}
		return a, nil
	}
	bodyY := msg.Y - frameBodyTop
	if bodyY < 0 || bodyY >= a.height-frameBodyTop-frameChromeBelow {
		return a, nil // breadcrumb, status, and reserved-footer rows are inert
	}
	target, ok := a.screens[a.tab].(mouseTarget)
	if !ok {
		return a, nil
	}
	a.note = ""
	res := target.handleClick(msg.X, bodyY, a.width, a.height, a.state)
	a.screens[a.tab] = res.model
	a.apply(res.actions)
	if res.note != "" {
		a.note = res.note
	}
	return a, res.cmd
}

// synthKey converts a footer/button key hint into the tea.KeyMsg a real
// keypress produces, so a click dispatches the exact same code path. Only
// single-key hints synthesize; combined navigation hints ("↑↓", "Tab/↑↓",
// "←→") are not one action and stay inert.
func synthKey(key string) (tea.KeyMsg, bool) {
	switch key {
	case "Enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}, true
	case "Esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}, true
	case "space":
		return tea.KeyPressMsg{Code: tea.KeySpace}, true
	}
	if runes := []rune(key); len(runes) == 1 {
		return tea.KeyPressMsg{Code: runes[0], Text: key}, true
	}
	return nil, false
}

// mustKey is synthKey for hints the code itself supplies — every button
// maps to a real key by construction.
func mustKey(key string) tea.KeyMsg {
	msg, ok := synthKey(key)
	if !ok {
		panic("mustKey: unmappable key hint " + key)
	}
	return msg
}

// paletteMatches filters the palette entries by the typed query.
func (a App) paletteMatches() []paletteEntry {
	q := strings.ToLower(a.palette.Value())
	var out []paletteEntry
	for _, e := range paletteEntries {
		if q == "" || strings.Contains(strings.ToLower(e.label), q) {
			out = append(out, e)
		}
	}
	return out
}

// View satisfies tea.Model. Alt-screen and cell-motion mouse reporting are
// enabled via the tea.View fields (tea.WithAltScreen()/WithMouse* options
// do not exist in Bubble Tea v2).
func (a App) View() tea.View {
	v := tea.NewView(a.render())
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

// render composes the frame: the active overlay's body when one is open,
// otherwise the active screen's view.
func (a App) render() string {
	sv := a.screens[a.tab].view(a.state, a.width, a.height)
	status := sv.status
	tone := sv.statusTone
	if a.note != "" {
		status = a.note
		tone = "info"
	}
	crumbs := sv.crumbs
	actions := sv.actions
	body := sv.body
	capturesKeys := sv.capturesKeys

	switch a.overlay {
	case overlayHelp:
		body = a.renderHelp()
		crumbs = []string{"Help"}
		actions = []FooterAction{{Key: "Esc/?", Label: "close"}}
		capturesKeys = false
	case overlayQuit:
		body = a.renderQuitPrompt()
		crumbs = []string{"Quit"}
		actions = []FooterAction{{Key: "Enter", Label: "quit"}, {Key: "Esc", Label: "stay"}}
		capturesKeys = false
	case overlayPalette:
		body = a.renderPalette()
		crumbs = []string{"Palette"}
		actions = []FooterAction{{Key: "Enter", Label: "open first match"}, {Key: "Esc", Label: "close"}}
		capturesKeys = true // the palette filter input swallows q and ?
	case overlayNone:
	}
	// D4 (checkpoint-2 contract): advertise the top-level plain-arrow view
	// switch on non-capturing states, EXCEPT Global SSH — its own ←/→
	// already means "Options / Storage" there (that footer hint stays;
	// top-level arrows never reach the tab switcher from that screen).
	if a.overlay == overlayNone && !capturesKeys && a.tab != tabGlobalSSH {
		actions = append(actions, FooterAction{Key: "←→", Label: "switch view"})
	}
	return RenderFrame(a.width, a.height, a.state, a.tab, crumbs, status, tone, actions, capturesKeys, body)
}

// renderHelp renders the `?` overlay: the key map plus the full 8-state
// legend (tone glyph · state word · S pip · G pip · meaning).
func (a App) renderHelp() string {
	var b strings.Builder
	b.WriteString(" " + styleBold.Render("gitid — keys & state legend") + "\n")
	b.WriteString(" " + styleFaint.Render("Everything is dummy, in-memory data — actions really change the demo state (lists, badges,") + "\n")
	b.WriteString(" " + styleFaint.Render("header counts), but nothing on your machine is touched.") + "\n")
	for _, row := range helpKeys {
		b.WriteString("  " + styleBold.Render(padRight(row[0], 18)) + row[1] + "\n")
	}
	b.WriteString("\n " + styleFaint.Render("Identity state legend — tone glyph = health · S/G pips = capability (✓ wired · – none · ✗ broken)") + "\n")
	b.WriteString("  " + styleFaint.Render(padRight("tone", 6)+padRight("state", 23)+padRight("S", 3)+padRight("G", 3)+"meaning") + "\n")
	for _, row := range helpLegend {
		tone := toneStyle(IdentityManagerStateTone[row.state]).Render(padRight(IdentityManagerGlyphByState[row.state], 6))
		b.WriteString("  " + tone + styleBold.Render(padRight(row.state, 23)) + padRight(row.s, 3) + padRight(row.g, 3) + row.meaning + "\n")
	}
	return b.String()
}

// renderQuitPrompt renders the `q` prompt — a real quit, unlike the
// browser demo.
func (a App) renderQuitPrompt() string {
	var b strings.Builder
	b.WriteString("\n " + styleBold.Render("Quit gitid?") + "\n\n")
	b.WriteString(" " + styleFaint.Render("All data is dummy and in-memory — nothing on your machine was touched.") + "\n\n")
	b.WriteString(" " + styleSelected.Render(" Stay (Esc) ") + " " + styleBold.Render(" Quit (Enter) "))
	return b.String()
}

// renderPalette renders the Ctrl+P command palette.
func (a App) renderPalette() string {
	var b strings.Builder
	b.WriteString(" " + styleBold.Render("Command palette") + "\n")
	b.WriteString(" " + a.palette.View() + "\n\n")
	b.WriteString(" " + styleFaint.Render("Views & actions") + "\n")
	for i, e := range a.paletteMatches() {
		if i == 0 {
			b.WriteString("  " + styleSelected.Render(" "+e.label+" ") + "\n")
		} else {
			b.WriteString("   " + e.label + "\n")
		}
	}
	b.WriteString("\n " + styleFaint.Render("Esc closes"))
	return b.String()
}

// padRight pads s with spaces to width (display cells not counted — the
// callers pass ASCII-or-narrow strings).
func padRight(s string, width int) string {
	for len([]rune(s)) < width {
		s += " "
	}
	return s
}

// newScreens wires the four tab child models in header order.
func newScreens() [4]screenModel {
	return [4]screenModel{
		newIdentitiesModel(),
		newGlobalSSHModel(),
		newGlobalGitModel(),
		newDoctorModel(),
	}
}
