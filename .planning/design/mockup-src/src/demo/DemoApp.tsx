/**
 * DemoApp — interactive, keyboard-driven demo of the gitid TUI design,
 * rebuilt to 02-REDESIGN-SPEC.md: four primary views in a persistent header
 * nav (`1 Identities · 2 Global SSH · 3 Global Git · 4 Doctor` — the Fixer
 * is a consequence inside Doctor, FIX-02), contextual-only footer, live
 * master-detail everywhere, no vim keys (arrows + mouse), `?` help with the
 * full state legend, `Ctrl+P` palette. All data is dummy and in-memory.
 */

import { useCallback, useEffect, useMemo, useReducer, useRef, useState } from 'react';
import {
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  List,
  ListItemButton,
  ListItemText,
  ListSubheader,
  Snackbar,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableRow,
  TextField,
  Typography,
} from '@mui/material';
import { screenSignatures } from '../data/screenSignatures';
import {
  identityManagerStateGlyph,
  type IdentityManagerState,
} from '../data/recipeFixtures';
import { DemoContext, TAB_ORDER, type DemoContextValue, type LocalKeyHandler, type TabId } from './DemoContext';
import { demoReducer, initialDemoState } from './store';
import Identities from './screens/Identities';
import GlobalSsh from './screens/GlobalSsh';
import GlobalGit from './screens/GlobalGit';
import Doctor from './screens/Doctor';

const HELP_KEYS: Array<[string, string]> = [
  ['1 · 2 · 3 · 4', 'Switch view: Identities / Global SSH / Global Git / Doctor'],
  ['↑ ↓', 'Move the selection — the detail pane updates live'],
  ['← →', 'Switch sub-tabs (e.g. Options / Storage on Global SSH)'],
  ['Enter', 'Activate the focused control / primary action of the pane'],
  ['Esc', 'Back out one level (form → detail, modal → cancel). Never destructive'],
  ['Tab / Shift+Tab', 'Move between fields and buttons in a form'],
  ['n · e · g · c · d', 'Identities: new / edit SSH / configure Git / clone / delete'],
  ['f · F', 'Doctor: fix the selected finding / fix all (each still previews)'],
  ['Ctrl+P', 'Command palette — views, actions, and the 50 static reference mockups'],
  ['?', 'This help'],
  ['q', 'Quit (the browser demo shows a prompt instead)'],
];

/** Full MGR-02 legend (spec §2): tone = health, pips = capability. */
const LEGEND: Array<{ state: IdentityManagerState; s: string; g: string; meaning: string }> = [
  { state: 'complete', s: '✓', g: '✓', meaning: 'SSH Host block + Git fragment both present' },
  { state: 'key-used-both', s: '✓', g: '✓', meaning: 'Key wired for SSH auth AND commit signing' },
  { state: 'key-used-ssh-only', s: '✓', g: '–', meaning: 'Key wired for SSH; not for Git signing' },
  { state: 'incomplete', s: '✓', g: '–', meaning: 'SSH present; no Git identity yet' },
  { state: 'git-only', s: '–', g: '✓', meaning: 'Git identity relies on the global SSH config' },
  { state: 'key-unused', s: '–', g: '–', meaning: 'Key file exists; nothing references it' },
  { state: 'key-missing', s: '✗', g: '–', meaning: 'Host block references an absent key file' },
  { state: 'fragment-path-missing', s: '✓', g: '✗', meaning: 'includeIf points at a missing fragment' },
];

