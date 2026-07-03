/**
 * DemoApp — the interactive, keyboard-driven demo of the whole gitid TUI
 * design (design-review checkpoint feedback: "live navigation … observe
 * the data workflow and steps"). A navigation STACK over stateful screens,
 * with the TUI's exact reserved key map:
 *
 *   1 identities (home) · 2 global-ssh · 3 global-git · 4 health · 5 fixer
 *   n new identity · g configure Git · ? help · Ctrl+P palette
 *   Esc back/cancel · q "quit" (returns home in the browser)
 *
 * Everything is dummy data in memory (`store.ts`); "writes" only mutate
 * demo state after the same confirm + backup ceremony the real product
 * uses. The 50 static reference screens remain at their own routes and are
 * reachable from the palette.
 */

import { useCallback, useEffect, useMemo, useReducer, useRef, useState } from 'react';
import {
  Dialog,
  DialogContent,
  DialogTitle,
  List,
  ListItemButton,
  ListItemText,
  ListSubheader,
  Snackbar,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableRow,
  TextField,
  Typography,
} from '@mui/material';
import { screenSignatures } from '../data/screenSignatures';
import {
  DemoContext,
  type DemoContextValue,
  type Dest,
  type LocalKeyHandler,
} from './DemoContext';
import { demoReducer, initialDemoState } from './store';
import HomeScreen from './screens/HomeScreen';
import CreateWizard from './screens/CreateWizard';
import GitScreen from './screens/GitScreen';
import IdentityDetail from './screens/IdentityDetail';
import GlobalSsh from './screens/GlobalSsh';
import GlobalGit from './screens/GlobalGit';
import Health from './screens/Health';
import Fixer from './screens/Fixer';

const HELP_KEYS: Array<[string, string]> = [
  ['1', 'Identities (home)'],
  ['2', 'Global SSH options'],
  ['3', 'Global Git options'],
  ['4', 'Health / doctor (read-only)'],
  ['5', 'Fixer (write-side, confirm + backup)'],
  ['n', 'New identity wizard'],
  ['g', 'Configure Git for the selected identity'],
  ['↑↓ / j k', 'Move selection in lists'],
  ['Enter / v', 'Activate / view detail'],
  ['a · c · d · k', 'Action menu · clone · delete · new key (on an identity)'],
  ['/', 'Filter the identity list'],
  ['Ctrl+P', 'Command palette (screens, actions, static reference mockups)'],
  ['Esc', 'Back / cancel (steps back inside wizards)'],
  ['q', 'Quit (in the browser demo: return home)'],
  ['?', 'This help'],
];

function sameDest(a: Dest | undefined, b: Dest): boolean {
  return (
    !!a &&
    a.surface === b.surface &&
    JSON.stringify(a.params ?? {}) === JSON.stringify(b.params ?? {})
  );
}

