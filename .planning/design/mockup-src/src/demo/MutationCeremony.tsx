/**
 * Compressed write ceremony (02-REDESIGN-SPEC.md §6) — two visible states,
 * reused by every mutating flow (create, edit, delete, global apply, fixes):
 *
 *   A. Preview + confirm — the exact diff/managed-block, the target files,
 *      and the timestamped backup shown as a PROMISE inline; destructive
 *      rewrites additionally require a typed confirm word, and the
 *      affirmative action is never default-focused.
 *   B. Result — a success receipt: message + `Wrote →` + `Backed up →`.
 *
 * Backup stays visible on BOTH sides (promised in A, receipted in B) while
 * cutting the old 4-card sequence to the k9s/lazygit "confirm → receipt"
 * cadence.
 */

import { useCallback, useState, type ReactNode } from 'react';
import { Alert, Box, Button, Paper, Stack, TextField, Typography } from '@mui/material';
import { useLocalKeys } from './DemoContext';

export interface MutationCeremonyProps {
  /** Short name of the operation, e.g. `Write managed block to ~/.ssh/config`. */
  heading: string;
  /** The files this write will touch. */
  targets: string[];
  /** Exact preview of the change (managed block text, diff lines…). */
  preview: ReactNode;
  /** Require typing this word/name to enable the confirm button. */
  destructive?: { confirmWord: string; warning: string };
  /** Timestamped backup paths written BEFORE the change. */
  backups: string[];
  resultMessage: string;
  confirmLabel?: string;
  onCancel: () => void;
  /** Called when the user acknowledges the result receipt. */
  onDone: () => void;
}

export function MutationCeremony({
  heading,
  targets,
  preview,
  destructive,
  backups,
  resultMessage,
  confirmLabel = 'Confirm write',
  onCancel,
  onDone,
}: MutationCeremonyProps) {
  const [done, setDone] = useState(false);
  const [typed, setTyped] = useState('');

  const confirmEnabled = !destructive || typed === destructive.confirmWord;

  const advance = useCallback(() => {
    if (!done && confirmEnabled) setDone(true);
    else if (done) onDone();
  }, [done, confirmEnabled, onDone]);

  useLocalKeys(
    useCallback(
      (key) => {
        if (key === 'Escape' && !done) {
          onCancel();
          return true;
        }
        if (key === 'Enter' || (key === 'y' && !done)) {
          advance();
          return true;
        }
        return false;
      },
      [done, onCancel, advance],
    ),
  );

  if (done) {
    return (
      <Paper variant="outlined" sx={{ p: 2 }}>
        <Alert severity="success" variant="outlined" sx={{ borderRadius: 0, mb: 1.5 }}>
          {resultMessage}
        </Alert>
        <Stack spacing={0.25} sx={{ mb: 2 }}>
          {targets.map((t) => (
            <Typography key={t} sx={{ color: 'text.secondary' }}>
              Wrote → <Box component="span" sx={{ color: 'text.primary' }}>{t}</Box>
            </Typography>
          ))}
          {backups.map((b) => (
            <Typography key={b} sx={{ color: 'text.secondary' }}>
              Backed up → <Box component="span" sx={{ color: 'text.primary' }}>{b}</Box>
            </Typography>
          ))}
        </Stack>
        <Button variant="contained" onClick={advance} autoFocus>
          Done (Enter)
        </Button>
      </Paper>
    );
  }

  return (
    <Paper variant="outlined" sx={{ p: 2, borderColor: destructive ? '#e05252' : 'divider' }}>
      <Typography variant="h6" component="h2" gutterBottom>
        {heading}
      </Typography>
      <Stack spacing={1.5}>
        <Box>
          <Typography variant="subtitle2" sx={{ color: 'text.secondary' }}>
            Touches {targets.join(' · ')}
          </Typography>
          {backups.map((b) => (
            <Typography key={b} sx={{ color: 'text.secondary', fontSize: 13 }}>
              Backup → {b} <Box component="span" sx={{ color: 'text.disabled' }}>(written first — restore it to undo)</Box>
            </Typography>
          ))}
        </Box>
        <Box>
          <Typography variant="subtitle2" sx={{ color: 'text.secondary', mb: 0.5 }}>
            Exact change — everything outside the managed block is preserved verbatim
          </Typography>
          {preview}
        </Box>
        {destructive && (
          <Alert severity="error" variant="outlined" sx={{ borderRadius: 0 }}>
            {destructive.warning}
            <TextField
              fullWidth
              size="small"
              autoFocus
              value={typed}
              onChange={(e) => setTyped(e.target.value)}
              placeholder={`Type "${destructive.confirmWord}" to enable the destructive action`}
              sx={{ mt: 1 }}
            />
          </Alert>
        )}
        <Stack direction="row" spacing={2}>
          <Button variant="outlined" onClick={onCancel} autoFocus={!destructive}>
            Cancel (Esc)
          </Button>
          <Button
            variant="contained"
            color={destructive ? 'error' : 'primary'}
            disabled={!confirmEnabled}
            onClick={advance}
          >
            {confirmLabel}
          </Button>
        </Stack>
      </Stack>
    </Paper>
  );
}

/**
 * Label for a preview area — deliberately DIMMER than field labels so
 * read-only previews never read as editable inputs (round-3 feedback).
 */
export function PreviewLabel({ children }: { children: ReactNode }) {
  return (
    <Typography variant="subtitle2" sx={{ color: 'text.disabled' }}>
      {children}
    </Typography>
  );
}

/**
 * Monospace block for config/diff previews — dimmer background + text and a
 * dashed border, visually distinct from editable fields (round-3 feedback).
 */
export function PreviewBlock({ text, diff = false }: { text: string; diff?: boolean }) {
  return (
    <Box
      component="pre"
      sx={{
        m: 0,
        p: 1.5,
        border: 1,
        borderStyle: 'dashed',
        borderColor: 'divider',
        bgcolor: 'background.default',
        color: 'text.secondary',
        opacity: 0.9,
        overflowX: 'auto',
        fontSize: 13,
        lineHeight: 1.6,
      }}
    >
      {diff
        ? text.split('\n').map((line, i) => (
            <Box
              // eslint-disable-next-line react/no-array-index-key
              key={i}
              component="span"
              sx={{
                display: 'block',
                color: line.startsWith('+')
                  ? '#4caf50'
                  : line.startsWith('-')
                    ? '#e05252'
                    : 'text.secondary',
              }}
            >
              {line}
            </Box>
          ))
        : text}
    </Box>
  );
}

export default MutationCeremony;
