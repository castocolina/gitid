/**
 * screenSignatures.ts — one screen-specific `SIG-...` marker per static
 * reference route, keyed by the SAME "<surface>/<screen>" ScreenID every
 * route's `Shell` `title` prop already carries. `Shell.tsx` looks up
 * `screenSignatures[title]` and renders it as a small marker, so any
 * capture/assertion can require BOTH the breadcrumb and the signature —
 * a breadcrumb alone cannot catch a "right route, wrong BODY" false
 * positive (review B1 / Codex cross-vendor finding HIGH; T-02-FP).
 *
 * Originally this map mirrored the per-surface capture `manifest.json`
 * files and the static Go dummy's own `sig*` constants; both were removed
 * when the static PNG reference set was superseded by the interactive
 * demo (`src/demo/`). The map stays: it is now the single authority for
 * the per-screen signatures the 50 static reference routes render, a
 * static, diff-able contract (NOT derived at build time — Vite cannot
 * read outside `mockup-src/`'s root).
 */
export const screenSignatures: Record<string, string> = {
  'create-flow/algo-catalog': 'SIG-ALGO-CATALOG-ED25519-DEFAULT',
  'create-flow/backup-notice': 'SIG-BACKUP-NOTICE',
  'create-flow/confirm-write': 'SIG-CONFIRM-WRITE',
  'create-flow/macos-globals-block': 'SIG-MACOS-GLOBALS-BLOCK',
  'create-flow/result-success': 'SIG-RESULT-SUCCESS',
  'create-flow/reuse-key-vs-generate': 'SIG-REUSE-KEY-VS-GENERATE',
  'create-flow/ssh-form-blank-prefix': 'SIG-SSH-FORM-BLANK-PREFIX-WYSIWYG',
  'create-flow/ssh-form-empty': 'SIG-SSH-FORM-EMPTY',
  'create-flow/ssh-form-filled': 'SIG-SSH-FORM-FILLED-LIVE-PREVIEW',
  'create-flow/test-fail': 'SIG-TEST-FAIL',
  'create-flow/test-stage1-direct': 'SIG-TEST-STAGE1-DIRECT',
  'create-flow/test-stage2-by-alias': 'SIG-TEST-STAGE2-BY-ALIAS',
  'fixer/backup-notice': 'SIG-FIX-BACKUP-NOTICE',
  'fixer/confirm-destructive': 'SIG-FIX-CONFIRM-DESTRUCTIVE-REWRITE',
  'fixer/fix-preview': 'SIG-FIX-PREVIEW-IDENTITIESONLY-DIFF',
  'fixer/fixer-list': 'SIG-FIX-LIST-SSH-GIT-SPLIT',
  'fixer/nothing-to-fix': 'SIG-FIX-NOTHING-TO-FIX-EMPTY',
  'fixer/result-applied': 'SIG-FIX-RESULT-APPLIED',
  'git-screen/backup-notice': 'SIG-GIT-BACKUP-NOTICE',
  'git-screen/confirm-write': 'SIG-GIT-CONFIRM-WRITE',
  'git-screen/git-form-empty': 'SIG-GIT-FORM-EMPTY',
  'git-screen/git-form-filled': 'SIG-GIT-FORM-FILLED',
  'git-screen/match-strategy-select': 'SIG-MATCH-STRATEGY-SELECT-DEFAULT-GITDIR',
  'git-screen/result-success': 'SIG-GIT-RESULT-SUCCESS',
  'git-screen/review-readonly': 'SIG-REVIEW-READONLY-ALLOWED-SIGNERS',
  'global-git/backup-notice': 'SIG-GGIT-BACKUP-NOTICE',
  'global-git/confirm-write': 'SIG-GGIT-CONFIRM-WRITE',
  'global-git/fix-preview': 'SIG-GGIT-FIX-PREVIEW-BASELINE-APPLY',
  'global-git/option-detail': 'SIG-GGIT-OPTION-DETAIL-DEFAULTBRANCH',
  'global-git/options-list': 'SIG-GGIT-OPTIONS-LIST-11-BASELINE',
  'global-git/result-applied': 'SIG-GGIT-RESULT-APPLIED',
  'global-ssh/backup-notice': 'SIG-GSSH-BACKUP-NOTICE',
  'global-ssh/confirm-write': 'SIG-GSSH-CONFIRM-WRITE',
  'global-ssh/fix-preview': 'SIG-GSSH-FIX-PREVIEW-PARTIAL-APPLY',
  'global-ssh/option-detail': 'SIG-GSSH-OPTION-DETAIL-IDENTITIESONLY',
  'global-ssh/options-list': 'SIG-GSSH-OPTIONS-LIST-6-DANGEROUS',
  'global-ssh/result-applied': 'SIG-GSSH-RESULT-APPLIED',
  'health/finding-detail': 'SIG-HLTH-FINDING-DETAIL-IDENTITIESONLY-CONTRADICTION',
  'health/health-all-green': 'SIG-HLTH-ALL-GREEN',
  'health/health-with-findings': 'SIG-HLTH-WITH-FINDINGS-SSH-GIT-SPLIT',
  'health/parse-error': 'SIG-HLTH-PARSE-ERROR-GITCONFIG-FRAGMENT',
  'health/per-identity-health': 'SIG-HLTH-PER-IDENTITY-LEGACY-FRAGMENT-MISSING',
  'identity-manager/action-menu': 'SIG-IM-ACTION-MENU',
  'identity-manager/backup-notice': 'SIG-IM-BACKUP-NOTICE',
  'identity-manager/clone-name-prompt': 'SIG-IM-CLONE-NAME-PROMPT-DISTINCT-NAME',
  'identity-manager/confirm-destructive': 'SIG-IM-CONFIRM-DESTRUCTIVE-STRONGEST-CONFIRM',
  'identity-manager/delete-choice': 'SIG-IM-DELETE-CHOICE-SAFE-DEFAULT',
  'identity-manager/detail-ssh-first': 'SIG-IM-DETAIL-SSH-FIRST-NO-GIT-ATTRS',
  'identity-manager/list-empty': 'SIG-IM-LIST-EMPTY-FIRST-RUN',
  'identity-manager/list-populated': 'SIG-IM-LIST-POPULATED-8-LABEL',
};

export default screenSignatures;