export function DemoApp() {
  const [state, dispatch] = useReducer(demoReducer, initialDemoState);
  const [stack, setStack] = useState<Dest[]>([{ surface: 'home' }]);
  const [toast, setToast] = useState<string | null>(null);
  const [helpOpen, setHelpOpen] = useState(false);
  const [paletteOpen, setPaletteOpen] = useState(false);
  const [paletteQuery, setPaletteQuery] = useState('');

  const handlersRef = useRef<LocalKeyHandler[]>([]);
  const stateRef = useRef(state);
  stateRef.current = state;

  const registerLocalKeys = useCallback((handler: LocalKeyHandler) => {
    handlersRef.current.push(handler);
    return () => {
      handlersRef.current = handlersRef.current.filter((h) => h !== handler);
    };
  }, []);

  const go = useCallback((dest: Dest) => {
    setStack((s) => (sameDest(s[s.length - 1], dest) ? s : [...s, dest]));
  }, []);
  const back = useCallback(() => {
    setStack((s) => (s.length > 1 ? s.slice(0, -1) : s));
  }, []);
  const home = useCallback(() => setStack([{ surface: 'home' }]), []);
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
      if (helpOpen || paletteOpen) return; // dialogs close via their own Esc/onClose
      const target = e.target as HTMLElement | null;
      // Radios/checkboxes are <input>s too, but they are NOT text fields:
      // clicking one leaves it focused, and Enter/y must still advance the
      // flow. Only space + arrows stay native there (toggle / group nav).
      const isToggle =
        target instanceof HTMLInputElement && (target.type === 'radio' || target.type === 'checkbox');
      if (isToggle) {
        if (e.key === ' ' || e.key.startsWith('Arrow')) return;
      } else if (
        target instanceof HTMLInputElement ||
        target instanceof HTMLTextAreaElement ||
        (target && target.isContentEditable)
      ) {
        // Typing in a text field — the field owns its keys, except Esc,
        // which blurs the field so keyboard-only users can leave it.
        if (e.key === 'Escape') target.blur();
        return;
      }
      for (let i = handlersRef.current.length - 1; i >= 0; i -= 1) {
        const handler = handlersRef.current[i];
        if (handler && handler(e.key, e)) {
          e.preventDefault();
          return;
        }
      }
      const firstIdentity = stateRef.current.identities[0]?.name;
      // Every key the global map handles must preventDefault, or the
      // keystroke leaks into whatever the new screen autofocuses (e.g.
      // pressing n typed an "n" into the wizard's alias field).
      if (['1', '2', '3', '4', '5', 'n', 'g', '?', 'q', 'Escape'].includes(e.key)) {
        e.preventDefault();
      }
      switch (e.key) {
        case '1':
          home();
          break;
        case '2':
          go({ surface: 'global-ssh' });
          break;
        case '3':
          go({ surface: 'global-git' });
          break;
        case '4':
          go({ surface: 'health' });
          break;
        case '5':
          go({ surface: 'fixer' });
          break;
        case 'n':
          go({ surface: 'create' });
          break;
        case 'g':
          if (firstIdentity) go({ surface: 'git-screen', params: { name: firstIdentity } });
          break;
        case '?':
          setHelpOpen(true);
          break;
        case 'q':
          home();
          setToast('q quits the real TUI — the browser demo returns home instead.');
          break;
        case 'Escape':
          back();
          break;
        default:
          break;
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [helpOpen, paletteOpen, go, back, home, openPalette]);

  const contextValue = useMemo<DemoContextValue>(
    () => ({ state, dispatch, go, back, home, notify, registerLocalKeys, openHelp, openPalette }),
    [state, go, back, home, notify, registerLocalKeys, openHelp, openPalette],
  );

  const top = stack[stack.length - 1] ?? { surface: 'home' as const };

  const paletteEntries = useMemo(() => {
    const demoEntries = [
      { label: '1 · Identities (home)', action: () => home() },
      { label: 'n · New identity wizard', action: () => go({ surface: 'create' }) },
      { label: 'g · Configure Git (first identity)', action: () => {
          const name = stateRef.current.identities[0]?.name;
          if (name) go({ surface: 'git-screen', params: { name } });
        } },
      { label: '2 · Global SSH options', action: () => go({ surface: 'global-ssh' }) },
      { label: '3 · Global Git options', action: () => go({ surface: 'global-git' }) },
      { label: '4 · Health / doctor', action: () => go({ surface: 'health' }) },
      { label: '5 · Fixer', action: () => go({ surface: 'fixer' }) },
      { label: '? · Help / key map', action: () => setHelpOpen(true) },
    ];
    const refEntries = Object.keys(screenSignatures).map((id) => ({
      label: `ref: ${id}`,
      action: () => {
        window.location.hash = `#/${id}`;
      },
    }));
    return { demoEntries, refEntries };
  }, [go, home]);

  const q = paletteQuery.toLowerCase();
  const filteredDemo = paletteEntries.demoEntries.filter((e) => e.label.toLowerCase().includes(q));
  const filteredRef = paletteEntries.refEntries.filter((e) => e.label.toLowerCase().includes(q));

  return (
    <DemoContext.Provider value={contextValue}>
      {top.surface === 'home' && <HomeScreen />}
      {top.surface === 'create' && <CreateWizard key={stack.length} />}
      {top.surface === 'git-screen' && (
        <GitScreen key={`git-${top.params?.name}-${stack.length}`} name={top.params?.name ?? ''} />
      )}
      {top.surface === 'identity' && (
        <IdentityDetail
          key={`id-${top.params?.name}-${stack.length}`}
          name={top.params?.name ?? ''}
          {...(top.params?.action ? { initialAction: top.params.action } : {})}
        />
      )}
      {top.surface === 'global-ssh' && <GlobalSsh />}
      {top.surface === 'global-git' && <GlobalGit />}
      {top.surface === 'health' && <Health />}
      {top.surface === 'fixer' && <Fixer />}

      <Snackbar
        open={toast !== null}
        autoHideDuration={4000}
        onClose={() => setToast(null)}
        message={toast ?? ''}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'center' }}
      />

      {/* -------- help (?) -------- */}
      <Dialog open={helpOpen} onClose={() => setHelpOpen(false)} fullWidth>
        <DialogTitle>gitid — interactive demo key map</DialogTitle>
        <DialogContent>
          <Typography sx={{ color: 'text.secondary', mb: 1 }}>
            Everything here is dummy, in-memory data — actions really change the demo state (list,
            chips, header rollup), but nothing on your machine is touched. The same keys drive the
            real TUI.
          </Typography>
          <Table size="small">
            <TableBody>
              {HELP_KEYS.map(([key, label]) => (
                <TableRow key={key}>
                  <TableCell sx={{ fontWeight: 700, whiteSpace: 'nowrap', borderColor: 'divider' }}>{key}</TableCell>
                  <TableCell sx={{ borderColor: 'divider' }}>{label}</TableCell>
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
                const first = filteredDemo[0] ?? filteredRef[0];
                if (first) {
                  first.action();
                  setPaletteOpen(false);
                }
              }
            }}
            sx={{ mb: 1 }}
          />
          <List dense sx={{ maxHeight: 380, overflowY: 'auto' }}>
            {filteredDemo.length > 0 && <ListSubheader disableSticky>Demo screens & actions</ListSubheader>}
            {filteredDemo.map((entry) => (
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
            {filteredRef.length > 0 && (
              <ListSubheader disableSticky>
                Static reference mockups (the approved 50 — browser Back returns to the demo)
              </ListSubheader>
            )}
            {filteredRef.map((entry) => (
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
          <Stack direction="row" justifyContent="flex-end">
            <Typography sx={{ color: 'text.disabled', fontSize: 12 }}>Esc closes</Typography>
          </Stack>
        </DialogContent>
      </Dialog>
    </DemoContext.Provider>
  );
}

export default DemoApp;
