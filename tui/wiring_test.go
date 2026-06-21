package tui

import (
	"testing"

	"github.com/castocolina/gitid/internal/adopter"
	"github.com/castocolina/gitid/internal/doctor"
	"github.com/castocolina/gitid/internal/gitconfig"
	"github.com/castocolina/gitid/internal/identity"
	"github.com/castocolina/gitid/internal/repoclone"
	"github.com/castocolina/gitid/internal/tester"
	"github.com/castocolina/gitid/internal/uploader"
)

// These tests exercise the LIVE program wiring rather than model internals in
// isolation. The Phase 5 CR-01..CR-04 tests from the old screen-stack
// architecture are ported/replaced below as Phase 5.6 equivalents.
//
// Phase 5.6 architectural change (D-15): the screen-stack (push/pop) is
// replaced by a persistent two-pane layout with a single activeView enum and a
// single activeModal field. The old push-based tests (TestPushInvokesProveInit,
// TestDepsThreadedEndToEnd, TestRunWriteCmdDispatchesUpdate,
// TestRunWriteCmdDispatchesAddAccount, TestProveKeyPathIsPrivateKeyNotSSHConfig,
// TestDetailPubLineCachedFromPub) are retired; their behavioral coverage is
// carried forward by the per-slice tests in Plans 02-06.

// --- Helper factories (used by Plans 02-06 test files and the guards below) ---

// fakeDocDeps returns a doctor.Deps that returns no findings for all families.
func fakeDocDeps() doctor.Deps {
	noFindings := func(_ doctor.Deps) []doctor.Finding { return nil }
	return doctor.Deps{
		CheckDeps:      noFindings,
		CheckPerms:     noFindings,
		CheckCoherence: noFindings,
		CheckOrphans:   noFindings,
		CheckSigning:   noFindings,
		CheckAgent:     noFindings,
		CheckBaseline:  noFindings,
		CheckOverlap:   noFindings,
	}
}

// fakeIdentityDeps returns an identity.Deps with no-op stubs.
func fakeIdentityDeps() identity.Deps {
	return identity.Deps{}
}

// fakeWriteTUIDeps returns a tuiDeps with no-op identity write stubs.
func fakeWriteTUIDeps(_ *bool) tuiDeps {
	return tuiDeps{
		identity: identity.Deps{
			Generate: func(_ identity.CreateInput) (identity.StagedKey, error) {
				return identity.StagedKey{}, nil
			},
			PersistKey: func(_ identity.StagedKey) (identity.KeyResult, error) {
				return identity.KeyResult{}, nil
			},
			Cleanup:             func(_ identity.StagedKey) {},
			CopyPub:             func(_ string) error { return nil },
			PreWrite:            func(_, _ string, _ int) tester.Result { return tester.Result{Outcome: tester.PASS} },
			WriteSSH:            func(_, _, _ string) (string, error) { return "", nil },
			WriteGitconfig:      func(_, _, _ string, _ []gitconfig.Match) (string, error) { return "", nil },
			WriteFragment:       func(_, _, _, _ string, _ bool) error { return nil },
			WriteAllowedSigners: func(_, _, _ string) (string, error) { return "", nil },
			Resolved:            func(_ string) (tester.Result, tester.ResolvedConfig) { return tester.Result{}, tester.ResolvedConfig{} },
			PubExists:           func(_ string) bool { return true },
			DerivePub:           func(_, _ string) (string, error) { return "", nil },
			WritePub:            func(_, _ string) error { return nil },
		},
	}
}

// makeTestInput returns a minimal CreateInput for test use.
func makeTestInput() identity.CreateInput {
	return identity.CreateInput{
		Name:     "personal",
		Provider: "github.com",
		Hostname: "github.com",
		Port:     22,
		Alias:    "personal.github.com",
	}
}

// fakeTUIDocDeps wraps fakeDocDeps in a tuiDeps for sub-models that take the
// full tuiDeps.
func fakeTUIDocDeps() tuiDeps {
	return tuiDeps{doctor: fakeDocDeps()}
}

