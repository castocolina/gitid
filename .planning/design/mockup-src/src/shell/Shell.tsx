import type { ReactNode } from 'react';
import { Box, Typography } from '@mui/material';
import Header, { type HeaderContext } from './Header';
import StatusLine, { type StatusTone } from './StatusLine';
import Keybar, { type KeybarEntry } from './Keybar';
import { screenSignatures } from '../data/screenSignatures';

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
 *
 * `minHeight: '100vh'` (not a fixed `height: '100vh'`) + `main`'s natural
 * flow (no `overflow: 'auto'`) is a deliberate review-C1 fix: a FIXED
 * height plus an inner `overflow: auto` scroll region clips any body taller
 * than the viewport INSIDE that inner scroll container — a full-page
 * screenshot (`page.Screenshot(true, ...)`, go-rod's `fullPage` capture)
 * only captures the OUTER document's scroll height, which a fixed-height
 * shell with its own internal scroll never exceeds, so rows past the fold
 * (global-ssh/global-git options-list, identity-manager list-populated)
 * were invisible in every captured reference PNG even though they exist in
 * the DOM. Letting the shell grow to its natural content height (instead of
 * scrolling internally) means the SAME fixed capture viewport
 * (mockupViewportWidth/mockupViewportHeight,
 * internal/screenshot/design_adapter.go — unchanged by this fix) still
 * frames every short screen identically, while a full-page capture of a
 * taller screen now captures the page's real, natural height instead of
 * clipping it.
 *
 * The trailing `[SIG-...]` line (review B1 fix) mirrors every TUI screen's
 * own `imBody`/`cfBody`/etc. trailing `"[" + sig + "]"` marker — looked up
 * from `screenSignatures` by `title` (the SAME "<surface>/<screen>" ScreenID
 * every route already passes), so `design_capture_test.go`'s HTML capture
 * path can require BOTH the breadcrumb and the manifest's own per-screen
 * Signature before ever writing a PNG, closing the SAME
 * same-shaped-but-wrong-state false-positive gap the TUI signature already
 * closed. Renders nothing when `title` has no manifest entry (e.g. the
 * `_shell/shell-demo` internal-only route) — additive, never a broken
 * lookup.
 */
export function Shell({
  title,
  headerContext,
  statusMessage = 'Ready.',
  statusTone = 'info',
  keybarEntries,
  children,
}: ShellProps) {
  const signature = screenSignatures[title];
  return (
    <Box
      sx={{
        display: 'flex',
        flexDirection: 'column',
        minHeight: '100vh',
        maxWidth: 1280,
        mx: 'auto',
        bgcolor: 'background.default',
        color: 'text.primary',
      }}
    >
      <Header title={title} context={headerContext} />
      <Box component="main" sx={{ flex: 1, px: 2, py: 2 }}>
        {children}
      </Box>
      <StatusLine message={statusMessage} tone={statusTone} />
      <Keybar entries={keybarEntries} />
      {signature && (
        <Typography
          component="p"
          data-testid="screen-signature"
          sx={{ px: 2, py: 0.5, fontSize: 11, color: 'text.disabled', fontFamily: 'inherit' }}
        >
          [{signature}]
        </Typography>
      )}
    </Box>
  );
}

export default Shell;
