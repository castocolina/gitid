package tui

// adopt.go — Adopt modal sub-model (Plan 07, Task 1).
//
// adoptModel implements the 4-step fragment adoption flow (ADOPT-01, UI-SPEC §2b):
//
//  1. adoptPhaseConfirm  — method selector (Migrate default per D-04) + preview.
//  2. adoptPhaseRunning  — adopt cmd dispatched; spinner while writing.
//  3. adoptPhaseDone     — write succeeded; offer-remove step for Migrate.
//  4. adoptPhaseOfferRemove — (Migrate only) y/N prompt; N default, never auto-removes.
//  5. adoptPhaseError    — write failed; error text shown.
//
// Security invariants:
//   - NO os/exec in this file — all effects go through deps.adopt (T-05.7-07-01).
//   - The remove step NEVER auto-deletes; two explicit confirmations required (D-05).
//   - runAdoptCmd calls adopter.Adopt which routes through filewriter (backup+atomic).

import (
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/castocolina/gitid/internal/adopter"
	"github.com/castocolina/gitid/internal/gitconfig"
)

// adoptPhase tracks the current step of the Adopt modal state machine.
type adoptPhase int

const (
	// adoptPhaseConfirm is the initial step: method selector + Enter to proceed.
	adoptPhaseConfirm adoptPhase = iota
	// adoptPhaseRunning means the adopt cmd was dispatched; waiting for result.
	adoptPhaseRunning
	// adoptPhaseDone means the adopt write succeeded; show offer-remove for Migrate.
	adoptPhaseDone
	// adoptPhaseOfferRemove is the optional remove-original step (Migrate only, D-05).
	adoptPhaseOfferRemove
	// adoptPhaseError means the adopt write failed; show error text.
	adoptPhaseError
)

// adoptModel is the Adopt modal sub-model.
//
// Mirror: tui/copy.go copyPubkeyModel — same struct + update + view pattern.
type adoptModel struct {
	// Input fields (set at construction).
	sourcePath   string
	identityName string
	method       adopter.AdoptMethod // default: AdoptMigrate (D-04)
	matches      []gitconfig.Match

	// State machine.
	phase   adoptPhase
	result  adopter.AdoptResult
	errText string

	// Offer-remove step (Migrate only, D-05).
	removeChoice bool // false = N (do not remove); only y+second-Enter removes

	deps tuiDeps
}

// newAdoptModel constructs an adoptModel for the given fragment.
// Default method is AdoptMigrate (D-04).
// Mirror: newCopyPubkeyModel (tui/copy.go lines 55-65).
func newAdoptModel(sourcePath, identityName string, matches []gitconfig.Match, deps tuiDeps) adoptModel {
	return adoptModel{
		sourcePath:   sourcePath,
		identityName: identityName,
		method:       adopter.AdoptMigrate, // D-04: default is migrate
		matches:      matches,
		phase:        adoptPhaseConfirm,
		deps:         deps,
	}
}

