package main

// identity_read.go implements the D-03 read surface: `gitid identity list`
// and `gitid identity show <name>`, each rendering a TTY-aligned table, a
// piped tab-delimited stream, or a stable `--json` document. Reads are
// ALWAYS headless (D-02) — neither command ever opens the TUI regardless of
// TTY state.
//
// Classification is NEVER re-derived here — every health fact comes straight
// from identity.BuildInventory (05-RESEARCH.md's Architectural Responsibility
// Map: that logic lives in internal/identity, not cmd/gitid).

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/term"

	"github.com/castocolina/gitid/internal/gitconfig"
	"github.com/castocolina/gitid/internal/identity"
)

// identityRecord is the D-03 frozen `--json` shape for ONE identity.
//
// FREEZE (D-03's "keep it stable once shipped" clause): this struct marshals
// the RICHER two-axis shape — identity_state (IdentityHealth.IdentityState),
// key_state (IdentityHealth.KeyState), and problems (IdentityHealth.Problems)
// — ALONGSIDE the collapsed single-label state (ClassifyState). A consumer
// can always flatten to state alone, but can never recover detail that was
// never emitted — so later needs are satisfied by ADDING a field, never
// renaming or removing one.
type identityRecord struct {
	Name          string   `json:"name"`
	IdentityState string   `json:"identity_state"`
	KeyState      string   `json:"key_state"`
	State         string   `json:"state"`
	Problems      []string `json:"problems"`
	Alias         string   `json:"alias"`
	Hostname      string   `json:"hostname"`
	Port          int      `json:"port"`
	KeyPath       string   `json:"key_path"`
	PubPath       string   `json:"pub_path"`
	FragmentPath  string   `json:"fragment_path"`
	Provider      string   `json:"provider"`
	ForceSSH      bool     `json:"force_ssh"`
	GitName       string   `json:"git_name"`
	GitEmail      string   `json:"git_email"`
	MatchStrategy string   `json:"match_strategy"`
	Complete      bool     `json:"complete"`
}

// identityListDocument is the frozen `list --json` shape: TWO independent
// collections, never merged into one array (review R-06) — Identities are
// reconstructed rows; UnusedKeys are private key files on disk that belong
// to NO identity (identity.Inventory.UnusedKeys) and therefore cannot be
// expressed as an identity row.
type identityListDocument struct {
	Identities []identityRecord `json:"identities"`
	UnusedKeys []string         `json:"unused_keys"`
}

// newIdentityListVerb builds the `list` verb spec.
func newIdentityListVerb() identityVerb {
	var jsonOut, dryRun bool
	return identityVerb{
		use:     "list",
		aliases: nil,
		short:   "List every gitid-managed identity",
		args:    cobra.NoArgs,
		bindFlags: func(fs *pflag.FlagSet) {
			fs.BoolVar(&jsonOut, "json", false, "print a single JSON document with identities and unused_keys")
			fs.BoolVar(&dryRun, "dry-run", false, "reserved: reads never write, so --dry-run is a usage error")
		},
		run: func(cmd *cobra.Command, _ []string) error {
			if dryRun {
				return fmt.Errorf("gitid: --dry-run is not valid for a read command: `identity list` never writes")
			}
			home, err := resolveHomeForCLI()
			if err != nil {
				return err
			}
			records, unused, err := buildIdentityRecords(home)
			if err != nil {
				return err
			}
			isTTY := term.IsTerminal(int(os.Stdout.Fd()))
			return renderIdentityList(cmd.OutOrStdout(), isTTY, jsonOut, records, unused)
		},
	}
}

// newIdentityShowVerb builds the `show <name>` verb spec.
func newIdentityShowVerb() identityVerb {
	var jsonOut, dryRun bool
	return identityVerb{
		use:     "show <name>",
		aliases: nil,
		short:   "Show one gitid-managed identity",
		args:    cobra.ExactArgs(1),
		bindFlags: func(fs *pflag.FlagSet) {
			fs.BoolVar(&jsonOut, "json", false, "print a single JSON identity record")
			fs.BoolVar(&dryRun, "dry-run", false, "reserved: reads never write, so --dry-run is a usage error")
		},
		run: func(cmd *cobra.Command, args []string) error {
			if dryRun {
				return fmt.Errorf("gitid: --dry-run is not valid for a read command: `identity show` never writes")
			}
			home, err := resolveHomeForCLI()
			if err != nil {
				return err
			}
			records, _, err := buildIdentityRecords(home)
			if err != nil {
				return err
			}
			for _, r := range records {
				if r.Name == args[0] {
					isTTY := term.IsTerminal(int(os.Stdout.Fd()))
					return renderIdentityShow(cmd.OutOrStdout(), isTTY, jsonOut, r)
				}
			}
			return fmt.Errorf("gitid: no such identity: %q", args[0])
		},
	}
}

// resolveHomeForCLI resolves the real HOME directory the same way
// buildBackend does — honoring $HOME (t.Setenv in tests), never a
// hardcoded path.
func resolveHomeForCLI() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("gitid: resolving home directory: %w", err)
	}
	return home, nil
}

