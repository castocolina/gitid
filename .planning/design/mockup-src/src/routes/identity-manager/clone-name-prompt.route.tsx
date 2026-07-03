import { Alert, Paper, Stack, TextField, Typography } from '@mui/material';
import Shell from '../../shell/Shell';
import type { RouteModule } from '../../App';
import { identityManagerActionTarget, identityManagerCloneSuggestedName } from '../../data/recipeFixtures';

/**
 * identity-manager / clone-name-prompt (MGR-04) — clone the targeted
 * identity into a DISTINCT new name; never a bare duplicate of the source
 * name.
 */
function CloneNamePromptScreen() {
  const source = identityManagerActionTarget; // 'personal'

  return (
    <Shell
      title="identity-manager/clone-name-prompt"
      headerContext={{ identityCount: 1, health: 'healthy' }}
      statusMessage={`Cloning ${source.name} — choose a distinct name.`}
      keybarEntries={[{ key: 'w', label: 'Write clone' }]}
    >
      <Typography variant="h6" component="h1" gutterBottom>
        Clone identity
      </Typography>
      <Paper variant="outlined" sx={{ p: 2, maxWidth: 480 }}>
        <Stack spacing={2}>
          <TextField label="Source identity" value={source.name} fullWidth size="small" disabled />
          <TextField
            label="New identity name"
            value={identityManagerCloneSuggestedName}
            fullWidth
            size="small"
            helperText="Must differ from the source name (MGR-04) — gitid rejects a bare duplicate."
          />
          <Alert severity="info" sx={{ borderRadius: 0 }}>
            Cloning copies the SSH Host block and Git fragment shape under the new name — the key
            material itself is not copied; a new key is generated for the clone.
          </Alert>
        </Stack>
      </Paper>
    </Shell>
  );
}

const route: RouteModule = {
  path: '/identity-manager/clone-name-prompt',
  element: <CloneNamePromptScreen />,
  title: 'identity-manager/clone-name-prompt',
};

export default route;
