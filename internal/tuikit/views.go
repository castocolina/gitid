package tuikit

// views.go defines the tuikit-LOCAL view DTOs every Backend method
// signature speaks.
//
// internal/tuikit imports ZERO first-party backend packages — no identity,
// tester, sshconfig, keygen, filewriter, doctor, adopter, platform,
// clipboard or uploader. That is a hard, mechanically-enforced contract
// (internal/dummytui's TestNoBackendAllowlist plus the Makefile's
// gate-no-backend-files target), so the seam can never name a backend
// type. The DTOs below exist precisely so it does not have to.
//
// CONVERSION HAPPENS IN EXACTLY ONE PLACE. keygen.ReusableKey becomes
// ReusableKeyView, and tester.Result / tester.Outcome become
// TestResultView / TestOutcome, inside the composition root
// cmd/gitid/wiring.go — never inside this package, and never inside a
// screen. The dummy's FixtureBackend constructs these DTOs directly from
// its fixtures. If a future screen appears to need a backend value, the
// answer is a new DTO here plus a conversion in cmd/gitid/wiring.go; it is
// never a new entry in the import allowlist.

// ReusableKeyView is one candidate key on the D-10 "reuse an existing key"
// picker — the view projection of a scanned private key. It REPLACES every
// use of keygen.ReusableKey inside this package.
type ReusableKeyView struct {
	// Path is the private key's path as shown to the user.
	Path string
	// Algorithm is the key's algorithm id (ed25519, rsa-4096, …).
	Algorithm string
	// Fingerprint is the SHA256 fingerprint shown next to the path.
	Fingerprint string
	// HasPub reports whether the sibling .pub file exists on disk.
	HasPub bool
	// Encrypted reports whether the private key is passphrase-protected —
	// informational (D-13), never a reason to hide the entry.
	Encrypted bool
	// InUseBy carries the D-12 "in use by: personal" label. Empty means the
	// key is not referenced by any existing identity.
	InUseBy string
}

// TestOutcome is the three-state result of a connectivity test — the view
// mirror of internal/tester's Outcome. It REPLACES every use of
// tester.Outcome inside this package. There are exactly three states; a
// fourth would be a design change, not an implementation detail.
type TestOutcome int

const (
	// TestOutcomePass is a successful authentication against the provider.
	TestOutcomePass TestOutcome = iota
	// TestOutcomeReachableNotUploaded means the provider answered but
	// rejected the key — reachable, key not registered yet. This is the
	// D-02 WARNING state, never a failure.
	TestOutcomeReachableNotUploaded
	// TestOutcomeFailure is everything else: unreachable host, timeout,
	// misconfiguration.
	TestOutcomeFailure
)

// TestResultView is one connectivity test's outcome as the create wizard
// renders it (TEST-01/TEST-02). It REPLACES every use of tester.Result
// inside this package.
type TestResultView struct {
	// Outcome classifies the attempt.
	Outcome TestOutcome
	// Command is the exact connectivity command that was run — TEST-01's
	// shown == run contract, so the string displayed is never hand-built
	// separately from the one executed.
	Command string
	// Detail is the real ssh connectivity output line surfaced to the user
	// (the "Hi <name>! You've successfully authenticated…" greeting, the
	// rejection line, or the raw connectivity output).
	Detail string
	// ResolutionCommand is the exact ssh -G command that was run to prove the
	// alias resolves through the staged config to the expected key (CR-05/CR-06).
	// Empty for stage-1 (which has no resolution check).
	ResolutionCommand string
	// ResolutionOutput is the raw stdout of the ssh -G command, providing the
	// complete effective-field proof (user, hostname, port, identitiesonly,
	// identityfile) that the UI can render verbatim (CR-05/CR-06).
	ResolutionOutput string
}

