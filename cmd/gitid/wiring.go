package main

// wiring.go is the REAL Backend composition root — the single place where the
// backend-free render stack (internal/tuikit) is joined to the user's actual
// machine.
//
// Three contracts this file exists to uphold:
//
//  1. EVERY identity.Deps field is non-nil. A nil injected seam is the
//     project's recurring wiring blindspot: the nil branch silently falls back
//     to a different behavior while every unit test still passes. wiring_test.go
//     reflects over the whole struct and fails naming the offending FIELD.
//
//  2. This is the ONLY backend -> view conversion site. internal/tuikit imports
//     zero first-party backend packages, so keygen.ReusableKey and tester.Result
//     become tuikit.ReusableKeyView / tuikit.TestResultView HERE and nowhere
//     else. If a screen appears to need a backend value the answer is a new view
//     DTO plus a converter here — never a new entry in the import allowlist.
//
//  3. Every write routes through internal/filewriter (backup + atomic
//     temp->rename->chmod) via internal/sshconfig. This file never calls
//     os.WriteFile, and it never mutates the live ~/.ssh/config for a test:
//     both connectivity stages run against a throwaway staged config
//     (SSHUI-04).

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	tea "charm.land/bubbletea/v2"

	"github.com/castocolina/gitid/internal/clipboard"
	"github.com/castocolina/gitid/internal/filewriter"
	"github.com/castocolina/gitid/internal/gitconfig"
	"github.com/castocolina/gitid/internal/identity"
	"github.com/castocolina/gitid/internal/keygen"
	"github.com/castocolina/gitid/internal/platform"
	"github.com/castocolina/gitid/internal/sshconfig"
	"github.com/castocolina/gitid/internal/tester"
	"github.com/castocolina/gitid/internal/tuikit"
)

// keyFileMode / pubFileMode are the explicit permission bits for generated key
// material — never inherited from the process umask.
const (
	keyFileMode os.FileMode = 0o600
	pubFileMode os.FileMode = 0o644
	sshDirMode  os.FileMode = 0o700
)

// gitidConfigFileName is the gitid-owned Include'd storage file created on a
// fresh machine (D-06). It lives inside ~/.ssh/config.d, which
// sshconfig.ReservedPaths registers as gitid-owned (L4).
const gitidConfigFileName = "gitid.config"

// backupSuffixPreview is how a pending timestamped backup is NAMED in the
// confirm-write preview. filewriter stamps the real name with the write's own
// UnixNano at write time, so the ceremony shows the shape and the receipt shows
// the actual path — it never invents a timestamp that will not be used.
const backupSuffixPreview = ".bak.<timestamp>"

// realBackend is the live tuikit.Backend: every effect answered from the
// user's real configuration through the internal packages.
//
// It carries the in-flight create's staged key so the two connectivity stages
// and the confirmed write all speak about the SAME key material. Backend
// methods are called both from the Bubble Tea update loop and from the
// goroutines its tea.Cmds run in, so that state is mutex-guarded.
type realBackend struct {
	home           string
	sshDir         string
	sshConfigPath  string
	includeDir     string
	gitconfigPath  string
	allowedSigners string
	fragmentDir    string
	deps           identity.Deps

	// initErr records a construction-time failure (no resolvable home
	// directory). Every effect degrades to a safe no-op rather than writing
	// to an unknown location.
	initErr error

	mu sync.Mutex
	// stageDir is the throwaway directory both test stages run against.
	stageDir string
	// staged/stagedFor hold the key material generated for the in-flight
	// create, keyed by identity name. stagedReuseKeyPath additionally keys
	// the cache on the D-10 reuse choice so switching between "generate" and
	// "reuse <path>" mid-flow re-stages instead of serving a stale cache hit.
	staged             identity.StagedKey
	stagedFor          string
	stagedReuseKeyPath string
	stagedIn           identity.CreateInput
	// stage1Outcome/stage2Outcome record the accepted outcome for each stage
	// when it belongs to the CURRENT create specification. Together they are
	// the D-01 STORE GATE at the backend seam: PASS and ReachableNotUploaded
	// both unlock the write; a hard Failure or any stale/absent proof blocks.
	stage1Outcome tuikit.TestOutcome
	stage1Known   bool
	stage2Outcome tuikit.TestOutcome
	stage2Known   bool
	outcomeSpec   string // fingerprint of the spec the outcomes belong to
	persistErr    error

	// failCommitAt is a test-only injection point: when non-nil, CommitCreate
	// fails before the named mutation step. The steps are "private-key",
	// "public-key", "include-line", and "host-block".
	failCommitAt func(step string) error
}

// compile-time proof the real composition root satisfies the seam.
var _ tuikit.Backend = (*realBackend)(nil)

// buildBackend constructs the real tuikit.Backend. It is the only production
// caller of buildIdentityDeps, and the only place cmd/gitid resolves the
// user's configuration paths.
func buildBackend() tuikit.Backend {
	b := &realBackend{}
	home, err := os.UserHomeDir()
	if err != nil {
		b.initErr = fmt.Errorf("gitid: resolving home directory: %w", err)
		return b
	}
	return newBackendForHome(home)
}

// newBackendForHome builds a backend rooted at home. Split out from
// buildBackend so tests can drive the whole composition root over a hermetic
// fake home without ever touching the developer's real ~/.ssh.
func newBackendForHome(home string) *realBackend {
	b := &realBackend{
		home:           home,
		sshDir:         filepath.Join(home, ".ssh"),
		sshConfigPath:  filepath.Join(home, ".ssh", "config"),
		includeDir:     filepath.Join(home, ".ssh", "config.d"),
		gitconfigPath:  filepath.Join(home, ".gitconfig"),
		allowedSigners: filepath.Join(home, ".ssh", "allowed_signers"),
		fragmentDir:    filepath.Join(home, ".gitconfig.d"),
	}
	b.deps = buildIdentityDeps(b)
	return b
}

// ---------------------------------------------------------------------------
// identity.Deps — every field non-nil (the injected-seam wiring rule)
// ---------------------------------------------------------------------------

