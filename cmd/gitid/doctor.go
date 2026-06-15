package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/castocolina/gitid/internal/deps"
	"github.com/castocolina/gitid/internal/doctor"
	"github.com/castocolina/gitid/internal/doctor/checks"
	"github.com/castocolina/gitid/internal/filewriter"
	"github.com/castocolina/gitid/internal/gitconfig"
	"github.com/castocolina/gitid/internal/identity"
	"github.com/castocolina/gitid/internal/keygen"
	"github.com/castocolina/gitid/internal/platform"
	"github.com/castocolina/gitid/internal/sshconfig"
)

// doctorExitCode holds the tiered exit code (0/1/2/3) set by the doctor RunE
// so that main() can propagate it to os.Exit instead of collapsing to a flat 1
// (IN-03). Zero is the safe default for all other commands.
var doctorExitCode int

// newDoctorCmd builds `gitid doctor`. The handler runs the full health
// check suite and renders a grouped-by-family report. --fix and --yes flags
// are declared here; full apply-fixes gate logic is Plan 05.
func newDoctorCmd() *cobra.Command {
	var fix, yes bool
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Run a health check on the gitid-managed environment",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if yes && !fix {
				return fmt.Errorf("doctor: --yes requires --fix")
			}
			// Store the tiered exit code (0/1/2/3) for main() to propagate via
			// os.Exit (IN-03). We deliberately do NOT return it as an error:
			// doing so made Cobra print a spurious "Error: exit code N" line even
			// after --fix had repaired everything (Bug B). main() reads
			// doctorExitCode directly, so real errors (e.g. "--yes requires
			// --fix") still print while the tiered code stays silent.
			doctorExitCode = runDoctor(cmd.OutOrStdout(), fix, yes)
			return nil
		},
	}
	// SilenceUsage prevents Cobra printing the usage block on RunE errors.
	cmd.SilenceUsage = true
	cmd.Flags().BoolVar(&fix, "fix", false, "apply auto-fixable findings (per-finding confirm)")
	cmd.Flags().BoolVar(&yes, "yes", false, "apply all fixes without prompts (requires --fix; SAFE-03)")
	return cmd
}

// runDoctor is the report orchestration handler. It resolves home, reads the
// two managed config files, builds doctor.Deps, runs all checks, renders the
// report, and returns the D-07 tiered exit code (0/1/2/3). The caller
// (RunE / tests) translates the return value.
//
// Exit-code semantics (Bug B):
//   - A plain `gitid doctor` (no fix gate entered) returns the PRE-fix severity
//     so a diagnosing/CI caller is never misled into thinking the env was healthy.
//   - A fix run (--fix, --fix --yes, or an accepted interactive gate) APPLIES
//     fixes, RE-EVALUATES from disk, and returns the POST-fix severity. The
//     caller asked to fix; the honest answer is the state after fixing. The
//     fix→re-check loop repeats until nothing fixable remains or a pass makes no
//     progress (convergeFixes), so a single `--fix --yes` heals what it can in
//     one invocation instead of requiring the user to re-run it.
func runDoctor(out io.Writer, fix, yes bool) int {
	home, err := os.UserHomeDir()
	if err != nil {
		fp(out, fmt.Sprintf("doctor: resolving home dir: %v\n", err))
		return 2
	}

	sshBytes, gcBytes, rerr := readManagedConfigs(out, home)
	if rerr != 0 {
		return rerr
	}

	findings := doctor.Run(buildDoctorDeps(home, sshBytes, gcBytes))

	// D-17: append install-info finding (SeverityInfo) so it shows under
	// Dependencies but never raises the exit tier.
	findings = append(findings, installInfoFinding())

	pre := doctor.ExitCode(findings)

	colorEnabled := isTerminalOutput(os.Stdout)
	renderReport(out, findings, colorEnabled)

	// DOC-GAP-03: only enter the fix flow when --fix/--yes was passed or stdin is
	// a TTY. A bare `gitid doctor` in a pipe/CI skips it — the pre-fix exit code
	// is the machine-readable signal.
	inFixMode := fix || isTerminalInput(os.Stdin)
	if len(collectFixable(findings)) == 0 || !inFixMode {
		return pre
	}

	// Fix + re-evaluate until clean or stuck. Re-reading disk and re-running every
	// check after each pass is the single source of truth for "is it fixed yet?".
	in := bufio.NewReader(os.Stdin)
	const maxPasses = 10
	final := convergeFixes(
		findings,
		func(fixable []doctor.Finding) int {
			applied, _ := applyFixes(in, out, fixable, fix, yes)
			return applied
		},
		func() []doctor.Finding {
			sb, gb, code := readManagedConfigs(out, home)
			if code != 0 {
				return nil
			}
			return doctor.Run(buildDoctorDeps(home, sb, gb))
		},
		maxPasses,
	)

	// Re-render only when the state actually changed (avoids a duplicate report
	// when the user declined every fix). Return the POST-fix severity.
	if findingsSignature(final) != findingsSignature(findings) {
		fp(out, "\n")
		renderReport(out, final, colorEnabled)
	}
	return doctor.ExitCode(final)
}

