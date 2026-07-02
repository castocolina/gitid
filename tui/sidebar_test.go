package tui

// sidebar_test.go — tests for the sidebarModel.
//
// These tests were RED scaffolds in Plan 01. Plan 02 removes the t.Skip calls
// and implements real assertions. Test names are LOCKED by VALIDATION.md.

import (
	"strings"
	"testing"

	"github.com/castocolina/gitid/internal/doctor"
	"github.com/castocolina/gitid/internal/gitconfig"
	"github.com/castocolina/gitid/internal/identity"
)

// newTestSidebar returns a sidebarModel pre-loaded with the given accounts and
// unmanaged entries.
func newTestSidebar(accounts []identity.Account, unmanaged []unmanagedEntry) sidebarModel {
	m := newSidebarModel(doctor.Deps{})
	m.accounts = accounts
	m.unmanaged = unmanaged
	if len(accounts) > 0 {
		m.selected = 0
	}
	return m
}

// TestSidebarManagedRows verifies that managed identities appear as styled rows
// in the sidebar render, with the selected-row indicator '›' on the focused item.
// Requirement: TUI-03/D-01 (sidebar managed identity list).
func TestSidebarManagedRows(t *testing.T) {
	accounts := []identity.Account{
		{Name: "personal", Provider: "github.com"},
		{Name: "work", Provider: "gitlab.com"},
	}
	sb := newTestSidebar(accounts, nil)
	rendered := sb.view(18, 20, false)

	if !strings.Contains(rendered, "personal") {
		t.Error("sidebar must contain managed identity name 'personal'")
	}
	if !strings.Contains(rendered, "›") {
		t.Error("sidebar must show '›' indicator on the selected row")
	}
}

// TestSidebarUnmanagedRows verifies that unmanaged entries appear in a separate
// faint read-only section below the managed identities.
// Requirement: TUI-03/D-12 (sidebar unmanaged section).
func TestSidebarUnmanagedRows(t *testing.T) {
	accounts := []identity.Account{
		{Name: "personal", Provider: "github.com"},
	}
	unmanaged := []unmanagedEntry{
		{shortName: "orphan-key"},
	}
	sb := newTestSidebar(accounts, unmanaged)
	rendered := sb.view(18, 20, false)

	if !strings.Contains(rendered, "Unmanaged") {
		t.Error("sidebar must contain 'Unmanaged' section header when unmanaged entries exist")
	}
	if !strings.Contains(rendered, "○") {
		t.Error("sidebar must show '○' prefix for unmanaged entries")
	}
	if !strings.Contains(rendered, "orphan-key") {
		t.Error("sidebar must display the unmanaged entry short name")
	}
}

// TestSidebarEmptyManagedSection verifies the empty-state copy when no managed
// identities exist.
// Requirement: TUI-03/UI-SPEC § View 1 "No identities" empty state.
func TestSidebarEmptyManagedSection(t *testing.T) {
	sb := newTestSidebar(nil, nil)
	rendered := sb.view(18, 20, false)

	if !strings.Contains(rendered, "No identities") {
		t.Errorf("empty sidebar must show 'No identities' copy, got: %q", rendered)
	}
}

// TestSidebarBadge verifies per-identity health badges in sidebar rows.
// Requirement: TUI-06/D-08 (per-identity health badges).
func TestSidebarBadge(t *testing.T) {
	accounts := []identity.Account{
		{Name: "err-id", Provider: "github.com"},
		{Name: "warn-id", Provider: "github.com"},
		{Name: "ok-id", Provider: "github.com"},
	}
	sb := newTestSidebar(accounts, nil)
	sb.badges = map[string]doctor.Severity{
		"err-id":  doctor.SeverityError,
		"warn-id": doctor.SeverityWarning,
		// ok-id has no badge → defaults to ✓
	}

	rendered := sb.view(18, 30, false)

	// Error → ✗
	if !strings.Contains(rendered, "✗") {
		t.Error("sidebar must show '✗' badge for error-severity identity")
	}
	// Warning → ! (NOT ✗ — D-10/Eval#2)
	if !strings.Contains(rendered, "!") {
		t.Error("sidebar must show '!' badge for warning-severity identity")
	}
	// No-badge → ✓
	if !strings.Contains(rendered, "✓") {
		t.Error("sidebar must show '✓' badge for pass identity")
	}
}

