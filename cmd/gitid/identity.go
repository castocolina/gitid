package main

// identity.go formalizes the D-01 noun-verb CLI taxonomy: the `gitid
// identity` noun group, the shared identityVerb/newVerbCmd constructor every
// verb (list/show/delete) is built from, its D-01 flat root-level aliases,
// and the reserved noun groups (ssh/git/health/fix) that claim their spot in
// the tree before the phase that implements them lands.

import (
	"fmt"

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
		newIdentityListVerb(),
		newIdentityShowVerb(),
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
