package dummytui

import "strconv"

// This file is the Go mirror of
// .planning/design/mockup-src/src/data/recipeFixtures.ts — the single typed
// fixture source every design surface renders from (real values only, no
// placeholder option lists per 02-UX-DIRECTION.md §0 Risk 3). Every string
// is derived directly from recipes/ssh-config.recipe and
// recipes/gitconfig.recipe (the North Star; see recipes/README.md) —
// structure and field values are recipe-faithful, but the key ALGORITHM is
// ed25519, not the gists' RSA, per the recipes' own "structure, not key
// type" caveat. The identity alias used throughout is "personal"
// (personal.github.com), matching the recipes' own worked example.
//
// The values below were extracted verbatim from the removed static-surface
// files (surface_*.go) so the upcoming live Go TUI demo can seed from the
// SAME canonical data the interactive web demo renders. Rendering concerns
// (styles, screen registries, signatures tied to the removed capture
// manifests) were intentionally NOT carried over.

// ---------------------------------------------------------------------------
// Shell: header global-context chip (02-UX-DIRECTION.md §2 region 1)
// ---------------------------------------------------------------------------

// ShellHeaderIdentityCount is the header chip's identity count — the number
// of IdentityManagerRows entries, mirroring the mockup Header's
// headerContext for the home screen.
const ShellHeaderIdentityCount = 8

// ShellHeaderHealthGlyph is the header chip's global-health glyph ("needs
// action" tone), always paired with ShellHeaderHealthWord — never color or
// glyph alone.
const ShellHeaderHealthGlyph = "!"

// ShellHeaderHealthWord is the WORD paired with ShellHeaderHealthGlyph
// (NO_COLOR legibility: the word carries the meaning).
const ShellHeaderHealthWord = "needs action"

// ---------------------------------------------------------------------------
// Create flow: SSH identity creation (recipes/ssh-config.recipe;
// recipeFixtures.ts sshIdentityAlias* / algorithmCatalog / test* exports)
// ---------------------------------------------------------------------------

// Recipe-accurate literal copy for the create flow. Identity alias
// "personal" / host "personal.github.com" matches the recipe's own worked
// example and recipeFixtures.ts.
const (
	// CreateFlowAliasPrefix is the identity alias prefix ("personal").
	CreateFlowAliasPrefix = "personal"
	// CreateFlowSSHHost is the alias Host ssh connects to.
	CreateFlowSSHHost = "personal.github.com"
	// CreateFlowRealHostname is the real hostname behind the alias
	// (port-443 alt-SSH endpoint).
	CreateFlowRealHostname = "ssh.github.com"
	// CreateFlowPort is the alt-SSH port (443) the recipe pins.
	CreateFlowPort = "443"
	// CreateFlowIdentityFile is the per-identity ed25519 key path.
	CreateFlowIdentityFile = "~/.ssh/id_ed25519_personal"
	// CreateFlowBlankPrefixHost is the WYSIWYG Host when the alias prefix
	// is left blank (provider host verbatim, no invented suffix).
	CreateFlowBlankPrefixHost = "github.com"

	// CreateFlowSSHHostBlock is the exact Host block gitid writes for the
	// alias — a literal, so the recipe-critical values (Port 443,
	// IdentitiesOnly yes) stay byte-visible in source.
	CreateFlowSSHHostBlock = `Host personal.github.com
    Hostname ssh.github.com
    Port 443
    User git
    IdentityFile ~/.ssh/id_ed25519_personal
    IdentitiesOnly yes`

	// CreateFlowMacGlobalsBlock is the one-time macOS Keychain globals
	// block (guarded by IgnoreUnknown so it is a documented no-op on Linux).
	CreateFlowMacGlobalsBlock = `IgnoreUnknown UseKeychain

Host *
    UseKeychain yes
    AddKeysToAgent yes`

	// CreateFlowTestTmpConfig is the throwaway temp config connectivity
	// tests run against — the live ~/.ssh/config is untouched.
	CreateFlowTestTmpConfig = "/tmp/gitid-test-a1b2c3.config"
	// CreateFlowTestStage1Command is the stage-1 direct connectivity test
	// (provider URL, no alias).
	CreateFlowTestStage1Command = "ssh -T -F " + CreateFlowTestTmpConfig + " -p " + CreateFlowPort + " -i " + CreateFlowIdentityFile + " git@" + CreateFlowRealHostname
	// CreateFlowTestStage1Output is stage 1's success output.
	CreateFlowTestStage1Output = "Hi personal! You've successfully authenticated, but GitHub does not provide shell access."
	// CreateFlowTestStage2Command is the stage-2 by-alias test (ssh -G
	// proof that IdentitiesOnly + this key are what will actually be used).
	CreateFlowTestStage2Command = "ssh -G " + CreateFlowSSHHost + " -F " + CreateFlowTestTmpConfig + " | grep identityfile"
	// CreateFlowTestStage2Output is stage 2's success output.
	CreateFlowTestStage2Output = "identityfile " + CreateFlowIdentityFile
	// CreateFlowTestFailCommand is the command shown on the failed-test
	// screen (same as stage 1).
	CreateFlowTestFailCommand = CreateFlowTestStage1Command
	// CreateFlowTestFailOutput is the failed-test output.
	CreateFlowTestFailOutput = "git@" + CreateFlowRealHostname + ": Permission denied (publickey)."

	// CreateFlowTargetFile is the file the create flow writes to.
	CreateFlowTargetFile = "~/.ssh/config"
	// CreateFlowBackupPath is the timestamped backup taken before writing.
	CreateFlowBackupPath = "~/.ssh/config.backup.2026-07-03T03-59-12Z"

	// CreateFlowSentinelBegin opens the identity's managed block.
	CreateFlowSentinelBegin = "# BEGIN gitid managed: personal"
	// CreateFlowSentinelEnd closes the identity's managed block.
	CreateFlowSentinelEnd = "# END gitid managed: personal"
)

