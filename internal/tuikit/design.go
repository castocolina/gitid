package tuikit

import "strconv"

// design.go holds the frozen DESIGN vocabulary the render stack draws
// through — state/severity taxonomies, the option catalogs the Global SSH
// and Global Git screens enumerate, the key-algorithm catalog, and the
// managed-block sentinel format. It was split out of internal/dummytui's
// data.go during the D-17 extraction: the recipe FIXTURES (identity rows,
// health findings, create-flow literals) stayed in the dummy, while the
// values below — which every renderer references directly and which both
// binaries must render identically — moved here with their values
// unchanged, byte for byte.
//
// Nothing in this file is machine state: it is copy and taxonomy. Anything
// that has to be READ from the user's machine reaches the render stack
// through the Backend seam (backend.go), never from a package-level var.

// ---------------------------------------------------------------------------
// Managed blocks (CLAUDE.md "Engineering": idempotent managed blocks, never
// a blind append).
// ---------------------------------------------------------------------------

// ManagedBlockSentinels returns the BEGIN/END sentinel pair delimiting the
// managed block gitid owns for identityName — the Go mirror of
// recipeFixtures.ts's managedBlockSentinels.
func ManagedBlockSentinels(identityName string) (begin, end string) {
	return "# BEGIN gitid managed: " + identityName, "# END gitid managed: " + identityName
}

// ---------------------------------------------------------------------------
// Key algorithms (KEY-01 catalog).
// ---------------------------------------------------------------------------

// AlgorithmCatalogEntry mirrors recipeFixtures.ts's AlgorithmCatalogEntry
// shape (KEY-01's top-5 catalog; ed25519 is best/default, KEY-03's
// macOS/Linux local-availability notes for the other four).
//
// Implemented and Available are explicit, orthogonal flags so rendering and
// selection never have to infer availability from note text (CR-03).
type AlgorithmCatalogEntry struct {
	ID          string
	Security    string
	MacOS       string
	Linux       string
	Recommended bool
	Implemented bool
	Available   bool
}

// ValidationError is a field-keyed error returned by Backend.ValidateHostBlock.
// The SSH form maps Field to the offending control and renders Message inline.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string { return e.Message }

// AlgorithmCatalog is the Go mirror of recipeFixtures.ts's
// algorithmCatalog — the KEY-01 top-5 key-algorithm catalog. It is the
// DEFAULT catalog a Backend may return from AlgorithmCatalog(); the real
// binary is free to return a machine-probed subset of the same shape.
var AlgorithmCatalog = []AlgorithmCatalogEntry{
	{
		ID:          "ed25519",
		Recommended: true,
		Implemented: true,
		Available:   true,
		Security:    "Modern EdDSA curve — small keys, fast, constant-time (timing-attack resistant). The recommended default.",
		MacOS:       "Native (LibreSSL) — always available",
		Linux:       "Native (OpenSSL) — always available",
	},
	{
		ID:          "ed25519-sk",
		Implemented: false,
		Available:   true,
		Security:    "Hardware-backed: private key material never leaves the security key; requires a physical touch to sign.",
		MacOS:       "Needs libfido2 + a FIDO2 security key",
		Linux:       "Needs libfido2 + a FIDO2 security key",
	},
	{
		ID:          "rsa-4096",
		Implemented: true,
		Available:   true,
		Security:    "Strong at 4096 bits; widely compatible, larger keys and slower signing than ed25519.",
		MacOS:       "Native — always available",
		Linux:       "Native — always available",
	},
	{
		ID:          "ecdsa-p256",
		Implemented: false,
		Available:   true,
		Security:    "Compact NIST P-256 curve; smaller than RSA, though some users distrust NIST curve provenance versus ed25519.",
		MacOS:       "Native — always available",
		Linux:       "Native — always available",
	},
	{
		ID:          "ecdsa-sk",
		Implemented: false,
		Available:   true,
		Security:    "Hardware-backed ECDSA variant of ed25519-sk; physical security-key touch required.",
		MacOS:       "Needs libfido2 + a FIDO2 security key",
		Linux:       "Needs libfido2 + a FIDO2 security key",
	},
}

// ---------------------------------------------------------------------------
// Identity state taxonomy (MGR-02's 8 labels).
// ---------------------------------------------------------------------------

// IdentityManagerGlyphByState pairs each MGR-02 label with its
// color-semantics glyph (02-UX-DIRECTION.md §2: healthy=✓,
// needs-action/advisory=!, error/destructive/missing=✗) — always rendered
// together with the state's own WORD (never color alone, the
// NO_COLOR-legibility requirement).
var IdentityManagerGlyphByState = map[string]string{
	"complete":              "✓",
	"incomplete":            "!",
	"git-only":              "!",
	"key-unused":            "!",
	"key-used-ssh-only":     "✓",
	"key-used-both":         "✓",
	"key-missing":           "✗",
	"fragment-path-missing": "✗",
}

// IdentityManagerStateTone pairs each MGR-02 label with its health tone
// (success/warning/error) — the Go mirror of recipeFixtures.ts's
// identityManagerStateTone. The tone colors the state glyph; the S/G
// capability pips carry capability separately (02-REDESIGN-SPEC.md §2).
var IdentityManagerStateTone = map[string]string{
	"complete":              "success",
	"incomplete":            "warning",
	"git-only":              "warning",
	"key-unused":            "warning",
	"key-used-ssh-only":     "success",
	"key-used-both":         "success",
	"key-missing":           "error",
	"fragment-path-missing": "error",
}

