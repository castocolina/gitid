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
	// lastOutcome is the most recent connectivity-test outcome, and
	// outcomeKnown whether any stage has answered yet. Together they are the
	// D-01 STORE GATE at the backend seam: PASS and ReachableNotUploaded both
	// unlock the write (a brand-new key cannot authenticate before its .pub is
	// uploaded — that is a warning, not a failure); only a hard Failure blocks.
	lastOutcome  tuikit.TestOutcome
	outcomeKnown bool
	persistErr   error
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
		// Generate writes the key pair straight to its final ~/.ssh path so the
		// key exists (and is uploadable) before the connectivity gate runs.
		// TempPrivatePath == FinalPrivatePath; Cleanup is therefore a no-op.
		Generate: func(in identity.CreateInput) (identity.StagedKey, error) {
			if err := filewriter.EnsureDir(b.sshDir, sshDirMode); err != nil {
				return identity.StagedKey{}, fmt.Errorf("gitid: ensuring %s: %w", b.sshDir, err)
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
			if _, werr := filewriter.Write(finalPriv, mat.PrivPEM, keyFileMode); werr != nil {
				return identity.StagedKey{}, fmt.Errorf("gitid: writing private key: %w", werr)
			}
			if _, werr := filewriter.Write(finalPub, []byte(mat.PubLine), pubFileMode); werr != nil {
				return identity.StagedKey{}, fmt.Errorf("gitid: writing public key: %w", werr)
			}
			return identity.StagedKey{
				TempPrivatePath:  finalPriv,
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
		// Generate writes to the final path directly, so there is no temp
		// staging directory to remove.
		Cleanup: func(identity.StagedKey) {},
		CopyPub: clipboard.Copy,
		PreWrite: func(keyPath, hostname string, port int) tester.Result {
			return tester.PreWrite(keyPath, hostname, port)
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
		StageTestConfig: func(in identity.CreateInput, staged identity.StagedKey) (string, error) {
			if staged.TempPrivatePath == "" {
				return "", fmt.Errorf("gitid: no staged key path for the test config")
			}
			dir, derr := b.stagingDir()
			if derr != nil {
				return "", derr
			}
			hostBlock := sshconfig.RenderHostBlock(in.Alias, in.Hostname, in.Port, staged.TempPrivatePath, in.Provider)
			configPath := filepath.Join(dir, "config")
			if _, werr := filewriter.Write(configPath, []byte(hostBlock), keyFileMode); werr != nil {
				return "", fmt.Errorf("gitid: staging the test ssh config: %w", werr)
			}
			return configPath, nil
		},
		ResolvedVia: tester.ResolvedVia,
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
	if !b.storeUnlocked() {
		b.setPersistErr(fmt.Errorf(
			"gitid: refusing to write %q: the connectivity test failed, so nothing was proven about the provider", a.Identity.Name))
		return state
	}
	in := b.createInput(a.Identity)
	staged, err := b.stagedKeyFor(in, a.Identity.ReuseKeyPath)
	if err != nil {
		b.setPersistErr(err)
		return state
	}
	if _, err := identity.PersistSSH(in, staged, b.deps); err != nil {
		b.setPersistErr(err)
		return state
	}
	b.clearStaged()
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
// sshconfig.RenderHostBlock the confirmed write uses — so "written exactly
// like this on confirm" is structurally true, not a re-typed lookalike
// (SSHUI-03: Port 443 + IdentitiesOnly yes per recipes/).
func (b *realBackend) HostBlockPreview(spec tuikit.CreateSpec) string {
	return sshconfig.RenderHostBlock(
		spec.Alias,
		spec.Hostname,
		atoiOr(spec.Port, identity.DefaultPort()),
		spec.KeyPath,
		providerFromAlias(spec.Alias),
	)
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

// AliasCollision reports whether identity is already claimed — the D-09 gate
// the wizard holds step 1 on. It checks the user's REAL configuration
// Include-aware (a fresh D-06 machine keeps every identity block in
// config.d/gitid.config, so reading ~/.ssh/config alone would see none), plus
// every Host pattern in the file, managed AND hand-written: gitid must never
// write an ambiguous first-match-wins alias.
func (b *realBackend) AliasCollision(state tuikit.DemoState, name string) bool {
	if strings.TrimSpace(name) == "" {
		return false
	}
	for _, row := range state.Identities {
		if row.Name == name {
			return true
		}
	}
	if b.initErr != nil {
		return false
	}
	names, err := sshconfig.ManagedBlockNames(b.sshConfigPath)
	if err == nil {
		for _, n := range names {
			if n == name && !sshconfig.IsReservedBlockName(n) {
				return true
			}
		}
	}
	for _, host := range b.hostPatterns() {
		if host == name {
			return true
		}
	}
	return false
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
	return tester.PreWriteCommand(b.resolveKeyPath(spec.KeyPath), spec.Hostname, atoiOr(spec.Port, identity.DefaultPort()))
}

// Stage2Command is the exact stage-2 command: the alias resolved THROUGH the
// staged temp config (TEST-02).
func (b *realBackend) Stage2Command(spec tuikit.CreateSpec) string {
	return tester.ResolvedViaCommand(b.TestConfigPath(), b.resolveKeyPath(spec.KeyPath), spec.Alias)
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
			return b.stageFailure(1, b.Stage1Command(spec), err)
		}
		res := b.deps.PreWrite(staged.TempPrivatePath, in.Hostname, in.Port)
		view := toTestResultView(res, res.Command)
		b.recordOutcome(view.Outcome)
		return tuikit.WizardStageMsg{Stage: 1, Result: view}
	}
}

// TestStage2 runs the real stage-2 test: the alias resolved BY NAME through the
// staged throwaway config — no -i by design, since proving the config supplies
// the key is the whole point (TEST-02).
func (b *realBackend) TestStage2(spec tuikit.CreateSpec) tea.Cmd {
	return func() tea.Msg {
		in := b.createInputFromSpec(spec)
		staged, err := b.stagedKeyFor(in, spec.ReuseKeyPath)
		if err != nil {
			return b.stageFailure(2, b.Stage2Command(spec), err)
		}
		configPath, err := b.deps.StageTestConfig(in, staged)
		if err != nil {
			return b.stageFailure(2, b.Stage2Command(spec), err)
		}
		res, resolved := b.deps.ResolvedVia(configPath, staged.TempPrivatePath, in.Alias)
		view := toTestResultView(res, res.Command)
		b.recordOutcome(view.Outcome)
		if len(resolved.IdentityFiles) > 0 {
			// The stage-2 proof the user asked for: which key the ALIAS
			// actually resolves to.
			view.Detail = "identityfile " + resolved.IdentityFiles[0]
		}
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

// gitStepDisabledReason is the D-19 frozen reason string the REAL binary
// shows under the wizard's Git-identity step [ Continue ] button — the
// existing form-validity reason ("— needs user.name + a valid email") would
// be a LIE about capability here: there is no Git backend behind Continue
// until Phase 4, so it must never enable regardless of what the user typed.
const gitStepDisabledReason = "— Git configuration arrives with the next build"

// GitStepDisabledReason implements D-19: the real binary ALWAYS disables
// the wizard's Git-identity step [ Continue ] button, with its own honest
// reason — never the dummy's validity-based one.
func (b *realBackend) GitStepDisabledReason() (string, bool) {
	return gitStepDisabledReason, true
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
	view := tuikit.TestResultView{Command: command, Detail: strings.TrimSpace(res.Output)}
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
		if strings.HasPrefix(filepath.Clean(d.Expanded), filepath.Clean(b.includeDir)) {
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

// hostPatterns is every Host alias in the user's configuration — gitid-managed
// AND hand-written — for the D-09 collision check.
func (b *realBackend) hostPatterns() []string {
	deps := identity.BuildInventoryDeps()
	sshBytes, err := deps.ReadSSHConfig()
	if err != nil {
		return nil
	}
	hosts, err := sshconfig.ParseManagedHosts(sshBytes)
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(hosts))
	for _, info := range hosts {
		if info.Alias != "" {
			out = append(out, info.Alias)
		}
	}
	return out
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
	if hostname == "" {
		hostname = identity.DefaultHostname(providerFromAlias(row.SSHHost))
	}
	return identity.CreateInput{
		Name:               row.Name,
		GitName:            row.GitName,
		GitEmail:           row.GitEmail,
		Provider:           providerFromAlias(row.SSHHost),
		Algo:               "ed25519",
		Alias:              row.SSHHost,
		Hostname:           hostname,
		Port:               port,
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
	return identity.CreateInput{
		Name:               spec.Identity,
		Provider:           providerFromAlias(spec.Alias),
		Algo:               algo,
		Alias:              spec.Alias,
		Hostname:           spec.Hostname,
		Port:               atoiOr(spec.Port, identity.DefaultPort()),
		FragmentPath:       filepath.Join(b.fragmentDir, spec.Identity),
		GitconfigPath:      b.gitconfigPath,
		SSHConfigPath:      b.storageTargetPath(),
		AllowedSignersPath: b.allowedSigners,
		GlobalBlock:        sshconfig.RenderGlobalBlock(platform.CurrentOS()),
	}
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

// recordOutcome stores a stage's outcome for the D-01 store gate.
func (b *realBackend) recordOutcome(o tuikit.TestOutcome) {
	b.mu.Lock()
	b.lastOutcome, b.outcomeKnown = o, true
	b.mu.Unlock()
}

// storeUnlocked reports whether the confirmed write may proceed (D-01): PASS
// and ReachableNotUploaded both unlock it — a brand-new key legitimately
// cannot authenticate until its .pub is uploaded (Phase 9) — and only a hard
// Failure blocks. A create whose stages never ran is not blocked here; the
// wizard is what sequences them.
func (b *realBackend) storeUnlocked() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return !b.outcomeKnown || b.lastOutcome != tuikit.TestOutcomeFailure
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
	dir, err := os.MkdirTemp("", "gitid-stage-")
	if err != nil {
		return "", fmt.Errorf("gitid: creating the staging directory: %w", err)
	}
	if cerr := os.Chmod(dir, sshDirMode); cerr != nil {
		return "", fmt.Errorf("gitid: securing the staging directory: %w", cerr)
	}
	b.stageDir = dir
	return dir, nil
}

// ---------------------------------------------------------------------------
// Small helpers
// ---------------------------------------------------------------------------

// stageFailure builds the WizardStageMsg for a stage that could not even run,
// and records the outcome so the D-01 store gate blocks the write. A local
// failure is a hard Failure, never a reachable-not-uploaded warning: nothing
// was proven about the provider.
func (b *realBackend) stageFailure(stage int, command string, err error) tuikit.WizardStageMsg {
	b.recordOutcome(tuikit.TestOutcomeFailure)
	return tuikit.WizardStageMsg{Stage: stage, Result: tuikit.TestResultView{
		Outcome: tuikit.TestOutcomeFailure,
		Command: command,
		Detail:  err.Error(),
	}}
}

// matchesFor renders the includeIf match rules for spec's strategy. "both"
// emits the gitdir and hasconfig rules together in one managed block (GIT-02).
func matchesFor(spec tuikit.GitSpec) []gitconfig.Match {
	gitdir := gitconfig.Match{Kind: gitconfig.MatchGitdir, Value: "~/git/" + spec.Identity + "/"}
	hasconfig := gitconfig.Match{
		Kind:  gitconfig.MatchHasconfig,
		Value: "remote.*.url:git@" + spec.Identity + ".*:*/**",
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
