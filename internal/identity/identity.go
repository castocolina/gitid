// Package identity provides the domain model and create-new orchestration for
// gitid: the Account type, the CreateInput gathered from the user, and the
// Create orchestration that coordinates the four coordinated writes (SSH config,
// gitconfig includeIf, per-identity fragment, and ~/.ssh/allowed_signers).
//
// Every external effect (key generation, clipboard, connectivity tests, file
// writes) is an injected dependency on the Deps struct, so Create is fully
// testable with fakes and reusable by the future TUI. No business logic lives in
// cmd/ — the Cobra handlers gather input, build Deps from the real internal
// packages, and call Create.
package identity

import (
	"fmt"

	"github.com/castocolina/gitid/internal/gitconfig"
	"github.com/castocolina/gitid/internal/keygen"
	"github.com/castocolina/gitid/internal/sshconfig"
	"github.com/castocolina/gitid/internal/tester"
)

// Account is the persisted shape of a gitid identity, reconstructable from the
// managed blocks across ~/.ssh/config and ~/.gitconfig. The filesystem is the
// source of truth; this struct is the in-memory translation.
type Account struct {
	Name     string
	GitName  string
	GitEmail string
	Provider string
	Alias    string
	Hostname string
	Port     int
	KeyPath  string
	PubPath  string
	Matches  []gitconfig.Match

	// Gitid-managed target paths the lifecycle modes (rotate/add-account) write
	// to. They mirror the CreateInput target fields and are filled by the command
	// layer from platform defaults when an account is loaded.
	FragmentPath       string
	GitconfigPath      string
	SSHConfigPath      string
	AllowedSignersPath string

	// Incomplete is non-empty when reconstruction found this identity name in
	// some but not all four artifacts (D-02). It names the missing pieces
	// (comma-separated) for display in `gitid identity list`. Deep diagnosis
	// stays in Phase 4 doctor.
	Incomplete string
}

// CreateInput carries the user-gathered inputs plus the resolved gitid-managed
// paths the create-new flow writes to. The command layer fills it from
// interactive prompts and platform defaults; Confirmed gates the actual writes
// (false == preview/dry-run only, SAFE-03).
type CreateInput struct {
	Name     string
	GitName  string
	GitEmail string
	Provider string
	Algo     string
	Alias    string
	Hostname string
	Port     int
	// Passphrase, when non-empty, encrypts the generated private key (D-07).
	Passphrase string
	Matches    []gitconfig.Match

	// Gitid-managed target paths (supplied in-process, trusted).
	FragmentPath       string
	GitconfigPath      string
	SSHConfigPath      string
	AllowedSignersPath string

	// GlobalBlock is the rendered macOS `Host *` block body (empty off darwin).
	GlobalBlock string

	// Confirmed is the single explicit user consent. When false, Create renders
	// the previews and returns without performing any write (SAFE-03 / --dry-run).
	Confirmed bool
}

// KeyResult is the subset of keygen.Result the orchestration needs, decoupling
// Create from the concrete keygen package so the Generate dep stays fakeable.
type KeyResult struct {
	PrivatePath string
	PubPath     string
	PubLine     string
}

// StagedKey carries the in-memory state produced by the Generate dep for the
// GENERATE paths (create-new, Rotate). TempPrivatePath is the hermetic staging
// path where PrivPEM was written for the pre-write gate; FinalPrivatePath and
// FinalPubPath are the ~/.ssh destination paths. PrivPEM holds the private-key
// bytes in memory so PersistKey can write the final file without re-reading the
// temp (no gosec G304). PrivPEM is private key material — it must never be
// logged or printed. For existing-key paths (Reuse/AddAccount) PrivPEM is nil,
// which is the sentinel meaning "existing key, nothing to persist".
type StagedKey struct {
	// TempPrivatePath is the hermetic temp path used for the pre-write gate.
	// For existing-key paths (Reuse/AddAccount) it equals FinalPrivatePath.
	TempPrivatePath string
	// FinalPrivatePath is the ~/.ssh destination for the private key.
	FinalPrivatePath string
	// FinalPubPath is the ~/.ssh destination for the public key (.pub sibling).
	FinalPubPath string
	// PubLine is the authorized-key line ("ssh-ed25519 AAAA…\n").
	PubLine string
	// PrivPEM holds the private-key bytes generated in memory. It is nil for
	// existing-key paths (Reuse/AddAccount). NEVER log or print this field.
	PrivPEM []byte
}

