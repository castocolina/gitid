import { Alert, Box, Paper, Typography } from '@mui/material';
import Shell from '../../shell/Shell';
import type { RouteModule } from '../../App';
import { sshTestFailCommand, sshTestFailOutput } from '../../data/recipeFixtures';

/**
 * create-flow / test-fail — TEST-01's error state: the connection test
 * failed. Red ✗ + word (never color alone), and the live config was still
 * never touched — only the throwaway temp config was exercised.
 */
function TestFailScreen() {
  return (
    <Shell
      title="create-flow/test-fail"
      headerContext={{ identityCount: 0, health: 'healthy' }}
      statusMessage="Connection test failed."
      statusTone="error"
      keybarEntries={[]}
    >
      <Typography variant="h6" component="h1" gutterBottom>
        4. Test connectivity — failed
      </Typography>
      <Paper variant="outlined" sx={{ p: 2, maxWidth: 720, borderColor: 'error.main' }}>
        <Typography variant="subtitle2" sx={{ color: 'text.secondary', mb: 1 }}>
          Command run
        </Typography>
        <Box
          component="pre"
          sx={{ m: 0, mb: 2, fontFamily: 'inherit', fontSize: 13, whiteSpace: 'pre-wrap' }}
        >
          $ {sshTestFailCommand}
        </Box>
        <Typography variant="subtitle2" sx={{ color: 'text.secondary', mb: 1 }}>
          Output
        </Typography>
        <Box component="pre" sx={{ m: 0, color: 'error.main', fontFamily: 'inherit', fontSize: 13 }}>
          ✗ {sshTestFailOutput}
        </Box>
      </Paper>
      <Alert severity="error" sx={{ mt: 2, maxWidth: 720, borderRadius: 0 }}>
        The key was not accepted. Nothing was written — this test ran only against the throwaway
        temp config. Go back and check the key, alias, or algorithm choice, then try again.
      </Alert>
    </Shell>
  );
}

const route: RouteModule = {
  path: '/create-flow/test-fail',
  element: <TestFailScreen />,
  title: 'create-flow/test-fail',
};

export default route;
