package main

// wiring_test.go guards the REAL composition root.
//
// Every test here runs over a hermetic t.TempDir() fake home — the developer's
// real ~/.ssh and ~/.gitconfig are never read or written.

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/castocolina/gitid/internal/filewriter"
	"github.com/castocolina/gitid/internal/gitconfig"
	"github.com/castocolina/gitid/internal/globalgit"
	"github.com/castocolina/gitid/internal/globalssh"
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

func TestRealBackendDoesNotEmbedNoopIdentityPlanner(t *testing.T) {
	rt := reflect.TypeOf(realBackend{})
	for i := range rt.NumField() {
		if rt.Field(i).Type == reflect.TypeOf(tuikit.NoopIdentityPlanner{}) {
			t.Fatal("realBackend must not embed NoopIdentityPlanner — a missing real implementation must be a compile error")
		}
	}
}

func TestRealBackendDoesNotEmbedNoopGitFallbackAuthorPlanner(t *testing.T) {
	var _ tuikit.GitFallbackAuthorPlanner = (*realBackend)(nil)
	rt := reflect.TypeOf(realBackend{})
	for i := range rt.NumField() {
		if rt.Field(i).Type == reflect.TypeOf(tuikit.NoopGitFallbackAuthorPlanner{}) {
			t.Fatal("realBackend must not embed NoopGitFallbackAuthorPlanner — a missing real implementation must be a compile error")
		}
	}
}

func TestGitFallbackAuthorVerifySeamIsRealWired(t *testing.T) {
	b := newBackendForHome(t.TempDir())
	if b.verifyAuthorResolution != nil {
		t.Fatal("real constructor must leave verifyAuthorResolution nil so VerifyAuthorResolution runs")
	}
	src, err := os.ReadFile(filepath.Join(testRepoRoot(t), "cmd", "gitid", "lifecycle.go")) //nolint:gosec // repository source
	if err != nil {
		t.Fatalf("reading lifecycle.go: %v", err)
	}
	if !strings.Contains(string(src), "globalgit.BuildProbeDeps") {
		t.Fatal("runGitFallbackAuthorApply verify stage must call BuildProbeDeps — a nil seam silently changes behavior")
	}
	if !strings.Contains(string(src), "globalgit.VerifyAuthorResolution") {
		t.Fatal("runGitFallbackAuthorApply verify stage must call VerifyAuthorResolution")
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
			if view.Detail != res.Output {
				t.Errorf("Detail = %q, want byte-identical real ssh output %q", view.Detail, res.Output)
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
			managedBlock(sshconfig.GlobalBlockName, "IgnoreUnknown UseKeychain\n\nHost *\n  UseKeychain yes\n  AddKeysToAgent yes\n"))

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

	if _, err := b.writeSSHBlock("personal", hostBlock, "darwin"); err != nil {
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
	if _, err := b.writeSSHBlock("work", hostBlock, "linux"); err != nil {
		t.Fatalf("writeSSHBlock: %v", err)
	}
	included := readFile(t, filepath.Join(home, ".ssh", "config.d", "gitid.config"))
	if strings.Contains(included, "UseKeychain yes") {
		t.Errorf("the darwin UseKeychain DEFAULT must never be supplied on linux (the guard directive's own name is expected):\n%s", included)
	}
}

// TestCreateInputCarriesPlatformGlobals proves the globals block is derived
// from the ACTUAL platform on every create (D-08), not hardcoded: the
// CreateInput carries the PLATFORM token, and the single EnsureGlobals owner
// does the rendering.
func TestCreateInputCarriesPlatformGlobals(t *testing.T) {
	b := newBackendForHome(t.TempDir())
	in := b.createInput(tuikit.DemoIdentity{Name: "personal", SSHHost: "personal.github.com"})
	if in.GlobalsGOOS != platform.CurrentOS() {
		t.Errorf("CreateInput.GlobalsGOOS = %q, want platform.CurrentOS() = %q", in.GlobalsGOOS, platform.CurrentOS())
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

// TestNewBackendForHomeAccountsIsHermeticWithoutSetenvHOME is WR-35's
// required fix (iteration 4): newBackendForHome(home) alone — WITHOUT the
// caller ALSO calling t.Setenv("HOME", home) — must read identities from
// home, never from the real developer's actual $HOME. This is the exact gap
// a reviewer's CR-09 probe found by accident: newBackendForHome(t.TempDir())
// returned the reviewer's OWN real GitHub identity read from their real
// ~/.ssh/config, despite the doc comment's claim that the seam exists "so
// tests can drive the whole composition root over a hermetic fake home
// without ever touching the developer's real ~/.ssh". Deliberately does NOT
// call t.Setenv("HOME", ...) — that is the point of this test.
func TestNewBackendForHomeAccountsIsHermeticWithoutSetenvHOME(t *testing.T) {
	home := t.TempDir()
	seedSSHDir(t, home)
	const sentinelAlias = "wr35-hermeticity-sentinel.github.com"
	writeFile(t, filepath.Join(home, ".ssh", "config"),
		managedBlock("wr35-sentinel",
			sshconfig.RenderHostBlock(sentinelAlias, "ssh.github.com", 443, "~/.ssh/id_ed25519_wr35-sentinel", "")))

	b := newBackendForHome(home) // no t.Setenv("HOME", home) — proving the seam itself

	state := b.InitialState()
	if len(state.Identities) != 1 {
		t.Fatalf("InitialState().Identities = %v (len %d), want exactly the 1 sentinel identity seeded under home — a real developer identity leaked in if this is wrong",
			state.Identities, len(state.Identities))
	}
	if got := state.Identities[0].SSHHost; got != sentinelAlias {
		t.Fatalf("InitialState().Identities[0].SSHHost = %q, want the sentinel alias %q seeded under the sandboxed home — accounts() read some OTHER home ($HOME or the real developer's)",
			got, sentinelAlias)
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

// TestGitWritePlanReportsFreshHomeNoBackupsButCreatesEveryDir is CR-12's
// required fix: on a completely fresh home (nothing pre-exists), GitWritePlan
// must declare NO backups (filewriter never backs up a file that does not
// yet exist) and must disclose all three directories commitGitArtifacts
// actually creates: the fragment dir, the gitdir root, and ~/.ssh.
func TestGitWritePlanReportsFreshHomeNoBackupsButCreatesEveryDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	b := newBackendForHome(home)

	plan := b.GitWritePlan(tuikit.GitSpec{
		Identity: "acme", Name: "Acme User", Email: "acme@example.test",
		SSHHost: "acme.github.com", Provider: "github.com", Strategy: "gitdir",
	})

	wantTargets := []string{"~/.gitconfig.d/acme", "~/.gitconfig", "~/.ssh/allowed_signers"}
	if len(plan.Targets) != len(wantTargets) {
		t.Fatalf("Targets = %v, want %v", plan.Targets, wantTargets)
	}
	for i, want := range wantTargets {
		if plan.Targets[i] != want {
			t.Errorf("Targets[%d] = %q, want %q", i, plan.Targets[i], want)
		}
	}
	if len(plan.Backups) != 0 {
		t.Errorf("Backups = %v, want none — nothing pre-exists on a fresh home", plan.Backups)
	}
	wantCreates := []string{"~/.gitconfig.d", "~/git/acme", "~/.ssh"}
	if len(plan.CreatedDirs) != len(wantCreates) {
		t.Fatalf("CreatedDirs = %v, want %v (fragment dir, gitdir root, and ~/.ssh — all absent on a fresh home)", plan.CreatedDirs, wantCreates)
	}
	for i, want := range wantCreates {
		if plan.CreatedDirs[i] != want {
			t.Errorf("CreatedDirs[%d] = %q, want %q", i, plan.CreatedDirs[i], want)
		}
	}
}

// TestGitWritePlanBacksUpGitconfigTwiceWhenForceSSHTakesTheProviderRewritePath
// is CR-12's exact reproduction of the review's finding: on a from-scratch
// home with ForceSSH set, WriteIncludeIf's own backup is empty (the file did
// not exist yet), but WriteProviderRewrite's later call backs up the SAME
// file a second time — because by then WriteIncludeIf has already created
// it. A caller who only checked "does ~/.gitconfig currently exist" once,
// up front, would silently miss this second, real backup.
func TestGitWritePlanBacksUpGitconfigTwiceWhenForceSSHTakesTheProviderRewritePath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	b := newBackendForHome(home)

	plan := b.GitWritePlan(tuikit.GitSpec{
		Identity: "acme", Name: "Acme User", Email: "acme@example.test",
		SSHHost: "acme.github.com", Provider: "github.com", Strategy: "gitdir",
		ForceSSH: true,
	})
	if len(plan.Backups) != 1 || !strings.HasPrefix(plan.Backups[0], "~/.gitconfig.bak.") {
		t.Errorf("Backups = %v, want exactly one ~/.gitconfig.bak.<nanos> backup (taken by WriteProviderRewrite, since WriteIncludeIf created the file moments before)", plan.Backups)
	}
}

// TestGitWritePlanReportsExistingHomeBackupsWithRealFilewriterNaming is CR-12's
// pre-existing-identity (edit) scenario: every target already exists, so
// every one is backed up, using the SAME ".bak.<nanos>" naming filewriter
// actually mints — never NewBackupPath's ".backup.<ISO>" shape.
func TestGitWritePlanReportsExistingHomeBackupsWithRealFilewriterNaming(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedSSHDir(t, home)
	b := newBackendForHome(home)

	if err := os.MkdirAll(filepath.Join(home, ".gitconfig.d"), 0o700); err != nil {
		t.Fatalf("seeding fragment dir: %v", err)
	}
	writeFile(t, filepath.Join(home, ".gitconfig.d", "acme"), "[user]\n\tname = Acme User\n")
	writeFile(t, b.gitconfigPath, "[core]\n\teditor = vim\n")
	writeFile(t, b.allowedSigners, "acme@example.test namespaces=\"git\" ssh-ed25519 AAAA\n")
	if err := os.MkdirAll(filepath.Join(home, "git", "acme"), 0o700); err != nil {
		t.Fatalf("seeding gitdir root: %v", err)
	}

	plan := b.GitWritePlan(tuikit.GitSpec{
		Identity: "acme", Name: "Acme User", Email: "acme@example.test",
		SSHHost: "acme.github.com", Provider: "github.com", Strategy: "gitdir",
	})

	if len(plan.Backups) != 3 {
		t.Fatalf("Backups = %v, want exactly 3 (fragment + gitconfig + allowed_signers, all pre-existing, ForceSSH off)", plan.Backups)
	}
	for _, want := range []string{"~/.gitconfig.d/acme.bak.", "~/.gitconfig.bak.", "~/.ssh/allowed_signers.bak."} {
		found := false
		for _, b := range plan.Backups {
			if strings.HasPrefix(b, want) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Backups = %v, missing a %q-prefixed entry", plan.Backups, want)
		}
	}
	if len(plan.CreatedDirs) != 0 {
		t.Errorf("CreatedDirs = %v, want none — every directory already exists", plan.CreatedDirs)
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

// TestScanReusableKeysLabelsTildeSpelledIdentityKey is the WR-05 regression:
// keyOwners() keyed its map on the VERBATIM Account.KeyPath (accounts(),
// not normalizedAccounts()), while it is looked up with ABSOLUTE paths
// (toReusableKeyViews' owners[k.Path], built from keygen.ScanReusableKeys'
// filepath.Glob results). For any recipe-shaped identity — IdentityFile
// spelled "~/.ssh/id_ed25519_<name>", exactly what seedDeleteFixture writes
// — the lookup missed entirely, so the reuse picker showed a key already
// owned by another identity with an empty InUseBy label: the same class of
// miss wave 9 fixed for delete, in the safety label that warns a user
// before they point a second identity at an existing key.
func TestScanReusableKeysLabelsTildeSpelledIdentityKey(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedDeleteFixture(t, home, "work")

	keyPath := filepath.Join(home, ".ssh", "id_ed25519_work")
	views := newBackendForHome(home).ScanReusableKeys()
	var found bool
	for _, v := range views {
		if v.Path == keyPath {
			found = true
			if v.InUseBy != "work (github.com)" {
				t.Errorf("WR-05: InUseBy = %q, want %q — a recipe-shaped tilde IdentityFile must still label the key as in-use", v.InUseBy, "work (github.com)")
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

// TestDemoBannerOnlyDoctorIsUnwired proves D-16: the Identities, Global SSH,
// and Global Git views are live; Doctor remains the sole fixture-backed tab
// and must still say so.
func TestDemoBannerOnlyDoctorIsUnwired(t *testing.T) {
	b := newBackendForHome(t.TempDir())
	for _, tab := range []tuikit.TabID{tuikit.TabIdentities, tuikit.TabGlobalSSH, tuikit.TabGlobalGit} {
		if b.DemoBanner(tab) {
			t.Errorf("tab %v is wired to live data and must not carry the demo banner", tab)
		}
	}
	if !b.DemoBanner(tuikit.TabDoctor) {
		t.Error("the Doctor tab is not wired yet; it must carry the D-16 demo banner")
	}
}

// ---------------------------------------------------------------------------
// D-18/D-19 — real git-disabled reason + functional Skip Git
// ---------------------------------------------------------------------------

// TestGitStepDisabledReasonUsesFormValidityInRealBinary proves Phase 4 removes
// the retired capability gate: the real binary now uses the shared form
// validity reason rather than claiming Git configuration is unavailable.
func TestGitStepDisabledReasonUsesFormValidityInRealBinary(t *testing.T) {
	b := newBackendForHome(t.TempDir())
	reason, always := b.GitStepDisabledReason()
	if always {
		t.Fatal("the real binary must let a valid Git form advance")
	}
	if reason != "" {
		t.Errorf("GitStepDisabledReason() reason = %q, want no capability override", reason)
	}
}

func TestCommitCreateWritesDefaultGitArtifacts(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedSSHDir(t, home)
	b := newBackendForHome(home)
	id := tuikit.DemoIdentity{
		Name:          "personal",
		SSHHost:       "personal.github.com",
		Hostname:      "ssh.github.com",
		Port:          443,
		KeyPath:       "~/.ssh/id_ed25519_personal",
		Provider:      "github.com",
		State:         "complete",
		GitName:       "Personal Identity",
		GitEmail:      "you@personal.example",
		MatchStrategy: "gitdir",
		GitConfigured: true,
	}
	unlockStoreForIdentity(t, b, id)

	msg := runCommitCreate(t, b, id)
	if msg.Err != "" {
		t.Fatalf("CommitCreate: %s", msg.Err)
	}

	fragment := readFile(t, filepath.Join(home, ".gitconfig.d", "personal"))
	for _, want := range []string{
		"name = Personal Identity",
		"email = you@personal.example",
		"format = ssh",
		"signingkey = ~/.ssh/id_ed25519_personal.pub",
		"gpgsign = true",
	} {
		if !strings.Contains(fragment, want) {
			t.Errorf("fragment missing %q:\n%s", want, fragment)
		}
	}
	gitconfig := readFile(t, filepath.Join(home, ".gitconfig"))
	if !strings.Contains(gitconfig, `[includeIf "gitdir:~/git/personal/"]`) {
		t.Errorf("gitconfig missing default includeIf:\n%s", gitconfig)
	}
	signers := readFile(t, filepath.Join(home, ".ssh", "allowed_signers"))
	if !strings.Contains(signers, `you@personal.example namespaces="git"`) {
		t.Errorf("allowed_signers missing byte-identical email principal:\n%s", signers)
	}
}

// TestGitTransactionTakesAtMostTwoGitconfigBackups proves the WR-06 fix: a
// single Configure-Git write must not mint a THIRD ~/.gitconfig.bak.<nanos>
// purely to obtain a backup path. With ForceSSH true (the worst case: both
// WriteIncludeIf and WriteProviderRewrite genuinely mutate ~/.gitconfig),
// exactly two real backups are expected — never a third, redundant one
// taken moments later with no new content.
func TestGitTransactionTakesAtMostTwoGitconfigBackups(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedSSHDir(t, home)
	keyPath := filepath.Join(home, ".ssh", "id_ed25519_personal")
	seedGeneratedKey(t, keyPath, "personal", "")
	writeFile(t, filepath.Join(home, ".gitconfig"), "[core]\n\teditor = vi\n")
	b := newBackendForHome(home)
	_, _, err := b.commitGitTransaction(tuikit.GitSpec{
		Identity: "personal", Name: "Personal", Email: "personal@example.test", Strategy: "gitdir",
		KeyPath: keyPath, PublicKeyPath: keyPath + ".pub", SSHHost: "personal.github.com",
		Provider: "github.com", GitDir: "~/git/personal/", ForceSSH: true,
	})
	if err != nil {
		t.Fatalf("commitGitTransaction: %v", err)
	}
	matches, globErr := filepath.Glob(filepath.Join(home, ".gitconfig.bak.*"))
	if globErr != nil {
		t.Fatalf("globbing for gitconfig backups: %v", globErr)
	}
	if len(matches) > 2 {
		t.Errorf("commitGitTransaction took %d ~/.gitconfig backups, want at most 2 (includeIf + provider-rewrite): %v", len(matches), matches)
	}
}

func TestGitTransactionCreatesSelectedContainedGitDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedSSHDir(t, home)
	b := newBackendForHome(home)
	keyPath := filepath.Join(home, ".ssh", "id_ed25519_personal")
	seedGeneratedKey(t, keyPath, "personal", "")
	gitDir := filepath.Join(home, "repos", "personal")
	_, _, err := b.commitGitTransaction(tuikit.GitSpec{
		Identity: "personal", Name: "Personal", Email: "personal@example.test",
		Strategy: "gitdir", SSHHost: "personal.github.com", Provider: "github.com",
		PublicKeyPath: "~/.ssh/id_ed25519_personal.pub", GitDir: "~/repos/personal/",
	})
	if err != nil {
		t.Fatalf("commitGitTransaction: %v", err)
	}
	info, statErr := os.Stat(gitDir)
	if statErr != nil || !info.IsDir() {
		t.Fatalf("selected gitdir %s was not created as a directory: %v", gitDir, statErr)
	}
}

// TestGitTransactionDoesNotChmodPreExistingGitDir proves the CR-01 fix:
// ensureDir must never chmod a directory the transaction did not create
// itself. Before the fix, os.Chmod(path, mode) ran unconditionally on the
// final path component, so pointing the editable gitdir field at any
// pre-existing directory (including HOME) silently tightened its mode to
// 0700 on success, with no restore.
func TestGitTransactionDoesNotChmodPreExistingGitDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedSSHDir(t, home)
	keyPath := filepath.Join(home, ".ssh", "id_ed25519_personal")
	seedGeneratedKey(t, keyPath, "personal", "")
	preExisting := filepath.Join(home, "Documents")
	if err := os.Mkdir(preExisting, 0o750); err != nil {
		t.Fatalf("seeding pre-existing gitdir: %v", err)
	}
	b := newBackendForHome(home)
	_, _, err := b.commitGitTransaction(tuikit.GitSpec{
		Identity: "personal", Name: "Personal", Email: "personal@example.test",
		Strategy: "gitdir", SSHHost: "personal.github.com", Provider: "github.com",
		PublicKeyPath: "~/.ssh/id_ed25519_personal.pub", GitDir: "~/Documents/",
	})
	if err != nil {
		t.Fatalf("commitGitTransaction: %v", err)
	}
	info, statErr := os.Stat(preExisting)
	if statErr != nil {
		t.Fatalf("stat pre-existing gitdir: %v", statErr)
	}
	if got := info.Mode().Perm(); got != 0o750 {
		t.Errorf("pre-existing gitdir mode changed to %o, want unchanged 0750", got)
	}
}

// TestGitTransactionRejectsHomeAsGitDir proves ensureDir refuses to manage
// HOME itself: a bare "~" (normalized to "~/") must never resolve to a path
// that gets chmod'ed or otherwise mutated.
func TestGitTransactionRejectsHomeAsGitDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedSSHDir(t, home)
	keyPath := filepath.Join(home, ".ssh", "id_ed25519_personal")
	seedGeneratedKey(t, keyPath, "personal", "")
	before, statErr := os.Stat(home)
	if statErr != nil {
		t.Fatalf("stat home: %v", statErr)
	}
	b := newBackendForHome(home)
	_, _, err := b.commitGitTransaction(tuikit.GitSpec{
		Identity: "personal", Name: "Personal", Email: "personal@example.test",
		Strategy: "gitdir", SSHHost: "personal.github.com", Provider: "github.com",
		PublicKeyPath: "~/.ssh/id_ed25519_personal.pub", GitDir: "~/",
	})
	if err == nil {
		t.Fatal("commitGitTransaction with GitDir=\"~/\" succeeded, want refusal to manage HOME")
	}
	after, statErr := os.Stat(home)
	if statErr != nil {
		t.Fatalf("stat home after failed transaction: %v", statErr)
	}
	if before.Mode().Perm() != after.Mode().Perm() {
		t.Errorf("HOME mode changed from %o to %o", before.Mode().Perm(), after.Mode().Perm())
	}
}

// TestCommitCreateHardensPreExistingSSHDir proves the CR-05 fix: unlike the
// user-editable gitdir (CR-01), gitid's own managed roots — starting with
// ~/.ssh — must still be secured to their documented mode even when they
// pre-exist the transaction. The CR-01 fix over-corrected by dropping the
// chmod for every path ensureDir touches, including gitid's own roots; a
// stale ~/.ssh at 0777 survived a full confirmed create untouched, and gitid
// wrote a new private key into a world-writable directory.
func TestCommitCreateHardensPreExistingSSHDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".ssh"), 0o777); err != nil { //nolint:gosec // intentionally loose fixture: proves gitid hardens it back to sshDirMode
		t.Fatalf("seeding loose ~/.ssh: %v", err)
	}
	b := newBackendForHome(home)
	id := tuikit.DemoIdentity{
		Name:          "personal",
		SSHHost:       "personal.github.com",
		Hostname:      "ssh.github.com",
		Port:          443,
		KeyPath:       "~/.ssh/id_ed25519_personal",
		Provider:      "github.com",
		State:         "complete",
		GitName:       "Personal Identity",
		GitEmail:      "you@personal.example",
		MatchStrategy: "gitdir",
		GitConfigured: true,
	}
	unlockStoreForIdentity(t, b, id)

	msg := runCommitCreate(t, b, id)
	if msg.Err != "" {
		t.Fatalf("CommitCreate: %s", msg.Err)
	}
	info, statErr := os.Stat(filepath.Join(home, ".ssh"))
	if statErr != nil {
		t.Fatalf("stat ~/.ssh after commit: %v", statErr)
	}
	if got := info.Mode().Perm(); got != sshDirMode {
		t.Errorf("~/.ssh mode after a confirmed create = %o, want %o (pre-existing at 0777 must still be hardened)", got, sshDirMode)
	}
}

// TestCommitGitTransactionHardensPreExistingSSHDir is WR-24's positive
// counterpart to TestCommitCreateHardensPreExistingSSHDir, above, for the
// STANDALONE Configure-Git path (commitGitTransaction / commitGitArtifacts):
// CR-05's ensureManagedDir hardening previously only ran on the CREATE path
// (createStagedKey) — a stale ~/.ssh at 0777 survived a confirmed standalone
// Git-config commit untouched, even though WriteAllowedSignersReplacing
// writes into that same directory.
func TestCommitGitTransactionHardensPreExistingSSHDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".ssh"), 0o777); err != nil { //nolint:gosec // intentionally loose fixture: proves gitid hardens it back to sshDirMode
		t.Fatalf("seeding loose ~/.ssh: %v", err)
	}
	keyPath := filepath.Join(home, ".ssh", "id_ed25519_personal")
	seedGeneratedKey(t, keyPath, "personal", "")
	b := newBackendForHome(home)
	_, _, err := b.commitGitTransaction(tuikit.GitSpec{
		Identity: "personal", Name: "Personal", Email: "personal@example.test",
		Strategy: "gitdir", SSHHost: "personal.github.com", Provider: "github.com",
		PublicKeyPath: "~/.ssh/id_ed25519_personal.pub", GitDir: "~/git/personal/",
	})
	if err != nil {
		t.Fatalf("commitGitTransaction: %v", err)
	}
	info, statErr := os.Stat(filepath.Join(home, ".ssh"))
	if statErr != nil {
		t.Fatalf("stat ~/.ssh after commit: %v", statErr)
	}
	if got := info.Mode().Perm(); got != sshDirMode {
		t.Errorf("~/.ssh mode after a confirmed standalone Git-config commit = %o, want %o (pre-existing at 0777 must still be hardened)", got, sshDirMode)
	}
}

// TestCommitGitArtifactsCreatesAbsentSSHDir is WR-24's other half: on a
// from-scratch HOME with no ~/.ssh at all, the standalone Configure-Git path
// used to fail outright at the LAST step (WriteAllowedSignersReplacing
// creating a temp file inside a directory that does not exist), AFTER the
// Git fragment and includeIf block had already been written and had to be
// rolled back. ensureManagedDir creates the directory (like it already does
// for b.fragmentDir), so the transaction now succeeds end to end. Calls
// commitGitArtifacts directly with an explicit pubLine (rather than going
// through commitGitTransaction, which reads the public key file from disk)
// so this test isolates the ~/.ssh-absence path from the unrelated concern
// of where the signing key itself lives.
func TestCommitGitArtifactsCreatesAbsentSSHDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// Deliberately no ~/.ssh at all — not even seedSSHDir.
	b := newBackendForHome(home)
	_, _, err := b.commitGitArtifacts(tuikit.GitSpec{
		Identity: "personal", Name: "Personal", Email: "personal@example.test",
		Strategy: "gitdir", SSHHost: "personal.github.com", Provider: "github.com",
		// KeyPath only supplies the (never-read, since pubLine is passed
		// explicitly below) public-key path shape — it does not need to
		// exist on disk.
		KeyPath: "~/.ssh/id_ed25519_personal", GitDir: "~/git/personal/",
	}, "ssh-ed25519 AAAAstubkeymaterial stub@gitid-test\n", nil)
	if err != nil {
		t.Fatalf("commitGitArtifacts on a from-scratch HOME (no ~/.ssh): %v", err)
	}
	info, statErr := os.Stat(filepath.Join(home, ".ssh"))
	if statErr != nil {
		t.Fatalf("~/.ssh was never created even though the transaction reached the allowed-signers write: %v", statErr)
	}
	if got := info.Mode().Perm(); got != sshDirMode {
		t.Errorf("~/.ssh created mode = %o, want %o", got, sshDirMode)
	}
	if _, statErr := os.Stat(filepath.Join(home, ".ssh", "allowed_signers")); statErr != nil {
		t.Errorf("allowed_signers was not written into the freshly created ~/.ssh: %v", statErr)
	}
}

