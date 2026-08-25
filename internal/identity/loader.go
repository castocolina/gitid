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
//
// Bitbucket's two endpoints were added by 05-04 review R2-04: Bitbucket is
// already a first-class product provider (tuikit.wizardProviders,
// providerHostname, gitconfig/baseline.go's rewrite order, and
// recipes/ssh-config.recipe's "Host bitbucket.org" / "Hostname
// altssh.bitbucket.org" stanza), so leaving it out of this table under-counts
// a surviving hand-written Bitbucket stanza during a provider-rewrite delete
// — the same silent https-clone breaker review R-05 named for GitHub, one
// provider over.
var hostnameToProvider = map[string]string{
	"ssh.github.com":       "github",
	"github.com":           "github",
	"altssh.gitlab.com":    "gitlab",
	"gitlab.com":           "gitlab",
	"altssh.bitbucket.org": "bitbucket",
	"bitbucket.org":        "bitbucket",
}

// shortToFQDNProvider maps the short provider token hostnameToProvider's
// VALUES use (no dot) to the FQDN-shaped provider host the
// gitconfig.WriteProviderRewrite managed block is keyed under
// ("provider-rewrite:<FQDN>"). Every provider gitid can create MUST have
// exactly one entry here — TestProviderTableRoundTripsWizardProviders (in
// loader_test.go) drives this table from providerHostname's own switch (in
// identity.go — DefaultHostname) and asserts every case that switch handles
// round-trips through ProviderHostForSSHHostname to the same FQDN key, so a
// provider the wizard can create can never again be silently absent from
// the count path (review R2-04).
var shortToFQDNProvider = map[string]string{
	"github":    "github.com",
	"gitlab":    "gitlab.com",
	"bitbucket": "bitbucket.org",
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

		// CR-09: ForceSSH must reflect the REAL on-disk state, never a
		// default. The rewrite block gitconfig.WriteProviderRewrite manages
		// is always keyed by the FQDN-shaped provider host the pane writes
		// with (tuikit's g.provider, e.g. "github.com" — see
		// providerFromSSHHost/providerHost in internal/tuikit/identities.go),
		// but acct.Provider here can ALSO be the short form from
		// hostnameToProvider ("github", no dot) when no "# gitid: provider="
		// marker is present. rewriteLookupProvider mirrors the pane's own
		// alias-suffix fallback so the lookup key always matches what was
		// actually written, regardless of which form acct.Provider holds. A
		// malformed/unrecognized host (HasProviderRewrite erroring on
		// hostname validation) is treated as "no rewrite" rather than
		// failing the whole reconstruction — this is a read-only display
		// value, not a write-path decision.
		if lookupProvider := RewriteProviderKey(acct.Provider, acct.Alias); lookupProvider != "" {
			if has, herr := gitconfig.HasProviderRewrite(gcBytes, lookupProvider); herr == nil {
				acct.ForceSSH = has
			}
		}

		// Gitconfig includeIf side.
		if gc, ok := gcBlocks[name]; ok && gc.FragmentPath != "" {
			acct.Matches = gc.Matches
			acct.FragmentPath = gc.FragmentPath
		} else {
			missing = append(missing, "gitconfig-includeif-block")
		}

		// Fragment side (only when we have a path to read). FragmentPath is
		// stored verbatim from the parsed includeIf `path =` value (often
		// "~/.gitconfig.d/<name>" — git itself expands "~" when resolving
		// includeIf at runtime, but readFrag opens the file directly via
		// os.Stat/exec, which never expands "~"). Expand it (WR-02, same
		// helper update.go uses for pub-key paths) for THIS read only —
		// acct.FragmentPath itself stays in its original verbatim form,
		// since callers display it and re-derive write targets from it.
		if acct.FragmentPath != "" {
			readPath, expErr := expandTilde(acct.FragmentPath)
			if expErr != nil {
				readPath = acct.FragmentPath
			}
			frag, ferr := readFrag(readPath)
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

// RewriteProviderKey returns the FQDN-shaped provider host the
// gitconfig.WriteProviderRewrite managed block is keyed under, given a
// reconstructed account's Provider and Alias. This is the promoted, exported
// form of the prior unexported rewriteLookupProvider (review R-05): the SAME
// normalizer now backs both the WRITE path (Reconstruct's ForceSSH lookup,
// above) and the COUNT path (ProviderRefCount, delete.go) — a caller can
// never compare a raw acct.Provider against a normalized key, because there
// is only ever one place that does the normalizing.
//
// Precedence:
//  1. provider is already dotted (the FQDN marker/CreateInput form every
//     current write path produces, e.g. "github.com") -> used as-is.
//  2. provider is a non-empty, DOTLESS short form the provider table knows
//     (e.g. "github") -> resolved to its FQDN form via shortToFQDNProvider.
//     Review R2-11: this guard is inserted BEFORE the alias-suffix
//     fallback, so a short provider paired with a dotless alias
//     ("github"/"mygh") no longer falls through to that fallback and
//     returns the alias itself — a one-label string
//     gitconfig.validProviderHostname rejects, silently making the account
//     count as a reference to nothing. This corrects the WRITE path's
//     answer too, for exactly the accounts whose PRIOR answer was that
//     invalid one-label host — the intended correction, not a side effect.
//  3. otherwise, re-derived from alias's suffix (e.g. "work.github.com" ->
//     "github.com"), mirroring internal/tuikit/identities.go's
//     providerFromSSHHost fallback (CR-09).
//
// Returns "" only when neither source yields anything usable (e.g. an empty
// provider AND a two-label-or-shorter alias), matching
// gitconfig.HasProviderRewrite's contract that an empty provider never has a
// block.
func RewriteProviderKey(provider, alias string) string {
	if provider != "" && strings.Contains(provider, ".") {
		return provider
	}
	if provider != "" {
		if fqdn, ok := shortToFQDNProvider[provider]; ok {
			return fqdn
		}
	}
	parts := strings.Split(alias, ".")
	if len(parts) <= 2 {
		return alias
	}
	return strings.Join(parts[1:], ".")
}

// ProviderHostForSSHHostname maps a Host stanza's Hostname value — the
// recipe's own canonical alt-SSH endpoint, or the bare provider host — to
// the FQDN-shaped provider key the gitconfig rewrite block is stored under.
// Built from the SAME hostnameToProvider + shortToFQDNProvider tables
// RewriteProviderKey uses, so adding a fourth provider is a one-line change
// in ONE place (review R-05, extended to Bitbucket by R2-04). Returns "" for
// an unrecognized hostname (D-13's honest-unknown convention) — never a
// guess.
func ProviderHostForSSHHostname(hostname string) string {
	short, ok := hostnameToProvider[hostname]
	if !ok {
		return ""
	}
	return shortToFQDNProvider[short]
}

// ProviderKeyForHost resolves ONE SSH Host stanza — its alias, its Hostname
// value, and (when present) its "# gitid: provider=" marker comment — to the
// FQDN-shaped provider key its rewrite block would be counted under.
// Documented precedence (most authoritative first):
//
//  1. providerMarker, when non-empty — the explicit "# gitid: provider="
//     comment a gitid-managed block carries. Normalized through
//     RewriteProviderKey so a short-form marker is still corrected (R2-11).
//  2. ProviderHostForSSHHostname(hostname) — the recipe-canonical alt-SSH
//     endpoint or bare provider hostname (e.g. "ssh.github.com",
//     "altssh.bitbucket.org", "bitbucket.org").
//  3. RewriteProviderKey("", alias)'s alias-suffix fallback — the last
//     resort for a hand-written stanza with neither a marker nor a
//     recognized hostname.
//
// Returns "" when none of the three yields anything (an unrecognized custom
// host paired with a two-label-or-shorter alias) — D-13's honest unknown.
func ProviderKeyForHost(alias, hostname, providerMarker string) string {
	if providerMarker != "" {
		return RewriteProviderKey(providerMarker, alias)
	}
	if host := ProviderHostForSSHHostname(hostname); host != "" {
		return host
	}
	return RewriteProviderKey("", alias)
}

// ProviderRefCount counts every account in accounts (other than
// excludingName — the identity being deleted) whose
// RewriteProviderKey(acct.Provider, acct.Alias) equals providerKey — the
// number of OTHER gitid-managed identities still referencing the same
// provider rewrite block (D-09). Both sides of the comparison are
// NORMALIZED keys; ProviderRefCount never compares a raw acct.Provider
// directly against providerKey.
func ProviderRefCount(accounts []Account, providerKey, excludingName string) int {
	count := 0
	for _, acct := range accounts {
		if acct.Name == excludingName {
			continue
		}
		if RewriteProviderKey(acct.Provider, acct.Alias) == providerKey {
			count++
		}
	}
	return count
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
