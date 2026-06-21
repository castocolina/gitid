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
	"strings"

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
// interactive prompts and platform defaults. For the create-new flow the
// persist decision is made by the auth-gated loop in cmd/gitid/add.go
// (runCreateLoop), not by a static field here (D-02/D-05/D-06).
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
	WriteFragment       func(fragmentPath, name, email, signingKeyPath string, signing bool) error
	WriteAllowedSigners func(path, identity, line string) (backupPath string, err error)
	Resolved            func(alias string) (tester.Result, tester.ResolvedConfig)

	// PubExists, DerivePub, and WritePub support the reuse-existing-key flow
	// (IDENT-02): PubExists reports whether the existing key's `.pub` is present,
	// DerivePub recomputes the authorized-key line from the private key when it is
	// absent, and WritePub persists the derived line at 0644 via filewriter. They
	// are nil for the create-new flow, which always generates a fresh `.pub`.
	PubExists func(pubPath string) bool
	DerivePub func(privateKeyPath, comment string) (pubLine string, err error)
	WritePub  func(pubPath, pubLine string) error

	// WriteProvisionalSSH, PromoteSSH, and DropProvisionalSSH are the three
	// provisional-block lifecycle seams for the staged create wizard (Plan 14).
	// They mirror the sshconfig.WriteProvisional/Promote/DropProvisional
	// signatures so the TUI can wire live implementations without importing
	// internal/sshconfig directly. Adding these fields is additive; existing
	// callers that do not use the provisional lifecycle leave them nil.
	//
	//   WriteProvisionalSSH — write a provisional Host block (staged key path).
	//   PromoteSSH          — atomic provisional → managed swap (final key path).
	//   DropProvisionalSSH  — remove the provisional block on cancel or failure.
	WriteProvisionalSSH func(name, hostBlock string) (backupPath string, err error)
	PromoteSSH          func(name, hostBlock string) (backupPath string, err error)
	DropProvisionalSSH  func(name string) (backupPath string, err error)
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
	// (dry-run: PreWriteOnly=true).
	PreWriteOnly bool
	Resolved     tester.ResolvedConfig
	ResolvedTest tester.Result

	// Backup paths returned by the four filewriter-backed writers on a confirmed
	// write (empty when no prior file existed or on a preview-only run). Surfaced
	// to the user so a confirmed write reports where each timestamped backup went
	// (CLAUDE.md safe-write invariant, WR-05). Mirrors DeleteResult's backup
	// fields.
	SSHBackup            string
	GitconfigBackup      string
	AllowedSignersBackup string
}

// DefaultHostname returns the recipe-canonical alt-SSH hostname for a provider
// (D-10 parity, T-05.7-09-03). The lookup is case-insensitive and uses the
// leading token before the first "." so both "github" and "github.com" resolve
// to the same endpoint. Recipe sources (recipes/ssh-config.recipe):
//
//	github    -> ssh.github.com       Port 443
//	gitlab    -> altssh.gitlab.com    Port 443
//	bitbucket -> altssh.bitbucket.org Port 443
//
// Unknown providers fall back to "<token>.com" when the token contains no dot,
// or the token verbatim when it already contains a dot, matching the prior cmd
// defaultHostname fallback shape (D-10 parity). UI-free: no charm/bubbletea import.
func DefaultHostname(provider string) string {
	// Extract the leading keyword (the part before the first ".").
	token := strings.ToLower(provider)
	if idx := strings.IndexByte(token, '.'); idx >= 0 {
		token = token[:idx]
	}
	switch token {
	case "github":
		return "ssh.github.com"
	case "gitlab":
		return "altssh.gitlab.com"
	case "bitbucket":
		return "altssh.bitbucket.org"
	default:
		// For unknown providers: if the original provider string contains a dot,
		// return it verbatim; otherwise append ".com" (cmd parity).
		if strings.ContainsRune(strings.ToLower(provider), '.') {
			return strings.ToLower(provider)
		}
		return strings.ToLower(provider) + ".com"
	}
}