// buildIdentityDeps wires identity.Deps from the real internal packages.
// EVERY field is filled: a nil seam here is a silent behavior change, not a
// missing feature (see wiring_test.go's reflection guard).
func buildIdentityDeps(b *realBackend) identity.Deps {
	return identity.Deps{
		// Generate writes the private test key to the backend's mode-0700
		// staging directory; final paths are carried in StagedKey but not
		// touched until the confirmed transaction (CR-02).
		Generate: func(in identity.CreateInput) (identity.StagedKey, error) {
			// CR-08: Do NOT create or chmod the real ~/.ssh directory here.
			// The final SSH directory is created/chmoded only inside the
			// confirmed commitCreateTransaction — never before explicit consent.
			// Key generation runs entirely in the throwaway staging directory.
			stageDir, derr := b.stagingDir()
			if derr != nil {
				return identity.StagedKey{}, derr
			}
			finalPriv, finalPub := keygen.KeyPaths(b.sshDir, in.Algo, in.Name)
			mat, gerr := keygen.GenerateMaterial(keygen.Params{
				Algo:       in.Algo,
				Identity:   in.Name,
				Comment:    in.Name + "@gitid",
				Passphrase: in.Passphrase,
			})
			if gerr != nil {
				return identity.StagedKey{}, fmt.Errorf("gitid: generating key material: %w", gerr)
			}
			tempPriv := filepath.Join(stageDir, "id_"+in.Algo+"_"+in.Name)
			if _, werr := filewriter.Write(tempPriv, mat.PrivPEM, keyFileMode); werr != nil {
				return identity.StagedKey{}, fmt.Errorf("gitid: writing staged private key: %w", werr)
			}
			return identity.StagedKey{
				TempPrivatePath:  tempPriv,
				FinalPrivatePath: finalPriv,
				FinalPubPath:     finalPub,
				PubLine:          mat.PubLine,
				PrivPEM:          mat.PrivPEM,
			}, nil
		},
		PersistKey: func(s identity.StagedKey) (identity.KeyResult, error) {
			result := identity.KeyResult{
				PrivatePath: s.FinalPrivatePath,
				PubPath:     s.FinalPubPath,
				PubLine:     s.PubLine,
			}
			if s.PrivPEM == nil {
				return result, nil // existing-key path: nothing to persist
			}
			if _, werr := filewriter.Write(s.FinalPrivatePath, s.PrivPEM, keyFileMode); werr != nil {
				return identity.KeyResult{}, fmt.Errorf("gitid: writing private key: %w", werr)
			}
			if _, werr := filewriter.Write(s.FinalPubPath, []byte(s.PubLine), pubFileMode); werr != nil {
				return identity.KeyResult{}, fmt.Errorf("gitid: writing public key: %w", werr)
			}
			return result, nil
		},
		// Cleanup removes the throwaway staging directory after the confirmed
		// transaction, but only for generated keys where TempPrivatePath is
		// distinct from the final path. Reuse leaves the existing key untouched.
		Cleanup: func(s identity.StagedKey) {
			if s.TempPrivatePath != "" && s.TempPrivatePath != s.FinalPrivatePath {
				_ = os.RemoveAll(filepath.Dir(s.TempPrivatePath))
			}
		},
		CopyPub: clipboard.Copy,
		PreWrite: func(keyPath, hostname string, port int) tester.Result {
			knownHosts, _ := b.knownHostsPath()
			return tester.PreWrite(keyPath, hostname, port, knownHosts)
		},
		// WriteSSH resolves the STORAGE LAYOUT first (D-05/D-06) and writes the
		// Host block plus the macOS globals block into the resolved target.
		WriteSSH: func(accountName, hostBlock, globalBlock string) (string, error) {
			return b.writeSSHBlock(accountName, hostBlock, globalBlock)
		},
		WriteGitconfig: func(id, fragmentPath, allowedSignersPath string, matches []gitconfig.Match) (string, error) {
			backup, werr := gitconfig.WriteIncludeIf(b.gitconfigPath, id, fragmentPath, matches)
			if werr != nil {
				return backup, werr
			}
			if serr := gitconfig.SetAllowedSignersFile(b.gitconfigPath, allowedSignersPath); serr != nil {
				return backup, serr
			}
			return backup, nil
		},
		WriteFragment:       gitconfig.WriteFragment,
		WriteAllowedSigners: keygen.WriteAllowedSigners,
		Resolved:            tester.Resolved,
		// StageTestConfig renders the identity's Host block into a THROWAWAY
		// config file. Both test stages run against it, so the live
		// ~/.ssh/config is never mutated for a test (SSHUI-04).
		//
		// CR-08: Uses RenderCheckedHostBlock so an unsafe IdentityFile token is
		// caught here — before the staged config is handed to ssh — rather than
		// silently producing a malformed directive.
		StageTestConfig: func(in identity.CreateInput, staged identity.StagedKey) (string, error) {
			if staged.TempPrivatePath == "" {
				return "", fmt.Errorf("gitid: no staged key path for the test config")
			}
			dir, derr := b.stagingDir()
			if derr != nil {
				return "", derr
			}
			hostBlock, cerr := sshconfig.RenderCheckedHostBlock(in.Alias, in.Hostname, in.Port, staged.TempPrivatePath, in.Provider)
			if cerr != nil {
				return "", fmt.Errorf("gitid: validating staged test config: %w", cerr)
			}
			configPath := filepath.Join(dir, "config")
			if _, werr := filewriter.Write(configPath, []byte(hostBlock), keyFileMode); werr != nil {
				return "", fmt.Errorf("gitid: staging the test ssh config: %w", werr)
			}
			return configPath, nil
		},
		ResolvedVia: func(configPath, keyPath, alias string) (tester.Result, tester.ResolvedConfig) {
			knownHosts, _ := b.knownHostsPath()
			return tester.ResolvedVia(configPath, keyPath, alias, knownHosts)
		},
		PubExists: func(pubPath string) bool {
			_, err := os.Stat(pubPath)
			return err == nil
		},
		DerivePub: keygen.DerivePublicKey,
		WritePub: func(pubPath, pubLine string) error {
			_, werr := filewriter.Write(pubPath, []byte(pubLine), pubFileMode)
			return werr
		},
		// ReadPub is the D-11 seam: when an existing `.pub` is present its line
		// is used VERBATIM instead of re-deriving it from the private key —
		// which would require parsing a passphrase-protected key and therefore
		// break encrypted-key reuse (KEY-06). identity.ensurePub nil-guards this
		// field and silently falls back to DerivePub, so leaving it nil in the
		// REAL constructor re-breaks the feature while every unit test passes.
		// That is exactly the recurring injected-seam wiring blindspot; the
		// raw-keystroke PTY proof that closes it lands in plan 03-06.
		ReadPub: func(pubPath string) (string, error) {
			data, rerr := os.ReadFile(pubPath) //nolint:gosec // pubPath is a trusted gitid-managed .pub path (G304)
			if rerr != nil {
				return "", fmt.Errorf("gitid: reading public key %s: %w", pubPath, rerr)
			}
			return strings.TrimRight(string(data), "\n"), nil
		},
		WriteProvisionalSSH: func(name, hostBlock string) (string, error) {
			return sshconfig.WriteProvisional(b.storageTargetPath(), name, hostBlock)
		},
		PromoteSSH: func(name, hostBlock string) (string, error) {
			return sshconfig.Promote(b.storageTargetPath(), name, hostBlock)
		},
		DropProvisionalSSH: func(name string) (string, error) {
			return sshconfig.DropProvisional(b.storageTargetPath(), name)
		},
	}
}

// ---------------------------------------------------------------------------
// Backend: data
// ---------------------------------------------------------------------------

// InitialState reads the user's ACTUAL configuration: every reconstructed
// identity plus the detected STORE-01 storage layout. Health findings stay
// empty in Phase 3 — the Doctor tab still renders demo content behind the
// D-16 banner (see DemoBanner).
func (b *realBackend) InitialState() tuikit.DemoState {
	state := tuikit.DemoState{SSHStorage: tuikit.StorageSentinel}
	if b.initErr != nil {
		return state
	}
	if b.storage().includeLayout {
		state.SSHStorage = tuikit.StorageInclude
	}
	for _, acct := range b.accounts() {
		state.Identities = append(state.Identities, b.toDemoIdentity(acct))
	}
	return state
}

// DemoBanner raises the D-16 "still demo data" banner on every tab EXCEPT
// Identities: the create flow is the only surface Phase 3 wires to live data.
// Each later phase removes its own banner as it wires its view.
func (b *realBackend) DemoBanner(tab tuikit.TabID) bool {
	return tab != tuikit.TabIdentities
}

// Persist commits one action against the real machine and re-reads the
// configuration, so the list always reflects what is actually on disk rather
// than an optimistic in-memory guess.
//
// Only AddIdentity performs a real write in Phase 3 (the SSH leg — the Git leg
// is Phase 4, D-18: a skipped Git step stores an SSH-only, incomplete
// identity). Every other action still reduces in memory so the not-yet-wired
// screens behind the D-16 banner keep working.
func (b *realBackend) Persist(state tuikit.DemoState, action tuikit.Action) tuikit.DemoState {
	switch a := action.(type) {
	case tuikit.Reset:
		return b.InitialState()
	case tuikit.AddIdentity:
		return b.persistCreate(state, a)
	default:
		return tuikit.Reduce(state, action)
	}
}

