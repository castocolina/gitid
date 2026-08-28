package tuikit

// store.go is the Go mirror of
// .planning/design/mockup-src/src/demo/store.ts — the view state every
// screen renders from, plus the pure reducer that transitions it.
//
// Nothing here performs I/O. Reduce is a pure (state, action) → state
// function: the INITIAL state and the effect of a COMMITTED action both
// come from the injected Backend (InitialState / Persist), so the dummy
// reduces in memory while the real binary performs a backed-up write and
// re-reads. Screens never call Reduce directly — actions flow back to the
// App, the single Persist caller.

import (
	"strings"
	"time"
)

// DemoIdentity is one identity row of the live demo — an
// IdentityManagerRow extended with the Git author values, match strategy,
// and real SSH endpoint the web demo's DemoIdentity carries.
type DemoIdentity struct {
	Name            string
	State           string
	SSHHost         string
	KeyPath         string
	GitFragmentPath string
	Note            string
	// Git author values, present once the identity's Git side is configured.
	GitName       string
	GitEmail      string
	MatchStrategy string
	GitDir        string
	ForceSSH      bool
	PublicKeyPath string
	GitOriginal   GitOriginal
	// GitConfigured distinguishes an explicit create-flow confirmation from
	// incidental Git-shaped row data used by read-only views and tests.
	GitConfigured bool
	// Real SSH endpoint + port (SSHUI-01); optional for seeded rows.
	Hostname string
	Port     int
	// SigningKeyPath is the fragment's user.signingkey value as parsed,
	// never derived from KeyPath (MGR-03). Empty means absent.
	SigningKeyPath string
	// ReuseKeyPath carries the D-10 "reuse an existing key" selection through
	// to the committed write — non-empty means Persist must NOT generate a
	// new key pair, it must point the identity at this existing key (KEY-06).
	ReuseKeyPath string
	// Algorithm is the selected key algorithm id (ed25519, rsa-4096, …)
	// carried from CreateSpec through finishIdentity so CommitCreate never
	// defaults or reconstructs it (CR-10).
	Algorithm string
	// Provider is the validated provider host carried from CreateSpec through
	// finishIdentity so CommitCreate never re-derives it via providerFromAlias
	// (WR-01). Multi-label providers (e.g. "company.co.uk") survive intact.
	Provider string
}

// SSHStorageLayout is STORE-01's dual storage strategy for the SSH config:
// "sentinel" (blocks in place inside ~/.ssh/config) or "include"
// (gitid-owned ~/.ssh/config.d/gitid.config via one Include line).
type SSHStorageLayout string

// The two STORE-01 storage layouts.
const (
	// StorageSentinel keeps gitid blocks sentinel-delimited in place.
	StorageSentinel SSHStorageLayout = "sentinel"
	// StorageInclude moves gitid blocks to an Include'd owned file.
	StorageInclude SSHStorageLayout = "include"
)

// DemoFinding is a HealthFinding attributed to the identity it is about
// (drives per-identity health); empty Identity means a global finding.
type DemoFinding struct {
	HealthFinding
	Identity string
	// Rewrite, when non-nil, describes the D-09 surgical single-directive
	// rewrite this finding's fix performs — a tuikit-local mirror of
	// doctor.FixRewrite (internal/tuikit imports zero internal/doctor
	// identifiers, so it cannot reuse that type directly). Backend.FixPlanFor
	// reads it to render the real before/after diff and the typed-confirm
	// target. Nil for every finding that does not perform a surgical rewrite.
	Rewrite *FixRewriteTarget
}

// FixRewriteTarget is the tuikit-local mirror of doctor.FixRewrite: the Host
// stanza pattern, directive name, and new value a D-09 surgical rewrite
// targets.
type FixRewriteTarget struct {
	HostPattern string
	Directive   string
	NewValue    string
}

// DemoState is the whole live-demo state — the Go mirror of store.ts's
// DemoState. Reduce transitions it; nothing else mutates it.
type DemoState struct {
	Identities []DemoIdentity
	Findings   []DemoFinding
	// Scanned is whether the doctor scan has run at least once this session.
	Scanned bool
	// SSHApplied holds global-ssh option keys applied via the ceremony.
	SSHApplied []string
	// GitBaselineApplied is whether the global-git baseline was applied.
	GitBaselineApplied bool
	// GitGlobalName / GitGlobalEmail are the D-04 two-field global-fallback
	// pair — empty (unset) by default (recipes default preserved); set only
	// via its own dedicated apply ceremony, never folded into the baseline.
	GitGlobalName  string
	GitGlobalEmail string
	// SSHStorage is STORE-01's current layout.
	SSHStorage SSHStorageLayout
	// Backups holds timestamped backup paths "created" by write
	// ceremonies, newest first.
	Backups []string
}

