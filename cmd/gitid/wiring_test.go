package main

// wiring_test.go guards the REAL composition root.
//
// Every test here runs over a hermetic t.TempDir() fake home — the developer's
// real ~/.ssh and ~/.gitconfig are never read or written.

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/castocolina/gitid/internal/identity"
	"github.com/castocolina/gitid/internal/keygen"
	"github.com/castocolina/gitid/internal/platform"
	"github.com/castocolina/gitid/internal/sshconfig"
	"github.com/castocolina/gitid/internal/tester"
	"github.com/castocolina/gitid/internal/tuikit"
)

// ---------------------------------------------------------------------------
// L2 — the injected-seam wiring guard
// ---------------------------------------------------------------------------

// TestBuildBackendSatisfiesSeam proves the real constructor produces a usable
// Backend. tuikit.NewApp panics on a nil Backend by design, so a broken
// composition root must fail here rather than at the user's first keystroke.
func TestBuildBackendSatisfiesSeam(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	b := buildBackend()
	if b == nil {
		t.Fatal("buildBackend() returned nil; tuikit.NewApp would panic")
	}
	if _, ok := b.(*realBackend); !ok {
		t.Fatalf("buildBackend() returned %T, want *realBackend", b)
	}
}

// TestIdentityDepsEveryFieldIsWired is the L2 real-constructor guard: EVERY
// identity.Deps function field the real composition root builds must be
// non-nil, and the failure must NAME the field.
//
// This is the project's recurring injected-seam wiring blindspot. identity.Deps
// nil-guards several seams (PubExists, ReadPub, the provisional trio), so a
// field left nil in the REAL constructor does not panic — it silently selects a
// different behavior while every unit test that injects its own fake still
// passes. Reflecting over the struct means a FUTURE field addition fails loudly
// here instead of being silently skipped.
func TestIdentityDepsEveryFieldIsWired(t *testing.T) {
	home := t.TempDir()
	deps := buildIdentityDeps(newBackendForHome(home))

	v := reflect.ValueOf(deps)
	typ := v.Type()
	if typ.NumField() == 0 {
		t.Fatal("identity.Deps has no fields; the guard would be vacuous")
	}
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.Type.Kind() != reflect.Func {
			continue
		}
		if v.Field(i).IsNil() {
			t.Errorf("identity.Deps.%s is nil in the REAL constructor — a nil seam silently changes behavior (L2)", field.Name)
		}
	}
}

// TestReadPubIsWiredToARealImplementation proves the D-11 seam plan 03-01
// introduced is not merely non-nil but actually reads the `.pub` from disk and
// returns its line verbatim (trimmed). A nil or fake ReadPub makes
// identity.ensurePub fall back to DerivePub, which parses the PRIVATE key and
// therefore fails on a passphrase-protected one — re-breaking encrypted-key
// reuse (KEY-06/D-11) with every unit test still green.
//
// The remaining half of the L2 obligation — driving this path through the real
// binary over a raw-keystroke PTY — lands in plan 03-06.
func TestReadPubIsWiredToARealImplementation(t *testing.T) {
	home := t.TempDir()
	deps := buildIdentityDeps(newBackendForHome(home))

	pubPath := filepath.Join(home, "id_ed25519_test.pub")
	const want = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIExampleKeyMaterialForTest test@gitid"
	if err := os.WriteFile(pubPath, []byte(want+"\n"), 0o644); err != nil { //nolint:gosec // 0644 is the correct mode for a public key fixture
		t.Fatalf("seeding .pub fixture: %v", err)
	}

	got, err := deps.ReadPub(pubPath)
	if err != nil {
		t.Fatalf("ReadPub: %v", err)
	}
	if got != want {
		t.Errorf("ReadPub = %q, want %q (the existing .pub line must be used verbatim)", got, want)
	}

	// A missing .pub must report the path it could not read.
	_, err = deps.ReadPub(filepath.Join(home, "absent.pub"))
	if err == nil {
		t.Fatal("ReadPub on a missing file = nil error, want an error")
	}
	if !strings.Contains(err.Error(), "absent.pub") {
		t.Errorf("ReadPub error = %v, want it to name the unreadable path", err)
	}
}

// ---------------------------------------------------------------------------
// Backend -> view converters (the single conversion boundary)
// ---------------------------------------------------------------------------

// TestToTestResultViewMapsEveryOutcome covers ALL THREE tester outcomes. The
// mapping must be total: a missed case would silently render a warning as a
// success (or worse) while the classifier itself was right.
func TestToTestResultViewMapsEveryOutcome(t *testing.T) {
	cases := []struct {
		name string
		in   tester.Outcome
		want tuikit.TestOutcome
	}{
		{"pass", tester.PASS, tuikit.TestOutcomePass},
		{"reachable-not-uploaded", tester.ReachableNotUploaded, tuikit.TestOutcomeReachableNotUploaded},
		{"failure", tester.Failure, tuikit.TestOutcomeFailure},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := tester.Result{
				Command: "ssh -T git@ssh.github.com",
				Output:  "  some real ssh output\n",
				Outcome: tc.in,
			}
			view := toTestResultView(res, res.Command)
			if view.Outcome != tc.want {
				t.Errorf("Outcome = %v, want %v", view.Outcome, tc.want)
			}
			if view.Command != res.Command {
				t.Errorf("Command = %q, want the command that was RUN (%q)", view.Command, res.Command)
			}
			if view.Detail != "some real ssh output" {
				t.Errorf("Detail = %q, want the real ssh output line", view.Detail)
			}
		})
	}
}

// TestToReusableKeyViewsCarriesFlagsAndOwner proves the picker projection keeps
// every field the D-10/D-12/D-13 rows need, including the "in use by" label,
// and that an unreferenced key gets an EMPTY label rather than a fabricated one.
func TestToReusableKeyViewsCarriesFlagsAndOwner(t *testing.T) {
	keys := []keygen.ReusableKey{
		{Path: "/h/.ssh/id_ed25519_personal", Algorithm: "ssh-ed25519", Fingerprint: "SHA256:aaa", HasPub: true},
		{Path: "/h/.ssh/id_rsa_old", Algorithm: "ssh-rsa", Fingerprint: "SHA256:bbb", HasPub: false, Encrypted: true},
	}
	owners := map[string]string{"/h/.ssh/id_ed25519_personal": "personal"}

	views := toReusableKeyViews(keys, owners)
	if len(views) != 2 {
		t.Fatalf("got %d views, want 2 (no candidate may be dropped — D-13)", len(views))
	}
	if views[0].InUseBy != "personal" {
		t.Errorf("views[0].InUseBy = %q, want %q (D-12)", views[0].InUseBy, "personal")
	}
	if views[1].InUseBy != "" {
		t.Errorf("views[1].InUseBy = %q, want empty for an unreferenced key", views[1].InUseBy)
	}
	if !views[1].Encrypted || views[1].HasPub {
		t.Errorf("views[1] flags = {Encrypted:%v HasPub:%v}, want {true false} carried verbatim",
			views[1].Encrypted, views[1].HasPub)
	}
	if views[0].Algorithm != "ssh-ed25519" || views[0].Fingerprint != "SHA256:aaa" || views[0].Path != keys[0].Path {
		t.Errorf("views[0] = %+v, want path/algorithm/fingerprint carried verbatim", views[0])
	}
}

