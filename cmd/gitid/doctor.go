package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/castocolina/gitid/internal/deps"
	"github.com/castocolina/gitid/internal/doctor"
	"github.com/castocolina/gitid/internal/doctor/checks"
	"github.com/castocolina/gitid/internal/gitconfig"
	"github.com/castocolina/gitid/internal/identity"
	"github.com/castocolina/gitid/internal/platform"
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
func runDoctor(out io.Writer, _, _ bool) int {
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
	code := doctor.ExitCode(findings)
	colorEnabled := isTerminalOutput(os.Stdout)
	renderReport(out, findings, colorEnabled)
	return code
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

	// Reconstruct identity list to extract key paths for the perms check.
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

		// Fix fields (D-01: cmd layer owns chmod, doctor core does not).
		FixPerm: func(path string, mode os.FileMode) error {
			return os.Chmod(path, mode) //nolint:gosec // chmod to KEY-02 target modes (G306)
		},

		// Check function fields wired from internal/doctor/checks.
		// Wave 2 plans replace these in place (same paths, same signatures).
		CheckPerms: checks.CheckPermissions,
		// Remaining six families are stubs returning nil until Wave 2:
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
