/**
 * Identities view (02-REDESIGN-SPEC.md §2–3) — live master-detail:
 * left sidebar (name + tone glyph + S/G capability pips + short note) with an
 * inline legend line; moving the selection (arrows or click) renders the
 * right detail pane IMMEDIATELY. The right pane also hosts every form:
 * the ≤4-state create wizard, edit-SSH, the merged Git form, clone, delete,
 * and per-finding fix ceremonies. The sidebar never disappears.
 */

import { useCallback, useState } from 'react';
import {
  Alert,
  Autocomplete,
  Box,
  Button,
  Checkbox,
  Chip,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControlLabel,
  List,
  ListItemButton,
  Paper,
  Radio,
  RadioGroup,
  Stack,
  TextField,
  Typography,
} from '@mui/material';
import {
  algorithmCatalog,
  defaultMatchStrategy,
  gitScreenMatchStrategyPreview,
  globalGitDefaults,
  healthSeverityGlyph,
  identityManagerDeleteChoices,
  identityManagerStateGlyph,
  identityManagerStateTone,
  managedBlockSentinels,
  sshTestTmpConfigPath,
  type HealthSeverity,
  type MatchStrategy,
} from '../../data/recipeFixtures';
import { roles, semanticColors } from '../../theme';
import Frame, { type FrameAction } from '../Frame';
import { useDemo, useLocalKeys } from '../DemoContext';
import MutationCeremony, { PreviewBlock } from '../MutationCeremony';
import { planFor } from '../fixplans';
import { findingsFor, newBackupPath, type DemoIdentity } from '../store';

// Checkpoint feedback U2: every semantic color routes through the named
// theme roles (matching the TUI's toneStyle over DefaultTheme).
const toneColor: Record<'success' | 'warning' | 'error', string> = {
  success: roles.healthy.color,
  warning: roles.warning.color,
  error: roles.error.color,
};

// Checkpoint feedback U2 (upgrades review-finding F11): severity colors
// route through the named theme roles — roles.info itself references the
// shared healthInfoColor constant, so the value is still defined exactly
// once (recipeFixtures.ts).
const severityColor: Record<HealthSeverity, string> = {
  info: roles.info.color,
  warning: roles.warning.color,
  error: roles.error.color,
  critical: roles.error.color,
};

type Pip = '✓' | '–' | '✗';

/** S/G capability pips (spec §2): tone carries health, pips carry capability. */
function pips(row: DemoIdentity): { s: Pip; g: Pip } {
  const s: Pip = row.state === 'key-missing' ? '✗' : row.sshHost ? '✓' : '–';
  const g: Pip = row.state === 'fragment-path-missing' ? '✗' : row.gitFragmentPath ? '✓' : '–';
  return { s, g };
}

// '#5a5a5a' is a pure LAYOUT gray (the "no capability" pip), not a semantic
// state — a documented non-role exception (theme.ts roles docstring, U2).
const pipColor: Record<Pip, string> = {
  '✓': roles.healthy.color,
  '–': '#5a5a5a',
  '✗': roles.error.color,
};

type PaneMode =
  | { kind: 'detail' }
  | { kind: 'create' }
  | { kind: 'edit-ssh' }
  | { kind: 'git' }
  | { kind: 'clone' }
  | { kind: 'delete'; scope: 'everything' | 'git-only' }
  | { kind: 'fix'; findingId: string };

const PROVIDERS = ['github.com', 'gitlab.com', 'bitbucket.org'];

function providerDefaults(provider: string): { hostname: string; port: string } {
  if (provider === 'github.com') return { hostname: 'ssh.github.com', port: '443' };
  return { hostname: provider || 'github.com', port: '22' };
}

// D1 (checkpoint-2 contract): the MUI outlined TextField never reflows on
// focus (no box/border-radius change) — the rest state uses roles.blurredField
// (dim outline, opacity 0.85) and focus tints VALUE + LABEL with
// roles.focusedField.color plus a 2px accent outline. One shared sx
// fragment, merged into every SSH/Git form field below; no new layout, no
// height change (superseding the field-contour box 02-14 shipped — that
// component never existed on the web, so this is purely a rename +
// blurred-state addition, not a behavior change).
const fieldSx = {
  '& .MuiOutlinedInput-root': {
    opacity: roles.blurredField.opacity,
  },
  '& .MuiOutlinedInput-root .MuiOutlinedInput-notchedOutline': {
    borderColor: '#2a2d33',
  },
  '& .MuiOutlinedInput-root.Mui-focused': {
    opacity: 1,
  },
  '& .MuiOutlinedInput-root.Mui-focused .MuiOutlinedInput-notchedOutline': {
    borderColor: roles.focusedField.color,
    borderWidth: 2,
  },
  '& .MuiOutlinedInput-root.Mui-focused .MuiOutlinedInput-input': {
    color: roles.focusedField.color,
  },
  '& .MuiInputLabel-root.Mui-focused': {
    color: roles.focusedField.color,
  },
} as const;

/** Read-only inherited global baseline strip (GITUI-01 kept intact). */
function BaselineStrip() {
  const { state, setTab } = useDemo();
  return (
    <Paper variant="outlined" sx={{ p: 1, mt: 1, bgcolor: 'background.paper' }}>
      <Typography sx={{ fontSize: 12, color: 'text.secondary' }}>
        Global baseline (inherited{state.gitBaselineApplied ? ', applied ✓' : ' — not applied yet'}):
        {' '}init.defaultBranch={globalGitDefaults.initDefaultBranch} · core.ignorecase=
        {String(globalGitDefaults.coreIgnorecase)} · autocrlf=input/lf · push.autoSetupRemote=
        {String(globalGitDefaults.pushAutoSetupRemote)} · pull.rebase={String(globalGitDefaults.pullRebase)} ·
        merge={globalGitDefaults.mergeConflictstyle}{' '}
        <Box
          component="span"
          onClick={() => setTab('global-git')}
          // semanticColors.focus is the role-less focus/selection surface —
          // a documented U2 exception mirroring the TUI's styleReverse/
          // styleSelected (theme.ts roles docstring).
          sx={{ color: semanticColors.focus, cursor: 'pointer', textDecoration: 'underline' }}
        >
          Edit in Global Git (3)
        </Box>
      </Typography>
    </Paper>
  );
}

// ---------------------------------------------------------------------------
// Shared SSH form (SSHUI-01 field order) — ONE component for both the create
// wizard and edit-SSH; "edit" is just data (`lockIdentity`), never a second
// copy of the fields.
// ---------------------------------------------------------------------------

interface SshFormValues {
  provider: string;
  prefix: string;
  sshHost: string;
  hostname: string;
  port: string;
}

