package identity

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/castocolina/gitid/internal/gitconfig"
	"github.com/castocolina/gitid/internal/sshconfig"
)

// Inventory is the aggregated result of BuildInventory: every reconstructed
// identity's IdentityHealth report, plus the global unused-key list (keys
// found on disk that are referenced by no Host block anywhere).
type Inventory struct {
	Identities []IdentityHealth
	UnusedKeys []string
}

// InventoryDeps holds every external effect BuildInventory needs, each as an
// injected function field, so the builder is fully testable with fakes and
// deterministic — mirroring the injectable-Deps pattern already used
// elsewhere in this package (see identity.Deps) and in internal/platform's
// Deps/BuildProbeDeps. The real wiring is BuildInventoryDeps.
type InventoryDeps struct {
	// ReadSSHConfig returns the Include-aware merged bytes of ~/.ssh/config
	// (main file bytes followed by every globbed ~/.ssh/config.d/*.config
	// file's bytes), so managed blocks in EITHER storage layout are visible
	// to the raw-sentinel-scan parser (ParseManagedHosts does not resolve
	// Include on its own).
	ReadSSHConfig func() ([]byte, error)
	// ReadGitconfig returns the raw bytes of ~/.gitconfig.
	ReadGitconfig func() ([]byte, error)
	// ReadFragment reads one per-identity gitconfig fragment (the same
	// injectable seam Reconstruct already takes).
	ReadFragment func(fragPath string) (gitconfig.FragmentInfo, error)
	// Stat resolves whether a path exists on disk (key files, fragments).
	Stat func(path string) (os.FileInfo, error)
	// ListKeyFiles enumerates every gitid-managed private key file on disk,
	// for the global unused-key cross-reference.
	ListKeyFiles func() ([]string, error)
	// IsReservedKeyPath reports whether a key path is a gitid-owned reserved
	// location (D-06: the key-archive directory) that must never surface as
	// a doctor orphan/unused-key finding. BuildInventory applies this to the
	// RESULT of ListKeyFiles — the causal exclusion point (review R-04) —
	// rather than relying on any particular enumerator's glob shape to
	// happen to miss the archive directory. A nil predicate performs no
	// filtering.
	IsReservedKeyPath func(path string) bool
}

// BuildInventory is the impure aggregation layer: it reads the managed SSH
// and gitconfig bytes (Include-aware), reconstructs every []Account, resolves
// the real key-existence/usage facts for each via the injected deps, calls
// the pure Classify for each identity, and computes the global unused-key
// list. No sidecar DB — every fact is derived from the parsed managed blocks
// and the injected filesystem seam on each call (DLV-07).
func BuildInventory(deps InventoryDeps) (Inventory, error) {
	sshBytes, err := deps.ReadSSHConfig()
	if err != nil {
		return Inventory{}, fmt.Errorf("identity: build inventory: reading ssh config: %w", err)
	}
	gcBytes, err := deps.ReadGitconfig()
	if err != nil {
		return Inventory{}, fmt.Errorf("identity: build inventory: reading gitconfig: %w", err)
	}

	accounts, err := Reconstruct(sshBytes, gcBytes, deps.ReadFragment)
	if err != nil {
		return Inventory{}, fmt.Errorf("identity: build inventory: reconstructing accounts: %w", err)
	}

	// The union of every Host block's IdentityFile (gitid-managed AND
	// hand-written) — the D-12 data source for both the per-identity
	// keyUsedInSSH fact and the global unused-key cross-reference.
	referencedIdentityFiles := sshconfig.ParseAllHostIdentityFiles(sshBytes)
	referenced := make(map[string]bool, len(referencedIdentityFiles))
	for _, p := range referencedIdentityFiles {
		referenced[p] = true
	}

	identities := make([]IdentityHealth, 0, len(accounts))
	for _, acct := range accounts {
		keyExists := false
		if acct.KeyPath != "" {
			if _, statErr := deps.Stat(acct.KeyPath); statErr == nil {
				keyExists = true
			}
		}
		keyUsedInSSH := acct.KeyPath != "" && referenced[acct.KeyPath]
		keyUsedInGit := resolveKeyUsedInGit(acct, deps.ReadFragment)

		identities = append(identities, Classify(acct, keyExists, keyUsedInSSH, keyUsedInGit))
	}

	keyFiles, err := deps.ListKeyFiles()
	if err != nil {
		return Inventory{}, fmt.Errorf("identity: build inventory: listing key files: %w", err)
	}
	// D-06 / review R-04: drop reserved (archive) paths from the RESULT of
	// ListKeyFiles before the unused-key cross-reference runs, so the
	// exclusion is causal for ANY enumeration source — not merely an
	// incidental consequence of today's non-recursive id_* glob never
	// matching the archive directory in the first place.
	keyFiles = filterReservedKeyPaths(keyFiles, deps.IsReservedKeyPath)
	unusedKeys := crossReferenceUnusedKeys(keyFiles, referencedIdentityFiles)

	return Inventory{Identities: identities, UnusedKeys: unusedKeys}, nil
}

