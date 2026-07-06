/**
 * Global Git view (02-REDESIGN-SPEC.md §4) — GGIT-01 baseline master-detail
 * with per-row apply checkboxes, the main-vs-master highlight, and a
 * sentinel-preserving apply ceremony. gitid never writes a [user] section
 * here — identities own their author via includeIf fragments.
 */

import { useCallback, useMemo, useState } from 'react';
import {
  Alert,
  Box,
  Button,
  Checkbox,
  Chip,
  List,
  ListItemButton,
  Paper,
  Stack,
  TextField,
  Typography,
} from '@mui/material';
import {
  globalGitAdvisoryNote,
  globalGitDetailExplanation,
  globalGitEmailCeremonyHeading,
  globalGitEmailDiffAnnotation,
  globalGitEmailFallbackAdvisory,
  globalGitEmailFallbackHelper,
  globalGitEmailFallbackKey,
  globalGitEmailResultMessage,
  globalGitFullManagedBlockText,
  globalGitOptions,
  globalGitResultMessage,
} from '../../data/recipeFixtures';
import { roles } from '../../theme';
import Frame, { CEREMONY_FOOTER_ACTIONS, type FrameAction } from '../Frame';
import { useDemo, useLocalKeys } from '../DemoContext';
import MutationCeremony, { PreviewBlock } from '../MutationCeremony';
import { fieldSx } from './Identities';
import { newBackupPath } from '../store';

