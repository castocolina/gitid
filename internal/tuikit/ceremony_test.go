package tuikit

import (
	"strings"
	"testing"
)

func plainCeremony() ceremonyModel {
	return newCeremony(ceremonyConfig{
		Heading:       "Write Host * managed block to ~/.ssh/config",
		Targets:       []string{"~/.ssh/config"},
		Backups:       []string{"~/.ssh/config.backup.2026-07-03T03-59-12Z"},
		Preview:       "+ IdentitiesOnly yes",
		PreviewDiff:   true,
		ResultMessage: "3 of 4 recommended options applied.",
	})
}

func destructiveCeremony() ceremonyModel {
	return newCeremony(ceremonyConfig{
		Heading:       `Delete EVERYTHING for "personal" (SSH + Git + key)`,
		Targets:       []string{"~/.ssh/config", "~/.gitconfig"},
		Backups:       []string{"~/.ssh/config.backup.X", "~/.gitconfig.backup.X"},
		Preview:       "- Host personal.github.com (managed block removed)",
		PreviewDiff:   true,
		Destructive:   &FixDestructive{ConfirmWord: "personal", Warning: "This removes the key file too — it cannot be regenerated."},
		ResultMessage: `Identity "personal" deleted.`,
		ConfirmLabel:  "Delete",
	})
}

// typeWord feeds each rune of word into the ceremony's typed-confirm input.
func typeWord(c ceremonyModel, word string) ceremonyModel {
	for _, r := range word {
		c, _ = c.handleKey(pressKey(string(r)))
	}
	return c
}

func TestCeremonyStateAShowsBackupPromise(t *testing.T) {
	view := stripANSI(plainCeremony().view(80))
	for _, want := range []string{
		"Write Host * managed block to ~/.ssh/config",
		"Touches ~/.ssh/config",
		"Backup → ~/.ssh/config.backup.2026-07-03T03-59-12Z",
		"(written first — restore it to undo)",
		"+ IdentitiesOnly yes",
		"Cancel (Esc)",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("state A missing %q", want)
		}
	}
}

// TestCeremonyStateAWithNoBackupsNeverClaimsOne pins design-review finding
// U-1 (03-06 visual-regression gate, DLV-04.2): a target with nothing to
// back up (empty Backups — e.g. no pre-existing ~/.ssh/config) must NOT
// render the "(written first — restore it to undo)" line, since there is
// no backup line above it for that claim to refer to. It renders an
// explicit "nothing to back up" note instead — the ceremony must never be
// silent, or misleading, about its own backup state.
func TestCeremonyStateAWithNoBackupsNeverClaimsOne(t *testing.T) {
	c := newCeremony(ceremonyConfig{
		Heading:       `Create identity "acme"`,
		Targets:       []string{"~/.ssh/config"},
		Backups:       nil,
		Preview:       "+ Host acme.github.com",
		ResultMessage: `Identity "acme" created.`,
	})
	view := stripANSI(c.view(80))
	if strings.Contains(view, "written first — restore it to undo") {
		t.Errorf("state A with no backups still claims one was written first:\n%s", view)
	}
	if strings.Contains(view, "Backup → ") {
		t.Errorf("state A with no backups rendered a Backup → line:\n%s", view)
	}
	if !strings.Contains(view, "nothing to back up") {
		t.Errorf("state A with no backups is silent about its own backup state:\n%s", view)
	}
}

func TestCeremonyPlainConfirmThenReceipt(t *testing.T) {
	c := plainCeremony()
	c, outcome := c.handleKey(pressKey("enter"))
	if outcome != ceremonyConfirmed {
		t.Fatalf("enter outcome = %v, want confirmed", outcome)
	}
	receipt := stripANSI(c.view(80))
	for _, want := range []string{
		"✓ 3 of 4 recommended options applied.",
		"Wrote → ~/.ssh/config",
		"Backed up → ~/.ssh/config.backup.2026-07-03T03-59-12Z",
		"Done (Enter)",
	} {
		if !strings.Contains(receipt, want) {
			t.Errorf("receipt missing %q", want)
		}
	}
	_, outcome = c.handleKey(pressKey("enter"))
	if outcome != ceremonyFinished {
		t.Errorf("enter on receipt = %v, want finished (host dispatches now)", outcome)
	}
}

func TestCeremonyYConfirmsPlainWrites(t *testing.T) {
	_, outcome := plainCeremony().handleKey(pressKey("y"))
	if outcome != ceremonyConfirmed {
		t.Errorf("y outcome = %v, want confirmed on non-destructive ceremonies", outcome)
	}
}