// persistCreate performs the confirmed create write: the key pair (already on
// disk from the test stage, or generated now) plus the managed Host block and
// the macOS globals block, into the auto-detected storage layout, every write
// backed up first.
//
// Persist has no error channel by design (the seam is a pure state transition),
// so a failure is recorded on the backend and the PREVIOUS state is returned
// unchanged — the user's configuration is never reported as changed when it was
// not. Surfacing the recorded error in the ceremony is plan 03-05's render work;
// PersistError() below is the accessor it consumes.
func (b *realBackend) persistCreate(state tuikit.DemoState, a tuikit.AddIdentity) tuikit.DemoState {
	if b.initErr != nil {
		b.setPersistErr(b.initErr)
		return state
	}
	in := b.createInput(a.Identity)
	if !b.storeUnlockedFor(in) {
		b.setPersistErr(fmt.Errorf(
			"gitid: refusing to write %q: the connectivity test failed, so nothing was proven about the provider", a.Identity.Name))
		return state
	}
	staged, err := b.stagedKeyFor(in, a.Identity.ReuseKeyPath)
	if err != nil {
		b.setPersistErr(err)
		return state
	}
	if _, err := b.commitCreateTransaction(in, staged, a.Identity); err != nil {
		b.setPersistErr(err)
		return state
	}
	b.clearStaged()
	b.clearOutcomes()
	b.setPersistErr(nil)
	return b.InitialState()
}

// PersistError reports the error the last committed write failed with, or nil.
// The render surface that shows it is wired in plan 03-05; exposing it here
// keeps the failure observable instead of swallowed.
func (b *realBackend) PersistError() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.persistErr
}

// ---------------------------------------------------------------------------
// Backend: create-flow effects
// ---------------------------------------------------------------------------

// AlgorithmCatalog is the KEY-01 catalog cross-referenced against THIS
// machine's toolchain: keygen.Catalog resolved through the internal/platform
// probe, so an algorithm the local ssh-keygen cannot produce is shown with its
// real unavailability note (KEY-03).
func (b *realBackend) AlgorithmCatalog() []tuikit.AlgorithmCatalogEntry {
	supported, err := platform.ProbeKeyTypes()
	if err != nil {
		supported = nil // probe failed: nothing is provably available
	}
	fidoUsable := false
	for _, tok := range supported {
		if strings.HasPrefix(tok, "sk-") {
			fidoUsable = true
			break
		}
	}
	resolved := keygen.ResolveAvailability(keygen.Catalog(), supported, fidoUsable)
	out := make([]tuikit.AlgorithmCatalogEntry, 0, len(resolved))
	for _, a := range resolved {
		out = append(out, toAlgorithmCatalogEntry(a))
	}
	return out
}

// ProviderDefaults resolves a provider host to its endpoint and port. Known
// providers get the recipe-canonical alt-SSH pairing (D-20:
// github.com -> ssh.github.com:443); an unknown/custom host keeps itself on
// port 22 with no invented alt-SSH endpoint (D-21).
//
// The provider table lives in internal/identity (DefaultHostname/DefaultPort) —
// cmd/gitid deliberately does NOT re-derive one.
func (b *realBackend) ProviderDefaults(provider string) (hostname, port string) {
	if strings.TrimSpace(provider) == "" {
		// The wizard's initial, untouched form value — kept byte-identical to
		// the approved design's starting render.
		return "github.com", "22"
	}
	resolved := identity.DefaultHostname(provider)
	if resolved == strings.ToLower(provider) {
		return resolved, "22" // unknown/custom provider (D-21)
	}
	return resolved, strconv.Itoa(identity.DefaultPort())
}

// DefaultMatchStrategy is the includeIf match strategy a new identity starts
// on: gitdir, per recipes/ (GITUI-03).
func (b *realBackend) DefaultMatchStrategy() string { return "gitdir" }

// HostBlockPreview is the live Host block for spec, rendered by the SAME
// sshconfig.RenderCheckedHostBlock the confirmed write uses — so "written
// exactly like this on confirm" is structurally true, not a re-typed
// lookalike (SSHUI-03: Port 443 + IdentitiesOnly yes per recipes/).
//
// WR-01: Uses spec.Provider (the validated provider the wizard carried
// forward) rather than providerFromAlias(spec.Alias), which would truncate
// multi-label providers like "company.co.uk" to "co.uk".
//
// CR-08: Uses the checked render boundary so any unsafe IdentityFile value
// is caught before it reaches the preview; on validation failure the preview
// returns an empty string (the UI renders nothing, same as a blank spec).
func (b *realBackend) HostBlockPreview(spec tuikit.CreateSpec) string {
	block, err := sshconfig.RenderCheckedHostBlock(
		spec.Alias,
		spec.Hostname,
		atoiOr(spec.Port, identity.DefaultPort()),
		spec.KeyPath,
		spec.Provider,
	)
	if err != nil {
		return ""
	}
	return block
}

// GitFragmentPreview is the ~/.gitconfig.d/<identity> fragment the Git step
// will write. user.signingkey is a PATH to the public half, never key material
// (SIGN-02). The Git leg itself lands in Phase 4 (D-18/D-19).
func (b *realBackend) GitFragmentPreview(spec tuikit.GitSpec) string {
	return "[user]\n    name = " + spec.Name + "\n    email = " + spec.Email +
		"\n    signingkey = " + spec.KeyPath + ".pub\n\n[gpg]\n    format = ssh\n\n[commit]\n    gpgsign = true"
}

// IncludeIfPreview is the ~/.gitconfig includeIf block for spec's match
// strategy, rendered by the same gitconfig.RenderIncludeIf Phase 4 will write.
func (b *realBackend) IncludeIfPreview(spec tuikit.GitSpec) string {
	fragment := filepath.Join("~/.gitconfig.d", spec.Identity)
	return gitconfig.RenderIncludeIf(spec.Identity, fragment, matchesFor(spec))
}

// ValidateHostBlock validates the four SSH form values before they are
// interpolated into an OpenSSH Host block.
func (b *realBackend) ValidateHostBlock(alias, hostname, port, identityFile string) *tuikit.ValidationError {
	if err := sshconfig.ValidateHostBlock(alias, hostname, port, identityFile); err != nil {
		if validationErr, ok := err.(*sshconfig.ValidationError); ok {
			return &tuikit.ValidationError{Field: validationErr.Field, Message: validationErr.Message}
		}
		return &tuikit.ValidationError{Message: err.Error()}
	}
	return nil
}

// AliasCollision reports whether alias is already claimed — the D-09 gate
// the wizard holds step 1 on. It checks the user's REAL configuration
// Include-aware (a fresh D-06 machine keeps every identity block in
// config.d/gitid.config, so reading ~/.ssh/config alone would see none), plus
// every Host pattern in the file, managed AND hand-written: gitid must never
// write an ambiguous first-match-wins alias.
func (b *realBackend) AliasCollision(alias string) (bool, error) {
	if strings.TrimSpace(alias) == "" {
		return false, nil
	}
	if b.initErr != nil {
		return false, b.initErr
	}
	return sshconfig.AliasCollision(b.sshConfigPath, alias)
}

// ScanReusableKeys lists the D-10 reuse candidates found in ~/.ssh. Encrypted
// and non-catalog keys are surfaced with their flags, never dropped (D-13);
// keys already referenced by an identity carry the D-12 "in use by" label.
func (b *realBackend) ScanReusableKeys() []tuikit.ReusableKeyView {
	if b.initErr != nil {
		return nil
	}
	keys, err := keygen.ScanReusableKeys(b.sshDir)
	if err != nil {
		return nil
	}
	return toReusableKeyViews(keys, b.keyOwners())
}

// ManualReusePath resolves the picker's manual-path row (D-10) against the
// user's real filesystem: a symlinked candidate is rejected before parsing
// (T-03-13, keygen.ScanManualKey's os.Lstat check), and a usable candidate
// carries the SAME D-12 "in use by" label ScanReusableKeys' rows do.
func (b *realBackend) ManualReusePath(path string) (tuikit.ReusableKeyView, error) {
	if b.initErr != nil {
		return tuikit.ReusableKeyView{}, b.initErr
	}
	resolved := b.resolveKeyPath(strings.TrimSpace(path))
	key, err := keygen.ScanManualKey(resolved)
	if err != nil {
		return tuikit.ReusableKeyView{}, err
	}
	views := toReusableKeyViews([]keygen.ReusableKey{key}, b.keyOwners())
	return views[0], nil
}

// TestConfigPath is the throwaway config both stages run against — the
// SSHUI-04 guarantee that the live ~/.ssh/config is untouched until confirm.
func (b *realBackend) TestConfigPath() string {
	dir, err := b.stagingDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "config")
}

