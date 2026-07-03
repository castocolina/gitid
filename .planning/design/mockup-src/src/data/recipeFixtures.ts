/**
 * Recipe-accurate canonical config copy — the single typed source every
 * mockup surface pulls from (real values only, no placeholder option lists
 * per 02-UX-DIRECTION.md §0 Risk 3).
 *
 * Every string here is derived directly from `recipes/ssh-config.recipe` and
 * `recipes/gitconfig.recipe` (the North Star; see `recipes/README.md`) —
 * structure and field values are recipe-faithful, but the key ALGORITHM is
 * ed25519, not the gists' RSA, per the recipes' own "structure, not key
 * type" caveat. The identity alias used throughout is `personal`
 * (`personal.github.com`), matching the recipes' own worked example.
 */

// ---------------------------------------------------------------------------
// SSH: alias-per-identity Host block (recipes/ssh-config.recipe)
// ---------------------------------------------------------------------------

export const sshIdentityAlias = {
  identityName: 'personal',
  host: 'personal.github.com',
  hostname: 'ssh.github.com',
  port: 443,
  user: 'git',
  identityFile: '~/.ssh/id_ed25519_personal',
  identitiesOnly: true,
} as const;

/**
 * The exact `Host` block gitid writes for the alias above. Written as a
 * literal (not interpolated from `sshIdentityAlias`) so the recipe-critical
 * field values (`Port 443`, `IdentitiesOnly yes`) are byte-visible in this
 * source file, not just at runtime — a static contract, not just a rendered
 * one. A test in a later plan should assert this literal stays in sync with
 * `sshIdentityAlias`.
 */
export const sshIdentityAliasBlockText = `Host personal.github.com
    Hostname ssh.github.com
    Port 443
    User git
    IdentityFile ~/.ssh/id_ed25519_personal
    IdentitiesOnly yes`;

// ---------------------------------------------------------------------------
// SSH: macOS globals block (recipes/ssh-config.recipe, guarded by
// IgnoreUnknown so it is a no-op on Linux)
// ---------------------------------------------------------------------------

export const sshMacGlobalsBlockText = `IgnoreUnknown UseKeychain

Host *
    UseKeychain yes
    AddKeysToAgent yes`;

// ---------------------------------------------------------------------------
// Git: match-strategy — hasconfig (recipe PRIMARY) and gitdir (recipe
// ALTERNATIVE, and gitid's own §3 default per 02-UX-DIRECTION.md). Both
// values are recipe-accurate and both are shown in the match-strategy
// picker (create-flow / git-screen surfaces).
// ---------------------------------------------------------------------------

export type MatchStrategy = 'hasconfig' | 'gitdir' | 'both';

/** gitid's own default match strategy (02-UX-DIRECTION.md §3, §6). */
export const defaultMatchStrategy: MatchStrategy = 'gitdir';

export const gitconfigFragmentPath = `~/.gitconfig_${sshIdentityAlias.identityName}`;

export const includeIfHasconfigLine = `[includeIf "hasconfig:remote.*.url:git@${sshIdentityAlias.host}:*/**"]
    path = ${gitconfigFragmentPath}`;

export const includeIfGitdirLine = `[includeIf "gitdir:~/personal/"]
    path = ${gitconfigFragmentPath}`;

// ---------------------------------------------------------------------------
// Git: URL rewriting — insteadOf (recipes/gitconfig.recipe)
// ---------------------------------------------------------------------------

// The insteadOf URL rewrite is a PROVIDER-level baseline concern (git@github.com:),
// NOT keyed to a per-identity alias — matches recipes/README.md and the shipped
// gitconfig.DefaultURLRewrites(). Keeping it provider-generic prevents the wrong
// shape from propagating to every later surface that consumes this fixture.
export const insteadOfBlockText = `[url "git@github.com:"]
    insteadOf = https://github.com/`;

