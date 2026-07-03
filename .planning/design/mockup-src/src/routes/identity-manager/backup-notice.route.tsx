import { Paper, Stack, Typography } from '@mui/material';
import Shell from '../../shell/Shell';
import type { RouteModule } from '../../App';
import { identityManagerActionTarget, identityManagerBackupPaths } from '../../data/recipeFixtures';

/**
 * identity-manager / backup-notice — beat 3 of the mutation ceremony (§5):
 * the timestamped backup path(s) for every file this delete touches, shown
 * before the change is finalized — the backup IS the undo story.
 */
function BackupNoticeScreen() {
  const target = identityManagerActionTarget; // 'personal'

  return (
    <Shell
      title="identity-manager/backup-notice"
      headerContext={{ identityCount: 0, health: 'healthy' }}
      statusMessage={`Backups created before removing ${target.name}.`}
      statusTone="healthy"
    >
      <Typography variant="h6" component="h1" gutterBottom>
        Backups created
      </Typography>
      <Paper variant="outlined" sx={{ p: 2, maxWidth: 520 }}>
        <Stack spacing={1}>
          <Typography sx={{ color: 'success.main' }}>
            ✓ ~/.ssh/config backup: {identityManagerBackupPaths.sshConfig}
          </Typography>
          <Typography sx={{ color: 'success.main' }}>
            ✓ ~/.gitconfig backup: {identityManagerBackupPaths.gitconfig}
          </Typography>
          <Typography sx={{ color: 'text.secondary', mt: 1 }}>
            A full copy of each previous file was saved before any change was applied — these
            backup paths are the undo story.
          </Typography>
        </Stack>
      </Paper>
    </Shell>
  );
}

const route: RouteModule = {
  path: '/identity-manager/backup-notice',
  element: <BackupNoticeScreen />,
  title: 'identity-manager/backup-notice',
};

export default route;
