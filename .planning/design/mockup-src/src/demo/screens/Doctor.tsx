/**
 * Doctor view (02-REDESIGN-SPEC.md §5) — absorbs the Fixer (FIX-02).
 * Left: findings grouped SSH → per-identity (+ global), then Git.
 * Right: finding detail; `[Fix this]` swaps the pane to the compressed
 * ceremony; success removes the finding LIVE (header counts decrement,
 * identity states heal). `Fix all (n)` walks each fixable finding through
 * the SAME per-fix ceremony with a running counter — never a silent batch.
 * Health diagnoses; only explicit fixes write.
 */

import { useCallback, useEffect, useMemo, useState } from 'react';
import {
  Alert,
  Box,
  Button,
  Chip,
  CircularProgress,
  List,
  ListItemButton,
  ListSubheader,
  Paper,
  Stack,
  Typography,
} from '@mui/material';
import {
  fixerNothingToFixSummary,
  healthSeverityGlyph,
  type HealthSeverity,
} from '../../data/recipeFixtures';
import { roles } from '../../theme';
import Frame, { type FrameAction } from '../Frame';
import { useDemo, useLocalKeys } from '../DemoContext';
import MutationCeremony, { PreviewBlock } from '../MutationCeremony';
import { planFor } from '../fixplans';
import { newBackupPath, type DemoFinding } from '../store';

// Checkpoint feedback U2 (upgrades review-finding F11): severity colors
// route through the named theme roles — roles.info itself references the
// shared healthInfoColor constant, so the value is still defined exactly
// once (recipeFixtures.ts).
const severityColor: Record<HealthSeverity, string> = {
  info: roles.info.color,
  warning: roles.warning.color,
  error: roles.error.color,
  critical: roles.error.color,
};

const severityRank: Record<HealthSeverity, number> = { critical: 0, error: 1, warning: 2, info: 3 };

interface Group {
  label: string;
  findings: DemoFinding[];
}

