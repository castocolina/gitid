package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/castocolina/gitid/internal/doctor"
	"github.com/castocolina/gitid/internal/gitconfig"
	"github.com/castocolina/gitid/internal/identity"
	"github.com/castocolina/gitid/internal/tester"
)

// These tests exercise the LIVE program wiring rather than model internals in
// isolation. The Phase 5 blockers (CR-01..CR-04) all passed unit tests that
// called init()/the deps seam directly; they were never driven through the
// rootModel push/navigation path the running program uses. Each test below
// drives the same path the live program drives.

// recordingTUIDeps returns a tuiDeps whose identity/update write fields are
// no-ops EXCEPT that the update WriteSSH seam records into ranUpdate when the
// update mode runs. PreWrite/Resolved pass so the prove screen reaches the
// confirm gate. Tests that need to observe other modes override specific seams.
func recordingTUIDeps(ranUpdate *bool) tuiDeps {
	return tuiDeps{
		identity: identity.Deps{
			Generate:   func(_ identity.CreateInput) (identity.StagedKey, error) { return identity.StagedKey{}, nil },
			PersistKey: func(_ identity.StagedKey) (identity.KeyResult, error) { return identity.KeyResult{}, nil },
			Cleanup:    func(_ identity.StagedKey) {},
			CopyPub:    func(_ string) error { return nil },
			PreWrite:   func(_, _ string, _ int) tester.Result { return tester.Result{Outcome: tester.PASS} },
			WriteSSH: func(_, _, _ string) (string, error) {
				// Create and AddAccount both funnel through runPipeline's WriteSSH;
				// the action that invoked the write is recorded by the caller before
				// dispatch, so here we only need a benign backup path.
				return "ssh.bak", nil
			},
			WriteGitconfig:      func(_, _, _ string, _ []gitconfig.Match) (string, error) { return "gc.bak", nil },
			WriteFragment:       func(_, _, _, _ string, _ bool) error { return nil },
			WriteAllowedSigners: func(_, _, _ string) (string, error) { return "as.bak", nil },
			Resolved: func(_ string) (tester.Result, tester.ResolvedConfig) {
				return tester.Result{Outcome: tester.PASS}, tester.ResolvedConfig{}
			},
			PubExists: func(_ string) bool { return true },
			DerivePub: func(_ string) (string, error) { return "ssh-ed25519 AAAA", nil },
			WritePub:  func(_, _ string) error { return nil },
		},
		update: identity.UpdateDeps{
			WriteSSH: func(_, _, _ string) (string, error) {
				if ranUpdate != nil {
					*ranUpdate = true
				}
				return "ssh.bak", nil
			},
			WriteGitconfig:       func(_, _, _ string, _ []gitconfig.Match) (string, error) { return "gc.bak", nil },
			WriteFragment:        func(_, _, _, _ string, _ bool) error { return nil },
			WriteAllowedSigners:  func(_, _, _ string) (string, error) { return "as.bak", nil },
			RemoveAllowedSigners: func(_, _ string) (string, error) { return "as.bak", nil },
			Resolved: func(_ string) (tester.Result, tester.ResolvedConfig) {
				return tester.Result{Outcome: tester.PASS}, tester.ResolvedConfig{}
			},
			ReadPub: func(_ string) (string, error) { return "ssh-ed25519 AAAA", nil },
		},
	}
}

// TestPushInvokesProveInit drives a pushScreenMsg carrying a prove screen
// through rootModel.Update (the SAME path the live program uses for push
// navigation) and asserts the push handler fires the pushed screen's init()
// cmd, so phase 1 actually starts (CR-01). Previously the handler returned
// (m, nil), so a pushed prove screen sat in provePhase1Running forever.
func TestPushInvokesProveInit(t *testing.T) {
	root := newFakeRootModel()
	deps := fakeWriteTUIDeps(nil)

	prove := newProveScreen("create", makeTestInput(), identity.Account{}, "~/.ssh/gitid_personal", deps)

	updated, cmd := root.Update(pushScreenMsg{next: prove})
	rm, ok := updated.(rootModel)
	if !ok {
		t.Fatalf("Update(pushScreenMsg) returned %T; want rootModel", updated)
	}
	if len(rm.stack) < 2 {
		t.Fatalf("push must append the prove screen; stack len = %d", len(rm.stack))
	}
	if cmd == nil {
		t.Fatal("pushing a prove screen must return a non-nil startup cmd (CR-01: init must fire)")
	}

	// The startup cmd must (transitively) produce the phase-1 preWriteResultMsg,
	// proving phase 1 was actually scheduled by the push handler.
	if !producesPreWrite(cmd) {
		t.Fatal("pushed prove screen's startup cmd must produce preWriteResultMsg (phase 1 started)")
	}
}

