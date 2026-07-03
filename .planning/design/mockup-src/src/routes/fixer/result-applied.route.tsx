import { Alert, Paper, Typography } from '@mui/material';
import Shell from '../../shell/Shell';
import type { RouteModule } from '../../App';
import { fixerBackupPath, fixerResultMessage, fixerTargetFile } from '../../data/recipeFixtures';

/**
 * fixer / result-applied — mutation-ceremony beat 4 (§5): states what
 * changed, in which file, and how to restore (the backup path again).
 * Success is green ✓, never color alone.
 */
function ResultAppliedScreen() {
  return (
    <Shell
      title="fixer/result-applied"
      headerContext={{ identityCount: 8, health: 'healthy' }}
      statusMessage="Fix applied."
      statusTone="healthy"
      keybarEntries={[]}
    >
      <Typography variant="h6" component="h1" gutterBottom sx={{ color: 'success.main' }}>
        ✓ Fix applied
      </Typography>
      <Paper variant="outlined" sx={{ p: 2, maxWidth: 640 }}>
        <Typography sx={{ color: 'success.main', fontWeight: 700, mb: 1 }}>
          ✓ {fixerResultMessage}
        </Typography>
        <Typography sx={{ color: 'text.secondary' }}>
          Only the rewritten directive changed — the rest of {fixerTargetFile} was preserved
          verbatim.
        </Typography>
      </Paper>
      <Alert severity="info" sx={{ mt: 2, maxWidth: 640, borderRadius: 0 }}>
        To restore the previous state by hand, the backup is at <code>{fixerBackupPath}</code>.
      </Alert>
    </Shell>
  );
}

const route: RouteModule = {
  path: '/fixer/result-applied',
  element: <ResultAppliedScreen />,
  title: 'fixer/result-applied',
};

export default route;
