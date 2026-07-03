import { Alert, Box, Chip, List, ListItemText, Paper, Stack, Typography } from '@mui/material';
import Shell from '../../shell/Shell';
import type { RouteModule } from '../../App';
import {
  identityManagerDetailTarget,
  identityManagerRows,
  identityManagerStateGlyph,
} from '../../data/recipeFixtures';

/**
 * identity-manager / detail-ssh-first — MGR-03/MGR-07's highest-value
 * proof: SSH details are shown FIRST, and Git attributes are NEVER
 * rendered for an SSH-only identity. Targets `work` (state `incomplete`,
 * SSH-only) precisely because it has no Git fragment — the Git section
 * explicitly says "not configured" rather than fabricating fields.
 */
function DetailSSHFirstScreen() {
  const target = identityManagerDetailTarget; // 'work'

  return (
    <Shell
      title="identity-manager/detail-ssh-first"
      headerContext={{ identityCount: identityManagerRows.length, health: 'warning' }}
      statusMessage={`${target.name} — SSH details shown first, per MGR-03.`}
      keybarEntries={[{ key: 'a', label: 'Action menu' }]}
    >
      <Typography variant="h6" component="h1" gutterBottom>
        Identity detail — {target.name}
      </Typography>
      <Stack direction="row" spacing={3}>
        <Paper variant="outlined" sx={{ flex: 1, maxWidth: 300, p: 1 }}>
          <List dense disablePadding>
            {identityManagerRows.map((row) => (
              <ListItemText
                key={row.name}
                sx={{ opacity: row.name === target.name ? 1 : 0.5, px: 1, py: 0.5 }}
                primary={`${identityManagerStateGlyph[row.state]} ${row.name}`}
              />
            ))}
          </List>
        </Paper>
        <Stack spacing={2} sx={{ flex: 1, maxWidth: 520 }}>
          <Paper variant="outlined" sx={{ p: 2 }}>
            <Typography variant="subtitle2" sx={{ color: 'text.secondary', mb: 1 }}>
              1. SSH (shown first)
            </Typography>
            <Stack spacing={0.5}>
              <Typography>Host: {target.sshHost}</Typography>
              <Typography>IdentityFile: {target.keyPath}</Typography>
              <Typography>IdentitiesOnly: yes</Typography>
            </Stack>
          </Paper>
          <Paper variant="outlined" sx={{ p: 2 }}>
            <Typography variant="subtitle2" sx={{ color: 'text.secondary', mb: 1 }}>
              2. Git
            </Typography>
            <Alert severity="info" sx={{ borderRadius: 0 }}>
              No Git identity configured for this alias — this identity is SSH-only. gitid never
              renders fabricated Git attributes here (MGR-03/MGR-07).
            </Alert>
          </Paper>
          <Paper variant="outlined" sx={{ p: 2 }}>
            <Typography variant="subtitle2" sx={{ color: 'text.secondary', mb: 1 }}>
              Per-identity health (MGR-07)
            </Typography>
            <Chip
              size="small"
              variant="outlined"
              label={`${identityManagerStateGlyph[target.state]} ${target.state}`}
              sx={{ borderRadius: 0, fontFamily: 'inherit' }}
            />
            <Box component="p" sx={{ color: 'text.secondary', m: 0, mt: 1 }}>
              {target.note}
            </Box>
          </Paper>
        </Stack>
      </Stack>
    </Shell>
  );
}

const route: RouteModule = {
  path: '/identity-manager/detail-ssh-first',
  element: <DetailSSHFirstScreen />,
  title: 'identity-manager/detail-ssh-first',
};

export default route;