// Action is one typed state transition — the Go mirror of store.ts's
// DemoAction union. Reduce is the only consumer.
type Action interface{ isAction() }

// AddIdentity appends a freshly-created identity (create wizard finish).
type AddIdentity struct {
	Identity DemoIdentity
	Backup   string
}

// ConfigureGit writes the Git side of an existing identity.
type ConfigureGit struct {
	Name          string
	GitName       string
	GitEmail      string
	MatchStrategy string
	GitDir        string
	ForceSSH      bool
	PublicKeyPath string
	Backup        string
}

// CloneIdentity copies Source as CloneName with its own new key path,
// Host alias, and fragment (MGR-04).
type CloneIdentity struct {
	Source    string
	CloneName string
}

// DeleteIdentity removes an identity — Scope "everything" drops the row
// and its findings; Scope "git-only" heals it to incomplete.
type DeleteIdentity struct {
	Name   string
	Scope  string // "everything" | "git-only"
	Backup string
}

// NewKey is the repair action that generates a fresh key at the identity's
// own canonical path and never touches pre-existing key material. Distinct
// from RotateIdentity, which is the retirement ceremony that archives.
type NewKey struct {
	Name   string
	Backup string
}

// RotateIdentity is the key-retirement ceremony: it archives the previous
// key and records the rotation. Distinct from NewKey, which is the repair
// action that never touches pre-existing key material.
type RotateIdentity struct {
	Name            string
	Backup          string
	ArchivedKeyPath string
}

// MarkScanned records that the doctor scan ran this session.
type MarkScanned struct{}

// FixFinding applies a doctor fix — the finding disappears and healed
// identities (legacy) change state.
type FixFinding struct {
	ID     string
	Backup string
}

// ApplySSH applies the chosen global-SSH option keys.
type ApplySSH struct {
	Keys   []string
	Backup string
}

// ApplyGitBaseline applies the global-git baseline managed block.
type ApplyGitBaseline struct {
	Backup string
}

// ApplyGitGlobalEmail applies the D-04 two-field global-fallback pair
// through its own dedicated ceremony — never folded into the baseline
// managed block.
type ApplyGitGlobalEmail struct {
	Name   string
	Email  string
	Backup string
}

// EditSSH rewrites an identity's managed Host block values.
type EditSSH struct {
	Name     string
	SSHHost  string
	Hostname string
	Port     int
	Backup   string
}

// SetSSHStorage migrates STORE-01's storage layout.
type SetSSHStorage struct {
	Layout SSHStorageLayout
	Backup string
}

// Reset restores the initial state. Reduce cannot answer it — only the
// Backend knows what "initial" means (fixtures for the dummy, a fresh read
// of the user's configuration for the real binary) — so a Backend's
// Persist implementation MUST handle Reset by returning InitialState().
type Reset struct{}

func (AddIdentity) isAction()         {}
func (ConfigureGit) isAction()        {}
func (CloneIdentity) isAction()       {}
func (DeleteIdentity) isAction()      {}
func (NewKey) isAction()              {}
func (RotateIdentity) isAction()      {}
func (MarkScanned) isAction()         {}
func (FixFinding) isAction()          {}
func (ApplySSH) isAction()            {}
func (ApplyGitBaseline) isAction()    {}
func (ApplyGitGlobalEmail) isAction() {}
func (EditSSH) isAction()             {}
func (SetSSHStorage) isAction()       {}
func (Reset) isAction()               {}

