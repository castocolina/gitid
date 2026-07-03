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

export const insteadOfBlockText = `[url "git@${sshIdentityAlias.host}:"]
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