// CreateFlowManagedBlockText is the full sentinel-delimited managed block
// the create flow appends to ~/.ssh/config.
var CreateFlowManagedBlockText = CreateFlowSentinelBegin + "\n" + CreateFlowSSHHostBlock + "\n" + CreateFlowSentinelEnd

// AlgorithmCatalogEntry mirrors recipeFixtures.ts's AlgorithmCatalogEntry
// shape (KEY-01's top-5 catalog; ed25519 is best/default, KEY-03's
// macOS/Linux local-availability notes for the other four).
type AlgorithmCatalogEntry struct {
	ID          string
	Security    string
	MacOS       string
	Linux       string
	Recommended bool
}

// AlgorithmCatalog is the Go mirror of recipeFixtures.ts's
// algorithmCatalog — the KEY-01 top-5 key-algorithm catalog.
var AlgorithmCatalog = []AlgorithmCatalogEntry{
	{
		ID:          "ed25519",
		Recommended: true,
		Security:    "Modern EdDSA curve — small keys, fast, constant-time (timing-attack resistant). The recommended default.",
		MacOS:       "Native (LibreSSL) — always available",
		Linux:       "Native (OpenSSL) — always available",
	},
	{
		ID:       "ed25519-sk",
		Security: "Hardware-backed: private key material never leaves the security key; requires a physical touch to sign.",
		MacOS:    "Needs libfido2 + a FIDO2 security key",
		Linux:    "Needs libfido2 + a FIDO2 security key",
	},
	{
		ID:       "rsa-4096",
		Security: "Strong at 4096 bits; widely compatible, larger keys and slower signing than ed25519.",
		MacOS:    "Native — always available",
		Linux:    "Native — always available",
	},
	{
		ID:       "ecdsa-p256",
		Security: "Compact NIST P-256 curve; smaller than RSA, though some users distrust NIST curve provenance versus ed25519.",
		MacOS:    "Native — always available",
		Linux:    "Native — always available",
	},
	{
		ID:       "ecdsa-sk",
		Security: "Hardware-backed ECDSA variant of ed25519-sk; physical security-key touch required.",
		MacOS:    "Needs libfido2 + a FIDO2 security key",
		Linux:    "Needs libfido2 + a FIDO2 security key",
	},
}

// ---------------------------------------------------------------------------
// Git screen: per-identity Git configuration (recipes/gitconfig.recipe;
// recipeFixtures.ts git* exports). Fragment path uses the GITUI-02
// convention (~/.gitconfig.d/<identity>).
// ---------------------------------------------------------------------------

