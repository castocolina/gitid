package tuikit

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
	{Key: "AddKeysToAgent", Current: "yes", Risk: "Low", Recommended: "yes", NeedsAction: false, OneLiner: "Already set — keys stay available in the agent for the session (recipes/ssh-config.recipe Host * block)."},
	{Key: "UseKeychain", Current: "yes (macOS only)", Risk: "Low", Recommended: "yes", NeedsAction: false, OneLiner: "Already set — stores the key passphrase in the macOS Keychain (guarded by IgnoreUnknown on Linux)."},
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

// ---------------------------------------------------------------------------
// Global Git options (GGIT-01's baseline/recipe-default catalog).
// ---------------------------------------------------------------------------

// D9 (checkpoint-2 contract) frozen copy for the promoted, editable
// global-fallback user.email row — byte-exact; shared by globalgit.go's
// detail render, apply checkbox, and the dedicated apply ceremony. This is
// a DOCUMENTED, CONSCIOUS divergence from recipes/ (which leave user.email
// unset by default) — recorded in FIELDS.md + 02-STYLE-SPEC.md (Task 3).
const (
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
	{Key: "push.autoSetupRemote", Current: "not set (git default: false)", Recommended: "true", NeedsAction: true, OneLiner: "Lets `git push` on a new branch set its upstream automatically, instead of requiring --set-upstream every time."},
	{Key: "pull.rebase", Current: "not set (git default: false -- merge)", Recommended: "true", NeedsAction: true, OneLiner: "Replays local commits on top of the fetched branch instead of creating a merge commit on every pull."},
	{Key: "fetch.prune", Current: "not set (git default: false)", Recommended: "true", NeedsAction: true, OneLiner: "Removes local references to remote branches that were deleted upstream, every fetch."},
	{Key: "alias (8 shortcuts)", Current: "not set", Recommended: "st, co, br, ci, df, lg, unstage, last", NeedsAction: true, OneLiner: "Short, common-workflow aliases (status, checkout, branch, commit, diff, a graph log, unstage, last commit)."},
	{Key: "color (ui/branch/diff/status)", Current: "not set (ui defaults to auto in modern git; the rest vary)", Recommended: "auto for all four", NeedsAction: true, OneLiner: "Colorizes status, branch, diff, and general UI output consistently, even where a specific subcommand's own default might differ."},
	{Key: "merge.conflictstyle", Current: "not set (git default: merge)", Recommended: "diff3", NeedsAction: true, OneLiner: "Shows the common ancestor alongside both sides of a conflict, making it easier to tell what each side actually changed."},
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

	// GlobalGitResultMessage is the success message after the baseline
	// apply — global user.email is always left alone.
	GlobalGitResultMessage = "10 of 10 baseline options applied to ~/.gitconfig. Global user.email was left alone, as always -- each identity's commits use their own includeIf fragment."
)

// GlobalGitBaselineStripText is the read-only inherited global-baseline
// strip rendered on per-identity Git surfaces (GITUI-01 kept intact) —
// values interpolated from recipeFixtures.ts's globalGitDefaults.
const GlobalGitBaselineStripText = "init.defaultBranch=main · core.ignorecase=false · autocrlf=input/lf · push.autoSetupRemote=true · pull.rebase=true · merge=diff3"

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
    conflictstyle = diff3

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
// Health severities (HLTH-*). Health is READ-ONLY: it diagnoses, it never
// mutates.
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
