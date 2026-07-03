import { Paper, Stack, Typography } from '@mui/material';
import Shell from '../../shell/Shell';
import type { RouteModule } from '../../App';
import { sshIdentityAlias } from '../../data/recipeFixtures';

/**
 * create-flow / reuse-key-vs-generate — KEY-06: create an identity that
 * reuses an existing key instead of generating a new one.
 */
function ReuseKeyVsGenerateScreen() {
  return (
    <Shell
      title="create-flow/reuse-key-vs-generate"
      headerContext={{ identityCount: 0, health: 'healthy' }}
      statusMessage="Reuse an existing key, or generate a new ed25519 key?"
      keybarEntries={[]}
    >
      <Typography variant="h6" component="h1" gutterBottom>
        3. Key source
      </Typography>
      <Stack spacing={1.5} sx={{ maxWidth: 640 }}>
        <Paper variant="outlined" sx={{ p: 1.5, borderColor: 'success.main', borderWidth: 2 }}>
          <Typography sx={{ fontWeight: 700 }}>Generate a new key</Typography>
          <Typography sx={{ color: 'text.secondary' }}>
            gitid generates a fresh {sshIdentityAlias.identityFile} using the algorithm chosen in
            step 1. Recommended for a brand-new identity.
          </Typography>
        </Paper>
        <Paper variant="outlined" sx={{ p: 1.5 }}>
          <Typography sx={{ fontWeight: 700 }}>Reuse an existing key</Typography>
          <Typography sx={{ color: 'text.secondary' }}>
            Point this identity at a key file that already exists on disk instead of generating a
            new one. Useful when re-registering an identity for an already-provisioned key.
          </Typography>
        </Paper>
      </Stack>
    </Shell>
  );
}

const route: RouteModule = {
  path: '/create-flow/reuse-key-vs-generate',
  element: <ReuseKeyVsGenerateScreen />,
  title: 'create-flow/reuse-key-vs-generate',
};

export default route;
