package sshconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

// configDirName is the gitid-owned Include'd storage directory under ~/.ssh.
// It is the directory half of sshIncludeLineBody's glob; the two MUST stay in
// sync (the Include line is what makes this directory load at all).
const configDirName = "config.d"

// configFileExt is the extension the gitid-owned Include glob matches. Only
// `*.config` files inside configDirName are gitid storage — anything else the
// user drops in that directory is theirs.
const configFileExt = ".config"

// ArchiveDirName is the gitid-owned key-archive directory name under
// ~/.ssh (D-06). Archived key pairs land here, in ONE dedicated directory —
// distinct from filewriter's sibling ".bak.<nanos>" convention used for
// config files, because key material gets its own retention location rather
// than living next to the canonical slot it vacated.
const ArchiveDirName = "gitid-archive"

// ArchiveDir returns the absolute archive directory path under sshDir
// (D-06). This is the single source of the archive location — every package
// that needs it (keygen's archive primitives, identity's inventory
// exclusion, the doctor reserved-path registry) resolves it from here,
// never from a duplicated literal.
func ArchiveDir(sshDir string) string {
	return filepath.Join(sshDir, ArchiveDirName)
}

// IsReservedBlockName reports whether a gitid-managed SSH block name is a
// reserved, non-identity block. Two names are reserved:
//
//   - sshIncludeBlockName ("ssh-include") — the gitid-owned Include line, which
//     has no per-identity Host block and no gitconfig counterpart by design.
//   - globalBlockName ("_global") — the macOS `Host *` keychain/agent stanza,
//     which Phase 3 D-08 rewrites on EVERY create. It is wiring, not an
//     identity: without this registration the doctor Orphans check reports it
//     as an SSH block with no gitconfig partner and offers a removal fix that
//     deletes the block the next create immediately re-writes — the project's
//     documented destructive false-positive loop (L4).
//
// Mirrors gitconfig.IsReservedBlockName, so identity discovery and the doctor
// Orphans check can exclude both the same way the gitconfig side already does
// (Pitfall 4 / project memory "Doctor reserved-block false-positive loop").
//
// Hand-off: renaming `_global` to `global-ssh` is Phase 6's job (LEGACY-TRIAGE
// P6 D-08). This registration is deliberately ADDITIVE — the constant and its
// value stay untouched here.
func IsReservedBlockName(name string) bool {
	return name == sshIncludeBlockName || name == globalBlockName
}

// ReservedPaths returns the gitid-owned Include'd storage locations under
// sshDir: the `config.d` directory and the `*.config` glob inside it, PLUS
// (D-06) the key-archive directory and a recursive glob beneath it. They are
// the filesystem artifacts D-06 creates on a fresh machine, and no fix path may
// propose removing or rewriting them (L4). The archive entries are appended
// AFTER the two config.d entries so callers relying on the config.d prefix
// (e.g. this package's own TestReservedPaths ordering assertion) are
// unaffected.
//
// Hand-off: Phase 8 D-06.2 generalizes this into a cross-cutting reserved-PATH
// registry; these entries are the SSH-side seed it consumes.
func ReservedPaths(sshDir string) []string {
	dir := filepath.Join(sshDir, configDirName)
	archiveDir := ArchiveDir(sshDir)
	return []string{
		dir,
		filepath.Join(dir, "*"+configFileExt),
		archiveDir,
		filepath.Join(archiveDir, "**"),
	}
}

// IsReservedPath reports whether path is one of the gitid-owned Include'd
// storage locations under sshDir — the `config.d` directory itself, a
// `*.config` file directly inside it, OR (D-06) the key-archive directory
// itself or any path nested underneath it AT ANY DEPTH. Comparison is on
// filepath.Clean'ed values, so `~/.ssh/config.d/./gitid.config` matches.
//
// The archive containment check is a cleaned, separator-bounded PREFIX
// check — not a filepath.Dir equality test (review R-19): a future
// per-identity or per-generation archive layout would nest files one or more
// directories deeper, and a Dir-equality guard would silently stop covering
// it the moment that happens. A prefix check covers every depth by
// construction.
//
// `~/.ssh/config` itself is NOT reserved (it is the user's file; gitid only
// owns sentinel-delimited blocks inside it), and a non-`.config` file the user
// drops into config.d is theirs, not gitid storage.
//
// Hand-off: see ReservedPaths — Phase 8 D-06.2 generalizes the registry.
func IsReservedPath(sshDir, path string) bool {
	dir := filepath.Clean(filepath.Join(sshDir, configDirName))
	clean := filepath.Clean(path)
	if clean == dir {
		return true
	}
	if filepath.Dir(clean) == dir && filepath.Ext(clean) == configFileExt {
		return true
	}

	archiveDir := filepath.Clean(ArchiveDir(sshDir))
	if clean == archiveDir || strings.HasPrefix(clean, archiveDir+string(filepath.Separator)) {
		return true
	}
	return false
}

// ManagedBlockNames returns every gitid-managed block name reachable from
// configPath: the blocks written directly into it, UNIONED with the blocks of
// every file its `Include` directives resolve to. Order is main-file blocks
// first, then Include'd files in directive order; duplicates are collapsed.
//
// This is Include-AWARE discovery, and it is mandatory from Phase 3 onward.
// D-06 makes the Include'd layout the fresh-machine DEFAULT, so on a fresh
// machine EVERY identity Host block lives in `config.d/gitid.config` and
// `~/.ssh/config` carries only the reserved wiring. A doctor composition root
// that reads the main file alone therefore sees ZERO identity blocks, and
// CheckOrphans Class 2 offers a DESTRUCTIVE removal fix for every legitimate
// gitconfig block (L4).
//
// Only absolute or `~/.ssh`-relative Include paths are followed, matching
// Adopt's boundary rule. A directive whose glob resolves to nothing is skipped
// (the Include'd file was never created — the common first-run case). A match
// that exists but cannot be read is an error: the glob proved the file is
// there, so silently dropping its blocks would reintroduce exactly the
// blindness this function exists to remove.
func ManagedBlockNames(configPath string) ([]string, error) {
	content, err := os.ReadFile(configPath) //nolint:gosec // configPath is a trusted gitid-managed path supplied in-process
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("sshconfig: reading %s: %w", configPath, err)
	}

	seen := make(map[string]bool)
	var names []string
	appendBlocks := func(b []byte) {
		for _, block := range filewriter.ListBlocks(b) {
			if seen[block.Name] {
				continue
			}
			seen[block.Name] = true
			names = append(names, block.Name)
		}
	}
	appendBlocks(content)

	directives, err := DetectInclude(configPath)
	if err != nil {
		return nil, fmt.Errorf("sshconfig: managed block names: %w", err)
	}
	for _, d := range directives {
		if !isAcceptablePathForm(d.Raw) {
			continue // bare-relative / non-~/.ssh path — outside gitid's boundary
		}
		matches, gerr := filepath.Glob(d.Expanded)
		if gerr != nil {
			return nil, fmt.Errorf("sshconfig: globbing included %s: %w", d.Expanded, gerr)
		}
		for _, m := range matches {
			b, rerr := os.ReadFile(m) //nolint:gosec // m comes from globbing a trusted gitid-managed Include path (G304)
			if rerr != nil {
				return nil, fmt.Errorf("sshconfig: reading included %s: %w", m, rerr)
			}
			appendBlocks(b)
		}
	}
	return names, nil
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