// CreateSpec is the create wizard's current SSH values, handed to the
// Backend for every create-flow effect. It carries only what the user has
// typed plus what the wizard derived from it — never a backend handle.
type CreateSpec struct {
	// Identity is the resolved identity name (the alias prefix, or the
	// provider host when the prefix is left blank).
	Identity string
	// Provider is the provider host inferred from the SSH Host alias and
	// the backend's known-provider table (D-20). It is preserved here so
	// multi-label/custom providers are not later truncated by a two-label
	// suffix reconstruction (WR-03).
	Provider string
	// Alias is the SSH Host alias the managed block declares.
	Alias string
	// Hostname is the real endpoint behind the alias.
	Hostname string
	// Port is the SSH port as typed (443 for the alt-SSH endpoint).
	Port string
	// KeyPath is the per-identity key the block points at.
	KeyPath string
	// Algorithm is the selected key algorithm id.
	Algorithm string
	// SimulateFailure is the D-16 demo control on the wizard's test step:
	// the dummy uses it to preview the error path. The real Backend
	// ignores it — a real test never simulates.
	SimulateFailure bool
	// ReuseKeyPath is the D-10 "reuse an existing key" selection: non-empty
	// means the create must NOT generate a new key pair, it must point the
	// identity at this existing key instead (KEY-06). Empty means generate.
	ReuseKeyPath string
}

// GitSpec is the UI-local request for a per-identity Git configuration.
// It deliberately contains only form values and rendered originals; cmd/gitid
// converts it to the filesystem transaction inputs at the backend boundary.
type GitSpec struct {
	// Identity is the identity name the fragment and includeIf are keyed by.
	Identity string
	// Name is user.name.
	Name string
	// Email is user.email — kept byte-identical to the allowed_signers entry.
	Email string
	// Strategy is the includeIf match strategy (gitdir/hasconfig/both).
	Strategy string
	// KeyPath is the identity's private key, retained only to read its public half.
	KeyPath string
	// PublicKeyPath is the user.signingkey path shown and written by this flow.
	PublicKeyPath string
	// SSHHost is the exact configured SSH alias used by hasconfig matching.
	SSHHost string
	// Provider is the provider hostname used for its optional insteadOf block.
	Provider string
	// GitDir is the editable gitdir includeIf path, retaining its trailing slash.
	GitDir string
	// ForceSSH requests the provider-level HTTPS-to-SSH rewrite. False never
	// removes another identity's managed rewrite.
	ForceSSH bool
	// Original is the parsed state used for truthful edit-mode review diffs.
	Original GitOriginal
}

// GitOriginal is the prior parsed Git state carried by the reusable UI flow.
// It contains rendered text rather than backend types to preserve tuikit's
// backend boundary.
type GitOriginal struct {
	Fragment       string
	IncludeIf      string
	AllowedSigners string
}

// WritePlanView is what a committed create will touch: the files written
// and the timestamped backups taken first (TEST-03/D-05..D-09). The dummy
// returns fixture paths; the real Backend returns the resolved storage
// target and the real backup paths.
type WritePlanView struct {
	// Targets are the files the write touches, in write order.
	Targets []string
	// Backups are the timestamped backup paths taken before writing.
	Backups []string
	// CreatedDirs are directories the transaction creates if absent (CR-12):
	// e.g. a from-scratch ~/.gitconfig.d/, ~/git/<identity>/, or ~/.ssh.
	// Empty when every directory the write touches already exists.
	CreatedDirs []string
}

// WizardCommitMsg reports the result of an asynchronous create commit. It is
// delivered from the tea.Cmd returned by Backend.CommitCreate — success carries
// the real timestamped backup paths, failure carries the concrete error string.
type WizardCommitMsg struct {
	// Backups are the timestamped backup paths the transaction took.
	Backups []string
	// Err is non-empty when the transaction failed; the ceremony renders it
	// and offers retry/cancel instead of a receipt.
	Err string
}

// GitCommitMsg is the standalone reusable Git-flow counterpart of
// WizardCommitMsg. Restore details stay explicit so a failed receipt can never
// claim that nothing changed when restoration itself failed.
type GitCommitMsg struct {
	Backups  []string
	Restored []string
	Err      string
}