// DefaultPort returns the recipe-canonical default SSH port (443) shared by
// all alt-SSH alt-SSH endpoints. The single constant ensures cmd and TUI agree
// on the default without duplicating the literal (D-10 parity).
func DefaultPort() int { return 443 }

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
//	→ render the four artifact previews → write all four → resolved test
//
// The pre-write test gates the write (D-01): a Failure outcome aborts with an
// error and NO writes; PASS or ReachableNotUploaded proceed. All FOUR writers
// run — WriteSSH, WriteGitconfig, WriteFragment, and WriteAllowedSigners — then
// the resolved test captures the live config.
//
// For the create-new flow in the CLI (cmd/gitid/add.go) use the auth-gated loop:
// Generate + runCreateLoop + PersistAll. This function is retained for the TUI
// prove-screen path which drives Create through the injected deps seam.
func Create(in CreateInput, deps Deps) (CreateResult, error) {
	staged, err := deps.Generate(in)
	if err != nil {
		return CreateResult{}, fmt.Errorf("identity: generating key: %w", err)
	}
	defer deps.Cleanup(staged)
	return runPipeline(in, staged, deps)
}

// RenderPreviews builds the four artifact preview strings from CreateInput and
// StagedKey using FINAL paths. It is a pure function — it performs NO writes,
// NO clipboard operations, NO network calls, and NO file reads. Exported so
// cmd/gitid/add.go can render previews for the --dry-run path without calling
// the full pipeline. The SSH host block includes the provider marker comment
// (Plan 02 provider arg, D-11).
func RenderPreviews(in CreateInput, staged StagedKey) CreateResult {
	final := KeyResult{
		PrivatePath: staged.FinalPrivatePath,
		PubPath:     staged.FinalPubPath,
		PubLine:     staged.PubLine,
	}
	signersLine := keygen.AllowedSignersLine(in.GitEmail, staged.PubLine)
	hostBlock := sshconfig.RenderHostBlock(in.Alias, in.Hostname, in.Port, final.PrivatePath, in.Provider)
	gitPreview := gitconfig.RenderIncludeIf(in.Name, in.FragmentPath, in.Matches)
	return CreateResult{
		Key:                   final,
		SSHPreview:            hostBlock,
		GitconfigPreview:      gitPreview,
		FragmentPreview:       renderFragmentPreview(in, final.PubPath),
		AllowedSignersPreview: signersLine,
		AllowedSignersLine:    signersLine,
		PreWriteOnly:          true,
	}
}

// PersistSSH is LEG 1 of the staged write path: it persists the private key
// (when staged.PrivPEM != nil) to staged.FinalPrivatePath BEFORE writing the
// SSH Host block, so the block never references a non-existent key
// (T-05.7-09-04). It does NOT write the gitconfig fragment, includeIf block, or
// allowed_signers, and does NOT run the Resolved test. The returned CreateResult
// carries Key, SSHPreview, and SSHBackup; all gitconfig/signers fields are zero.
//
// Use PersistSSH for the staged wizard to write LEG 1 at the end of the SSH
// screens; follow with PersistGitconfig at the end of the git screen.
func PersistSSH(in CreateInput, staged StagedKey, deps Deps) (CreateResult, error) {
	final := KeyResult{
		PrivatePath: staged.FinalPrivatePath,
		PubPath:     staged.FinalPubPath,
		PubLine:     staged.PubLine,
	}
	hostBlock := sshconfig.RenderHostBlock(in.Alias, in.Hostname, in.Port, final.PrivatePath, in.Provider)
	res := CreateResult{
		Key:        final,
		SSHPreview: hostBlock,
	}

	// Persist the key BEFORE WriteSSH so a persist failure aborts before any
	// config references a non-existent key. Skip when PrivPEM is nil (existing-
	// key reuse/add-account paths — no new key to write).
	if staged.PrivPEM != nil {
		if _, perr := deps.PersistKey(staged); perr != nil {
			return res, fmt.Errorf("identity: persisting key pair: %w", perr)
		}
	}

	sshBak, werr := deps.WriteSSH(in.Name, hostBlock, in.GlobalBlock)
	if werr != nil {
		return res, fmt.Errorf("identity: writing ssh config: %w", werr)
	}
	res.SSHBackup = sshBak
	return res, nil
}

