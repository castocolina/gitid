package identity

import (
	"fmt"
)

// Reuse orchestrates the reuse-existing-key flow (IDENT-02, D-10 mode 2): instead
// of generating a fresh key it points the identity at an existing private key at
// existingKeyPath. When the matching `<key>.pub` is absent it derives the
// authorized-key line from the private key and writes it 0644 (RESEARCH Q3), then
// funnels into the SAME copy→pre-write→preview→write(four artifacts incl.
// allowed_signers)→resolved pipeline as Create — there is no parallel write path.
//
// For Reuse, TempPrivatePath == FinalPrivatePath (the existing ~/.ssh key) and
// PrivPEM is nil, so PersistKey and Cleanup are guaranteed no-ops.
//
// The derived `.pub` line is the only public material that leaves the private key
// (T-02-28); the private key body is never copied or printed.
func Reuse(in CreateInput, existingKeyPath string, deps Deps) (CreateResult, error) {
	staged, err := StageReuse(existingKeyPath, in.Name+"@gitid", deps)
	if err != nil {
		return CreateResult{}, err
	}
	// The staged path is read-only: if the .pub was missing it was derived in
	// memory but not written. A confirmed reuse writes the public sibling now
	// so the persisted identity has a matched key pair on disk.
	if _, perr := writeReusePub(staged, deps); perr != nil {
		return CreateResult{}, perr
	}
	return runPipeline(in, staged, deps)
}

// writeReusePub writes staged.FinalPubPath when it does not already exist and
// the staged key carries a public line. It is idempotent and safe to call from
// both the single-shot Reuse path and the TUI's confirmed CommitCreate.
func writeReusePub(staged StagedKey, deps Deps) (string, error) {
	if staged.PubLine == "" {
		return "", nil
	}
	if deps.PubExists != nil && deps.PubExists(staged.FinalPubPath) {
		return "", nil
	}
	if werr := deps.WritePub(staged.FinalPubPath, staged.PubLine); werr != nil {
		return "", fmt.Errorf("identity: writing derived public key %s: %w", staged.FinalPubPath, werr)
	}
	return staged.FinalPubPath, nil
}

// StageReuse builds the StagedKey for an existing-key reuse (IDENT-02, D-10
// mode 2) WITHOUT running the write pipeline — the staging half of Reuse,
// extracted so a staged caller (the TUI create wizard, which runs its two
// connectivity stages against a throwaway temp config BEFORE ever touching
// ~/.ssh/config) shares exactly the same ensurePub logic Reuse itself uses,
// rather than a second, divergent copy.
//
// StageReuse is READ-ONLY with respect to the public sibling: if the .pub is
// absent it is derived in memory but NOT written to disk. The confirmed write
// transaction writes/normalizes the .pub later (CR-02). This prevents any
// mutation of the user's files before consent.
//
// TempPrivatePath == FinalPrivatePath (the existing ~/.ssh key) and PrivPEM
// is nil, so a caller's PersistKey/Cleanup on the returned StagedKey are
// guaranteed no-ops — identical to Reuse's own contract.
func StageReuse(existingKeyPath, comment string, deps Deps) (StagedKey, error) {
	pubPath := existingKeyPath + ".pub"

	pubLine, err := ensurePubReadOnly(existingKeyPath, pubPath, comment, deps)
	if err != nil {
		return StagedKey{}, err
	}

	return StagedKey{
		TempPrivatePath:  existingKeyPath,
		FinalPrivatePath: existingKeyPath,
		FinalPubPath:     pubPath,
		PubLine:          pubLine,
		PrivPEM:          nil,
	}, nil
}