func TestToDemoIdentityProjectsGitEditFields(t *testing.T) {
	b := newBackendForHome(t.TempDir())
	row := b.toDemoIdentity(identity.Account{
		Name: "personal", GitName: "Personal", GitEmail: "personal@example.test", Provider: "github.com",
		Alias: "personal.github.com", KeyPath: filepath.Join(b.home, ".ssh", "id_personal"),
		PubPath: filepath.Join(b.home, ".ssh", "id_personal.pub"), FragmentPath: filepath.Join(b.home, ".gitconfig.d", "personal"),
		Matches: []gitconfig.Match{{Kind: gitconfig.MatchGitdir, Value: "~/repos/personal/"}, {Kind: gitconfig.MatchHasconfig, Value: "remote.*.url:git@personal.github.com:*/**"}},
	})
	if row.Provider != "github.com" || row.PublicKeyPath != "~/.ssh/id_personal.pub" || row.GitDir != "~/repos/personal/" || row.MatchStrategy != "both" || !row.GitConfigured {
		t.Errorf("DemoIdentity projection = %+v", row)
	}
}

func TestGitTransactionRollbackMatrixPreservesSnapshotsAndSafetyBackups(t *testing.T) {
	// WR-24: "ssh-dir" is the new ensureManagedDir(b.sshDir, sshDirMode)
	// boundary commitGitArtifacts now carries — a distinct fault-injection
	// point from createStagedKey's own "ssh-dir" step (never reached on
	// this standalone commitGitTransaction path).
	steps := []string{"git-fragment-dir", "gitdir", "git-fragment-backup", "git-fragment", "git-includeif", "provider-rewrite", "allowed-signers-file", "ssh-dir", "allowed-signers"}
	for _, step := range steps {
		t.Run(step, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			seedSSHDir(t, home)
			keyPath := filepath.Join(home, ".ssh", "id_ed25519_personal")
			pubLine := seedGeneratedKey(t, keyPath, "personal", "")
			fragmentPath := filepath.Join(home, ".gitconfig.d", "personal")
			gitconfigPath := filepath.Join(home, ".gitconfig")
			signersPath := filepath.Join(home, ".ssh", "allowed_signers")
			if err := os.MkdirAll(filepath.Dir(fragmentPath), 0o700); err != nil {
				t.Fatalf("seeding fragment directory: %v", err)
			}
			writeFile(t, fragmentPath, "[user]\n\tname = Before\n\temail = before@example.test\n")
			writeFile(t, gitconfigPath, "[core]\n\teditor = vi\n")
			writeFile(t, signersPath, managedBlock("personal", mustAllowedSignersLine(t, "before@example.test", pubLine)))
			gitDir := filepath.Join(home, "git", "personal")
			paths := []string{fragmentPath, gitconfigPath, signersPath}
			before := snapshotPaths(t, paths)
			b := newBackendForHome(home)
			b.failCommitAt = func(boundary string) error {
				if boundary == step {
					return fmt.Errorf("injected failure at %s", boundary)
				}
				return nil
			}
			backups, restored, err := b.commitGitTransaction(tuikit.GitSpec{
				Identity: "personal", Name: "After", Email: "after@example.test", Strategy: "both",
				KeyPath: keyPath, PublicKeyPath: keyPath + ".pub", SSHHost: "personal.github.com",
				Provider: "github.com", GitDir: gitDir, ForceSSH: true,
			})
			if err == nil || !strings.Contains(err.Error(), "injected failure at "+step) {
				t.Fatalf("error = %v, want injected boundary %q", err, step)
			}
			assertUnchanged(t, before, snapshotPaths(t, paths))
			if _, statErr := os.Stat(gitDir); !os.IsNotExist(statErr) {
				t.Errorf("rollback left transaction-created gitdir %s: %v", gitDir, statErr)
			}
			if _, statErr := os.Stat(filepath.Join(home, ".gitconfig.d")); statErr != nil {
				t.Errorf("pre-existing fragment directory was removed: %v", statErr)
			}
			if len(backups) > 0 {
				for _, backup := range backups {
					if _, statErr := os.Stat(backup); statErr != nil {
						t.Errorf("rollback removed safety backup %s: %v", backup, statErr)
					}
				}
			}
			if step != "git-fragment-dir" && len(restored) == 0 {
				t.Error("rollback must report restoration outcomes after a mutation boundary")
			}
		})
	}
}

// TestRollbackKeepsHardenedRootSecuredWhenAFileUnderItFailsToRestore is
// BL-15's required fault-injection case (was WR-25, escalated from skip):
// restore() must NEVER re-loosen a managed root (~/.ssh) back to its
// pre-transaction mode when a file underneath it — here, the freshly
// written allowed_signers file — could not itself be restored/removed. The
// old unconditional chmodDirs loop would revert ~/.ssh to whatever loose
// mode it had before the transaction (0755 in this fixture), leaving a
// partially-rolled-back transaction's leftover file inside a
// group/world-readable directory.
func TestRollbackKeepsHardenedRootSecuredWhenAFileUnderItFailsToRestore(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// ~/.ssh pre-exists at a LOOSE mode — ensureManagedDir will harden it to
	// sshDirMode (0700) and record 0755 as the mode restore() would
	// normally revert to.
	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0o755); err != nil { //nolint:gosec // deliberately loose fixture mode — proving BL-15's guard rejects reverting to it
		t.Fatalf("seeding loose ~/.ssh: %v", err)
	}
	keyPath := filepath.Join(sshDir, "id_ed25519_personal")
	seedGeneratedKey(t, keyPath, "personal", "")
	signersPath := filepath.Join(sshDir, "allowed_signers")

	b := newBackendForHome(home)
	b.failCommitAt = func(boundary string) error {
		switch boundary {
		case "allowed-signers":
			// Fail the LAST mutation step (after ensureManagedDir(~/.ssh)
			// has already hardened the directory) so restore() runs.
			return fmt.Errorf("injected failure at allowed-signers")
		case "restore:" + signersPath:
			// The allowed_signers path never got a real file this run (the
			// write never happened), so restore()'s own os.Remove would
			// otherwise succeed as a harmless no-op — force a failure here
			// so a real "file under ~/.ssh could not be restored" case
			// exists for the chmodDirs guard to react to.
			return fmt.Errorf("injected restore failure for allowed_signers")
		}
		return nil
	}
	_, restored, err := b.commitGitTransaction(tuikit.GitSpec{
		Identity: "personal", Name: "After", Email: "after@example.test", Strategy: "gitdir",
		KeyPath: keyPath, PublicKeyPath: keyPath + ".pub", SSHHost: "personal.github.com",
		Provider: "github.com", ForceSSH: false,
	})
	if err == nil || !strings.Contains(err.Error(), "allowed-signers") {
		t.Fatalf("error = %v, want the injected allowed-signers failure", err)
	}
	if !strings.Contains(err.Error(), "injected restore failure for allowed_signers") {
		t.Fatalf("error = %v, want the restore-path failure surfaced too", err)
	}

	info, statErr := os.Stat(sshDir)
	if statErr != nil {
		t.Fatalf("stat ~/.ssh after rollback: %v", statErr)
	}
	if got, want := info.Mode().Perm(), os.FileMode(0o700); got != want {
		t.Fatalf("BL-15: ~/.ssh mode after rollback = %v, want %v (kept SECURED — a file under it failed to restore, so the pre-transaction loose mode must NOT be reapplied)", got, want)
	}

	foundKeptMsg := false
	for _, line := range restored {
		if strings.Contains(line, ".ssh") && strings.Contains(line, "mode kept at") {
			foundKeptMsg = true
		}
	}
	if !foundKeptMsg {
		t.Errorf("restore() outcomes = %v, want a %q line naming ~/.ssh", restored, "mode kept at")
	}
}

func TestCombinedTransactionRollsBackEverySSHAndGitTarget(t *testing.T) {
	steps := []string{"ssh-dir", "private-key", "public-key", "include-line", "host-block", "git-fragment-dir", "gitdir", "git-fragment-backup", "git-fragment", "git-includeif", "provider-rewrite", "allowed-signers-file", "allowed-signers"}
	for _, step := range steps {
		t.Run(step, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			seedSSHDir(t, home)
			sshConfig := filepath.Join(home, ".ssh", "config")
			fragment := filepath.Join(home, ".gitconfig.d", "personal")
			gitconfigPath := filepath.Join(home, ".gitconfig")
			signers := filepath.Join(home, ".ssh", "allowed_signers")
			writeFile(t, sshConfig, "Host legacy\n  Hostname example.test\n")
			if err := os.MkdirAll(filepath.Dir(fragment), 0o700); err != nil {
				t.Fatalf("seeding fragment dir: %v", err)
			}
			if err := os.Chmod(filepath.Dir(fragment), 0o700); err != nil { //nolint:gosec // directory fixture must preserve mode through rollback
				t.Fatalf("securing fragment dir fixture: %v", err)
			}
			writeFile(t, fragment, "[user]\n\tname = Before\n")
			writeFile(t, gitconfigPath, "[core]\n\teditor = vi\n")
			writeFile(t, signers, "before@example.test namespaces=\"git\" ssh-ed25519 AAAABefore\n")
			paths := []string{filepath.Join(home, ".ssh", "id_ed25519_personal"), filepath.Join(home, ".ssh", "id_ed25519_personal.pub"), sshConfig, filepath.Join(home, ".ssh", "config.d", "gitid.config"), fragment, gitconfigPath, signers}
			before := snapshotPaths(t, paths)
			b := newBackendForHome(home)
			id := tuikit.DemoIdentity{Name: "personal", SSHHost: "personal.github.com", Hostname: "ssh.github.com", Port: 443, KeyPath: "~/.ssh/id_ed25519_personal", Provider: "github.com", GitConfigured: true, GitName: "After", GitEmail: "after@example.test", MatchStrategy: "both", GitDir: "~/repos/personal/", ForceSSH: true}
			unlockStoreForIdentity(t, b, id)
			b.failCommitAt = func(boundary string) error {
				if boundary == step {
					return fmt.Errorf("injected failure at %s", boundary)
				}
				return nil
			}
			msg := runCommitCreate(t, b, id)
			if msg.Err == "" || !strings.Contains(msg.Err, "mutation "+step+" failed") || !strings.Contains(msg.Err, "restoration results:") {
				t.Fatalf("error = %q, want failed target and restoration results", msg.Err)
			}
			assertUnchanged(t, before, snapshotPaths(t, paths))
			if info, err := os.Stat(filepath.Dir(fragment)); err != nil || info.Mode().Perm() != 0o700 {
				t.Fatalf("fragment directory restoration = (%v, %v), want mode 0700", info, err)
			}
			if _, err := os.Stat(filepath.Join(home, "repos", "personal")); !os.IsNotExist(err) {
				t.Errorf("rollback left created gitdir: %v", err)
			}
		})
	}
}

func TestCombinedTransactionReportsRestorationFailure(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedSSHDir(t, home)
	b := newBackendForHome(home)
	id := tuikit.DemoIdentity{Name: "personal", SSHHost: "personal.github.com", Hostname: "ssh.github.com", Port: 443, KeyPath: "~/.ssh/id_ed25519_personal", Provider: "github.com", GitConfigured: true, GitName: "Personal", GitEmail: "personal@example.test", MatchStrategy: "gitdir"}
	unlockStoreForIdentity(t, b, id)
	b.failCommitAt = func(boundary string) error {
		if boundary == "git-fragment" {
			return fmt.Errorf("injected failure at git-fragment")
		}
		if strings.HasPrefix(boundary, "restore:") {
			return fmt.Errorf("forced restoration failure")
		}
		return nil
	}
	msg := runCommitCreate(t, b, id)
	if msg.Err == "" || !strings.Contains(msg.Err, "mutation git-artifacts failed") || !strings.Contains(msg.Err, "forced restoration failure") || !strings.Contains(msg.Err, "restoration results:") {
		t.Fatalf("error = %q, want original failed target and forced restoration outcome", msg.Err)
	}
}

