/**
 * Interactive global-ssh (key 2) — GSSH-01 dangerous-by-default option
 * review. Advisory, never blocking: pick which recommendations to apply
 * (ForwardAgent is deliberately unticked by default, mirroring the
 * reference walkthrough), preview the exact Host * block, then the shared
 * confirm + backup + result ceremony. Applied options flip to ✓ live.
 */

import { useCallback, useMemo, useState } from 'react';
import {
  Alert,
  Box,
  Button,
  Checkbox,
  Chip,
  FormControlLabel,
  List,
  ListItemButton,
  ListItemText,
  Paper,
  Stack,
  Typography,
} from '@mui/material';
import {
  globalSshAdvisoryNote,
  globalSshDetailExplanation,
  globalSshOptions,
  managedBlockSentinels,
} from '../../data/recipeFixtures';
import { LiveShell, useDemo, useLocalKeys } from '../DemoContext';
import MutationCeremony, { PreviewBlock } from '../MutationCeremony';
import { newBackupPath } from '../store';

type Mode = 'list' | 'fix' | 'ceremony';

export function GlobalSsh() {
  const { state, dispatch, go, notify } = useDemo();
  const [mode, setMode] = useState<Mode>('list');
  const [detailKey, setDetailKey] = useState('IdentitiesOnly');
  const [chosen, setChosen] = useState<string[]>(
    globalSshOptions.filter((o) => o.needsAction && o.key !== 'ForwardAgent').map((o) => o.key),
  );

  const options = useMemo(
    () =>
      globalSshOptions.map((o) =>
        state.sshApplied.includes(o.key)
          ? { ...o, currentValue: o.recommendedValue, needsAction: false, oneLiner: `Applied by gitid — ${o.oneLiner}` }
          : o,
      ),
    [state.sshApplied],
  );
  const pending = options.filter((o) => o.needsAction);
  const detail = options.find((o) => o.key === detailKey) ?? options[0];

  const sshFindings = state.findings.filter((f) => f.section === 'SSH');

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

  const applyChosen = chosen.filter((k) => pending.some((o) => o.key === k));
  const previewLines = [
    ...applyChosen.map((k) => {
      const o = globalSshOptions.find((x) => x.key === k);
      return `+ ${k} ${o?.recommendedValue ?? ''}`;
    }),
    ...options.filter((o) => !o.needsAction).map((o) => `  ${o.key} ${o.recommendedValue} (already set)`),
    ...pending.filter((o) => !applyChosen.includes(o.key)).map((o) => `  ${o.key} — left unchanged (declined; advisory, not required)`),
  ].join('\n');

  const sentinels = managedBlockSentinels('global-ssh');

  return (
    <LiveShell
      title={mode === 'fix' || mode === 'ceremony' ? 'global-ssh/fix-preview' : 'global-ssh/options-list'}
      statusMessage={
        pending.length > 0
          ? `${pending.length} of ${options.length} options need action — ${globalSshAdvisoryNote}`
          : 'All recommendations applied or already set. Advisory, never a compliance gate.'
      }
      statusTone={pending.length > 0 ? 'warning' : 'info'}
      keybarEntries={[
        { key: 'Enter/click', label: 'option detail' },
        ...(pending.length > 0 ? [{ key: 'f', label: 'fix recommended…', onActivate: () => setMode('fix') }] : []),
        { key: '4', label: 'health', onActivate: () => go({ surface: 'health' }) },
      ]}
    >
      {sshFindings.length > 0 && mode === 'list' && (
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
          The doctor found {sshFindings.length} SSH finding{sshFindings.length > 1 ? 's' : ''} beyond
          these global options — review them in Health.
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
                  sx={{ borderBottom: 1, borderColor: 'divider' }}
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
                        <Chip size="small" variant="outlined" label={`risk ${o.risk}`} sx={{ borderRadius: 0, fontFamily: 'inherit' }} />
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
              {detail?.key === 'IdentitiesOnly' ? globalSshDetailExplanation : detail?.oneLiner}
            </Typography>
            <Alert severity="info" variant="outlined" sx={{ mt: 2, borderRadius: 0 }}>
              {globalSshAdvisoryNote}
            </Alert>
            {pending.length > 0 && (
              <Button variant="contained" sx={{ mt: 2 }} onClick={() => setMode('fix')}>
                f · Fix recommended…
              </Button>
            )}
          </Paper>
        </Stack>
      )}

      {mode === 'fix' && (
        <Paper variant="outlined" sx={{ p: 2, maxWidth: 860 }}>
          <Typography variant="h6" gutterBottom>
            Choose what to apply — every box is optional
          </Typography>
          {pending.map((o) => (
            <FormControlLabel
              key={o.key}
              sx={{ display: 'block' }}
              control={
                <Checkbox
                  checked={chosen.includes(o.key)}
                  onChange={(e) =>
                    setChosen((c) => (e.target.checked ? [...c, o.key] : c.filter((k) => k !== o.key)))
                  }
                />
              }
              label={`${o.key} → ${o.recommendedValue} (risk ${o.risk}) — ${o.oneLiner}`}
            />
          ))}
          <Typography variant="subtitle2" sx={{ color: 'text.secondary', mt: 2 }}>
            Fix preview
          </Typography>
          <PreviewBlock diff text={previewLines} />
          <Stack direction="row" spacing={2} sx={{ mt: 2 }}>
            <Button variant="outlined" onClick={() => setMode('list')}>
              Esc · Back
            </Button>
            <Button variant="contained" disabled={applyChosen.length === 0} onClick={() => setMode('ceremony')}>
              Apply {applyChosen.length} option{applyChosen.length === 1 ? '' : 's'}…
            </Button>
          </Stack>
        </Paper>
      )}

      {mode === 'ceremony' && (
        <MutationCeremony
          heading="Write Host * managed block to ~/.ssh/config"
          targets={['~/.ssh/config']}
          preview={<PreviewBlock text={`${sentinels.begin}\nIgnoreUnknown UseKeychain\n\nHost *\n${applyChosen.map((k) => `    ${k} ${globalSshOptions.find((o) => o.key === k)?.recommendedValue ?? ''}`).join('\n')}\n    UseKeychain yes\n    AddKeysToAgent yes\n${sentinels.end}`} />}
          backups={[newBackupPath('~/.ssh/config')]}
          resultMessage={`${applyChosen.length} of ${pending.length} recommended options applied to Host * in ~/.ssh/config. ${pending.length - applyChosen.length > 0 ? 'The rest were left unchanged, as chosen — advisory, never required.' : ''}`}
          onCancel={() => setMode('fix')}
          onDone={() => {
            dispatch({ type: 'apply-ssh', keys: applyChosen, backup: newBackupPath('~/.ssh/config') });
            notify(`${applyChosen.length} global SSH option${applyChosen.length === 1 ? '' : 's'} applied.`);
            setMode('list');
          }}
        />
      )}
    </LiveShell>
  );
}

export default GlobalSsh;