// MGR-06 delete-scope choices — byte-identical to recipeFixtures.ts's
// identityManagerDeleteChoices.
const (
	// IdentityManagerDeleteChoiceGitOnly is the safe-default delete scope.
	IdentityManagerDeleteChoiceGitOnly = "Delete Git identity only"
	// IdentityManagerDeleteChoiceEverything is the full destructive delete
	// scope (SSH + Git + key).
	IdentityManagerDeleteChoiceEverything = "Delete everything (SSH + Git + key)"

	// IdentityManagerActionViewDetail is the action-menu's first approved row.
	IdentityManagerActionViewDetail = "View SSH-first detail"
	// IdentityManagerActionClone is the action-menu's second approved row.
	IdentityManagerActionClone = "Clone (c)"
	// IdentityManagerActionNewKey is the action-menu's third approved row —
	// one label that routes to rotate or repair from classified state.
	IdentityManagerActionNewKey = "Generate new key"
	// IdentityManagerActionDelete is the action-menu's fourth approved row.
	IdentityManagerActionDelete = "Delete (d)"

	// DeleteSharedKeyNotePrefix is the D-12 downgrade-note lead-in. Sibling
	// names are comma-joined after it (plural-safe; review R-17).
	DeleteSharedKeyNotePrefix = `This key is also used by `
	// DeleteSharedKeyNoteSuffix is the D-12 downgrade-note close.
	DeleteSharedKeyNoteSuffix = ` — it will be kept. Only this identity's SSH and Git artifacts are removed.`
	// DeleteSharedKeyMoreFmt is the R-17 remaining-count suffix. The count is
	// the format argument.
	DeleteSharedKeyMoreFmt = `(+%d more)`
	// DeleteScanHitFmt is one D-13 unmanaged-reference hit line. Args: alias,
	// file, line.
	DeleteScanHitFmt = `Found %q referenced in %s: %d — review before continuing.`
	// DeleteScanHitNeedle is the stable substring tests use to count hit lines.
	DeleteScanHitNeedle = `referenced in`
	// DeleteCannotBeUndone is the confirm-destructive heading, scoped to
	// managed-block removals (D-11) — never applied to a recoverable key copy.
	DeleteCannotBeUndone = `This action is irreversible`
	// DeleteKeyRemovedFmt is the D-11 key-copy sentence. Args: identity name,
	// key-copy path.
	DeleteKeyRemovedFmt = `%s will be removed from active use; a copy of the key pair exists at %s.`
	// DeleteKeyCopyNeedle is the stable D-11 substring.
	DeleteKeyCopyNeedle = `will be removed from active use`

	// IdentityManagerEmptyStateCopy is the frozen list-empty landing copy
	// (identity-manager/FIELDS.md's `empty_state_copy` field) — the true
	// first-run state, rendered instead of a blank list when the sandbox
	// home has no identities at all (05-09-PLAN.md Task 1).
	IdentityManagerEmptyStateCopy = "No identities yet"
	// IdentityManagerEmptyStateCTA is the frozen list-empty call to action
	// (identity-manager/FIELDS.md's `empty_state_cta` field), pointing at
	// create-flow's `n` LaunchKey.
	IdentityManagerEmptyStateCTA = "Press n to create your first identity"
)

// ---------------------------------------------------------------------------
// Global SSH options (GSSH-01's dangerous-by-default catalog).
// ---------------------------------------------------------------------------

// GlobalSSHOption mirrors recipeFixtures.ts's GlobalSSHOption shape — one
// entry per GSSH-01 dangerous-by-default option.
type GlobalSSHOption struct {
	Key         string
	Current     string
	Risk        string
	Recommended string
	OneLiner    string
	NeedsAction bool
}

// GlobalSSHOptions is the Go mirror of recipeFixtures.ts's
// globalSshOptions — byte-identical keys/values/one-liners, not derived (a
// static, diff-able contract). Order matches 02-UX-DIRECTION.md §4.4's
// verbatim list.
var GlobalSSHOptions = []GlobalSSHOption{
	{Key: "StrictHostKeyChecking", Current: "not set (OpenSSH default: ask)", Risk: "Medium", Recommended: "accept-new", NeedsAction: true, OneLiner: "accept-new pins first-seen keys and hard-fails on a changed key; it requires OpenSSH 7.6 or newer."},
	{Key: "ForwardAgent", Current: "not set (OpenSSH default: no)", Risk: "High", Recommended: "no", NeedsAction: true, OneLiner: "Globally forwarding your agent lets any host you connect to authenticate elsewhere as you."},
	{Key: "HashKnownHosts", Current: "not set", Risk: "Low", Recommended: "yes", NeedsAction: true, OneLiner: "Hashing known_hosts hides which hosts you connect to if the file ever leaks."},
	{Key: "IdentitiesOnly", Current: "not set globally (set per-Host by gitid)", Risk: "High", Recommended: "yes", NeedsAction: true, OneLiner: "Without it, ssh may offer every key it knows about to every host — leaking which OTHER keys you hold."},
	{Key: "AddKeysToAgent", Current: "yes", Risk: "Low", Recommended: "yes", NeedsAction: false, OneLiner: "Keeps keys available in the agent for the session (recipes/ssh-config.recipe Host * block)."},
	{Key: "UseKeychain", Current: "yes (macOS only)", Risk: "Low", Recommended: "yes", NeedsAction: false, OneLiner: "Stores the key passphrase in the macOS Keychain (guarded by IgnoreUnknown on Linux)."},
}

// GlobalSSHDetailExplanation is GSSH-01's contractual (verbatim, §3)
// explanation copy — byte-identical to recipeFixtures.ts's
// globalSshDetailExplanation.
const GlobalSSHDetailExplanation = `When IdentitiesOnly is not set (or set to "no"), ssh may try EVERY key it can find -- every file in ~/.ssh matching the default names, plus every key already loaded in your ssh-agent -- against any host you connect to. On a machine with multiple identities (personal, work, client keys), this means:

  - the wrong key can be offered first, revealing to a server which OTHER keys you hold;
  - a host you don't fully trust can trigger authentication attempts meant for a completely different identity.

Setting "IdentitiesOnly yes" on a Host block restricts ssh to ONLY the IdentityFile(s) listed for that host -- this is why every gitid-managed Host block (recipes/ssh-config.recipe) already sets it per-identity. This screen recommends also stating it explicitly in the global Host * block, as a safety net for any Host entries gitid does not manage.`

// GlobalSSHAdvisoryNote — byte-identical to recipeFixtures.ts's
// globalSshAdvisoryNote. Recommendations are ADVISORY, never blocking.
const GlobalSSHAdvisoryNote = "Recommended, not required -- you can leave any option unchanged. This is advisory, never a compliance gate."

