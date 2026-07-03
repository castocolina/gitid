import { Alert, Box, Paper, Stack, Typography } from '@mui/material';
import Shell from '../../shell/Shell';
import type { RouteModule } from '../../App';
import { gitScreenAllowedSignersBackupPath, sampleGitconfigBackupPath } from '../../data/recipeFixtures';

/**
 * git-screen / backup-notice — mutation-ceremony beat 3 (§5): the
 * timestamped backup paths for every EXISTING file this screen mutates
 * (~/.gitconfig and ~/.ssh/allowed_signers — the new fragment file has no
 * prior version to back up). The backup IS the undo story, so it is
 * visible here, not silent.
 */
function BackupNoticeScreen() {
  return (
    <Shell
      title="git-screen/backup-notice"
      headerContext={{ identityCount: 1, health: 'healthy' }}
      statusMessage="Backups created before write."
      statusTone="healthy"
      keybarEntries={[{ key: 'z', label: 'Continue' }]}
    >
      <Typography variant="h6" component="h1" gutterBottom>
        Backups created
      </Typography>
      <Stack spacing={2} sx={{ maxWidth: 640 }}>
        <Paper variant="outlined" sx={{ p: 2 }}>
          <Typography variant="subtitle2" sx={{ color: 'text.secondary', mb: 1 }}>
            ~/.gitconfig backup path
          </Typography>
          <Box component="pre" sx={{ m: 0, color: 'success.main', fontFamily: 'inherit', fontSize: 13 }}>
            ✓ {sampleGitconfigBackupPath}
          </Box>
        </Paper>
        <Paper variant="outlined" sx={{ p: 2 }}>
          <Typography variant="subtitle2" sx={{ color: 'text.secondary', mb: 1 }}>
            ~/.ssh/allowed_signers backup path
          </Typography>
          <Box component="pre" sx={{ m: 0, color: 'success.main', fontFamily: 'inherit', fontSize: 13 }}>
            ✓ {gitScreenAllowedSignersBackupPath}
          </Box>
        </Paper>
      </Stack>
      <Alert severity="success" sx={{ mt: 2, maxWidth: 640, borderRadius: 0 }}>
        A full copy of each previous file was saved before any change was applied — these backup
        paths are the undo story. Keep them if you ever need to restore the prior state by hand.
      </Alert>
    </Shell>
  );
}

const route: RouteModule = {
  path: '/git-screen/backup-notice',
  element: <BackupNoticeScreen />,
  title: 'git-screen/backup-notice',
};

export default route;
