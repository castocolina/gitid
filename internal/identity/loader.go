package identity

import (
	"fmt"
	"sort"
	"strings"

	"github.com/castocolina/gitid/internal/gitconfig"
	"github.com/castocolina/gitid/internal/sshconfig"
)

// hostnameToProvider maps known SSH hostnames to their provider names.
// Used as fallback when no "# gitid: provider=" marker is present (D-12).
// Legacy identities without a marker resolve via this map; custom hosts
// with unknown hostnames get an empty provider (D-13 — honest unknown).
var hostnameToProvider = map[string]string{
	"ssh.github.com":    "github",
	"github.com":        "github",
	"altssh.gitlab.com": "gitlab",
	"gitlab.com":        "gitlab",
}

// Reconstruct assembles []Account from the four managed artifacts.
// sshBytes and gcBytes are the raw bytes of ~/.ssh/config and ~/.gitconfig.
// readFrag is injectable for testing (fake reads). The join key is the identity
// name (D-01). Accounts with missing pieces are included with Incomplete set
// (D-02); deep diagnosis stays in Phase 4 doctor.
func Reconstruct(
	sshBytes []byte,
	gcBytes []byte,
	readFrag func(fragPath string) (gitconfig.FragmentInfo, error),
) ([]Account, error) {
	sshHosts, err := sshconfig.ParseManagedHosts(sshBytes)
	if err != nil {
		return nil, fmt.Errorf("identity: reconstruct: parsing ssh config: %w", err)
	}
	gcBlocks := gitconfig.ParseManagedIncludeIf(gcBytes)

	// Union of all known identity names across both files.
	names := nameUnion(sshHosts, gcBlocks)
	if len(names) == 0 {
		return nil, nil
	}

	var accounts []Account
	for _, name := range names {
		acct := Account{Name: name}
		var missing []string

		// SSH side.
		if ssh, ok := sshHosts[name]; ok && ssh.Alias != "" {
			acct.Alias = ssh.Alias
			acct.Hostname = ssh.Hostname
			acct.Port = ssh.Port
			acct.KeyPath = ssh.IdentityFile
			acct.PubPath = ssh.IdentityFile + ".pub"
			// Prefer explicit marker (D-11); fall back to hostname map (D-12).
			// Custom hosts with unknown hostnames get empty provider (D-13 — honest unknown).
			if ssh.Provider != "" {
				acct.Provider = ssh.Provider
			} else if p, ok := hostnameToProvider[ssh.Hostname]; ok {
				acct.Provider = p
			}
		} else {
			missing = append(missing, "ssh-host-block")
		}

		// Gitconfig includeIf side.
		if gc, ok := gcBlocks[name]; ok && gc.FragmentPath != "" {
			acct.Matches = gc.Matches
			acct.FragmentPath = gc.FragmentPath
		} else {
			missing = append(missing, "gitconfig-includeif-block")
		}

		// Fragment side (only when we have a path to read).
		if acct.FragmentPath != "" {
			frag, ferr := readFrag(acct.FragmentPath)
			if ferr == nil && !frag.Missing {
				acct.GitName = frag.GitName
				acct.GitEmail = frag.GitEmail
			} else {
				missing = append(missing, "fragment-file")
			}
		}

		acct.Incomplete = strings.Join(missing, ",")
		accounts = append(accounts, acct)
	}
	return accounts, nil
}

// nameUnion returns a sorted slice of all unique identity names found in
// either sshHosts or gcBlocks maps.
func nameUnion(sshHosts map[string]sshconfig.SSHHostInfo, gcBlocks map[string]gitconfig.IncludeIfInfo) []string {
	seen := make(map[string]struct{})
	for name := range sshHosts {
		seen[name] = struct{}{}
	}
	for name := range gcBlocks {
		seen[name] = struct{}{}
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