func TestCeremonyEscCancelsStateAWithoutDispatch(t *testing.T) {
	_, outcome := plainCeremony().handleKey(pressKey("esc"))
	if outcome != ceremonyCancelled {
		t.Errorf("esc outcome = %v, want cancelled", outcome)
	}
	// Esc on the receipt is NOT a cancel — the write already "happened".
	c := plainCeremony()
	c, _ = c.handleKey(pressKey("enter"))
	_, outcome = c.handleKey(pressKey("esc"))
	if outcome != ceremonyNone {
		t.Errorf("esc on receipt = %v, want none", outcome)
	}
}

func TestCeremonyDestructiveGatesOnTypedWord(t *testing.T) {
	c := destructiveCeremony()

	// Enter before the word matches is a no-op.
	c, outcome := c.handleKey(pressKey("enter"))
	if outcome != ceremonyNone {
		t.Fatalf("enter before typing = %v, want none", outcome)
	}

	// Partial word: still disabled.
	c = typeWord(c, "perso")
	c, outcome = c.handleKey(pressKey("enter"))
	if outcome != ceremonyNone {
		t.Fatalf("enter on partial word = %v, want none", outcome)
	}

	// `y` must NOT confirm a destructive ceremony (it types into the field).
	c2 := destructiveCeremony()
	c2 = typeWord(c2, "personal")
	if c2.typed.Value() != "personal" {
		t.Fatalf("typed value = %q", c2.typed.Value())
	}
	c2, outcome = c2.handleKey(pressKey("y"))
	if outcome == ceremonyConfirmed {
		t.Error("y must not confirm a destructive ceremony")
	}
	if c2.typed.Value() != "personal"+"y" {
		t.Errorf("y should feed the typed input; value = %q", c2.typed.Value())
	}

	// Exact word: enter confirms.
	c = typeWord(c, "nal") // completes "personal"
	c, outcome = c.handleKey(pressKey("enter"))
	if outcome != ceremonyConfirmed {
		t.Errorf("enter with exact word = %v, want confirmed", outcome)
	}
	if !strings.Contains(stripANSI(c.view(80)), `Identity "personal" deleted.`) {
		t.Error("receipt missing the result message")
	}
}

func TestCeremonyDestructiveConfirmWordCanContainViewportKey(t *testing.T) {
	c := newCeremony(ceremonyConfig{
		Preview:     "a preview that can be focused",
		Destructive: &FixDestructive{ConfirmWord: "dev", Warning: "destructive"},
	})
	c = typeWord(c, "dev")
	if c.typed.Value() != "dev" {
		t.Fatalf("typed destructive confirmation = %q, want dev", c.typed.Value())
	}
	if c.preview.Focused {
		t.Fatal("typing v into a destructive confirmation must not focus the viewport")
	}
	_, outcome := c.handleKey(pressKey("enter"))
	if outcome != ceremonyConfirmed {
		t.Fatalf("enter after exact destructive confirmation = %v, want confirmed", outcome)
	}
}

// ---------------------------------------------------------------------------
// 03-07 Task 1 — the asynchronous create-commit ceremony (CR-01).
// ---------------------------------------------------------------------------

// asyncCeremony is the create-flow ceremony shape: Async means confirmation
// dispatches a backend commit and the receipt is reachable ONLY from that
// commit's explicit success result.
func asyncCeremony() ceremonyModel {
	return newCeremony(ceremonyConfig{
		Heading:       `Create identity "acme" — ed25519, test passed ✓`,
		Targets:       []string{"~/.ssh/config.d/gitid.config"},
		Backups:       []string{"~/.ssh/config.bak.<timestamp>"},
		Preview:       "+ Host acme.github.com",
		ResultMessage: `Identity "acme" created.`,
		ConfirmLabel:  "Write it",
		Async:         true,
	})
}