// Frozen 06-03 copy: D-12 differs words, the safe-by-default already-set
// phrasing, and the four not-applicable reason sentences (D-11/D-13).
const (
	// GlobalSSHWordDiffersUser is the D-12 line-2 word when gitid parsed the value.
	GlobalSSHWordDiffersUser = "set, differs — yours, would be a no-op here"
	// GlobalSSHWordDiffersOutside is the D-12 line-2 word when the value came from outside gitid's files.
	GlobalSSHWordDiffersOutside = "set, differs — external, would be a no-op here"
	// GlobalSSHWordAlreadySet is the line-2 word when the value was set somewhere and equals the recommendation.
	GlobalSSHWordAlreadySet = "already set"
	// GlobalSSHWordSafeByDefault is the line-2 word when OpenSSH's own default already equals the recommendation.
	GlobalSSHWordSafeByDefault = "safe by default"
	// GlobalSSHNAPlatform is the D-11 not-applicable sentence (UseKeychain off macOS).
	GlobalSSHNAPlatform = "not applicable (macOS-only setting)"
	// GlobalSSHNAVersionTooOld is the D-13 not-applicable sentence when OpenSSH is below the minimum.
	GlobalSSHNAVersionTooOld = "not applicable (OpenSSH too old for accept-new)"
	// GlobalSSHNAVersionUnverified is the D-13 not-applicable sentence when ssh -V could not be read.
	GlobalSSHNAVersionUnverified = "not applicable (OpenSSH version could not be verified)"
	// GlobalSSHNANothingToVerify is the IdentitiesOnly sentence when no managed hosts exist.
	GlobalSSHNANothingToVerify = "not applicable (nothing on this machine to verify)"
	// GlobalSSHNAProbeFailed is the WR-14 not-applicable sentence when a probe
	// this row depends on returned an error — the row's own ProbeError line
	// carries the detail; this is the master-list summary.
	GlobalSSHNAProbeFailed = "not applicable (could not be probed)"
)

// ---------------------------------------------------------------------------
// Global Git options (GGIT-01's baseline/recipe-default catalog).
// ---------------------------------------------------------------------------

// D9 (checkpoint-2 contract) frozen copy for the promoted, editable
// global-fallback user.email row — byte-exact; shared by globalgit.go's
// detail render, apply checkbox, and the dedicated apply ceremony. This is
// a DOCUMENTED, CONSCIOUS divergence from recipes/ (which leave user.email
// unset by default) — recorded in FIELDS.md + 02-STYLE-SPEC.md (Task 3).
const (
	// GlobalGitNameFallbackKey is the D-04 sibling of GlobalGitEmailFallbackKey
	// — the user.name half of the two-field fallback pair. Same wording
	// shape; registered in gate-copy-freeze alongside its sibling. The six
	// existing D9 constants need no rewrite (07-UI-SPEC.md: D-04 is a
	// behavioural amendment, not a copy amendment).
	GlobalGitNameFallbackKey = "user.name (global fallback)"
	// GlobalGitEmailFallbackKey is the row label AND the frozen copy the
	// copy-freeze grep requires present in both demos.
	GlobalGitEmailFallbackKey = "user.email (global fallback)"
	// GlobalGitEmailFallbackHelper is the always-visible helper line —
	// byte-exact, ONE line.
	GlobalGitEmailFallbackHelper = "Fallback author for repos no identity matches. Identities always override this through their includeIf fragment — setting it never changes an identity's author."
	// GlobalGitEmailFallbackAdvisory is the always-visible advisory line —
	// byte-exact, ONE line.
	GlobalGitEmailFallbackAdvisory = "Recipes leave this unset by default. Set it only if you want a catch-all author for unmatched repos."
	// GlobalGitEmailCeremonyHeading is the dedicated apply ceremony's
	// heading (distinct from the baseline managed-block ceremony).
	GlobalGitEmailCeremonyHeading = "Set global fallback user.email"
	// GlobalGitEmailDiffAnnotation is spliced onto the ceremony's diff
	// preview line, pinning the includeIf-precedence invariant.
	GlobalGitEmailDiffAnnotation = "(global fallback — identities override via includeIf)"
	// GlobalGitEmailResultMessage is the ceremony's receipt message —
	// pins the SAME includeIf-precedence invariant.
	GlobalGitEmailResultMessage = "Global fallback user.email set — used only where no identity matches; identity fragments still win."

	// GlobalGitCaseSensitivityCaveat is appended to core.ignorecase's
	// explanation (07-03-PLAN.md Task 3): git's own init/clone filesystem
	// probe can write a repository-local override that beats this global
	// setting, so recommending it without saying so would be dishonest.
	GlobalGitCaseSensitivityCaveat = "git's own init and clone commands probe the filesystem and may write a repository-local core.ignorecase that overrides this global setting."

	// GlobalGitConflictStyleGateNote is the STATIC half of the D-08 hard-gate
	// explanation for merge.conflictstyle — it names the required git version
	// and states the fallback is written instead, but NEVER the machine's
	// actual version (that lives on its own separate dynamic VersionNote
	// line, internal/globalgit.VersionGate, excluded from this gate per the
	// Phase 6 D-13 precedent — splitting the two is what makes this half
	// freezable). Shown only when the gate is NOT met.
	GlobalGitConflictStyleGateNote = "Requires git 2.35 or newer to write zdiff3 — on older git, gitid writes diff3 instead (old git errors on an unrecognized merge style value)."

	// GlobalGitCrossWarningNameMissing and GlobalGitCrossWarningEmailMissing
	// are D-07's mandatory cross-warning, as TWO fully-static constants (one
	// per missing half) rather than one interpolated sentence — an
	// interpolated sentence would only be half-frozen. Shown when the
	// user.useConfigOnly row is selected and exactly one fallback-author half
	// is set.
	GlobalGitCrossWarningNameMissing  = "user.useConfigOnly is selected but the fallback author has no name set — a commit with no matching identity will hard-fail instead of falling back, because only the email half is configured."
	GlobalGitCrossWarningEmailMissing = "user.useConfigOnly is selected but the fallback author has no email set — a commit with no matching identity will hard-fail instead of falling back, because only the name half is configured."

	// GlobalGitGuessedNameWarning fires independently of user.useConfigOnly's
	// selection state: a fallback email with no fallback name means git will
	// guess the commit author's NAME from the OS account while using the
	// explicit fallback email — the exact half-works-by-construction problem
	// D-04 exists to fix, still possible while useConfigOnly is off.
	GlobalGitGuessedNameWarning = "The fallback email is set but the fallback name is empty — git will guess the author name from your OS account for any commit that falls back to this email."
)