function SshFormFields({
  values,
  onChange,
  lockIdentity = false,
  prefixError,
  hostHelper,
}: {
  values: SshFormValues;
  onChange: (v: SshFormValues) => void;
  /** Edit mode: identity name/provider never change in place (rename = clone). */
  lockIdentity?: boolean;
  prefixError?: string;
  hostHelper?: string;
}) {
  return (
    <>
      <Autocomplete
        freeSolo
        disablePortal
        options={PROVIDERS}
        value={values.provider}
        disabled={lockIdentity}
        onInputChange={(_, v) => onChange({ ...values, provider: v })}
        renderInput={(params) => (
          <TextField
            {...params}
            label="Provider"
            size="small"
            sx={fieldSx}
            helperText={
              lockIdentity
                ? 'Locked — the provider comes from the Host alias'
                : 'github.com · gitlab.com · bitbucket.org — or type any host'
            }
          />
        )}
      />
      <TextField
        label="Alias prefix"
        size="small"
        sx={fieldSx}
        value={values.prefix}
        disabled={lockIdentity}
        autoFocus={!lockIdentity}
        onChange={(e) => onChange({ ...values, prefix: e.target.value })}
        error={!lockIdentity && prefixError !== undefined}
        helperText={
          lockIdentity
            ? 'Locked — the identity name never changes in place; use Clone to rename'
            : (prefixError ?? 'Blank prefix → SSH Host = the provider host itself')
        }
      />
      <TextField
        label="SSH Host (alias)"
        size="small"
        sx={fieldSx}
        value={values.sshHost}
        onChange={(e) => onChange({ ...values, sshHost: e.target.value })}
        helperText={hostHelper ?? ''}
      />
      <TextField
        label="Real hostname"
        size="small"
        sx={fieldSx}
        value={values.hostname}
        onChange={(e) => onChange({ ...values, hostname: e.target.value })}
        helperText="The true SSH endpoint"
      />
      <TextField
        label="Port"
        size="small"
        sx={{ maxWidth: 160, ...fieldSx }}
        value={values.port}
        error={!/^\d+$/.test(values.port)}
        onChange={(e) => onChange({ ...values, port: e.target.value })}
        // review-findings F9: the Port field had no hint on either side.
        helperText="Default 22; 443 for alt-SSH"
      />
    </>
  );
}

// WIZARD_STEPS are the LONG step labels — the breadcrumb/help source AND
// (D5, checkpoint-2 contract) the StepDots render source. The bracketed
// short-segment stepper 02-14 shipped (bracket-number + short word per
// step, joined by middots) is SUPERSEDED: D5 reverts to `Step n/4 · <label>
// ● ○ ○ ○` using these long labels — the bracket format moves onto the
// MAIN NAV instead (D4).
const WIZARD_STEPS = ['SSH details', 'Test connection', 'Git identity', 'Review & write'];

// wizardChordHint (D5/D7, checkpoint-2 contract) is the ALWAYS-visible,
// step-conditional line directly under StepDots advertising the hoisted
// Shift+←/→ chord gate — frozen copy, verbatim, byte-identical to the TUI's
// wizardChordHint (identities.go).
function wizardChordHint(step: number): string {
  if (step === 0) return 'Shift+→ next section · Shift+← exits the wizard';
  if (step === WIZARD_STEPS.length - 1) return 'Shift+← back to Git · Enter writes';
  return 'Shift+←/→ jump sections · forward needs a valid step';
}

// D5 (checkpoint-2 contract): StepDots REVERTS to `Step n/4 · <label>
// ● ○ ○ ○` using the LONG WIZARD_STEPS labels — the bracketed short-segment
// format 02-14 shipped is superseded (it moves onto the MAIN NAV per D4).
// The counter is bold, the active label carries roles.activeArea accent at
// weight 700 (never dimmer than body text — review-findings F3's fix
// still holds), and dots render `●` accent for indices ≤ step / `○` dim.
function StepDots({ step }: { step: number }) {
  return (
    <>
      <Typography component="div" sx={{ fontSize: 13 }}>
        <Box component="span" sx={{ fontWeight: 700 }}>
          Step {step + 1}/{WIZARD_STEPS.length}
        </Box>{' '}
        <Box component="span" sx={{ color: 'text.secondary' }}>
          ·
        </Box>{' '}
        <Box component="span" sx={{ fontWeight: 700, color: roles.activeArea.color }}>
          {WIZARD_STEPS[step]}
        </Box>{' '}
        {WIZARD_STEPS.map((label, i) => (
          <Box
            key={label}
            component="span"
            title={label}
            sx={{ color: i <= step ? roles.activeArea.color : 'text.secondary', mr: 0.5 }}
          >
            {i <= step ? '●' : '○'}
          </Box>
        ))}
      </Typography>
      <Typography component="div" sx={{ fontSize: 12, color: 'text.secondary', mb: 1 }}>
        {wizardChordHint(step)}
      </Typography>
    </>
  );
}

// ---------------------------------------------------------------------------
// Merged Git form (author + signing + match strategy + dual preview) — used
// by wizard state 3 and by "Configure Git" on an existing identity.
// ---------------------------------------------------------------------------

interface GitFormValues {
  gitName: string;
  gitEmail: string;
  strategy: MatchStrategy;
}

