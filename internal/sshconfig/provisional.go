package sshconfig

import (
	"fmt"
	"os"

	"github.com/castocolina/gitid/internal/filewriter"
)

// WriteProvisional composes the existing SSH config with a PROVISIONAL block
// for name set to hostBlock (via filewriter.ReplaceProvisionalBlock), validates
// the composed bytes parse cleanly (parse round-trip guard, same as Write), then
// writes via filewriter.Write (atomic + 0600 + timestamped backup). Returns
// backupPath (non-empty when the config pre-existed).
//
// hostBlock is the BODY rendered by sshconfig.RenderHostBlock with the STAGED
// (temp) key path as IdentityFile; the caller supplies it. WriteProvisional does
// NOT touch the managed block or the _global block.
//
// On a parse failure of the composed config, WriteProvisional returns an error
// and does NOT write — refuse-to-corrupt invariant (T-05.7-14-02).
func WriteProvisional(configPath, name, hostBlock string) (backupPath string, err error) {
	existing, err := os.ReadFile(configPath) //nolint:gosec // configPath is a trusted gitid-managed path
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("reading ssh config %s: %w", configPath, err)
	}

	composed := filewriter.ReplaceProvisionalBlock(existing, name, hostBlock)

	// Round-trip safety: the composed config must parse cleanly before we
	// commit it to disk (parse -> compose -> parse stability, T-05.7-14-02).
	if _, perr := Parse(composed); perr != nil {
		return "", fmt.Errorf("composed ssh config (provisional) is not parseable, refusing to write: %w", perr)
	}

	backupPath, err = filewriter.Write(configPath, composed, configMode)
	if err != nil {
		return "", fmt.Errorf("writing ssh config %s (provisional): %w", configPath, err)
	}
	return backupPath, nil
}

// Promote atomically swaps the provisional block for name into a managed block
// in ONE composed write (T-05.7-14-05). It removes the provisional block for
// name (filewriter.RemoveProvisionalBlock) AND sets the managed block for name
// to managedHostBlock (filewriter.ReplaceBlock) on the composed bytes, so the
// result has the managed block and NO provisional block. Then it validates the
// composed bytes parse cleanly, and writes via filewriter.Write (backup). Returns
// backupPath.
//
// managedHostBlock is the BODY rendered by sshconfig.RenderHostBlock with the
// FINAL key path as IdentityFile; the caller supplies it. Promote does NOT touch
// the _global block — the normal managed Write path owns that.
//
// On a parse failure of the composed config, Promote returns an error and does
// NOT write (T-05.7-14-02).
func Promote(configPath, name, managedHostBlock string) (backupPath string, err error) {
	existing, err := os.ReadFile(configPath) //nolint:gosec // configPath is a trusted gitid-managed path
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("reading ssh config %s: %w", configPath, err)
	}

	// Compose: remove provisional sentinel THEN set managed block — single
	// transaction (atomic when committed via filewriter.Write).
	composed := filewriter.RemoveProvisionalBlock(existing, name)
	composed = filewriter.ReplaceBlock(composed, name, managedHostBlock)

	// Round-trip safety (T-05.7-14-02).
	if _, perr := Parse(composed); perr != nil {
		return "", fmt.Errorf("composed ssh config (promote) is not parseable, refusing to write: %w", perr)
	}

	backupPath, err = filewriter.Write(configPath, composed, configMode)
	if err != nil {
		return "", fmt.Errorf("writing ssh config %s (promote): %w", configPath, err)
	}
	return backupPath, nil
}

// DropProvisional removes ONLY the provisional block for name (via
// filewriter.RemoveProvisionalBlock), validates the composed bytes parse
// cleanly, then writes via filewriter.Write (backup). Returns backupPath.
//
// Idempotent: when no provisional block exists for name, the content is written
// back unchanged (still backs up the pre-existing file). A missing config file
// returns ("", nil) — idempotent (T-05.7-14-02, T-05.7-14-03).
func DropProvisional(configPath, name string) (backupPath string, err error) {
	existing, err := os.ReadFile(configPath) //nolint:gosec // configPath is a trusted gitid-managed path
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("reading ssh config %s: %w", configPath, err)
	}

	composed := filewriter.RemoveProvisionalBlock(existing, name)

	// Round-trip safety (T-05.7-14-02).
	if _, perr := Parse(composed); perr != nil {
		return "", fmt.Errorf("composed ssh config (drop-provisional) is not parseable, refusing to write: %w", perr)
	}

	backupPath, err = filewriter.Write(configPath, composed, configMode)
	if err != nil {
		return "", fmt.Errorf("writing ssh config %s (drop-provisional): %w", configPath, err)
	}
	return backupPath, nil
}

// ListProvisional returns the provisional block names from content in file
// order (via filewriter.ListProvisionalBlocks). Managed blocks are NOT returned
// (mutually exclusive sentinels, T-05.7-14-01). Used by the wizard bootstrap
// and doctor to enumerate in-flight provisional blocks.
func ListProvisional(content []byte) []string {
	blocks := filewriter.ListProvisionalBlocks(content)
	if len(blocks) == 0 {
		return nil
	}
	names := make([]string, len(blocks))
	for i, b := range blocks {
		names[i] = b.Name
	}
	return names
}