// ---------------------------------------------------------------------------
// D-20 / D-21 provider defaults, SSHUI-03 preview
// ---------------------------------------------------------------------------

// TestProviderDefaults proves known providers get the recipe-canonical alt-SSH
// pairing (D-20) and an unknown/custom host keeps itself on port 22 (D-21),
// resolved through internal/identity — cmd/gitid re-derives no provider table.
func TestProviderDefaults(t *testing.T) {
	b := newBackendForHome(t.TempDir())
	cases := []struct{ provider, hostname, port string }{
		{"github.com", "ssh.github.com", "443"},
		{"gitlab.com", "altssh.gitlab.com", "443"},
		{"bitbucket.org", "altssh.bitbucket.org", "443"},
		{"git.internal.example", "git.internal.example", "22"},
	}
	for _, tc := range cases {
		host, port := b.ProviderDefaults(tc.provider)
		if host != tc.hostname || port != tc.port {
			t.Errorf("ProviderDefaults(%q) = (%q, %q), want (%q, %q)", tc.provider, host, port, tc.hostname, tc.port)
		}
	}
}

// TestHostBlockPreviewIsTheWrittenBlock proves the live preview is rendered by
// the SAME sshconfig.RenderHostBlock the confirmed write uses — "written
// exactly like this on confirm" (SSHUI-03), with the recipe's Port 443 and
// IdentitiesOnly yes.
func TestHostBlockPreviewIsTheWrittenBlock(t *testing.T) {
	b := newBackendForHome(t.TempDir())
	spec := tuikit.CreateSpec{
		Identity: "personal",
		// WR-01: Provider must be explicitly set so HostBlockPreview (which now
		// uses spec.Provider directly rather than providerFromAlias) renders the
		// correct provider comment — matching what the confirmed write produces.
		Provider: "github.com",
		Alias:    "personal.github.com",
		Hostname: "ssh.github.com",
		Port:     "443",
		KeyPath:  "~/.ssh/id_ed25519_personal",
	}
	got := b.HostBlockPreview(spec)
	want := sshconfig.RenderHostBlock("personal.github.com", "ssh.github.com", 443, "~/.ssh/id_ed25519_personal", "github.com")
	if got != want {
		t.Errorf("HostBlockPreview =\n%s\nwant (the block the writer produces):\n%s", got, want)
	}
	for _, must := range []string{"Port 443", "IdentitiesOnly yes", "User git"} {
		if !strings.Contains(got, must) {
			t.Errorf("preview is missing the recipe-canonical %q:\n%s", must, got)
		}
	}
}

// ---------------------------------------------------------------------------
// D-05 / D-06 — storage auto-detect
// ---------------------------------------------------------------------------

// TestStorageFreshMachineDefaultsToIncludeLayout proves D-06: with no existing
// gitid layout, a create targets ~/.ssh/config.d/gitid.config and adds the
// Include line to ~/.ssh/config. This SUPERSEDES STORE-01's in-file default.
func TestStorageFreshMachineDefaultsToIncludeLayout(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	b := newBackendForHome(home)

	st := b.storage()
	want := filepath.Join(home, ".ssh", "config.d", "gitid.config")
	if st.targetPath != want {
		t.Errorf("fresh-machine target = %q, want %q (D-06 Include'd default)", st.targetPath, want)
	}
	if !st.needsIncludeLine {
		t.Error("fresh-machine layout must add the Include line to ~/.ssh/config (D-06)")
	}
	if !st.includeLayout {
		t.Error("fresh-machine layout must be the Include'd layout (D-06)")
	}
}

// TestStorageExistingInFileBlocksKeepsInFile proves D-05: a machine already
// using in-file managed blocks keeps writing in-file. Reserved wiring blocks
// alone (the Include line, the macOS globals) must NOT count as "in use".
func TestStorageExistingInFileBlocksKeepsInFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedSSHDir(t, home)
	configPath := filepath.Join(home, ".ssh", "config")
	writeFile(t, configPath, managedBlock("personal",
		sshconfig.RenderHostBlock("personal.github.com", "ssh.github.com", 443, "~/.ssh/id_ed25519_personal", "")))

	st := newBackendForHome(home).storage()
	if st.targetPath != configPath {
		t.Errorf("in-file layout target = %q, want %q", st.targetPath, configPath)
	}
	if st.needsIncludeLine || st.includeLayout {
		t.Error("an existing in-file layout must NOT be migrated to the Include'd layout (D-07: no layout switching in Phase 3)")
	}
}

// TestStorageReservedBlocksAloneStayFresh proves the reserved wiring blocks are
// not mistaken for identity blocks: a config carrying only the Include line and
// the macOS globals is still a FRESH machine.
func TestStorageReservedBlocksAloneStayFresh(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedSSHDir(t, home)
	configPath := filepath.Join(home, ".ssh", "config")
	writeFile(t, configPath,
		managedBlock("ssh-include", "Include ~/.ssh/config.d/*.config")+
			managedBlock("_global", sshconfig.RenderGlobalBlock("darwin")))

	st := newBackendForHome(home).storage()
	if !st.includeLayout {
		t.Errorf("target = %q; reserved wiring blocks alone must not pin the in-file layout", st.targetPath)
	}
}