function GitFormFields({
  name,
  keyPath,
  values,
  onChange,
}: {
  name: string;
  keyPath: string;
  values: GitFormValues;
  onChange: (v: GitFormValues) => void;
}) {
  const fragment = `[user]\n    name = ${values.gitName}\n    email = ${values.gitEmail}\n    signingkey = ${keyPath}.pub\n\n[gpg]\n    format = ssh\n\n[commit]\n    gpgsign = true`;
  const includeIf = gitScreenMatchStrategyPreview[values.strategy].replace(/personal/g, name);

  return (
    <Stack spacing={1.5}>
      <Stack direction="row" spacing={2}>
        <TextField
          label="user.name"
          size="small"
          fullWidth
          sx={fieldSx}
          value={values.gitName}
          onChange={(e) => onChange({ ...values, gitName: e.target.value })}
        />
        <TextField
          label="user.email"
          size="small"
          fullWidth
          sx={fieldSx}
          value={values.gitEmail}
          error={!values.gitEmail.includes('@')}
          helperText="Kept byte-identical to ~/.ssh/allowed_signers (GITUI-04)"
          onChange={(e) => onChange({ ...values, gitEmail: e.target.value })}
        />
      </Stack>
      <Typography sx={{ fontSize: 13, color: 'text.secondary' }}>
        Signing: gpg.format = ssh (fixed) · signingkey = {keyPath}.pub — a PATH, never key material.
      </Typography>
      {/* D2 (checkpoint-2 contract): the match-strategy Select becomes a
          RadioGroup — ALL options render ALWAYS (no expand/collapse), and
          the (←/→ change) hint moves onto the group's HEADER line, visible
          regardless of focus (mirrors the delete-scope RadioGroup pattern
          already established below). */}
      <Box sx={{ maxWidth: 520 }}>
        <Typography component="label" sx={{ fontSize: 13, fontWeight: 700, display: 'block' }}>
          Match strategy — when does this Git identity apply?{' '}
          <Box component="span" sx={{ color: roles.hint.color, fontWeight: 400 }}>
            (←/→ change)
          </Box>
        </Typography>
        <RadioGroup
          value={values.strategy}
          onChange={(e) => onChange({ ...values, strategy: e.target.value as MatchStrategy })}
        >
          <FormControlLabel
            value="gitdir"
            control={<Radio size="small" />}
            label={`gitdir (default) — applies inside ~/${name}/`}
          />
          <FormControlLabel
            value="hasconfig"
            control={<Radio size="small" />}
            label="hasconfig — repos whose remote uses this alias"
          />
          <FormControlLabel
            value="both"
            control={<Radio size="small" />}
            label="both — either condition (two includeIf blocks = OR)"
          />
        </RadioGroup>
        {/* Hint-persistence (02-STYLE-SPEC.md): ALWAYS rendered — the
            always-expanded radio group can never push this line away. */}
        <Typography sx={{ fontSize: 12, color: roles.hint.color }}>
          gitdir matches by working-directory path; hasconfig matches by remote URL; both = either condition (OR).
        </Typography>
      </Box>
      {/* review-findings F1: PreviewBlock's title/maxHeight props (added in
          02-14 Task 1) had no call site — routed through here instead of a
          separate PreviewLabel, mirroring the TUI's title-in-border-top-edge
          treatment. */}
      <Stack direction={{ xs: 'column', lg: 'row' }} spacing={1.5}>
        <Box sx={{ flex: 1 }}>
          <PreviewBlock title={`~/.gitconfig.d/${name} (fragment file — preview)`} maxHeight={140} text={fragment} />
        </Box>
        <Box sx={{ flex: 1 }}>
          <PreviewBlock title="~/.gitconfig (includeIf block — preview)" maxHeight={140} text={includeIf} />
        </Box>
      </Stack>
      <BaselineStrip />
    </Stack>
  );
}

// ---------------------------------------------------------------------------
// Create wizard — 4 pane-states in the detail pane (spec §3).
// ---------------------------------------------------------------------------

type TestPhase = 'idle' | 'running1' | 'stage1' | 'running2' | 'stage2' | 'failed';