// ensurePubReadOnly returns the reused identity's public-key line, reading an
// existing `.pub` verbatim or deriving it from the private key when absent. It
// does NOT write the derived line — staging must be read-only (CR-02).
//
// When the `.pub` ALREADY EXISTS it is read back verbatim via the ReadPub seam
// and the private key is never parsed. That is what makes D-11 / KEY-06 work:
// an encrypted private key cannot be parsed without a passphrase, but its public
// half is right there on disk and is authoritative — so reuse succeeds with no
// passphrase prompt. ReadPub is nil-guarded like PubExists on the same line: an
// unwired caller falls back to DerivePub (the previous behavior) instead of
// panicking.
func ensurePubReadOnly(privateKeyPath, pubPath, comment string, deps Deps) (string, error) {
	if deps.PubExists != nil && deps.PubExists(pubPath) {
		// .pub present: return it verbatim — never re-derive, which would parse
		// the private key and fail on an encrypted one (D-11).
		if deps.ReadPub != nil {
			line, err := deps.ReadPub(pubPath)
			if err != nil {
				return "", fmt.Errorf("identity: reading existing public key %s: %w", pubPath, err)
			}
			return line, nil
		}
		// Nil-guard fallback for callers that have not wired ReadPub: keep the
		// previous derive-from-private-key behavior rather than nil-panicking.
		line, err := deps.DerivePub(privateKeyPath, comment)
		if err != nil {
			return "", fmt.Errorf("identity: deriving public key for reuse: %w", err)
		}
		return line, nil
	}

	line, err := deps.DerivePub(privateKeyPath, comment)
	if err != nil {
		return "", fmt.Errorf("identity: deriving missing public key for reuse: %w", err)
	}
	return line, nil
}

// AddAccount orchestrates adding a second account/alias for an already-created
// identity (IDENT-06, D-10 mode 3): it renders a second `Host <newAlias>` block
// and a matching includeIf that SHARE the existing identity's key path, so
// several identities can map to one provider key via distinct aliases. It reuses
// the existing key (no keygen) and runs the shared pipeline; the resolved test
// then confirms `ssh -G <newAlias>` resolves to the same key as the original.
//
// newProvider/newAlias are the user-chosen provider and host alias for the new
// account; the alias must be a distinct gitid-managed Host so it does not collide
// with the existing block.
func AddAccount(existing Account, newProvider, newAlias string, deps Deps) (CreateResult, error) {
	in := CreateInput{
		Name:               existing.Name,
		GitName:            existing.GitName,
		GitEmail:           existing.GitEmail,
		Provider:           newProvider,
		Alias:              newAlias,
		Hostname:           existing.Hostname,
		Port:               existing.Port,
		Matches:            existing.Matches,
		FragmentPath:       fragmentPathFor(existing),
		GitconfigPath:      existing.GitconfigPath,
		SSHConfigPath:      existing.SSHConfigPath,
		AllowedSignersPath: existing.AllowedSignersPath,
		GlobalsGOOS:        "",
	}

	pubLine := "" // derived below if needed for the allowed_signers line
	// Derive the public line from the shared key so the allowed_signers line and
	// previews are populated even though no key is generated.
	if deps.DerivePub != nil {
		line, err := deps.DerivePub(existing.KeyPath, existing.Name+"@gitid")
		if err != nil {
			return CreateResult{}, fmt.Errorf("identity: deriving public key for add-account: %w", err)
		}
		pubLine = line
	}

	// Construct a StagedKey for the existing key: TempPrivatePath ==
	// FinalPrivatePath (gate runs on the real key), PrivPEM nil (no new bytes to
	// persist), so PersistKey and Cleanup are guaranteed no-ops.
	staged := StagedKey{
		TempPrivatePath:  existing.KeyPath,
		FinalPrivatePath: existing.KeyPath,
		FinalPubPath:     existing.PubPath,
		PubLine:          pubLine,
		PrivPEM:          nil,
	}
	return runPipeline(in, staged, deps)
}

// Rotate has moved to rotate.go (this plan's Task 2): it is now a retirement
// ceremony composing the four phases from identity.go directly (archive →
// persist → append-write → resolved), never runPipeline. See rotate.go for
// the full doc comment and rotateInput/algoFromKeyPath for the CreateInput it
// builds.

// fragmentPathFor returns the gitid-managed fragment path for an account: the
// account's persisted FragmentPath when set, otherwise the conventional
// ~/.gitconfig.d/<name> location keyed by identity name.
func fragmentPathFor(a Account) string {
	if a.FragmentPath != "" {
		return a.FragmentPath
	}
	return "~/.gitconfig.d/" + a.Name
}
