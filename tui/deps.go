package tui

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/castocolina/gitid/internal/clipboard"
	"github.com/castocolina/gitid/internal/deps"
	"github.com/castocolina/gitid/internal/doctor"
	"github.com/castocolina/gitid/internal/doctor/checks"
	"github.com/castocolina/gitid/internal/filewriter"
	"github.com/castocolina/gitid/internal/gitconfig"
	"github.com/castocolina/gitid/internal/identity"
	"github.com/castocolina/gitid/internal/keygen"
	"github.com/castocolina/gitid/internal/platform"
	"github.com/castocolina/gitid/internal/sshconfig"
	"github.com/castocolina/gitid/internal/tester"
)

// buildTUIDeps assembles both doctor.Deps and identity.Deps from real internal
// packages. The TUI cannot import package main, so it replicates the wiring
// from cmd/gitid/add.go (buildDeps) and cmd/gitid/doctor.go (buildDoctorDeps)
// here (RESEARCH.md Pitfall 6, Assumption A3).
func buildTUIDeps() (doctor.Deps, identity.Deps, identity.UpdateDeps, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return doctor.Deps{}, identity.Deps{}, identity.UpdateDeps{}, fmt.Errorf("tui: resolving home dir: %w", err)
	}

	sshConfigPath := filepath.Join(home, ".ssh", "config")
	gitconfigPath := filepath.Join(home, ".gitconfig")

	sshBytes, err := os.ReadFile(sshConfigPath) //nolint:gosec // path is a trusted gitid-managed path (G304)
	if err != nil && !os.IsNotExist(err) {
		return doctor.Deps{}, identity.Deps{}, identity.UpdateDeps{}, fmt.Errorf("tui: reading ssh config: %w", err)
	}

	gcBytes, err := os.ReadFile(gitconfigPath) //nolint:gosec // path is a trusted gitid-managed path (G304)
	if err != nil && !os.IsNotExist(err) {
		return doctor.Deps{}, identity.Deps{}, identity.UpdateDeps{}, fmt.Errorf("tui: reading gitconfig: %w", err)
	}

	docDeps := buildTUIDoctorDeps(home, sshBytes, gcBytes)
	idDeps := buildIdentityDeps()
	upDeps := buildTUIUpdateDeps()
	return docDeps, idDeps, upDeps, nil
}

// buildTUIUpdateDeps wires identity.UpdateDeps from real internal packages for
// the in-place edit write path (CR-02/CR-03). Mirrors cmd/gitid/update.go
// buildUpdateDeps() — the TUI cannot import package main.
func buildTUIUpdateDeps() identity.UpdateDeps {
	return identity.UpdateDeps{
		WriteSSH: func(accountName, hostBlock, globalBlock string) (string, error) {
			home, herr := os.UserHomeDir()
			if herr != nil {
				return "", herr
			}
			return sshconfig.Write(filepath.Join(home, ".ssh", "config"), accountName, hostBlock, globalBlock)
		},
		WriteGitconfig: func(id, fragmentPath, allowedSignersPath string, matches []gitconfig.Match) (string, error) {
			home, herr := os.UserHomeDir()
			if herr != nil {
				return "", herr
			}
			gitconfigPath := filepath.Join(home, ".gitconfig")
			backup, werr := gitconfig.WriteIncludeIf(gitconfigPath, id, fragmentPath, matches)
			if werr != nil {
				return backup, werr
			}
			if serr := gitconfig.SetAllowedSignersFile(gitconfigPath, allowedSignersPath); serr != nil {
				return backup, serr
			}
			return backup, nil
		},
		WriteFragment: func(fragPath, name, email, signingKeyPath string, signing bool) error {
			return gitconfig.WriteFragment(fragPath, name, email, signingKeyPath, signing)
		},
		WriteAllowedSigners: keygen.WriteAllowedSigners,
		RemoveAllowedSigners: func(path, name string) (string, error) {
			return gitconfig.RemoveAllowedSignersBlock(path, name)
		},
		Resolved: tester.Resolved,
		ReadPub: func(pubPath string) (string, error) {
			data, rerr := os.ReadFile(pubPath) //nolint:gosec // gitid-managed .pub path (G304)
			if rerr != nil {
				return "", rerr
			}
			return strings.TrimRight(string(data), "\n"), nil
		},
	}
}