// Stage1Command is the exact stage-1 command (key DIRECT against the provider,
// TEST-01). It is built from the SAME argument slice tester.PreWrite executes,
// so the shown string can never drift from the run one.
func (b *realBackend) Stage1Command(spec tuikit.CreateSpec) string {
	knownHosts, _ := b.knownHostsPath()
	return tester.PreWriteCommand(b.resolveKeyPath(spec.KeyPath), spec.Hostname, atoiOr(spec.Port, identity.DefaultPort()), knownHosts)
}

// Stage2Command is the exact stage-2 command: the alias resolved THROUGH the
// staged temp config (TEST-02).
func (b *realBackend) Stage2Command(spec tuikit.CreateSpec) string {
	knownHosts, _ := b.knownHostsPath()
	return tester.ResolvedViaCommand(b.TestConfigPath(), b.resolveKeyPath(spec.KeyPath), spec.Alias, knownHosts)
}

// TestStage1 runs the real stage-1 connectivity test: the key is generated (if
// it does not exist yet) and tested DIRECTLY against the provider endpoint.
// Classification stays tester's job — an outcome is never re-derived from an
// exit code here.
func (b *realBackend) TestStage1(spec tuikit.CreateSpec) tea.Cmd {
	return func() tea.Msg {
		in := b.createInputFromSpec(spec)
		staged, err := b.stagedKeyFor(in, spec.ReuseKeyPath)
		if err != nil {
			return b.stageFailure(1, b.Stage1Command(spec), err, in)
		}
		res := b.deps.PreWrite(staged.TempPrivatePath, in.Hostname, in.Port)
		view := toTestResultView(res, res.Command)
		b.recordOutcomeFor(1, view.Outcome, in)
		return tuikit.WizardStageMsg{Stage: 1, Result: view}
	}
}

// TestStage2 runs the real stage-2 test: the alias resolved BY NAME through the
// staged throwaway config — no -i by design, since proving the config supplies
// the key is the whole point (TEST-02).
//
// CR-05: validates the ssh -G resolution fields (User, Hostname, Port,
// IdentitiesOnly, first IdentityFile) BEFORE recording the accepted outcome
// so a wrong, empty, or failed resolution cannot unlock persistence.
//
// CR-06: carries both the connectivity command+output AND the resolution
// command+output in the TestResultView for the TUI to render complete proof.
func (b *realBackend) TestStage2(spec tuikit.CreateSpec) tea.Cmd {
	return func() tea.Msg {
		in := b.createInputFromSpec(spec)
		staged, err := b.stagedKeyFor(in, spec.ReuseKeyPath)
		if err != nil {
			return b.stageFailure(2, b.Stage2Command(spec), err, in)
		}
		configPath, err := b.deps.StageTestConfig(in, staged)
		if err != nil {
			return b.stageFailure(2, b.Stage2Command(spec), err, in)
		}
		res, resolved := b.deps.ResolvedVia(configPath, staged.TempPrivatePath, in.Alias)

		// Build the base view from the connectivity result.
		view := toTestResultView(res, res.Command)

		// Attach the ssh -G resolution command and the exact stdout from the
		// same execution that produced the parsed fields for validation.
		view.ResolutionCommand = tester.ResolvedViaGCommand(configPath, in.Alias)
		view.ResolutionOutput = res.ResolutionOutput

		// CR-05: Validate all five required resolution fields BEFORE recording
		// the accepted outcome. A connectivity PASS with a wrong/empty resolution
		// (ssh -G spawn failure, wrong config, or truncated output) is NOT a
		// valid stage-2 proof — downgrade to Failure without recording acceptance.
		keyPath := staged.TempPrivatePath
		if keyPath == "" {
			keyPath = staged.FinalPrivatePath
		}
		expected := tester.ExpectedResolution{
			User:            "git",
			Hostname:        in.Hostname,
			Port:            fmt.Sprintf("%d", in.Port),
			IdentitiesOnly:  "yes",
			ExpectedKeyPath: keyPath,
		}
		if verr := tester.ValidateResolvedConfig(resolved, expected); verr != nil {
			// Resolution validation failed — this is a hard stage-2 failure.
			// Record Failure so the store gate blocks persistence.
			b.recordOutcomeFor(2, tuikit.TestOutcomeFailure, in)
			view.Outcome = tuikit.TestOutcomeFailure
			view.Detail = "stage-2 proof failed: " + verr.Error()
			return tuikit.WizardStageMsg{Stage: 2, Result: view}
		}

		// Resolution validated — record the connectivity outcome.
		b.recordOutcomeFor(2, view.Outcome, in)
		return tuikit.WizardStageMsg{Stage: 2, Result: view}
	}
}

// ResolvedStorageTarget is the file gitid's managed blocks actually land in —
// the auto-detected layout (D-05), Include'd by default on a fresh machine
// (D-06). It is a real resolved path, never a hardcoded ~/.ssh/config.
func (b *realBackend) ResolvedStorageTarget(tuikit.DemoState) string {
	if b.initErr != nil {
		return ""
	}
	return b.displayPath(b.storageTargetPath())
}

// CreateWritePlan is what a committed create will touch, with the timestamped
// backups taken FIRST (TEST-03). On a fresh machine the Include'd layout is
// created, so the plan reports BOTH file changes: the gitid.config write AND
// the Include line added to ~/.ssh/config (D-06).
func (b *realBackend) CreateWritePlan(spec tuikit.CreateSpec, git *tuikit.GitSpec) tuikit.WritePlanView {
	if b.initErr != nil {
		return tuikit.WritePlanView{}
	}
	st := b.storage()

	// Targets, in write order. On a fresh machine the Include'd layout is
	// created, so BOTH file changes are previewed: the gitid.config write AND
	// the Include line added to ~/.ssh/config (D-06).
	targets := []string{st.targetPath}
	if st.needsIncludeLine {
		targets = append(targets, b.sshConfigPath)
	}
	if git != nil {
		targets = append(targets,
			filepath.Join(b.fragmentDir, spec.Identity),
			b.gitconfigPath,
			b.allowedSigners,
		)
	}

	plan := tuikit.WritePlanView{}
	for _, p := range targets {
		plan.Targets = append(plan.Targets, b.displayPath(p))
		// Only files that ALREADY exist get a backup — filewriter backs up
		// nothing when it creates a file for the first time, and promising a
		// backup that will not be taken would be a lie in the ceremony.
		if fileExists(p) {
			plan.Backups = append(plan.Backups, b.displayPath(p)+backupSuffixPreview)
		}
	}
	return plan
}

// CopyPublicKey copies the identity's public key line to the system clipboard
// (D-03) — offered on the reachable-but-not-uploaded path so the user can
// register it with the provider and retry. Only the `.pub` line ever leaves
// this function: private key material is never read, copied or displayed.
func (b *realBackend) CopyPublicKey(pubKeyPath string) (string, error) {
	path := b.resolveKeyPath(pubKeyPath)
	line, err := b.deps.ReadPub(path)
	if err != nil {
		return "", err
	}
	if err := clipboard.Copy(line); err != nil {
		return "", fmt.Errorf("gitid: copying the public key to the clipboard: %w", err)
	}
	return "Public key copied to clipboard (" + b.displayPath(path) + ").", nil
}

// GitStepDisabledReason implements D-19: the real binary ALWAYS disables
// the wizard's Git-identity step [ Continue ] button, with its own honest
// reason — never the dummy's validity-based one.
func (b *realBackend) GitStepDisabledReason() (string, bool) {
	return "", false
}

// CommitGit is the standalone Git-flow async seam. Its detailed transaction is
// introduced with the reusable flow; this first contract implementation keeps
// the existing no-op behavior safe until an explicit confirmed Git request is
// wired by the reducer.
func (b *realBackend) CommitGit(_ tuikit.GitSpec) tea.Cmd {
	return func() tea.Msg {
		if b.initErr != nil {
			return tuikit.GitCommitMsg{Err: b.initErr.Error()}
		}
		return tuikit.GitCommitMsg{}
	}
}