// GlobalGitOption mirrors recipeFixtures.ts's GlobalGitOption shape — one
// entry per GGIT-01 baseline/recipe-default option.
type GlobalGitOption struct {
	Key         string
	Current     string
	Recommended string
	OneLiner    string
	NeedsAction bool
	Highlight   bool // main-vs-master (GGIT-01's own dedicated highlight)
}

// GlobalGitOptions is the Go mirror of recipeFixtures.ts's
// globalGitOptions — byte-identical keys/values/one-liners, not derived (a
// static, diff-able contract). Order matches 02-UX-DIRECTION.md §4.5's
// verbatim list.
var GlobalGitOptions = []GlobalGitOption{
	{Key: "init.defaultBranch", Current: "not set (git's built-in default: master)", Recommended: "main", NeedsAction: true, Highlight: true, OneLiner: "Distros still default new repos to \"master\"; main matches the modern GitHub/GitLab default without renaming existing repos."},
	{Key: "core.ignorecase", Current: "not set (OS-dependent: true on macOS/Windows, false on Linux)", Recommended: "false", NeedsAction: true, OneLiner: "Keeps file-name case always significant, so a case-only rename is never silently ignored on a case-insensitive filesystem."},
	{Key: "core.autocrlf / core.eol", Current: "not set (line-ending handling varies by OS)", Recommended: "input / lf", NeedsAction: true, OneLiner: "Normalizes line endings to LF in the repository and on checkout, avoiding CRLF diff noise across contributors on different platforms."},
	// D9 (checkpoint-2 contract): promoted from awareness-only to a
	// first-class EDITABLE global-fallback field + apply checkbox —
	// unchecked/empty by default (recipes leave it unset; setting it is
	// explicit opt-in). NeedsAction:true so the checkbox/click plumbing
	// (shared with every other row) applies unmodified; newGlobalGitModel
	// special-cases this ONE key to stay un-chosen by default, and
	// gitApplyChosen/the "pending" status metric both exclude it from the
	// generic baseline count — it is a DOCUMENTED divergence from recipes/,
	// applied through its OWN dedicated ceremony (globalgit.go), never
	// folded into the baseline managed block.
	{Key: GlobalGitEmailFallbackKey, Current: "unset (recipes default)", Recommended: "left unset unless explicitly opted in", NeedsAction: true, OneLiner: GlobalGitEmailFallbackHelper},
	// D-07: the fail-loud companion to includeIf setups — its own opt-in
	// advisory row, unchecked by default (recipes leave it unset). Pinned
	// position: immediately after the fallback-author row (07-03-PLAN.md
	// <authority>), so D-07's cross-warning has both participants on screen
	// together. Joins the SAME baseline ceremony when selected (its [user] line
	// is in GlobalGitFullManagedBlockText) — never the fallback ceremony.
	{Key: "user.useConfigOnly", Current: "unset (recipes default)", Recommended: "true", NeedsAction: true, OneLiner: "Turns a commit with no matching identity into a hard error instead of git silently guessing an author from your OS account — the fail-loud companion to includeIf setups."},
	{Key: "push.autoSetupRemote", Current: "not set (git default: false)", Recommended: "true", NeedsAction: true, OneLiner: "Lets `git push` on a new branch set its upstream automatically, instead of requiring --set-upstream every time."},
	{Key: "pull.rebase", Current: "not set (git default: false -- merge)", Recommended: "true", NeedsAction: true, OneLiner: "Replays local commits on top of the fetched branch instead of creating a merge commit on every pull."},
	{Key: "fetch.prune", Current: "not set (git default: false)", Recommended: "true", NeedsAction: true, OneLiner: "Removes local references to remote branches that were deleted upstream, every fetch."},
	{Key: "alias (8 shortcuts)", Current: "not set", Recommended: "st, co, br, ci, df, lg, unstage, last", NeedsAction: true, OneLiner: "Short, common-workflow aliases (status, checkout, branch, commit, diff, a graph log, unstage, last commit)."},
	{Key: "color (ui/branch/diff/status)", Current: "not set (ui defaults to auto in modern git; the rest vary)", Recommended: "auto for all four", NeedsAction: true, OneLiner: "Colorizes status, branch, diff, and general UI output consistently, even where a specific subcommand's own default might differ."},
	{Key: "merge.conflictstyle", Current: "not set (git default: merge)", Recommended: "zdiff3", NeedsAction: true, OneLiner: "Shows the common ancestor plus both sides of a merge conflict (zdiff3; git >= 2.35, with a fallback to the older three-way form on older git)."},
	{Key: "diff.colorMoved", Current: "not set", Recommended: "zebra", NeedsAction: true, OneLiner: "Highlights moved blocks of code distinctly from genuine additions/deletions in colorized diffs, striping each moved block."},
}

// GlobalGitDetailExplanation is GGIT-01's contractual (verbatim, §3)
// explanation copy — byte-identical to recipeFixtures.ts's
// globalGitDetailExplanation.
const GlobalGitDetailExplanation = `Until Git 2.28 (July 2020), every new repository's default branch was named "master" -- a name inherited from Git's early conventions. GitHub, GitLab, and Bitbucket now all default new repositories to "main" instead, and many teams have followed suit for their own local defaults.

Setting init.defaultBranch = main only affects repositories created AFTER this is set -- it never renames an existing "master" branch in a repository you already have. If you clone or work in a repository whose default branch is still "master" (many older projects have not renamed it), that repository's branch is completely unaffected; this setting only decides what "git init" names the FIRST branch of a brand-new repository.

This is a naming convention, not a security or correctness fix -- it is included here because it is one of the most visible defaults a new gitid user will notice, and stating it explicitly (rather than relying on git's own compiled-in default, or a value some other tool set) keeps the choice intentional and self-documenting.`

// GlobalGitAdvisoryNote — byte-identical to recipeFixtures.ts's
// globalGitAdvisoryNote.
const GlobalGitAdvisoryNote = "Recommended, not required -- you can leave any option unchanged. This is advisory, never a compliance gate."

// Global-git write ceremony copy.
const (
	// GlobalGitSentinelBegin opens the global-git managed block.
	GlobalGitSentinelBegin = "# BEGIN gitid managed: global-git"
	// GlobalGitSentinelEnd closes the global-git managed block.
	GlobalGitSentinelEnd = "# END gitid managed: global-git"

	// GlobalGitResultTail is the FROZEN static tail of the baseline apply's
	// success message — "N of M ... applied to <target>. " is prefixed at
	// runtime with the real selected/pending counts (globalgit.go's
	// baselineCeremonyFor), so only this sentence is a fixed, freezable
	// string (07-03-PLAN.md Task 3: "freeze only the static tail"). It stays
	// TRUE by construction: the baseline ceremony never touches the
	// fallback-author block (a separate managed block, a separate ceremony),
	// proven by TestGlobalGitBaselineApplyLeavesFallbackAuthorUntouched.
	GlobalGitResultTail = "Global user.email was left alone, as always -- each identity's commits use their own includeIf fragment."
)

