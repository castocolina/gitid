import { List, ListItemButton, ListItemText, Paper, Typography } from '@mui/material';
import Shell from '../../shell/Shell';
import type { RouteModule } from '../../App';
import { identityManagerActionTarget } from '../../data/recipeFixtures';

/**
 * identity-manager / action-menu — the hub of per-identity actions, opened
 * via `a` from the list or detail screen. `c` clones, `d` deletes (MGR-04,
 * MGR-06); "generate new key" is MGR-05, referenced here though not its own
 * named §4(3) state.
 */
function ActionMenuScreen() {
  const target = identityManagerActionTarget; // 'personal'

  return (
    <Shell
      title="identity-manager/action-menu"
      headerContext={{ identityCount: 1, health: 'healthy' }}
      statusMessage={`Actions for ${target.name}.`}
      keybarEntries={[
        { key: 'c', label: 'Clone' },
        { key: 'd', label: 'Delete' },
      ]}
    >
      <Typography variant="h6" component="h1" gutterBottom>
        Action menu — {target.name}
      </Typography>
      <Paper variant="outlined" sx={{ maxWidth: 420 }}>
        <List disablePadding>
          <ListItemButton sx={{ borderBottom: 1, borderColor: 'divider' }}>
            <ListItemText primary="View SSH-first detail" />
          </ListItemButton>
          <ListItemButton sx={{ borderBottom: 1, borderColor: 'divider' }}>
            <ListItemText primary="Clone (c)" secondary="Create a new identity from this one, under a distinct name (MGR-04)." />
          </ListItemButton>
          <ListItemButton sx={{ borderBottom: 1, borderColor: 'divider' }}>
            <ListItemText primary="Generate new key" secondary="Rotate this identity's key (MGR-05)." />
          </ListItemButton>
          <ListItemButton>
            <ListItemText primary="Delete (d)" secondary="Choose Git-identity-only, or delete everything (MGR-06)." />
          </ListItemButton>
        </List>
      </Paper>
    </Shell>
  );
}

const route: RouteModule = {
  path: '/identity-manager/action-menu',
  element: <ActionMenuScreen />,
  title: 'identity-manager/action-menu',
};

export default route;