// TestStorageExistingIncludedFileIsTargeted proves D-05: once
// config.d/gitid.config exists it is the write target, not ~/.ssh/config.
func TestStorageExistingIncludedFileIsTargeted(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedSSHDir(t, home)
	configPath := filepath.Join(home, ".ssh", "config")
	included := filepath.Join(home, ".ssh", "config.d", "gitid.config")
	if err := os.MkdirAll(filepath.Dir(included), 0o700); err != nil {
		t.Fatalf("seeding config.d: %v", err)
	}
	writeFile(t, configPath, managedBlock("ssh-include", "Include ~/.ssh/config.d/*.config"))
	writeFile(t, included, managedBlock("personal",
		sshconfig.RenderHostBlock("personal.github.com", "ssh.github.com", 443, "~/.ssh/id_ed25519_personal", "")))

	b := newBackendForHome(home)
	st := b.storage()
	if st.targetPath != included {
		t.Errorf("target = %q, want the existing Include'd file %q", st.targetPath, included)
	}
	if st.needsIncludeLine {
		t.Error("the Include line already exists; it must not be re-added")
	}
	if got, want := b.ResolvedStorageTarget(tuikit.DemoState{}), "~/.ssh/config.d/gitid.config"; got != want {
		t.Errorf("ResolvedStorageTarget = %q, want the dynamic resolved path %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// D-06 / D-08 / TEST-03 — the confirmed write
// ---------------------------------------------------------------------------

// TestWriteSSHBlockCreatesIncludeLayoutWithGlobals proves the fresh-machine
// confirmed write: the Host block lands in the Include'd file, the Include line
// is floored into ~/.ssh/config, the macOS globals block is written after the
// specific host (D-08), and the config.d directory is 0700 (STORE-01).
func TestWriteSSHBlockCreatesIncludeLayoutWithGlobals(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedSSHDir(t, home)
	b := newBackendForHome(home)

	hostBlock := sshconfig.RenderHostBlock("personal.github.com", "ssh.github.com", 443,
		filepath.Join(home, ".ssh", "id_ed25519_personal"), "github.com")
	globals := sshconfig.RenderGlobalBlock("darwin")

	if _, err := b.writeSSHBlock("personal", hostBlock, globals); err != nil {
		t.Fatalf("writeSSHBlock: %v", err)
	}

	included := readFile(t, filepath.Join(home, ".ssh", "config.d", "gitid.config"))
	if !strings.Contains(included, "Host personal.github.com") {
		t.Errorf("the identity Host block did not land in the Include'd file:\n%s", included)
	}
	if !strings.Contains(included, "UseKeychain yes") {
		t.Errorf("the macOS globals block was not written (D-08):\n%s", included)
	}
	if strings.Index(included, "Host *") < strings.Index(included, "Host personal.github.com") {
		t.Errorf("the `Host *` globals block must come AFTER the specific host (first-match-wins):\n%s", included)
	}

	mainConfig := readFile(t, filepath.Join(home, ".ssh", "config"))
	if !strings.Contains(mainConfig, "Include ~/.ssh/config.d/*.config") {
		t.Errorf("the Include line was not added to ~/.ssh/config (D-06):\n%s", mainConfig)
	}

	info, err := os.Stat(filepath.Join(home, ".ssh", "config.d"))
	if err != nil {
		t.Fatalf("stat config.d: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o700 {
		t.Errorf("config.d mode = %o, want 0700 (STORE-01)", perm)
	}
}

// TestWriteSSHBlockOffDarwinWritesNoGlobals proves the macOS-only block is
// absent off darwin: an Apple-only directive must never be written elsewhere.
func TestWriteSSHBlockOffDarwinWritesNoGlobals(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedSSHDir(t, home)
	b := newBackendForHome(home)

	hostBlock := sshconfig.RenderHostBlock("work.github.com", "ssh.github.com", 443, "~/.ssh/id_ed25519_work", "")
	if _, err := b.writeSSHBlock("work", hostBlock, sshconfig.RenderGlobalBlock("linux")); err != nil {
		t.Fatalf("writeSSHBlock: %v", err)
	}
	included := readFile(t, filepath.Join(home, ".ssh", "config.d", "gitid.config"))
	if strings.Contains(included, "UseKeychain") {
		t.Errorf("UseKeychain must never be written off darwin:\n%s", included)
	}
}

// TestCreateInputCarriesPlatformGlobals proves the globals block is derived
// from the ACTUAL platform on every create (D-08), not hardcoded.
func TestCreateInputCarriesPlatformGlobals(t *testing.T) {
	b := newBackendForHome(t.TempDir())
	in := b.createInput(tuikit.DemoIdentity{Name: "personal", SSHHost: "personal.github.com"})
	want := sshconfig.RenderGlobalBlock(platform.CurrentOS())
	if in.GlobalBlock != want {
		t.Errorf("CreateInput.GlobalBlock = %q, want RenderGlobalBlock(%q) = %q", in.GlobalBlock, platform.CurrentOS(), want)
	}
}

// TestWriteSSHBlockBacksUpAnExistingTarget proves TEST-03/CLAUDE.md's safe-write
// invariant: an existing config is backed up before it is rewritten, and foreign
// hand-written content survives byte-for-byte.
func TestWriteSSHBlockBacksUpAnExistingTarget(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedSSHDir(t, home)
	configPath := filepath.Join(home, ".ssh", "config")
	const foreign = "# hand-written, gitid must never touch this\nHost legacy\n  Hostname example.com\n"
	writeFile(t, configPath, foreign+managedBlock("personal", "Host personal.github.com\n"))

	b := newBackendForHome(home)
	backup, err := b.writeSSHBlock("personal",
		sshconfig.RenderHostBlock("personal.github.com", "ssh.github.com", 443, "~/.ssh/id_ed25519_personal", ""), "")
	if err != nil {
		t.Fatalf("writeSSHBlock: %v", err)
	}
	if backup == "" {
		t.Fatal("no backup path returned for a pre-existing config (safe-write invariant)")
	}
	if !strings.Contains(readFile(t, backup), foreign) {
		t.Error("the backup does not contain the pre-write content")
	}
	if !strings.Contains(readFile(t, configPath), foreign) {
		t.Error("hand-written foreign content was lost by the managed-block rewrite")
	}
}

// TestCreateWritePlanReportsBothFreshIncludeChanges proves the D-06 ceremony
// contract: a fresh Include'd create previews BOTH file changes — the
// gitid.config write AND the Include line added to ~/.ssh/config — with dynamic
// paths, never a hardcoded ~/.ssh/config.
func TestCreateWritePlanReportsBothFreshIncludeChanges(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedSSHDir(t, home)
	writeFile(t, filepath.Join(home, ".ssh", "config"), "Host legacy\n  Hostname example.com\n")

	b := newBackendForHome(home)
	plan := b.CreateWritePlan(tuikit.CreateSpec{Identity: "personal", Alias: "personal.github.com"}, nil)

	wantTargets := []string{"~/.ssh/config.d/gitid.config", "~/.ssh/config"}
	if len(plan.Targets) != len(wantTargets) {
		t.Fatalf("Targets = %v, want %v (both fresh-Include changes previewed)", plan.Targets, wantTargets)
	}
	for i, want := range wantTargets {
		if plan.Targets[i] != want {
			t.Errorf("Targets[%d] = %q, want %q", i, plan.Targets[i], want)
		}
	}
	// ~/.ssh/config exists, so it is backed up; the new gitid.config is not.
	if len(plan.Backups) != 1 || !strings.HasPrefix(plan.Backups[0], "~/.ssh/config.bak.") {
		t.Errorf("Backups = %v, want exactly the pre-existing ~/.ssh/config backed up", plan.Backups)
	}
}

// TestStagedTestConfigLeavesLiveConfigUntouched is the SSHUI-04 invariant: the
// pre-confirm test stages write ONLY to the throwaway staging config. The live
// ~/.ssh/config must be byte-identical afterwards.
func TestStagedTestConfigLeavesLiveConfigUntouched(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedSSHDir(t, home)
	configPath := filepath.Join(home, ".ssh", "config")
	const live = "Host legacy\n  Hostname example.com\n"
	writeFile(t, configPath, live)

	b := newBackendForHome(home)
	keyPath := filepath.Join(home, ".ssh", "id_ed25519_personal")
	writeFile(t, keyPath, "not-a-real-key")

	in := b.createInputFromSpec(tuikit.CreateSpec{
		Identity: "personal", Alias: "personal.github.com",
		Hostname: "ssh.github.com", Port: "443", KeyPath: keyPath,
	})
	staged := identity.StagedKey{TempPrivatePath: keyPath, FinalPrivatePath: keyPath}

	stagedConfig, err := b.deps.StageTestConfig(in, staged)
	if err != nil {
		t.Fatalf("StageTestConfig: %v", err)
	}
	if !strings.Contains(readFile(t, stagedConfig), "Host personal.github.com") {
		t.Error("the staged config does not carry the identity Host block")
	}
	if got := readFile(t, configPath); got != live {
		t.Errorf("the LIVE ~/.ssh/config was mutated before confirm (SSHUI-04):\n%s", got)
	}
	if filepath.Dir(stagedConfig) == filepath.Dir(configPath) {
		t.Errorf("the staged config %q lives next to the live config; it must be a throwaway location", stagedConfig)
	}
	if b.TestConfigPath() != stagedConfig {
		t.Errorf("TestConfigPath() = %q, want the staged config the stages run against (%q)", b.TestConfigPath(), stagedConfig)
	}
}

// ---------------------------------------------------------------------------
// D-01 store gate + D-09 collision
// ---------------------------------------------------------------------------

// TestStoreGateUnlocksOnPassAndReachable proves D-01 at the backend seam: a
// brand-new key that the provider has not been given yet is a WARNING, not a
// failure, and must not block the write. Only a hard Failure does.
func TestStoreGateUnlocksOnPassAndReachable(t *testing.T) {
	cases := []struct {
		outcome tuikit.TestOutcome
		unlock  bool
	}{
		{tuikit.TestOutcomePass, true},
		{tuikit.TestOutcomeReachableNotUploaded, true},
		{tuikit.TestOutcomeFailure, false},
	}
	for _, tc := range cases {
		b := newBackendForHome(t.TempDir())
		b.recordOutcome(tc.outcome)
		if got := b.storeUnlocked(); got != tc.unlock {
			t.Errorf("storeUnlocked() after outcome %v = %v, want %v (D-01)", tc.outcome, got, tc.unlock)
		}
	}
}

// TestPersistRefusesAfterAHardFailure proves the gate is actually enforced on
// the write path: nothing is written and the failure is recorded, not swallowed.
func TestPersistRefusesAfterAHardFailure(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedSSHDir(t, home)
	b := newBackendForHome(home)
	b.recordOutcome(tuikit.TestOutcomeFailure)

	state := tuikit.DemoState{}
	got := b.Persist(state, tuikit.AddIdentity{Identity: tuikit.DemoIdentity{Name: "personal", SSHHost: "personal.github.com"}})

	if len(got.Identities) != 0 {
		t.Errorf("a blocked create must not report an identity: %+v", got.Identities)
	}
	if b.PersistError() == nil {
		t.Error("a blocked create must record why it refused, not swallow it")
	}
	if _, err := os.Stat(filepath.Join(home, ".ssh", "config.d", "gitid.config")); err == nil {
		t.Error("a blocked create must not have written the storage target")
	}
}

// TestAliasCollisionIsIncludeAware proves D-09 holds on a fresh D-06 machine:
// the identity's Host block lives in config.d/gitid.config, so a collision
// check reading ~/.ssh/config alone would miss it and let gitid write an
// ambiguous first-match-wins alias.
func TestAliasCollisionIsIncludeAware(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedSSHDir(t, home)
	included := filepath.Join(home, ".ssh", "config.d", "gitid.config")
	if err := os.MkdirAll(filepath.Dir(included), 0o700); err != nil {
		t.Fatalf("seeding config.d: %v", err)
	}
	writeFile(t, filepath.Join(home, ".ssh", "config"), managedBlock("ssh-include", "Include ~/.ssh/config.d/*.config"))
	writeFile(t, included, managedBlock("personal",
		sshconfig.RenderHostBlock("personal.github.com", "ssh.github.com", 443, "~/.ssh/id_ed25519_personal", "")))

	b := newBackendForHome(home)
	collides, err := b.AliasCollision("personal.github.com")
	if err != nil {
		t.Fatalf(`AliasCollision("personal.github.com") error: %v`, err)
	}
	if !collides {
		t.Error(`AliasCollision("personal.github.com") = false; the Include'd identity block was not seen (D-09)`)
	}
	collides, err = b.AliasCollision("brand-new")
	if err != nil {
		t.Fatalf(`AliasCollision("brand-new") error: %v`, err)
	}
	if collides {
		t.Error(`AliasCollision("brand-new") = true, want false`)
	}
	// The reserved wiring block names are never identities.
	collides, err = b.AliasCollision("ssh-include")
	if err != nil {
		t.Fatalf(`AliasCollision("ssh-include") error: %v`, err)
	}
	if collides {
		t.Error(`AliasCollision("ssh-include") = true; a reserved block name is not an identity`)
	}
}

// ---------------------------------------------------------------------------
// D-10/D-11/D-12/D-13 — reuse-existing-key picker
// ---------------------------------------------------------------------------

// TestScanReusableKeysLabelsInUseByWithProvider proves D-12's "in use by"
// label is "<identity> (<provider-host>)" — not the bare identity name — so
// the picker's same-provider check (a plain string comparison in tuikit) has
// the provider to compare against. Derived Include-aware from the SAME
// reconstruction the identity list renders, never a second, divergent source.
func TestScanReusableKeysLabelsInUseByWithProvider(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedSSHDir(t, home)
	included := filepath.Join(home, ".ssh", "config.d", "gitid.config")
	if err := os.MkdirAll(filepath.Dir(included), 0o700); err != nil {
		t.Fatalf("seeding config.d: %v", err)
	}
	keyPath := filepath.Join(home, ".ssh", "id_ed25519_personal")
	writeFile(t, filepath.Join(home, ".ssh", "config"), managedBlock("ssh-include", "Include ~/.ssh/config.d/*.config"))
	writeFile(t, included, managedBlock("personal",
		sshconfig.RenderHostBlock("personal.github.com", "ssh.github.com", 443, keyPath, "")))
	seedGeneratedKey(t, keyPath, "personal", "")

	views := newBackendForHome(home).ScanReusableKeys()
	var found bool
	for _, v := range views {
		if v.Path == keyPath {
			found = true
			if v.InUseBy != "personal (github.com)" {
				t.Errorf("InUseBy = %q, want %q (D-12)", v.InUseBy, "personal (github.com)")
			}
		}
	}
	if !found {
		t.Fatalf("the seeded key was not among the scanned candidates: %+v", views)
	}
}

// TestManualReusePathResolvesRegularFile proves the picker's manual-path row
// (D-10) resolves a plain, non-symlinked candidate the SAME way the
// directory scan would — same fields, same D-12 label.
func TestManualReusePathResolvesRegularFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedSSHDir(t, home)
	keyPath := filepath.Join(home, ".ssh", "id_ed25519_manual")
	pubLine := seedGeneratedKey(t, keyPath, "manual", "")

	b := newBackendForHome(home)
	view, err := b.ManualReusePath(keyPath)
	if err != nil {
		t.Fatalf("ManualReusePath: %v", err)
	}
	if view.Path != keyPath {
		t.Errorf("view.Path = %q, want %q", view.Path, keyPath)
	}
	if view.Fingerprint == "" || !strings.Contains(pubLine, view.Algorithm) {
		t.Errorf("view = %+v, want algorithm/fingerprint derived from the real key material", view)
	}
}

// TestManualReusePathRejectsSymlink is the T-03-13 guard threaded all the way
// through the real Backend: a symlinked manual candidate is rejected before
// parsing, never silently followed.
func TestManualReusePathRejectsSymlink(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedSSHDir(t, home)
	realKey := filepath.Join(home, ".ssh", "id_ed25519_real")
	seedGeneratedKey(t, realKey, "real", "")
	link := filepath.Join(home, ".ssh", "id_ed25519_link")
	if err := os.Symlink(realKey, link); err != nil {
		t.Fatalf("seeding symlink fixture: %v", err)
	}

	b := newBackendForHome(home)
	if _, err := b.ManualReusePath(link); err == nil {
		t.Fatal("ManualReusePath on a symlinked candidate = nil error, want a rejection (T-03-13)")
	}
}

// TestReuseEncryptedKeyWithExistingPubSucceeds proves KEY-06/D-11 through the
// REAL constructor end to end: an encrypted private key with an EXISTING
// sibling .pub reuses successfully — no passphrase prompt. ensurePub reads
// the .pub verbatim via the real ReadPub seam (plan 03-01's fix, wired for
// real in plan 03-03) instead of trying to parse the encrypted private key,
// which is exactly the gap plan 03-06's PTY e2e closes the L2 half of.
func TestReuseEncryptedKeyWithExistingPubSucceeds(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedSSHDir(t, home)
	keyPath := filepath.Join(home, ".ssh", "id_ed25519_locked")
	seedGeneratedKey(t, keyPath, "locked", "s3cret")

	b := newBackendForHome(home)
	id := tuikit.DemoIdentity{
		Name: "personal", SSHHost: "personal.github.com", Hostname: "ssh.github.com", Port: 443,
		KeyPath: keyPath, ReuseKeyPath: keyPath,
	}
	unlockStoreForIdentity(t, b, id)
	state := b.Persist(tuikit.DemoState{}, tuikit.AddIdentity{Identity: id})

	if err := b.PersistError(); err != nil {
		t.Fatalf("Persist recorded an error reusing an encrypted key with an existing .pub: %v", err)
	}
	if len(state.Identities) != 1 {
		t.Fatalf("Identities = %v, want exactly the reused identity", state.Identities)
	}
	included := readFile(t, filepath.Join(home, ".ssh", "config.d", "gitid.config"))
	if !strings.Contains(included, "IdentityFile "+keyPath) {
		t.Errorf("the written Host block does not reference the REUSED key path:\n%s", included)
	}
}

// TestReuseDoesNotGenerateANewKey proves the D-10 reuse path never calls the
// generate seam: the reused key's own bytes on disk are untouched (no new
// key material written next to it).
func TestReuseDoesNotGenerateANewKey(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedSSHDir(t, home)
	keyPath := filepath.Join(home, ".ssh", "id_ed25519_existing")
	seedGeneratedKey(t, keyPath, "existing", "")
	before := readFile(t, keyPath)

	b := newBackendForHome(home)
	staged, err := b.stagedKeyFor(identity.CreateInput{Name: "existing"}, keyPath)
	if err != nil {
		t.Fatalf("stagedKeyFor: %v", err)
	}
	if staged.PrivPEM != nil {
		t.Error("a reused key's StagedKey must carry nil PrivPEM — PersistKey/Cleanup must be no-ops")
	}
	if staged.FinalPrivatePath != keyPath || staged.TempPrivatePath != keyPath {
		t.Errorf("staged paths = {Temp:%q Final:%q}, want both == the reused key path %q",
			staged.TempPrivatePath, staged.FinalPrivatePath, keyPath)
	}
	if got := readFile(t, keyPath); got != before {
		t.Error("the reused key's own bytes were modified — reuse must never regenerate")
	}
}

// TestDemoBannerOnlyIdentitiesIsWired proves D-16: the create-flow tab is live,
// every other tab still shows demo content and must say so.
func TestDemoBannerOnlyIdentitiesIsWired(t *testing.T) {
	b := newBackendForHome(t.TempDir())
	if b.DemoBanner(tuikit.TabIdentities) {
		t.Error("the Identities tab is wired to live data in Phase 3; it must not carry the demo banner")
	}
	for _, tab := range []tuikit.TabID{tuikit.TabGlobalSSH, tuikit.TabGlobalGit, tuikit.TabDoctor} {
		if !b.DemoBanner(tab) {
			t.Errorf("tab %v is not wired yet; it must carry the D-16 demo banner", tab)
		}
	}
}

// ---------------------------------------------------------------------------
// D-18/D-19 — real git-disabled reason + functional Skip Git
// ---------------------------------------------------------------------------

// TestGitStepDisabledReasonIsAlwaysDisabledInRealBinary proves D-19: the real
// binary's wizard Git-identity step Continue is UNCONDITIONALLY disabled,
// with its own honest reason — never the dummy's validity-based one, and
// never reused to lie about capability (there is no Git backend until
// Phase 4).
func TestGitStepDisabledReasonIsAlwaysDisabledInRealBinary(t *testing.T) {
	b := newBackendForHome(t.TempDir())
	reason, always := b.GitStepDisabledReason()
	if !always {
		t.Fatal("the real binary must ALWAYS disable the wizard's Git-step Continue button (D-19)")
	}
	if reason != "— Git configuration arrives with the next build" {
		t.Errorf("GitStepDisabledReason() reason = %q, want the frozen D-19 string", reason)
	}
}

// TestPersistSkipGitWritesSSHOnlyNoGitArtifacts proves D-18 through the REAL
// seam: a create commits ONLY the SSH leg (Host block + key) — no Git
// fragment, includeIf, or allowed_signers entry, EVEN when the identity
// carries Git fields (proving the guarantee is structural, from
// PersistSSH never calling PersistGitconfig in Phase 3 — not merely that
// the wizard happened not to fill them in).
func TestPersistSkipGitWritesSSHOnlyNoGitArtifacts(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedSSHDir(t, home)
	b := newBackendForHome(home)
	id := tuikit.DemoIdentity{
		Name: "personal", SSHHost: "personal.github.com", Hostname: "ssh.github.com", Port: 443,
		State: "complete", GitName: "Acme Identity", GitEmail: "you@acme.example",
	}
	unlockStoreForIdentity(t, b, id)

	state := b.Persist(tuikit.DemoState{}, tuikit.AddIdentity{Identity: id})

	if err := b.PersistError(); err != nil {
		t.Fatalf("Persist recorded an error on a Skip-Git create: %v", err)
	}
	if len(state.Identities) != 1 {
		t.Fatalf("Identities = %v, want exactly the SSH-only identity", state.Identities)
	}
	if _, err := os.Stat(filepath.Join(home, ".ssh", "config.d", "gitid.config")); err != nil {
		t.Errorf("the SSH leg must be written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".gitconfig.d", "personal")); !os.IsNotExist(err) {
		t.Errorf("Skip Git must not write a Git fragment (Phase 4's job); stat err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".gitconfig")); !os.IsNotExist(err) {
		t.Errorf("Skip Git must not write ~/.gitconfig (Phase 4's job); stat err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".ssh", "allowed_signers")); !os.IsNotExist(err) {
		t.Errorf("Skip Git must not write allowed_signers (Phase 4's job); stat err = %v", err)
	}
}

// ---------------------------------------------------------------------------
// 03-07 Task 1 — hermetic pre-confirm staging + transactional confirmed write
// (CR-02: nothing final before consent; CR-09: rollback-capable transaction;
// CR-01: explicit persistence result, never a premature receipt)
// ---------------------------------------------------------------------------

// finalArtifactPaths is the set of user-visible paths the create flow may
// touch, keyed for snapshot assertions: the final key pair, the live SSH
// config, the Include'd storage target, and known_hosts.
func finalArtifactPaths(home, identityName string) []string {
	sshDir := filepath.Join(home, ".ssh")
	return []string{
		filepath.Join(sshDir, "id_ed25519_"+identityName),
		filepath.Join(sshDir, "id_ed25519_"+identityName+".pub"),
		filepath.Join(sshDir, "config"),
		filepath.Join(sshDir, "config.d", "gitid.config"),
		filepath.Join(sshDir, "known_hosts"),
	}
}

// fileState is one path's captured existence + bytes + mode.
type fileState struct {
	exists bool
	bytes  []byte
	mode   os.FileMode
}

// snapshotPaths captures the state of every path for a later unchanged
// assertion — the CR-02/CR-09 proof is byte/mode equality, not "looks
// similar".
func snapshotPaths(t *testing.T, paths []string) map[string]fileState {
	t.Helper()
	out := make(map[string]fileState, len(paths))
	for _, p := range paths {
		info, err := os.Stat(p) //nolint:gosec // hermetic t.TempDir() fixture path (G304)
		if err != nil {
			out[p] = fileState{}
			continue
		}
		data, rerr := os.ReadFile(p) //nolint:gosec // hermetic t.TempDir() fixture path (G304)
		if rerr != nil {
			t.Fatalf("snapshot: reading %s: %v", p, rerr)
		}
		out[p] = fileState{exists: true, bytes: data, mode: info.Mode().Perm()}
	}
	return out
}

// assertUnchanged fails naming every path whose state drifted from want.
func assertUnchanged(t *testing.T, want, got map[string]fileState) {
	t.Helper()
	for p, w := range want {
		g := got[p]
		if w.exists != g.exists {
			t.Errorf("%s: existence changed (before=%v after=%v) — no final mutation may happen before consent", p, w.exists, g.exists)
			continue
		}
		if !w.exists {
			continue
		}
		if string(w.bytes) != string(g.bytes) {
			t.Errorf("%s: bytes changed — before:\n%s\n--- after ---\n%s", p, w.bytes, g.bytes)
		}
		if w.mode != g.mode {
			t.Errorf("%s: mode changed (%o -> %o)", p, w.mode, g.mode)
		}
	}
}

// TestGenerateStagesKeyOutsideFinalPaths proves CR-02's generate half: key
// generation writes the private test key ONLY under the backend's mode-0700
// staging directory, carrying the final destinations in StagedKey — the final
// ~/.ssh paths stay absent until the confirmed transaction.
func TestGenerateStagesKeyOutsideFinalPaths(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedSSHDir(t, home)
	b := newBackendForHome(home)

	in := b.createInputFromSpec(tuikit.CreateSpec{Identity: "fresh", Alias: "fresh.github.com", Hostname: "ssh.github.com", Port: "443"})
	staged, err := b.deps.Generate(in)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if staged.TempPrivatePath == "" || staged.TempPrivatePath == staged.FinalPrivatePath {
		t.Errorf("staged paths = {Temp:%q Final:%q}; the test key must live at a DISTINCT staging path", staged.TempPrivatePath, staged.FinalPrivatePath)
	}
	stageDir, derr := b.stagingDir()
	if derr != nil {
		t.Fatalf("stagingDir: %v", derr)
	}
	if filepath.Dir(staged.TempPrivatePath) != stageDir {
		t.Errorf("the staged test key %q must live under the staging dir %q", staged.TempPrivatePath, stageDir)
	}
	if info, serr := os.Stat(staged.TempPrivatePath); serr != nil {
		t.Errorf("the staged test key must exist for the connectivity stages: %v", serr)
	} else if info.Mode().Perm() != 0o600 {
		t.Errorf("staged test key mode = %o, want 0600", info.Mode().Perm())
	}
	if info, serr := os.Stat(stageDir); serr == nil && info.Mode().Perm() != 0o700 {
		t.Errorf("staging dir mode = %o, want 0700", info.Mode().Perm())
	}
	if _, serr := os.Stat(staged.FinalPrivatePath); !os.IsNotExist(serr) {
		t.Errorf("the FINAL private key must not exist before confirmation; stat err = %v", serr)
	}
	if _, serr := os.Stat(staged.FinalPubPath); !os.IsNotExist(serr) {
		t.Errorf("the FINAL public key must not exist before confirmation; stat err = %v", serr)
	}
	if staged.PubLine == "" || staged.PrivPEM == nil {
		t.Error("Generate must carry the public line and private bytes in memory for the confirmed transaction")
	}
}

// TestPreConfirmStagesAndCancelLeaveHomeUntouched is the CR-02/SSHUI-04
// tracer: generated-key preparation and BOTH test stages' config staging run
// against a temp HOME, then the flow is CANCELLED — every final artifact
// (private/public key, live config, Include target, known_hosts) must be
// absent or byte-identical to the pre-test snapshot.
func TestPreConfirmStagesAndCancelLeaveHomeUntouched(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedSSHDir(t, home)
	const foreign = "# hand-written, gitid must never touch this\nHost legacy\n  Hostname example.com\n"
	writeFile(t, filepath.Join(home, ".ssh", "config"), foreign)

	paths := finalArtifactPaths(home, "cancelme")
	before := snapshotPaths(t, paths)

	b := newBackendForHome(home)
	spec := tuikit.CreateSpec{Identity: "cancelme", Alias: "cancelme.github.com", Hostname: "ssh.github.com", Port: "443", KeyPath: "~/.ssh/id_ed25519_cancelme"}
	in := b.createInputFromSpec(spec)
	staged, err := b.stagedKeyFor(in, "")
	if err != nil {
		t.Fatalf("stagedKeyFor: %v", err)
	}
	if _, serr := b.deps.StageTestConfig(in, staged); serr != nil {
		t.Fatalf("StageTestConfig: %v", serr)
	}
	// The user cancels the wizard: no confirmation ever happens.

	assertUnchanged(t, before, snapshotPaths(t, paths))
	// The staging artifacts live OUTSIDE the sandbox HOME (OS temp dir).
	if strings.HasPrefix(staged.TempPrivatePath, home) {
		t.Errorf("the staged test key %q lives inside the user HOME; it must be a throwaway location", staged.TempPrivatePath)
	}
}

// TestConfirmedCreateCommitsTheCompleteSSHUnit proves the success half of the
// transaction: a confirmed create writes the key pair at 0600/0644 PLUS the
// Include line PLUS the Host block as one unit, preserving foreign bytes.
func TestConfirmedCreateCommitsTheCompleteSSHUnit(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedSSHDir(t, home)
	const foreign = "# hand-written, gitid must never touch this\nHost legacy\n  Hostname example.com\n"
	writeFile(t, filepath.Join(home, ".ssh", "config"), foreign)

	b := newBackendForHome(home)
	id := tuikit.DemoIdentity{Name: "personal", SSHHost: "personal.github.com", Hostname: "ssh.github.com", Port: 443, KeyPath: "~/.ssh/id_ed25519_personal"}
	unlockStoreForIdentity(t, b, id)

	state := b.Persist(tuikit.DemoState{}, tuikit.AddIdentity{Identity: id})
	if err := b.PersistError(); err != nil {
		t.Fatalf("Persist recorded an error on a confirmed create: %v", err)
	}
	if len(state.Identities) != 1 {
		t.Fatalf("Identities = %v, want the created identity re-read from disk", state.Identities)
	}

	privPath := filepath.Join(home, ".ssh", "id_ed25519_personal")
	pubPath := privPath + ".pub"
	if info, err := os.Stat(privPath); err != nil {
		t.Fatalf("the confirmed create must write the private key: %v", err)
	} else if info.Mode().Perm() != 0o600 {
		t.Errorf("private key mode = %o, want 0600", info.Mode().Perm())
	}
	if info, err := os.Stat(pubPath); err != nil {
		t.Fatalf("the confirmed create must write the public key: %v", err)
	} else if info.Mode().Perm() != 0o644 {
		t.Errorf("public key mode = %o, want 0644", info.Mode().Perm())
	}

	mainConfig := readFile(t, filepath.Join(home, ".ssh", "config"))
	if !strings.Contains(mainConfig, foreign) {
		t.Error("foreign hand-written content was lost by the confirmed write")
	}
	if !strings.Contains(mainConfig, "Include ~/.ssh/config.d/*.config") {
		t.Errorf("the Include line is missing from the live config:\n%s", mainConfig)
	}
	included := readFile(t, filepath.Join(home, ".ssh", "config.d", "gitid.config"))
	for _, want := range []string{"Host personal.github.com", "Port 443", "IdentitiesOnly yes", "User git", "IdentityFile " + privPath} {
		if !strings.Contains(included, want) {
			t.Errorf("the written Host block is missing %q:\n%s", want, included)
		}
	}
}

// TestCommitTransactionRollsBackAfterEveryInjectedFailure is the CR-09 proof:
// a failure injected BEFORE EACH ordered mutation restores every pre-existing
// byte/mode and removes every newly-created final artifact — no dangling
// Include line, no half-written key pair, no unreachable Host block.
func TestCommitTransactionRollsBackAfterEveryInjectedFailure(t *testing.T) {
	steps := []string{"private-key", "public-key", "include-line", "host-block"}
	for _, step := range steps {
		t.Run(step, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			seedSSHDir(t, home)
			const foreign = "# hand-written, gitid must never touch this\nHost legacy\n  Hostname example.com\n"
			writeFile(t, filepath.Join(home, ".ssh", "config"), foreign)

			paths := finalArtifactPaths(home, "personal")
			before := snapshotPaths(t, paths)
			configDDir := filepath.Join(home, ".ssh", "config.d")
			if _, err := os.Stat(configDDir); err == nil {
				t.Fatal("fixture invalid: config.d must not pre-exist for this rollback proof")
			}

			b := newBackendForHome(home)
			id := tuikit.DemoIdentity{Name: "personal", SSHHost: "personal.github.com", Hostname: "ssh.github.com", Port: 443, KeyPath: "~/.ssh/id_ed25519_personal"}
			unlockStoreForIdentity(t, b, id)
			b.failCommitAt = func(s string) error {
				if s == step {
					return fmt.Errorf("injected failure at %s", s)
				}
				return nil
			}

			state := b.Persist(tuikit.DemoState{}, tuikit.AddIdentity{Identity: id})

			if b.PersistError() == nil {
				t.Fatal("an injected transaction failure must surface as a recorded persistence error")
			}
			if !strings.Contains(b.PersistError().Error(), "injected failure at "+step) {
				t.Errorf("PersistError = %v, want the injected failure to propagate verbatim", b.PersistError())
			}
			if len(state.Identities) != 0 {
				t.Errorf("a failed create must not report identities: %+v", state.Identities)
			}

			assertUnchanged(t, before, snapshotPaths(t, paths))

			// The key pair stays a MATCHED pair: both halves absent after a
			// rollback, never a dangling private or public half.
			privPath := filepath.Join(home, ".ssh", "id_ed25519_personal")
			_, privErr := os.Stat(privPath)
			_, pubErr := os.Stat(privPath + ".pub")
			if os.IsNotExist(privErr) != os.IsNotExist(pubErr) {
				t.Errorf("rollback left an unmatched key half (private missing=%v, public missing=%v)",
					os.IsNotExist(privErr), os.IsNotExist(pubErr))
			}

			// A transaction-created config.d directory is removed when the
			// rollback leaves it empty.
			if entries, err := os.ReadDir(configDDir); err == nil && len(entries) == 0 {
				t.Error("rollback left an empty transaction-created config.d directory behind")
			}

			// No stale backup from the failed transaction survive: the
			// restored live config is byte-identical to its own backup, so
			// the backup is redundant residue the rollback must clean up.
			matches, _ := filepath.Glob(filepath.Join(home, ".ssh", "config.bak.*"))
			if len(matches) != 0 {
				t.Errorf("rollback left transaction backups behind: %v", matches)
			}
		})
	}
}

// TestCommitCreateDeliversExplicitResults proves the CR-01 seam: the create
// ceremony's confirmation dispatches an asynchronous commit whose EXPLICIT
// result message — never the confirmation itself — decides receipt or
// failure. Success carries the real timestamped backup paths.
func TestCommitCreateDeliversExplicitResults(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		seedSSHDir(t, home)
		writeFile(t, filepath.Join(home, ".ssh", "config"), "Host legacy\n  Hostname example.com\n")
		b := newBackendForHome(home)
		id := tuikit.DemoIdentity{Name: "personal", SSHHost: "personal.github.com", Hostname: "ssh.github.com", Port: 443, KeyPath: "~/.ssh/id_ed25519_personal"}
		unlockStoreForIdentity(t, b, id)

		msg := runCommitCreate(t, b, id)
		if msg.Err != "" {
			t.Fatalf("CommitCreate reported a failure on a valid create: %s", msg.Err)
		}
		if len(msg.Backups) == 0 {
			t.Error("a successful commit over a pre-existing config must report the timestamped backup it took")
		}
		if _, err := os.Stat(filepath.Join(home, ".ssh", "config.d", "gitid.config")); err != nil {
			t.Errorf("the commit must have written the storage target: %v", err)
		}
	})

	t.Run("failure", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		seedSSHDir(t, home)
		writeFile(t, filepath.Join(home, ".ssh", "config"), "Host legacy\n  Hostname example.com\n")
		before := snapshotPaths(t, finalArtifactPaths(home, "personal"))

		b := newBackendForHome(home)
		id := tuikit.DemoIdentity{Name: "personal", SSHHost: "personal.github.com", Hostname: "ssh.github.com", Port: 443, KeyPath: "~/.ssh/id_ed25519_personal"}
		unlockStoreForIdentity(t, b, id)
		b.failCommitAt = func(string) error { return fmt.Errorf("disk on fire") }

		msg := runCommitCreate(t, b, id)
		if msg.Err == "" {
			t.Fatal("CommitCreate must report the transaction failure explicitly")
		}
		if !strings.Contains(msg.Err, "disk on fire") {
			t.Errorf("CommitCreate error = %q, want the concrete operation error", msg.Err)
		}
		assertUnchanged(t, before, snapshotPaths(t, finalArtifactPaths(home, "personal")))
	})
}

// runCommitCreate executes the backend's CommitCreate command synchronously
// and returns its WizardCommitMsg.
func runCommitCreate(t *testing.T, b *realBackend, id tuikit.DemoIdentity) tuikit.WizardCommitMsg {
	t.Helper()
	cmd := b.CommitCreate(id)
	if cmd == nil {
		t.Fatal("CommitCreate returned no command")
	}
	msg, ok := cmd().(tuikit.WizardCommitMsg)
	if !ok {
		t.Fatalf("CommitCreate delivered %T, want tuikit.WizardCommitMsg", cmd())
	}
	return msg
}

// unlockStoreForIdentity records accepted stage-1 and stage-2 outcomes for
// id's current spec. Persistence tests that intend to exercise the confirmed
// write MUST establish this two-stage proof; the production gate is
// intentionally fail-closed and does not accept a single-stage record or a
// stale/absent proof.
func unlockStoreForIdentity(t *testing.T, b *realBackend, id tuikit.DemoIdentity) {
	t.Helper()
	in := b.createInput(id)
	b.recordOutcomeFor(1, tuikit.TestOutcomePass, in)
	b.recordOutcomeFor(2, tuikit.TestOutcomePass, in)
}

// ---------------------------------------------------------------------------
// Fixture helpers
// ---------------------------------------------------------------------------

// seedSSHDir creates a hermetic ~/.ssh at 0700 under home.
func seedSSHDir(t *testing.T, home string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(home, ".ssh"), 0o700); err != nil {
		t.Fatalf("seeding .ssh: %v", err)
	}
}

// seedGeneratedKey writes a real ed25519 private key (optionally
// passphrase-encrypted) plus its `.pub` sibling at keyPath, and returns the
// generated authorized-key line — the reuse-picker tests' way of seeding a
// candidate keygen.ScanReusableKeys/ScanManualKey can actually parse.
func seedGeneratedKey(t *testing.T, keyPath, identityName, passphrase string) string {
	t.Helper()
	mat, err := keygen.GenerateMaterial(keygen.Params{
		Algo: "ed25519", Identity: identityName, Comment: identityName + "@gitid", Passphrase: passphrase,
	})
	if err != nil {
		t.Fatalf("generating key fixture %s: %v", keyPath, err)
	}
	writeFile(t, keyPath, string(mat.PrivPEM))
	writeFile(t, keyPath+".pub", mat.PubLine+"\n")
	return mat.PubLine
}

// managedBlock wraps body in gitid managed sentinels for name.
func managedBlock(name, body string) string {
	return "# BEGIN gitid managed: " + name + "\n" + strings.TrimRight(body, "\n") + "\n# END gitid managed: " + name + "\n"
}

// writeFile writes a fixture file at 0600.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writing fixture %s: %v", path, err)
	}
}

// readFile reads path or fails the test.
func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path) //nolint:gosec // hermetic t.TempDir() fixture path (G304)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return string(b)
}
