package identity

import (
	"fmt"
	"path/filepath"
	"strings"
)

// RotateResult reports everything a rotation ceremony needs to display,
// wrapping the shared CreateResult (previews, tests, backups) plus the D-08
// grace-window facts: where the retired key pair landed, and which provider
// host the caller renders the D-08 "old key still verifies" hint against.
// ArchivedPrivatePath/ArchivedPublicPath are populated for a failure at ANY
// step from the archive onward — including a failure inside the archive step
// itself (review R3-01) — so a caller's rollback journal always knows what to
// undo, never just on a clean success.
type RotateResult struct {
	CreateResult
	ArchivedPrivatePath string
	ArchivedPublicPath  string
	ProviderHost        string
}

// Rotate orchestrates the D-05/D-06/D-07/D-08 key RETIREMENT ceremony for an
// existing identity: generate a fresh key pair, archive the previous pair
// with MOVE semantics (vacating the canonical path — D-06), persist the new
// pair at that now-vacant canonical path exactly once, APPEND the new
// signing line while KEEPING the old one (D-07), and re-point the SSH/git
// artifacts to the SAME canonical paths the identity already used (D-08: the
// IdentityFile/signingkey VALUES never change — only the bytes on disk at
// that path do).
//
// Rotate composes the four phases from identity.go DIRECTLY — preWriteGate,
// persistStagedKey, writeArtifacts, resolvedPhase — with an archive step
// inserted between the gate and the persist. It never calls runPipeline:
// that is the structural fix for the double-persist defect cross-AI review
// found (review R-01) — persistStagedKey is invoked exactly once here, and
// nothing in this function re-enters it.
//
// Sequencing, and why this exact order:
//
//  1. deps.Generate — into hermetic staging; nothing on disk moves yet.
//  2. preWriteGate  — gate on the STAGED key BEFORE anything on disk moves,
//     so an archive only ever happens once a proven replacement is ready.
//  3. deps.ArchiveKeyPair — move the OLD pair out of the canonical paths.
//     Archiving BEFORE persisting is what vacates the canonical path rather
//     than overwriting it (D-06). The returned archive paths are assigned
//     onto the result UNCONDITIONALLY, before the seam's error is ever
//     inspected (see below) — this is deliberate, not an oversight.
//  4. persistStagedKey — the new pair takes the vacated canonical paths.
//     Exactly once, by construction (phase 2 of identity.go, called nowhere
//     else in this function).
//  5. writeArtifacts — with the APPENDING signer writer (D-07), never the
//     replacing deps.WriteAllowedSigners.
//  6. resolvedPhase — the closing post-write test.
//
// On an error at step 3 or LATER, Rotate returns the RotateResult carrying
// whatever archive paths were created, alongside the error — with NO gap.
// Step 3 is structured as: call the seam, assign BOTH returned paths onto
// the result, THEN check the error. keygen.MoveKeyPairToArchive (via the
// composition root's archiveKeyPairSeam) returns its ArchivedPair together
// with an ErrArchiveIncomplete-wrapped error when a source removal fails —
// the archive copies are valid either way, and the obvious
// `if err != nil { return RotateResult{}, err }` shape at this step would
// silently discard that information, leaving the one step that can fail
// with copies already on disk as the single case where the caller gets an
// error and no paths to clean up (review R3-01). Restoring the archived
// pair to the canonical paths on a downstream failure is the CALLER's
// transaction responsibility (cmd/gitid's mutationJournal, wired through
// depsForTransaction) — this domain function's only contract is to report
// precisely what it created, at every step, with no gap.
func Rotate(existing Account, deps Deps) (RotateResult, error) {
	res := RotateResult{ProviderHost: existing.Provider}

	in := rotateInput(existing)

	staged, gerr := deps.Generate(in)
	if gerr != nil {
		return res, fmt.Errorf("identity: generating rotation key: %w", gerr)
	}
	defer deps.Cleanup(staged)

	gateRes, hostBlock, err := preWriteGate(in, staged, deps)
	res.CreateResult = gateRes
	if err != nil {
		return res, err
	}

	// Step 3: archive the OLD pair, vacating the canonical path (D-06).
	// Assign both returned paths onto the result BEFORE inspecting the
	// error — see the doc comment above (review R3-01).
	archivedPriv, archivedPub, aerr := deps.ArchiveKeyPair(existing.KeyPath, existing.PubPath)
	res.ArchivedPrivatePath = archivedPriv
	res.ArchivedPublicPath = archivedPub
	if aerr != nil {
		return res, fmt.Errorf("identity: archiving previous key pair: %w", aerr)
	}

	if perr := persistStagedKey(staged, deps); perr != nil {
		return res, perr
	}

	// D-07: append the new signer line, keep the old one(s). The adapter
	// ignores writeArtifacts' pre-built signersLine parameter and instead
	// calls deps.AppendAllowedSigners with the account's email and the NEW
	// staged public line — AppendAllowedSigners builds its own line
	// internally (through the same CR-18-guarded keygen.AllowedSignersLine
	// helper preWriteGate used to build the preview), so the two stay
	// consistent without threading a second copy of the line through.
	appendWriter := signersWriter(func(path, identity, _ string) (string, error) {
		return deps.AppendAllowedSigners(path, identity, in.GitEmail, staged.PubLine)
	})

	artifactRes, werr := writeArtifacts(in, hostBlock, res.AllowedSignersLine, res.CreateResult, deps, appendWriter)
	res.CreateResult = artifactRes
	if werr != nil {
		return res, werr
	}

	res.CreateResult = resolvedPhase(in, res.CreateResult, deps)
	return res, nil
}

// rotateInput builds the CreateInput that re-points an existing account's
// four artifacts. It carries the SAME identity name, alias, matches, and
// managed target paths as the account, PLUS the SAME algorithm the existing
// key was generated with (algoFromKeyPath, parsed from existing.KeyPath's
// own id_<algo>_<name> naming convention) — so the real composition root's
// deps.Generate (which derives the final key path from CreateInput.Algo and
// CreateInput.Name via keygen.KeyPaths) recomputes the IDENTICAL canonical
// path rather than guessing. Without this, an empty Algo would either fail
// generation outright or land the new key at the wrong filename, breaking
// D-08's "the IdentityFile/signingkey VALUES never change" contract.
func rotateInput(a Account) CreateInput {
	return CreateInput{
		Name:               a.Name,
		GitName:            a.GitName,
		GitEmail:           a.GitEmail,
		Provider:           a.Provider,
		Algo:               algoFromKeyPath(a.KeyPath, a.Name),
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

// algoFromKeyPath extracts the algorithm segment embedded in a gitid-managed
// key filename — keygen.KeyPaths' own "id_<algo>_<identityName>" naming
// convention — by stripping the "id_" prefix and the exact "_<identityName>"
// suffix, leaving whatever remains as the algorithm. Stripping the KNOWN
// exact suffix (rather than a naive split on "_") keeps this correct even
// when identityName itself contains underscores, and even when the
// algorithm segment itself contains a hyphen (e.g. "rsa-4096").
func algoFromKeyPath(keyPath, identityName string) string {
	base := filepath.Base(keyPath)
	base = strings.TrimPrefix(base, "id_")
	base = strings.TrimSuffix(base, "_"+identityName)
	return base
}
