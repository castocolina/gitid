package identity

import (
	"fmt"
	"strings"

	"github.com/castocolina/gitid/internal/sshconfig"
)

// EffectiveAlias returns the alias to use for the SSH Host stanza. When
// aliasField (trimmed) is non-empty it is returned verbatim. When aliasField is
// blank or whitespace-only, the PROVIDER HOST is returned — the canonical SSH
// hostname for the provider (e.g. "github.com") — so the Host line reads
// `Host github.com` for a default identity (WYSIWYG matcher, UAT G-5 fix).
//
// EffectiveAlias never invents an <name>.<provider> suffix (that was
// DefaultAlias, which is appropriate only for additional identities where the
// user explicitly chose a custom alias). When the provider contains a dot it is
// returned lowercased; when it has no dot, ".com" is appended.
//
// This is a pure string helper — no I/O, no charm/bubbletea import.
func EffectiveAlias(aliasField, provider string) string {
	if trimmed := strings.TrimSpace(aliasField); trimmed != "" {
		return trimmed
	}
	// Blank alias → use the provider host (WYSIWYG: blank → `Host github.com`).
	lp := strings.ToLower(provider)
	if strings.ContainsRune(lp, '.') {
		return lp
	}
	return lp + ".com"
}

// PersistSSHProvisional is the PRE-TEST step of the staged create wizard: it
// renders the Host block via sshconfig.RenderHostBlock with the STAGED
// (TempPrivatePath) key as IdentityFile, and writes it under the provisional
// sentinel via deps.WriteProvisionalSSH. It does NOT install the key
// (deps.PersistKey is not called), does NOT write the managed block, does NOT
// write gitconfig, and does NOT call deps.Resolved. The returned CreateResult
// carries SSHPreview (the provisional body) and SSHBackup.
//
// After the caller has tested the alias (ssh -T git@<alias>) and received a
// PASS, it calls PromoteSSH to atomically swap the provisional block into a
// managed block pointing at the final key.
func PersistSSHProvisional(in CreateInput, staged StagedKey, deps Deps) (CreateResult, error) {
	// Render Host block with the STAGED key path — no final key install yet.
	hostBlock := sshconfig.RenderHostBlock(in.Alias, in.Hostname, in.Port, staged.TempPrivatePath, in.Provider)
	res := CreateResult{SSHPreview: hostBlock}

	backupPath, err := deps.WriteProvisionalSSH(in.Name, hostBlock)
	if err != nil {
		return res, fmt.Errorf("identity: writing provisional ssh config: %w", err)
	}
	res.SSHBackup = backupPath
	return res, nil
}

// PromoteSSH is the POST-TEST step of the staged create wizard: it installs the
// key (deps.PersistKey when staged.PrivPEM != nil) FIRST — so a persist failure
// aborts before any config references a non-existent key (T-05.7-09-04 ordering)
// — then promotes the provisional block to a managed block pointing at the FINAL
// key via deps.PromoteSSH. It does NOT call WriteSSH, WriteGitconfig, or
// WriteProvisionalSSH.
//
// The returned CreateResult carries Key, SSHPreview (the managed body rendered
// with FinalPrivatePath), and SSHBackup.
func PromoteSSH(in CreateInput, staged StagedKey, deps Deps) (CreateResult, error) {
	final := KeyResult{
		PrivatePath: staged.FinalPrivatePath,
		PubPath:     staged.FinalPubPath,
		PubLine:     staged.PubLine,
	}

	// Persist the key BEFORE promoting so a persist failure aborts before any
	// config references a non-existent key (mirrors PersistSSH ordering,
	// T-05.7-09-04). Skip when PrivPEM is nil (existing-key reuse path).
	if staged.PrivPEM != nil {
		if _, perr := deps.PersistKey(staged); perr != nil {
			return CreateResult{Key: final}, fmt.Errorf("identity: persisting key pair during promote: %w", perr)
		}
	}

	// Render the managed Host block with the FINAL key path.
	hostBlock := sshconfig.RenderHostBlock(in.Alias, in.Hostname, in.Port, final.PrivatePath, in.Provider)
	res := CreateResult{Key: final, SSHPreview: hostBlock}

	backupPath, err := deps.PromoteSSH(in.Name, hostBlock)
	if err != nil {
		return res, fmt.Errorf("identity: promoting provisional ssh config: %w", err)
	}
	res.SSHBackup = backupPath
	return res, nil
}

// DropProvisionalSSH removes the provisional SSH Host block for in.Name via
// deps.DropProvisionalSSH, returning the backup path. It does NOT delete the
// staged key — staging teardown (Cleanup) is the caller's responsibility.
// Used on wizard cancel or SSH test failure to leave ~/.ssh/config clean.
func DropProvisionalSSH(in CreateInput, deps Deps) (string, error) {
	backupPath, err := deps.DropProvisionalSSH(in.Name)
	if err != nil {
		return "", fmt.Errorf("identity: dropping provisional ssh config: %w", err)
	}
	return backupPath, nil
}