// filterReservedKeyPaths drops every path in keyFiles for which isReserved
// reports true. isReserved is nil-tolerant: a nil predicate (the zero value
// of a caller-constructed InventoryDeps that does not set the field)
// performs no filtering, preserving existing callers' behavior.
func filterReservedKeyPaths(keyFiles []string, isReserved func(string) bool) []string {
	if isReserved == nil {
		return keyFiles
	}
	filtered := make([]string, 0, len(keyFiles))
	for _, p := range keyFiles {
		if isReserved(p) {
			continue
		}
		filtered = append(filtered, p)
	}
	return filtered
}

// resolveKeyUsedInGit reports whether acct's key is wired for git commit
// signing: the fragment must enable ssh-format signing (GPGFormat=="ssh" &&
// CommitSign) AND its SigningKey must reference acct's key (by .pub path or
// private-key path — SigningKey is stored as the literal git config value,
// Pitfall E, so both forms are checked). Returns false when acct has no
// FragmentPath (nothing to read) or the fragment read fails/is missing.
//
// acct.FragmentPath is stored VERBATIM by Reconstruct — often a literal
// "~/.gitconfig.d/<name>" (the shape every gitid-managed includeIf block
// uses; see loader.go's own Reconstruct doc comment on this exact point).
// readFragment (gitconfig.ReadFragment in production) opens the path
// directly via os.Stat/exec, which never expands "~" itself. This function
// therefore expands the tilde HERE, exactly mirroring the tilde-expand-then-
// read Reconstruct already performs for its OWN, separate fragment read
// (loader.go, WR-02) — without it, key-used-both/key-used-ssh-only's
// SIGNING half of the git axis is silently unreachable for every
// recipe-shaped identity (found empirically while seeding 05-09-PLAN.md's
// list-populated eight-taxonomy PTY fixture: every identity with a
// tilde-form FragmentPath and a correctly-configured signing fragment still
// classified as git-signing-unused).
func resolveKeyUsedInGit(acct Account, readFragment func(string) (gitconfig.FragmentInfo, error)) bool {
	if acct.FragmentPath == "" || acct.KeyPath == "" {
		return false
	}
	readPath, expErr := expandTilde(acct.FragmentPath)
	if expErr != nil {
		readPath = acct.FragmentPath
	}
	frag, err := readFragment(readPath)
	if err != nil || frag.Missing {
		return false
	}
	if frag.GPGFormat != "ssh" || !frag.CommitSign {
		return false
	}
	return frag.SigningKey == acct.PubPath || frag.SigningKey == acct.KeyPath
}

// configDirGlob is the config.d/*.config glob literal (keep in sync with internal/sshconfig/include.go).
// It mirrors internal/sshconfig/include.go's canonical `Include ~/.ssh/config.d/*.config`
// literal (mirrored, not a shared symbol, to preserve Wave-1 independence —
// ACCEPTED DUPLICATION, MEDIUM #4 option b; see 01-04-PLAN.md objective). This
// literal MUST NOT be extracted into a shared exported constant, or this
// Wave-1 plan would depend_on 01-03 and force a re-wave of the DAG.
const configDirGlob = "config.d/*.config"

