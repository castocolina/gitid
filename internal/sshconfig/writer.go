package sshconfig

import (
	"fmt"
	"os"

	"github.com/castocolina/gitid/internal/filewriter"
)

// configMode is the restrictive mode for ~/.ssh/config. The file can reference
// private-key paths, so it is never world-readable (T-02-01).
const configMode os.FileMode = 0o600

// Write composes the gitid-managed SSH config blocks into configPath and writes
// the result atomically through the filewriter chokepoint.
//
// It reads the existing config (treating a missing file as empty), replaces the
// managed block keyed by accountName with hostBlock, and — when the platform
// argument is non-empty — normalises the gitid `Host *` globals block through
// EnsureGlobals. The composition delegates every byte of foreign (hand-written)
// content untouched to filewriter.ReplaceBlock, which splices only the
// sentinel-delimited range (SAFE-02, T-02-17).
//
// globalsGOOS is a PLATFORM token whose meaning replaces the retired "empty
// block leaves the previous one in place" contract:
//
//   - a NON-EMPTY value means "normalise the globals block through
//     sshconfig.EnsureGlobals with no explicit overlay" — the single owner of
//     the gitid `Host *` block (D-06) — using that platform's defaults;
//   - an EMPTY value means "do not touch the globals block at all". The
//     rotate/repair/update ceremonies pass the empty value because their write
//     must not re-normalise a block they did not change.
//
// The composed bytes are validated with a second Parse pass (parse -> compose
// -> parse) so a render that would not round-trip is caught before the write.
// The actual write goes through filewriter.Write (atomic temp -> rename, 0600,
// timestamped backup) — this package never writes the config file directly
// (T-02-16).
//
// backupPath is non-empty only when configPath pre-existed.
func Write(configPath, accountName, hostBlock, globalsGOOS string) (backupPath string, err error) {
	existing, err := os.ReadFile(configPath) //nolint:gosec // configPath is a trusted gitid-managed path supplied in-process
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("reading ssh config %s: %w", configPath, err)
	}

	composed := filewriter.ReplaceBlock(existing, accountName, hostBlock)
	if globalsGOOS != "" {
		composed, err = EnsureGlobals(composed, nil, globalsGOOS)
		if err != nil {
			return "", fmt.Errorf("composing the globals block for %s: %w", configPath, err)
		}
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
