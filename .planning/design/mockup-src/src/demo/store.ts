/**
 * Interactive-demo state store (design-review checkpoint feedback):
 * a plain reducer over dummy data seeded from `recipeFixtures.ts` — the
 * SAME single source of truth every static mockup route and the Go TUI
 * dummy render — so the live flows cannot drift from the approved copy.
 *
 * Everything here is dummy/in-memory: "writes" only mutate this state,
 * mirroring how the real product stages changes before its confirm +
 * backup ceremony. No backend, no persistence.
 */

import {
  healthFindings,
  identityManagerRows,
  type HealthFinding,
  type IdentityManagerRow,
  type IdentityManagerState,
  type MatchStrategy,
} from '../data/recipeFixtures';

export interface DemoIdentity extends IdentityManagerRow {
  /** Git author values, present once the identity's Git side is configured. */
  gitName?: string;
  gitEmail?: string;
  matchStrategy?: MatchStrategy;
  /** Real SSH endpoint + port (SSHUI-01); optional for seeded rows. */
  hostname?: string;
  port?: number;
}

/** STORE-01 dual storage strategy for the SSH config. */
export type SshStorageLayout = 'sentinel' | 'include';

export interface DemoFinding extends HealthFinding {
  /** Which identity this finding is about (drives per-identity health). */
  identity?: string;
}

export interface DemoState {
  identities: DemoIdentity[];
  findings: DemoFinding[];
  /** Whether the doctor scan has been run at least once this session. */
  scanned: boolean;
  /** global-ssh option keys the user applied via the fix ceremony. */
  sshApplied: string[];
  /** global-git baseline applied via the fix ceremony. */
  gitBaselineApplied: boolean;
  /** STORE-01: where gitid-managed SSH config lives. */
  sshStorage: SshStorageLayout;
  /** Timestamped backup paths "created" by write ceremonies, newest first. */
  backups: string[];
}

const seedIdentities: DemoIdentity[] = identityManagerRows.map((row) => ({
  ...row,
  ...(row.gitFragmentPath
    ? { gitName: `${row.name} identity`, gitEmail: `you@${row.name}.example` }
    : {}),
}));

const findingIdentity: Record<string, string> = {
  'ssh-key-perms-archived': 'archived',
  'ssh-identitiesonly-contradiction': 'clientB',
  'git-includeif-missing-fragment': 'legacy',
  'git-opensource-no-host-block': 'opensource',
};

const seedFindings: DemoFinding[] = healthFindings.map((f) => ({
  ...f,
  ...(findingIdentity[f.id] ? { identity: findingIdentity[f.id] } : {}),
}));

export const initialDemoState: DemoState = {
  identities: seedIdentities,
  findings: seedFindings,
  scanned: false,
  sshApplied: [],
  gitBaselineApplied: false,
  sshStorage: 'sentinel',
  backups: [],
};

export type DemoAction =
  | { type: 'add-identity'; identity: DemoIdentity; backup: string }
  | {
      type: 'configure-git';
      name: string;
      gitName: string;
      gitEmail: string;
      matchStrategy: MatchStrategy;
      backup: string;
    }
  | { type: 'clone-identity'; source: string; cloneName: string }
  | { type: 'delete-identity'; name: string; scope: 'everything' | 'git-only'; backup: string }
  | { type: 'new-key'; name: string; backup: string }
  | { type: 'mark-scanned' }
  | { type: 'fix-finding'; id: string; backup: string }
  | { type: 'apply-ssh'; keys: string[]; backup: string }
  | { type: 'apply-git-baseline'; backup: string }
  | {
      type: 'edit-ssh';
      name: string;
      sshHost: string;
      hostname: string;
      port: number;
      backup: string;
    }
  | { type: 'set-ssh-storage'; layout: SshStorageLayout; backup: string }
  | { type: 'reset' };

/** State an identity lands in once BOTH its SSH and Git sides exist. */
function recomputeAfterGit(row: DemoIdentity): IdentityManagerState {
  return row.sshHost ? 'complete' : 'git-only';
}