// BuildInventoryDeps wires the real, filesystem-backed InventoryDeps
// (EXPORTED — capital B — so cmd/gitid and the 01-06 e2e test can call it
// across the package boundary, mirroring internal/platform's exported
// BuildProbeDeps and closing the project's documented injected-seam wiring
// blindspot: every field here is non-nil).
//
// This is the $HOME-derived wiring — every field resolves home from
// os.UserHomeDir() internally. Any caller that already owns an explicit
// home (a Backend constructed via newBackendForHome, a test sandbox) must
// use InventoryDepsForHome instead (WR-35): calling this function from
// such a caller silently reads the REAL developer's ~/.ssh and ~/.gitconfig
// regardless of what home the caller passed elsewhere, unless the caller
// ALSO remembers to set $HOME — an easy-to-forget, silent hermeticity leak
// this project has hit twice before (doctor, Phase 5 TUI).
func BuildInventoryDeps() InventoryDeps {
	return InventoryDeps{
		ReadSSHConfig: readSSHConfigIncludeAware,
		ReadGitconfig: readGitconfigReal,
		ReadFragment:  gitconfig.ReadFragment,
		Stat: func(path string) (os.FileInfo, error) {
			return os.Stat(path) //nolint:gosec // path is a trusted gitid-managed path (G304)
		},
		ListKeyFiles: listKeyFilesReal,
		IsReservedKeyPath: func(p string) bool {
			home, err := os.UserHomeDir()
			if err != nil {
				return false
			}
			return sshconfig.IsReservedPath(filepath.Join(home, ".ssh"), p)
		},
	}
}

// InventoryDepsForHome wires InventoryDeps EXPLICITLY rooted at home (WR-35,
// iteration 4) — ReadSSHConfig/ReadGitconfig/ListKeyFiles never call
// os.UserHomeDir() or read $HOME at all, so a caller that already owns a
// resolved home (e.g. realBackend.home) cannot accidentally read the real
// developer's ~/.ssh/~/.gitconfig just because it forgot to also
// t.Setenv("HOME", home) somewhere else. Stat is unchanged — it operates on
// already-resolved absolute paths handed to it by the caller, which are
// never $HOME-relative on their own.
func InventoryDepsForHome(home string) InventoryDeps {
	return InventoryDeps{
		ReadSSHConfig: func() ([]byte, error) { return readSSHConfigIncludeAwareForHome(home) },
		ReadGitconfig: func() ([]byte, error) { return readGitconfigRealForHome(home) },
		ReadFragment:  gitconfig.ReadFragment,
		Stat: func(path string) (os.FileInfo, error) {
			return os.Stat(path) //nolint:gosec // path is a trusted gitid-managed path (G304)
		},
		ListKeyFiles: func() ([]string, error) { return listKeyFilesRealForHome(home) },
		IsReservedKeyPath: func(p string) bool {
			return sshconfig.IsReservedPath(filepath.Join(home, ".ssh"), p)
		},
	}
}

// readSSHConfigIncludeAware reads ~/.ssh/config, then globs+merges every
// ~/.ssh/config.d/*.config file's bytes onto it, so managed blocks in EITHER
// the in-file layout OR the STORE-01 Include'd config.d layout are visible to
// the raw-sentinel-scan ParseManagedHosts parser (which does not resolve the
// SSH `Include` directive on its own) — upholding D-11 (no layout carve-out).
// A missing main config file is tolerated (the common first-run case,
// treated as empty); an individual config.d read failure is skipped
// (best-effort merge — one unreadable fragment must not abort the whole
// inventory). Glob matches are sorted for deterministic merge order.
//
// This resolves home from os.UserHomeDir() ($HOME) — the production wiring
// via BuildInventoryDeps. WR-35: a caller that wants an EXPLICITLY-rooted
// inventory (e.g. a test-sandboxed HOME, or a Backend that already knows its
// own home) must use InventoryDepsForHome instead, which never touches
// os.UserHomeDir()/$HOME at all.
func readSSHConfigIncludeAware() ([]byte, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("identity: resolving home directory: %w", err)
	}
	return readSSHConfigIncludeAwareForHome(home)
}

