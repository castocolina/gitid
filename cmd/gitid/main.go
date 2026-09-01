// Command gitid manages Git identities by coordinating SSH and Git configuration.
package main

import (
	"fmt"
	"io"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/castocolina/gitid/internal/tuikit"
)

// These three identifiers are vars rather than consts because the Go
// linker's -X flag can only overwrite a variable's initial value; a const
// is folded at compile time. The linker reports NO error when -X names a
// symbol that does not exist, so a mismatch between these names and the
// Makefile's -X main.<name> paths silently produces an unstamped binary.
// e2e/release_e2e_test.go is the guard that catches that mismatch.
var (
	version   = "0.0.0-dev"
	commit    = "none"
	buildDate = "unknown"
)

func composeVersion(version, commit, buildDate string) string {
	return fmt.Sprintf("%s (%s, %s)", version, commit, buildDate)
}

// versionString returns the fully composed stamp assigned to Cobra's Version
// field. Cobra v1.10.2's built-in defaultVersionTemplate (command.go:2064)
// already renders `{{DisplayName}} version {{.Version}}`, and Use is already
// "gitid", so assigning the composed string yields
// `gitid version 1.2.3 (abc1234, 2026-08-30)` with no custom template
// (09.3-RESEARCH.md Pattern 2 / Pitfall 4).
func versionString() string {
	return composeVersion(version, commit, buildDate)
}

// noArgsAction handles the no-args case for main(): if isTTY is true, calls
// run() (the real app shell) and returns 0 on success or 1 on error; if isTTY
// is false, writes the usage hint to errw and returns 1 (TUI-01 non-TTY
// contract, UI-SPEC §"Non-TTY / Piped Behavior Contract").
//
// Extracted as a named helper so tests can drive both branches without
// invoking the real TUI or os.Exit.
func noArgsAction(isTTY bool, run func() error, out io.Writer, errw io.Writer) int {
	if !isTTY {
		_, _ = fmt.Fprintln(errw, "gitid: no subcommand given. Run 'gitid --help' for usage.")
		return 1
	}
	if err := run(); err != nil {
		_, _ = fmt.Fprintf(errw, "gitid: tui: %v\n", err)
		return 1
	}
	_ = out // out is reserved for future use; currently no TTY success output
	return 0
}

// runApp launches the real approved app shell: the shared, backend-free
// tuikit render stack driven by the REAL Backend composition root
// (cmd/gitid/wiring.go). This is D-15 — bare `gitid` in a TTY opens the
// approved chrome with live backend state, replacing the retired 0.0.1 POC
// tui/ package entry point (D-14).
func runApp() error {
	_, err := tea.NewProgram(tuikit.NewApp(buildBackend())).Run()
	return err
}

func main() {
	if len(os.Args) == 1 {
		// No subcommand: branch on TTY — launch the app shell or print the hint.
		isTTY := term.IsTerminal(int(os.Stdout.Fd()))
		code := noArgsAction(isTTY, runApp, os.Stdout, os.Stderr)
		os.Exit(code)
	}
	if err := Execute(); err != nil {
		// A real command error. Cobra has already printed it. Write verbs
		// that use the frozen ssh exit-status table return *exitCodeError so
		// the process status matches the JSON envelope's exit_code field.
		os.Exit(exitStatusOf(err))
	}
}

// Execute builds the root command and runs it, returning any error so main()
// owns the single exit point (thin main()->Execute() indirection preserved).
func Execute() error {
	return newRootCmd().Execute()
}

// newRootCmd assembles the gitid Cobra command tree.
//
// D-14 archived the 0.0.1 POC command surface (identity add/list/test/rotate/
// update/delete/copy, baseline, doctor, adopt, host, add repo). Phase 5
// (SHELL-03) rebuilt the v1.0 CLI surface deliberately: the D-01 noun-verb
// taxonomy — the `identity` noun group (create/list/show/clone/new-key/
// rotate/delete, added incrementally across plans 05-01 and 05-08) plus its
// flat root-level aliases. Phase 6 replaces the reserved `ssh` noun with
// real verbs; `git`/`health`/`fix` stay reserved until Phases 7-8. What else
// remains is the root, the Phase 1 `debug` diagnostic readout, and the
// `completion` subcommand Cobra auto-registers for bash/zsh/fish/PowerShell
// (D-08/CLI-02). Plan 05-08 adds the adaptive-depth write verbs: every
// product outcome has a CLI command and no ceremony step (preview/confirm/
// backup/re-test) does (D-04), enforced by the requirement-keyed parity
// matrix check in docs/cli-parity-matrix.md.
func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "gitid",
		Short:         "Manage multiple Git identities by coordinating SSH and Git configuration",
		Version:       versionString(),
		SilenceUsage:  true,
		SilenceErrors: false,
	}

	// D-08: debug/list command surface (KEY-01/PLAT-01/MGR-02 diagnostic readout).
	root.AddCommand(newDebugCmd())

	// D-01: the identity noun group, its flat root-level aliases (built from
	// the SAME spec values — review R-15), the real ssh noun group (Phase 6),
	// the real git noun group (Phase 7), and the reserved health/fix noun
	// groups that claim their taxonomy slot before Phase 8 implements them.
	specs := identityVerbSpecs()
	root.AddCommand(newIdentityCmd(specs))
	registerFlatAliases(root, specs)
	root.AddCommand(newSSHCmd())
	root.AddCommand(newGitCmd())
	root.AddCommand(newHealthCmd())
	root.AddCommand(newFixCmd())
	root.AddCommand(newDoctorAliasCmd())

	return root
}
