package sshconfig

import (
	"fmt"
	"os"

	"github.com/castocolina/gitid/internal/filewriter"
)

// sshIncludeBlockName is the reserved, non-identity managed block name for the
// gitid-owned SSH Include line. It has no per-identity Host block and no
// gitconfig counterpart by design — IsReservedBlockName lets identity
// discovery and the doctor Orphans check exclude it, mirroring
// gitconfig.IsReservedBlockName (Pitfall 4).
const sshIncludeBlockName = "ssh-include"

// sshIncludeLineBody is the gitid-owned Include line floored at the top of
// ~/.ssh/config (STORE-01). It pulls in every file matched by the config.d
// glob below, first-match-wins ahead of any later hand-written Host block.
//
// canonical config.d glob — keep in sync with internal/identity/inventory.go
// (mirrored, not shared, to preserve Wave-1 independence — ACCEPTED
// DUPLICATION, MEDIUM #4 option b; see 01-03-PLAN.md objective). The literal
// is "config.d/*.config"; it MUST NOT be extracted into a shared exported
// constant, or 01-04 would depend_on 01-03 and force a re-wave of the DAG.
const sshIncludeLineBody = "Include ~/.ssh/config.d/*.config"

// includeDirMode / includeFileMode are the restrictive permission bits for the
// Include'd storage layout: the config.d directory is never world-readable
// (0700) and its files may reference private-key paths (0600) — never relying
// on the process umask (STORE-01).
const (
	includeDirMode  os.FileMode = 0o700
	includeFileMode os.FileMode = 0o600
)

// IsReservedBlockName reports whether a gitid-managed SSH block name is a
// reserved, non-identity block (currently only the Include line). Mirrors
// gitconfig.IsReservedBlockName exactly, so identity discovery and the doctor
// Orphans check can exclude it the same way the gitconfig side already does
// (Pitfall 4 / project memory "Doctor reserved-block false-positive loop").
func IsReservedBlockName(name string) bool {
	return name == sshIncludeBlockName
}

// EnsureIncludeDir creates configDir (~/.ssh/config.d) at mode 0700 via the
// filewriter chokepoint, chmod'ing an already-existing directory back to 0700
// (STORE-01, STORE-04). The mode is always set explicitly, never inherited
// from the umask.
func EnsureIncludeDir(configDir string) error {
	if err := filewriter.EnsureDir(configDir, includeDirMode); err != nil {
		return fmt.Errorf("sshconfig: ensuring include dir %s: %w", configDir, err)
	}
	return nil
}

// EnsureIncludeLine floors a single gitid-managed Include line
// ("Include ~/.ssh/config.d/*.config") at the TOP of configPath (floor model —
// D-10, mirroring gitconfig.WriteBaselineInclude's [include] placement), via
// filewriter.PrependBlockIfNotFound.
//
// A missing configPath is tolerated (os.IsNotExist) — the common first-run
// case, treated as an empty starting file. The composed bytes are re-parsed
// via Parse before the write is committed, so a result that would not
// round-trip is rejected rather than persisted (refuse-to-corrupt invariant,
// mirroring sshconfig.Write). Re-running EnsureIncludeLine is idempotent: the
// Include line appears exactly once and its floor position is preserved
// (delegated to PrependBlockIfNotFound's existing-block ReplaceBlock path).
//
// The write goes through filewriter.Write at mode 0600 (STORE-04) —
// EnsureIncludeLine never calls os.WriteFile directly.
//
// backupPath is non-empty only when configPath pre-existed.
func EnsureIncludeLine(configPath string) (backupPath string, err error) {
	existing, err := os.ReadFile(configPath) //nolint:gosec // configPath is a trusted gitid-managed path supplied in-process
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("sshconfig: reading %s: %w", configPath, err)
	}

	composed := filewriter.PrependBlockIfNotFound(existing, sshIncludeBlockName, sshIncludeLineBody)

	// Round-trip safety: the composed config must parse cleanly before we
	// commit it to disk (parse -> compose -> parse stability).
	if _, perr := Parse(composed); perr != nil {
		return "", fmt.Errorf("sshconfig: composed config with Include line is not parseable, refusing to write: %w", perr)
	}

	backupPath, err = filewriter.Write(configPath, composed, includeFileMode)
	if err != nil {
		return "", fmt.Errorf("sshconfig: writing %s: %w", configPath, err)
	}
	return backupPath, nil
}
