import { Alert, Box, Paper, Typography } from '@mui/material';
import Shell from '../../shell/Shell';
import type { RouteModule } from '../../App';
import {
  confirmWriteTargetFile,
  resultSuccessMessage,
  sampleBackupPath,
} from '../../data/recipeFixtures';

/**
 * create-flow / result-success — mutation-ceremony beat 4 (§5): states what
 * changed, in which file, and how to restore (the backup path again).
 * Success is a green ✓, never color alone.
 */
function ResultSuccessScreen() {
  return (
    <Shell
      title="create-flow/result-success"
      headerContext={{ identityCount: 1, health: 'healthy' }}
      statusMessage="Identity created successfully."
      statusTone="healthy"
      keybarEntries={[]}
    >
      <Typography variant="h6" component="h1" gutterBottom sx={{ color: 'success.main' }}>
        ✓ Identity created
      </Typography>
      <Paper variant="outlined" sx={{ p: 2, maxWidth: 640 }}>
        <Typography sx={{ color: 'success.main', fontWeight: 700, mb: 1 }}>
          ✓ {resultSuccessMessage}
        </Typography>
        <Typography sx={{ color: 'text.secondary' }}>
          Written to <code>{confirmWriteTargetFile}</code>.
        </Typography>
      </Paper>
      <Alert severity="info" sx={{ mt: 2, maxWidth: 640, borderRadius: 0 }}>
        To restore the previous state by hand, the backup is at{' '}
        <Box component="code">{sampleBackupPath}</Box>.
      </Alert>
    </Shell>
  );
}

const route: RouteModule = {
  path: '/create-flow/result-success',
  element: <ResultSuccessScreen />,
  title: 'create-flow/result-success',
};

export default route;