// AllActions returns one zero-value instance of every Action type, in
// declaration order (the order this file declares them). It is the explicit
// registry review R-08 replaces a reflection-based enumeration with: Go
// reflection cannot list the concrete types implementing an interface, so a
// completeness check over "every action" must have a hand-maintained source
// of truth — AllActions is that source. A new action MUST be added here.
// store_test.go's source-parsing test keeps this registry honest: it parses
// this file with go/ast and asserts the set of isAction() receivers equals
// the set of dynamic types AllActions returns.
func AllActions() []Action {
	return []Action{
		AddIdentity{},
		ConfigureGit{},
		CloneIdentity{},
		DeleteIdentity{},
		NewKey{},
		RotateIdentity{},
		MarkScanned{},
		FixFinding{},
		ApplySSH{},
		ApplyGitBaseline{},
		ApplyGitGlobalEmail{},
		EditSSH{},
		SetSSHStorage{},
		Reset{},
	}
}

// recomputeAfterGit is the state an identity lands in once BOTH its SSH
// and Git sides exist — the Go mirror of store.ts's recomputeAfterGit.
func recomputeAfterGit(row DemoIdentity) string {
	if row.SSHHost != "" {
		return "complete"
	}
	return "git-only"
}

// cloneState returns a deep copy of s so Reduce never mutates its input.
func cloneState(s DemoState) DemoState {
	next := s
	next.Identities = append([]DemoIdentity(nil), s.Identities...)
	next.Findings = append([]DemoFinding(nil), s.Findings...)
	next.SSHApplied = append([]string(nil), s.SSHApplied...)
	next.Backups = append([]string(nil), s.Backups...)
	return next
}

// Reduce applies action to state and returns the next state — a pure
// function mirroring store.ts's demoReducer transition-for-transition.
// The input state is never mutated. Reset is deliberately absent: it is
// the Backend's to answer (see Reset).
func Reduce(state DemoState, action Action) DemoState { //nolint:gocyclo // one case per action type, mirroring store.ts's switch verbatim
	next := cloneState(state)
	switch a := action.(type) {
	case AddIdentity:
		next.Identities = append(next.Identities, a.Identity)
		next.Backups = append([]string{a.Backup}, next.Backups...)
	case ConfigureGit:
		for i, row := range next.Identities {
			if row.Name != a.Name {
				continue
			}
			row.GitFragmentPath = "~/.gitconfig.d/" + row.Name
			row.GitName = a.GitName
			row.GitEmail = a.GitEmail
			row.MatchStrategy = a.MatchStrategy
			row.GitDir = a.GitDir
			row.ForceSSH = a.ForceSSH
			row.PublicKeyPath = a.PublicKeyPath
			row.GitConfigured = true
			row.State = recomputeAfterGit(row)
			row.Note = "SSH Host block and Git fragment both present."
			next.Identities[i] = row
		}
		next.Backups = append([]string{a.Backup}, next.Backups...)
	case CloneIdentity:
		var source *DemoIdentity
		for i := range next.Identities {
			if next.Identities[i].Name == a.Source {
				source = &next.Identities[i]
			}
			if next.Identities[i].Name == a.CloneName {
				return state // name taken — no-op, mirror store.ts
			}
		}
		if source == nil {
			return state
		}
		clone := *source
		clone.Name = a.CloneName
		if source.SSHHost != "" {
			clone.SSHHost = a.CloneName + ".github.com"
		}
		clone.KeyPath = "~/.ssh/id_ed25519_" + a.CloneName
		if source.GitFragmentPath != "" {
			clone.GitFragmentPath = "~/.gitconfig.d/" + a.CloneName
		}
		clone.Note = `Cloned from "` + a.Source + `" — new key + own Host block, same Git author.`
		next.Identities = append(next.Identities, clone)
	case DeleteIdentity:
		if a.Scope == "everything" {
			identities := next.Identities[:0]
			for _, row := range next.Identities {
				if row.Name != a.Name {
					identities = append(identities, row)
				}
			}
			next.Identities = identities
			findings := next.Findings[:0]
			for _, f := range next.Findings {
				if f.Identity != a.Name {
					findings = append(findings, f)
				}
			}
			next.Findings = findings
		} else {
			for i, row := range next.Identities {
				if row.Name != a.Name {
					continue
				}
				row.State = "incomplete"
				row.GitFragmentPath = ""
				row.GitName = ""
				row.GitEmail = ""
				row.MatchStrategy = ""
				row.Note = "SSH Host block present; Git identity was deleted."
				next.Identities[i] = row
			}
		}
		next.Backups = append([]string{a.Backup}, next.Backups...)
	case NewKey:
		for i, row := range next.Identities {
			if row.Name != a.Name {
				continue
			}
			row.KeyPath = "~/.ssh/id_ed25519_" + row.Name
			if row.State == "key-missing" {
				if row.GitFragmentPath != "" {
					row.State = "complete"
				} else {
					row.State = "incomplete"
				}
				row.Note = "New key generated; Host block re-points at it."
			}
			next.Identities[i] = row
		}
		next.Backups = append([]string{a.Backup}, next.Backups...)
	case RotateIdentity:
		found := false
		for i, row := range next.Identities {
			if row.Name != a.Name {
				continue
			}
			found = true
			row.Note = "Key rotated — previous key archived."
			next.Identities[i] = row
		}
		if !found {
			return state
		}
		next.Backups = append([]string{a.Backup}, next.Backups...)
	case MarkScanned:
		next.Scanned = true
	case FixFinding:
		found := false
		for _, f := range next.Findings {
			if f.ID == a.ID {
				found = true
			}
		}
		if !found {
			return state
		}
		if a.ID == "git-includeif-missing-fragment" {
			for i, row := range next.Identities {
				if row.Name != "legacy" {
					continue
				}
				row.State = "complete"
				if row.KeyPath == "" {
					row.KeyPath = "~/.ssh/id_ed25519_legacy"
				}
				row.Note = "Fragment restored — SSH Host block and Git fragment both present."
				next.Identities[i] = row
			}
		}
		findings := next.Findings[:0]
		for _, f := range next.Findings {
			if f.ID != a.ID {
				findings = append(findings, f)
			}
		}
		next.Findings = findings
		next.Backups = append([]string{a.Backup}, next.Backups...)
	case ApplySSH:
		for _, key := range a.Keys {
			present := false
			for _, existing := range next.SSHApplied {
				if existing == key {
					present = true
				}
			}
			if !present {
				next.SSHApplied = append(next.SSHApplied, key)
			}
		}
		next.Backups = append([]string{a.Backup}, next.Backups...)
	case ApplyGitBaseline:
		next.GitBaselineApplied = true
		next.Backups = append([]string{a.Backup}, next.Backups...)
	case ApplyGitGlobalEmail:
		next.GitGlobalName = a.Name
		next.GitGlobalEmail = a.Email
		next.Backups = append([]string{a.Backup}, next.Backups...)
	case EditSSH:
		for i, row := range next.Identities {
			if row.Name != a.Name {
				continue
			}
			row.SSHHost = a.SSHHost
			row.Hostname = a.Hostname
			row.Port = a.Port
			next.Identities[i] = row
		}
		next.Backups = append([]string{a.Backup}, next.Backups...)
	case SetSSHStorage:
		next.SSHStorage = a.Layout
		next.Backups = append([]string{a.Backup}, next.Backups...)
	case Reset:
		// Answered by the Backend's Persist (see Reset) — Reduce has no
		// notion of what the initial state is.
		return state
	}
	return next
}

