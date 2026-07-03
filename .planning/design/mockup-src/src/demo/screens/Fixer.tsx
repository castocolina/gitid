/**
 * Interactive fixer (key 5) — the write-side counterpart of Health. Lists
 * ONLY actionable findings from live state; each fix walks preview-diff →
 * confirm (destructive rewrite requires the typed host) → backup → result,
 * then REALLY updates state: the finding disappears (here and in Health),
 * and restoring legacy's fragment flips that identity back to complete.
 */

import { useCallback, useMemo, useState } from 'react';
import {
  Alert,
  Box,
  Button,
  List,
  ListItemButton,
  ListItemText,
  Paper,
  Stack,
  Typography,
} from '@mui/material';
import {
  fixerFixPreviewLines,
  fixerNothingToFixSummary,
  fixerSafetyNote,
  healthSeverityGlyph,
  type HealthSeverity,
} from '../../data/recipeFixtures';
import { LiveShell, useDemo, useLocalKeys } from '../DemoContext';
import MutationCeremony, { PreviewBlock } from '../MutationCeremony';
import { newBackupPath, type DemoFinding } from '../store';

const severityColor: Record<HealthSeverity, string> = {
  info: '#3aa6a6',
  warning: '#d4b106',
  error: '#e05252',
  critical: '#e05252',
};

interface FixPlan {
  file: string;
  diff: string;
  destructive?: { confirmWord: string; warning: string };
  result: string;
}

function planFor(finding: DemoFinding): FixPlan {
  switch (finding.id) {
    case 'ssh-key-perms-archived':
      return {
        file: '~/.ssh/id_ed25519_archived',
        diff: '- mode 0644 (world-readable)\n+ mode 0600 (owner only)',
        result: 'chmod 0600 ~/.ssh/id_ed25519_archived applied.',
      };
    case 'ssh-identitiesonly-contradiction':
      return {
        file: '~/.ssh/config',
        diff: fixerFixPreviewLines.join('\n'),
        destructive: {
          confirmWord: 'clientb.github.com',
          warning:
            'This rewrites a directive already present in your SSH config. Type the Host name "clientb.github.com" to confirm — this cannot be undone without restoring the backup.',
        },
        result: 'IdentitiesOnly set to yes on Host clientb.github.com in ~/.ssh/config.',
      };
    case 'git-includeif-missing-fragment':
      return {
        file: '~/.gitconfig.d/legacy',
        diff: '+ ~/.gitconfig.d/legacy (fragment restored from template)\n  [includeIf "gitdir:~/legacy/"] → path now resolves',
        result: '~/.gitconfig.d/legacy restored — the includeIf resolves again; "legacy" is complete.',
      };
    case 'ssh-duplicate-host-star':
      return {
        file: '~/.ssh/config',
        diff: '- Host * (line 41 — duplicate stanza removed)\n+ (its directives merged into the Host * at line 4)',
        result: 'The two Host * stanzas were merged into one.',
      };
    default:
      return {
        file: '~/.ssh/config',
        diff: `+ ${finding.suggestedFix ?? ''}`,
        result: 'Fix applied.',
      };
  }
}