// PersistGitconfig is LEG 2 of the staged write path: it writes the
// ~/.gitconfig.d/<name> fragment, the includeIf block in ~/.gitconfig, and the
// ~/.ssh/allowed_signers entry. It does NOT touch the private key or the SSH
// Host block, and does NOT run the Resolved test. The returned CreateResult
// carries GitconfigPreview, FragmentPreview, AllowedSignersPreview,
// AllowedSignersLine, GitconfigBackup, and AllowedSignersBackup.
//
// Use PersistGitconfig for the staged wizard to write LEG 2 at the end of the
// git-config screen; it is idempotent when called after PersistSSH.
func PersistGitconfig(in CreateInput, staged StagedKey, deps Deps) (CreateResult, error) {
	final := KeyResult{
		PrivatePath: staged.FinalPrivatePath,
		PubPath:     staged.FinalPubPath,
		PubLine:     staged.PubLine,
	}
	signersLine := keygen.AllowedSignersLine(in.GitEmail, staged.PubLine)
	gitPreview := gitconfig.RenderIncludeIf(in.Name, in.FragmentPath, in.Matches)
	res := CreateResult{
		GitconfigPreview:      gitPreview,
		FragmentPreview:       renderFragmentPreview(in, final.PubPath),
		AllowedSignersPreview: signersLine,
		AllowedSignersLine:    signersLine,
	}

	gcBak, werr := deps.WriteGitconfig(in.Name, in.FragmentPath, in.AllowedSignersPath, in.Matches)
	if werr != nil {
		return res, fmt.Errorf("identity: writing gitconfig includeIf: %w", werr)
	}
	res.GitconfigBackup = gcBak
	if werr := deps.WriteFragment(in.FragmentPath, in.GitName, in.GitEmail, final.PubPath, true); werr != nil {
		return res, fmt.Errorf("identity: writing gitconfig fragment: %w", werr)
	}
	signBak, werr := deps.WriteAllowedSigners(in.AllowedSignersPath, in.Name, signersLine)
	if werr != nil {
		return res, fmt.Errorf("identity: writing allowed_signers: %w", werr)
	}
	res.AllowedSignersBackup = signBak
	return res, nil
}

// PersistAll writes the four config artifacts in order: PersistSSH (LEG 1 —
// optionally PersistKey then WriteSSH), then PersistGitconfig (LEG 2 —
// WriteGitconfig, WriteFragment, WriteAllowedSigners), then Resolved. It is a
// THIN COMPOSITION of the two leg-steps so the existing CLI single-shot create
// flow (cmd/gitid/add.go runCreateNew) keeps byte-identical behavior (D-10
// parity, T-05.7-09-02). The merged CreateResult is field-for-field equivalent
// to the prior monolith: same previews, same backups, same Resolved fields.
//
// Exported so cmd/gitid/add.go's runCreateNew can call it after PASS or after
// explicit skip+confirm (D-03/D-05).
func PersistAll(in CreateInput, staged StagedKey, deps Deps) (CreateResult, error) {
	// LEG 1: persist key + write SSH Host block.
	res1, err := PersistSSH(in, staged, deps)
	if err != nil {
		return res1, err
	}

	// LEG 2: write gitconfig fragment + includeIf + allowed_signers.
	res2, err := PersistGitconfig(in, staged, deps)
	if err != nil {
		// Return what LEG 1 produced so the caller knows the SSH block was written.
		return mergeCreateResults(res1, res2), err
	}

	// Merge both legs into the combined result.
	merged := mergeCreateResults(res1, res2)

	// Run the Resolved test and attach it to the merged result.
	resolvedTest, resolved := deps.Resolved(in.Alias)
	merged.ResolvedTest = resolvedTest
	merged.Resolved = resolved
	return merged, nil
}

