/**
 * Interactive home — the live Identity Manager list (MGR-01/02). Every row
 * comes from demo state, so create/clone/delete/fix actions visibly change
 * this list, its state chips, and the header's global context chip.
 */

import { useCallback, useMemo, useState } from 'react';
import {
  Box,
  Button,
  Chip,
  List,
  ListItemButton,
  ListItemText,
  Paper,
  Stack,
  TextField,
  Typography,
} from '@mui/material';
import {
  identityManagerStateGlyph,
  identityManagerStateTone,
} from '../../data/recipeFixtures';
import { LiveShell, useDemo, useLocalKeys } from '../DemoContext';
import { findingsFor } from '../store';

const toneColor: Record<'success' | 'warning' | 'error', string> = {
  success: '#4caf50',
  warning: '#d4b106',
  error: '#e05252',
};

export function HomeScreen() {
  const { state, go, openPalette } = useDemo();
  const [filter, setFilter] = useState('');
  const [cursor, setCursor] = useState(0);

  const rows = useMemo(
    () =>
      state.identities.filter((row) =>
        row.name.toLowerCase().includes(filter.toLowerCase()),
      ),
    [state.identities, filter],
  );
  const selected = rows[Math.min(cursor, Math.max(rows.length - 1, 0))];

  useLocalKeys(
    useCallback(
      (key, event) => {
        if (key === 'ArrowDown' || key === 'j') {
          setCursor((c) => Math.min(c + 1, rows.length - 1));
          return true;
        }
        if (key === 'ArrowUp' || key === 'k') {
          setCursor((c) => Math.max(c - 1, 0));
          return true;
        }
        if ((key === 'Enter' || key === 'v') && selected) {
          go({ surface: 'identity', params: { name: selected.name } });
          return true;
        }
        if (key === 'a' && selected) {
          go({ surface: 'identity', params: { name: selected.name, action: 'menu' } });
          return true;
        }
        if (key === 'c' && selected) {
          go({ surface: 'identity', params: { name: selected.name, action: 'clone' } });
          return true;
        }
        if (key === 'd' && selected) {
          go({ surface: 'identity', params: { name: selected.name, action: 'delete' } });
          return true;
        }
        if (key === 'g' && selected) {
          // Configure Git for the SELECTED row, not the global fallback.
          go({ surface: 'git-screen', params: { name: selected.name } });
          return true;
        }
        if (key === '/') {
          event.preventDefault();
          document.getElementById('identity-filter')?.focus();
          return true;
        }
        return false;
      },
      [rows.length, selected, go],
    ),
  );

  const selectedFindings = selected ? findingsFor(state, selected.name) : [];

  return (
    <LiveShell
      title="identity-manager/list-populated"
      statusMessage={`${state.identities.length} identities — interactive demo: every action below really changes this dummy state.`}
      keybarEntries={[
        { key: '↑↓/jk', label: 'select' },
        { key: 'Enter/v', label: 'view detail' },
        { key: 'n', label: 'new identity' },
        { key: 'g', label: 'configure Git' },
        { key: 'a', label: 'action menu' },
        { key: 'c', label: 'clone' },
        { key: 'd', label: 'delete' },
        { key: '2..5', label: 'global-ssh / global-git / health / fixer' },
        { key: 'Ctrl+P', label: 'palette', onActivate: openPalette },
      ]}
    >
      <Stack direction="row" alignItems="baseline" spacing={2} sx={{ mb: 1 }}>
        <Typography variant="h6" component="h1">
          Identities
        </Typography>
        <TextField
          id="identity-filter"
          size="small"
          placeholder="/ filter"
          value={filter}
          onChange={(e) => {
            setFilter(e.target.value);
            setCursor(0);
          }}
          onKeyDown={(e) => {
            if (e.key === 'Escape' || e.key === 'Enter') (e.target as HTMLElement).blur();
          }}
          sx={{ width: 180 }}
        />
        <Box sx={{ flex: 1 }} />
        <Button size="small" variant="contained" onClick={() => go({ surface: 'create' })}>
          n · New identity
        </Button>
        <Button
          size="small"
          variant="outlined"
          onClick={() => selected && go({ surface: 'git-screen', params: { name: selected.name } })}
        >
          g · Configure Git
        </Button>
      </Stack>

      <Stack direction="row" spacing={3}>
        <Paper variant="outlined" sx={{ flex: 1, maxWidth: 520 }}>
          <List disablePadding>
            {rows.map((row, i) => {
              const rowFindings = findingsFor(state, row.name);
              return (
                <ListItemButton
                  key={row.name}
                  selected={i === Math.min(cursor, rows.length - 1)}
                  onClick={() => {
                    setCursor(i);
                    go({ surface: 'identity', params: { name: row.name } });
                  }}
                  sx={{ borderBottom: 1, borderColor: 'divider', '&:last-of-type': { borderBottom: 0 } }}
                >
                  <ListItemText
                    primary={
                      <Stack direction="row" spacing={1} alignItems="center" flexWrap="wrap">
                        <Box component="span" sx={{ color: toneColor[identityManagerStateTone[row.state]] }}>
                          {identityManagerStateGlyph[row.state]}
                        </Box>
                        <Box component="span" sx={{ fontWeight: 700 }}>
                          {row.name}
                        </Box>
                        <Chip
                          size="small"
                          variant="outlined"
                          label={row.state}
                          sx={{
                            borderRadius: 0,
                            fontFamily: 'inherit',
                            color: toneColor[identityManagerStateTone[row.state]],
                            borderColor: toneColor[identityManagerStateTone[row.state]],
                          }}
                        />
                        <Chip
                          size="small"
                          variant="outlined"
                          label={row.gitFragmentPath ? 'git ✓' : 'git —'}
                          sx={{ borderRadius: 0, fontFamily: 'inherit', color: 'text.secondary' }}
                        />
                        {rowFindings.length > 0 && (
                          <Chip
                            size="small"
                            variant="outlined"
                            label={`${rowFindings.length} finding${rowFindings.length > 1 ? 's' : ''}`}
                            sx={{ borderRadius: 0, fontFamily: 'inherit', color: '#d4b106', borderColor: '#d4b106' }}
                          />
                        )}
                      </Stack>
                    }
                    secondary={row.note}
                  />
                </ListItemButton>
              );
            })}
            {rows.length === 0 && (
              <Typography sx={{ p: 2, color: 'text.secondary' }}>
                No identity matches “{filter}”.
              </Typography>
            )}
          </List>
        </Paper>

        <Paper variant="outlined" sx={{ flex: 1, p: 2, minHeight: 260 }}>
          {selected ? (
            <>
              <Typography variant="subtitle2" sx={{ color: 'text.secondary', mb: 1 }}>
                Preview — {selected.name}
              </Typography>
              <Stack spacing={0.5}>
                <Typography>SSH Host: {selected.sshHost ?? '— none (git-only)'}</Typography>
                <Typography>Key: {selected.keyPath ?? '— none'}</Typography>
                <Typography>Git fragment: {selected.gitFragmentPath ?? '— not configured'}</Typography>
                {selectedFindings.length > 0 && (
                  <Typography sx={{ color: '#d4b106' }}>
                    ! {selectedFindings.length} doctor finding{selectedFindings.length > 1 ? 's' : ''} —
                    press Enter, then open its health section, or press 4.
                  </Typography>
                )}
              </Stack>
              <Typography sx={{ color: 'text.secondary', mt: 2 }}>
                Enter/v detail · a action menu · c clone · d delete — or click the row.
              </Typography>
            </>
          ) : (
            <Typography sx={{ color: 'text.secondary' }}>No identity selected.</Typography>
          )}
        </Paper>
      </Stack>
    </LiveShell>
  );
}

export default HomeScreen;
