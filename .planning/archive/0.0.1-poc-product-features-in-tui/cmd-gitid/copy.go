package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/castocolina/gitid/internal/clipboard"
	"github.com/castocolina/gitid/internal/gitconfig"
	"github.com/castocolina/gitid/internal/identity"
	"github.com/castocolina/gitid/internal/upload"
	"github.com/castocolina/gitid/internal/uploader"
)

// newCopyCmd builds `gitid copy <name>` (CLI-01 / D-06). Copies the public key
// for the named identity to the clipboard and prints provider upload instructions.
// Optionally uploads to gh/glab when --upload-keys is set (AUTOUP-01).
func newCopyCmd() *cobra.Command {
	var uploadKeys bool
	var yes bool
	cmd := &cobra.Command{
		Use:   "copy <name>",
		Short: "Copy the public key to the clipboard and print upload instructions",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCopy(cmd.OutOrStdout(), args[0], uploadKeys, yes, buildUploaderDeps)
		},
	}
	cmd.Flags().BoolVar(&uploadKeys, "upload-keys", false, "assist key upload via gh or glab (AUTOUP-01)")
	cmd.Flags().BoolVar(&yes, "yes", false, "skip per-key upload confirmation (non-interactive)")
	return cmd
}

// newIdentityCopyCmd builds `gitid identity copy <name>` (CLI-01 / D-06).
// Same behavior as the top-level copy command.
func newIdentityCopyCmd() *cobra.Command {
	var uploadKeys bool
	var yes bool
	cmd := &cobra.Command{
		Use:   "copy <name>",
		Short: "Copy the public key to the clipboard and print upload instructions",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCopy(cmd.OutOrStdout(), args[0], uploadKeys, yes, buildUploaderDeps)
		},
	}
	cmd.Flags().BoolVar(&uploadKeys, "upload-keys", false, "assist key upload via gh or glab (AUTOUP-01)")
	cmd.Flags().BoolVar(&yes, "yes", false, "skip per-key upload confirmation (non-interactive)")
	return cmd
}

// newHostAddCmd builds `gitid host add` (CLI-01 / D-07). Adds a host alias
// (SSH account) to an existing identity, delegating to the add-account flow.
func newHostAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add",
		Short: "Add a host alias (SSH account) to an existing identity",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runAddAccount(bufio.NewReader(cmd.InOrStdin()), cmd.OutOrStdout(), false, buildDeps)
		},
	}
}

// runCopy is the handler for `gitid copy <name>` and `gitid identity copy <name>`.
// It validates the identity name (T-05-05), reconstructs the account from managed
// config files, reads the public key (.pub), copies it to the clipboard
// (CLIP-02 graceful degradation on no-tool), prints provider upload instructions,
// and optionally assists key upload via gh/glab (AUTOUP-01).
//
// Upload NEVER gates the copy (D-11): all errors from the upload path are
// non-blocking — manual instructions are always printed regardless.
func runCopy(out io.Writer, name string, uploadKeys, yes bool, uploaderDepsFor func() uploader.Deps) error {
	// T-05-05: validate name before any filesystem access.
	name = sanitizeName(name)
	if err := identity.ValidateName(name); err != nil {
		return fmt.Errorf("copy: %w", err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("copy: resolving home dir: %w", err)
	}

	// Read managed config files to reconstruct account list.
	sshBytes, _ := os.ReadFile(filepath.Join(home, ".ssh", "config")) //nolint:gosec // trusted gitid-managed path (G304)
	gcBytes, _ := os.ReadFile(filepath.Join(home, ".gitconfig"))      //nolint:gosec // trusted gitid-managed path (G304)

	accounts, err := identity.Reconstruct(sshBytes, gcBytes, gitconfig.ReadFragment)
	if err != nil {
		return fmt.Errorf("copy: reconstructing identities: %w", err)
	}

	// Find the account by name.
	var acct identity.Account
	found := false
	for _, a := range accounts {
		if a.Name == name {
			acct = a
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("copy: identity %q not found", name)
	}

	// Derive the .pub path from the account KeyPath.
	// SECURITY: only pubPath (.pub) is ever passed to uploader — never the private key.
	pubPath := acct.PubPath
	if pubPath == "" && acct.KeyPath != "" {
		pubPath = acct.KeyPath + ".pub"
	}
	if pubPath == "" {
		return fmt.Errorf("copy: identity %q has no key path; was it created with gitid?", name)
	}

	pubBytes, err := os.ReadFile(pubPath) //nolint:gosec // trusted gitid-managed .pub path (G304)
	if err != nil {
		return fmt.Errorf("copy: reading public key for %q: %w", name, err)
	}
	pubLine := strings.TrimSpace(string(pubBytes))

	// Copy to clipboard (CLIP-02: on failure, print key for manual copy and continue).
	copyErr := clipboard.Copy(pubLine)
	if copyErr != nil {
		fp(out, fmt.Sprintf("! clipboard copy failed [info]\n%v\nKey is printed below — copy manually.\n\n", copyErr))
	} else {
		fp(out, fmt.Sprintf("Copied public key for %q to clipboard.\n", name))
	}

	// Always print the key line (whether clipboard succeeded or not).
	fp(out, "Key: "+pubLine+"\n\n")

	// Print provider upload instructions (UP-02) — always shown (D-11).
	provider := acct.Provider
	if provider == "" {
		provider = "unknown"
	}
	fp(out, upload.Instructions(provider)+"\n")

	// Optional gh/glab assisted upload (AUTOUP-01).
	// Upload NEVER gates the copy (D-11): errors here are non-blocking.
	if uploadKeys {
		deps := uploaderDepsFor()
		tool, toolPath, status := uploader.Detect(deps)
		switch status {
		case uploader.AuthAuthenticated:
			fp(out, "\n--- Assisted Upload ---\n")
			fp(out, fmt.Sprintf("Detected: %s (authenticated)\n", uploader.ToolName(tool)))
			title := fmt.Sprintf("gitid: %s", name)

			// Auth key (never the private key — only .pub).
			preview := uploader.CommandPreview(tool, toolPath, pubPath, title, uploader.KeyAuthentication)
			fp(out, fmt.Sprintf("Command: %s\n", preview))
			if yes {
				result, upErr := uploader.UploadKey(tool, toolPath, pubPath, title, uploader.KeyAuthentication, deps)
				if result != "" {
					fp(out, result+"\n")
				}
				if upErr != nil {
					fp(out, fmt.Sprintf("Upload error (non-blocking): %v\n", upErr))
				}
			}

		case uploader.AuthNotLoggedIn:
			fp(out, fmt.Sprintf("\n%s: not authenticated — manual upload recommended.\n", uploader.ToolName(tool)))

		case uploader.AuthToolNotFound:
			// Tool not present: no extra section — manual instructions already shown.
		}
	}

	return nil
}

// buildUploaderDeps wires uploader.Deps from the real exec packages.
// This is the CLI equivalent of buildTUIUploaderDeps() in tui/deps.go.
// LookPath and RunCmd use arg-slice form — no shell, G204-clean.
func buildUploaderDeps() uploader.Deps {
	return uploader.Deps{
		LookPath: exec.LookPath,
		RunCmd: func(name string, args ...string) (string, int, error) {
			cmd := exec.Command(name, args...) //nolint:gosec // arg-slice; no shell; name is a trusted resolved binary path (G204)
			out, err := cmd.CombinedOutput()
			output := string(out)
			if err == nil {
				return output, 0, nil
			}
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				return output, exitErr.ExitCode(), nil
			}
			return "", 2, err
		},
	}
}
