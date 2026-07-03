import { Alert, Box, Paper, Typography } from '@mui/material';
import Shell from '../../shell/Shell';
import type { RouteModule } from '../../App';
import { globalGitResultMessage, globalGitTargetFile, sampleGitconfigBackupPath } from '../../data/recipeFixtures';

/**
 * global-git / result-applied — mutation-ceremony beat 4 (§5): states what
 * changed, in which file, and how to restore (the backup path again).
 * Success is green ✓, never color alone. Restates that global user.email
 * was left alone — not a user decline, a structural gitid rule (each
 * identity's commits come from its own includeIf fragment).
 */
function ResultAppliedScreen() {
  return (
    <Shell
      title="global-git/result-applied"
      headerContext={{ identityCount: 8, health: 'healthy' }}
      statusMessage="Global Git options updated."
      statusTone="healthy"
      keybarEntries={[]}
    >
      <Typography variant="h6" component="h1" gutterBottom sx={{ color: 'success.main' }}>
        ✓ Options applied
      </Typography>
      <Paper variant="outlined" sx={{ p: 2, maxWidth: 640 }}>
        <Typography sx={{ color: 'success.main', fontWeight: 700, mb: 1 }}>
          ✓ {globalGitResultMessage}
        </Typography>
        <Typography sx={{ color: 'text.secondary' }}>
          Written to the managed block in <code>{globalGitTargetFile}</code>. Everything outside
          the sentinels — including any hand-written <code>[user]</code>, <code>[includeIf]</code>,
          or <code>[url]</code> sections — was preserved verbatim.
        </Typography>
      </Paper>
      <Alert severity="info" sx={{ mt: 2, maxWidth: 640, borderRadius: 0 }}>
        To restore the previous state by hand, the backup is at{' '}
        <Box component="code">{sampleGitconfigBackupPath}</Box>.
      </Alert>
    </Shell>
  );
}

const route: RouteModule = {
  path: '/global-git/result-applied',
  element: <ResultAppliedScreen />,
  title: 'global-git/result-applied',
};

export default route;