// mergeCreateResults combines the fields from LEG 1 (res1) and LEG 2 (res2)
// into a single CreateResult. Fields are non-overlapping by design: res1 owns
// Key/SSHPreview/SSHBackup; res2 owns the gitconfig/signers previews/backups.
// ResolvedTest and Resolved are populated by the caller after both legs succeed.
func mergeCreateResults(res1, res2 CreateResult) CreateResult {
	return CreateResult{
		Key:                   res1.Key,
		SSHPreview:            res1.SSHPreview,
		SSHBackup:             res1.SSHBackup,
		GitconfigPreview:      res2.GitconfigPreview,
		GitconfigBackup:       res2.GitconfigBackup,
		FragmentPreview:       res2.FragmentPreview,
		AllowedSignersPreview: res2.AllowedSignersPreview,
		AllowedSignersLine:    res2.AllowedSignersLine,
		AllowedSignersBackup:  res2.AllowedSignersBackup,
	}
}

// runPipeline is the write path for Reuse, AddAccount, and Rotate: copy the
// .pub → build the allowed_signers line → pre-write test (gates the write,
// runs against staged.TempPrivatePath) → render the four artifact previews from
// FINAL paths → persist key (when PrivPEM != nil) → write all four → run the
// resolved test. These modes always write (they are confirmed write paths) so
// there is no static consent gate here — these are always-confirmed write paths.
//
// The pre-write test gates the write: a Failure outcome aborts with an error and
// NO writes; PASS or ReachableNotUploaded proceed. If staged.PrivPEM != nil,
// PersistKey is called FIRST (before the four writers) so a persist failure
// aborts before any config references a non-existent key. Then all FOUR writers
// run — WriteSSH, WriteGitconfig, WriteFragment, WriteAllowedSigners — then the
// resolved test captures the live config.
//
// For the create-new flow, use Generate + RenderPreviews + PersistAll (with the
// auth-gated loop in runCreateLoop in cmd/gitid/add.go).
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
	hostBlock := sshconfig.RenderHostBlock(in.Alias, in.Hostname, in.Port, final.PrivatePath, in.Provider)
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

	// Persist the key BEFORE the four writers so a persist failure aborts
	// before any config references a non-existent key. Skip when PrivPEM is nil
	// (existing-key reuse/add-account paths — no new key to write).
	if staged.PrivPEM != nil {
		if _, perr := deps.PersistKey(staged); perr != nil {
			return res, fmt.Errorf("identity: persisting key pair: %w", perr)
		}
	}

	sshBak, werr := deps.WriteSSH(in.Name, hostBlock, in.GlobalBlock)
	if werr != nil {
		return res, fmt.Errorf("identity: writing ssh config: %w", werr)
	}
	res.SSHBackup = sshBak
	gcBak, werr := deps.WriteGitconfig(in.Name, in.FragmentPath, in.AllowedSignersPath, in.Matches)
	if werr != nil {
		return res, fmt.Errorf("identity: writing gitconfig includeIf: %w", werr)
	}
	res.GitconfigBackup = gcBak
	if werr := deps.WriteFragment(in.FragmentPath, in.GitName, in.GitEmail, final.PubPath, true); werr != nil {
		return res, fmt.Errorf("identity: writing gitconfig fragment: %w", werr)
	}
	signBak, werr := deps.WriteAllowedSigners(in.AllowedSignersPath, in.Name, signersLine)
	if werr != nil {
		return res, fmt.Errorf("identity: writing allowed_signers: %w", werr)
	}
	res.AllowedSignersBackup = signBak

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
