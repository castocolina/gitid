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
	pubPath := existingKeyPath + ".pub"

	pubLine, err := ensurePub(existingKeyPath, pubPath, in.Name+"@gitid", deps)
	if err != nil {
		return CreateResult{}, err
	}

	// Construct a StagedKey for the existing key: TempPrivatePath ==
	// FinalPrivatePath (gate runs on the real key), PrivPEM nil (no new bytes to
	// persist), so PersistKey and Cleanup are guaranteed no-ops.
	staged := StagedKey{
		TempPrivatePath:  existingKeyPath,
		FinalPrivatePath: existingKeyPath,
		FinalPubPath:     pubPath,
		PubLine:          pubLine,
		PrivPEM:          nil,
	}
	return runPipeline(in, staged, deps)
}

// ensurePub returns the reused identity's public-key line, deriving and writing
// it (0644 via the WritePub dep) when the existing `.pub` file is absent. When
// the `.pub` already exists it is read back via DerivePub so the returned line
// always reflects the on-disk private key, keeping the allowed_signers line and
// the pipeline's PubLine consistent.
func ensurePub(privateKeyPath, pubPath, comment string, deps Deps) (string, error) {
	if deps.PubExists != nil && deps.PubExists(pubPath) {
		// .pub present: derive from the private key so the returned line is
		// guaranteed to match the key actually in use (the existing .pub may be
		// stale or for a different key).
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
	if werr := deps.WritePub(pubPath, line); werr != nil {
		return "", fmt.Errorf("identity: writing derived public key %s: %w", pubPath, werr)
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
		GlobalBlock:        "",
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

// Rotate orchestrates replacing the key for an existing identity (KEY-01, D-11
// fast-follow): it generates a fresh key via the injected Generate dep, then
// re-points ALL FOUR managed artifacts to the new key by running the SAME shared
// pipeline keyed by the identity's existing name. Because every writer splices
// its managed block via filewriter.ReplaceBlock keyed by identity name, the old
// key references are REPLACED, not duplicated (T-02-29, SAFE-02), and each
// mutated file is backed up first by the filewriter chokepoint (SAFE-01). After
// the write the two-phase resolved test re-runs against the new key.
//
// Confirmation (SAFE-03) is gathered by the command layer before Rotate is
// called; Rotate uses runPipeline which always writes (consented by the caller).
func Rotate(existing Account, deps Deps) (CreateResult, error) {
	in := rotateInput(existing)

	staged, err := deps.Generate(in)
	if err != nil {
		return CreateResult{}, fmt.Errorf("identity: generating rotation key: %w", err)
	}
	defer deps.Cleanup(staged)
	return runPipeline(in, staged, deps)
}

// rotateInput builds the CreateInput that re-points an existing account's four
// artifacts. It carries the SAME identity name, alias, matches, and managed
// target paths as the account so every ReplaceBlock rewrite targets the existing
// managed block (replacing the old key references in place).
func rotateInput(a Account) CreateInput {
	return CreateInput{
		Name:               a.Name,
		GitName:            a.GitName,
		GitEmail:           a.GitEmail,
		Provider:           a.Provider,
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

// fragmentPathFor returns the gitid-managed fragment path for an account: the
// account's persisted FragmentPath when set, otherwise the conventional
// ~/.gitconfig.d/<name> location keyed by identity name.
func fragmentPathFor(a Account) string {
	if a.FragmentPath != "" {
		return a.FragmentPath
	}
	return "~/.gitconfig.d/" + a.Name
}
