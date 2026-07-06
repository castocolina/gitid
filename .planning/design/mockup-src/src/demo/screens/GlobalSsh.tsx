/**
 * Global SSH view (02-REDESIGN-SPEC.md §4) — sub-tabs:
 *   [Options]           GSSH-01 master-detail with per-row apply checkboxes;
 *                       advisory, never blocking; Apply selected → ceremony.
 *   [Storage & preview] STORE-01 dual strategy: sentinel block in
 *                       ~/.ssh/config vs gitid-owned ~/.ssh/config.d/gitid.config
 *                       via ONE `Include` line near the top — with the
 *                       resulting config rendered per strategy; switching
 *                       layouts walks the ceremony (STORE-03: migration is a
 *                       backed-up write).
 */

import { useCallback, useMemo, useState } from 'react';
import {
  Alert,
  Box,
  Button,
  Checkbox,
  Chip,
  FormControlLabel,
  List,
  ListItemButton,
  Paper,
  Radio,
  RadioGroup,
  Stack,
  Typography,
} from '@mui/material';
import {
  globalSshAdvisoryNote,
  globalSshDetailExplanation,
  globalSshOptions,
  managedBlockSentinels,
} from '../../data/recipeFixtures';
import { roles, semanticColors } from '../../theme';
import Frame, { CEREMONY_FOOTER_ACTIONS, type FrameAction } from '../Frame';
import { useDemo, useLocalKeys } from '../DemoContext';
import MutationCeremony, { PreviewBlock } from '../MutationCeremony';
import { newBackupPath, type SshStorageLayout } from '../store';

type SubTab = 'options' | 'storage';
type Mode = 'browse' | 'apply-ceremony' | 'storage-ceremony';

function managedHostStar(applied: string[]): string {
  const sentinels = managedBlockSentinels('global-ssh');
  const lines = applied
    .map((k) => {
      const o = globalSshOptions.find((x) => x.key === k);
      return o ? `    ${o.key} ${o.recommendedValue}` : '';
    })
    .filter(Boolean)
    .join('\n');
  return `${sentinels.begin}\nIgnoreUnknown UseKeychain\n\nHost *\n${lines ? `${lines}\n` : ''}    UseKeychain yes\n    AddKeysToAgent yes\n${sentinels.end}`;
}