// Deps holds every external effect Create performs, injected as function fields
// so Create is testable with fakes and reusable by the TUI. The FOUR writers —
// WriteSSH, WriteGitconfig, WriteFragment, WriteAllowedSigners — persist the
// four coordinated artifacts; WriteAllowedSigners is the fourth writer that
// makes SIGN-01 real (the signing line is written, not merely generated).
type Deps struct {
	// Generate generates key material AND stages the private key to a hermetic
	// temp location for the pre-write gate. It returns a StagedKey carrying
	// PrivPEM, TempPrivatePath (for the gate), and the FINAL ~/.ssh paths.
	Generate func(in CreateInput) (StagedKey, error)
	// PersistKey writes staged.PrivPEM to the final private-key and public-key
	// paths via filewriter (backup+atomic+chmod 0600/0644). When staged.PrivPEM
	// is nil (existing-key paths) it is a guaranteed no-op.
	PersistKey func(s StagedKey) (KeyResult, error)
	// Cleanup removes the hermetic temp staging directory created by Generate.
	// For existing-key paths (TempPrivatePath == FinalPrivatePath / PrivPEM nil)
	// it must not delete anything.
	Cleanup             func(s StagedKey)
	CopyPub             func(pubLine string) error
	PreWrite            func(keyPath, hostname string, port int) tester.Result
	WriteSSH            func(accountName, hostBlock, globalBlock string) (backupPath string, err error)
	WriteGitconfig      func(identity, fragmentPath, allowedSignersPath string, matches []gitconfig.Match) (backupPath string, err error)
	WriteFragment       func(fragmentPath, name, email, signingKeyPath string) error
	WriteAllowedSigners func(path, identity, line string) (backupPath string, err error)
	Resolved            func(alias string) (tester.Result, tester.ResolvedConfig)

	// PubExists, DerivePub, and WritePub support the reuse-existing-key flow
	// (IDENT-02): PubExists reports whether the existing key's `.pub` is present,
	// DerivePub recomputes the authorized-key line from the private key when it is
	// absent, and WritePub persists the derived line at 0644 via filewriter. They
	// are nil for the create-new flow, which always generates a fresh `.pub`.
	PubExists func(pubPath string) bool
	DerivePub func(privateKeyPath string) (pubLine string, err error)
	WritePub  func(pubPath, pubLine string) error
}

// CreateResult reports everything the command layer needs to display: the four
// rendered artifact previews, both test results, the resolved config, and
// whether the run stopped at preview (no write performed).
type CreateResult struct {
	Key                   KeyResult
	PreWrite              tester.Result
	SSHPreview            string
	GitconfigPreview      string
	FragmentPreview       string
	AllowedSignersPreview string
	AllowedSignersLine    string
	// PreWriteOnly is true when the run produced previews but performed no write
	// (Confirmed was false / dry-run).
	PreWriteOnly bool
	Resolved     tester.ResolvedConfig
	ResolvedTest tester.Result
}

// DefaultAlias renders the recommended alias form <identity>.<provider> (D-12).
func DefaultAlias(identity, provider string) string {
	return identity + "." + provider
}

// DefaultMatch renders the default gitdir match strategy
// gitdir:~/git/<identity>/ with the mandatory trailing slash (D-13).
func DefaultMatch(identity string) gitconfig.Match {
	return gitconfig.Match{Kind: gitconfig.MatchGitdir, Value: "~/git/" + identity + "/"}
}

// Create orchestrates the create-new identity flow with all effects injected:
//
//	generate key → copy .pub → build allowed_signers line → pre-write test
//	→ render the four artifact previews → (Confirmed?) write all four → resolved test
//
// The pre-write test gates the write (D-01): a Failure outcome aborts with an
// error and NO writes; PASS or ReachableNotUploaded proceed. When Confirmed is
// false the previews are returned with PreWriteOnly set and no write or resolved
// test runs (SAFE-03 / --dry-run). On a confirmed write all FOUR writers run —
// WriteSSH, WriteGitconfig, WriteFragment, and WriteAllowedSigners — then the
// resolved test captures the live config.
func Create(in CreateInput, deps Deps) (CreateResult, error) {
	staged, err := deps.Generate(in)
	if err != nil {
		return CreateResult{}, fmt.Errorf("identity: generating key: %w", err)
	}
	defer deps.Cleanup(staged)
	return runPipeline(in, staged, deps)
}