// readManagedConfigs reads the two gitid-managed config files for a home dir.
// A missing file is not an error (returns empty bytes); any other read error
// prints to out and returns errCode 2 so the caller can abort.
func readManagedConfigs(out io.Writer, home string) (sshBytes, gcBytes []byte, errCode int) {
	sshConfigPath := filepath.Join(home, ".ssh", "config")
	gitconfigPath := filepath.Join(home, ".gitconfig")

	sb, err := os.ReadFile(sshConfigPath) //nolint:gosec // gitid-managed path (G304)
	if err != nil && !os.IsNotExist(err) {
		fp(out, fmt.Sprintf("doctor: reading %s: %v\n", sshConfigPath, err))
		return nil, nil, 2
	}
	gb, err := os.ReadFile(gitconfigPath) //nolint:gosec // gitid-managed path (G304)
	if err != nil && !os.IsNotExist(err) {
		fp(out, fmt.Sprintf("doctor: reading %s: %v\n", gitconfigPath, err))
		return nil, nil, 2
	}
	return sb, gb, 0
}

// collectFixable returns the findings that carry a Fix descriptor, in order.
func collectFixable(findings []doctor.Finding) []doctor.Finding {
	var fixable []doctor.Finding
	for _, f := range findings {
		if f.Fix != nil {
			fixable = append(fixable, f)
		}
	}
	return fixable
}

// findingsSignature returns a stable, order-independent signature of a finding
// set (sorted family|title pairs). The convergence loop uses it to detect a pass
// that changed nothing observable, so it stops instead of spinning.
func findingsSignature(findings []doctor.Finding) string {
	parts := make([]string, 0, len(findings))
	for _, f := range findings {
		parts = append(parts, string(f.Family)+"|"+f.Title)
	}
	sort.Strings(parts)
	return strings.Join(parts, "\n")
}

// convergeFixes repeatedly applies fixable findings and re-evaluates until no
// fixable findings remain, a pass makes no observable progress (identical
// signature), or maxPasses is reached (a hard backstop guaranteeing termination
// even if two checks ever disagree about the same artifact). It returns the
// final, re-evaluated findings. apply applies the supplied fixable findings and
// returns how many were applied; runChecks re-reads disk and re-runs every check.
func convergeFixes(
	initial []doctor.Finding,
	apply func(fixable []doctor.Finding) int,
	runChecks func() []doctor.Finding,
	maxPasses int,
) []doctor.Finding {
	findings := initial
	prevSig := findingsSignature(findings)
	for pass := 0; pass < maxPasses; pass++ {
		fixable := collectFixable(findings)
		if len(fixable) == 0 {
			break
		}
		if apply(fixable) == 0 {
			break // nothing applied (declined / all failed) — no progress possible
		}
		findings = runChecks()
		sig := findingsSignature(findings)
		if sig == prevSig {
			break // pass changed nothing observable — stop instead of spinning
		}
		prevSig = sig
	}
	return findings
}

