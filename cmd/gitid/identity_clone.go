package main

// identity_clone.go implements `gitid identity clone <source>` — the D-02
// adaptive-depth clone verb. A --name flag runs headless through the SAME
// create ceremony `identity create` uses (D-15: clone has NO second write
// pipeline — it re-derives the source into a CreateInput and feeds the create
// lifecycle); without a name on a terminal it opens the wizard pre-filled via
// the same ClonePrefill seam the TUI's clone prompt uses; without a name off
// a terminal it errors naming the name flag.

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/castocolina/gitid/internal/identity"
	"github.com/castocolina/gitid/internal/sshconfig"
	"github.com/castocolina/gitid/internal/tuikit"
)

// identityCloneFlags carries the clone verb's flags as a plain struct.
type identityCloneFlags struct {
	Name     string
	NewKey   bool
	Yes      bool
	DryRun   bool
	NoUpload bool
}

// cloneRequiredFlags is the one flag an `identity clone` invocation must
// supply to run headless.
var cloneRequiredFlags = []string{"name"}

// newIdentityCloneVerb builds the `clone <source>` verb spec.
func newIdentityCloneVerb() identityVerb {
	var flags identityCloneFlags
	return identityVerb{
		use:     "clone <source>",
		aliases: nil,
		short:   "Clone an existing identity into a new name",
		args:    cobra.ExactArgs(1),
		bindFlags: func(fs *pflag.FlagSet) {
			fs.StringVar(&flags.Name, "name", "", "clone name (must differ from the source; default suggests <source>-clone)")
			fs.BoolVar(&flags.NewKey, "new-key", false, "generate a fresh key for the clone instead of reusing the source's key")
			fs.BoolVar(&flags.Yes, "yes", false, "skip the confirmation prompt; the timestamped backup is still taken unconditionally")
			fs.BoolVar(&flags.DryRun, "dry-run", false, "run both connectivity stages, print the artifact previews, and exit 0 without writing")
			fs.BoolVar(&flags.NoUpload, "no-upload", false, noUploadFlagHelp)
		},
		run: func(cmd *cobra.Command, args []string) error {
			return runIdentityClone(cmd, args[0], flags, termIsStdinTTY(), termIsStdoutTTY())
		},
	}
}

// termIsStdinTTY / termIsStdoutTTY are the two independent terminal facts the
// depth resolver keys on (review R-21).
func termIsStdinTTY() bool  { return isTTY(os.Stdin.Fd()) }
func termIsStdoutTTY() bool { return isTTY(os.Stdout.Fd()) }

// runIdentityClone is the clone verb's D-02 adaptive-depth entry point.
func runIdentityClone(cmd *cobra.Command, source string, flags identityCloneFlags, stdinTTY, stdoutTTY bool) error {
	res, missing := depthResolver{
		required:  cloneRequiredFlags,
		supplied:  map[string]bool{"name": flags.Name != ""},
		stdinTTY:  stdinTTY,
		stdoutTTY: stdoutTTY,
	}.resolve()

	switch res {
	case resolvePrefilledTUI:
		home, err := resolveHomeForCLI()
		if err != nil {
			return err
		}
		b := newBackendForHome(home)
		name := strings.TrimSpace(flags.Name)
		if name == "" {
			name = b.SuggestCloneName(source)
		}
		pre, err := b.ClonePrefill(source, name, !flags.NewKey)
		if err != nil {
			return err
		}
		return runPrefilledTUI(b, &pre)
	case resolveMissingFlags:
		return missingFlagErr("identity clone", missing)
	case resolveHeadless:
	}

	home, err := resolveHomeForCLI()
	if err != nil {
		return err
	}
	b := newBackendForHome(home)
	in, id, err := cloneCeremonyInputs(b, source, strings.TrimSpace(flags.Name), !flags.NewKey)
	if err != nil {
		return err
	}
	return runCreateCeremony(cmd, b, in, id, flags.Yes, flags.DryRun, flags.NoUpload, stdinTTY, stdoutTTY)
}