// Recipe-accurate literal copy for the per-identity Git screen.
const (
	// GitScreenUserName is the identity's user.name.
	GitScreenUserName = "Personal Identity"
	// GitScreenUserEmail is the identity's user.email — must byte-match
	// the allowed_signers entry (GITUI-04).
	GitScreenUserEmail = "you@personal.example"
	// GitScreenGpgFormat is gpg.format (fixed to ssh).
	GitScreenGpgFormat = "ssh"
	// GitScreenSigningKey is user.signingkey — a PATH to the public key,
	// never the key material itself.
	GitScreenSigningKey = "~/.ssh/id_ed25519_personal.pub"
	// GitScreenCommitGpgSign is commit.gpgsign.
	GitScreenCommitGpgSign = "true"

	// GitScreenFragmentFile is the per-identity fragment path (GITUI-02).
	GitScreenFragmentFile = "~/.gitconfig.d/personal"
	// GitScreenGitconfigFile is the global gitconfig the includeIf block
	// is appended to.
	GitScreenGitconfigFile = "~/.gitconfig"
	// GitScreenAllowedSignersFile is the allowed_signers file the signing
	// entry is appended to.
	GitScreenAllowedSignersFile = "~/.ssh/allowed_signers"

	// GitScreenFragmentText is the full per-identity fragment content.
	GitScreenFragmentText = `[user]
    name = ` + GitScreenUserName + `
    email = ` + GitScreenUserEmail + `
    signingkey = ` + GitScreenSigningKey + `

[gpg]
    format = ` + GitScreenGpgFormat + `

[commit]
    gpgsign = ` + GitScreenCommitGpgSign

	// GitScreenIncludeIfGitdirLine is the gitdir-match includeIf block
	// (the default match strategy).
	GitScreenIncludeIfGitdirLine = `[includeIf "gitdir:~/personal/"]
    path = ` + GitScreenFragmentFile

	// GitScreenIncludeIfHasconfigLine is the hasconfig-match alternative.
	GitScreenIncludeIfHasconfigLine = `[includeIf "hasconfig:remote.*.url:git@personal.github.com:*/**"]
    path = ` + GitScreenFragmentFile

	// GitScreenMatchStrategyDefault is the default includeIf match
	// strategy ("gitdir"; "hasconfig" and "both" are the alternatives).
	GitScreenMatchStrategyDefault = "gitdir"

	// GitScreenAllowedSignersKeyMaterial is the fixture public-key
	// material (NOT a real key).
	GitScreenAllowedSignersKeyMaterial = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIDesignMockupFixtureKeyNotReal0"
	// GitScreenAllowedSignersLine is the exact allowed_signers line
	// written (email must byte-match GitScreenUserEmail).
	GitScreenAllowedSignersLine = GitScreenUserEmail + " " + GitScreenAllowedSignersKeyMaterial

	// GitScreenSentinelBegin opens the identity's managed gitconfig block.
	GitScreenSentinelBegin = "# BEGIN gitid managed: personal"
	// GitScreenSentinelEnd closes the identity's managed gitconfig block.
	GitScreenSentinelEnd = "# END gitid managed: personal"

	// GitScreenGitconfigBackupPath is ~/.gitconfig's timestamped backup.
	GitScreenGitconfigBackupPath = "~/.gitconfig.backup.2026-07-03T03-59-12Z"
	// GitScreenAllowedSignersBackupPath is allowed_signers' timestamped
	// backup.
	GitScreenAllowedSignersBackupPath = "~/.ssh/allowed_signers.backup.2026-07-03T03-59-12Z"

	// GitScreenResultMessage is the success message after the write.
	GitScreenResultMessage = `Git identity "personal" configured — ` + GitScreenFragmentFile + ` now applies via the ` + GitScreenMatchStrategyDefault + ` match strategy.`
)

// GitScreenGitconfigIncludeBlockText is the sentinel-delimited includeIf
// block appended to ~/.gitconfig.
var GitScreenGitconfigIncludeBlockText = GitScreenSentinelBegin + "\n" + GitScreenIncludeIfGitdirLine + "\n" + GitScreenSentinelEnd

// GitScreenIncludeIfBothLines is the "both" match strategy's includeIf
// preview (two blocks = OR semantics) — the Go mirror of recipeFixtures.ts's
// gitScreenIncludeIfBothLines.
var GitScreenIncludeIfBothLines = GitScreenIncludeIfGitdirLine + "\n\n" + GitScreenIncludeIfHasconfigLine

// GitScreenMatchStrategyPreview keys the live includeIf preview by match
// strategy — the Go mirror of recipeFixtures.ts's
// gitScreenMatchStrategyPreview ("gitdir" is the default, GITUI-03).
var GitScreenMatchStrategyPreview = map[string]string{
	"gitdir":    GitScreenIncludeIfGitdirLine,
	"hasconfig": GitScreenIncludeIfHasconfigLine,
	"both":      GitScreenIncludeIfBothLines,
}