// producesPreWrite reports whether cmd (a single cmd or a tea.BatchMsg) yields a
// preWriteResultMsg, unwrapping one batch level (init batches the pre-write cmd
// with the spinner tick).
func producesPreWrite(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	switch msg := cmd().(type) {
	case preWriteResultMsg:
		return true
	case tea.BatchMsg:
		for _, c := range msg {
			if c == nil {
				continue
			}
			if _, ok := c().(preWriteResultMsg); ok {
				return true
			}
		}
	}
	return false
}

// TestDepsThreadedEndToEnd navigates dashboard → identity list → create form →
// prove using the real key sequence and asserts the prove screen receives
// non-nil identity.Deps function fields (CR-02). Before the fix the chain only
// propagated doctor.Deps, so the prove screen got identity.Deps{} (nil funcs)
// and the confirmed write nil-panicked.
func TestDepsThreadedEndToEnd(t *testing.T) {
	idDeps := fakeWriteTUIDeps(nil).identity
	root := newRootModel(fakeDocDeps(), idDeps, identity.UpdateDeps{})

	// Dashboard (stack[0]) → Enter pushes the identity list, carrying tuiDeps.
	dash, ok := root.stack[0].(dashboardModel)
	if !ok {
		t.Fatalf("stack[0] is %T; want dashboardModel", root.stack[0])
	}
	_, cmd := dash.update(tea.KeyPressMsg{Code: tea.KeyEnter})
	listScreen := mustPushTarget(t, cmd)
	list, ok := listScreen.(identityListModel)
	if !ok {
		t.Fatalf("dashboard Enter pushed %T; want identityListModel", listScreen)
	}
	if list.deps.identity.Generate == nil {
		t.Fatal("identity list must carry non-nil identity.Deps (CR-02)")
	}

	// Identity list → 'a' pushes the create form, carrying tuiDeps.
	_, cmd = list.update(tea.KeyPressMsg{Text: "a"})
	formScreen := mustPushTarget(t, cmd)
	form, ok := formScreen.(createFormModel)
	if !ok {
		t.Fatalf("'a' pushed %T; want createFormModel", formScreen)
	}
	if form.deps.identity.Generate == nil {
		t.Fatal("create form must carry non-nil identity.Deps (CR-02)")
	}

	// Create form → fill a valid name and submit to push the prove screen.
	form.inputs[0].SetValue("personal")
	form.focusIdx = len(form.inputs) - 1
	_, cmd = form.update(tea.KeyPressMsg{Code: tea.KeyEnter})
	proveScreen := mustPushTarget(t, cmd)
	prove, ok := proveScreen.(proveModel)
	if !ok {
		t.Fatalf("form submit pushed %T; want proveModel", proveScreen)
	}

	// The prove screen — the screen that performs the write — must hold non-nil
	// identity.Deps function fields threaded end-to-end (CR-02).
	if prove.deps.identity.Generate == nil ||
		prove.deps.identity.WriteSSH == nil ||
		prove.deps.identity.PreWrite == nil {
		t.Fatal("prove screen must receive non-nil identity.Deps funcs threaded dashboard→list→form→prove (CR-02)")
	}
}

// mustPushTarget executes cmd, asserts it produced a pushScreenMsg, and returns
// the pushed screen.
func mustPushTarget(t *testing.T, cmd tea.Cmd) screenModel {
	t.Helper()
	if cmd == nil {
		t.Fatal("expected a non-nil push cmd")
	}
	push, ok := cmd().(pushScreenMsg)
	if !ok {
		t.Fatalf("cmd produced %T; want pushScreenMsg", cmd())
	}
	return push.next
}

// TestRunWriteCmdDispatchesUpdate asserts that confirming an "update" prove
// screen dispatches through identity.Update (not identity.Create), proving the
// action branch in runWriteCmd (CR-03). The update-only WriteSSH seam records
// that it ran.
func TestRunWriteCmdDispatchesUpdate(t *testing.T) {
	var ranUpdate bool
	deps := recordingTUIDeps(&ranUpdate)

	acct := identity.Account{
		Name:               "work",
		GitEmail:           "work@example.com",
		Provider:           "github.com",
		Alias:              "work.github.com",
		KeyPath:            "~/.ssh/gitid_work",
		PubPath:            "~/.ssh/gitid_work.pub",
		SSHConfigPath:      "/home/u/.ssh/config",
		GitconfigPath:      "/home/u/.gitconfig",
		AllowedSignersPath: "/home/u/.ssh/allowed_signers",
		FragmentPath:       "~/.gitconfig.d/work",
	}
	in := identity.CreateInput{Name: acct.Name, Alias: acct.Alias, Provider: acct.Provider}

	// FIX-2: runWriteCmd now takes a distinct original vs edited account plus the
	// preserved signing flag. Here original == edited (no structural change) and
	// signing is preserved as true for this email-bearing identity.
	cmd := runWriteCmd("update", in, acct, acct, true, deps)
	msg := cmd()
	wr, ok := msg.(writeResultMsg)
	if !ok {
		t.Fatalf("runWriteCmd(update) produced %T; want writeResultMsg", msg)
	}
	if wr.err != nil {
		t.Fatalf("update write returned error: %v", wr.err)
	}
	if !ranUpdate {
		t.Fatal("runWriteCmd(\"update\") must dispatch through identity.Update, not Create (CR-03)")
	}
	if wr.backupPath == "" {
		t.Error("update write must surface the ssh config backup path (WR-05)")
	}
}

