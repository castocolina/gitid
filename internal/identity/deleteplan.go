// Package identity — deleteplan.go provides DeletePlan: a pure, error-
// returning preview of exactly what a Delete call WILL do, built from the
// SAME target-derivation helper (deleteTargets) Delete itself uses, so the
// confirm screen and the CLI dry-run render facts that can never drift from
// what the write actually performs (review R-11).
package identity

import (
	"fmt"

	"github.com/castocolina/gitid/internal/gitconfig"
)

// DeleteTarget names a LOGICAL region a delete touches: a managed block
// inside a shared file (Block non-empty) or a whole file (Block empty) —
// never just a bare file path, because a provider rewrite, an
// allowed_signers block, and an SSH Host block are all regions inside
// files OTHER identities also touch (review R-11): a test comparing "the
// set of files modified" against a target list could pass while the wrong
// block inside the right file was removed.
type DeleteTarget struct {
	File  string // the physical file the region lives in
	Block string // the managed block name, or "" for a whole-file target
	Label string // the render label; the UI never re-derives one
}

// DeletePlan is the pure preview both the confirm screen and the CLI
// dry-run render. Building one performs NO write (PlanDelete takes only
// read-only PlanDeps).
type DeletePlan struct {
	Scope DeleteScope
	Name  string
	// Targets is every logical region this delete WILL touch — the exact
	// set deleteTargets derives, which Delete's own DeleteResult.Modified
	// is built from too (review R-11's equality guarantee).
	Targets []DeleteTarget
	// ProviderRewriteTarget is non-nil exactly when the D-09 ref-count
	// found no surviving reference and the provider rewrite block will be
	// removed. nil when the ref-count says it survives, or when the
	// identity has no resolvable provider key.
	ProviderRewriteTarget *DeleteTarget
	// SharedKeyOwners names every sibling identity still referencing this
	// identity's key path (D-12) — non-empty exactly when the key SURVIVES
	// and is therefore omitted from Targets.
	SharedKeyOwners []string
	// UnmanagedHits are the D-13 advisory scan hits — never populated for
	// DeleteScopeGitOnly (the scan only runs ahead of an irreversible
	// everything-delete).
	UnmanagedHits []UnmanagedHit
	// Disclaimer is UnmanagedScanDisclaimer, carried onto the plan so the
	// render layer never needs a second import/reference to find it.
	Disclaimer string
	// KeyPaths carries the identity's key-pair paths when they ARE a
	// target (scope everything, key does not survive) — empty otherwise.
	KeyPaths []string
}

// PlanDeps holds the READ-ONLY seams PlanDelete needs. There is no writer
// field of any kind — that absence is the structural guarantee that
// building a plan cannot mutate anything.
type PlanDeps struct {
	// Accounts returns every reconstructed account (including the one being
	// planned for) — the SAME list DeleteDeps.Accounts supplies to Delete.
	Accounts func() ([]Account, error)
	// ForeignProviderRefs mirrors DeleteDeps.ForeignProviderRefs: the D-09
	// hand-written-alias half of the provider reference count.
	ForeignProviderRefs func(providerKey string) (int, error)
	// ScanSources returns the D-13 scan sources (SSH config, gitconfig, the
	// identity's fragment, and allowed_signers — never a private key file).
	// Nil is tolerated (no scan performed, UnmanagedHits stays empty).
	ScanSources func() ([]ScanSource, error)
}