// ---------------------------------------------------------------------------
// Git: per-identity fragment (recipes/gitconfig.recipe "Example:
// ~/.gitconfig_personal", superseded from GPG to ssh-signing per PROJECT.md
// "Signing: one ed25519 key per identity for both auth and signing via
// gpg.format=ssh + allowed_signers; no GPG")
// ---------------------------------------------------------------------------

export const personalIdentityGitFragment = {
  userName: 'Personal Identity',
  // Non-PII placeholder, structurally identical to the recipe's own worked
  // example ("john@personal.com") — this exact string must be
  // byte-identical to the allowedSignersLine email below (GITUI-04).
  userEmail: 'you@personal.example',
  gpgFormat: 'ssh',
  // signingkey is a PATH to the public key, never the key material itself.
  signingKey: `${sshIdentityAlias.identityFile}.pub`,
  commitGpgsign: true,
} as const;

export const personalIdentityGitFragmentText = `[user]
    name = ${personalIdentityGitFragment.userName}
    email = ${personalIdentityGitFragment.userEmail}
    signingkey = ${personalIdentityGitFragment.signingKey}

[gpg]
    format = ${personalIdentityGitFragment.gpgFormat}

[commit]
    gpgsign = ${personalIdentityGitFragment.commitGpgsign}`;

// ---------------------------------------------------------------------------
// ~/.ssh/allowed_signers — email MUST be byte-identical to user.email above
// (GITUI-04, the git-screen's highest-risk affordance per
// 02-UX-DIRECTION.md §4(2)).
// ---------------------------------------------------------------------------

export const allowedSignersLine = `${personalIdentityGitFragment.userEmail} ${
  // Fixed, valid-shaped ed25519 public-key material for design purposes
  // only — not a real key.
  'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIDesignMockupFixtureKeyNotReal0'
}`;

// ---------------------------------------------------------------------------
// Git: global recipe defaults (recipes/gitconfig.recipe "Example:
// ~/.gitconfig_default" + [core]/[push] top-level blocks)
// ---------------------------------------------------------------------------

export const globalGitDefaults = {
  initDefaultBranch: 'main',
  coreIgnorecase: false,
  pushAutoSetupRemote: true,
  pullRebase: true,
  fetchPrune: true,
  mergeConflictstyle: 'diff3',
  diffColorMoved: 'zebra',
} as const;

export const globalGitDefaultsBlockText = `[init]
    defaultBranch = ${globalGitDefaults.initDefaultBranch}

[core]
    ignorecase = ${globalGitDefaults.coreIgnorecase}

[push]
    autoSetupRemote = ${globalGitDefaults.pushAutoSetupRemote}

[pull]
    rebase = ${globalGitDefaults.pullRebase}

[fetch]
    prune = ${globalGitDefaults.fetchPrune}

[merge]
    conflictstyle = ${globalGitDefaults.mergeConflictstyle}

[diff]
    colorMoved = ${globalGitDefaults.diffColorMoved}`;

// ---------------------------------------------------------------------------
// Managed-block sentinels (CLAUDE.md "Engineering": idempotent managed
// blocks, never a blind append) — every gitconfig write is wrapped in these
// so the live preview can show gitid only owns its own block.
// ---------------------------------------------------------------------------

export function managedBlockSentinels(identityName: string) {
  return {
    begin: `# BEGIN gitid managed: ${identityName}`,
    end: `# END gitid managed: ${identityName}`,
  };
}

export const personalManagedBlockSentinels = managedBlockSentinels(
  sshIdentityAlias.identityName,
);

export const personalManagedBlockText = `${personalManagedBlockSentinels.begin}
${personalIdentityGitFragmentText}
${personalManagedBlockSentinels.end}`;

// ---------------------------------------------------------------------------
// Backup-notice sample — a stable, timestamped path string for the mutation-
// ceremony `backup-notice` states (02-UX-DIRECTION.md §5, beat 3).
// ---------------------------------------------------------------------------