// ---------------------------------------------------------------------------
// Backend -> view converters (the ONLY conversion site)
// ---------------------------------------------------------------------------

// toReusableKeyViews projects scanned private keys into the picker's view DTO.
// Path/Algorithm/Fingerprint/HasPub/Encrypted carry through verbatim; InUseBy
// is filled from the identity inventory for the D-12 "in use by: personal"
// label and is empty when no identity references the key.
func toReusableKeyViews(keys []keygen.ReusableKey, owners map[string]string) []tuikit.ReusableKeyView {
	out := make([]tuikit.ReusableKeyView, 0, len(keys))
	for _, k := range keys {
		out = append(out, tuikit.ReusableKeyView{
			Path:        k.Path,
			Algorithm:   k.Algorithm,
			Fingerprint: k.Fingerprint,
			HasPub:      k.HasPub,
			Encrypted:   k.Encrypted,
			InUseBy:     owners[k.Path],
		})
	}
	return out
}

// toTestResultView projects one connectivity test into the wizard's view DTO.
// The outcome mapping is total and one-way — the classification itself stays
// tester's job (it reads output substrings, never an exit code), and command is
// the string that was actually RUN, upholding TEST-01's shown == run contract.
func toTestResultView(res tester.Result, command string) tuikit.TestResultView {
	view := tuikit.TestResultView{Command: command, Detail: res.Output}
	switch res.Outcome {
	case tester.PASS:
		view.Outcome = tuikit.TestOutcomePass
	case tester.ReachableNotUploaded:
		view.Outcome = tuikit.TestOutcomeReachableNotUploaded
	case tester.Failure:
		view.Outcome = tuikit.TestOutcomeFailure
	}
	return view
}

// toAlgorithmCatalogEntry projects one probed catalog entry into the wizard's
// view DTO, keeping the machine-specific availability notes honest.
func toAlgorithmCatalogEntry(a keygen.AlgoInfo) tuikit.AlgorithmCatalogEntry {
	return tuikit.AlgorithmCatalogEntry{
		ID:          a.Name,
		Security:    a.Security,
		MacOS:       a.DarwinNote,
		Linux:       a.LinuxNote,
		Recommended: a.Default,
		Implemented: a.Implemented,
		Available:   a.Available,
	}
}

// toDemoIdentity projects one reconstructed account into the identity list's
// view row, classified through the locked MGR-02 state vocabulary.
func (b *realBackend) toDemoIdentity(acct identity.Account) tuikit.DemoIdentity {
	keyExists := acct.KeyPath != "" && fileExists(acct.KeyPath)
	state := identity.ClassifyState(acct, keyExists, keyExists && acct.Alias != "", acct.FragmentPath != "")
	return tuikit.DemoIdentity{
		Name:            acct.Name,
		State:           string(state),
		SSHHost:         acct.Alias,
		KeyPath:         b.displayPath(acct.KeyPath),
		GitFragmentPath: b.displayPath(acct.FragmentPath),
		GitName:         acct.GitName,
		GitEmail:        acct.GitEmail,
		Hostname:        acct.Hostname,
		Port:            acct.Port,
	}
}

// ---------------------------------------------------------------------------
// Storage layout resolution (D-05 / D-06)
// ---------------------------------------------------------------------------

// storageLayout is the resolved answer to "where do gitid's managed blocks go
// on THIS machine".
type storageLayout struct {
	// targetPath is the file the Host block is written into.
	targetPath string
	// needsIncludeLine reports that ~/.ssh/config must also gain the gitid
	// Include line — true only on the fresh-machine first create (D-06).
	needsIncludeLine bool
	// includeLayout reports whether blocks live in an Include'd file rather
	// than in ~/.ssh/config itself.
	includeLayout bool
}

// storage resolves the active layout (D-05 — auto-detected, never an in-flow
// choice):
//
//   - an existing gitid-owned Include'd file (or an adopted external Include)
//     wins: keep writing where the blocks already are;
//   - otherwise, existing in-file managed blocks keep the in-file layout;
//   - otherwise the machine is fresh, and the Include'd layout is the DEFAULT
//     (D-06 — this supersedes STORE-01's documented in-file default).
func (b *realBackend) storage() storageLayout {
	inFile := storageLayout{targetPath: b.sshConfigPath}
	fresh := storageLayout{
		targetPath:       filepath.Join(b.includeDir, gitidConfigFileName),
		needsIncludeLine: true,
		includeLayout:    true,
	}
	if b.initErr != nil {
		return inFile
	}

	// 1. An existing gitid-owned Include'd target (sentinel-bearing, or the
	//    canonical config.d/gitid.config) is authoritative.
	if adopted, err := sshconfig.Adopt(b.sshConfigPath, sshconfig.AdoptSentinelBearing, "", sshconfig.RealAdoptDeps()); err == nil {
		if adopted.TargetPath != "" {
			return storageLayout{targetPath: adopted.TargetPath, includeLayout: true}
		}
	}
	canonical := filepath.Join(b.includeDir, gitidConfigFileName)
	if fileExists(canonical) {
		return storageLayout{targetPath: canonical, includeLayout: true, needsIncludeLine: !b.hasIncludeLine()}
	}

	// 2. Existing in-file identity blocks keep the in-file layout.
	content, err := os.ReadFile(b.sshConfigPath) //nolint:gosec // trusted gitid-managed path (G304)
	if err == nil {
		for _, block := range filewriter.ListBlocks(content) {
			if !sshconfig.IsReservedBlockName(block.Name) {
				return inFile
			}
		}
	}

	// 3. Fresh machine — the Include'd layout is the default (D-06).
	return fresh
}

// storageTargetPath is the resolved file the managed Host block lands in.
func (b *realBackend) storageTargetPath() string { return b.storage().targetPath }

// hasIncludeLine reports whether ~/.ssh/config already pulls in the gitid-owned
// config.d directory, resolved through sshconfig's own Include detection rather
// than a text match, so a hand-edited but equivalent directive still counts.
func (b *realBackend) hasIncludeLine() bool {
	directives, err := sshconfig.DetectInclude(b.sshConfigPath)
	if err != nil {
		return false
	}
	for _, d := range directives {
		if filepath.Clean(d.Expanded) == filepath.Clean(b.includeDir) {
			return true
		}
	}
	return false
}

// writeSSHBlock persists the managed Host block and the macOS globals block
// into the resolved storage target, creating the Include'd layout first when
// this is a fresh machine's first create (D-06). Every write routes through
// internal/sshconfig, and therefore through the filewriter backup + atomic
// temp->rename->chmod chokepoint — never os.WriteFile.
func (b *realBackend) writeSSHBlock(accountName, hostBlock, globalBlock string) (string, error) {
	if b.initErr != nil {
		return "", b.initErr
	}
	st := b.storage()
	if st.needsIncludeLine {
		if err := sshconfig.EnsureIncludeDir(b.includeDir); err != nil {
			return "", err
		}
		if _, err := sshconfig.EnsureIncludeLine(b.sshConfigPath); err != nil {
			return "", err
		}
	}
	return sshconfig.Write(st.targetPath, accountName, hostBlock, globalBlock)
}

// ---------------------------------------------------------------------------
// Reads over the user's real configuration
// ---------------------------------------------------------------------------

// accounts reconstructs every identity from the user's configuration,
// Include-aware so a fresh D-06 machine's identities are visible.
func (b *realBackend) accounts() []identity.Account {
	deps := identity.BuildInventoryDeps()
	sshBytes, err := deps.ReadSSHConfig()
	if err != nil {
		return nil
	}
	gcBytes, err := deps.ReadGitconfig()
	if err != nil {
		return nil
	}
	accounts, err := identity.Reconstruct(sshBytes, gcBytes, deps.ReadFragment)
	if err != nil {
		return nil
	}
	return accounts
}