// DeleteCommitMsg completes an asynchronous delete commit — mirroring
// GitCommitMsg's shape exactly, delivered from the tea.Cmd
// Backend.CommitDelete returns. Removed lists the artifact paths the
// transaction actually removed (fragment file, etc.), distinct from Backups
// (the timestamped backup paths taken first) and Restored (populated only on
// a rolled-back failure).
type DeleteCommitMsg struct {
	Backups  []string
	Restored []string
	Removed  []string
	Err      string
}

// ClonePrefillView is the D-14/D-15 clone pre-fill DTO: what a confirmed
// clone-name prompt hands the create wizard as its INITIAL state. D-15 is
// explicit that clone gets no second write pipeline — this is a pre-fill for
// the SAME wizard a fresh create uses, never a parallel spec type.
//
// CopiedFields names exactly which fields were COPIED verbatim from the
// source (D-14 restricts this to user.name/user.email) so the Git-step
// renderer can attach the "copied from <source> — review" flag to exactly
// those two rows and no others — every other field here was RE-DERIVED from
// the new name, never copied, even though it is also "pre-filled".
type ClonePrefillView struct {
	// SourceName is the identity the clone was derived from — the flag's
	// "copied from <source>" text names it.
	SourceName string
	// CloneName is the resolved, validated new identity name.
	CloneName string
	// AliasPrefix seeds the wizard's "Alias prefix" field — the identity/
	// prefix half of the SSH Host alias, re-derived from CloneName (D-14).
	AliasPrefix string
	// Hostname is the re-derived SSH endpoint (copied from the source
	// verbatim — the endpoint itself is not identity-scoped).
	Hostname string
	// Port is the re-derived SSH port, as a string for the form field.
	Port string
	// GitName/GitEmail are COPIED verbatim from the source (D-14's two
	// author fields) — the only two entries CopiedFields ever names.
	GitName  string
	GitEmail string
	// MatchStrategy is the re-derived includeIf match strategy, preserving
	// the source's match KIND (review R-23's documented divergence note).
	MatchStrategy string
	// GitDir is the re-derived gitdir path when the strategy uses one.
	GitDir string
	// ReuseKeyPath is the source's key path when the clone reuses it, empty
	// when the clone will generate a fresh key (D-16 still gates either way).
	ReuseKeyPath string
	// CopiedFields names the field identifiers ("user.name", "user.email")
	// that carry the D-14 review flag — exactly two entries, always.
	CopiedFields []string
}

// KeyCeremonyModeRotate / KeyCeremonyModeRepair are the plain-string forms of
// the D-05 routing answer IdentityPlanner.KeyActionFor returns.
const (
	KeyCeremonyModeRotate = "rotate"
	KeyCeremonyModeRepair = "repair"
)

// DeletePlanView is the render DTO both delete screens consume. Task 2 fills
// the fields; the type exists here so IdentityPlanner can name it.
type DeletePlanView struct {
	Name                  string
	Scope                 string
	Targets               []DeleteTargetView
	ProviderRewriteTarget *DeleteTargetView
	SharedKeyOwners       []string
	Hits                  []UnmanagedHitView
	Disclaimer            string
	KeyCopyPath           string
	Backups               []string
}

// DeleteTargetView mirrors identity.DeleteTarget field-for-field so the one
// conversion site in cmd/gitid is a straight field copy.
type DeleteTargetView struct {
	File  string
	Block string
	Label string
}

// UnmanagedHitView mirrors identity.UnmanagedHit field-for-field.
type UnmanagedHitView struct {
	File   string
	Line   int
	Region string
	Text   string
}

// KeyCeremonyView is the render DTO the rotate/repair ceremony consumes.
// Task 3 fills the fields; the type exists here so IdentityPlanner can name it.
type KeyCeremonyView struct {
	Mode            string
	IdentityName    string
	ProviderHost    string
	KeyPath         string
	PubKeyPath      string
	ArchivedKeyPath string
	Targets         []string
	Backups         []string
}

// KeyCommitMsg completes an asynchronous rotate or new-key commit — delivered
// from the tea.Cmd IdentityPlanner.CommitRotate / CommitNewKey return.
type KeyCommitMsg struct {
	Mode            string
	Backups         []string
	Restored        []string
	ArchivedKeyPath string
	Err             string
}

