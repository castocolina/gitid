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
	unusedKeys := crossReferenceUnusedKeys(keyFiles, referencedIdentityFiles)

	return Inventory{Identities: identities, UnusedKeys: unusedKeys}, nil
}

// resolveKeyUsedInGit reports whether acct's key is wired for git commit
// signing: the fragment must enable ssh-format signing (GPGFormat=="ssh" &&
// CommitSign) AND its SigningKey must reference acct's key (by .pub path or
// private-key path — SigningKey is stored as the literal git config value,
// Pitfall E, so both forms are checked). Returns false when acct has no
// FragmentPath (nothing to read) or the fragment read fails/is missing.
func resolveKeyUsedInGit(acct Account, readFragment func(string) (gitconfig.FragmentInfo, error)) bool {
	if acct.FragmentPath == "" || acct.KeyPath == "" {
		return false
	}
	frag, err := readFragment(acct.FragmentPath)
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
func BuildInventoryDeps() InventoryDeps {
	return InventoryDeps{
		ReadSSHConfig: readSSHConfigIncludeAware,
		ReadGitconfig: readGitconfigReal,
		ReadFragment:  gitconfig.ReadFragment,
		Stat: func(path string) (os.FileInfo, error) {
			return os.Stat(path) //nolint:gosec // path is a trusted gitid-managed path (G304)
		},
		ListKeyFiles: listKeyFilesReal,
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
func readSSHConfigIncludeAware() ([]byte, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("identity: resolving home directory: %w", err)
	}

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

// readGitconfigReal reads the raw bytes of ~/.gitconfig. A missing file is
// tolerated (the common first-run case, treated as empty).
func readGitconfigReal() ([]byte, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("identity: resolving home directory: %w", err)
	}
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
// referenced against Host block IdentityFile values).
func listKeyFilesReal() ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("identity: resolving home directory: %w", err)
	}
	matches, err := filepath.Glob(filepath.Join(home, ".ssh", "id_*"))
	if err != nil {
		return nil, fmt.Errorf("identity: globbing ssh key files: %w", err)
	}
	keys := make([]string, 0, len(matches))
	for _, m := range matches {
		if filepath.Ext(m) == ".pub" {
			continue
		}
		keys = append(keys, m)
	}
	return keys, nil
}
