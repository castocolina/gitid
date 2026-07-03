/**
 * Interactive identity detail — SSH-first (MGR-03/07): SSH section first,
 * Git section says "not configured" honestly, per-identity health slice
 * comes from the SAME findings the Health surface shows. Actions: action
 * menu (a), configure Git (g), clone (c), new key (k), delete (d) — delete
 * walks choice → typed destructive confirm → backup → result and really
 * removes the row.
 */

import { useCallback, useEffect, useState } from 'react';
import {
  Alert,
  Button,
  Chip,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControlLabel,
  List,
  ListItemButton,
  ListItemText,
  Paper,
  Radio,
  RadioGroup,
  Stack,
  TextField,
  Typography,
} from '@mui/material';
import {
  healthSeverityGlyph,
  identityManagerDeleteChoices,
  identityManagerStateGlyph,
  identityManagerStateTone,
} from '../../data/recipeFixtures';
import { LiveShell, useDemo, useLocalKeys } from '../DemoContext';
import MutationCeremony, { PreviewBlock } from '../MutationCeremony';
import { findingsFor, newBackupPath } from '../store';

const toneColor: Record<'success' | 'warning' | 'error', string> = {
  success: '#4caf50',
  warning: '#d4b106',
  error: '#e05252',
};

type Action = 'none' | 'menu' | 'clone' | 'delete-choice' | 'delete-ceremony' | 'new-key';

