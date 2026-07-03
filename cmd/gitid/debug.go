package main

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/castocolina/gitid/internal/identity"
	"github.com/castocolina/gitid/internal/keygen"
	"github.com/castocolina/gitid/internal/platform"
)

// newDebugCmd builds `gitid debug` (D-08), the Phase-1 diagnostic surface
// that proves KEY-01 (algorithm catalog + resolved local availability),
// PLAT-01 (local capability probe), and MGR-02 (per-identity state) via a
// real, test-exercised command. It is thin glue: RunE only gathers input
// from the internal packages and prints — no classification or aggregation
// logic lives here (RESEARCH.md's Architectural Responsibility Map;
// 01-PATTERNS.md "Cobra thin-glue command surface").
func newDebugCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "debug",
		Short: "Print diagnostic information about the local environment (D-08)",
	}
	cmd.AddCommand(newDebugCapsCmd())
	return cmd
}

// newDebugCapsCmd builds `gitid debug caps`. The handler prints three
// sections: the local capability probe, the top-5 algorithm catalog with
// availability resolved from the probe's raw `ssh -Q key` tokens, and every
// gitid-managed identity's IdentityHealth (consumed from
// identity.BuildInventory, never re-derived here).
func newDebugCapsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "caps",
		Short: "Print the algorithm catalog, local capability probe, and per-identity state (KEY-01/PLAT-01/MGR-02)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runDebugCaps(cmd.Context(), cmd.OutOrStdout())
		},
	}
	return cmd
}

// runDebugCaps is the debug-caps orchestration handler. It gathers local
// capabilities via the EXPORTED platform.BuildProbeDeps real wiring,
// resolves catalog availability from the RAW `ssh -Q key` protocol tokens
// (caps.KeyTypes — NOT the already-mapped caps.Algorithms, which would
// silently resolve every catalog entry Available=false), builds the
// identity inventory via identity.BuildInventory, and prints all three
// sections in order. It NEVER references gitid's private-key-material type,
// prints passphrase fields, dumps raw private-key contents, or dumps the
// process environment (T-01-15).
func runDebugCaps(ctx context.Context, out io.Writer) error {
	return runDebugCapsWithDeps(ctx, out, platform.BuildProbeDeps(), identity.BuildInventoryDeps())
}

// runDebugCapsWithDeps is runDebugCaps with its two dependency sets injected,
// so tests can exercise the exact same orchestration logic against fakes
// (hermetic, no real exec/filesystem) while runDebugCaps itself wires the
// real platform.BuildProbeDeps/identity.BuildInventoryDeps closures.
func runDebugCapsWithDeps(ctx context.Context, out io.Writer, probeDeps platform.Deps, invDeps identity.InventoryDeps) error {
	caps, err := platform.Probe(ctx, probeDeps)
	if err != nil {
		return fmt.Errorf("debug caps: probing local capabilities: %w", err)
	}
	printCapabilities(out, caps)

	cat := keygen.ResolveAvailability(keygen.Catalog(), caps.KeyTypes, caps.FIDO.Usable())
	printCatalog(out, cat)

	inv, err := identity.BuildInventory(invDeps)
	if err != nil {
		return fmt.Errorf("debug caps: building identity inventory: %w", err)
	}
	printInventory(out, inv)

	return nil
}

// printCapabilities renders the structured local capability probe result:
// the parsed SSH version fields, the SSL flavor/version, and the
// agent/FIDO/keychain STATUS strings (never a raw bool — PLAT-02).
func printCapabilities(out io.Writer, caps platform.Capabilities) {
	fp(out, "=== Capabilities ===\n")
	fp(out, fmt.Sprintf("  openssh version: %s\n", caps.SSHVersion.OpenSSHVersion))
	fp(out, fmt.Sprintf("  ssl flavor:      %s\n", caps.SSHVersion.SSLFlavor))
	fp(out, fmt.Sprintf("  ssl version:     %s\n", caps.SSHVersion.SSLVersion))
	fp(out, fmt.Sprintf("  raw version:     %s\n", caps.SSHVersion.Raw))
	fp(out, fmt.Sprintf("  agent:           %s\n", caps.Agent))
	fp(out, fmt.Sprintf("  fido:            %s\n", caps.FIDO))
	fp(out, fmt.Sprintf("  keychain:        %s\n", caps.Keychain))
	fp(out, "\n")
}

// printCatalog renders the top-5 algorithm catalog with its Implemented/
// Available/Generatable/Default flags, security note, and the per-OS note
// for the current OS (platform.CurrentOS). cat is expected to already have
// Available resolved (via keygen.ResolveAvailability) by the caller.
func printCatalog(out io.Writer, cat []keygen.AlgoInfo) {
	fp(out, "=== Algorithm Catalog ===\n")
	osName := platform.CurrentOS()
	for _, a := range cat {
		marker := ""
		if a.Default {
			marker = " (default)"
		}
		fp(out, fmt.Sprintf("  %s%s\n", a.Name, marker))
		fp(out, fmt.Sprintf("    implemented: %t  available: %t  generatable: %t\n",
			a.Implemented, a.Available, keygen.Generatable(a)))
		fp(out, fmt.Sprintf("    security:    %s\n", a.Security))
		fp(out, fmt.Sprintf("    note (%s):   %s\n", osName, osNote(a, osName)))
	}
	fp(out, "\n")
}

// osNote picks the algorithm's per-OS note for osName ("darwin" or
// "linux"); any other value falls back to the Linux note (the safe default
// for other Unix-likes, mirroring platform.InstallHint's fallback shape).
func osNote(a keygen.AlgoInfo, osName string) string {
	if osName == "darwin" {
		return a.DarwinNote
	}
	return a.LinuxNote
}

// printInventory renders every gitid-managed identity's IdentityHealth
// (both axes + Problems) plus the global unused-key list, consumed directly
// from identity.BuildInventory's result — no state is re-derived here
// (MGR-02, Codex HIGH #4). Key paths (not key material) are safe to print.
func printInventory(out io.Writer, inv identity.Inventory) {
	fp(out, "=== Identities ===\n")
	if len(inv.Identities) == 0 {
		fp(out, "  no gitid-managed identities found\n")
	}
	for _, h := range inv.Identities {
		fp(out, fmt.Sprintf("  %s\n", h.Name))
		fp(out, fmt.Sprintf("    identity state: %s\n", h.IdentityState))
		fp(out, fmt.Sprintf("    key state:      %s\n", h.KeyState))
		if len(h.Problems) > 0 {
			fp(out, fmt.Sprintf("    problems:       %s\n", renderProblems(h.Problems)))
		}
	}
	if len(inv.UnusedKeys) > 0 {
		fp(out, fmt.Sprintf("  unused keys: %s\n", joinStrings(inv.UnusedKeys)))
	}
}

// renderProblems joins a Problem slice into a comma-separated string for
// single-line rendering.
func renderProblems(problems []identity.Problem) string {
	parts := make([]string, len(problems))
	for i, p := range problems {
		parts[i] = string(p)
	}
	return joinStrings(parts)
}

// joinStrings is a tiny comma-join helper local to this file, avoiding an
// extra strings import for a single call site.
func joinStrings(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += ", "
		}
		out += p
	}
	return out
}