// buildIdentityDeps wires identity.Deps from real internal packages.
// Mirrors cmd/gitid/add.go buildDeps() — the TUI cannot import package main.
func buildIdentityDeps() identity.Deps {
	return identity.Deps{
		Generate: func(in identity.CreateInput) (identity.StagedKey, error) {
			home, herr := os.UserHomeDir()
			if herr != nil {
				return identity.StagedKey{}, herr
			}
			sshDir := filepath.Join(home, ".ssh")
			finalPriv, finalPub := keygen.KeyPaths(sshDir, in.Algo, in.Name)
			mat, gerr := keygen.GenerateMaterial(keygen.Params{
				Algo:       in.Algo,
				Identity:   in.Name,
				Comment:    in.Name + "@gitid",
				Passphrase: in.Passphrase,
			})
			if gerr != nil {
				return identity.StagedKey{}, gerr
			}
			tempDir, terr := os.MkdirTemp("", "gitid-key-*")
			if terr != nil {
				return identity.StagedKey{}, fmt.Errorf("tui: creating staging dir: %w", terr)
			}
			tempPriv := filepath.Join(tempDir, "key")
			if _, werr := filewriter.Write(tempPriv, mat.PrivPEM, 0o600); werr != nil { //nolint:gosec // gitid-managed staging path (G306)
				_ = os.RemoveAll(tempDir)
				return identity.StagedKey{}, fmt.Errorf("tui: staging private key: %w", werr)
			}
			return identity.StagedKey{
				TempPrivatePath:  tempPriv,
				FinalPrivatePath: finalPriv,
				FinalPubPath:     finalPub,
				PubLine:          mat.PubLine,
				PrivPEM:          mat.PrivPEM,
			}, nil
		},
		PersistKey: func(staged identity.StagedKey) (identity.KeyResult, error) {
			if staged.PrivPEM == nil {
				return identity.KeyResult{
					PrivatePath: staged.FinalPrivatePath,
					PubPath:     staged.FinalPubPath,
					PubLine:     staged.PubLine,
				}, nil
			}
			if _, werr := filewriter.Write(staged.FinalPrivatePath, staged.PrivPEM, 0o600); werr != nil {
				return identity.KeyResult{}, fmt.Errorf("tui: writing final private key: %w", werr)
			}
			if _, werr := filewriter.Write(staged.FinalPubPath, []byte(staged.PubLine), 0o644); werr != nil {
				return identity.KeyResult{}, fmt.Errorf("tui: writing final public key: %w", werr)
			}
			return identity.KeyResult{
				PrivatePath: staged.FinalPrivatePath,
				PubPath:     staged.FinalPubPath,
				PubLine:     staged.PubLine,
			}, nil
		},
		Cleanup: func(staged identity.StagedKey) {
			if staged.PrivPEM == nil || staged.TempPrivatePath == staged.FinalPrivatePath {
				return
			}
			_ = os.RemoveAll(filepath.Dir(staged.TempPrivatePath))
		},
		CopyPub: clipboard.Copy,
		PreWrite: func(keyPath, hostname string, port int) tester.Result {
			return tester.PreWrite(keyPath, hostname, port)
		},
		WriteSSH: func(accountName, hostBlock, globalBlock string) (string, error) {
			home, herr := os.UserHomeDir()
			if herr != nil {
				return "", herr
			}
			return sshconfig.Write(filepath.Join(home, ".ssh", "config"), accountName, hostBlock, globalBlock)
		},
		WriteGitconfig: func(id, fragmentPath, allowedSignersPath string, matches []gitconfig.Match) (string, error) {
			home, herr := os.UserHomeDir()
			if herr != nil {
				return "", herr
			}
			gitconfigPath := filepath.Join(home, ".gitconfig")
			backup, werr := gitconfig.WriteIncludeIf(gitconfigPath, id, fragmentPath, matches)
			if werr != nil {
				return backup, werr
			}
			if serr := gitconfig.SetAllowedSignersFile(gitconfigPath, allowedSignersPath); serr != nil {
				return backup, serr
			}
			return backup, nil
		},
		WriteFragment: func(fragPath, name, email, signingKeyPath string, signing bool) error {
			return gitconfig.WriteFragment(fragPath, name, email, signingKeyPath, signing)
		},
		WriteAllowedSigners: keygen.WriteAllowedSigners,
		Resolved:            tester.Resolved,
		PubExists: func(pubPath string) bool {
			_, err := os.Stat(pubPath)
			return err == nil
		},
		DerivePub: keygen.DerivePublicKey,
		WritePub: func(pubPath, pubLine string) error {
			_, werr := filewriter.Write(pubPath, []byte(pubLine), 0o644)
			return werr
		},
	}
}

