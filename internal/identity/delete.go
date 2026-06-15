package identity

import (
	"fmt"

	"github.com/castocolina/gitid/internal/filewriter"
)

// DeleteDeps holds every external effect Delete performs, injected as function
// fields so Delete is testable with fakes and reusable by the TUI. It mirrors
// the Deps convention from identity.go.
type DeleteDeps struct {
	// ReadSSH reads the raw bytes of ~/.ssh/config.
	ReadSSH func() ([]byte, error)
	// ReadGitconfig reads the raw bytes of ~/.gitconfig.
	ReadGitconfig func() ([]byte, error)
	// WriteSSH writes the updated ~/.ssh/config bytes and returns a backup path.
	WriteSSH func(content []byte) (backupPath string, err error)
	// WriteGitconfig writes the updated ~/.gitconfig bytes and returns a backup path.
	WriteGitconfig func(content []byte) (backupPath string, err error)
	// RemoveFragment removes the whole per-identity fragment file with backup.
	RemoveFragment func(fragPath string) (backupPath string, err error)
	// RemoveAllowedSigners removes the identity's line from the allowed_signers
	// file (matched by email + namespaces="git"). Returns backup path.
	RemoveAllowedSigners func(path, email string) (backupPath string, err error)
	// RemoveKeyFiles removes the private and public key files. Called only when
	// keepKey is false (irreversible — D-07).
	RemoveKeyFiles func(keyPath, pubPath string) error
}

// DeleteResult holds the backup paths produced by Delete.
type DeleteResult struct {
	SSHBackup            string
	GitconfigBackup      string
	FragmentBackup       string
	AllowedSignersBackup string
}

// Delete removes the four per-identity artifacts (SSH Host block, includeIf
// block, fragment file, allowed_signers line) with backup via the injected
// DeleteDeps. When keepKey is false, a separate deps.RemoveKeyFiles call
// removes the private and public key files (irreversible — D-07). Shared /
// global blocks (e.g. the macOS "_global" SSH block and the global signing
// wiring) are NEVER touched — only acct.Name is passed to RemoveBlock (D-08).
// RemoveBlock is idempotent: if the block is already absent the file is
// returned unchanged (no error).
func Delete(acct Account, keepKey bool, deps DeleteDeps) (DeleteResult, error) {
	var res DeleteResult

	// Read the SSH config, remove ONLY the per-identity block, write it back.
	sshBytes, err := deps.ReadSSH()
	if err != nil {
		return res, fmt.Errorf("identity: reading ssh config: %w", err)
	}
	updatedSSH := filewriter.RemoveBlock(sshBytes, acct.Name)
	sshBak, err := deps.WriteSSH(updatedSSH)
	if err != nil {
		return res, fmt.Errorf("identity: removing ssh block: %w", err)
	}
	res.SSHBackup = sshBak

	// Read the gitconfig, remove ONLY the per-identity includeIf block, write it back.
	gcBytes, err := deps.ReadGitconfig()
	if err != nil {
		return res, fmt.Errorf("identity: reading gitconfig: %w", err)
	}
	updatedGC := filewriter.RemoveBlock(gcBytes, acct.Name)
	gcBak, err := deps.WriteGitconfig(updatedGC)
	if err != nil {
		return res, fmt.Errorf("identity: removing gitconfig block: %w", err)
	}
	res.GitconfigBackup = gcBak

	// Remove the whole fragment file (whole-file backup+remove).
	fragBak, err := deps.RemoveFragment(acct.FragmentPath)
	if err != nil {
		return res, fmt.Errorf("identity: removing fragment file: %w", err)
	}
	res.FragmentBackup = fragBak

	// Remove the allowed_signers line matched by email + namespaces="git".
	signBak, err := deps.RemoveAllowedSigners(acct.AllowedSignersPath, acct.GitEmail)
	if err != nil {
		return res, fmt.Errorf("identity: removing allowed_signers line: %w", err)
	}
	res.AllowedSignersBackup = signBak

	// Key deletion is intentionally gated behind keepKey=false (D-07).
	// Block/file removals above are reversible (backups exist); key deletion is not.
	if !keepKey {
		if kerr := deps.RemoveKeyFiles(acct.KeyPath, acct.PubPath); kerr != nil {
			return res, fmt.Errorf("identity: removing key files: %w", kerr)
		}
	}

	return res, nil
}