function CreateWizard({ onDone, onCancel }: { onDone: (name: string) => void; onCancel: () => void }) {
  const { state, dispatch, notify } = useDemo();
  const [step, setStep] = useState(0);

  const [provider, setProvider] = useState('github.com');
  const [prefix, setPrefix] = useState('acme');
  const [hostTouched, setHostTouched] = useState(false);
  const [hostOverride, setHostOverride] = useState('');
  const [endpointTouched, setEndpointTouched] = useState(false);
  const [hostname, setHostname] = useState('ssh.github.com');
  const [port, setPort] = useState('443');
  const [algo, setAlgo] = useState('ed25519');

  const [testPhase, setTestPhase] = useState<TestPhase>('idle');
  const [simulateFail, setSimulateFail] = useState(false);

  const [configureGit, setConfigureGit] = useState(true);
  const [git, setGit] = useState<GitFormValues>({
    gitName: 'Acme Identity',
    gitEmail: 'you@acme.example',
    strategy: defaultMatchStrategy,
  });

  const name = prefix.trim() || (provider.split('.')[0] ?? 'github');
  const autoHost = prefix.trim() ? `${prefix.trim()}.${provider}` : provider;
  const sshHost = hostTouched ? hostOverride : autoHost;
  const keyPath = `~/.ssh/id_ed25519_${name}`;
  const nameTaken = state.identities.some((row) => row.name === name);

  const applyProvider = (p: string) => {
    setProvider(p);
    if (!endpointTouched) {
      const d = providerDefaults(p);
      setHostname(d.hostname);
      setPort(d.port);
    }
  };

  const sentinels = managedBlockSentinels(name);
  const hostBlock = `Host ${sshHost}\n    Hostname ${hostname}\n    Port ${port}\n    User git\n    IdentityFile ${keyPath}\n    IdentitiesOnly yes`;
  const managedBlock = `${sentinels.begin}\n${hostBlock}\n${sentinels.end}`;

  const stage1Cmd = `ssh -T -F ${sshTestTmpConfigPath} -p ${port} -i ${keyPath} git@${hostname}`;
  const stage2Cmd = `ssh -G -F ${sshTestTmpConfigPath} ${sshHost} | grep identityfile`;

  const fragment = `[user]\n    name = ${git.gitName}\n    email = ${git.gitEmail}\n    signingkey = ${keyPath}.pub\n\n[gpg]\n    format = ssh\n\n[commit]\n    gpgsign = true`;
  const includeIf = gitScreenMatchStrategyPreview[git.strategy].replace(/personal/g, name);
  const reviewText = configureGit
    ? `${managedBlock}\n\n# ~/.gitconfig.d/${name}\n${fragment}\n\n# ~/.gitconfig\n${includeIf}`
    : managedBlock;

  const step0Valid = !nameTaken && hostname.trim() !== '' && /^\d+$/.test(port) && sshHost.trim() !== '';

  const runStage1 = useCallback(() => {
    setTestPhase('running1');
    window.setTimeout(() => setTestPhase(simulateFail ? 'failed' : 'stage1'), 900);
  }, [simulateFail]);
  const runStage2 = useCallback(() => {
    setTestPhase('running2');
    window.setTimeout(() => setTestPhase('stage2'), 700);
  }, []);

  // D7 (checkpoint-2 contract) blocked-forward status notes — frozen copy,
  // verbatim, byte-identical to the TUI's blockedForwardNote (identities.go).
  const blockedForwardNote = (s: number): string => {
    if (s === 0) return "Can't continue yet — check the alias prefix, hostname, and port.";
    if (s === 2) return "Can't continue yet — add user.name and a valid email.";
    return '';
  };

  // stepForward mirrors the TUI's wizardModel.stepForward: advances ONLY
  // when the current step's own validity gate passes, reporting whether it
  // did — shared by BOTH the plain-arrow/Enter path and the Shift chord, so
  // Shift is a FOCUS-OVERRIDE only, never a validity override.
  const stepForward = useCallback((): boolean => {
    if (step === 0 && step0Valid) {
      setStep(1);
      return true;
    }
    if (step === 1 && testPhase === 'stage2') {
      setStep(2);
      return true;
    }
    if (step === 2 && git.gitName.trim() !== '' && git.gitEmail.includes('@')) {
      setConfigureGit(true);
      setStep(3);
      return true;
    }
    return false;
  }, [step, step0Valid, testPhase, git]);

  useLocalKeys(
    useCallback(
      (key, event) => {
        // D7 (checkpoint-2 contract): ONE hoisted Shift+←/→ chord gate,
        // checked BEFORE the `step === 3` early-return below — reaches
        // wizard step-nav from EVERY step, including the PREVIOUSLY-DEAD
        // review ceremony (step 3), where the ceremony's own local key
        // handler used to swallow the chord entirely.
        if (event.shiftKey && (key === 'ArrowLeft' || key === 'ArrowRight')) {
          if (key === 'ArrowLeft') {
            if (step === 0) onCancel();
            else setStep((s) => s - 1);
            return true;
          }
          if (!stepForward()) notify(blockedForwardNote(step));
          return true;
        }
        if (step === 3) return false; // ceremony owns keys (Enter/Esc/Tab)
        if (key === 'Escape') {
          if (step === 0) onCancel();
          else setStep((s) => s - 1);
          return true;
        }
        if (key === 'Enter') {
          if (step === 0 && step0Valid) setStep(1);
          else if (step === 1) {
            if (testPhase === 'idle') runStage1();
            else if (testPhase === 'stage1') runStage2();
            else if (testPhase === 'failed') setTestPhase('idle');
            else if (testPhase === 'stage2') setStep(2);
          } else if (step === 2) {
            if (git.gitName.trim() !== '' && git.gitEmail.includes('@')) {
              setConfigureGit(true);
              setStep(3);
            }
          }
          return true;
        }
        // Plain ←/→: back always allowed; forward validity-gated. A
        // focused native radio input never reaches here (DemoApp's global
        // `isToggle` guard returns before screen-local handlers run), so no
        // Select/combobox target check is needed (D2 replaced both the
        // match-strategy and algorithm Selects with RadioGroups).
        if (key === 'ArrowLeft') {
          if (step === 0) onCancel();
          else setStep((s) => s - 1);
          return true;
        }
        if (key === 'ArrowRight') {
          stepForward();
          return true;
        }
        return false;
      },
      [step, step0Valid, testPhase, git, onCancel, runStage1, runStage2, stepForward, notify],
    ),
  );

  const finish = useCallback(() => {
    const identity: DemoIdentity = {
      name,
      state: configureGit ? 'complete' : 'incomplete',
      sshHost,
      keyPath,
      hostname,
      port: Number(port),
      ...(configureGit
        ? {
            gitFragmentPath: `~/.gitconfig.d/${name}`,
            gitName: git.gitName,
            gitEmail: git.gitEmail,
            matchStrategy: git.strategy,
            note: 'SSH Host block and Git fragment both present.',
          }
        : { note: 'SSH Host block present; no Git identity configured for this alias.' }),
    };
    dispatch({ type: 'add-identity', identity, backup: newBackupPath('~/.ssh/config') });
    notify(
      configureGit
        ? `Identity "${name}" created — SSH + Git configured (${git.strategy}).`
        : `Identity "${name}" created — SSH only (incomplete). Configure Git from its detail.`,
    );
    onDone(name);
  }, [name, configureGit, sshHost, keyPath, hostname, port, git, dispatch, notify, onDone]);

  return (
    <Box>
      <StepDots step={step} />

      {step === 0 && (
        <Stack spacing={1.5} sx={{ maxWidth: 620 }}>
          {/* One field per row (round-3 feedback): multi-field rows made the
              inputs read as labels — a single column keeps every editable
              box unmistakable. */}
          <SshFormFields
            values={{ provider, prefix, sshHost, hostname, port }}
            prefixError={nameTaken ? `"${name}" already exists — pick another prefix.` : undefined}
            hostHelper={hostTouched ? 'Manually edited — auto-join off' : 'Auto-joined: <prefix>.<provider> — editable'}
            onChange={(v) => {
              if (v.provider !== provider) applyProvider(v.provider);
              if (v.prefix !== prefix) setPrefix(v.prefix);
              if (v.sshHost !== sshHost) {
                setHostTouched(true);
                setHostOverride(v.sshHost);
              }
              if (v.hostname !== hostname) {
                setEndpointTouched(true);
                setHostname(v.hostname);
              }
              if (v.port !== port) {
                setEndpointTouched(true);
                setPort(v.port);
              }
            }}
          />
          {/* D2 (checkpoint-2 contract): the algorithm Select becomes a
              RadioGroup too — every option already rendered ALWAYS (no
              expand/collapse existed here), so this is a glyph/markup
              change only; per-option availability sub-text + disabled are
              kept, and the (←/→ change) hint moves onto the header. */}
          <Box>
            <Typography component="label" sx={{ fontSize: 13, fontWeight: 700, display: 'block' }}>
              Key algorithm{' '}
              <Box component="span" sx={{ color: roles.hint.color, fontWeight: 400 }}>
                (←/→ change)
              </Box>
            </Typography>
            <RadioGroup value={algo} onChange={(e) => setAlgo(e.target.value)}>
              {algorithmCatalog.map((entry) => (
                <FormControlLabel
                  key={entry.id}
                  value={entry.id}
                  disabled={entry.macosAvailability === 'requires-libfido2'}
                  control={<Radio size="small" />}
                  label={
                    <Box>
                      <Typography component="p">
                        {entry.label}
                        {entry.recommended ? ' — ★ recommended' : ''}
                      </Typography>
                      <Typography sx={{ fontSize: 12, color: 'text.secondary', whiteSpace: 'normal' }}>
                        {entry.macosAvailability === 'requires-libfido2'
                          ? 'Disabled: needs libfido2 + a FIDO2 security key — none detected on this machine'
                          : entry.macos}
                      </Typography>
                    </Box>
                  }
                />
              ))}
            </RadioGroup>
            <Typography sx={{ fontSize: 12, color: roles.hint.color }}>
              gitid probes the local toolchain (ssh-keygen, libfido2, FIDO2 key present?) and disables what this
              machine cannot generate, with the reason shown per option (KEY-03/PLAT-01). Demo simulates: no FIDO2
              key plugged in.
            </Typography>
          </Box>
          {/* review-findings F1: routed through PreviewBlock's title prop
              (mirrors the TUI's renderHostBlockPreview). */}
          <Box>
            <PreviewBlock title="Live Host-block preview — written exactly like this on confirm" maxHeight={170} text={hostBlock} />
          </Box>
          <Stack direction="row" spacing={2}>
            <Button variant="outlined" onClick={onCancel}>
              Cancel (Esc)
            </Button>
            <Button variant="contained" disabled={!step0Valid} onClick={() => setStep(1)}>
              Next: test connection (Enter)
            </Button>
          </Stack>
        </Stack>
      )}

      {step === 1 && (
        <Stack spacing={1.5}>
          <Alert severity="info" variant="outlined" sx={{ borderRadius: 0 }}>
            Key {keyPath} generated ({algo}). Both stages run against {sshTestTmpConfigPath} — your
            live ~/.ssh/config is untouched until the final confirm.
          </Alert>
          <Box>
            <FormControlLabel
              control={
                <Checkbox
                  checked={simulateFail}
                  onChange={(e) => setSimulateFail(e.target.checked)}
                  disabled={testPhase !== 'idle' && testPhase !== 'failed'}
                />
              }
              label="Demo control — simulate a provider failure (key not registered) to preview the error path"
            />
            <Typography sx={{ fontSize: 12, color: 'text.disabled', pl: 4 }}>
              Review aid only, not part of the real flow. It locks while a stage is running and once
              the test has passed — there is nothing left to simulate then.
            </Typography>
          </Box>
          {/* review-findings F1: the "Stage 1 — ..." label moves into
              PreviewBlock's title prop, mirroring the TUI's border-top-edge
              treatment. */}
          <PreviewBlock title="Stage 1 — key DIRECT against the provider (TEST-01)" maxHeight={70} text={`$ ${stage1Cmd}`} />
          {testPhase === 'idle' && (
            <Button variant="contained" onClick={runStage1} sx={{ alignSelf: 'flex-start' }}>
              Run stage 1 (Enter)
            </Button>
          )}
          {(testPhase === 'running1' || testPhase === 'running2') && (
            <Stack direction="row" spacing={1} alignItems="center">
              <CircularProgress size={16} />
              <Typography sx={{ color: 'text.secondary' }}>running ssh…</Typography>
            </Stack>
          )}
          {testPhase === 'failed' && (
            <>
              <Box sx={{ color: roles.error.color }}>✗ git@{hostname}: Permission denied (publickey).</Box>
              <Alert severity="error" variant="outlined" sx={{ borderRadius: 0 }}>
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
            <Box sx={{ color: roles.healthy.color }}>
              ✓ Hi {name}! You've successfully authenticated, but GitHub does not provide shell access.
            </Box>
          )}
          {(testPhase === 'stage1' || testPhase === 'stage2') && (
            <>
              {/* review-findings F1: the short "Stage 2 — ..." label moves
                  into PreviewBlock's title prop; the longer no-`-i`-on-
                  purpose rationale stays as an adjacent hint line, mirroring
                  the TUI split. */}
              <Typography variant="subtitle2" sx={{ color: 'text.secondary' }}>
                No -i here on purpose: the config must supply the key; that is exactly what this
                stage proves.
              </Typography>
              <PreviewBlock title="Stage 2 — resolve BY ALIAS (TEST-02)" maxHeight={70} text={`$ ${stage2Cmd}`} />
              {testPhase === 'stage1' && (
                <Button variant="contained" onClick={runStage2} sx={{ alignSelf: 'flex-start' }}>
                  Run stage 2 (Enter)
                </Button>
              )}
              {testPhase === 'stage2' && (
                <Box sx={{ color: roles.healthy.color }}>✓ identityfile {keyPath}</Box>
              )}
            </>
          )}
          <Stack direction="row" spacing={2}>
            <Button variant="outlined" onClick={() => setStep(0)}>
              Back (Esc)
            </Button>
            <Button variant="contained" disabled={testPhase !== 'stage2'} onClick={() => setStep(2)}>
              Next: Git identity (Enter)
            </Button>
          </Stack>
        </Stack>
      )}

      {step === 2 && (
        <Stack spacing={1.5}>
          {/* Round-3 feedback: assume the user wants Git configured — show
              the FULL form immediately; Skip is just a button at the end. */}
          <GitFormFields name={name} keyPath={keyPath} values={git} onChange={setGit} />
          <Stack direction="row" spacing={2} alignItems="flex-start">
            <Button variant="outlined" onClick={() => setStep(1)}>
              Back (Esc)
            </Button>
            <Stack>
              <Button
                variant="outlined"
                onClick={() => {
                  setConfigureGit(false);
                  setStep(3);
                }}
              >
                [ Skip Git ]
              </Button>
              <Typography sx={{ fontSize: 12, color: roles.hint.color, mt: 0.25 }}>
                Skip keeps this identity SSH-only and marks it incomplete.
              </Typography>
            </Stack>
            <Stack>
              <Button
                variant="contained"
                disabled={git.gitName.trim() === '' || !git.gitEmail.includes('@')}
                onClick={() => {
                  setConfigureGit(true);
                  setStep(3);
                }}
              >
                [ Continue ]
              </Button>
              {/* D7 (checkpoint-2 contract): the disabled reason text —
                  byte-identical to the TUI's gitFormDisabledSuffix. */}
              {(git.gitName.trim() === '' || !git.gitEmail.includes('@')) && (
                <Typography sx={{ fontSize: 12, color: roles.error.color, mt: 0.25 }}>
                  — needs user.name + a valid email
                </Typography>
              )}
              <Typography sx={{ fontSize: 12, color: roles.hint.color, mt: 0.25 }}>
                Continue reviews the Git fragment, includeIf, and allowed_signers entries before writing.
              </Typography>
            </Stack>
          </Stack>
        </Stack>
      )}

      {step === 3 && (
        <MutationCeremony
          heading={`Create identity "${name}" — ${algo}, test passed ✓`}
          targets={
            configureGit
              ? ['~/.ssh/config', `~/.gitconfig.d/${name}`, '~/.gitconfig', '~/.ssh/allowed_signers']
              : ['~/.ssh/config']
          }
          preview={
            <Stack spacing={1}>
              <Typography sx={{ fontSize: 13, color: 'text.secondary' }}>
                SSH: {sshHost} → {hostname}:{port} · key {keyPath} · Git:{' '}
                {configureGit ? `${git.gitName} <${git.gitEmail}>, strategy ${git.strategy}` : 'skipped'}
              </Typography>
              {/* review-findings F1: bounded (maxHeight), but WITHOUT a
                  duplicate title — MutationCeremony already renders "Exact
                  change — ..." directly above this preview node; a second
                  title here would repeat that heading. */}
              <PreviewBlock maxHeight={260} text={reviewText} />
            </Stack>
          }
          backups={
            configureGit
              ? [newBackupPath('~/.ssh/config'), newBackupPath('~/.gitconfig')]
              : [newBackupPath('~/.ssh/config')]
          }
          resultMessage={`Identity "${name}" created — ${sshHost} now resolves to ${keyPath}.`}
          confirmLabel="Write it"
          onCancel={() => setStep(2)}
          onDone={finish}
        />
      )}
    </Box>
  );
}