// ManagedBlockSentinels returns the BEGIN/END sentinel pair delimiting the
// managed block gitid owns for identityName — the Go mirror of
// recipeFixtures.ts's managedBlockSentinels (CLAUDE.md "Engineering":
// idempotent managed blocks, never a blind append).
func ManagedBlockSentinels(identityName string) (begin, end string) {
	return "# BEGIN gitid managed: " + identityName, "# END gitid managed: " + identityName
}

// ---------------------------------------------------------------------------
// Identity manager (recipeFixtures.ts identityManager* exports)
// ---------------------------------------------------------------------------

// IdentityManagerRow mirrors recipeFixtures.ts's IdentityManagerRow shape —
// one fixture identity per MGR-02 8-label state, so a populated list
// demonstrates every label at once.
type IdentityManagerRow struct {
	Name            string
	State           string
	SSHHost         string
	KeyPath         string
	GitFragmentPath string
	Note            string
}

// IdentityManagerRows is the Go mirror of recipeFixtures.ts's
// identityManagerRows — byte-identical names/states/notes, not derived (a
// static, diff-able contract). The "personal" row reuses the SAME
// alias/paths the CreateFlow* and GitScreen* constants use, so "personal"
// stays canonical across surfaces.
var IdentityManagerRows = []IdentityManagerRow{
	{Name: "personal", State: "complete", SSHHost: "personal.github.com", KeyPath: "~/.ssh/id_ed25519_personal", GitFragmentPath: "~/.gitconfig.d/personal", Note: "SSH Host block and Git fragment both present."},
	{Name: "work", State: "incomplete", SSHHost: "work.github.com", KeyPath: "~/.ssh/id_ed25519_work", Note: "SSH Host block present; no Git identity configured for this alias."},
	{Name: "opensource", State: "git-only", GitFragmentPath: "~/.gitconfig.d/opensource", Note: "Git identity relies on the global SSH config; no own Host block."},
	{Name: "archived", State: "key-unused", KeyPath: "~/.ssh/id_ed25519_archived", Note: "Key file exists on disk but no identity references it."},
	{Name: "staging", State: "key-used-ssh-only", SSHHost: "staging.github.com", KeyPath: "~/.ssh/id_ed25519_staging", Note: "Key referenced by a Host block; not wired for Git commit signing."},
	{Name: "clientA", State: "key-used-both", SSHHost: "clienta.github.com", KeyPath: "~/.ssh/id_ed25519_clientA", GitFragmentPath: "~/.gitconfig.d/clientA", Note: "Key wired for both SSH auth and Git commit signing."},
	{Name: "clientB", State: "key-missing", SSHHost: "clientb.github.com", KeyPath: "~/.ssh/id_ed25519_clientB", Note: "Host block references a key file that is absent from disk."},
	{Name: "legacy", State: "fragment-path-missing", SSHHost: "legacy.github.com", GitFragmentPath: "~/.gitconfig.d/legacy", Note: "includeIf points at a Git fragment file that does not exist."},
}

// IdentityManagerActionTarget mirrors recipeFixtures.ts's
// identityManagerActionTarget: action-menu, clone-name-prompt,
// delete-choice, and confirm-destructive all target the fully-populated
// "personal" row.
var IdentityManagerActionTarget = IdentityManagerRows[0] // "personal"

// IdentityManagerDetailTarget mirrors recipeFixtures.ts's
// identityManagerDetailTarget: the detail screen deliberately targets the
// SSH-only "work" row to prove MGR-03/MGR-07 (never fabricate Git
// attributes for an SSH-only identity).
var IdentityManagerDetailTarget = IdentityManagerRows[1] // "work"

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

// MGR-04/MGR-06 literal copy — byte-identical to recipeFixtures.ts's
// identityManagerCloneSuggestedName/identityManagerDeleteChoices.
const (
	// IdentityManagerCloneSuggestedName is the clone prompt's suggested
	// (distinct) name.
	IdentityManagerCloneSuggestedName = "personal-clone"
	// IdentityManagerDeleteChoiceGitOnly is the safe-default delete scope.
	IdentityManagerDeleteChoiceGitOnly = "Delete Git identity only"
	// IdentityManagerDeleteChoiceEverything is the full destructive delete
	// scope (SSH + Git + key).
	IdentityManagerDeleteChoiceEverything = "Delete everything (SSH + Git + key)"

	// IdentityManagerSSHConfigBackupPath is the §5 beat-3 timestamped
	// ~/.ssh/config backup path.
	IdentityManagerSSHConfigBackupPath = "~/.ssh/config.backup.2026-07-03T03-59-12Z"
	// IdentityManagerGitconfigBackupPath is the §5 beat-3 timestamped
	// ~/.gitconfig backup path.
	IdentityManagerGitconfigBackupPath = "~/.gitconfig.backup.2026-07-03T03-59-12Z"
)

