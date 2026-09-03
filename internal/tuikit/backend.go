package tuikit

import (
	"errors"

	tea "charm.land/bubbletea/v2"
)

// ErrPlannerNotImplemented is the sentinel NoopIdentityPlanner returns from
// every method. Fixtures and test stubs embed the noop and override only the
// methods they exercise; a missing real implementation must be a compile
// error, not this sentinel at runtime.
var ErrPlannerNotImplemented = errors.New("identity planner not implemented")

// IdentityPlanner is the five Phase-5 preview and key-write seams.
//
// Ownership rule (review R2-10): Backend owns CommitDelete; IdentityPlanner
// owns KeyActionFor, DeletePlan, KeyCeremonyPlan, CommitRotate, and
// CommitNewKey. The dividing line is preview versus write — delete's preview
// (DeletePlan) is a planner seam and delete's write (CommitDelete) is a
// Backend seam, exactly one of each. IdentityPlanner contains exactly these
// five methods and does NOT absorb CommitDelete.
type IdentityPlanner interface {
	// KeyActionFor returns the routing answer for one identity ("rotate" or
	// "repair"). A non-nil error must fail closed: the menu row renders the
	// error and does not open the ceremony pane.
	KeyActionFor(name string) (string, error)
	// DeletePlan returns the one plan value both delete screens render from.
	// A non-nil error must fail closed: the confirm screen renders the error
	// state with the confirm control disabled and no partial target list.
	DeletePlan(name, scope string) (DeletePlanView, error)
	// KeyCeremonyPlan returns the facts the rotate/repair ceremony renders.
	// A non-nil error must fail closed: the ceremony pane renders the error
	// and never advances to a beat that would trigger a commit.
	KeyCeremonyPlan(name, mode string) (KeyCeremonyView, error)
	// CommitRotate dispatches the confirmed rotate transaction off the
	// update loop and eventually delivers a KeyCommitMsg.
	CommitRotate(name string) tea.Cmd
	// CommitNewKey dispatches the confirmed repair (new-key) transaction
	// off the update loop and eventually delivers a KeyCommitMsg.
	CommitNewKey(name string) tea.Cmd
}

// NoopIdentityPlanner implements every IdentityPlanner method with a
// zero-value view plus ErrPlannerNotImplemented (and a command delivering
// that error for the two commit seams). Fixtures and test stubs embed it
// and override only what they exercise, so a sixth method in a later phase
// does not break every implementer. The real backend must NOT embed it — a
// missing real implementation must be a compile error.
type NoopIdentityPlanner struct{}

// KeyActionFor implements IdentityPlanner.
func (NoopIdentityPlanner) KeyActionFor(string) (string, error) {
	return "", ErrPlannerNotImplemented
}

// DeletePlan implements IdentityPlanner.
func (NoopIdentityPlanner) DeletePlan(string, string) (DeletePlanView, error) {
	return DeletePlanView{}, ErrPlannerNotImplemented
}

// KeyCeremonyPlan implements IdentityPlanner.
func (NoopIdentityPlanner) KeyCeremonyPlan(string, string) (KeyCeremonyView, error) {
	return KeyCeremonyView{}, ErrPlannerNotImplemented
}

// CommitRotate implements IdentityPlanner.
func (NoopIdentityPlanner) CommitRotate(string) tea.Cmd {
	return func() tea.Msg {
		return KeyCommitMsg{Err: ErrPlannerNotImplemented.Error()}
	}
}

// CommitNewKey implements IdentityPlanner.
func (NoopIdentityPlanner) CommitNewKey(string) tea.Cmd {
	return func() tea.Msg {
		return KeyCommitMsg{Err: ErrPlannerNotImplemented.Error()}
	}
}

var _ IdentityPlanner = NoopIdentityPlanner{}

// ErrGlobalSSHPlannerNotImplemented is the sentinel NoopGlobalSSHPlanner
// returns from every method. Fixtures and test stubs embed the noop and
// override only the methods they exercise; a missing real implementation must
// be a compile error, not this sentinel at runtime.
var ErrGlobalSSHPlannerNotImplemented = errors.New("global SSH planner not implemented")

