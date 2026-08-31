package dummytui

import (
	"strings"
	"testing"

	"github.com/castocolina/gitid/internal/tuikit"
)

// TestRunUploadForIdentityQuotesTitle proves WR-02 (review iteration 4): the
// D-08 register-key pane's preview commands must be shell-quoted exactly
// like FixtureBackend.RunUpload (the create-wizard sibling) already is —
// the D-07 title always contains spaces ("gitid: <identity> @ <machine>"),
// so an unquoted preview pasted into a shell runs a DIFFERENT command than
// the one gitid actually runs. RunUploadForIdentity is the method that
// feeds the D-08 register-key pane and the key-ceremony upload beat — the
// exact register-key-modal surface iteration-3's WR-05 named but left half
// applied.
func TestRunUploadForIdentityQuotesTitle(t *testing.T) {
	cmd := FixtureBackend{}.RunUploadForIdentity("personal")
	if cmd == nil {
		t.Fatal("RunUploadForIdentity(\"personal\") returned a nil cmd")
	}
	msg := cmd()
	run, ok := msg.(tuikit.UploadRunMsg)
	if !ok {
		t.Fatalf("cmd() delivered %T, want tuikit.UploadRunMsg", msg)
	}
	if len(run.View.Rows) == 0 {
		t.Fatal("UploadRunMsg.View.Rows is empty")
	}
	for _, row := range run.View.Rows {
		title := "gitid: personal @ demo-machine"
		quoted := "'" + title + "'"
		if !strings.Contains(row.Command, quoted) {
			t.Errorf("row.Command = %q, want the title shell-quoted as %q", row.Command, quoted)
		}
	}
}
