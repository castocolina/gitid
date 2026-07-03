import { Paper, Stack, Typography } from '@mui/material';
import Shell from '../../shell/Shell';
import type { RouteModule } from '../../App';

/**
 * identity-manager / list-empty — the TRUE first-run landing state: no
 * identities yet. Designed, not an afterthought (02-UX-DIRECTION.md §4(3),
 * §6 checklist item B) — explicit empty-state copy, never a blank list.
 */
function ListEmptyScreen() {
  return (
    <Shell
      title="identity-manager/list-empty"
      headerContext={{ identityCount: 0, health: 'healthy' }}
      statusMessage="No identities configured yet."
      keybarEntries={[{ key: 'n', label: 'Create your first identity' }]}
    >
      <Typography variant="h6" component="h1" gutterBottom>
        Identities
      </Typography>
      <Paper variant="outlined" sx={{ p: 4, maxWidth: 560, textAlign: 'left' }}>
        <Typography variant="subtitle1" sx={{ fontWeight: 700, mb: 1 }}>
          No identities yet
        </Typography>
        <Stack spacing={1}>
          <Typography sx={{ color: 'text.secondary' }}>
            gitid manages `~/.ssh/config` and `~/.gitconfig` per identity — nothing has been
            configured on this machine yet.
          </Typography>
          <Typography sx={{ color: 'text.secondary' }}>
            Press <code>n</code> to create your first identity (SSH connection + Git
            configuration, end to end).
          </Typography>
        </Stack>
      </Paper>
    </Shell>
  );
}

const route: RouteModule = {
  path: '/identity-manager/list-empty',
  element: <ListEmptyScreen />,
  title: 'identity-manager/list-empty',
};

export default route;
