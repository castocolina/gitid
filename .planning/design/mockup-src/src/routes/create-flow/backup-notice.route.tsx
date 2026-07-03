import { Alert, Box, Paper, Typography } from '@mui/material';
import Shell from '../../shell/Shell';
import type { RouteModule } from '../../App';
import { sampleBackupPath } from '../../data/recipeFixtures';

/**
 * create-flow / backup-notice — mutation-ceremony beat 3 (§5): the
 * timestamped backup path, shown at/immediately after confirm. The backup
 * IS the undo story, so it is visible here, not silent.
 */
function BackupNoticeScreen() {
  return (
    <Shell
      title="create-flow/backup-notice"
      headerContext={{ identityCount: 0, health: 'healthy' }}
      statusMessage="Backup created before write."
      statusTone="healthy"
      keybarEntries={[{ key: 'z', label: 'Continue' }]}
    >
      <Typography variant="h6" component="h1" gutterBottom>
        6. Backup created
      </Typography>
      <Paper variant="outlined" sx={{ p: 2, maxWidth: 640 }}>
        <Typography variant="subtitle2" sx={{ color: 'text.secondary', mb: 1 }}>
          Backup path
        </Typography>
        <Box component="pre" sx={{ m: 0, color: 'success.main', fontFamily: 'inherit', fontSize: 13 }}>
          ✓ {sampleBackupPath}
        </Box>
      </Paper>
      <Alert severity="success" sx={{ mt: 2, maxWidth: 640, borderRadius: 0 }}>
        A full copy of your previous config was saved before any change was applied — this backup
        path is the undo story. Keep it if you ever need to restore the prior state by hand.
      </Alert>
    </Shell>
  );
}

const route: RouteModule = {
  path: '/create-flow/backup-notice',
  element: <BackupNoticeScreen />,
  title: 'create-flow/backup-notice',
};

export default route;
