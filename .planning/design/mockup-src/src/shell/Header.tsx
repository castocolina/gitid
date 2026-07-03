import { Box, Chip, Stack, Typography } from '@mui/material';
import { semanticColors } from '../theme';

export interface HeaderContext {
  /** Total identity count shown in the global context chip. */
  identityCount: number;
  /** Global health rollup — mirrors 02-UX-DIRECTION.md §2 semantics. */
  health: 'healthy' | 'warning' | 'error';
}

export interface HeaderProps {
  /**
   * The active surface + active screen, in the exact machine-checkable
   * `<surface>/<screen>` form (e.g. `identity-manager/delete-choice`).
   *
   * This is a real design element — a breadcrumb / current-view label — that
   * DOUBLES as the screen-ID marker both the HTML capture and the TUI e2e
   * assert against, so tests prove WHICH screen is shown, not merely that a
   * generic text signature appears (review HIGH-3(b)).
   */
  title: string;
  context?: HeaderContext;
}

const healthGlyph: Record<HeaderContext['health'], string> = {
  healthy: '✓', // ✓
  warning: '!',
  error: '✗', // ✗
};

const healthColor: Record<HeaderContext['health'], string> = {
  healthy: semanticColors.healthy,
  warning: semanticColors.warning,
  error: semanticColors.error,
};

const healthWord: Record<HeaderContext['health'], string> = {
  healthy: 'healthy',
  warning: 'needs action',
  error: 'error',
};

/**
 * Header / context bar — layout region 1 of 4 (02-UX-DIRECTION.md §2).
 *
 * One line, left-aligned: app name, the breadcrumb screen-ID marker, and a
 * global context chip. Every colored state pairs a glyph with a word (never
 * color alone), per the accessibility non-negotiables in §2.
 */
export function Header({ title, context }: HeaderProps) {
  const resolvedContext: HeaderContext = context ?? {
    identityCount: 0,
    health: 'healthy',
  };

  return (
    <Box
      component="header"
      sx={{
        px: 2,
        py: 1,
        borderBottom: 1,
        borderColor: 'divider',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
      }}
    >
      <Stack direction="row" spacing={2} alignItems="baseline">
        <Typography component="span" sx={{ fontWeight: 700 }}>
          gitid
        </Typography>
        <Typography
          component="span"
          data-testid="breadcrumb"
          aria-label="current screen breadcrumb"
          sx={{ color: 'text.secondary' }}
        >
          {title}
        </Typography>
      </Stack>
      <Chip
        size="small"
        variant="outlined"
        label={
          <Box component="span" sx={{ display: 'flex', gap: 0.75, alignItems: 'center' }}>
            <span>{resolvedContext.identityCount} identities</span>
            <span aria-hidden="true">·</span>
            <span style={{ color: healthColor[resolvedContext.health] }}>
              {healthGlyph[resolvedContext.health]} {healthWord[resolvedContext.health]}
            </span>
          </Box>
        }
        sx={{ borderRadius: 0, fontFamily: 'inherit' }}
      />
    </Box>
  );
}

export default Header;
