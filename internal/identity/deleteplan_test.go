package identity

import (
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
)

// planTestAccount returns a fully-populated Account for plan tests, sharing
// baseDeleteAccount's shape (delete_test.go) so both suites describe the
// same fixture identity.
func planTestAccount() Account {
	return baseDeleteAccount()
}

// fakePlanDeps builds PlanDeps from static fixtures.
func fakePlanDeps(accounts []Account, foreignRefs int, foreignErr error, sources []ScanSource) PlanDeps {
	return PlanDeps{
		Accounts: func() ([]Account, error) { return accounts, nil },
		ForeignProviderRefs: func(string) (int, error) {
			return foreignRefs, foreignErr
		},
		ScanSources: func() ([]ScanSource, error) { return sources, nil },
	}
}

// TestPlanDelete_SoleProvider_CarriesProviderRewriteTarget asserts the
// everything-scope plan for a sole-provider identity carries a non-nil
// ProviderRewriteTarget whose Block is provider-rewrite:<host>.
func TestPlanDelete_SoleProvider_CarriesProviderRewriteTarget(t *testing.T) {
	acct := planTestAccount()
	deps := fakePlanDeps([]Account{acct}, 0, nil, nil)

	plan, err := PlanDelete(acct, DeleteScopeEverything, deps)
	if err != nil {
		t.Fatalf("PlanDelete error: %v", err)
	}
	if plan.ProviderRewriteTarget == nil {
		t.Fatal("ProviderRewriteTarget = nil, want non-nil (sole provider reference)")
	}
	if plan.ProviderRewriteTarget.Block != "provider-rewrite:github.com" {
		t.Errorf("ProviderRewriteTarget.Block = %q, want %q", plan.ProviderRewriteTarget.Block, "provider-rewrite:github.com")
	}
}

// TestPlanDelete_SharedProvider_NilProviderRewriteTarget is the survival
// direction: a sibling on the same provider means ProviderRewriteTarget is nil.
func TestPlanDelete_SharedProvider_NilProviderRewriteTarget(t *testing.T) {
	acct := planTestAccount()
	sibling := acct
	sibling.Name = "personal"
	sibling.Alias = "personal.github.com"
	deps := fakePlanDeps([]Account{acct, sibling}, 0, nil, nil)

	plan, err := PlanDelete(acct, DeleteScopeEverything, deps)
	if err != nil {
		t.Fatalf("PlanDelete error: %v", err)
	}
	if plan.ProviderRewriteTarget != nil {
		t.Errorf("ProviderRewriteTarget = %+v, want nil (a sibling identity shares the provider)", plan.ProviderRewriteTarget)
	}
}

// TestPlanDelete_GitOnly_OmitsSSHKeyAndSigners asserts the Git-only plan
// omits the SSH config, the key paths, and the allowed_signers file from its
// target list.
func TestPlanDelete_GitOnly_OmitsSSHKeyAndSigners(t *testing.T) {
	acct := planTestAccount()
	deps := fakePlanDeps([]Account{acct}, 0, nil, nil)

	plan, err := PlanDelete(acct, DeleteScopeGitOnly, deps)
	if err != nil {
		t.Fatalf("PlanDelete error: %v", err)
	}
	for _, target := range plan.Targets {
		if target.File == acct.SSHConfigPath || target.File == acct.KeyPath || target.File == acct.PubPath || target.File == acct.AllowedSignersPath {
			t.Errorf("git-only plan target %+v references an everything-only file", target)
		}
	}
	if plan.ProviderRewriteTarget != nil {
		t.Error("git-only plan carries a non-nil ProviderRewriteTarget, want nil")
	}
	if len(plan.KeyPaths) != 0 {
		t.Errorf("git-only plan KeyPaths = %v, want empty", plan.KeyPaths)
	}
}