export function GlobalGit() {
  const { state, dispatch, setTab, notify } = useDemo();
  const [mode, setMode] = useState<'browse' | 'ceremony' | 'email-ceremony'>('browse');
  const [detailKey, setDetailKey] = useState('init.defaultBranch');
  // D9: the global-fallback row defaults UNCHECKED (recipes leave it
  // unset) — every OTHER needs-action row still defaults chosen.
  const [chosen, setChosen] = useState<string[]>(
    globalGitOptions.filter((o) => o.needsAction && o.key !== globalGitEmailFallbackKey).map((o) => o.key),
  );
  const [emailValue, setEmailValue] = useState('');

  const options = useMemo(
    () =>
      globalGitOptions.map((o) => {
        if (o.key === globalGitEmailFallbackKey) {
          // review-findings F4: the email-fallback row has its OWN
          // dedicated apply ceremony and must NEVER join the generic
          // baseline-applied overlay — sweeping it in here (it also
          // carries needsAction: true) falsely marked it "Applied by
          // gitid" and made it permanently untogglable the instant the
          // baseline was applied. Its currentValue instead reflects the
          // ACTUALLY-applied gitGlobalEmail, and it stays toggleable no
          // matter what the baseline state is.
          return state.gitGlobalEmail ? { ...o, currentValue: state.gitGlobalEmail } : o;
        }
        return state.gitBaselineApplied && o.needsAction
          ? { ...o, currentValue: o.recommendedValue, needsAction: false, oneLiner: `Applied by gitid — ${o.oneLiner}` }
          : o;
      }),
    [state.gitBaselineApplied, state.gitGlobalEmail],
  );
  // review-findings F10: gate the fallback apply on a plausible email —
  // reusing the wizard git-form's own contains-@ check.
  const emailValid = emailValue.includes('@');
  // D9: the global-fallback row is EXCLUDED from the generic
  // "pending"/apply-count — it is a separate, opt-in concern with its own
  // detail render and ceremony, never folded into the baseline.
  const pending = options.filter((o) => o.needsAction && o.key !== globalGitEmailFallbackKey);
  const detail = options.find((o) => o.key === detailKey) ?? options[0];
  const detailIdx = options.findIndex((o) => o.key === detail?.key);
  const applyChosen = chosen.filter((k) => pending.some((o) => o.key === k));
  const emailChosen = chosen.includes(globalGitEmailFallbackKey);
  const gitFindings = state.findings.filter((f) => f.section === 'Git');

  useLocalKeys(
    useCallback(
      (key) => {
        if (mode !== 'browse') return false;
        if (key === 'ArrowDown' || key === 'ArrowUp') {
          const next = options[key === 'ArrowDown' ? Math.min(detailIdx + 1, options.length - 1) : Math.max(detailIdx - 1, 0)];
          if (next) setDetailKey(next.key);
          return true;
        }
        if (key === ' ') {
          // review-findings F2(g): the footer now advertises `space toggle`
          // (mirroring globalgit.go's `space` binding) — wire the actual
          // toggle on the selected row so the advertised affordance is
          // real, not just copy.
          const current = options[detailIdx];
          if (current?.needsAction) {
            setChosen((c) => (c.includes(current.key) ? c.filter((k) => k !== current.key) : [...c, current.key]));
          }
          return true;
        }
        return false;
      },
      [mode, options, detailIdx],
    ),
  );

  // review-findings F9: the TUI (globalgit.go handleKey "a") prioritizes the
  // dedicated email ceremony FIRST whenever the fallback row is selected AND
  // chosen — the web used to check applyChosen.length first, so a pending
  // baseline apply elsewhere would steal "a" away from the fallback row even
  // while it was the active selection. F10: the email branch additionally
  // requires emailValid — an implausible email is never applicable.
  // F2(g): the browse footer now also advertises `space toggle` (mirroring
  // globalgit.go's `space` FooterAction).
  const actions: FrameAction[] =
    mode !== 'browse'
      ? CEREMONY_FOOTER_ACTIONS
      : [
          { key: '↑↓', label: 'select option' },
          { key: 'space', label: 'toggle' },
          ...(detail?.key === globalGitEmailFallbackKey && emailChosen && emailValid
            ? [{ key: 'a', label: 'set global fallback email', onActivate: () => setMode('email-ceremony') }]
            : applyChosen.length > 0
              ? [{ key: 'a', label: `apply ${applyChosen.length} selected`, onActivate: () => setMode('ceremony') }]
              : []),
        ];

  return (
    <Frame
      crumbs={['Options']}
      statusMessage={
        pending.length > 0
          ? `${pending.length} baseline options not set — ${globalGitAdvisoryNote}`
          : 'Baseline applied. user.email stays untouched — identities own their author.'
      }
      statusTone={pending.length > 0 ? 'warning' : 'info'}
      actions={actions}
      // review-findings F4: dim the header nav while an apply ceremony owns
      // the keys (mirrors the TUI's capturesKeys on all four tabs).
      capturesKeys={mode !== 'browse'}
    >
      {gitFindings.length > 0 && mode === 'browse' && (
        <Alert
          severity="warning"
          variant="outlined"
          sx={{ mb: 1.5, borderRadius: 0 }}
          action={
            <Button color="warning" size="small" onClick={() => setTab('doctor')}>
              4 · Open Doctor
            </Button>
          }
        >
          The doctor found {gitFindings.length} Git finding{gitFindings.length > 1 ? 's' : ''} beyond
          this baseline.
        </Alert>
      )}

      {mode === 'browse' && (
        <Stack direction="row" spacing={2}>
          <Paper variant="outlined" sx={{ width: '44%', minWidth: 360 }}>
            <List disablePadding>
              {options.map((o) => (
                <ListItemButton
                  key={o.key}
                  selected={o.key === detail?.key}
                  onClick={() => setDetailKey(o.key)}
                  sx={{
                    borderBottom: 1,
                    borderColor: 'divider',
                    py: 0.5,
                    display: 'block',
                    ...(o.highlight ? { bgcolor: 'rgba(212,177,6,0.08)' } : {}),
                  }}
                >
                  <Stack direction="row" spacing={1} alignItems="center">
                    <Checkbox
                      size="small"
                      sx={{ p: 0 }}
                      disabled={!o.needsAction}
                      checked={o.needsAction ? chosen.includes(o.key) : true}
                      onClick={(e) => e.stopPropagation()}
                      onChange={(e) =>
                        setChosen((c) => (e.target.checked ? [...c, o.key] : c.filter((k) => k !== o.key)))
                      }
                    />
                    <Box component="span" sx={{ color: o.needsAction ? roles.warning.color : roles.healthy.color }}>
                      {o.needsAction ? '!' : '✓'}
                    </Box>
                    <Box component="span" sx={{ fontWeight: 700, flex: 1 }}>
                      {o.key}
                    </Box>
                    {o.highlight && (
                      <Chip
                        size="small"
                        variant="outlined"
                        label="main vs master"
                        sx={{ borderRadius: 0, fontFamily: 'inherit', color: roles.warning.color, borderColor: roles.warning.color }}
                      />
                    )}
                  </Stack>
                  <Typography noWrap sx={{ fontSize: 12, color: 'text.secondary', pl: 4 }}>
                    now: {o.currentValue} → {o.recommendedValue}
                  </Typography>
                </ListItemButton>
              ))}
            </List>
          </Paper>
          <Paper variant="outlined" sx={{ flex: 1, p: 1.5 }}>
            {detail?.key === globalGitEmailFallbackKey ? (
              // D9 (checkpoint-2 contract): the promoted, editable
              // global-fallback row — an editable field + apply checkbox
              // (already wired via the master-list row above), its own
              // always-visible helper/advisory lines (byte-exact, verbatim).
              <>
                <TextField
                  label={globalGitEmailFallbackKey}
                  size="small"
                  fullWidth
                  // review-findings F6: this field carried NO fieldSx (D1's
                  // rest/focus tint) — every other editable field in both
                  // screens does.
                  sx={{ mb: 1, ...fieldSx }}
                  value={emailValue}
                  // review-findings F10: reuse the wizard's own inline-error
                  // idiom (GitFormFields' user.email `error={!includes('@')}`)
                  // instead of silently allowing an implausible apply.
                  error={!emailValid}
                  onChange={(e) => setEmailValue(e.target.value)}
                />
                {!emailValid && (
                  <Typography sx={{ fontSize: 12, color: roles.error.color, mt: -0.5, mb: 0.5 }}>needs @</Typography>
                )}
                <Typography sx={{ fontSize: 13, color: roles.hint.color }}>{globalGitEmailFallbackHelper}</Typography>
                <Typography sx={{ fontSize: 13, color: roles.hint.color, mt: 0.5 }}>
                  {globalGitEmailFallbackAdvisory}
                </Typography>
                {emailChosen && emailValid && (
                  <Button variant="contained" sx={{ mt: 1.5 }} onClick={() => setMode('email-ceremony')}>
                    Set global fallback email…
                  </Button>
                )}
              </>
            ) : (
              <>
                <Typography variant="subtitle2" sx={{ color: 'text.secondary' }}>
                  {detail?.key}
                </Typography>
                <Typography sx={{ whiteSpace: 'pre-wrap', mt: 0.5, fontSize: 14 }}>
                  {detail?.key === 'init.defaultBranch' ? globalGitDetailExplanation : detail?.oneLiner}
                </Typography>
                <Alert severity="info" variant="outlined" sx={{ mt: 1.5, borderRadius: 0 }}>
                  {globalGitAdvisoryNote}
                </Alert>
                {applyChosen.length > 0 && (
                  <Button variant="contained" sx={{ mt: 1.5 }} onClick={() => setMode('ceremony')}>
                    Apply {applyChosen.length} selected…
                  </Button>
                )}
              </>
            )}
          </Paper>
        </Stack>
      )}

      {mode === 'ceremony' && (
        <MutationCeremony
          heading="Write baseline managed block to ~/.gitconfig"
          targets={['~/.gitconfig']}
          preview={<PreviewBlock text={globalGitFullManagedBlockText} />}
          backups={[newBackupPath('~/.gitconfig')]}
          resultMessage={globalGitResultMessage}
          confirmLabel="Apply baseline"
          onCancel={() => setMode('browse')}
          onDone={() => {
            dispatch({ type: 'apply-git-baseline', backup: newBackupPath('~/.gitconfig') });
            notify('Global git baseline applied — user.email untouched.');
            setMode('browse');
          }}
        />
      )}

      {mode === 'email-ceremony' && (
        // D9: the global-fallback user.email's OWN dedicated ceremony —
        // separate heading/target/annotated-diff/result from the baseline
        // managed-block ceremony (gitid never folds a fallback author into
        // the baseline block).
        <MutationCeremony
          heading={globalGitEmailCeremonyHeading}
          targets={['~/.gitconfig']}
          preview={
            <PreviewBlock text={`+ [user]\n+     email = ${emailValue}  ${globalGitEmailDiffAnnotation}`} />
          }
          backups={[newBackupPath('~/.gitconfig')]}
          resultMessage={globalGitEmailResultMessage}
          confirmLabel="Apply"
          onCancel={() => setMode('browse')}
          onDone={() => {
            dispatch({
              type: 'apply-git-global-email',
              email: emailValue,
              backup: newBackupPath('~/.gitconfig'),
            });
            notify('Global fallback user.email set — identities override it via includeIf.');
            setMode('browse');
          }}
        />
      )}
    </Frame>
  );
}

export default GlobalGit;