// ---------------------------------------------------------------------------
// Global SSH options (recipeFixtures.ts globalSsh* exports)
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
	{Key: "StrictHostKeyChecking", Current: "not set (OpenSSH default: ask)", Risk: "Medium", Recommended: "ask", NeedsAction: true, OneLiner: "Stating \"ask\" explicitly removes ambiguity about how an unknown host key is handled."},
	{Key: "ForwardAgent", Current: "not set (OpenSSH default: no)", Risk: "Medium", Recommended: "no", NeedsAction: true, OneLiner: "Globally forwarding your agent lets any host you connect to authenticate elsewhere as you."},
	{Key: "HashKnownHosts", Current: "not set", Risk: "Low", Recommended: "yes", NeedsAction: true, OneLiner: "Hashing known_hosts hides which hosts you connect to if the file ever leaks."},
	{Key: "IdentitiesOnly", Current: "not set globally (set per-Host by gitid)", Risk: "High", Recommended: "yes", NeedsAction: true, OneLiner: "Without it, ssh may offer every key it knows about to every host — leaking which OTHER keys you hold."},
	{Key: "AddKeysToAgent", Current: "yes", Risk: "Low", Recommended: "yes", NeedsAction: false, OneLiner: "Already set — keys stay available in the agent for the session (recipes/ssh-config.recipe Host * block)."},
	{Key: "UseKeychain", Current: "yes (macOS only)", Risk: "Low", Recommended: "yes", NeedsAction: false, OneLiner: "Already set — stores the key passphrase in the macOS Keychain (guarded by IgnoreUnknown on Linux)."},
}

// GlobalSSHDetailTarget mirrors recipeFixtures.ts's globalSshDetailTarget —
// the option-detail deep-dive target (IdentitiesOnly, the highest-risk
// entry).
var GlobalSSHDetailTarget = GlobalSSHOptions[3] // IdentitiesOnly

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

// §4.4/§5 highest-risk affordance: 3 of 4 "needs action" options applied,
// ForwardAgent deliberately declined — byte-identical to recipeFixtures.ts's
// globalSshChosenToApply/globalSshDeclinedOption.
const (
	// GlobalSSHChosenSummary summarizes how many recommendations the
	// fixture user chose to apply.
	GlobalSSHChosenSummary = "3 of 4"
	// GlobalSSHDeclinedOption is the recommendation deliberately left
	// unchanged (advisory, never required).
	GlobalSSHDeclinedOption = "ForwardAgent"

	// GlobalSSHTargetFile is the file the global-SSH fix writes to.
	GlobalSSHTargetFile = "~/.ssh/config"

	// GlobalSSHSentinelBegin opens the global-ssh managed block.
	GlobalSSHSentinelBegin = "# BEGIN gitid managed: global-ssh"
	// GlobalSSHSentinelEnd closes the global-ssh managed block.
	GlobalSSHSentinelEnd = "# END gitid managed: global-ssh"

	// GlobalSSHHostStarBlockText mirrors recipeFixtures.ts's
	// globalSshHostStarBlockText — extends the recipe's own Host * shape
	// (recipes/ssh-config.recipe) with the 3 chosen recommendations plus
	// the 2 already-recommended options. ForwardAgent is intentionally
	// absent — declined by the user.
	GlobalSSHHostStarBlockText = `IgnoreUnknown UseKeychain

Host *
    StrictHostKeyChecking ask
    HashKnownHosts yes
    IdentitiesOnly yes
    UseKeychain yes
    AddKeysToAgent yes`

	// GlobalSSHBackupPath is ~/.ssh/config's timestamped backup path.
	GlobalSSHBackupPath = "~/.ssh/config.backup.2026-07-03T03-59-12Z"

	// GlobalSSHResultMessage is the success message after the partial
	// apply.
	GlobalSSHResultMessage = "3 of 4 recommended options applied to Host * in ~/.ssh/config. ForwardAgent was left unchanged, as chosen -- advisory, never required."
)

