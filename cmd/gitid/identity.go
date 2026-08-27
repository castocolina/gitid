package main

// identity.go formalizes the D-01 noun-verb CLI taxonomy: the `gitid
// identity` noun group, the shared identityVerb/newVerbCmd constructor every
// verb (list/show/delete) is built from, its D-01 flat root-level aliases,
// and the reserved noun groups (ssh/git/health/fix) that claim their spot in
// the tree before the phase that implements them lands.

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// identityVerb declares ONE identity CLI verb — its Use/Short/Args/flags/
// handler — as a single specification consumed TWICE by newVerbCmd (review
// R-15): once to build the `identity <verb>` noun child, once to build the
// flat-alias `gitid <verb>` root command. A Cobra command has exactly one
// parent, which is why two distinct *cobra.Command objects are required;
// building BOTH from the SAME spec — never one command delegating to the
// other's RunE — is what keeps their flags, Args validator, and handler
// byte-identical. Delegating to a sibling's RunE would not reproduce that
// sibling's flag parsing, Args validator, PreRunE, context, or usage output,
// so the flat form and the noun form would silently diverge the moment
// either grows a flag.
type identityVerb struct {
	use       string
	aliases   []string
	short     string
	args      cobra.PositionalArgs
	bindFlags func(*pflag.FlagSet)
	run       func(cmd *cobra.Command, args []string) error
}

// newVerbCmd builds a FRESH *cobra.Command from v, binding v.bindFlags onto
// THAT command's own flag set. Called twice per spec (identity.go's
// newIdentityCmd and registerFlatAliases) — two distinct command objects,
// one specification.
func newVerbCmd(v identityVerb) *cobra.Command {
	cmd := &cobra.Command{
		Use:          v.use,
		Aliases:      v.aliases,
		Short:        v.short,
		Args:         v.args,
		RunE:         v.run,
		SilenceUsage: true,
	}
	if v.bindFlags != nil {
		v.bindFlags(cmd.Flags())
	}
	return cmd
}

// identityVerbSpecs is the single source of every identity verb spec — built
// ONCE by newRootCmd and handed to BOTH newIdentityCmd (the noun children)
// and registerFlatAliases (the flat root-level forms), so the two command
// trees are built from the identical spec values, never two independently
// (re-)constructed ones.
func identityVerbSpecs() []identityVerb {
	return []identityVerb{
		newIdentityCreateVerb(),
		newIdentityListVerb(),
		newIdentityShowVerb(),
		newIdentityCloneVerb(),
		newIdentityRotateVerb(),
		newIdentityNewKeyVerb(),
		newIdentityDeleteVerb(),
	}
}

// newIdentityCmd builds the `gitid identity` noun group (D-01) and registers
// every spec in specs as a child.
func newIdentityCmd(specs []identityVerb) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "identity",
		Aliases: []string{"id"},
		Short:   "Manage gitid identities (list, show, delete)",
	}
	for _, v := range specs {
		cmd.AddCommand(newVerbCmd(v))
	}
	return cmd
}

// registerFlatAliases adds every spec directly onto root as a D-01 flat
// alias (`gitid list`, `gitid show`, `gitid delete`) — via newVerbCmd's
// shared-spec pattern (review R-15), never a wrapper calling the noun-form's
// RunE.
func registerFlatAliases(root *cobra.Command, specs []identityVerb) {
	for _, v := range specs {
		root.AddCommand(newVerbCmd(v))
	}
}

// newReservedNounCmd builds a placeholder noun group that claims a D-01
// taxonomy slot before the phase that implements it lands — guaranteeing a
// later phase's real verb can never collide with a top-level flat alias.
// Its RunE returns a non-nil error naming the implementing phase; it
// performs no other action.
func newReservedNounCmd(use, short, phase string) *cobra.Command {
	return &cobra.Command{
		Use:          use,
		Short:        short,
		SilenceUsage: true,
		RunE: func(*cobra.Command, []string) error {
			return fmt.Errorf("gitid: %q arrives in %s — not yet implemented", use, phase)
		},
	}
}

// ---------------------------------------------------------------------------
// The D-02 adaptive-depth resolver (review R-21)
// ---------------------------------------------------------------------------

// resolveOutcome is one of the three D-02 branches every write verb's resolver
// can return.
type resolveOutcome int

const (
	// resolveHeadless: every required flag was supplied — run to completion
	// with no interactive surface, exactly as a script would.
	resolveHeadless resolveOutcome = iota
	// resolvePrefilledTUI: a required flag is missing AND both stdin and
	// stdout are terminals — open the app with the wizard pre-filled from the
	// flags that WERE supplied.
	resolvePrefilledTUI
	// resolveMissingFlags: a required flag is missing AND not both descriptors
	// are terminals — exit non-zero naming exactly the missing flags, never
	// an interactive surface and never a silent default.
	resolveMissingFlags
)