// PlanDelete builds the pure preview for acct under scope. It returns an
// ERROR for any read/parse failure inside it — a plan that failed to read a
// file must never be indistinguishable from a legitimately small plan (the
// domain half of review R-07's fail-closed requirement). On any error the
// returned DeletePlan is the ZERO VALUE, never a partial plan with a nil
// error.
func PlanDelete(acct Account, scope DeleteScope, deps PlanDeps) (DeletePlan, error) {
	switch scope {
	case DeleteScopeGitOnly, DeleteScopeEverything:
	default:
		return DeletePlan{}, fmt.Errorf("identity: delete scope %q: %w", scope, ErrScopeNotAvailable)
	}

	providerSurvives := true
	keySurvives := false
	var sharedKeyOwners []string
	var unmanagedHits []UnmanagedHit

	if scope == DeleteScopeEverything {
		if deps.Accounts == nil {
			return DeletePlan{}, fmt.Errorf("identity: plan delete: PlanDeps.Accounts is required for the everything scope")
		}
		accounts, aerr := deps.Accounts()
		if aerr != nil {
			return DeletePlan{}, fmt.Errorf("identity: listing accounts: %w", aerr)
		}

		if acct.KeyPath != "" {
			sharedKeyOwners = SharedKeyOwners(accounts, acct.KeyPath, acct.Name)
			keySurvives = len(sharedKeyOwners) > 0
		}

		providerKey := RewriteProviderKey(acct.Provider, acct.Alias)
		if providerKey != "" {
			managedRefs := ProviderRefCount(accounts, providerKey, acct.Name)
			foreignRefs := 0
			if deps.ForeignProviderRefs != nil {
				var ferr error
				foreignRefs, ferr = deps.ForeignProviderRefs(providerKey)
				if ferr != nil {
					return DeletePlan{}, fmt.Errorf("identity: counting foreign provider refs: %w", ferr)
				}
			}
			providerSurvives = managedRefs > 0 || foreignRefs > 0
		}

		if deps.ScanSources != nil {
			sources, serr := deps.ScanSources()
			if serr != nil {
				return DeletePlan{}, fmt.Errorf("identity: gathering scan sources: %w", serr)
			}
			unmanagedHits = ScanUnmanagedReferences(acct.Alias, sources)
		}
	}

	plan := DeletePlan{
		Scope:                 scope,
		Name:                  acct.Name,
		Targets:               deleteTargets(acct, scope, providerSurvives, keySurvives),
		ProviderRewriteTarget: providerRewriteDeleteTarget(acct, providerSurvives),
		SharedKeyOwners:       sharedKeyOwners,
		UnmanagedHits:         unmanagedHits,
		Disclaimer:            UnmanagedScanDisclaimer,
	}
	if scope == DeleteScopeEverything && acct.KeyPath != "" && !keySurvives {
		plan.KeyPaths = nonEmptyStrings(acct.KeyPath, acct.PubPath)
	}
	return plan, nil
}

// deleteTargets is the ONE target-derivation helper both PlanDelete and
// Delete (delete.go) call, so the preview can never promise a target the
// write does not touch, or omit one it does (review R-11).
//
// keySurvives is a REQUIRED input, symmetric with providerSurvives (review
// R2-01): the key targets are emitted only when scope is everything AND the
// key does NOT survive, exactly as the provider-rewrite target is emitted
// only when scope is everything AND the provider rewrite does NOT survive.
// Both booleans are computed exactly ONCE by the caller (Delete computes
// them at the top of its call; PlanDelete computes them from the same
// PlanDeps.Accounts list) and passed in here — this function never
// re-derives either one. (A comment-stripped grep gate in
// deleteplan_test.go pins that this function's body contains no
// SharedKeyOwners call, so keySurvives can only arrive as a parameter.)
func deleteTargets(acct Account, scope DeleteScope, providerSurvives, keySurvives bool) []DeleteTarget {
	targets := []DeleteTarget{
		{File: acct.GitconfigPath, Block: acct.Name, Label: "Git includeIf block"},
		{File: acct.FragmentPath, Block: "", Label: "Git fragment file"},
	}

	if scope != DeleteScopeEverything {
		return targets
	}

	targets = append(targets,
		DeleteTarget{File: acct.SSHConfigPath, Block: acct.Name, Label: "SSH Host block"},
		DeleteTarget{File: acct.AllowedSignersPath, Block: acct.Name, Label: "allowed_signers entry"},
	)

	if pr := providerRewriteDeleteTarget(acct, providerSurvives); pr != nil {
		targets = append(targets, *pr)
	}

	if acct.KeyPath != "" && !keySurvives {
		targets = append(targets, DeleteTarget{File: acct.KeyPath, Block: "", Label: "Key pair"})
	}

	return targets
}

// providerRewriteDeleteTarget builds the DeleteTarget for the shared
// provider-rewrite block, or nil when providerSurvives (the ref-count says
// it stays) or when the identity has no resolvable provider key. Shared by
// deleteTargets (folded into its returned slice) and PlanDelete/Delete's
// separate ProviderRewriteTarget field, so the two can never disagree about
// the block's name.
func providerRewriteDeleteTarget(acct Account, providerSurvives bool) *DeleteTarget {
	if providerSurvives {
		return nil
	}
	providerKey := RewriteProviderKey(acct.Provider, acct.Alias)
	if providerKey == "" {
		return nil
	}
	blockName, err := gitconfig.ProviderRewriteBlockName(providerKey)
	if err != nil {
		return nil
	}
	return &DeleteTarget{File: acct.GitconfigPath, Block: blockName, Label: "Provider rewrite (shared)"}
}

// nonEmptyStrings returns vals with every empty string filtered out.
func nonEmptyStrings(vals ...string) []string {
	var out []string
	for _, v := range vals {
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}
