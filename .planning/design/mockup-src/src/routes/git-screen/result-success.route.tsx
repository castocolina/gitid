import { Alert, Box, Paper, Typography } from '@mui/material';
import Shell from '../../shell/Shell';
import type { RouteModule } from '../../App';
import {
  gitScreenAllowedSignersBackupPath,
  gitScreenConfirmTargets,
  gitScreenResultSuccessMessage,
  sampleGitconfigBackupPath,
} from '../../data/recipeFixtures';

/**
 * git-screen / result-success — mutation-ceremony beat 4 (§5): states what
 * changed, in which files, and how to restore (both backup paths again).
 * Success is a green ✓, never color alone.
 */
function ResultSuccessScreen() {
  return (
    <Shell
      title="git-screen/result-success"
      headerContext={{ identityCount: 1, health: 'healthy' }}
      statusMessage="Git identity configured successfully."
      statusTone="healthy"
      keybarEntries={[]}
    >
      <Typography variant="h6" component="h1" gutterBottom sx={{ color: 'success.main' }}>
        ✓ Git identity configured
      </Typography>
      <Paper variant="outlined" sx={{ p: 2, maxWidth: 640 }}>
        <Typography sx={{ color: 'success.main', fontWeight: 700, mb: 1 }}>
          ✓ {gitScreenResultSuccessMessage}
        </Typography>
        <Typography sx={{ color: 'text.secondary' }}>
          Written to <code>{gitScreenConfirmTargets.fragmentFile}</code>, appended to{' '}
          <code>{gitScreenConfirmTargets.gitconfigFile}</code> and{' '}
          <code>{gitScreenConfirmTargets.allowedSignersFile}</code>.
        </Typography>
      </Paper>
      <Alert severity="info" sx={{ mt: 2, maxWidth: 640, borderRadius: 0 }}>
        To restore the previous state by hand, the backups are at{' '}
        <Box component="code">{sampleGitconfigBackupPath}</Box> and{' '}
        <Box component="code">{gitScreenAllowedSignersBackupPath}</Box>.
      </Alert>
    </Shell>
  );
}

const route: RouteModule = {
  path: '/git-screen/result-success',
  element: <ResultSuccessScreen />,
  title: 'git-screen/result-success',
};

export default route;