// Global Git Ignore screen copy (09.2-01 / 09.2-UI-SPEC.md copywriting table).
const (
	GitIgnoreHeading                    = "Global Git Ignore"
	GitIgnoreWiringWired                = "✓ Wired — core.excludesfile points at this file; Git reads it."
	GitIgnoreWiringKeyUnset             = "! core.excludesfile is not set — Git does not read any global ignore file yet. Confirming here will set it."
	GitIgnoreWiringNoBaseline           = "! No gitid-managed Git baseline configuration was found on this machine — open the Doctor to set that up before this screen can wire core.excludesfile."
	GitIgnoreNoManagedBlock             = "No managed block found yet in ~/.gitignore_global — showing the curated defaults below. Nothing has been written."
	GitIgnoreTwoTargetNote              = "This write will also set core.excludesfile in your Git baseline, since it is not set yet."
	GitIgnoreReceiptSingleTarget        = "Global gitignore written to ~/.gitignore_global."
	GitIgnoreReceiptTwoTarget           = "Global gitignore written to ~/.gitignore_global. core.excludesfile was also set to point at this file."
	GitIgnoreReceiptNoBackup            = "No backup was needed — the content was unchanged or the file is new."
	GitIgnoreReceiptChangedSincePreview = "This file changed since you last reviewed it — press a to review the current content again before writing."
	GitIgnoreCeremonyHeading            = "Review your global gitignore before writing."
	GitIgnoreEditLabel                  = "Edit"
	GitIgnoreResetLabel                 = "Reset to defaults"
	GitIgnoreApplyLabel                 = "Review & write"
	GitIgnoreDoneEditingLabel           = "Done editing"
	GitIgnoreDiscardedEditsStatus       = "Leaving this screen discards unsaved edits."
)

// GitIgnoreWiringPointsElsewhere formats the "points at a different file"
// wiring sentence. otherPath is the display path gitid actually read.
func GitIgnoreWiringPointsElsewhere(otherPath string) string {
	return "! core.excludesfile points at " + otherPath + " instead of this file — that choice is left alone; writing here only affects the file below."
}

// Global Git Ignore malformed-file reason phrases (09.2-UI-SPEC.md's
// Malformed file refusal row). A backend translates a structured
// gitconfig.ManagedBlockError into one of these four phrases — the raw
// internal diagnostic text must never reach the screen (09.2-UI-REVIEW.md
// finding 2).
//
// GitIgnoreMalformedReasonUnspecified is a defensive fallback only — a
// genuine gitconfig.ManagedBlockError always sets one of the three named
// reasons above it; this exists so an unrecognized/zero reason value reports
// as "unspecified" instead of silently mislabeling itself as one of the
// three known categories (09.2-REVIEW.md IN-02).
//
// GitIgnoreMalformedReasonMismatchedMarker covers TWO distinct shapes
// classified under the one ManagedBlockReasonMismatchedMarker reason: a
// standalone END with no opening marker AT ALL, and an END whose name
// disagrees with the BEGIN it closes (baseline.go's InspectManagedBlockFile
// doc comment). The phrase is worded to be accurate for both — "a closing
// marker that doesn't match its opening marker" would misdescribe the
// standalone-END shape as having SOME opening marker that merely disagrees,
// when in fact there is none (09.2-REVIEW.md IN-03).
const (
	GitIgnoreMalformedReasonUnclosedMarker   = "an opening marker with no matching closing marker"
	GitIgnoreMalformedReasonMismatchedMarker = "a closing marker with no valid matching opening marker"
	GitIgnoreMalformedReasonDuplicateBlock   = "two complete gitid blocks in one file"
	GitIgnoreMalformedReasonUnspecified      = "a malformed gitid marker"
)

// GitIgnoreMalformedFileMessage formats the frozen malformed-file refusal
// sentence — no leading glyph; the screen's rendering path is responsible
// for prefixing a glyph consistently regardless of which field (stateErr or
// applyErr) carries this text (09.2-REVIEW.md WR-05). displayPath is the
// HOME-relative path of the file that actually failed to parse — the
// gitignore file OR the baseline fragment, since InspectManagedBlockFile is
// shared between them (09.2-REVIEW.md CR-01: a malformed baseline fragment
// must not be reported under the gitignore file's path). line is the
// offending 1-based line number; reason is one of the
// GitIgnoreMalformedReason* constants above.
func GitIgnoreMalformedFileMessage(displayPath string, line int, reason string) string {
	return displayPath + " has a broken gitid marker at line " + strconv.Itoa(line) + " (" + reason + ") — repair the file by hand before this screen can read or write it."
}

// GitIgnoreSentinelRejectedMessage formats the frozen sentinel-injection
// rejection sentence — this screen's ONE styleError (Error-red) state. It is
// prefixed with a glyph so meaning survives without color, matching every
// other advisory state on this screen (09.2-UI-REVIEW.md finding 3).
func GitIgnoreSentinelRejectedMessage(line int) string {
	return "✗ Line " + strconv.Itoa(line) + " looks like a gitid managed-block marker and can't be part of your content — edit or remove that line before applying."
}

// GitIgnoreReceiptWrongTarget formats the wrong-target success receipt.
// otherPath is the display path gitid actually read.
func GitIgnoreReceiptWrongTarget(otherPath string) string {
	return "Global gitignore written to ~/.gitignore_global. core.excludesfile still points at " + otherPath + ", so Git is not reading this file — that setting was left as you configured it."
}

// GlobalGitBaselineStripText is the read-only inherited global-baseline
// strip rendered on per-identity Git surfaces (GITUI-01 kept intact) —
// values interpolated from recipeFixtures.ts's globalGitDefaults.
const GlobalGitBaselineStripText = "init.defaultBranch=main · core.ignorecase=false · autocrlf=input/lf · push.autoSetupRemote=true · pull.rebase=true · merge=zdiff3"

