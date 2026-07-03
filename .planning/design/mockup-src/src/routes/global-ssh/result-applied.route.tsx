import { Alert, Box, Paper, Typography } from '@mui/material';
import Shell from '../../shell/Shell';
import type { RouteModule } from '../../App';
import {
  globalSshBackupPath,
  globalSshDeclinedOption,
  globalSshResultMessage,
  globalSshTargetFile,
} from '../../data/recipeFixtures';

/**
 * global-ssh / result-applied — mutation-ceremony beat 4 (§5): states what
 * changed, in which file, and how to restore (the backup path again).
 * Success is green ✓, never color alone. Explicitly restates that
 * ForwardAgent was left unchanged — the recommendation was advisory, not
 * required, and the user's choice is respected and visible in the result.
 */
function ResultAppliedScreen() {
  return (
    <Shell
      title="global-ssh/result-applied"
      headerContext={{ identityCount: 8, health: 'healthy' }}
      statusMessage="Global SSH options updated."
      statusTone="healthy"
      keybarEntries={[]}
    >
      <Typography variant="h6" component="h1" gutterBottom sx={{ color: 'success.main' }}>
        ✓ Options applied
      </Typography>
      <Paper variant="outlined" sx={{ p: 2, maxWidth: 640 }}>
        <Typography sx={{ color: 'success.main', fontWeight: 700, mb: 1 }}>
          ✓ {globalSshResultMessage}
        </Typography>
        <Typography sx={{ color: 'text.secondary' }}>
          Written to the <code>Host *</code> block in <code>{globalSshTargetFile}</code>. You can
          revisit <code>{globalSshDeclinedOption}</code> here any time — nothing was ever required.
        </Typography>
      </Paper>
      <Alert severity="info" sx={{ mt: 2, maxWidth: 640, borderRadius: 0 }}>
        To restore the previous state by hand, the backup is at{' '}
        <Box component="code">{globalSshBackupPath}</Box>.
      </Alert>
    </Shell>
  );
}

const route: RouteModule = {
  path: '/global-ssh/result-applied',
  element: <ResultAppliedScreen />,
  title: 'global-ssh/result-applied',
};

export default route;