// update handles messages for the adopt modal.
// Mirror: copyPubkeyModel.update (tui/copy.go lines 75-94).
func (m adoptModel) update(msg tea.Msg) (adoptModel, tea.Cmd) {
	switch msg := msg.(type) {

	case adoptResultMsg:
		if msg.err != nil {
			m.phase = adoptPhaseError
			m.errText = msg.err.Error()
		} else {
			m.result = msg.result
			// Always land at adoptPhaseDone first (test contract: adoptResultMsg{err:nil}
			// must set adoptPhaseDone). For Migrate, the next Enter/continue key
			// transitions to adoptPhaseOfferRemove (the remove-original prompt) after
			// the user sees the "Adopt complete" confirmation.
			m.phase = adoptPhaseDone
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

// handleKey processes key presses within the adopt modal.
func (m adoptModel) handleKey(msg tea.KeyMsg) (adoptModel, tea.Cmd) {
	key := msg.String()

	switch m.phase {
	case adoptPhaseConfirm:
		switch key {
		case "enter":
			// Dispatch the adopt cmd.
			m.phase = adoptPhaseRunning
			return m, runAdoptCmd(m.sourcePath, m.identityName, m.matches, m.method, m.deps)
		case "m":
			m.method = adopter.AdoptMigrate
		case "r":
			m.method = adopter.AdoptReferenceInPlace
		case "tab", " ":
			// Toggle between migrate and reference in place.
			if m.method == adopter.AdoptMigrate {
				m.method = adopter.AdoptReferenceInPlace
			} else {
				m.method = adopter.AdoptMigrate
			}
		case "esc", "q":
			return m, func() tea.Msg { return adoptCancelMsg{} }
		}

	case adoptPhaseRunning:
		// Only Esc is handled while running (cancel is a best-effort no-op here;
		// the cmd has already been dispatched).
		if key == "esc" {
			return m, func() tea.Msg { return adoptCancelMsg{} }
		}

	case adoptPhaseDone:
		// For Migrate, Enter advances to the offer-remove step (D-05: never auto-removes).
		// For Reference, Enter/Esc simply closes the modal.
		if key == "enter" {
			if m.method == adopter.AdoptMigrate {
				m.phase = adoptPhaseOfferRemove
				return m, nil
			}
			return m, clearModalCmd()
		}
		if key == "esc" {
			return m, clearModalCmd()
		}

	case adoptPhaseOfferRemove:
		switch key {
		case "y":
			m.removeChoice = true
		case "n", "N":
			m.removeChoice = false
		case "enter":
			if m.removeChoice {
				// Two-step confirm: first Enter with y → dispatch remove cmd.
				m.phase = adoptPhaseRunning
				return m, runRemoveOriginalCmd(m.sourcePath, m.deps)
			}
			// N (default): close modal without removing.
			return m, clearModalCmd()
		case "esc":
			// Esc = keep (N default per D-05).
			return m, clearModalCmd()
		}

	case adoptPhaseError:
		if key == "esc" || key == "enter" {
			return m, func() tea.Msg { return adoptCancelMsg{} }
		}
	}

	return m, nil
}

// view renders the adopt modal at the given terminal width.
// Mirror: copyPubkeyModel.view (tui/copy.go lines 97-135).
func (m adoptModel) view(w int) string {
	mw := modalWidth(w)
	var sb strings.Builder

	// Title.
	sb.WriteString(StyleModalTitle.Render("Adopt Fragment: " + shortName(m.sourcePath, 20)))
	sb.WriteString("\n\n")

	switch m.phase {
	case adoptPhaseConfirm:
		sb.WriteString(StyleFaint.Render("Fragment: " + m.sourcePath))
		sb.WriteString("\n\n")
		sb.WriteString(StyleBody.Render("Choose adoption method:"))
		sb.WriteString("\n\n")

		// Method radio buttons.
		migrateRadio := "[ ]"
		refRadio := "[ ]"
		if m.method == adopter.AdoptMigrate {
			migrateRadio = "[x]"
		} else {
			refRadio = "[x]"
		}

		sb.WriteString("  " + StylePass.Render(migrateRadio) + " " + StyleBody.Render("Migrate"))
		sb.WriteString("\n")
		sb.WriteString(StyleFaint.Render("      (copy into ~/.gitconfig.d/, repoint includeIf)"))
		sb.WriteString("\n\n")
		sb.WriteString("  " + StylePass.Render(refRadio) + " " + StyleBody.Render("Reference in place"))
		sb.WriteString("\n")
		sb.WriteString(StyleFaint.Render("      (point includeIf at the original file)"))
		sb.WriteString("\n\n")
		sb.WriteString(StyleFaint.Render("Note: original file is preserved. After migrate you\nwill be asked whether to remove the original."))
		sb.WriteString("\n\n")
		sb.WriteString(StyleFaint.Render("[m/r] select  [tab] toggle  [enter] next  [esc] cancel"))

	case adoptPhaseRunning:
		sb.WriteString(StyleFaint.Render("[...] writing..."))

	case adoptPhaseDone:
		sb.WriteString(StylePass.Render("Adopt complete."))
		if m.result.MigratedPath != "" {
			sb.WriteString("\n")
			sb.WriteString(StylePass.Render("✓ Migrated to " + m.result.MigratedPath))
		}
		sb.WriteString("\n\n")
		if m.method == adopter.AdoptMigrate {
			sb.WriteString(StyleFaint.Render("[enter] next (remove original?)  [esc] done"))
		} else {
			sb.WriteString(StyleFaint.Render("[esc / enter] close"))
		}

	case adoptPhaseOfferRemove:
		sb.WriteString(StylePass.Render("Adopt complete."))
		if m.result.MigratedPath != "" {
			sb.WriteString("\n")
			sb.WriteString(StylePass.Render("✓ Migrated to " + m.result.MigratedPath))
		}
		sb.WriteString("\n\n")
		sb.WriteString(StyleBody.Render("Original file preserved at " + m.sourcePath))
		sb.WriteString("\n")
		sb.WriteString(StyleFaint.Render("It is now unreferenced (gitid points to the new copy)."))
		sb.WriteString("\n\n")

		choice := "N"
		if m.removeChoice {
			choice = "y"
		}
		sb.WriteString(StyleBody.Render("Remove the original? [y/N]:  [" + choice + "]"))
		sb.WriteString("\n\n")
		sb.WriteString(StyleFaint.Render("[y] yes  [n] no  [enter] confirm  [esc] keep"))

	case adoptPhaseError:
		sb.WriteString(SeverityStyle(0).Render("✗ adopt failed [critical]"))
		sb.WriteString("\n")
		sb.WriteString(StyleBody.Render(m.errText))
		sb.WriteString("\n\n")
		sb.WriteString(StyleFaint.Render("No changes were written. Press Esc to go back."))
	}

	return StyleModal.Width(mw).Render(sb.String())
}

// ─── Message types ────────────────────────────────────────────────────────────

// adoptResultMsg carries the outcome of a runAdoptCmd execution.
type adoptResultMsg struct {
	result adopter.AdoptResult
	err    error
}

// adoptCancelMsg signals the root model that the Adopt modal was cancelled.
// The root model handles this by closing the modal with noModal.
type adoptCancelMsg struct{}

// removeOriginalResultMsg carries the outcome of the optional remove-original step.
type removeOriginalResultMsg struct {
	backupPath string
	err        error
}

// ─── Commands ────────────────────────────────────────────────────────────────

// runAdoptCmd dispatches the adopt.Adopt call through the injected deps.adopt seam.
// NO os/exec in this function — all effects via deps.adopt (T-05.7-07-01).
//
// Mirror: runClipboardCopyCmd (tui/copy.go lines 143-148).
func runAdoptCmd(sourcePath, identityName string, matches []gitconfig.Match, method adopter.AdoptMethod, deps tuiDeps) tea.Cmd {
	return func() tea.Msg {
		// Derive gitconfigPath from home dir (the WriteIncludeIf closure captures it).
		home, err := os.UserHomeDir()
		if err != nil {
			return adoptResultMsg{err: err}
		}
		gitconfigPath := home + "/.gitconfig"
		result, adoptErr := adopter.Adopt(sourcePath, identityName, gitconfigPath, method, matches, deps.adopt)
		return adoptResultMsg{result: result, err: adoptErr}
	}
}

// runRemoveOriginalCmd dispatches the optional remove-original step via deps.adopt.BackupAndRemove.
// Called ONLY when the user explicitly confirms removal with y + Enter (D-05).
// NO os/exec — effects via deps.adopt (T-05.7-07-01).
func runRemoveOriginalCmd(sourcePath string, deps tuiDeps) tea.Cmd {
	return func() tea.Msg {
		backupPath, err := deps.adopt.BackupAndRemove(sourcePath)
		return removeOriginalResultMsg{backupPath: backupPath, err: err}
	}
}
