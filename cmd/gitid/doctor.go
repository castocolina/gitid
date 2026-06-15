package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

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
			code := runDoctor(cmd.OutOrStdout(), fix, yes)
			if code != 0 {
				// Use SilenceErrors pattern: return a non-nil error so Cobra
				// propagates the non-zero exit, but suppress duplicate printing
				// (the report already shows everything).
				return fmt.Errorf("exit code %d", code)
			}
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
// D-07/WARNING 5: the exit code reflects the PRE-fix severity state. Even
// when --fix --yes repairs every finding, the process exits with the highest
// pre-fix severity so CI is never misled into thinking the env was already
// healthy. `pre` is captured immediately after doctor.Run and returned
// unconditionally regardless of how many fixes succeed.
func runDoctor(out io.Writer, fix, yes bool) int {
	home, err := os.UserHomeDir()
	if err != nil {
		fp(out, fmt.Sprintf("doctor: resolving home dir: %v\n", err))
		return 2
	}

	sshConfigPath := filepath.Join(home, ".ssh", "config")
	gitconfigPath := filepath.Join(home, ".gitconfig")

	sshBytes, err := os.ReadFile(sshConfigPath) //nolint:gosec // sshConfigPath is a gitid-managed path (G304)
	if err != nil && !os.IsNotExist(err) {
		fp(out, fmt.Sprintf("doctor: reading %s: %v\n", sshConfigPath, err))
		return 2
	}

	gcBytes, err := os.ReadFile(gitconfigPath) //nolint:gosec // gitconfigPath is a gitid-managed path (G304)
	if err != nil && !os.IsNotExist(err) {
		fp(out, fmt.Sprintf("doctor: reading %s: %v\n", gitconfigPath, err))
		return 2
	}

	d := buildDoctorDeps(home, sshBytes, gcBytes)
	findings := doctor.Run(d)

	// D-07: capture pre-fix exit code BEFORE any fix is applied (WARNING 5).
	pre := doctor.ExitCode(findings)

	colorEnabled := isTerminalOutput(os.Stdout)
	renderReport(out, findings, colorEnabled)

	// Collect fixable findings (Fix != nil) in report order.
	var fixable []doctor.Finding
	for _, f := range findings {
		if f.Fix != nil {
			fixable = append(fixable, f)
		}
	}

	if len(fixable) > 0 {
		in := bufio.NewReader(os.Stdin)
		applyFixes(in, out, fixable, fix, yes)
	}

	// Return the PRE-fix severity state unconditionally (D-07).
	return pre
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

		// FixPerm tightens a file to the KEY-02 target mode via os.Chmod (never widens).
		FixPerm: func(path string, mode os.FileMode) error {
			return os.Chmod(path, mode) //nolint:gosec // chmod to KEY-02 target modes (G306)
		},

		// RemoveBlock removes a sentinel-delimited managed block from a file using
		// filewriter.RemoveBlock (idempotent splice) + filewriter.Write (atomic + backup).
		// Mitigates T-04-16/T-04-17: only the targeted block is removed, content
		// outside the block is preserved byte-for-byte, and a timestamped backup is
		// created before every mutation. A second call with the same name is idempotent
		// (filewriter.RemoveBlock returns input unchanged when the block is absent).
		RemoveBlock: func(path, name string) error {
			content, err := os.ReadFile(path) //nolint:gosec // path is a gitid-managed trusted path (G304)
			if err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("doctor: reading %s for block removal: %w", path, err)
			}
			removed := filewriter.RemoveBlock(content, name)
			mode := os.FileMode(0o600) // config files are always 0600 (KEY-02 / T-04-19)
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
				hostBlock := sshconfig.RenderHostBlock(alias, hostname, port, identityFile)
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

		// Check function fields wired from internal/doctor/checks.
		// Wave 2 plans replace these in place (same paths, same signatures).
		CheckPerms:     checks.CheckPermissions,
		CheckDeps:      checks.CheckDeps,
		CheckCoherence: checks.CheckCoherence,
		CheckOrphans:   checks.CheckOrphans,
		CheckSigning:   checks.CheckSigning,
		CheckAgent:     checks.CheckAgent,
		CheckBaseline:  checks.CheckBaseline,
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
			// --fix --yes: apply silently.
			if err := f.Fix.Fn(); err != nil {
				fp(out, fmt.Sprintf("doctor: fix failed: %s: %v\n", f.Fix.Summary, err))
			} else {
				fp(out, fmt.Sprintf("  fixed: %s\n", f.Fix.Summary))
				applied++
			}
		} else {
			// Per-finding confirm.
			fp(out, fmt.Sprintf("Fix: %s\n", f.Fix.Summary))
			if confirm(r, out, "Apply?") {
				if err := f.Fix.Fn(); err != nil {
					fp(out, fmt.Sprintf("doctor: fix failed: %s: %v\n", f.Fix.Summary, err))
				} else {
					fp(out, fmt.Sprintf("  fixed: %s\n", f.Fix.Summary))
					applied++
				}
			} else {
				fp(out, fmt.Sprintf("  skipped: %s\n", f.Fix.Summary))
				skipped++
			}
		}
	}

	// Print tally line (always, even when all were skipped).
	fp(out, fmt.Sprintf("doctor: %d fix(es) applied, %d skipped.\n", applied, skipped))

	return applied, skipped
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
