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
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/castocolina/gitid/internal/clipboard"
	"github.com/castocolina/gitid/internal/deps"
	"github.com/castocolina/gitid/internal/doctor"
	"github.com/castocolina/gitid/internal/doctor/checks"
	"github.com/castocolina/gitid/internal/filewriter"
	"github.com/castocolina/gitid/internal/gitconfig"
	"github.com/castocolina/gitid/internal/globalgit"
	"github.com/castocolina/gitid/internal/globalssh"
	"github.com/castocolina/gitid/internal/identity"
	"github.com/castocolina/gitid/internal/keygen"
	"github.com/castocolina/gitid/internal/platform"
	"github.com/castocolina/gitid/internal/sshconfig"
	"github.com/castocolina/gitid/internal/tester"
	"github.com/castocolina/gitid/internal/tuikit"
	"github.com/castocolina/gitid/internal/upload"
	"github.com/castocolina/gitid/internal/uploader"
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
	// txMu (WR-04) serializes the file-mutation transactions themselves —
	// commitCreateTransaction and commitGitTransaction/commitGitArtifacts.
	// CommitCreate and CommitGit each return a tea.Cmd that Bubble Tea runs
	// in its OWN goroutine; without this lock, two overlapping transactions
	// could interleave read-modify-write cycles on ~/.gitconfig and
	// ~/.ssh/allowed_signers into a lost update, and both journals would
	// snapshot the same pre-state, so a rollback could resurrect a stale
	// file. mu (above) guards only the staged*/stage*Outcome/persistErr
	// fields — a narrower, unrelated concern.
	txMu sync.Mutex
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

	// fixFnOverride is a test-only injection point (mirroring failCommitAt's
	// precedent): when non-nil, persistFixFinding calls it INSTEAD of the
	// matched finding's real Fix.Fn — the only way to reproduce D-14's
	// convergence-alarm path with a real "the fix reported success but the
	// finding's signature is still present" disagreement without a flaky
	// external race (every REAL check's Fix.Fn either genuinely fixes the
	// condition or genuinely fails; only a test double can report success
	// while leaving the condition unchanged). Nil in production.
	fixFnOverride func() error

	// verifyAuthorResolution is a test-only override of the D-06 post-write
	// probe. Nil means the real globalgit.VerifyAuthorResolution.
	verifyAuthorResolution func(deps globalgit.Deps, matchedDir, unmatchedDir string) (globalgit.AuthorResolution, error)

	// probeSSHVersion is a test-only override for platform.ProbeSSHVersion so
	// version-gate outcomes can be driven without a live ssh -V. Nil means the
	// real probe.
	probeSSHVersion func() (platform.SSHVersion, error)

	// gitGate is a test-only override for the global-git version-gate outcome
	// (D-08's only WRITE-changing gate: merge.conflictstyle). Nil means
	// globalgit.RealGateForRow, which reads the single git-version probe in
	// internal/deps. The second return is the version note (informational);
	// the gate OUTCOME alone decides the written value.
	gitGate func() (globalgit.GateOutcome, string)

	// failArchiveRemoveAt is a test-only injection point (mirroring
	// failCommitAt's precedent): when non-nil, archiveKeyPairSeam's source
	// removal calls it before removing path, letting a test drive a
	// deterministic second-source-removal failure through the REAL
	// composition root (review R3-01) rather than a hand-built fake.
	failArchiveRemoveAt func(path string) error

	// archiveClockNow is a test-only override for the clock
	// archiveKeyPairSeam uses to build its UnixNano archive stamp — nil
	// means time.Now. Injecting a clock that returns the same instant twice
	// makes a stamp collision reproducible without racing a real clock
	// (review R2-05/R-21's composition-root retry).
	archiveClockNow func() time.Time

	// pendingMigration is the ONE previewed MigrationPlan the backend holds
	// between SSHStorageMigrationPlan and CommitSSHStorage. It is covered by
	// pendingMigrationMu — a SEPARATE mutex from txMu, per <lock_contract>:
	// txMu covers the two config files as a coherent pair (every read AND
	// write of them); pendingMigrationMu covers only this slot, never held
	// across I/O. If it were txMu, runSSHStorageMigrate — which already holds
	// txMu — would self-deadlock the moment it called takePendingMigration.
	// Acquisition order when both are held: txMu first, pendingMigrationMu
	// second — never the reverse, so a lock cycle is not expressible.
	pendingMigration   sshconfig.MigrationPlan
	pendingMigrationMu sync.Mutex
	pendingToken       string // the opaque token the view carried

	// convergenceAlarmed tracks (08-02-PLAN.md Task 3, D-14) the stable IDs
	// of findings whose fix reported success but reappeared after the
	// mandatory post-fix full re-scan — session-scoped, in-memory only (no
	// persistence needed per 08-RESEARCH.md's Assumptions Log), guarded by
	// its own mutex since it is read/written independently of every other
	// field above.
	convergenceAlarmed   map[string]bool
	convergenceAlarmedMu sync.Mutex

	// uploadEligibilityMemo caches UploadEligibility's answer PER PROVIDER
	// KEY ("github"/"gitlab"), not per host — the probe answers a question
	// about the TOOL (is gh/glab present and authenticated for the
	// canonical domain), which is the same question for every host that
	// shares a provider key (github.com and ssh.github.com share one
	// cache entry). This exists so a wizard session cannot spawn one
	// "gh auth status" per host edit (R3). Guarded by its own mutex since
	// it is read/written independently of every other field above.
	uploadEligibilityMemo   map[string]tuikit.UploadEligibilityView
	uploadEligibilityMemoMu sync.Mutex
	// uploadEligibilityLocks holds one *sync.Mutex per provider key. WR-17:
	// the check-then-set critical section (09-04's fix for a duplicate-probe
	// race) used to be made atomic by holding uploadEligibilityMemoMu across
	// the ENTIRE probe, including DetectFor+AuthCheck's real subprocess
	// calls (up to providerCommandTimeout each) — so a slow/hung "gh auth
	// status" blocked an unrelated "glab" probe for the full timeout, even
	// though the two share no real resource. Locking per-provider (guarded
	// by uploadEligibilityMemoMu only for the brief get-or-create/map-access
	// steps, never across a subprocess call) keeps same-provider calls
	// serialized — the property TestUploadEligibilityMemoizesConcurrentProviderProbes
	// asserts — while different providers run fully concurrently.
	uploadEligibilityLocks map[string]*sync.Mutex

	// uploaderDeps is the real gh/glab exec wiring (buildUploaderDeps()).
	// Set once in newBackendForHome; tests may overwrite it directly with a
	// fake uploader.Deps to drive UploadEligibility/RunUpload without a real
	// gh/glab on PATH, mirroring how b.deps itself is test-overridable.
	uploaderDeps uploader.Deps

	// uploadConfirmSleep is a test-only override for the D-17 post-upload
	// confirmation retry's bounded wait — nil means the real
	// uploadConfirmRetryInterval, following the archiveClockNow precedent
	// (above) so the unit test can drive the interval to zero instead of
	// sleeping in real time.
	uploadConfirmSleep func(time.Duration)

	// uploadPhase is a TEST-VISIBLE marker (R8, 09-04-PLAN.md) set
	// immediately before every uploader.Inventory call RunUpload issues —
	// "dedupe" for the pre-upload missing-type diff, "confirmation" for the
	// D-17 post-upload read(s). It exists SOLELY so a test's fake RunCmd
	// closure (which captures the *realBackend) can record which phase each
	// invocation belongs to; production code never reads this field. Guarded
	// by its own mutex since it is written from the upload tea.Cmd goroutine
	// and read from a test fake that may run on the same goroutine.
	uploadPhase   string
	uploadPhaseMu sync.Mutex
}

// setUploadPhase and currentUploadPhase implement the R8 phase marker above.
func (b *realBackend) setUploadPhase(phase string) {
	b.uploadPhaseMu.Lock()
	b.uploadPhase = phase
	b.uploadPhaseMu.Unlock()
}

func (b *realBackend) currentUploadPhase() string {
	b.uploadPhaseMu.Lock()
	defer b.uploadPhaseMu.Unlock()
	return b.uploadPhase
}

// confirmSleep honors the test-only uploadConfirmSleep override (nil means
// a real time.Sleep for uploadConfirmRetryInterval).
func (b *realBackend) confirmSleep() {
	if b.uploadConfirmSleep != nil {
		b.uploadConfirmSleep(uploadConfirmRetryInterval)
		return
	}
	time.Sleep(uploadConfirmRetryInterval)
}

// uploadConfirmRetryInterval is the production D-17 post-upload confirmation
// retry wait: chosen to comfortably absorb ordinary GitHub/GitLab API
// read-after-write propagation lag (typically well under a second in
// practice) without making the wizard feel stalled during its single retry.
var uploadConfirmRetryInterval = 2 * time.Second

// compile-time proof the real composition root satisfies the seam.
var _ tuikit.Backend = (*realBackend)(nil)
var _ tuikit.IdentityPlanner = (*realBackend)(nil)

// plan 06-01 seam pin: the real composition root implements the global-SSH
// planner seam from backend.go. It must NOT get there by embedding
// NoopGlobalSSHPlanner — a reflection test in wiring_test.go asserts the
// struct carries no such anonymous field, so a missing real implementation
// stays a compile error, not a silent sentinel.
var _ tuikit.GlobalSSHPlanner = (*realBackend)(nil)

// plan 07-01 seam pin: the real composition root implements the global-git
// planner seam from backend.go. It must NOT get there by embedding
// NoopGlobalGitPlanner — a reflection test in wiring_test.go asserts the
// struct carries no such anonymous field, so a missing real implementation
// stays a compile error, not a silent sentinel.
var _ tuikit.GlobalGitPlanner = (*realBackend)(nil)

// plan 07-02 seam pin: the real composition root implements the fallback-
// author planner seam from backend.go. It must NOT get there by embedding
// NoopGitFallbackAuthorPlanner — a reflection test in wiring_test.go asserts
// the struct carries no such anonymous field.
var _ tuikit.GitFallbackAuthorPlanner = (*realBackend)(nil)

// plan 06-05 seam pin: the real composition root implements the storage
// planner seam. It must NOT embed NoopSSHStoragePlanner.
var _ tuikit.SSHStoragePlanner = (*realBackend)(nil)

// newMigrateDeps is the package-level indirection both SSHStorageMigrationPlan
// and runSSHStorageMigrate use to construct sshconfig.MigrateDeps. Using a
// variable rather than an inline call lets tests override it to wrap WriteFile
// (which IS exported) so they can inject failures and pauses —
// MigrateDeps.afterStep is unexported and unreachable from cmd/gitid, so this
// indirection is the only way a cmd/gitid test can wrap the seam without
// reaching into the internal package.
var newMigrateDeps = func(configPath, includePath string, aliases []string) sshconfig.MigrateDeps {
	return sshconfig.RealMigrateDeps(configPath, includePath, aliases)
}

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
	b.uploaderDeps = buildUploaderDeps()
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
		// Host block into the resolved target, normalising the globals block
		// for the requested platform through the single EnsureGlobals owner.
		WriteSSH: func(accountName, hostBlock, globalsGOOS string) (string, error) {
			return b.writeSSHBlock(accountName, hostBlock, globalsGOOS)
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
		// ArchiveKeyPair is the BACKEND-WIDE binding (review R3-01): it
		// REFUSES to archive, because a closure built here (buildIdentityDeps
		// runs ONCE, inside newBackendForHome, with no transaction in scope)
		// can never reach a rollback journal. The field stays non-nil so the
		// reflection guard is satisfied and its intent is preserved, but the
		// value fails CLOSED — an archive attempted with no journal watching
		// returns an error BEFORE any copy is made, rather than creating an
		// untracked archive entry. depsForTransaction is the ONLY way to
		// obtain a working archive seam; a fail-open no-op observer here
		// would reintroduce exactly the defect this refusal exists to
		// remove.
		ArchiveKeyPair: b.archiveKeyPairSeam(func(string) error { return errArchiveOutsideTransaction }),
		// AppendAllowedSigners is the D-07 append-not-replace writer, used by
		// BOTH key-lifecycle ceremonies (rotate, repair). Its signature
		// matches keygen.AppendAllowedSigners exactly — no adapter needed.
		AppendAllowedSigners: keygen.AppendAllowedSigners,
	}
}

// errArchiveOutsideTransaction is the backend-wide ArchiveKeyPair binding's
// fail-closed refusal (review R3-01): an archive attempted outside a
// mutationJournal transaction is refused before any copy is made. Only
// depsForTransaction(j) rebinds ArchiveKeyPair to a seam that actually
// archives.
var errArchiveOutsideTransaction = errors.New("gitid: refusing to archive a key pair outside a transaction")

// ---------------------------------------------------------------------------
// uploader.Deps — the Phase 9 (UP-02/UP-03) gh/glab wiring
// ---------------------------------------------------------------------------

// providerCommandTimeout bounds every subprocess buildUploaderDeps' RunCmd
// invokes (R3: no provider subprocess may hang the TUI). It is a package
// `var`, not a `const`, SOLELY so a test can shorten it — the same
// test-only-override precedent archiveClockNow (above) establishes for a
// different seam. The production value is chosen to comfortably cover an
// ordinary network-backed `gh`/`glab` call (auth status, ssh-key add) on a
// slow connection without leaving a hung process indefinitely blocking the
// wizard's upload beat.
var providerCommandTimeout = 20 * time.Second