// runSSHAdd runs `ssh-add -l` via arg-slice exec (no shell, G204-clean) and
// returns the combined output and the exit code. On a non-ExitError exec
// failure (binary not found, permission error) it returns ("", 2) so
// classifyAgentState treats it as unreachable — consistent with the semantics
// of an inaccessible agent. (DOC-GAP-02 runner; mitigates T-04-22.)
func runSSHAdd() (string, int) {
	cmd := exec.Command("ssh-add", "-l") //nolint:gosec // arg-slice form, no shell; fixed args (G204)
	out, err := cmd.CombinedOutput()
	output := string(out)
	if err == nil {
		return output, 0
	}
	var exitErr *exec.ExitError
	if ok := errors.As(err, &exitErr); ok {
		return output, exitErr.ExitCode()
	}
	// Non-ExitError (e.g. binary not found) → treat as unreachable.
	return "", 2
}

// runSSHKeygenFingerprint runs `ssh-keygen -lf <path>` via arg-slice exec
// (no shell, G204-clean) and returns the first output line and any error.
// path is a gitid-managed .pub path (G304-annotated). (DOC-GAP-02 runner;
// mitigates T-04-22.)
func runSSHKeygenFingerprint(path string) (string, error) {
	cmd := exec.Command("ssh-keygen", "-lf", path) //nolint:gosec // arg-slice form, no shell; path is trusted gitid-managed .pub (G204/G304)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	line := strings.SplitN(string(out), "\n", 2)[0]
	return line, nil
}

// installInfoFinding calls platform.BinaryInstallInfo and returns a SeverityInfo
// finding under FamilyDeps (D-17). It reports the resolved binary path and whether
// it is on PATH, with an export hint when not found. Info severity means it is
// advisory only and never changes the exit tier.
func installInfoFinding() doctor.Finding {
	binPath, onPATH, err := platform.BinaryInstallInfo()
	if err != nil {
		return doctor.Finding{
			Family:      doctor.FamilyDeps,
			Severity:    doctor.SeverityInfo,
			Title:       "binary location: unknown",
			Explanation: fmt.Sprintf("could not resolve binary path: %v", err),
		}
	}

	title := fmt.Sprintf("binary: %s", binPath)
	var explanation, suggestedFix string
	if onPATH {
		explanation = "on PATH: yes"
	} else {
		binDir := filepath.Dir(binPath)
		explanation = "on PATH: no"
		suggestedFix = fmt.Sprintf(`export PATH="$PATH:%s"`, binDir)
	}

	return doctor.Finding{
		Family:       doctor.FamilyDeps,
		Severity:     doctor.SeverityInfo,
		Title:        title,
		Explanation:  explanation,
		SuggestedFix: suggestedFix,
		Fix:          nil,
	}
}

