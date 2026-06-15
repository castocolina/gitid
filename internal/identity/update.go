package identity

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/castocolina/gitid/internal/gitconfig"
	"github.com/castocolina/gitid/internal/keygen"
	"github.com/castocolina/gitid/internal/sshconfig"
	"github.com/castocolina/gitid/internal/tester"
)

// UpdateDeps holds every external effect Update performs, injected as function
// fields so Update is testable with fakes and reusable by the TUI. It mirrors the
// Deps convention from identity.go.
type UpdateDeps struct {
	WriteSSH            func(accountName, hostBlock, globalBlock string) (string, error)
	WriteGitconfig      func(identity, fragmentPath, allowedSignersPath string, matches []gitconfig.Match) (string, error)
	WriteFragment       func(fragPath, name, email, signingKeyPath string, signing bool) error
	WriteAllowedSigners func(path, identity, line string) (string, error)
	// RemoveAllowedSigners removes the identity's managed block from the
	// allowed_signers file, keyed by identity NAME (symmetric with the
	// block-keyed WriteAllowedSigners). Keying by name rather than email avoids
	// leaving an orphan sentinel and works even when GitEmail is empty
	// (findings #2/#3).
	RemoveAllowedSigners func(path, name string) (string, error)
	Resolved             func(alias string) (tester.Result, tester.ResolvedConfig)
	// ReadPub reads the public key line from a .pub file for building the
	// allowed_signers line when signing is on. When nil, the default implementation
	// reads the file directly via os.ReadFile.
	ReadPub func(pubPath string) (pubLine string, err error)
}

// UpdateResult reports the outcome of an Update call.
type UpdateResult struct {
	// Resolved holds the parsed config from ssh -G after a structural change.
	Resolved tester.ResolvedConfig
	// ResolvedTest holds the connectivity result after a structural change.
	ResolvedTest tester.Result
	// Structural is true when alias/hostname/port changed and a re-test was run.
	Structural bool
	// PreviewOnly is true when no writes were performed (Confirmed was false).
	PreviewOnly bool
}

// Update applies the edited fields to the existing identity, re-renders the four
// artifacts via the safe-write path, and runs the resolved re-test when a
// structural field changed (D-05, D-06). The identity name is immutable (D-04):
// edited.Name is forced to existing.Name before any write. The signing parameter
// controls whether signing keys are written (signing=true) or removed (signing=false).
func Update(existing Account, edited Account, deps UpdateDeps, signing bool) (UpdateResult, error) {
	// D-04: name is immutable — force it back regardless of what the caller supplied.
	edited.Name = existing.Name

	// D-05: structural change detection — alias/hostname/port can change SSH resolution.
	structural := edited.Alias != existing.Alias ||
		edited.Hostname != existing.Hostname ||
		edited.Port != existing.Port

	// Re-render the SSH host block with the (potentially updated) alias/hostname/port/key.
	hostBlock := sshconfig.RenderHostBlock(edited.Alias, edited.Hostname, edited.Port, edited.KeyPath)
	if _, werr := deps.WriteSSH(existing.Name, hostBlock, ""); werr != nil {
		return UpdateResult{}, fmt.Errorf("identity: writing ssh config: %w", werr)
	}

	// Re-render the gitconfig includeIf block with the (potentially updated) matches.
	if _, werr := deps.WriteGitconfig(existing.Name, edited.FragmentPath, edited.AllowedSignersPath, edited.Matches); werr != nil {
		return UpdateResult{}, fmt.Errorf("identity: writing gitconfig includeIf: %w", werr)
	}

	// Handle the signing transition for the allowed_signers file.
	if signing {
		// Signing on: write the allowed_signers line for this identity.
		// Read the pub key line from the .pub file (trusted gitid-managed path).
		readPub := deps.ReadPub
		if readPub == nil {
			readPub = readPubLine
		}
		pubLine, readErr := readPub(edited.PubPath)
		if readErr != nil {
			return UpdateResult{}, fmt.Errorf("identity: reading public key for signing: %w", readErr)
		}
		signersLine := keygen.AllowedSignersLine(edited.GitEmail, pubLine)
		if _, werr := deps.WriteAllowedSigners(edited.AllowedSignersPath, existing.Name, signersLine); werr != nil {
			return UpdateResult{}, fmt.Errorf("identity: writing allowed_signers: %w", werr)
		}
	} else {
		// Signing off: remove the allowed_signers managed block keyed by identity
		// NAME (findings #2/#3). The name is immutable and always known, so this
		// works even when GitEmail is empty, and it removes the whole block (no
		// orphan sentinel) symmetric with the block-keyed writer.
		if _, werr := deps.RemoveAllowedSigners(edited.AllowedSignersPath, existing.Name); werr != nil {
			return UpdateResult{}, fmt.Errorf("identity: removing allowed_signers block: %w", werr)
		}
	}

	// Write the fragment with the (potentially updated) signing state.
	if werr := deps.WriteFragment(edited.FragmentPath, edited.GitName, edited.GitEmail, edited.PubPath, signing); werr != nil {
		return UpdateResult{}, fmt.Errorf("identity: writing gitconfig fragment: %w", werr)
	}

	res := UpdateResult{Structural: structural}

	// D-05: re-run the resolved test only when a structural field changed.
	if structural {
		resolvedTest, resolved := deps.Resolved(edited.Alias)
		res.ResolvedTest = resolvedTest
		res.Resolved = resolved
	}

	return res, nil
}

// readPubLine reads the public key line from a .pub file path. It returns the
// raw file content (the authorized-key line) trimmed of surrounding whitespace.
// The path is a gitid-managed path supplied in-process (trusted, not user input).
//
// A leading "~/" is expanded to the user home first (WR-02): reconstruction
// copies IdentityFile verbatim from ~/.ssh/config, which commonly carries a
// literal tilde that os.ReadFile cannot resolve — without expansion the
// signing-on update path would half-apply (SSH + gitconfig already rewritten)
// then hard-fail here.
func readPubLine(pubPath string) (string, error) {
	resolved, err := expandTilde(pubPath)
	if err != nil {
		return "", fmt.Errorf("resolving pub key path %s: %w", pubPath, err)
	}
	data, err := os.ReadFile(resolved) //nolint:gosec // resolved from a gitid-managed .pub path (G304)
	if err != nil {
		return "", fmt.Errorf("reading pub key %s: %w", resolved, err)
	}
	return strings.TrimRight(string(data), "\n"), nil
}

// expandTilde expands a leading "~/" (or a bare "~") in path to the user home
// directory. Paths without a leading tilde are returned unchanged. It is used
// wherever a reconstructed path (copied verbatim from a user config that may use
// "~") is opened on disk (WR-02).
func expandTilde(path string) (string, error) {
	if path != "~" && !strings.HasPrefix(path, "~/") {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if path == "~" {
		return home, nil
	}
	return filepath.Join(home, path[len("~/"):]), nil
}