// buildUploaderDeps wires uploader.Deps from the real exec package —
// re-derived from the archived cmd/gitid/copy.go's buildUploaderDeps
// (Phase-9-tracer POC layout), the ONLY function this task resurrects from
// that file. EVERY field is filled: a nil seam here is a silent behavior
// change, not a missing feature (TestUploaderDepsEveryFieldIsWired mirrors
// TestIdentityDepsEveryFieldIsWired's reflection guard).
//
// RunCmd is time-bounded via exec.CommandContext over a per-invocation
// context.WithTimeout (R3): a command that outlives providerCommandTimeout
// is killed and RunCmd returns a non-zero exit code and a non-nil error
// naming the timeout, instead of blocking the caller forever. That error
// text reaches the user only through the RedactCLIOutput-bounded
// last-resort reason path (plan 09-03) — it is operational runner text of
// the same class as raw CLI output, not new product copy subject to the
// copy-freeze.
func buildUploaderDeps() uploader.Deps {
	return uploader.Deps{
		LookPath: exec.LookPath,
		ReadFile: os.ReadFile,
		RunCmd: func(name string, args ...string) (string, int, error) {
			ctx, cancel := context.WithTimeout(context.Background(), providerCommandTimeout)
			defer cancel()
			cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec // arg-slice; no shell; name is a trusted resolved binary path (G204)
			out, err := cmd.CombinedOutput()
			output := string(out)
			if ctx.Err() == context.DeadlineExceeded {
				return output, 124, fmt.Errorf("gitid: %s timed out after %s: %w", name, providerCommandTimeout, ctx.Err())
			}
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

// archiveKeyPairSeam is the ONE archive implementation identity.Deps'
// ArchiveKeyPair field can be bound to — only its onCreated OBSERVER varies
// between the backend-wide refusing binding (buildIdentityDeps, above) and a
// transaction-bound working seam (depsForTransaction, below). It resolves
// the D-06 archive directory, builds the project's UnixNano stamp, and calls
// keygen.MoveKeyPairToArchive — retrying EXACTLY ONCE with a monotonically
// bumped stamp on a same-nanosecond collision (review R2-05/R-21: plan
// 05-02 ships the primitive's hard-error collision detection; this
// composition-root retry is what consumes it, and it lives here because
// this is the first task that writes this closure at all).
func (b *realBackend) archiveKeyPairSeam(onCreated keygen.CreatedFunc) func(privPath, pubPath string) (string, string, error) {
	return func(privPath, pubPath string) (string, string, error) {
		archiveDir := sshconfig.ArchiveDir(b.sshDir)
		remove := b.archiveRemove()

		stampNanos := b.archiveClock().UnixNano()
		pair, err := keygen.MoveKeyPairToArchive(archiveDir, privPath, pubPath, strconv.FormatInt(stampNanos, 10), remove, onCreated)
		if err != nil && errors.Is(err, os.ErrExist) {
			// Same-nanosecond stamp collision: the primitive's exclusive-
			// create leaves nothing behind and announces nothing, so a
			// monotonic bump — not a fresh clock draw, which the injected
			// test clock may not advance — is safe to retry exactly once.
			stampNanos++
			pair, err = keygen.MoveKeyPairToArchive(archiveDir, privPath, pubPath, strconv.FormatInt(stampNanos, 10), remove, onCreated)
		}
		return pair.PrivatePath, pair.PublicPath, err
	}
}

// archiveClock returns the clock archiveKeyPairSeam uses, honoring the
// test-only archiveClockNow override (nil means time.Now).
func (b *realBackend) archiveClock() time.Time {
	if b.archiveClockNow != nil {
		return b.archiveClockNow()
	}
	return time.Now()
}

// archiveRemove returns the source-removal function archiveKeyPairSeam
// passes to keygen.MoveKeyPairToArchive, honoring the test-only
// failArchiveRemoveAt injection point (nil means os.Remove) — the same
// precedent failCommitAt establishes elsewhere in this file, applied to the
// archive's own removal step so a test can drive the second-source-removal
// failure through the REAL composition root.
func (b *realBackend) archiveRemove() func(path string) error {
	return func(path string) error {
		if b.failArchiveRemoveAt != nil {
			if err := b.failArchiveRemoveAt(path); err != nil {
				return err
			}
		}
		return os.Remove(path)
	}
}

// depsForTransaction returns a COPY of b.deps (identity.Deps is a value
// type, so this re-binds one seam rather than building a second wiring —
// review R3-01/plan 05-07's "do not build a second Deps value inside the
// commit path" rule protects against a second WIRING, not against re-binding
// one seam) with ArchiveKeyPair rebound to a working archive seam whose
// onCreated observer is j.recordCreatedFile. Every archive copy this seam
// creates is announced to j BEFORE any source removal is attempted
// (keygen.MoveKeyPairToArchive's own ordering guarantee), so a rollback can
// always discover and undo it. Every transactional caller (Rotate/RepairKey
// callers) must pass depsForTransaction(j) — never b.deps, whose
// ArchiveKeyPair binding refuses by design.
func (b *realBackend) depsForTransaction(j *mutationJournal) identity.Deps {
	deps := b.deps
	deps.ArchiveKeyPair = b.archiveKeyPairSeam(j.recordCreatedFile)
	return deps
}

// deleteSSHConfigMode / deleteGitconfigMode are the modes buildDeleteDeps
// writes raw config bytes at — matching internal/sshconfig's unexported
// configMode (0o600) and internal/gitconfig's unexported gitconfigMode
// (0o644) respectively, since neither is exported for reuse here.
const (
	deleteSSHConfigMode os.FileMode = 0o600
	deleteGitconfigMode os.FileMode = 0o644
)

// buildDeleteDeps wires identity.DeleteDeps from the real internal packages —
// EVERY field filled, mirroring buildIdentityDeps' non-nil contract
// (wiring_test.go's reflection guard covers this struct too).
//
// Under DeleteScopeGitOnly (the only scope this plan's runDelete reaches),
// ReadSSH/WriteSSH are wired but never invoked by identity.Delete — the SSH
// branch is structurally skipped at the domain layer (D-10, review R-13).
// RemoveAllowedSigners/RemoveKeyFiles are likewise wired but unreachable
// under git-only for the same reason; they are filled here so the everything
// scope plan 05-04 lands can use this SAME deps struct without any rewiring.
func buildDeleteDeps(b *realBackend) identity.DeleteDeps {
	return identity.DeleteDeps{
		ReadSSH: func() ([]byte, error) {
			content, err := os.ReadFile(b.storageTargetPath()) //nolint:gosec // trusted gitid-managed path
			if err != nil && !os.IsNotExist(err) {
				return nil, fmt.Errorf("gitid: reading ssh config: %w", err)
			}
			return content, nil
		},
		ReadGitconfig: func() ([]byte, error) {
			content, err := os.ReadFile(b.gitconfigPath) //nolint:gosec // trusted gitid-managed path
			if err != nil && !os.IsNotExist(err) {
				return nil, fmt.Errorf("gitid: reading gitconfig: %w", err)
			}
			return content, nil
		},
		WriteSSH: func(content []byte) (string, error) {
			return filewriter.Write(b.storageTargetPath(), content, deleteSSHConfigMode)
		},
		WriteGitconfig: func(content []byte) (string, error) {
			return filewriter.Write(b.gitconfigPath, content, deleteGitconfigMode)
		},
		RemoveFragment: func(fragPath string) (string, error) {
			if fragPath == "" {
				return "", nil
			}
			return filewriter.BackupAndRemove(fragPath)
		},
		RemoveAllowedSigners: gitconfig.RemoveAllowedSignersBlock,
		// RemoveKeyFiles is the LIVE-key removal seam Delete calls LAST under
		// DeleteScopeEverything, AFTER CopyKeyPairToArchive has already
		// landed the recoverable copy (D-11) — so this is a plain remove,
		// not a second filewriter.BackupAndRemove timestamped backup: the
		// archive copy already IS the backup. Tolerates an already-absent
		// file (no error).
		RemoveKeyFiles: func(keyPath, pubPath string) (string, string, error) {
			if keyPath != "" {
				if err := os.Remove(keyPath); err != nil && !os.IsNotExist(err) {
					return "", "", fmt.Errorf("gitid: removing private key: %w", err)
				}
			}
			if pubPath != "" {
				if err := os.Remove(pubPath); err != nil && !os.IsNotExist(err) {
					return "", "", fmt.Errorf("gitid: removing public key: %w", err)
				}
			}
			return "", "", nil
		},
		// Accounts is the SAME reconstruction the identity list/CommitDelete/
		// CLI delete already read — never a second, divergent lookup — but
		// NORMALIZED (b.normalizedAccounts(), not the bare b.accounts()) so
		// every entry's KeyPath is comparable, via plain string equality,
		// against acct.KeyPath below (which normalizeAccountForWrite has
		// ALSO already expanded to an absolute path). Delete's own
		// SharedKeyOwners gate compares these two sources directly; a
		// mismatched tilde-vs-absolute pairing here would make it silently
		// report a genuinely shared key as unshared for any recipe-shaped
		// (tilde-path) identity.
		Accounts: func() ([]identity.Account, error) { return b.normalizedAccounts(), nil },
		// ForeignProviderRefs walks the real ~/.ssh/config for the D-09
		// hand-written-alias half of the reference count.
		ForeignProviderRefs: func(providerKey string) (int, error) {
			return countForeignProviderRefs(b, providerKey)
		},
		RemoveProviderRewrite: func(providerKey string) (string, error) {
			return gitconfig.RemoveProviderRewrite(b.gitconfigPath, providerKey)
		},
		// CopyKeyPairToArchive is the BACKEND-WIDE binding (review R3-01,
		// delete's half — mirrors ArchiveKeyPair's precedent exactly): it
		// REFUSES to archive, because buildDeleteDeps runs with no
		// transaction in scope. deleteDepsForTransaction is the ONLY way to
		// obtain a working archive-copy seam.
		CopyKeyPairToArchive: b.copyKeyPairSeam(func(string) error { return errArchiveOutsideTransaction }),
	}
}

// copyKeyPairSeam is the ONE archive-COPY implementation
// identity.DeleteDeps' CopyKeyPairToArchive field can be bound to — only its
// onCreated OBSERVER varies between the backend-wide refusing binding
// (buildDeleteDeps, above) and a transaction-bound working seam
// (deleteDepsForTransaction, below). It mirrors archiveKeyPairSeam's shape
// exactly but calls keygen.CopyKeyPairToArchive (NEVER touches sources —
// delete-everything's primitive, review R-03) with the same monotonic
// stamp-collision retry.
func (b *realBackend) copyKeyPairSeam(onCreated keygen.CreatedFunc) func(privPath, pubPath string) (string, string, error) {
	return func(privPath, pubPath string) (string, string, error) {
		archiveDir := sshconfig.ArchiveDir(b.sshDir)
		stampNanos := b.archiveClock().UnixNano()
		pair, err := keygen.CopyKeyPairToArchive(archiveDir, privPath, pubPath, strconv.FormatInt(stampNanos, 10), onCreated)
		if err != nil && errors.Is(err, os.ErrExist) {
			// Same-nanosecond stamp collision (review R2-05/R-21's
			// composition-root retry, mirrored from archiveKeyPairSeam).
			stampNanos++
			pair, err = keygen.CopyKeyPairToArchive(archiveDir, privPath, pubPath, strconv.FormatInt(stampNanos, 10), onCreated)
		}
		return pair.PrivatePath, pair.PublicPath, err
	}
}

// deleteDepsForTransaction returns a COPY of buildDeleteDeps(b) (review
// R3-01, delete's half — mirrors depsForTransaction's identity.Deps
// precedent) with CopyKeyPairToArchive rebound to a working archive seam
// whose onCreated observer is j.recordCreatedFile. Every archive copy this
// seam creates is announced to j BEFORE the live key files are ever removed
// (deleteEverything's own ordering guarantee), so a rollback can always
// discover and undo it. runDelete is the production caller — never
// buildDeleteDeps(b) directly for the everything scope, whose
// CopyKeyPairToArchive binding refuses by design.
func (b *realBackend) deleteDepsForTransaction(j *mutationJournal) identity.DeleteDeps {
	deps := buildDeleteDeps(b)
	deps.CopyKeyPairToArchive = b.copyKeyPairSeam(j.recordCreatedFile)
	return deps
}

// countForeignProviderRefs counts every Host stanza OUTSIDE any gitid-managed
// block whose resolved provider key (identity.ProviderKeyForHost) equals
// providerKey — the D-09 hand-written-alias half of the reference count.
// "Foreign" means literally outside every managed block: every managed
// block's raw text is stripped first (filewriter.RemoveBlock, once per
// block name filewriter.ListBlocks reports) before parsing, so a gitid-
// managed identity's OWN Host stanza is never double-counted here —
// ProviderRefCount already counts every managed account. The Include-aware
// merged bytes are used (identity.InventoryDepsForHome), so a fresh D-06
// machine's config.d-only layout is covered too.
func countForeignProviderRefs(b *realBackend, providerKey string) (int, error) {
	content, err := identity.InventoryDepsForHome(b.home).ReadSSHConfig()
	if err != nil {
		return 0, fmt.Errorf("gitid: reading ssh config for foreign provider refs: %w", err)
	}
	foreign := content
	for _, blk := range filewriter.ListBlocks(content) {
		foreign = filewriter.RemoveBlock(foreign, blk.Name)
	}
	count := 0
	for _, stanza := range sshconfig.AllHostStanzas(foreign) {
		if identity.ProviderKeyForHost(stanza.Alias, stanza.Hostname, "") == providerKey {
			count++
		}
	}
	return count, nil
}

// ---------------------------------------------------------------------------
// Backend: data
// ---------------------------------------------------------------------------

// InitialState reads the user's ACTUAL configuration: every reconstructed
// identity plus the detected STORE-01 storage layout. The per-identity glyph
// (row.State, via collapseState) stays sourced from
// identity.BuildInventory's IdentityHealth exactly as before (MGR-07) — it
// never reads state.Findings. Findings themselves are computed ONCE, by
// doctorFindings(b.home) — internal/doctor.Run(deps)'s converged output —
// the SOLE findings source for the Health/Fixer tabs and `gitid health
// --json` (08-01-PLAN.md's binding architecture decision). The retired
// identity.Problem-to-DemoFinding synthesis this replaced produced a SECOND,
// independent findings stream that could disagree with doctor.Run's own
// Coherence/Orphans/etc. checks for the same underlying issue
// (08-RESEARCH.md Pitfall 3) — that failure mode is now structurally
// impossible: there is only one findings computation.
func (b *realBackend) InitialState() tuikit.DemoState {
	state := tuikit.DemoState{SSHStorage: tuikit.StorageSentinel}
	if b.initErr != nil {
		return state
	}
	if b.storage().includeLayout {
		state.SSHStorage = tuikit.StorageInclude
	}
	healthByName := b.healthByName()
	for _, acct := range b.accounts() {
		row := b.toDemoIdentity(acct)
		if h, ok := healthByName[acct.Name]; ok {
			row.State = string(collapseState(h))
		}
		state.Identities = append(state.Identities, row)
	}
	state.Findings = doctorFindings(b.home)
	return state
}

// DemoBanner raises the D-16 "still demo data" banner for each view that is
// still wired to fixture data. The condition is an explicit enumeration of the
// STILL-UNWIRED views rather than a negated single comparison, so the next
// phase removing its own banner edits one entry instead of restructuring the
// expression.
// Phases that have wired their views and therefore do NOT raise the banner:
//   - TabIdentities (plan 03)
//   - TabGlobalSSH (plan 06-05: both Options and Storage sub-tabs are now live)
//   - TabGlobalGit (plan 07-04: real option states, ceremonies, and probe error render are live)
//   - TabDoctor (09.4-01: merged Health + Fixer; both already rendered
//     doctor.Run(deps)'s converged output as of 08-01-PLAN.md Task 1)
func (b *realBackend) DemoBanner(tuikit.TabID) bool {
	return false
}

// errUnhandledAction is what Persist records when a mutating action has NO
// classification in the real backend's exhaustive switch. Reaching it is the
// bug class 05-RESEARCH.md Pitfall 1 describes — the dummy reducer reporting
// success with no write — made loud instead of silent: the D-16-equivalent
// in-memory behavior is NOT a valid outcome for the real binary. The
// sentinel keeps `errors.Is` checks stable even when a future action wraps
// a different root cause around it.
var errUnhandledAction = errors.New("gitid: unhandled action for the real backend")

// Persist commits one action against the real machine and re-reads the
// configuration, so the list always reflects what is actually on disk rather
// than an optimistic in-memory guess.
//
// The cases enumerate the FULL action union as it exists in store.go today,
// read from that file, never from memory (review R-09-CG: ConfigureGit was
// missed exactly that way before). Every action is classified explicitly:
//
//   - REAL-OWNED — the write has already happened through this backend's
//     corresponding async commit seam (CommitGit/CommitDelete/CommitRotate/
//     CommitNewKey/persistCreate); Persist only re-reads disk. Returning
//     tuikit.Reduce here would mock a write the real machine already did.
//   - DEMO-ONLY — the Phase 6-8 views behind the D-16 banner still need
//     their approved in-memory behavior; delegating to tuikit.Reduce keeps
//     those banner screens' behavior unchanged (review R-09-DEMO). The
//     comment on each names the phase that will make it real.
//   - INVALID — CloneIdentity: the real binary never emits it (D-15 routes
//     clone through the create wizard's own commit path). If it ever
//     arrives that is a bug, so it records a persist error NAMING it — a
//     classified refusal, not an omission, hence deliberately NOT
//     errUnhandledAction.
//   - UNCLASSIFIED (default) — a future action added to the union and to
//     AllActions() but not to this switch records an errUnhandledAction
//     persist error naming its dynamic type, failing the all-actions test
//     loudly instead of silently reducing in memory.
//
// The permissive default branch is GONE because that branch reached the
// DUMMY's reducer from the real backend — indistinguishable from success
// while producing no write at all (05-RESEARCH.md Pitfall 1's defect class).
func (b *realBackend) Persist(state tuikit.DemoState, action tuikit.Action) tuikit.DemoState {
	switch a := action.(type) {
	case tuikit.Reset:
		b.setPersistErr(nil)
		return b.InitialState()
	case tuikit.AddIdentity:
		return b.persistCreate(state, a)
	case tuikit.ConfigureGit:
		// Real-owned: the Git edit already committed through CommitGit (the
		// UI waits for GitCommitMsg before reducing this action, so no
		// optimistic success can mask a failed transaction).
		b.setPersistErr(nil)
		return b.InitialState()
	case tuikit.DeleteIdentity:
		// Real-owned: the write already happened in CommitDelete (the
		// async seam) — Persist must re-read disk here, never fall through
		// to tuikit.Reduce's in-memory guess (RESEARCH.md Pitfall 1).
		b.setPersistErr(nil)
		return b.InitialState()
	case tuikit.NewKey:
		// Real-owned: the repair write already happened through
		// CommitNewKey; this action only refreshes the list.
		b.setPersistErr(nil)
		return b.InitialState()
	case tuikit.RotateIdentity:
		// Real-owned: the rotation write already happened through
		// CommitRotate; this action only refreshes the list.
		b.setPersistErr(nil)
		return b.InitialState()
	case tuikit.EditSSH:
		// Real-owned: the Host-block rewrite ceremony confirmed before this
		// action dispatched; re-read disk (the write-path ownership details
		// belong to the ceremony's commit seam).
		b.setPersistErr(nil)
		return b.InitialState()
	case tuikit.MarkScanned:
		// Demo-only: the Phase 8 doctor scan does not write in-disk config;
		// keep the approved in-memory reducer behavior behind the D-16 banner.
		return tuikit.Reduce(state, action)
	case tuikit.FixFinding:
		return b.persistFixFinding(a)
	case tuikit.ApplySSH:
		// Real-owned: the write already happened in CommitGlobalSSH (the async
		// seam — which mirrors CommitDelete); Persist must re-read disk here,
		// never fall through to tuikit.Reduce's in-memory guess. There is NO
		// persistApplySSH function in this design: if execution finds itself
		// writing one, that is the retired architecture reappearing.
		b.setPersistErr(nil)
		return b.InitialState()
	case tuikit.SetSSHStorage:
		// Real-owned (plan 06-05): the migration write already happened in
		// CommitSSHStorage (the async seam). Persist must re-read disk here,
		// never fall through to tuikit.Reduce's in-memory guess. There is no
		// persistSetSSHStorage function in this design.
		b.setPersistErr(nil)
		return b.InitialState()
	case tuikit.ApplyGitBaseline:
		// Demo-only: Phase 7 will make the global-git baseline real.
		return tuikit.Reduce(state, action)
	case tuikit.ApplyGitGlobalEmail:
		// Demo-only: Phase 7 owns the global user.email ceremony.
		return tuikit.Reduce(state, action)
	case tuikit.CloneIdentity:
		// Invalid for the real backend: D-15 routes clone through the
		// create wizard's own commit path, so the real binary never emits
		// this action. Arriving here is a bug — refuse loudly, with the
		// action named (a classified refusal, intentionally NOT
		// errUnhandledAction so it cannot be mistaken for an omission).
		b.setPersistErr(fmt.Errorf("gitid: %T is not valid for the real backend (D-15 routes clone through the create wizard)", action))
		return state
	default:
		b.setPersistErr(fmt.Errorf("%w: %T", errUnhandledAction, action))
		return state
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

// ---------------------------------------------------------------------------
// Upload / Credentials Assist (Phase 9, UP-02/UP-03)
// ---------------------------------------------------------------------------

// providerDisplayName maps a provider key ("github"/"gitlab") to its
// checkbox-label display form ("GitHub"/"GitLab"). uploader never carries
// display strings itself (it deals in tool/provider KEYS), so the mapping
// lives at this one conversion site alongside the rest of the DTO
// conversions (views.go:13-20's "one place" rule).
func providerDisplayName(provider string) string {
	switch provider {
	case "github":
		return "GitHub"
	case "gitlab":
		return "GitLab"
	default:
		return provider
	}
}

// providerToolName maps a provider key to its CLI tool name.
func providerToolName(provider string) string {
	switch provider {
	case "github":
		return "gh"
	case "gitlab":
		return "glab"
	default:
		return ""
	}
}

// UploadEligibility resolves whether autonomous key upload can run for
// hostname — ASYNCHRONOUSLY (R3): the returned tea.Cmd runs the LookPath +
// "auth status" probe in Bubble Tea's own goroutine, never on the render
// path. A hostname whose provider is not one of D-13's gated main domains
// answers Omitted with NO subprocess call at all — ProviderForHostname
// alone decides that, and it is pure.
//
// The probe is MEMOIZED per PROVIDER KEY (not per host): two hosts that
// share one provider key (github.com and ssh.github.com) share one cache
// entry, because the probe answers a question about the TOOL, not the host.
// This is what keeps a wizard session from spawning one "gh auth status"
// per host edit.
//
// AuthCheck is always called with ProviderForHostname's SECOND return value
// (the canonical host), never the raw hostname parameter (R18) — gh/glab
// track authentication per canonical web domain, not per SSH endpoint, and
// this project's alt-SSH recipe (ssh.github.com, port 443) makes that
// divergence the common case for real identities.
func (b *realBackend) UploadEligibility(hostname string) tea.Cmd {
	return func() tea.Msg {
		provider, canonicalHost := uploader.ProviderForHostname(hostname)
		if provider == "" {
			return tuikit.UploadEligibilityMsg{Hostname: hostname, View: tuikit.UploadEligibilityView{
				State: tuikit.UploadEligibilityOmitted,
			}}
		}

		// WR-17: the per-provider lock — NOT uploadEligibilityMemoMu — is
		// what serializes concurrent same-provider callers across the real
		// probe. It is held for this whole function's remainder, but it is
		// scoped to provider ALONE, so an unrelated provider's probe never
		// waits behind it.
		providerLock := b.eligibilityLockFor(provider)
		providerLock.Lock()
		defer providerLock.Unlock()

		if cached, ok := b.eligibilityMemoGet(provider); ok {
			return tuikit.UploadEligibilityMsg{Hostname: hostname, View: cached}
		}

		view := tuikit.UploadEligibilityView{
			ProviderName: providerDisplayName(provider),
			ToolName:     providerToolName(provider),
			Hostname:     canonicalHost,
		}
		_, toolPath, found := uploader.DetectFor(provider, b.uploaderDeps)
		if !found {
			view.State = tuikit.UploadEligibilityDisabled
		} else if uploader.AuthCheck(toolPath, b.uploaderDeps, canonicalHost) == uploader.AuthAuthenticated {
			view.State = tuikit.UploadEligibilityReady
		} else {
			view.State = tuikit.UploadEligibilityUnauth
		}

		b.eligibilityMemoSet(provider, view)
		return tuikit.UploadEligibilityMsg{Hostname: hostname, View: view}
	}
}

// eligibilityLockFor returns provider's dedicated mutex, creating it under
// uploadEligibilityMemoMu if this is the first call for that provider. The
// outer mutex is held only for this brief map access, never across the
// caller's actual probe.
func (b *realBackend) eligibilityLockFor(provider string) *sync.Mutex {
	b.uploadEligibilityMemoMu.Lock()
	defer b.uploadEligibilityMemoMu.Unlock()
	if b.uploadEligibilityLocks == nil {
		b.uploadEligibilityLocks = make(map[string]*sync.Mutex)
	}
	lock, ok := b.uploadEligibilityLocks[provider]
	if !ok {
		lock = &sync.Mutex{}
		b.uploadEligibilityLocks[provider] = lock
	}
	return lock
}

// eligibilityMemoGet and eligibilityMemoSet are the memo map's only
// accessors — each holds uploadEligibilityMemoMu only for the duration of
// the map operation itself, never across a subprocess call. Callers must
// already hold provider's own eligibilityLockFor lock to keep the overall
// check-then-set sequence atomic per provider.
func (b *realBackend) eligibilityMemoGet(provider string) (tuikit.UploadEligibilityView, bool) {
	b.uploadEligibilityMemoMu.Lock()
	defer b.uploadEligibilityMemoMu.Unlock()
	if b.uploadEligibilityMemo == nil {
		return tuikit.UploadEligibilityView{}, false
	}
	view, ok := b.uploadEligibilityMemo[provider]
	return view, ok
}

func (b *realBackend) eligibilityMemoSet(provider string, view tuikit.UploadEligibilityView) {
	b.uploadEligibilityMemoMu.Lock()
	defer b.uploadEligibilityMemoMu.Unlock()
	if b.uploadEligibilityMemo == nil {
		b.uploadEligibilityMemo = make(map[string]tuikit.UploadEligibilityView)
	}
	b.uploadEligibilityMemo[provider] = view
}

// shortHostname returns the local machine's hostname truncated at the first
// "." (D-07's machine-scoped key title; hostname normalization is
// explicitly Claude's Discretion), falling back to "unknown-host" when
// os.Hostname fails.
func shortHostname() string {
	h, err := os.Hostname()
	if err != nil || h == "" {
		return "unknown-host"
	}
	if idx := strings.Index(h, "."); idx >= 0 {
		return h[:idx]
	}
	return h
}

// RunUpload dispatches the confirmed autonomous upload beat. The existing
// stage-1/stage-2 gate performs D-17's ssh -T verification immediately after
// this command auto-advances the wizard, so no duplicate probe belongs here.
func toUploadResultRow(tool uploader.Tool, result uploader.RegistrationResult, providerHost, home string) tuikit.UploadResultRow {
	row := tuikit.UploadResultRow{Command: result.Command}
	switch result.Registration {
	case uploader.RegistrationAuthentication:
		row.Registration, row.Label = tuikit.UploadRegistrationAuthentication, tuikit.UploadRegistrationLabelAuth
	case uploader.RegistrationSigning:
		row.Registration, row.Label = tuikit.UploadRegistrationSigning, tuikit.UploadRegistrationLabelSigning
	case uploader.RegistrationCombined:
		row.Registration, row.Label = tuikit.UploadRegistrationCombined, tuikit.UploadRegistrationLabelCombined
	}
	switch result.Outcome {
	case uploader.OutcomeUploaded:
		if uploader.ClassifyGHDuplicate(result) {
			row.Outcome = tuikit.UploadRowAlreadyPresent
		} else {
			row.Outcome = tuikit.UploadRowUploaded
		}
	case uploader.OutcomeAlreadyPresent:
		row.Outcome = tuikit.UploadRowAlreadyPresent
	default:
		row.Outcome = tuikit.UploadRowFailed
		switch uploader.ClassifyUploadFailure(tool, result.Registration, result) {
		case uploader.FailureScopeAuth:
			row.Reason = fmt.Sprintf(tuikit.UploadScopeRemediationAuthFmt, providerHost)
		case uploader.FailureScopeSigning:
			row.Reason = fmt.Sprintf(tuikit.UploadScopeRemediationSigningFmt, providerHost)
		case uploader.FailureCrossAccountConflict:
			row.Reason = tuikit.UploadCrossAccountConflict
		case uploader.FailureNotAuthenticated:
			// WR-03: previously fell through to the raw-CLI-output default
			// below, showing the user a truncated CLI line instead of the
			// "run gh auth login" guidance the D-01 scenario-2 "check
			// anyway" path was designed to give.
			row.Reason = fmt.Sprintf(tuikit.UploadNotAuthenticatedFmt, uploader.ToolName(tool), providerHost)
		default:
			raw := result.Output
			if raw == "" && result.Err != nil {
				raw = result.Err.Error()
			}
			row.Reason = uploader.RedactCLIOutput(raw, home, 58)
		}
	}
	return row
}

func uploadFailureView(reason, provider string) tuikit.UploadRunView {
	return tuikit.UploadRunView{Rows: []tuikit.UploadResultRow{{
		Registration: tuikit.UploadRegistrationAuthentication,
		Label:        tuikit.UploadRegistrationLabelAuth,
		Outcome:      tuikit.UploadRowFailed,
		Reason:       reason,
	}}, ManualFallback: upload.Instructions(provider)}
}

func desiredRegistrations(tool uploader.Tool) []uploader.Registration {
	if tool == uploader.ToolGLab {
		return []uploader.Registration{uploader.RegistrationCombined}
	}
	return []uploader.Registration{uploader.RegistrationAuthentication, uploader.RegistrationSigning}
}

// RunUpload composes upload_run.go's planUpload/executeUpload with the
// observable announce-before-run gap R7 requires: planUpload's read-only
// decision steps (including the pre-upload inventory read) run first and
// their commands are rendered via UploadStartedMsg BEFORE executeUpload's
// FollowUp actually runs anything — a terminal message could only ever
// arrive after the subprocess returned, so the gap must live in HOW this
// method sequences the two functions, not inside either one. The CLI's
// runUploadFor (upload_run.go) calls the SAME two functions back-to-back
// with no gap, since a one-shot process has no live rendering loop to show
// an intermediate state to. There is exactly one implementation of the
// decision logic (upload_run.go); this method contains none of it.
func (b *realBackend) RunUpload(spec tuikit.CreateSpec) tea.Cmd {
	return func() (msg tea.Msg) {
		defer func() {
			if recovered := recover(); recovered != nil {
				// Expected provider errors become ordinary result rows above. This
				// branch keeps a gitid programming defect visible while preserving
				// the never-gates guarantee; laundering it as a provider rejection
				// would hide the defect the recover is intended to surface.
				msg = tuikit.UploadRunMsg{Name: spec.Identity, View: uploadFailureView("gitid internal defect: "+uploader.RedactCLIOutput(fmt.Sprint(recovered), b.home, 58), spec.Hostname)}
			}
		}()
		req, err := b.uploadRequestFromSpec(spec)
		if err != nil {
			// WR-05: uploadRequestFromSpec's staging errors embed the
			// staging temp dir's absolute path — redact exactly like the
			// classified-CLI-output and recovered-panic paths already do.
			return tuikit.UploadRunMsg{Name: spec.Identity, View: uploadFailureView(uploader.RedactCLIOutput(err.Error(), b.home, 58), spec.Hostname)}
		}
		plan, terminal := b.planUpload(req)
		if terminal != nil {
			return tuikit.UploadRunMsg{Name: spec.Identity, View: *terminal}
		}
		return tuikit.UploadStartedMsg{Commands: plan.commands, FollowUp: func() tea.Msg {
			return tuikit.UploadRunMsg{Name: spec.Identity, View: b.executeUpload(spec.Hostname, plan)}
		}}
	}
}

// RegisterKeyPlan resolves the D-08 register-key pane's eligibility answer
// for an EXISTING identity, looked up by name from the same b.accounts()
// list every other identity seam reads (findAccount) — the async logic
// otherwise mirrors UploadEligibility exactly (R3, R18), just keyed by
// identity name instead of a live form hostname, so there is no separate
// per-provider memo here: a register-key probe is a one-shot pane open, not
// a per-keystroke wizard re-check.
func (b *realBackend) RegisterKeyPlan(name string) tea.Cmd {
	return func() tea.Msg {
		acct, ok := b.findAccount(name)
		if !ok {
			return tuikit.RegisterKeyPlanMsg{Name: name, Err: fmt.Errorf("identity %q not found", name)}
		}
		provider, canonicalHost := uploader.ProviderForHostname(acct.Hostname)
		if provider == "" {
			return tuikit.RegisterKeyPlanMsg{Name: name, View: tuikit.UploadEligibilityView{
				State: tuikit.UploadEligibilityOmitted,
			}}
		}
		view := tuikit.UploadEligibilityView{
			ProviderName: providerDisplayName(provider),
			ToolName:     providerToolName(provider),
			Hostname:     canonicalHost,
		}
		_, toolPath, found := uploader.DetectFor(provider, b.uploaderDeps)
		if !found {
			view.State = tuikit.UploadEligibilityDisabled
		} else if uploader.AuthCheck(toolPath, b.uploaderDeps, canonicalHost) == uploader.AuthAuthenticated {
			view.State = tuikit.UploadEligibilityReady
		} else {
			view.State = tuikit.UploadEligibilityUnauth
		}
		return tuikit.RegisterKeyPlanMsg{Name: name, View: view}
	}
}

// RunUploadForIdentity dispatches the D-08 pane's confirmed upload beat for
// an EXISTING identity's own already-on-disk key. It composes the SAME
// upload_run.go orchestration the CLI's register-key verb uses
// (uploadRequestForAccount + runUploadFor) — there is exactly one
// implementation of the upload decision logic, and this method contains
// none of it, mirroring RunUpload's own doc comment.
func (b *realBackend) RunUploadForIdentity(name string) tea.Cmd {
	return func() (msg tea.Msg) {
		acct, ok := b.findAccount(name)
		if !ok {
			return tuikit.UploadRunMsg{Name: name, View: uploadFailureView(fmt.Sprintf("identity %q not found", name), "")}
		}
		defer func() {
			if recovered := recover(); recovered != nil {
				msg = tuikit.UploadRunMsg{Name: name, View: uploadFailureView("gitid internal defect: "+uploader.RedactCLIOutput(fmt.Sprint(recovered), b.home, 58), acct.Hostname)}
			}
		}()
		return tuikit.UploadRunMsg{Name: name, View: b.runUploadFor(uploadRequestForAccount(acct, b.home))}
	}
}

// RotateDeleteOffer resolves D-04's interactive old-key delete offer for an
// EXISTING identity via a FRESH provider inventory read — never on the
// render path (R3), and never cached across calls: D-04's Open Question 2 is
// resolved lazily, at result-screen time, because an eager resolution during
// KeyCeremonyPlan would cache an ID captured before the user even confirmed
// the rotate. Matching is by EXACT title equality against THIS machine's
// title (uploader.KeyTitle + uploader.FindByTitle) — never a substring or a
// name-only match — because D-07's whole point is that a rotate on one
// machine must never offer to delete a DIFFERENT machine's still-in-use key.
// Every failure path (unknown identity, non-qualifying provider, tool
// absent/unauthenticated, inventory error, no match) returns an
// Available=false view with a reason; it never returns an error the caller
// must special-case.
func (b *realBackend) RotateDeleteOffer(name string) tea.Cmd {
	return func() tea.Msg {
		return tuikit.RotateDeleteOfferMsg{Name: name, View: b.rotateDeleteOfferFor(name)}
	}
}

// CommitRotateDeleteOldKey is the ONE remotely-destructive call this phase
// makes — reachable only from a confirmed choice on the D-04 offer, never
// autonomously. keyID is the opaque candidate set upload_run.go's
// rotateDeleteOfferFor encoded and the user reviewed (via KeyDetail); it is
// decoded back into its {ID, Registration} pairs here and NOT re-resolved
// from a fresh inventory read, because re-resolving after confirmation would
// let a provider-side change between display and confirm redirect the
// deletion to a different key (review R12). Every candidate is deleted
// through its OWN registration-scoped endpoint (CR-02: GitHub's
// authentication and signing key IDs are independent namespaces) and success
// is claimed only once EVERY candidate is gone — a rotated GitHub key
// legitimately carries both an authentication and a signing registration, so
// stopping after the first success would leave the old key still able to
// authenticate or sign while gitid reports it fully removed.
func (b *realBackend) CommitRotateDeleteOldKey(name, keyID string) tea.Cmd {
	return func() tea.Msg {
		acct, ok := b.findAccount(name)
		if !ok {
			return tuikit.RotateDeleteCommitMsg{Name: name, Err: fmt.Sprintf("identity %q not found", name), RemainingKeyID: keyID}
		}
		provider, _ := uploader.ProviderForHostname(acct.Hostname)
		if provider == "" {
			return tuikit.RotateDeleteCommitMsg{Name: name, Err: "provider not eligible for autonomous key management", RemainingKeyID: keyID}
		}
		tool, toolPath, found := uploader.DetectFor(provider, b.uploaderDeps)
		if !found {
			return tuikit.RotateDeleteCommitMsg{Name: name, Err: fmt.Sprintf("%s CLI not found on PATH", providerToolName(provider)), RemainingKeyID: keyID}
		}
		candidates, derr := decodeDeleteCandidates(keyID)
		if derr != nil {
			return tuikit.RotateDeleteCommitMsg{Name: name, Err: "could not resolve the confirmed delete target", RemainingKeyID: keyID}
		}
		// WR-09 (review iteration 3): track which candidates did NOT delete
		// successfully separately from the human-readable error text, so a
		// partial failure's retry target can be narrowed to just those --
		// re-sending an already-deleted candidate gets a permanent 404 from
		// the provider, making the retry fail forever even once the goal
		// state (every candidate gone) is genuinely reached.
		var failed []string
		var remaining []uploader.ExistingKey
		for _, c := range candidates {
			// WR-14: DeleteRecordedKey (not the lower-level DeleteKey) is the
			// preferred entry point — it exists specifically to carry a
			// record's ID and Registration together rather than as two loose
			// arguments a caller could mismatch.
			rec := uploader.ExistingKey{ID: c.ID, Registration: c.Registration}
			if _, err := uploader.DeleteRecordedKey(tool, toolPath, rec, b.uploaderDeps); err != nil {
				failed = append(failed, uploader.RedactCLIOutput(err.Error(), b.home, 58))
				remaining = append(remaining, rec)
			}
		}
		if len(failed) > 0 {
			remainingKeyID, eerr := encodeDeleteCandidates(remaining)
			if eerr != nil {
				// Encoding failure must never silently widen the retry back
				// to the full original set -- fall back to it explicitly
				// (the R12-safe default) rather than leaving RemainingKeyID
				// empty, which decodeDeleteCandidates on the next attempt
				// would reject as "empty delete-candidate set".
				remainingKeyID = keyID
			}
			return tuikit.RotateDeleteCommitMsg{Name: name, Err: strings.Join(failed, "; "), RemainingKeyID: remainingKeyID}
		}
		return tuikit.RotateDeleteCommitMsg{Name: name}
	}
}

// UploadInstructions returns the manual-fallback text, byte-identical to
// internal/upload.Instructions(provider) — this method exists so
// internal/tuikit never imports internal/upload directly (09-UI-SPEC.md's
// byte-identical-reuse requirement, and views.go's no-backend-import rule).
func (b *realBackend) UploadInstructions(provider string) string {
	return upload.Instructions(provider)
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

// GitWritePlan is CreateWritePlan's sibling for the standalone Configure-Git
// ceremony (CR-12). Before this seam existed, gitCeremonyFor built its
// disclosure from hardcoded strings: it named ~/.gitconfig.backup.<ISO> and
// ~/.ssh/allowed_signers.backup.<ISO> — paths NewBackupPath mints but
// filewriter never does (filewriter always mints <file>.bak.<unix-nanos>) —
// declared exactly 2 backups when commitGitArtifacts can take up to 4, and
// never mentioned that it creates ~/.gitconfig.d/, ~/git/<identity>/, or
// ~/.ssh from scratch. This mirrors commitGitArtifacts' own write order and
// existence checks exactly, using the SAME backupSuffixPreview convention
// CreateWritePlan already uses (the real receipt stamps the actual
// UnixNano at write time; this preview only shows the shape).
func (b *realBackend) GitWritePlan(spec tuikit.GitSpec) tuikit.WritePlanView {
	if b.initErr != nil {
		return tuikit.WritePlanView{}
	}
	if spec.Provider == "" || !strings.Contains(spec.Provider, ".") {
		spec.Provider = providerFromAlias(spec.SSHHost)
	}
	fragmentPath := filepath.Join(b.fragmentDir, spec.Identity)
	gitDirPath := ""
	if spec.Strategy != "hasconfig" {
		gitDirPath = b.resolveKeyPath(strings.TrimSpace(spec.GitDir))
		if gitDirPath == "" {
			gitDirPath = filepath.Join(b.home, "git", spec.Identity)
		}
	}

	plan := tuikit.WritePlanView{
		Targets: []string{
			b.displayPath(fragmentPath),
			b.displayPath(b.gitconfigPath),
			b.displayPath(b.allowedSigners),
		},
	}

	// Backups, in commitGitArtifacts' exact write order:
	//  1. the fragment — backed up only if it already exists (edit, not
	//     create).
	//  2. ~/.gitconfig via gitconfig.WriteIncludeIf — backed up only if it
	//     already exists BEFORE this transaction.
	//  3. ~/.gitconfig AGAIN via gitconfig.WriteProviderRewrite, when
	//     ForceSSH is on — by the time this second write runs, step 2 has
	//     already created the file if it did not exist, so this backup is
	//     ALWAYS taken when ForceSSH+Provider are set, independent of the
	//     file's original state. This is the real, easily-missed reason a
	//     single transaction can back up ~/.gitconfig twice.
	//  4. ~/.ssh/allowed_signers — backed up only if it already exists.
	if fileExists(fragmentPath) {
		plan.Backups = append(plan.Backups, b.displayPath(fragmentPath)+backupSuffixPreview)
	}
	if fileExists(b.gitconfigPath) {
		plan.Backups = append(plan.Backups, b.displayPath(b.gitconfigPath)+backupSuffixPreview)
	}
	if spec.ForceSSH && spec.Provider != "" {
		plan.Backups = append(plan.Backups, b.displayPath(b.gitconfigPath)+backupSuffixPreview)
	}
	if fileExists(b.allowedSigners) {
		plan.Backups = append(plan.Backups, b.displayPath(b.allowedSigners)+backupSuffixPreview)
	}

	// CreatedDirs: every managed root commitGitArtifacts creates via
	// ensureManagedDir/ensureDir when absent — b.fragmentDir and b.sshDir
	// always (gitid's own managed roots), gitDirPath only when the match
	// strategy actually uses one ("hasconfig" writes no gitdir root).
	if !fileExists(b.fragmentDir) {
		plan.CreatedDirs = append(plan.CreatedDirs, b.displayPath(b.fragmentDir))
	}
	if gitDirPath != "" && !fileExists(gitDirPath) {
		plan.CreatedDirs = append(plan.CreatedDirs, b.displayPath(gitDirPath))
	}
	if !fileExists(b.sshDir) {
		plan.CreatedDirs = append(plan.CreatedDirs, b.displayPath(b.sshDir))
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

// FixPlanFor is the real Backend.FixPlanFor implementation (08-02-PLAN.md
// Task 2): for a finding carrying a doctor-originated Rewrite descriptor
// (currently the D-09 hand-written IdentitiesOnly contradiction), it reads
// the ACTUAL ~/.ssh/config content and renders the true before/after diff
// via sshconfig.DiffHostDirective — never the frozen fixture text. Every
// other finding falls back to the free tuikit.PlanFor(finding) switch
// unchanged — this wave introduces exactly one new rewrite kind; other
// fixable findings' plans are out of its scope.
func (b *realBackend) FixPlanFor(finding tuikit.DemoFinding) tuikit.FixPlan {
	rw := finding.Rewrite
	if rw == nil {
		return tuikit.PlanFor(finding)
	}
	content, err := os.ReadFile(b.sshConfigPath) //nolint:gosec // b.sshConfigPath is gitid's own resolved, trusted path (G304)
	if err != nil {
		return tuikit.PlanFor(finding)
	}
	diff, derr := sshconfig.DiffHostDirective(content, rw.HostPattern, rw.Directive, rw.NewValue)
	if derr != nil {
		return tuikit.PlanFor(finding)
	}
	return tuikit.FixPlan{
		File: b.displayPath(b.sshConfigPath),
		Diff: diff,
		Destructive: &tuikit.FixDestructive{
			ConfirmWord: rw.HostPattern,
			Warning: fmt.Sprintf(
				"This rewrites a directive already present in your SSH config. Type the Host name %q to confirm — this cannot be undone without restoring the backup.",
				rw.HostPattern),
		},
		Result: fmt.Sprintf("%s set to %s on Host %s in %s.", rw.Directive, rw.NewValue, rw.HostPattern, b.displayPath(b.sshConfigPath)),
	}
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
func (b *realBackend) CommitGit(spec tuikit.GitSpec) tea.Cmd {
	return func() tea.Msg {
		if b.initErr != nil {
			return tuikit.GitCommitMsg{Err: b.displayMessage(b.initErr.Error())}
		}
		backups, restored, err := b.commitGitTransaction(spec)
		// WR-01: the standalone Git ceremony's receipt rendered raw absolute
		// backup paths (wrapping across terminal rows) because this path,
		// unlike commitCreateTransaction's, never mapped through
		// b.displayPath before returning.
		displayBackups := make([]string, len(backups))
		for i, backup := range backups {
			displayBackups[i] = b.displayPath(backup)
		}
		// WR-23: restored's own outcome lines already run their PATH through
		// displayPath (mutationJournal.restore), but each line's trailing
		// error text is not path-scrubbed — a wrapped os/gitconfig error
		// (e.g. `git config --file %s %s: ...`) still embeds the raw
		// absolute sandbox path. Scrub the whole line here, the one place
		// that is about to make it user-facing.
		displayRestored := make([]string, len(restored))
		for i, outcome := range restored {
			displayRestored[i] = b.displayMessage(outcome)
		}
		if err != nil {
			return tuikit.GitCommitMsg{Backups: displayBackups, Restored: displayRestored, Err: b.displayMessage(err.Error())}
		}
		return tuikit.GitCommitMsg{Backups: displayBackups}
	}
}

// commitGitTransaction reads the signing key before delegating every mutation to
// the shared journal. Standalone and combined create writes therefore have the
// same snapshot, backup, rollback, and failure-reporting behavior.
//
// WR-04: txMu serializes this against any concurrent commitCreateTransaction
// (both run off the Bubble Tea update loop, each in its own goroutine, and
// both read-modify-write ~/.gitconfig and ~/.ssh/allowed_signers).
func (b *realBackend) commitGitTransaction(spec tuikit.GitSpec) ([]string, []string, error) {
	b.txMu.Lock()
	defer b.txMu.Unlock()
	return b.commitGitArtifacts(spec, "", nil)
}

// CommitDelete is the delete async seam, mirroring CommitGit's shape exactly:
// resolve the scope, call runDelete — the ONE complete per-verb delete
// ceremony in lifecycle.go, which owns its own txMu locking, same as
// commitGitTransaction — and report the result as a DeleteCommitMsg. The
// delete-choice ceremony screen IS the confirmation, so the lifecycle is
// authorized with confirmationAlreadyObtained (the only layer permitted to
// assert that value). Every backup/restored path is scrubbed through
// b.displayPath/b.displayMessage before it becomes user-facing, the same
// WR-01/WR-23 discipline CommitGit applies.
func (b *realBackend) CommitDelete(name, scope string) tea.Cmd {
	return func() tea.Msg {
		if b.initErr != nil {
			return tuikit.DeleteCommitMsg{Err: b.displayMessage(b.initErr.Error())}
		}
		deleteScope, serr := identity.DeleteScopeFrom(scope)
		if serr != nil {
			return tuikit.DeleteCommitMsg{Err: b.displayMessage(serr.Error())}
		}
		res, err := b.runDelete(name, deleteScope, lifecyclePolicy{Confirm: confirmationAlreadyObtained})
		displayBackups := displayPaths(b, res.Backups)
		displayRestored := displayMessages(b, res.Restored)
		// Removed names what the everything scope took off disk (the D-09
		// provider rewrite and the D-11 key copy target); displayPath leaves
		// the non-path provider label alone and shortens the archive paths.
		displayRemoved := displayPaths(b, res.Removed)
		if err != nil {
			return tuikit.DeleteCommitMsg{
				Backups: displayBackups, Restored: displayRestored, Removed: displayRemoved,
				Err: b.displayMessage(err.Error()),
			}
		}
		return tuikit.DeleteCommitMsg{Backups: displayBackups, Removed: displayRemoved}
	}
}

// runDelete is the ONE production delete writer, defined in lifecycle.go
// (cmd/gitid/lifecycle.go) as the complete per-verb delete ceremony — plan
// 05-01 shipped this function under this exact name so plan 05-07 extends
// one function rather than introducing a differently-named replacement
// (review R3-05). It must never be redefined here: its signature takes a
// lifecyclePolicy and returns a lifecycleResult, and the compiler forces
// every call site to follow. Do NOT coin a second, wave-local name beside
// it.

// CommitGlobalSSH is the global-SSH apply async seam, mirroring CommitDelete
// exactly: resolve nothing cached, call runGlobalSSHApply — the ONE complete
// per-verb global apply ceremony in lifecycle.go, which owns its own txMu
// locking — and report the result as a GlobalSSHCommitMsg. The apply-ceremony
// screen IS the confirmation, so the lifecycle is authorized with
// confirmationAlreadyObtained (the only layer permitted to assert that
// value). Every backup/restored path is scrubbed through b.displayPath /
// b.displayMessage before it becomes user-facing, the same WR-01/WR-23
// discipline CommitGit and CommitDelete apply.
func (b *realBackend) CommitGlobalSSH(keys []string) tea.Cmd {
	return func() tea.Msg {
		if b.initErr != nil {
			return tuikit.GlobalSSHCommitMsg{Err: b.displayMessage(b.initErr.Error())}
		}
		res, err := b.runGlobalSSHApply(keys, lifecyclePolicy{Confirm: confirmationAlreadyObtained})
		msg := tuikit.GlobalSSHCommitMsg{
			Backups:          displayPaths(b, res.Backups),
			Restored:         displayMessages(b, res.Restored),
			ShadowAdvisories: displayMessages(b, res.Advisories),
		}
		if err != nil {
			msg.Err = b.displayMessage(err.Error())
		}
		return msg
	}
}

// GlobalSSHOptionStates is the ONE conversion site from the globalssh engine
// to the render DTO: it runs the D-01 probe set through the real constructor
// and renders the D-03 provenance LABEL here (tuikit must never learn the
// source-class enum — Provenance is a rendered string, per the view boundary).
//
// Statuses never returns an error (a probe failure degrades to a per-option
// advisory note), so this method's only error path is a construction failure;
// the fail-open choice lives in the engine, documented there.
func (b *realBackend) GlobalSSHOptionStates() ([]tuikit.GlobalSSHOptionView, error) {
	if b.initErr != nil {
		return nil, b.initErr
	}
	statuses := globalssh.Statuses(globalssh.BuildProbeDeps(b.sshConfigPath))
	sshVersion := b.readSSHVersion()
	fixture := make(map[string]tuikit.GlobalSSHOption, len(tuikit.GlobalSSHOptions))
	for _, o := range tuikit.GlobalSSHOptions {
		fixture[o.Key] = o
	}
	out := make([]tuikit.GlobalSSHOptionView, 0, len(statuses))
	for _, st := range statuses {
		oneLiner := fixture[st.Key].OneLiner
		explanation := oneLiner
		if st.Key == "IdentitiesOnly" {
			explanation = tuikit.GlobalSSHDetailExplanation
		}
		policy, _ := globalssh.PolicyFor(st.Key)
		view := tuikit.GlobalSSHOptionView{
			Key:                 st.Key,
			CurrentValue:        st.CurrentValue,
			Provenance:          b.globalSSHProvenanceLabel(st),
			Recommended:         st.RecommendedValue,
			Risk:                st.Risk,
			OneLiner:            oneLiner,
			Explanation:         explanation,
			ProbeError:          st.ProbeError,
			State:               toGlobalSSHOptionState(st.State),
			NotApplicableReason: tuikit.GlobalSSHNotApplicableReason(st.NotApplicableReason),
			AttributedToUser:    st.Source == globalssh.SourceGitidParsed,
			WritableToHostStar:  policy.WritableToHostStar(),
		}
		if policy.MinOpenSSH != "" {
			outcome, note := globalssh.VersionGate(sshVersion, policy)
			view.VersionNote = note
			switch outcome {
			case globalssh.VersionTooOld:
				view.State = tuikit.GlobalSSHNotApplicable
				view.NotApplicableReason = tuikit.GlobalSSHReasonVersionTooOld
			case globalssh.VersionUnverified:
				view.State = tuikit.GlobalSSHNotApplicable
				view.NotApplicableReason = tuikit.GlobalSSHReasonVersionUnverified
			}
		}
		out = append(out, view)
	}
	return out, nil
}

func (b *realBackend) readSSHVersion() platform.SSHVersion {
	probe := b.probeSSHVersion
	if probe == nil {
		probe = platform.ProbeSSHVersion
	}
	v, err := probe()
	if err != nil {
		return platform.SSHVersion{}
	}
	return v
}

// GlobalSSHApplyPlan is the global-SSH apply preview scene: the resolved
// targets, the promised backup path, and a diff computed from the target
// file's CURRENT bytes and the EnsureGlobals candidate. The shadow-warning
// and simulation-inconclusive fields are filled by BuildGraph+Simulate.
func (b *realBackend) GlobalSSHApplyPlan(keys []string) (tuikit.GlobalSSHApplyPlanView, error) {
	if b.initErr != nil {
		return tuikit.GlobalSSHApplyPlanView{}, b.initErr
	}
	explicit := make(map[string]string, len(keys))
	for _, k := range keys {
		policy, ok := globalssh.PolicyFor(k)
		if !ok || policy.Scope == "per-alias" {
			return tuikit.GlobalSSHApplyPlanView{}, fmt.Errorf("gitid: option %q cannot be applied to the global Host * block", k)
		}
		explicit[k] = policy.Recommended
	}
	st := b.storage()
	targets := []string{st.targetPath}
	if st.needsIncludeLine {
		targets = append(targets, b.sshConfigPath)
	}
	view := tuikit.GlobalSSHApplyPlanView{}
	for _, p := range targets {
		view.Targets = append(view.Targets, b.displayPath(p))
		// Only files that ALREADY exist get a backup — filewriter backs up
		// nothing when it creates a file for the first time, and promising a
		// backup that will not be taken would be a lie in the ceremony.
		if fileExists(p) {
			view.Backups = append(view.Backups, b.displayPath(p)+backupSuffixPreview)
		}
	}
	existing, err := os.ReadFile(st.targetPath) //nolint:gosec // trusted gitid-managed path (G304)
	if err != nil && !os.IsNotExist(err) {
		return tuikit.GlobalSSHApplyPlanView{}, err
	}
	candidate, err := sshconfig.EnsureGlobals(existing, explicit, platform.CurrentOS())
	if err != nil {
		return tuikit.GlobalSSHApplyPlanView{}, err
	}
	view.Diff = globalsTextDiff(globalsBodyText(existing), globalsBodyText(candidate))

	// D-04 pre-write simulation: build the whole config graph (recursively,
	// cycle-detected) and simulate the candidate against the mirror's entry
	// point — not the candidate file alone, which cannot see directives in
	// the main config (06-REVIEWS.md HIGH). A BuildGraph error becomes
	// SimulationInconclusive; the apply is still permitted (a graph gitid
	// cannot model is not grounds for refusing a backed-up, reversible write).
	graph, buildErr := globalssh.BuildGraph(b.sshConfigPath, st.targetPath, candidate)
	if buildErr != nil {
		view.SimulationInconclusive = true
		view.SimulationNote = globalSSHSimInconclusiveNote
	} else {
		simResult := globalssh.Simulate(globalssh.BuildProbeDeps(b.sshConfigPath), graph, keys)
		if simResult.Inconclusive {
			view.SimulationInconclusive = true
			view.SimulationNote = globalSSHSimInconclusiveNote
		} else {
			for _, f := range simResult.Findings {
				if f.ShadowedByFile != "" {
					view.ShadowWarnings = append(view.ShadowWarnings,
						fmt.Sprintf(globalSSHShadowWarningFmt, f.Key, b.displayPath(f.ShadowedByFile), f.ShadowedByLine))
				} else {
					view.ShadowWarnings = append(view.ShadowWarnings,
						fmt.Sprintf(globalSSHShadowWarnNoFileFmt, f.Key))
				}
			}
		}
	}
	return view, nil
}

// globalSSHShadowWarningFmt is the frozen warning text for a nameable shadow
// source. Registered in gate-copy-freeze; the format string sentinel must
// stay in this package.
const globalSSHShadowWarningFmt = "shadow warning: %s will be shadowed by %s (line %d)"

// globalSSHShadowWarnNoFileFmt is the frozen warning text when the shadowing
// source cannot be named. Registered in gate-copy-freeze.
const globalSSHShadowWarnNoFileFmt = "shadow warning: %s will be shadowed (source unnameable)"

// globalSSHSimInconclusiveNote is the frozen warning when the simulation
// cannot faithfully mirror the config graph. Registered in gate-copy-freeze.
const globalSSHSimInconclusiveNote = "simulation inconclusive — gitid could not fully read your config graph; apply will continue but shadowing cannot be checked"

// ---------------------------------------------------------------------------
// GlobalGitPlanner (plan 07-01)
// ---------------------------------------------------------------------------

// GlobalGitOptionStates is the ONE conversion site from the globalgit engine
// to the render DTO, mirroring GlobalSSHOptionStates: it runs the D-03 probe
// set through the real constructor and renders the provenance LABEL here —
// tuikit must never learn the source-class enum.
//
// globalgit.Statuses itself never returns an error for a probe failure (it
// degrades per-row to a ProbeError-carrying row). THIS method turns that
// per-row degradation into a hard error when the first row carries one
// (below), so the caller sees one uniform "probe failed" error regardless
// of whether the failure originated in Statuses' own construction or in the
// probe it ran — driving globalgit.go's optionsErr advisory path either way.
func (b *realBackend) GlobalGitOptionStates() ([]tuikit.GlobalGitOptionView, error) {
	if b.initErr != nil {
		return nil, fmt.Errorf("git probe failed: %w", b.initErr)
	}
	fixture := make(map[string]tuikit.GlobalGitOption, len(tuikit.GlobalGitOptions))
	for _, o := range tuikit.GlobalGitOptions {
		fixture[o.Key] = o
	}
	probeDeps := globalgit.BuildProbeDeps(b.fragmentDir)
	rows, err := globalgit.Statuses(probeDeps, b.baselineTargetPath())
	if err != nil {
		return nil, fmt.Errorf("git probe failed: %w", err)
	}
	if len(rows) > 0 && rows[0].ProbeError != "" {
		return nil, fmt.Errorf("git probe failed: %s", rows[0].ProbeError)
	}
	// One git-version read per screen activation (D-08, the D-13 precedent:
	// the dynamic version line is non-contractual). The gate outcome itself is
	// computed per row by VersionGate.
	gitVersion, _ := deps.GitVersion()
	out := make([]tuikit.GlobalGitOptionView, 0, len(rows))
	for _, row := range rows {
		policy, _ := globalgit.PolicyFor(row.Key)
		view := tuikit.GlobalGitOptionView{
			Key:                 row.Key,
			CurrentValue:        row.CurrentValue,
			Provenance:          b.globalGitProvenanceLabel(row),
			Recommended:         row.Recommended,
			OneLiner:            fixture[row.Key].OneLiner,
			Explanation:         fixture[row.Key].OneLiner,
			GitDefault:          row.GitDefault,
			ProbeError:          row.ProbeError,
			State:               toGlobalGitOptionState(row.State),
			NotApplicableReason: toGlobalGitNotApplicableReason(row.NotApplicableReason),
			PolicyBacked:        true, // every policy row is backed by the D-08 table
			HasWritableMember:   len(policy.Members) > 0,
			AttributedToUser:    row.Source == globalgit.SourceSetByUser,
			VersionNote:         globalGitVersionNote(policy, gitVersion),
			GateNotMet:          policy.Gate == globalgit.GateHard && globalGitGateOutcome(policy, gitVersion) != globalgit.GateMet,
		}
		// Bundle rows' current cell is the D-09 aggregate ("3 of 8 set, 1
		// differs"), computed from the probes — the detail pane names the
		// differing members underneath.
		if row.BundleTotal > 1 {
			view.CurrentValue = bundleAggregateCell(row)
			view.BundleAggregate = view.CurrentValue
			view.BundlePerKeyNotes = bundlePerKeyNotes(row)
		}
		out = append(out, view)
	}
	return out, nil
}

// globalGitGateOutcome resolves the gate outcome for policy against
// gitVersion, treating an unreadable version the same way globalGitVersionNote
// does: GateUnreadable when the version could not be read, otherwise whatever
// globalgit.VersionGate decides. A row with no MinVersion has no gate at all
// (globalgit.VersionGate itself returns GateMet for that case).
func globalGitGateOutcome(policy globalgit.OptionPolicy, gitVersion string) globalgit.GateOutcome {
	if strings.TrimSpace(gitVersion) == "" {
		if policy.MinVersion == "" {
			return globalgit.GateMet
		}
		return globalgit.GateUnreadable
	}
	outcome, _ := globalgit.VersionGate(gitVersion, policy)
	return outcome
}

// globalGitVersionNote renders the NON-contractual dynamic version line for a
// version-gated row's detail pane (D-08). It must never reach the copy-freeze
// gate — the prefix "Your git:" is versioned by the machine. An unreadable
// version is silent for informational gates (nothing changes when unsupported)
// and names the fallback for the hard gate (the fallback IS written).
func globalGitVersionNote(policy globalgit.OptionPolicy, gitVersion string) string {
	if policy.MinVersion == "" {
		return ""
	}
	if strings.TrimSpace(gitVersion) == "" {
		if policy.Gate == globalgit.GateHard {
			return globalgit.VersionNoteUnreadable
		}
		return ""
	}
	_, note := globalgit.VersionGate(gitVersion, policy)
	return note
}

// bundleAggregateCell renders the D-09 aggregate summary for a bundle row's
// current cell. It carries counts and is therefore DYNAMIC — excluded from the
// copy-freeze gate.
func bundleAggregateCell(row globalgit.OptionRow) string {
	cell := fmt.Sprintf("%d of %d set", row.BundleSet, row.BundleTotal)
	if row.BundleDiffers > 0 {
		cell += fmt.Sprintf(", %d differs", row.BundleDiffers)
	}
	return cell
}

// bundlePerKeyNotes renders the per-key "yours differs — yours wins" notes the
// detail pane shows for a bundle row (D-09). The tail wording is the frozen
// copy; the member key is dynamic.
func bundlePerKeyNotes(row globalgit.OptionRow) []string {
	notes := make([]string, 0, len(row.BundleDiffersKeys))
	for _, key := range row.BundleDiffersKeys {
		notes = append(notes, fmt.Sprintf("%s — your value differs, so yours wins", key))
	}
	return notes
}

// toGlobalGitNotApplicableReason maps the engine's reason to the render DTO by
// value, pinned numeric-for-numeric by the parity test (mirrors globalssh).
func toGlobalGitNotApplicableReason(r globalgit.NotApplicableReason) tuikit.GlobalGitNotApplicableReason {
	switch r {
	case globalgit.ReasonProbeFailed:
		return tuikit.GlobalGitReasonProbeFailed
	default:
		return tuikit.GlobalGitReasonNone
	}
}

// globalGitProvenanceLabel renders the D-03 provenance label from the
// classifier's source class — the render package never learns the enum. The
// five-word vocabulary mirrors Phase 6's registered set on the git side:
// the user's own file, a scope gitid cannot change, unset naming git's
// built-in default, applied by gitid, and set somewhere gitid cannot name.
func (b *realBackend) globalGitProvenanceLabel(row globalgit.OptionRow) string {
	switch row.Source {
	case globalgit.SourceSetByGitid:
		return fmt.Sprintf("set by gitid in %s", b.displayPath(b.baselineTargetPath()))
	case globalgit.SourceSetByUser:
		if row.EffectiveOrigin != "" {
			return fmt.Sprintf("set by you in %s", b.displayPath(row.EffectiveOrigin))
		}
		return "set by you"
	case globalgit.SourceUnchangeable:
		// A named file (e.g. the system config) is still a file — the label
		// names it and states the scope is unchangeable. A non-file origin
		// ("command line", "blob:…") cannot be named at all.
		if row.EffectiveOrigin != "" && !strings.ContainsRune(row.EffectiveOrigin, ' ') {
			return fmt.Sprintf("set in %s — gitid cannot change this", b.displayPath(row.EffectiveOrigin))
		}
		return "set somewhere gitid cannot name"
	default:
		if row.ProbeError != "" {
			return "the probe did not answer — see the advisory note"
		}
		if row.GitDefault != "" {
			return fmt.Sprintf("not set (git's built-in default: %s)", row.GitDefault)
		}
		return "not set"
	}
}

// toGlobalGitOptionState maps the engine's OptionRowState to the render DTO's
// row state. The one-to-one correspondence is structurally pinned by the
// exported constants on both sides.
func toGlobalGitOptionState(s globalgit.OptionRowState) tuikit.GlobalGitOptionState {
	switch s {
	case globalgit.StateAlreadySet:
		return tuikit.GlobalGitAlreadySet
	case globalgit.StateSetButDiffers:
		return tuikit.GlobalGitSetButDiffers
	case globalgit.StateNotApplicable:
		return tuikit.GlobalGitNotApplicable
	default:
		return tuikit.GlobalGitNeedsAction
	}
}

// GlobalGitApplyPlan is the global-git apply preview scene: the resolved
// baseline target, the promised backup paths (main config + baseline file,
// whichever already exist), and a diff of the baseline file's candidate body.
func (b *realBackend) GlobalGitApplyPlan(keys []string) (tuikit.GlobalGitApplyPlanView, error) {
	if b.initErr != nil {
		return tuikit.GlobalGitApplyPlanView{}, b.initErr
	}
	explicit := make(map[string]string, len(keys)*2)
	for _, k := range keys {
		policy, ok := globalgit.PolicyFor(k)
		if !ok {
			return tuikit.GlobalGitApplyPlanView{}, fmt.Errorf("gitid: unknown global git option %q", k)
		}
		gate := b.gitGateOutcome(policy)
		for _, member := range policy.Members {
			explicit[member.Key] = globalgit.WriteValueFor(policy, member.Key, gate)
		}
	}
	target := b.baselineTargetPath()
	view := tuikit.GlobalGitApplyPlanView{}
	for _, p := range []string{b.gitconfigPath, target} {
		view.Targets = append(view.Targets, b.displayPath(p))
		// Only files that ALREADY exist get a backup — filewriter backs up
		// nothing when it creates a file for the first time, and promising a
		// backup that will not be taken would be a lie in the ceremony.
		if fileExists(p) {
			view.Backups = append(view.Backups, b.displayPath(p)+backupSuffixPreview)
		}
	}
	existing, err := os.ReadFile(target) //nolint:gosec // trusted gitid-managed path (G304)
	if err != nil && !os.IsNotExist(err) {
		return tuikit.GlobalGitApplyPlanView{}, err
	}
	candidate, err := gitconfig.EnsureGlobalGit(existing, explicit)
	if err != nil {
		return tuikit.GlobalGitApplyPlanView{}, err
	}
	view.Diff = globalsTextDiff(string(existing), string(candidate))
	return view, nil
}

// CommitGlobalGit is the global-git apply async seam, mirroring CommitGlobalSSH
// exactly: resolve nothing cached, call runGlobalGitApply — the ONE complete
// per-verb global apply ceremony in lifecycle.go, which owns its own txMu
// locking — and report the result as a GlobalGitCommitMsg. The apply-ceremony
// screen IS the confirmation, so the lifecycle is authorized with
// confirmationAlreadyObtained (the only layer permitted to assert that
// value). Every backup/restored path is scrubbed through b.displayPath /
// b.displayMessage before it becomes user-facing.
func (b *realBackend) CommitGlobalGit(keys []string) tea.Cmd {
	return func() tea.Msg {
		if b.initErr != nil {
			return tuikit.GlobalGitCommitMsg{Err: b.displayMessage(b.initErr.Error())}
		}
		res, err := b.runGlobalGitApply(keys, lifecyclePolicy{Confirm: confirmationAlreadyObtained})
		msg := tuikit.GlobalGitCommitMsg{
			Backups:    displayPaths(b, res.Backups),
			Restored:   displayMessages(b, res.Restored),
			Advisories: displayMessages(b, res.Advisories),
		}
		if err != nil {
			msg.Err = b.displayMessage(err.Error())
		}
		return msg
	}
}

func (b *realBackend) gitignorePath() string {
	return filepath.Join(b.home, ".gitignore_global")
}

func (b *realBackend) gitignoreSeed() string {
	return gitconfig.RenderGitignoreBlock(gitconfig.DefaultGitignorePatterns())
}

// gitIgnorePreimageToken hashes both preimages the ApplyPlan read so
// CommitGlobalGitIgnore can detect a TOCTOU change to EITHER file before it
// writes. The two payloads are length-prefixed (not simply concatenated)
// so the hash cannot collide across the boundary between them — bytes
// shifted from the tail of gitignore into the head of baseline would
// otherwise produce the identical digest for two genuinely different pairs
// of file contents, letting a real change slip past the ChangedSincePreview
// guard (09.2-REVIEW.md WR-08).
func gitIgnorePreimageToken(gitignore, baseline []byte) string {
	h := sha256.New()
	var lenBuf [8]byte
	binary.BigEndian.PutUint64(lenBuf[:], uint64(len(gitignore)))
	h.Write(lenBuf[:])
	h.Write(gitignore)
	binary.BigEndian.PutUint64(lenBuf[:], uint64(len(baseline)))
	h.Write(lenBuf[:])
	h.Write(baseline)
	return hex.EncodeToString(h.Sum(nil))
}

// inspectBaselineFragment reads the managed baseline fragment and refuses a
// malformed shape through the SAME translateGitIgnoreErr boundary every
// other gitconfig-error path on this screen uses, naming the baseline
// fragment's own display path — never the gitignore file's — so a
// malformed baseline fragment can never masquerade as a
// ~/.gitignore_global problem (09.2-REVIEW.md CR-01).
func (b *realBackend) inspectBaselineFragment() ([]byte, error) {
	path := b.baselineTargetPath()
	content, err := os.ReadFile(path) //nolint:gosec // trusted gitid-managed path (G304)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if _, ierr := gitconfig.InspectManagedBlockFile(content, "baseline"); ierr != nil {
		return content, translateGitIgnoreErr(b.displayPath(path), ierr)
	}
	return content, nil
}

// gitIgnoreMalformedReasonText maps a gitconfig.ManagedBlockErrorReason to
// the frozen phrase 09.2-UI-SPEC.md names for it, so the UI layer never
// invents new reason wording of its own. An unrecognized reason (should be
// unreachable — every gitconfig.ManagedBlockError sets one of the three
// named values) falls back to the neutral "unspecified" phrase rather than
// silently mislabeling itself as one of the three known categories
// (09.2-REVIEW.md IN-02).
func gitIgnoreMalformedReasonText(reason gitconfig.ManagedBlockErrorReason) string {
	switch reason {
	case gitconfig.ManagedBlockReasonUnclosedMarker:
		return tuikit.GitIgnoreMalformedReasonUnclosedMarker
	case gitconfig.ManagedBlockReasonMismatchedMarker:
		return tuikit.GitIgnoreMalformedReasonMismatchedMarker
	case gitconfig.ManagedBlockReasonDuplicateBlock:
		return tuikit.GitIgnoreMalformedReasonDuplicateBlock
	default:
		return tuikit.GitIgnoreMalformedReasonUnspecified
	}
}

// gitIgnoreTranslatedErr carries the frozen, user-facing message as Error()
// while preserving the original structured gitconfig error via Unwrap, so
// callers that need the machine-readable Line/Reason (logs, tests) can still
// recover it with errors.As — translation must not be a dead end for that
// data (09.2-REVIEW.md IN-01).
type gitIgnoreTranslatedErr struct {
	msg string
	src error
}

func (e *gitIgnoreTranslatedErr) Error() string { return e.msg }
func (e *gitIgnoreTranslatedErr) Unwrap() error { return e.src }

// translateGitIgnoreErr is the ONE place a gitconfig error is rewritten into
// the tuikit/design.go frozen copy the Global Git Ignore screen renders — it
// must be called at EVERY boundary where a gitconfig error reaches this
// screen's State/ApplyPlan/Commit seams, not just the ones a specific bug
// report named, or the raw internal diagnostic text (baseline.go's
// fmt.Errorf strings) leaks right back in on the untranslated paths
// (09.2-REVIEW.md CR-01). displayPath is the HOME-relative path of the file
// InspectManagedBlockFile/InspectGitignoreFile actually inspected — callers
// must pass the path for the SPECIFIC file the error came from (gitignore
// file vs baseline fragment), never a fixed literal. Any other error passes
// through unchanged.
func translateGitIgnoreErr(displayPath string, err error) error {
	if err == nil {
		return nil
	}
	var mbErr *gitconfig.ManagedBlockError
	if errors.As(err, &mbErr) {
		return &gitIgnoreTranslatedErr{
			msg: tuikit.GitIgnoreMalformedFileMessage(displayPath, mbErr.Line, gitIgnoreMalformedReasonText(mbErr.Reason)),
			src: err,
		}
	}
	var sentinelErr *gitconfig.SentinelLineError
	if errors.As(err, &sentinelErr) {
		return &gitIgnoreTranslatedErr{
			msg: tuikit.GitIgnoreSentinelRejectedMessage(sentinelErr.Line),
			src: err,
		}
	}
	return err
}

func (b *realBackend) classifyGitIgnoreWiring(baselineBytes []byte) (tuikit.GlobalGitIgnoreWiring, string) {
	shape, ierr := gitconfig.InspectManagedBlockFile(baselineBytes, "baseline")
	if ierr != nil || !shape.Managed {
		return tuikit.GitIgnoreNoBaselineBlock, ""
	}
	// gitconfig.ParseBlockKeys is the ONE section/indent/first-"=" scanner
	// for a managed block body — this used to be a byte-for-byte duplicate
	// (parseExcludesfileFromBody) that could silently drift from the
	// original on a future fix (quoted values, "#" comments, subsections);
	// exported and reused instead (09.2-REVIEW.md IN-04).
	keys := gitconfig.ParseBlockKeys(shape.Body)
	value := keys["core.excludesfile"]
	if value == "" {
		return tuikit.GitIgnoreKeyUnset, ""
	}
	if expandTildeForHome(value, b.home) == b.gitignorePath() {
		return tuikit.GitIgnoreWiredAtManaged, value
	}
	return tuikit.GitIgnorePointsElsewhere, value
}

// GlobalGitIgnoreState reads ~/.gitignore_global and the baseline fragment
// through InspectManagedBlockFile so a malformed or duplicate block is
// refused before anything is treated as content. Path comparison uses
// expandTildeForHome(value, b.home) — never a raw == and never
// os.UserHomeDir() — because RenderBaselineBlock writes the literal tilde
// form `excludesfile = ~/.gitignore_global`.
func (b *realBackend) GlobalGitIgnoreState() (tuikit.GlobalGitIgnoreView, error) {
	if b.initErr != nil {
		return tuikit.GlobalGitIgnoreView{}, b.initErr
	}
	path := b.gitignorePath()
	seed := b.gitignoreSeed()
	view := tuikit.GlobalGitIgnoreView{
		Path:           b.displayPath(path),
		DefaultContent: seed,
		Content:        seed,
	}
	existing, err := os.ReadFile(path) //nolint:gosec // trusted gitid-managed path (G304)
	if err != nil && !os.IsNotExist(err) {
		return view, err
	}
	if err == nil {
		shape, ierr := gitconfig.InspectGitignoreFile(existing)
		if ierr != nil {
			return view, translateGitIgnoreErr(view.Path, ierr)
		}
		if shape.Managed {
			view.Content = shape.Body
			view.Managed = true
		}
	}
	// inspectBaselineFragment already runs its own error through
	// translateGitIgnoreErr with the baseline fragment's own display path —
	// do not re-translate (view.Path names the WRONG file for this error).
	baselineBytes, berr := b.inspectBaselineFragment()
	if berr != nil {
		return view, berr
	}
	view.Wiring, view.ExcludesFile = b.classifyGitIgnoreWiring(baselineBytes)
	if view.ExcludesFile != "" {
		// displayPath already shortens anything under b.home to its `~/...`
		// form (or leaves an outside-home path as the absolute path it
		// received) — its OWN output can never start with the raw b.home
		// prefix either way, so a second displayPath call here was always a
		// no-op (09.2-REVIEW.md IN-07).
		view.ExcludesFile = b.displayPath(expandTildeForHome(view.ExcludesFile, b.home))
	}
	return view, nil
}

// GlobalGitIgnoreApplyPlan normalizes content, re-inspects both the gitignore
// file and the baseline fragment, and returns a plan whose token hashes every
// preimage the plan READ. When core.excludesfile is unset it is a two-target
// plan: the gitignore file plus the baseline fragment, composed with the same
// patchExcludesfileInBaselineBody helper the doctor fix uses. When the key
// already points at the managed target or at a different file the plan stays
// single-target. A missing or malformed baseline block fails closed — no
// ceremony, no write.
func (b *realBackend) GlobalGitIgnoreApplyPlan(content string) (tuikit.GlobalGitIgnoreApplyPlanView, error) {
	if b.initErr != nil {
		return tuikit.GlobalGitIgnoreApplyPlanView{}, b.initErr
	}
	path := b.gitignorePath()
	lines, nerr := gitconfig.NormalizeGitignoreLines(content)
	if nerr != nil {
		return tuikit.GlobalGitIgnoreApplyPlanView{}, translateGitIgnoreErr(b.displayPath(path), nerr)
	}
	existing, err := os.ReadFile(path) //nolint:gosec // trusted gitid-managed path (G304)
	if err != nil && !os.IsNotExist(err) {
		return tuikit.GlobalGitIgnoreApplyPlanView{}, err
	}
	if _, ierr := gitconfig.InspectGitignoreFile(existing); ierr != nil {
		return tuikit.GlobalGitIgnoreApplyPlanView{}, translateGitIgnoreErr(b.displayPath(path), ierr)
	}
	// inspectBaselineFragment already translates its own error against the
	// baseline fragment's display path — pass it through as-is.
	baselineBytes, berr := b.inspectBaselineFragment()
	if berr != nil {
		return tuikit.GlobalGitIgnoreApplyPlanView{}, berr
	}
	wiring, _ := b.classifyGitIgnoreWiring(baselineBytes)
	if wiring == tuikit.GitIgnoreNoBaselineBlock {
		return tuikit.GlobalGitIgnoreApplyPlanView{}, errors.New(tuikit.GitIgnoreWiringNoBaseline) //nolint:staticcheck // frozen UI copy, not a Go error-string convention violation
	}
	candidate := gitconfig.ComposeGlobalGitignore(existing, lines)
	view := tuikit.GlobalGitIgnoreApplyPlanView{
		Targets:   []string{b.displayPath(path)},
		Diff:      globalsTextDiff(string(existing), string(candidate)),
		PlanToken: gitIgnorePreimageToken(existing, baselineBytes),
	}
	if fileExists(path) && !bytes.Equal(existing, candidate) {
		view.Backups = []string{b.displayPath(path) + backupSuffixPreview}
	}
	if wiring == tuikit.GitIgnoreKeyUnset {
		shape, _ := gitconfig.InspectManagedBlockFile(baselineBytes, "baseline")
		newBody := patchExcludesfileInBaselineBody(shape.Body, "~/.gitignore_global")
		baselineCandidate := filewriter.ReplaceBlock(baselineBytes, "baseline", newBody)
		baselinePath := b.baselineTargetPath()
		view.Targets = append(view.Targets, b.displayPath(baselinePath))
		if fileExists(baselinePath) && !bytes.Equal(baselineBytes, baselineCandidate) {
			view.Backups = append(view.Backups, b.displayPath(baselinePath)+backupSuffixPreview)
		}
		view.Diff = view.Diff + "\n" + globalsTextDiff(string(baselineBytes), string(baselineCandidate))
	}
	return view, nil
}

// gitIgnoreRollback records one write so a failed pair can be undone.
// existed, not the backup path, decides the undo shape: filewriter.Write
// returns an empty backup only when the target did not pre-exist, so
// branching on a non-empty backup would leave a brand-new file behind.
type gitIgnoreRollback struct {
	path     string
	preimage []byte
	existed  bool
}

func undoGitIgnoreWrite(b *realBackend, rec gitIgnoreRollback) ([]string, error) {
	display := b.displayPath(rec.path)
	if rec.existed {
		if err := filewriter.WriteNoBackup(rec.path, rec.preimage, deleteGitconfigMode); err != nil {
			return nil, fmt.Errorf("restoring %s: %w", display, err)
		}
		return []string{display}, nil
	}
	if err := os.Remove(rec.path); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("removing newly created %s: %w", display, err)
	}
	return []string{display}, nil
}

func (b *realBackend) injectGitIgnoreStep(step string) error {
	if b.failCommitAt == nil {
		return nil
	}
	return b.failCommitAt(step)
}

// CommitGlobalGitIgnore writes the gitignore managed block and, when
// core.excludesfile is unset, the matching key inside the managed baseline
// block — both under txMu, both covered by the plan token. Order: the
// baseline fragment FIRST, then the gitignore file. Each write records
// {path, preimage, existed} before mutating; if the second write fails the
// first is undone from that record (restored from the captured bytes when
// it pre-existed, removed when this transaction created it). A token
// mismatch, a missing baseline block, or a malformed fragment refuses
// without writing.
func (b *realBackend) CommitGlobalGitIgnore(content, planToken string) tea.Cmd {
	return func() tea.Msg {
		if b.initErr != nil {
			return tuikit.GlobalGitIgnoreCommitMsg{Err: b.displayMessage(b.initErr.Error())}
		}
		b.txMu.Lock()
		defer b.txMu.Unlock()
		path := b.gitignorePath()
		lines, nerr := gitconfig.NormalizeGitignoreLines(content)
		if nerr != nil {
			return tuikit.GlobalGitIgnoreCommitMsg{Err: b.displayMessage(translateGitIgnoreErr(b.displayPath(path), nerr).Error())}
		}
		existing, err := os.ReadFile(path) //nolint:gosec // trusted gitid-managed path (G304)
		if err != nil && !os.IsNotExist(err) {
			return tuikit.GlobalGitIgnoreCommitMsg{Err: b.displayMessage(err.Error())}
		}
		if _, ierr := gitconfig.InspectGitignoreFile(existing); ierr != nil {
			return tuikit.GlobalGitIgnoreCommitMsg{Err: b.displayMessage(translateGitIgnoreErr(b.displayPath(path), ierr).Error())}
		}
		// inspectBaselineFragment already translates its own error against
		// the baseline fragment's display path.
		baselineBytes, berr := b.inspectBaselineFragment()
		if berr != nil {
			return tuikit.GlobalGitIgnoreCommitMsg{Err: b.displayMessage(berr.Error())}
		}
		if gitIgnorePreimageToken(existing, baselineBytes) != planToken {
			return tuikit.GlobalGitIgnoreCommitMsg{
				Err:                 b.displayMessage(tuikit.GitIgnoreReceiptChangedSincePreview),
				ChangedSincePreview: true,
			}
		}
		wiring, _ := b.classifyGitIgnoreWiring(baselineBytes)
		if wiring == tuikit.GitIgnoreNoBaselineBlock {
			return tuikit.GlobalGitIgnoreCommitMsg{
				Err: b.displayMessage(tuikit.GitIgnoreWiringNoBaseline),
			}
		}

		var backups []string
		// failPair's Err is the CAUSE only — the successfully-restored-paths
		// sentence belongs to the view (gitignore.go's handleMsg already
		// appends "(restored: ...)" from Restored), so appending it here too
		// would show the same path list twice in the one message that
		// matters most: a partially failed two-file write
		// (09.2-REVIEW.md WR-09). "undo failed" stays in Err: unlike a
		// successful restore, that failure is NOT captured by Restored (it
		// stays empty/partial in that case) and would otherwise be lost.
		failPair := func(cause error, rec gitIgnoreRollback) tuikit.GlobalGitIgnoreCommitMsg {
			restored, undoErr := undoGitIgnoreWrite(b, rec)
			errText := b.displayMessage(cause.Error())
			if undoErr != nil {
				errText += "; undo failed: " + b.displayMessage(undoErr.Error())
			}
			return tuikit.GlobalGitIgnoreCommitMsg{Err: errText, Restored: restored}
		}

		if wiring == tuikit.GitIgnoreKeyUnset {
			baselinePath := b.baselineTargetPath()
			shape, ierr := gitconfig.InspectManagedBlockFile(baselineBytes, "baseline")
			if ierr != nil {
				return tuikit.GlobalGitIgnoreCommitMsg{Err: b.displayMessage(translateGitIgnoreErr(b.displayPath(baselinePath), ierr).Error())}
			}
			newBody := patchExcludesfileInBaselineBody(shape.Body, "~/.gitignore_global")
			composed := filewriter.ReplaceBlock(baselineBytes, "baseline", newBody)
			rec := gitIgnoreRollback{
				path:     baselinePath,
				preimage: append([]byte{}, baselineBytes...),
				existed:  fileExists(baselinePath),
			}
			if ferr := b.injectGitIgnoreStep("baseline-write"); ferr != nil {
				return tuikit.GlobalGitIgnoreCommitMsg{Err: b.displayMessage(ferr.Error())}
			}
			if !bytes.Equal(baselineBytes, composed) {
				bbak, werr := filewriter.Write(baselinePath, composed, deleteGitconfigMode)
				if werr != nil {
					return tuikit.GlobalGitIgnoreCommitMsg{Err: b.displayMessage(werr.Error())}
				}
				if bbak != "" {
					backups = append(backups, b.displayPath(bbak))
				}
			}
			if ferr := b.injectGitIgnoreStep("gitignore-write"); ferr != nil {
				return failPair(ferr, rec)
			}
			gbak, werr := gitconfig.WriteGlobalGitignore(path, lines)
			if werr != nil {
				return failPair(werr, rec)
			}
			if gbak != "" {
				backups = append(backups, b.displayPath(gbak))
			}
			return tuikit.GlobalGitIgnoreCommitMsg{Backups: backups}
		}

		if ferr := b.injectGitIgnoreStep("gitignore-write"); ferr != nil {
			return tuikit.GlobalGitIgnoreCommitMsg{Err: b.displayMessage(ferr.Error())}
		}
		backup, werr := gitconfig.WriteGlobalGitignore(path, lines)
		if werr != nil {
			return tuikit.GlobalGitIgnoreCommitMsg{Err: b.displayMessage(werr.Error())}
		}
		msg := tuikit.GlobalGitIgnoreCommitMsg{}
		if backup != "" {
			msg.Backups = []string{b.displayPath(backup)}
		}
		return msg
	}
}

// GitFallbackAuthorState reads the current fallback-author pair from the
// main config via ReadGitFallbackAuthor. Empty strings mean unset.
func (b *realBackend) GitFallbackAuthorState() (tuikit.GitFallbackAuthorView, error) {
	if b.initErr != nil {
		return tuikit.GitFallbackAuthorView{}, b.initErr
	}
	existing, err := os.ReadFile(b.gitconfigPath) //nolint:gosec // trusted gitid-managed path
	if err != nil && !os.IsNotExist(err) {
		return tuikit.GitFallbackAuthorView{}, err
	}
	name, email := gitconfig.ReadGitFallbackAuthor(existing)
	return tuikit.GitFallbackAuthorView{Name: name, Email: email}, nil
}

// GitFallbackAuthorPlan is the fallback-author apply preview: the resolved
// main-config target, the promised backup, and a diff of the candidate body.
func (b *realBackend) GitFallbackAuthorPlan(name, email string) (tuikit.GitFallbackAuthorPlanView, error) {
	if b.initErr != nil {
		return tuikit.GitFallbackAuthorPlanView{}, b.initErr
	}
	// Uses gitconfig.ValidateEmail — the SAME stricter check the write path
	// (lifecycle.go's runGitFallbackAuthorApply) applies, so the preview
	// can never accept a value the actual write would then reject.
	if email != "" {
		if err := gitconfig.ValidateEmail(email); err != nil {
			return tuikit.GitFallbackAuthorPlanView{}, fmt.Errorf("gitid: malformed fallback email: %w", err)
		}
	}
	existing, err := os.ReadFile(b.gitconfigPath) //nolint:gosec // trusted gitid-managed path
	if err != nil && !os.IsNotExist(err) {
		return tuikit.GitFallbackAuthorPlanView{}, err
	}
	currentName, currentEmail := gitconfig.ReadGitFallbackAuthor(existing)
	hasBlock := currentName != "" || currentEmail != ""
	emptyPair := name == "" && email == ""
	view := tuikit.GitFallbackAuthorPlanView{
		Targets: []string{b.displayPath(b.gitconfigPath)},
		Removal: emptyPair && hasBlock,
	}
	if fileExists(b.gitconfigPath) && (!emptyPair || hasBlock) {
		view.Backups = []string{b.displayPath(b.gitconfigPath) + backupSuffixPreview}
	}
	composed := gitconfig.ComposeBaselineInclude(existing, b.displayBaselineTargetPath())
	composed, err = gitconfig.EnsureGitFallbackAuthor(composed, name, email)
	if err != nil {
		return tuikit.GitFallbackAuthorPlanView{}, err
	}
	view.Diff = globalsTextDiff(string(existing), string(composed))
	return view, nil
}

// CommitGitFallbackAuthor is the fallback-author apply async seam: call
// runGitFallbackAuthorApply — the ONE complete per-verb ceremony — with
// confirmationAlreadyObtained and map the result onto the commit message.
func (b *realBackend) CommitGitFallbackAuthor(name, email string) tea.Cmd {
	return func() tea.Msg {
		if b.initErr != nil {
			return tuikit.GitFallbackAuthorCommitMsg{Err: b.displayMessage(b.initErr.Error())}
		}
		res, err := b.runGitFallbackAuthorApply(name, email, lifecyclePolicy{Confirm: confirmationAlreadyObtained})
		msg := tuikit.GitFallbackAuthorCommitMsg{
			Backups:    displayPaths(b, res.Backups),
			Restored:   displayMessages(b, res.Restored),
			Advisories: displayMessages(b, res.Advisories),
		}
		if err != nil {
			msg.Err = b.displayMessage(err.Error())
		}
		return msg
	}
}

// ---------------------------------------------------------------------------
// SSHStoragePlanner (plan 06-05)
// ---------------------------------------------------------------------------

// errReopenPreview is the sentinel error returned when CommitSSHStorage is
// called with a token that does not match the held plan — either because the
// selection changed, the screen was re-entered, the plan was already
// committed, or the token was never issued. Distinct from a generic migration
// failure so the screen can render the re-open-the-preview message for exactly
// this cause.
var errReopenPreview = errors.New("gitid: re-open the preview — the held plan no longer matches this token")

// planTokenFor derives the opaque token for a MigrationPlan. It is a
// deterministic fingerprint over direction + both file paths + both After
// byte slices — enough to uniquely identify the plan without leaking any
// file content across the boundary, and stable across a process restart
// only by accident (the digests change when disk changes). The token is
// treated as opaque by tuikit and by the caller: never parsed, never
// compared to anything but itself.
//
// WR-04: the token is a REAL hash (sha256 of the fingerprint string), not a
// hex ENCODING of it. `%x` on a string/[]byte is reversible hex encoding, not
// hashing — hex-encoding the fingerprint (which itself embeds both absolute
// file paths, i.e. the user's home directory) made the token a trivially
// reversible ~350-byte dump of the user's home directory, double-hex-encoded.
// The token crosses into the view layer and PTY frame captures, so it must
// not leak path content. sha256+hex here produces a genuinely
// collision-resistant, fixed-length (64 hex chars), non-reversible token.
func planTokenFor(plan sshconfig.MigrationPlan) string {
	h := fmt.Sprintf("%d|%s|%s|%s|%s",
		plan.Direction, plan.SourcePath, plan.DestPath,
		plan.Digests[plan.SourcePath], plan.Digests[plan.DestPath])
	sum := sha256.Sum256([]byte(h))
	return hex.EncodeToString(sum[:])
}

// putPendingMigration stores plan under token, replacing any previously held
// plan. Called while txMu is held (rule 1's order: txMu first).
func (b *realBackend) putPendingMigration(token string, plan sshconfig.MigrationPlan) {
	b.pendingMigrationMu.Lock()
	b.pendingToken = token
	b.pendingMigration = plan
	b.pendingMigrationMu.Unlock()
}

// takePendingMigration is the atomic lookup-and-consume: one
// pendingMigrationMu hold that compares the token, copies the plan out,
// CLEARS the slot and returns. Not a get followed by a separate clear,
// because the gap between them is a window where a second confirmation could
// commit the same plan twice. A non-matching token clears NOTHING and
// returns false, so a bogus commit cannot evict a legitimate held plan.
// This helper NEVER takes txMu; its caller (runSSHStorageMigrate) already
// holds txMu and Go's sync.Mutex is not reentrant — taking txMu here would
// self-deadlock (the cycle-3 MEDIUM the <lock_contract> was written to prevent).
func (b *realBackend) takePendingMigration(token string) (sshconfig.MigrationPlan, bool) {
	b.pendingMigrationMu.Lock()
	defer b.pendingMigrationMu.Unlock()
	if b.pendingToken != token {
		return sshconfig.MigrationPlan{}, false
	}
	plan := b.pendingMigration
	b.pendingToken = ""
	b.pendingMigration = sshconfig.MigrationPlan{}
	return plan, true
}

// managedAliases returns the set of all managed SSH aliases from the current
// storage target — these are the aliases the migration engine validates
// resolution for after the move.
func (b *realBackend) managedAliases() []string {
	st := b.storage()
	content, err := os.ReadFile(st.targetPath) //nolint:gosec // trusted gitid-managed path
	if err != nil {
		return nil
	}
	hosts, err := sshconfig.ParseManagedHosts(content)
	if err != nil {
		return nil
	}
	aliases := make([]string, 0, len(hosts))
	for alias := range hosts {
		aliases = append(aliases, alias)
	}
	return aliases
}

// SSHStorageMigrationPlan is the Storage sub-tab preview seam.
//
// It acquires txMu BEFORE resolving the current layout — per <lock_contract>
// rule 5, every cross-file READ of the two config files takes txMu so a
// preview opened between the migration engine's destination write and its
// source trim waits for the transaction to finish and observes only the final
// coherent state. (The migration engine writes the destination in step 3 and
// trims the source in step 4 as two SEPARATE writes; without this lock a
// preview landing between them renders a diff of a state that never existed.)
//
// Both locks are released before returning to the UI: txMu must NOT cross the
// tea.Cmd boundary, and pendingMigrationMu is never held across I/O
// (<lock_contract> rule 2).
//
// MUST NOT be called from runSSHStorageMigrate: that function already holds
// txMu, and Go's sync.Mutex is not reentrant. The CLI branch (plan 06-06)
// calls sshconfig.PlanMigration directly for exactly this reason — stated
// there too.
func (b *realBackend) SSHStorageMigrationPlan(layout tuikit.SSHStorageLayout) (tuikit.SSHStorageMigrationView, error) {
	if b.initErr != nil {
		return tuikit.SSHStorageMigrationView{}, b.initErr
	}

	// Rule 5: acquire txMu before ANY read of the two config files.
	b.txMu.Lock()
	defer b.txMu.Unlock()

	st := b.storage()
	currentLayout := tuikit.StorageSentinel
	if st.includeLayout {
		currentLayout = tuikit.StorageInclude
	}
	// CR-05: requesting a plan for the CURRENT layout is NOT an error — it is
	// the pane's normal first-activation state (activate() seeds
	// m.storageChoice from the live layout, so every entry to the tab used
	// to hit this branch). PlanMigration naturally computes a no-op plan for
	// this case (the "moving" source file has nothing gitid-managed to move,
	// so DestAfter is just the current managed content unchanged) — it is a
	// legitimate "resulting config if you stay here" preview, not an error
	// condition. Do NOT special-case it; let it fall through to the same
	// planning path every other layout choice takes.
	direction := sshconfig.MigrateToInclude
	if layout == tuikit.StorageSentinel {
		direction = sshconfig.MigrateToInFile
	}

	aliases := b.managedAliases()
	deps := newMigrateDeps(b.sshConfigPath, filepath.Join(b.includeDir, gitidConfigFileName), aliases)
	plan, err := sshconfig.PlanMigration(direction, deps)
	if err != nil {
		return tuikit.SSHStorageMigrationView{}, fmt.Errorf("gitid: planning migration: %w", err)
	}

	token := planTokenFor(plan)

	// Rule 1 acquisition order: txMu first (already held), then pendingMigrationMu.
	b.putPendingMigration(token, plan)

	toInclude := layout == tuikit.StorageInclude
	headingTail := "sentinel blocks in ~/.ssh/config"
	if toInclude {
		headingTail = "Include\xe2\x80\x99d gitid.config"
	}

	targets := []string{b.displayPath(b.sshConfigPath), b.displayPath(filepath.Join(b.includeDir, gitidConfigFileName))}
	backups := []string{}
	for _, p := range []string{b.sshConfigPath, filepath.Join(b.includeDir, gitidConfigFileName)} {
		if fileExists(p) {
			backups = append(backups, b.displayPath(p)+backupSuffixPreview)
		}
	}

	view := tuikit.SSHStorageMigrationView{
		CurrentLayout: currentLayout,
		TargetLayout:  layout,
		Heading:       "Migrate SSH storage layout \xe2\x86\x92 " + headingTail,
		Targets:       targets,
		Backups:       backups,
		Diff:          plan.Diff,
		SourceBefore:  string(plan.SourceBefore),
		DestBefore:    string(plan.DestBefore),
		PlanToken:     token,
	}
	// The three preview fields are set exactly once each, below, based on
	// PlanMigration's own source/dest assignment for each direction — never
	// pre-seeded in the struct literal above, so there is no dead write to
	// spot the correct branch overwriting.
	//
	// PlanMigration(MigrateToInclude): source=~/.ssh/config, dest=gitid.config.
	// PlanMigration(MigrateToInFile):  source=gitid.config, dest=~/.ssh/config.
	if toInclude {
		view.MainPreview = string(plan.SourceAfter) // ~/.ssh/config after migration (has Include line, no blocks)
		view.OwnedPreview = string(plan.DestAfter)  // gitid.config after migration (has all blocks)
	} else {
		view.SentinelPreview = string(plan.DestAfter) // ~/.ssh/config after migration (has all blocks)
	}
	return view, nil
}

// CommitSSHStorage is the storage-migration async seam, mirroring
// CommitGlobalSSH exactly: a tea.Cmd closure that guards b.initErr, calls
// runSSHStorageMigrate with the token and confirmationAlreadyObtained,
// scrubs paths and messages, and returns an SSHStorageCommitMsg. The token
// must pass through UNCHANGED — a commit path that regenerates or ignores the
// token puts the two-reads-of-disk defect straight back.
func (b *realBackend) CommitSSHStorage(layout tuikit.SSHStorageLayout, planToken string) tea.Cmd {
	return func() tea.Msg {
		if b.initErr != nil {
			return tuikit.SSHStorageCommitMsg{Err: b.displayMessage(b.initErr.Error())}
		}
		res, err := b.runSSHStorageMigrate(layout, planToken, lifecyclePolicy{Confirm: confirmationAlreadyObtained})
		msg := tuikit.SSHStorageCommitMsg{
			Backups:  displayPaths(b, res.Backups),
			Restored: displayMessages(b, res.Restored),
		}
		if err != nil {
			msg.Err = b.displayMessage(err.Error())
			if errors.Is(err, sshconfig.ErrConfigChangedSincePreview) {
				msg.ConfigChangedSincePreview = true
			}
		}
		return msg
	}
}

// globalSSHProvenanceLabel renders the D-03 provenance label for one source
// class, scoped to exactly what each class proves (06-REVIEWS.md HIGH pin):
// the parsed class names the file and line gitid actually read; the system
// class names the system file gitid actually parsed and states gitid cannot
// change it; the baseline class states the option is not set in any file
// gitid reads and gives the value OpenSSH resolves without a user
// configuration on this machine; the outside class states the value comes
// from somewhere gitid does not read; the inconclusive class states the probe
// did not answer. 06-03 owns exact copy-freeze wording; these forms are the
// scoped placeholders.
func (b *realBackend) globalSSHProvenanceLabel(st globalssh.OptionStatus) string {
	switch st.Source {
	case globalssh.SourceGitidParsed:
		return fmt.Sprintf("set by you at %s line %d", b.displayPath(st.SourceFile), st.SourceLine)
	case globalssh.SourceSystemFile:
		return fmt.Sprintf("set in %s — gitid cannot change this", b.displayPath(st.SourceFile))
	case globalssh.SourceBaseline:
		return fmt.Sprintf("not set (OpenSSH default: %s)", st.CurrentValue)
	case globalssh.SourceOutsideGitid:
		return "set outside your config"
	default:
		return "the probe did not answer — see the advisory note"
	}
}

// toGlobalSSHOptionState maps the engine's OptionState to the render DTO's
// row state. The one-to-one correspondence is structurally pinned by the
// exported constants on both sides.
func toGlobalSSHOptionState(s globalssh.OptionState) tuikit.GlobalSSHOptionState {
	switch s {
	case globalssh.StateAlreadySet:
		return tuikit.GlobalSSHAlreadySet
	case globalssh.StateDiffers:
		return tuikit.GlobalSSHDiffers
	case globalssh.StateNotApplicable:
		return tuikit.GlobalSSHNotApplicable
	default:
		return tuikit.GlobalSSHNeedsAction
	}
}

// globalsBodyText returns the body of the gitid `Host *` managed block in
// content (empty when absent) — the /candidate diff is computed over the
// block that EnsureGlobals actually owns and changes.
func globalsBodyText(content []byte) string {
	for _, blk := range filewriter.ListBlocks(content) {
		if blk.Name == sshconfig.GlobalBlockName {
			return blk.Body
		}
	}
	return ""
}

// globalsTextDiff is a compact +/− line diff of the block body before and
// after. It is the honest, best-effort preview text the apply ceremony shows;
// plan 06-04's pre-write simulation supersedes it with a real `ssh -G -F`
// re-verification.
func globalsTextDiff(before, after string) string {
	oldLines := splitLines(before)
	newLines := splitLines(after)
	var out []string
	i, j := 0, 0
	for i < len(oldLines) && j < len(newLines) {
		if oldLines[i] == newLines[j] {
			i++
			j++
			continue
		}
		// A line that still appears later in the new set was removed from
		// before; otherwise it was added to after.
		if containsLine(newLines[j:], oldLines[i]) {
			out = append(out, "- "+oldLines[i])
			i++
		} else {
			out = append(out, "+ "+newLines[j])
			j++
		}
	}
	for ; i < len(oldLines); i++ {
		out = append(out, "- "+oldLines[i])
	}
	for ; j < len(newLines); j++ {
		out = append(out, "+ "+newLines[j])
	}
	return strings.Join(out, "\n")
}

// splitLines splits s into non-empty trimmed lines.
func splitLines(s string) []string {
	seen := make([]string, 0, 8)
	for _, line := range strings.Split(s, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		seen = append(seen, line)
	}
	return seen
}

// containsLine reports whether lines contains line (exact match).
func containsLine(lines []string, line string) bool {
	for _, l := range lines {
		if l == line {
			return true
		}
	}
	return false
}

// injectDeleteFailures wraps deps' WriteGitconfig/RemoveFragment with
// b.failCommitAt fault-injection checkpoints ("delete-gitconfig",
// "delete-fragment") — the same test-only injection point
// commitGitArtifacts's inject() closure uses — so a test can prove a
// mid-transaction delete failure restores every touched file via
// runDelete's journal.restore() call above.
func injectDeleteFailures(b *realBackend, deps identity.DeleteDeps) identity.DeleteDeps {
	origWriteGitconfig := deps.WriteGitconfig
	deps.WriteGitconfig = func(content []byte) (string, error) {
		if err := b.failCommitAt("delete-gitconfig"); err != nil {
			return "", err
		}
		return origWriteGitconfig(content)
	}
	origRemoveFragment := deps.RemoveFragment
	deps.RemoveFragment = func(fragPath string) (string, error) {
		if err := b.failCommitAt("delete-fragment"); err != nil {
			return "", err
		}
		return origRemoveFragment(fragPath)
	}
	return deps
}

// injectRotateFailures wraps each rotate transaction step's Deps seam with a
// b.failCommitAt fault-injection checkpoint — the same test-only injection
// point commitGitArtifacts's inject() and injectDeleteFailures use — so a
// test can prove a failure at ANY step of a rotation restores every watched
// file (bytes AND mode, including the removed-and-replaced key pair) and
// removes every archive copy this transaction created. Checkpoint names:
// rotate-archive, rotate-persist-key, rotate-ssh, rotate-gitconfig,
// rotate-fragment, rotate-signers.
func injectRotateFailures(b *realBackend, deps identity.Deps) identity.Deps {
	origArchive := deps.ArchiveKeyPair
	deps.ArchiveKeyPair = func(privPath, pubPath string) (string, string, error) {
		if err := b.failCommitAt("rotate-archive"); err != nil {
			return "", "", err
		}
		return origArchive(privPath, pubPath)
	}
	origPersist := deps.PersistKey
	deps.PersistKey = func(s identity.StagedKey) (identity.KeyResult, error) {
		if err := b.failCommitAt("rotate-persist-key"); err != nil {
			return identity.KeyResult{}, err
		}
		return origPersist(s)
	}
	origWriteSSH := deps.WriteSSH
	deps.WriteSSH = func(accountName, hostBlock, globalsGOOS string) (string, error) {
		if err := b.failCommitAt("rotate-ssh"); err != nil {
			return "", err
		}
		return origWriteSSH(accountName, hostBlock, globalsGOOS)
	}
	origWriteGit := deps.WriteGitconfig
	deps.WriteGitconfig = func(id, fragmentPath, allowedSignersPath string, matches []gitconfig.Match) (string, error) {
		if err := b.failCommitAt("rotate-gitconfig"); err != nil {
			return "", err
		}
		return origWriteGit(id, fragmentPath, allowedSignersPath, matches)
	}
	origWriteFrag := deps.WriteFragment
	deps.WriteFragment = func(fragmentPath, name, email, signingKeyPath string, signing bool) error {
		if err := b.failCommitAt("rotate-fragment"); err != nil {
			return err
		}
		return origWriteFrag(fragmentPath, name, email, signingKeyPath, signing)
	}
	origAppend := deps.AppendAllowedSigners
	deps.AppendAllowedSigners = func(path, identity, email, pubLine string) (string, error) {
		if err := b.failCommitAt("rotate-signers"); err != nil {
			return "", err
		}
		return origAppend(path, identity, email, pubLine)
	}
	return deps
}

// injectRepairFailures is injectRotateFailures's sibling for the repair
// path. Repair never archives, so there is no rotate-archive checkpoint; the
// step names are repair-persist-key, repair-ssh, repair-gitconfig,
// repair-fragment, repair-signers (plus the shared restore: boundaries the
// journal itself injects).
func injectRepairFailures(b *realBackend, deps identity.Deps) identity.Deps {
	origPersist := deps.PersistKey
	deps.PersistKey = func(s identity.StagedKey) (identity.KeyResult, error) {
		if err := b.failCommitAt("repair-persist-key"); err != nil {
			return identity.KeyResult{}, err
		}
		return origPersist(s)
	}
	origWriteSSH := deps.WriteSSH
	deps.WriteSSH = func(accountName, hostBlock, globalsGOOS string) (string, error) {
		if err := b.failCommitAt("repair-ssh"); err != nil {
			return "", err
		}
		return origWriteSSH(accountName, hostBlock, globalsGOOS)
	}
	origWriteGit := deps.WriteGitconfig
	deps.WriteGitconfig = func(id, fragmentPath, allowedSignersPath string, matches []gitconfig.Match) (string, error) {
		if err := b.failCommitAt("repair-gitconfig"); err != nil {
			return "", err
		}
		return origWriteGit(id, fragmentPath, allowedSignersPath, matches)
	}
	origWriteFrag := deps.WriteFragment
	deps.WriteFragment = func(fragmentPath, name, email, signingKeyPath string, signing bool) error {
		if err := b.failCommitAt("repair-fragment"); err != nil {
			return err
		}
		return origWriteFrag(fragmentPath, name, email, signingKeyPath, signing)
	}
	origAppend := deps.AppendAllowedSigners
	deps.AppendAllowedSigners = func(path, identity, email, pubLine string) (string, error) {
		if err := b.failCommitAt("repair-signers"); err != nil {
			return "", err
		}
		return origAppend(path, identity, email, pubLine)
	}
	return deps
}

// collectDeleteBackups gathers every non-empty backup path a DeleteResult
// carries, in the same field order the result declares them.
func collectDeleteBackups(res identity.DeleteResult) []string {
	var out []string
	for _, p := range []string{
		res.SSHBackup, res.GitconfigBackup, res.FragmentBackup, res.AllowedSignersBackup,
		res.KeyBackup, res.PubBackup, res.ProviderRewriteBackup,
	} {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Clone (D-14/D-15/D-16/D-17, MGR-04) — plan 05-05.
//
// SuggestCloneName and ClonePrefill are PREVIEW-ONLY seams: neither writes
// anything. D-15 is explicit that clone gets no second write pipeline — the
// pre-filled wizard ClonePrefill returns feeds into commits through the
// SAME CommitCreate a fresh create uses (identities.go's handleCloneKey +
// newWizardPrefilled).
// ---------------------------------------------------------------------------

// takenCloneNames builds the D-17 taken-name list: every existing identity
// name UNION every LITERAL (non-wildcard, non-negated) parsed Host
// pattern's derived name, compared case-insensitively by
// identity.SuggestCloneName. A wildcard pattern is deliberately NOT part of
// this list — it can never be silently bumped past (review R-29); it is
// caught by the SEPARATE pattern-shadowing check inside ClonePrefill.
func (b *realBackend) takenCloneNames() []string {
	var taken []string
	for _, a := range b.accounts() {
		taken = append(taken, a.Name)
	}
	if b.initErr != nil {
		return taken
	}
	content, err := os.ReadFile(b.sshConfigPath) //nolint:gosec // b.sshConfigPath is a trusted gitid-managed path resolved in-process
	if err != nil {
		return taken
	}
	for _, stanza := range sshconfig.AllHostStanzas(content) {
		if strings.ContainsAny(stanza.Alias, "*?") || strings.HasPrefix(stanza.Alias, "!") {
			continue // wildcard/negated — the shadowing check's job, not availability's
		}
		taken = append(taken, nameFromAlias(stanza.Alias))
	}
	return taken
}

// nameFromAlias strips a two-label provider suffix off alias to recover the
// NAME half of the `<name>.<provider>` convention (identity.DefaultAlias's
// inverse) — used only to compare a hand-written literal alias against a
// candidate NAME for the D-17 taken-list union.
func nameFromAlias(alias string) string {
	parts := strings.Split(alias, ".")
	if len(parts) <= 2 {
		return alias
	}
	return parts[0]
}

// SuggestCloneName is the D-17 suggested clone name for source, silently
// auto-bumped past every taken name (existing identities plus every literal
// hand-written alias) so the clone-name prompt never opens already claimed.
func (b *realBackend) SuggestCloneName(source string) string {
	return identity.SuggestCloneName(source, b.takenCloneNames())
}

// matchStrategyFromMatches projects a derived Matches slice back into the
// wizard's three-value strategy vocabulary (gitdir/hasconfig/both) — the
// SAME vocabulary buildMatchesForStrategy's switch produces, run in reverse.
func matchStrategyFromMatches(matches []gitconfig.Match) string {
	hasGitdir, hasHasconfig := false, false
	for _, m := range matches {
		switch m.Kind {
		case gitconfig.MatchGitdir:
			hasGitdir = true
		case gitconfig.MatchHasconfig:
			hasHasconfig = true
		}
	}
	switch {
	case hasGitdir && hasHasconfig:
		return "both"
	case hasHasconfig:
		return "hasconfig"
	default:
		return "gitdir"
	}
}

// gitDirFromMatches returns the first gitdir-kind match's value, or "" when
// the derived Matches carry no gitdir entry (a pure-hasconfig clone).
func gitDirFromMatches(matches []gitconfig.Match) string {
	for _, m := range matches {
		if m.Kind == gitconfig.MatchGitdir {
			return m.Value
		}
	}
	return ""
}

// ClonePrefill derives cloneName's pre-fill values from source (D-14) and
// returns them as the DTO the wizard opens with — never a write. Two
// DISTINCT checks run before a value is returned (review R-29):
//
//  1. Availability (identity.DeriveCloneInput's own name/alias validation,
//     via ValidateName + sshconfig.ValidateHostBlock) — the caller
//     (SuggestCloneName) already silently bumped past every taken literal
//     name, so this mostly re-confirms rather than surprises.
//  2. Pattern shadowing — the derived ALIAS is checked against every parsed
//     Host pattern using REAL OpenSSH pattern semantics
//     (sshconfig.MatchingHostStanzas, never a hand-rolled globber). A
//     wildcard stanza the availability scan can never see (it only sees
//     LITERAL patterns) is a typed, non-bumpable refusal naming the
//     shadowing pattern — the user needs to know a wildcard stanza is in
//     play, not have the name silently changed under them.
func (b *realBackend) ClonePrefill(source, cloneName string, reuseSourceKey bool) (tuikit.ClonePrefillView, error) {
	if b.initErr != nil {
		return tuikit.ClonePrefillView{}, b.initErr
	}
	src, found := b.findAccount(source)
	if !found {
		return tuikit.ClonePrefillView{}, fmt.Errorf("clone: source identity %q not found", source)
	}
	// Reconstruct stores KeyPath verbatim from the parsed config (often a
	// literal "~/…") — expand it before it becomes the resolved
	// ReuseKeyPath the wizard's D-10 picker looks up against
	// ScanReusableKeys' absolute paths (runDelete's own precedent, above).
	src.KeyPath = expandTildeForHome(src.KeyPath, b.home)

	targets := identity.CloneTargets{
		GitconfigPath:      b.gitconfigPath,
		SSHConfigPath:      b.sshConfigPath,
		AllowedSignersPath: b.allowedSigners,
		FragmentDir:        b.fragmentDir,
	}
	in, notices, derr := identity.DeriveCloneInput(src, cloneName, reuseSourceKey, targets)
	if derr != nil {
		return tuikit.ClonePrefillView{}, derr
	}

	content, rerr := os.ReadFile(b.sshConfigPath) //nolint:gosec // b.sshConfigPath is a trusted gitid-managed path resolved in-process
	if rerr == nil {
		if matches := sshconfig.MatchingHostStanzas(content, in.Alias); len(matches) > 0 {
			return tuikit.ClonePrefillView{}, &identity.FieldError{
				Field: "alias",
				Message: fmt.Sprintf(
					"%q is already matched by the existing Host pattern %q — pick a different name or edit that pattern first",
					in.Alias, matches[0]),
			}
		}
	}

	return tuikit.ClonePrefillView{
		SourceName:    source,
		CloneName:     in.Name,
		AliasPrefix:   in.Name,
		Hostname:      in.Hostname,
		Port:          strconv.Itoa(in.Port),
		GitName:       in.GitName,
		GitEmail:      in.GitEmail,
		MatchStrategy: matchStrategyFromMatches(in.Matches),
		GitDir:        gitDirFromMatches(in.Matches),
		ReuseKeyPath:  in.ReuseKeyPath,
		CopiedFields:  notices.CopiedFields,
	}, nil
}

// mutationJournal captures the exact pre-transaction state of every mutable
// Git artifact. Its rollback intentionally restores from in-memory snapshots,
// not by moving timestamped backups: backups remain durable safety artifacts.
type mutationJournal struct {
	b        *realBackend
	files    []gitFileSnapshot
	dirs     []gitDirSnapshot
	seenFile map[string]bool
	seenDir  map[string]bool
	// chmodDirs records the pre-transaction mode of every PRE-EXISTING
	// directory this transaction actually chmods via ensureManagedDir (CR-05).
	// restore() reverts only these — not every watched-but-untouched ancestor
	// (WR-17) — so a failed transaction never rewrites permission metadata on
	// directories (including HOME) it never modified.
	chmodDirs []gitDirSnapshot
	backups   []string

	// createdFiles records paths a mid-transaction seam CREATED (not
	// snapshotted) — currently the D-06 archive copies recordCreatedFile
	// receives via depsForTransaction's onCreated observer. restore() removes
	// every one of these AFTER restoring every watched file (review R3-01).
	// seenCreated is the disjoint counterpart to seenFile (review R2-08): a
	// path recorded as CREATED can never also be recorded as WATCHED (a
	// snapshot-then-restore) and vice versa, because a path in both sets has
	// no correct rollback outcome.
	createdFiles []string
	seenCreated  map[string]bool

	// createdDirs (final two fields) records directories THIS transaction
	// created. Two mechanisms feed it: ensureDir appends each directory it
	// Mkdirs (a freshly created transaction root), and recordCreatedDir
	// appends a directory a mid-transaction seam created (currently the D-06
	// archive directory, which is timestamp-created DURING the transaction
	// and unknowable at watch time). seenCreatedDir is the disjoint
	// counterpart to the watched-set (review R2-08), enforced at
	// registration: a created DIRECTORY is removed on rollback (after every
	// created file), never snapshot-restored.
	createdDirs    []string
	seenCreatedDir map[string]bool
}

type gitFileSnapshot struct {
	path    string
	exists  bool
	content []byte
	mode    os.FileMode
}

type gitDirSnapshot struct {
	path   string
	exists bool
	mode   os.FileMode
}

func newMutationJournal(b *realBackend) *mutationJournal {
	return &mutationJournal{
		b:              b,
		seenFile:       make(map[string]bool),
		seenDir:        make(map[string]bool),
		seenCreated:    make(map[string]bool),
		seenCreatedDir: make(map[string]bool),
	}
}

// recordCreatedFile records path as a file THIS transaction created mid-
// flight (currently: a D-06 archive copy), so restore() can remove it on
// rollback. It is the identity.Deps.ArchiveKeyPair observer bound by
// depsForTransaction — passed directly as a keygen.CreatedFunc, so a
// refusal here STOPS the archive before the copy is trusted, rather than
// merely being noticed afterwards.
//
// Recording the SAME path twice is an idempotent no-op (plan 05-07's
// lifecycle also records whatever the domain result names, and the two
// sources will normally agree). Recording a path already registered as
// WATCHED is a programming error: review R2-08's disjointness contract — a
// path in both sets has no correct rollback outcome (restore first reverts
// the watched snapshot, then would try to remove the very file it just
// restored).
func (j *mutationJournal) recordCreatedFile(path string) error {
	if j.seenCreated[path] {
		return nil
	}
	if j.seenFile[path] {
		return fmt.Errorf("gitid: internal: %s is already watched, cannot also be recorded as created", path)
	}
	j.seenCreated[path] = true
	j.createdFiles = append(j.createdFiles, path)
	return nil
}

// recordCreatedDir records path as a DIRECTORY this transaction created
// mid-flight (currently: the D-06 archive directory, whose timestamped name
// cannot be known at watch time), so restore() removes it too — after every
// recorded created file, so the copies inside a fresh archive shell go first
// (review R-10). It mirrors recordCreatedFile's disjointness rule: a path
// already registered as a WATCHED file or as a created FILE has no correct
// rollback outcome (a path in both sets cannot be both snapshot-restored and
// removed), so a mid-transaction seam that trips the check ABORTS before the
// directory is trusted, exactly like a recordCreatedFile refusal.
//
// Recording the SAME directory twice is an idempotent no-op. It is only ever
// called when the directory did NOT exist before the transaction (the
// lifecycle stats the archive directory first): a pre-existing directory must
// NEVER be recorded as created, or rollback would remove existing archives.
func (j *mutationJournal) recordCreatedDir(path string) error {
	clean := filepath.Clean(path)
	if j.seenCreatedDir[clean] {
		return nil
	}
	if j.seenFile[clean] {
		return fmt.Errorf("gitid: internal: %s is already watched, cannot also be recorded as created", path)
	}
	if j.seenCreated[clean] {
		return fmt.Errorf("gitid: internal: %s is already recorded as created, cannot also be recorded as a created directory", path)
	}
	if j.seenDir[clean] {
		return fmt.Errorf("gitid: internal: %s is already watched as a directory, cannot also be recorded as created", path)
	}
	j.seenCreatedDir[clean] = true
	j.createdDirs = append(j.createdDirs, clean)
	return nil
}

func (j *mutationJournal) watchFile(path string) error {
	if j.seenFile[path] {
		return nil
	}
	if j.seenCreated[path] {
		return fmt.Errorf("gitid: internal: %s is already recorded as created, cannot also be watched", path)
	}
	clean := filepath.Clean(path)
	if j.seenCreatedDir[clean] {
		return fmt.Errorf("gitid: internal: %s is already recorded as a created directory, cannot also be watched", path)
	}
	if err := containedRegularPath(path, j.b.home); err != nil {
		return err
	}
	j.seenFile[path] = true
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		j.files = append(j.files, gitFileSnapshot{path: path})
		return nil
	}
	if err != nil {
		return fmt.Errorf("gitid: stat transaction target %s: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("gitid: refusing non-regular transaction target: %s", path)
	}
	content, err := os.ReadFile(path) //nolint:gosec // path was contained and checked above
	if err != nil {
		return fmt.Errorf("gitid: snapshotting %s: %w", path, err)
	}
	j.files = append(j.files, gitFileSnapshot{path: path, exists: true, content: content, mode: info.Mode().Perm()})
	return nil
}

func (j *mutationJournal) watchDir(path string) error {
	clean := filepath.Clean(path)
	if j.seenDir[clean] {
		return nil
	}
	// Disjointness (review R2-08): a directory recorded as CREATED by this
	// transaction cannot ALSO be watched — its only correct rollback outcome
	// is removal, never a snapshot-restore.
	if j.seenCreatedDir[clean] {
		return fmt.Errorf("gitid: internal: %s is already recorded as created, cannot also be watched", path)
	}
	if err := containedRegularPath(path, j.b.home); err != nil {
		return err
	}
	j.seenDir[clean] = true
	info, err := os.Lstat(clean)
	if os.IsNotExist(err) {
		j.dirs = append(j.dirs, gitDirSnapshot{path: clean})
		return nil
	}
	if err != nil {
		return fmt.Errorf("gitid: stat transaction directory %s: %w", path, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("gitid: refusing non-directory transaction path: %s", path)
	}
	j.dirs = append(j.dirs, gitDirSnapshot{path: clean, exists: true, mode: info.Mode().Perm()})
	return nil
}

func (j *mutationJournal) ensureDir(path string, mode os.FileMode) error {
	clean := filepath.Clean(path)
	if clean == filepath.Clean(j.b.home) {
		return fmt.Errorf("gitid: refusing to manage the home directory itself: %s", path)
	}
	var missing []string
	for current := clean; ; current = filepath.Dir(current) {
		if err := j.watchDir(current); err != nil {
			return err
		}
		if current == filepath.Clean(j.b.home) {
			break
		}
		if _, err := os.Lstat(current); os.IsNotExist(err) {
			missing = append(missing, current)
		} else if err != nil {
			return fmt.Errorf("gitid: checking transaction directory %s: %w", current, err)
		}
	}
	// Only directories THIS transaction creates get their mode set (and
	// restored on rollback via createdDirs). A pre-existing directory the
	// user names via an editable path (e.g. gitdir) must never be
	// permission-mutated as a side effect of being referenced here.
	for i := len(missing) - 1; i >= 0; i-- {
		if err := os.Mkdir(missing[i], mode); err != nil {
			return fmt.Errorf("gitid: creating transaction directory %s: %w", missing[i], err)
		}
		j.createdDirs = append(j.createdDirs, missing[i])
		if err := os.Chmod(missing[i], mode); err != nil {
			return fmt.Errorf("gitid: securing transaction directory %s: %w", missing[i], err)
		}
	}
	return nil
}

// dir returns the snapshot watchDir recorded for path, if any.
func (j *mutationJournal) dir(path string) (gitDirSnapshot, bool) {
	clean := filepath.Clean(path)
	for _, snapshot := range j.dirs {
		if snapshot.path == clean {
			return snapshot, true
		}
	}
	return gitDirSnapshot{}, false
}

// ensureManagedDir hardens a root gitid itself owns (~/.ssh, the Git fragment
// directory, the SSH include directory) — CR-05. Unlike ensureDir, which is
// intentionally mode-neutral toward pre-existing paths (CR-01: a user-named
// gitdir must never be permission-mutated just for being referenced), a
// managed root gitid documents itself as securing must end up at mode even
// when it pre-existed the transaction (e.g. a stale `~/.ssh` at 0777).
//
// The prior mode of a pre-existing managed root is recorded in chmodDirs so
// restore() can revert exactly this chmod on rollback (WR-17) — freshly
// created roots are already covered by createdDirs' full removal and need no
// mode revert.
func (j *mutationJournal) ensureManagedDir(path string, mode os.FileMode) error {
	if err := j.ensureDir(path, mode); err != nil {
		return err
	}
	clean := filepath.Clean(path)
	for _, created := range j.createdDirs {
		if created == clean {
			// Freshly created by this transaction: ensureDir already
			// Mkdir+Chmod'd it at mode, and rollback removes it wholesale.
			return nil
		}
	}
	snapshot, ok := j.dir(clean)
	if !ok {
		return fmt.Errorf("gitid: internal: %s not watched before securing", clean)
	}
	if err := os.Chmod(clean, mode); err != nil {
		return fmt.Errorf("gitid: securing managed directory %s: %w", clean, err)
	}
	j.chmodDirs = append(j.chmodDirs, snapshot)
	return nil
}

func (j *mutationJournal) addBackup(path string) {
	if path != "" {
		j.backups = append(j.backups, path)
	}
}

func (j *mutationJournal) file(path string) (gitFileSnapshot, bool) {
	for _, snapshot := range j.files {
		if snapshot.path == path {
			return snapshot, true
		}
	}
	return gitFileSnapshot{}, false
}

func (j *mutationJournal) restore() ([]string, error) {
	// WR-01: every outcome line is user-facing (folded into ceremony
	// receipts and error messages), so it must read like the rest of the
	// UI — the `~/`-shortened form, never the real absolute sandbox path.
	var outcomes []string
	var failures []string
	// failedFilePaths (BL-15, was WR-25) tracks the RAW (undisplayed) path
	// of every file this loop failed to restore or remove — kept separate
	// from failures/outcomes (which are already display-formatted strings)
	// so the chmodDirs loop below can do an exact, unambiguous prefix match
	// against real filesystem paths rather than parsing formatted text.
	var failedFilePaths []string
	for i := len(j.files) - 1; i >= 0; i-- {
		s := j.files[i]
		var err error
		if j.b.failCommitAt != nil {
			err = j.b.failCommitAt("restore:" + s.path)
		}
		if err == nil && s.exists {
			err = filewriter.WriteNoBackup(s.path, s.content, s.mode)
		} else if err == nil {
			if removeErr := os.Remove(s.path); removeErr != nil && !os.IsNotExist(removeErr) {
				err = removeErr
			}
		}
		if err != nil {
			outcome := j.b.displayPath(s.path) + ": restoration failed: " + err.Error()
			outcomes = append(outcomes, outcome)
			failures = append(failures, outcome)
			failedFilePaths = append(failedFilePaths, filepath.Clean(s.path))
		} else {
			outcomes = append(outcomes, j.b.displayPath(s.path)+": restored")
		}
	}
	// createdFiles (review R3-01): remove every path a mid-transaction seam
	// CREATED — currently the D-06 archive copies recordCreatedFile
	// received — AFTER every watched file above has been restored. A
	// created path was never snapshotted (there was nothing to restore it
	// TO), so the only correct rollback action is removal, mirroring
	// createdDirs' own removal-only treatment below.
	for i := len(j.createdFiles) - 1; i >= 0; i-- {
		path := j.createdFiles[i]
		var err error
		if j.b.failCommitAt != nil {
			err = j.b.failCommitAt("restore:" + path)
		}
		if err == nil {
			if removeErr := os.Remove(path); removeErr != nil && !os.IsNotExist(removeErr) {
				err = removeErr
			}
		}
		if err != nil {
			outcome := j.b.displayPath(path) + ": restoration failed: " + err.Error()
			outcomes = append(outcomes, outcome)
			failures = append(failures, outcome)
			failedFilePaths = append(failedFilePaths, filepath.Clean(path))
		} else {
			outcomes = append(outcomes, j.b.displayPath(path)+": restored")
		}
	}
	// BL-15 (was WR-25, escalated from skip): a managed root must NEVER be
	// re-loosened to its pre-transaction (potentially group/world-readable)
	// mode when a file underneath it could not be restored/removed — e.g. a
	// freshly written private key at ~/.ssh/id_ed25519_<name> that failed to
	// delete would otherwise be left inside a directory rollback just
	// reverted from 0700 back to 0755/0777. under reports whether dirPath is
	// the exact path of, or an ancestor of, any file this transaction failed
	// to restore.
	under := func(dirPath string) bool {
		clean := filepath.Clean(dirPath)
		prefix := clean + string(os.PathSeparator)
		for _, f := range failedFilePaths {
			if f == clean || strings.HasPrefix(f, prefix) {
				return true
			}
		}
		return false
	}
	// WR-17: iterate chmodDirs, not the full dirs snapshot. dirs holds every
	// ancestor watchDir walked over while resolving a target path (including
	// HOME) whether or not this transaction ever changed its mode; reverting
	// all of them on every rollback rewrote permission metadata on
	// directories gitid never touched and could clobber a legitimate
	// concurrent permission change made between snapshot and rollback.
	// chmodDirs (populated only by ensureManagedDir, CR-05) holds exactly the
	// pre-existing managed roots this transaction actually chmod'd.
	for i := len(j.chmodDirs) - 1; i >= 0; i-- {
		s := j.chmodDirs[i]
		if under(s.path) {
			// BL-15: a file under this hardened root failed to restore —
			// keep the root at its SECURED mode rather than reverting to
			// its looser pre-transaction mode (s.mode; that is the mode
			// BEING avoided, not the current one — read the real on-disk
			// mode for the message instead of assuming what ensureManagedDir
			// set it to). This is deliberately NOT counted as a restoration
			// failure (nothing here failed to revert; the revert was
			// correctly skipped), so it does not widen the returned error's
			// failure set.
			current := "unknown"
			if info, statErr := os.Stat(s.path); statErr == nil {
				current = info.Mode().Perm().String()
			}
			outcomes = append(outcomes, j.b.displayPath(s.path)+
				": mode kept at "+current+" — a file under it could not be restored")
			continue
		}
		err := error(nil)
		if j.b.failCommitAt != nil {
			err = j.b.failCommitAt("restore:" + s.path)
		}
		if err == nil {
			err = os.Chmod(s.path, s.mode)
		}
		if err != nil {
			outcome := j.b.displayPath(s.path) + ": restoration failed: " + err.Error()
			outcomes = append(outcomes, outcome)
			failures = append(failures, outcome)
		} else {
			outcomes = append(outcomes, j.b.displayPath(s.path)+": restored")
		}
	}
	for i := len(j.createdDirs) - 1; i >= 0; i-- {
		path := j.createdDirs[i]
		err := error(nil)
		if j.b.failCommitAt != nil {
			err = j.b.failCommitAt("restore:" + path)
		}
		if err == nil {
			err = os.Remove(path)
		}
		if err != nil && !os.IsNotExist(err) {
			outcome := j.b.displayPath(path) + ": restoration failed: " + err.Error()
			outcomes = append(outcomes, outcome)
			failures = append(failures, outcome)
		} else {
			outcomes = append(outcomes, j.b.displayPath(path)+": restored")
		}
	}
	if len(failures) != 0 {
		return outcomes, fmt.Errorf("%s", strings.Join(failures, "; "))
	}
	return outcomes, nil
}

// commitGitArtifacts performs all confirmed Git mutations. pubLine is supplied
// by CommitCreate so the signing line is derived from the key already staged for
// that create; standalone writes read the requested public-key path instead.
func (b *realBackend) commitGitArtifacts(spec tuikit.GitSpec, pubLine string, transaction *mutationJournal) (backups, restored []string, err error) {
	if strings.TrimSpace(spec.Identity) == "" || strings.TrimSpace(spec.Name) == "" || strings.TrimSpace(spec.Email) == "" || strings.TrimSpace(spec.SSHHost) == "" {
		return nil, nil, fmt.Errorf("gitid: incomplete Git configuration")
	}
	if spec.Provider == "" || !strings.Contains(spec.Provider, ".") {
		spec.Provider = providerFromAlias(spec.SSHHost)
	}
	fragmentPath := filepath.Join(b.fragmentDir, spec.Identity)
	publicKeyPath := spec.PublicKeyPath
	if publicKeyPath == "" {
		publicKeyPath = spec.KeyPath + ".pub"
	}
	resolvedPublicKeyPath := b.resolveKeyPath(publicKeyPath)
	gitDirPath := ""
	if spec.Strategy != "hasconfig" {
		gitDirPath = b.resolveKeyPath(strings.TrimSpace(spec.GitDir))
		if gitDirPath == "" {
			gitDirPath = filepath.Join(b.home, "git", spec.Identity)
		}
	}
	journal := transaction
	ownsJournal := journal == nil
	if journal == nil {
		journal = newMutationJournal(b)
	}
	for _, path := range []string{fragmentPath, b.gitconfigPath, b.allowedSigners} {
		if err := journal.watchFile(path); err != nil {
			return nil, nil, err
		}
	}
	// WR-24: ~/.ssh (b.sshDir) is deliberately NOT watch-only here — it is
	// one of gitid's own managed roots (same class as b.fragmentDir), and
	// WriteAllowedSignersReplacing below writes into it. It gets the same
	// ensureManagedDir hardening treatment b.fragmentDir gets a few lines
	// down, applied just before the write that actually needs it (mirrors
	// createStagedKey's own ensureManagedDir(b.sshDir, ...) placement).
	// Leaving it watch-only (as before this fix) meant CR-05's "gitid's own
	// managed roots must end at the documented mode" held for CommitCreate
	// and not for this standalone Git-config path — and left the directory
	// uncreated, so a from-scratch home (no ~/.ssh at all) failed the whole
	// transaction at the LAST step, after the fragment and includeIf were
	// already written and had to be rolled back.
	if err := journal.watchDir(b.fragmentDir); err != nil {
		return nil, nil, err
	}
	if gitDirPath != "" {
		if err := journal.watchDir(gitDirPath); err != nil {
			return nil, nil, err
		}
	}
	if err := containedRegularPath(resolvedPublicKeyPath, b.home); err != nil {
		return nil, nil, err
	}
	if pubLine == "" {
		pub, readErr := os.ReadFile(resolvedPublicKeyPath) //nolint:gosec // contained and checked above
		if readErr != nil {
			return nil, nil, fmt.Errorf("gitid: reading signing key: %w", readErr)
		}
		pubLine = string(pub)
	}
	fail := func(step string, cause error) ([]string, []string, error) {
		cause = fmt.Errorf("gitid: mutation %s failed: %w", step, cause)
		if !ownsJournal {
			return journal.backups, nil, cause
		}
		outcomes, restoreErr := journal.restore()
		if restoreErr != nil {
			cause = fmt.Errorf("%w; restoration results: %s", cause, strings.Join(outcomes, "; "))
		}
		return journal.backups, outcomes, cause
	}
	inject := func(step string) error {
		if b.failCommitAt == nil {
			return nil
		}
		return b.failCommitAt(step)
	}
	if err := inject("git-fragment-dir"); err != nil {
		return fail("git-fragment-dir", err)
	}
	if err := journal.ensureManagedDir(b.fragmentDir, 0o700); err != nil {
		return fail("git-fragment-dir", err)
	}
	if gitDirPath != "" {
		if err := inject("gitdir"); err != nil {
			return fail("gitdir", err)
		}
		// CR-01: gitDirPath is the user-editable gitdir — never mode-mutated
		// as a side effect of being referenced, unlike gitid's own managed
		// roots above/below (CR-05: ensureManagedDir).
		if err := journal.ensureDir(gitDirPath, 0o700); err != nil {
			return fail("gitdir", err)
		}
	}
	if err := inject("git-fragment-backup"); err != nil {
		return fail("git-fragment-backup", err)
	}
	if snapshot, ok := journal.file(fragmentPath); ok && snapshot.exists {
		backup, writeErr := filewriter.Write(fragmentPath, snapshot.content, snapshot.mode)
		if writeErr != nil {
			return fail("git-fragment-backup", fmt.Errorf("gitid: backing up Git fragment: %w", writeErr))
		}
		journal.addBackup(backup)
	}
	if err := inject("git-fragment"); err != nil {
		return fail("git-fragment", err)
	}
	if writeErr := gitconfig.WriteFragment(fragmentPath, spec.Name, spec.Email, publicKeyPath, true); writeErr != nil {
		return fail("git-fragment", fmt.Errorf("gitid: writing Git fragment: %w", writeErr))
	}
	if err := inject("git-includeif"); err != nil {
		return fail("git-includeif", err)
	}
	backup, writeErr := gitconfig.WriteIncludeIf(b.gitconfigPath, spec.Identity, "~/.gitconfig.d/"+spec.Identity, matchesFor(spec))
	if writeErr != nil {
		return fail("git-includeif", fmt.Errorf("gitid: writing Git includeIf: %w", writeErr))
	}
	journal.addBackup(backup)
	if spec.ForceSSH && spec.Provider != "" {
		if err := inject("provider-rewrite"); err != nil {
			return fail("provider-rewrite", err)
		}
		backup, writeErr = gitconfig.WriteProviderRewrite(b.gitconfigPath, spec.Provider, true)
		if writeErr != nil {
			return fail("provider-rewrite", fmt.Errorf("gitid: writing provider rewrite for %q: %w", spec.Provider, writeErr))
		}
		journal.addBackup(backup)
	}
	// WR-06: no separate "take another backup of ~/.gitconfig just before
	// the raw `git config` mutation" step. That used to read the file back
	// and rewrite it byte-for-byte through filewriter.Write purely to mint
	// a THIRD .bak.<nanos> path — moments after WriteIncludeIf (and,
	// conditionally, WriteProviderRewrite) already backed the same file up.
	// The journal's in-memory pre-transaction snapshot
	// (journal.file(b.gitconfigPath)) is what actually drives rollback
	// (restore() never touches these timestamped files); the on-disk
	// backup from the FIRST mutation of this phase is enough of a durable,
	// user-visible recovery point. Skipping this step also removes an
	// extra non-atomic-window rewrite of the user's config for no new
	// information.
	if err := inject("allowed-signers-file"); err != nil {
		return fail("allowed-signers-file", err)
	}
	if writeErr := gitconfig.SetAllowedSignersFile(b.gitconfigPath, b.allowedSigners); writeErr != nil {
		return fail("allowed-signers-file", fmt.Errorf("gitid: setting allowed signers file: %w", writeErr))
	}
	// WR-24: harden (and, on a from-scratch home, CREATE) ~/.ssh right
	// before the write that needs it — the same ensureManagedDir CR-05 gave
	// CommitCreate's ~/.ssh handling, now applied on the standalone
	// Configure-Git path too.
	if err := inject("ssh-dir"); err != nil {
		return fail("ssh-dir", err)
	}
	if err := journal.ensureManagedDir(b.sshDir, sshDirMode); err != nil {
		return fail("ssh-dir", err)
	}
	if err := inject("allowed-signers"); err != nil {
		return fail("allowed-signers", err)
	}
	backup, writeErr = keygen.WriteAllowedSignersReplacing(b.allowedSigners, spec.Identity, spec.Email, pubLine)
	if writeErr != nil {
		return fail("allowed-signers", fmt.Errorf("gitid: writing allowed signers: %w", writeErr))
	}
	journal.addBackup(backup)
	return journal.backups, nil, nil
}

// expandTildeForHome expands a leading "~/" (or a bare "~") in path against
// home explicitly. Paths without a leading tilde are returned unchanged.
//
// Deliberately NOT internal/identity's own unexported expandTilde (which
// resolves against os.UserHomeDir()/$HOME): a caller here already owns an
// explicit, possibly-sandboxed home (b.home) and must never silently
// re-derive it from the process environment — the exact WR-35 hermeticity
// lesson (see accounts()'s doc comment) applied to path expansion instead of
// config reads.
func expandTildeForHome(path, home string) string {
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:])
	}
	return path
}

func containedRegularPath(path, root string) error {
	cleanRoot, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("gitid: resolving managed root: %w", err)
	}
	cleanPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("gitid: resolving managed path: %w", err)
	}
	if cleanPath != cleanRoot && !strings.HasPrefix(cleanPath, cleanRoot+string(os.PathSeparator)) {
		return fmt.Errorf("gitid: refusing path outside managed home: %s", path)
	}
	for current := cleanPath; current != cleanRoot; current = filepath.Dir(current) {
		info, statErr := os.Lstat(current)
		if statErr == nil && info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("gitid: refusing symlinked managed path: %s", current)
		}
		if statErr != nil && !os.IsNotExist(statErr) {
			return fmt.Errorf("gitid: stat managed path: %w", statErr)
		}
	}
	return nil
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
	keyExists := acct.KeyPath != "" && fileExists(expandTildeForHome(acct.KeyPath, b.home))
	state := identity.ClassifyState(acct, keyExists, keyExists && acct.Alias != "", acct.FragmentPath != "")
	row := tuikit.DemoIdentity{
		Name:            acct.Name,
		State:           string(state),
		SSHHost:         acct.Alias,
		KeyPath:         b.displayPath(acct.KeyPath),
		PublicKeyPath:   b.displayPath(acct.PubPath),
		GitFragmentPath: b.displayPath(acct.FragmentPath),
		SigningKeyPath:  b.displayPath(acct.SigningKeyPath),
		GitName:         acct.GitName,
		GitEmail:        acct.GitEmail,
		Provider:        acct.Provider,
		Hostname:        acct.Hostname,
		Port:            acct.Port,
		// CR-09: project the REAL on-disk provider-rewrite state, not a
		// default. identity.Reconstruct already populated acct.ForceSSH from
		// the actual ~/.gitconfig bytes (gitconfig.HasProviderRewrite) — this
		// is a straight passthrough, never a heuristic.
		ForceSSH: acct.ForceSSH,
	}
	for _, match := range acct.Matches {
		switch match.Kind {
		case gitconfig.MatchGitdir:
			row.GitDir = match.Value
		case gitconfig.MatchHasconfig:
			row.MatchStrategy = "hasconfig"
		}
	}
	if row.GitDir != "" {
		if row.MatchStrategy == "hasconfig" {
			row.MatchStrategy = "both"
		} else {
			row.MatchStrategy = "gitdir"
		}
	}
	if row.MatchStrategy == "" && row.GitFragmentPath != "" {
		row.MatchStrategy = "gitdir"
	}
	row.GitConfigured = row.GitFragmentPath != "" && row.GitName != "" && row.GitEmail != ""
	return row
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

// writeSSHBlock persists the managed Host block and normalises the gitid
// `Host *` globals block through sshconfig.EnsureGlobals into the resolved
// storage target, creating the Include'd layout first when this is a fresh
// machine's first write (D-06). globalsGOOS follows sshconfig.Write's
// contract: a non-empty platform means "normalise the globals block", the
// empty value means "do not touch it at all" (rotate/repair/update). Every
// write routes through internal/sshconfig, and therefore through the
// filewriter backup + atomic temp->rename->chmod chokepoint — never
// os.WriteFile.
func (b *realBackend) writeSSHBlock(accountName, hostBlock, globalsGOOS string) (string, error) {
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
	return sshconfig.Write(st.targetPath, accountName, hostBlock, globalsGOOS)
}

// ---------------------------------------------------------------------------
// Reads over the user's real configuration
// ---------------------------------------------------------------------------

// accounts reconstructs every identity from the user's configuration,
// Include-aware so a fresh D-06 machine's identities are visible.
//
// WR-35 (iteration 4): this MUST use identity.InventoryDepsForHome(b.home),
// never identity.BuildInventoryDeps() — the latter resolves home from
// os.UserHomeDir()/$HOME internally, silently ignoring b.home entirely. A
// reviewer's own CR-09 probe against newBackendForHome(t.TempDir()) proved
// this: without an accompanying t.Setenv("HOME", home), accounts() returned
// the REAL developer's identity read from their real ~/.ssh/config, despite
// newBackendForHome's doc comment claiming hermeticity. Threading b.home
// explicitly here closes that gap regardless of whether $HOME happens to
// also be set correctly elsewhere.
func (b *realBackend) accounts() []identity.Account {
	deps := identity.InventoryDepsForHome(b.home)
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

// normalizedAccounts returns b.accounts() with EVERY entry's tilde-prefixed
// artifact paths expanded against b.home via normalizeAccountForWrite — the
// consistent-comparison basis any cross-identity path equality check (D-12's
// SharedKeyOwners, D-09's ProviderRefCount) must use. b.accounts() alone
// returns paths verbatim from Reconstruct (often literal "~/.ssh/id_..."),
// while a caller comparing against ITS OWN already-normalized (absolute)
// acct.KeyPath — as every lifecycle function does, via
// normalizeAccountForWrite — would silently see every OTHER account's key
// path as never matching, since a literal tilde string is never equal to an
// absolute path. That mismatch made D-12's "this key is also used by
// <sibling>" detection unreachable for every recipe-shaped (tilde-path)
// identity, both in the confirm-screen PREVIEW (DeletePlan) and in the
// REAL delete decision (identity.Delete's own keySurvives gate) — found
// while seeding 05-09-PLAN.md's delete-everything shared-key PTY fixture.
func (b *realBackend) normalizedAccounts() []identity.Account {
	raw := b.accounts()
	out := make([]identity.Account, len(raw))
	for i, a := range raw {
		out[i] = b.normalizeAccountForWrite(a)
	}
	return out
}

// inventoryDeps is InventoryDepsForHome with Stat expanding "~/" against
// b.home, so recipe-shaped IdentityFile values classify against real files
// rather than always looking missing.
func (b *realBackend) inventoryDeps() identity.InventoryDeps {
	deps := identity.InventoryDepsForHome(b.home)
	deps.Stat = func(path string) (os.FileInfo, error) {
		return os.Stat(expandTildeForHome(path, b.home)) //nolint:gosec // trusted gitid-managed key path
	}
	return deps
}

// healthByName is the per-identity classification InitialState and the
// findings section share — one BuildInventory call, never a second policy.
func (b *realBackend) healthByName() map[string]identity.IdentityHealth {
	inv, err := identity.BuildInventory(b.inventoryDeps())
	if err != nil {
		return nil
	}
	out := make(map[string]identity.IdentityHealth, len(inv.Identities))
	for _, h := range inv.Identities {
		out[h.Name] = h
	}
	return out
}

// findAccount resolves ONE reconstructed account by name from the SAME
// b.accounts() list the identity list/CommitDelete/CLI delete all read —
// never a second, divergent lookup.
func (b *realBackend) findAccount(name string) (identity.Account, bool) {
	for _, a := range b.accounts() {
		if a.Name == name {
			return a, true
		}
	}
	return identity.Account{}, false
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
	// WR-05: normalizedAccounts() (not accounts()) — this map is looked up
	// with ABSOLUTE paths (toReusableKeyViews' owners[k.Path], where k.Path
	// comes from keygen.ScanReusableKeys' filepath.Glob results), while
	// accounts() returns KeyPath verbatim from Reconstruct (usually the
	// recipe-shaped tilde literal). Keying on the raw tilde path made every
	// recipe-shaped identity's key silently miss this lookup, so the reuse
	// picker showed a key already owned by another identity with an empty
	// InUseBy label — the exact safety warning D-12 exists to show BEFORE a
	// second identity is pointed at an existing key.
	for _, acct := range b.normalizedAccounts() {
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
// doctor.Deps — the Phase 8 Health/Fixer/CLI findings source
// ---------------------------------------------------------------------------

// buildDoctorDeps wires a real internal/doctor.Deps from home. It is the
// SHARED constructor realBackend.InitialState() (via doctorFindings) and
// `gitid health --json` (cmd/gitid/health.go) both call — one construction
// site, so the TUI Health/Fixer tabs and the CLI can never disagree about
// what doctor.Run(deps) sees (08-01-PLAN.md Task 1's "one source, three
// consumers" contract).
//
// Every DATA/path/read/fix-effect field is wired for real (the injected-seam
// "every field non-nil" rule from wiring.go's own top-of-file doc comment).
// The 9 CheckFn fields are the one deliberate exception during this plan's
// tracer wave: only CheckCoherence is wired here (Task 1's "prove one real
// finding" scope) — the other 8 are wired by Task 2, which replaces this
// function's CheckFn block in place. doctor.Run skips a nil CheckFn by
// design (doctor.go:286-288), so leaving them nil for now is not a bug.
//
// SetupBaseline (the Interactive `gitid baseline setup` fix for the
// baseline-missing finding) is left nil: no such command exists yet in this
// binary (it lived only in the retired POC). CheckBaseline's own fix
// fallback (AddWiring's "baseline-include:" case, wired below) still
// restores the bare [include] pointer when SetupBaseline is nil — see
// checks/baseline.go's own nil-guard.
func buildDoctorDeps(home string) doctor.Deps {
	sshConfigPath := filepath.Join(home, ".ssh", "config")
	gitconfigPath := filepath.Join(home, ".gitconfig")
	allowedSignersPath := filepath.Join(home, ".ssh", "allowed_signers")
	sshDir := filepath.Join(home, ".ssh")
	baselineFilePath := filepath.Join(home, ".gitconfig.d", "00-baseline")
	gitignorePath := filepath.Join(home, ".gitignore_global")

	// Reconstruct the SAME identity list InitialState()/accounts() reads
	// (identity.InventoryDepsForHome — WR-35 hermetic, never os.UserHomeDir()
	// /$HOME) so a doctor.Deps built from home never diverges from what the
	// Identity Manager itself shows for that home.
	invDeps := identity.InventoryDepsForHome(home)
	sshBytes, _ := invDeps.ReadSSHConfig()
	gcBytes, _ := invDeps.ReadGitconfig()
	accounts, _ := identity.Reconstruct(sshBytes, gcBytes, invDeps.ReadFragment)

	var keyPaths, pubKeyPaths []string
	for _, a := range accounts {
		if a.KeyPath != "" {
			keyPaths = append(keyPaths, a.KeyPath)
		}
		if a.PubPath != "" {
			pubKeyPaths = append(pubKeyPaths, a.PubPath)
		}
	}
	keyPaths = filterReservedDoctorKeyPaths(keyPaths, sshDir)

	managedHosts, _ := sshconfig.ParseManagedHosts(sshBytes)
	sshBlockNames := make([]string, 0, len(managedHosts))
	for name := range managedHosts {
		sshBlockNames = append(sshBlockNames, name)
	}

	gcBlocks := filewriter.ListBlocks(gcBytes)
	gcBlockNames := make([]string, 0, len(gcBlocks))
	for _, blk := range gcBlocks {
		gcBlockNames = append(gcBlockNames, blk.Name)
	}

	allSSHHostIDFiles := sshconfig.ParseAllHostIdentityFiles(sshBytes)
	allHostBlocks := sshconfig.ParseAllHostBlocks(sshBytes)
	gitConfigPaths := []string{gitconfigPath}
	if entries, err := os.ReadDir(filepath.Join(home, ".gitconfig.d")); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				gitConfigPaths = append(gitConfigPaths, filepath.Join(home, ".gitconfig.d", entry.Name()))
			}
		}
	}

	return doctor.Deps{
		// Read fields.
		ReadFile: func(path string) ([]byte, error) {
			return os.ReadFile(path) //nolint:gosec // path is a trusted gitid-managed path (G304)
		},
		Stat: func(path string) (os.FileInfo, error) {
			return os.Stat(expandTildeForHome(path, home)) //nolint:gosec // path is a trusted gitid-managed path (G304)
		},

		// Process fields.
		RunSSHAdd:               runDoctorSSHAdd,
		RunSSHKeygenFingerprint: runDoctorSSHKeygenFingerprint,
		RunGitConfigGet:         gitconfig.RunGitConfigGet,

		// Injected data and seams.
		GitVersionAtLeast: deps.GitVersionAtLeast,
		CurrentOS:         platform.CurrentOS,
		InstallHint:       platform.InstallHint,
		DetectTools:       deps.Detect,
		ReadBaselineState: gitconfig.ReadBaselineState,

		// Path fields.
		SSHDir:             sshDir,
		SSHConfigPath:      sshConfigPath,
		GitconfigPath:      gitconfigPath,
		AllowedSignersPath: allowedSignersPath,
		BaselineFilePath:   baselineFilePath,
		GitignorePath:      gitignorePath,
		GitConfigPaths:     gitConfigPaths,

		KeyPaths:    keyPaths,
		PubKeyPaths: pubKeyPaths,

		Identities:                 accounts,
		ManagedHosts:               managedHosts,
		SSHManagedBlockNames:       sshBlockNames,
		GitconfigManagedBlockNames: gcBlockNames,
		AllSSHHostIdentityFiles:    allSSHHostIDFiles,
		AllHostBlocks:              allHostBlocks,
		GlobalSSHShadowCheck:       buildGlobalSSHShadowCheck(home, sshConfigPath),
		AuthorResolutionCheck:      buildAuthorResolutionCheck(home, gitconfigPath),

		// Fix fields (D-01: doctor core never calls os.Chmod/filewriter
		// directly — every mutation is injected from here).
		FixPerm: func(path string, mode os.FileMode) error {
			return os.Chmod(expandTildeForHome(path, home), mode) //nolint:gosec // chmod to a caller-supplied tighten-only mode (G306)
		},
		RemoveBlock: func(path, name string) error {
			content, rerr := os.ReadFile(path) //nolint:gosec // path is a gitid-managed trusted path (G304)
			if rerr != nil && !os.IsNotExist(rerr) {
				return fmt.Errorf("doctor: reading %s for block removal: %w", path, rerr)
			}
			removed := filewriter.RemoveBlock(content, name)
			mode := os.FileMode(0o600)
			if path == allowedSignersPath {
				mode = 0o644
			}
			if _, werr := filewriter.Write(path, removed, mode); werr != nil {
				return fmt.Errorf("doctor: removing block %q from %s: %w", name, path, werr)
			}
			return nil
		},
		AddWiring:       doctorAddWiring(allowedSignersPath),
		FixExcludesfile: fixExcludesfile(baselineFilePath),

		// Check function fields — all 9 families wired to their real
		// internal/doctor/checks function (08-01-PLAN.md Task 2; Task 1
		// proved only CheckCoherence, the tracer's "one real finding").
		CheckDeps:       checks.CheckDeps,
		CheckPerms:      checks.CheckPermissions,
		CheckCoherence:  checks.CheckCoherence,
		CheckOrphans:    checks.CheckOrphans,
		CheckSigning:    checks.CheckSigning,
		CheckAgent:      checks.CheckAgent,
		CheckBaseline:   checks.CheckBaseline,
		CheckOverlap:    checks.CheckOverlap,
		CheckRedundancy: checks.CheckRedundancy,
		CheckFiles:      checks.CheckFiles,
	}
}

// buildGlobalSSHShadowCheck returns the doctor.Deps.GlobalSSHShadowCheck
// closure: it resolves where gitid's global "Host *" managed block actually
// lives on THIS machine (in-file or the Include'd config.d/gitid.config,
// auto-detected the SAME way (*realBackend).storage() detects it —
// sshconfig.Adopt with AdoptSentinelBearing, falling back to the canonical
// config.d path — a read-only doctor check has no need for storage()'s
// write-time needsIncludeLine/includeLayout fields), collects the
// non-per-alias policy keys gitid has actually WRITTEN into that block (a
// key gitid never applied is "not configured", never "shadowed"), and runs
// internal/globalssh.Verify (Phase 6's own D-04 post-write probe, reused
// unchanged) against those keys.
func buildGlobalSSHShadowCheck(home, sshConfigPath string) func() globalssh.ShadowResult {
	return func() globalssh.ShadowResult {
		targetPath := resolveGlobalSSHTargetPath(home, sshConfigPath)
		keys := appliedGlobalSSHKeys(targetPath)
		if len(keys) == 0 {
			return globalssh.ShadowResult{}
		}
		return globalssh.Verify(globalssh.BuildProbeDeps(sshConfigPath), keys)
	}
}

// resolveGlobalSSHTargetPath locates the file gitid's global "Host *"
// managed block currently lives in, mirroring (*realBackend).storage()'s own
// detection order (steps 1-2 only — an adopted sentinel-bearing target, then
// the canonical config.d/gitid.config path): a doctor check only needs to
// know where to LOOK for the block, never storage()'s write-time layout
// decision for a machine that has none yet.
func resolveGlobalSSHTargetPath(home, sshConfigPath string) string {
	if adopted, err := sshconfig.Adopt(sshConfigPath, sshconfig.AdoptSentinelBearing, "", sshconfig.RealAdoptDeps()); err == nil && adopted.TargetPath != "" {
		return adopted.TargetPath
	}
	canonical := filepath.Join(home, ".ssh", "config.d", gitidConfigFileName)
	if fileExists(canonical) {
		return canonical
	}
	return sshConfigPath
}

// appliedGlobalSSHKeys returns every non-per-alias globalssh.Policy key that
// is currently WRITTEN into gitid's "global-ssh" (or the pre-D-08 "_global")
// managed block at targetPath. This is a thin selector over the block's own
// raw lines — PolicyFor/Verify remain the sole source of truth for
// recommendation values and shadow-detection logic; this only decides which
// keys are worth asking them about.
func appliedGlobalSSHKeys(targetPath string) []string {
	content, err := os.ReadFile(targetPath) //nolint:gosec // targetPath is a trusted gitid-managed path (G304)
	if err != nil {
		return nil
	}
	var body string
	for _, b := range filewriter.ListBlocks(content) {
		if b.Name == sshconfig.GlobalBlockName || b.Name == sshconfig.LegacyGlobalBlockName {
			body = b.Body
			break
		}
	}
	if body == "" {
		return nil
	}
	present := make(map[string]bool)
	for _, line := range strings.Split(body, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) >= 1 {
			present[strings.ToLower(fields[0])] = true
		}
	}
	var keys []string
	for _, p := range globalssh.Policy {
		if p.Scope != "global" {
			continue
		}
		if present[strings.ToLower(p.Key)] {
			keys = append(keys, p.Key)
		}
	}
	return keys
}

// buildAuthorResolutionCheck returns the doctor.Deps.AuthorResolutionCheck
// closure: for one identity name, it looks up that identity's own includeIf
// record (internal/gitconfig.ParseManagedIncludeIf), resolves a real,
// currently-existing directory matching its gitdir: pattern (findGitWorkTree,
// the SAME helper cmd/gitid/lifecycle.go's fallbackMatchedDir already uses),
// picks a representative unmatched directory (the SAME b.home /
// b.fragmentDir fallback cmd/gitid/lifecycle.go's
// appendFallbackAuthorAdvisories already uses, reproduced here because
// buildDoctorDeps is a free function with no *realBackend receiver), and
// runs internal/globalgit.VerifyAuthorResolution (Phase 7's own D-06
// post-write probe, reused unchanged).
func buildAuthorResolutionCheck(home, gitconfigPath string) func(identityName string) (globalgit.AuthorResolution, bool, error) {
	fragmentDir := filepath.Join(home, ".gitconfig.d")
	return func(identityName string) (globalgit.AuthorResolution, bool, error) {
		content, err := os.ReadFile(gitconfigPath) //nolint:gosec // gitconfigPath is a trusted gitid-managed path (G304)
		if err != nil {
			return globalgit.AuthorResolution{}, false, nil //nolint:nilerr // unreadable gitconfig -- graceful no-finding, matches MatchedNotVerifiable
		}
		info, ok := gitconfig.ParseManagedIncludeIf(content)[identityName]
		if !ok {
			return globalgit.AuthorResolution{}, false, nil
		}
		var matchedDir string
		for _, m := range info.Matches {
			if m.Kind != gitconfig.MatchGitdir {
				continue
			}
			if found := findGitWorkTree(expandTildeForHome(m.Value, home)); found != "" {
				matchedDir = found
				break
			}
		}
		if matchedDir == "" {
			return globalgit.AuthorResolution{}, false, nil
		}
		unmatchedDir := home
		if _, err := os.Stat(fragmentDir); err == nil {
			unmatchedDir = fragmentDir
		}
		res, err := globalgit.VerifyAuthorResolution(globalgit.BuildProbeDeps(unmatchedDir), matchedDir, unmatchedDir)
		if err != nil {
			return globalgit.AuthorResolution{}, false, nil //nolint:nilerr // probe failure -- graceful no-finding, never a false positive
		}
		return res, res.MatchedOutcome == globalgit.MatchedVerified, nil
	}
}

// fixExcludesfile writes BOTH halves of the gitignore pair: the managed
// pattern file (via gitconfig.WriteGlobalGitignore) AND the core.excludesfile
// key — patched directly into the EXISTING "baseline" managed block's body
// inside baselineFilePath, never as a bare `git config --file gitconfigPath
// core.excludesfile ...` write. Two independently-verified real bugs drove
// this shape (found empirically, not assumed):
//
//  1. `git config --file <path> <key>` does NOT follow [include] directives
//     when READING — CheckBaseline's gitignore-pair check correctly reads
//     core.excludesfile from state.BaselineKeys (parsed from the baseline
//     FRAGMENT's own block body by gitconfig.ReadBaselineState), never from
//     gitconfigPath. A fix that WRITES to gitconfigPath while the check
//     READS from baselineFilePath can never converge — the check would keep
//     reporting the same finding.
//  2. `git config --file <path> --set` on ANY file appends a plain,
//     UNMANAGED directive outside any sentinel block. Even writing to
//     baselineFilePath directly this way would land the key OUTSIDE the
//     "# BEGIN gitid managed: baseline" ... "# END" markers — invisible to
//     gitconfig.ParseBlockKeys, which only reads the block's own body — so
//     the same non-convergence would recur one file over.
//
// The fix therefore patches the block's body in place (inserting or
// replacing the "excludesfile" line under [core], preserving every other
// line — including the user's own Tier-2 choices like autocrlf/pager/
// init.defaultBranch and any [alias]/[merge] sections — byte-for-byte) and
// writes it back through filewriter.ReplaceBlock + filewriter.Write, the
// SAME chokepoint every other managed-block mutation in this codebase uses.
// fixExcludesfile returns a doctor fix that seeds or patches the managed
// global gitignore and wires core.excludesfile in the baseline fragment.
//
// Shape guarding: Both the gitignore target and baseline fragment are inspected
// for healthy structure (one complete managed block, no orphans or duplicates)
// BEFORE the first mutation. The baseline body comes from the inspector, not a
// first-match ListBlocks scan, so the fix always patches the same block the
// doctor finding was classified from (the read and write paths are made to mean
// the same bytes).
//
// User-owned content: The managed gitignore block is now user-editable (GIGN-01).
// If the block exists and is populated, it is preserved byte-for-byte; only an
// empty block is seeded with the curated defaults. This prevents the destructive
// false-positive loop where a fix re-seeds a block the user has already reviewed
// and edited.
//
// Error recovery: Both writes are recoverable. If the second write fails after
// the first succeeded, the first is undone. For a pre-existing file, this means
// restoring from the timestamped backup; for a file this fix created, it means
// removal (so the home returns to its genuine pre-fix state).
func fixExcludesfile(baselineFilePath string) func(path string) error {
	return func(path string) error {
		// Shape-guard both files BEFORE any mutation.
		gitignoreBytes, _ := os.ReadFile(path) //nolint:gosec // path is the managed gitignore target (G304)
		gitignoreShape, err := gitconfig.InspectGitignoreFile(gitignoreBytes)
		if err != nil {
			return fmt.Errorf("doctor: inspecting %s: %w", path, err)
		}

		if err := filewriter.EnsureDir(filepath.Dir(baselineFilePath), 0o700); err != nil {
			return fmt.Errorf("doctor: ensuring %s: %w", filepath.Dir(baselineFilePath), err)
		}
		baselineBytes, _ := os.ReadFile(baselineFilePath) //nolint:gosec // baselineFilePath is a trusted gitid-managed path (G304)
		baselineShape, err := gitconfig.InspectManagedBlockFile(baselineBytes, "baseline")
		if err != nil {
			return fmt.Errorf("doctor: inspecting %s: %w", baselineFilePath, err)
		}

		// Gitignore write: seed with defaults only if the block is empty/absent.
		// If the block exists and is populated, preserve it byte-for-byte.
		gitignorePreexisted := gitignoreBytes != nil
		if !gitignoreShape.Managed {
			if _, err := gitconfig.WriteGlobalGitignore(path, gitconfig.DefaultGitignorePatterns()); err != nil {
				return fmt.Errorf("doctor: writing global gitignore: %w", err)
			}
		}

		// Re-read baseline after gitignore write to ensure a fresh read before patch.
		baselineBytes, _ = os.ReadFile(baselineFilePath) //nolint:gosec // trusted path (G304)
		if err := filewriter.EnsureDir(filepath.Dir(baselineFilePath), 0o700); err != nil {
			return fmt.Errorf("doctor: ensuring %s: %w", filepath.Dir(baselineFilePath), err)
		}
		// Re-inspect to get the fresh body from the inspector (not a first-match scan).
		baselineShape, err = gitconfig.InspectManagedBlockFile(baselineBytes, "baseline")
		if err != nil {
			// Baseline shape guard AFTER gitignore write. If the gitignore was just created
			// and the baseline inspection fails, roll back the gitignore.
			if !gitignorePreexisted {
				_ = os.Remove(path) //nolint:errcheck,gosec // best-effort removal; don't mask baseline error
			}
			return fmt.Errorf("doctor: re-inspecting %s after gitignore write: %w", baselineFilePath, err)
		}

		newBody := patchExcludesfileInBaselineBody(baselineShape.Body, path)
		composed := filewriter.ReplaceBlock(baselineBytes, "baseline", newBody)

		_, werr := filewriter.Write(baselineFilePath, composed, deleteGitconfigMode)
		if werr != nil {
			// Second write failed. Undo the first write (gitignore).
			if !gitignorePreexisted {
				// The gitignore was created by this fix; remove it (best-effort).
				_ = os.Remove(path) //nolint:errcheck,gosec // best-effort removal; don't mask write error
			} else {
				// The gitignore pre-existed; it should have been backed up by the first Write.
				// However, WriteGlobalGitignore only produces a backup if the file pre-existed
				// and was modified. If it was byte-identical, no backup is returned.
				// For safety, re-read and attempt restoration (though the exact state is lost
				// if the backup wasn't made).
				if origBytes, err := os.ReadFile(path); err == nil { //nolint:gosec // trusted path (G304)
					_ = os.WriteFile(path, origBytes, 0o644) //nolint:gosec,errcheck // trusted path; best-effort restore (G304)
				}
			}
			return fmt.Errorf("doctor: setting core.excludesfile in %s: %w", baselineFilePath, werr)
		}
		return nil
	}
}

// patchExcludesfileInBaselineBody returns body with its "excludesfile" line
// (under [core]) set to path — replacing an existing line's value if found,
// otherwise inserting a new line immediately after the [core] section header
// (matching gitconfig.RenderBaselineBlock's own Tier-1 key ordering:
// ignorecase, then excludesfile). Every other line is preserved verbatim. If
// body has no [core] section at all (should not happen for a gitid-authored
// block, per RenderBaselineBlock's own unconditional Tier-1 guarantee — but
// handled defensively rather than silently dropping the key), a fresh
// [core] section is prepended.
func patchExcludesfileInBaselineBody(body, path string) string {
	lines := strings.Split(body, "\n")
	inCore := false
	coreLineIdx := -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") {
			inCore = trimmed == "[core]"
			if inCore {
				coreLineIdx = i
			}
			continue
		}
		if inCore && strings.HasPrefix(strings.ToLower(strings.TrimSpace(trimmed)), "excludesfile") {
			lines[i] = "\texcludesfile = " + path
			return strings.Join(lines, "\n")
		}
	}
	if coreLineIdx >= 0 {
		insertAt := coreLineIdx + 1
		lines = append(lines[:insertAt], append([]string{"\texcludesfile = " + path}, lines[insertAt:]...)...)
		return strings.Join(lines, "\n")
	}
	// No [core] section found — prepend one.
	return "[core]\n\texcludesfile = " + path + "\n" + body
}

func filterReservedDoctorKeyPaths(keyPaths []string, sshDir string) []string {
	filtered := make([]string, 0, len(keyPaths))
	for _, path := range keyPaths {
		if sshconfig.IsReservedPath(sshDir, path) {
			continue
		}
		filtered = append(filtered, path)
	}
	return filtered
}

// runDoctorSSHAdd runs `ssh-add -l` via arg-slice exec (no shell, G204-clean)
// and returns the combined output and the exit code. A non-ExitError exec
// failure (binary not found, permission error) returns ("", 2) so
// classifyAgentState treats it as unreachable.
func runDoctorSSHAdd() (string, int) {
	cmd := exec.Command("ssh-add", "-l") //nolint:gosec // arg-slice form, no shell; fixed args (G204)
	out, err := cmd.CombinedOutput()
	output := string(out)
	if err == nil {
		return output, 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return output, exitErr.ExitCode()
	}
	return "", 2
}

// runDoctorSSHKeygenFingerprint runs `ssh-keygen -lf <path>` via arg-slice
// exec (no shell, G204-clean) and returns the first output line and any
// error. path is a gitid-managed .pub path (G304-annotated).
func runDoctorSSHKeygenFingerprint(path string) (string, error) {
	cmd := exec.Command("ssh-keygen", "-lf", path) //nolint:gosec // arg-slice form, no shell; path is trusted gitid-managed .pub (G204/G304)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	line := strings.SplitN(string(out), "\n", 2)[0]
	return line, nil
}

// doctorAddWiring returns the AddWiring dispatcher, closing over
// allowedSignersPath so the "signers:" case's target-file identity is
// available without threading it through the line payload. It dispatches to
// the correct existing writer for the finding being fixed, per the `line`
// payload's prefix:
//
//   - "ssh-host:<alias>:<hostname>:<port>:<keyPath>"   — re-add a Host block
//     (with IdentitiesOnly yes) via sshconfig.Write.
//   - "signers:<email>:<pubLine>"                       — re-add an
//     allowed_signers entry via keygen.WriteAllowedSigners.
//   - "baseline-include:<baselineFilePath>"              — restore the
//     baseline [include] block via gitconfig.WriteBaselineInclude.
//
// Every sub-path delegates to an existing writer that routes through
// internal/filewriter (backup + atomic write) — this function never calls
// os.WriteFile directly (CLAUDE.md).
func doctorAddWiring(allowedSignersPath string) func(path, name, line string) error {
	return func(path, name, line string) error {
		switch {
		case strings.HasPrefix(line, "ssh-host:"):
			rest := strings.TrimPrefix(line, "ssh-host:")
			parts := strings.SplitN(rest, ":", 4)
			if len(parts) != 4 {
				return fmt.Errorf("doctor: AddWiring ssh-host: malformed line %q", line)
			}
			alias, hostname, portStr, identityFile := parts[0], parts[1], parts[2], parts[3]
			port := 22
			if portStr != "" {
				if _, serr := fmt.Sscanf(portStr, "%d", &port); serr != nil {
					port = 22
				}
			}
			hostBlock := sshconfig.RenderHostBlock(alias, hostname, port, identityFile, "")
			if _, werr := sshconfig.Write(path, name, hostBlock, platform.CurrentOS()); werr != nil {
				return fmt.Errorf("doctor: AddWiring ssh-host for %q: %w", name, werr)
			}
		case strings.HasPrefix(line, "signers:"):
			rest := strings.TrimPrefix(line, "signers:")
			parts := strings.SplitN(rest, ":", 2)
			if len(parts) != 2 {
				return fmt.Errorf("doctor: AddWiring signers: malformed line %q", line)
			}
			email, pubLine := parts[0], parts[1]
			signerLine, lerr := keygen.AllowedSignersLine(email, pubLine)
			if lerr != nil {
				return fmt.Errorf("doctor: AddWiring signers for %q: %w", name, lerr)
			}
			if _, werr := keygen.WriteAllowedSigners(allowedSignersPath, name, signerLine); werr != nil {
				return fmt.Errorf("doctor: AddWiring signers for %q: %w", name, werr)
			}
		case strings.HasPrefix(line, "baseline-include:"):
			target := strings.TrimPrefix(line, "baseline-include:")
			if _, werr := gitconfig.WriteBaselineInclude(path, target); werr != nil {
				return fmt.Errorf("doctor: AddWiring baseline-include: %w", werr)
			}
		default:
			return fmt.Errorf("doctor: AddWiring: unknown wiring type in line %q", line)
		}
		return nil
	}
}

// doctorFindings converts internal/doctor.Run(deps)'s output into
// []tuikit.DemoFinding. This is the D-01/08-01-PLAN.md Task 1 architecture
// decision's ONLY conversion site: internal/doctor.Run(deps) is the SOLE
// findings source for both TUI tabs (Health, Fixer) and `gitid health
// --json` — identity.BuildInventory's Problem taxonomy is an INPUT to
// doctor.Deps (via Identities/ManagedHosts/KeyPaths above), never a second,
// independently-rendered output. This function — and any Family/Severity
// mapping helper it uses — MUST live in cmd/gitid only: internal/tuikit
// continues importing ZERO internal/doctor identifiers (the no-backend-import
// gate). Do not reintroduce a second findings source in internal/tuikit.
func doctorFindings(home string) []tuikit.DemoFinding {
	_, converted := runDoctorAndConvert(buildDoctorDeps(home))
	return converted
}

// findingStableID computes the D-01 stable ID for a doctor.Finding against
// the seen-occurrence map — the ONE ID scheme every consumer (the TUI,
// gitid health --json, and the fix-apply path's raw<->converted lookup)
// must agree on, extracted once so it can never drift between callers.
func findingStableID(f doctor.Finding, seen map[string]int) string {
	id := string(f.Family) + "|" + f.Title
	if f.IdentityName != "" {
		id += "|" + f.IdentityName
	}
	// Two distinct findings can share Family+Title+IdentityName (e.g. two
	// separate global Redundancy findings, IdentityName empty on both) —
	// disambiguate with an occurrence counter rather than collapse them
	// onto the same ID (08-01-PLAN.md Task 1).
	seen[id]++
	if n := seen[id]; n > 1 {
		id += fmt.Sprintf("|%d", n)
	}
	return id
}

// runDoctorAndConvert runs doctor.Run(deps) once and returns BOTH the raw
// findings (carrying their Fix descriptors, for the apply path) and their
// tuikit.DemoFinding conversion (for every render/JSON consumer) — index-
// aligned by construction, so a converted finding's ID always resolves back
// to its raw counterpart via the SAME stable-ID scheme (08-02-PLAN.md Task 2:
// realBackend.Persist's FixFinding case needs the raw Fix.Fn/Fix.Interactive
// a tuikit.DemoFinding cannot carry, since internal/tuikit imports zero
// internal/doctor identifiers).
func runDoctorAndConvert(deps doctor.Deps) (raw []doctor.Finding, converted []tuikit.DemoFinding) {
	raw = doctor.Run(deps)
	converted = make([]tuikit.DemoFinding, 0, len(raw))
	seen := make(map[string]int, len(raw))
	for _, f := range raw {
		id := findingStableID(f, seen)
		var rewrite *tuikit.FixRewriteTarget
		if f.Rewrite != nil {
			rewrite = &tuikit.FixRewriteTarget{
				HostPattern: f.Rewrite.HostPattern,
				Directive:   f.Rewrite.Directive,
				NewValue:    f.Rewrite.NewValue,
			}
		}
		var parseError *tuikit.ParseErrorView
		if f.ParseError != nil {
			parseError = &tuikit.ParseErrorView{File: f.ParseError.File, Raw: f.ParseError.Raw, Snippet: f.ParseError.Snippet}
		}
		converted = append(converted, tuikit.DemoFinding{
			HealthFinding: tuikit.HealthFinding{
				ID:           id,
				Section:      f.Target,
				Family:       string(f.Family),
				Title:        f.Title,
				Explanation:  f.Explanation,
				SuggestedFix: f.SuggestedFix,
				Severity:     tuikit.HealthSeverity(f.Severity.String()),
				Fixable:      f.Fix != nil,
			},
			Identity:   f.IdentityName,
			Rewrite:    rewrite,
			ParseError: parseError,
		})
	}
	return raw, converted
}

// persistFixFinding implements realBackend.Persist's FixFinding case
// (08-02-PLAN.md Task 2/3): locate the raw doctor.Finding matching
// action.ID (built fresh, since a stale Findings slice must never drive a
// write), call its Fix.Fn, then — REGARDLESS of which check produced the
// finding, not only the D-09 flagship — re-run doctor.Run(deps) in full
// (D-13) and replace state.Findings with the fresh conversion. If the fixed
// finding's stable ID is STILL present after the re-run, replace it with a
// D-14 convergence-alarm finding and remember that ID so it is never
// offered as fixable again this session.
func (b *realBackend) persistFixFinding(action tuikit.FixFinding) tuikit.DemoState {
	deps := buildDoctorDeps(b.home)
	raw, converted := runDoctorAndConvert(deps)

	b.convergenceAlarmedMu.Lock()
	alreadyWithdrawn := b.convergenceAlarmed[action.ID]
	b.convergenceAlarmedMu.Unlock()
	if alreadyWithdrawn {
		// D-14: a withdrawn fix is never re-offered, even once conditions on
		// disk would let the real Fn genuinely succeed this time — the alarm
		// substitution below (driven by convergenceAlarmed) still applies.
		b.setPersistErr(nil)
		return b.stateWithFreshFindings(applyConvergenceAlarms(b, converted))
	}

	var target *doctor.Finding
	for i := range raw {
		if converted[i].ID == action.ID {
			target = &raw[i]
			break
		}
	}
	if target == nil || target.Fix == nil || target.Fix.Fn == nil {
		// The finding is already gone, or carries no direct Fn (an
		// Interactive-only fix is a CLI concern, Task 3) — nothing to apply;
		// return the current converged state rather than erroring the TUI.
		b.setPersistErr(nil)
		return b.stateWithFreshFindings(converted)
	}

	fixFn := target.Fix.Fn
	if b.fixFnOverride != nil {
		fixFn = b.fixFnOverride
	}
	fixErr := fixFn()
	b.setPersistErr(fixErr)

	// D-13: re-run the full scan against a FRESHLY BUILT Deps — buildDoctorDeps
	// reads every config file EAGERLY (sshBytes/gcBytes are captured once, not
	// lazily), so reusing the pre-fix `deps` here would re-scan the SAME
	// stale bytes the fix already changed on disk, silently defeating the
	// entire point of a re-run. A failed Fn may still have partially mutated
	// disk state, and a local prune of the previous Findings slice must
	// never substitute for a real re-scan either.
	_, rescanned := runDoctorAndConvert(buildDoctorDeps(b.home))

	fixedID := action.ID
	stillPresent := false
	for i := range rescanned {
		if rescanned[i].ID == fixedID {
			stillPresent = true
			break
		}
	}
	if fixErr == nil && stillPresent {
		// D-14: the fix reported success but the SAME finding survived the
		// re-scan — replace it with the alarm and withdraw it from re-offer.
		b.convergenceAlarmedMu.Lock()
		if b.convergenceAlarmed == nil {
			b.convergenceAlarmed = make(map[string]bool)
		}
		b.convergenceAlarmed[fixedID] = true
		b.convergenceAlarmedMu.Unlock()
	}

	return b.stateWithFreshFindings(applyConvergenceAlarms(b, rescanned))
}

// applyConvergenceAlarms replaces every finding in findings whose ID is in
// b.convergenceAlarmed with the D-14 alarm shape (error severity, empty
// SuggestedFix so it renders as an unfixable row — the copy contract
// 08-UI-SPEC.md pins). Shared by persistFixFinding's early-withdrawal
// return and its main post-fix path, so the alarm rendering can never drift
// between the two.
func applyConvergenceAlarms(b *realBackend, findings []tuikit.DemoFinding) []tuikit.DemoFinding {
	out := make([]tuikit.DemoFinding, 0, len(findings))
	for _, f := range findings {
		b.convergenceAlarmedMu.Lock()
		alarmed := b.convergenceAlarmed[f.ID]
		b.convergenceAlarmedMu.Unlock()
		if alarmed {
			out = append(out, tuikit.DemoFinding{
				HealthFinding: tuikit.HealthFinding{
					ID:      f.ID,
					Section: f.Section,
					Family:  f.Family,
					Title:   f.Title,
					Explanation: fmt.Sprintf(
						"%s did not resolve after its own fix reported success -- this fix has been withdrawn from the Fixer; re-run Health after investigating manually.",
						f.Title),
					Severity: tuikit.SeverityError,
					// SuggestedFix left empty so this renders as an unfixable
					// row (D-14's copy contract).
				},
				Identity: f.Identity,
			})
			continue
		}
		out = append(out, f)
	}
	return out
}

// stateWithFreshFindings returns InitialState() with its Findings replaced
// by findings — the shared tail every persistFixFinding return path uses so
// Scanned/Identities/every other DemoState field stays freshly re-derived
// from disk, never a stale in-memory copy.
func (b *realBackend) stateWithFreshFindings(findings []tuikit.DemoFinding) tuikit.DemoState {
	state := b.InitialState()
	state.Findings = findings
	state.Scanned = true
	return state
}

// ---------------------------------------------------------------------------
// In-flight create state
// ---------------------------------------------------------------------------

// createInput builds the CreateInput for a committed create from the view row
// the wizard produced, filling the gitid-managed target paths and the globals
// PLATFORM (D-08 semantics, D-06 single owner): create passes
// platform.CurrentOS() so sshconfig.EnsureGlobals normalises the `Host *`
// block for the actual machine rather than re-rendering a body here.
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
		GlobalsGOOS:        platform.CurrentOS(),
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
			return tuikit.WizardCommitMsg{Err: b.displayMessage(b.initErr.Error())}
		}
		in := b.createInput(id)
		if !b.storeUnlockedFor(in) {
			return tuikit.WizardCommitMsg{Err: fmt.Sprintf(
				"gitid: refusing to write %q: the connectivity test failed or is stale, so nothing was proven about the provider", id.Name)}
		}
		staged, err := b.stagedKeyFor(in, id.ReuseKeyPath)
		if err != nil {
			return tuikit.WizardCommitMsg{Err: b.displayMessage(err.Error())}
		}
		backups, err := b.commitCreateTransaction(in, staged, id)
		// WR-21: map through displayPath and surface on BOTH outcomes — the
		// same way CommitGit does (:749-754) — so a failed create's
		// retained timestamped backups are as discoverable as a successful
		// one's, instead of being mentionable only inside the error string.
		displayBackups := make([]string, len(backups))
		for i, backup := range backups {
			displayBackups[i] = b.displayPath(backup)
		}
		if err != nil {
			// WR-23: commitCreateTransaction's own message already maps its
			// displayed BACKUP paths through displayPath (see fail(), above),
			// but the leading `gitid: mutation %s failed: %v` wraps errors
			// from gitConfigSet/filewriter/sshconfig that embed raw absolute
			// paths inline in their own text — scrub the whole message here.
			return tuikit.WizardCommitMsg{Backups: displayBackups, Err: b.displayMessage(err.Error())}
		}
		b.clearStaged()
		b.clearOutcomes()
		b.setPersistErr(nil)
		return tuikit.WizardCommitMsg{Backups: displayBackups}
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
//
// WR-03: this superseded a commitCreateTransactionLegacy that duplicated the
// same rollback semantics with the OLD os.Rename(backup, target) model —
// kept alive only by a `//nolint:unused` + package-level `var _` reference,
// which let it silently rot out of sync with the shared mutationJournal.
// Deleted; nothing behaved differently once the sole caller (CommitCreate)
// was confirmed to already call this function, not the legacy one.
//
// WR-04: txMu serializes this against any concurrent commitGitTransaction
// (both run off the Bubble Tea update loop, each in its own goroutine, and
// both read-modify-write ~/.gitconfig and ~/.ssh/allowed_signers).
func (b *realBackend) commitCreateTransaction(in identity.CreateInput, staged identity.StagedKey, id tuikit.DemoIdentity) ([]string, error) {
	b.txMu.Lock()
	defer b.txMu.Unlock()
	journal := newMutationJournal(b)
	fail := func(target string, cause error) ([]string, error) {
		// CR-02: never delete the timestamped backups on the failure path —
		// this mirrors commitGitArtifacts.fail's existing (correct)
		// behavior and mutationJournal's own contract ("backups remain
		// durable safety artifacts"). Deleting them here, especially when
		// restore() itself failed, destroys the only durable recovery copy
		// alongside a half-written file.
		outcomes, restoreErr := journal.restore()
		message := fmt.Sprintf("gitid: mutation %s failed: %v; restoration results: %s", target, cause, strings.Join(outcomes, "; "))
		if restoreErr != nil {
			// Restoration itself failed: the backups are the ONLY
			// remaining recovery path. WR-01: display them the same
			// `~/`-shortened way every other path in this message reads.
			displayBackups := make([]string, len(journal.backups))
			for i, backup := range journal.backups {
				displayBackups[i] = b.displayPath(backup)
			}
			message += "; timestamped backups retained: " + strings.Join(displayBackups, ", ")
		}
		// WR-21: return journal.backups (not nil) — CR-02 already stopped
		// deleting them, but restore() succeeding does not mean there is
		// nothing to report: the retained .bak.<nanos> paths existed only
		// inside the error string, and only when restoreErr != nil. When
		// rollback succeeds, those stale backups were retained on disk with
		// no way for the caller to find or clean them. This mirrors
		// commitGitArtifacts.fail, which already returns journal.backups on
		// the same kind of failure — the two transaction entry points now
		// report symmetrically.
		return journal.backups, fmt.Errorf("%s", message)
	}
	inject := func(step string) error {
		if b.failCommitAt == nil {
			return nil
		}
		return b.failCommitAt(step)
	}
	if err := journal.watchDir(b.sshDir); err != nil {
		return nil, err
	}
	if err := inject("ssh-dir"); err != nil {
		return fail("ssh-dir", err)
	}
	if err := journal.ensureManagedDir(b.sshDir, sshDirMode); err != nil {
		return fail("ssh-dir", err)
	}
	if err := journal.watchFile(staged.FinalPrivatePath); err != nil {
		return fail("private-key", err)
	}
	if err := inject("private-key"); err != nil {
		return fail("private-key", err)
	}
	if staged.PrivPEM != nil {
		backup, err := filewriter.Write(staged.FinalPrivatePath, staged.PrivPEM, keyFileMode)
		if err != nil {
			return fail("private-key", err)
		}
		journal.addBackup(backup)
	} else if err := os.Chmod(staged.FinalPrivatePath, keyFileMode); err != nil {
		return fail("private-key", err)
	}
	if err := journal.watchFile(staged.FinalPubPath); err != nil {
		return fail("public-key", err)
	}
	if err := inject("public-key"); err != nil {
		return fail("public-key", err)
	}
	if staged.PubLine != "" {
		backup, err := filewriter.Write(staged.FinalPubPath, []byte(staged.PubLine), pubFileMode)
		if err != nil {
			return fail("public-key", err)
		}
		journal.addBackup(backup)
	}
	st := b.storage()
	if st.needsIncludeLine {
		if err := journal.watchDir(b.includeDir); err != nil {
			return fail("include-line", err)
		}
		if err := journal.watchFile(b.sshConfigPath); err != nil {
			return fail("include-line", err)
		}
		if err := inject("include-line"); err != nil {
			return fail("include-line", err)
		}
		if err := journal.ensureManagedDir(b.includeDir, sshDirMode); err != nil {
			return fail("include-line", err)
		}
		backup, err := sshconfig.EnsureIncludeLine(b.sshConfigPath)
		if err != nil {
			return fail("include-line", err)
		}
		journal.addBackup(backup)
	}
	if err := journal.watchFile(st.targetPath); err != nil {
		return fail("host-block", err)
	}
	if err := inject("host-block"); err != nil {
		return fail("host-block", err)
	}
	hostBlock, err := sshconfig.RenderCheckedHostBlock(in.Alias, in.Hostname, in.Port, staged.FinalPrivatePath, in.Provider)
	if err != nil {
		return fail("host-block", err)
	}
	backup, err := sshconfig.Write(st.targetPath, in.Name, hostBlock, in.GlobalsGOOS)
	if err != nil {
		return fail("host-block", err)
	}
	journal.addBackup(backup)
	if id.GitConfigured && id.GitName != "" && id.GitEmail != "" {
		gitSpec := tuikit.GitSpec{Identity: id.Name, Name: id.GitName, Email: id.GitEmail, Strategy: id.MatchStrategy, PublicKeyPath: id.PublicKeyPath, SSHHost: id.SSHHost, Provider: id.Provider, GitDir: id.GitDir, ForceSSH: id.ForceSSH}
		if gitSpec.PublicKeyPath == "" {
			gitSpec.PublicKeyPath = id.KeyPath + ".pub"
		}
		if gitSpec.GitDir == "" {
			gitSpec.GitDir = "~/git/" + id.Name + "/"
		}
		if gitSpec.Strategy == "" {
			gitSpec.Strategy = "gitdir"
		}
		_, _, err := b.commitGitArtifacts(gitSpec, staged.PubLine, journal)
		if err != nil {
			return fail("git-artifacts", err)
		}
	}
	backups := make([]string, 0, len(journal.backups))
	for _, backup := range journal.backups {
		backups = append(backups, b.displayPath(backup))
	}
	return backups, nil
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
		// The cached directory may have been REMOVED by a prior transaction's
		// Cleanup seam (deps.Cleanup removes the stage dir after a confirmed
		// write). Running a second ceremony on the same backend — rotate work,
		// then rotate personal, in one TUI session — must recreate it rather
		// than write into a deleted directory.
		if info, err := os.Stat(b.stageDir); err == nil && info.IsDir() {
			return b.stageDir, nil
		}
		if err := os.MkdirAll(b.stageDir, sshDirMode); err != nil { //nolint:gosec // gitid-owned stage path under the invoking user's temp
			return "", fmt.Errorf("gitid: recreating the staging directory: %w", err)
		}
		if cerr := os.Chmod(b.stageDir, sshDirMode); cerr != nil { //nolint:gosec // gitid-owned stage path under the invoking user's temp
			return "", fmt.Errorf("gitid: securing the recreated staging directory: %w", cerr)
		}
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
	// SSHHost is the validated alias from the SSH block. A missing host must
	// fail validation at the transaction boundary; never synthesize an alias.
	sshHost := spec.SSHHost
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
	return strings.Join(parts[1:], ".")
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

// displayMessage is WR-23's fix: WR-01 shortened the explicit Backups list
// through displayPath, but wrapped errors from the low-level packages
// (internal/gitconfig's gitConfigSet/gitConfigUnsetAll, filewriter,
// sshconfig) embed raw absolute sandbox paths inline in their own message
// text (e.g. `git config --file %s %s: ...`), not as a separate field — so
// they never went through displayPath and still reached
// GitCommitMsg/WizardCommitMsg.Err verbatim. Those packages are
// intentionally UI-free (CLAUDE.md) and have no concept of HOME-relative
// display formatting; the shortening belongs at this backend boundary, the
// one place that owns both b.home and the message about to become
// user-facing text. A plain substring replace is sufficient and safe here:
// b.home is an absolute path with no regex metacharacters to escape.
func (b *realBackend) displayMessage(msg string) string {
	if msg == "" || b.home == "" {
		return msg
	}
	return strings.ReplaceAll(msg, b.home, "~")
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

// ---------------------------------------------------------------------------
// IdentityPlanner (plan 05-06). Real Commit/Plan bodies land in 05-07; these
// methods exist so a missing real implementation is a compile error rather
// than a silent NoopIdentityPlanner embed.
// ---------------------------------------------------------------------------

// KeyActionFor implements tuikit.IdentityPlanner: the D-05 routing answer
// ("rotate" or "repair") for one identity, classified through the SAME
// BuildInventory/Classify the identity list rows render (MGR-07's
// never-re-derived rule), with the owner count for the account's current key
// path computed via identity.SharedKeyOwners against the same reconstruction.
// It fails closed on any read or resolution failure — never a zero-value
// answer with a nil error.
func (b *realBackend) KeyActionFor(name string) (string, error) {
	if b.initErr != nil {
		return "", b.initErr
	}
	acct, found := b.findAccount(name)
	if !found {
		return "", fmt.Errorf("gitid: no such identity: %q", name)
	}
	inventory, err := identity.BuildInventory(b.inventoryDeps())
	if err != nil {
		return "", fmt.Errorf("gitid: computing key action for %q: %w", name, err)
	}
	var health identity.IdentityHealth
	for _, h := range inventory.Identities {
		if h.Name == name {
			health = h
			break
		}
	}
	if health.Name == "" {
		return "", fmt.Errorf("gitid: key action: no health report for identity %q", name)
	}
	// CR-02: both sides of this comparison must be normalized. acct.KeyPath
	// and b.accounts() are both raw/tilde here, which only happens to work
	// while every identity in the file spells its IdentityFile the same
	// way — a gitconfig mixing a gitid-written absolute IdentityFile with a
	// recipe-written tilde one for the SAME physical key hid the sharing and
	// routed the destructive rotate path (CR-01) instead of repair.
	normalizedAcct := b.normalizeAccountForWrite(acct)
	ownerCount := len(identity.SharedKeyOwners(b.normalizedAccounts(), normalizedAcct.KeyPath, name)) + 1
	return string(identity.KeyActionFor(health, ownerCount)), nil
}

// scanSourcesForIdentity returns the D-13 scan-source seam for one identity:
// SplitScanRegions over exactly FOUR artifacts — the SSH config, the
// gitconfig, the identity's fragment, and the allowed_signers file — never a
// key file. A missing artifact is skipped (a fresh machine has nothing to
// scan there); any other read failure aborts the plan (review R-07's
// fail-closed rule). The identity's own managed blocks are tagged
// own-managed, so its own alias inside its own Host block is never reported
// as an unmanaged reference (T-05-34).
func (b *realBackend) scanSourcesForIdentity(name string, fragmentPath string) func() ([]identity.ScanSource, error) {
	return func() ([]identity.ScanSource, error) {
		files := []string{b.storageTargetPath(), b.gitconfigPath, b.allowedSigners}
		if fragmentPath != "" {
			files = append(files, fragmentPath)
		}
		var sources []identity.ScanSource
		for _, file := range files {
			content, rerr := os.ReadFile(file) //nolint:gosec // trusted gitid-managed scan source path
			if os.IsNotExist(rerr) {
				continue
			}
			if rerr != nil {
				return nil, fmt.Errorf("gitid: gathering scan source %s: %w", file, rerr)
			}
			sources = append(sources, identity.SplitScanRegions(file, content, name)...)
		}
		return sources, nil
	}
}

// deletePlanView is the ONE conversion site from identity.DeletePlan to the
// render DTO — a straight field copy with no policy in it (plan: "convert the
// result at this one conversion site, propagating the domain error
// unchanged"). KeyCopyPath is populated from the D-11 evidence actually
// available at PLAN time: the archive directory the key pair WILL be copied
// into for an everything-scope delete where the key does not survive.
func (b *realBackend) deletePlanView(p identity.DeletePlan) tuikit.DeletePlanView {
	v := tuikit.DeletePlanView{
		Name:            p.Name,
		Scope:           string(p.Scope),
		SharedKeyOwners: p.SharedKeyOwners,
		Disclaimer:      p.Disclaimer,
	}
	for _, t := range p.Targets {
		v.Targets = append(v.Targets, tuikit.DeleteTargetView{File: t.File, Block: t.Block, Label: t.Label})
	}
	if p.ProviderRewriteTarget != nil {
		v.ProviderRewriteTarget = &tuikit.DeleteTargetView{
			File: p.ProviderRewriteTarget.File, Block: p.ProviderRewriteTarget.Block, Label: p.ProviderRewriteTarget.Label,
		}
	}
	for _, h := range p.UnmanagedHits {
		v.Hits = append(v.Hits, tuikit.UnmanagedHitView{File: h.File, Line: h.Line, Region: string(h.Region), Text: h.Text})
	}
	if p.Scope == identity.DeleteScopeEverything && len(p.KeyPaths) > 0 {
		v.KeyCopyPath = b.displayPath(sshconfig.ArchiveDir(b.sshDir))
	}
	return v
}

// DeletePlan implements tuikit.IdentityPlanner: the real preview both delete
// screens render, built from identity.PlanDelete over the resolved account
// (the SAME reconstruction the list rows render) at the one conversion site.
// Every read or parse failure propagates as an error — never a zero-value
// view with a nil error — and loses no domain context.
func (b *realBackend) DeletePlan(name, scope string) (tuikit.DeletePlanView, error) {
	if b.initErr != nil {
		return tuikit.DeletePlanView{}, b.initErr
	}
	deleteScope, serr := identity.DeleteScopeFrom(scope)
	if serr != nil {
		return tuikit.DeletePlanView{}, serr
	}
	acct, found := b.findAccount(name)
	if !found {
		return tuikit.DeletePlanView{}, fmt.Errorf("gitid: no such identity: %q", name)
	}
	acct = b.normalizeAccountForWrite(acct)
	deps := identity.PlanDeps{
		// Normalized for the same reason buildDeleteDeps' own Accounts field
		// is (see its comment): PlanDelete's SharedKeyOwners/ProviderRefCount
		// checks compare these entries' paths against acct's — which is
		// ALREADY normalized two lines up — by plain string equality.
		Accounts: func() ([]identity.Account, error) { return b.normalizedAccounts(), nil },
		ForeignProviderRefs: func(providerKey string) (int, error) {
			return countForeignProviderRefs(b, providerKey)
		},
		ScanSources: b.scanSourcesForIdentity(name, acct.FragmentPath),
	}
	plan, err := identity.PlanDelete(acct, deleteScope, deps)
	if err != nil {
		return tuikit.DeletePlanView{}, err
	}
	return b.deletePlanView(plan), nil
}

// KeyCeremonyPlan implements tuikit.IdentityPlanner: the facts the
// rotate/repair ceremony renders — the resolved account (provider host, key
// pair paths), the D-06 archive directory for the rotate ceremony, and the
// real target list. It returns an error on any resolution failure, never a
// zero-value view.
func (b *realBackend) KeyCeremonyPlan(name, mode string) (tuikit.KeyCeremonyView, error) {
	if b.initErr != nil {
		return tuikit.KeyCeremonyView{}, b.initErr
	}
	if mode != tuikit.KeyCeremonyModeRotate && mode != tuikit.KeyCeremonyModeRepair {
		return tuikit.KeyCeremonyView{}, fmt.Errorf("gitid: unknown key ceremony mode %q", mode)
	}
	acct, found := b.findAccount(name)
	if !found {
		return tuikit.KeyCeremonyView{}, fmt.Errorf("gitid: no such identity: %q", name)
	}
	acct = b.normalizeAccountForWrite(acct)

	v := tuikit.KeyCeremonyView{
		Mode:         mode,
		IdentityName: name,
		ProviderHost: acct.Provider,
		KeyPath:      b.displayPath(acct.KeyPath),
		PubKeyPath:   b.displayPath(acct.PubPath),
		Targets:      []string{b.displayPath(b.storageTargetPath()), b.displayPath(b.gitconfigPath), b.displayPath(b.allowedSigners)},
	}
	if acct.FragmentPath != "" {
		v.Targets = append(v.Targets, b.displayPath(acct.FragmentPath))
	}
	if acct.KeyPath != "" {
		v.Targets = append(v.Targets, b.displayPath(acct.KeyPath), b.displayPath(acct.PubPath))
	}
	if mode == tuikit.KeyCeremonyModeRotate {
		// D-06: the archived pair lands inside the dedicated archive
		// directory; the timestamped filename is generated DURING the
		// transaction, so the ceremony preview names the directory itself.
		v.ArchivedKeyPath = b.displayPath(sshconfig.ArchiveDir(b.sshDir))
	}
	return v, nil
}

// commitRotateInto and commitRepairInto are the test-only seams the two TUI
// commit closures below delegate their lifecycle through (introduced so a
// recording double can prove the seam contributes NO stage of its own —
// "one call, then message marshalling"). nil in production means the real
// runRotate / runRepair. Mirrors the failCommitAt precedent of a test-only
// injection point on the composition root.
var (
	commitRotateInto = func(b *realBackend, name string, p lifecyclePolicy) (lifecycleResult, error) {
		return b.runRotate(name, p)
	}
	commitRepairInto = func(b *realBackend, name string, p lifecyclePolicy) (lifecycleResult, error) {
		return b.runRepair(name, p)
	}
)

// CommitRotate implements tuikit.IdentityPlanner: the TUI's confirmed rotate
// ceremony. It is a THIN ADAPTER over runRotate — the ONE complete rotate
// lifecycle in lifecycle.go — and adds nothing but message marshalling: all
// journal handling, file watching, confirmation, and rollback live inside the
// lifecycle function (review R-11-CLI). The ceremony screen IS the
// confirmation (review R2-03), so it authorizes with
// confirmationAlreadyObtained — the only layer permitted to assert it. Every
// backup/restored path is scrubbed through displayPath/displayMessage, the
// same WR-01/WR-23 discipline CommitGit applies.
func (b *realBackend) CommitRotate(name string) tea.Cmd {
	return func() tea.Msg {
		if b.initErr != nil {
			return tuikit.KeyCommitMsg{Mode: "rotate", Err: b.displayMessage(b.initErr.Error())}
		}
		res, err := commitRotateInto(b, name, lifecyclePolicy{Confirm: confirmationAlreadyObtained})
		msg := tuikit.KeyCommitMsg{
			Mode:     "rotate",
			Backups:  displayPaths(b, res.Backups),
			Restored: displayMessages(b, res.Restored),
		}
		for _, pth := range res.ArchivedKeyPaths {
			if msg.ArchivedKeyPath == "" {
				msg.ArchivedKeyPath = b.displayPath(pth)
			}
		}
		if err != nil {
			msg.Err = b.displayMessage(err.Error())
		}
		return msg
	}
}

// CommitNewKey implements tuikit.IdentityPlanner: the TUI's confirmed
// new-key (repair) ceremony — the same thin adapter as CommitRotate, over
// runRepair. Repair never archives (D-05), so the delivered commit message
// carries an EMPTY archived path and the archive directory is untouched.
func (b *realBackend) CommitNewKey(name string) tea.Cmd {
	return func() tea.Msg {
		if b.initErr != nil {
			return tuikit.KeyCommitMsg{Mode: "repair", Err: b.displayMessage(b.initErr.Error())}
		}
		res, err := commitRepairInto(b, name, lifecyclePolicy{Confirm: confirmationAlreadyObtained})
		msg := tuikit.KeyCommitMsg{
			Mode:     "repair",
			Backups:  displayPaths(b, res.Backups),
			Restored: displayMessages(b, res.Restored),
		}
		if err != nil {
			msg.Err = b.displayMessage(err.Error())
		}
		return msg
	}
}
