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
}

// CheckFn is the type of a per-family check function. All seven check
// families implement this signature. The cmd layer wires concrete
// implementations from internal/doctor/checks into the Deps.Checks field
// so that doctor.Run can call them without importing the checks package
// (which itself imports doctor for Finding/Deps types — avoiding a cycle).
type CheckFn func(Deps) []Finding

// Deps holds every external read, injected-fix function field, and the seven
// per-family check functions that doctor.Run dispatches. The field set is the
// Wave-2 contract: Plans 02/03/04/05 wire against these exact names. Any
// change after 04-01-SUMMARY is published requires notifying all Wave-2 plans.
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
//	CheckSigning, CheckAgent, CheckBaseline — the seven per-family functions
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
}

// Run calls all seven check families in the fixed UI-SPEC order and returns
// the aggregated findings slice. Each check function is called only when its
// Deps field is non-nil (nil == stub not yet wired). Run never imports
// filewriter or os.Chmod — fix capabilities are injected via deps (D-01).
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
	} {
		if fn != nil {
			all = append(all, fn(deps)...)
		}
	}
	return all
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

// Families returns the seven family constants in the fixed UI-SPEC display
// order: Dependencies, Permissions, Coherence, Orphans, Signing, Agent, Baseline.
func Families() []Family {
	return []Family{
		FamilyDeps,
		FamilyPerms,
		FamilyCoherence,
		FamilyOrphans,
		FamilySigning,
		FamilyAgent,
		FamilyBaseline,
	}
}
