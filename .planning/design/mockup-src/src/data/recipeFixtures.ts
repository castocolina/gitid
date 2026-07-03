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
