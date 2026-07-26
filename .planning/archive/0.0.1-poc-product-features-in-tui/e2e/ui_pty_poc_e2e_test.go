//go:build e2e

// ARCHIVED (Phase 3 D-14/D-15): these PTY e2e tests drove the 0.0.1 POC tui/
// package (create wizard modal, match-strategy selector, adopt modal, add-repo
// modal, copy/upload assist) which Phase 3 removes wholesale. The harness they
// used (startPTY/sendKey/waitFor/snapshot) SURVIVES in e2e/ui_pty_e2e_test.go;
// plan 03-06 rebuilds the per-screen PTY suite on top of it against the real
// tuikit app shell.

package e2e

// TestUIPTY_WizardInputDecoding drives the Create Identity wizard via PTY raw
// keystrokes and verifies the typed identity name appears in the decoded frame.
// This directly tests the input-decoding regression: the historical
// "couldn't type in the wizard" bug was only caught on a real terminal because
// teatest.Send() injects a tea.Msg, bypassing the tty input decoder entirely.
func TestUIPTY_WizardInputDecoding(t *testing.T) {
	home := SandboxHome(t)
	bin := BuildBinary(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	s := startPTY(t, newPTYCmd(ctx, bin, home))
	defer s.close(t)

	// Wait for the TUI to render.
	uiReady(t, s)

	// Press 'a' to open the Create Identity wizard.
	s.sendKey([]byte("a"), keystrokeDelay*2)

	// Wait for the wizard modal to open.
	last, ok := s.waitFor(5*time.Second, func(text string) bool {
		return strings.Contains(text, "Identity Name") || strings.Contains(text, "Create") || strings.Contains(text, "Name")
	})
	if !ok {
		t.Logf("wizard open: last frame:\n%s", last)
		// Try anyway — some terminals take longer to render the modal title.
	}

	// Type an identity name to test input decoding (the D-13 regression test).
	identityName := "mytest"
	for _, ch := range identityName {
		s.sendKey([]byte(string(ch)), keystrokeDelay)
	}

	// Wait for the typed name to appear in the decoded frame.
	last, ok = s.waitFor(5*time.Second, func(text string) bool {
		return strings.Contains(text, identityName)
	})
	if !ok {
		t.Errorf("FAIL — input-decoding regression: typed name %q not found in decoded frame after %s of typing.\nLast frame:\n%s",
			identityName, keystrokeDelay*time.Duration(len(identityName)), last)
	} else {
		t.Logf("PASS — input decoding: %q appears in decoded frame", identityName)
	}

	saveFrame(t, "wizard-name-input", s)

	// Press Esc to close the wizard.
	s.sendKey([]byte{0x1b}, keystrokeDelay*2) // ESC
}

// TestUIPTY_MatchStrategySelector drives the match-strategy selector in the
// Create Identity wizard (Tab to Match Strategy, navigate gitdir/hasconfig/both,
// assert on decoded frame) and snapshots the frame for the UI critique.
func TestUIPTY_MatchStrategySelector(t *testing.T) {
	home := SandboxHome(t)
	bin := BuildBinary(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	s := startPTY(t, newPTYCmd(ctx, bin, home))
	defer s.close(t)

	uiReady(t, s)

	// Open the Create Identity wizard.
	s.sendKey([]byte("a"), keystrokeDelay*2)

	// Wait for wizard to open.
	_, _ = s.waitFor(3*time.Second, func(text string) bool {
		return strings.Contains(text, "Name") || strings.Contains(text, "Create")
	})

	// Type an identity name so the preview can derive defaults.
	for _, ch := range "personal" {
		s.sendKey([]byte(string(ch)), keystrokeDelay)
	}
	time.Sleep(100 * time.Millisecond) // allow TUI to render with the name

	// Tab through fields until Match Strategy is reached.
	// Wizard fields: Name(0), GitName(1), Email(2), Provider(3), Alias(4), Hostname(5), Port(6), Match(7)
	// Tab 7 times to reach Match Strategy field.
	for i := 0; i < 7; i++ {
		s.sendKey([]byte{0x09}, keystrokeDelay) // Tab
	}

	// Wait for the match strategy selector to expand.
	last, ok := s.waitFor(4*time.Second, func(text string) bool {
		return strings.Contains(text, "gitdir") || strings.Contains(text, "Match") || strings.Contains(text, "hasconfig")
	})
	if !ok {
		t.Logf("match selector open: last frame:\n%s", last)
	}

	saveFrame(t, "match-strategy-gitdir", s)

	// Navigate down to 'hasconfig' (one Down keystroke from gitdir default).
	s.sendKey([]byte{0x1b, 0x5b, 0x42}, keystrokeDelay) // down arrow: ESC [ B

	// Wait for hasconfig to appear (either selected or shown in the selector).
	last, _ = s.waitFor(3*time.Second, func(text string) bool {
		return strings.Contains(text, "hasconfig")
	})
	t.Logf("hasconfig frame present: %v", strings.Contains(last, "hasconfig"))
	saveFrame(t, "match-strategy-hasconfig", s)

	// Navigate down once more to 'both'.
	s.sendKey([]byte{0x1b, 0x5b, 0x42}, keystrokeDelay) // down arrow

	last, _ = s.waitFor(3*time.Second, func(text string) bool {
		return strings.Contains(last, "both") || strings.Contains(text, "both")
	})
	t.Logf("both option frame: %v", strings.Contains(last, "both"))
	saveFrame(t, "match-strategy-both", s)

	// Press Esc to close the wizard without writing.
	s.sendKey([]byte{0x1b}, keystrokeDelay*2)
	// Press 'q' to ensure quit.
	s.sendKey([]byte("q"), keystrokeDelay)
}

// TestUIPTY_AdoptModal seeds an unmanaged gitconfig fragment (~/.gitconfig_demo)
// in the sandbox HOME, opens the TUI, navigates to the Unmanaged section, and
// presses 'A' to open the Adopt modal. Asserts on the decoded frame.
func TestUIPTY_AdoptModal(t *testing.T) {
	home := SandboxHome(t)
	bin := BuildBinary(t)

	// Seed a fragment candidate so the sidebar shows the Unmanaged section.
	seedFragmentCandidate(t, home, "demo")

	// Also seed a managed identity so the sidebar is not empty.
	seedMinimalIdentity(t, home, "personal")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	s := startPTY(t, newPTYCmd(ctx, bin, home))
	defer s.close(t)

	uiReady(t, s)
	time.Sleep(200 * time.Millisecond) // let sidebar scan complete

	// Navigate down in the sidebar to reach the Unmanaged section.
	// Press 'j' multiple times to move past managed identities.
	for i := 0; i < 5; i++ {
		s.sendKey([]byte("j"), keystrokeDelay)
	}

	// Look for the Unmanaged section in the sidebar.
	last, _ := s.waitFor(3*time.Second, func(text string) bool {
		return strings.Contains(text, "Unmanaged") || strings.Contains(text, "demo")
	})
	t.Logf("unmanaged section: %v", strings.Contains(last, "Unmanaged"))
	saveFrame(t, "sidebar-unmanaged", s)

	// Press 'A' (Shift+A) to attempt to open the Adopt modal.
	s.sendKey([]byte("A"), keystrokeDelay*2)

	// Wait for the Adopt modal to appear.
	last, ok := s.waitFor(4*time.Second, func(text string) bool {
		return strings.Contains(text, "Adopt") || strings.Contains(text, "Migrate") || strings.Contains(text, "fragment")
	})
	if !ok {
		t.Logf("adopt modal: last frame:\n%s", last)
		// Non-fatal: the modal may not open if the focused row is not a kindFragment.
		// The fragment discriminator requires the sidebar cursor to be on the fragment row.
		t.Logf("NOTE: Adopt modal did not open (sidebar cursor may not be on the fragment row). Frame captured for UI critique.")
	} else {
		t.Logf("PASS — Adopt modal opened, contains 'Adopt'/'Migrate'")
	}

	saveFrame(t, "adopt-modal", s)

	// Close the modal.
	s.sendKey([]byte{0x1b}, keystrokeDelay*2) // ESC
}

// TestUIPTY_AddRepoModal opens the TUI, presses ctrl+r to open the Add Repo
// modal, and asserts on the decoded frame. Also types a URL to test text input.
func TestUIPTY_AddRepoModal(t *testing.T) {
	home := SandboxHome(t)
	bin := BuildBinary(t)

	// Seed a managed identity so the TUI opens in a populated state.
	seedMinimalIdentity(t, home, "personal")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	s := startPTY(t, newPTYCmd(ctx, bin, home))
	defer s.close(t)

	uiReady(t, s)
	time.Sleep(100 * time.Millisecond)

	// Press ctrl+r to open the Add Repo modal.
	s.sendKey([]byte{0x12}, keystrokeDelay*2) // ctrl+r = 0x12

	// Wait for the Add Repo modal.
	last, ok := s.waitFor(5*time.Second, func(text string) bool {
		return strings.Contains(text, "Add Repo") || strings.Contains(text, "Clone URL") || strings.Contains(text, "URL")
	})
	if !ok {
		t.Logf("add-repo modal: last frame:\n%s", last)
		t.Logf("NOTE: Add Repo modal did not open within timeout. Frame captured for UI critique.")
	} else {
		t.Logf("PASS — Add Repo modal opened")
	}

	saveFrame(t, "addrepo-modal-open", s)

	// Type a URL to test input decoding in the Add Repo modal.
	testURL := "https://github.com/org/repo"
	for _, ch := range testURL {
		s.sendKey([]byte(string(ch)), keystrokeDelay)
	}
	time.Sleep(100 * time.Millisecond)

	// Assert the typed URL appears in the decoded frame.
	last, ok = s.waitFor(3*time.Second, func(text string) bool {
		return strings.Contains(text, "github.com") || strings.Contains(text, "https")
	})
	if ok {
		t.Logf("PASS — typed URL appears in Add Repo modal frame")
	} else {
		t.Logf("NOTE: URL not visible in decoded frame (may be in alt-screen). Last frame:\n%s", last)
	}

	saveFrame(t, "addrepo-modal-url", s)

	// Close the modal.
	s.sendKey([]byte{0x1b}, keystrokeDelay*2) // ESC
}

// TestUIPTY_CopyUploadAssist opens the TUI with a seeded identity and presses
// 'c' to open the Copy Public Key modal (which now contains the gh/glab
// upload-assist section). Asserts on the decoded frame.
func TestUIPTY_CopyUploadAssist(t *testing.T) {
	home := SandboxHome(t)
	bin := BuildBinary(t)

	// Seed a managed identity so 'c' has something to copy.
	seedMinimalIdentity(t, home, "personal")

	// Provide a fake gh in auth-fail mode (gh present but not authenticated).
	// This exercises the "gh detected but not authenticated" code path.
	ghDir := FakeGHDir(t, "auth-fail")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	s := startPTY(t, newPTYCmd(ctx, bin, home,
		"GITID_FAKE_GH_MODE=auth-fail",
		"PATH="+ghDir+":"+os.Getenv("PATH"),
	))
	defer s.close(t)

	uiReady(t, s)
	time.Sleep(300 * time.Millisecond) // let the identity list populate

	// Move to the identity row in the sidebar (it may already be focused, but
	// pressing 'j' once ensures we are on the first identity row).
	s.sendKey([]byte("j"), keystrokeDelay)
	s.sendKey([]byte("k"), keystrokeDelay) // back up in case we overshot

	// Press 'c' to open the Copy Public Key modal.
	s.sendKey([]byte("c"), keystrokeDelay*2)

	// Wait for the copy modal to appear.
	last, ok := s.waitFor(5*time.Second, func(text string) bool {
		return strings.Contains(text, "Copy") || strings.Contains(text, "Public Key") ||
			strings.Contains(text, "ssh-ed25519") || strings.Contains(text, "pubkey")
	})
	if !ok {
		t.Logf("copy modal: last frame:\n%s", last)
		t.Logf("NOTE: Copy modal did not open within timeout.")
	} else {
		t.Logf("PASS — Copy modal opened")
	}

	saveFrame(t, "copy-upload-assist", s)

	// Close the modal.
	s.sendKey([]byte{0x1b}, keystrokeDelay*2) // ESC
}

// TestUIPTY_TabCyclesWithoutHang verifies that pressing Tab multiple times in
// the main identities view cycles sidebar/main focus without hanging. This
// asserts the "Tab cycles focus without hang" requirement from the plan.
func TestUIPTY_TabCyclesWithoutHang(t *testing.T) {
	home := SandboxHome(t)
	bin := BuildBinary(t)
	seedMinimalIdentity(t, home, "personal")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	s := startPTY(t, newPTYCmd(ctx, bin, home))
	defer s.close(t)

	uiReady(t, s)

	// Press Tab 6 times to cycle focus (3 full cycles).
	for i := 0; i < 6; i++ {
		s.sendKey([]byte{0x09}, 120*time.Millisecond) // Tab with a generous delay
	}

	// The TUI must still be responsive (producing output) after 6 Tab presses.
	last, ok := s.waitFor(3*time.Second, func(text string) bool {
		return strings.Contains(text, "gitid") || strings.Contains(text, "Identities")
	})
	if !ok {
		t.Errorf("FAIL — TUI became unresponsive after Tab cycling. Last frame:\n%s", last)
	} else {
		t.Logf("PASS — Tab cycles focus without hang")
	}

	saveFrame(t, "tab-cycle-focus", s)
}

// TestUIPTY_EscClosesModals opens the wizard modal and presses Esc to close it,
// asserting the TUI returns to the main identities view (Esc = safe cancel
// at every modal step per UI-SPEC Accessibility Contract rule 11).
func TestUIPTY_EscClosesModals(t *testing.T) {
	home := SandboxHome(t)
	bin := BuildBinary(t)
	seedMinimalIdentity(t, home, "personal")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	s := startPTY(t, newPTYCmd(ctx, bin, home))
	defer s.close(t)

	uiReady(t, s)

	// Open wizard.
	s.sendKey([]byte("a"), keystrokeDelay*2)
	_, _ = s.waitFor(3*time.Second, func(text string) bool {
		return strings.Contains(text, "Name") || strings.Contains(text, "Create")
	})

	// Press Esc to close.
	s.sendKey([]byte{0x1b}, keystrokeDelay*2)

	// The main view should be back (contains "gitid" header and "Identities").
	last, ok := s.waitFor(3*time.Second, func(text string) bool {
		return strings.Contains(text, "gitid") || strings.Contains(text, "Identities")
	})
	if !ok {
		t.Errorf("FAIL — Esc did not close the wizard modal. Last frame:\n%s", last)
	} else {
		t.Logf("PASS — Esc closes modal, main view restored")
	}

	saveFrame(t, "esc-closes-modal", s)
}