export function GlobalSsh() {
  const { state, dispatch, setTab, notify } = useDemo();
  const [subTab, setSubTab] = useState<SubTab>('options');
  const [mode, setMode] = useState<Mode>('browse');
  const [detailKey, setDetailKey] = useState('IdentitiesOnly');
  const [chosen, setChosen] = useState<string[]>(
    globalSshOptions.filter((o) => o.needsAction && o.key !== 'ForwardAgent').map((o) => o.key),
  );
  const [storageChoice, setStorageChoice] = useState<SshStorageLayout>(state.sshStorage);

  const options = useMemo(
    () =>
      globalSshOptions.map((o) =>
        state.sshApplied.includes(o.key)
          ? { ...o, currentValue: o.recommendedValue, needsAction: false, oneLiner: `Applied by gitid — ${o.oneLiner}` }
          : o,
      ),
    [state.sshApplied],
  );
  const pending = options.filter((o) => o.needsAction);
  const detail = options.find((o) => o.key === detailKey) ?? options[0];
  const detailIdx = options.findIndex((o) => o.key === detail?.key);
  const applyChosen = chosen.filter((k) => pending.some((o) => o.key === k));
  const sshFindings = state.findings.filter((f) => f.section === 'SSH');

  useLocalKeys(
    useCallback(
      (key) => {
        if (mode !== 'browse') return false; // ceremony owns keys
        if (key === 'ArrowDown' || key === 'ArrowUp') {
          if (subTab !== 'options') {
            // review-findings F2(f): the Storage sub-tab footer now
            // advertises `↑↓ layout` (mirroring globalssh.go's storage
            // `↑↓`) — wire the actual toggle so the advertised affordance
            // is real, not just copy.
            setStorageChoice((s) => (s === 'sentinel' ? 'include' : 'sentinel'));
            return true;
          }
          const next = options[key === 'ArrowDown' ? Math.min(detailIdx + 1, options.length - 1) : Math.max(detailIdx - 1, 0)];
          if (next) setDetailKey(next.key);
          return true;
        }
        if (key === 'ArrowLeft' || key === 'ArrowRight') {
          setSubTab((t) => (t === 'options' ? 'storage' : 'options'));
          return true;
        }
        if (key === ' ' && subTab === 'options') {
          // review-findings F2(e): the Options footer now advertises
          // `space toggle` (mirroring globalssh.go's `space` binding) —
          // wire the actual toggle on the selected row so the advertised
          // affordance is real, not just copy.
          const current = options[detailIdx];
          if (current?.needsAction) {
            setChosen((c) => (c.includes(current.key) ? c.filter((k) => k !== current.key) : [...c, current.key]));
          }
          return true;
        }
        return false;
      },
      [mode, subTab, options, detailIdx],
    ),
  );

  const includePreviewMain = `# ~/.ssh/config (top of file)\nInclude ~/.ssh/config.d/gitid.config\n\n# …everything else in your config, untouched…`;
  const includePreviewOwned = `# ~/.ssh/config.d/gitid.config (gitid-owned file)\nHost personal.github.com\n    Hostname ssh.github.com\n    Port 443\n    User git\n    IdentityFile ~/.ssh/id_ed25519_personal\n    IdentitiesOnly yes\n\n${managedHostStar(state.sshApplied)}`;
  const sentinelPreview = `# ~/.ssh/config — gitid blocks live in place, sentinel-delimited\n\nHost personal.github.com\n    Hostname ssh.github.com\n    Port 443\n    User git\n    IdentityFile ~/.ssh/id_ed25519_personal\n    IdentitiesOnly yes\n\n${managedHostStar(state.sshApplied)}`;

  // review-findings F2(e)/F2(f): the Options browse footer was missing
  // `space toggle`, and the Storage sub-tab footer was missing `↑↓ layout`
  // — both present on the TUI (globalssh.go's renderOptions/renderStorage
  // FooterAction sets). The ceremony footer now mirrors the TUI's
  // ceremonyFooterActions (F2(a)) instead of a bare `Esc cancel`.
  const actions: FrameAction[] =
    mode !== 'browse'
      ? CEREMONY_FOOTER_ACTIONS
      : subTab === 'options'
        ? [
            { key: '↑↓', label: 'select option' },
            { key: '←→', label: 'Options / Storage' },
            { key: 'space', label: 'toggle' },
            ...(applyChosen.length > 0
              ? [{ key: 'a', label: `apply ${applyChosen.length} selected`, onActivate: () => setMode('apply-ceremony') }]
              : []),
          ]
        : [
            { key: '←→', label: 'Options / Storage' },
            { key: '↑↓', label: 'layout' },
            ...(storageChoice !== state.sshStorage
              ? [{ key: 'Enter', label: 'migrate layout…', onActivate: () => setMode('storage-ceremony') }]
              : []),
          ];

  return (
    <Frame
      crumbs={[subTab === 'options' ? 'Options' : 'Storage & preview']}
      statusMessage={
        pending.length > 0
          ? `${pending.length} of ${options.length} options need action — ${globalSshAdvisoryNote}`
          : 'All recommendations applied or already set. Advisory, never a compliance gate.'
      }
      statusTone={pending.length > 0 ? 'warning' : 'info'}
      actions={actions}
      // review-findings F4: dim the header nav while an apply/storage
      // ceremony owns the keys (mirrors the TUI's capturesKeys on all four
      // tabs).
      capturesKeys={mode !== 'browse'}
    >
      {/* sub-tab strip */}
      <Stack direction="row" spacing={1} sx={{ mb: 1.5 }}>
        {(['options', 'storage'] as SubTab[]).map((t) => (
          <Box
            key={t}
            component="button"
            onClick={() => setSubTab(t)}
            sx={{
              font: 'inherit',
              border: 1,
              // semanticColors.focus is the role-less focus/selection
              // surface (sub-tab selection = the TUI's styleReverse at
              // globalssh.go's subTabStrip) — a documented U2 exception,
              // deliberately DISTINCT from the main nav's activeNav accent.
              borderColor: subTab === t ? semanticColors.focus : 'divider',
              cursor: 'pointer',
              px: 1.5,
              py: 0.25,
              bgcolor: subTab === t ? semanticColors.focus : 'transparent',
              color: subTab === t ? 'background.default' : 'text.secondary',
            }}
          >
            {t === 'options' ? 'Options' : 'Storage & preview'}
          </Box>
        ))}
      </Stack>

      {sshFindings.length > 0 && mode === 'browse' && (
        <Alert
          severity="warning"
          variant="outlined"
          sx={{ mb: 1.5, borderRadius: 0 }}
          action={
            <Button color="warning" size="small" onClick={() => setTab('doctor')}>
              4 · Open Doctor
            </Button>
          }
        >
          The doctor found {sshFindings.length} SSH finding{sshFindings.length > 1 ? 's' : ''} beyond
          these global options.
        </Alert>
      )}

      {subTab === 'options' && mode === 'browse' && (
        <Stack direction="row" spacing={2}>
          <Paper variant="outlined" sx={{ width: '44%', minWidth: 360 }}>
            <List disablePadding>
              {options.map((o) => (
                <ListItemButton
                  key={o.key}
                  selected={o.key === detail?.key}
                  onClick={() => setDetailKey(o.key)}
                  sx={{ borderBottom: 1, borderColor: 'divider', py: 0.5, display: 'block' }}
                >
                  <Stack direction="row" spacing={1} alignItems="center">
                    <Checkbox
                      size="small"
                      sx={{ p: 0 }}
                      disabled={!o.needsAction}
                      checked={o.needsAction ? chosen.includes(o.key) : true}
                      onClick={(e) => e.stopPropagation()}
                      onChange={(e) =>
                        setChosen((c) => (e.target.checked ? [...c, o.key] : c.filter((k) => k !== o.key)))
                      }
                    />
                    <Box component="span" sx={{ color: o.needsAction ? roles.warning.color : roles.healthy.color }}>
                      {o.needsAction ? '!' : '✓'}
                    </Box>
                    <Box component="span" sx={{ fontWeight: 700, flex: 1 }}>
                      {o.key}
                    </Box>
                    <Chip size="small" variant="outlined" label={o.risk} sx={{ borderRadius: 0, fontFamily: 'inherit' }} />
                  </Stack>
                  <Typography noWrap sx={{ fontSize: 12, color: 'text.secondary', pl: 4 }}>
                    now: {o.currentValue} → {o.recommendedValue}
                  </Typography>
                </ListItemButton>
              ))}
            </List>
          </Paper>
          <Paper variant="outlined" sx={{ flex: 1, p: 1.5 }}>
            <Typography variant="subtitle2" sx={{ color: 'text.secondary' }}>
              {detail?.key}
            </Typography>
            <Typography sx={{ whiteSpace: 'pre-wrap', mt: 0.5, fontSize: 14 }}>
              {detail?.key === 'IdentitiesOnly' ? globalSshDetailExplanation : detail?.oneLiner}
            </Typography>
            <Alert severity="info" variant="outlined" sx={{ mt: 1.5, borderRadius: 0 }}>
              {globalSshAdvisoryNote}
            </Alert>
            {applyChosen.length > 0 && (
              <Button variant="contained" sx={{ mt: 1.5 }} onClick={() => setMode('apply-ceremony')}>
                Apply {applyChosen.length} selected…
              </Button>
            )}
          </Paper>
        </Stack>
      )}

      {subTab === 'options' && mode === 'apply-ceremony' && (
        <MutationCeremony
          heading="Write Host * managed block to ~/.ssh/config"
          targets={[state.sshStorage === 'include' ? '~/.ssh/config.d/gitid.config' : '~/.ssh/config']}
          preview={
            <PreviewBlock
              diff
              text={[
                ...applyChosen.map((k) => `+ ${k} ${globalSshOptions.find((o) => o.key === k)?.recommendedValue ?? ''}`),
                ...options.filter((o) => !o.needsAction).map((o) => `  ${o.key} ${o.recommendedValue} (already set)`),
                ...pending.filter((o) => !applyChosen.includes(o.key)).map((o) => `  ${o.key} — left unchanged (declined; advisory)`),
              ].join('\n')}
            />
          }
          backups={[newBackupPath('~/.ssh/config')]}
          resultMessage={`${applyChosen.length} of ${pending.length} recommended options applied to Host *. ${
            pending.length - applyChosen.length > 0 ? 'The rest were left unchanged, as chosen.' : ''
          }`}
          confirmLabel="Apply selected"
          onCancel={() => setMode('browse')}
          onDone={() => {
            dispatch({ type: 'apply-ssh', keys: applyChosen, backup: newBackupPath('~/.ssh/config') });
            notify(`${applyChosen.length} global SSH option${applyChosen.length === 1 ? '' : 's'} applied.`);
            setMode('browse');
          }}
        />
      )}

      {subTab === 'storage' && mode === 'browse' && (
        <Stack direction={{ xs: 'column', md: 'row' }} spacing={2}>
          <Paper variant="outlined" sx={{ p: 1.5, width: { md: '44%' } }}>
            <Typography variant="subtitle2" sx={{ color: 'text.secondary', mb: 1 }}>
              STORE-01 — where gitid-managed SSH config lives
            </Typography>
            <RadioGroup value={storageChoice} onChange={(e) => setStorageChoice(e.target.value as SshStorageLayout)}>
              <FormControlLabel
                value="sentinel"
                control={<Radio />}
                label={`Sentinel blocks in ~/.ssh/config (default)${state.sshStorage === 'sentinel' ? ' — current' : ''}`}
              />
              <FormControlLabel
                value="include"
                control={<Radio />}
                label={`gitid-owned ~/.ssh/config.d/gitid.config via one Include line${state.sshStorage === 'include' ? ' — current' : ''}`}
              />
            </RadioGroup>
            <Typography sx={{ fontSize: 12, color: 'text.secondary', mt: 1 }}>
              Include paths must be absolute or ~/.ssh-relative; the Include line goes NEAR THE TOP
              of ~/.ssh/config. Migration between layouts is backed-up and reversible (STORE-03).
            </Typography>
            {storageChoice !== state.sshStorage && (
              <Button variant="contained" sx={{ mt: 1.5 }} onClick={() => setMode('storage-ceremony')}>
                Migrate layout…
              </Button>
            )}
          </Paper>
          <Box sx={{ flex: 1, minWidth: 0 }}>
            <Typography variant="subtitle2" sx={{ color: 'text.secondary' }}>
              Resulting config — {storageChoice === 'sentinel' ? 'sentinel blocks in place' : 'Include + owned file'}
            </Typography>
            {storageChoice === 'sentinel' ? (
              <PreviewBlock text={sentinelPreview} />
            ) : (
              <>
                <PreviewBlock text={includePreviewMain} />
                <Box sx={{ mt: 1 }}>
                  <PreviewBlock text={includePreviewOwned} />
                </Box>
              </>
            )}
          </Box>
        </Stack>
      )}

      {subTab === 'storage' && mode === 'storage-ceremony' && (
        <MutationCeremony
          heading={`Migrate SSH storage layout → ${storageChoice === 'include' ? 'Include’d gitid.config' : 'sentinel blocks in ~/.ssh/config'}`}
          targets={['~/.ssh/config', '~/.ssh/config.d/gitid.config']}
          preview={
            <PreviewBlock
              diff
              text={
                storageChoice === 'include'
                  ? `+ Include ~/.ssh/config.d/gitid.config   (near the top of ~/.ssh/config)\n+ ~/.ssh/config.d/gitid.config (all gitid blocks move here)\n- # BEGIN/END gitid managed blocks removed from ~/.ssh/config\n  everything outside gitid blocks: untouched`
                  : `+ gitid blocks written back, sentinel-delimited, into ~/.ssh/config\n- Include ~/.ssh/config.d/gitid.config (line removed)\n- ~/.ssh/config.d/gitid.config (file retired)\n  everything outside gitid blocks: untouched`
              }
            />
          }
          backups={[newBackupPath('~/.ssh/config')]}
          resultMessage={`SSH storage layout migrated to ${storageChoice === 'include' ? 'the Include’d gitid-owned file' : 'in-place sentinel blocks'} — reversible via this same screen.`}
          confirmLabel="Migrate"
          onCancel={() => setMode('browse')}
          onDone={() => {
            dispatch({ type: 'set-ssh-storage', layout: storageChoice, backup: newBackupPath('~/.ssh/config') });
            notify(`SSH storage layout: ${storageChoice}.`);
            setMode('browse');
          }}
        />
      )}
    </Frame>
  );
}

export default GlobalSsh;