// GlobalGitFullManagedBlockText is the exact managed-block text gitid
// writes to ~/.gitconfig — the Go mirror of recipeFixtures.ts's
// globalGitFullManagedBlockText. Global user.email is intentionally ABSENT:
// gitid never writes a [user] section here (each identity's commits come
// from its own includeIf fragment).
const GlobalGitFullManagedBlockText = GlobalGitSentinelBegin + `
[init]
    defaultBranch = main

[core]
    ignorecase = false
    autocrlf = input
    eol = lf

[user]
    useConfigOnly = true

[push]
    autoSetupRemote = true

[pull]
    rebase = true

[fetch]
    prune = true

[color]
    ui = auto
    branch = auto
    diff = auto
    status = auto

[merge]
    conflictstyle = zdiff3

[diff]
    colorMoved = zebra

[alias]
    st = status
    co = checkout
    br = branch
    ci = commit
    df = diff
    lg = log --graph --pretty=format:'%Cred%h%Creset -%C(yellow)%d%Creset %s %Cgreen(%cr) %C(bold blue)<%an>%Creset' --abbrev-commit
    unstage = reset HEAD --
    last = log -1 HEAD
` + GlobalGitSentinelEnd

// ---------------------------------------------------------------------------
// Health severities (HLTH-*). These labels classify doctor findings; they
// do not describe a separate read-only tab.
// ---------------------------------------------------------------------------

// HealthSeverity mirrors recipeFixtures.ts's HealthSeverity — the four
// severity levels, byte-identical lowercase labels.
type HealthSeverity string

// The four health severity levels.
const (
	// SeverityInfo is the informational level (cyan ~).
	SeverityInfo HealthSeverity = "info"
	// SeverityWarning is the advisory level (yellow !).
	SeverityWarning HealthSeverity = "warning"
	// SeverityError is the error level (red ✗).
	SeverityError HealthSeverity = "error"
	// SeverityCritical is the critical level (red ✗, distinguished from
	// error by the WORD, never the glyph/color alone).
	SeverityCritical HealthSeverity = "critical"
)

// HealthSeverityGlyph pairs each severity with its LOCKED glyph — the Go
// mirror of recipeFixtures.ts's healthSeverityGlyph. warning is ALWAYS `!`
// (yellow), error AND critical both use `✗` (red) — distinguished by the
// WORD, never by a different glyph — info is `~` (cyan). Never reuse `✗`
// for warning.
var HealthSeverityGlyph = map[HealthSeverity]string{
	SeverityInfo:     "~",
	SeverityWarning:  "!",
	SeverityError:    "✗",
	SeverityCritical: "✗",
}

// HealthFinding mirrors recipeFixtures.ts's HealthFinding shape — one
// concrete health finding, scoped to either the SSH or Git section.
type HealthFinding struct {
	ID           string
	Section      string
	Family       string
	Title        string
	Explanation  string
	SuggestedFix string
	Severity     HealthSeverity
	// Fixable is the authoritative "can the Fixer actually apply this"
	// signal (set from doctor.Finding.Fix != nil at conversion time,
	// cmd/gitid/wiring.go's runDoctorAndConvert). SuggestedFix is NOT this
	// signal: many report-only checks set non-empty SuggestedFix text
	// (advisory "do this by hand" prose) while leaving Fix nil -- a
	// pre-Fixable bug let the Fixer offer a full preview/confirm/backup
	// ceremony for these, ending in a fabricated backup path and a fake
	// "applied" receipt while persistFixFinding silently no-oped (08-08
	// code review CR-01).
	Fixable bool
}

// ---------------------------------------------------------------------------
// Fixer copy the Doctor screen and the fix plans render (FIX-01/02).
// ---------------------------------------------------------------------------

// FixerTargetHost is the Host block the flagship §4.7 fix rewrites.
const FixerTargetHost = "clientb.github.com"

// FixerFixPreviewLines mirrors recipeFixtures.ts's fixerFixPreviewLines — a
// true `-`/`+` rewrite diff (not additions-only), because this fix REWRITES
// an existing directive's value rather than adding a new one (T-02-FIX).
// Two-space context lines show the rest of the existing Host block is
// untouched.
var FixerFixPreviewLines = []string{
	"  Host " + FixerTargetHost,
	"      Hostname ssh.github.com",
	"      Port 443",
	"      User git",
	"      IdentityFile ~/.ssh/id_ed25519_clientB",
	"-     IdentitiesOnly no",
	"+     IdentitiesOnly yes",
}

// FixerNothingToFix* mirror recipeFixtures.ts's fixerNothingToFixSummary —
// the zero-findings summary for both sections (§4.7's healthy empty state).
const (
	// FixerNothingToFixSSH is the healthy-empty SSH section summary.
	FixerNothingToFixSSH = "SSH -- 0 fixable problems. Every Host block is coherent, every key is 0600."
	// FixerNothingToFixGit is the healthy-empty Git section summary.
	FixerNothingToFixGit = "Git -- 0 fixable problems. Every includeIf target exists, every allowed_signers email matches."
)