export const sampleBackupPath = '~/.ssh/config.backup.2026-07-03T03-59-12Z';
export const sampleGitconfigBackupPath =
  '~/.gitconfig.backup.2026-07-03T03-59-12Z';

// ---------------------------------------------------------------------------
// Create-flow pilot surface (02-UX-DIRECTION.md §4.1) — algorithm catalog,
// SSH-form field defaults, two-stage test commands/output, and the
// confirm/backup/result copy every `create-flow/*.route.tsx` screen and the
// mirrored `internal/dummytui/surface_createflow.go` render byte-identically
// (REQUIREMENTS.md KEY-01/KEY-03, SSHUI-01/02/03, TEST-01/02).
// ---------------------------------------------------------------------------

export type AlgorithmAvailability = 'native' | 'requires-libfido2';

export interface AlgorithmCatalogEntry {
  id: string;
  label: string;
  recommended: boolean;
  security: string;
  macos: string;
  macosAvailability: AlgorithmAvailability;
  linux: string;
  linuxAvailability: AlgorithmAvailability;
}

/**
 * KEY-01's top-5 algorithm catalog. `ed25519` is the best/default
 * recommendation; the other four are registered-but-not-generatable stubs on
 * the real backend (01-02-PLAN.md) — the mockup still SHOWS all five with
 * accurate local-availability notes (macOS LibreSSL / Linux OpenSSL parity;
 * the two `-sk` hardware variants need `libfido2` + a physical FIDO2 key on
 * both platforms).
 */
export const algorithmCatalog: AlgorithmCatalogEntry[] = [
  {
    id: 'ed25519',
    label: 'ed25519',
    recommended: true,
    security:
      'Modern EdDSA curve — small keys, fast, constant-time (timing-attack resistant). The recommended default for every new identity.',
    macos: 'Native (LibreSSL) — always available',
    macosAvailability: 'native',
    linux: 'Native (OpenSSL) — always available',
    linuxAvailability: 'native',
  },
  {
    id: 'ed25519-sk',
    label: 'ed25519-sk',
    recommended: false,
    security:
      'Hardware-backed: private key material never leaves the security key; requires a physical touch to sign. Strongest theft resistance of the five.',
    macos: 'Needs libfido2 + a FIDO2 security key plugged in',
    macosAvailability: 'requires-libfido2',
    linux: 'Needs libfido2 + a FIDO2 security key plugged in',
    linuxAvailability: 'requires-libfido2',
  },
  {
    id: 'rsa-4096',
    label: 'rsa-4096',
    recommended: false,
    security:
      'Strong at 4096 bits; widely compatible with legacy servers, but larger keys and slower signing than ed25519.',
    macos: 'Native — always available',
    macosAvailability: 'native',
    linux: 'Native — always available',
    linuxAvailability: 'native',
  },
  {
    id: 'ecdsa-p256',
    label: 'ecdsa-p256',
    recommended: false,
    security:
      'Compact NIST P-256 curve; smaller than RSA, though some users distrust NIST curve provenance versus ed25519.',
    macos: 'Native — always available',
    macosAvailability: 'native',
    linux: 'Native — always available',
    linuxAvailability: 'native',
  },
  {
    id: 'ecdsa-sk',
    label: 'ecdsa-sk',
    recommended: false,
    security:
      'Hardware-backed ECDSA variant of ed25519-sk; physical security-key touch required to sign.',
    macos: 'Needs libfido2 + a FIDO2 security key plugged in',
    macosAvailability: 'requires-libfido2',
    linux: 'Needs libfido2 + a FIDO2 security key plugged in',
    linuxAvailability: 'requires-libfido2',
  },
];