// buildDoctorDeps wires real packages into doctor.Deps. The FixPerm field
// closes over os.Chmod so internal/doctor never imports os.Chmod directly
// (D-01 approach b). The check-function fields are wired from checks.*
// here (in the cmd layer) to avoid an import cycle: checks imports doctor,
// so doctor must not import checks.
func buildDoctorDeps(home string, sshBytes, gcBytes []byte) doctor.Deps {
	sshConfigPath := filepath.Join(home, ".ssh", "config")
	gitconfigPath := filepath.Join(home, ".gitconfig")
	allowedSignersPath := filepath.Join(home, ".ssh", "allowed_signers")
	sshDir := filepath.Join(home, ".ssh")

	// Reconstruct identity list — used by Coherence, Orphans, and Perms checks.
	accounts, _ := identity.Reconstruct(sshBytes, gcBytes, gitconfig.ReadFragment)
	var keyPaths, pubKeyPaths []string
	for _, a := range accounts {
		if a.KeyPath != "" {
			keyPaths = append(keyPaths, a.KeyPath)
		}
		if a.PubPath != "" {
			pubKeyPaths = append(pubKeyPaths, a.PubPath)
		}
	}

	// Build ManagedHosts and SSHManagedBlockNames for the Coherence/Orphans checks.
	managedHosts, _ := sshconfig.ParseManagedHosts(sshBytes)
	sshBlockNames := make([]string, 0, len(managedHosts))
	for name := range managedHosts {
		sshBlockNames = append(sshBlockNames, name)
	}

	// Build GitconfigManagedBlockNames from the raw gitconfig blocks.
	gcBlocks := filewriter.ListBlocks(gcBytes)
	gcBlockNames := make([]string, 0, len(gcBlocks))
	for _, b := range gcBlocks {
		gcBlockNames = append(gcBlockNames, b.Name)
	}

	// Build AllSSHHostIdentityFiles — every IdentityFile from every Host block
	// (managed + hand-written) for the D-12 unused-key cross-reference.
	allSSHHostIDFiles := sshconfig.ParseAllHostIdentityFiles(sshBytes)

	baselineFilePath := filepath.Join(home, ".gitconfig.d", "00-baseline")
	gitignorePath := filepath.Join(home, ".gitignore_global")

	return doctor.Deps{
		// Read fields.
		ReadFile: func(path string) ([]byte, error) {
			return os.ReadFile(path) //nolint:gosec // path is a trusted gitid-managed path (G304)
		},
		Stat: func(path string) (os.FileInfo, error) {
			return os.Stat(path) //nolint:gosec // path is a trusted gitid-managed path (G304)
		},

		// Process fields.
		// DOC-GAP-02: wire real ssh-add -l and ssh-keygen -lf runners so
		// CheckAgent and CheckSigning can probe the running ssh-agent.
		// Both use arg-slice exec (no shell) for G204 compliance (T-04-22).
		RunSSHAdd:               runSSHAdd,
		RunSSHKeygenFingerprint: runSSHKeygenFingerprint,
		RunGitConfigGet: func(file, key string) (string, error) {
			return gitconfig.RunGitConfigGet(file, key)
		},

		// Injected data and seams.
		GitVersionAtLeast: deps.GitVersionAtLeast,
		CurrentOS:         platform.CurrentOS,
		InstallHint:       platform.InstallHint,
		DetectTools:       deps.Detect,
		ReadBaselineState: gitconfig.ReadBaselineState,

		// Path fields.
		SSHDir:             sshDir,
		SSHConfigPath:      sshConfigPath,
		GitconfigPath:      gitconfigPath,
		AllowedSignersPath: allowedSignersPath,
		BaselineFilePath:   baselineFilePath,
		GitignorePath:      gitignorePath,

		// Key path lists for perms check.
		KeyPaths:    keyPaths,
		PubKeyPaths: pubKeyPaths,

		// Coherence + Orphans data (Plan 03 wave-2 fields).
		Identities:                 accounts,
		ManagedHosts:               managedHosts,
		SSHManagedBlockNames:       sshBlockNames,
		GitconfigManagedBlockNames: gcBlockNames,
		AllSSHHostIdentityFiles:    allSSHHostIDFiles,

		// Fix fields (D-01: cmd layer owns chmod/write, doctor core does not import filewriter).

		// FixPerm chmods the file to the caller-supplied mode via os.Chmod. The
		// tighten-only guarantee is enforced upstream in checks/perms.go: checkPath
		// flags only when got &^ want != 0 and passes got & want as the mode, so
		// FixPerm is never called with a mode that adds a bit the file lacked.
		FixPerm: func(path string, mode os.FileMode) error {
			return os.Chmod(path, mode) //nolint:gosec // chmod to caller-supplied tighten-only mode (G306)
		},

		// RemoveBlock removes a sentinel-delimited managed block from a file using
		// filewriter.RemoveBlock (idempotent splice) + filewriter.Write (atomic + backup).
		// Mitigates T-04-16/T-04-17: only the targeted block is removed, content
		// outside the block is preserved byte-for-byte, and a timestamped backup is
		// created before every mutation. A second call with the same name is idempotent
		// (filewriter.RemoveBlock returns input unchanged when the block is absent).
		//
		// WR-02: the file mode is derived from the target path. ~/.ssh/allowed_signers
		// is a public file and must remain 0644 after removal of an orphaned signer block.
		// All other gitid-managed config files (~/.ssh/config, ~/.gitconfig) are 0600.
		RemoveBlock: func(path, name string) error {
			content, err := os.ReadFile(path) //nolint:gosec // path is a gitid-managed trusted path (G304)
			if err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("doctor: reading %s for block removal: %w", path, err)
			}
			removed := filewriter.RemoveBlock(content, name)
			// Path-derived mode: allowed_signers is 0644 (public); all others 0600 (KEY-02 / T-04-19).
			mode := os.FileMode(0o600)
			if path == allowedSignersPath {
				mode = 0o644
			}
			if _, werr := filewriter.Write(path, removed, mode); werr != nil {
				return fmt.Errorf("doctor: removing block %q from %s: %w", name, path, werr)
			}
			return nil
		},

		// AddWiring dispatches to the correct existing writer for the finding being
		// fixed. The `path` and `name` parameters identify the target file and identity;
		// `line` carries the family-specific payload:
		//   - SSH config IdentitiesOnly re-add: line == "ssh-host:<alias>:<hostname>:<port>:<keyPath>"
		//   - allowed_signers entry re-add:     line == "signers:<email>:<pubLine>"
		//   - baseline [include] restore:       line == "baseline-include:<baselineFilePath>"
		// Each sub-path delegates entirely to an existing writer (sshconfig.Write,
		// keygen.WriteAllowedSigners, gitconfig.WriteBaselineInclude) that routes
		// through filewriter (backup + atomic + idempotent) — no direct os.WriteFile (CLAUDE.md).
		AddWiring: func(path, name, line string) error {
			switch {
			case strings.HasPrefix(line, "ssh-host:"):
				// Re-add a complete SSH Host block (with IdentitiesOnly yes) via sshconfig.Write.
				// Format: "ssh-host:<alias>:<hostname>:<port>:<keyPath>"
				rest := strings.TrimPrefix(line, "ssh-host:")
				parts := strings.SplitN(rest, ":", 4)
				if len(parts) != 4 {
					return fmt.Errorf("doctor: AddWiring ssh-host: malformed line %q", line)
				}
				alias, hostname, portStr, identityFile := parts[0], parts[1], parts[2], parts[3]
				port := 22
				if portStr != "" {
					if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil {
						port = 22
					}
				}
				hostBlock := sshconfig.RenderHostBlock(alias, hostname, port, identityFile, "")
				globalBlock := sshconfig.RenderGlobalBlock(platform.CurrentOS())
				if _, err := sshconfig.Write(path, name, hostBlock, globalBlock); err != nil {
					return fmt.Errorf("doctor: AddWiring ssh-host for %q: %w", name, err)
				}
			case strings.HasPrefix(line, "signers:"):
				// Re-add a missing allowed_signers line via keygen.WriteAllowedSigners.
				// Format: "signers:<email>:<pubLine>"
				rest := strings.TrimPrefix(line, "signers:")
				parts := strings.SplitN(rest, ":", 2)
				if len(parts) != 2 {
					return fmt.Errorf("doctor: AddWiring signers: malformed line %q", line)
				}
				email, pubLine := parts[0], parts[1]
				signerLine := keygen.AllowedSignersLine(email, pubLine)
				if _, err := keygen.WriteAllowedSigners(path, name, signerLine); err != nil {
					return fmt.Errorf("doctor: AddWiring signers for %q: %w", name, err)
				}
			case strings.HasPrefix(line, "baseline-include:"):
				// Restore a missing baseline [include] block via gitconfig.WriteBaselineInclude.
				// Format: "baseline-include:<baselineFilePath>"
				baselineFilePath := strings.TrimPrefix(line, "baseline-include:")
				if _, err := gitconfig.WriteBaselineInclude(path, baselineFilePath); err != nil {
					return fmt.Errorf("doctor: AddWiring baseline-include: %w", err)
				}
			default:
				return fmt.Errorf("doctor: AddWiring: unknown wiring type in line %q", line)
			}
			return nil
		},

		// SetupBaseline runs the full interactive baseline setup so the
		// baseline-missing fix restores a COMPLETE baseline (fragment + gitignore +
		// include), not a dangling include pointer (Fix A). assumeYes (--fix --yes)
		// writes defaults without prompts.
		SetupBaseline: func(in io.Reader, out io.Writer, assumeYes bool) error {
			return runBaselineSetup(in, out, false /* dryRun */, assumeYes)
		},

		// Check function fields wired from internal/doctor/checks.
		// Wave 2 plans replace these in place (same paths, same signatures).
		CheckPerms:     checks.CheckPermissions,
		CheckDeps:      checks.CheckDeps,
		CheckCoherence: checks.CheckCoherence,
		CheckOrphans:   checks.CheckOrphans,
		CheckSigning:   checks.CheckSigning,
		CheckAgent:     checks.CheckAgent,
		CheckBaseline:  checks.CheckBaseline,
		// DOC-08 / F-7: overlap detector. Identities is already populated above.
		CheckOverlap: checks.CheckOverlap,
	}
}

