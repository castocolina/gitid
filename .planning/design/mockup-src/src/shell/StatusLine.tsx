import { Box, Typography } from '@mui/material';
import { semanticColors } from '../theme';

export type StatusTone = 'info' | 'healthy' | 'warning' | 'error';

export interface StatusLineProps {
  /** Transient feedback: what is happening, the file being written, the
   * backup path, or a validation error (02-UX-DIRECTION.md §2, region 3). */
  message: string;
  tone?: StatusTone;
}

const toneColor: Record<StatusTone, string> = {
  info: semanticColors.dim,
  healthy: semanticColors.healthy,
  warning: semanticColors.warning,
  error: semanticColors.error,
};

const toneGlyph: Record<StatusTone, string> = {
  info: '',
  healthy: '✓ ',
  warning: '! ',
  error: '✗ ',
};

/**
 * Status / message line — layout region 3 of 4 (02-UX-DIRECTION.md §2).
 *
 * One line for transient feedback. Every non-info tone pairs a glyph with a
 * word (never color alone), matching the color-semantics table in §2.
 */
export function StatusLine({ message, tone = 'info' }: StatusLineProps) {
  return (
    <Box
      component="div"
      sx={{
        px: 2,
        py: 0.5,
        borderTop: 1,
        borderColor: 'divider',
        minHeight: '1.5em',
      }}
    >
      <Typography component="span" sx={{ color: toneColor[tone] }}>
        {toneGlyph[tone]}
        {message}
      </Typography>
    </Box>
  );
}

export default StatusLine;