// GlobalSSHPlanner is the Phase 6 global-SSH seam: the options-list read, the
// apply-preview write plan, and the asynchronous apply commit. The THREE
// methods are the whole seam — no speculative surface. The storage
// (migration) seam is deliberately a SEPARATE interface owned by plan 06-05,
// so this interface never grows migration methods.
type GlobalSSHPlanner interface {
	// GlobalSSHOptionStates returns the Options sub-tab's live rows: the
	// real current value and the provable provenance LABEL per policy option.
	// A non-nil error must fail loosely per GSSH-01's advisory posture — the
	// pane renders an error note rather than a blank body.
	GlobalSSHOptionStates() ([]GlobalSSHOptionView, error)
	// GlobalSSHApplyPlan returns the confirmed-apply preview: the resolved
	// targets, the promised backup paths, and the diff of the candidate
	// write. A non-nil error must fail closed: the confirm screen renders the
	// error state.
	GlobalSSHApplyPlan(keys []string) (GlobalSSHApplyPlanView, error)
	// CommitGlobalSSH dispatches the confirmed global-SSH apply transaction
	// off the update loop and delivers a GlobalSSHCommitMsg.
	CommitGlobalSSH(keys []string) tea.Cmd
}

// NoopGlobalSSHPlanner implements every GlobalSSHPlanner method with a
// zero-value view plus ErrGlobalSSHPlannerNotImplemented (and a command
// delivering that error for the commit seam). Fixtures and test stubs embed
// it and override only what they exercise. The REAL backend must NOT embed
// it — a missing real implementation must be a compile error, pinned by the
// compile-time assertion in cmd/gitid/wiring.go and a reflection test.
type NoopGlobalSSHPlanner struct{}

// GlobalSSHOptionStates implements GlobalSSHPlanner.
func (NoopGlobalSSHPlanner) GlobalSSHOptionStates() ([]GlobalSSHOptionView, error) {
	return nil, ErrGlobalSSHPlannerNotImplemented
}

// GlobalSSHApplyPlan implements GlobalSSHPlanner.
func (NoopGlobalSSHPlanner) GlobalSSHApplyPlan([]string) (GlobalSSHApplyPlanView, error) {
	return GlobalSSHApplyPlanView{}, ErrGlobalSSHPlannerNotImplemented
}

// CommitGlobalSSH implements GlobalSSHPlanner.
func (NoopGlobalSSHPlanner) CommitGlobalSSH([]string) tea.Cmd {
	return func() tea.Msg {
		return GlobalSSHCommitMsg{Err: ErrGlobalSSHPlannerNotImplemented.Error()}
	}
}

var _ GlobalSSHPlanner = NoopGlobalSSHPlanner{}

// ErrGlobalGitPlannerNotImplemented is the sentinel NoopGlobalGitPlanner
// returns from every method. Fixtures and test stubs embed the noop and
// override only the methods they exercise; a missing real implementation must
// be a compile error, not this sentinel at runtime.
var ErrGlobalGitPlannerNotImplemented = errors.New("global git planner not implemented")

// GlobalGitPlanner is the Phase 7 global-git seam: the options-list read, the
// apply-preview write plan, and the asynchronous apply commit. The THREE
// methods are the whole seam — no speculative surface. The D9 fallback-author
// seam is deliberately a SEPARATE interface owned by plan 07-02, so this
// interface never grows author methods.
type GlobalGitPlanner interface {
	// GlobalGitOptionStates returns the Options pane's live rows: the real
	// current value and the provable provenance LABEL per policy option.
	// A non-nil error must fail loosely per GGIT-01's advisory posture — the
	// pane renders an error note rather than a blank body.
	GlobalGitOptionStates() ([]GlobalGitOptionView, error)
	// GlobalGitApplyPlan returns the confirmed-apply preview: the resolved
	// target, the promised backup paths, and the diff of the candidate write.
	// A non-nil error must fail closed: the confirm screen renders the error.
	GlobalGitApplyPlan(keys []string) (GlobalGitApplyPlanView, error)
	// CommitGlobalGit dispatches the confirmed global-git apply transaction
	// off the update loop and delivers a GlobalGitCommitMsg.
	CommitGlobalGit(keys []string) tea.Cmd
}

// NoopGlobalGitPlanner implements every GlobalGitPlanner method with a
// zero-value view plus ErrGlobalGitPlannerNotImplemented (and a command
// delivering that error for the commit seam). Fixtures and test stubs embed
// it and override only what they exercise. The REAL backend must NOT embed
// it — a missing real implementation must be a compile error, pinned by the
// compile-time assertion in cmd/gitid/wiring.go and a reflection test.
type NoopGlobalGitPlanner struct{}

