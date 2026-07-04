/**
 * Fix plans — the exact target file, diff, confirm semantics, and result copy
 * for every fixable doctor finding. Shared by the Doctor screen and the
 * per-identity findings panel (FIX-01/02: the fixer is a consequence of the
 * doctor, reachable wherever a finding is shown — never its own view).
 */

import { fixerFixPreviewLines } from '../data/recipeFixtures';
import type { DemoFinding } from './store';

export interface FixPlan {
  file: string;
  diff: string;
  destructive?: { confirmWord: string; warning: string };
  result: string;
}

export function planFor(finding: DemoFinding): FixPlan {
  switch (finding.id) {
    case 'ssh-key-perms-archived':
      return {
        file: '~/.ssh/id_ed25519_archived',
        diff: '- mode 0644 (world-readable)\n+ mode 0600 (owner only)',
        result: 'chmod 0600 ~/.ssh/id_ed25519_archived applied.',
      };
    case 'ssh-identitiesonly-contradiction':
      return {
        file: '~/.ssh/config',
        diff: fixerFixPreviewLines.join('\n'),
        destructive: {
          confirmWord: 'clientb.github.com',
          warning:
            'This rewrites a directive already present in your SSH config. Type the Host name "clientb.github.com" to confirm — this cannot be undone without restoring the backup.',
        },
        result: 'IdentitiesOnly set to yes on Host clientb.github.com in ~/.ssh/config.',
      };
    case 'git-includeif-missing-fragment':
      return {
        file: '~/.gitconfig.d/legacy',
        diff: '+ ~/.gitconfig.d/legacy (fragment restored from template)\n  [includeIf "gitdir:~/legacy/"] → path now resolves',
        result: '~/.gitconfig.d/legacy restored — the includeIf resolves again; "legacy" is complete.',
      };
    case 'ssh-duplicate-host-star':
      return {
        file: '~/.ssh/config',
        diff: '- Host * (line 41 — duplicate stanza removed)\n+ (its directives merged into the Host * at line 4)',
        result: 'The two Host * stanzas were merged into one.',
      };
    default:
      return {
        file: '~/.ssh/config',
        diff: `+ ${finding.suggestedFix ?? ''}`,
        result: 'Fix applied.',
      };
  }
}
