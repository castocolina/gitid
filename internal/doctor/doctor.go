// Package doctor performs health checks on a gitid-managed environment:
// key permissions, SSH config coherence, gitconfig coherence, orphaned managed
// blocks, signing key wiring, ssh-agent presence, and required tool availability.
// It never writes to any file — it returns structured findings only. Fix
// capabilities (chmod, block removal, wiring re-add) are injected as function
// fields on doctor.Deps so the cmd layer executes mutations without importing
// filewriter (D-01).
package doctor

import (
	"io"
	"os"

	"github.com/castocolina/gitid/internal/deps"
	"github.com/castocolina/gitid/internal/gitconfig"
	"github.com/castocolina/gitid/internal/identity"
	"github.com/castocolina/gitid/internal/sshconfig"
)

// Severity classifies the urgency of a finding. The four levels map directly
// to the D-05 bands and the tiered exit code (D-07).
type Severity int

const (
	// SeverityInfo is advisory — something optional is missing or suboptimal.
	SeverityInfo Severity = iota
	// SeverityWarning is degraded or risky but not immediately broken.
	SeverityWarning
	// SeverityError means broken — authentication or config resolution will fail.
	SeverityError
	// SeverityCritical means key/secret exposure — immediate action required.
	SeverityCritical
)

// String returns the canonical lowercase label for the severity level.
func (s Severity) String() string {
	switch s {
	case SeverityInfo:
		return "info"
	case SeverityWarning:
		return "warning"
	case SeverityError:
		return "error"
	case SeverityCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// Family is the named check category. Constants are the exact strings used in
// report headers and in the UI-SPEC fixed ordering.
type Family string

// Family constants define the seven check categories in the fixed UI-SPEC
// display order (Dependencies, Permissions, Coherence, Orphans, Signing,
// Agent, Baseline). Families() returns them in this order.
const (
	FamilyDeps      Family = "Dependencies"
	FamilyPerms     Family = "Permissions"
	FamilyCoherence Family = "Coherence"
	FamilyOrphans   Family = "Orphans"
	FamilySigning   Family = "Signing"
	FamilyAgent     Family = "Agent"
	FamilyBaseline  Family = "Baseline"
	// FamilyOverlap surfaces ambiguous/overlapping includeIf match conditions
	// across identities (DOC-08 / F-7). Severity is always warning (D-15).
	FamilyOverlap Family = "Overlap"
	// FamilyRedundancy surfaces SSH-config structural redundancy: multiple
	// "Host *" stanzas and duplicate global directives (UseKeychain /
	// AddKeysToAgent / IgnoreUnknown) across the user's pre-existing config
	// AND gitid's managed _global block (UAT G-4 / SSH-03 / DOC-08).
	// Severity is always SeverityWarning; Fix is always nil (advisory-only).
	FamilyRedundancy Family = "Redundancy"
)

// FixDescriptor carries metadata and the callable for an auto-fixable finding.
// The cmd layer calls Fn; internal/doctor never calls os.Chmod or filewriter
// directly (D-01).
type FixDescriptor struct {
	// Summary is the human-readable action (e.g. "chmod 0600 ~/.ssh/key").
	Summary string
	// Fn is the injected function that performs the fix when invoked.
	Fn func() error
	// Interactive, when non-nil, performs a richer fix that may prompt the user
	// (e.g. the baseline-missing fix runs the full `gitid baseline setup` flow so
	// it restores the fragment AND the include — not just a dangling pointer).
	// The cmd-layer apply gate prefers Interactive over Fn when set, threading the
	// shared stdin reader and out writer; assumeYes is true under --fix --yes
	// (apply with defaults, no prompts).
	Interactive func(in io.Reader, out io.Writer, assumeYes bool) error
}

// Finding is a single diagnostic result from one check family. Fix is nil for
// report-only findings (D-03); non-nil signals the cmd layer can auto-apply.
type Finding struct {
	Family       Family
	Severity     Severity
	Title        string
	Explanation  string
	SuggestedFix string
	Fix          *FixDescriptor
	// IdentityName is the name of the managed identity this finding belongs to.
	// Empty string means the finding is global (not scoped to a single identity).
	// Set by per-family check functions that iterate deps.Identities.
	// Used by the TUI to derive per-identity sidebar badge severity (D-08).
	IdentityName string
	// Target is the D-01 section this finding belongs to on the Health/Fixer
	// screens and the CLI: always "SSH" or "Git", never a third "System"
	// sub-label. Single-domain families (every Finding they emit is always
	// about the same file domain) leave Target unset on the literal and let
	// Run resolve it via defaultTargetForFamily. Families that mix SSH and
	// Git targets WITHIN themselves (Coherence, Orphans, Signing, Redundancy,
	// Deps) must set Target explicitly on every Finding literal — Run's
	// fallback returns "" for those families by design, so a missed literal
	// fails the D-01 guard test loudly instead of silently defaulting.
	Target string
}

// CheckFn is the type of a per-family check function. All seven check
// families implement this signature. The cmd layer wires concrete
// implementations from internal/doctor/checks into the Deps.Checks field
// so that doctor.Run can call them without importing the checks package
// (which itself imports doctor for Finding/Deps types — avoiding a cycle).
type CheckFn func(Deps) []Finding

// Deps holds every external read, injected-fix function field, and the per-family
// check functions that doctor.Run dispatches. The field set is the Wave-2 contract:
// Plans 02/03/04/05 wire against these exact names. Any change after 04-01-SUMMARY
// is published requires notifying all Wave-2 plans.
//
// Read fields:
//
//	ReadFile  — read a file by trusted path
//	Stat      — stat a trusted gitid-managed path (used by perms, coherence)
//
// Process fields:
//
//	RunSSHAdd                  — run "ssh-add -l", return (output, exitCode)
//	RunSSHKeygenFingerprint    — run "ssh-keygen -lf <path>", return (line, err)
//	RunGitConfigGet            — run "git config --file <file> <key>", return (val, err)
//
// Injected data and seams:
//
//	GitVersionAtLeast — gate on git major.minor
//	CurrentOS         — runtime.GOOS seam
//	InstallHint       — per-OS per-tool hint string
//
// Path fields:
//
//	SSHDir             — absolute path to ~/.ssh
//	SSHConfigPath      — absolute path to ~/.ssh/config
//	GitconfigPath      — absolute path to ~/.gitconfig
//	AllowedSignersPath — absolute path to ~/.ssh/allowed_signers
//
// Fix fields (injected, D-01 — doctor never calls os.Chmod or filewriter directly):
//
//	FixPerm      — chmod a path to a target mode
//	RemoveBlock  — remove a sentinel-delimited managed block from a file
//	AddWiring    — re-add a missing wiring line (allowed_signers, IdentitiesOnly)
//
// Check function fields (wired by cmd layer from internal/doctor/checks):
//
//	CheckDeps, CheckPerms, CheckCoherence, CheckOrphans,
//	CheckSigning, CheckAgent, CheckBaseline — the seven original per-family functions
//	CheckOverlap — the eighth check (DOC-08 / F-7); FamilyOverlap, SeverityWarning
//	CheckRedundancy — the ninth check (UAT G-4 / SSH-03); FamilyRedundancy, SeverityWarning, Fix nil
type Deps struct {
	// Read fields.
	ReadFile func(path string) ([]byte, error)
	Stat     func(path string) (os.FileInfo, error)

	// Process fields.
	RunSSHAdd               func() (string, int)
	RunSSHKeygenFingerprint func(path string) (string, error)
	RunGitConfigGet         func(file, key string) (string, error)

	// Injected data and seams.
	GitVersionAtLeast func(major, minor int) bool
	CurrentOS         func() string
	InstallHint       func(tool, os string) string
	// DetectTools probes PATH for required and optional tools. The cmd layer
	// wires deps.Detect; tests inject a fake returning a controlled deps.Report.
	DetectTools func() deps.Report
	// ReadBaselineState reconstructs the managed baseline state from disk.
	// The cmd layer wires gitconfig.ReadBaselineState; tests inject a fake.
	ReadBaselineState func(gitconfigPath, baselineFilePath, gitignorePath string) (gitconfig.BaselineState, error)

	// Path fields.
	SSHDir             string
	SSHConfigPath      string
	GitconfigPath      string
	AllowedSignersPath string
	// BaselineFilePath is the absolute path to ~/.gitconfig.d/00-baseline.
	BaselineFilePath string
	// GitignorePath is the absolute path to ~/.gitignore_global.
	GitignorePath string

	// Key and pub-key paths to check. These are the gitid-managed private key
	// paths (0600 targets) and their .pub counterparts (0644 targets). The cmd
	// layer fills them from the reconstructed identity list before calling Run.
	KeyPaths    []string
	PubKeyPaths []string

	// Identities is the pre-reconstructed identity list used by Coherence and
	// Orphans checks. The cmd layer wires identity.Reconstruct before calling Run
	// so the checks remain fake-testable (Plan 03 wave-2 fields).
	Identities []identity.Account
	// ManagedHosts is a map from identity name to SSHHostInfo for every
	// gitid-managed SSH Host block. Used by CheckCoherence for IdentitiesOnly
	// checks. The cmd layer wires sshconfig.ParseManagedHosts (Plan 03).
	ManagedHosts map[string]sshconfig.SSHHostInfo
	// GitconfigManagedBlockNames is the ordered list of identity names from all
	// gitid-managed includeIf blocks in ~/.gitconfig. Used by CheckOrphans to
	// detect fragment files on disk with no owning block (Plan 03).
	GitconfigManagedBlockNames []string
	// SSHManagedBlockNames is the ordered list of identity names from all
	// gitid-managed Host blocks in ~/.ssh/config. Used by CheckOrphans to detect
	// SSH Host blocks with no matching gitconfig includeIf (Plan 03).
	SSHManagedBlockNames []string
	// AllSSHHostIdentityFiles is every IdentityFile path from every Host block in
	// ~/.ssh/config — gitid-managed AND hand-written. Used by CheckOrphans for
	// the D-12 unused-key cross-reference (Plan 03).
	AllSSHHostIdentityFiles []string

	// Fix fields (cmd layer injects; doctor core never calls directly, D-01).
	FixPerm     func(path string, mode os.FileMode) error
	RemoveBlock func(path, name string) error
	AddWiring   func(path, name, line string) error
	// SetupBaseline runs the full `gitid baseline setup` flow (fragment + gitignore
	// + include, atomically, with prompts unless assumeYes). Wired by the cmd layer
	// so the baseline-missing finding's Interactive fix restores a COMPLETE baseline
	// rather than a dangling include pointer (Fix A). Nil in unit tests that do not
	// exercise the baseline fix.
	SetupBaseline func(in io.Reader, out io.Writer, assumeYes bool) error

	// Check function fields — wired by cmd layer from internal/doctor/checks so
	// doctor.Run dispatches without importing checks (avoids import cycle).
	// Wave 2 plans replace these fields with their real implementations.
	CheckDeps      CheckFn
	CheckPerms     CheckFn
	CheckCoherence CheckFn
	CheckOrphans   CheckFn
	CheckSigning   CheckFn
	CheckAgent     CheckFn
	CheckBaseline  CheckFn
	// CheckOverlap detects ambiguous/overlapping includeIf match conditions across
	// identities (DOC-08 / F-7). Called after CheckOrphans; SeverityWarning only.
	CheckOverlap CheckFn
	// CheckRedundancy detects SSH-config structural redundancy: multiple "Host *"
	// stanzas and duplicate global directives (UseKeychain / AddKeysToAgent /
	// IgnoreUnknown) across the whole ~/.ssh/config (UAT G-4 / SSH-03 / DOC-08).
	// Advisory-only: SeverityWarning, Fix nil, never blocks doctor or any write flow.
	// Called last in Run — appended after CheckOverlap (nil-guarded).
	CheckRedundancy CheckFn
}

// Run calls all check families in the fixed UI-SPEC order and returns the
// aggregated findings slice. Each check function is called only when its Deps
// field is non-nil (nil == stub not yet wired). Run never imports filewriter or
// os.Chmod — fix capabilities are injected via deps (D-01).
// Order: Dependencies, Permissions, Coherence, Orphans, Signing, Agent, Baseline,
// Overlap, Redundancy.
//
// D-01: every returned Finding carries a resolved, non-empty Target ("SSH" or
// "Git"). A Finding whose construction site already set Target explicitly
// (the cross-file families: Coherence, Orphans, Signing, Redundancy, Deps —
// see defaultTargetForFamily's own doc comment) keeps that value unchanged;
// every other Finding's empty Target is resolved from its Family via
// defaultTargetForFamily. This resolution happens ONCE, here, so every
// consumer (the Health/Fixer TUI screens, gitid health --json, the
// HLTH-05/MGR-07 per-identity slice) sees the same Target.
func Run(deps Deps) []Finding {
	var all []Finding
	for _, fn := range []CheckFn{
		deps.CheckDeps,
		deps.CheckPerms,
		deps.CheckCoherence,
		deps.CheckOrphans,
		deps.CheckSigning,
		deps.CheckAgent,
		deps.CheckBaseline,
		deps.CheckOverlap,
		deps.CheckRedundancy,
	} {
		if fn == nil {
			continue
		}
		for _, f := range fn(deps) {
			if f.Target == "" {
				f.Target = defaultTargetForFamily(f.Family)
			}
			all = append(all, f)
		}
	}
	return all
}

// defaultTargetForFamily returns the D-01 family-default Target for
// families whose findings are ALWAYS single-domain — verified against each
// family's own internal/doctor/checks/*.go source, not guessed:
//
//   - FamilyPerms: mostly SSH (ssh dir, private/public keys, ssh config) but
//     checkGitconfigPath also emits a FamilyPerms finding for ~/.gitconfig's
//     write-access risk — that ONE call site sets Target explicitly
//     ("Git"), so the family default here only ever backfills the four
//     SSH-domain checkPath call sites that leave Target unset.
//   - FamilyBaseline: every finding is about ~/.gitconfig's baseline
//     [include] block, core.excludesfile/core.ignorecase, or the curated
//     ~/.gitignore_global patterns — always Git.
//   - FamilyOverlap: every finding is about overlapping gitconfig includeIf
//     match conditions — always Git.
//   - FamilyAgent: every finding is about ssh-agent reachability or a
//     gitid-managed key not being loaded in it — always SSH.
//
// FamilyCoherence, FamilyOrphans, FamilySigning, FamilyRedundancy, and
// FamilyDeps mix SSH- and Git-domain findings WITHIN the same family (or are
// explicitly required by D-01 to route per-tool) and therefore return "" —
// every Finding literal in those families' checks/*.go files sets Target
// explicitly, and Run's resolution deliberately does not paper over a
// missed literal with a guessed default.
func defaultTargetForFamily(f Family) string {
	switch f {
	case FamilyPerms:
		return "SSH"
	case FamilyBaseline:
		return "Git"
	case FamilyOverlap:
		return "Git"
	case FamilyAgent:
		return "SSH"
	default:
		return ""
	}
}

// ExitCode returns the tiered exit code for a findings slice (D-07):
//
//	0 — no findings
//	1 — highest severity is warning or info
//	2 — highest severity is error
//	3 — highest severity is critical
func ExitCode(findings []Finding) int {
	if len(findings) == 0 {
		return 0
	}
	return severityToCode(highestSeverity(findings))
}

// highestSeverity returns the highest Severity value present in findings.
// Caller must ensure findings is non-empty.
func highestSeverity(findings []Finding) Severity {
	top := findings[0].Severity
	for _, f := range findings[1:] {
		if f.Severity > top {
			top = f.Severity
		}
	}
	return top
}

// severityToCode maps a Severity to the D-07 tiered exit code.
func severityToCode(s Severity) int {
	switch s {
	case SeverityCritical:
		return 3
	case SeverityError:
		return 2
	default: // SeverityWarning, SeverityInfo
		return 1
	}
}

// Families returns all family constants in the fixed UI-SPEC display order:
// Dependencies, Permissions, Coherence, Orphans, Signing, Agent, Baseline,
// Overlap, Redundancy.
// FamilyOverlap and FamilyRedundancy are listed last so they render as advisory
// sections after the structural/integrity checks (D-15 — severity warning, not
// blocking). FamilyRedundancy is appended after FamilyOverlap (UAT G-4 / SSH-03).
func Families() []Family {
	return []Family{
		FamilyDeps,
		FamilyPerms,
		FamilyCoherence,
		FamilyOrphans,
		FamilySigning,
		FamilyAgent,
		FamilyBaseline,
		FamilyOverlap,
		FamilyRedundancy,
	}
}