// SSH-form field defaults (SSHUI-01 field order: Alias prefix -> SSH Host ->
// Real hostname -> Port, default 443). `sshFormFilled` is the filled state;
// `sshFormAliasPrefix`/`sshFormBlankPrefixHost` demonstrate the blank-prefix
// WYSIWYG rule (SSHUI-01: blank prefix -> SSH Host = the provider host
// itself, no invented suffix).
export const sshFormFilled = {
  aliasPrefix: sshIdentityAlias.identityName,
  sshHost: sshIdentityAlias.host,
  realHostname: sshIdentityAlias.hostname,
  port: sshIdentityAlias.port,
} as const;

export const sshFormBlankPrefixHost = 'github.com';

// Two-stage connectivity test (TEST-01/TEST-02): stage 1 tests the key
// DIRECT against the bare provider URL (no alias yet); stage 2 tests BY THE
// ALIAS and proves, via `ssh -G`, which IdentityFile actually resolves for
// that alias. Both stages run against a throwaway temp config
// (SSHUI-04) — the live `~/.ssh/config` is untouched until confirm-write.
export const sshTestTmpConfigPath = '/tmp/gitid-test-a1b2c3.config';

export const sshTestStage1Command = `ssh -T -F ${sshTestTmpConfigPath} -p ${sshIdentityAlias.port} -i ${sshIdentityAlias.identityFile} git@${sshIdentityAlias.hostname}`;
export const sshTestStage1Output =
  "Hi personal! You've successfully authenticated, but GitHub does not provide shell access.";

export const sshTestStage2Command = `ssh -G ${sshIdentityAlias.host} -F ${sshTestTmpConfigPath} | grep identityfile`;
export const sshTestStage2Output = `identityfile ${sshIdentityAlias.identityFile}`;

export const sshTestFailCommand = sshTestStage1Command;
export const sshTestFailOutput = `git@${sshIdentityAlias.hostname}: Permission denied (publickey).`;

// Mutation-ceremony copy for the create flow's confirm/backup/result states
// (§5 four-beat ceremony). `confirmWriteTargetFile` names the file that gets
// the managed block; `sampleBackupPath` (above) is reused for the backup
// notice so both media show the SAME timestamped path.
export const confirmWriteTargetFile = '~/.ssh/config';
export const createFlowManagedBlockSentinels = managedBlockSentinels(
  sshIdentityAlias.identityName,
);
export const createFlowManagedBlockText = `${createFlowManagedBlockSentinels.begin}
${sshIdentityAliasBlockText}
${createFlowManagedBlockSentinels.end}`;

export const resultSuccessMessage = `Identity "${sshIdentityAlias.identityName}" created — ${sshIdentityAlias.host} now resolves to ${sshIdentityAlias.identityFile}.`;

// ---------------------------------------------------------------------------
// git-screen surface (02-UX-DIRECTION.md §4(2), Phase 4) — the
// per-identity Git configuration screen. REQUIREMENTS.md GITUI-02 (already
// built) fixes the fragment target at `~/.gitconfig.d/<identity>` — the
// PROJECT'S OWN established convention, distinct from
// `recipes/gitconfig.recipe`'s own `~/.gitconfig_<identity>` naming that the
// create-flow pilot's `includeIf*Line` literals above reuse verbatim
// (CLAUDE.md "Surface any divergence between current behavior and the
// recipes explicitly" — the divergence is the fragment PATH convention, not
// structure). These are NEW exports; nothing above this section is modified.
// ---------------------------------------------------------------------------

/** GITUI-02: the per-identity Git fragment file gitid actually writes to. */
export const gitScreenFragmentPath = `~/.gitconfig.d/${sshIdentityAlias.identityName}`;

export const gitScreenIncludeIfGitdirLine = `[includeIf "gitdir:~/${sshIdentityAlias.identityName}/"]
    path = ${gitScreenFragmentPath}`;

export const gitScreenIncludeIfHasconfigLine = `[includeIf "hasconfig:remote.*.url:git@${sshIdentityAlias.host}:*/**"]
    path = ${gitScreenFragmentPath}`;