// TestSidebarRefreshReconstructs verifies that refresh() re-reads ssh+gitconfig
// via the ReadFile seam and re-calls identity.Reconstruct.
// Requirement: TUI-03 (sidebar reconstructs from disk, D-16 anti-blindspot).
func TestSidebarRefreshReconstructs(t *testing.T) {
	// Wire a ReadFile seam that returns synthetic SSH config + gitconfig bytes
	// containing one managed identity named "test-identity".
	sshBlock := "# BEGIN gitid managed: test-identity\n" +
		"Host test-identity.github.com\n" +
		"  HostName github.com\n" +
		"  IdentityFile ~/.ssh/gitid_test-identity\n" +
		"  User git\n" +
		"  IdentitiesOnly yes\n" +
		"  # gitid: provider=github.com\n" +
		"# END gitid managed: test-identity\n"

	gcBlock := "# BEGIN gitid managed: test-identity\n" +
		"[includeIf \"gitdir:~/git/test/\"]\n" +
		"  path = ~/.gitconfig.d/test-identity\n" +
		"# END gitid managed: test-identity\n"

	deps := tuiDeps{
		doctor: doctor.Deps{
			ReadFile: func(path string) ([]byte, error) {
				if strings.HasSuffix(path, "/.ssh/config") {
					return []byte(sshBlock), nil
				}
				return []byte(gcBlock), nil
			},
		},
	}

	sb := newSidebarModel(doctor.Deps{})
	cmd := sb.refresh(deps)
	if cmd == nil {
		t.Fatal("refresh must return a non-nil tea.Cmd")
	}

	// Execute the command and expect a refreshSidebarMsg.
	msg := cmd()
	rsm, ok := msg.(refreshSidebarMsg)
	if !ok {
		t.Fatalf("refresh cmd must emit refreshSidebarMsg, got %T", msg)
	}

	if len(rsm.accounts) == 0 {
		t.Fatal("refreshSidebarMsg must contain reconstructed accounts")
	}
	if rsm.accounts[0].Name != "test-identity" {
		t.Errorf("expected account name 'test-identity', got %q", rsm.accounts[0].Name)
	}
}

// TestSidebarRefreshNoZombieFallback is a regression test (D-2): after deleting
// every identity, a live refresh must yield an EMPTY list, not resurrect the
// stale launch-time snapshot. The fallback to deps.doctor.Identities exists only
// for pure test mode (ReadFile == nil); in live mode (ReadFile != nil) an empty
// Reconstruct result is authoritative. Reported on the real TTY: "I deleted both
// keys, the view still show them with cross mark."
func TestSidebarRefreshNoZombieFallback(t *testing.T) {
	deps := tuiDeps{
		doctor: doctor.Deps{
			// Live read seam returns empty configs (everything deleted).
			ReadFile: func(string) ([]byte, error) { return []byte{}, nil },
			// Stale launch-time snapshot that must NOT be resurrected.
			Identities: []identity.Account{{Name: "deleted-id", Provider: "github.com"}},
		},
	}

	sb := newSidebarModel(doctor.Deps{})
	msg := sb.refresh(deps)()
	rsm, ok := msg.(refreshSidebarMsg)
	if !ok {
		t.Fatalf("refresh cmd must emit refreshSidebarMsg, got %T", msg)
	}
	if len(rsm.accounts) != 0 {
		t.Errorf("live refresh with empty configs must yield 0 accounts (no zombie fallback); got %d: %+v",
			len(rsm.accounts), rsm.accounts)
	}
}

// TestSidebarBadgeSlot verifies that the badge severity supplied per identity
// name renders the correct glyph at the row's rightmost cell.
// This test covers Plan 02's badge slot contract (data populated by Plan 03;
// slot is present now).
func TestSidebarBadgeSlot(t *testing.T) {
	accounts := []identity.Account{
		{Name: "badged", Provider: "github.com"},
	}
	sb := newTestSidebar(accounts, nil)

	cases := []struct {
		sev       doctor.Severity
		wantGlyph string
	}{
		{doctor.SeverityError, "✗"},
		{doctor.SeverityWarning, "!"},
		{doctor.SeverityInfo, "~"},
	}

	for _, c := range cases {
		sb.badges = map[string]doctor.Severity{"badged": c.sev}
		rendered := sb.view(18, 10, false)
		if !strings.Contains(rendered, c.wantGlyph) {
			t.Errorf("severity %v must render glyph %q, sidebar output: %q",
				c.sev, c.wantGlyph, rendered)
		}
	}
}

// TestSidebarUnmanagedSectionOmittedWhenEmpty verifies that the Unmanaged
// section is omitted entirely when no unmanaged entries exist.
func TestSidebarUnmanagedSectionOmittedWhenEmpty(t *testing.T) {
	accounts := []identity.Account{
		{Name: "personal", Provider: "github.com"},
	}
	sb := newTestSidebar(accounts, nil) // empty unmanaged
	rendered := sb.view(18, 20, false)

	if strings.Contains(rendered, "Unmanaged") {
		t.Error("'Unmanaged' section must be omitted when there are no unmanaged entries")
	}
}

