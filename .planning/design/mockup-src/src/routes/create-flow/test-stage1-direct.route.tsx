import { Alert, Box, Paper, Typography } from '@mui/material';
import Shell from '../../shell/Shell';
import type { RouteModule } from '../../App';
import { sshTestStage1Command, sshTestStage1Output } from '../../data/recipeFixtures';

/**
 * create-flow / test-stage1-direct — TEST-01 stage 1: test the key DIRECT
 * against the bare provider URL (no alias yet), showing the exact command
 * and its real output. SSHUI-04: runs against a throwaway temp config; the
 * live `~/.ssh/config` is untouched.
 */
function TestStage1DirectScreen() {
  return (
    <Shell
      title="create-flow/test-stage1-direct"
      headerContext={{ identityCount: 0, health: 'healthy' }}
      statusMessage="Testing against a throwaway temp config — live config untouched."
      keybarEntries={[{ key: 'a', label: 'Test by alias (ssh -G)' }]}
    >
      <Typography variant="h6" component="h1" gutterBottom>
        4. Test connectivity — stage 1: direct (provider URL, no alias)
      </Typography>
      <Paper variant="outlined" sx={{ p: 2, maxWidth: 720 }}>
        <Typography variant="subtitle2" sx={{ color: 'text.secondary', mb: 1 }}>
          Command run
        </Typography>
        <Box
          component="pre"
          sx={{ m: 0, mb: 2, fontFamily: 'inherit', fontSize: 13, whiteSpace: 'pre-wrap' }}
        >
          $ {sshTestStage1Command}
        </Box>
        <Typography variant="subtitle2" sx={{ color: 'text.secondary', mb: 1 }}>
          Output
        </Typography>
        <Box component="pre" sx={{ m: 0, color: 'success.main', fontFamily: 'inherit', fontSize: 13 }}>
          ✓ {sshTestStage1Output}
        </Box>
      </Paper>
      <Alert severity="info" sx={{ mt: 2, maxWidth: 720, borderRadius: 0 }}>
        This test runs against a throwaway temp config file — your live{' '}
        <code>~/.ssh/config</code> is untouched until you confirm the write.
      </Alert>
    </Shell>
  );
}

const route: RouteModule = {
  path: '/create-flow/test-stage1-direct',
  element: <TestStage1DirectScreen />,
  title: 'create-flow/test-stage1-direct',
};

export default route;