export function Fixer() {
  const { state, dispatch, go, notify } = useDemo();
  const [activeId, setActiveId] = useState<string | null>(null);
  const [batchQueue, setBatchQueue] = useState<string[]>([]);

  const actionable = useMemo(
    () => state.findings.filter((f) => f.suggestedFix !== undefined),
    [state.findings],
  );
  const active = actionable.find((f) => f.id === activeId) ?? null;

  useLocalKeys(
    useCallback(
      (key) => {
        if (active) return false; // ceremony owns keys
        if (key === 'b' && actionable.length > 0) {
          setBatchQueue(actionable.map((f) => f.id));
          setActiveId(actionable[0]?.id ?? null);
          return true;
        }
        return false;
      },
      [active, actionable],
    ),
  );

  const finishOne = (finding: DemoFinding) => {
    dispatch({ type: 'fix-finding', id: finding.id, backup: newBackupPath(planFor(finding).file) });
    notify(planFor(finding).result);
    const remaining = batchQueue.filter((id) => id !== finding.id);
    setBatchQueue(remaining);
    setActiveId(remaining[0] ?? null);
  };

  return (
    <LiveShell
      title={active ? 'fixer/fix-preview' : actionable.length === 0 ? 'fixer/nothing-to-fix' : 'fixer/fixer-list'}
      statusMessage={
        actionable.length === 0
          ? 'Nothing to fix. Every fix is previewed, confirmed, and backed up — never a blind write.'
          : `${actionable.length} fixable problem${actionable.length === 1 ? '' : 's'} — ${fixerSafetyNote}`
      }
      statusTone={actionable.length === 0 ? 'info' : 'warning'}
      keybarEntries={[
        { key: 'Enter/click', label: 'fix one…' },
        ...(actionable.length > 1 && !active
          ? [{ key: 'b', label: 'fix all (each still previews)', onActivate: () => {
              setBatchQueue(actionable.map((f) => f.id));
              setActiveId(actionable[0]?.id ?? null);
            } }]
          : []),
        { key: '4', label: 'health', onActivate: () => go({ surface: 'health' }) },
      ]}
    >
      {actionable.length === 0 && (
        <Paper variant="outlined" sx={{ p: 3, maxWidth: 720 }}>
          <Typography sx={{ color: '#4caf50', mb: 1 }}>✓ {fixerNothingToFixSummary.ssh}</Typography>
          <Typography sx={{ color: '#4caf50' }}>✓ {fixerNothingToFixSummary.git}</Typography>
          <Button variant="outlined" sx={{ mt: 2 }} onClick={() => go({ surface: 'health' })}>
            4 · Back to Health
          </Button>
        </Paper>
      )}

      {!active && actionable.length > 0 && (
        <>
          <Alert severity="info" variant="outlined" sx={{ mb: 2, borderRadius: 0, maxWidth: 860 }}>
            Apply all {actionable.length} fixes — each one still previews its own diff and backup
            path before writing; nothing is applied silently.
          </Alert>
          <Paper variant="outlined" sx={{ maxWidth: 860 }}>
            <List disablePadding>
              {actionable.map((f) => (
                <ListItemButton key={f.id} onClick={() => setActiveId(f.id)} sx={{ borderBottom: 1, borderColor: 'divider' }}>
                  <ListItemText
                    primary={
                      <Stack direction="row" spacing={1}>
                        <Box component="span" sx={{ color: severityColor[f.severity] }}>
                          {healthSeverityGlyph[f.severity]} {f.severity}
                        </Box>
                        <Box component="span" sx={{ fontWeight: 700 }}>
                          {f.title}
                        </Box>
                      </Stack>
                    }
                    secondary={f.suggestedFix}
                  />
                </ListItemButton>
              ))}
            </List>
          </Paper>
        </>
      )}

      {active && (
        <>
          {batchQueue.length > 1 && (
            <Alert severity="info" variant="outlined" sx={{ mb: 2, borderRadius: 0, maxWidth: 860 }}>
              Batch fix — {batchQueue.length} remaining; each change still previews its own diff and
              backup path before writing.
            </Alert>
          )}
          <MutationCeremony
            heading={`Fix: ${active.title}`}
            targets={[planFor(active).file]}
            preview={<PreviewBlock diff text={planFor(active).diff} />}
            destructive={planFor(active).destructive}
            backups={[newBackupPath(planFor(active).file)]}
            resultMessage={planFor(active).result}
            confirmLabel="Apply fix"
            onCancel={() => {
              setActiveId(null);
              setBatchQueue([]);
            }}
            onDone={() => finishOne(active)}
          />
        </>
      )}
    </LiveShell>
  );
}

export default Fixer;