// runPipeline is the single write path shared by Create, Rotate, and the reuse
// flow: copy the .pub → build the allowed_signers line → pre-write test (gates
// the write, D-01, runs against staged.TempPrivatePath) → render the four
// artifact previews from FINAL paths → (Confirmed? persist key FIRST, then)
// write all four → run the resolved test. Both the create-new, rotate, and
// reuse-existing modes funnel through here so there is exactly one writer
// sequence (no parallel write path).
//
// The pre-write test gates the write: a Failure outcome aborts with an error and
// NO writes; PASS or ReachableNotUploaded proceed. When Confirmed is false the
// previews are returned with PreWriteOnly set and no write or resolved test runs
// (SAFE-03 / --dry-run). On a confirmed write: if staged.PrivPEM != nil,
// PersistKey is called FIRST (before the four writers) so a persist failure
// aborts before any config references a non-existent key. Then all FOUR writers
// run — WriteSSH, WriteGitconfig, WriteFragment, WriteAllowedSigners — then the
// resolved test captures the live config.
func runPipeline(in CreateInput, staged StagedKey, deps Deps) (CreateResult, error) {
	if cerr := deps.CopyPub(staged.PubLine); cerr != nil {
		// Clipboard is best-effort (CLIP-02): a copy failure never aborts the
		// flow; the command layer prints the key for manual copy.
		_ = cerr
	}

	signersLine := keygen.AllowedSignersLine(in.GitEmail, staged.PubLine)

	// Gate on the TEMP path (BUG-4: pre-write test must use the staged key so
	// it runs before any ~/.ssh write; for existing-key paths TempPrivatePath ==
	// FinalPrivatePath so behavior is unchanged).
	pre := deps.PreWrite(staged.TempPrivatePath, in.Hostname, in.Port)
	if pre.Outcome == tester.Failure {
		return CreateResult{PreWrite: pre}, fmt.Errorf(
			"identity: pre-write connectivity test failed for %q, aborting before any write:\n%s\n%s",
			in.Alias, pre.Command, pre.Output)
	}

	// Render previews using the FINAL paths, never the temp path.
	final := KeyResult{
		PrivatePath: staged.FinalPrivatePath,
		PubPath:     staged.FinalPubPath,
		PubLine:     staged.PubLine,
	}
	hostBlock := sshconfig.RenderHostBlock(in.Alias, in.Hostname, in.Port, final.PrivatePath)
	gitPreview := gitconfig.RenderIncludeIf(in.Name, in.FragmentPath, in.Matches)

	res := CreateResult{
		Key:                   final,
		PreWrite:              pre,
		SSHPreview:            hostBlock,
		GitconfigPreview:      gitPreview,
		FragmentPreview:       renderFragmentPreview(in, final.PubPath),
		AllowedSignersPreview: signersLine,
		AllowedSignersLine:    signersLine,
	}

	if !in.Confirmed {
		res.PreWriteOnly = true
		return res, nil
	}

	// Persist the key BEFORE the four writers so a persist failure aborts
	// before any config references a non-existent key. Skip when PrivPEM is nil
	// (existing-key reuse/add-account paths — no new key to write).
	if staged.PrivPEM != nil {
		if _, perr := deps.PersistKey(staged); perr != nil {
			return res, fmt.Errorf("identity: persisting key pair: %w", perr)
		}
	}

	if _, werr := deps.WriteSSH(in.Name, hostBlock, in.GlobalBlock); werr != nil {
		return res, fmt.Errorf("identity: writing ssh config: %w", werr)
	}
	if _, werr := deps.WriteGitconfig(in.Name, in.FragmentPath, in.AllowedSignersPath, in.Matches); werr != nil {
		return res, fmt.Errorf("identity: writing gitconfig includeIf: %w", werr)
	}
	if werr := deps.WriteFragment(in.FragmentPath, in.GitName, in.GitEmail, final.PubPath); werr != nil {
		return res, fmt.Errorf("identity: writing gitconfig fragment: %w", werr)
	}
	if _, werr := deps.WriteAllowedSigners(in.AllowedSignersPath, in.Name, signersLine); werr != nil {
		return res, fmt.Errorf("identity: writing allowed_signers: %w", werr)
	}

	resolvedTest, resolved := deps.Resolved(in.Alias)
	res.ResolvedTest = resolvedTest
	res.Resolved = resolved
	return res, nil
}

// renderFragmentPreview describes the per-identity fragment keys for the unified
// preview. The fragment itself is written by git config (WriteFragment); this is
// the human-readable summary of the values that will be set (SIGN-02: signing key
// is the .pub PATH, never an inline key).
func renderFragmentPreview(in CreateInput, pubPath string) string {
	return fmt.Sprintf(
		"[%s fragment]\n  user.name       = %s\n  user.email      = %s\n  gpg.format      = ssh\n  user.signingkey = %s\n  commit.gpgsign  = true\n",
		in.FragmentPath, in.GitName, in.GitEmail, pubPath)
}