// GlobalSSHOptionState is the four-state row model the Options sub-tab
// renders (D-11/D-12). The real backend converts the globalssh engine's own
// OptionState into this DTO at the wiring boundary; the render package never
// learns the backend source-class enum.
type GlobalSSHOptionState int

const (
	// GlobalSSHNeedsAction is the zero value: the option is unset (or differs
	// from the recommendation), so applying is meaningful.
	GlobalSSHNeedsAction GlobalSSHOptionState = iota
	// GlobalSSHAlreadySet means the effective value equals the recommendation.
	GlobalSSHAlreadySet
	// GlobalSSHDiffers means the option is explicitly set to a non-recommended
	// value — a deliberate choice still flagged with the same `!` glyph (D-12
	// word state; 06-03 owns the word).
	GlobalSSHDiffers
	// GlobalSSHNotApplicable means the option does not apply on this machine
	// (wrong platform, OpenSSH too old or unverified, or nothing to verify).
	GlobalSSHNotApplicable
)

// GlobalSSHNotApplicableReason mirrors globalssh.NotApplicableReason by VALUE
// ONLY. The backend boundary forbids tuikit importing globalssh; cmd/gitid
// pins the numeric pairing so a silent renumbering cannot drift the copy.
type GlobalSSHNotApplicableReason int

const (
	// GlobalSSHReasonNone is the zero value: the row is applicable.
	GlobalSSHReasonNone GlobalSSHNotApplicableReason = iota
	// GlobalSSHReasonPlatform means the option does not exist on this OS.
	GlobalSSHReasonPlatform
	// GlobalSSHReasonVersionTooOld means the recommended value needs a newer OpenSSH.
	GlobalSSHReasonVersionTooOld
	// GlobalSSHReasonVersionUnverified means gitid could not read the OpenSSH version.
	GlobalSSHReasonVersionUnverified
	// GlobalSSHReasonNothingToVerify means IdentitiesOnly has no managed hosts to check.
	GlobalSSHReasonNothingToVerify
	// GlobalSSHReasonProbeFailed means a probe this row depends on returned
	// an error (WR-14) — no state claim is possible, so the row is
	// not-applicable rather than a selectable "needs action" the user might
	// try to "fix" on a machine that cannot verify it.
	GlobalSSHReasonProbeFailed
)

// GlobalSSHOptionView is one Options-sub-tab row as the render stack knows it.
// Provenance is a rendered LABEL string computed in cmd/gitid/wiring.go —
// the view deliberately carries no source-class enum. 06-03 owns the exact
// frozen copy wording for OneLiner/Explanation/VersionNote.
type GlobalSSHOptionView struct {
	Key                 string
	CurrentValue        string
	Provenance          string
	Recommended         string
	Risk                string
	OneLiner            string
	Explanation         string
	VersionNote         string
	ProbeError          string
	State               GlobalSSHOptionState
	NotApplicableReason GlobalSSHNotApplicableReason
	AttributedToUser    bool
	WritableToHostStar  bool
}

// Selectable is the one predicate for the toggle key, the checkbox click,
// and the checkbox glyph: only a needs-action or set-but-differs row that
// is writable to Host * and carries no probe error can be chosen.
func (o GlobalSSHOptionView) Selectable() bool {
	if o.ProbeError != "" || !o.WritableToHostStar {
		return false
	}
	return o.State == GlobalSSHNeedsAction || o.State == GlobalSSHDiffers
}

// GlobalSSHApplyPlanView is the confirmed-apply preview scene: the resolved
// targets, the promised backup paths, and the diff the ceremony previews.
// ShadowWarnings is filled by 06-04's pre-write simulation.
// SimulationInconclusive is true when the simulation could not faithfully
// reproduce the config graph; SimulationNote carries the human-readable reason.
type GlobalSSHApplyPlanView struct {
	Targets                []string
	Backups                []string
	Diff                   string
	ShadowWarnings         []string
	SimulationInconclusive bool
	SimulationNote         string
}