export const gitScreenIncludeIfBothLines = `${gitScreenIncludeIfGitdirLine}

${gitScreenIncludeIfHasconfigLine}`;

/**
 * The live `includeIf` preview shown on `match-strategy-select`, keyed by
 * the same `MatchStrategy` union create-flow's `defaultMatchStrategy` uses.
 * `gitdir` is the default (02-UX-DIRECTION.md §3, §6; GITUI-03).
 */
export const gitScreenMatchStrategyPreview: Record<MatchStrategy, string> = {
  gitdir: gitScreenIncludeIfGitdirLine,
  hasconfig: gitScreenIncludeIfHasconfigLine,
  both: gitScreenIncludeIfBothLines,
};

export const gitScreenManagedBlockSentinels = managedBlockSentinels(
  sshIdentityAlias.identityName,
);

/** The exact fragment-file contents gitid writes to `gitScreenFragmentPath`. */
export const gitScreenManagedFragmentText = `${gitScreenManagedBlockSentinels.begin}
${personalIdentityGitFragmentText}
${gitScreenManagedBlockSentinels.end}`;

/**
 * The block appended to `~/.gitconfig` itself (the default `gitdir`
 * strategy's `includeIf`, sentineled the same way as every other managed
 * block so the live preview shows containment, §2/§5).
 */
export const gitScreenGitconfigIncludeBlockText = `${gitScreenManagedBlockSentinels.begin}
${gitScreenIncludeIfGitdirLine}
${gitScreenManagedBlockSentinels.end}`;

/** GITUI-05: confirm-write shows all three targets this screen mutates. */
export const gitScreenConfirmTargets = {
  fragmentFile: gitScreenFragmentPath,
  gitconfigFile: '~/.gitconfig',
  allowedSignersFile: '~/.ssh/allowed_signers',
} as const;

export const gitScreenAllowedSignersBackupPath =
  '~/.ssh/allowed_signers.backup.2026-07-03T03-59-12Z';

export const gitScreenResultSuccessMessage = `Git identity "${sshIdentityAlias.identityName}" configured — ${gitScreenFragmentPath} now applies via the ${defaultMatchStrategy} match strategy.`;

// ---------------------------------------------------------------------------
// identity-manager surface (02-UX-DIRECTION.md §4(3), Phase 5) — the app's
// HOME view (number key `1`). One fixture identity per MGR-02 8-label state
// taxonomy (internal/identity/state.go's LOCKED `State` vocabulary), so
// `list-populated` demonstrates every label at once, legibly under
// `NO_COLOR` (glyph + word, never color alone — 02-UX-DIRECTION.md §2). The
// `personal` row reuses `sshIdentityAlias`/`gitScreenFragmentPath` above so
// the SAME "personal" alias/copy stays canonical across create-flow,
// git-screen, and identity-manager. These are NEW exports; nothing above
// this section is modified.
// ---------------------------------------------------------------------------

/** The 8 locked MGR-02 state labels — MUST stay byte-identical to
 * internal/identity/state.go's State constants (the shared vocabulary). */
export type IdentityManagerState =
  | 'complete'
  | 'incomplete'
  | 'git-only'
  | 'key-unused'
  | 'key-used-ssh-only'
  | 'key-used-both'
  | 'key-missing'
  | 'fragment-path-missing';

export interface IdentityManagerRow {
  name: string;
  state: IdentityManagerState;
  sshHost?: string;
  keyPath?: string;
  gitFragmentPath?: string;
  /** Per-row explanation of WHY this identity is in this state — the
   * legible-under-NO_COLOR word half of the glyph+word pairing. */
  note: string;
}

/** Glyph half of the color-semantics table (02-UX-DIRECTION.md §2):
 * healthy=✓, needs-action/advisory=!, error/destructive/missing=✗. Paired
 * with `identityManagerStateTone`'s color AND the state's own word (the
 * label itself) so meaning is never carried by color alone. */