// readSSHConfigIncludeAwareForHome is readSSHConfigIncludeAware's home-
// parameterized core (WR-35) — the ONLY logic difference from the exported
// wiring is where home comes from.
func readSSHConfigIncludeAwareForHome(home string) ([]byte, error) {
	mainPath := filepath.Join(home, ".ssh", "config")
	mainBytes, err := os.ReadFile(mainPath) //nolint:gosec // trusted gitid-managed path
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("identity: reading %s: %w", mainPath, err)
	}

	matches, globErr := filepath.Glob(filepath.Join(home, ".ssh", configDirGlob))
	if globErr != nil {
		return nil, fmt.Errorf("identity: globbing config.d: %w", globErr)
	}
	sort.Strings(matches)

	merged := mainBytes
	for _, m := range matches {
		b, rerr := os.ReadFile(m) //nolint:gosec // path from filepath.Glob under the trusted ~/.ssh/config.d dir
		if rerr != nil {
			continue // best-effort merge; one unreadable file must not abort the inventory
		}
		if len(merged) > 0 && !bytes.HasSuffix(merged, []byte("\n")) {
			merged = append(merged, '\n')
		}
		merged = append(merged, b...)
	}
	return merged, nil
}

// readGitconfigReal reads the raw bytes of ~/.gitconfig, resolved from
// os.UserHomeDir() ($HOME) — the production wiring via BuildInventoryDeps.
// A missing file is tolerated (the common first-run case, treated as
// empty). See readGitconfigRealForHome (WR-35) for the explicitly-rooted
// variant InventoryDepsForHome uses.
func readGitconfigReal() ([]byte, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("identity: resolving home directory: %w", err)
	}
	return readGitconfigRealForHome(home)
}

// readGitconfigRealForHome is readGitconfigReal's home-parameterized core.
func readGitconfigRealForHome(home string) ([]byte, error) {
	path := filepath.Join(home, ".gitconfig")
	b, err := os.ReadFile(path) //nolint:gosec // trusted gitid-managed path
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("identity: reading %s: %w", path, err)
	}
	return b, nil
}

// listKeyFilesReal enumerates every gitid-managed private key file under
// ~/.ssh, matching the "id_*" naming convention used by keygen.KeyPaths, and
// excluding the ".pub" siblings (only the private-key paths are cross-
// referenced against Host block IdentityFile values). home is resolved from
// os.UserHomeDir() ($HOME) — the production wiring via BuildInventoryDeps.
// See listKeyFilesRealForHome (WR-35) for the explicitly-rooted variant
// InventoryDepsForHome uses.
func listKeyFilesReal() ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("identity: resolving home directory: %w", err)
	}
	return listKeyFilesRealForHome(home)
}

// listKeyFilesRealForHome is listKeyFilesReal's home-parameterized core.
func listKeyFilesRealForHome(home string) ([]string, error) {
	sshDir := filepath.Join(home, ".ssh")
	matches, err := filepath.Glob(filepath.Join(sshDir, "id_*"))
	if err != nil {
		return nil, fmt.Errorf("identity: globbing ssh key files: %w", err)
	}
	keys := make([]string, 0, len(matches))
	for _, m := range matches {
		if filepath.Ext(m) == ".pub" {
			continue
		}
		// Defense in depth (D-06): the "id_*" glob is non-recursive and
		// never matches anything under gitid-archive/ in the first place,
		// so this branch is currently unreachable in production — the
		// CAUSAL guard is InventoryDeps.IsReservedKeyPath applied in
		// BuildInventory (review R-04). Kept here so a future recursive
		// enumerator inherits the guard for free rather than silently
		// reintroducing the archive-visibility bug.
		if sshconfig.IsReservedPath(sshDir, m) {
			continue
		}
		keys = append(keys, m)
	}
	return keys, nil
}