// GlobalSSHCommitMsg completes an asynchronous global-SSH apply commit —
// delivered from the tea.Cmd Backend.CommitGlobalSSH returns. Restore details
// stay explicit so a failed receipt can never claim nothing changed when
// restoration itself failed. ShadowAdvisories stays empty in plan 06-01 and
// is filled by 06-04's post-write verification.
type GlobalSSHCommitMsg struct {
	Backups          []string
	Restored         []string
	ShadowAdvisories []string
	Err              string
}

// SSHStorageMigrationView is the Storage sub-tab's live preview: the resolved
// current layout, the requested target, the ceremony heading/targets/backups
// and the resulting-config bytes PlanMigration produced. The three preview
// fields exist because the pane renders one or two blocks depending on the
// layout and the render package must not compose that text from backend
// knowledge.
//
// PlanToken is how one plan reaches the commit without a backend type
// crossing the boundary. internal/tuikit must treat it as opaque: never
// parse it, never construct one, never compare it to anything but itself.
type SSHStorageMigrationView struct {
	CurrentLayout   SSHStorageLayout
	TargetLayout    SSHStorageLayout
	Heading         string
	Targets         []string
	Backups         []string
	Diff            string
	MainPreview     string
	OwnedPreview    string
	SentinelPreview string
	// SourceBefore / DestBefore are the bytes PlanMigration read, carried so
	// a concurrent-preview test can assert each managed block sits in exactly
	// one file. The render path does not display them.
	SourceBefore string
	DestBefore   string
	// PlanToken identifies the plan the backend still holds. Opaque: tuikit
	// must never parse it, never construct one, never compare it to anything
	// but itself.
	PlanToken string
}

// SSHStorageCommitMsg completes an asynchronous storage-migration commit —
// delivered from the tea.Cmd Backend.CommitSSHStorage returns. Restore details
// stay explicit so a failed receipt can never claim nothing changed when
// restoration itself failed. ConfigChangedSincePreview is what lets the pane
// render the re-open-the-preview message for that one cause without parsing
// the error string.
type SSHStorageCommitMsg struct {
	Backups                   []string
	Restored                  []string
	Err                       string
	ConfigChangedSincePreview bool
}

// ---------------------------------------------------------------------------
// Global Git view DTOs (plan 07-01)
// ---------------------------------------------------------------------------

// GlobalGitOptionState is the actionable state of one global-git option row.
// The render stack reads this; the real backend converts globalgit.OptionRowState
// into this DTO at the wiring boundary so the render package never learns the
// backend source-class enum.
type GlobalGitOptionState int

const (
	// GlobalGitNeedsAction is the zero value: the option is unset (or a bundle
	// has an unset member), so applying is meaningful.
	GlobalGitNeedsAction GlobalGitOptionState = iota
	// GlobalGitAlreadySet means the effective value equals the recommendation.
	GlobalGitAlreadySet
	// GlobalGitSetButDiffers means the option is set to a non-recommended
	// value — a deliberate choice flagged with `!` (D-02 word state). Differs
	// rows are INFORMATIONAL: gitid's block sits at the floor, so a write into
	// a key the user set later is provably a no-op. They render, they are
	// explained, and they are NOT selectable.
	GlobalGitSetButDiffers
	// GlobalGitNotApplicable means the option does not apply on this machine —
	// a probe failure with ReasonProbeFailed today; the reason enum keeps the
	// vocabulary open for future reasons.
	GlobalGitNotApplicable
)

// GlobalGitNotApplicableReason mirrors globalgit's not-applicable reasons by
// VALUE ONLY, following globalssh. cmd/gitid pins the numeric pairing in a
// parity test so a silent renumbering cannot drift the copy.
type GlobalGitNotApplicableReason int

const (
	// GlobalGitReasonNone is the zero value: the row is applicable.
	GlobalGitReasonNone GlobalGitNotApplicableReason = iota
	// GlobalGitReasonProbeFailed means a probe this row depends on returned an
	// error — no state claim is possible.
	GlobalGitReasonProbeFailed
)