// GlobalGitOptionStates implements GlobalGitPlanner.
func (NoopGlobalGitPlanner) GlobalGitOptionStates() ([]GlobalGitOptionView, error) {
	return nil, ErrGlobalGitPlannerNotImplemented
}

// GlobalGitApplyPlan implements GlobalGitPlanner.
func (NoopGlobalGitPlanner) GlobalGitApplyPlan([]string) (GlobalGitApplyPlanView, error) {
	return GlobalGitApplyPlanView{}, ErrGlobalGitPlannerNotImplemented
}

// CommitGlobalGit implements GlobalGitPlanner.
func (NoopGlobalGitPlanner) CommitGlobalGit([]string) tea.Cmd {
	return func() tea.Msg {
		return GlobalGitCommitMsg{Err: ErrGlobalGitPlannerNotImplemented.Error()}
	}
}

var _ GlobalGitPlanner = NoopGlobalGitPlanner{}

// ErrGlobalGitIgnorePlannerNotImplemented is the sentinel
// NoopGlobalGitIgnorePlanner returns from every method. Fixtures and test
// stubs embed the noop and override only the methods they exercise; a missing
// real implementation must be a compile error, not this sentinel at runtime.
var ErrGlobalGitIgnorePlannerNotImplemented = errors.New("global git ignore planner not implemented")

// GlobalGitIgnorePlanner is the Phase 9.2 global-gitignore seam: the live
// state read, the apply-preview write plan, and the asynchronous apply
// commit. The THREE methods are the whole seam — no speculative surface.
type GlobalGitIgnorePlanner interface {
	GlobalGitIgnoreState() (GlobalGitIgnoreView, error)
	GlobalGitIgnoreApplyPlan(content string) (GlobalGitIgnoreApplyPlanView, error)
	CommitGlobalGitIgnore(content, planToken string) tea.Cmd
}

// NoopGlobalGitIgnorePlanner implements every GlobalGitIgnorePlanner method
// with a zero-value view plus ErrGlobalGitIgnorePlannerNotImplemented (and a
// command delivering that error for the commit seam). Fixtures and test stubs
// embed it and override only what they exercise. The REAL backend must NOT
// embed it — a missing real implementation must be a compile error, pinned
// by the compile-time assertion in cmd/gitid/wiring.go.
type NoopGlobalGitIgnorePlanner struct{}

// GlobalGitIgnoreState implements GlobalGitIgnorePlanner.
func (NoopGlobalGitIgnorePlanner) GlobalGitIgnoreState() (GlobalGitIgnoreView, error) {
	return GlobalGitIgnoreView{}, ErrGlobalGitIgnorePlannerNotImplemented
}

// GlobalGitIgnoreApplyPlan implements GlobalGitIgnorePlanner.
func (NoopGlobalGitIgnorePlanner) GlobalGitIgnoreApplyPlan(string) (GlobalGitIgnoreApplyPlanView, error) {
	return GlobalGitIgnoreApplyPlanView{}, ErrGlobalGitIgnorePlannerNotImplemented
}

// CommitGlobalGitIgnore implements GlobalGitIgnorePlanner.
func (NoopGlobalGitIgnorePlanner) CommitGlobalGitIgnore(string, string) tea.Cmd {
	return func() tea.Msg {
		return GlobalGitIgnoreCommitMsg{Err: ErrGlobalGitIgnorePlannerNotImplemented.Error()}
	}
}

var _ GlobalGitIgnorePlanner = NoopGlobalGitIgnorePlanner{}

// ErrGitFallbackAuthorPlannerNotImplemented is the sentinel
// NoopGitFallbackAuthorPlanner returns from every method. Fixtures and test
// stubs embed the noop and override only the methods they exercise; a missing
// real implementation must be a compile error, not this sentinel at runtime.
var ErrGitFallbackAuthorPlannerNotImplemented = errors.New("git fallback author planner not implemented")