// renderReport iterates all seven families in fixed UI-SPEC order, printing
// a `=== Family ===` header and then either findings or a ✓ pass line.
// The summary line and exit-code line are printed at the end.
func renderReport(out io.Writer, findings []doctor.Finding, colorEnabled bool) {
	// Index findings by family for O(1) lookup.
	byFamily := make(map[doctor.Family][]doctor.Finding, len(doctor.Families()))
	for _, f := range findings {
		byFamily[f.Family] = append(byFamily[f.Family], f)
	}

	// Count by severity for the summary line.
	counts := [4]int{} // indexed by Severity iota

	first := true
	for _, fam := range doctor.Families() {
		if !first {
			fp(out, "\n")
		}
		first = false

		header := fmt.Sprintf("=== %s ===", string(fam))
		fp(out, ansi("1", header, colorEnabled)+"\n")

		famFindings := byFamily[fam]
		if len(famFindings) == 0 {
			fp(out, ansi("32", "  ✓", colorEnabled)+" all checks passed\n")
			continue
		}

		for _, f := range famFindings {
			counts[f.Severity]++
			fp(out, renderFinding(f, colorEnabled))
		}
	}

	// Summary line.
	fp(out, "\n---\n")
	totalCrit := counts[doctor.SeverityCritical]
	totalErr := counts[doctor.SeverityError]
	totalWarn := counts[doctor.SeverityWarning]
	totalInfo := counts[doctor.SeverityInfo]

	if totalCrit == 0 && totalErr == 0 && totalWarn == 0 && totalInfo == 0 {
		fp(out, "doctor: all checks passed\n")
		fp(out, "exit code: 0\n")
		return
	}

	fp(out, fmt.Sprintf("doctor: %d critical, %d error, %d warning, %d info\n",
		totalCrit, totalErr, totalWarn, totalInfo))
	fp(out, fmt.Sprintf("exit code: %d\n", doctor.ExitCode(findings)))
}