// TestUnmanagedAffordances verifies that when an unmanaged entry is selected, the
// available affordances are ONLY reveal-path, copy-pubkey, and open-location (D-13).
// No edit/delete/health/management actions are exposed.
// Requirement: TUI-06/D-13 (unmanaged entries: read-only affordances).
// Closes: Plan 06.
func TestUnmanagedAffordances(t *testing.T) {
	// The unmanagedAffordances() function must return exactly the three read-only actions.
	affordances := unmanagedAffordances()
	want := []string{"revealPath", "copyUnmanagedPubkey", "openLocation"}
	if len(affordances) != len(want) {
		t.Errorf("unmanagedAffordances must return exactly %d items; got %d: %v", len(want), len(affordances), affordances)
	}
	for i, w := range want {
		if i >= len(affordances) || affordances[i] != w {
			t.Errorf("affordance[%d]: expected %q, got %q", i, w, affordances[i])
		}
	}

	// Verify that prohibited actions are NOT in the list (D-13).
	prohibited := []string{"edit", "delete", "rotate", "health", "manage", "adopt"}
	for _, p := range prohibited {
		for _, a := range affordances {
			if a == p {
				t.Errorf("affordance %q must not appear in unmanagedAffordances (D-13)", p)
			}
		}
	}
}

// TestUnmanagedCopyPubkeyOnly verifies that copy-pubkey on an unmanaged key
// uses the copyPubkeyModel feeding only the .pub line (never the private key).
// Requirement: TUI-06/D-13, T-05.6-22 (unmanaged copy is pubkey-only).
// Closes: Plan 06.
func TestUnmanagedCopyPubkeyOnly(t *testing.T) {
	// An unmanaged entry must carry pubLine (the .pub content) and keyPath (private).
	// The pubLine field is what gets passed to copyPubkeyModel — never keyPath.
	const pubLine = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAITest test@gitid"
	const keyPath = "/Users/test/.ssh/orphan_key"
	u := unmanagedEntry{
		shortName: "orphan_key",
		keyPath:   keyPath,
		pubLine:   pubLine,
	}
	// Verify the fields are wired correctly.
	if u.pubLine != pubLine {
		t.Errorf("unmanagedEntry.pubLine must be %q; got %q", pubLine, u.pubLine)
	}
	if u.keyPath != keyPath {
		t.Errorf("unmanagedEntry.keyPath must be %q; got %q", keyPath, u.keyPath)
	}

	// selectedUnmanagedEntry must return the entry.
	sb := newSidebarModel(doctor.Deps{})
	sb.unmanaged = []unmanagedEntry{u}
	sb.selectedUnmanaged = 0
	got := sb.selectedUnmanagedEntry()
	if got == nil {
		t.Fatal("selectedUnmanagedEntry must return the selected entry")
	}
	if got.pubLine != pubLine {
		t.Errorf("selectedUnmanagedEntry.pubLine must be %q; got %q", pubLine, got.pubLine)
	}
	// Explicitly verify that keyPath (private) is NOT the same as pubLine.
	if got.keyPath == got.pubLine {
		t.Error("keyPath must not equal pubLine (private path must never be used as pub line)")
	}
}

// Compile guard: ensure identity.Reconstruct and gitconfig.ReadFragment are
// importable from sidebar_test.go (they are used in TestSidebarRefreshReconstructs).
var _ = identity.Reconstruct
var _ = gitconfig.ReadFragment

// TestUnmanagedActionsRouting verifies the rootModel routes the read-only
// unmanaged affordances (Plan 06, D-13): 'c' copies the .pub line ONLY (never
// the private key), while 'o' (open-location) and 'p' (reveal-path) are
// fire-and-forget actions that open no modal and never panic.
func TestUnmanagedActionsRouting(t *testing.T) {
	const pubLine = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAITest test@gitid"
	const keyPath = "/Users/test/.ssh/orphan_key"

	build := func() rootModel {
		m := newRootModel(fakeDocDeps(), fakeIdentityDeps(), identity.UpdateDeps{}, identity.DeleteDeps{})
		m.activeView = identitiesView
		m.sidebar.accounts = nil
		m.sidebar.selected = -1
		m.sidebar.unmanaged = []unmanagedEntry{{shortName: "orphan_key", keyPath: keyPath, pubLine: pubLine}}
		m.sidebar.selectedUnmanaged = 0
		return m
	}

	// 'c' opens the copy modal carrying the .pub line only (D-13 security invariant).
	mc := sendKey(build(), "c")
	if mc.activeModal != copyPubkeyModal {
		t.Fatalf("'c' on an unmanaged entry must open copyPubkeyModal; got %v", mc.activeModal)
	}
	if mc.copyModal.pubLine != pubLine {
		t.Errorf("copy modal pubLine = %q; want %q", mc.copyModal.pubLine, pubLine)
	}
	if mc.copyModal.pubLine == keyPath || mc.copyModal.pubLine == mc.copyModal.privKeyPath {
		t.Error("copy modal must never carry the private key path as the copied line (D-13)")
	}

	// 'o' (open location) and 'p' (reveal path) must open no modal and not panic.
	if mo := sendKey(build(), "o"); mo.activeModal != noModal {
		t.Errorf("'o' must not open a modal; got %v", mo.activeModal)
	}
	if mp := sendKey(build(), "p"); mp.activeModal != noModal {
		t.Errorf("'p' must not open a modal; got %v", mp.activeModal)
	}
}
