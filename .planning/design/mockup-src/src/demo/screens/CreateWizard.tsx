/**
 * Interactive create-identity wizard — the full KEY-01/SSHUI-01..04/
 * TEST-01/02 workflow with live dummy state:
 *
 *   ssh-form → algorithm → key generated → two-stage connectivity test
 *   (against a throwaway temp config, with a simulate-failure toggle) →
 *   git details (or skip → incomplete identity) → match strategy →
 *   review → confirm + backup + result (shared MutationCeremony).
 *
 * Esc steps BACK one step; at the first step it asks to discard.
 */

import { useCallback, useMemo, useState } from 'react';
import {
  Alert,
  Box,
  Button,
  Checkbox,
  CircularProgress,
  FormControlLabel,
  List,
  ListItemButton,
  ListItemText,
  Paper,
  Radio,
  RadioGroup,
  Stack,
  Step,
  StepLabel,
  Stepper,
  TextField,
  Typography,
} from '@mui/material';
import {
  algorithmCatalog,
  defaultMatchStrategy,
  gitScreenMatchStrategyPreview,
  managedBlockSentinels,
  sshTestTmpConfigPath,
  type MatchStrategy,
} from '../../data/recipeFixtures';
import { LiveShell, useDemo, useLocalKeys } from '../DemoContext';
import MutationCeremony, { PreviewBlock } from '../MutationCeremony';
import { newBackupPath, type DemoIdentity } from '../store';

const STEPS = [
  'SSH details',
  'Algorithm',
  'Test connection',
  'Git identity',
  'Match strategy',
  'Review',
  'Write',
] as const;

type TestPhase = 'idle' | 'running1' | 'stage1' | 'running2' | 'stage2' | 'failed';

