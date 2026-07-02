package sshconfig

import (
	"fmt"
	"os"

	"github.com/castocolina/gitid/internal/filewriter"
)

// globalBlockName is the fixed sentinel key for the macOS `Host *` managed
// block. Keying it separately from per-identity blocks lets the writer rewrite
// it idempotently and always append it LAST, after every specific host block,
// so first-match-wins resolution keeps the aliases authoritative (Pitfall 5 /
// T-02-15).
const globalBlockName = "_global"

// configMode is the restrictive mode for ~/.ssh/config. The file can reference
// private-key paths, so it is never world-readable (T-02-01).
const configMode os.FileMode = 0o600

// Write composes the gitid-managed SSH config blocks into configPath and writes
// the result atomically through the filewriter chokepoint.
//
// It reads the existing config (treating a missing file as empty), replaces the
// managed block keyed by accountName with hostBlock, then replaces the
// `_global` block with globalBlock so the wildcard stanza always lands after
// the specific host blocks. The composition delegates every byte of foreign
// (hand-written) content untouched to filewriter.ReplaceBlock, which splices
// only the sentinel-delimited range (SAFE-02, T-02-17).
//
// When globalBlock is empty (non-darwin) no global block is written and any
// previously written `_global` block is left in place rather than emptied.
//
// The composed bytes are validated with a second Parse pass (parse -> compose
// -> parse) so a render that would not round-trip is caught before the write.
// The actual write goes through filewriter.Write (atomic temp -> rename, 0600,
// timestamped backup) — this package never writes the config file directly
// (T-02-16).
//
// backupPath is non-empty only when configPath pre-existed.
func Write(configPath, accountName, hostBlock, globalBlock string) (backupPath string, err error) {
	existing, err := os.ReadFile(configPath) //nolint:gosec // configPath is a trusted gitid-managed path supplied in-process
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("reading ssh config %s: %w", configPath, err)
	}

	composed := filewriter.ReplaceBlock(existing, accountName, hostBlock)
	if globalBlock != "" {
		composed = filewriter.ReplaceBlock(composed, globalBlockName, globalBlock)
	}

	// Round-trip safety: the composed config must parse cleanly before we
	// commit it to disk (parse -> compose -> parse stability).
	if _, perr := Parse(composed); perr != nil {
		return "", fmt.Errorf("composed ssh config is not parseable, refusing to write: %w", perr)
	}

	backupPath, err = filewriter.Write(configPath, composed, configMode)
	if err != nil {
		return "", fmt.Errorf("writing ssh config %s: %w", configPath, err)
	}
	return backupPath, nil
}