// TestRunWriteCmdDispatchesAddAccount asserts that confirming an "add-account"
// prove screen dispatches through identity.AddAccount and shares the existing
// key (no keygen) — verified by the Generate seam never being called (CR-03).
func TestRunWriteCmdDispatchesAddAccount(t *testing.T) {
	var generateCalled bool
	deps := recordingTUIDeps(nil)
	deps.identity.Generate = func(_ identity.CreateInput) (identity.StagedKey, error) {
		generateCalled = true
		return identity.StagedKey{}, nil
	}

	acct := identity.Account{
		Name:               "work",
		GitEmail:           "work@example.com",
		KeyPath:            "~/.ssh/gitid_work",
		PubPath:            "~/.ssh/gitid_work.pub",
		SSHConfigPath:      "/home/u/.ssh/config",
		GitconfigPath:      "/home/u/.gitconfig",
		AllowedSignersPath: "/home/u/.ssh/allowed_signers",
		FragmentPath:       "~/.gitconfig.d/work",
		Matches:            []gitconfig.Match{{Kind: gitconfig.MatchGitdir, Value: "~/git/work/"}},
	}
	in := identity.CreateInput{Name: acct.Name, Provider: "gitlab.com", Alias: "work.gitlab.com"}

	// add-account ignores original/signing; pass the edited account as both.
	cmd := runWriteCmd("add-account", in, acct, acct, false, deps)
	msg := cmd()
	wr, ok := msg.(writeResultMsg)
	if !ok {
		t.Fatalf("runWriteCmd(add-account) produced %T; want writeResultMsg", msg)
	}
	if wr.err != nil {
		t.Fatalf("add-account write returned error: %v", wr.err)
	}
	if generateCalled {
		t.Fatal("runWriteCmd(\"add-account\") must reuse the existing key (no Generate), not run the create-new keygen (CR-03)")
	}
}

// TestProveKeyPathIsPrivateKeyNotSSHConfig asserts that the prove screen's
// pre-write gate runs against the PRIVATE-KEY path, not the ssh config path
// (CR-04). It captures the keyPath the gate is invoked with via a recording
// PreWrite seam and confirms it equals the key path the form supplied, and is
// NOT the SSHConfigPath.
func TestProveKeyPathIsPrivateKeyNotSSHConfig(t *testing.T) {
	const keyPath = "/home/u/.ssh/gitid_personal"
	const sshConfigPath = "/home/u/.ssh/config"

	var gotKeyPath string
	deps := fakeWriteTUIDeps(nil)
	deps.identity.PreWrite = func(kp, _ string, _ int) tester.Result {
		gotKeyPath = kp
		return tester.Result{Outcome: tester.PASS}
	}

	in := makeTestInput()
	in.SSHConfigPath = sshConfigPath // the OLD bug sourced keyPath from this field

	prove := newProveScreen("create", in, identity.Account{}, keyPath, deps)

	// init() schedules phase 1; execute the pre-write cmd so PreWrite runs.
	_, cmd := prove.init()
	if !producesPreWrite(cmd) {
		t.Fatal("init() must schedule the phase-1 pre-write cmd")
	}

	if gotKeyPath == sshConfigPath {
		t.Fatal("pre-write gate ran against the ssh config path; it must use the private-key path (CR-04)")
	}
	if gotKeyPath != keyPath {
		t.Fatalf("pre-write gate keyPath = %q; want the private-key path %q (CR-04)", gotKeyPath, keyPath)
	}
}

// fakeDocDepsWithPub returns a doctor.Deps whose ReadFile returns a fixed pub
// line, used to verify WR-02 pubLine caching.
func fakeDocDepsWithPub(pub string) doctor.Deps {
	d := fakeDocDeps()
	d.ReadFile = func(_ string) ([]byte, error) { return []byte(pub + "\n"), nil }
	return d
}

// TestDetailPubLineCachedFromPub asserts the identity detail model caches the
// public-key line from the account's .pub via the injected ReadFile seam (WR-02),
// so the copy action does not copy an empty string.
func TestDetailPubLineCachedFromPub(t *testing.T) {
	const pub = "ssh-ed25519 AAAAfakekey comment"
	deps := tuiDeps{doctor: fakeDocDepsWithPub(pub)}
	acct := identity.Account{Name: "personal", Provider: "github.com", PubPath: "/home/u/.ssh/gitid_personal.pub"}

	m := newIdentityDetailModel(acct, deps)
	if m.pubLine != pub {
		t.Fatalf("pubLine = %q; want %q (WR-02: read from .pub via ReadFile seam)", m.pubLine, pub)
	}
}