// TestPlanDelete_PerformsNoWrite snapshots every seeded file's bytes, calls
// PlanDelete, and asserts all bytes are unchanged.
func TestPlanDelete_PerformsNoWrite(t *testing.T) {
	dir := t.TempDir()
	acct := planTestAccount()
	acct.GitconfigPath = dir + "/gitconfig"
	if err := os.WriteFile(acct.GitconfigPath, []byte("original content"), 0o600); err != nil {
		t.Fatalf("seeding gitconfig fixture: %v", err)
	}
	before, err := os.ReadFile(acct.GitconfigPath) //nolint:gosec // hermetic t.TempDir() fixture path
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}

	deps := fakePlanDeps([]Account{acct}, 0, nil, nil)
	if _, err := PlanDelete(acct, DeleteScopeEverything, deps); err != nil {
		t.Fatalf("PlanDelete error: %v", err)
	}

	after, err := os.ReadFile(acct.GitconfigPath) //nolint:gosec // hermetic t.TempDir() fixture path
	if err != nil {
		t.Fatalf("reading fixture after PlanDelete: %v", err)
	}
	if string(before) != string(after) {
		t.Errorf("PlanDelete mutated a seeded file: before=%q after=%q", before, after)
	}
}

// TestPlanDelete_SharedKey_OmitsKeyPairCarriesOwners asserts the everything-
// scope plan for a SHARED-key identity carries no key-pair target and a
// non-empty SharedKeyOwners, and that the same identity with a sole-owner
// key DOES carry the key-pair target.
func TestPlanDelete_SharedKey_OmitsKeyPairCarriesOwners(t *testing.T) {
	acct := planTestAccount()
	sibling := Account{Name: "sibling", KeyPath: acct.KeyPath, Alias: "sibling.github.com"}

	shared := fakePlanDeps([]Account{acct, sibling}, 0, nil, nil)
	plan, err := PlanDelete(acct, DeleteScopeEverything, shared)
	if err != nil {
		t.Fatalf("PlanDelete error: %v", err)
	}
	if len(plan.SharedKeyOwners) != 1 || plan.SharedKeyOwners[0] != "sibling" {
		t.Errorf("SharedKeyOwners = %v, want [\"sibling\"]", plan.SharedKeyOwners)
	}
	for _, target := range plan.Targets {
		if target.File == acct.KeyPath {
			t.Errorf("shared-key plan still carries a key-pair target: %+v", target)
		}
	}

	solo := fakePlanDeps([]Account{acct}, 0, nil, nil)
	plan2, err := PlanDelete(acct, DeleteScopeEverything, solo)
	if err != nil {
		t.Fatalf("PlanDelete error: %v", err)
	}
	var foundKeyTarget bool
	for _, target := range plan2.Targets {
		if target.File == acct.KeyPath {
			foundKeyTarget = true
		}
	}
	if !foundKeyTarget {
		t.Error("sole-owner plan is missing the key-pair target")
	}
}

// deleteTargetSet converts a []DeleteTarget into a set of "File\x00Block" keys.
func deleteTargetSet(targets []DeleteTarget) map[string]bool {
	set := make(map[string]bool, len(targets))
	for _, t := range targets {
		set[t.File+"\x00"+t.Block] = true
	}
	return set
}

func setsEqual(a, b map[string]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}

// planWriteFixture builds a fake DeleteDeps + PlanDeps pair over the SAME
// accounts/providerKey/keyOwnership facts, so PlanDelete and Delete can be
// driven over literally the same inputs.
func planWriteFixture(accounts []Account, sshFixture, gcFixture []byte, foreignRefs int) (DeleteDeps, PlanDeps) {
	var log deleteCallLog
	dd := newFakeEverythingDeps(&log, sshFixture, gcFixture, accounts)
	dd.ForeignProviderRefs = func(string) (int, error) { return foreignRefs, nil }

	pd := fakePlanDeps(accounts, foreignRefs, nil, nil)
	return dd, pd
}