// renderFinding formats a single finding per the UI-SPEC layout contract:
//
//	✗ title [severity label if not error]
//	  explanation
//	  fix: suggested command
//	  [fix]  ← only when Fix is non-nil
func renderFinding(f doctor.Finding, colorEnabled bool) string {
	var s string

	// Glyph + title line (severity-colored).
	glyph := "  ✗ "
	if f.Severity == doctor.SeverityInfo {
		glyph = "  ! "
	}
	colorCode := severityCode(f.Severity)
	titleLine := ansi(colorCode, glyph+f.Title, colorEnabled)

	// Inline severity label (omit for error — ✗ implies it).
	switch f.Severity {
	case doctor.SeverityCritical:
		titleLine += " [critical]"
	case doctor.SeverityWarning:
		titleLine += " [warning]"
	case doctor.SeverityInfo:
		titleLine += " [info]"
	}
	s += titleLine + "\n"

	// Explanation (4-space indent).
	if f.Explanation != "" {
		s += "    " + f.Explanation + "\n"
	}

	// Suggested fix (4-space indent, dim).
	if f.SuggestedFix != "" {
		s += ansi("2", "    fix: "+f.SuggestedFix, colorEnabled) + "\n"
	}

	// Fixable marker.
	if f.Fix != nil {
		s += "    [fix]\n"
	}

	return s
}

