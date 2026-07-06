package dummytui

// ceremony.go is the Go mirror of
// .planning/design/mockup-src/src/demo/MutationCeremony.tsx — the
// compressed 2-state write ceremony (02-REDESIGN-SPEC.md §6) reused by
// every mutating flow (create, edit, delete, global apply, fixes):
//
//	A. Preview + confirm — the exact diff/managed-block, the target files,
//	   and the timestamped backup shown as a PROMISE inline; destructive
//	   rewrites additionally require a typed confirm word, and the
//	   affirmative action is never default-focused.
//	B. Result — a success receipt: message + `Wrote →` + `Backed up →`.
//
// The ceremony is a self-contained component model usable inside any pane;
// it reports cancel/finish outcomes to the host pane, which dispatches the
// reducer action on ceremonyFinished.

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
)

// maxInt returns the larger of a and b.
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ceremonyOutcome is what a keystroke did to the ceremony.
type ceremonyOutcome int

const (
	// ceremonyNone: the key changed nothing the host cares about.
	ceremonyNone ceremonyOutcome = iota
	// ceremonyCancelled: Esc in state A — the host returns to its pane
	// WITHOUT dispatching anything (never destructive).
	ceremonyCancelled
	// ceremonyConfirmed: the write was confirmed — state B (receipt) shows.
	ceremonyConfirmed
	// ceremonyFinished: the receipt was acknowledged — the host dispatches
	// the reducer action now.
	ceremonyFinished
)

// ceremonyConfig declares one write ceremony.
type ceremonyConfig struct {
	Heading       string
	Targets       []string
	Backups       []string
	Preview       string
	PreviewDiff   bool
	Destructive   *FixDestructive
	ResultMessage string
	ConfirmLabel  string
}

// ceremonyFocus is which state-A control carries the focus.
type ceremonyFocus int

const (
	// ceremonyFocusPrimary: nothing was explicitly focused — Enter falls
	// through to the primary action (confirm when enabled), mirroring the
	// web's ceremony-level Enter handler (MutationCeremony useLocalKeys).
	// Cancel carries the focused RENDERING (the web's Cancel autoFocus) —
	// the affirmative is never the default (§6).
	ceremonyFocusPrimary ceremonyFocus = iota
	// ceremonyFocusCancel: the user tabbed onto Cancel — Enter cancels.
	ceremonyFocusCancel
	// ceremonyFocusConfirm: the user tabbed onto the affirmative — Enter
	// confirms when enabled.
	ceremonyFocusConfirm
)

// ceremonyModel is the 2-state ceremony component (state A: confirm,
// state B: receipt).
type ceremonyModel struct {
	cfg   ceremonyConfig
	done  bool
	typed textinput.Model
	focus ceremonyFocus
}

// newCeremony builds a ceremony in state A. For destructive ceremonies the
// typed-confirm input is focused (the affirmative action never is).
func newCeremony(cfg ceremonyConfig) ceremonyModel {
	if cfg.ConfirmLabel == "" {
		cfg.ConfirmLabel = "Confirm write"
	}
	ti := textinput.New()
	ti.Prompt = ""
	if cfg.Destructive != nil {
		ti.Placeholder = `Type "` + cfg.Destructive.ConfirmWord + `" to enable the destructive action`
		ti.Focus()
	}
	return ceremonyModel{cfg: cfg, typed: ti}
}

// confirmEnabled reports whether the confirm action is enabled — always
// for plain writes, only after the typed word matches exactly for
// destructive ones.
func (c ceremonyModel) confirmEnabled() bool {
	return c.cfg.Destructive == nil || c.typed.Value() == c.cfg.Destructive.ConfirmWord
}

// toggleFocus moves the state-A button focus Cancel ↔ Confirm (Tab, and
// ←/→ once a button is the focused slot). From the primary (untouched)
// state — where Cancel merely renders focused — the first move lands on
// the affirmative, matching a native Tab off the web's autofocused Cancel.
func (c ceremonyModel) toggleFocus() ceremonyModel {
	if c.focus == ceremonyFocusConfirm {
		c.focus = ceremonyFocusCancel
	} else {
		c.focus = ceremonyFocusConfirm
	}
	return c
}

// handleKey routes one keystroke: Esc cancels (state A only); Tab (or ←/→
// while a button is focused) toggles Cancel ↔ Confirm; Enter activates the
// focused button — or, while the typed-confirm field / primary state owns
// the focus, falls through to the primary action (confirm when enabled),
// exactly like the web's ceremony-level Enter handler. `y` confirms
// non-destructive ceremonies; anything else feeds the typed-confirm input
// when destructive. Enter on the receipt finishes.
func (c ceremonyModel) handleKey(msg tea.KeyMsg) (ceremonyModel, ceremonyOutcome) {
	key := msg.String()
	if c.done {
		if key == "enter" {
			return c, ceremonyFinished
		}
		return c, ceremonyNone
	}
	switch {
	case key == "esc":
		return c, ceremonyCancelled
	case key == "tab" || key == "shift+tab":
		return c.toggleFocus(), ceremonyNone
	case (key == "left" || key == "right") &&
		(c.cfg.Destructive == nil || c.focus != ceremonyFocusPrimary):
		// On destructive ceremonies the primary state keeps ←/→ for the
		// typed-confirm input's cursor; button slots move like Tab.
		if c.cfg.Destructive == nil && c.focus == ceremonyFocusPrimary {
			c.focus = ceremonyFocusCancel // leave the primary state first
		}
		return c.toggleFocus(), ceremonyNone
	case key == "enter" && c.focus == ceremonyFocusCancel:
		return c, ceremonyCancelled
	case key == "enter" || (key == "y" && c.cfg.Destructive == nil):
		if c.confirmEnabled() {
			c.done = true
			return c, ceremonyConfirmed
		}
		return c, ceremonyNone
	default:
		if c.cfg.Destructive != nil {
			c.typed, _ = c.typed.Update(msg)
		}
		return c, ceremonyNone
	}
}