// fakeDeleteDeps returns an identity.DeleteDeps with no-op stubs for tests that
// need a valid (non-nil-field) DeleteDeps without real file I/O.
func fakeDeleteDeps() identity.DeleteDeps {
	return identity.DeleteDeps{
		ReadSSH:              func() ([]byte, error) { return []byte{}, nil },
		ReadGitconfig:        func() ([]byte, error) { return []byte{}, nil },
		WriteSSH:             func(_ []byte) (string, error) { return "", nil },
		WriteGitconfig:       func(_ []byte) (string, error) { return "", nil },
		RemoveFragment:       func(_ string) (string, error) { return "", nil },
		RemoveAllowedSigners: func(_, _ string) (string, error) { return "", nil },
		RemoveKeyFiles:       func(_, _ string) (string, string, error) { return "", "", nil },
	}
}

// TestHelpersCompileAndBuildClean is a compile-only guard that ensures the
// shared test helpers used by Plans 02-06 compile and are exercised, so the
// strict unused linter does not fire on helper-only definitions.
// Each helper is invoked exactly once; the results are discarded.
func TestHelpersCompileAndBuildClean(t *testing.T) {
	_ = fakeDocDeps()
	_ = fakeIdentityDeps()
	_ = fakeWriteTUIDeps(nil)
	_ = makeTestInput()
	_ = fakeTUIDocDeps()
	_ = fakeDeleteDeps()
	_ = newRootModel(fakeDocDeps(), fakeIdentityDeps(), identity.UpdateDeps{}, identity.DeleteDeps{})
	t.Log("shared test helpers compile and build clean")
}

// --- D-16 anti-blindspot guard ---

// TestBuildTUIDepsNilGuard asserts that the live buildTUIDeps() wiring has
// non-nil function fields in all critical positions, closing the recurring
// D-16 blindspot (injected-seam tests were green while the live wiring was nil).
//
// This test deliberately drives buildTUIDeps() — not the fake helpers — so any
// nil seam in the real wiring is caught before it reaches the running TUI.
//
// Coverage (D-16 mandate):
//   - All 8 doctor.Deps CheckFn fields
//   - FixPerm / RemoveBlock / AddWiring
//   - identity.Deps.Generate
//   - identity.UpdateDeps.WriteSSH
//   - All 7 identity.DeleteDeps fields
func TestBuildTUIDepsNilGuard(t *testing.T) {
	docDeps, idDeps, upDeps, deleteDeps, err := buildTUIDeps()
	if err != nil {
		t.Fatalf("buildTUIDeps returned error: %v", err)
	}

	// All 8 doctor CheckFn fields must be non-nil.
	if docDeps.CheckDeps == nil {
		t.Error("doctor.Deps.CheckDeps nil")
	}
	if docDeps.CheckPerms == nil {
		t.Error("doctor.Deps.CheckPerms nil")
	}
	if docDeps.CheckCoherence == nil {
		t.Error("doctor.Deps.CheckCoherence nil")
	}
	if docDeps.CheckOrphans == nil {
		t.Error("doctor.Deps.CheckOrphans nil")
	}
	if docDeps.CheckSigning == nil {
		t.Error("doctor.Deps.CheckSigning nil")
	}
	if docDeps.CheckAgent == nil {
		t.Error("doctor.Deps.CheckAgent nil")
	}
	if docDeps.CheckBaseline == nil {
		t.Error("doctor.Deps.CheckBaseline nil")
	}
	if docDeps.CheckOverlap == nil {
		t.Error("doctor.Deps.CheckOverlap nil")
	}

	// Fix functions must be non-nil.
	if docDeps.FixPerm == nil {
		t.Error("doctor.Deps.FixPerm nil")
	}
	if docDeps.RemoveBlock == nil {
		t.Error("doctor.Deps.RemoveBlock nil")
	}
	if docDeps.AddWiring == nil {
		t.Error("doctor.Deps.AddWiring nil")
	}

	// Identity create/add deps must be non-nil.
	if idDeps.Generate == nil {
		t.Error("identity.Deps.Generate nil")
	}
	if upDeps.WriteSSH == nil {
		t.Error("identity.UpdateDeps.WriteSSH nil")
	}

	// Delete deps — all 7 fields must be non-nil (Plan 06, D-16).
	if deleteDeps.ReadSSH == nil {
		t.Error("identity.DeleteDeps.ReadSSH nil")
	}
	if deleteDeps.ReadGitconfig == nil {
		t.Error("identity.DeleteDeps.ReadGitconfig nil")
	}
	if deleteDeps.WriteSSH == nil {
		t.Error("identity.DeleteDeps.WriteSSH nil")
	}
	if deleteDeps.WriteGitconfig == nil {
		t.Error("identity.DeleteDeps.WriteGitconfig nil")
	}
	if deleteDeps.RemoveFragment == nil {
		t.Error("identity.DeleteDeps.RemoveFragment nil")
	}
	if deleteDeps.RemoveAllowedSigners == nil {
		t.Error("identity.DeleteDeps.RemoveAllowedSigners nil")
	}
	if deleteDeps.RemoveKeyFiles == nil {
		t.Error("identity.DeleteDeps.RemoveKeyFiles nil")
	}
}

