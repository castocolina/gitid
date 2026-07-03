import { Alert, Box, Paper, Stack, Typography } from '@mui/material';
import Shell from '../../shell/Shell';
import type { RouteModule } from '../../App';
import { identityManagerActionTarget, identityManagerDeleteChoices } from '../../data/recipeFixtures';

/**
 * identity-manager / confirm-destructive — beat 2 of the mutation ceremony
 * (§5), specific to the "delete everything" irreversible path: the
 * STRONGEST confirm the medium allows. Destructive actions never
 * default-focus "yes" (§5) — the default-focused option here is "No".
 */
function ConfirmDestructiveScreen() {
  const target = identityManagerActionTarget; // 'personal'

  return (
    <Shell
      title="identity-manager/confirm-destructive"
      headerContext={{ identityCount: 1, health: 'healthy' }}
      statusMessage="This cannot be undone — review carefully."
      statusTone="error"
      keybarEntries={[{ key: 'y', label: 'Yes, delete everything (typed confirm)' }]}
    >
      <Typography variant="h6" component="h1" gutterBottom>
        Confirm: {identityManagerDeleteChoices.everything}
      </Typography>
      <Stack spacing={2} sx={{ maxWidth: 560 }}>
        <Alert severity="error" sx={{ borderRadius: 0 }}>
          This action is irreversible. It removes ALL of the following for{' '}
          <code>{target.name}</code>:
        </Alert>
        <Paper variant="outlined" sx={{ p: 2 }}>
          <Stack spacing={0.5}>
            <Typography>• SSH Host block ({target.sshHost})</Typography>
            <Typography>• Git configuration ({target.gitFragmentPath})</Typography>
            <Typography>• Key file ({target.keyPath})</Typography>
          </Stack>
        </Paper>
        <Box>
          <Typography sx={{ color: 'text.secondary' }}>
            Default-focused: <strong>No, cancel</strong>. To proceed, type the identity name to
            confirm — destructive actions never default to "yes" (§5).
          </Typography>
        </Box>
      </Stack>
    </Shell>
  );
}

const route: RouteModule = {
  path: '/identity-manager/confirm-destructive',
  element: <ConfirmDestructiveScreen />,
  title: 'identity-manager/confirm-destructive',
};

export default route;