// view renders the ceremony: state A (preview + backup promise + confirm)
// or state B (receipt with Wrote → / Backed up → lines).
func (c ceremonyModel) view(width int) string {
	var b strings.Builder
	if c.done {
		b.WriteString(styleHealthy.Render("✓ "+c.cfg.ResultMessage) + "\n\n")
		for _, t := range c.cfg.Targets {
			b.WriteString(styleFaint.Render("Wrote → ") + t + "\n")
		}
		for _, bk := range c.cfg.Backups {
			b.WriteString(styleFaint.Render("Backed up → ") + bk + "\n")
		}
		b.WriteString("\n" + styleSelected.Render(" Done (Enter) "))
		return b.String()
	}

	wrap := lipgloss.NewStyle().Width(maxInt(20, width-2))
	b.WriteString(styleBold.Render(c.cfg.Heading) + "\n")
	b.WriteString(styleFaint.Render(wrap.Render("Touches "+strings.Join(c.cfg.Targets, " · "))) + "\n")
	for _, bk := range c.cfg.Backups {
		b.WriteString(styleFaint.Render("Backup → ") + bk + "\n")
	}
	b.WriteString(styleFaint.Render("  (written first — restore it to undo)") + "\n")
	// Routed through the bounded, titled PreviewBlock (review-findings F1):
	// the title is spliced into the border's top edge instead of a separate
	// PreviewLabel row, saving one row per ceremony — this component is
	// shared by every mutating flow (create, edit, delete, global apply,
	// fixes), so this change applies everywhere ceremony.view renders. The
	// wording is shortened from the original PreviewLabel text to fit the
	// narrowest caller's pane width (identities.go's detailWidth=62).
	b.WriteString(PreviewBlock("Exact change — everything else preserved verbatim", c.cfg.Preview, c.cfg.PreviewDiff, width, 10) + "\n")
	if c.cfg.Destructive != nil {
		b.WriteString(styleError.Render(wrap.Render(c.cfg.Destructive.Warning)) + "\n")
		b.WriteString(styleError.Render("> ") + c.typed.View() + "\n")
	}
	// The affirmative action is NEVER default-focused — Cancel carries the
	// focused rendering until the user tabs onto the affirmative
	// (mirroring the web's Cancel autoFocus; Tab/←→ move the reverse-video
	// focus ring like every other button pair).
	cancel := " " + c.cancelLabel() + " "
	if c.focus == ceremonyFocusConfirm {
		cancel = styleBold.Render(cancel)
	} else {
		cancel = styleSelected.Render(cancel)
	}
	confirm := " " + c.confirmText() + " "
	switch {
	case !c.confirmEnabled():
		// D7 (checkpoint-2 contract) forbids the generic `— disabled`
		// suffix repo-wide (the extended copy-freeze grep) — this
		// destructive-confirm suffix keeps its pinned substring ("disabled
		// until the confirm word matches") but drops the leading em dash.
		confirm = styleFaint.Render(confirm + "(disabled until the confirm word matches)")
		if c.focus == ceremonyFocusConfirm {
			confirm = lipgloss.NewStyle().Faint(true).Reverse(true).
				Render(" " + c.confirmText() + " (disabled until the confirm word matches) ")
		}
	case c.focus == ceremonyFocusConfirm:
		confirm = styleSelected.Render(confirm)
	default:
		confirm = styleBold.Render(confirm)
	}
	b.WriteString("\n" + cancel + " " + confirm)
	return b.String()
}

// cancelLabel / confirmText / doneLabel are the exact button texts view
// renders — the click hit-tests derive their zones from these same strings
// so the two can never drift.
func (c ceremonyModel) cancelLabel() string { return "Cancel (Esc)" }
func (c ceremonyModel) confirmText() string { return c.cfg.ConfirmLabel + " (Enter)" }
func (c ceremonyModel) doneLabel() string   { return "Done (Enter)" }

// ceremonyClickKey maps a left click on a rendered block containing this
// ceremony to the key that button dispatches: Cancel → Esc, the affirmative
// → Enter (focused first, so Enter activates it even if the user had tabbed
// onto Cancel), the receipt's Done → Enter. Coordinates are relative to the
// block's top-left; hosts feed the returned key through their normal key
// path so clicks and keys share one code path.
func ceremonyClickKey(c ceremonyModel, block string, x, y int) (ceremonyModel, tea.KeyMsg, bool) {
	if c.done {
		if hitNeedle(block, x, y, " "+c.doneLabel()+" ") {
			key, _ := synthKey("Enter")
			return c, key, true
		}
		return c, nil, false
	}
	if hitNeedle(block, x, y, " "+c.cancelLabel()+" ") {
		key, _ := synthKey("Esc")
		return c, key, true
	}
	if hitNeedle(block, x, y, " "+c.confirmText()+" ") {
		c.focus = ceremonyFocusConfirm
		key, _ := synthKey("Enter")
		return c, key, true
	}
	return c, nil, false
}
