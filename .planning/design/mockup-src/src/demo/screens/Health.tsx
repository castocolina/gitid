/**
 * Interactive health / doctor (key 4) — read-only integrity (HLTH-*): run
 * a (simulated) scan, browse SSH/Git findings from live demo state, open a
 * finding's detail, and hand off to the Fixer. Health NEVER mutates —
 * every fix happens on the Fixer surface, and fixed findings disappear
 * here because both surfaces read the same state.
 */

import { useCallback, useState } from 'react';
import {
  Alert,
  Box,
  Button,
  Chip,
  CircularProgress,
  List,
  ListItemButton,
  ListItemText,
  Paper,
  Stack,
  Typography,
} from '@mui/material';
import {
  healthAllGreenSummary,
  healthInfoColor,
  healthReadOnlyNote,
  healthSeverityGlyph,
  type HealthSeverity,
} from '../../data/recipeFixtures';
import { LiveShell, useDemo, useLocalKeys } from '../DemoContext';
import type { DemoFinding } from '../store';

const severityColor: Record<HealthSeverity, string> = {
  info: healthInfoColor,
  warning: '#d4b106',
  error: '#e05252',
  critical: '#e05252',
};

const severityRank: Record<HealthSeverity, number> = { critical: 0, error: 1, warning: 2, info: 3 };

export function Health() {
  const { state, dispatch, go } = useDemo();
  const [scanning, setScanning] = useState(false);
  const [detail, setDetail] = useState<DemoFinding | null>(null);

  const runScan = useCallback(() => {
    setScanning(true);
    setDetail(null);
    window.setTimeout(() => {
      setScanning(false);
      dispatch({ type: 'mark-scanned' });
    }, 900);
  }, [dispatch]);

  useLocalKeys(
    useCallback(
      (key) => {
        if (key === 'r' && !scanning) {
          runScan();
          return true;
        }
        if (key === 'Escape' && detail) {
          setDetail(null);
          return true;
        }
        return false;
      },
      [scanning, runScan, detail],
    ),
  );

  const findings = [...state.findings].sort(
    (a, b) => severityRank[a.severity] - severityRank[b.severity],
  );
  const sections: Array<'SSH' | 'Git'> = ['SSH', 'Git'];
  const allGreen = state.scanned && findings.length === 0;

  return (
    <LiveShell
      title={
        detail
          ? 'health/finding-detail'
          : allGreen
            ? 'health/health-all-green'
            : 'health/health-with-findings'
      }
      statusMessage={
        state.scanned
          ? `${findings.length} finding${findings.length === 1 ? '' : 's'} — ${healthReadOnlyNote}`
          : 'Doctor has not run yet this session — press r to scan. Health only diagnoses; it never writes.'
      }
      statusTone={findings.some((f) => f.severity !== 'info') && state.scanned ? 'warning' : 'info'}
      keybarEntries={[
        { key: 'r', label: 'run scan', onActivate: runScan },
        { key: 'Enter/click', label: 'finding detail' },
        { key: '5', label: 'open fixer', onActivate: () => go({ surface: 'fixer' }) },
      ]}
    >
      <Stack direction="row" spacing={2} alignItems="center" sx={{ mb: 2 }}>
        <Button variant="contained" onClick={runScan} disabled={scanning}>
          r · Run doctor scan
        </Button>
        {scanning && (
          <Stack direction="row" spacing={1} alignItems="center">
            <CircularProgress size={18} />
            <Typography sx={{ color: 'text.secondary' }}>
              checking ~/.ssh/config, ~/.gitconfig, fragments, keys, allowed_signers…
            </Typography>
          </Stack>
        )}
        <Box sx={{ flex: 1 }} />
        <Typography sx={{ color: 'text.secondary' }}>{healthReadOnlyNote}</Typography>
      </Stack>

      {!state.scanned && !scanning && (
        <Paper variant="outlined" sx={{ p: 3, maxWidth: 640 }}>
          <Typography sx={{ color: 'text.secondary' }}>
            No scan yet. The doctor reads every gitid-managed file and reports SSH and Git findings
            with a four-level severity model (info ~ / warning ! / error ✗ / critical ✗).
          </Typography>
        </Paper>
      )}

      {state.scanned && !scanning && !detail && (
        <Stack direction={{ xs: 'column', md: 'row' }} spacing={3}>
          {sections.map((section) => {
            const sectionFindings = findings.filter((f) => f.section === section);
            return (
              <Paper key={section} variant="outlined" sx={{ flex: 1 }}>
                <Typography variant="subtitle2" sx={{ p: 1.5, borderBottom: 1, borderColor: 'divider', color: 'text.secondary' }}>
                  {section}
                </Typography>
                {sectionFindings.length === 0 ? (
                  <Typography sx={{ p: 2, color: '#4caf50' }}>
                    ✓ {section === 'SSH' ? healthAllGreenSummary.ssh : healthAllGreenSummary.git}
                  </Typography>
                ) : (
                  <List disablePadding>
                    {sectionFindings.map((f) => (
                      <ListItemButton key={f.id} onClick={() => setDetail(f)} sx={{ borderBottom: 1, borderColor: 'divider' }}>
                        <ListItemText
                          primary={
                            <Stack direction="row" spacing={1} alignItems="center">
                              <Box component="span" sx={{ color: severityColor[f.severity] }}>
                                {healthSeverityGlyph[f.severity]} {f.severity}
                              </Box>
                              <Box component="span" sx={{ fontWeight: 700 }}>
                                {f.title}
                              </Box>
                              {f.identity && (
                                <Chip size="small" variant="outlined" label={f.identity} sx={{ borderRadius: 0, fontFamily: 'inherit' }} />
                              )}
                            </Stack>
                          }
                          secondary={`${f.family} — click for detail`}
                        />
                      </ListItemButton>
                    ))}
                  </List>
                )}
              </Paper>
            );
          })}
        </Stack>
      )}

      {detail && (
        <Paper variant="outlined" sx={{ p: 2, maxWidth: 860 }}>
          <Stack direction="row" spacing={1} alignItems="center" sx={{ mb: 1 }}>
            <Box component="span" sx={{ color: severityColor[detail.severity], fontWeight: 700 }}>
              {healthSeverityGlyph[detail.severity]} {detail.severity}
            </Box>
            <Typography variant="h6">{detail.title}</Typography>
            <Chip size="small" variant="outlined" label={detail.family} sx={{ borderRadius: 0, fontFamily: 'inherit' }} />
          </Stack>
          <Typography sx={{ whiteSpace: 'pre-wrap' }}>{detail.explanation}</Typography>
          {detail.suggestedFix ? (
            <Alert
              severity="info"
              variant="outlined"
              sx={{ mt: 2, borderRadius: 0 }}
              action={
                <Button size="small" onClick={() => go({ surface: 'fixer' })}>
                  5 · Open Fixer
                </Button>
              }
            >
              Suggested fix: {detail.suggestedFix}
            </Alert>
          ) : (
            <Alert severity="info" variant="outlined" sx={{ mt: 2, borderRadius: 0 }}>
              Informational only — nothing to fix.
            </Alert>
          )}
          <Button variant="outlined" sx={{ mt: 2 }} onClick={() => setDetail(null)}>
            Esc · Back to findings
          </Button>
        </Paper>
      )}
    </LiveShell>
  );
}

export default Health;