export const identityManagerStateGlyph: Record<IdentityManagerState, string> = {
  complete: '✓',
  incomplete: '!',
  'git-only': '!',
  'key-unused': '!',
  'key-used-ssh-only': '✓',
  'key-used-both': '✓',
  'key-missing': '✗',
  'fragment-path-missing': '✗',
};

export const identityManagerStateTone: Record<
  IdentityManagerState,
  'success' | 'warning' | 'error'
> = {
  complete: 'success',
  incomplete: 'warning',
  'git-only': 'warning',
  'key-unused': 'warning',
  'key-used-ssh-only': 'success',
  'key-used-both': 'success',
  'key-missing': 'error',
  'fragment-path-missing': 'error',
};

/**
 * `list-populated`'s 8 rows — exactly one per MGR-02 label, in the label's
 * own severity order (complete first, the two `error`-tone labels last).
 */
export const identityManagerRows: IdentityManagerRow[] = [
  {
    name: sshIdentityAlias.identityName, // 'personal'
    state: 'complete',
    sshHost: sshIdentityAlias.host,
    keyPath: sshIdentityAlias.identityFile,
    gitFragmentPath: gitScreenFragmentPath,
    note: 'SSH Host block and Git fragment both present.',
  },
  {
    name: 'work',
    state: 'incomplete',
    sshHost: 'work.github.com',
    keyPath: '~/.ssh/id_ed25519_work',
    note: 'SSH Host block present; no Git identity configured for this alias.',
  },
  {
    name: 'opensource',
    state: 'git-only',
    gitFragmentPath: '~/.gitconfig.d/opensource',
    note: 'Git identity relies on the global SSH config; no own Host block.',
  },
  {
    name: 'archived',
    state: 'key-unused',
    keyPath: '~/.ssh/id_ed25519_archived',
    note: 'Key file exists on disk but no identity references it.',
  },
  {
    name: 'staging',
    state: 'key-used-ssh-only',
    sshHost: 'staging.github.com',
    keyPath: '~/.ssh/id_ed25519_staging',
    note: 'Key referenced by a Host block; not wired for Git commit signing.',
  },
  {
    name: 'clientA',
    state: 'key-used-both',
    sshHost: 'clienta.github.com',
    keyPath: '~/.ssh/id_ed25519_clientA',
    gitFragmentPath: '~/.gitconfig.d/clientA',
    note: 'Key wired for both SSH auth and Git commit signing.',
  },
  {
    name: 'clientB',
    state: 'key-missing',
    sshHost: 'clientb.github.com',
    keyPath: '~/.ssh/id_ed25519_clientB',
    note: 'Host block references a key file that is absent from disk.',
  },
  {
    name: 'legacy',
    state: 'fragment-path-missing',
    sshHost: 'legacy.github.com',
    gitFragmentPath: '~/.gitconfig.d/legacy',
    note: 'includeIf points at a Git fragment file that does not exist.',
  },
];

/**
 * `detail-ssh-first`'s target: the `work` identity (state `incomplete`,
 * SSH-only). Chosen deliberately over the fully-populated `personal` row so
 * the screen proves MGR-03/07's highest-value case: SSH details shown
 * first, and the Git section explicitly says "not configured" rather than
 * ever rendering fabricated Git attributes for an SSH-only identity.
 */
export const identityManagerDetailTarget = identityManagerRows[1] as IdentityManagerRow; // 'work'

/**
 * `action-menu` / `clone-name-prompt` / `delete-choice` / `confirm-
 * destructive` target the fully-populated `personal` identity — the
 * richest row, so both the safe clone path and the irreversible delete
 * path are demonstrated against a complete identity with a Git fragment.
 */
export const identityManagerActionTarget = identityManagerRows[0] as IdentityManagerRow; // 'personal'

/** clone-name-prompt (MGR-04): the suggested name is DISTINCT from the
 * source identity's own name — never a bare duplicate. */