// GlobalSSHManagedBlockText is the sentinel-delimited managed block the
// global-SSH fix appends to ~/.ssh/config.
var GlobalSSHManagedBlockText = GlobalSSHSentinelBegin + "\n" + GlobalSSHHostStarBlockText + "\n" + GlobalSSHSentinelEnd

// GlobalSSHFixPreviewLines mirrors recipeFixtures.ts's
// globalSshFixPreviewLines — the diff-style lines the fix preview shows.
var GlobalSSHFixPreviewLines = []string{
	"+ StrictHostKeyChecking ask",
	"+ HashKnownHosts yes",
	"+ IdentitiesOnly yes",
	"  UseKeychain yes (already set)",
	"  AddKeysToAgent yes (already set)",
	"  ForwardAgent -- left unchanged (declined; advisory, not required)",
}

// ---------------------------------------------------------------------------
// Global Git options (recipeFixtures.ts globalGit* exports)
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

// GlobalGitDetailTarget mirrors recipeFixtures.ts's globalGitDetailTarget —
// the option-detail deep-dive target (init.defaultBranch, the option
// carrying the main-vs-master highlight).
var GlobalGitDetailTarget = GlobalGitOptions[0] // init.defaultBranch

// GlobalGitDetailExplanation is GGIT-01's contractual (verbatim, §3)
// explanation copy — byte-identical to recipeFixtures.ts's
// globalGitDetailExplanation.
const GlobalGitDetailExplanation = `Until Git 2.28 (July 2020), every new repository's default branch was named "master" -- a name inherited from Git's early conventions. GitHub, GitLab, and Bitbucket now all default new repositories to "main" instead, and many teams have followed suit for their own local defaults.

Setting init.defaultBranch = main only affects repositories created AFTER this is set -- it never renames an existing "master" branch in a repository you already have. If you clone or work in a repository whose default branch is still "master" (many older projects have not renamed it), that repository's branch is completely unaffected; this setting only decides what "git init" names the FIRST branch of a brand-new repository.

This is a naming convention, not a security or correctness fix -- it is included here because it is one of the most visible defaults a new gitid user will notice, and stating it explicitly (rather than relying on git's own compiled-in default, or a value some other tool set) keeps the choice intentional and self-documenting.`

// GlobalGitAdvisoryNote — byte-identical to recipeFixtures.ts's
// globalGitAdvisoryNote.
const GlobalGitAdvisoryNote = "Recommended, not required -- you can leave any option unchanged. This is advisory, never a compliance gate."