// TestCombinedTransactionRetainsBackupsWhenRestorationFails proves the CR-02
// fix: commitCreateTransaction.fail must NEVER delete the timestamped
// backups it took, especially when restore() itself fails — that is exactly
// the case the backups exist for. Before the fix, fail() unconditionally
// os.Remove'd every entry in journal.backups regardless of restoreErr,
// destroying the only durable recovery copy alongside a half-written file.
func TestCombinedTransactionRetainsBackupsWhenRestorationFails(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedSSHDir(t, home)
	fragment := filepath.Join(home, ".gitconfig.d", "personal")
	if err := os.MkdirAll(filepath.Dir(fragment), 0o700); err != nil {
		t.Fatalf("seeding fragment dir: %v", err)
	}
	writeFile(t, fragment, "[user]\n\tname = Before\n\temail = before@example.test\n")
	b := newBackendForHome(home)
	id := tuikit.DemoIdentity{Name: "personal", SSHHost: "personal.github.com", Hostname: "ssh.github.com", Port: 443, KeyPath: "~/.ssh/id_ed25519_personal", Provider: "github.com", GitConfigured: true, GitName: "After", GitEmail: "after@example.test", MatchStrategy: "gitdir"}
	unlockStoreForIdentity(t, b, id)
	b.failCommitAt = func(boundary string) error {
		if boundary == "git-includeif" {
			return fmt.Errorf("injected failure at git-includeif")
		}
		if strings.HasPrefix(boundary, "restore:") {
			return fmt.Errorf("forced restoration failure")
		}
		return nil
	}
	msg := runCommitCreate(t, b, id)
	if msg.Err == "" || !strings.Contains(msg.Err, "timestamped backups retained:") {
		t.Fatalf("error = %q, want failure message to record retained backups", msg.Err)
	}
	matches, globErr := filepath.Glob(fragment + ".bak.*")
	if globErr != nil {
		t.Fatalf("globbing for retained fragment backup: %v", globErr)
	}
	if len(matches) == 0 {
		t.Fatal("commitCreateTransaction.fail deleted the fragment backup after a failed restoration")
	}
}

// TestCombinedTransactionSurfacesBackupsOnFailureEvenWhenRestorationSucceeds
// proves the WR-21 fix: commitCreateTransaction.fail returned nil backups
// unconditionally, so WizardCommitMsg.Backups was always empty on a failed
// create — even in the common case where rollback SUCCEEDS and the
// timestamped backups it took before failing are retained on disk (CR-02)
// with no way for the caller/UI to discover or clean them up. Before the
// fix, only the error STRING mentioned retained backups, and only when
// restoreErr != nil (a rarer, harder-to-hit case than this one).
func TestCombinedTransactionSurfacesBackupsOnFailureEvenWhenRestorationSucceeds(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedSSHDir(t, home)
	fragment := filepath.Join(home, ".gitconfig.d", "personal")
	if err := os.MkdirAll(filepath.Dir(fragment), 0o700); err != nil {
		t.Fatalf("seeding fragment dir: %v", err)
	}
	writeFile(t, fragment, "[user]\n\tname = Before\n\temail = before@example.test\n")
	b := newBackendForHome(home)
	id := tuikit.DemoIdentity{Name: "personal", SSHHost: "personal.github.com", Hostname: "ssh.github.com", Port: 443, KeyPath: "~/.ssh/id_ed25519_personal", Provider: "github.com", GitConfigured: true, GitName: "After", GitEmail: "after@example.test", MatchStrategy: "gitdir"}
	unlockStoreForIdentity(t, b, id)
	// Fail AFTER the fragment backup is taken (git-fragment-backup) but
	// BEFORE any restoration step — restore() itself must succeed normally
	// here (no injected restore failure), the common failure shape.
	b.failCommitAt = func(boundary string) error {
		if boundary == "git-includeif" {
			return fmt.Errorf("injected failure at git-includeif")
		}
		return nil
	}
	msg := runCommitCreate(t, b, id)
	if msg.Err == "" {
		t.Fatal("setup: expected the injected failure to fail the transaction")
	}
	if len(msg.Backups) == 0 {
		t.Fatal("WizardCommitMsg.Backups is empty on a failed create despite the transaction taking (and CR-02 retaining) a real backup")
	}
	for _, backup := range msg.Backups {
		if strings.HasPrefix(backup, home) {
			t.Errorf("WizardCommitMsg.Backups entry %q is a raw absolute sandbox path, want it mapped through displayPath (~/-shortened)", backup)
		}
		if !strings.Contains(backup, ".bak.") {
			t.Errorf("WizardCommitMsg.Backups entry %q does not look like a timestamped backup path", backup)
		}
	}
}

// TestCombinedTransactionErrorMessageIsDisplayShortened proves the WR-23 fix
// for commitCreateTransaction's own entry point (CommitCreate), mirroring
// TestCommitGitErrorMessageIsDisplayShortened for the standalone Git flow.
// Force a REAL gitConfigSet failure (the fragment path is a directory) via a
// combined (GitConfigured) create and assert WizardCommitMsg.Err never
// leaks the raw sandbox HOME path.
func TestCombinedTransactionErrorMessageIsDisplayShortened(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedSSHDir(t, home)
	fragmentPath := filepath.Join(home, ".gitconfig.d", "personal")
	if err := os.MkdirAll(fragmentPath, 0o700); err != nil {
		t.Fatalf("seeding fragment path as a directory: %v", err)
	}
	b := newBackendForHome(home)
	id := tuikit.DemoIdentity{
		Name: "personal", SSHHost: "personal.github.com", Hostname: "ssh.github.com", Port: 443,
		KeyPath: "~/.ssh/id_ed25519_personal", Provider: "github.com", GitConfigured: true,
		GitName: "Personal", GitEmail: "personal@example.test", MatchStrategy: "gitdir",
	}
	unlockStoreForIdentity(t, b, id)
	msg := runCommitCreate(t, b, id)
	if msg.Err == "" {
		t.Fatal("setup: expected the directory-as-fragment-path to fail the write")
	}
	if strings.Contains(msg.Err, home) {
		t.Errorf("WizardCommitMsg.Err leaks the raw absolute sandbox HOME path:\n%s", msg.Err)
	}
	if !strings.Contains(msg.Err, "~/.gitconfig.d/personal") {
		t.Errorf("WizardCommitMsg.Err missing the expected ~/-shortened fragment path:\n%s", msg.Err)
	}
}

func TestProviderFromAliasPreservesMultiLabelProvider(t *testing.T) {
	if got, want := providerFromAlias("work.github.com"), "github.com"; got != want {
		t.Errorf("providerFromAlias(work.github.com) = %q, want %q", got, want)
	}
	if got, want := providerFromAlias("work.enterprise.company.co.uk"), "enterprise.company.co.uk"; got != want {
		t.Errorf("providerFromAlias(work.enterprise.company.co.uk) = %q, want %q", got, want)
	}
}

// TestTransactionsAreSerializedAgainstEachOther proves the WR-04 fix:
// commitGitTransaction and commitCreateTransaction must be mutually
// exclusive, since both read-modify-write ~/.gitconfig and
// ~/.ssh/allowed_signers off separate tea.Cmd goroutines. Holding txMu
// manually and observing that a concurrent commitGitTransaction call
// blocks until release — then completes promptly once released — proves
// the lock is real, not merely declared.
func TestTransactionsAreSerializedAgainstEachOther(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedSSHDir(t, home)
	keyPath := filepath.Join(home, ".ssh", "id_ed25519_personal")
	seedGeneratedKey(t, keyPath, "personal", "")
	b := newBackendForHome(home)

	b.txMu.Lock()
	done := make(chan error, 1)
	go func() {
		_, _, err := b.commitGitTransaction(tuikit.GitSpec{
			Identity: "personal", Name: "Personal", Email: "personal@example.test", Strategy: "gitdir",
			KeyPath: keyPath, PublicKeyPath: keyPath + ".pub", SSHHost: "personal.github.com",
			Provider: "github.com", GitDir: "~/git/personal/", ForceSSH: true,
		})
		done <- err
	}()

	select {
	case <-done:
		t.Fatal("commitGitTransaction completed while txMu was held externally — the transactions are not serialized")
	case <-time.After(100 * time.Millisecond):
		// Expected: still blocked on txMu.
	}

	b.txMu.Unlock()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("commitGitTransaction after lock release: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("commitGitTransaction never completed after txMu was released")
	}
}

// TestCommitGitReturnsDisplayShortenedBackupPaths proves the WR-01 fix: the
// standalone Git ceremony's tea.Cmd must map every backup path through
// b.displayPath before returning it in GitCommitMsg, exactly like
// commitCreateTransaction already does for the combined flow. Before the
// fix, CommitGit returned journal.backups verbatim — a full absolute
// sandbox path that wraps across terminal rows in the receipt.
func TestCommitGitReturnsDisplayShortenedBackupPaths(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedSSHDir(t, home)
	keyPath := filepath.Join(home, ".ssh", "id_ed25519_personal")
	seedGeneratedKey(t, keyPath, "personal", "")
	// Seed a pre-existing gitconfig so the write takes a real backup.
	writeFile(t, filepath.Join(home, ".gitconfig"), "[core]\n\teditor = vi\n")
	b := newBackendForHome(home)
	cmd := b.CommitGit(tuikit.GitSpec{
		Identity: "personal", Name: "Personal", Email: "personal@example.test", Strategy: "gitdir",
		KeyPath: keyPath, PublicKeyPath: keyPath + ".pub", SSHHost: "personal.github.com",
		Provider: "github.com", GitDir: "~/git/personal/", ForceSSH: true,
	})
	if cmd == nil {
		t.Fatal("CommitGit returned nil")
	}
	msg, ok := cmd().(tuikit.GitCommitMsg)
	if !ok {
		t.Fatalf("CommitGit delivered %T, want GitCommitMsg", cmd())
	}
	if msg.Err != "" {
		t.Fatalf("CommitGit: %v", msg.Err)
	}
	if len(msg.Backups) == 0 {
		t.Fatal("CommitGit reported no backups despite a pre-existing gitconfig")
	}
	for _, backup := range msg.Backups {
		if strings.HasPrefix(backup, home) {
			t.Errorf("backup path %q is a raw absolute path, want the ~/-shortened display form", backup)
		}
		if !strings.HasPrefix(backup, "~/") {
			t.Errorf("backup path %q does not start with ~/, want the display-shortened form", backup)
		}
	}
}

// TestCommitGitErrorMessageIsDisplayShortened proves the WR-23 fix: WR-01
// shortened the explicit Backups list, but the Err STRING itself is built
// from wrapped errors that embed raw absolute paths inline in their own
// text — e.g. internal/gitconfig's gitConfigSet returns
// "git config --file %s %s: ...: %s" with an absolute path — so the
// three-row wrapping problem WR-01 described still occurred on the failure
// receipt, the screen where legibility matters most. Force a REAL
// gitConfigSet failure (the fragment path is a directory, so `git config
// --file <dir> ...` fails) and assert the resulting Err never contains the
// raw sandbox HOME path, only its ~/-shortened form.
func TestCommitGitErrorMessageIsDisplayShortened(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedSSHDir(t, home)
	keyPath := filepath.Join(home, ".ssh", "id_ed25519_personal")
	seedGeneratedKey(t, keyPath, "personal", "")
	// Seed the fragment path AS A DIRECTORY so gitConfigSet's real `git
	// config --file <dir> ...` invocation fails with a genuine error that
	// embeds this absolute path — not a synthetic injected failure.
	fragmentPath := filepath.Join(home, ".gitconfig.d", "personal")
	if err := os.MkdirAll(fragmentPath, 0o700); err != nil {
		t.Fatalf("seeding fragment path as a directory: %v", err)
	}
	b := newBackendForHome(home)
	cmd := b.CommitGit(tuikit.GitSpec{
		Identity: "personal", Name: "Personal", Email: "personal@example.test", Strategy: "gitdir",
		KeyPath: keyPath, PublicKeyPath: keyPath + ".pub", SSHHost: "personal.github.com",
		Provider: "github.com", GitDir: "~/git/personal/", ForceSSH: true,
	})
	msg, ok := cmd().(tuikit.GitCommitMsg)
	if !ok {
		t.Fatalf("CommitGit delivered %T, want GitCommitMsg", cmd())
	}
	if msg.Err == "" {
		t.Fatal("setup: expected the directory-as-fragment-path to fail the write")
	}
	if strings.Contains(msg.Err, home) {
		t.Errorf("GitCommitMsg.Err leaks the raw absolute sandbox HOME path:\n%s", msg.Err)
	}
	if !strings.Contains(msg.Err, "~/.gitconfig.d/personal") {
		t.Errorf("GitCommitMsg.Err missing the expected ~/-shortened fragment path:\n%s", msg.Err)
	}
}

func TestGitTransactionDerivesProviderFromSSHHost(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedSSHDir(t, home)
	keyPath := filepath.Join(home, ".ssh", "id_ed25519_work")
	seedGeneratedKey(t, keyPath, "work", "")
	b := newBackendForHome(home)
	_, _, err := b.commitGitTransaction(tuikit.GitSpec{
		Identity: "work", Name: "Work", Email: "work@example.test", Strategy: "gitdir",
		KeyPath: keyPath, PublicKeyPath: keyPath + ".pub", SSHHost: "work.github.com", GitDir: "~/git/work/", ForceSSH: true,
	})
	if err != nil {
		t.Fatalf("commitGitTransaction with inferred provider: %v", err)
	}
	if gitconfigText := readFile(t, filepath.Join(home, ".gitconfig")); !strings.Contains(gitconfigText, `url "git@github.com:"`) {
		t.Errorf("gitconfig missing provider rewrite:\n%s", gitconfigText)
	}
}

func TestGitTransactionReplacesExistingSignerEmail(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedSSHDir(t, home)
	keyPath := filepath.Join(home, ".ssh", "id_ed25519_acme")
	pubLine := seedGeneratedKey(t, keyPath, "acme", "")
	signersPath := filepath.Join(home, ".ssh", "allowed_signers")
	writeFile(t, signersPath, managedBlock("acme", mustAllowedSignersLine(t, "old@example.test", pubLine)))
	b := newBackendForHome(home)
	_, _, err := b.commitGitTransaction(tuikit.GitSpec{
		Identity: "acme", Name: "Acme", Email: "new@example.test", Strategy: "gitdir",
		KeyPath: keyPath, PublicKeyPath: keyPath + ".pub", SSHHost: "acme.github.com",
		Provider: "github.com", GitDir: "~/git/acme/", ForceSSH: true,
	})
	if err != nil {
		t.Fatalf("commitGitTransaction: %v", err)
	}
	signers := readFile(t, signersPath)
	if !strings.Contains(signers, mustAllowedSignersLine(t, "new@example.test", pubLine)) || strings.Contains(signers, "old@example.test") {
		t.Errorf("allowed_signers =\n%s\nwant the replacement principal only", signers)
	}
}

func TestGitTransactionSuccessIsByteStableAndReplacesSignerEmail(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedSSHDir(t, home)
	keyPath := filepath.Join(home, ".ssh", "id_ed25519_personal")
	pubLine := seedGeneratedKey(t, keyPath, "personal", "")
	b := newBackendForHome(home)
	spec := tuikit.GitSpec{
		Identity: "personal", Name: "Personal", Email: "after@example.test", Strategy: "both",
		KeyPath: keyPath, PublicKeyPath: keyPath + ".pub", SSHHost: "personal.github.com",
		Provider: "github.com", GitDir: "~/git/personal/", ForceSSH: true,
	}
	if _, _, err := b.commitGitTransaction(spec); err != nil {
		t.Fatalf("first transaction: %v", err)
	}
	paths := []string{filepath.Join(home, ".gitconfig.d", "personal"), filepath.Join(home, ".gitconfig"), filepath.Join(home, ".ssh", "allowed_signers")}
	before := snapshotPaths(t, paths)
	if _, _, err := b.commitGitTransaction(spec); err != nil {
		t.Fatalf("idempotent transaction: %v", err)
	}
	assertUnchanged(t, before, snapshotPaths(t, paths))
	signers := readFile(t, filepath.Join(home, ".ssh", "allowed_signers"))
	if !strings.Contains(signers, mustAllowedSignersLine(t, spec.Email, pubLine)) || strings.Contains(signers, "before@example.test") {
		t.Errorf("allowed_signers did not contain exactly the replacement email block:\n%s", signers)
	}
	for _, want := range []string{`[includeIf "gitdir:~/git/personal/"]`, `[includeIf "hasconfig:remote.*.url:git@personal.github.com:*/**"]`, `[url "git@github.com:"]`} {
		if !strings.Contains(readFile(t, filepath.Join(home, ".gitconfig")), want) {
			t.Errorf("gitconfig missing %q", want)
		}
	}
}

