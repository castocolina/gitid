import { Box, Chip, List, ListItemButton, ListItemText, Paper, Stack, Typography } from '@mui/material';
import Shell from '../../shell/Shell';
import type { RouteModule } from '../../App';
import {
  identityManagerActionTarget,
  identityManagerRows,
  identityManagerStateGlyph,
  identityManagerStateTone,
} from '../../data/recipeFixtures';

const toneColor: Record<'success' | 'warning' | 'error', string> = {
  success: '#4caf50',
  warning: '#d4b106',
  error: '#e05252',
};

/**
 * identity-manager / list-populated — the app's HOME (entry) screen: the
 * MGR-01/MGR-02 master list, one row per identity, each labeled with the
 * 8-label MGR-02 state taxonomy as glyph + WORD (never color alone, §2).
 * Master-detail archetype (§2): list left, a lightweight preview of the
 * selected identity right.
 */
function ListPopulatedScreen() {
  const selected = identityManagerActionTarget; // 'personal'

  return (
    <Shell
      title="identity-manager/list-populated"
      headerContext={{ identityCount: identityManagerRows.length, health: 'warning' }}
      statusMessage={`${identityManagerRows.length} identities — every MGR-02 state label represented.`}
      keybarEntries={[
        { key: 'v', label: 'View SSH-first detail' },
        { key: 'a', label: 'Action menu' },
        { key: 'c', label: 'Clone' },
        { key: 'd', label: 'Delete' },
        { key: 'e', label: 'First-run empty state (demo)' },
      ]}
    >
      <Typography variant="h6" component="h1" gutterBottom>
        Identities
      </Typography>
      <Stack direction="row" spacing={3}>
        <Paper variant="outlined" sx={{ flex: 1, maxWidth: 480 }}>
          <List disablePadding>
            {identityManagerRows.map((row) => (
              <ListItemButton
                key={row.name}
                selected={row.name === selected.name}
                sx={{
                  borderBottom: 1,
                  borderColor: 'divider',
                  '&:last-of-type': { borderBottom: 0 },
                }}
              >
                <ListItemText
                  primary={
                    <Stack direction="row" spacing={1} alignItems="center">
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
                    </Stack>
                  }
                  secondary={row.note}
                />
              </ListItemButton>
            ))}
          </List>
        </Paper>
        <Paper variant="outlined" sx={{ flex: 1, p: 2, minHeight: 260 }}>
          <Typography variant="subtitle2" sx={{ color: 'text.secondary', mb: 1 }}>
            Preview — {selected.name}
          </Typography>
          <Stack spacing={0.5}>
            <Typography>SSH Host: {selected.sshHost}</Typography>
            <Typography>Key: {selected.keyPath}</Typography>
            <Typography>Git fragment: {selected.gitFragmentPath}</Typography>
          </Stack>
          <Typography sx={{ color: 'text.secondary', mt: 2 }}>
            Press <code>v</code> to open the full SSH-first detail, <code>a</code> for the action
            menu (clone / new key / delete).
          </Typography>
        </Paper>
      </Stack>
    </Shell>
  );
}

const route: RouteModule = {
  path: '/identity-manager/list-populated',
  element: <ListPopulatedScreen />,
  title: 'identity-manager/list-populated',
};

export default route;
