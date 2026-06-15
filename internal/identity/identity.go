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

// Deps holds every external effect Create performs, injected as function fields
// so Create is testable with fakes and reusable by the TUI. The FOUR writers —
// WriteSSH, WriteGitconfig, WriteFragment, WriteAllowedSigners — persist the
// four coordinated artifacts; WriteAllowedSigners is the fourth writer that
// makes SIGN-01 real (the signing line is written, not merely generated).
type Deps struct {
	Generate            func(in CreateInput) (KeyResult, error)
	CopyPub             func(pubLine string) error
	PreWrite            func(keyPath, host string) tester.Result
	WriteSSH            func(accountName, hostBlock, globalBlock string) (backupPath string, err error)
	WriteGitconfig      func(identity, fragmentPath, allowedSignersPath string, matches []gitconfig.Match) (backupPath string, err error)
	WriteFragment       func(fragmentPath, name, email, signingKeyPath string) error
	WriteAllowedSigners func(path, identity, line string) (backupPath string, err error)
	Resolved            func(alias string) (tester.Result, tester.ResolvedConfig)
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
	key, err := deps.Generate(in)
	if err != nil {
		return CreateResult{}, fmt.Errorf("identity: generating key: %w", err)
	}

	if cerr := deps.CopyPub(key.PubLine); cerr != nil {
		// Clipboard is best-effort (CLIP-02): a copy failure never aborts the
		// flow; the command layer prints the key for manual copy.
		_ = cerr
	}

	signersLine := keygen.AllowedSignersLine(in.GitEmail, key.PubLine)

	pre := deps.PreWrite(key.PrivatePath, in.Alias)
	if pre.Outcome == tester.Failure {
		return CreateResult{PreWrite: pre}, fmt.Errorf(
			"identity: pre-write connectivity test failed for %q, aborting before any write:\n%s\n%s",
			in.Alias, pre.Command, pre.Output)
	}

	hostBlock := sshconfig.RenderHostBlock(in.Alias, in.Hostname, in.Port, key.PrivatePath)
	gitPreview := gitconfig.RenderIncludeIf(in.Name, in.FragmentPath, in.Matches)

	res := CreateResult{
		Key:                   key,
		PreWrite:              pre,
		SSHPreview:            hostBlock,
		GitconfigPreview:      gitPreview,
		FragmentPreview:       renderFragmentPreview(in, key.PubPath),
		AllowedSignersPreview: signersLine,
		AllowedSignersLine:    signersLine,
	}

	if !in.Confirmed {
		res.PreWriteOnly = true
		return res, nil
	}

	if _, werr := deps.WriteSSH(in.Name, hostBlock, in.GlobalBlock); werr != nil {
		return res, fmt.Errorf("identity: writing ssh config: %w", werr)
	}
	if _, werr := deps.WriteGitconfig(in.Name, in.FragmentPath, in.AllowedSignersPath, in.Matches); werr != nil {
		return res, fmt.Errorf("identity: writing gitconfig includeIf: %w", werr)
	}
	if werr := deps.WriteFragment(in.FragmentPath, in.GitName, in.GitEmail, key.PubPath); werr != nil {
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
