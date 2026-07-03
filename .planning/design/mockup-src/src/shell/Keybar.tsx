import { Box, Stack, Typography } from '@mui/material';

export interface KeybarEntry {
  key: string;
  label: string;
}

export interface KeybarProps {
  /** Context-sensitive keybindings valid in the current screen, shown in
   * addition to the always-present reserved keys. */
  entries?: KeybarEntry[];
}

// Reserved keys, identical everywhere (02-UX-DIRECTION.md §2). Never
// reassigned per surface.
const reservedKeys: KeybarEntry[] = [
  { key: 'Esc', label: 'back/cancel' },
  { key: 'q', label: 'quit' },
  { key: '?', label: 'help' },
  { key: '/', label: 'filter' },
  { key: 'Enter', label: 'activate' },
];

/**
 * Keybar / footer — layout region 4 of 4 (02-UX-DIRECTION.md §2).
 *
 * Always visible (lazygit/k9s convention; Recognition-over-recall). Shows
 * only keys valid in the current context, plus the reserved keys that are
 * identical on every surface.
 */
export function Keybar({ entries = [] }: KeybarProps) {
  const allEntries = [...entries, ...reservedKeys];

  return (
    <Box
      component="footer"
      sx={{
        px: 2,
        py: 0.75,
        borderTop: 1,
        borderColor: 'divider',
        bgcolor: 'background.paper',
      }}
    >
      <Stack direction="row" spacing={2} flexWrap="wrap">
        {allEntries.map((entry) => (
          <Typography key={entry.key} component="span" sx={{ color: 'text.secondary' }}>
            <Box component="span" sx={{ color: 'text.primary', fontWeight: 700 }}>
              {entry.key}
            </Box>{' '}
            {entry.label}
          </Typography>
        ))}
      </Stack>
    </Box>
  );
}

export default Keybar;