// Global-git write ceremony literals.
const (
	// GlobalGitTargetFile is the file the global-git fix writes to.
	GlobalGitTargetFile = "~/.gitconfig"

	// GlobalGitSentinelBegin opens the global-git managed block.
	GlobalGitSentinelBegin = "# BEGIN gitid managed: global-git"
	// GlobalGitSentinelEnd closes the global-git managed block.
	GlobalGitSentinelEnd = "# END gitid managed: global-git"

	// GlobalGitBackupPath is ~/.gitconfig's timestamped backup path.
	GlobalGitBackupPath = "~/.gitconfig.backup.2026-07-03T03-59-12Z"

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
// Health (recipeFixtures.ts health* exports). Health is READ-ONLY: it
// diagnoses, it never mutates.
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

// HealthFindings is the Go mirror of recipeFixtures.ts's healthFindings —
// byte-identical ids/sections/severities/copy, not derived (a static,
// diff-able contract). Covers HLTH-03 (redundancy: duplicate Host *),
// HLTH-04 (contradictions: IdentitiesOnly no + an explicit IdentityFile; an
// includeIf targeting a missing fragment), and all four severity levels at
// once.
var HealthFindings = []HealthFinding{
	{
		ID: "ssh-key-perms-archived", Section: "SSH", Severity: SeverityCritical, Family: "Permissions",
		Title:        "Private key is world-readable",
		Explanation:  "~/.ssh/id_ed25519_archived is mode 0644 -- gitid-managed keys must be 0600. Any other account on this machine can read the key material.",
		SuggestedFix: "chmod 0600 ~/.ssh/id_ed25519_archived -- available on the Fixer screen.",
	},
	{
		ID: "ssh-identitiesonly-contradiction", Section: "SSH", Severity: SeverityError, Family: "Coherence",
		Title:        "IdentitiesOnly no contradicts an explicit IdentityFile",
		Explanation:  "Host clientb.github.com sets IdentitiesOnly no while also naming IdentityFile ~/.ssh/id_ed25519_clientB -- ssh may still offer every other key it knows before falling back to the one explicitly configured (HLTH-04).",
		SuggestedFix: "Set IdentitiesOnly yes on the clientb.github.com Host block -- available on the Fixer screen.",
	},
	{
		ID: "git-includeif-missing-fragment", Section: "Git", Severity: SeverityError, Family: "Orphans",
		Title:        "includeIf targets a missing fragment",
		Explanation:  "[includeIf \"gitdir:~/legacy/\"] in ~/.gitconfig points at ~/.gitconfig.d/legacy, which does not exist on disk -- commits made under ~/legacy/ silently fall back to your global git identity instead of \"legacy\" (HLTH-04).",
		SuggestedFix: "Restore ~/.gitconfig.d/legacy, or repoint the includeIf -- available on the Fixer screen.",
	},
	{
		ID: "ssh-duplicate-host-star", Section: "SSH", Severity: SeverityWarning, Family: "Redundancy",
		Title:        "Duplicate Host * stanza",
		Explanation:  "~/.ssh/config defines Host * twice -- line 4 and line 41. The second stanza silently overrides directives set by the first (HLTH-03).",
		SuggestedFix: "Merge the two Host * stanzas into one -- available on the Fixer screen.",
	},
	{
		ID: "git-opensource-no-host-block", Section: "Git", Severity: SeverityInfo, Family: "Overlap",
		Title:       "opensource has no dedicated SSH Host block",
		Explanation: "The \"opensource\" Git identity resolves correctly via its includeIf, but relies entirely on the global SSH config -- there is no gitid-managed Host block scoping which key ssh offers for it. Informational only.",
	},
}

// HealthFindingByID looks up a fixture finding by id, so derived targets
// stay traceably the SAME data (never re-derived copies).
func HealthFindingByID(id string) HealthFinding {
	for _, f := range HealthFindings {
		if f.ID == id {
			return f
		}
	}
	panic("dummytui: data.go: no fixture finding with id " + id)
}

// HealthFindingDetailTarget mirrors recipeFixtures.ts's
// healthFindingDetailTarget — the finding-detail deep-dive target, the
// IdentitiesOnly/IdentityFile contradiction.
var HealthFindingDetailTarget = HealthFindingByID("ssh-identitiesonly-contradiction")

// HealthPerIdentityGitFinding mirrors recipeFixtures.ts's
// healthPerIdentityGitFinding — the per-identity slice's Git finding,
// scoped to the "legacy" identity, the SAME finding as
// git-includeif-missing-fragment above (traceable, not re-derived).
var HealthPerIdentityGitFinding = HealthFindingByID("git-includeif-missing-fragment")

// HealthAllGreen* mirror recipeFixtures.ts's healthAllGreenSummary.
const (
	// HealthAllGreenSSH is the all-green SSH section summary.
	HealthAllGreenSSH = "SSH -- 3 identities, 3 Host blocks, 3 keys checked. All present, all mode 0600, no redundant Host * stanzas, no contradictions."
	// HealthAllGreenGit is the all-green Git section summary.
	HealthAllGreenGit = "Git -- 3 includeIf blocks checked. Every fragment file exists, every allowed_signers email matches its identity's user.email."
)

// HealthPerIdentity* mirror recipeFixtures.ts's healthPerIdentityTarget /
// healthPerIdentitySSHNote — the "legacy" identity (IdentityManagerRows,
// state fragment-path-missing), reused byte-identically so the per-identity
// health slice (HLTH-05) is traceably the SAME data MGR-07's Identity
// Manager row badge derives from.
const (
	// HealthPerIdentityName is the per-identity slice's identity name.
	HealthPerIdentityName = "legacy"
	// HealthPerIdentityState is that identity's MGR-02 state label.
	HealthPerIdentityState = "fragment-path-missing"
	// HealthPerIdentityNote is that identity's row note.
	HealthPerIdentityNote = "includeIf points at a Git fragment file that does not exist."
	// HealthPerIdentitySSHNote is the per-identity slice's healthy SSH
	// section note.
	HealthPerIdentitySSHNote = "Host block present (legacy.github.com), IdentityFile present, IdentitiesOnly yes. No SSH findings for this identity."
	// HealthPerIdentityMgrHandoff states the MGR-07 hand-off from this
	// slice to the Identity Manager row.
	HealthPerIdentityMgrHandoff = "This slice feeds the Identity Manager row for " + HealthPerIdentityName + " (MGR-07): " + HealthPerIdentityNote
)

// HealthParseError* mirror recipeFixtures.ts's healthParseErrorTarget —
// HLTH-02's parse-error example: the one condition Health can only report,
// reinforcing read-only integrity concretely.
const (
	// HealthParseErrorFile is the unparseable fragment's path.
	HealthParseErrorFile = "~/.gitconfig.d/work"
	// HealthParseErrorRaw is git's raw parse error output.
	HealthParseErrorRaw = "error: bad config line 4 in file ~/.gitconfig.d/work"
	// HealthParseErrorSnippet is the offending line snippet.
	HealthParseErrorSnippet = "line 4:     signingkey = \"~/.ssh/id_ed25519_work.pub"
	// HealthParseErrorExplanation explains the parse failure's effect.
	HealthParseErrorExplanation = "A signingkey value is missing its closing quote -- git cannot parse this file at all, so no Git identity check can run for \"work\" until it parses again."
)

// HealthReadOnlyNote — byte-identical to recipeFixtures.ts's
// healthReadOnlyNote: the explicit, negatively-checkable read-only
// statement shown on every health screen.
const HealthReadOnlyNote = "Health only diagnoses -- nothing here writes to your files. Open the Fixer (key 5) to change anything shown."

// ---------------------------------------------------------------------------
// Fixer (recipeFixtures.ts fixer* exports). The fixer lists only ACTIONABLE
// problems — the subset of HealthFindings carrying a SuggestedFix.
// ---------------------------------------------------------------------------

// FixerFindings is the Go mirror of recipeFixtures.ts's fixerFindings — the
// subset of HealthFindings that carries a SuggestedFix (§4.7's "each
// problem: severity + plain explanation + suggested fix"). Byte-identical
// ids/sections/severities/copy to HealthFindings — traceable, not
// re-derived (HLTH-04's own "available on the Fixer screen" hand-off).
var FixerFindings = []HealthFinding{
	HealthFindingByID("ssh-key-perms-archived"),
	HealthFindingByID("ssh-identitiesonly-contradiction"),
	HealthFindingByID("git-includeif-missing-fragment"),
	HealthFindingByID("ssh-duplicate-host-star"),
}

// FixerTarget mirrors recipeFixtures.ts's fixerTarget — the flagship §4.7
// walk-through target: a fix-in-place REWRITE of an EXISTING directive's
// value (IdentitiesOnly no -> yes on Host clientb.github.com), not merely
// an addition. The SAME finding HealthFindingDetailTarget deep-dives
// (traceable HLTH-04 hand-off).
var FixerTarget = HealthFindingByID("ssh-identitiesonly-contradiction")

// Fixer write-ceremony literals.
const (
	// FixerTargetFile is the file the flagship fix rewrites.
	FixerTargetFile = "~/.ssh/config"
	// FixerTargetHost is the Host block the flagship fix rewrites.
	FixerTargetHost = "clientb.github.com"

	// FixerBackupPath is the timestamped backup taken before applying.
	FixerBackupPath = "~/.ssh/config.backup.2026-07-03T03-59-12Z"

	// FixerResultMessage is the success message after the rewrite.
	FixerResultMessage = "IdentitiesOnly set to yes on Host " + FixerTargetHost + " in " + FixerTargetFile + "."

	// FixerConfirmDestructiveNote is the strongest-confirm copy shown
	// before the in-place rewrite is applied.
	FixerConfirmDestructiveNote = "This rewrites a directive already present in your SSH config. Review the diff above before confirming -- this cannot be undone without restoring the backup."

	// FixerSafetyNote mirrors recipeFixtures.ts's fixerSafetyNote — shown
	// on every fixer screen (§4.7, §5): fixes are always previewed,
	// confirmed, and backed up before anything is written.
	FixerSafetyNote = "Every fix is previewed, confirmed, and backed up before anything is written -- never a blind write."
)

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

// FixerBatchFixNote mirrors recipeFixtures.ts's fixerBatchFixNote (§4.7:
// "Batch-fix (if offered) must still preview every change; no silent
// multi-file mutation.").
var FixerBatchFixNote = "Apply all " + strconv.Itoa(len(FixerFindings)) + " fixes -- each one still previews its own diff and backup path before writing; nothing is applied silently."
