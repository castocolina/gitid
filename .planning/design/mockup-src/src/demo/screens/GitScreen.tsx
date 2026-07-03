/**
 * Interactive git-screen — configure/edit the Git side of an EXISTING
 * identity (GITUI-01..05): author form → match strategy (live includeIf
 * preview) → read-only review → confirm + backup + result. Completing it
 * really flips the identity's state (incomplete → complete) in the list.
 */

import { useCallback, useState } from 'react';
import {
  Button,
  FormControlLabel,
  Paper,
  Radio,
  RadioGroup,
  Stack,
  TextField,
  Typography,
} from '@mui/material';
import {
  defaultMatchStrategy,
  gitScreenMatchStrategyPreview,
  managedBlockSentinels,
  type MatchStrategy,
} from '../../data/recipeFixtures';
import { LiveShell, useDemo, useLocalKeys } from '../DemoContext';
import MutationCeremony, { PreviewBlock } from '../MutationCeremony';
import { newBackupPath } from '../store';

const STEP_TITLES = [
  'git-screen/git-form-filled',
  'git-screen/match-strategy-select',
  'git-screen/review-readonly',
  'git-screen/confirm-write',
] as const;

export function GitScreen({ name }: { name: string }) {
  const { state, dispatch, back, notify } = useDemo();
  const identity = state.identities.find((row) => row.name === name);

  const [step, setStep] = useState(0);
  const [gitName, setGitName] = useState(identity?.gitName ?? `${name} identity`);
  const [gitEmail, setGitEmail] = useState(identity?.gitEmail ?? `you@${name}.example`);
  const [strategy, setStrategy] = useState<MatchStrategy>(
    identity?.matchStrategy ?? defaultMatchStrategy,
  );

  const valid = gitName.trim() !== '' && gitEmail.includes('@');

  const next = useCallback(() => {
    if (valid) setStep((s) => Math.min(s + 1, STEP_TITLES.length - 1));
  }, [valid]);
  const prev = useCallback(() => {
    if (step === 0) back();
    else setStep((s) => s - 1);
  }, [step, back]);

  useLocalKeys(
    useCallback(
      (key) => {
        if (step === STEP_TITLES.length - 1) return false; // ceremony owns keys
        if (key === 'Escape') {
          prev();
          return true;
        }
        if (key === 'Enter') {
          next();
          return true;
        }
        return false;
      },
      [step, prev, next],
    ),
  );

  if (!identity) {
    return (
      <LiveShell title="git-screen/git-form-empty" statusMessage={`No identity named "${name}".`} statusTone="error">
        <Button variant="outlined" onClick={back}>
          Back (Esc)
        </Button>
      </LiveShell>
    );
  }

  const keyPath = identity.keyPath ?? `~/.ssh/id_ed25519_${name}`;
  const sentinels = managedBlockSentinels(name);
  const fragment = `[user]\n    name = ${gitName}\n    email = ${gitEmail}\n    signingkey = ${keyPath}.pub\n\n[gpg]\n    format = ssh\n\n[commit]\n    gpgsign = true`;
  const fragmentBlock = `${sentinels.begin}\n${fragment}\n${sentinels.end}`;
  const includeIf = gitScreenMatchStrategyPreview[strategy].replace(/personal/g, name);
  const allowedSigners = `${gitEmail} ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIDesignMockupFixtureKeyNotReal0`;

  return (
    <LiveShell
      title={STEP_TITLES[step] ?? STEP_TITLES[0]}
      statusMessage={`Git identity for "${name}" — ${identity.gitFragmentPath ? 'editing the existing fragment' : 'not configured yet; this flow completes the identity'}.`}
      keybarEntries={[
        { key: 'Enter', label: 'next' },
        { key: 'Esc', label: 'step back / cancel' },
      ]}
    >
      {step === 0 && (
        <Paper variant="outlined" sx={{ p: 2, maxWidth: 640 }}>
          <Typography variant="h6" gutterBottom>
            Git author for “{name}”
          </Typography>
          <Stack spacing={2}>
            <TextField
              label="user.name"
              value={gitName}
              onChange={(e) => setGitName(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter') next();
              }}
              autoFocus
            />
            <TextField
              label="user.email"
              value={gitEmail}
              onChange={(e) => setGitEmail(e.target.value)}
              error={!gitEmail.includes('@')}
              helperText="Kept byte-identical to ~/.ssh/allowed_signers (GITUI-04)."
            />
            <Typography sx={{ color: 'text.secondary' }}>
              signingkey = {keyPath}.pub — a PATH to the public key, never the key material itself.
            </Typography>
            <Stack direction="row" spacing={2}>
              <Button variant="outlined" onClick={prev}>
                Cancel (Esc)
              </Button>
              <Button variant="contained" disabled={!valid} onClick={next}>
                Next (Enter)
              </Button>
            </Stack>
          </Stack>
        </Paper>
      )}

      {step === 1 && (
        <Paper variant="outlined" sx={{ p: 2, maxWidth: 760 }}>
          <Typography variant="h6" gutterBottom>
            Match strategy
          </Typography>
          <RadioGroup value={strategy} onChange={(e) => setStrategy(e.target.value as MatchStrategy)}>
            <FormControlLabel value="gitdir" control={<Radio />} label={`gitdir (default) — applies inside ~/${name}/`} />
            <FormControlLabel value="hasconfig" control={<Radio />} label="hasconfig — applies to repos whose remote uses this identity's SSH alias" />
            <FormControlLabel value="both" control={<Radio />} label="both — either condition activates it" />
          </RadioGroup>
          <Typography variant="subtitle2" sx={{ mt: 2, color: 'text.secondary' }}>
            Live includeIf preview
          </Typography>
          <PreviewBlock text={includeIf} />
          <Stack direction="row" spacing={2} sx={{ mt: 2 }}>
            <Button variant="outlined" onClick={prev}>
              Back (Esc)
            </Button>
            <Button variant="contained" onClick={next}>
              Next (Enter)
            </Button>
          </Stack>
        </Paper>
      )}

      {step === 2 && (
        <Paper variant="outlined" sx={{ p: 2, maxWidth: 860 }}>
          <Typography variant="h6" gutterBottom>
            Review — read-only
          </Typography>
          <Typography variant="subtitle2" sx={{ color: 'text.secondary' }}>
            ~/.gitconfig.d/{name}
          </Typography>
          <PreviewBlock text={fragmentBlock} />
          <Typography variant="subtitle2" sx={{ color: 'text.secondary', mt: 2 }}>
            ~/.gitconfig (includeIf)
          </Typography>
          <PreviewBlock text={includeIf} />
          <Typography variant="subtitle2" sx={{ color: 'text.secondary', mt: 2 }}>
            ~/.ssh/allowed_signers
          </Typography>
          <PreviewBlock text={allowedSigners} />
          <Stack direction="row" spacing={2} sx={{ mt: 2 }}>
            <Button variant="outlined" onClick={prev}>
              Back (Esc)
            </Button>
            <Button variant="contained" onClick={next}>
              Write it (Enter)
            </Button>
          </Stack>
        </Paper>
      )}

      {step === 3 && (
        <MutationCeremony
          heading={`Write Git identity for "${name}"`}
          targets={[`~/.gitconfig.d/${name}`, '~/.gitconfig', '~/.ssh/allowed_signers']}
          preview={<PreviewBlock text={`${fragmentBlock}\n\n${includeIf}`} />}
          backups={[newBackupPath('~/.gitconfig'), newBackupPath('~/.ssh/allowed_signers')]}
          resultMessage={`Git identity "${name}" configured — ~/.gitconfig.d/${name} now applies via the ${strategy} match strategy.`}
          onCancel={prev}
          onDone={() => {
            dispatch({
              type: 'configure-git',
              name,
              gitName,
              gitEmail,
              matchStrategy: strategy,
              backup: newBackupPath('~/.gitconfig'),
            });
            notify(`Git identity "${name}" configured — state is now ${identity.sshHost ? 'complete' : 'git-only'}.`);
            back();
          }}
        />
      )}
    </LiveShell>
  );
}

export default GitScreen;