// TestCeremonyAsyncConfirmShowsNoReceiptUntilSuccess pins CR-01: confirming
// an Async ceremony enters an in-flight state — it must NOT mark the ceremony
// done, and no "Wrote →"/result copy may render before the backend's explicit
// success result arrives.
func TestCeremonyAsyncConfirmShowsNoReceiptUntilSuccess(t *testing.T) {
	c, outcome := asyncCeremony().handleKey(pressKey("enter"))
	if outcome != ceremonyConfirmed {
		t.Fatalf("confirm outcome = %v, want confirmed (the host dispatches the commit)", outcome)
	}
	if c.done {
		t.Fatal("an Async ceremony must NOT be done on confirmation — the write has not run yet")
	}
	pending := stripANSI(c.view(80))
	for _, banned := range []string{"Wrote →", "Backed up →", `Identity "acme" created.`, "Done (Enter)"} {
		if strings.Contains(pending, banned) {
			t.Errorf("the in-flight state renders receipt copy %q before any success result:\n%s", banned, pending)
		}
	}

	// Keys are inert while the commit is in flight — no accidental dismiss.
	c, outcome = c.handleKey(pressKey("enter"))
	if outcome != ceremonyNone {
		t.Errorf("enter while the commit is in flight = %v, want none", outcome)
	}

	// The explicit success result renders the receipt — with the REAL backup
	// paths the commit reports, replacing the preview placeholders.
	c = c.commitSucceeded([]string{"~/.ssh/config.bak.1700000000"})
	receipt := stripANSI(c.view(80))
	for _, want := range []string{`✓ Identity "acme" created.`, "Wrote → ~/.ssh/config.d/gitid.config", "Backed up → ~/.ssh/config.bak.1700000000", "Done (Enter)"} {
		if !strings.Contains(receipt, want) {
			t.Errorf("receipt after the success result missing %q:\n%s", want, receipt)
		}
	}
	if strings.Contains(receipt, "bak.<timestamp>") {
		t.Errorf("the receipt still shows the backup placeholder after the real backup path arrived:\n%s", receipt)
	}
	_, outcome = c.handleKey(pressKey("enter"))
	if outcome != ceremonyFinished {
		t.Errorf("enter on the receipt = %v, want finished", outcome)
	}
}

// TestCeremonyAsyncFailureIsVisibleAndRetryable pins the failure half of
// CR-01: a failed commit renders the concrete error with a retry affordance,
// never a success claim, and Esc backs out.
func TestCeremonyAsyncFailureIsVisibleAndRetryable(t *testing.T) {
	c, _ := asyncCeremony().handleKey(pressKey("enter"))
	c = c.commitFailed("gitid: writing ssh config: permission denied")

	failed := stripANSI(c.view(80))
	for _, want := range []string{"gitid: writing ssh config: permission denied", "Retry (Enter)", "Cancel (Esc)"} {
		if !strings.Contains(failed, want) {
			t.Errorf("failure state missing %q:\n%s", want, failed)
		}
	}
	for _, banned := range []string{"Wrote →", `Identity "acme" created.`, "Done (Enter)"} {
		if strings.Contains(failed, banned) {
			t.Errorf("failure state renders success copy %q:\n%s", banned, failed)
		}
	}

	// Enter retries: the ceremony re-enters the in-flight state and reports
	// confirmed so the host re-dispatches the commit.
	c, outcome := c.handleKey(pressKey("enter"))
	if outcome != ceremonyConfirmed {
		t.Fatalf("enter on the failure state = %v, want confirmed (retry)", outcome)
	}
	if !c.pending {
		t.Error("retry must re-enter the in-flight state")
	}

	// Esc from the failure state cancels back to the host.
	c2, _ := asyncCeremony().handleKey(pressKey("enter"))
	c2 = c2.commitFailed("boom")
	_, outcome = c2.handleKey(pressKey("esc"))
	if outcome != ceremonyCancelled {
		t.Errorf("esc on the failure state = %v, want cancelled", outcome)
	}
}

// TestCeremonySyncFlowsAreUnchanged proves the Async seam is additive: the
// existing synchronous ceremonies (plain + destructive) still mark done on
// confirmation and render the receipt immediately.
func TestCeremonySyncFlowsAreUnchanged(t *testing.T) {
	c, outcome := plainCeremony().handleKey(pressKey("enter"))
	if outcome != ceremonyConfirmed || !c.done {
		t.Fatalf("sync plain confirm = (%v, done=%v), want (confirmed, true)", outcome, c.done)
	}
	if !strings.Contains(stripANSI(c.view(80)), "Wrote →") {
		t.Error("sync ceremonies keep their immediate receipt")
	}
}

func TestCeremonyDestructiveAffirmativeNeverDefaultFocused(t *testing.T) {
	raw := destructiveCeremony().view(80)
	// Cancel carries the focused (reverse-video) rendering…
	if !strings.Contains(raw, "\x1b[1;7m Cancel (Esc) ") && !strings.Contains(raw, "\x1b[7m Cancel (Esc) ") {
		t.Error("Cancel must be the default-focused action on destructive ceremonies")
	}
	// …and the affirmative renders disabled until the word matches.
	plain := stripANSI(raw)
	if !strings.Contains(plain, "disabled until the confirm word matches") {
		t.Error("affirmative must render disabled before the typed word matches")
	}
	if !strings.Contains(plain, "This removes the key file too") {
		t.Error("destructive warning missing")
	}
}