export const identityManagerCloneSuggestedName = 'personal-clone';

/** delete-choice (MGR-06): two destructive options. The safer one (Git
 * identity only) is default-focused; the irreversible "everything" option
 * carries the strongest confirm on the NEXT screen (confirm-destructive,
 * 02-UX-DIRECTION.md §5). */
export const identityManagerDeleteChoices = {
  everything: 'Delete everything (SSH + Git + key)',
  gitOnly: 'Delete Git identity only',
} as const;

/** backup-notice (§5 beat 3): both files this delete touches get a
 * timestamped backup — reusing the SAME timestamp convention as
 * create-flow/git-screen's own backup paths. */
export const identityManagerBackupPaths = {
  sshConfig: sampleBackupPath,
  gitconfig: sampleGitconfigBackupPath,
} as const;

// ---------------------------------------------------------------------------
// global-ssh surface (02-UX-DIRECTION.md §4(4), Phase 6) — a master-detail
// surface (number key `2`) reviewing SSH options that are DANGEROUS BY
// DEFAULT WHEN UNSET (GSSH-01, REQUIREMENTS.md). Pins the previously-open
// "GSSH-01 option list" item to the exact 6-option set 02-07-PLAN.md
// specifies: StrictHostKeyChecking, ForwardAgent, HashKnownHosts,
// IdentitiesOnly, AddKeysToAgent, UseKeychain. Recommendations are
// ADVISORY, NEVER BLOCKING (§4.4, §5): a yellow `!`, never a red block, and
// the user may leave any option unchanged — `globalSshChosenToApply` /
// `globalSshDeclinedOption` below demonstrate this concretely (the user
// applies 3 of 4 "needs action" recommendations and deliberately leaves
// ForwardAgent unchanged). AddKeysToAgent/UseKeychain are already
// recipe-recommended (recipes/ssh-config.recipe's `Host *` block under
// `IgnoreUnknown UseKeychain`), so the option set demonstrates BOTH
// "already fine" (✓) and "needs action" (!) rows, not just a wall of
// warnings. These are NEW exports; nothing above this section is modified.
// ---------------------------------------------------------------------------

export type GlobalSSHRiskLevel = 'Low' | 'Medium' | 'High';

export interface GlobalSSHOption {
  key: string;
  currentValue: string;
  risk: GlobalSSHRiskLevel;
  recommendedValue: string;
  needsAction: boolean;
  oneLiner: string;
}

/** The GSSH-01 dangerous-by-default option set, each with current value +
 * risk + recommended value + a one-line explanation (§3 "explain each
 * option"). Order matches 02-UX-DIRECTION.md §4.4's verbatim list. */
export const globalSshOptions: GlobalSSHOption[] = [
  {
    key: 'StrictHostKeyChecking',
    currentValue: 'not set (OpenSSH default: ask)',
    risk: 'Medium',
    recommendedValue: 'ask',
    needsAction: true,
    oneLiner:
      'Stating "ask" explicitly removes ambiguity about how an unknown host key is handled.',
  },
  {
    key: 'ForwardAgent',
    currentValue: 'not set (OpenSSH default: no)',
    risk: 'Medium',
    recommendedValue: 'no',
    needsAction: true,
    oneLiner:
      'Globally forwarding your agent lets any host you connect to authenticate elsewhere as you.',
  },
  {
    key: 'HashKnownHosts',
    currentValue: 'not set',
    risk: 'Low',
    recommendedValue: 'yes',
    needsAction: true,
    oneLiner: 'Hashing known_hosts hides which hosts you connect to if the file ever leaks.',
  },
  {
    key: 'IdentitiesOnly',
    currentValue: 'not set globally (set per-Host by gitid)',
    risk: 'High',
    recommendedValue: 'yes',
    needsAction: true,
    oneLiner:
      'Without it, ssh may offer every key it knows about to every host — leaking which OTHER keys you hold.',
  },
  {
    key: 'AddKeysToAgent',
    currentValue: 'yes',
    risk: 'Low',
    recommendedValue: 'yes',
    needsAction: false,
    oneLiner:
      'Already set — keys stay available in the agent for the session (recipes/ssh-config.recipe Host * block).',
  },
  {
    key: 'UseKeychain',
    currentValue: 'yes (macOS only)',
    risk: 'Low',
    recommendedValue: 'yes',
    needsAction: false,
    oneLiner:
      'Already set — stores the key passphrase in the macOS Keychain (guarded by IgnoreUnknown on Linux).',
  },
];

