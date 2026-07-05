/**
 * App frame (02-REDESIGN-SPEC.md §1) — the common chrome every view renders
 * inside, k9s/lazygit/Textual style:
 *
 *   header:  brand · numbered nav tabs (1..4, active = reverse video) ·
 *            clickable health chip (`N ids · ! w · ✗ e` → jumps to Doctor)
 *   subline: thin dim breadcrumb ("Identities › New identity › Test")
 *   body:    the view's own master-detail content
 *   status:  transient feedback line
 *   footer:  CONTEXTUAL actions only + the reserved keys
 *            (Enter · Esc · ? · Ctrl+P · q) — never navigation, never vim.
 */

import type { ReactNode } from 'react';
import { Box, Chip, Stack, Typography } from '@mui/material';
import { roles, semanticColors } from '../theme';
import StatusLine, { type StatusTone } from '../shell/StatusLine';
import { TAB_LABEL, TAB_ORDER, useDemo } from './DemoContext';
import { findingCounts } from './store';

export interface FrameAction {
  key: string;
  label: string;
  onActivate?: () => void;
}

export interface FrameProps {
  /** Breadcrumb segments below the tabs, e.g. ['New identity', 'Test connection']. */
  crumbs?: string[];
  statusMessage?: string;
  statusTone?: StatusTone;
  /** Contextual footer actions for the CURRENT pane state. */
  actions?: FrameAction[];
  /**
   * True while a modal/edit/ceremony pane owns the keys — dims the header
   * nav tabs through the `disabled-nav` role (the active tab stays fully
   * lit) and carries the `active-area` accent on the breadcrumb divider
   * directly above the active pane (02-STYLE-SPEC.md §1 "ActiveArea
   * mechanism" / "dim-states"). Mirrors the TUI's RenderFrame capturesKeys.
   */
  capturesKeys?: boolean;
  children: ReactNode;
}

const RESERVED: FrameAction[] = [
  { key: 'Enter', label: 'activate' },
  { key: 'Esc', label: 'back' },
  { key: '?', label: 'help' },
  { key: 'Ctrl+P', label: 'palette' },
  { key: 'q', label: 'quit' },
];

export function Frame({
  crumbs = [],
  statusMessage = 'Ready.',
  statusTone = 'info',
  actions = [],
  capturesKeys = false,
  children,
}: FrameProps) {
  const { state, tab, setTab, openHelp, openPalette } = useDemo();
  const counts = findingCounts(state);

  const reserved: FrameAction[] = RESERVED.map((entry) =>
    entry.key === '?'
      ? { ...entry, onActivate: openHelp }
      : entry.key === 'Ctrl+P'
        ? { ...entry, onActivate: openPalette }
        : entry,
  );

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
      {/* header: brand · nav tabs · health chip */}
      <Box
        component="header"
        sx={{
          px: 2,
          py: 1,
          borderBottom: 1,
          borderColor: 'divider',
          display: 'flex',
          alignItems: 'center',
          gap: 3,
        }}
      >
        <Typography component="span" sx={{ fontWeight: 700 }}>
          gitid
        </Typography>
        <Stack direction="row" spacing={1} sx={{ flex: 1 }} component="nav" aria-label="primary views">
          {TAB_ORDER.map((id, i) => {
            const active = id === tab;
            return (
              <Box
                key={id}
                component="button"
                onClick={() => setTab(id)}
                aria-current={active ? 'page' : undefined}
                sx={{
                  font: 'inherit',
                  border: 1,
                  borderColor: active ? semanticColors.focus : 'divider',
                  cursor: 'pointer',
                  px: 1.5,
                  py: 0.25,
                  bgcolor: active ? semanticColors.focus : 'transparent',
                  color: active ? 'background.default' : 'text.secondary',
                  fontWeight: active ? 700 : 400,
                  // disabled-nav role (02-STYLE-SPEC.md dim-states): dim
                  // every INACTIVE tab while a pane captures keys; the
                  // active tab stays fully lit throughout.
                  opacity: !active && capturesKeys ? roles.disabledNav.opacity : 1,
                }}
              >
                {i + 1} {TAB_LABEL[id]}
              </Box>
            );
          })}
        </Stack>
        <Chip
          size="small"
          variant="outlined"
          onClick={() => setTab('doctor')}
          data-testid="health-chip"
          label={
            <Box component="span" sx={{ display: 'flex', gap: 0.75, alignItems: 'center' }}>
              <span>{state.identities.length} ids</span>
              <span aria-hidden="true">·</span>
              {counts.warnings + counts.errors === 0 ? (
                <span style={{ color: semanticColors.healthy }}>✓ ok</span>
              ) : (
                <>
                  <span style={{ color: semanticColors.warning }}>! {counts.warnings}</span>
                  <span style={{ color: semanticColors.error }}>✗ {counts.errors}</span>
                </>
              )}
            </Box>
          }
          sx={{ borderRadius: 0, fontFamily: 'inherit', cursor: 'pointer' }}
        />
      </Box>

      {/* thin breadcrumb sub-line — the ActiveArea mechanism
          (02-STYLE-SPEC.md §1): while a pane captures keys this divider
          carries the accent color instead of the default divider, at zero
          extra row/pixel cost (mirrors the TUI crumb-line treatment). */}
      <Typography
        component="p"
        data-testid="crumbs"
        sx={{
          px: 2,
          py: 0.25,
          fontSize: 12,
          color: 'text.disabled',
          borderBottom: 1,
          borderColor: capturesKeys ? semanticColors.accent : 'divider',
        }}
      >
        {[TAB_LABEL[tab], ...crumbs].join(' › ')}
      </Typography>

      <Box component="main" sx={{ flex: 1, px: 2, py: 1.5 }}>
        {children}
      </Box>

      <StatusLine message={statusMessage} tone={statusTone} />

      {/* footer: contextual actions + reserved keys */}
      <Box component="footer" sx={{ px: 2, py: 0.75, borderTop: 1, borderColor: 'divider', bgcolor: 'background.paper' }}>
        <Stack direction="row" spacing={2} flexWrap="wrap">
          {[...actions, ...reserved].map((entry) => (
            <Typography
              key={`${entry.key}-${entry.label}`}
              component="span"
              onClick={entry.onActivate}
              sx={{ color: 'text.secondary', cursor: entry.onActivate ? 'pointer' : 'default', whiteSpace: 'nowrap' }}
            >
              <Box component="span" sx={{ color: 'text.primary', fontWeight: 700 }}>
                {entry.key}
              </Box>{' '}
              {entry.label}
            </Typography>
          ))}
        </Stack>
      </Box>
    </Box>
  );
}

export default Frame;