// keyOwners maps a key path to the D-12 "in use by" label — "<identity>
// (<provider-host>)", e.g. "personal (github.com)" — so the picker's label
// renders verbatim and the wizard's same-provider check (D-12) can test
// whether the form's current SSH Host suffix occurs in it via a plain string
// comparison, without tuikit ever inspecting an identity type. It is derived
// from the SAME reconstruction the identity list renders, so the picker's
// labels can never disagree with the manager.
func (b *realBackend) keyOwners() map[string]string {
	owners := make(map[string]string)
	for _, acct := range b.accounts() {
		if acct.KeyPath == "" {
			continue
		}
		label := acct.Name
		if provider := providerFromAlias(acct.Alias); provider != "" {
			label += " (" + provider + ")"
		}
		owners[acct.KeyPath] = label
	}
	return owners
}

// ---------------------------------------------------------------------------
// In-flight create state
// ---------------------------------------------------------------------------

// createInput builds the CreateInput for a committed create from the view row
// the wizard produced, filling the gitid-managed target paths and the macOS
// globals block (D-08: written on EVERY create, empty off darwin).
func (b *realBackend) createInput(row tuikit.DemoIdentity) identity.CreateInput {
	port := row.Port
	if port == 0 {
		port = identity.DefaultPort()
	}
	hostname := row.Hostname
	// Derive the provider from the DemoIdentity.Provider field when present
	// (WR-01: the validated provider survives intact rather than being
	// reconstructed from the alias suffix by providerFromAlias, which
	// truncates multi-label providers like "company.co.uk" to "co.uk").
	provider := row.Provider
	if provider == "" {
		provider = providerFromAlias(row.SSHHost)
	}
	if hostname == "" {
		hostname = identity.DefaultHostname(provider)
	}
	// CR-10: use the selected algorithm from DemoIdentity.Algorithm; fall back
	// to "ed25519" only when no algorithm was recorded (e.g. legacy callers
	// that do not set the field).
	algo := row.Algorithm
	if algo == "" {
		algo = "ed25519"
	}
	// CR-07: ReuseKeyPath is included so specFingerprint distinguishes generate
	// vs reuse, and different reuse paths produce different fingerprints.
	// Normalize with resolveKeyPath so ~/... and /home/... for the same path
	// produce the same fingerprint.
	reuseKeyPath := ""
	if row.ReuseKeyPath != "" {
		reuseKeyPath = b.resolveKeyPath(row.ReuseKeyPath)
	}
	return identity.CreateInput{
		Name:               row.Name,
		GitName:            row.GitName,
		GitEmail:           row.GitEmail,
		Provider:           provider,
		Algo:               algo,
		Alias:              row.SSHHost,
		Hostname:           hostname,
		Port:               port,
		ReuseKeyPath:       reuseKeyPath,
		FragmentPath:       filepath.Join(b.fragmentDir, row.Name),
		GitconfigPath:      b.gitconfigPath,
		SSHConfigPath:      b.storageTargetPath(),
		AllowedSignersPath: b.allowedSigners,
		GlobalBlock:        sshconfig.RenderGlobalBlock(platform.CurrentOS()),
	}
}

// createInputFromSpec builds the CreateInput the test stages run against.
func (b *realBackend) createInputFromSpec(spec tuikit.CreateSpec) identity.CreateInput {
	algo := spec.Algorithm
	if algo == "" {
		algo = "ed25519"
	}
	in := b.createInput(tuikit.DemoIdentity{
		Name:         spec.Identity,
		SSHHost:      spec.Alias,
		Hostname:     spec.Hostname,
		Port:         atoiOr(spec.Port, identity.DefaultPort()),
		ReuseKeyPath: spec.ReuseKeyPath,
		Provider:     spec.Provider,
	})
	in.Algo = algo
	if spec.Provider != "" {
		in.Provider = spec.Provider
	}
	return in
}

// stagedKeyFor returns the key material for in's identity, keyed on BOTH the
// identity name and the D-10 reuse choice so a mid-flow switch between
// "generate" and "reuse <path>" re-stages instead of serving a stale cache
// hit. reuseKeyPath empty means generate a fresh key; non-empty stages the
// existing key at that path via identity.StageReuse (KEY-06/D-11) — the SAME
// ensurePub logic identity.Reuse itself uses, never a re-derived copy. The
// staged material is reused for every later stage and the confirmed write,
// so the key tested is provably the key written.
func (b *realBackend) stagedKeyFor(in identity.CreateInput, reuseKeyPath string) (identity.StagedKey, error) {
	b.mu.Lock()
	if b.stagedFor == in.Name && b.stagedReuseKeyPath == reuseKeyPath && b.staged.FinalPrivatePath != "" {
		staged := b.staged
		b.mu.Unlock()
		return staged, nil
	}
	b.mu.Unlock()

	var staged identity.StagedKey
	var err error
	if reuseKeyPath != "" {
		staged, err = identity.StageReuse(b.resolveKeyPath(reuseKeyPath), in.Name+"@gitid", b.deps)
	} else {
		staged, err = b.deps.Generate(in)
	}
	if err != nil {
		return identity.StagedKey{}, err
	}

	b.mu.Lock()
	b.staged, b.stagedFor, b.stagedReuseKeyPath, b.stagedIn = staged, in.Name, reuseKeyPath, in
	b.mu.Unlock()
	return staged, nil
}

// clearStaged drops the in-flight create's key material reference once the
// write is committed (the key itself stays on disk — it is the identity's).
func (b *realBackend) clearStaged() {
	b.mu.Lock()
	b.staged, b.stagedFor, b.stagedReuseKeyPath, b.stagedIn = identity.StagedKey{}, "", "", identity.CreateInput{}
	b.mu.Unlock()
}

// specFingerprint is a stable identifier for a CreateSpec + reuse choice. Two
// specs with the same fingerprint produce the same stage outcomes; changing
// any of these values invalidates prior proof (CR-07).
//
// Includes:
//   - Alias, Hostname, Port: the connection target
//   - Algo: the key algorithm (ed25519, rsa-4096, …)
//   - Name: the identity name
//   - Provider: the validated provider (not a truncated alias suffix)
//   - ReuseKeyPath: empty for generate, normalized absolute path for reuse —
//     so switching between generate and reuse, or between two reuse paths,
//     invalidates staged material and both accepted outcomes.
func specFingerprint(in identity.CreateInput) string {
	return fmt.Sprintf("%s|%s|%d|%s|%s|%s|%s", in.Alias, in.Hostname, in.Port, in.Algo, in.Name, in.Provider, in.ReuseKeyPath)
}

// recordOutcomeFor stores a stage's outcome bound to the current create spec.
func (b *realBackend) recordOutcomeFor(stage int, o tuikit.TestOutcome, spec identity.CreateInput) {
	fp := specFingerprint(spec)
	b.mu.Lock()
	defer b.mu.Unlock()
	b.outcomeSpec = fp
	switch stage {
	case 1:
		b.stage1Outcome, b.stage1Known = o, true
	case 2:
		b.stage2Outcome, b.stage2Known = o, true
	}
}

// recordOutcome is the test-facing wrapper that records a PASS on BOTH stages
// for a sentinel spec, preserving the D-01 gate tests written before the
// fail-closed spec-binding work.
func (b *realBackend) recordOutcome(o tuikit.TestOutcome) {
	b.recordOutcomeFor(1, o, identity.CreateInput{})
	b.recordOutcomeFor(2, o, identity.CreateInput{})
}

// clearOutcomes drops the staged stage results, used when the create spec
// changes or the transaction completes.
func (b *realBackend) clearOutcomes() {
	b.mu.Lock()
	b.stage1Known, b.stage2Known, b.outcomeSpec = false, false, ""
	b.mu.Unlock()
}

// storeUnlockedFor reports whether the confirmed write may proceed (D-01 /
// WR-02): the current spec must have accepted stage-1 AND stage-2 outcomes.
// PASS and ReachableNotUploaded both unlock; a hard Failure or any
// stale/absent proof blocks.
func (b *realBackend) storeUnlockedFor(spec identity.CreateInput) bool {
	fp := specFingerprint(spec)
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.outcomeSpec != fp {
		return false
	}
	if !b.stage1Known || !b.stage2Known {
		return false
	}
	return b.stage1Outcome != tuikit.TestOutcomeFailure && b.stage2Outcome != tuikit.TestOutcomeFailure
}