/** option-detail's target — the single highest-risk option (IdentitiesOnly)
 * gets the full explanatory treatment, mirroring identity-manager's
 * single-target `detail-ssh-first` precedent. */
export const globalSshDetailTarget = globalSshOptions[3] as GlobalSSHOption; // IdentitiesOnly

export const globalSshDetailExplanation = `When IdentitiesOnly is not set (or set to "no"), ssh may try EVERY key it can find — every file in ~/.ssh matching the default names, plus every key already loaded in your ssh-agent — against any host you connect to. On a machine with multiple identities (personal, work, client keys), this means:

  - the wrong key can be offered first, revealing to a server which OTHER keys you hold;
  - a host you don't fully trust can trigger authentication attempts meant for a completely different identity.

Setting "IdentitiesOnly yes" on a Host block restricts ssh to ONLY the IdentityFile(s) listed for that host — this is why every gitid-managed Host block (recipes/ssh-config.recipe) already sets it per-identity. This screen recommends also stating it explicitly in the global Host * block, as a safety net for any Host entries gitid does not manage.`;

export const globalSshAdvisoryNote =
  'Recommended, not required — you can leave any option unchanged. This is advisory, never a compliance gate.';

/** The 3 of 4 "needs action" options the user chose to apply on
 * fix-preview (ForwardAgent is deliberately LEFT unchanged) — a concrete
 * demonstration that recommendations are advisory, never blocking
 * (§4.4, §5). */
export const globalSshChosenToApply = ['StrictHostKeyChecking', 'HashKnownHosts', 'IdentitiesOnly'];
export const globalSshDeclinedOption = 'ForwardAgent';

export const globalSshTargetFile = '~/.ssh/config';
export const globalSshManagedBlockSentinels = managedBlockSentinels('global-ssh');

/** The exact Host * block gitid writes, extending the recipe's own
 * `IgnoreUnknown UseKeychain` / `Host *` shape (recipes/ssh-config.recipe)
 * with the 3 chosen recommendations plus the 2 already-recommended
 * options. ForwardAgent is intentionally absent — declined by the user. */
export const globalSshHostStarBlockText = `IgnoreUnknown UseKeychain

Host *
    StrictHostKeyChecking ask
    HashKnownHosts yes
    IdentitiesOnly yes
    UseKeychain yes
    AddKeysToAgent yes`;

export const globalSshManagedBlockText = `${globalSshManagedBlockSentinels.begin}
${globalSshHostStarBlockText}
${globalSshManagedBlockSentinels.end}`;

/** fix-preview's diff-style lines: `+` for newly-applied recommendations,
 * two spaces for already-set options, and an explicit "declined" line for
 * the one recommendation the user chose NOT to apply. */
export const globalSshFixPreviewLines = [
  '+ StrictHostKeyChecking ask',
  '+ HashKnownHosts yes',
  '+ IdentitiesOnly yes',
  '  UseKeychain yes (already set)',
  '  AddKeysToAgent yes (already set)',
  '  ForwardAgent — left unchanged (declined; advisory, not required)',
];

export const globalSshBackupPath = '~/.ssh/config.backup.2026-07-03T03-59-12Z';

export const globalSshResultMessage =
  '3 of 4 recommended options applied to Host * in ~/.ssh/config. ForwardAgent was left unchanged, as chosen — advisory, never required.';
