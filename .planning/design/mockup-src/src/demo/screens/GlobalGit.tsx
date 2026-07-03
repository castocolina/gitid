/**
 * Interactive global-git (key 3) — GGIT-01 baseline review with the
 * main-vs-master highlight, per-option explanations, and an apply-all
 * ceremony that writes the full managed block (never a [user] section —
 * identities own their author via includeIf fragments).
 */

import { useCallback, useMemo, useState } from 'react';
import {
  Alert,
  Box,
  Button,
  Chip,
  List,
  ListItemButton,
  ListItemText,
  Paper,
  Stack,
  Typography,
} from '@mui/material';
import {
  globalGitAdvisoryNote,
  globalGitDetailExplanation,
  globalGitFixPreviewLines,
  globalGitFullManagedBlockText,
  globalGitOptions,
  globalGitResultMessage,
} from '../../data/recipeFixtures';
import { LiveShell, useDemo, useLocalKeys } from '../DemoContext';
import MutationCeremony, { PreviewBlock } from '../MutationCeremony';
import { newBackupPath } from '../store';

type Mode = 'list' | 'fix' | 'ceremony';

export function GlobalGit() {
  const { state, dispatch, go, notify } = useDemo();
  const [mode, setMode] = useState<Mode>('list');
  const [detailKey, setDetailKey] = useState('init.defaultBranch');

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
  const gitFindings = state.findings.filter((f) => f.section === 'Git');

  useLocalKeys(
    useCallback(
      (key) => {
        if (mode === 'ceremony') return false;
        if (key === 'f' && pending.length > 0) {
          setMode('fix');
          return true;
        }
        if (key === 'Escape' && mode === 'fix') {
          setMode('list');
          return true;
        }
        return false;
      },
      [mode, pending.length],
    ),
  );

  return (
    <LiveShell
      title={mode === 'list' ? 'global-git/options-list' : 'global-git/fix-preview'}
      statusMessage={
        pending.length > 0
          ? `${pending.length} baseline options not set — ${globalGitAdvisoryNote}`
          : 'Baseline applied. user.email stays untouched — identities own their author.'
      }
      statusTone={pending.length > 0 ? 'warning' : 'info'}
      keybarEntries={[
        { key: 'Enter/click', label: 'option detail' },
        ...(pending.length > 0 ? [{ key: 'f', label: 'apply baseline…', onActivate: () => setMode('fix') }] : []),
        { key: '4', label: 'health', onActivate: () => go({ surface: 'health' }) },
      ]}
    >
      {gitFindings.length > 0 && mode === 'list' && (
        <Alert
          severity="warning"
          variant="outlined"
          sx={{ mb: 2, borderRadius: 0 }}
          action={
            <Button color="warning" size="small" onClick={() => go({ surface: 'health' })}>
              4 · Open Doctor
            </Button>
          }
        >
          The doctor found {gitFindings.length} Git finding{gitFindings.length > 1 ? 's' : ''} beyond
          this baseline — review them in Health.
        </Alert>
      )}

      {mode === 'list' && (
        <Stack direction={{ xs: 'column', md: 'row' }} spacing={3}>
          <Paper variant="outlined" sx={{ flex: 1.2 }}>
            <List disablePadding>
              {options.map((o) => (
                <ListItemButton
                  key={o.key}
                  selected={o.key === detailKey}
                  onClick={() => setDetailKey(o.key)}
                  sx={{
                    borderBottom: 1,
                    borderColor: 'divider',
                    ...(o.highlight ? { bgcolor: 'rgba(212,177,6,0.08)' } : {}),
                  }}
                >
                  <ListItemText
                    primary={
                      <Stack direction="row" spacing={1} alignItems="center">
                        <Box component="span" sx={{ color: o.needsAction ? '#d4b106' : '#4caf50' }}>
                          {o.needsAction ? '!' : '✓'}
                        </Box>
                        <Box component="span" sx={{ fontWeight: 700 }}>
                          {o.key}
                        </Box>
                        {o.highlight && (
                          <Chip size="small" variant="outlined" label="main vs master" sx={{ borderRadius: 0, fontFamily: 'inherit', color: '#d4b106', borderColor: '#d4b106' }} />
                        )}
                      </Stack>
                    }
                    secondary={`now: ${o.currentValue} → recommended: ${o.recommendedValue}`}
                  />
                </ListItemButton>
              ))}
            </List>
          </Paper>
          <Paper variant="outlined" sx={{ flex: 1, p: 2 }}>
            <Typography variant="subtitle2" sx={{ color: 'text.secondary' }}>
              {detail?.key}
            </Typography>
            <Typography sx={{ whiteSpace: 'pre-wrap', mt: 1 }}>
              {detail?.key === 'init.defaultBranch' ? globalGitDetailExplanation : detail?.oneLiner}
            </Typography>
            <Alert severity="info" variant="outlined" sx={{ mt: 2, borderRadius: 0 }}>
              {globalGitAdvisoryNote}
            </Alert>
            {pending.length > 0 && (
              <Button variant="contained" sx={{ mt: 2 }} onClick={() => setMode('fix')}>
                f · Apply baseline…
              </Button>
            )}
          </Paper>
        </Stack>
      )}

      {mode === 'fix' && (
        <Paper variant="outlined" sx={{ p: 2, maxWidth: 860 }}>
          <Typography variant="h6" gutterBottom>
            Fix preview — the whole baseline, one managed block
          </Typography>
          <PreviewBlock diff text={globalGitFixPreviewLines.join('\n')} />
          <Stack direction="row" spacing={2} sx={{ mt: 2 }}>
            <Button variant="outlined" onClick={() => setMode('list')}>
              Esc · Back
            </Button>
            <Button variant="contained" onClick={() => setMode('ceremony')}>
              Apply baseline…
            </Button>
          </Stack>
        </Paper>
      )}

      {mode === 'ceremony' && (
        <MutationCeremony
          heading="Write baseline managed block to ~/.gitconfig"
          targets={['~/.gitconfig']}
          preview={<PreviewBlock text={globalGitFullManagedBlockText} />}
          backups={[newBackupPath('~/.gitconfig')]}
          resultMessage={globalGitResultMessage}
          onCancel={() => setMode('fix')}
          onDone={() => {
            dispatch({ type: 'apply-git-baseline', backup: newBackupPath('~/.gitconfig') });
            notify('Global git baseline applied — user.email untouched.');
            setMode('list');
          }}
        />
      )}
    </LiveShell>
  );
}

export default GlobalGit;