// ansi wraps text in an ANSI SGR escape when colorEnabled is true.
func ansi(code, text string, colorEnabled bool) string {
	if !colorEnabled {
		return text
	}
	return "\033[" + code + "m" + text + "\033[0m"
}

// severityCode maps a Severity to the ANSI SGR color code string.
func severityCode(s doctor.Severity) string {
	switch s {
	case doctor.SeverityCritical, doctor.SeverityError:
		return "31" // red
	case doctor.SeverityWarning:
		return "33" // yellow
	default: // info
		return "36" // cyan
	}
}

// applyFixes implements the D-04 consent flow for the auto-fixable findings.
//
// fix=false: present a top-level gate ("Apply N fix(es)?" default N); on y
// proceed to per-finding confirm. fix=true: skip the gate, go straight to
// per-finding confirm. fix+yes: apply every fixable finding silently.
//
// Batching rule (D-04 hard rule):
//   - FamilyPerms findings MAY be batched under one "Fix N permission(s):" confirm.
//   - FamilyOrphans and any other family (Coherence/Baseline) are NEVER batched;
//     each gets its own individual confirm (higher blast radius).
//
// Returns (applied, skipped) counts. On Fix.Fn error, prints the error and
// continues to the next finding (does not abort the whole run). After all
// findings are processed, prints the tally line.
func applyFixes(r *bufio.Reader, out io.Writer, findings []doctor.Finding, fix, yes bool) (applied, skipped int) {
	if len(findings) == 0 {
		return 0, 0
	}

	// Top-level gate: bare `gitid doctor` (fix==false) presents a single gate.
	// On decline, print "No fixes applied." and return immediately.
	if !fix {
		label := fmt.Sprintf("Apply %d fix(es)?", len(findings))
		if !confirm(r, out, label) {
			fp(out, "No fixes applied.\n")
			return 0, 0
		}
	}

	// Separate permission findings from non-permission findings (in report order).
	var permFindings []doctor.Finding
	var otherFindings []doctor.Finding
	for _, f := range findings {
		if f.Family == doctor.FamilyPerms {
			permFindings = append(permFindings, f)
		} else {
			otherFindings = append(otherFindings, f)
		}
	}

	// Process permission findings as a batch (D-04 batching rule).
	if len(permFindings) > 0 {
		if yes {
			// --fix --yes: apply all silently.
			for _, f := range permFindings {
				if err := f.Fix.Fn(); err != nil {
					fp(out, fmt.Sprintf("doctor: fix failed: %s: %v\n", f.Fix.Summary, err))
				} else {
					fp(out, fmt.Sprintf("  fixed: %s\n", f.Fix.Summary))
					applied++
				}
			}
		} else {
			// Per-finding confirm (--fix or gate-accepted): present as a single batch.
			fp(out, fmt.Sprintf("Fix %d permission(s):\n", len(permFindings)))
			for _, f := range permFindings {
				fp(out, fmt.Sprintf("  - %s\n", f.Fix.Summary))
			}
			if confirm(r, out, "Apply all?") {
				for _, f := range permFindings {
					if err := f.Fix.Fn(); err != nil {
						fp(out, fmt.Sprintf("doctor: fix failed: %s: %v\n", f.Fix.Summary, err))
					} else {
						fp(out, fmt.Sprintf("  fixed: %s\n", f.Fix.Summary))
						applied++
					}
				}
			} else {
				for range permFindings {
					skipped++
				}
				for _, f := range permFindings {
					fp(out, fmt.Sprintf("  skipped: %s\n", f.Fix.Summary))
				}
			}
		}
	}

	// Process non-permission findings individually (D-04 hard rule: orphans/wiring
	// are never batched — each gets its own confirm due to higher blast radius).
	for _, f := range otherFindings {
		if yes {
			// --fix --yes: apply silently (Interactive fixes run with defaults).
			if applyOneFix(r, out, f, true) {
				applied++
			}
			continue
		}
		// An Interactive fix (e.g. the full baseline setup) runs its own preview
		// and confirm, so we announce and delegate — no second "Apply?" prompt.
		if f.Fix.Interactive != nil {
			fp(out, fmt.Sprintf("Fix: %s\n", f.Fix.Summary))
			if applyOneFix(r, out, f, false) {
				applied++
			} else {
				skipped++
			}
			continue
		}
		// Plain fix: per-finding confirm.
		fp(out, fmt.Sprintf("Fix: %s\n", f.Fix.Summary))
		if confirm(r, out, "Apply?") {
			if applyOneFix(r, out, f, false) {
				applied++
			}
		} else {
			fp(out, fmt.Sprintf("  skipped: %s\n", f.Fix.Summary))
			skipped++
		}
	}

	// Print tally line (always, even when all were skipped).
	fp(out, fmt.Sprintf("doctor: %d fix(es) applied, %d skipped.\n", applied, skipped))

	return applied, skipped
}