// cloneCeremonyInputs re-derives the clone's CreateInput + DemoIdentity for a
// headless clone. It mirrors the realBackend.ClonePrefill derivation exactly
// (same identity.DeriveCloneInput call, same wildcard-pattern shadowing check
// via real OpenSSH semantics) so the headless path refuses the same things
// the wizard would — this is read-only derivation, never a second write
// pipeline (D-15).
func cloneCeremonyInputs(b *realBackend, source, cloneName string, reuseSourceKey bool) (identity.CreateInput, tuikit.DemoIdentity, error) {
	if b.initErr != nil {
		return identity.CreateInput{}, tuikit.DemoIdentity{}, b.initErr
	}
	src, found := b.findAccount(source)
	if !found {
		return identity.CreateInput{}, tuikit.DemoIdentity{}, fmt.Errorf("clone: source identity %q not found", source)
	}
	src.KeyPath = expandTildeForHome(src.KeyPath, b.home)
	targets := identity.CloneTargets{
		GitconfigPath:      b.gitconfigPath,
		SSHConfigPath:      b.sshConfigPath,
		AllowedSignersPath: b.allowedSigners,
		FragmentDir:        b.fragmentDir,
	}
	in, _, derr := identity.DeriveCloneInput(src, cloneName, reuseSourceKey, targets)
	if derr != nil {
		return identity.CreateInput{}, tuikit.DemoIdentity{}, derr
	}
	content, rerr := os.ReadFile(b.sshConfigPath) //nolint:gosec // b.sshConfigPath is a trusted gitid-managed path resolved in-process
	if rerr == nil {
		if matches := sshconfig.MatchingHostStanzas(content, in.Alias); len(matches) > 0 {
			return identity.CreateInput{}, tuikit.DemoIdentity{}, &identity.FieldError{
				Field: "alias",
				Message: fmt.Sprintf(
					"%q is already matched by the existing Host pattern %q — pick a different name or edit that pattern first",
					in.Alias, matches[0]),
			}
		}
	}
	// TestStage1/2 reconstruct the CreateInput via createInputFromSpec, which
	// defaults an empty algorithm to ed25519 and resolves a ~/ reuse path.
	// The store gate fingerprints THAT reconstructed value, so a clone input
	// that leaves Algo empty (or a display-form reuse path) would pass both
	// stages and still be refused as stale.
	if in.Algo == "" {
		in.Algo = "ed25519"
	}
	if in.ReuseKeyPath != "" {
		in.ReuseKeyPath = b.resolveKeyPath(in.ReuseKeyPath)
	}
	id := tuikit.DemoIdentity{
		Name:          in.Name,
		SSHHost:       in.Alias,
		Hostname:      in.Hostname,
		Port:          in.Port,
		Provider:      in.Provider,
		GitConfigured: true,
		GitName:       in.GitName,
		GitEmail:      in.GitEmail,
		MatchStrategy: matchStrategyFromMatches(in.Matches),
		// WR-10: derive GitDir from in.Matches the SAME way realBackend.
		// ClonePrefill does (gitDirFromMatches) — a source whose gitdir is
		// "~/work/acme/" must clone to "~/work/acme/" headless too, not
		// silently re-derive "~/git/<clone>/" and diverge from what the
		// wizard's own prefill (and this verb's own resolvePrefilledTUI
		// branch a few lines above) would have produced. Only fall back to
		// the "~/git/<name>/" default for a pure-hasconfig clone, which
		// carries no gitdir match at all.
		GitDir: orDefault(gitDirFromMatches(in.Matches), "~/git/"+in.Name+"/"),
		// WR-11: PublicKeyPath is deliberately NOT set here — it is
		// unconditionally reassigned by the if/else below (reuse vs.
		// generated), so setting it in this literal was dead: when
		// ReuseKeyPath is empty, the literal produced the bare string
		// ".pub" (the exact value WR-14 in identities.go was written to
		// eliminate) before being immediately overwritten. Harmless today,
		// a trap on the next edit that reorders these blocks.
		// WR-06: default to the CLONE SOURCE's own current setting, never an
		// unconditional true. This is a machine-global rewrite of every
		// HTTPS clone URL for the provider — forcing it on for every
		// headless clone contradicts D-15's "copy the author fields,
		// re-derive the rest": a clone must not silently switch on a
		// global side effect the source never had.
		ForceSSH:        src.ForceSSH,
		Algorithm:       in.Algo,
		ReuseKeyPath:    in.ReuseKeyPath,
		GitFragmentPath: in.FragmentPath,
	}
	if id.MatchStrategy == "" {
		id.MatchStrategy = "gitdir"
	}
	if in.ReuseKeyPath != "" {
		// A reusable source key: point the signing key at it. Generated
		// clones use the canonical id_ed25519_<name> path.
		id.PublicKeyPath = in.ReuseKeyPath + ".pub"
	} else {
		id.PublicKeyPath = "~/.ssh/id_ed25519_" + in.Name + ".pub"
	}
	return in, id, nil
}

// orDefault returns v unless it is empty, in which case it returns fallback.
func orDefault(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}
