import type { ReactNode } from 'react';
import { Box } from '@mui/material';
import Header, { type HeaderContext } from './Header';
import StatusLine, { type StatusTone } from './StatusLine';
import Keybar, { type KeybarEntry } from './Keybar';

export interface ShellProps {
  /**
   * The active `<surface>/<screen>` breadcrumb screen-ID marker rendered by
   * the Header (see `Header.tsx` for the contract this satisfies).
   */
  title: string;
  headerContext?: HeaderContext;
  statusMessage?: string;
  statusTone?: StatusTone;
  keybarEntries?: KeybarEntry[];
  children: ReactNode;
}

/**
 * The shared four-region app shell every surface renders inside
 * (02-UX-DIRECTION.md §2): Header / Body / StatusLine / Keybar.
 *
 * Every one of the seven product surfaces composes THIS component around
 * its own body content, so the whole mockup reads as one product. Later
 * surface plans add route files under `src/routes/` that render `<Shell>`
 * with their own title/body — they never edit this file.
 */
export function Shell({
  title,
  headerContext,
  statusMessage = 'Ready.',
  statusTone = 'info',
  keybarEntries,
  children,
}: ShellProps) {
  return (
    <Box
      sx={{
        display: 'flex',
        flexDirection: 'column',
        height: '100vh',
        maxWidth: 1280,
        mx: 'auto',
        bgcolor: 'background.default',
        color: 'text.primary',
      }}
    >
      <Header title={title} context={headerContext} />
      <Box component="main" sx={{ flex: 1, overflow: 'auto', px: 2, py: 2 }}>
        {children}
      </Box>
      <StatusLine message={statusMessage} tone={statusTone} />
      <Keybar entries={keybarEntries} />
    </Box>
  );
}

export default Shell;