export function CreateWizard() {
  const { state, dispatch, back, notify } = useDemo();

  const [step, setStep] = useState(0);
  const [discardAsk, setDiscardAsk] = useState(false);

  // SSH form (SSHUI-01 field order; blank prefix → provider host itself).
  const [prefix, setPrefix] = useState('acme');
  const [hostname, setHostname] = useState('ssh.github.com');
  const [port, setPort] = useState('443');
  const [algo, setAlgo] = useState('ed25519');

  // Two-stage test simulation.
  const [testPhase, setTestPhase] = useState<TestPhase>('idle');
  const [simulateFail, setSimulateFail] = useState(false);

  // Git step.
  const [configureGit, setConfigureGit] = useState(true);
  const [gitName, setGitName] = useState('Acme Identity');
  const [gitEmail, setGitEmail] = useState('you@acme.example');
  const [strategy, setStrategy] = useState<MatchStrategy>(defaultMatchStrategy);

  const name = prefix.trim() || 'github';
  const sshHost = prefix.trim() ? `${prefix.trim()}.github.com` : 'github.com';
  const keyPath = `~/.ssh/id_ed25519_${name}`;
  const nameTaken = state.identities.some((row) => row.name === name);

  const sentinels = managedBlockSentinels(name);
  const hostBlock = `Host ${sshHost}\n    Hostname ${hostname}\n    Port ${port}\n    User git\n    IdentityFile ${keyPath}\n    IdentitiesOnly yes`;
  const managedBlock = `${sentinels.begin}\n${hostBlock}\n${sentinels.end}`;

  const stage1Cmd = `ssh -T -F ${sshTestTmpConfigPath} -p ${port} -i ${keyPath} git@${hostname}`;
  const stage1Ok = `Hi ${name}! You've successfully authenticated, but GitHub does not provide shell access.`;
  const stage1Fail = `git@${hostname}: Permission denied (publickey).`;
  const stage2Cmd = `ssh -G ${sshHost} -F ${sshTestTmpConfigPath} | grep identityfile`;
  const stage2Out = `identityfile ${keyPath}`;

  const runStage1 = useCallback(() => {
    setTestPhase('running1');
    window.setTimeout(() => setTestPhase(simulateFail ? 'failed' : 'stage1'), 900);
  }, [simulateFail]);

  const runStage2 = useCallback(() => {
    setTestPhase('running2');
    window.setTimeout(() => setTestPhase('stage2'), 700);
  }, []);

  const canAdvance = useMemo(() => {
    if (step === 0) return !nameTaken && hostname.trim() !== '' && /^\d+$/.test(port);
    if (step === 2) return testPhase === 'stage2';
    if (step === 3) return !configureGit || (gitName.trim() !== '' && gitEmail.includes('@'));
    return true;
  }, [step, nameTaken, hostname, port, testPhase, configureGit, gitName, gitEmail]);

  const next = useCallback(() => {
    if (!canAdvance) return;
    // Skipping Git details also skips the match-strategy step.
    if (step === 3 && !configureGit) setStep(5);
    else setStep((s) => Math.min(s + 1, STEPS.length - 1));
  }, [canAdvance, step, configureGit]);

  const prev = useCallback(() => {
    if (step === 0) setDiscardAsk(true);
    else if (step === 5 && !configureGit) setStep(3);
    else setStep((s) => s - 1);
  }, [step, configureGit]);

  useLocalKeys(
    useCallback(
      (key, event) => {
        if (discardAsk) {
          if (key === 'y' || key === 'Enter') {
            back();
            return true;
          }
          if (key === 'Escape' || key === 'n') {
            setDiscardAsk(false);
            return true;
          }
          return true; // modal: swallow everything else
        }
        if (step === STEPS.length - 1) return false; // ceremony owns keys
        if (key === 'Escape') {
          prev();
          return true;
        }
        if (key === 'Enter' && !(event.target instanceof HTMLTextAreaElement)) {
          if (step === 2 && testPhase === 'idle') runStage1();
          else if (step === 2 && testPhase === 'stage1') runStage2();
          else if (step === 2 && testPhase === 'failed') setTestPhase('idle');
          else next();
          return true;
        }
        return false;
      },
      [discardAsk, back, step, prev, next, testPhase, runStage1, runStage2],
    ),
  );

  const gitFragment = `[user]\n    name = ${gitName}\n    email = ${gitEmail}\n    signingkey = ${keyPath}.pub\n\n[gpg]\n    format = ssh\n\n[commit]\n    gpgsign = true`;

  const reviewText = configureGit
    ? `${managedBlock}\n\n# ~/.gitconfig.d/${name}\n${gitFragment}\n\n# ~/.gitconfig\n${gitScreenMatchStrategyPreview[strategy].replace(/personal/g, name)}`
    : managedBlock;

  const finish = useCallback(() => {
    const identity: DemoIdentity = {
      name,
      state: configureGit ? 'complete' : 'incomplete',
      sshHost,
      keyPath,
      ...(configureGit
        ? {
            gitFragmentPath: `~/.gitconfig.d/${name}`,
            gitName,
            gitEmail,
            matchStrategy: strategy,
            note: 'SSH Host block and Git fragment both present.',
          }
        : { note: 'SSH Host block present; no Git identity configured for this alias.' }),
    };
    dispatch({ type: 'add-identity', identity, backup: newBackupPath('~/.ssh/config') });
    notify(
      configureGit
        ? `Identity "${name}" created — SSH + Git configured (${strategy}).`
        : `Identity "${name}" created — SSH only (state: incomplete). Press g on it to add Git.`,
    );
    back();
  }, [name, configureGit, sshHost, keyPath, gitName, gitEmail, strategy, dispatch, notify, back]);

  return (
    <LiveShell
      title={`create-flow/${['ssh-form-filled', 'algo-catalog', testPhase === 'failed' ? 'test-fail' : testPhase === 'stage2' ? 'test-stage2-by-alias' : 'test-stage1-direct', 'git-details', 'match-strategy', 'review', 'confirm-write'][step] ?? 'ssh-form-filled'}`}
      statusMessage={
        discardAsk
          ? 'Discard this new identity? y discard · n keep editing'
          : `New identity wizard — step ${step + 1}/${STEPS.length}. Nothing touches your files until the final confirm.`
      }
      statusTone={discardAsk ? 'warning' : 'info'}
      keybarEntries={[
        { key: 'Enter', label: 'next / run' },
        { key: 'Esc', label: 'step back / cancel' },
      ]}
    >
      <Stepper activeStep={step} sx={{ mb: 3 }} alternativeLabel>
        {STEPS.map((label) => (
          <Step key={label}>
            <StepLabel>{label}</StepLabel>
          </Step>
        ))}
      </Stepper>

      {discardAsk && (
        <Alert severity="warning" variant="outlined" sx={{ mb: 2, borderRadius: 0 }}>
          Discard this new identity? Nothing has been written.{' '}
          <Button size="small" color="warning" onClick={back}>
            y · Discard
          </Button>
          <Button size="small" onClick={() => setDiscardAsk(false)}>
            n · Keep editing
          </Button>
        </Alert>
      )}

      {step === 0 && (
        <Paper variant="outlined" sx={{ p: 2, maxWidth: 640 }}>
          <Typography variant="h6" gutterBottom>
            SSH details
          </Typography>
          <Stack spacing={2}>
            <TextField
              label="Alias prefix (identity name)"
              value={prefix}
              onChange={(e) => setPrefix(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter') next();
              }}
              helperText={
                nameTaken
                  ? `An identity named "${name}" already exists — pick another prefix.`
                  : `SSH Host becomes: ${sshHost}  (blank prefix → the provider host itself)`
              }
              error={nameTaken}
              autoFocus
            />
            <TextField label="Real hostname" value={hostname} onChange={(e) => setHostname(e.target.value)} helperText="ssh.github.com = GitHub's port-443 alt-SSH endpoint (recipe default)" />
            <TextField label="Port" value={port} onChange={(e) => setPort(e.target.value)} error={!/^\d+$/.test(port)} sx={{ maxWidth: 160 }} />
            <PreviewBlock text={hostBlock} />
            <Stack direction="row" spacing={2}>
              <Button variant="outlined" onClick={prev}>
                Cancel (Esc)
              </Button>
              <Button variant="contained" disabled={!canAdvance} onClick={next}>
                Next (Enter)
              </Button>
            </Stack>
          </Stack>
        </Paper>
      )}

      {step === 1 && (
        <Paper variant="outlined" sx={{ p: 2, maxWidth: 760 }}>
          <Typography variant="h6" gutterBottom>
            Key algorithm — ed25519 recommended
          </Typography>
          <List disablePadding>
            {algorithmCatalog.map((entry) => (
              <ListItemButton
                key={entry.id}
                selected={algo === entry.id}
                onClick={() => setAlgo(entry.id)}
                disabled={entry.macosAvailability === 'requires-libfido2'}
                sx={{ borderBottom: 1, borderColor: 'divider' }}
              >
                <ListItemText
                  primary={
                    <Stack direction="row" spacing={1}>
                      <Radio size="small" checked={algo === entry.id} />
                      <Typography sx={{ fontWeight: 700, alignSelf: 'center' }}>
                        {entry.label}
                        {entry.recommended ? '  ★ recommended' : ''}
                        {entry.macosAvailability === 'requires-libfido2' ? '  (needs libfido2 + FIDO2 key — unavailable in this demo)' : ''}
                      </Typography>
                    </Stack>
                  }
                  secondary={entry.security}
                />
              </ListItemButton>
            ))}
          </List>
          <Stack direction="row" spacing={2} sx={{ mt: 2 }}>
            <Button variant="outlined" onClick={prev}>
              Back (Esc)
            </Button>
            <Button variant="contained" onClick={next}>
              Generate {algo} key (Enter)
            </Button>
          </Stack>
        </Paper>
      )}

      {step === 2 && (
        <Paper variant="outlined" sx={{ p: 2, maxWidth: 860 }}>
          <Typography variant="h6" gutterBottom>
            Two-stage connectivity test — against a throwaway temp config
          </Typography>
          <Alert severity="info" variant="outlined" sx={{ mb: 2, borderRadius: 0 }}>
            Key {keyPath} generated ({algo}). Both stages use {sshTestTmpConfigPath} — your live
            ~/.ssh/config is untouched until the final confirm.
          </Alert>
          <FormControlLabel
            control={<Checkbox checked={simulateFail} onChange={(e) => setSimulateFail(e.target.checked)} disabled={testPhase !== 'idle' && testPhase !== 'failed'} />}
            label="Simulate a failure (key not registered at the provider) to see the error path"
          />

          <Typography variant="subtitle2" sx={{ mt: 2, color: 'text.secondary' }}>
            Stage 1 — key DIRECT against the provider (TEST-01)
          </Typography>
          <PreviewBlock text={`$ ${stage1Cmd}`} />
          {testPhase === 'idle' && (
            <Button variant="contained" onClick={runStage1} sx={{ my: 1 }}>
              Run stage 1 (Enter)
            </Button>
          )}
          {(testPhase === 'running1' || testPhase === 'running2') && (
            <Stack direction="row" spacing={1} alignItems="center" sx={{ my: 1 }}>
              <CircularProgress size={18} />
              <Typography sx={{ color: 'text.secondary' }}>running ssh…</Typography>
            </Stack>
          )}
          {testPhase === 'failed' && (
            <>
              <Box sx={{ color: '#e05252', my: 1 }}>✗ {stage1Fail}</Box>
              <Alert severity="error" variant="outlined" sx={{ mb: 1, borderRadius: 0 }}>
                The provider rejected the key — usually it is not registered yet. Copy the public
                key, add it to your provider account, then retry.
              </Alert>
              <Stack direction="row" spacing={2}>
                <Button variant="outlined" onClick={() => notify('Public key copied to clipboard (demo).')}>
                  Copy public key
                </Button>
                <Button
                  variant="contained"
                  onClick={() => {
                    setSimulateFail(false);
                    setTestPhase('idle');
                  }}
                >
                  Retry (Enter)
                </Button>
              </Stack>
            </>
          )}
          {(testPhase === 'stage1' || testPhase === 'stage2') && (
            <Box sx={{ color: '#4caf50', my: 1 }}>✓ {stage1Ok}</Box>
          )}

          {(testPhase === 'stage1' || testPhase === 'stage2') && (
            <>
              <Typography variant="subtitle2" sx={{ mt: 2, color: 'text.secondary' }}>
                Stage 2 — resolve BY ALIAS, prove which key wins (TEST-02, ssh -G)
              </Typography>
              <PreviewBlock text={`$ ${stage2Cmd}`} />
              {testPhase === 'stage1' && (
                <Button variant="contained" onClick={runStage2} sx={{ my: 1 }}>
                  Run stage 2 (Enter)
                </Button>
              )}
              {testPhase === 'stage2' && <Box sx={{ color: '#4caf50', my: 1 }}>✓ {stage2Out}</Box>}
            </>
          )}

          <Stack direction="row" spacing={2} sx={{ mt: 2 }}>
            <Button variant="outlined" onClick={prev}>
              Back (Esc)
            </Button>
            <Button variant="contained" disabled={!canAdvance} onClick={next}>
              Next (Enter)
            </Button>
          </Stack>
        </Paper>
      )}

      {step === 3 && (
        <Paper variant="outlined" sx={{ p: 2, maxWidth: 640 }}>
          <Typography variant="h6" gutterBottom>
            Git identity for “{name}”
          </Typography>
          <RadioGroup value={configureGit ? 'now' : 'skip'} onChange={(e) => setConfigureGit(e.target.value === 'now')}>
            <FormControlLabel value="now" control={<Radio />} label="Configure Git now (author + ssh commit signing + includeIf)" />
            <FormControlLabel value="skip" control={<Radio />} label="Skip — SSH only for now (identity will show as incomplete; add Git later with g)" />
          </RadioGroup>
          {configureGit && (
            <Stack spacing={2} sx={{ mt: 2 }}>
              <TextField label="user.name" value={gitName} onChange={(e) => setGitName(e.target.value)} />
              <TextField
                label="user.email"
                value={gitEmail}
                onChange={(e) => setGitEmail(e.target.value)}
                helperText="Must match the allowed_signers entry byte-for-byte (GITUI-04) — gitid keeps them in sync."
                error={!gitEmail.includes('@')}
              />
              <Typography sx={{ color: 'text.secondary' }}>
                signingkey = {keyPath}.pub — a PATH to the public key, never the key material itself.
              </Typography>
            </Stack>
          )}
          <Stack direction="row" spacing={2} sx={{ mt: 2 }}>
            <Button variant="outlined" onClick={prev}>
              Back (Esc)
            </Button>
            <Button variant="contained" disabled={!canAdvance} onClick={next}>
              Next (Enter)
            </Button>
          </Stack>
        </Paper>
      )}

      {step === 4 && (
        <Paper variant="outlined" sx={{ p: 2, maxWidth: 760 }}>
          <Typography variant="h6" gutterBottom>
            Match strategy — when does this Git identity apply?
          </Typography>
          <RadioGroup value={strategy} onChange={(e) => setStrategy(e.target.value as MatchStrategy)}>
            <FormControlLabel value="gitdir" control={<Radio />} label={`gitdir (default) — applies inside ~/${name}/`} />
            <FormControlLabel value="hasconfig" control={<Radio />} label={`hasconfig — applies to any repo whose remote URL uses git@${sshHost}:`} />
            <FormControlLabel value="both" control={<Radio />} label="both — either condition activates it" />
          </RadioGroup>
          <Typography variant="subtitle2" sx={{ mt: 2, color: 'text.secondary' }}>
            Live includeIf preview
          </Typography>
          <PreviewBlock text={gitScreenMatchStrategyPreview[strategy].replace(/personal/g, name)} />
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

      {step === 5 && (
        <Paper variant="outlined" sx={{ p: 2, maxWidth: 860 }}>
          <Typography variant="h6" gutterBottom>
            Review — everything the final write will create
          </Typography>
          <Stack spacing={0.5} sx={{ mb: 2 }}>
            <Typography>Identity: {name} ({algo}) — test passed ✓</Typography>
            <Typography>SSH: {sshHost} → {hostname}:{port}, key {keyPath}</Typography>
            <Typography>
              Git: {configureGit ? `${gitName} <${gitEmail}>, strategy ${strategy}, ssh commit signing` : 'skipped — identity will be incomplete'}
            </Typography>
          </Stack>
          <PreviewBlock text={reviewText} />
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

      {step === 6 && (
        <MutationCeremony
          heading={`Write managed block for "${name}"`}
          targets={configureGit ? ['~/.ssh/config', `~/.gitconfig.d/${name}`, '~/.gitconfig', '~/.ssh/allowed_signers'] : ['~/.ssh/config']}
          preview={<PreviewBlock text={reviewText} />}
          backups={configureGit ? [newBackupPath('~/.ssh/config'), newBackupPath('~/.gitconfig')] : [newBackupPath('~/.ssh/config')]}
          resultMessage={`Identity "${name}" created — ${sshHost} now resolves to ${keyPath}.`}
          onCancel={prev}
          onDone={finish}
        />
      )}
    </LiveShell>
  );
}

export default CreateWizard;