// buildIdentityRecords joins identity.BuildInventory's health axes
// (IdentityState/KeyState/Problems + UnusedKeys) with b.accounts()'s
// Account-shaped fields, keyed by identity NAME — never re-deriving
// classification here (05-RESEARCH.md Architectural Responsibility Map).
// Two consecutive calls with no intervening write produce byte-identical
// records (MGR-08 — nothing is cached or persisted between calls).
func buildIdentityRecords(home string) (records []identityRecord, unusedKeys []string, err error) {
	inv, err := identity.BuildInventory(identity.InventoryDepsForHome(home))
	if err != nil {
		return nil, nil, fmt.Errorf("gitid: building identity inventory: %w", err)
	}

	b := newBackendForHome(home)
	acctByName := make(map[string]identity.Account, len(inv.Identities))
	for _, a := range b.accounts() {
		acctByName[a.Name] = a
	}

	records = make([]identityRecord, 0, len(inv.Identities))
	for _, h := range inv.Identities {
		records = append(records, toIdentityRecord(h, acctByName[h.Name]))
	}

	unusedKeys = inv.UnusedKeys
	if unusedKeys == nil {
		unusedKeys = []string{}
	}
	return records, unusedKeys, nil
}

// toIdentityRecord projects one IdentityHealth + its matching Account into
// the frozen identityRecord shape. An SSH-only identity's Account naturally
// carries GitName/GitEmail/FragmentPath as the Go zero value "" — never a
// fabricated placeholder (MGR-03) — because Reconstruct only populates them
// when a fragment was actually read.
func toIdentityRecord(h identity.IdentityHealth, acct identity.Account) identityRecord {
	problems := make([]string, 0, len(h.Problems))
	for _, p := range h.Problems {
		problems = append(problems, string(p))
	}
	state := collapseState(h)
	gitDir, strategy := matchStrategyFor(acct)
	_ = gitDir // GitDir is not part of the frozen D-03 record shape; kept local for clarity.
	return identityRecord{
		Name:          h.Name,
		IdentityState: string(h.IdentityState),
		KeyState:      string(h.KeyState),
		State:         string(state),
		Problems:      problems,
		Alias:         acct.Alias,
		Hostname:      acct.Hostname,
		Port:          acct.Port,
		KeyPath:       acct.KeyPath,
		PubPath:       acct.PubPath,
		FragmentPath:  acct.FragmentPath,
		Provider:      acct.Provider,
		ForceSSH:      acct.ForceSSH,
		GitName:       acct.GitName,
		GitEmail:      acct.GitEmail,
		MatchStrategy: strategy,
		Complete:      state == identity.StateComplete,
	}
}

// collapseState flattens an already-computed IdentityHealth's two axes into
// ClassifyState's single-label precedence (documented on ClassifyState
// itself): structural IdentityState blockers first
// (fragment-path-missing/git-only/incomplete), then KeyState problems
// (key-missing/key-unused/key-used-ssh-only), then complete. This is
// presentation of BuildInventory's ALREADY-COMPUTED result, not a second
// classification pass — the underlying facts (key existence, SSH/Git usage)
// are never re-derived here, only h's own two fields are read.
func collapseState(h identity.IdentityHealth) identity.State {
	switch h.IdentityState {
	case identity.StateFragmentPathMissing, identity.StateGitOnly, identity.StateIncomplete:
		return h.IdentityState
	}
	switch h.KeyState {
	case identity.StateKeyMissing, identity.StateKeyUnused, identity.StateKeyUsedSSHOnly:
		return h.KeyState
	}
	return identity.StateComplete
}

// matchStrategyFor derives the includeIf match strategy the same way
// wiring.go's toDemoIdentity does — the sole source of this derivation is
// acct.Matches; an SSH-only identity (empty FragmentPath, no Matches) yields
// an empty strategy, never a fabricated default.
func matchStrategyFor(acct identity.Account) (gitDir, strategy string) {
	for _, match := range acct.Matches {
		switch match.Kind {
		case gitconfig.MatchGitdir:
			gitDir = match.Value
		case gitconfig.MatchHasconfig:
			strategy = "hasconfig"
		}
	}
	if gitDir != "" {
		if strategy == "hasconfig" {
			strategy = "both"
		} else {
			strategy = "gitdir"
		}
	}
	if strategy == "" && acct.FragmentPath != "" {
		strategy = "gitdir"
	}
	return gitDir, strategy
}

// renderIdentityList dispatches list's three output modes. jsonOut wins over
// isTTY: `--json` always prints the frozen identityListDocument regardless
// of terminal state.
func renderIdentityList(w io.Writer, isTTY, jsonOut bool, records []identityRecord, unusedKeys []string) error {
	if jsonOut {
		return writeJSON(w, identityListDocument{Identities: records, UnusedKeys: unusedKeys})
	}
	if isTTY {
		writeTable(w, records)
		return nil
	}
	writeTabDelimited(w, records)
	return nil
}

// renderIdentityShow dispatches show's output modes for a single record.
func renderIdentityShow(w io.Writer, isTTY, jsonOut bool, r identityRecord) error {
	if jsonOut {
		return writeJSON(w, r)
	}
	if isTTY {
		writeTable(w, []identityRecord{r})
		return nil
	}
	writeTabDelimited(w, []identityRecord{r})
	return nil
}

// writeTable renders records as an aligned column table with a header row
// (text/tabwriter), for a TTY stdout.
func writeTable(w io.Writer, records []identityRecord) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "NAME\tSTATE\tALIAS\tGIT EMAIL\tKEY STATE")
	for _, r := range records {
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", r.Name, r.State, r.Alias, r.GitEmail, r.KeyState)
	}
	_ = tw.Flush()
}

// writeTabDelimited renders one tab-separated record per line with NO
// header decoration, for a piped stdout.
func writeTabDelimited(w io.Writer, records []identityRecord) {
	for _, r := range records {
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", r.Name, r.State, r.Alias, r.GitEmail, r.KeyState)
	}
}

// writeJSON marshals v with json.MarshalIndent (two-space indent) plus a
// single trailing newline.
func writeJSON(w io.Writer, v interface{}) error {
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("gitid: marshaling JSON: %w", err)
	}
	_, err = fmt.Fprintln(w, strings.TrimRight(string(out), "\n"))
	return err
}