// TestPlanDeleteAndDelete_AgreeAcrossThreeFixtures is the review R2-01 core
// equality proof: PlanDelete and Delete are driven over the SAME accounts
// slice across three fixture classes (sole owner of both key and provider,
// shared key, shared provider), and DeleteResult.Modified equals
// DeletePlan.Targets in each, as sets of (File, Block) pairs.
func TestPlanDeleteAndDelete_AgreeAcrossThreeFixtures(t *testing.T) {
	acct := planTestAccount()

	cases := []struct {
		name     string
		accounts []Account
	}{
		{"sole owner of key and provider", []Account{acct}},
		{"shared key", []Account{acct, {Name: "sibling-key", KeyPath: acct.KeyPath, Alias: "sibling-key.github.com"}}},
		{"shared provider", []Account{acct, {Name: "sibling-provider", KeyPath: "/tmp/.ssh/id_ed25519_other", Alias: "sibling-provider.github.com"}}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dd, pd := planWriteFixture(tc.accounts, sshFixtureWithBlocks(), gcFixtureWithBlocks(), 0)

			plan, err := PlanDelete(acct, DeleteScopeEverything, pd)
			if err != nil {
				t.Fatalf("PlanDelete error: %v", err)
			}
			res, err := Delete(acct, DeleteScopeEverything, dd)
			if err != nil {
				t.Fatalf("Delete error: %v", err)
			}

			planSet := deleteTargetSet(plan.Targets)
			writeSet := deleteTargetSet(res.Modified)
			if !setsEqual(planSet, writeSet) {
				t.Errorf("plan/write target sets differ:\nplan:  %v\nwrite: %v", plan.Targets, res.Modified)
			}
		})
	}
}

// TestPlanDeleteAndDelete_KeySurvivesInvertedNegativeControl proves the
// shared-key dimension of the equality comparison is load-bearing: calling
// deleteTargets with keySurvives INVERTED for one side makes the equality
// assertion FAIL.
func TestPlanDeleteAndDelete_KeySurvivesInvertedNegativeControl(t *testing.T) {
	acct := planTestAccount()

	// The "write" side believes the key survives (a sibling shares it).
	writeTargets := deleteTargets(acct, DeleteScopeEverything, true, true)
	// The "plan" side (inverted) believes the key does NOT survive.
	planTargets := deleteTargets(acct, DeleteScopeEverything, true, false)

	if setsEqual(deleteTargetSet(writeTargets), deleteTargetSet(planTargets)) {
		t.Fatal("plan/write sets are equal despite an inverted keySurvives input — the negative control is not load-bearing")
	}
}

// TestPlanDeleteAndDelete_MutatedOutputNegativeControl proves the equality
// comparison is not vacuous: mutating one side's output makes it fail.
func TestPlanDeleteAndDelete_MutatedOutputNegativeControl(t *testing.T) {
	acct := planTestAccount()
	targets := deleteTargets(acct, DeleteScopeEverything, true, false)
	mutated := append([]DeleteTarget{}, targets...)
	mutated = append(mutated, DeleteTarget{File: "/not/a/real/target", Block: "bogus"})

	if setsEqual(deleteTargetSet(targets), deleteTargetSet(mutated)) {
		t.Fatal("mutated target set compares equal to the original — the comparison is vacuous")
	}
}

// TestDeleteTargets_NeverCallsSharedKeyOwners is a structural, comment-
// stripped grep gate (review R3-03): deleteTargets's SOURCE must never
// reference SharedKeyOwners — keySurvives can only ever arrive as a
// parameter, never be re-derived inside the helper.
func TestDeleteTargets_NeverCallsSharedKeyOwners(t *testing.T) {
	src, err := os.ReadFile("deleteplan.go") //nolint:gosec // fixed relative path to this package's own source file
	if err != nil {
		t.Fatalf("reading deleteplan.go: %v", err)
	}
	body := extractFuncBody(t, string(src), "func deleteTargets(")
	stripped := stripGoComments(body)
	if strings.Contains(stripped, "SharedKeyOwners") {
		t.Error("deleteTargets's body calls SharedKeyOwners — keySurvives must arrive as a parameter only")
	}
}

