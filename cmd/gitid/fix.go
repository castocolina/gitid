package main

// fix.go is `gitid fix [--yes] [--dry-run]` (08-02-PLAN.md Task 3): it
// builds the SAME doctor.Deps health.go uses, runs doctor.Run(deps), and
// walks the fixable findings one at a time — printing each real diff (the
// same construction Backend.FixPlanFor uses), applying via the cmd-layer
// apply-gate precedence doctor.go's FixDescriptor doc comment documents
// (Fix.Interactive preferred over Fix.Fn when non-nil), and re-running the
// full scan after EVERY apply (mirroring persistFixFinding's D-13
// discipline) before considering the next finding — never acting on stale
// data. A maxPasses hard backstop (ported from the archived POC's
// convergeFixes algorithm) caps this walk so a pathological disagreement
// between two checks can never loop indefinitely.

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/castocolina/gitid/internal/doctor"
	"github.com/castocolina/gitid/internal/tuikit"
)

// fixMaxPasses bounds gitid fix --yes's batch-apply loop (T-08-06): the
// single-fix TUI ceremony path has no loop to bound (one user-confirmed
// apply, one re-scan) — this backstop guards only the CLI's unattended walk.
const fixMaxPasses = 10

// fixScanOverride is a test-only injection point: when non-nil, runFix
// calls it instead of runDoctorAndConvert(buildDoctorDeps(home)) for every
// scan/re-scan — the only way to drive runFix's own apply-gate precedence
// and maxPasses backstop against a synthetic Fix.Interactive/Fix.Fn
// descriptor without a real check producing one (no real check currently
// sets Interactive without Fn, and no real check can be made to
// artificially never converge). Nil in production.
var fixScanOverride func(home string) ([]doctor.Finding, []tuikit.DemoFinding)

func newFixCmd() *cobra.Command {
	var yes, dryRun bool
	return newVerbCmd(identityVerb{
		use:   "fix",
		short: "Apply suggested health fixes",
		args:  cobra.NoArgs,
		bindFlags: func(fs *pflag.FlagSet) {
			fs.BoolVar(&yes, "yes", false, "apply every fixable finding without a per-finding prompt")
			fs.BoolVar(&dryRun, "dry-run", false, "preview every fixable finding's diff and exit 0 without writing")
		},
		run: func(cmd *cobra.Command, _ []string) error {
			home, err := resolveHomeForCLI()
			if err != nil {
				return err
			}
			return runFix(cmd.OutOrStdout(), cmd.InOrStdin(), home, yes, dryRun)
		},
	})
}

// scanForFix returns the raw findings and their conversion for home — the
// real doctor.Run(deps) scan, or fixScanOverride's synthetic result when a
// test has set one.
func scanForFix(home string) ([]doctor.Finding, []tuikit.DemoFinding) {
	if fixScanOverride != nil {
		return fixScanOverride(home)
	}
	return runDoctorAndConvert(buildDoctorDeps(home))
}

// runFix is the testable body of `gitid fix`.
func runFix(out io.Writer, in io.Reader, home string, yes, dryRun bool) error {
	b := newBackendForHome(home)

	if dryRun {
		raw, converted := scanForFix(home)
		printed := false
		for i, f := range raw {
			if f.Fix == nil {
				continue
			}
			printed = true
			plan := b.FixPlanFor(converted[i])
			fmt.Fprintf(out, "would fix: %s\n%s\n\n", f.Title, plan.Diff) //nolint:errcheck // best-effort CLI output
		}
		if !printed {
			fmt.Fprintln(out, "no fixable findings") //nolint:errcheck // best-effort CLI output
		}
		return nil
	}

	reader := bufio.NewReader(in)
	for pass := 0; pass < fixMaxPasses; pass++ {
		raw, converted := scanForFix(home)
		fi, ci, ok := firstFixable(raw, converted)
		if !ok {
			return nil // converged: nothing left to fix
		}

		plan := b.FixPlanFor(ci)
		fmt.Fprintf(out, "%s\n%s\n", fi.Title, plan.Diff) //nolint:errcheck // best-effort CLI output

		var applyErr error
		switch {
		case fi.Fix.Interactive != nil:
			// Never call Fn on a fix that only sets Interactive, and never
			// skip an Interactive fix just because it is not Fn (the plan's
			// own apply-gate precedence, doctor.go's FixDescriptor doc
			// comment): thread the CLI's own stdin/stdout and --yes.
			applyErr = fi.Fix.Interactive(reader, out, yes)
		case fi.Fix.Fn != nil:
			if !yes && !confirmFix(reader, out, fi.Fix.Summary) {
				fmt.Fprintln(out, "skipped") //nolint:errcheck // best-effort CLI output
				return nil                   // a declined fix is not a batch failure — stop cleanly
			}
			applyErr = fi.Fix.Fn()
		default:
			// A finding whose SuggestedFix is set but Fix is nil (info-only
			// per the D-01 contract) is never fixable — firstFixable already
			// filters on Fix != nil, so this branch cannot be reached; guard
			// defensively rather than silently looping.
			return fmt.Errorf("gitid: fix: %q carries no Fix.Fn or Fix.Interactive", fi.Title)
		}
		if applyErr != nil {
			return fmt.Errorf("gitid: fix: %s: %w", fi.Title, applyErr)
		}
		fmt.Fprintln(out, "fixed") //nolint:errcheck // best-effort CLI output
	}
	return fmt.Errorf("gitid: fix: did not converge after %d passes — the same finding kept reappearing after its own fix reported success; investigate manually via `gitid health`", fixMaxPasses)
}

// firstFixable returns the first finding in raw carrying a non-nil Fix,
// plus its converted counterpart at the same index.
func firstFixable(raw []doctor.Finding, converted []tuikit.DemoFinding) (doctor.Finding, tuikit.DemoFinding, bool) {
	for i, f := range raw {
		if f.Fix != nil {
			return f, converted[i], true
		}
	}
	return doctor.Finding{}, tuikit.DemoFinding{}, false
}

// confirmFix reads one line from r and reports whether it is a "y"/"yes"
// (case-insensitive) confirmation — the CLI's per-finding gate for a
// Fn-based fix when --yes was not passed.
func confirmFix(r *bufio.Reader, out io.Writer, summary string) bool {
	fmt.Fprintf(out, "Apply %q? [y/N] ", summary) //nolint:errcheck // best-effort CLI output
	line, _ := r.ReadString('\n')
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes"
}
