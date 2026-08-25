package tuikit

import tea "charm.land/bubbletea/v2"

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