// TestBuildTUIDepsNilGuard_Phase57 extends the D-16 guard to cover the three
// new Deps structs added in Phase 5.7: adopter.Deps, repoclone.Deps, uploader.Deps.
//
// RED (Plan 01): zero-value form. This test constructs zero-value Deps structs of
// the three new types and asserts each function field is non-nil — every assertion
// FAILS because the fields are nil in a zero-value struct. This is a genuine
// non-vacuous RED guard: the types exist (go build ./... exits 0) but the live
// wiring is not yet done.
//
// Plan 06 (05.7-06) rewires this to drive the real 8-value buildTUIDeps() return.
func TestBuildTUIDepsNilGuard_Phase57(t *testing.T) {
	// Zero-value Deps: all function fields are nil — assertions below will FAIL (RED).
	var adoptDeps adopter.Deps
	var repoCloneDeps repoclone.Deps
	var uploadDeps uploader.Deps

	// adopter.Deps fields
	if adoptDeps.ReadFile == nil {
		t.Error("adopter.Deps.ReadFile nil")
	}
	if adoptDeps.WriteFile == nil {
		t.Error("adopter.Deps.WriteFile nil")
	}
	if adoptDeps.CopyFile == nil {
		t.Error("adopter.Deps.CopyFile nil")
	}
	if adoptDeps.BackupAndRemove == nil {
		t.Error("adopter.Deps.BackupAndRemove nil")
	}
	if adoptDeps.WriteIncludeIf == nil {
		t.Error("adopter.Deps.WriteIncludeIf nil")
	}
	if adoptDeps.ReadFragment == nil {
		t.Error("adopter.Deps.ReadFragment nil")
	}
	if adoptDeps.ListCandidates == nil {
		t.Error("adopter.Deps.ListCandidates nil")
	}

	// repoclone.Deps fields
	if repoCloneDeps.Stat == nil {
		t.Error("repoclone.Deps.Stat nil")
	}
	if repoCloneDeps.Clone == nil {
		t.Error("repoclone.Deps.Clone nil")
	}
	if repoCloneDeps.Pull == nil {
		t.Error("repoclone.Deps.Pull nil")
	}
	if repoCloneDeps.UserHomeDir == nil {
		t.Error("repoclone.Deps.UserHomeDir nil")
	}

	// uploader.Deps fields
	if uploadDeps.LookPath == nil {
		t.Error("uploader.Deps.LookPath nil")
	}
	if uploadDeps.RunCmd == nil {
		t.Error("uploader.Deps.RunCmd nil")
	}
}