func TestMatchesForUsesExactSSHHostAndGitDir(t *testing.T) {
	// Hypothesis: hasconfig conditions use the SSH alias byte-for-byte and the
	// editable gitdir path is preserved with its required trailing slash.
	spec := tuikit.GitSpec{
		Identity: "personal", Strategy: "both", SSHHost: "team.github.example",
		GitDir: "~/repos/personal",
	}
	matches := matchesFor(spec)
	if len(matches) != 2 {
		t.Fatalf("matchesFor returned %d matches, want gitdir + hasconfig", len(matches))
	}
	if got, want := matches[0].Value, "~/repos/personal/"; got != want {
		t.Errorf("gitdir match = %q, want editable path %q", got, want)
	}
	if got, want := matches[1].Value, "remote.*.url:git@team.github.example:*/**"; got != want {
		t.Errorf("hasconfig match = %q, want exact SSH host %q", got, want)
	}
	if strings.Contains(matches[1].Value, "personal.github") {
		t.Errorf("hasconfig match must not synthesize an identity alias: %q", matches[1].Value)
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

			// CR-02: the timestamped backup from the failed transaction MUST
			// survive rollback. mutationJournal's own contract is that
			// backups "remain durable safety artifacts" regardless of
			// whether the in-memory restore succeeded; deleting them here
			// would remove the only recovery path in the case restore()
			// itself fails partway through.
			if _, statErr := os.Stat(filepath.Join(home, ".ssh", "config")); statErr != nil {
				t.Fatalf("restored ssh config missing after rollback: %v", statErr)
			}
			matches, globErr := filepath.Glob(filepath.Join(home, ".ssh", "config.bak.*"))
			if globErr != nil {
				t.Fatalf("globbing for retained ssh config backup: %v", globErr)
			}
			// Only steps that run AFTER the ssh config backup was taken
			// (backup happens inside sshconfig.EnsureIncludeLine, which
			// runs after the "include-line" injection point) can prove
			// retention; earlier steps never produced a backup to retain.
			if step == "host-block" && len(matches) == 0 {
				t.Errorf("rollback deleted the ssh config backup it should have retained: %v", matches)
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
// mustAllowedSignersLine wraps keygen.AllowedSignersLine for test fixtures
// that don't expect CR-18's comma rejection to fire; failing the test on an
// unexpected error (instead of silently ignoring it) keeps a future accidental
// comma in a fixture email visible as a test failure, not a silently-wrong line.
func mustAllowedSignersLine(t *testing.T, email, pubLine string) string {
	t.Helper()
	line, err := keygen.AllowedSignersLine(email, pubLine)
	if err != nil {
		t.Fatalf("AllowedSignersLine(%q): %v", email, err)
	}
	return line
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil { //nolint:gosec // hermetic t.TempDir() fixture path (G703)
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

// ---------------------------------------------------------------------------
// 05-01 — delete tracer (buildDeleteDeps, runDelete, CommitDelete)
// ---------------------------------------------------------------------------

// TestDeleteDepsEveryFieldIsWired is the L2 guard extended to
// identity.DeleteDeps: EVERY function field buildDeleteDeps produces must be
// non-nil, and the failure must NAME the field — the same recurring
// injected-seam wiring blindspot TestIdentityDepsEveryFieldIsWired guards
// for identity.Deps.
func TestDeleteDepsEveryFieldIsWired(t *testing.T) {
	home := t.TempDir()
	deps := buildDeleteDeps(newBackendForHome(home))

	v := reflect.ValueOf(deps)
	typ := v.Type()
	if typ.NumField() == 0 {
		t.Fatal("identity.DeleteDeps has no fields; the guard would be vacuous")
	}
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.Type.Kind() != reflect.Func {
			continue
		}
		if v.Field(i).IsNil() {
			t.Errorf("identity.DeleteDeps.%s is nil in the REAL constructor — a nil seam silently changes behavior (L2)", field.Name)
		}
	}
}

// seedDeleteFixture writes a complete identity (SSH Host block + the macOS
// "_global" block, gitconfig includeIf block, fragment file, allowed_signers
// entry, and a real key pair) under home, and returns the pubLine used so
// callers can build the matching allowed_signers line themselves if needed.
func seedDeleteFixture(t *testing.T, home, name string) (pubLine string) {
	t.Helper()
	seedSSHDir(t, home)

	sshBody := "Host " + name + ".github.com\n" +
		"  Hostname ssh.github.com\n" +
		"  Port 443\n" +
		"  User git\n" +
		"  IdentityFile ~/.ssh/id_ed25519_" + name + "\n" +
		"  IdentitiesOnly yes\n"
	sshConfig := managedBlock("_global", "Host *\n  IdentitiesOnly yes\n") + "\n" + managedBlock(name, sshBody)
	writeFile(t, filepath.Join(home, ".ssh", "config"), sshConfig)

	pubLine = seedGeneratedKey(t, filepath.Join(home, ".ssh", "id_ed25519_"+name), name, "")

	if err := os.MkdirAll(filepath.Join(home, ".gitconfig.d"), 0o700); err != nil {
		t.Fatalf("seeding .gitconfig.d: %v", err)
	}
	fragBody := "[user]\n\tname = " + name + " User\n\temail = " + name + "@example.com\n" +
		"\tsigningkey = ~/.ssh/id_ed25519_" + name + ".pub\n\n[gpg]\n\tformat = ssh\n\n[commit]\n\tgpgsign = true\n"
	writeFile(t, filepath.Join(home, ".gitconfig.d", name), fragBody)

	gcBody := "[includeIf \"gitdir:~/git/" + name + "/\"]\n\tpath = ~/.gitconfig.d/" + name + "\n"
	gitconfig := "[user]\n\tname = Global User\n\n" + managedBlock(name, gcBody)
	writeFile(t, filepath.Join(home, ".gitconfig"), gitconfig)

	line := mustAllowedSignersLine(t, name+"@example.com", pubLine)
	writeFile(t, filepath.Join(home, ".ssh", "allowed_signers"), managedBlock(name, line))

	return pubLine
}

// seedSharedKeyFixture writes two complete identities, "work" and
// "personal", whose SSH Host blocks both reference the SAME canonical key
// pair (~/.ssh/id_ed25519_work) — the CR-01/CR-02 shared-key regression
// shape. When tildeSpelling is true both Host blocks spell the shared
// IdentityFile with the recipe-shaped tilde form; when false "personal"'s
// Host block spells it as an ABSOLUTE path instead (simulating a
// gitid-written absolute IdentityFile coexisting with a recipe-written
// tilde one — the exact mixed-spelling case CR-02 names for wiring.go:3136's
// un-normalized KeyActionFor comparison). Returns the shared key's pub line.
func seedSharedKeyFixture(t *testing.T, home string, tildeSpelling bool) (pubLine string) {
	t.Helper()
	seedSSHDir(t, home)

	tildeIdentityFile := "~/.ssh/id_ed25519_work"
	absoluteIdentityFile := filepath.Join(home, ".ssh", "id_ed25519_work")
	personalIdentityFile := tildeIdentityFile
	if !tildeSpelling {
		personalIdentityFile = absoluteIdentityFile
	}

	sshBody := func(name, identityFile string) string {
		return "Host " + name + ".github.com\n" +
			"  Hostname ssh.github.com\n" +
			"  Port 443\n" +
			"  User git\n" +
			"  IdentityFile " + identityFile + "\n" +
			"  IdentitiesOnly yes\n"
	}
	sshConfig := managedBlock("_global", "Host *\n  IdentitiesOnly yes\n") + "\n" +
		managedBlock("work", sshBody("work", tildeIdentityFile)) + "\n" +
		managedBlock("personal", sshBody("personal", personalIdentityFile))
	writeFile(t, filepath.Join(home, ".ssh", "config"), sshConfig)

	pubLine = seedGeneratedKey(t, absoluteIdentityFile, "work", "")

	if err := os.MkdirAll(filepath.Join(home, ".gitconfig.d"), 0o700); err != nil {
		t.Fatalf("seeding .gitconfig.d: %v", err)
	}
	fragBody := func(name, identityFile string) string {
		return "[user]\n\tname = " + name + " User\n\temail = " + name + "@example.com\n" +
			"\tsigningkey = " + identityFile + ".pub\n\n[gpg]\n\tformat = ssh\n\n[commit]\n\tgpgsign = true\n"
	}
	writeFile(t, filepath.Join(home, ".gitconfig.d", "work"), fragBody("work", tildeIdentityFile))
	writeFile(t, filepath.Join(home, ".gitconfig.d", "personal"), fragBody("personal", personalIdentityFile))

	gcBody := func(name string) string {
		return "[includeIf \"gitdir:~/git/" + name + "/\"]\n\tpath = ~/.gitconfig.d/" + name + "\n"
	}
	gitconfig := "[user]\n\tname = Global User\n\n" +
		managedBlock("work", gcBody("work")) + "\n" +
		managedBlock("personal", gcBody("personal"))
	writeFile(t, filepath.Join(home, ".gitconfig"), gitconfig)

	workLine := mustAllowedSignersLine(t, "work@example.com", pubLine)
	personalLine := mustAllowedSignersLine(t, "personal@example.com", pubLine)
	signers := managedBlock("work", workLine) + "\n" + managedBlock("personal", personalLine)
	writeFile(t, filepath.Join(home, ".ssh", "allowed_signers"), signers)

	return pubLine
}

// homeFileListing walks home and returns every regular file path found,
// relative to home, sorted — used to prove an idempotent re-run creates NO
// new file anywhere under the fake HOME.
func homeFileListing(t *testing.T, home string) []string {
	t.Helper()
	var out []string
	err := filepath.Walk(home, func(path string, info os.FileInfo, werr error) error {
		if werr != nil {
			return werr
		}
		if info.IsDir() {
			return nil
		}
		rel, rerr := filepath.Rel(home, path)
		if rerr != nil {
			return rerr
		}
		out = append(out, rel)
		return nil
	})
	if err != nil {
		t.Fatalf("walking home: %v", err)
	}
	sort.Strings(out)
	return out
}

// TestRunDeleteGitOnlyPreservesSSHKeyAndSigners proves D-10 through the REAL
// composition root: a git-only runDelete removes the gitconfig includeIf
// block and the fragment file, and leaves ~/.ssh/config, the key pair, and
// ~/.ssh/allowed_signers byte-identical to their pre-delete state.
func TestRunDeleteGitOnlyPreservesSSHKeyAndSigners(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedDeleteFixture(t, home, "work")

	untouched := []string{
		filepath.Join(home, ".ssh", "config"),
		filepath.Join(home, ".ssh", "id_ed25519_work"),
		filepath.Join(home, ".ssh", "id_ed25519_work.pub"),
		filepath.Join(home, ".ssh", "allowed_signers"),
	}
	before := snapshotPaths(t, untouched)

	b := newBackendForHome(home)
	res, err := b.runDelete("work", identity.DeleteScopeGitOnly, lifecyclePolicy{Confirm: confirmationBypassedWithYes})
	if err != nil {
		t.Fatalf("runDelete(git-only) error: %v", err)
	}
	if len(res.Restored) != 0 {
		t.Errorf("restored = %v on a successful delete, want empty", res.Restored)
	}
	if len(res.Backups) == 0 {
		t.Error("a successful git-only delete over a pre-existing gitconfig/fragment must report backups")
	}

	assertUnchanged(t, before, snapshotPaths(t, untouched))

	if _, statErr := os.Stat(filepath.Join(home, ".gitconfig.d", "work")); !os.IsNotExist(statErr) {
		t.Errorf("fragment file survived a git-only delete: statErr=%v", statErr)
	}
	gc := readFile(t, filepath.Join(home, ".gitconfig"))
	if strings.Contains(gc, "BEGIN gitid managed: work") {
		t.Errorf("gitconfig still carries the work includeIf block:\n%s", gc)
	}
}

// TestRunDeleteGitOnlyIdempotentNoSecondFileAnywhere runs the same git-only
// delete twice and diffs the FULL fake-HOME file listing (review R-14's
// idempotency contract, stronger than checking the error value alone) — the
// second run must create no new file anywhere under HOME and report no
// backups.
func TestRunDeleteGitOnlyIdempotentNoSecondFileAnywhere(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedDeleteFixture(t, home, "work")

	b := newBackendForHome(home)
	firstRes, err := b.runDelete("work", identity.DeleteScopeGitOnly, lifecyclePolicy{Confirm: confirmationBypassedWithYes})
	if err != nil {
		t.Fatalf("first runDelete error: %v", err)
	}
	if len(firstRes.Backups) == 0 {
		t.Fatal("first runDelete should have produced at least one backup")
	}

	listingAfterFirst := homeFileListing(t, home)

	secondRes, err := b.runDelete("work", identity.DeleteScopeGitOnly, lifecyclePolicy{Confirm: confirmationBypassedWithYes})
	if err != nil {
		t.Fatalf("second (idempotent) runDelete error: %v", err)
	}
	if len(secondRes.Backups) != 0 {
		t.Errorf("second runDelete backups = %v, want none (R-14 idempotency)", secondRes.Backups)
	}
	if len(secondRes.Restored) != 0 {
		t.Errorf("second runDelete restored = %v, want none", secondRes.Restored)
	}

	listingAfterSecond := homeFileListing(t, home)
	if len(listingAfterFirst) != len(listingAfterSecond) {
		t.Fatalf("second delete changed the fake-HOME file count: %d -> %d\nbefore: %v\nafter:  %v",
			len(listingAfterFirst), len(listingAfterSecond), listingAfterFirst, listingAfterSecond)
	}
	for i := range listingAfterFirst {
		if listingAfterFirst[i] != listingAfterSecond[i] {
			t.Fatalf("second delete changed the fake-HOME file listing:\nbefore: %v\nafter:  %v", listingAfterFirst, listingAfterSecond)
		}
	}
}

// TestRunDeleteMidTransactionFailureRestores injects a failure AFTER the
// gitconfig write has already happened but BEFORE the fragment removal
// completes, and proves runDelete's journal.restore() puts the gitconfig
// bytes back exactly as they were and reports the restored path.
func TestRunDeleteMidTransactionFailureRestores(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedDeleteFixture(t, home, "work")

	gitconfigPath := filepath.Join(home, ".gitconfig")
	before := snapshotPaths(t, []string{gitconfigPath})

	b := newBackendForHome(home)
	b.failCommitAt = func(step string) error {
		if step == "delete-fragment" {
			return fmt.Errorf("injected failure at %s", step)
		}
		return nil
	}

	res, err := b.runDelete("work", identity.DeleteScopeGitOnly, lifecyclePolicy{Confirm: confirmationBypassedWithYes})
	if err == nil {
		t.Fatal("runDelete must report the injected failure")
	}
	if !strings.Contains(err.Error(), "injected failure at delete-fragment") {
		t.Errorf("error = %v, want it to contain the injected failure", err)
	}
	if len(res.Restored) == 0 {
		t.Error("a mid-transaction failure must report the restored paths")
	}

	assertUnchanged(t, before, snapshotPaths(t, []string{gitconfigPath}))
}

// TestCLIAllAndTUIEverythingBothDeleteEverything proves the 05-04 successor
// to review R-16's refusal-parity test: now that DeleteScopeEverything is
// implemented, the CLI `--all` path and the TUI's everything CommitDelete
// both SUCCEED and produce byte-identical results — because both still call
// the SAME runDelete function (D-02's one-chokepoint claim holds for the
// everything scope too, not just git-only).
func TestCLIAllAndTUIEverythingBothDeleteEverything(t *testing.T) {
	homeA := t.TempDir()
	seedDeleteFixture(t, homeA, "work")
	homeB := t.TempDir()
	seedDeleteFixture(t, homeB, "work")

	// Path A: the TUI path, Backend.CommitDelete run synchronously.
	bA := newBackendForHome(homeA)
	tuiMsg, ok := bA.CommitDelete("work", "everything")().(tuikit.DeleteCommitMsg)
	if !ok {
		t.Fatalf("CommitDelete delivered a non-DeleteCommitMsg")
	}
	if tuiMsg.Err != "" {
		t.Fatalf("TUI everything CommitDelete error: %s", tuiMsg.Err)
	}

	// Path B: the CLI verb, `--all --yes`.
	t.Setenv("HOME", homeB)
	cliCmd := newVerbCmd(newIdentityDeleteVerb())
	cliCmd.SetArgs([]string{"work", "--all", "--yes"})
	var stdout, stderr bytes.Buffer
	cliCmd.SetOut(&stdout)
	cliCmd.SetErr(&stderr)
	if cliErr := cliCmd.Execute(); cliErr != nil {
		t.Fatalf("CLI --all error: %v (stderr: %s)", cliErr, stderr.String())
	}

	// Both paths must have removed the key pair and the SSH Host block.
	for _, home := range []string{homeA, homeB} {
		if _, statErr := os.Stat(filepath.Join(home, ".ssh", "id_ed25519_work")); !os.IsNotExist(statErr) {
			t.Errorf("%s: private key survived an everything-scope delete", home)
		}
	}

	gcA := readFile(t, filepath.Join(homeA, ".gitconfig"))
	gcB := readFile(t, filepath.Join(homeB, ".gitconfig"))
	if gcA != gcB {
		t.Errorf("gitconfig bytes differ between the TUI and CLI everything-scope deletes:\nA:\n%s\n--- B ---\n%s", gcA, gcB)
	}
	sshA := readFile(t, filepath.Join(homeA, ".ssh", "config"))
	sshB := readFile(t, filepath.Join(homeB, ".ssh", "config"))
	if sshA != sshB {
		t.Errorf("ssh config bytes differ between the TUI and CLI everything-scope deletes:\nA:\n%s\n--- B ---\n%s", sshA, sshB)
	}
}

// TestRunDeleteAndCLIVerbProduceByteIdenticalGitconfig runs the git-only
// delete twice over two IDENTICAL sandbox HOMEs — once via runDelete
// directly (the TUI's own call path) and once via the CLI verb — and asserts
// the resulting ~/.gitconfig bytes are equal, proving D-02's "one shared
// chokepoint, never a second write pipeline" claim at the wiring layer (the
// e2e suite proves the same thing end-to-end through the compiled binary).
func TestRunDeleteAndCLIVerbProduceByteIdenticalGitconfig(t *testing.T) {
	homeA := t.TempDir()
	seedDeleteFixture(t, homeA, "work")
	homeB := t.TempDir()
	seedDeleteFixture(t, homeB, "work")

	// Path A: the TUI's own call path (runDelete directly).
	bA := newBackendForHome(homeA)
	if _, err := bA.runDelete("work", identity.DeleteScopeGitOnly, lifecyclePolicy{Confirm: confirmationBypassedWithYes}); err != nil {
		t.Fatalf("runDelete over homeA: %v", err)
	}

	// Path B: the CLI verb.
	t.Setenv("HOME", homeB)
	cliCmd := newVerbCmd(newIdentityDeleteVerb())
	cliCmd.SetArgs([]string{"work", "--git-only", "--yes"})
	var stdout, stderr bytes.Buffer
	cliCmd.SetOut(&stdout)
	cliCmd.SetErr(&stderr)
	if err := cliCmd.Execute(); err != nil {
		t.Fatalf("CLI delete over homeB: %v (stderr: %s)", err, stderr.String())
	}

	gcA := readFile(t, filepath.Join(homeA, ".gitconfig"))
	gcB := readFile(t, filepath.Join(homeB, ".gitconfig"))
	if gcA != gcB {
		t.Errorf("gitconfig bytes differ between runDelete and the CLI verb:\nA:\n%s\n--- B ---\n%s", gcA, gcB)
	}
}

// ---------------------------------------------------------------------------
// 05-03 Task 2 — identity.Rotate's real archive/append wiring (D-06/D-07/D-08,
// review R3-01)
// ---------------------------------------------------------------------------

// realRotateAccount reconstructs a real, tilde-expanded Account for name
// through b.findAccount — the SAME lookup runDelete uses — so a rotation
// test exercises the real Reconstruct path, never a hand-built Account.
func realRotateAccount(t *testing.T, b *realBackend, name string) identity.Account {
	t.Helper()
	acct, found := b.findAccount(name)
	if !found {
		t.Fatalf("no such identity: %q", name)
	}
	acct.FragmentPath = expandTildeForHome(acct.FragmentPath, b.home)
	acct.KeyPath = expandTildeForHome(acct.KeyPath, b.home)
	acct.PubPath = expandTildeForHome(acct.PubPath, b.home)
	// Reconstruct populates only what it parses FROM the config files;
	// GitconfigPath/SSHConfigPath/AllowedSignersPath are gitid-managed
	// TARGET paths the command layer fills in when an account is loaded for
	// a write (Account's own doc comment) — the real composition root for
	// this (plan 05-07) does not exist yet, so this test fills them from the
	// backend's own resolved paths, mirroring createInput's existing
	// pattern (wiring.go).
	acct.GitconfigPath = b.gitconfigPath
	acct.SSHConfigPath = b.sshConfigPath
	acct.AllowedSignersPath = b.allowedSigners
	return acct
}

// realHermeticDeps overrides the two seams that would otherwise shell out to
// a REAL ssh handshake (PreWrite/Resolved) with deterministic fakes, leaving
// every other seam — Generate, PersistKey, the four writers, ArchiveKeyPair,
// AppendAllowedSigners — wired to the REAL implementation. identity.Deps is
// a value type, so this is a test-local override of a COPY, never a second
// production wiring.
func realHermeticDeps(deps identity.Deps) identity.Deps {
	deps.PreWrite = func(_, _ string, _ int) tester.Result {
		return tester.Result{Outcome: tester.ReachableNotUploaded}
	}
	deps.Resolved = func(_ string) (tester.Result, tester.ResolvedConfig) {
		return tester.Result{Outcome: tester.PASS}, tester.ResolvedConfig{}
	}
	return deps
}

// TestRotateEndToEndThroughRealConstructor drives identity.Rotate over a
// hermetic fake home through the REAL composition root
// (b.depsForTransaction(newMutationJournal(b)), never b.deps, whose archive
// binding refuses by design): asserts the new key pair lands at the SAME
// canonical paths with DIFFERENT bytes (D-06/D-08), the archived private key
// exists inside the D-06 archive directory carrying the OLD bytes, the
// journal recorded both archive paths, and the allowed_signers block ends
// up with two lines for the identity (D-07 APPEND).
func TestRotateEndToEndThroughRealConstructor(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedDeleteFixture(t, home, "work")

	b := newBackendForHome(home)
	acct := realRotateAccount(t, b, "work")
	privBefore := readFile(t, acct.KeyPath)

	j := newMutationJournal(b)
	deps := realHermeticDeps(b.depsForTransaction(j))

	res, err := identity.Rotate(acct, deps)
	if err != nil {
		t.Fatalf("Rotate returned error: %v", err)
	}

	// New key pair at the SAME canonical paths (D-08), different bytes.
	if _, statErr := os.Stat(acct.KeyPath); statErr != nil {
		t.Errorf("canonical private key missing after rotation: %v", statErr)
	}
	if _, statErr := os.Stat(acct.PubPath); statErr != nil {
		t.Errorf("canonical public key missing after rotation: %v", statErr)
	}
	privAfter := readFile(t, acct.KeyPath)
	if privAfter == privBefore {
		t.Error("canonical private key bytes unchanged after rotation")
	}

	// The archived private key exists inside the archive directory and
	// carries the OLD bytes.
	if res.ArchivedPrivatePath == "" || res.ArchivedPublicPath == "" {
		t.Fatal("RotateResult must carry both archived paths")
	}
	archiveDir := sshconfig.ArchiveDir(b.sshDir)
	if !strings.HasPrefix(res.ArchivedPrivatePath, archiveDir+string(filepath.Separator)) {
		t.Errorf("archived private key path %q is not inside the archive directory %q", res.ArchivedPrivatePath, archiveDir)
	}
	archivedBytes := readFile(t, res.ArchivedPrivatePath)
	if archivedBytes != privBefore {
		t.Error("archived private key bytes do not match the pre-rotation key")
	}

	// The journal recorded both archive copies as created.
	if len(j.createdFiles) != 2 {
		t.Errorf("journal recorded %d created files, want 2 (priv+pub archive copies): %v", len(j.createdFiles), j.createdFiles)
	}

	// allowed_signers now carries two lines for "work".
	signers := readFile(t, filepath.Join(home, ".ssh", "allowed_signers"))
	lineCount := 0
	for _, line := range strings.Split(signers, "\n") {
		if strings.HasPrefix(line, "work@example.com ") {
			lineCount++
		}
	}
	if lineCount != 2 {
		t.Errorf("allowed_signers has %d lines for work@example.com, want 2 (D-07 append)\n%s", lineCount, signers)
	}
}

// TestArchiveKeyPairBackendWideBindingRefuses proves the fail-closed half of
// review R3-01: b.deps.ArchiveKeyPair (the backend-wide binding built by
// buildIdentityDeps) refuses with errArchiveOutsideTransaction and creates
// NO archive entry, while the SAME call through b.depsForTransaction(j)
// succeeds and records its paths in j.
func TestArchiveKeyPairBackendWideBindingRefuses(t *testing.T) {
	home := t.TempDir()
	seedSSHDir(t, home)
	b := newBackendForHome(home)

	privPath := filepath.Join(home, ".ssh", "id_ed25519_x")
	seedGeneratedKey(t, privPath, "x", "")
	pubPath := privPath + ".pub"

	_, _, err := b.deps.ArchiveKeyPair(privPath, pubPath)
	if !errors.Is(err, errArchiveOutsideTransaction) {
		t.Errorf("b.deps.ArchiveKeyPair error = %v, want errArchiveOutsideTransaction", err)
	}
	if _, statErr := os.Stat(privPath); statErr != nil {
		t.Errorf("source private key must be untouched after a refused archive: %v", statErr)
	}
	if entries, rerr := os.ReadDir(sshconfig.ArchiveDir(b.sshDir)); rerr == nil && len(entries) != 0 {
		t.Errorf("a refused archive must create no entry; found %d", len(entries))
	}

	j := newMutationJournal(b)
	archivedPriv, archivedPub, terr := b.depsForTransaction(j).ArchiveKeyPair(privPath, pubPath)
	if terr != nil {
		t.Fatalf("depsForTransaction's ArchiveKeyPair returned error: %v", terr)
	}
	if archivedPriv == "" || archivedPub == "" {
		t.Error("a working archive seam must return non-empty paths")
	}
	if len(j.createdFiles) != 2 {
		t.Errorf("journal recorded %d created files, want 2", len(j.createdFiles))
	}
}

// TestJournalCreatedAndWatchedSetsAreDisjoint asserts recordCreatedFile
// rejects an already-watched path and watchFile rejects an already-recorded
// created path, each naming the path (review R2-08).
func TestJournalCreatedAndWatchedSetsAreDisjoint(t *testing.T) {
	home := t.TempDir()
	b := newBackendForHome(home)

	t.Run("recordCreatedFile rejects an already-watched path", func(t *testing.T) {
		j := newMutationJournal(b)
		path := filepath.Join(home, "watched-then-created")
		if err := j.watchFile(path); err != nil {
			t.Fatalf("watchFile: %v", err)
		}
		err := j.recordCreatedFile(path)
		if err == nil {
			t.Fatal("recordCreatedFile must reject a path already watched")
		}
		if !strings.Contains(err.Error(), path) {
			t.Errorf("error must name the path; got %v", err)
		}
	})

	t.Run("watchFile rejects an already-recorded-created path", func(t *testing.T) {
		j := newMutationJournal(b)
		path := filepath.Join(home, "created-then-watched")
		if err := j.recordCreatedFile(path); err != nil {
			t.Fatalf("recordCreatedFile: %v", err)
		}
		err := j.watchFile(path)
		if err == nil {
			t.Fatal("watchFile must reject a path already recorded as created")
		}
		if !strings.Contains(err.Error(), path) {
			t.Errorf("error must name the path; got %v", err)
		}
	})
}

// TestRestoreRemovesCreatedFilesAfterRestoringWatched asserts restore()
// restores every watched file's bytes AND removes every path passed to
// recordCreatedFile.
func TestRestoreRemovesCreatedFilesAfterRestoringWatched(t *testing.T) {
	home := t.TempDir()
	b := newBackendForHome(home)
	j := newMutationJournal(b)

	watchedPath := filepath.Join(home, "watched.txt")
	writeFile(t, watchedPath, "original")
	if err := j.watchFile(watchedPath); err != nil {
		t.Fatalf("watchFile: %v", err)
	}
	writeFile(t, watchedPath, "mutated") // simulate a transaction write

	createdPath := filepath.Join(home, "created.txt")
	writeFile(t, createdPath, "new material")
	if err := j.recordCreatedFile(createdPath); err != nil {
		t.Fatalf("recordCreatedFile: %v", err)
	}

	outcomes, rerr := j.restore()
	if rerr != nil {
		t.Fatalf("restore returned error: %v; outcomes=%v", rerr, outcomes)
	}
	if got := readFile(t, watchedPath); got != "original" {
		t.Errorf("watched file after restore = %q, want original bytes", got)
	}
	if _, statErr := os.Stat(createdPath); !os.IsNotExist(statErr) {
		t.Errorf("created file must be removed after restore; statErr=%v", statErr)
	}
}

// TestRotateSecondSourceRemovalRollback is the review R3-01 regression: a
// real backend over a hermetic fake home, a real key pair, a mutationJournal
// watching both canonical key paths, failArchiveRemoveAt failing on the
// SECOND source removal, and identity.Rotate run with
// b.depsForTransaction(j). Asserts, in this order: the call returned an
// error; the journal recorded BOTH archive paths (discoverable BEFORE the
// failing removal, not after); j.restore() succeeds; the archive directory
// afterwards holds no entry for this identity; and both canonical key paths
// hold their pre-transaction bytes.
func TestRotateSecondSourceRemovalRollback(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedDeleteFixture(t, home, "work")

	b := newBackendForHome(home)
	acct := realRotateAccount(t, b, "work")
	privBefore := readFile(t, acct.KeyPath)
	pubBefore := readFile(t, acct.PubPath)

	j := newMutationJournal(b)
	if err := j.watchFile(acct.KeyPath); err != nil {
		t.Fatalf("watchFile priv: %v", err)
	}
	if err := j.watchFile(acct.PubPath); err != nil {
		t.Fatalf("watchFile pub: %v", err)
	}

	// keygen.MoveKeyPairToArchive removes priv FIRST, then pub — fail the
	// SECOND (pub) removal so priv is genuinely gone from the canonical path
	// and pub survives there, exercising the asymmetric rollback case.
	b.failArchiveRemoveAt = func(path string) error {
		if path == acct.PubPath {
			return fmt.Errorf("injected failure removing %s", path)
		}
		return nil
	}

	deps := realHermeticDeps(b.depsForTransaction(j))
	_, err := identity.Rotate(acct, deps)
	if err == nil {
		t.Fatal("Rotate must return an error when the second source removal fails")
	}

	if len(j.createdFiles) != 2 {
		t.Fatalf("journal must have recorded BOTH archive paths before the failing removal; got %v", j.createdFiles)
	}

	outcomes, rerr := j.restore()
	if rerr != nil {
		t.Fatalf("journal.restore() returned error: %v; outcomes=%v", rerr, outcomes)
	}

	entries, rderr := os.ReadDir(sshconfig.ArchiveDir(b.sshDir))
	if rderr == nil && len(entries) != 0 {
		t.Errorf("archive directory must hold no entry for this identity after rollback; found %v", entries)
	}

	if got := readFile(t, acct.KeyPath); got != privBefore {
		t.Error("canonical private key bytes not restored to pre-transaction state")
	}
	if got := readFile(t, acct.PubPath); got != pubBefore {
		t.Error("canonical public key bytes not restored to pre-transaction state")
	}
}

// TestArchiveKeyPairSeamStampCollisionRetry asserts archiveKeyPairSeam
// bumps the UnixNano stamp monotonically and retries EXACTLY once on a
// same-nanosecond collision (review R2-05/R-21): a single pre-seeded
// collision produces a DISTINCT archive path on the retry; a second
// collision (on the retried stamp too) surfaces as an error rather than
// looping.
func TestArchiveKeyPairSeamStampCollisionRetry(t *testing.T) {
	home := t.TempDir()
	seedSSHDir(t, home)
	b := newBackendForHome(home)

	fixedTime := time.Unix(0, 1234567890)
	b.archiveClockNow = func() time.Time { return fixedTime }

	archiveDir := sshconfig.ArchiveDir(b.sshDir)
	if err := os.MkdirAll(archiveDir, 0o700); err != nil {
		t.Fatalf("mkdir archive dir: %v", err)
	}
	stamp := fixedTime.UnixNano()

	t.Run("single collision retries once and succeeds", func(t *testing.T) {
		privPath := filepath.Join(home, ".ssh", "id_ed25519_x")
		seedGeneratedKey(t, privPath, "x", "")
		pubPath := privPath + ".pub"

		collidingDst := filepath.Join(archiveDir, "id_ed25519_x."+strconv.FormatInt(stamp, 10))
		writeFile(t, collidingDst, "pre-existing collision")

		j := newMutationJournal(b)
		archivedPriv, _, err := b.depsForTransaction(j).ArchiveKeyPair(privPath, pubPath)
		if err != nil {
			t.Fatalf("a single collision must be retried and succeed: %v", err)
		}
		if archivedPriv == collidingDst {
			t.Errorf("retried archive path must be DISTINCT from the collision: got %q", archivedPriv)
		}
	})

	t.Run("second collision surfaces the failure", func(t *testing.T) {
		privPath := filepath.Join(home, ".ssh", "id_ed25519_y")
		seedGeneratedKey(t, privPath, "y", "")
		pubPath := privPath + ".pub"

		collidingDst := filepath.Join(archiveDir, "id_ed25519_y."+strconv.FormatInt(stamp, 10))
		writeFile(t, collidingDst, "pre-existing collision")
		collidingRetryDst := filepath.Join(archiveDir, "id_ed25519_y."+strconv.FormatInt(stamp+1, 10))
		writeFile(t, collidingRetryDst, "pre-existing collision on the retry stamp too")

		j := newMutationJournal(b)
		_, _, err := b.depsForTransaction(j).ArchiveKeyPair(privPath, pubPath)
		if err == nil {
			t.Fatal("a second collision (on the retried stamp too) must surface as an error, not loop")
		}
	})
}

// ---------------------------------------------------------------------------
// 05-04 — everything-scope delete (D-09 provider ref-count, D-11 archive)
// ---------------------------------------------------------------------------

// TestBuildDeleteDepsCopyKeyPairToArchiveBackendWideBindingRefuses is
// delete's half of review R3-01, mirroring
// TestArchiveKeyPairBackendWideBindingRefuses's proof exactly: the
// backend-wide buildDeleteDeps(b).CopyKeyPairToArchive refuses with
// errArchiveOutsideTransaction and creates nothing, while the same seam
// obtained from b.deleteDepsForTransaction(j) succeeds and records both
// archive paths in j.
func TestBuildDeleteDepsCopyKeyPairToArchiveBackendWideBindingRefuses(t *testing.T) {
	home := t.TempDir()
	seedSSHDir(t, home)
	b := newBackendForHome(home)

	privPath := filepath.Join(home, ".ssh", "id_ed25519_x")
	seedGeneratedKey(t, privPath, "x", "")
	pubPath := privPath + ".pub"

	_, _, err := buildDeleteDeps(b).CopyKeyPairToArchive(privPath, pubPath)
	if !errors.Is(err, errArchiveOutsideTransaction) {
		t.Errorf("buildDeleteDeps(b).CopyKeyPairToArchive error = %v, want errArchiveOutsideTransaction", err)
	}
	if entries, rerr := os.ReadDir(sshconfig.ArchiveDir(b.sshDir)); rerr == nil && len(entries) != 0 {
		t.Errorf("a refused archive-copy must create no entry; found %d", len(entries))
	}

	j := newMutationJournal(b)
	archivedPriv, archivedPub, terr := b.deleteDepsForTransaction(j).CopyKeyPairToArchive(privPath, pubPath)
	if terr != nil {
		t.Fatalf("deleteDepsForTransaction's CopyKeyPairToArchive returned error: %v", terr)
	}
	if archivedPriv == "" || archivedPub == "" {
		t.Error("a working archive-copy seam must return non-empty paths")
	}
	if len(j.createdFiles) != 2 {
		t.Errorf("journal recorded %d created files, want 2", len(j.createdFiles))
	}
	// COPY semantics (review R-03): the source private key must survive.
	if _, statErr := os.Stat(privPath); statErr != nil {
		t.Errorf("source private key must survive CopyKeyPairToArchive: %v", statErr)
	}
}

// seedTwoIdentitiesSameProvider writes two complete identities ("work" and
// "personal") both on github.com, plus the shared provider-rewrite block, so
// a real-constructor test can drive the D-09 ref-count through
// runDelete/CommitDelete against actual files.
func seedTwoIdentitiesSameProvider(t *testing.T, home string) {
	t.Helper()
	seedSSHDir(t, home)

	names := []string{"work", "personal"}
	var sshConfig strings.Builder
	sshConfig.WriteString(managedBlock("_global", "Host *\n  IdentitiesOnly yes\n"))
	var gitconfigContent strings.Builder
	gitconfigContent.WriteString("[user]\n\tname = Global User\n\n")

	for _, name := range names {
		sshBody := "Host " + name + ".github.com\n" +
			"  Hostname ssh.github.com\n" +
			"  Port 443\n" +
			"  User git\n" +
			"  IdentityFile ~/.ssh/id_ed25519_" + name + "\n" +
			"  IdentitiesOnly yes\n"
		sshConfig.WriteString(managedBlock(name, sshBody))

		pubLine := seedGeneratedKey(t, filepath.Join(home, ".ssh", "id_ed25519_"+name), name, "")

		if err := os.MkdirAll(filepath.Join(home, ".gitconfig.d"), 0o700); err != nil {
			t.Fatalf("seeding .gitconfig.d: %v", err)
		}
		fragBody := "[user]\n\tname = " + name + " User\n\temail = " + name + "@example.com\n" +
			"\tsigningkey = ~/.ssh/id_ed25519_" + name + ".pub\n\n[gpg]\n\tformat = ssh\n\n[commit]\n\tgpgsign = true\n"
		writeFile(t, filepath.Join(home, ".gitconfig.d", name), fragBody)

		gcBody := "[includeIf \"gitdir:~/git/" + name + "/\"]\n\tpath = ~/.gitconfig.d/" + name + "\n"
		gitconfigContent.WriteString(managedBlock(name, gcBody))

		line := mustAllowedSignersLine(t, name+"@example.com", pubLine)
		asPath := filepath.Join(home, ".ssh", "allowed_signers")
		existing, _ := os.ReadFile(asPath) //nolint:gosec // test fixture path
		writeFile(t, asPath, string(existing)+managedBlock(name, line))
	}

	writeFile(t, filepath.Join(home, ".ssh", "config"), sshConfig.String())

	gcPath := filepath.Join(home, ".gitconfig")
	writeFile(t, gcPath, gitconfigContent.String())
	if _, err := gitconfig.WriteProviderRewrite(gcPath, "github.com", true); err != nil {
		t.Fatalf("seeding provider rewrite block: %v", err)
	}
}

// TestRunDeleteEverything_TwoIdentitiesSameProvider is the D-09 real-
// constructor proof Task 1's acceptance criteria names: delete one of two
// identities sharing a provider and the rewrite block SURVIVES; delete the
// second and it is GONE.
func TestRunDeleteEverything_TwoIdentitiesSameProvider(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedTwoIdentitiesSameProvider(t, home)

	b := newBackendForHome(home)
	gcPath := filepath.Join(home, ".gitconfig")

	if _, err := b.runDelete("work", identity.DeleteScopeEverything, lifecyclePolicy{Confirm: confirmationBypassedWithYes}); err != nil {
		t.Fatalf("runDelete(work, everything) error: %v", err)
	}
	gc1, err := os.ReadFile(gcPath) //nolint:gosec // test fixture path
	if err != nil {
		t.Fatalf("reading gitconfig after first delete: %v", err)
	}
	has, herr := gitconfig.HasProviderRewrite(gc1, "github.com")
	if herr != nil {
		t.Fatalf("HasProviderRewrite: %v", herr)
	}
	if !has {
		t.Error("provider rewrite block removed after deleting only ONE of two identities sharing it")
	}
	if _, statErr := os.Stat(filepath.Join(home, ".ssh", "id_ed25519_work")); !os.IsNotExist(statErr) {
		t.Errorf("work's private key survived an everything-scope delete: statErr=%v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(home, ".ssh", "id_ed25519_personal")); statErr != nil {
		t.Errorf("personal's private key was disturbed by work's delete: %v", statErr)
	}

	if _, err := b.runDelete("personal", identity.DeleteScopeEverything, lifecyclePolicy{Confirm: confirmationBypassedWithYes}); err != nil {
		t.Fatalf("runDelete(personal, everything) error: %v", err)
	}
	gc2, err := os.ReadFile(gcPath) //nolint:gosec // test fixture path
	if err != nil {
		t.Fatalf("reading gitconfig after second delete: %v", err)
	}
	has2, herr := gitconfig.HasProviderRewrite(gc2, "github.com")
	if herr != nil {
		t.Fatalf("HasProviderRewrite: %v", herr)
	}
	if has2 {
		t.Error("provider rewrite block survived after deleting BOTH identities sharing it")
	}
}

// TestRunDeleteEverything_HandWrittenAliasKeepsRewrite is the review R-05
// real-constructor regression: a hand-written Host stanza whose Hostname is
// the recipe's canonical alt-SSH endpoint (ssh.github.com, port 443) counts
// as a github.com reference, so deleting the ONLY managed identity for that
// provider must NOT remove the rewrite block.
func TestRunDeleteEverything_HandWrittenAliasKeepsRewrite(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedDeleteFixture(t, home, "work")

	gcPath := filepath.Join(home, ".gitconfig")
	if _, err := gitconfig.WriteProviderRewrite(gcPath, "github.com", true); err != nil {
		t.Fatalf("seeding provider rewrite: %v", err)
	}

	sshPath := filepath.Join(home, ".ssh", "config")
	existing, err := os.ReadFile(sshPath) //nolint:gosec // test fixture path
	if err != nil {
		t.Fatalf("reading ssh config: %v", err)
	}
	handWritten := "\nHost foo.github.com\n  Hostname ssh.github.com\n  Port 443\n  IdentityFile ~/.ssh/id_ed25519_foo\n"
	writeFile(t, sshPath, string(existing)+handWritten)

	b := newBackendForHome(home)
	if _, err := b.runDelete("work", identity.DeleteScopeEverything, lifecyclePolicy{Confirm: confirmationBypassedWithYes}); err != nil {
		t.Fatalf("runDelete(work, everything) error: %v", err)
	}
	gc, err := os.ReadFile(gcPath) //nolint:gosec // test fixture path
	if err != nil {
		t.Fatalf("reading gitconfig: %v", err)
	}
	has, herr := gitconfig.HasProviderRewrite(gc, "github.com")
	if herr != nil {
		t.Fatalf("HasProviderRewrite: %v", herr)
	}
	if !has {
		t.Error("provider rewrite block removed despite a surviving hand-written Host stanza (R-05 regression)")
	}
}

// TestRunDeleteEverything_BitbucketHandWrittenAliasKeepsRewrite is the R2-04
// Bitbucket twin of the R-05 regression above: a hand-written Host stanza
// whose Hostname is altssh.bitbucket.org keeps the bitbucket.org rewrite
// block alive after the only managed Bitbucket identity is deleted.
func TestRunDeleteEverything_BitbucketHandWrittenAliasKeepsRewrite(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedSSHDir(t, home)

	sshBody := "Host bb.bitbucket.org\n" +
		"  Hostname altssh.bitbucket.org\n" +
		"  Port 443\n" +
		"  IdentityFile ~/.ssh/id_ed25519_bb\n" +
		"  IdentitiesOnly yes\n"
	sshConfig := managedBlock("_global", "Host *\n  IdentitiesOnly yes\n") + "\n" +
		managedBlock("bb", sshBody) +
		"\nHost foreign.bitbucket.org\n  Hostname altssh.bitbucket.org\n  Port 443\n  IdentityFile ~/.ssh/id_ed25519_foreign\n"
	writeFile(t, filepath.Join(home, ".ssh", "config"), sshConfig)

	pubLine := seedGeneratedKey(t, filepath.Join(home, ".ssh", "id_ed25519_bb"), "bb", "")

	if err := os.MkdirAll(filepath.Join(home, ".gitconfig.d"), 0o700); err != nil {
		t.Fatalf("seeding .gitconfig.d: %v", err)
	}
	fragBody := "[user]\n\tname = BB User\n\temail = bb@example.com\n" +
		"\tsigningkey = ~/.ssh/id_ed25519_bb.pub\n\n[gpg]\n\tformat = ssh\n\n[commit]\n\tgpgsign = true\n"
	writeFile(t, filepath.Join(home, ".gitconfig.d", "bb"), fragBody)

	gcBody := "[includeIf \"gitdir:~/git/bb/\"]\n\tpath = ~/.gitconfig.d/bb\n"
	gcPath := filepath.Join(home, ".gitconfig")
	writeFile(t, gcPath, "[user]\n\tname = Global User\n\n"+managedBlock("bb", gcBody))
	if _, err := gitconfig.WriteProviderRewrite(gcPath, "bitbucket.org", true); err != nil {
		t.Fatalf("seeding provider rewrite: %v", err)
	}

	line := mustAllowedSignersLine(t, "bb@example.com", pubLine)
	writeFile(t, filepath.Join(home, ".ssh", "allowed_signers"), managedBlock("bb", line))

	b := newBackendForHome(home)
	if _, err := b.runDelete("bb", identity.DeleteScopeEverything, lifecyclePolicy{Confirm: confirmationBypassedWithYes}); err != nil {
		t.Fatalf("runDelete(bb, everything) error: %v", err)
	}
	gc, err := os.ReadFile(gcPath) //nolint:gosec // test fixture path
	if err != nil {
		t.Fatalf("reading gitconfig: %v", err)
	}
	has, herr := gitconfig.HasProviderRewrite(gc, "bitbucket.org")
	if herr != nil {
		t.Fatalf("HasProviderRewrite: %v", herr)
	}
	if !has {
		t.Error("bitbucket.org provider rewrite block removed despite a surviving hand-written Host stanza (R2-04 regression)")
	}
}

// ---------------------------------------------------------------------------
// 05-05 — Clone: SuggestCloneName / ClonePrefill (D-14/D-15/D-16/D-17, R-29)
// ---------------------------------------------------------------------------

// TestClonePrefill_CopiesAuthorFieldsAndRederivesEverythingElse is the
// real-constructor happy path: DeriveCloneInput's D-14 copy/re-derive split
// survives the real Reconstruct round-trip, and ClonePrefill projects it
// into the exact DTO the wizard pre-fills from.
// seedCloneSourceFixture mirrors seedDeleteFixture's shape but renders the
// Host block through sshconfig.RenderHostBlock with an explicit provider,
// so the reconstructed account carries a FULL FQDN Provider ("github.com")
// via the "# gitid: provider=" marker (D-11) rather than seedDeleteFixture's
// markerless block, which Reconstruct's hostnameToProvider fallback would
// resolve to the SHORT form ("github") — correct for a legacy identity, but
// wrong for these tests' alias-shape assertions (DefaultAlias needs the
// FQDN to produce "<name>.github.com", not "<name>.github").
func seedCloneSourceFixture(t *testing.T, home, name string) (pubLine string) {
	t.Helper()
	seedSSHDir(t, home)

	hostBody := sshconfig.RenderHostBlock(name+".github.com", "ssh.github.com", 443, "~/.ssh/id_ed25519_"+name, "github.com")
	sshConfig := managedBlock("_global", "Host *\n  IdentitiesOnly yes\n") + "\n" + managedBlock(name, hostBody)
	writeFile(t, filepath.Join(home, ".ssh", "config"), sshConfig)

	pubLine = seedGeneratedKey(t, filepath.Join(home, ".ssh", "id_ed25519_"+name), name, "")

	if err := os.MkdirAll(filepath.Join(home, ".gitconfig.d"), 0o700); err != nil {
		t.Fatalf("seeding .gitconfig.d: %v", err)
	}
	fragBody := "[user]\n\tname = " + name + " User\n\temail = " + name + "@example.com\n" +
		"\tsigningkey = ~/.ssh/id_ed25519_" + name + ".pub\n\n[gpg]\n\tformat = ssh\n\n[commit]\n\tgpgsign = true\n"
	writeFile(t, filepath.Join(home, ".gitconfig.d", name), fragBody)

	gcBody := "[includeIf \"gitdir:~/git/" + name + "/\"]\n\tpath = ~/.gitconfig.d/" + name + "\n"
	gitconfigFile := "[user]\n\tname = Global User\n\n" + managedBlock(name, gcBody)
	writeFile(t, filepath.Join(home, ".gitconfig"), gitconfigFile)

	line := mustAllowedSignersLine(t, name+"@example.com", pubLine)
	writeFile(t, filepath.Join(home, ".ssh", "allowed_signers"), managedBlock(name, line))

	return pubLine
}

func TestClonePrefill_CopiesAuthorFieldsAndRederivesEverythingElse(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedDeleteFixture(t, home, "personal")
	b := newBackendForHome(home)

	pre, err := b.ClonePrefill("personal", "personal-clone", true)
	if err != nil {
		t.Fatalf("ClonePrefill: %v", err)
	}
	if pre.SourceName != "personal" || pre.CloneName != "personal-clone" {
		t.Errorf("SourceName/CloneName = %q/%q, want personal/personal-clone", pre.SourceName, pre.CloneName)
	}
	if pre.GitName != "personal User" || pre.GitEmail != "personal@example.com" {
		t.Errorf("GitName/GitEmail = %q/%q, want the source's copied author values", pre.GitName, pre.GitEmail)
	}
	if len(pre.CopiedFields) != 2 {
		t.Fatalf("CopiedFields = %v, want exactly the two author fields", pre.CopiedFields)
	}
	if pre.Hostname != "ssh.github.com" || pre.Port != "443" {
		t.Errorf("Hostname/Port = %q/%q, want the source's re-derived endpoint ssh.github.com/443", pre.Hostname, pre.Port)
	}
	wantKey := filepath.Join(home, ".ssh", "id_ed25519_personal")
	if pre.ReuseKeyPath != wantKey {
		t.Errorf("ReuseKeyPath = %q, want the source's resolved key %q", pre.ReuseKeyPath, wantKey)
	}
}

// TestSuggestCloneName_BumpsPastLiteralHostAlias is the D-17 availability
// half of the R-29 two-check proof: a hand-written LITERAL Host alias
// blocks the base suggestion, and the suggestion silently bumps past it —
// the prompt never opens already claimed.
func TestSuggestCloneName_BumpsPastLiteralHostAlias(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedDeleteFixture(t, home, "personal")
	sshPath := filepath.Join(home, ".ssh", "config")
	existing := readFile(t, sshPath)
	handWritten := "\nHost personal-clone.github.com\n  Hostname ssh.github.com\n  Port 443\n  IdentityFile ~/.ssh/id_ed25519_other\n"
	writeFile(t, sshPath, existing+handWritten)

	b := newBackendForHome(home)
	if got := b.SuggestCloneName("personal"); got != "personal-clone-2" {
		t.Errorf("SuggestCloneName(personal) = %q, want personal-clone-2 (bumped past the literal hand-written alias)", got)
	}
}

// TestClonePrefill_WildcardPatternShadowingReturnsTypedError is the D-17/
// R-29 pattern-shadowing half of the two-check proof: a WILDCARD Host
// stanza that no literal-name scan would ever catch must be caught here,
// as a typed error naming the shadowing pattern — never a silent bump.
func TestClonePrefill_WildcardPatternShadowingReturnsTypedError(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedCloneSourceFixture(t, home, "personal")
	sshPath := filepath.Join(home, ".ssh", "config")
	existing := readFile(t, sshPath)
	wildcard := "\nHost *.github.com\n  IdentitiesOnly yes\n"
	writeFile(t, sshPath, existing+wildcard)

	b := newBackendForHome(home)
	_, err := b.ClonePrefill("personal", "personal-clone", true)
	if err == nil {
		t.Fatal("ClonePrefill must refuse a clone whose derived alias is shadowed by a wildcard Host pattern")
	}
	var fe *identity.FieldError
	if !errors.As(err, &fe) {
		t.Fatalf("error = %v (%T), want *identity.FieldError", err, err)
	}
	if !strings.Contains(fe.Message, "*.github.com") {
		t.Errorf("error message = %q, must name the shadowing pattern *.github.com", fe.Message)
	}
}

// TestSuggestCloneNameAndShadowing_AreDistinctCodePaths proves the
// availability check (bump) and the pattern-shadowing check (typed error)
// are genuinely separate: the SAME hermetic home carries BOTH a literal
// alias (bumps the suggestion) and — once ClonePrefill is asked to derive
// the bumped name directly — no wildcard is present, so it succeeds; adding
// a wildcard afterward flips ClonePrefill's outcome to a refusal without
// touching SuggestCloneName's own bump behavior at all.
func TestSuggestCloneNameAndShadowing_AreDistinctCodePaths(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedCloneSourceFixture(t, home, "personal")
	sshPath := filepath.Join(home, ".ssh", "config")
	existing := readFile(t, sshPath)
	writeFile(t, sshPath, existing+"\nHost personal-clone.github.com\n  Hostname ssh.github.com\n  Port 443\n  IdentityFile ~/.ssh/id_ed25519_other\n")

	b := newBackendForHome(home)
	suggested := b.SuggestCloneName("personal")
	if suggested != "personal-clone-2" {
		t.Fatalf("SuggestCloneName = %q, want personal-clone-2 (availability bump)", suggested)
	}
	if _, err := b.ClonePrefill("personal", suggested, true); err != nil {
		t.Fatalf("ClonePrefill(%q) must succeed — no wildcard shadowing present yet: %v", suggested, err)
	}

	// Now add a wildcard that shadows the SAME bumped name's alias — the
	// availability path (SuggestCloneName) is untouched; only ClonePrefill's
	// shadowing check reacts.
	writeFile(t, sshPath, readFile(t, sshPath)+"\nHost *.github.com\n  IdentitiesOnly yes\n")
	b2 := newBackendForHome(home)
	if got := b2.SuggestCloneName("personal"); got != suggested {
		t.Errorf("adding a wildcard must not change SuggestCloneName's own bump result: got %q, want %q", got, suggested)
	}
	if _, err := b2.ClonePrefill("personal", suggested, true); err == nil {
		t.Error("ClonePrefill must now refuse the same name once a shadowing wildcard exists")
	}
}

// ---------------------------------------------------------------------------
// 05-05 — Clone: D-16 same-key clones re-run the full two-stage gate.
// ---------------------------------------------------------------------------

// TestSpecFingerprint_DistinguishesByIdentityNameAndAlias confirms
// specFingerprint already incorporates BOTH the identity name and the alias
// (not only the key path) — the fail-closed store gate is only as strong as
// what the fingerprint binds, and two clones of one source must never share
// a fingerprint.
func TestSpecFingerprint_DistinguishesByIdentityNameAndAlias(t *testing.T) {
	base := identity.CreateInput{
		Alias: "personal-clone.github.com", Hostname: "ssh.github.com", Port: 443,
		Algo: "ed25519", Name: "personal-clone", Provider: "github.com",
	}
	byName := base
	byName.Name = "personal-clone-2"
	if specFingerprint(base) == specFingerprint(byName) {
		t.Error("specFingerprint must differ when only Name differs")
	}
	byAlias := base
	byAlias.Alias = "personal-clone-2.github.com"
	if specFingerprint(base) == specFingerprint(byAlias) {
		t.Error("specFingerprint must differ when only Alias differs")
	}
}

// TestCloneStoreGate_LockedUntilBothStagesAccepted is the D-16 gate proof at
// the store-gate seam: a same-key clone's write stays locked until BOTH
// stages are accepted for its OWN fingerprint.
func TestCloneStoreGate_LockedUntilBothStagesAccepted(t *testing.T) {
	home := t.TempDir()
	b := newBackendForHome(home)
	spec := identity.CreateInput{
		Alias: "personal-clone.github.com", Hostname: "ssh.github.com", Port: 443,
		Algo: "ed25519", Name: "personal-clone", Provider: "github.com",
		ReuseKeyPath: filepath.Join(home, ".ssh", "id_ed25519_personal"),
	}
	if b.storeUnlockedFor(spec) {
		t.Fatal("store gate must be locked before either stage runs")
	}
	b.recordOutcomeFor(1, tuikit.TestOutcomePass, spec)
	if b.storeUnlockedFor(spec) {
		t.Fatal("store gate must stay locked with only stage 1 accepted")
	}
	b.recordOutcomeFor(2, tuikit.TestOutcomePass, spec)
	if !b.storeUnlockedFor(spec) {
		t.Fatal("store gate must unlock once both stages are accepted for THIS spec")
	}
}

// TestCloneStoreGate_SourceOutcomeDoesNotUnlockClone proves one identity's
// proof provably cannot unlock another's write: an accepted outcome
// recorded for the SOURCE specification must not unlock the CLONE
// specification, even though the clone reuses the source's key.
func TestCloneStoreGate_SourceOutcomeDoesNotUnlockClone(t *testing.T) {
	home := t.TempDir()
	b := newBackendForHome(home)
	keyPath := filepath.Join(home, ".ssh", "id_ed25519_personal")
	sourceSpec := identity.CreateInput{
		Alias: "personal.github.com", Hostname: "ssh.github.com", Port: 443,
		Algo: "ed25519", Name: "personal", Provider: "github.com",
	}
	cloneSpec := sourceSpec
	cloneSpec.Name = "personal-clone"
	cloneSpec.Alias = "personal-clone.github.com"
	cloneSpec.ReuseKeyPath = keyPath

	b.recordOutcomeFor(1, tuikit.TestOutcomePass, sourceSpec)
	b.recordOutcomeFor(2, tuikit.TestOutcomePass, sourceSpec)
	if !b.storeUnlockedFor(sourceSpec) {
		t.Fatal("sanity: the source spec should be unlocked by its own accepted outcomes")
	}
	if b.storeUnlockedFor(cloneSpec) {
		t.Error("an accepted outcome recorded for the SOURCE specification must not unlock the CLONE specification")
	}
}

// TestCloneStageCommandsNameCloneAlias_RealBackend is the real-Backend
// counterpart of the tuikit-level assertion: Stage2Command (TEST-02,
// resolve BY ALIAS) must name the clone's own alias, never the source's.
func TestCloneStageCommandsNameCloneAlias_RealBackend(t *testing.T) {
	home := t.TempDir()
	b := newBackendForHome(home)
	spec := tuikit.CreateSpec{
		Identity: "personal-clone", Alias: "personal-clone.github.com",
		Hostname: "ssh.github.com", Port: "443", KeyPath: filepath.Join(home, ".ssh", "id_ed25519_personal"),
	}
	cmd := b.Stage2Command(spec)
	if !strings.Contains(cmd, "personal-clone") {
		t.Errorf("Stage2Command = %q, must name the clone's own alias", cmd)
	}
}

// ---------------------------------------------------------------------------
// Plan 05-07 Task 2: real delete plan seams, exhaustive Persist, one writer
// ---------------------------------------------------------------------------

// TestDeletePlanScanReportsUnmanagedAndSiblingHits is the D-13/T-05-34
// admission test over the REAL plan seam: a hand-written stanza naming the
// alias is reported as an unmanaged hit with file+line, the same alias
// inside a SIBLING's managed block is reported as other-managed, and the
// identity's OWN managed block stays silent.
func TestDeletePlanScanReportsUnmanagedAndSiblingHits(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedDeleteFixture(t, home, "work")
	b := newBackendForHome(home)

	sshConfig := `# BEGIN gitid managed: _global
Host *
  IdentitiesOnly yes
# END gitid managed: _global

# BEGIN gitid managed: work
Host work.github.com
  Hostname ssh.github.com
  Port 443
  User git
  IdentityFile ~/.ssh/id_ed25519_work
  IdentitiesOnly yes
# END gitid managed: work

# BEGIN gitid managed: personal
Host personal.github.com
  Hostname ssh.github.com
  Port 443
  User git
  IdentityFile ~/.ssh/id_ed25519_personal
  PermitOpen work.github.com:443
# END gitid managed: personal

Host work.github.com
  Hostname ssh.github.com
  IdentityFile ~/.ssh/id_ed25519_work
`
	writeFile(t, filepath.Join(home, ".ssh", "config"), sshConfig)

	plan, err := b.DeletePlan("work", "everything")
	if err != nil {
		t.Fatalf("DeletePlan: %v", err)
	}
	sshFile := filepath.Join(home, ".ssh", "config")
	var gotOther, gotUnmanaged bool
	for _, h := range plan.Hits {
		if h.File != sshFile {
			t.Errorf("unexpected hit outside ~/.ssh/config: %q %+v", h.File, h)
			continue
		}
		switch h.Region {
		case "other-managed":
			if h.Line != 21 {
				t.Errorf("other-managed hit line = %d, want 21 (PermitOpen line)", h.Line)
			}
			gotOther = true
		case "unmanaged":
			if h.Line != 24 {
				t.Errorf("unmanaged hit line = %d, want 24 (hand-written Host line)", h.Line)
			}
			gotUnmanaged = true
		case "own-managed":
			t.Errorf("own-managed hit must be dropped: %+v", h)
		}
	}
	if !gotOther {
		t.Error("no other-managed hit — the alias inside the sibling's block must be reported")
	}
	if !gotUnmanaged {
		t.Error("no unmanaged hit — the alias in a hand-written stanza must be reported")
	}
}

// TestPlannerSeamsFailClosedOnMissingIdentity proves DeletePlan,
// KeyCeremonyPlan, and KeyActionFor each return a non-nil error plus a
// ZERO-VALUE view on a resolution failure — never a partial plan with a nil
// error (review R-07's fail-closed rule).
func TestPlannerSeamsFailClosedOnMissingIdentity(t *testing.T) {
	b := newBackendForHome(t.TempDir())

	v, err := b.DeletePlan("ghost", "everything")
	if err == nil {
		t.Error("DeletePlan on a missing identity must error")
	} else if !reflect.DeepEqual(v, tuikit.DeletePlanView{}) {
		t.Errorf("DeletePlan returned a non-zero view %+v alongside an error", v)
	}

	kv, err := b.KeyCeremonyPlan("ghost", "rotate")
	if err == nil {
		t.Error("KeyCeremonyPlan on a missing identity must error")
	} else if !reflect.DeepEqual(kv, tuikit.KeyCeremonyView{}) {
		t.Errorf("KeyCeremonyPlan returned a non-zero view %+v alongside an error", kv)
	}

	action, err := b.KeyActionFor("ghost")
	if err == nil {
		t.Error("KeyActionFor on a missing identity must error")
	} else if action != "" {
		t.Errorf("KeyActionFor returned %q alongside an error, want empty", action)
	}
}

// TestKeyActionForDetectsSharedKeyAcrossMixedPathSpelling is the CR-02
// regression for wiring.go's KeyActionFor: the owner-count comparison used
// to run acct.KeyPath and b.accounts() BOTH un-normalized. That happens to
// still work when every identity spells its IdentityFile the same way, but a
// gitconfig mixing a tilde-spelled IdentityFile with an absolute one for the
// SAME physical key file hid the sharing entirely, routing the destructive
// rotate path (CR-01) instead of repair. With the fix (normalizing both the
// account and the comparison list), KeyActionFor("work") must return
// "repair" regardless of how the sibling's IdentityFile is spelled.
func TestKeyActionForDetectsSharedKeyAcrossMixedPathSpelling(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedSharedKeyFixture(t, home, false) // false: "personal" spells the shared key as an ABSOLUTE path
	b := newBackendForHome(home)

	action, err := b.KeyActionFor("work")
	if err != nil {
		t.Fatalf("KeyActionFor(work): %v", err)
	}
	if action != string(identity.KeyActionRepair) {
		t.Errorf("KeyActionFor(work) = %q, want %q — a key shared across mixed tilde/absolute spellings must still route to repair", action, identity.KeyActionRepair)
	}
}

// TestDeletePlanFailsOnUnreadableScanSource proves the plan seam surfaces a
// READ failure as an error rather than as an eerily-small plan: an
// everything-scope plan whose allowed_signers file cannot be read aborts.
func TestDeletePlanFailsOnUnreadableScanSource(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedDeleteFixture(t, home, "work")
	b := newBackendForHome(home)

	allowed := filepath.Join(home, ".ssh", "allowed_signers")
	if err := os.Chmod(allowed, 0o000); err != nil {
		t.Fatalf("chmod 0000 allowed_signers: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(allowed, 0o600) })

	v, err := b.DeletePlan("work", "everything")
	if err == nil {
		t.Error("DeletePlan must error when a scan source is unreadable")
	} else if !reflect.DeepEqual(v, tuikit.DeletePlanView{}) {
		t.Errorf("DeletePlan returned a non-zero view %+v alongside an error", v)
	}
}

// TestForeignProviderCounterResolvesHostname is the D-09 hand-written-alias
// resolver test: a foreign Host stanza whose Hostname is the recipe-canonical
// alt-SSH endpoint counts against its provider key (never raw hostname
// equality against the alias).
func TestForeignProviderCounterResolvesHostname(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedSSHDir(t, home)
	writeFile(t, filepath.Join(home, ".ssh", "config"), `# BEGIN gitid managed: _global
Host *
  IdentitiesOnly yes
# END gitid managed: _global

Host foo.github.com
  Hostname ssh.github.com
  User git

Host bar.gitlab.com
  Hostname altssh.gitlab.com
  User git
`)
	b := newBackendForHome(home)

	if n, err := countForeignProviderRefs(b, "github.com"); err != nil || n != 1 {
		t.Errorf("countForeignProviderRefs(github.com) = %d, %v; want 1, nil — Hostname ssh.github.com must resolve to the github.com key", n, err)
	}
	if n, err := countForeignProviderRefs(b, "gitlab.com"); err != nil || n != 1 {
		t.Errorf("countForeignProviderRefs(gitlab.com) = %d, %v; want 1, nil", n, err)
	}
	if n, err := countForeignProviderRefs(b, "bitbucket.org"); err != nil || n != 0 {
		t.Errorf("countForeignProviderRefs(bitbucket.org) = %d, %v; want 0, nil", n, err)
	}
}

// TestNoWaveOneDeleteWriterName is the review R3-05 grep gate: the rejected
// wave-1 name for a second delete writer must appear NOWHERE in cmd/gitid,
// so the one-writer invariant cannot silently regress. Comment lines are
// stripped before the check, exactly as the plan's `grep -v '^\s*//'`
// gate does; the needle is built from parts so this very test does not trip
// its own gate.
func TestNoWaveOneDeleteWriterName(t *testing.T) {
	const needle = "deleteIdentity" + "Transaction"
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("globbing cmd/gitid: %v", err)
	}
	for _, file := range files {
		src, rerr := os.ReadFile(file) //nolint:gosec // test-only read under the repo (G304)
		if rerr != nil {
			t.Fatalf("reading %s: %v", file, rerr)
		}
		for lineNo, raw := range strings.Split(string(src), "\n") {
			code := strings.TrimSpace(strings.Split(raw, "//")[0])
			if strings.Contains(code, needle) {
				t.Errorf("%s:%d mentions the rejected wave-1 delete-writer name", file, lineNo+1)
			}
		}
	}
}

// TestPersistClassifiesEveryAction iterates tuikit.AllActions() and asserts
// no call lands in the unclassified branch: an errUnhandledAction persist
// error after a real Persist means a mutating action reached the dummy's
// reducer (05-RESEARCH.md Pitfall 1). Each action runs on a FRESH backend so
// a leftover persistErr from one action cannot mask a neighbor.
func TestPersistClassifiesEveryAction(t *testing.T) {
	seed := tuikit.DemoState{}
	for _, action := range tuikit.AllActions() {
		b := newBackendForHome(t.TempDir())
		_ = b.Persist(seed, action)
		if perr := b.PersistError(); perr != nil && errors.Is(perr, errUnhandledAction) {
			t.Errorf("Persist(%T) recorded errUnhandledAction %v — every action in the union must be classified explicitly", action, perr)
		}
	}
}

// TestPersistConfigureGitIsRealOwned pins review R-09-CG: ConfigureGit is a
// REAL action, so Persist returns a re-read of disk and records NO persist
// error — a working Git edit must never become a persist error.
func TestPersistConfigureGitIsRealOwned(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedDeleteFixture(t, home, "work")
	b := newBackendForHome(home)

	next := b.Persist(tuikit.DemoState{}, tuikit.ConfigureGit{
		Name: "work", GitName: "Work User", GitEmail: "work@example.com", MatchStrategy: "gitdir", GitDir: "~/git/work/",
	})
	if perr := b.PersistError(); perr != nil {
		t.Errorf("Persist(ConfigureGit) recorded a persist error where the plan requires re-read-only: %v", perr)
	}
	if len(next.Identities) == 0 {
		t.Error("Persist(ConfigureGit) must re-read disk — the seeded identity must appear in the returned state")
	}
}

// TestPersistDemoOnlyActionsPreserveTuikitReduce pins review R-09-DEMO: the
// Phase 6-8 banner actions keep producing the SAME state the dummy's reducer
// produces, so the D-16 banner screens' approved behavior is unchanged.
// ApplySSH is deliberately NOT in the list — plan 06-01 promotes it to
// REAL-OWNED (the write happens in CommitGlobalSSH; Persist re-reads disk),
// covered by TestPersistApplySSHIsReadOnly and the dedicated global-SSH
// wiring tests.
func TestPersistDemoOnlyActionsPreserveTuikitReduce(t *testing.T) {
	seed := tuikit.DemoState{Identities: []tuikit.DemoIdentity{{Name: "legacy", State: "complete"}}}
	demo := []tuikit.Action{
		tuikit.MarkScanned{},
		tuikit.FixFinding{ID: "git-includeif-missing-fragment"},
		tuikit.ApplyGitBaseline{},
		tuikit.ApplyGitGlobalEmail{Email: "dev@example.com"},
	}
	for _, action := range demo {
		b := newBackendForHome(t.TempDir())
		got := b.Persist(seed, action)
		want := tuikit.Reduce(seed, action)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("Persist(%T) = %+v, want Reduce's output %+v", action, got, want)
		}
	}
}

// TestPersistCloneIdentityIsClassifiedRefusal pins the CloneIdentity
// classification: the real backend never emits it (D-15), so Persist records
// a persist error naming it — and that error is NOT errUnhandledAction (it is
// a classified refusal, not an omission).
func TestPersistCloneIdentityIsClassifiedRefusal(t *testing.T) {
	b := newBackendForHome(t.TempDir())
	_ = b.Persist(tuikit.DemoState{}, tuikit.CloneIdentity{Source: "work", CloneName: "work-copy"})
	perr := b.PersistError()
	if perr == nil {
		t.Fatal("Persist(CloneIdentity) must record a persist error")
	}
	if !strings.Contains(perr.Error(), "CloneIdentity") {
		t.Errorf("persist error must name the action: %v", perr)
	}
	if errors.Is(perr, errUnhandledAction) {
		t.Errorf("CloneIdentity is a classified refusal, not an omission — error must not be errUnhandledAction: %v", perr)
	}
}

// TestCommitDeleteEverythingSurfacesRemoved drives the EVERYTHING-scope
// delete through the TUI CommitDelete seam and asserts the returned message
// names what was removed: the provider-rewrite block (sole-provider ref-count
// hits zero) and the D-11 key copy target — the "one delete writer, both
// skins" end of review R3-05.
func TestCommitDeleteEverythingSurfacesRemoved(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedDeleteFixture(t, home, "work")
	b := groupHermeticBackend(home)

	msgI := b.CommitDelete("work", "everything")()
	msg, ok := msgI.(tuikit.DeleteCommitMsg)
	if !ok {
		t.Fatalf("CommitDelete delivered %T, want DeleteCommitMsg", msgI)
	}
	if msg.Err != "" {
		t.Fatalf("CommitDelete(everything) error: %s", msg.Err)
	}
	if len(msg.Backups) == 0 {
		t.Error("an everything-scope delete must report timestamped backups")
	}
	var sawRewrite, sawArchive bool
	for _, removed := range msg.Removed {
		if strings.Contains(removed, "provider rewrite") {
			sawRewrite = true
		}
		if strings.Contains(removed, "gitid-archive") {
			sawArchive = true
		}
	}
	if !sawRewrite {
		t.Errorf("message.Removed must name the provider-rewrite removal (D-09), got %v", msg.Removed)
	}
	if !sawArchive {
		t.Errorf("message.Removed must name the D-11 key copy path, got %v", msg.Removed)
	}
}

// ---------------------------------------------------------------------------
// Global-SSH tracer (plan 06-01) — the ONE write authority end-to-end
// ---------------------------------------------------------------------------

// TestCommitGlobalSSHEndToEnd proves the tracer slice through the REAL
// constructor against an isolated temp home: running the command returned by
// CommitGlobalSSH writes the gitid `Host *` managed block (with the
// recommended directive) into the resolved include'd target, floors the
// Include line on the fresh machine, and a SECOND identical apply leaves the
// file bytes unchanged while taking its own timestamped backup.
func TestCommitGlobalSSHEndToEnd(t *testing.T) {
	home := t.TempDir()
	b := newBackendForHome(home)

	msg, ok := b.CommitGlobalSSH([]string{"HashKnownHosts"})().(tuikit.GlobalSSHCommitMsg)
	if !ok {
		t.Fatalf("CommitGlobalSSH delivered %T, want GlobalSSHCommitMsg", msg)
	}
	if msg.Err != "" {
		t.Fatalf("first apply error: %s", msg.Err)
	}

	targetPath := filepath.Join(home, ".ssh", "config.d", "gitid.config")
	first := readFile(t, targetPath)
	if !strings.Contains(first, "HashKnownHosts yes") {
		t.Fatalf("the managed block must carry the recommended directive after apply:\n%s", first)
	}
	var sawBlock bool
	for _, blk := range filewriter.ListBlocks([]byte(first)) {
		if blk.Name == sshconfig.GlobalBlockName {
			sawBlock = true
			break
		}
	}
	if !sawBlock {
		t.Fatalf("no %s managed block in the resolved target after apply:\n%s", sshconfig.GlobalBlockName, first)
	}
	main := readFile(t, filepath.Join(home, ".ssh", "config"))
	if !strings.Contains(main, "Include ~/.ssh/config.d/*.config") {
		t.Errorf("the Include line must be floored on a fresh machine:\n%s", main)
	}

	// Second identical apply: byte-identical content + an additional backup.
	beforeBackups := countBackupSiblings(t, targetPath)
	msg2, ok := b.CommitGlobalSSH([]string{"HashKnownHosts"})().(tuikit.GlobalSSHCommitMsg)
	if !ok || msg2.Err != "" {
		t.Fatalf("second apply: ok=%v err=%q", ok, msg2.Err)
	}
	second := readFile(t, targetPath)
	if second != first {
		t.Errorf("second identical apply must leave the file byte-identical\nfirst:\n%s\nsecond:\n%s", first, second)
	}
	if countBackupSiblings(t, targetPath) != beforeBackups+1 {
		t.Errorf("the second apply must take its own timestamped backup (before=%d after=%d)", beforeBackups, countBackupSiblings(t, targetPath))
	}
}

// countBackupSiblings counts the timestamped `.bak.*` siblings filewriter
// mints for targetPath.
func countBackupSiblings(t *testing.T, targetPath string) int {
	t.Helper()
	matches, err := filepath.Glob(targetPath + ".bak.*")
	if err != nil {
		t.Fatalf("globbing backup siblings: %v", err)
	}
	return len(matches)
}

// TestPersistApplySSHIsReadOnly proves Persist(ApplySSH) performs NO write of
// its own: calling it WITHOUT a preceding commit must leave the resolved
// target (and ~/.ssh/config) untouched — there is exactly ONE write authority,
// and it is the async commit seam, not Persist (T-06-30 pin).
func TestPersistApplySSHIsReadOnly(t *testing.T) {
	home := t.TempDir()
	b := newBackendForHome(home)

	_ = b.Persist(b.InitialState(), tuikit.ApplySSH{Keys: []string{"HashKnownHosts"}, Backup: "b"})

	if fileExists(b.storage().targetPath) {
		t.Errorf("Persist(ApplySSH) wrote %s — Persist must be re-read-only (the write belongs to CommitGlobalSSH)", b.storage().targetPath)
	}
	if fileExists(filepath.Join(home, ".ssh", "config")) {
		t.Error("Persist(ApplySSH) wrote ~/.ssh/config — Persist must be re-read-only")
	}
}

// TestApplyThenCreatePreservesGlobalFix is D-06's central guarantee on the
// apply→create order: a global fix applied through the real commit path must
// survive a subsequent REAL create (the create ceremony now normalises the
// globals block through the same EnsureGlobals owner instead of
// whole-block-replacing it).
func TestApplyThenCreatePreservesGlobalFix(t *testing.T) {
	home := t.TempDir()
	b := newBackendForHome(home)

	msg, ok := b.CommitGlobalSSH([]string{"HashKnownHosts"})().(tuikit.GlobalSSHCommitMsg)
	if !ok || msg.Err != "" {
		t.Fatalf("global fix apply: ok=%v err=%q", ok, msg.Err)
	}

	id := tuikit.DemoIdentity{
		Name: "work", SSHHost: "work.github.com", Provider: "github.com",
		Hostname: "ssh.github.com", Port: 443, KeyPath: filepath.Join(home, ".ssh", "id_ed25519_work"),
		GitConfigured: false,
	}
	unlockStoreForIdentity(t, b, id)
	if !b.storeUnlockedFor(b.createInput(id)) {
		t.Fatal("store gate must be unlocked for the real create")
	}
	createMsg := runCommitCreate(t, b, id)
	if createMsg.Err != "" {
		t.Fatalf("create error: %s", createMsg.Err)
	}

	target := readFile(t, filepath.Join(home, ".ssh", "config.d", "gitid.config"))
	for _, blk := range filewriter.ListBlocks([]byte(target)) {
		if blk.Name == sshconfig.GlobalBlockName {
			if !strings.Contains(blk.Body, "HashKnownHosts yes") {
				t.Fatalf("the fixed directive was erased by the create's globals normalisation:\n%s", target)
			}
			return
		}
	}
	t.Fatalf("no %s managed block after the create:\n%s", sshconfig.GlobalBlockName, target)
}

// ---------------------------------------------------------------------------
// Task 2 — placement (D-07, D-09) and survival across every other flow (D-06)
// ---------------------------------------------------------------------------

// TestGlobalsPlacementIncludeLayout writes the globals block into the
// gitid-owned included file, not the main config (D-07).
func TestGlobalsPlacementIncludeLayout(t *testing.T) {
	home := t.TempDir()
	b := newBackendForHome(home)

	msg, ok := b.CommitGlobalSSH([]string{"HashKnownHosts"})().(tuikit.GlobalSSHCommitMsg)
	if !ok || msg.Err != "" {
		t.Fatalf("apply: ok=%v err=%q", ok, msg.Err)
	}

	included := readFile(t, filepath.Join(home, ".ssh", "config.d", "gitid.config"))
	if !strings.Contains(included, filewriter.BeginPrefix+sshconfig.GlobalBlockName) {
		t.Fatalf("included file must carry the globals block:\n%s", included)
	}
	main := readFile(t, filepath.Join(home, ".ssh", "config"))
	if strings.Contains(main, filewriter.BeginPrefix+sshconfig.GlobalBlockName) {
		t.Fatalf("main config must NOT carry the globals block under the Include'd layout:\n%s", main)
	}
}

// TestGlobalsPlacementInFileLayout writes the globals block into the main
// config when the machine already uses in-file identity blocks (D-07).
func TestGlobalsPlacementInFileLayout(t *testing.T) {
	home := t.TempDir()
	seedInFileIdentity(t, home, "personal")
	b := newBackendForHome(home)
	if b.storage().includeLayout {
		t.Fatal("fixture must pin the in-file layout")
	}

	msg, ok := b.CommitGlobalSSH([]string{"HashKnownHosts"})().(tuikit.GlobalSSHCommitMsg)
	if !ok || msg.Err != "" {
		t.Fatalf("apply: ok=%v err=%q", ok, msg.Err)
	}

	main := readFile(t, filepath.Join(home, ".ssh", "config"))
	if !strings.Contains(main, filewriter.BeginPrefix+sshconfig.GlobalBlockName) {
		t.Fatalf("in-file layout must write the globals block into ~/.ssh/config:\n%s", main)
	}
	included := filepath.Join(home, ".ssh", "config.d", "gitid.config")
	if fileExists(included) {
		t.Errorf("in-file apply must not create the Include'd file; got:\n%s", readFile(t, included))
	}
}

// TestGlobalsPlacementFreshHomeFloorsIncludeLine pins the empty-machine
// contract: a global fix creates the include directory, floors the Include
// line, and that line's byte offset is smaller than any gitid block in the
// main file (D-07 + Phase 3's floor model).
func TestGlobalsPlacementFreshHomeFloorsIncludeLine(t *testing.T) {
	home := t.TempDir()
	b := newBackendForHome(home)

	msg, ok := b.CommitGlobalSSH([]string{"HashKnownHosts"})().(tuikit.GlobalSSHCommitMsg)
	if !ok || msg.Err != "" {
		t.Fatalf("apply: ok=%v err=%q", ok, msg.Err)
	}

	includeDir := filepath.Join(home, ".ssh", "config.d")
	info, err := os.Stat(includeDir)
	if err != nil {
		t.Fatalf("include directory missing after a fresh-HOME apply: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("%s exists but is not a directory", includeDir)
	}

	main := readFile(t, filepath.Join(home, ".ssh", "config"))
	includeOff := strings.Index(main, "Include ~/.ssh/config.d/*.config")
	if includeOff < 0 {
		t.Fatalf("Include line missing from ~/.ssh/config:\n%s", main)
	}
	// The Include line lives inside the ssh-include wiring block (floored at
	// the top). Compare against every OTHER gitid-managed block in the main
	// file — identity / globals must not precede the Include line (D-07).
	offset := 0
	for _, line := range strings.SplitAfter(main, "\n") {
		trimmed := strings.TrimRight(line, "\n\r")
		if strings.HasPrefix(trimmed, filewriter.BeginPrefix) {
			name := strings.TrimPrefix(trimmed, filewriter.BeginPrefix)
			if name != "ssh-include" && includeOff >= offset {
				t.Errorf("Include line offset %d is not smaller than gitid block %q at %d:\n%s", includeOff, name, offset, main)
			}
		}
		offset += len(line)
	}
}

// TestGlobalsFixThenCreateSurvivesCreate is D-06's ordered guarantee under
// BOTH layouts: apply a global fix, then create an identity through the real
// commit path; the fixed directive stays in the globals block AND the
// identity's begin-sentinel precedes the globals begin-sentinel (D-09).
func TestGlobalsFixThenCreateSurvivesCreate(t *testing.T) {
	for _, layout := range []string{"include", "in-file"} {
		t.Run(layout, func(t *testing.T) {
			home := t.TempDir()
			if layout == "in-file" {
				seedInFileIdentity(t, home, "personal")
			}
			b := newBackendForHome(home)

			msg, ok := b.CommitGlobalSSH([]string{"HashKnownHosts"})().(tuikit.GlobalSSHCommitMsg)
			if !ok || msg.Err != "" {
				t.Fatalf("apply: ok=%v err=%q", ok, msg.Err)
			}

			id := tuikit.DemoIdentity{
				Name: "work", SSHHost: "work.github.com", Provider: "github.com",
				Hostname: "ssh.github.com", Port: 443, KeyPath: filepath.Join(home, ".ssh", "id_ed25519_work"),
				GitConfigured: false,
			}
			unlockStoreForIdentity(t, b, id)
			if createMsg := runCommitCreate(t, b, id); createMsg.Err != "" {
				t.Fatalf("create error: %s", createMsg.Err)
			}

			target := readFile(t, b.globalsTargetPath())
			assertFixedDirectivePresent(t, target)
			identityOff := strings.Index(target, filewriter.BeginPrefix+"work\n")
			globalsOff := strings.Index(target, filewriter.BeginPrefix+sshconfig.GlobalBlockName+"\n")
			if identityOff < 0 || globalsOff < 0 {
				t.Fatalf("missing sentinels (identity=%d globals=%d) in:\n%s", identityOff, globalsOff, target)
			}
			if identityOff >= globalsOff {
				t.Errorf("identity begin-sentinel at %d does not precede globals at %d:\n%s", identityOff, globalsOff, target)
			}
		})
	}
}

// TestGlobalsCreateThenFixLeavesIdentityUntouched is the reverse order:
// create first, then apply a global fix; the identity block's body bytes
// stay identical and the fix is present.
func TestGlobalsCreateThenFixLeavesIdentityUntouched(t *testing.T) {
	for _, layout := range []string{"include", "in-file"} {
		t.Run(layout, func(t *testing.T) {
			home := t.TempDir()
			name := "work"
			if layout == "in-file" {
				seedInFileIdentity(t, home, "personal")
			}
			b := newBackendForHome(home)

			id := tuikit.DemoIdentity{
				Name: name, SSHHost: name + ".github.com", Provider: "github.com",
				Hostname: "ssh.github.com", Port: 443, KeyPath: filepath.Join(home, ".ssh", "id_ed25519_"+name),
				GitConfigured: false,
			}
			unlockStoreForIdentity(t, b, id)
			if createMsg := runCommitCreate(t, b, id); createMsg.Err != "" {
				t.Fatalf("create error: %s", createMsg.Err)
			}

			before := identityBlockBody(t, readFile(t, b.globalsTargetPath()), name)

			msg, ok := b.CommitGlobalSSH([]string{"HashKnownHosts"})().(tuikit.GlobalSSHCommitMsg)
			if !ok || msg.Err != "" {
				t.Fatalf("apply: ok=%v err=%q", ok, msg.Err)
			}

			afterContent := readFile(t, b.globalsTargetPath())
			after := identityBlockBody(t, afterContent, name)
			if after != before {
				t.Errorf("identity block body changed by the global fix;\nbefore:\n%s\nafter:\n%s", before, after)
			}
			assertFixedDirectivePresent(t, afterContent)
		})
	}
}

// TestGlobalsFixThenRotateLeavesGlobalsByteIdentical pins the empty-platform
// contract on rotate: a global fix, then a rotation, leaves the globals
// block's bytes identical.
func TestGlobalsFixThenRotateLeavesGlobalsByteIdentical(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedDeleteFixture(t, home, "work")
	b := groupHermeticBackend(home)

	msg, ok := b.CommitGlobalSSH([]string{"HashKnownHosts"})().(tuikit.GlobalSSHCommitMsg)
	if !ok || msg.Err != "" {
		t.Fatalf("apply: ok=%v err=%q", ok, msg.Err)
	}
	before := globalsBlockBytes(t, readFile(t, b.globalsTargetPath()))

	if _, err := b.runRotate("work", lifecyclePolicy{Confirm: confirmationAlreadyObtained}); err != nil {
		t.Fatalf("runRotate: %v", err)
	}

	after := globalsBlockBytes(t, readFile(t, b.globalsTargetPath()))
	if after != before {
		t.Errorf("rotate mutated the globals block (empty-platform contract);\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

// TestGlobalsFixThenRepairLeavesGlobalsByteIdentical pins the empty-platform
// contract on repair: a global fix, then a new-key ceremony, leaves the
// globals block's bytes identical.
func TestGlobalsFixThenRepairLeavesGlobalsByteIdentical(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedDeleteFixture(t, home, "work")
	b := groupHermeticBackend(home)

	msg, ok := b.CommitGlobalSSH([]string{"HashKnownHosts"})().(tuikit.GlobalSSHCommitMsg)
	if !ok || msg.Err != "" {
		t.Fatalf("apply: ok=%v err=%q", ok, msg.Err)
	}
	before := globalsBlockBytes(t, readFile(t, b.globalsTargetPath()))

	if _, err := b.runRepair("work", lifecyclePolicy{Confirm: confirmationAlreadyObtained}); err != nil {
		t.Fatalf("runRepair: %v", err)
	}

	after := globalsBlockBytes(t, readFile(t, b.globalsTargetPath()))
	if after != before {
		t.Errorf("repair mutated the globals block (empty-platform contract);\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

// TestGlobalSSHApplyPlanNamesResolvedTarget proves the ceremony heading's
// source of truth: GlobalSSHApplyPlan.Targets names the SAME file the write
// actually touches, under both layouts.
func TestGlobalSSHApplyPlanNamesResolvedTarget(t *testing.T) {
	for _, layout := range []string{"include", "in-file"} {
		t.Run(layout, func(t *testing.T) {
			home := t.TempDir()
			if layout == "in-file" {
				seedInFileIdentity(t, home, "personal")
			}
			b := newBackendForHome(home)
			plan, err := b.GlobalSSHApplyPlan([]string{"HashKnownHosts"})
			if err != nil {
				t.Fatalf("GlobalSSHApplyPlan: %v", err)
			}
			if len(plan.Targets) == 0 {
				t.Fatal("plan.Targets is empty")
			}
			want := b.displayPath(b.globalsTargetPath())
			if plan.Targets[0] != want {
				t.Errorf("plan.Targets[0] = %q, want the resolved storage target %q", plan.Targets[0], want)
			}
		})
	}
}

// seedInFileIdentity writes a single identity Host block into ~/.ssh/config
// so storage() pins the in-file layout (D-05).
func seedInFileIdentity(t *testing.T, home, name string) {
	t.Helper()
	seedSSHDir(t, home)
	body := sshconfig.RenderHostBlock(name+".github.com", "ssh.github.com", 443, "~/.ssh/id_ed25519_"+name, "github.com")
	writeFile(t, filepath.Join(home, ".ssh", "config"), managedBlock(name, body))
}

func assertFixedDirectivePresent(t *testing.T, content string) {
	t.Helper()
	for _, blk := range filewriter.ListBlocks([]byte(content)) {
		if blk.Name == sshconfig.GlobalBlockName {
			if !strings.Contains(blk.Body, "HashKnownHosts yes") {
				t.Fatalf("fixed HashKnownHosts yes missing from globals block:\n%s", content)
			}
			return
		}
	}
	t.Fatalf("no %s managed block in:\n%s", sshconfig.GlobalBlockName, content)
}

func identityBlockBody(t *testing.T, content, name string) string {
	t.Helper()
	for _, blk := range filewriter.ListBlocks([]byte(content)) {
		if blk.Name == name {
			return blk.Body
		}
	}
	t.Fatalf("no identity block %q in:\n%s", name, content)
	return ""
}

func TestGlobalSSHFixturePolicyParity(t *testing.T) {
	if len(tuikit.GlobalSSHOptions) != len(globalssh.Policy) {
		t.Fatalf("fixture has %d rows, policy has %d", len(tuikit.GlobalSSHOptions), len(globalssh.Policy))
	}
	for i := range globalssh.Policy {
		fix := tuikit.GlobalSSHOptions[i]
		pol := globalssh.Policy[i]
		if fix.Key != pol.Key || fix.Recommended != pol.Recommended || fix.Risk != pol.Risk {
			t.Errorf("row %d fixture=(%s %s %s) policy=(%s %s %s)", i, fix.Key, fix.Recommended, fix.Risk, pol.Key, pol.Recommended, pol.Risk)
		}
	}
	if int(tuikit.GlobalSSHReasonNone) != int(globalssh.ReasonNone) ||
		int(tuikit.GlobalSSHReasonPlatform) != int(globalssh.ReasonPlatform) ||
		int(tuikit.GlobalSSHReasonVersionTooOld) != int(globalssh.ReasonVersionTooOld) ||
		int(tuikit.GlobalSSHReasonVersionUnverified) != int(globalssh.ReasonVersionUnverified) ||
		int(tuikit.GlobalSSHReasonNothingToVerify) != int(globalssh.ReasonNothingToVerify) ||
		int(tuikit.GlobalSSHReasonProbeFailed) != int(globalssh.ReasonProbeFailed) {
		t.Fatal("NotApplicableReason enums drifted between tuikit and globalssh")
	}
	p, _ := globalssh.PolicyFor("StrictHostKeyChecking")
	if tuikit.GlobalSSHOptions[0].Recommended != p.Recommended || !strings.Contains(tuikit.GlobalSSHOptions[0].OneLiner, p.Recommended) {
		t.Fatalf("StrictHostKeyChecking fixture Recommended/OneLiner must name %q", p.Recommended)
	}
	fp, _ := globalssh.PolicyFor("ForwardAgent")
	if tuikit.GlobalSSHOptions[1].Risk != fp.Risk {
		t.Fatalf("ForwardAgent fixture Risk = %q, want %q", tuikit.GlobalSSHOptions[1].Risk, fp.Risk)
	}
}

// TestGlobalGitFixturePolicyParity walks internal/globalgit.Policy and
// internal/tuikit.GlobalGitOptions together and asserts they agree on row
// identity, order and recommended value — the mechanism that stops the demo
// and the real binary drifting apart again (07-03-PLAN.md Task 3), mirroring
// TestGlobalSSHFixturePolicyParity's own shape on the SSH side.
func TestGlobalGitFixturePolicyParity(t *testing.T) {
	if len(tuikit.GlobalGitOptions) != len(globalgit.Policy) {
		t.Fatalf("fixture has %d rows, policy has %d", len(tuikit.GlobalGitOptions), len(globalgit.Policy))
	}
	for i := range globalgit.Policy {
		fix := tuikit.GlobalGitOptions[i]
		pol := globalgit.Policy[i]
		if fix.Key != pol.Key {
			t.Errorf("row %d identity: fixture=%q policy=%q", i, fix.Key, pol.Key)
		}
		if fix.Recommended != pol.Recommended {
			t.Errorf("row %d (%s) recommended: fixture=%q policy=%q", i, fix.Key, fix.Recommended, pol.Recommended)
		}
	}
}

func TestGlobalSSHOptionStatesVersionUnverified(t *testing.T) {
	home := t.TempDir()
	b := newBackendForHome(home)
	b.probeSSHVersion = func() (platform.SSHVersion, error) { return platform.SSHVersion{}, nil }
	views, err := b.GlobalSSHOptionStates()
	if err != nil {
		t.Fatalf("GlobalSSHOptionStates: %v", err)
	}
	var found *tuikit.GlobalSSHOptionView
	for i := range views {
		if views[i].Key == "StrictHostKeyChecking" {
			found = &views[i]
			break
		}
	}
	if found == nil {
		t.Fatal("StrictHostKeyChecking row missing")
	}
	if found.State != tuikit.GlobalSSHNotApplicable || found.NotApplicableReason != tuikit.GlobalSSHReasonVersionUnverified {
		t.Fatalf("view state = (%v, %v), want not-applicable/unverified", found.State, found.NotApplicableReason)
	}
	if found.Explanation == "" {
		t.Fatal("explanation must remain non-empty when version is unverified")
	}
	_, applyErr := b.runGlobalSSHApply([]string{"StrictHostKeyChecking"}, lifecyclePolicy{Confirm: confirmationAlreadyObtained})
	if applyErr == nil || !strings.Contains(applyErr.Error(), "ssh -V") {
		t.Fatalf("runGlobalSSHApply err = %v, want a refusal naming ssh -V", applyErr)
	}
}

func TestGlobalSSHOptionStatesVersionTooOld(t *testing.T) {
	home := t.TempDir()
	b := newBackendForHome(home)
	b.probeSSHVersion = func() (platform.SSHVersion, error) {
		return platform.SSHVersion{OpenSSHVersion: "7.5"}, nil
	}
	views, err := b.GlobalSSHOptionStates()
	if err != nil {
		t.Fatalf("GlobalSSHOptionStates: %v", err)
	}
	var found tuikit.GlobalSSHOptionView
	for _, v := range views {
		if v.Key == "StrictHostKeyChecking" {
			found = v
		}
	}
	if found.NotApplicableReason != tuikit.GlobalSSHReasonVersionTooOld {
		t.Fatalf("reason = %v, want ReasonVersionTooOld", found.NotApplicableReason)
	}
	if strings.Contains(found.VersionNote, "macOS-only") {
		t.Fatalf("version-gated copy leaked the platform sentence: %q", found.VersionNote)
	}
}

func globalsBlockBytes(t *testing.T, content string) string {
	t.Helper()
	begin := filewriter.BeginPrefix + sshconfig.GlobalBlockName + "\n"
	end := filewriter.EndPrefix + sshconfig.GlobalBlockName + "\n"
	start := strings.Index(content, begin)
	stop := strings.Index(content, end)
	if start < 0 || stop < 0 || stop < start {
		t.Fatalf("globals block sentinels missing in:\n%s", content)
	}
	return content[start : stop+len(end)]
}

func TestGlobalGitOptionStatesWrapsProbeFailure(t *testing.T) {
	b := &realBackend{initErr: errors.New("bare probe failure")}
	_, err := b.GlobalGitOptionStates()
	if err == nil {
		t.Fatal("GlobalGitOptionStates must return its construction failure")
	}
	if got, want := err.Error(), "git probe failed: bare probe failure"; got != want {
		t.Errorf("wrapped error = %q, want %q", got, want)
	}
}
