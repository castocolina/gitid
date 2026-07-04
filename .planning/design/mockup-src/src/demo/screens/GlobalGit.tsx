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
  Typography,
} from '@mui/material';
import {
  globalGitAdvisoryNote,
  globalGitDetailExplanation,
  globalGitFullManagedBlockText,
  globalGitOptions,
  globalGitResultMessage,
} from '../../data/recipeFixtures';
import { semanticColors } from '../../theme';
import Frame, { type FrameAction } from '../Frame';
import { useDemo, useLocalKeys } from '../DemoContext';
import MutationCeremony, { PreviewBlock } from '../MutationCeremony';
import { newBackupPath } from '../store';

export function GlobalGit() {
  const { state, dispatch, setTab, notify } = useDemo();
  const [mode, setMode] = useState<'browse' | 'ceremony'>('browse');
  const [detailKey, setDetailKey] = useState('init.defaultBranch');
  const [chosen, setChosen] = useState<string[]>(
    globalGitOptions.filter((o) => o.needsAction).map((o) => o.key),
  );

  const options = useMemo(
    () =>
      globalGitOptions.map((o) =>
        state.gitBaselineApplied && o.needsAction
          ? { ...o, currentValue: o.recommendedValue, needsAction: false, oneLiner: `Applied by gitid — ${o.oneLiner}` }
          : o,
      ),
    [state.gitBaselineApplied],
  );
  const pending = options.filter((o) => o.needsAction);
  const detail = options.find((o) => o.key === detailKey) ?? options[0];
  const detailIdx = options.findIndex((o) => o.key === detail?.key);
  const applyChosen = chosen.filter((k) => pending.some((o) => o.key === k));
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
        return false;
      },
      [mode, options, detailIdx],
    ),
  );

  const actions: FrameAction[] =
    mode === 'ceremony'
      ? [{ key: 'Esc', label: 'cancel' }]
      : [
          { key: '↑↓', label: 'select option' },
          ...(applyChosen.length > 0
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
                    <Box component="span" sx={{ color: o.needsAction ? semanticColors.warning : semanticColors.healthy }}>
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
                        sx={{ borderRadius: 0, fontFamily: 'inherit', color: semanticColors.warning, borderColor: semanticColors.warning }}
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
    </Frame>
  );
}

export default GlobalGit;
