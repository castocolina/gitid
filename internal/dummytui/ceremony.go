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
)

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

// ceremonyModel is the 2-state ceremony component (state A: confirm,
// state B: receipt).
type ceremonyModel struct {
	cfg   ceremonyConfig
	done  bool
	typed textinput.Model
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

// handleKey routes one keystroke: Esc cancels (state A only), Enter — or
// `y` on non-destructive ceremonies — confirms when enabled, Enter on the
// receipt finishes; everything else feeds the typed-confirm input when
// destructive.
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

	b.WriteString(styleBold.Render(c.cfg.Heading) + "\n")
	b.WriteString(styleFaint.Render("Touches ") + strings.Join(c.cfg.Targets, styleFaint.Render(" · ")) + "\n")
	for _, bk := range c.cfg.Backups {
		b.WriteString(styleFaint.Render("Backup → ") + bk + " " + styleFaint.Render("(written first — restore it to undo)") + "\n")
	}
	b.WriteString(PreviewLabel("Exact change — everything outside the managed block is preserved verbatim") + "\n")
	b.WriteString(previewBlockClipped(c.cfg.Preview, c.cfg.PreviewDiff, width, 12) + "\n")
	if c.cfg.Destructive != nil {
		b.WriteString(styleError.Render(c.cfg.Destructive.Warning) + "\n")
		b.WriteString(styleError.Render("> ") + c.typed.View() + "\n")
	}
	// The affirmative action is NEVER default-focused when destructive —
	// Cancel carries the focused rendering (mirroring the web's autoFocus).
	confirm := " " + c.cfg.ConfirmLabel + " (Enter) "
	if c.confirmEnabled() {
		confirm = styleBold.Render(confirm)
	} else {
		confirm = styleFaint.Render(confirm + "— disabled until the confirm word matches")
	}
	b.WriteString("\n" + styleSelected.Render(" Cancel (Esc) ") + " " + confirm)
	return b.String()
}
