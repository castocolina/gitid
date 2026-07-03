import { Alert, Box, Paper, Typography } from '@mui/material';
import Shell from '../../shell/Shell';
import type { RouteModule } from '../../App';
import { sshTestStage2Command, sshTestStage2Output } from '../../data/recipeFixtures';

/**
 * create-flow / test-stage2-by-alias — TEST-01 stage 2 + TEST-02: test
 * TARGETED by the alias, and prove via `ssh -G` which `IdentityFile` the
 * config actually resolves for that alias.
 */
function TestStage2ByAliasScreen() {
  return (
    <Shell
      title="create-flow/test-stage2-by-alias"
      headerContext={{ identityCount: 0, health: 'healthy' }}
      statusMessage="Testing by alias — ssh -G proves which IdentityFile resolves."
      keybarEntries={[
        { key: 'w', label: 'Confirm write' },
        { key: 'x', label: 'Simulate failure (demo)' },
      ]}
    >
      <Typography variant="h6" component="h1" gutterBottom>
        4. Test connectivity — stage 2: by alias (ssh -G proof)
      </Typography>
      <Paper variant="outlined" sx={{ p: 2, maxWidth: 720 }}>
        <Typography variant="subtitle2" sx={{ color: 'text.secondary', mb: 1 }}>
          Command run
        </Typography>
        <Box
          component="pre"
          sx={{ m: 0, mb: 2, fontFamily: 'inherit', fontSize: 13, whiteSpace: 'pre-wrap' }}
        >
          $ {sshTestStage2Command}
        </Box>
        <Typography variant="subtitle2" sx={{ color: 'text.secondary', mb: 1 }}>
          Output
        </Typography>
        <Box component="pre" sx={{ m: 0, color: 'success.main', fontFamily: 'inherit', fontSize: 13 }}>
          ✓ {sshTestStage2Output}
        </Box>
      </Paper>
      <Alert severity="info" sx={{ mt: 2, maxWidth: 720, borderRadius: 0 }}>
        <code>ssh -G</code> resolves the effective configuration for the alias without connecting
        — proof that <code>IdentitiesOnly yes</code> and this specific key are what will actually
        be used, still against the throwaway temp config.
      </Alert>
    </Shell>
  );
}

const route: RouteModule = {
  path: '/create-flow/test-stage2-by-alias',
  element: <TestStage2ByAliasScreen />,
  title: 'create-flow/test-stage2-by-alias',
};

export default route;