// GitFallbackAuthorPlanner is the Phase 7 D9 fallback-author seam: the
// current-pair read, the apply-preview write plan, and the asynchronous
// apply commit. The THREE methods are the whole seam — no speculative
// surface. This is deliberately SEPARATE from GlobalGitPlanner because that
// interface owns the option catalogue and its write, and this one owns the
// fallback author and its own write — two focused interfaces let a fixture
// adopt one without hand-writing the other, and the separation is the
// structural expression of D-05's "own dedicated ceremony, never folded
// into the baseline block".
type GitFallbackAuthorPlanner interface {
	GitFallbackAuthorState() (GitFallbackAuthorView, error)
	GitFallbackAuthorPlan(name, email string) (GitFallbackAuthorPlanView, error)
	CommitGitFallbackAuthor(name, email string) tea.Cmd
}

// NoopGitFallbackAuthorPlanner implements every GitFallbackAuthorPlanner
// method with a zero-value view plus ErrGitFallbackAuthorPlannerNotImplemented.
// Fixtures and test stubs embed it and override only what they exercise.
// The REAL backend must NOT embed it — a missing real implementation must
// be a compile error, pinned by the compile-time assertion in
// cmd/gitid/wiring.go and a reflection test.
type NoopGitFallbackAuthorPlanner struct{}

// GitFallbackAuthorState implements GitFallbackAuthorPlanner.
func (NoopGitFallbackAuthorPlanner) GitFallbackAuthorState() (GitFallbackAuthorView, error) {
	return GitFallbackAuthorView{}, ErrGitFallbackAuthorPlannerNotImplemented
}

// GitFallbackAuthorPlan implements GitFallbackAuthorPlanner.
func (NoopGitFallbackAuthorPlanner) GitFallbackAuthorPlan(string, string) (GitFallbackAuthorPlanView, error) {
	return GitFallbackAuthorPlanView{}, ErrGitFallbackAuthorPlannerNotImplemented
}

// CommitGitFallbackAuthor implements GitFallbackAuthorPlanner.
func (NoopGitFallbackAuthorPlanner) CommitGitFallbackAuthor(string, string) tea.Cmd {
	return func() tea.Msg {
		return GitFallbackAuthorCommitMsg{Err: ErrGitFallbackAuthorPlannerNotImplemented.Error()}
	}
}

var _ GitFallbackAuthorPlanner = NoopGitFallbackAuthorPlanner{}

// ErrSSHStoragePlannerNotImplemented is the sentinel NoopSSHStoragePlanner
// returns from every method. Fixtures and test stubs embed the noop and
// override only the methods they exercise; a missing real implementation must
// be a compile error, not this sentinel at runtime.
var ErrSSHStoragePlannerNotImplemented = errors.New("SSH storage planner not implemented")

// SSHStoragePlanner is the Phase 6 storage-migration seam: the layout
// preview and the asynchronous migrate commit. It is deliberately SEPARATE
// from GlobalSSHPlanner — that interface owns the option catalogue and its
// write; this one owns the layout and its migration. Two focused interfaces
// let a fixture or a test stub adopt one without hand-writing the other.
type SSHStoragePlanner interface {
	// SSHStorageMigrationPlan returns the Storage sub-tab's live preview
	// for the requested target layout: current vs target, the resulting
	// config bytes, and an opaque PlanToken identifying the held plan.
	// A non-nil error must fail closed: the pane renders the error and
	// suppresses the migrate action.
	SSHStorageMigrationPlan(layout SSHStorageLayout) (SSHStorageMigrationView, error)
	// CommitSSHStorage dispatches the confirmed storage-migration
	// transaction off the update loop and delivers an SSHStorageCommitMsg.
	// planToken must be the opaque identifier the view carried when the
	// ceremony opened — the commit path must pass it through unchanged.
	CommitSSHStorage(layout SSHStorageLayout, planToken string) tea.Cmd
}

// NoopSSHStoragePlanner implements every SSHStoragePlanner method with a
// zero-value view plus ErrSSHStoragePlannerNotImplemented (and a command
// delivering that error for the commit seam). Fixtures and test stubs embed
// it and override only what they exercise. The REAL backend must NOT embed
// it — a missing real implementation must be a compile error, pinned by the
// compile-time assertion in cmd/gitid/wiring.go and a reflection test.
type NoopSSHStoragePlanner struct{}

