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

const version = "0.0.0-dev"

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
		// A real command error. Cobra has already printed it; exit non-zero.
		os.Exit(1)
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
// flat root-level aliases — and the reserved `ssh`/`git`/`health`/`fix` noun
// groups that claim their taxonomy slot before the phases that implement them
// (6, 7, 8, 8) land — guaranteeing a later phase's verb can never collide
// with a top-level flat alias. What else remains is the root, the Phase 1
// `debug` diagnostic readout, and the `completion` subcommand Cobra
// auto-registers for bash/zsh/fish/PowerShell (D-08/CLI-02). Plan 05-08 adds
// the adaptive-depth write verbs: every product outcome has a CLI command and
// no ceremony step (preview/confirm/backup/re-test) does (D-04), enforced by
// the requirement-keyed parity matrix check in docs/cli-parity-matrix.md.
func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "gitid",
		Short:         "Manage multiple Git identities by coordinating SSH and Git configuration",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: false,
	}

	// D-08: debug/list command surface (KEY-01/PLAT-01/MGR-02 diagnostic readout).
	root.AddCommand(newDebugCmd())

	// D-01: the identity noun group, its flat root-level aliases (built from
	// the SAME spec values — review R-15), and the reserved noun groups that
	// claim ssh/git/health/fix before Phases 6-8 implement them.
	specs := identityVerbSpecs()
	root.AddCommand(newIdentityCmd(specs))
	registerFlatAliases(root, specs)
	root.AddCommand(newReservedNounCmd("ssh", "Manage global SSH options (arrives in Phase 6)", "Phase 6 (Global SSH Options)"))
	root.AddCommand(newReservedNounCmd("git", "Manage global Git options (arrives in Phase 7)", "Phase 7 (Global Git Options)"))
	root.AddCommand(newReservedNounCmd("health", "Show identity/config health (arrives in Phase 8)", "Phase 8 (Health + Fixer)"))
	root.AddCommand(newReservedNounCmd("fix", "Apply suggested health fixes (arrives in Phase 8)", "Phase 8 (Health + Fixer)"))

	return root
}