// HealthRollup is the header health rollup: the worst live finding
// severity wins — "healthy", "warning", or "error".
func HealthRollup(state DemoState) string {
	rollup := "healthy"
	for _, f := range state.Findings {
		if f.Severity == SeverityError || f.Severity == SeverityCritical {
			return "error"
		}
		if f.Severity == SeverityWarning {
			rollup = "warning"
		}
	}
	return rollup
}

// FindingCounts holds the header chip's per-severity counts
// (`N ids · ! w · ✗ e`): errors folds error AND critical together.
type FindingCounts struct {
	Warnings int
	Errors   int
}

// CountFindings computes the live per-severity counts for the header chip.
func CountFindings(state DemoState) FindingCounts {
	var counts FindingCounts
	for _, f := range state.Findings {
		switch f.Severity {
		case SeverityWarning:
			counts.Warnings++
		case SeverityError, SeverityCritical:
			counts.Errors++
		case SeverityInfo:
		}
	}
	return counts
}

// FindingsFor returns the live findings attributed to identityName.
func FindingsFor(state DemoState, identityName string) []DemoFinding {
	var out []DemoFinding
	for _, f := range state.Findings {
		if f.Identity == identityName {
			out = append(out, f)
		}
	}
	return out
}

// NewBackupPath returns a fresh timestamped backup path in the same shape
// as the fixtures' (`<file>.backup.<ISO stamp with dashes for colons>`).
func NewBackupPath(file string) string {
	stamp := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	return file + ".backup." + strings.ReplaceAll(stamp, ":", "-")
}
