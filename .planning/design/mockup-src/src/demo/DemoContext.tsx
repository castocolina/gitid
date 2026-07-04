/**
 * Demo context — 02-REDESIGN-SPEC.md frame model: FOUR top-level views in a
 * persistent header nav (`1 Identities · 2 Global SSH · 3 Global Git ·
 * 4 Doctor`); each view owns its internal pane states (wizard steps,
 * ceremonies) locally. The footer is contextual-only; there is no
 * navigation stack — tabs are direct, Esc backs out of in-pane states.
 */

import { createContext, useContext, useEffect, type Dispatch } from 'react';
import type { DemoAction, DemoState } from './store';

export type TabId = 'identities' | 'global-ssh' | 'global-git' | 'doctor';

export const TAB_ORDER: TabId[] = ['identities', 'global-ssh', 'global-git', 'doctor'];

export const TAB_LABEL: Record<TabId, string> = {
  identities: 'Identities',
  'global-ssh': 'Global SSH',
  'global-git': 'Global Git',
  doctor: 'Doctor',
};

export type LocalKeyHandler = (key: string, event: KeyboardEvent) => boolean;

export interface DemoContextValue {
  state: DemoState;
  dispatch: Dispatch<DemoAction>;
  tab: TabId;
  setTab: (tab: TabId) => void;
  /** Bottom snackbar feedback ("identity deleted", "3 options applied"…). */
  notify: (message: string) => void;
  /**
   * Screen-local key handler registration. Handlers stack: the most
   * recently registered (e.g. an open ceremony) sees keys first; returning
   * `false` lets the key fall through. Returns an unregister fn.
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

export function useLocalKeys(handler: LocalKeyHandler): void {
  const { registerLocalKeys } = useDemo();
  useEffect(() => registerLocalKeys(handler), [registerLocalKeys, handler]);
}