export function IdentityDetail({ name, initialAction }: { name: string; initialAction?: string }) {
  const { state, dispatch, go, back, notify } = useDemo();
  const identity = state.identities.find((row) => row.name === name);

  const [action, setAction] = useState<Action>(
    initialAction === 'menu' || initialAction === 'clone' ? (initialAction as Action) : 'none',
  );
  const [deleteScope, setDeleteScope] = useState<'everything' | 'git-only'>('git-only');
  const [cloneName, setCloneName] = useState(`${name}-clone`);

  // `d` straight from the list lands on the delete CHOICE, never the ceremony.
  useEffect(() => {
    if (initialAction === 'delete') setAction('delete-choice');
  }, [initialAction]);

  useLocalKeys(
    useCallback(
      (key) => {
        if (action !== 'none') {
          if (key === 'Escape') {
            setAction(action === 'delete-ceremony' ? 'delete-choice' : 'none');
            return true;
          }
          return false; // dialogs/ceremony own their remaining keys
        }
        if (key === 'g') {
          // THIS identity, not the global fallback (first identity).
          go({ surface: 'git-screen', params: { name } });
          return true;
        }
        if (key === 'a') {
          setAction('menu');
          return true;
        }
        if (key === 'c') {
          setAction('clone');
          return true;
        }
        if (key === 'd') {
          setAction('delete-choice');
          return true;
        }
        if (key === 'k') {
          setAction('new-key');
          return true;
        }
        if (key === 'h') {
          go({ surface: 'health' });
          return true;
        }
        return false;
      },
      [action, go],
    ),
  );

  if (!identity) {
    // Deleted mid-view (e.g. after the delete ceremony) — the caller
    // already navigated back; render a harmless placeholder.
    return (
      <LiveShell title="identity-manager/list-empty" statusMessage={`Identity "${name}" no longer exists.`}>
        <Button variant="outlined" onClick={back}>
          Back (Esc)
        </Button>
      </LiveShell>
    );
  }

  const findings = findingsFor(state, name);
  const tone = identityManagerStateTone[identity.state];

  return (
    <LiveShell
      title="identity-manager/detail-ssh-first"
      statusMessage={`Identity "${name}" — ${identity.note}`}
      statusTone={tone === 'success' ? 'info' : tone}
      keybarEntries={[
        { key: 'g', label: 'configure/edit Git', onActivate: () => go({ surface: 'git-screen', params: { name } }) },
        { key: 'a', label: 'action menu', onActivate: () => setAction('menu') },
        { key: 'c', label: 'clone', onActivate: () => setAction('clone') },
        { key: 'k', label: 'new key', onActivate: () => setAction('new-key') },
        { key: 'd', label: 'delete', onActivate: () => setAction('delete-choice') },
        { key: 'h', label: 'health', onActivate: () => go({ surface: 'health' }) },
      ]}
    >
      <Stack direction="row" spacing={1} alignItems="center" sx={{ mb: 2 }}>
        <Typography variant="h6" component="h1">
          {identity.name}
        </Typography>
        <Chip
          size="small"
          variant="outlined"
          label={`${identityManagerStateGlyph[identity.state]} ${identity.state}`}
          sx={{ borderRadius: 0, fontFamily: 'inherit', color: toneColor[tone], borderColor: toneColor[tone] }}
        />
      </Stack>

      <Stack direction={{ xs: 'column', md: 'row' }} spacing={3}>
        <Paper variant="outlined" sx={{ flex: 1, p: 2 }}>
          <Typography variant="subtitle2" sx={{ color: 'text.secondary', mb: 1 }}>
            SSH — shown first, always
          </Typography>
          {identity.sshHost ? (
            <Stack spacing={0.5}>
              <Typography>Host alias: {identity.sshHost}</Typography>
              <Typography>Hostname: ssh.github.com · Port 443 · User git</Typography>
              <Typography>IdentityFile: {identity.keyPath ?? '— missing'}</Typography>
              <Typography>IdentitiesOnly: yes</Typography>
            </Stack>
          ) : (
            <Typography sx={{ color: '#d4b106' }}>
              ! No gitid-managed Host block — this identity relies on the global SSH config.
            </Typography>
          )}

          <Typography variant="subtitle2" sx={{ color: 'text.secondary', mt: 2, mb: 1 }}>
            Git
          </Typography>
          {identity.gitFragmentPath ? (
            <Stack spacing={0.5}>
              <Typography>Fragment: {identity.gitFragmentPath}</Typography>
              <Typography>
                Author: {identity.gitName ?? '—'} &lt;{identity.gitEmail ?? '—'}&gt;
              </Typography>
              <Typography>Signing: gpg.format=ssh, signingkey {identity.keyPath ?? '?'}.pub</Typography>
              {identity.matchStrategy && <Typography>Match strategy: {identity.matchStrategy}</Typography>}
            </Stack>
          ) : (
            <Typography sx={{ color: '#d4b106' }}>
              ! Git not configured for this alias — press g to configure it now (no fabricated
              values are ever shown here).
            </Typography>
          )}
        </Paper>

        <Paper variant="outlined" sx={{ flex: 1, p: 2 }}>
          <Typography variant="subtitle2" sx={{ color: 'text.secondary', mb: 1 }}>
            Health for this identity — same findings the doctor shows (key 4)
          </Typography>
          {findings.length === 0 ? (
            <Typography sx={{ color: '#4caf50' }}>✓ No findings for “{name}”.</Typography>
          ) : (
            <List disablePadding>
              {findings.map((f) => (
                <ListItemButton key={f.id} onClick={() => go({ surface: 'health' })} sx={{ borderBottom: 1, borderColor: 'divider' }}>
                  <ListItemText
                    primary={`${healthSeverityGlyph[f.severity]} ${f.severity} · ${f.title}`}
                    secondary={f.suggestedFix ?? f.explanation}
                  />
                </ListItemButton>
              ))}
            </List>
          )}
          {findings.some((f) => f.suggestedFix) && (
            <Button variant="outlined" size="small" sx={{ mt: 1 }} onClick={() => go({ surface: 'fixer' })}>
              5 · Open Fixer
            </Button>
          )}
        </Paper>
      </Stack>

      {/* -------- action menu (a) -------- */}
      <Dialog open={action === 'menu'} onClose={() => setAction('none')}>
        <DialogTitle>Actions — {name}</DialogTitle>
        <DialogContent>
          <List>
            <ListItemButton onClick={() => go({ surface: 'git-screen', params: { name } })}>
              <ListItemText primary="g · Configure / edit Git identity" />
            </ListItemButton>
            <ListItemButton onClick={() => setAction('clone')}>
              <ListItemText primary="c · Clone (new key + own Host block, same Git author)" />
            </ListItemButton>
            <ListItemButton onClick={() => setAction('new-key')}>
              <ListItemText primary="k · Generate a new key for this identity" />
            </ListItemButton>
            <ListItemButton onClick={() => setAction('delete-choice')}>
              <ListItemText primary="d · Delete…" sx={{ color: '#e05252' }} />
            </ListItemButton>
          </List>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setAction('none')}>Esc · Close</Button>
        </DialogActions>
      </Dialog>

      {/* -------- clone (c) -------- */}
      <Dialog open={action === 'clone'} onClose={() => setAction('none')} fullWidth>
        <DialogTitle>Clone “{name}”</DialogTitle>
        <DialogContent>
          <Typography sx={{ mb: 2, color: 'text.secondary' }}>
            The clone gets its own new key and Host alias; the Git author is copied. The suggested
            name is never a bare duplicate (MGR-04).
          </Typography>
          <TextField
            fullWidth
            autoFocus
            label="New identity name"
            value={cloneName}
            onChange={(e) => setCloneName(e.target.value)}
            error={state.identities.some((row) => row.name === cloneName) || cloneName.trim() === ''}
            helperText={
              state.identities.some((row) => row.name === cloneName)
                ? 'That name already exists.'
                : `Creates ${cloneName}.github.com + ~/.ssh/id_ed25519_${cloneName}`
            }
          />
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setAction('none')}>Esc · Cancel</Button>
          <Button
            variant="contained"
            disabled={state.identities.some((row) => row.name === cloneName) || cloneName.trim() === ''}
            onClick={() => {
              dispatch({ type: 'clone-identity', source: name, cloneName });
              notify(`Identity "${cloneName}" cloned from "${name}".`);
              setAction('none');
              go({ surface: 'identity', params: { name: cloneName } });
            }}
          >
            Clone
          </Button>
        </DialogActions>
      </Dialog>

      {/* -------- new key (k) -------- */}
      <Dialog open={action === 'new-key'} onClose={() => setAction('none')} fullWidth>
        <DialogTitle>New key for “{name}”</DialogTitle>
        <DialogContent>
          <Alert severity="info" variant="outlined" sx={{ borderRadius: 0, mb: 1 }}>
            Generates a fresh ed25519 key at ~/.ssh/id_ed25519_{name} and re-points the Host block
            at it. The old key file is backed up, never silently deleted.
          </Alert>
          {identity.state === 'key-missing' && (
            <Typography sx={{ color: '#4caf50' }}>
              This also heals the current “key-missing” state.
            </Typography>
          )}
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setAction('none')}>Esc · Cancel</Button>
          <Button
            variant="contained"
            onClick={() => {
              dispatch({ type: 'new-key', name, backup: newBackupPath(`~/.ssh/id_ed25519_${name}`) });
              notify(`New ed25519 key generated for "${name}".`);
              setAction('none');
            }}
          >
            Generate key
          </Button>
        </DialogActions>
      </Dialog>

      {/* -------- delete choice (d) — safer option default-focused -------- */}
      <Dialog open={action === 'delete-choice'} onClose={() => setAction('none')} fullWidth>
        <DialogTitle>Delete “{name}” — choose scope</DialogTitle>
        <DialogContent>
          <RadioGroup value={deleteScope} onChange={(e) => setDeleteScope(e.target.value as 'everything' | 'git-only')}>
            <FormControlLabel value="git-only" control={<Radio />} label={`${identityManagerDeleteChoices.gitOnly} (safer — SSH stays)`} />
            <FormControlLabel value="everything" control={<Radio />} label={`${identityManagerDeleteChoices.everything} — irreversible`} sx={{ color: '#e05252' }} />
          </RadioGroup>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setAction('none')} autoFocus>
            Esc · Cancel
          </Button>
          <Button color="error" variant="outlined" onClick={() => setAction('delete-ceremony')}>
            Continue
          </Button>
        </DialogActions>
      </Dialog>

      {/* -------- delete ceremony -------- */}
      {action === 'delete-ceremony' && (
        <Paper sx={{ mt: 3 }} elevation={0}>
          <MutationCeremony
            heading={
              deleteScope === 'everything'
                ? `Delete EVERYTHING for "${name}" (SSH + Git + key)`
                : `Delete the Git identity of "${name}" (SSH stays)`
            }
            targets={
              deleteScope === 'everything'
                ? ['~/.ssh/config', '~/.gitconfig', identity.gitFragmentPath ?? `~/.gitconfig.d/${name}`, identity.keyPath ?? `~/.ssh/id_ed25519_${name}`]
                : ['~/.gitconfig', identity.gitFragmentPath ?? `~/.gitconfig.d/${name}`, '~/.ssh/allowed_signers']
            }
            preview={
              <PreviewBlock
                diff
                text={
                  deleteScope === 'everything'
                    ? `- Host ${identity.sshHost ?? `${name}.github.com`} (managed block removed)\n- [includeIf] → ${identity.gitFragmentPath ?? '—'} (removed)\n- ${identity.keyPath ?? '—'} (key file removed)`
                    : `- [includeIf] → ${identity.gitFragmentPath ?? '—'} (removed)\n- ${identity.gitFragmentPath ?? '—'} (fragment removed)\n  Host ${identity.sshHost ?? '—'} (unchanged)`
                }
              />
            }
            destructive={
              deleteScope === 'everything'
                ? {
                    confirmWord: name,
                    warning: `This removes the key file too — it cannot be regenerated. Type the identity name "${name}" to confirm.`,
                  }
                : undefined
            }
            backups={[newBackupPath('~/.ssh/config'), newBackupPath('~/.gitconfig')]}
            resultMessage={
              deleteScope === 'everything'
                ? `Identity "${name}" deleted — SSH block, Git fragment, and key removed (backups kept).`
                : `Git identity of "${name}" deleted — the SSH side is untouched (state: incomplete).`
            }
            confirmLabel="Delete"
            onCancel={() => setAction('delete-choice')}
            onDone={() => {
              dispatch({
                type: 'delete-identity',
                name,
                scope: deleteScope,
                backup: newBackupPath(deleteScope === 'everything' ? '~/.ssh/config' : '~/.gitconfig'),
              });
              notify(
                deleteScope === 'everything'
                  ? `Identity "${name}" deleted (backups kept).`
                  : `Git identity of "${name}" deleted — SSH kept.`,
              );
              setAction('none');
              if (deleteScope === 'everything') back();
            }}
          />
        </Paper>
      )}
    </LiveShell>
  );
}

export default IdentityDetail;