// storeUnlocked is the test-facing wrapper: it returns true when both stages
// are known and neither is a hard Failure, regardless of spec fingerprint.
func (b *realBackend) storeUnlocked() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.stage1Known || !b.stage2Known {
		return false
	}
	return b.stage1Outcome != tuikit.TestOutcomeFailure && b.stage2Outcome != tuikit.TestOutcomeFailure
}

// CommitCreate performs the confirmed create transaction off the Bubble Tea
// update loop and returns a command that delivers a WizardCommitMsg with the
// real result. The transaction writes the key pair, the Include line (when
// needed), and the Host block in dependency order; any failure rolls every
// earlier mutation back in reverse order.
func (b *realBackend) CommitCreate(id tuikit.DemoIdentity) tea.Cmd {
	return func() tea.Msg {
		if b.initErr != nil {
			return tuikit.WizardCommitMsg{Err: b.initErr.Error()}
		}
		in := b.createInput(id)
		if !b.storeUnlockedFor(in) {
			return tuikit.WizardCommitMsg{Err: fmt.Sprintf(
				"gitid: refusing to write %q: the connectivity test failed or is stale, so nothing was proven about the provider", id.Name)}
		}
		staged, err := b.stagedKeyFor(in, id.ReuseKeyPath)
		if err != nil {
			return tuikit.WizardCommitMsg{Err: err.Error()}
		}
		backups, err := b.commitCreateTransaction(in, staged, id)
		if err != nil {
			return tuikit.WizardCommitMsg{Err: err.Error()}
		}
		b.clearStaged()
		b.clearOutcomes()
		b.setPersistErr(nil)
		return tuikit.WizardCommitMsg{Backups: backups}
	}
}

// commitCreateTransaction applies the confirmed SSH create as one coordinated
// unit and rolls back on any error. The write order is:
//  0. Real SSH directory creation/mode (CR-08: only on confirmation, not pre-confirm)
//  1. Private key (written for generated; chmod 0600 for reused — CR-09)
//  2. Public key (written for generated; chmod 0644 for reused)
//  3. Include line in ~/.ssh/config (fresh-machine layout)
//  4. Host block in the resolved storage target
//
// Rollback restores pre-write backups and removes transaction-created files so
// a failure at any step leaves the user's config exactly as it was. For reused
// keys, rollback restores the original file permissions (CR-09).
func (b *realBackend) commitCreateTransaction(in identity.CreateInput, staged identity.StagedKey, id tuikit.DemoIdentity) ([]string, error) {
	type backupOp struct {
		target string
		backup string // empty means target did not exist before the transaction
	}
	// modeOp records a file whose permission was changed so rollback can
	// restore it (CR-09: reused-key permission normalization).
	type modeOp struct {
		path     string
		prevMode os.FileMode
	}
	var ops []backupOp
	var modeOps []modeOp
	var createdDirs []string
	var rollback bool

	defer func() {
		if !rollback {
			return
		}
		for i := len(ops) - 1; i >= 0; i-- {
			op := ops[i]
			if op.backup != "" {
				_ = os.Remove(op.target)
				_ = os.Rename(op.backup, op.target)
			} else {
				_ = os.Remove(op.target)
			}
		}
		// Restore file modes in reverse order (CR-09).
		for i := len(modeOps) - 1; i >= 0; i-- {
			_ = os.Chmod(modeOps[i].path, modeOps[i].prevMode)
		}
		// Remove created directories in REVERSE order (CR-09): children must be
		// removed before their parents. The forward order tried to remove ~/.ssh
		// while ~/.ssh/config.d still existed, leaving both directories behind.
		for i := len(createdDirs) - 1; i >= 0; i-- {
			_ = os.Remove(createdDirs[i])
		}
	}()

	addOp := func(target, backup string) {
		ops = append(ops, backupOp{target: target, backup: backup})
	}

	inject := func(step string) error {
		if b.failCommitAt != nil {
			return b.failCommitAt(step)
		}
		return nil
	}

	// 0. Real SSH directory (CR-08: create and/or chmod only in the confirmed
	// transaction, never before consent). On a fresh machine, the directory
	// does not yet exist; on an existing machine, we ensure it is 0700.
	if err := inject("ssh-dir"); err != nil {
		rollback = true
		return nil, err
	}
	sshDirInfo, sshDirErr := os.Stat(b.sshDir)
	if os.IsNotExist(sshDirErr) {
		// Fresh machine: create ~/.ssh at mode 0700.
		if merr := os.MkdirAll(b.sshDir, sshDirMode); merr != nil {
			rollback = true
			return nil, fmt.Errorf("gitid: creating %s: %w", b.sshDir, merr)
		}
		createdDirs = append(createdDirs, b.sshDir)
	} else if sshDirErr == nil && sshDirInfo.Mode().Perm() != sshDirMode {
		// Existing directory with wrong mode: chmod it and record the original
		// mode for rollback.
		prevMode := sshDirInfo.Mode().Perm()
		if cerr := os.Chmod(b.sshDir, sshDirMode); cerr != nil {
			rollback = true
			return nil, fmt.Errorf("gitid: securing %s: %w", b.sshDir, cerr)
		}
		modeOps = append(modeOps, modeOp{path: b.sshDir, prevMode: prevMode})
	} else if sshDirErr != nil {
		rollback = true
		return nil, fmt.Errorf("gitid: checking %s: %w", b.sshDir, sshDirErr)
	}

	// 1. Private key.
	if err := inject("private-key"); err != nil {
		rollback = true
		return nil, err
	}
	if staged.PrivPEM != nil {
		// Generated key: write the PEM at 0600.
		bk, err := filewriter.Write(staged.FinalPrivatePath, staged.PrivPEM, keyFileMode)
		if err != nil {
			rollback = true
			return nil, fmt.Errorf("gitid: writing private key: %w", err)
		}
		addOp(staged.FinalPrivatePath, bk)
	} else {
		// Reused key (nil PrivPEM — CR-09): normalise the private key to 0600.
		// Record the original mode so it can be restored on rollback.
		privInfo, serr := os.Stat(staged.FinalPrivatePath)
		if serr != nil {
			rollback = true
			return nil, fmt.Errorf("gitid: checking reused private key %s: %w", staged.FinalPrivatePath, serr)
		}
		prevMode := privInfo.Mode().Perm()
		if prevMode != keyFileMode {
			if cerr := os.Chmod(staged.FinalPrivatePath, keyFileMode); cerr != nil {
				rollback = true
				return nil, fmt.Errorf("gitid: normalising reused private key mode: %w", cerr)
			}
			modeOps = append(modeOps, modeOp{path: staged.FinalPrivatePath, prevMode: prevMode})
		}
	}

	// 2. Public key.
	if err := inject("public-key"); err != nil {
		rollback = true
		return nil, err
	}
	pubLine := staged.PubLine
	if pubLine != "" {
		bk, err := filewriter.Write(staged.FinalPubPath, []byte(pubLine), pubFileMode)
		if err != nil {
			rollback = true
			return nil, fmt.Errorf("gitid: writing public key: %w", err)
		}
		addOp(staged.FinalPubPath, bk)
	}

	// 3. Include line in ~/.ssh/config (fresh-machine layout only).
	st := b.storage()
	if st.needsIncludeLine {
		if err := inject("include-line"); err != nil {
			rollback = true
			return nil, err
		}
		if err := sshconfig.EnsureIncludeDir(b.includeDir); err != nil {
			rollback = true
			return nil, err
		}
		createdDirs = append(createdDirs, b.includeDir)
		bk, err := sshconfig.EnsureIncludeLine(b.sshConfigPath)
		if err != nil {
			rollback = true
			return nil, fmt.Errorf("gitid: ensuring Include line: %w", err)
		}
		addOp(b.sshConfigPath, bk)
	}

	// 4. Host block in the resolved storage target.
	if err := inject("host-block"); err != nil {
		rollback = true
		return nil, err
	}
	// CR-08: Use the checked renderer at the final write boundary — this is the
	// last safety gate before any bytes enter the user's SSH config. An unsafe
	// IdentityFile token would otherwise silently produce a malformed directive.
	hostBlock, err := sshconfig.RenderCheckedHostBlock(in.Alias, in.Hostname, in.Port, staged.FinalPrivatePath, in.Provider)
	if err != nil {
		rollback = true
		return nil, fmt.Errorf("gitid: validating host block: %w", err)
	}
	bk, err := sshconfig.Write(st.targetPath, in.Name, hostBlock, in.GlobalBlock)
	if err != nil {
		rollback = true
		return nil, fmt.Errorf("gitid: writing ssh config: %w", err)
	}
	addOp(st.targetPath, bk)

	if id.GitConfigured && id.GitName != "" && id.GitEmail != "" {
		if err := inject("git-fragment"); err != nil {
			rollback = true
			return nil, err
		}
		fragmentPath := filepath.Join(b.fragmentDir, in.Name)
		if err := gitconfig.WriteFragment(fragmentPath, id.GitName, id.GitEmail, id.KeyPath+".pub", true); err != nil {
			rollback = true
			return nil, fmt.Errorf("gitid: writing Git fragment: %w", err)
		}

		if err := inject("git-includeif"); err != nil {
			rollback = true
			return nil, err
		}
		gitSpec := tuikit.GitSpec{Identity: id.Name, Strategy: id.MatchStrategy, SSHHost: id.SSHHost}
		if gitSpec.Strategy == "" {
			gitSpec.Strategy = "gitdir"
		}
		bk, err = gitconfig.WriteIncludeIf(b.gitconfigPath, in.Name, "~/.gitconfig.d/"+in.Name, matchesFor(gitSpec))
		if err != nil {
			rollback = true
			return nil, fmt.Errorf("gitid: writing Git includeIf: %w", err)
		}
		addOp(b.gitconfigPath, bk)
		if err := gitconfig.SetAllowedSignersFile(b.gitconfigPath, b.allowedSigners); err != nil {
			rollback = true
			return nil, fmt.Errorf("gitid: setting allowed signers file: %w", err)
		}

		if err := inject("allowed-signers"); err != nil {
			rollback = true
			return nil, err
		}
		bk, err = keygen.WriteAllowedSigners(b.allowedSigners, in.Name, keygen.AllowedSignersLine(id.GitEmail, staged.PubLine))
		if err != nil {
			rollback = true
			return nil, fmt.Errorf("gitid: writing allowed_signers: %w", err)
		}
		addOp(b.allowedSigners, bk)
	}

	// Collect timestamped backup paths for the ceremony receipt.
	var displayBackups []string
	for _, op := range ops {
		if op.backup != "" {
			displayBackups = append(displayBackups, b.displayPath(op.backup))
		}
	}
	return displayBackups, nil
}