// ---------------------------------------------------------------------------
// The Identities view.
// ---------------------------------------------------------------------------

export function Identities() {
  const { state, dispatch, notify, setTab } = useDemo();
  const [selectedName, setSelectedName] = useState<string>(state.identities[0]?.name ?? '');
  const [pane, setPane] = useState<PaneMode>({ kind: 'detail' });
  const [deleteAsk, setDeleteAsk] = useState(false);
  const [deleteScope, setDeleteScope] = useState<'everything' | 'git-only'>('git-only');
  const [cloneName, setCloneName] = useState('');
  const [git, setGit] = useState<GitFormValues | null>(null);
  const [editHost, setEditHost] = useState<SshFormValues>({
    provider: '',
    prefix: '',
    sshHost: '',
    hostname: '',
    port: '',
  });

  const rows = state.identities;
  const selected = rows.find((r) => r.name === selectedName) ?? rows[0];
  const selectedIdx = selected ? rows.findIndex((r) => r.name === selected.name) : -1;
  const findings = selected ? findingsFor(state, selected.name) : [];

  const toDetail = useCallback(() => setPane({ kind: 'detail' }), []);

  const openGitForm = useCallback(() => {
    if (!selected) return;
    setGit({
      gitName: selected.gitName ?? `${selected.name} identity`,
      gitEmail: selected.gitEmail ?? `you@${selected.name}.example`,
      strategy: selected.matchStrategy ?? defaultMatchStrategy,
    });
    setPane({ kind: 'git' });
  }, [selected]);

  const openEditSsh = useCallback(() => {
    if (!selected) return;
    const sshHost = selected.sshHost ?? `${selected.name}.github.com`;
    setEditHost({
      provider: sshHost.split('.').slice(-2).join('.') || 'github.com',
      prefix: selected.name,
      sshHost,
      hostname: selected.hostname ?? 'ssh.github.com',
      port: String(selected.port ?? 443),
    });
    setPane({ kind: 'edit-ssh' });
  }, [selected]);

  useLocalKeys(
    useCallback(
      (key) => {
        if (pane.kind !== 'detail') {
          if (key === 'Escape') {
            toDetail();
            return true;
          }
          return false;
        }
        if (key === 'ArrowDown') {
          const next = rows[Math.min(selectedIdx + 1, rows.length - 1)];
          if (next) setSelectedName(next.name);
          return true;
        }
        if (key === 'ArrowUp') {
          const prev = rows[Math.max(selectedIdx - 1, 0)];
          if (prev) setSelectedName(prev.name);
          return true;
        }
        if (key === 'n') {
          setPane({ kind: 'create' });
          return true;
        }
        if (key === 'c' && selected) {
          setCloneName(`${selected.name}-clone`);
          setPane({ kind: 'clone' });
          return true;
        }
        if (key === 'd' && selected) {
          setDeleteAsk(true);
          return true;
        }
        if (key === 'e' && selected) {
          openEditSsh();
          return true;
        }
        if (key === 'g' && selected) {
          openGitForm();
          return true;
        }
        return false;
      },
      [pane.kind, rows, selectedIdx, selected, toDetail, openEditSsh, openGitForm],
    ),
  );

  const crumbs: string[] =
    pane.kind === 'create'
      ? ['New identity']
      : pane.kind === 'git'
        ? [selected?.name ?? '', 'Configure Git']
        : pane.kind === 'edit-ssh'
          ? [selected?.name ?? '', 'Edit SSH']
          : pane.kind === 'clone'
            ? [selected?.name ?? '', 'Clone']
            : pane.kind === 'delete'
              ? [selected?.name ?? '', 'Delete']
              : pane.kind === 'fix'
                ? [selected?.name ?? '', 'Fix']
                : selected
                  ? [selected.name]
                  : [];

  const actions: FrameAction[] =
    pane.kind === 'detail'
      ? [
          { key: '↑↓', label: 'select identity' },
          { key: 'n', label: 'new', onActivate: () => setPane({ kind: 'create' }) },
          { key: 'e', label: 'edit SSH', onActivate: openEditSsh },
          { key: 'g', label: 'configure Git', onActivate: openGitForm },
          { key: 'c', label: 'clone', onActivate: () => selected && (setCloneName(`${selected.name}-clone`), setPane({ kind: 'clone' })) },
          { key: 'd', label: 'delete', onActivate: () => setDeleteAsk(true) },
        ]
      : []; // reserved "Esc back" already covers leaving a form

  const fixFinding = pane.kind === 'fix' ? findings.find((f) => f.id === pane.findingId) : undefined;

  return (
    <Frame
      crumbs={crumbs}
      statusMessage={
        pane.kind === 'detail'
          ? `${rows.length} identities — selection renders the detail live; every action is dummy but really changes this state.`
          : 'Esc returns to the identity detail without writing anything.'
      }
      actions={actions}
      capturesKeys={pane.kind !== 'detail'}
    >
      <Stack direction="row" spacing={2} alignItems="stretch">
        {/* -------- sidebar -------- */}
        <Paper variant="outlined" sx={{ width: '38%', minWidth: 330, opacity: pane.kind === 'detail' ? 1 : 0.75 }}>
          <Typography sx={{ px: 1.5, py: 0.5, fontSize: 12, color: 'text.disabled', borderBottom: 1, borderColor: 'divider' }}>
            S ssh · G git&nbsp;&nbsp;&nbsp;✓ ok&nbsp; ! attn&nbsp; ✗ broken
          </Typography>
          <List disablePadding>
            {rows.map((row) => {
              const p = pips(row);
              const rowFindings = findingsFor(state, row.name);
              return (
                <ListItemButton
                  key={row.name}
                  selected={row.name === selected?.name}
                  onClick={() => {
                    setSelectedName(row.name);
                    setPane({ kind: 'detail' });
                  }}
                  sx={{ borderBottom: 1, borderColor: 'divider', py: 0.5, display: 'block' }}
                >
                  <Stack direction="row" spacing={1} alignItems="center">
                    <Box component="span" sx={{ color: toneColor[identityManagerStateTone[row.state]], width: 14 }}>
                      {identityManagerStateGlyph[row.state]}
                    </Box>
                    <Box component="span" sx={{ fontWeight: 700, flex: 1 }}>
                      {row.name}
                    </Box>
                    {rowFindings.length > 0 && (
                      <Box component="span" sx={{ color: roles.warning.color, fontSize: 12 }}>
                        {rowFindings.length}⚑
                      </Box>
                    )}
                    <Box component="span" sx={{ fontSize: 12 }}>
                      <span style={{ color: '#8a8a8a' }}>S</span>
                      <span style={{ color: pipColor[p.s] }}>{p.s}</span>{' '}
                      <span style={{ color: '#8a8a8a' }}>G</span>
                      <span style={{ color: pipColor[p.g] }}>{p.g}</span>
                    </Box>
                  </Stack>
                  <Typography noWrap sx={{ fontSize: 12, color: 'text.secondary', pl: 2.75 }}>
                    {row.note}
                  </Typography>
                </ListItemButton>
              );
            })}
          </List>
        </Paper>

        {/* -------- detail / form pane -------- */}
        <Box sx={{ flex: 1, minWidth: 0 }}>
          {pane.kind === 'create' && (
            <CreateWizard
              onCancel={toDetail}
              onDone={(name) => {
                setSelectedName(name);
                toDetail();
              }}
            />
          )}

          {pane.kind === 'detail' && selected && (
            <Stack spacing={1.5}>
              <Stack direction="row" spacing={1} alignItems="center">
                <Typography variant="h6" component="h1">
                  {selected.name}
                </Typography>
                <Chip
                  size="small"
                  variant="outlined"
                  label={`${identityManagerStateGlyph[selected.state]} ${selected.state}`}
                  sx={{
                    borderRadius: 0,
                    fontFamily: 'inherit',
                    color: toneColor[identityManagerStateTone[selected.state]],
                    borderColor: toneColor[identityManagerStateTone[selected.state]],
                  }}
                />
              </Stack>

              <Paper variant="outlined" sx={{ p: 1.5 }}>
                <Typography variant="subtitle2" sx={{ color: 'text.secondary', mb: 0.5 }}>
                  SSH — shown first, always
                </Typography>
                {selected.sshHost ? (
                  <Stack spacing={0.25}>
                    <Typography>Host alias: {selected.sshHost}</Typography>
                    <Typography>
                      Hostname: {selected.hostname ?? 'ssh.github.com'} · Port {selected.port ?? 443} · User git
                    </Typography>
                    <Typography>IdentityFile: {selected.keyPath ?? '— missing'}</Typography>
                    <Typography>IdentitiesOnly: yes</Typography>
                  </Stack>
                ) : (
                  <Typography sx={{ color: roles.warning.color }}>
                    ! No gitid-managed Host block — relies on the global SSH config.
                  </Typography>
                )}
              </Paper>

              <Paper variant="outlined" sx={{ p: 1.5 }}>
                <Typography variant="subtitle2" sx={{ color: 'text.secondary', mb: 0.5 }}>
                  Git
                </Typography>
                {selected.gitFragmentPath ? (
                  <Stack spacing={0.25}>
                    <Typography>Fragment: {selected.gitFragmentPath}</Typography>
                    <Typography>
                      Author: {selected.gitName ?? '—'} &lt;{selected.gitEmail ?? '—'}&gt;
                    </Typography>
                    <Typography>
                      Signing: gpg.format=ssh · signingkey {selected.keyPath ?? '?'}.pub
                      {selected.matchStrategy ? ` · strategy ${selected.matchStrategy}` : ''}
                    </Typography>
                  </Stack>
                ) : (
                  <Stack direction="row" spacing={2} alignItems="center">
                    <Typography sx={{ color: roles.warning.color }}>
                      ! Git not configured — no fabricated values shown.
                    </Typography>
                    <Button size="small" variant="contained" onClick={openGitForm}>
                      Configure now
                    </Button>
                  </Stack>
                )}
                <BaselineStrip />
              </Paper>

              <Paper
                variant="outlined"
                sx={{ p: 1.5, borderColor: findings.length > 0 ? roles.warning.color : 'divider' }}
              >
                <Typography variant="subtitle2" sx={{ color: 'text.secondary', mb: 0.5 }}>
                  Findings ({findings.length}) — same data the Doctor shows (4)
                </Typography>
                {findings.length === 0 ? (
                  <Typography sx={{ color: roles.healthy.color }}>✓ No findings for “{selected.name}”.</Typography>
                ) : (
                  <Stack spacing={0.75}>
                    {findings.map((f) => (
                      <Stack key={f.id} direction="row" spacing={1} alignItems="center">
                        <Box component="span" sx={{ color: severityColor[f.severity], whiteSpace: 'nowrap' }}>
                          {healthSeverityGlyph[f.severity]} {f.severity}
                        </Box>
                        <Typography sx={{ flex: 1 }} noWrap title={f.explanation}>
                          {f.title}
                        </Typography>
                        {f.suggestedFix ? (
                          <Button size="small" variant="outlined" onClick={() => setPane({ kind: 'fix', findingId: f.id })}>
                            Fix…
                          </Button>
                        ) : (
                          <Typography sx={{ fontSize: 12, color: 'text.disabled' }}>info only</Typography>
                        )}
                      </Stack>
                    ))}
                    <Typography
                      // semanticColors.focus: role-less focus/selection
                      // surface — documented U2 exception (theme.ts).
                      sx={{ fontSize: 12, color: semanticColors.focus, cursor: 'pointer', textDecoration: 'underline' }}
                      onClick={() => setTab('doctor')}
                    >
                      Open the Doctor (4) for the global picture
                    </Typography>
                  </Stack>
                )}
              </Paper>
            </Stack>
          )}

          {pane.kind === 'edit-ssh' && selected && (
            <Stack spacing={1.5} sx={{ maxWidth: 620 }}>
              <Typography variant="h6">Edit SSH — {selected.name}</Typography>
              {/* SAME form component as "new identity" — edit is just
                  lockIdentity=true, never a second copy of the fields. */}
              <SshFormFields
                values={editHost}
                lockIdentity
                onChange={setEditHost}
              />
              <MutationCeremony
                heading={`Rewrite the managed Host block for "${selected.name}"`}
                targets={['~/.ssh/config']}
                preview={
                  <PreviewBlock
                    text={`Host ${editHost.sshHost}\n    Hostname ${editHost.hostname}\n    Port ${editHost.port}\n    User git\n    IdentityFile ${selected.keyPath ?? `~/.ssh/id_ed25519_${selected.name}`}\n    IdentitiesOnly yes`}
                  />
                }
                backups={[newBackupPath('~/.ssh/config')]}
                resultMessage={`Host block for "${selected.name}" rewritten.`}
                confirmLabel="Save changes"
                onCancel={toDetail}
                onDone={() => {
                  dispatch({
                    type: 'edit-ssh',
                    name: selected.name,
                    sshHost: editHost.sshHost,
                    hostname: editHost.hostname,
                    port: Number(editHost.port),
                    backup: newBackupPath('~/.ssh/config'),
                  });
                  notify(`SSH settings of "${selected.name}" updated.`);
                  toDetail();
                }}
              />
            </Stack>
          )}

          {pane.kind === 'git' && selected && git && (
            <Stack spacing={1.5}>
              <Typography variant="h6">
                Git identity — {selected.name}
                {selected.gitFragmentPath ? ' (editing existing fragment)' : ' (completes this identity)'}
              </Typography>
              <GitFormFields
                name={selected.name}
                keyPath={selected.keyPath ?? `~/.ssh/id_ed25519_${selected.name}`}
                values={git}
                onChange={setGit}
              />
              <MutationCeremony
                heading={`Write Git identity for "${selected.name}"`}
                targets={[`~/.gitconfig.d/${selected.name}`, '~/.gitconfig', '~/.ssh/allowed_signers']}
                preview={
                  <PreviewBlock
                    text={gitScreenMatchStrategyPreview[git.strategy].replace(/personal/g, selected.name)}
                  />
                }
                backups={[newBackupPath('~/.gitconfig'), newBackupPath('~/.ssh/allowed_signers')]}
                resultMessage={`Git identity "${selected.name}" configured — applies via the ${git.strategy} strategy.`}
                confirmLabel="Write it"
                onCancel={toDetail}
                onDone={() => {
                  dispatch({
                    type: 'configure-git',
                    name: selected.name,
                    gitName: git.gitName,
                    gitEmail: git.gitEmail,
                    matchStrategy: git.strategy,
                    backup: newBackupPath('~/.gitconfig'),
                  });
                  notify(`Git identity "${selected.name}" configured.`);
                  toDetail();
                }}
              />
            </Stack>
          )}

          {pane.kind === 'clone' && selected && (
            <Stack spacing={1.5} sx={{ maxWidth: 520 }}>
              <Typography variant="h6">Clone “{selected.name}”</Typography>
              <Typography sx={{ color: 'text.secondary' }}>
                The clone gets its own new key and Host alias; the Git author is copied (MGR-04).
              </Typography>
              <TextField
                label="New identity name"
                size="small"
                autoFocus
                value={cloneName}
                onChange={(e) => setCloneName(e.target.value)}
                error={rows.some((r) => r.name === cloneName) || cloneName.trim() === ''}
                helperText={
                  rows.some((r) => r.name === cloneName)
                    ? 'That name already exists.'
                    : `Creates ${cloneName}.github.com + ~/.ssh/id_ed25519_${cloneName}`
                }
              />
              <Stack direction="row" spacing={2}>
                <Button variant="outlined" onClick={toDetail}>
                  Cancel (Esc)
                </Button>
                <Button
                  variant="contained"
                  disabled={rows.some((r) => r.name === cloneName) || cloneName.trim() === ''}
                  onClick={() => {
                    dispatch({ type: 'clone-identity', source: selected.name, cloneName });
                    notify(`Identity "${cloneName}" cloned from "${selected.name}".`);
                    setSelectedName(cloneName);
                    toDetail();
                  }}
                >
                  Clone
                </Button>
              </Stack>
            </Stack>
          )}

          {pane.kind === 'delete' && selected && (
            <MutationCeremony
              heading={
                pane.scope === 'everything'
                  ? `Delete EVERYTHING for "${selected.name}" (SSH + Git + key)`
                  : `Delete the Git identity of "${selected.name}" (SSH stays)`
              }
              targets={
                pane.scope === 'everything'
                  ? ['~/.ssh/config', '~/.gitconfig', selected.gitFragmentPath ?? `~/.gitconfig.d/${selected.name}`, selected.keyPath ?? `~/.ssh/id_ed25519_${selected.name}`]
                  : ['~/.gitconfig', selected.gitFragmentPath ?? `~/.gitconfig.d/${selected.name}`, '~/.ssh/allowed_signers']
              }
              preview={
                <PreviewBlock
                  diff
                  text={
                    pane.scope === 'everything'
                      ? `- Host ${selected.sshHost ?? `${selected.name}.github.com`} (managed block removed)\n- [includeIf] → ${selected.gitFragmentPath ?? '—'} (removed)\n- ${selected.keyPath ?? '—'} (key file removed)`
                      : `- [includeIf] → ${selected.gitFragmentPath ?? '—'} (removed)\n- ${selected.gitFragmentPath ?? '—'} (fragment removed)\n  Host ${selected.sshHost ?? '—'} (unchanged)`
                  }
                />
              }
              destructive={
                pane.scope === 'everything'
                  ? {
                      confirmWord: selected.name,
                      warning: `This removes the key file too — it cannot be regenerated. Type the identity name "${selected.name}" to confirm.`,
                    }
                  : undefined
              }
              backups={[newBackupPath('~/.ssh/config'), newBackupPath('~/.gitconfig')]}
              resultMessage={
                pane.scope === 'everything'
                  ? `Identity "${selected.name}" deleted — SSH block, Git fragment, and key removed (backups kept).`
                  : `Git identity of "${selected.name}" deleted — the SSH side is untouched (state: incomplete).`
              }
              confirmLabel="Delete"
              onCancel={toDetail}
              onDone={() => {
                const deletedName = selected.name;
                dispatch({
                  type: 'delete-identity',
                  name: deletedName,
                  scope: pane.scope,
                  backup: newBackupPath(pane.scope === 'everything' ? '~/.ssh/config' : '~/.gitconfig'),
                });
                notify(
                  pane.scope === 'everything'
                    ? `Identity "${deletedName}" deleted (backups kept).`
                    : `Git identity of "${deletedName}" deleted — SSH kept.`,
                );
                if (pane.scope === 'everything') {
                  const fallback = rows.find((r) => r.name !== deletedName);
                  if (fallback) setSelectedName(fallback.name);
                }
                toDetail();
              }}
            />
          )}

          {pane.kind === 'fix' && selected && fixFinding && (
            <MutationCeremony
              heading={`Fix: ${fixFinding.title}`}
              targets={[planFor(fixFinding).file]}
              preview={<PreviewBlock diff text={planFor(fixFinding).diff} />}
              destructive={planFor(fixFinding).destructive}
              backups={[newBackupPath(planFor(fixFinding).file)]}
              resultMessage={planFor(fixFinding).result}
              confirmLabel="Apply fix"
              onCancel={toDetail}
              onDone={() => {
                dispatch({ type: 'fix-finding', id: fixFinding.id, backup: newBackupPath(planFor(fixFinding).file) });
                notify(planFor(fixFinding).result);
                toDetail();
              }}
            />
          )}
        </Box>
      </Stack>

      {/* delete scope dialog — safer option default-focused (§5) */}
      <Dialog open={deleteAsk} onClose={() => setDeleteAsk(false)} fullWidth>
        <DialogTitle>Delete “{selected?.name}” — choose scope</DialogTitle>
        <DialogContent>
          <RadioGroup value={deleteScope} onChange={(e) => setDeleteScope(e.target.value as 'everything' | 'git-only')}>
            <FormControlLabel value="git-only" control={<Radio />} label={`${identityManagerDeleteChoices.gitOnly} (safer — SSH stays)`} />
            <FormControlLabel value="everything" control={<Radio />} label={`${identityManagerDeleteChoices.everything} — irreversible`} sx={{ color: roles.error.color }} />
          </RadioGroup>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setDeleteAsk(false)} autoFocus>
            Esc · Cancel
          </Button>
          <Button
            color="error"
            variant="outlined"
            onClick={() => {
              setDeleteAsk(false);
              setPane({ kind: 'delete', scope: deleteScope });
            }}
          >
            Continue
          </Button>
        </DialogActions>
      </Dialog>
    </Frame>
  );
}

export default Identities;
