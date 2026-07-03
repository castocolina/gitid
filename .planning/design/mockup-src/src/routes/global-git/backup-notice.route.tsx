import { Alert, Box, Paper, Typography } from '@mui/material';
import Shell from '../../shell/Shell';
import type { RouteModule } from '../../App';
import { globalGitTargetFile, sampleGitconfigBackupPath } from '../../data/recipeFixtures';

/**
 * global-git / backup-notice — mutation-ceremony beat 3 (§5): the
 * timestamped backup path for ~/.gitconfig, shown at (or immediately
 * after) confirm. The backup IS the undo story, so it is visible here, not
 * silent.
 */
function BackupNoticeScreen() {
  return (
    <Shell
      title="global-git/backup-notice"
      headerContext={{ identityCount: 8, health: 'warning' }}
      statusMessage="Backup created before write."
      statusTone="healthy"
      keybarEntries={[{ key: 'z', label: 'Continue' }]}
    >
      <Typography variant="h6" component="h1" gutterBottom>
        Backup created
      </Typography>
      <Paper variant="outlined" sx={{ p: 2, maxWidth: 640 }}>
        <Typography variant="subtitle2" sx={{ color: 'text.secondary', mb: 1 }}>
          {globalGitTargetFile} backup path
        </Typography>
        <Box component="pre" sx={{ m: 0, color: 'success.main', fontFamily: 'inherit', fontSize: 13 }}>
          ✓ {sampleGitconfigBackupPath}
        </Box>
      </Paper>
      <Alert severity="success" sx={{ mt: 2, maxWidth: 640, borderRadius: 0 }}>
        A full copy of the previous file was saved before any change was applied — this backup
        path is the undo story. Keep it if you ever need to restore the prior state by hand.
      </Alert>
    </Shell>
  );
}

const route: RouteModule = {
  path: '/global-git/backup-notice',
  element: <BackupNoticeScreen />,
  title: 'global-git/backup-notice',
};

export default route;