// SSHStorageMigrationPlan implements SSHStoragePlanner.
func (NoopSSHStoragePlanner) SSHStorageMigrationPlan(SSHStorageLayout) (SSHStorageMigrationView, error) {
	return SSHStorageMigrationView{}, ErrSSHStoragePlannerNotImplemented
}

// CommitSSHStorage implements SSHStoragePlanner.
func (NoopSSHStoragePlanner) CommitSSHStorage(SSHStorageLayout, string) tea.Cmd {
	return func() tea.Msg {
		return SSHStorageCommitMsg{Err: ErrSSHStoragePlannerNotImplemented.Error()}
	}
}

var _ SSHStoragePlanner = NoopSSHStoragePlanner{}

// backend.go defines the ONE injected seam this package is built around.
//
// tuikit renders the approved, frozen gitid design. It never reads or
// writes a file, never shells out, and never imports a first-party backend
// package. Everything it needs from the outside world arrives through a
// Backend value handed to NewApp:
//
//	cmd/gitid-dummy  → dummytui.NewFixtureBackend()  (recipe fixtures, in-memory)
//	cmd/gitid        → the real composition root in cmd/gitid/wiring.go
//
// Every method speaks either a plain Go type, a tuikit view state
// (DemoState/Action), or a view DTO from views.go. No method may ever name
// a keygen/tester/identity type — that is what views.go exists for.