// extractFuncBody returns the substring of src from the given function
// signature marker through its closing brace at column 0 (the standard
// gofmt top-level function closing brace).
func extractFuncBody(t *testing.T, src, marker string) string {
	t.Helper()
	idx := strings.Index(src, marker)
	if idx < 0 {
		t.Fatalf("marker %q not found in source", marker)
	}
	rest := src[idx:]
	end := strings.Index(rest, "\n}\n")
	if end < 0 {
		t.Fatalf("could not find closing brace for %q", marker)
	}
	return rest[:end]
}

// stripGoComments removes // line comments (a lightweight pass sufficient
// for this file's own straightforward comment style — no /* */ blocks or
// string literals containing "//" appear inside deleteTargets).
func stripGoComments(src string) string {
	lines := strings.Split(src, "\n")
	var out []string
	for _, line := range lines {
		if idx := strings.Index(line, "//"); idx >= 0 {
			line = line[:idx]
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

// TestPlanDelete_ReadFailureReturnsErrorAndZeroPlan injects a failing read
// seam into PlanDeps and asserts PlanDelete returns a non-nil error and a
// zero-value plan, never a partial plan with a nil error.
func TestPlanDelete_ReadFailureReturnsErrorAndZeroPlan(t *testing.T) {
	acct := planTestAccount()
	sentinel := errors.New("injected accounts read failure")
	deps := PlanDeps{
		Accounts: func() ([]Account, error) { return nil, sentinel },
	}

	plan, err := PlanDelete(acct, DeleteScopeEverything, deps)
	if err == nil {
		t.Fatal("PlanDelete must return an error when Accounts fails")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("error = %v, want it to wrap the injected sentinel", err)
	}
	if !reflect.DeepEqual(plan, DeletePlan{}) {
		t.Errorf("plan = %+v, want the zero value on error", plan)
	}
}

// TestPlanDelete_UnmanagedHitsUntruncated asserts DeletePlan.UnmanagedHits
// carries untruncated matched lines — the domain never truncates.
func TestPlanDelete_UnmanagedHitsUntruncated(t *testing.T) {
	acct := planTestAccount()
	longLine := "Host " + strings.Repeat("x", 300) + " " + acct.Alias
	sources := []ScanSource{{File: "ssh-config", Bytes: []byte(longLine + "\n"), Region: ScanRegionUnmanaged}}
	deps := fakePlanDeps([]Account{acct}, 0, nil, sources)

	plan, err := PlanDelete(acct, DeleteScopeEverything, deps)
	if err != nil {
		t.Fatalf("PlanDelete error: %v", err)
	}
	if len(plan.UnmanagedHits) != 1 {
		t.Fatalf("UnmanagedHits = %v, want exactly 1 hit", plan.UnmanagedHits)
	}
	if plan.UnmanagedHits[0].Text != longLine {
		t.Errorf("hit text was altered/truncated: got %d chars, want %d chars", len(plan.UnmanagedHits[0].Text), len(longLine))
	}
	if plan.Disclaimer != UnmanagedScanDisclaimer {
		t.Errorf("Disclaimer = %q, want the frozen constant", plan.Disclaimer)
	}
}

// TestPlanDelete_ForeignRefsErrorPropagates asserts a ForeignProviderRefs
// failure surfaces as a PlanDelete error, not a silently-wrong plan.
func TestPlanDelete_ForeignRefsErrorPropagates(t *testing.T) {
	acct := planTestAccount()
	sentinel := errors.New("injected foreign-refs failure")
	deps := PlanDeps{
		Accounts:            func() ([]Account, error) { return []Account{acct}, nil },
		ForeignProviderRefs: func(string) (int, error) { return 0, sentinel },
	}
	_, err := PlanDelete(acct, DeleteScopeEverything, deps)
	if !errors.Is(err, sentinel) {
		t.Errorf("error = %v, want it to wrap the injected sentinel", err)
	}
}
