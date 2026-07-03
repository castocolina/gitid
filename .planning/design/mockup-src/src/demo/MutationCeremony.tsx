/**
 * The shared four-beat write ceremony (02-UX-DIRECTION.md §5): preview →
 * confirm → backup notice → result. Every mutating demo flow (create,
 * git-screen, delete, fixer, global-ssh/git apply) walks THIS component,
 * so the ceremony is byte-consistent across surfaces — exactly like the
 * real product, where no write ever skips confirm + timestamped backup.
 *
 * Destructive variants (`destructive` prop) require typing a confirm word
 * and never default-focus the affirmative action (§5).
 */

import { useCallback, useMemo, useState, type ReactNode } from 'react';
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
  /** Timestamped backup paths "created" between confirm and result. */
  backups: string[];
  resultMessage: string;
  confirmLabel?: string;
  onCancel: () => void;
  /** Called when the user acknowledges the result beat. */
  onDone: () => void;
}

type Beat = 'confirm' | 'backup' | 'result';

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
  const [beat, setBeat] = useState<Beat>('confirm');
  const [typed, setTyped] = useState('');

  const confirmEnabled = !destructive || typed === destructive.confirmWord;

  const advance = useCallback(() => {
    if (beat === 'confirm' && confirmEnabled) setBeat('backup');
    else if (beat === 'backup') setBeat('result');
    else if (beat === 'result') onDone();
  }, [beat, confirmEnabled, onDone]);

  useLocalKeys(
    useCallback(
      (key) => {
        if (key === 'Escape' && beat === 'confirm') {
          onCancel();
          return true;
        }
        if (key === 'Enter' || (key === 'y' && beat === 'confirm')) {
          advance();
          return true;
        }
        return false;
      },
      [beat, onCancel, advance],
    ),
  );

  const beatLabel = useMemo(
    () => ({ confirm: 'Beat 1–2 · preview + confirm', backup: 'Beat 3 · backup', result: 'Beat 4 · result' }),
    [],
  );

  return (
    <Paper variant="outlined" sx={{ p: 2, borderColor: destructive ? '#e05252' : 'divider' }}>
      <Typography variant="overline" sx={{ color: 'text.secondary' }}>
        {beatLabel[beat]}
      </Typography>
      <Typography variant="h6" component="h2" gutterBottom>
        {heading}
      </Typography>

      {beat === 'confirm' && (
        <Stack spacing={2}>
          <Box>
            <Typography variant="subtitle2" sx={{ color: 'text.secondary' }}>
              Files this write touches
            </Typography>
            {targets.map((t) => (
              <Typography key={t} component="p" sx={{ fontWeight: 700 }}>
                {t}
              </Typography>
            ))}
          </Box>
          <Box>
            <Typography variant="subtitle2" sx={{ color: 'text.secondary', mb: 0.5 }}>
              Exact change (managed block only — everything outside it is preserved verbatim)
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
            <Button variant="outlined" onClick={onCancel}>
              Cancel (Esc)
            </Button>
            <Button
              variant="contained"
              color={destructive ? 'error' : 'primary'}
              disabled={!confirmEnabled}
              onClick={advance}
            >
              {confirmLabel} (y)
            </Button>
          </Stack>
        </Stack>
      )}

      {beat === 'backup' && (
        <Stack spacing={2}>
          <Alert severity="info" variant="outlined" sx={{ borderRadius: 0 }}>
            Timestamped backups written BEFORE anything changed — restore any of them to undo:
          </Alert>
          {backups.map((b) => (
            <Typography key={b} component="p" sx={{ fontFamily: 'inherit' }}>
              {b}
            </Typography>
          ))}
          <Stack direction="row" spacing={2}>
            <Button variant="contained" onClick={advance} autoFocus>
              Continue (Enter)
            </Button>
          </Stack>
        </Stack>
      )}

      {beat === 'result' && (
        <Stack spacing={2}>
          <Alert severity="success" variant="outlined" sx={{ borderRadius: 0 }}>
            {resultMessage}
          </Alert>
          <Stack direction="row" spacing={2}>
            <Button variant="contained" onClick={advance} autoFocus>
              Done (Enter)
            </Button>
          </Stack>
        </Stack>
      )}
    </Paper>
  );
}

/** Monospace block for preview text — shared look for config/diff previews. */
export function PreviewBlock({ text, diff = false }: { text: string; diff?: boolean }) {
  return (
    <Box
      component="pre"
      sx={{
        m: 0,
        p: 1.5,
        border: 1,
        borderColor: 'divider',
        bgcolor: 'background.paper',
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
