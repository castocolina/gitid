package tuikit

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
	"fmt"
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
	Heading string
	Targets []string
	Backups []string
	// Creates lists directories the transaction will create if absent
	// (CR-12/WR-16): e.g. a from-scratch ~/.gitconfig.d/, ~/git/<identity>/,
	// or ~/.ssh. Empty by default, so every existing ceremonyConfig literal
	// that never creates a directory is unaffected. Rendered as its own
	// disclosed line in state A, directly below "Touches" — a confirmation
	// ceremony that stays silent about a directory it is about to create is
	// not fully disclosing what the confirmed write will do.
	Creates       []string
	Preview       string
	PreviewDiff   bool
	Destructive   *FixDestructive
	ResultMessage string
	// ResultExtra is optional result content rendered directly below
	// ResultMessage. Key ceremonies use the shared test-stage renderer here.
	ResultExtra string
	// ResultHint is an OPTIONAL faint follow-on line rendered directly below
	// ResultMessage on the receipt (state B) — for a ceremony whose result
	// is not fully "done" (e.g. the create-flow's D-01 key-unused store:
	// design-review F4.2 found the receipt was a dead end with no pointer to
	// finish the job once the user leaves the wizard). Empty by default, so
	// every existing ceremonyConfig literal is unaffected.
	ResultHint string
	// ArchivePath is the D-06 key-retirement archive notice. It is empty for
	// ceremonies that do not retire existing key material.
	ArchivePath  string
	ConfirmLabel string
	// Hint is an optional faint row rendered under the destructive warning
	// (D-12 shared-key downgrade note). Empty by default.
	Hint string
	// ScanPreview is an optional extra preview under the warning and
	// above the default-no control (D-13 unmanaged-reference scan). Empty
	// means the warning block does not render at all.
	ScanPreview string
	// ScanDisclaimer is the D-13 honest disclaimer, rendered in full below
	// the clipped hit list so a long sentence is never lost to truncation.
	ScanDisclaimer string
	// PreviewMaxLines caps the Exact-change preview. Zero means the default
	// 10-line budget; delete shrinks this so the D-12/D-13 rows stay inside
	// the fixed frame.
	PreviewMaxLines int
	// Async means confirmation dispatches a backend commit and the receipt
	// is reachable ONLY from that commit's explicit success result. The
	// ceremony enters an in-flight state on confirmation and remains there
	// until commitSucceeded or commitFailed is called.
	Async bool
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
// state B: receipt). Async ceremonies add an in-flight pending state between
// confirmation and the explicit commit result.
type ceremonyModel struct {
	cfg       ceremonyConfig
	done      bool
	pending   bool
	commitErr string
	typed     textinput.Model
	focus     ceremonyFocus
	preview   ExactTextViewport
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
	return ceremonyModel{cfg: cfg, typed: ti, preview: ExactTextViewport{Text: cfg.Preview, VisibleLines: 10, Width: 58}}
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
//
// Async ceremonies: confirmation enters the in-flight pending state and
// returns ceremonyConfirmed so the host dispatches the backend commit. While
// pending, keys are inert. A failed result transitions to a retryable error
// state where Enter re-enters pending and Esc cancels.
func (c ceremonyModel) handleKey(msg tea.KeyMsg) (ceremonyModel, ceremonyOutcome) {
	key := msg.String()
	if c.done {
		if key == "enter" {
			return c, ceremonyFinished
		}
		return c, ceremonyNone
	}
	if c.pending {
		// In flight: ignore all keys until the commit result arrives.
		return c, ceremonyNone
	}
	if c.commitErr != "" {
		// Retryable failure state.
		switch key {
		case "enter":
			c.pending = true
			c.commitErr = ""
			return c, ceremonyConfirmed
		case "esc":
			return c, ceremonyCancelled
		default:
			return c, ceremonyNone
		}
	}
	// A destructive confirmation owns printable keys. In particular, `v` must
	// reach a confirm word such as "dev", not toggle the preview viewport.
	if key == "v" && c.cfg.Destructive == nil && c.preview.Text != "" {
		c.preview.Focused = !c.preview.Focused
		return c, ceremonyNone
	}
	if key == "pgdown" {
		c.preview = c.preview.ScrollDown(8)
		return c, ceremonyNone
	}
	if key == "pgup" {
		c.preview = c.preview.ScrollUp(8)
		return c, ceremonyNone
	}
	if c.preview.Focused {
		switch key {
		case "left":
			c.preview = c.preview.ScrollLeft(8)
			return c, ceremonyNone
		case "right":
			c.preview = c.preview.ScrollRight(8)
			return c, ceremonyNone
		case "up":
			c.preview = c.preview.ScrollUp(1)
			return c, ceremonyNone
		case "down":
			c.preview = c.preview.ScrollDown(1)
			return c, ceremonyNone
		}
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
			if c.cfg.Async {
				c.pending = true
			} else {
				c.done = true
			}
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

// commitSucceeded transitions an async ceremony from pending to the receipt
// state, replacing the preview backup placeholders with the real backup paths.
func (c ceremonyModel) commitSucceeded(backups []string) ceremonyModel {
	c.pending = false
	c.done = true
	c.commitErr = ""
	if len(backups) > 0 {
		c.cfg.Backups = backups
	}
	return c
}

// commitFailed transitions an async ceremony from pending to a retryable error
// state that renders the concrete error with Retry / Cancel affordances.
func (c ceremonyModel) commitFailed(err string) ceremonyModel {
	c.pending = false
	c.commitErr = err
	c.focus = ceremonyFocusPrimary
	return c
}

// receiptListMaxLines caps the receipt's Wrote→/Backed up→ lists (05-UI-
// REVIEW.md top fix #2): an everything-scope delete or global apply can
// touch dozens of files, and without a cap the list pushes the "Done
// (Enter)" CTA off the pane exactly like an unbounded PreviewBlock would —
// every other list-of-lines preview in this codebase clips with a visible
// cue instead of silently overflowing.
const receiptListMaxLines = 6

// renderReceiptList renders a receipt line list with the given label
// prefix, clipping to maxLines with a "+N more" cue so the CTA below it
// always stays visible.
func renderReceiptList(label string, entries []string, maxLines int) string {
	var b strings.Builder
	if len(entries) <= maxLines {
		for _, e := range entries {
			b.WriteString(styleFaint.Render(label) + e + "\n")
		}
		return b.String()
	}
	for _, e := range entries[:maxLines] {
		b.WriteString(styleFaint.Render(label) + e + "\n")
	}
	hidden := len(entries) - maxLines
	b.WriteString(styleFaint.Render(fmt.Sprintf("… (+%d more lines)", hidden)) + "\n")
	return b.String()
}

// view renders the ceremony: state A (preview + backup promise + confirm),
// state B (receipt with Wrote → / Backed up → lines), or for async ceremonies
// the in-flight pending state and the retryable failure state.
func (c ceremonyModel) view(width int) string {
	var b strings.Builder
	if c.done {
		b.WriteString(styleHealthy.Render("✓ "+c.cfg.ResultMessage) + "\n")
		if c.cfg.ResultExtra != "" {
			b.WriteString(c.cfg.ResultExtra)
		}
		if c.cfg.ResultHint != "" {
			b.WriteString(" " + styleFaint.Render(c.cfg.ResultHint) + "\n")
		}
		b.WriteString("\n")
		b.WriteString(renderReceiptList("Wrote → ", c.cfg.Targets, receiptListMaxLines))
		b.WriteString(renderReceiptList("Backed up → ", c.cfg.Backups, receiptListMaxLines))
		b.WriteString("\n" + styleSelected.Render(" Done (Enter) "))
		return b.String()
	}
	if c.pending {
		b.WriteString(styleFaint.Render("Writing…") + "\n\n")
		for _, t := range c.cfg.Targets {
			b.WriteString(styleFaint.Render("Will write → ") + t + "\n")
		}
		return b.String()
	}
	if c.commitErr != "" {
		b.WriteString(styleError.Render("✗ "+c.commitErr) + "\n\n")
		cancel := " " + c.cancelLabel() + " "
		retry := " Retry (Enter) "
		if c.focus == ceremonyFocusConfirm {
			cancel = styleBold.Render(cancel)
			retry = styleSelected.Render(retry)
		} else {
			cancel = styleSelected.Render(cancel)
			retry = styleBold.Render(retry)
		}
		b.WriteString(cancel + " " + retry)
		return b.String()
	}

	wrap := lipgloss.NewStyle().Width(maxInt(20, width-2))
	b.WriteString(styleBold.Render(c.cfg.Heading) + "\n")
	b.WriteString(styleFaint.Render(wrap.Render("Touches "+strings.Join(c.cfg.Targets, " · "))) + "\n")
	if len(c.cfg.Creates) > 0 {
		// CR-12/WR-16: disclose directory creation explicitly instead of
		// leaving it an undisclosed side effect of "Touches".
		b.WriteString(styleFaint.Render(wrap.Render("Creates "+strings.Join(c.cfg.Creates, " · ")+" (new directory)")) + "\n")
	}
	if len(c.cfg.Backups) > 0 || c.cfg.ArchivePath != "" {
		for _, bk := range c.cfg.Backups {
			b.WriteString(styleFaint.Render("Backup → ") + bk + "\n")
		}
		if c.cfg.ArchivePath != "" {
			b.WriteString(styleFaint.Render("Old key archived to ") + c.cfg.ArchivePath + "\n")
		}
		b.WriteString(styleFaint.Render("  (written first — restore it to undo)") + "\n")
	} else {
		// design-review U-1 (03-06 visual-regression gate): the explainer
		// line was previously unconditional, so a target with nothing to
		// back up (e.g. no pre-existing ~/.ssh/config) still claimed
		// "written first — restore it to undo" with no backup line above
		// it — a false safety claim about the ceremony's own undo story.
		b.WriteString(styleFaint.Render("  No existing file — nothing to back up.") + "\n")
	}
	// Routed through the bounded, titled PreviewBlock (review-findings F1):
	// the title is spliced into the border's top edge instead of a separate
	// PreviewLabel row, saving one row per ceremony — this component is
	// shared by every mutating flow (create, edit, delete, global apply,
	// fixes), so this change applies everywhere ceremony.view renders. The
	// wording is shortened from the original PreviewLabel text to fit the
	// narrowest caller's pane width (identities.go's detailWidth=62).
	if c.cfg.Destructive != nil {
		b.WriteString(styleError.Render(wrap.Render(c.cfg.Destructive.Warning)) + "\n")
	}
	if c.cfg.Hint != "" {
		b.WriteString(styleFaint.Render(wrap.Render(c.cfg.Hint)) + "\n")
	}
	if c.cfg.ScanPreview != "" {
		b.WriteString(previewBlockClipped(c.cfg.ScanPreview, false, width, 4) + "\n")
	}
	if c.cfg.ScanDisclaimer != "" {
		b.WriteString(styleWarning.Render(wrap.Render(c.cfg.ScanDisclaimer)) + "\n")
	}
	v := c.preview
	v.Width = maxInt(20, width-4)
	previewLines := 10
	if c.cfg.PreviewMaxLines > 0 {
		previewLines = c.cfg.PreviewMaxLines
	}
	v.VisibleLines = previewLines
	v = v.Clamp()
	hint := "Exact change: PgUp/PgDn scroll · v focus · ←/→ columns"
	if v.Focused {
		hint = "Exact change focused: PgUp/PgDn and ←/→ scroll"
	}
	b.WriteString(styleFaint.Render(hint) + "\n")
	b.WriteString(PreviewBlock("Exact change — everything else preserved verbatim", v.View(), c.cfg.PreviewDiff, width, previewLines) + "\n")
	b.WriteString(styleFaint.Render("Nothing has changed yet") + "\n")
	if c.cfg.Destructive != nil {
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
