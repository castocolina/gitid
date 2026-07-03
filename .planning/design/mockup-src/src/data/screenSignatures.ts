/**
 * screenSignatures.ts — review B1 fix (Codex cross-vendor finding HIGH):
 * `design_capture_test.go` previously gated an HTML capture on ONLY the
 * "<surface>/<screen>" breadcrumb (the route's `title`), never on the
 * manifest's own per-screen `signature` — so a wrong BODY rendered under
 * the right breadcrumb (e.g. a copy-paste bug reusing another screen's
 * content) would have been saved as a valid reference PNG. The TUI side
 * already had an equivalent, screen-specific `[SIG-...]` marker baked into
 * every `internal/dummytui/surface_*.go` render function; the HTML mockup
 * had no equivalent, so it could never be checked the same way.
 *
 * This map is the byte-identical mirror of every `.planning/design/*
 * /manifest.json`'s `signature` field, keyed by the SAME "<surface>/<screen>"
 * ScreenID every route's `Shell` `title` prop already carries — NOT
 * derived at build time (Vite cannot read outside `mockup-src/`'s root), a
 * static, diff-able contract mirroring the Go dummy's own `sig*` constants
 * precedent (`surface_identitymanager.go` etc.: "byte-identical... not
 * derived, so it stays a static, diff-able contract"). `Shell.tsx` looks up
 * `screenSignatures[title]` and renders it as a small marker, the same way
 * every TUI screen renders its own `[SIG-...]` bracket — so
 * `design_capture_test.go`'s HTML capture path can require BOTH the
 * breadcrumb and the signature before ever writing a PNG, closing the
 * SAME false-positive gap on the HTML side that the signature already
 * closed on the TUI side.
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