// setPersistErr records (or clears) the last committed write's failure.
func (b *realBackend) setPersistErr(err error) {
	b.mu.Lock()
	b.persistErr = err
	b.mu.Unlock()
}

// stagingDir returns the throwaway directory both test stages run against,
// created once per session at 0700.
func (b *realBackend) stagingDir() (string, error) {
	if b.initErr != nil {
		return "", b.initErr
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.stageDir != "" {
		return b.stageDir, nil
	}
	dir := os.Getenv("GITID_STAGE_DIR")
	if dir == "" {
		var err error
		dir, err = os.MkdirTemp("", "gitid-stage-")
		if err != nil {
			return "", fmt.Errorf("gitid: creating the staging directory: %w", err)
		}
	} else if !filepath.IsAbs(dir) {
		return "", fmt.Errorf("gitid: GITID_STAGE_DIR must be an absolute path")
	} else if err := os.MkdirAll(dir, sshDirMode); err != nil { //nolint:gosec // absolute staging path is explicitly supplied by this invoking user
		return "", fmt.Errorf("gitid: creating configured staging directory: %w", err)
	}
	if cerr := os.Chmod(dir, sshDirMode); cerr != nil { //nolint:gosec // absolute staging path is explicitly supplied by this invoking user
		return "", fmt.Errorf("gitid: securing the staging directory: %w", cerr)
	}
	b.stageDir = dir
	return dir, nil
}

// knownHostsPath returns the throwaway known_hosts file both test stages write
// to, keeping the user's ~/.ssh/known_hosts untouched before confirmation.
func (b *realBackend) knownHostsPath() (string, error) {
	dir, err := b.stagingDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "known_hosts"), nil
}

// ---------------------------------------------------------------------------
// Small helpers
// ---------------------------------------------------------------------------

// stageFailure builds the WizardStageMsg for a stage that could not even run,
// and records the outcome so the D-01 store gate blocks the write. A local
// failure is a hard Failure, never a reachable-not-uploaded warning: nothing
// was proven about the provider.
func (b *realBackend) stageFailure(stage int, command string, err error, in identity.CreateInput) tuikit.WizardStageMsg {
	b.recordOutcomeFor(stage, tuikit.TestOutcomeFailure, in)
	return tuikit.WizardStageMsg{Stage: stage, Result: tuikit.TestResultView{
		Outcome: tuikit.TestOutcomeFailure,
		Command: command,
		Detail:  err.Error(),
	}}
}

// matchesFor renders the includeIf match rules for spec's strategy. "both"
// emits the gitdir and hasconfig rules together in one managed block (GIT-02).
func matchesFor(spec tuikit.GitSpec) []gitconfig.Match {
	gitdirPath := strings.TrimSpace(spec.GitDir)
	if gitdirPath == "" {
		gitdirPath = "~/git/" + spec.Identity + "/"
	}
	if !strings.HasSuffix(gitdirPath, "/") {
		gitdirPath += "/"
	}
	gitdir := gitconfig.Match{Kind: gitconfig.MatchGitdir, Value: gitdirPath}
	// SSHHost is the validated alias from the SSH block. Falling back only when
	// legacy callers omit it preserves older create flow data without creating
	// a glob or changing a provided alias.
	sshHost := spec.SSHHost
	if sshHost == "" {
		sshHost = spec.Identity + ".github.com"
	}
	hasconfig := gitconfig.Match{
		Kind:  gitconfig.MatchHasconfig,
		Value: "remote.*.url:git@" + sshHost + ":*/**",
	}
	switch spec.Strategy {
	case "hasconfig":
		return []gitconfig.Match{hasconfig}
	case "both":
		return []gitconfig.Match{gitdir, hasconfig}
	default:
		return []gitconfig.Match{gitdir}
	}
}

// providerFromAlias infers the provider host from an SSH alias (D-20: the
// provider is read off the Host suffix, never a separate field).
// "personal.github.com" -> "github.com"; a bare "github.com" -> itself.
func providerFromAlias(alias string) string {
	alias = strings.TrimSpace(alias)
	parts := strings.Split(alias, ".")
	if len(parts) <= 2 {
		return alias
	}
	return strings.Join(parts[len(parts)-2:], ".")
}

// resolveKeyPath expands a `~/`-relative display path into a real filesystem
// path. The wizard speaks display paths; the tester and the filesystem do not.
func (b *realBackend) resolveKeyPath(path string) string {
	if !strings.HasPrefix(path, "~/") || b.home == "" {
		return path
	}
	return filepath.Join(b.home, path[2:])
}

// displayPath is resolveKeyPath's inverse: the `~/`-shortened form shown to the
// user, so previews and ceremonies read like the recipes do.
func (b *realBackend) displayPath(path string) string {
	if path == "" || b.home == "" {
		return path
	}
	if rel, err := filepath.Rel(b.home, path); err == nil && !strings.HasPrefix(rel, "..") {
		return filepath.Join("~", rel)
	}
	return path
}

// atoiOr parses s, falling back to fallback when it is not a plain integer.
// The SSH form gates advancing on a valid port, so the fallback is only ever
// visible mid-keystroke in the live preview.
func atoiOr(s string, fallback int) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return fallback
	}
	return n
}

// fileExists reports whether path is present on disk.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
