import { Alert, Box, Paper, Typography } from '@mui/material';
import Shell from '../../shell/Shell';
import type { RouteModule } from '../../App';
import { fixerBackupPath, fixerTargetFile } from '../../data/recipeFixtures';

/**
 * fixer / backup-notice — mutation-ceremony beat 3 (§5): the timestamped
 * backup path, shown at (or immediately after) confirm. The backup IS the
 * undo story, so it is visible here, not silent — named BEFORE applying
 * (§4.7 "names the backup path before applying").
 */
function BackupNoticeScreen() {
  return (
    <Shell
      title="fixer/backup-notice"
      headerContext={{ identityCount: 8, health: 'error' }}
      statusMessage="Backup created before write."
      statusTone="healthy"
      keybarEntries={[{ key: 'z', label: 'Apply the fix' }]}
    >
      <Typography variant="h6" component="h1" gutterBottom>
        Backup created
      </Typography>
      <Paper variant="outlined" sx={{ p: 2, maxWidth: 640 }}>
        <Typography variant="subtitle2" sx={{ color: 'text.secondary', mb: 1 }}>
          {fixerTargetFile} backup path
        </Typography>
        <Box component="pre" sx={{ m: 0, color: 'success.main', fontFamily: 'inherit', fontSize: 13 }}>
          ✓ {fixerBackupPath}
        </Box>
      </Paper>
      <Alert severity="success" sx={{ mt: 2, maxWidth: 640, borderRadius: 0 }}>
        A full copy of {fixerTargetFile} was saved BEFORE any change is applied — this backup path
        is the undo story. Keep it if you ever need to restore the prior state by hand.
      </Alert>
    </Shell>
  );
}

const route: RouteModule = {
  path: '/fixer/backup-notice',
  element: <BackupNoticeScreen />,
  title: 'fixer/backup-notice',
};

export default route;