export function DemoApp() {
  const [state, dispatch] = useReducer(demoReducer, initialDemoState);
  const [tab, setTab] = useState<TabId>('identities');
  const [toast, setToast] = useState<string | null>(null);
  const [helpOpen, setHelpOpen] = useState(false);
  const [quitOpen, setQuitOpen] = useState(false);
  const [paletteOpen, setPaletteOpen] = useState(false);
  const [paletteQuery, setPaletteQuery] = useState('');

  const handlersRef = useRef<LocalKeyHandler[]>([]);

  const registerLocalKeys = useCallback((handler: LocalKeyHandler) => {
    handlersRef.current.push(handler);
    return () => {
      handlersRef.current = handlersRef.current.filter((h) => h !== handler);
    };
  }, []);

  const notify = useCallback((message: string) => setToast(message), []);
  const openHelp = useCallback(() => setHelpOpen(true), []);
  const openPalette = useCallback(() => {
    setPaletteQuery('');
    setPaletteOpen(true);
  }, []);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.ctrlKey && (e.key === 'p' || e.key === 'P')) {
        e.preventDefault();
        openPalette();
        return;
      }
      if (helpOpen || paletteOpen || quitOpen) return; // dialogs own their keys
      const target = e.target as HTMLElement | null;
      // 02-STYLE-SPEC.md §2 clause 5 — Shift+<-/-> is a FOCUS-OVERRIDE chord:
      // it reaches the screen-local wizard-step-nav handlers even when focus
      // is inside a text input, a toggle, or an expanded select. It is NEVER
      // a validity override (forward stays gated on step validity inside the
      // screen handler itself) — only the FOCUS short-circuit below is
      // bypassed. This deliberately overrides the browser's native
      // Shift+Arrow text-selection gesture inside inputs (documented
      // tradeoff, 02-STYLE-SPEC.md §2 note).
      const isShiftArrowOverride = e.shiftKey && (e.key === 'ArrowLeft' || e.key === 'ArrowRight');
      const isToggle =
        target instanceof HTMLInputElement && (target.type === 'radio' || target.type === 'checkbox');
      if (!isShiftArrowOverride) {
        if (isToggle) {
          if (e.key === ' ' || e.key.startsWith('Arrow')) return; // native toggle/group nav
        } else if (
          target instanceof HTMLInputElement ||
          target instanceof HTMLTextAreaElement ||
          (target && target.isContentEditable)
        ) {
          if (e.key === 'Escape') target.blur();
          // Enter in a single-line input = the pane's primary action (spec §7)
          // — fall through to the screen handlers UNLESS a component already
          // consumed it (e.g. Autocomplete selecting a suggestion sets
          // defaultPrevented). Every other key belongs to the field.
          if (!(e.key === 'Enter' && target instanceof HTMLInputElement && !e.defaultPrevented)) {
            return;
          }
        }
      }
      // Screen-local handlers first (newest wins).
      for (let i = handlersRef.current.length - 1; i >= 0; i -= 1) {
        const handler = handlersRef.current[i];
        if (handler && handler(e.key, e)) {
          e.preventDefault();
          return;
        }
      }
      const tabIdx = ['1', '2', '3', '4'].indexOf(e.key);
      if (tabIdx >= 0) {
        e.preventDefault();
        const next = TAB_ORDER[tabIdx];
        if (next) setTab(next);
        return;
      }
      if (e.key === '?') {
        e.preventDefault();
        setHelpOpen(true);
      } else if (e.key === 'q') {
        e.preventDefault();
        setQuitOpen(true);
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [helpOpen, paletteOpen, quitOpen, openPalette]);

  const contextValue = useMemo<DemoContextValue>(
    () => ({ state, dispatch, tab, setTab, notify, registerLocalKeys, openHelp, openPalette }),
    [state, tab, notify, registerLocalKeys, openHelp, openPalette],
  );

  const paletteEntries = useMemo(() => {
    const views = [
      { label: '1 · Identities', action: () => setTab('identities') },
      { label: '2 · Global SSH options', action: () => setTab('global-ssh') },
      { label: '3 · Global Git options', action: () => setTab('global-git') },
      { label: '4 · Doctor', action: () => setTab('doctor') },
      { label: '? · Help / key map / state legend', action: () => setHelpOpen(true) },
    ];
    const refs = Object.keys(screenSignatures).map((id) => ({
      label: `ref: ${id}`,
      action: () => {
        window.location.hash = `#/${id}`;
      },
    }));
    return { views, refs };
  }, []);

  const q = paletteQuery.toLowerCase();
  const filteredViews = paletteEntries.views.filter((e) => e.label.toLowerCase().includes(q));
  const filteredRefs = paletteEntries.refs.filter((e) => e.label.toLowerCase().includes(q));

  return (
    <DemoContext.Provider value={contextValue}>
      {tab === 'identities' && <Identities />}
      {tab === 'global-ssh' && <GlobalSsh />}
      {tab === 'global-git' && <GlobalGit />}
      {tab === 'doctor' && <Doctor />}

      <Snackbar
        open={toast !== null}
        autoHideDuration={4000}
        onClose={() => setToast(null)}
        message={toast ?? ''}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'center' }}
      />

      {/* -------- quit prompt (q) -------- */}
      <Dialog open={quitOpen} onClose={() => setQuitOpen(false)}>
        <DialogTitle>Quit gitid?</DialogTitle>
        <DialogContent>
          <Typography sx={{ color: 'text.secondary' }}>
            In the real TUI, q exits the app. In this browser demo, just close the tab — or stay and
            keep exploring (all data is dummy and in-memory).
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button autoFocus variant="contained" onClick={() => setQuitOpen(false)}>
            Stay (Esc)
          </Button>
        </DialogActions>
      </Dialog>

      {/* -------- help (?) -------- */}
      <Dialog open={helpOpen} onClose={() => setHelpOpen(false)} fullWidth maxWidth="md">
        <DialogTitle>gitid — keys & state legend</DialogTitle>
        <DialogContent>
          <Typography sx={{ color: 'text.secondary', mb: 1 }}>
            Everything is dummy, in-memory data — actions really change the demo state (lists,
            badges, header counts), but nothing on your machine is touched.
          </Typography>
          <Table size="small" sx={{ mb: 2 }}>
            <TableBody>
              {HELP_KEYS.map(([key, label]) => (
                <TableRow key={key}>
                  <TableCell sx={{ fontWeight: 700, whiteSpace: 'nowrap', borderColor: 'divider' }}>{key}</TableCell>
                  <TableCell sx={{ borderColor: 'divider' }}>{label}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
          <Typography variant="subtitle2" sx={{ color: 'text.secondary', mb: 0.5 }}>
            Identity state legend — tone glyph = health · S/G pips = capability (✓ wired · – none · ✗ broken)
          </Typography>
          <Table size="small">
            <TableHead>
              <TableRow>
                {['tone', 'state', 'S', 'G', 'meaning'].map((h) => (
                  <TableCell key={h} sx={{ color: 'text.secondary', borderColor: 'divider' }}>
                    {h}
                  </TableCell>
                ))}
              </TableRow>
            </TableHead>
            <TableBody>
              {LEGEND.map((row) => (
                <TableRow key={row.state}>
                  <TableCell sx={{ borderColor: 'divider' }}>{identityManagerStateGlyph[row.state]}</TableCell>
                  <TableCell sx={{ borderColor: 'divider', fontWeight: 700 }}>{row.state}</TableCell>
                  <TableCell sx={{ borderColor: 'divider' }}>{row.s}</TableCell>
                  <TableCell sx={{ borderColor: 'divider' }}>{row.g}</TableCell>
                  <TableCell sx={{ borderColor: 'divider' }}>{row.meaning}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </DialogContent>
      </Dialog>

      {/* -------- command palette (Ctrl+P) -------- */}
      <Dialog open={paletteOpen} onClose={() => setPaletteOpen(false)} fullWidth>
        <DialogTitle>Command palette</DialogTitle>
        <DialogContent>
          <TextField
            fullWidth
            autoFocus
            size="small"
            placeholder="Type to filter — Enter opens the first match"
            value={paletteQuery}
            onChange={(e) => setPaletteQuery(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') {
                const first = filteredViews[0] ?? filteredRefs[0];
                if (first) {
                  first.action();
                  setPaletteOpen(false);
                }
              }
            }}
            sx={{ mb: 1 }}
          />
          <List dense sx={{ maxHeight: 380, overflowY: 'auto' }}>
            {filteredViews.length > 0 && <ListSubheader disableSticky>Views & actions</ListSubheader>}
            {filteredViews.map((entry) => (
              <ListItemButton
                key={entry.label}
                onClick={() => {
                  entry.action();
                  setPaletteOpen(false);
                }}
              >
                <ListItemText primary={entry.label} />
              </ListItemButton>
            ))}
            {filteredRefs.length > 0 && (
              <ListSubheader disableSticky>
                Static reference mockups (browser Back returns to the demo)
              </ListSubheader>
            )}
            {filteredRefs.map((entry) => (
              <ListItemButton
                key={entry.label}
                onClick={() => {
                  entry.action();
                  setPaletteOpen(false);
                }}
              >
                <ListItemText primary={entry.label} />
              </ListItemButton>
            ))}
          </List>
          <Box sx={{ textAlign: 'right' }}>
            <Typography component="span" sx={{ color: 'text.disabled', fontSize: 12 }}>
              Esc closes
            </Typography>
          </Box>
        </DialogContent>
      </Dialog>
    </DemoContext.Provider>
  );
}

export default DemoApp;