// depthResolver is the ONE D-02 adaptive-depth decision point shared by every
// write verb. It carries the verb's required flag names, the flag set the
// invocation actually supplied, and TWO independent terminal facts
// (review R-21): stdinTTY and stdoutTTY. A single isTTY conflates two distinct
// situations — a terminal stdout with a piped stdin means the confirmation
// prompt cannot be answered, and a terminal stdin with a piped stdout means
// the output is being captured — and in BOTH cases an interactive surface is
// wrong. The rule: the pre-filled TUI launches ONLY for (stdinTTY &&
// stdoutTTY); every other combination routes the incomplete-flags case to the
// non-interactive error branch. The two booleans are injected rather than
// read from the terminal inside the resolver, mirroring noArgsAction's
// existing seam (main.go), so all three branches are unit-testable.
type depthResolver struct {
	required  []string
	supplied  map[string]bool
	stdinTTY  bool
	stdoutTTY bool
}

// resolve returns the outcome plus the missing flag names (empty except for
// the missing-flags outcome, populated for the pre-filled branch so the
// caller knows what it could not fill from flags).
func (r depthResolver) resolve() (resolveOutcome, []string) {
	var missing []string
	for _, name := range r.required {
		if !r.supplied[name] {
			missing = append(missing, name)
		}
	}
	if len(missing) == 0 {
		return resolveHeadless, nil
	}
	if r.stdinTTY && r.stdoutTTY {
		return resolvePrefilledTUI, missing
	}
	return resolveMissingFlags, missing
}

// missingFlagErr names EVERY missing flag and no flag that was supplied — the
// D-02 non-interactive contract for an incomplete invocation.
func missingFlagErr(verb string, missing []string) error {
	return fmt.Errorf("gitid: %s requires the flag(s) %s to run headless", verb, strings.Join(missing, ", "))
}

// ---------------------------------------------------------------------------
// The exhaustive flag-to-confirmationMode mapping (review R2-03)
// ---------------------------------------------------------------------------

// confirmationPolicyFrom maps a CLI write verb's authorization flags onto plan
// 05-07's three-valued confirmationMode with NO fallback branch. The two
// inputs are analyzed exhaustively:
//
//	--yes given                     -> confirmationBypassedWithYes, Prompt nil
//	--yes absent, both TTYs          -> confirmationRequired + the prompt closure
//	--yes absent, not both TTYs      -> a refusal error BEFORE any lifecycle
//	                                     function is constructed or called
//
// The last branch treats a missing authorization exactly like a missing
// required flag: a non-interactive destructive run without --yes must never be
// mistaken for pre-confirmed (05-07's confirmationRequired is the enum's ZERO
// value and fails closed regardless, but the refusal here happens even
// earlier — before a policy value exists). The TUI-only authorization value
// (the mode that asserts a human already accepted a confirm screen) is NEVER
// produced from a CLI path — asserted by a source-level test that greps the
// CLI handler files for the identifier and demands zero occurrences — so a
// future handler cannot borrow it to silence a prompt.
//
// A dry run is exempt from this gate because it cannot write: --dry-run is
// accepted without a terminal and without --yes, and 05-07's lifecycle stops
// after the plan stage, before the confirmation gate. Callers route dry runs
// BEFORE invoking this function.
// confirmationPolicyFrom builds the confirmation half of a write ceremony's
// lifecyclePolicy. prompt receives the SAME preview string confirmGate
// passes to lifecyclePolicy.Prompt (the resolved target path plus what will
// change) — WR-08: prompt used to take no argument, so every caller's
// wrapper silently discarded confirmGate's preview and the interactive user
// was asked to confirm a write having been shown neither the resolved target
// nor the diff nor any shadow warnings.
func confirmationPolicyFrom(_ *cobra.Command, refuseVerb string, stdinTTY, stdoutTTY, yes bool, prompt func(preview string) (bool, error)) (lifecyclePolicy, error) {
	if yes {
		return lifecyclePolicy{Confirm: confirmationBypassedWithYes}, nil
	}
	if stdinTTY && stdoutTTY {
		return lifecyclePolicy{Confirm: confirmationRequired, Prompt: prompt}, nil
	}
	return lifecyclePolicy{}, fmt.Errorf("gitid: refusing to %s without --yes in non-interactive mode (use --yes to skip only the confirmation prompt; the timestamped backup is still taken)", refuseVerb)
}
