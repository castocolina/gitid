/**
 * Demo navigation + state context. The interactive demo is deliberately a
 * TUI-style state machine (a navigation STACK + reducer, no URL routing):
 * the HTML demo and internal/dummytui share the same mental model and the
 * same reserved key map (doc.go key-allocation table), so reviewing one
 * medium teaches the other.
 */

import { createContext, useContext, useEffect, type Dispatch, type ReactNode } from 'react';
import type { KeybarEntry } from '../shell/Keybar';
import Shell from '../shell/Shell';
import type { StatusTone } from '../shell/StatusLine';
import { healthRollup, type DemoAction, type DemoState } from './store';

export type SurfaceName =
  | 'home'
  | 'create'
  | 'git-screen'
  | 'identity'
  | 'global-ssh'
  | 'global-git'
  | 'health'
  | 'fixer';

export interface Dest {
  surface: SurfaceName;
  params?: Record<string, string>;
}

export type LocalKeyHandler = (key: string, event: KeyboardEvent) => boolean;

export interface DemoContextValue {
  state: DemoState;
  dispatch: Dispatch<DemoAction>;
  /** Push a destination onto the navigation stack. */
  go: (dest: Dest) => void;
  /** Pop the stack (Esc). No-op at the home screen. */
  back: () => void;
  /** Reset the stack to home (q). */
  home: () => void;
  /** Bottom snackbar feedback ("identity deleted", "3 options applied"…). */
  notify: (message: string) => void;
  /**
   * Screen-local key handler registration (j/k/Enter/wizard-Esc…).
   * Handlers stack: the most recently registered (e.g. an open ceremony
   * panel) sees keys first; returning `false` lets the key fall through to
   * older handlers and finally the global map. Returns an unregister fn.
   */
  registerLocalKeys: (handler: LocalKeyHandler) => () => void;
  openHelp: () => void;
  openPalette: () => void;
}

export const DemoContext = createContext<DemoContextValue | null>(null);

export function useDemo(): DemoContextValue {
  const value = useContext(DemoContext);
  if (!value) throw new Error('useDemo must be used inside <DemoApp>');
  return value;
}

/**
 * Register a screen-local key handler for the lifetime of the calling
 * screen. Handlers return `true` when they consumed the key; unconsumed
 * keys fall through to the global map (1..5, n, g, ?, q, Esc).
 */
export function useLocalKeys(handler: LocalKeyHandler): void {
  const { registerLocalKeys } = useDemo();
  useEffect(() => registerLocalKeys(handler), [registerLocalKeys, handler]);
}

export interface LiveShellProps {
  title: string;
  statusMessage?: string;
  statusTone?: StatusTone;
  keybarEntries?: KeybarEntry[];
  children: ReactNode;
}

/**
 * Shell wrapper whose header context chip is LIVE: identity count and the
 * health rollup are recomputed from demo state on every render, so every
 * action's effect (create, delete, fix) is immediately visible in the
 * persistent header — the same global-context affordance the TUI shell has.
 */
export function LiveShell({
  title,
  statusMessage,
  statusTone,
  keybarEntries,
  children,
}: LiveShellProps) {
  const { state } = useDemo();
  return (
    <Shell
      title={title}
      headerContext={{ identityCount: state.identities.length, health: healthRollup(state) }}
      {...(statusMessage !== undefined ? { statusMessage } : {})}
      {...(statusTone !== undefined ? { statusTone } : {})}
      {...(keybarEntries !== undefined ? { keybarEntries } : {})}
    >
      {children}
    </Shell>
  );
}