// GlobalGitOptionView is one row of the Global Git options pane. Provenance is
// a rendered LABEL string computed in cmd/gitid/wiring.go — the view
// deliberately carries no source-class enum.
type GlobalGitOptionView struct {
	Key          string
	CurrentValue string
	Provenance   string
	Recommended  string
	OneLiner     string
	Explanation  string
	GitDefault   string
	ProbeError   string
	State        GlobalGitOptionState
	// NotApplicableReason is populated only when State is
	// GlobalGitNotApplicable. cmd/gitid lands this from the backend enum.
	NotApplicableReason GlobalGitNotApplicableReason
	// BundleAggregate is the "set / differs" summary for a bundle row's
	// current cell ("3 of 8 set, 1 differs"), rendered by the backend — it is
	// dynamic, so it must stay OUT of the copy-freeze gate.
	BundleAggregate string
	// BundlePerKeyNotes lists the per-member "yours differs — yours wins"
	// notes the detail pane shows. Each note is a frozen sentence naming a
	// member key.
	BundlePerKeyNotes []string
	// PolicyBacked is answered by the backend at the wiring boundary — tuikit
	// must never import internal/globalgit to ask PolicyFor itself (the
	// no-backend import-graph gate forbids it). True only for a key the live
	// policy table resolves.
	PolicyBacked bool
	// HasWritableMember is answered by the backend: true for a row with at
	// least one member key gitid may write (false for the fallback-author row,
	// which has none). Coupled with NeedsAction it drives Selectable().
	HasWritableMember bool
	// AttributedToUser is true when the effective value's origin is a file
	// gitid does not own (the "your choice" differs word). It is the render
	// layer's projection of the source class — the view never carries the
	// enum itself.
	AttributedToUser bool
	// VersionNote is the non-contractual "your git: X.Y" line for a
	// version-gated row's detail pane. Dynamically assembled at runtime; MUST
	// be excluded from the copy-freeze gate.
	VersionNote string
}

// Selectable reports whether this row can be toggled and have a checkbox
// rendered — the ONE predicate the toggle key, checkbox glyph render, and
// click hit-test all route through, mirroring GlobalSSHOptionView.Selectable.
//
// A row is selectable only when it is needs-action, has at least one writable
// member key (PolicyBacked && HasWritableMember), and carries no probe error.
// This makes the differs, not-applicable, probe-error, and fallback-author rows
// non-selectable BY CONSTRUCTION rather than by four separate guards.
func (o GlobalGitOptionView) Selectable() bool {
	if o.ProbeError != "" || !o.PolicyBacked || !o.HasWritableMember {
		return false
	}
	return o.State == GlobalGitNeedsAction
}

// GlobalGitApplyPlanView is the confirmed-apply preview scene: the resolved
// target, the promised backup paths, and the diff the ceremony previews.
type GlobalGitApplyPlanView struct {
	Targets []string
	Backups []string
	Diff    string
}

// GlobalGitCommitMsg completes an asynchronous global-git apply commit —
// delivered from the tea.Cmd Backend.CommitGlobalGit returns. Restored stays
// explicit so a failed receipt can never claim nothing changed when restoration
// itself failed. Advisories carries post-write floor-model notes (D-02).
type GlobalGitCommitMsg struct {
	Backups    []string
	Restored   []string
	Advisories []string
	Err        string
}

// GitFallbackAuthorView is the fallback block's current contents — the two
// fields the D9 pane seeds from on activate (D-04 / 07-UI-SPEC.md partial
// row). Empty strings mean the key is currently unset.
type GitFallbackAuthorView struct {
	Name  string
	Email string
}

// GitFallbackAuthorPlanView is the confirmed-apply preview scene for the
// fallback-author ceremony. Removal distinguishes a block-clearing apply
// from a write so the ceremony can word itself honestly.
type GitFallbackAuthorPlanView struct {
	Targets []string
	Backups []string
	Diff    string
	Removal bool
}

// GitFallbackAuthorCommitMsg completes an asynchronous fallback-author
// apply commit — delivered from the tea.Cmd Backend.CommitGitFallbackAuthor
// returns. Advisories carries the D-06 post-write precedence notes.
type GitFallbackAuthorCommitMsg struct {
	Backups    []string
	Restored   []string
	Advisories []string
	Err        string
}
