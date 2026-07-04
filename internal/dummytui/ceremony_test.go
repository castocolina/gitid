package dummytui

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