// Backend is the data + effects contract the shared render stack calls
// into. Both binaries satisfy it; neither the screens nor the create
// wizard know which one they are talking to.
type Backend interface {
	IdentityPlanner
	// GlobalSSHPlanner: plan 06-01's Options-sub-tab seam. The real backend
	// must NOT embed NoopGlobalSSHPlanner — a missing real implementation must
	// be a compile error, pinned by wiring.go's compile-time assertion and by
	// a reflection test.
	GlobalSSHPlanner
	// GlobalGitPlanner: plan 07-01's global-git Options seam. The real backend
	// must NOT embed NoopGlobalGitPlanner — a missing real implementation must
	// be a compile error, pinned by wiring.go's compile-time assertion and by
	// a reflection test. The D9 fallback-author seam is a SEPARATE interface
	// owned by plan 07-02, so this interface never grows author methods.
	GlobalGitPlanner
	// GitFallbackAuthorPlanner: plan 07-02's D9 fallback-author seam.
	// Deliberately separate from GlobalGitPlanner — one interface owns the
	// option catalogue and its write, this one owns the fallback author and
	// its own write. The real backend must NOT embed
	// NoopGitFallbackAuthorPlanner.
	GitFallbackAuthorPlanner
	// SSHStoragePlanner: plan 06-05's Storage-sub-tab seam. Deliberately
	// separate from GlobalSSHPlanner — one interface owns the option
	// catalogue and its write, this one owns the layout and its migration.
	// The real backend must NOT embed NoopSSHStoragePlanner.
	SSHStoragePlanner
	// GlobalGitIgnorePlanner: plan 09.2-01's Global Git Ignore seam. The
	// real backend must NOT embed NoopGlobalGitIgnorePlanner — a missing
	// real implementation must be a compile error.
	GlobalGitIgnorePlanner

	// ----- Data -------------------------------------------------------

	// InitialState is the state the App starts from: identities, health
	// findings, storage layout, and known backups. The dummy seeds it from
	// its fixtures; the real binary reads the user's actual configuration.
	InitialState() DemoState

	// DemoBanner reports whether tab must render the D-16 "this screen is
	// still demo data" banner. The dummy returns false for every tab — the
	// whole dummy IS demo data, so a banner would be noise. The real
	// binary returns true for tabs not yet wired to live data.
	DemoBanner(tab TabID) bool

	// Persist applies one committed Action and returns the resulting
	// state. The dummy reduces it in memory (Reduce); the real binary
	// performs the backed-up write and re-reads the configuration. It is
	// the ONLY place a committed mutation leaves the render stack.
	Persist(state DemoState, action Action) DemoState

	// PersistError reports the error the LAST Persist call failed with, or
	// nil. App.handleKey consults it immediately after Persist runs (D-16):
	// a FixFinding dispatched during a Doctor batch walk that fails
	// halts the walk instead of silently advancing (doctor_screen.go's
	// haltBatch).
	// The dummy always returns nil (Reduce never fails); the real binary
	// records the last commit's error and returns it here (unchanged from
	// its pre-existing realBackend.PersistError()).
	PersistError() error

	// ----- Create-flow effects ---------------------------------------

	// AlgorithmCatalog is the KEY-01 key-algorithm catalog offered on the
	// wizard's SSH step, in display order.
	AlgorithmCatalog() []AlgorithmCatalogEntry

	// ProviderDefaults resolves a provider host to its default endpoint
	// and port (D-20/D-21: github.com → ssh.github.com:443 alt-SSH).
	ProviderDefaults(provider string) (hostname, port string)

	// DefaultMatchStrategy is the includeIf match strategy a new identity
	// starts on (GITUI-03; "gitdir" per recipes/).
	DefaultMatchStrategy() string

	// ValidateHostBlock validates the four SSH form values before they are
	// interpolated into an OpenSSH Host block. A non-nil error is a
	// *ValidationError with Field set to alias|hostname|port|identityFile
	// so the UI can render it inline.
	ValidateHostBlock(alias, hostname, port, identityFile string) *ValidationError

	// HostBlockPreview is the live, WYSIWYG Host block text for spec —
	// written exactly like this on confirm (SSHUI-03).
	HostBlockPreview(spec CreateSpec) string

	// GitFragmentPreview is the ~/.gitconfig.d/<identity> fragment text
	// for spec.
	GitFragmentPreview(spec GitSpec) string

	// IncludeIfPreview is the ~/.gitconfig includeIf block for spec's
	// match strategy, aliased to spec.Identity.
	IncludeIfPreview(spec GitSpec) string

	// AliasCollision reports whether alias is already claimed by an
	// existing Host stanza in the target SSH config — the D-09 collision
	// check the wizard gates step 1 on.
	AliasCollision(alias string) (bool, error)

	// ScanReusableKeys lists the existing keys the D-10 picker offers for
	// reuse. Unparseable/encrypted keys are surfaced with a note, never
	// dropped (D-13).
	ScanReusableKeys() []ReusableKeyView

	// ManualReusePath resolves a user-typed path (the picker's manual-path
	// row, D-10) into a reuse candidate. A symlinked candidate is rejected
	// before parsing (T-03-13) like every other "path the user points at"
	// input. A non-nil error means the candidate is unusable; the picker
	// shows it inline and never blocks the rest of the form.
	ManualReusePath(path string) (ReusableKeyView, error)

	// TestConfigPath is the throwaway config both test stages run against,
	// so the live ~/.ssh/config stays untouched until the final confirm.
	TestConfigPath() string

	// Stage1Command is the exact stage-1 command string shown to the user
	// (key DIRECT against the provider, TEST-01).
	Stage1Command(spec CreateSpec) string

	// Stage2Command is the exact stage-2 command string shown to the user
	// (resolve BY ALIAS — no -i by design, TEST-02).
	Stage2Command(spec CreateSpec) string

	// TestStage1 runs the stage-1 test asynchronously. The returned
	// command MUST eventually deliver a WizardStageMsg with Stage 1 and a
	// populated Result.
	TestStage1(spec CreateSpec) tea.Cmd

	// TestStage2 runs the stage-2 test asynchronously. The returned
	// command MUST eventually deliver a WizardStageMsg with Stage 2 and a
	// populated Result.
	TestStage2(spec CreateSpec) tea.Cmd

	// ResolvedStorageTarget is the file gitid's managed blocks actually
	// land in for state's STORE-01 layout — ~/.ssh/config under the
	// sentinel layout, the gitid-owned included file otherwise (D-05/D-06).
	ResolvedStorageTarget(state DemoState) string

	// CreateWritePlan is what a committed create will touch: the target
	// files and the timestamped backups taken first (TEST-03). git is nil
	// when the user skipped the Git step.
	CreateWritePlan(spec CreateSpec, git *GitSpec) WritePlanView

	// GitWritePlan is CreateWritePlan's sibling for the standalone
	// Configure-Git ceremony (CR-12): what a committed Git-config write
	// will touch, the timestamped backups actually taken (existence-gated,
	// same naming convention filewriter really uses), and any directories
	// the transaction creates. Before this seam existed, gitCeremonyFor
	// built its disclosure from hardcoded UI-layer strings that named
	// backup files that would never exist, undercounted the real backup
	// total, and never mentioned directory creation at all.
	GitWritePlan(spec GitSpec) WritePlanView

	// FixPlanFor returns finding's fix plan (target file, diff, destructive
	// gating, result receipt) — the ONE seam every internal/tuikit call site
	// routes through instead of calling the free PlanFor(finding) function
	// directly (08-02-PLAN.md Task 2). FixtureBackend delegates unchanged to
	// PlanFor, preserving the frozen Phase-2 visual-regression fixture; the
	// real backend reads actual target-file content and renders a true
	// before/after diff (currently for findings carrying a
	// doctor-originated Rewrite descriptor — the D-09 surgical rewrite this
	// wave introduces — falling back to PlanFor's frozen shape otherwise).
	FixPlanFor(finding DemoFinding) FixPlan

	// CopyPublicKey copies the identity's public key to the system
	// clipboard (D-03) — offered on the ReachableNotUploaded warning path so
	// the user can register it with the provider and retry. It returns the
	// receipt note to show, because only the Backend knows whether a REAL
	// clipboard was written (the dummy says so explicitly).
	CopyPublicKey(pubKeyPath string) (note string, err error)

	// GitStepDisabledReason resolves the create wizard's Git-identity step
	// [ Continue ] gating (D-19). alwaysDisabled=true means Continue never
	// enables regardless of the form's own validity, and reason is the
	// suffix to show instead of the form-validity one — the real binary
	// returns alwaysDisabled=false now that the real Git path is wired. The
	// method remains for temporary backend failures and test doubles.
	GitStepDisabledReason() (reason string, alwaysDisabled bool)

	// CommitCreate dispatches the confirmed create transaction off the
	// Bubble Tea update loop and returns a command that will eventually
	// deliver a WizardCommitMsg with the real result. This async seam keeps
	// the ceremony honest: the receipt renders only after the transaction
	// succeeds, and a failure is surfaced instead of swallowed.
	CommitCreate(identity DemoIdentity) tea.Cmd

	// CommitGit writes a standalone create/edit Git flow asynchronously. The
	// UI must wait for GitCommitMsg before reducing ConfigureGit, so no
	// optimistic success can mask a failed transaction.
	CommitGit(spec GitSpec) tea.Cmd

	// CommitDelete performs a confirmed identity delete asynchronously,
	// scoped by scope ("git-only" | "everything" — mirrors
	// identity.DeleteScope as plain strings, since this file may never name
	// an identity type). The UI must wait for DeleteCommitMsg before
	// reducing DeleteIdentity, so no optimistic success can mask a failed or
	// rolled-back transaction — the exact GitCommitMsg sequencing this
	// mirrors.
	//
	// Ownership rule (review R2-10), stated once here: CommitDelete lives on
	// Backend and STAYS there. Plan 05-06 introduces an IdentityPlanner
	// sub-interface for five NEW Phase-5 seams (KeyActionFor, DeletePlan,
	// KeyCeremonyPlan, CommitRotate, CommitNewKey) — delete's PREVIEW
	// (DeletePlan) belongs there, but delete's WRITE (this method) does not
	// move: CommitDelete ships in wave 1, every Backend implementer already
	// has it by the time 05-06 lands, and migrating it later would mean
	// moving three implementers to buy nothing. There is exactly one preview
	// seam and one write seam for delete, and neither duplicates the other.
	CommitDelete(name string, scope string) tea.Cmd

	// ----- Clone (D-14/D-15/D-16/D-17, MGR-04) -------------------------

	// SuggestCloneName is the D-17 suggested clone name for source: the
	// source name plus the frozen clone suffix, silently auto-bumped to the
	// next free variant against every existing identity name AND every
	// literal (non-wildcard) parsed Host pattern — the clone-name prompt
	// never opens in an error state.
	SuggestCloneName(source string) string

	// ClonePrefill derives the pre-fill values for cloneName from source
	// (D-14: copy the two author fields, re-derive everything else) and
	// returns them as a ClonePrefillView the wizard opens with — NOT a
	// write. reuseSourceKey selects whether the pre-fill carries the
	// source's key path (ReuseKeyPath) or leaves it empty for a fresh
	// generate; either way D-16 still requires the full two-stage gate
	// before any write. A non-nil error is a validation or D-17/R-29
	// pattern-shadowing refusal the prompt renders inline — it never
	// silently substitutes a different name.
	ClonePrefill(source, cloneName string, reuseSourceKey bool) (ClonePrefillView, error)

	// ----- Upload / Credentials Assist (Phase 9, UP-02/UP-03) ---------

	// UploadEligibility resolves whether autonomous key upload can run for
	// hostname right now — RESOLVED ASYNCHRONOUSLY, never on the render
	// path. Answering requires exec.LookPath plus a "gh/glab auth status"
	// subprocess (real backend); a Bubble Tea View or Update pass must
	// never block on an external process that can be slow or hung (review
	// R3). This is a hard rule: no future implementation may make this
	// method synchronous. An implementation MAY answer without any
	// subprocess when the hostname's provider is not one of D-13's gated
	// main domains — that path is pure and still returns a command for
	// call-site uniformity. The delivered UploadEligibilityMsg carries the
	// ORIGINAL hostname argument (never a canonicalized form) so a stale
	// reply — the user changed the host before this arrived — can be
	// discarded by the caller.
	UploadEligibility(hostname string) tea.Cmd

	// RunUpload dispatches the confirmed autonomous upload beat off the
	// update loop — async, like TestStage1/TestStage2 — and MUST
	// eventually deliver an UploadRunMsg. It never returns an error that
	// could stop the wizard: every failure (staging, detect, auth,
	// upload) is reported as a failed UploadResultRow inside the
	// delivered view (D-03/D-11 — upload never gates).
	RunUpload(spec CreateSpec) tea.Cmd

	// UploadInstructions is the manual-fallback text slot: the real
	// backend returns internal/upload.Instructions(provider) byte-
	// identically; this method exists so internal/tuikit never imports
	// internal/upload and never carries a second copy of the instruction
	// text (09-UI-SPEC.md requires byte-identical reuse).
	UploadInstructions(provider string) string

	// RegisterKeyPlan resolves the D-08 register-key pane's eligibility
	// answer for the named identity — RESOLVED ASYNCHRONOUSLY (R3), never on
	// the render path, mirroring UploadEligibility. The delivered
	// RegisterKeyPlanMsg carries name so a reply arriving after the pane has
	// moved to a different identity can be discarded.
	RegisterKeyPlan(name string) tea.Cmd

	// RunUploadForIdentity dispatches the confirmed autonomous upload beat
	// for the named (already-existing) identity's own key off the update
	// loop — async, like RunUpload — and MUST eventually deliver an
	// UploadRunMsg. It never returns an error that could stop the pane:
	// every failure is reported as a failed UploadResultRow inside the
	// delivered view (D-03/D-11 — upload never gates).
	RunUploadForIdentity(name string) tea.Cmd

	// RotateDeleteOffer resolves D-04's interactive old-key delete offer
	// for the named identity — RESOLVED ASYNCHRONOUSLY (R3), never on the
	// render path: it performs a FRESH provider inventory read, precisely
	// the kind of network-backed CLI call that must never block the Bubble
	// Tea loop at the moment the user reaches the rotate result screen (the
	// same reasoning UploadEligibility already established). An internal
	// failure (inventory error, no match, non-qualifying provider) fails
	// CLOSED by returning an Available=false view with a reason, never by
	// surfacing an error the caller must special-case — this method may
	// NEVER be converted to a synchronous shape.
	RotateDeleteOffer(name string) tea.Cmd

	// CommitRotateDeleteOldKey is the ONE remotely-destructive call in this
	// phase — reachable ONLY from a confirmed choice on the D-04 offer,
	// never autonomously. It deletes exactly the given keyID (the ID the
	// user reviewed on the offer) and MUST NOT re-resolve it: re-resolving
	// after the user has confirmed would let a provider-side change between
	// display and confirm redirect the deletion (review R12's retry rule
	// depends on this). Async, like every other provider-I/O seam; this
	// method may NEVER be converted to a synchronous shape.
	CommitRotateDeleteOldKey(name, keyID string) tea.Cmd
}

// WizardStageMsg completes a create-wizard test stage. Backends deliver it
// from the tea.Cmd returned by TestStage1/TestStage2 — the dummy after a
// brief tick, the real binary once ssh has actually answered.
type WizardStageMsg struct {
	// Stage is 1 or 2.
	Stage int
	// Result is that stage's outcome, command, and output line.
	Result TestResultView
}