// buildTUIDoctorDeps constructs doctor.Deps for the TUI dashboard.
// Mirrors cmd/gitid/doctor.go buildDoctorDeps() — the TUI cannot import
// package main, so the wiring is replicated here (RESEARCH.md Pitfall 6, A3).
//
// All os.ReadFile and os.Stat calls use gitid-managed trusted paths (T-05-02).
func buildTUIDoctorDeps(home string, sshBytes, gcBytes []byte) doctor.Deps {
	sshConfigPath := filepath.Join(home, ".ssh", "config")
	gitconfigPath := filepath.Join(home, ".gitconfig")
	allowedSignersPath := filepath.Join(home, ".ssh", "allowed_signers")
	sshDir := filepath.Join(home, ".ssh")

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

	managedHosts, _ := sshconfig.ParseManagedHosts(sshBytes)
	sshBlockNames := make([]string, 0, len(managedHosts))
	for name := range managedHosts {
		sshBlockNames = append(sshBlockNames, name)
	}

	gcBlocks := filewriter.ListBlocks(gcBytes)
	gcBlockNames := make([]string, 0, len(gcBlocks))
	for _, b := range gcBlocks {
		gcBlockNames = append(gcBlockNames, b.Name)
	}

	allSSHHostIDFiles := sshconfig.ParseAllHostIdentityFiles(sshBytes)

	baselineFilePath := filepath.Join(home, ".gitconfig.d", "00-baseline")
	gitignorePath := filepath.Join(home, ".gitignore_global")

	return doctor.Deps{
		ReadFile: func(path string) ([]byte, error) {
			return os.ReadFile(path) //nolint:gosec // path is a trusted gitid-managed path (G304)
		},
		Stat: func(path string) (os.FileInfo, error) {
			return os.Stat(path) //nolint:gosec // path is a trusted gitid-managed path (G304)
		},

		RunSSHAdd:               tuiRunSSHAdd,
		RunSSHKeygenFingerprint: tuiRunSSHKeygenFingerprint,
		RunGitConfigGet: func(file, key string) (string, error) {
			return gitconfig.RunGitConfigGet(file, key)
		},

		GitVersionAtLeast: deps.GitVersionAtLeast,
		CurrentOS:         platform.CurrentOS,
		InstallHint:       platform.InstallHint,
		DetectTools:       deps.Detect,
		ReadBaselineState: gitconfig.ReadBaselineState,

		SSHDir:             sshDir,
		SSHConfigPath:      sshConfigPath,
		GitconfigPath:      gitconfigPath,
		AllowedSignersPath: allowedSignersPath,
		BaselineFilePath:   baselineFilePath,
		GitignorePath:      gitignorePath,

		KeyPaths:    keyPaths,
		PubKeyPaths: pubKeyPaths,

		Identities:                 accounts,
		ManagedHosts:               managedHosts,
		SSHManagedBlockNames:       sshBlockNames,
		GitconfigManagedBlockNames: gcBlockNames,
		AllSSHHostIdentityFiles:    allSSHHostIDFiles,

		FixPerm: func(path string, mode os.FileMode) error {
			return os.Chmod(path, mode) //nolint:gosec // chmod to caller-supplied tighten-only mode (G306)
		},

		RemoveBlock: func(path, name string) error {
			content, err := os.ReadFile(path) //nolint:gosec // path is a gitid-managed trusted path (G304)
			if err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("tui: reading %s for block removal: %w", path, err)
			}
			removed := filewriter.RemoveBlock(content, name)
			mode := os.FileMode(0o600)
			if path == allowedSignersPath {
				mode = 0o644
			}
			if _, werr := filewriter.Write(path, removed, mode); werr != nil {
				return fmt.Errorf("tui: removing block %q from %s: %w", name, path, werr)
			}
			return nil
		},

		AddWiring: func(path, name, line string) error {
			switch {
			case strings.HasPrefix(line, "ssh-host:"):
				rest := strings.TrimPrefix(line, "ssh-host:")
				parts := strings.SplitN(rest, ":", 4)
				if len(parts) != 4 {
					return fmt.Errorf("tui: AddWiring ssh-host: malformed line %q", line)
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
					return fmt.Errorf("tui: AddWiring ssh-host for %q: %w", name, err)
				}
			case strings.HasPrefix(line, "signers:"):
				rest := strings.TrimPrefix(line, "signers:")
				parts := strings.SplitN(rest, ":", 2)
				if len(parts) != 2 {
					return fmt.Errorf("tui: AddWiring signers: malformed line %q", line)
				}
				email, pubLine := parts[0], parts[1]
				signerLine := keygen.AllowedSignersLine(email, pubLine)
				if _, err := keygen.WriteAllowedSigners(path, name, signerLine); err != nil {
					return fmt.Errorf("tui: AddWiring signers for %q: %w", name, err)
				}
			case strings.HasPrefix(line, "baseline-include:"):
				baselineFilePath := strings.TrimPrefix(line, "baseline-include:")
				if _, err := gitconfig.WriteBaselineInclude(path, baselineFilePath); err != nil {
					return fmt.Errorf("tui: AddWiring baseline-include: %w", err)
				}
			default:
				return fmt.Errorf("tui: AddWiring: unknown wiring type in line %q", line)
			}
			return nil
		},

		CheckPerms:     checks.CheckPermissions,
		CheckDeps:      checks.CheckDeps,
		CheckCoherence: checks.CheckCoherence,
		CheckOrphans:   checks.CheckOrphans,
		CheckSigning:   checks.CheckSigning,
		CheckAgent:     checks.CheckAgent,
		CheckBaseline:  checks.CheckBaseline,
	}
}

// tuiRunSSHAdd runs `ssh-add -l` via arg-slice exec (no shell, G204-clean).
// Mirrors runSSHAdd from cmd/gitid/doctor.go.
func tuiRunSSHAdd() (string, int) {
	cmd := exec.Command("ssh-add", "-l") //nolint:gosec // arg-slice form, no shell; fixed args (G204)
	out, err := cmd.CombinedOutput()
	output := string(out)
	if err == nil {
		return output, 0
	}
	var exitErr *exec.ExitError
	if ok := errors.As(err, &exitErr); ok {
		return output, exitErr.ExitCode()
	}
	return "", 2
}

// tuiRunSSHKeygenFingerprint runs `ssh-keygen -lf <path>` via arg-slice exec
// (no shell, G204-clean). Mirrors runSSHKeygenFingerprint from cmd/gitid/doctor.go.
func tuiRunSSHKeygenFingerprint(path string) (string, error) {
	cmd := exec.Command("ssh-keygen", "-lf", path) //nolint:gosec // arg-slice form; path is trusted gitid-managed .pub (G204/G304)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	line := strings.SplitN(string(out), "\n", 2)[0]
	return line, nil
}