export function Doctor() {
  const { state, dispatch, notify } = useDemo();
  const [scanning, setScanning] = useState(!state.scanned);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [fixing, setFixing] = useState(false);
  const [batch, setBatch] = useState<{ queue: string[]; total: number } | null>(null);

  // Auto-run the first scan on mount — the view must show value immediately.
  useEffect(() => {
    if (!state.scanned) {
      const t = window.setTimeout(() => {
        setScanning(false);
        dispatch({ type: 'mark-scanned' });
      }, 900);
      return () => window.clearTimeout(t);
    }
    setScanning(false);
    return undefined;
  }, [state.scanned, dispatch]);

  const ordered = useMemo(
    () => [...state.findings].sort((a, b) => severityRank[a.severity] - severityRank[b.severity]),
    [state.findings],
  );

  const groups = useMemo(() => {
    const bySection = (section: 'SSH' | 'Git'): Group[] => {
      const sectionFindings = ordered.filter((f) => f.section === section);
      const identities = [...new Set(sectionFindings.map((f) => f.identity ?? 'global'))];
      return identities.map((id) => ({
        label: `${section} · ${id}`,
        findings: sectionFindings.filter((f) => (f.identity ?? 'global') === id),
      }));
    };
    return [...bySection('SSH'), ...bySection('Git')];
  }, [ordered]);

  const selected = ordered.find((f) => f.id === selectedId) ?? ordered[0];
  const selectedIdxFlat = selected ? ordered.findIndex((f) => f.id === selected.id) : -1;
  const fixable = ordered.filter((f) => f.suggestedFix !== undefined);

  const startBatch = useCallback(() => {
    if (fixable.length === 0) return;
    setBatch({ queue: fixable.map((f) => f.id), total: fixable.length });
    setSelectedId(fixable[0]?.id ?? null);
    setFixing(true);
  }, [fixable]);

  useLocalKeys(
    useCallback(
      (key) => {
        if (fixing) return false; // ceremony owns keys
        if (key === 'ArrowDown' || key === 'ArrowUp') {
          const next = ordered[key === 'ArrowDown' ? Math.min(selectedIdxFlat + 1, ordered.length - 1) : Math.max(selectedIdxFlat - 1, 0)];
          if (next) setSelectedId(next.id);
          return true;
        }
        if (key === 'f' && selected?.suggestedFix) {
          setFixing(true);
          return true;
        }
        if (key === 'F' && fixable.length > 0) {
          startBatch();
          return true;
        }
        return false;
      },
      [fixing, ordered, selectedIdxFlat, selected, fixable.length, startBatch],
    ),
  );

  const finishFix = useCallback(
    (finding: DemoFinding) => {
      dispatch({ type: 'fix-finding', id: finding.id, backup: newBackupPath(planFor(finding).file) });
      notify(planFor(finding).result);
      if (batch) {
        const queue = batch.queue.filter((id) => id !== finding.id);
        if (queue.length > 0) {
          setBatch({ ...batch, queue });
          setSelectedId(queue[0] ?? null);
          return; // stay in fixing mode — next ceremony renders for the next finding
        }
        setBatch(null);
      }
      setFixing(false);
      setSelectedId(null);
    },
    [dispatch, notify, batch],
  );

  const actions: FrameAction[] = fixing
    ? [{ key: 'Esc', label: 'cancel fix' }]
    : [
        { key: '↑↓', label: 'select finding' },
        ...(selected?.suggestedFix ? [{ key: 'f', label: 'fix this', onActivate: () => setFixing(true) }] : []),
        ...(fixable.length > 1 ? [{ key: 'F', label: `fix all (${fixable.length})`, onActivate: startBatch }] : []),
      ];

  const allGreen = state.scanned && ordered.length === 0;

  return (
    <Frame
      crumbs={fixing && selected ? ['Fix', selected.title] : []}
      statusMessage={
        scanning
          ? 'Scanning ~/.ssh/config, ~/.gitconfig, fragments, keys, allowed_signers…'
          : `${ordered.length} finding${ordered.length === 1 ? '' : 's'} — Health only diagnoses; a fix runs right here, always previewed + confirmed + backed up.`
      }
      statusTone={ordered.filter((f) => f.severity !== 'info').length > 0 && !scanning ? 'warning' : 'info'}
      actions={actions}
      // review-findings F4: dim the header nav while the fix ceremony owns
      // the keys (mirrors the TUI's capturesKeys on all four tabs).
      capturesKeys={fixing}
    >
      {scanning && (
        <Stack direction="row" spacing={1} alignItems="center" sx={{ p: 3 }}>
          <CircularProgress size={18} />
          <Typography sx={{ color: 'text.secondary' }}>running doctor scan…</Typography>
        </Stack>
      )}

      {!scanning && allGreen && (
        <Paper variant="outlined" sx={{ p: 3, maxWidth: 760 }}>
          <Typography sx={{ color: roles.healthy.color, mb: 1 }}>✓ {fixerNothingToFixSummary.ssh}</Typography>
          <Typography sx={{ color: roles.healthy.color }}>✓ {fixerNothingToFixSummary.git}</Typography>
        </Paper>
      )}

      {!scanning && !allGreen && (
        <Stack direction="row" spacing={2}>
          {/* -------- grouped findings list -------- */}
          <Paper variant="outlined" sx={{ width: '44%', minWidth: 360, opacity: fixing ? 0.75 : 1 }}>
            <List disablePadding>
              {groups.map((group) => (
                <Box key={group.label}>
                  <ListSubheader
                    disableSticky
                    sx={{ bgcolor: 'background.paper', borderBottom: 1, borderColor: 'divider', lineHeight: '28px', fontFamily: 'inherit' }}
                  >
                    {group.label}
                  </ListSubheader>
                  {group.findings.map((f) => (
                    <ListItemButton
                      key={f.id}
                      selected={f.id === selected?.id}
                      onClick={() => {
                        setSelectedId(f.id);
                        setFixing(false);
                        setBatch(null);
                      }}
                      sx={{ borderBottom: 1, borderColor: 'divider', py: 0.5, display: 'block' }}
                    >
                      <Stack direction="row" spacing={1} alignItems="center">
                        <Box component="span" sx={{ color: severityColor[f.severity], whiteSpace: 'nowrap' }}>
                          {healthSeverityGlyph[f.severity]} {f.severity}
                        </Box>
                        <Typography noWrap sx={{ flex: 1, fontWeight: 700 }}>
                          {f.title}
                        </Typography>
                      </Stack>
                      <Typography noWrap sx={{ fontSize: 12, color: 'text.secondary', pl: 0.25 }}>
                        {f.family}
                        {f.suggestedFix ? ' · fixable' : ' · info only'}
                      </Typography>
                    </ListItemButton>
                  ))}
                </Box>
              ))}
            </List>
          </Paper>

          {/* -------- detail / fix pane -------- */}
          <Box sx={{ flex: 1, minWidth: 0 }}>
            {batch && fixing && (
              <Alert severity="info" variant="outlined" sx={{ mb: 1.5, borderRadius: 0 }}>
                Fix all — {batch.total - batch.queue.length} / {batch.total} fixed; each change still
                previews its own diff and backup before writing.
              </Alert>
            )}

            {selected && !fixing && (
              <Paper variant="outlined" sx={{ p: 1.5 }}>
                <Stack direction="row" spacing={1} alignItems="center" sx={{ mb: 1 }}>
                  <Box component="span" sx={{ color: severityColor[selected.severity], fontWeight: 700 }}>
                    {healthSeverityGlyph[selected.severity]} {selected.severity}
                  </Box>
                  <Typography variant="h6">{selected.title}</Typography>
                  <Chip size="small" variant="outlined" label={selected.family} sx={{ borderRadius: 0, fontFamily: 'inherit' }} />
                  {selected.identity && (
                    <Chip size="small" variant="outlined" label={selected.identity} sx={{ borderRadius: 0, fontFamily: 'inherit' }} />
                  )}
                </Stack>
                <Typography sx={{ whiteSpace: 'pre-wrap' }}>{selected.explanation}</Typography>
                {selected.suggestedFix ? (
                  <>
                    <Alert severity="info" variant="outlined" sx={{ mt: 1.5, borderRadius: 0 }}>
                      Suggested fix: {selected.suggestedFix}
                    </Alert>
                    <Button variant="contained" sx={{ mt: 1.5 }} onClick={() => setFixing(true)}>
                      f · Fix this…
                    </Button>
                  </>
                ) : (
                  <Alert severity="info" variant="outlined" sx={{ mt: 1.5, borderRadius: 0 }}>
                    Informational only — nothing to fix.
                  </Alert>
                )}
              </Paper>
            )}

            {selected && fixing && (
              <MutationCeremony
                key={selected.id}
                heading={`Fix: ${selected.title}`}
                targets={[planFor(selected).file]}
                preview={<PreviewBlock diff text={planFor(selected).diff} />}
                destructive={planFor(selected).destructive}
                backups={[newBackupPath(planFor(selected).file)]}
                resultMessage={planFor(selected).result}
                confirmLabel="Apply fix"
                onCancel={() => {
                  setFixing(false);
                  setBatch(null);
                }}
                onDone={() => finishFix(selected)}
              />
            )}
          </Box>
        </Stack>
      )}
    </Frame>
  );
}

export default Doctor;
