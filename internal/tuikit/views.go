package tuikit

import tea "charm.land/bubbletea/v2"

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

// SSHDirectiveView is one row of the "All directives" sub-tab's flat list
// (PROP-01): every directive `ssh -G` resolves for the Host * wildcard
// context, not just gitid's curated six-row Policy subset. Key is the
// lowercase spelling ssh -G emits (never normalized to gitid's canonical
// camelCase — that would misrepresent what the machine actually resolved).
type SSHDirectiveView struct {
	Key   string
	Value string
	// PolicyBacked is answered by the backend at the wiring boundary —
	// tuikit must never import internal/globalssh to ask PolicyFor itself
	// (the no-backend import-graph gate forbids it). True only when the
	// directive's key resolves in the live globalssh.Policy table, mirroring
	// GlobalGitOptionView.PolicyBacked's identical rule. This is what the
	// properties browser's cross-reference note renders from.
	PolicyBacked bool
}

// GitSetKeyView is one row of the "Set keys" sub-tab's flat list (PROP-02,
// 09.5-02): a git config key actually SET somewhere on the machine — never a
// catalogue of possible keys, because git's key space is open-ended and has
// no such catalogue (D-01/D-02). Key is the lowercase spelling `git config
// --list` emits. Scope and Origin carry the provenance a user needs to tell
// a system-wide value from one they set themselves.
type GitSetKeyView struct {
	Key, Value, Scope, Origin string
	// PolicyBacked is answered by the backend at the wiring boundary —
	// tuikit must never import internal/globalgit to ask PolicyFor itself
	// (the no-backend import-graph gate forbids it). True only when the
	// key resolves in the live globalgit.Policy table, mirroring
	// SSHDirectiveView.PolicyBacked's identical rule. This is what the
	// properties browser's cross-reference note renders from.
	PolicyBacked bool
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
	// GateNotMet is true only for the hard-gated row (merge.conflictstyle)
	// when the machine's git version is below the gate or unreadable — it
	// gates whether the STATIC GlobalGitConflictStyleGateNote renders
	// (07-03-PLAN.md Task 3: shown when not met, hidden when met). Kept
	// separate from VersionNote's dynamic sentence so the static half stays
	// freezable.
	GateNotMet bool
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

// GlobalGitIgnoreWiring is the four-state enum of how core.excludesfile
// relates to the managed ~/.gitignore_global this screen owns.
type GlobalGitIgnoreWiring int

const (
	// GitIgnoreWiredAtManaged means core.excludesfile points at the managed file.
	GitIgnoreWiredAtManaged GlobalGitIgnoreWiring = iota
	// GitIgnoreKeyUnset means the key is missing from the managed baseline block.
	GitIgnoreKeyUnset
	// GitIgnorePointsElsewhere means the key points at a different file.
	GitIgnorePointsElsewhere
	// GitIgnoreNoBaselineBlock means no gitid-managed baseline block exists.
	GitIgnoreNoBaselineBlock
)

// GlobalGitIgnoreView is the Global Git Ignore pane's live state.
type GlobalGitIgnoreView struct {
	Path           string
	Content        string
	Managed        bool
	DefaultContent string
	ExcludesFile   string
	Wiring         GlobalGitIgnoreWiring
}

// GlobalGitIgnoreApplyPlanView is the confirmed-apply preview: targets,
// promised backups, the canonicalized diff, and the plan token.
type GlobalGitIgnoreApplyPlanView struct {
	Targets   []string
	Backups   []string
	Diff      string
	PlanToken string
}

// GlobalGitIgnoreCommitMsg completes an asynchronous global-gitignore write.
type GlobalGitIgnoreCommitMsg struct {
	Backups             []string
	Restored            []string
	Err                 string
	ChangedSincePreview bool
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

// GitCustomKeyPlanView is the custom-key ceremony's preview scene: the
// resolved targets (main config + baseline file), the promised backup paths
// (only for a file that already exists), and the diff the ceremony
// previews — mirroring GlobalGitApplyPlanView's shape exactly, one field for
// one write target class (Phase 9.5 plan 09.5-03, PROP-03).
type GitCustomKeyPlanView struct {
	Targets []string
	Backups []string
	Diff    string
}

// GitCustomKeyCommitMsg completes an asynchronous custom-key write commit —
// delivered from the tea.Cmd Backend.CommitCustomGitKey returns. Restored
// stays explicit so a failed receipt can never claim nothing changed when
// restoration itself failed, mirroring GlobalGitCommitMsg's contract.
// Advisories carries WR-06's skipped-entry notes: an entry that could not be
// re-rendered (and was therefore dropped rather than failing the whole
// write) is named here, mirroring SSHCustomDirectiveCommitMsg's own
// Advisories field.
type GitCustomKeyCommitMsg struct {
	Backups    []string
	Restored   []string
	Advisories []string
	Err        string
}

// SSHDirectiveProofView is the render-boundary mirror of
// globalssh.DirectiveProof (Phase 9.5 plan 09.5-04, PROP-04): the
// staged-config `ssh -G` classification for one candidate SSH directive
// name/value pair. Command and Output carry the EXACT command line and its
// VERBATIM real output — never a paraphrase — because 09.5-UI-SPEC.md's
// stage-2 render requires showing the exact command, then its real output
// (TEST-01's contract, reused here rather than invented anew).
type SSHDirectiveProofView struct {
	// OK is true only when the staged probe accepted BOTH the candidate's
	// name and its value.
	OK bool
	// UnknownName is true when the candidate's OWN directive name was not
	// recognized by the locally installed OpenSSH.
	UnknownName bool
	// PreexistingError is true when a DIFFERENT directive already had a
	// problem in the current global block — never blamed on the entry just
	// submitted (D-I).
	PreexistingError bool
	// OffendingName is the lowercased directive name a failing proof named,
	// populated only when UnknownName or PreexistingError is true.
	OffendingName string
	// Command is the exact staged-config `ssh -F <staged> -G <host>`
	// command line that was run.
	Command string
	// Output is the verbatim combined output the staged probe produced.
	Output string
}

// SSHCustomDirectiveProofMsg completes the async stage-2 validate+prove
// dispatch — delivered from the tea.Cmd
// Backend.ValidateCustomSSHDirective returns. Err carries a TRANSPORT-level
// failure (the probe itself could not run); a completed classification
// (accepted, unknown name, pre-existing error, or a known-name value
// rejection) always arrives with Err empty and the outcome encoded in Proof.
type SSHCustomDirectiveProofMsg struct {
	Proof SSHDirectiveProofView
	Err   string
}

// SSHCustomDirectivePlanView is the custom-directive ceremony's preview
// scene: the resolved targets, the promised backup paths (only for a file
// that already exists), and the diff the ceremony previews — mirroring
// GlobalSSHApplyPlanView's shape for the one write target class this seam
// owns (Phase 9.5 plan 09.5-04, PROP-04). Reached ONLY after stage 2's proof
// returns OK: true.
type SSHCustomDirectivePlanView struct {
	Targets []string
	Backups []string
	Diff    string
}

// SSHCustomDirectiveCommitMsg completes an asynchronous custom-directive
// write commit — delivered from the tea.Cmd
// Backend.CommitCustomSSHDirective returns. Restored stays explicit so a
// failed receipt can never claim nothing changed when restoration itself
// failed, mirroring GlobalSSHCommitMsg's contract. Advisories carries the
// custom-directive-aware post-write re-verification's notes (the writer
// re-reads via globalssh.AllDirectives rather than the policy-gated
// globalssh.Verify, which would silently skip an arbitrary directive).
type SSHCustomDirectiveCommitMsg struct {
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

// ---------------------------------------------------------------------------
// Upload / Credentials Assist view DTOs (Phase 9, plan 09-02 tracer).
//
// These mirror internal/uploader's provider/auth concepts by VALUE ONLY —
// this package never imports internal/uploader (the no-backend-import rule
// at the top of this file names it explicitly). cmd/gitid/wiring.go is the
// one conversion site.
// ---------------------------------------------------------------------------

// UploadEligibilityState is the D-01 four-scenario checkbox state: whether
// autonomous upload is offered at all, and if so, in which of the three
// visible shapes (ready/unauth/disabled).
type UploadEligibilityState int

const (
	// UploadEligibilityOmitted means the identity's provider host is not
	// one of D-13's gated main domains (github.com/gitlab.com or a
	// subdomain) — the checkbox row does not render at all, and no
	// provider subprocess is ever invoked to answer this. Omitted must not
	// render an empty row because even a blank row consumes the 100x30 budget.
	UploadEligibilityOmitted UploadEligibilityState = iota
	// UploadEligibilityDisabled means the provider is gated, but neither
	// gh nor glab (whichever matches) was found on PATH.
	UploadEligibilityDisabled
	// UploadEligibilityUnauth means the matching tool is present but not
	// authenticated for this host — the row renders unchecked but
	// toggleable.
	UploadEligibilityUnauth
	// UploadEligibilityReady means the matching tool is present and
	// authenticated — the row renders pre-checked.
	UploadEligibilityReady
)

// UploadEligibilityView is the wizard's live answer to "can this identity's
// key be uploaded autonomously right now" — resolved OFF the render path
// (Backend.UploadEligibility is async; see backend.go) and cached per
// provider key.
type UploadEligibilityView struct {
	State UploadEligibilityState
	// ProviderName is the display form ("GitHub"/"GitLab") the checkbox
	// label's %s verb interpolates.
	ProviderName string
	// ToolName is the resolved CLI name ("gh"/"glab") the unauth label's
	// %s verbs interpolate.
	ToolName string
	// Hostname is the canonical host ("github.com"/"gitlab.com") the
	// unauth label's remaining %s verb interpolates.
	Hostname string
}

// UploadEligibilityMsg is the asynchronous answer Backend.UploadEligibility
// delivers. Hostname carries the ORIGINAL hostname the caller probed
// for — never the canonicalized value — so a stale reply (the user changed
// the host before this arrived) can be discarded by comparing against
// whatever host the wizard most recently dispatched with, mirroring the
// existing KeyCommitMsg stale-guard idiom.
type UploadEligibilityMsg struct {
	Hostname string
	View     UploadEligibilityView
}

// UploadRegistration identifies WHICH key role a result row is about.
// gh registers authentication and signing separately; glab registers one
// combined key. This is a named seam: plan 09-03 adds the signing row,
// 09-04 the combined row — no later plan reshapes this type.
type UploadRegistration int

const (
	// UploadRegistrationAuthentication is gh's authentication-key registration.
	UploadRegistrationAuthentication UploadRegistration = iota
	// UploadRegistrationSigning is gh's signing-key registration.
	UploadRegistrationSigning
	// UploadRegistrationCombined is glab's single auth_and_signing registration.
	UploadRegistrationCombined
)

// UploadRowOutcome is one registration attempt's result.
type UploadRowOutcome int

const (
	// UploadRowUploaded means the key was newly registered.
	UploadRowUploaded UploadRowOutcome = iota
	// UploadRowAlreadyPresent is the D-15 idempotent-dedupe outcome.
	UploadRowAlreadyPresent
	// UploadRowFailed means the registration attempt failed.
	UploadRowFailed
)

// UploadResultRow is one registration's shown-and-run record: the label,
// the exact command that was run (shown==run, UP-02), the outcome, and the
// reason text — on a UploadRowFailed outcome, the classified failure
// reason; on UploadRowUploaded/UploadRowAlreadyPresent, an OPTIONAL D-17
// post-upload confirmation note (UploadUnconfirmedReasonFmt) set when the
// registration was accepted but gitid's own re-check still could not see it
// (CR-01, review iteration 5) — never a failure, so it is rendered as an
// extra continuation row, not in place of the row's own success line.
type UploadResultRow struct {
	Registration UploadRegistration
	Label        string
	Command      string
	Outcome      UploadRowOutcome
	Reason       string
}

// UploadRunView is the D-02 announce-and-do beat's full result: one row per
// attempted registration. InventoryDegraded, ManualFallback, and Skipped are
// named seams later plans fill (D-15 inventory-read failure, the manual
// instructions text, and a --no-upload-style skip) — declared now so no
// later plan reshapes this struct's field set.
//
// Skipped vs SkippedByFlag (WR-02): Skipped means autonomy DID NOT APPLY
// here — planUpload sets it for two DERIVED terminal states that were never
// a user choice: the provider is not gated (Omitted) or no matching
// provider CLI is on PATH (Disabled, which also carries ManualFallback).
// SkippedByFlag means the OPPOSITE: the user explicitly passed --no-upload,
// the one state where "Auto-upload skipped (--no-upload)." is actually
// true. The two must never share a rendering condition — a self-hosted
// GHE/GitLab create (Omitted, Skipped=true, SkippedByFlag=false) must never
// tell the user they passed a flag they did not pass.
type UploadRunView struct {
	Rows              []UploadResultRow
	InventoryDegraded bool
	ManualFallback    string
	Skipped           bool
	SkippedByFlag     bool
	AlreadyComplete   bool
	// ProviderName is the display name (e.g. "GitHub") the degraded and
	// already-complete notes format themselves with. The wizard's own
	// render path (identities.go) reads its provider name from the
	// separate uploadEligibility field it already carries; this field
	// exists so a caller with ONLY a UploadRunView in hand — the CLI's
	// printUploadOutcome (09-05-PLAN.md Task 1) — can render the same two
	// frozen lines without a second source of truth.
	ProviderName string
}

// UploadStartedMsg is delivered before any provider registration command runs.
// FollowUp executes only after the model has rendered Commands, preserving
// D-02's announce-before-run contract.
type UploadStartedMsg struct {
	Commands []string
	FollowUp tea.Cmd
}

// UploadRunMsg completes the asynchronous upload beat Backend.RunUpload/
// Backend.RunUploadForIdentity dispatches — the wizard renders View's rows,
// then auto-advances into the existing test-stage gate with no user
// keystroke (D-02). Name is the identity this run is about ("" for the
// create wizard, since no identity exists yet) — it lets a consumer discard
// a stale reply from a DIFFERENT identity's still-in-flight upload beat
// (WR-01, review iteration 3): the beat is multi-second, and an in-flight
// command cannot be cancelled when the user navigates away, so a reply
// outliving its originating pane state is a real, not hypothetical, race.
// Mirrors the existing RegisterKeyPlanMsg.Name / UploadEligibilityMsg.Hostname
// stale-guard idiom.
type UploadRunMsg struct {
	Name string
	View UploadRunView
}

// RegisterKeyPlanMsg completes the asynchronous eligibility probe
// Backend.RegisterKeyPlan dispatches for the D-08 register-key pane. Name
// carries the identity the probe was resolved for, so a reply arriving
// after the user has navigated to a different identity can be discarded
// (mirrors the existing KeyCommitMsg stale-guard idiom). A non-nil Err
// fails closed: the pane renders the error and never dispatches an upload.
type RegisterKeyPlanMsg struct {
	Name string
	View UploadEligibilityView
	Err  error
}

// RotateDeleteOfferView is D-04's answer to "can the old remote key be
// offered for removal right now, and if so, what is it": Available=false
// plus a populated Unavailable reason means the offer does not render at
// all and the caller falls back to the existing frozen grace-window hint —
// an inventory-read failure, no matching key, or a non-qualifying provider
// all take this path (fail CLOSED on the destructive offer, never fail
// open). KeyID is the FRESH, machine-scoped-exact-match provider ID(s) the
// confirmed delete will remove — resolved once, at result-screen time, never
// re-resolved after the user reviews it (D-04, review R12). Because
// internal/tuikit never imports internal/uploader (this file's no-backend-
// import rule), KeyID is an OPAQUE string the backend encodes and decodes on
// its own: it may carry more than one provider registration (GitHub records
// a separate authentication AND signing entry for the same physical key, so
// a rotate's delete offer legitimately deletes more than one), and tuikit
// never parses it — it only round-trips the value through
// CommitRotateDeleteOldKey unread. KeyDetail is the SEPARATE, human-readable
// counterpart: the reviewed ID(s) plus a short key-blob suffix, rendered so
// the confirmation identifies the exact target(s) rather than only the
// title the old and new keys share (CR-01).
type RotateDeleteOfferView struct {
	Available    bool
	ProviderName string
	// IdentityName/MachineName are KeyTitle's two components, carried
	// separately so the D-04 body format (RotateDeleteOfferBodyFmt, whose
	// two %s verbs are the identity name and the machine name) can
	// interpolate them directly rather than re-parsing the assembled title.
	IdentityName  string
	MachineName   string
	KeyTitle      string
	KeyID         string
	KeyDetail     string
	ManualCommand string
	Unavailable   string
}

// RotateDeleteOfferMsg completes the asynchronous D-04 offer probe
// Backend.RotateDeleteOffer dispatches when the rotate result screen is
// reached. Name carries the identity the probe was resolved for so a reply
// arriving after the user has navigated away can be discarded, mirroring
// RegisterKeyPlanMsg's stale-guard shape.
type RotateDeleteOfferMsg struct {
	Name string
	View RotateDeleteOfferView
}

// RotateDeleteCommitMsg completes the ONE remotely-destructive call this
// phase makes, Backend.CommitRotateDeleteOldKey. A non-empty Err means the
// delete failed and (per review R12) the confirmed target must remain
// available on the model for exactly one retry — this message alone never
// decides retention; the caller keeps or clears the confirmed pair.
//
// Name (WR-01, review iteration 5) carries the identity this delete was
// dispatched for, mirroring the stale-guard shape every other Phase-9 async
// reply already carries (RegisterKeyPlanMsg.Name, RotateDeleteOfferMsg.Name,
// UploadRunMsg.Name). This is the MOST consequential message in the phase
// to leave unguarded: a stale reply does not merely display stale text — it
// rewrites rotateDeleteConfirmedID, the ID set the NEXT destructive retry
// sends. Reachability was defensive-only at the time this was found (a
// key-swallowing branch elsewhere made the pending window practically
// uninterruptible), but that invariant was accidental, not stated — the
// same shape as the UploadRunMsg race review iteration 3 already treated as
// real.
//
// RemainingKeyID (WR-09, review iteration 3) is the SAME opaque encoding
// rotateDeleteConfirmedID already carries, narrowed to only the candidates
// that did NOT delete successfully — never the full original set. A rotated
// key can carry more than one registration (authentication + signing); when
// Err is non-empty because only SOME of them failed, re-sending the full
// original set on retry re-attempts an already-deleted candidate, which the
// provider now reports as gone (a permanent 404), so the retry can never
// succeed and the "✓ Old key removed" state becomes unreachable even once
// the goal state is in fact true. Only meaningful when Err != "" — the
// success path clears the confirmed pair entirely and never reads this
// field.
type RotateDeleteCommitMsg struct {
	Name           string
	Err            string
	RemainingKeyID string
}
