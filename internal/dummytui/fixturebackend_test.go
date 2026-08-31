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
// TestRunUploadForIdentityOmitsForNonGatedHost is the WR-02 regression
// (review iteration 5): RunUploadForIdentity used to branch on the fixture
// SSH host and fall through to the GitHub two-row success for anything that
// is NOT gitlab, including the empty string an identity with no SSHHost at
// all resolves to ("opensource"/"git-only" and "archived"/"key-unused" in
// IdentityManagerRows) — both reachable through the key ceremony, which
// dispatches RunUploadForIdentity unconditionally after any successful
// commit. The real backend's planUpload D-13 gate returns Skipped=true for
// provider == "" and renders nothing; RegisterKeyPlan's sibling probe
// already models this correctly (Omitted in the default case) — only
// RunUploadForIdentity fabricated a registration. Mirror RegisterKeyPlan's
// three-way branch: a non-gated host must return an empty, Skipped view.
func TestRunUploadForIdentityOmitsForNonGatedHost(t *testing.T) {
	for _, name := range []string{"opensource", "archived"} {
		t.Run(name, func(t *testing.T) {
			cmd := FixtureBackend{}.RunUploadForIdentity(name)
			if cmd == nil {
				t.Fatalf("RunUploadForIdentity(%q) returned a nil cmd", name)
			}
			msg := cmd()
			run, ok := msg.(tuikit.UploadRunMsg)
			if !ok {
				t.Fatalf("cmd() delivered %T, want tuikit.UploadRunMsg", msg)
			}
			if !run.View.Skipped {
				t.Errorf("View.Skipped = false, want true for a provider-less identity (matches planUpload's D-13 gate)")
			}
			if len(run.View.Rows) != 0 {
				t.Errorf("View.Rows = %+v, want empty — no fabricated registration for an identity with no provider", run.View.Rows)
			}
		})
	}
}

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