// ---------------------------------------------------------------------------
// Upload / Credentials Assist (Phase 9, D-08) frozen copy.
//
// Every constant below is drafted in 09-UI-SPEC.md's Copywriting Contract
// (checker-approved 2026-08-28) and is byte-exact against that table. Two
// scoped deviations from the DRAFT strings are recorded there and apply
// here: (1) the `☐`/`☑` glyph is NOT baked into the checkbox label
// constants — the render composes the existing glyphCheckOff/glyphCheckOn
// constants (theme.go) with the label, mirroring the existing demo-failure
// toggle at identities.go:4211-4215, so there is never a second,
// independent copy of the glyph pair; (2) `<Provider>`/`<tool>`/`<host>`
// placeholders become `%s` format verbs, matching every other `*Fmt`
// constant in this file.
//
// Authority split (R21, 09-01-PLAN.md cross-AI review): TestFrozenUploadCopy
// (upload_copy_test.go) is the AUTHORITATIVE, byte-exact contract for these
// values. `make gate-copy-freeze` is a SECONDARY source-presence guard —
// it only proves a string appears SOMEWHERE under the scanned roots (a
// comment or a dead declaration would satisfy it too), so a green gate
// alone must never be read as proof of the value.
//
// Amendment rule for operational copy (R22): if a later wave (3 or 8)
// discovers that real gh/glab behavior makes one of the operational
// remediation sentences below inaccurate, the correct response is a
// REVIEWED amendment — edit the constant, update TestFrozenUploadCopy, and
// update the gate-copy-freeze entry, recording the amendment plus the
// observed provider behavior in that plan's SUMMARY — never rendering
// different text at a call site, and never leaving inaccurate guidance in
// place because the string is "frozen."
const (
	// UploadCheckboxLabelReadyFmt is the D-01 scenario-1 checkbox label
	// (tool present, authenticated — pre-checked). 09-UI-SPEC.md
	// Copywriting Contract row `UploadCheckboxLabelReady`.
	UploadCheckboxLabelReadyFmt = "Register with %s automatically (auth + signing)"
	// UploadCheckboxLabelUnauthFmt is the D-01 scenario-2 checkbox label
	// (tool present, not authenticated — unchecked but toggleable).
	// 09-UI-SPEC.md Copywriting Contract row `UploadCheckboxLabelUnauth`.
	UploadCheckboxLabelUnauthFmt = "Register with %s automatically — not logged in to %s; run \"%s auth login\" first, or check anyway"
	// UploadCheckboxLabelDisabledFmt is the D-01 scenario-3 checkbox label
	// (no matching CLI — disabled). 09-UI-SPEC.md Copywriting Contract row
	// `UploadCheckboxLabelDisabled`.
	UploadCheckboxLabelDisabledFmt = "Auto-registration unavailable — %s has no gh/glab match here. Manual steps are shown after create."
	// UploadRunningLineFmt is the D-02 announce-and-do line, one per
	// attempted registration. The %s argument MUST be the literal string
	// uploader.CommandPreview/buildArgs produced — never a hand-typed
	// approximation (UP-02/UP-03 shown==run, structural).
	// 09-UI-SPEC.md Copywriting Contract row `UploadRunningLineFmt`.
	UploadRunningLineFmt = "Running: %s"
	// UploadResultOKFmt is the D-16 per-key success result row. The %s
	// argument is one of UploadRegistrationLabelAuth/Signing/Combined.
	// 09-UI-SPEC.md Copywriting Contract row `UploadResultOK`.
	UploadResultOKFmt = "✓ %s key registered"
	// UploadResultSkippedFmt is the D-15 idempotent-dedupe result row.
	// 09-UI-SPEC.md Copywriting Contract row `UploadResultSkipped`.
	UploadResultSkippedFmt = "✓ %s key already registered (skipped)"
	// UploadResultFailedFmt is the D-16 per-type failure result row. The
	// second %s is the classified reason (scope error, cross-account
	// conflict, or the raw trimmed CLI output as last resort).
	// 09-UI-SPEC.md Copywriting Contract row `UploadResultFailed`.
	UploadResultFailedFmt = "✗ %s key registration failed: %s"
	// UploadUnconfirmedReasonFmt is D-17's post-upload confirmation note
	// (CR-01, review iteration 5): marks a row whose registration the
	// provider ACCEPTED (Uploaded/AlreadyPresent) but whose post-upload
	// confirmation read still could not see after the one bounded retry.
	// This is NOT a failure — turning it into one would misreport a
	// successful upload as rejected — so it renders as an extra faint
	// continuation row under the row's own UploadResultOKFmt/
	// UploadResultSkippedFmt line, never in place of it. Originally a
	// cmd/gitid-local constant that neither renderer ever read; moved here
	// and wired into both renderUploadSection (TUI) and printUploadOutcome
	// (CLI) as part of the CR-01 fix so the "shown == run" contract covers
	// it (upload_copy_test.go). Not in 09-UI-SPEC.md's original Copywriting
	// Contract table — added by this fix, same precedent as
	// UploadNotAuthenticatedFmt above.
	UploadUnconfirmedReasonFmt = "accepted but not yet visible in %s's inventory — this can lag briefly after upload; re-run gitid's test to confirm"
	// UploadScopeRemediationAuthFmt is the D-14 scope-error remediation for
	// the authentication-key registration. 09-UI-SPEC.md Copywriting
	// Contract row `UploadScopeRemediationAuth`.
	UploadScopeRemediationAuthFmt = "insufficient scope — run \"gh auth refresh -h %s -s admin:public_key\", then retry from the Identity Manager"
	// UploadScopeRemediationSigningFmt is the D-14 scope-error remediation
	// for the signing-key registration. 09-UI-SPEC.md Copywriting Contract
	// row `UploadScopeRemediationSigning`.
	UploadScopeRemediationSigningFmt = "insufficient scope — run \"gh auth refresh -h %s -s admin:ssh_signing_key\", then retry from the Identity Manager"
	// UploadNotAuthenticatedFmt is the WR-03 remediation for a registration
	// attempt that failed because the provider CLI session was not
	// authenticated (uploader.FailureNotAuthenticated) — the D-01 scenario-2
	// "check anyway" path the UI invites the user into. Not originally in
	// 09-UI-SPEC.md's Copywriting Contract table (FailureNotAuthenticated
	// was classified but never rendered, falling through to a raw,
	// truncated CLI line instead); follows the same "<problem> — run
	// <command>, then retry from the Identity Manager" shape as the two
	// scope-remediation siblings above. First %s is the CLI tool name
	// ("gh"/"glab"), second is the provider hostname.
	UploadNotAuthenticatedFmt = "not authenticated — run \"%s auth login -h %s\", then retry from the Identity Manager"
	// UploadCrossAccountConflict is the D-15 glab-only cross-account
	// conflict finding — NEVER silently classified as success (GitLab
	// fingerprints are globally unique). 09-UI-SPEC.md Copywriting Contract
	// row `UploadCrossAccountConflict`.
	UploadCrossAccountConflict = "GitLab rejected this key — it is already registered to a DIFFERENT account. If that's expected, remove it there first; otherwise check \"glab auth status\"."
	// UploadInventoryDegradedFmt is the D-15 inventory-read-failure notice
	// — degrades to plain upload, NEVER gates (D-11). 09-UI-SPEC.md
	// Copywriting Contract row `UploadInventoryDegraded`.
	UploadInventoryDegradedFmt = "Could not check %s for existing keys — uploading anyway; duplicates are handled safely."
	// UploadDryRunNote is the D-06 --dry-run trailing note — the SAME
	// announcing-shape lines render, this note replaces per-key-results.
	// 09-UI-SPEC.md Copywriting Contract row `UploadDryRunNote`.
	UploadDryRunNote = "--dry-run: the command(s) above were shown, not run."
	// UploadManualHeading precedes the existing, byte-identical
	// internal/upload.Instructions(provider) block. 09-UI-SPEC.md
	// Copywriting Contract row `UploadManualHeading`.
	UploadManualHeading = "Auto-registration wasn't available. Register it yourself:"
	// UploadKeyTitleFmt is the D-07 machine-scoped key title
	// ("gitid: <name> @ <hostname>"). Deliberately EXCLUDED from
	// gate-copy-freeze's frozen-string list (Makefile) — its rendered
	// output varies per machine, matching the four existing dynamic-text
	// exclusion precedents. 09-UI-SPEC.md Copywriting Contract row
	// `UploadKeyTitleFmt`.
	UploadKeyTitleFmt = "gitid: %s @ %s"
	// UploadSkippedByFlagNote is the D-06 --no-upload opt-out note,
	// immediately followed by the manual-fallback block. 09-UI-SPEC.md
	// Copywriting Contract row "--no-upload opt-out note".
	UploadSkippedByFlagNote = "Auto-upload skipped (--no-upload)."
	// UploadAlreadyCompleteFmt closes 09-UI-SPEC.md's "zero-one-many" UI
	// Consideration row, left to planner discretion: when the D-15
	// inventory reports every registration already present (both GitHub
	// types, or GitLab's single combined registration), the section
	// renders this ONE collapsed line instead of repeating
	// UploadResultSkippedFmt once per type.
	UploadAlreadyCompleteFmt = "✓ Already registered with %s — nothing to do."
	// UploadRegistrationLabelAuth is one of the three %s values
	// UploadResultOKFmt/UploadResultSkippedFmt/UploadResultFailedFmt
	// interpolate — gh's authentication-key registration.
	UploadRegistrationLabelAuth = "Authentication"
	// UploadRegistrationLabelSigning is gh's signing-key registration
	// label — the sibling of UploadRegistrationLabelAuth.
	UploadRegistrationLabelSigning = "Signing"
	// UploadRegistrationLabelCombined is glab's single
	// auth_and_signing registration label (D-12).
	UploadRegistrationLabelCombined = "Key"

	// RotateDeleteOfferHeadingFmt is the D-04 interactive old-key delete
	// offer's heading, replacing keyCeremonyGraceHintFmt as the sole
	// message on the path where the offer applies. 09-UI-SPEC.md
	// Copywriting Contract row `RotateDeleteOfferHeading`.
	RotateDeleteOfferHeadingFmt = "Remove the old key from %s?"
	// RotateDeleteOfferBodyFmt names the old key (D-07 machine-scoped
	// title) and states it remains valid until removed. 09-UI-SPEC.md
	// Copywriting Contract row `RotateDeleteOfferBody`.
	RotateDeleteOfferBodyFmt = "The old key (\"gitid: %s @ %s\") still authenticates there until you remove it. Delete it now?"
	// RotateDeleteOfferChoiceDeleteFmt is the offer's destructive choice —
	// never default-focused (D-04's non-destructive-by-default posture).
	// 09-UI-SPEC.md Copywriting Contract row `RotateDeleteOfferChoiceDelete`.
	RotateDeleteOfferChoiceDeleteFmt = "[ Delete old key from %s ]"
	// RotateDeleteOfferChoiceLeave is the offer's default-focused,
	// non-destructive choice (D-04). 09-UI-SPEC.md Copywriting Contract row
	// `RotateDeleteOfferChoiceLeave`.
	RotateDeleteOfferChoiceLeave = "[ Leave it — I'll remove it myself ]"
	// RotateDeleteOfferResultRemovedFmt is the D-04 offer's accepted-path
	// result line. 09-UI-SPEC.md Copywriting Contract row
	// `RotateDeleteOfferResult` (removed half).
	RotateDeleteOfferResultRemovedFmt = "✓ Old key removed from %s."
	// RotateDeleteOfferResultLeftFmt is the D-04 offer's declined-path
	// result line, naming the manual delete command/URL. 09-UI-SPEC.md
	// Copywriting Contract row `RotateDeleteOfferResult` (left-in-place
	// half).
	RotateDeleteOfferResultLeftFmt = "Left in place — remove it yourself: %s"

	// IdentityManagerActionRegisterKey is the action-menu's fifth row
	// (D-08/D-09 Phase 9 amendment, identity-manager/FIELDS.md
	// action_register_key) — joins IdentityManagerAction{ViewDetail,
	// Clone,NewKey,Delete} above.
	//
	// D2 amendment (260831-3a9, deliberate 09-UI-SPEC.md Copywriting
	// Contract divergence — the project's D-09 precedent for a scoped
	// frozen-copy amendment): reworded from the shorter pre-D2 label so the
	// immediate `gh`/`glab` provider mutation this row triggers is legible
	// BEFORE it is pressed (D-08's no-confirm contract, "opening the modal
	// IS the opt-in", is unchanged — this only names what the opt-in does).
	IdentityManagerActionRegisterKey = "Register key with provider now (u)"
	// RegisterKeyModalHeadingFmt is the D-08 "copy modal" heading — the
	// manual re-trigger surface for a key-unused/key-used-ssh-only
	// identity. 09-UI-SPEC.md Copywriting Contract row "Identity Manager
	// copy-modal heading".
	RegisterKeyModalHeadingFmt = "Register %s's key with %s"
	// RegisterKeyStatusRan is the D2 (260831-3a9) status-line variant for
	// every register-key pane state where a registration actually ran or
	// is running: registerKeyPending, or a completed run
	// (uploadRunHasContent). Replaces the single self-contradicting status
	// line ("without writing anything ... registration runs on open") that
	// used to render in every state regardless of whether a registration
	// had actually run.
	RegisterKeyStatusRan = "Esc closes — registration ran when this opened; no local files were changed."
	// RegisterKeyStatusNothingRan is the D2 (260831-3a9) status-line variant
	// for every register-key pane state where nothing has run: the
	// not-yet-loaded probe, a probe error, and the pre-run manual-fallback
	// state. Never paired with RegisterKeyStatusRan's "registration ran"
	// claim in the same render.
	RegisterKeyStatusNothingRan = "Esc closes — nothing was registered."
)