// applyOneFix runs a single finding's fix, preferring the Interactive variant
// (which may prompt and prints its own progress) over the plain Fn. assumeYes is
// forwarded to Interactive fixes (true under --fix --yes → apply defaults, no
// prompts). Returns true when the fix ran without error. On error it prints the
// failure and returns false so the caller continues to the next finding.
func applyOneFix(r *bufio.Reader, out io.Writer, f doctor.Finding, assumeYes bool) bool {
	if f.Fix.Interactive != nil {
		if err := f.Fix.Interactive(r, out, assumeYes); err != nil {
			fp(out, fmt.Sprintf("doctor: fix failed: %s: %v\n", f.Fix.Summary, err))
			return false
		}
		return true // the interactive flow prints its own outcome (e.g. "Baseline written.")
	}
	if err := f.Fix.Fn(); err != nil {
		fp(out, fmt.Sprintf("doctor: fix failed: %s: %v\n", f.Fix.Summary, err))
		return false
	}
	fp(out, fmt.Sprintf("  fixed: %s\n", f.Fix.Summary))
	return true
}

// isTerminalOutput reports whether stdout is an interactive terminal. It
// respects NO_COLOR (D-08) before checking the ModeCharDevice bit (RESEARCH
// Pattern 5).
func isTerminalOutput(f *os.File) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	stat, err := f.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) != 0
}

// isTerminalInput reports whether the given file descriptor is an interactive
// terminal. Used by the fix gate (DOC-GAP-03) to suppress the Apply prompt
// when stdin is piped or redirected (CI/scripted use). We use golang.org/x/term
// for a reliable cross-platform IsTerminal check (same approach as
// isTerminalOutput's ModeCharDevice, but term.IsTerminal is more portable on
// Windows and avoids the NO_COLOR semantics which apply only to output).
func isTerminalInput(f *os.File) bool {
	return term.IsTerminal(int(f.Fd()))
}
