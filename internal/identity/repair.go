package identity

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/castocolina/gitid/internal/keygen"
)

// repairAlgo is the algorithm RepairKey always generates: ed25519, gitid's
// committed one-key-per-identity signing model (CLAUDE.md, recipes/). Repair
// never reads or reuses pre-existing key material (D-05), so — unlike
// Rotate, which must reconstruct the SAME algorithm to land at the SAME
// canonical path — there is no prior algorithm to preserve here. Shared
// between RepairKeyPath and repairInput so the two can never drift apart.
const repairAlgo = "ed25519"

// ErrRepairTargetShared is returned when an identity's OWN canonical key
// path (RepairKeyPath) is referenced by another identity. Repair fails
// closed rather than overwrite a credential a sibling identity depends on
// (D-05): the caller supplies the owner list computed against the repair
// TARGET path, not against Account.KeyPath.
var ErrRepairTargetShared = errors.New("identity: repair target key is referenced by another identity")

// RepairKeyPath returns the key-pair paths canonical FOR THIS IDENTITY —
// derived from its OWN name via keygen.KeyPaths — NEVER from
// Account.KeyPath (review R-02). This is the whole fix for the
// contradiction review found in the naive reading of "repair at the
// canonical path": that path is canonical for THIS identity, derived from
// its own name, not whatever path Account.KeyPath currently happens to
// name. When an identity shares a sibling's key, Account.KeyPath is
// id_ed25519_<sibling> while the repair target is id_ed25519_<name> — a
// DIFFERENT file — so re-pointing this identity is inherently
// non-destructive to the sibling.
//
// The directory is resolved from the identity's own KeyPath when set (this
// works for BOTH the key-missing case — the path still names where the
// absent file WOULD be — and the shared-key case, since every identity's
// key lives in the SAME ~/.ssh directory by construction), falling back to
// the directory of SSHConfigPath when KeyPath is empty.
func RepairKeyPath(existing Account) (privPath, pubPath string) {
	return keygen.KeyPaths(keyDirFor(existing), repairAlgo, existing.Name)
}

// keyDirFor resolves the ~/.ssh directory an identity's key material lives
// in (or would live in), per RepairKeyPath's doc comment above.
func keyDirFor(a Account) string {
	if a.KeyPath != "" {
		return filepath.Dir(a.KeyPath)
	}
	return filepath.Dir(a.SSHConfigPath)
}

// RepairKey orchestrates the D-05/KEY-07/MGR-05 key REPAIR ceremony: it
// generates a fresh key at THIS identity's own canonical path
// (RepairKeyPath) and re-points the four artifacts to it — WITHOUT ever
// touching pre-existing key material. Repair never calls deps.ArchiveKeyPair
// and never stats, reads, backs up, or removes any pre-existing key file:
// D-05's rule is that repair never touches pre-existing key material,
// because that material may be entirely absent (key-missing) or may belong
// to a sibling identity (the shared-key case) — either way, disturbing it is
// wrong.
//
// otherOwnersOfTarget is the list of OTHER identities referencing the repair
// TARGET path (RepairKeyPath's result), computed by the caller (the
// composition root's SharedKeyOwners against the TARGET path, never against
// Account.KeyPath — the pathological case a naive "repair at
// Account.KeyPath" reading would get wrong). When non-empty, RepairKey
// returns a wrapped ErrRepairTargetShared naming every entry and performs NO
// effect at all — no seam on Deps is invoked. Failing closed here is
// correct: the alternative is overwriting a key another identity depends
// on, which D-05 forbids.
//
// This plan's "Planner resolution: repair signer semantics" resolves
// 05-RESEARCH.md's Open Question 1 to APPEND, not REPLACE: RepairKey selects
// the SAME appending signer writer Rotate uses (deps.AppendAllowedSigners,
// D-07's rationale extended verbatim to repair — a line whose private key
// was never generated, or belonged to nobody, still costs nothing to keep,
// and pruning is deliberately deferred to Phase 8).
//
// RepairKey composes identity.go's four phases directly — preWriteGate,
// persistStagedKey, writeArtifacts, resolvedPhase — minus the retirement
// steps Rotate adds (no archive step, no pre-existing-key handling). It
// never calls runPipeline.
func RepairKey(existing Account, otherOwnersOfTarget []string, deps Deps) (CreateResult, error) {
	if len(otherOwnersOfTarget) > 0 {
		return CreateResult{}, fmt.Errorf("%w: %s", ErrRepairTargetShared, strings.Join(otherOwnersOfTarget, ", "))
	}

	in := repairInput(existing)

	staged, gerr := deps.Generate(in)
	if gerr != nil {
		return CreateResult{}, fmt.Errorf("identity: generating repair key: %w", gerr)
	}
	defer deps.Cleanup(staged)

	res, hostBlock, err := preWriteGate(in, staged, deps)
	if err != nil {
		return res, err
	}

	if perr := persistStagedKey(staged, deps); perr != nil {
		return res, perr
	}

	// D-07 (extended to repair per this plan's signer-semantics resolution):
	// append the new signer line rather than replace. The adapter ignores
	// writeArtifacts' pre-built signersLine parameter and calls
	// deps.AppendAllowedSigners directly with the account's email and the
	// NEW staged public line, mirroring rotate.go's identical adapter.
	appendWriter := signersWriter(func(path, identity, _ string) (string, error) {
		return deps.AppendAllowedSigners(path, identity, in.GitEmail, staged.PubLine)
	})

	res, err = writeArtifacts(in, hostBlock, res.AllowedSignersLine, res, deps, appendWriter)
	if err != nil {
		return res, err
	}

	return resolvedPhase(in, res, deps), nil
}

// repairInput builds the CreateInput RepairKey generates against: the
// account's own name/alias/matches/managed target paths, plus the FIXED
// repairAlgo (ed25519) — never an algorithm parsed from any existing key,
// because repair never reads pre-existing key material.
func repairInput(a Account) CreateInput {
	return CreateInput{
		Name:               a.Name,
		GitName:            a.GitName,
		GitEmail:           a.GitEmail,
		Provider:           a.Provider,
		Algo:               repairAlgo,
		Alias:              a.Alias,
		Hostname:           a.Hostname,
		Port:               a.Port,
		Matches:            a.Matches,
		FragmentPath:       a.FragmentPath,
		GitconfigPath:      a.GitconfigPath,
		SSHConfigPath:      a.SSHConfigPath,
		AllowedSignersPath: a.AllowedSignersPath,
		GlobalBlock:        "",
	}
}