export function demoReducer(state: DemoState, action: DemoAction): DemoState {
  switch (action.type) {
    case 'add-identity':
      return {
        ...state,
        identities: [...state.identities, action.identity],
        backups: [action.backup, ...state.backups],
      };
    case 'configure-git':
      return {
        ...state,
        identities: state.identities.map((row) =>
          row.name === action.name
            ? {
                ...row,
                gitFragmentPath: `~/.gitconfig.d/${row.name}`,
                gitName: action.gitName,
                gitEmail: action.gitEmail,
                matchStrategy: action.matchStrategy,
                state: recomputeAfterGit(row),
                note: 'SSH Host block and Git fragment both present.',
              }
            : row,
        ),
        backups: [action.backup, ...state.backups],
      };
    case 'clone-identity': {
      const source = state.identities.find((row) => row.name === action.source);
      if (!source || state.identities.some((row) => row.name === action.cloneName)) {
        return state;
      }
      const clone: DemoIdentity = {
        ...source,
        name: action.cloneName,
        ...(source.sshHost ? { sshHost: `${action.cloneName}.github.com` } : {}),
        keyPath: `~/.ssh/id_ed25519_${action.cloneName}`,
        ...(source.gitFragmentPath
          ? { gitFragmentPath: `~/.gitconfig.d/${action.cloneName}` }
          : {}),
        note: `Cloned from "${action.source}" — new key + own Host block, same Git author.`,
      };
      return { ...state, identities: [...state.identities, clone] };
    }
    case 'delete-identity':
      if (action.scope === 'everything') {
        return {
          ...state,
          identities: state.identities.filter((row) => row.name !== action.name),
          findings: state.findings.filter((f) => f.identity !== action.name),
          backups: [action.backup, ...state.backups],
        };
      }
      return {
        ...state,
        identities: state.identities.map((row) => {
          if (row.name !== action.name) return row;
          const next: DemoIdentity = { ...row, state: 'incomplete' };
          delete next.gitFragmentPath;
          delete next.gitName;
          delete next.gitEmail;
          delete next.matchStrategy;
          next.note = 'SSH Host block present; Git identity was deleted.';
          return next;
        }),
        backups: [action.backup, ...state.backups],
      };
    case 'new-key':
      return {
        ...state,
        identities: state.identities.map((row) =>
          row.name === action.name
            ? {
                ...row,
                keyPath: `~/.ssh/id_ed25519_${row.name}`,
                ...(row.state === 'key-missing'
                  ? {
                      state: (row.gitFragmentPath
                        ? 'complete'
                        : 'incomplete') as IdentityManagerState,
                      note: 'New key generated; Host block re-points at it.',
                    }
                  : {}),
              }
            : row,
        ),
        backups: [action.backup, ...state.backups],
      };
    case 'mark-scanned':
      return { ...state, scanned: true };
    case 'fix-finding': {
      const finding = state.findings.find((f) => f.id === action.id);
      if (!finding) return state;
      let identities = state.identities;
      if (action.id === 'git-includeif-missing-fragment') {
        identities = identities.map((row) =>
          row.name === 'legacy'
            ? {
                ...row,
                state: 'complete',
                keyPath: row.keyPath ?? '~/.ssh/id_ed25519_legacy',
                note: 'Fragment restored — SSH Host block and Git fragment both present.',
              }
            : row,
        );
      }
      return {
        ...state,
        identities,
        findings: state.findings.filter((f) => f.id !== action.id),
        backups: [action.backup, ...state.backups],
      };
    }
    case 'apply-ssh':
      return {
        ...state,
        sshApplied: [...new Set([...state.sshApplied, ...action.keys])],
        backups: [action.backup, ...state.backups],
      };
    case 'apply-git-baseline':
      return {
        ...state,
        gitBaselineApplied: true,
        backups: [action.backup, ...state.backups],
      };
    case 'edit-ssh':
      return {
        ...state,
        identities: state.identities.map((row) =>
          row.name === action.name
            ? { ...row, sshHost: action.sshHost, hostname: action.hostname, port: action.port }
            : row,
        ),
        backups: [action.backup, ...state.backups],
      };
    case 'set-ssh-storage':
      return {
        ...state,
        sshStorage: action.layout,
        backups: [action.backup, ...state.backups],
      };
    case 'reset':
      return initialDemoState;
    default:
      return state;
  }
}

/** Header health rollup: worst live finding severity wins. */
export function healthRollup(state: DemoState): 'healthy' | 'warning' | 'error' {
  if (state.findings.some((f) => f.severity === 'error' || f.severity === 'critical')) {
    return 'error';
  }
  if (state.findings.some((f) => f.severity === 'warning')) return 'warning';
  return 'healthy';
}

/** Per-severity counts for the header chip (`N ids · ! w · ✗ e`). */
export function findingCounts(state: DemoState): { warnings: number; errors: number } {
  return {
    warnings: state.findings.filter((f) => f.severity === 'warning').length,
    errors: state.findings.filter((f) => f.severity === 'error' || f.severity === 'critical')
      .length,
  };
}

/** Fresh timestamped backup path in the same shape as the fixtures'. */
export function newBackupPath(file: string): string {
  const stamp = new Date()
    .toISOString()
    .replace(/:/g, '-')
    .replace(/\.\d+Z$/, 'Z');
  return `${file}.backup.${stamp}`;
}

export function findingsFor(state: DemoState, identityName: string): DemoFinding[] {
  return state.findings.filter((f) => f.identity === identityName);
}
