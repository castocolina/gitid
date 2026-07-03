import { Alert, Box, Paper, Typography } from '@mui/material';
import Shell from '../../shell/Shell';
import type { RouteModule } from '../../App';
import { globalGitFixPreviewLines, globalGitTargetFile } from '../../data/recipeFixtures';

/**
 * global-git / fix-preview — mutation-ceremony beat 1 (§5): a read-only
 * preview of the EXACT resulting additions to ~/.gitconfig's managed
 * block. Nothing has changed yet. Global user.email is explicitly called
 * out as intentionally absent — not a user decline (unlike global-ssh's
 * ForwardAgent), but a structural gitid rule: no [user] section is ever
 * written here.
 */
function FixPreviewScreen() {
  return (
    <Shell
      title="global-git/fix-preview"
      headerContext={{ identityCount: 8, health: 'warning' }}
      statusMessage="Nothing has changed yet — review before confirming."
      statusTone="warning"
      keybarEntries={[{ key: 'w', label: 'Confirm write' }]}
    >
      <Typography variant="h6" component="h1" gutterBottom>
        Preview fix
      </Typography>
      <Alert severity="warning" sx={{ mb: 2, maxWidth: 720, borderRadius: 0 }}>
        Applying 10 of 10 baseline options to the managed block in {globalGitTargetFile}.{' '}
        <code>user.email</code> is intentionally absent — gitid never writes a global{' '}
        <code>[user]</code> section.
      </Alert>
      <Paper variant="outlined" sx={{ p: 2, maxWidth: 720 }}>
        <Typography variant="subtitle2" sx={{ color: 'text.secondary', mb: 1 }}>
          Diff — managed block in {globalGitTargetFile}
        </Typography>
        <Box component="pre" sx={{ m: 0, fontFamily: 'inherit', fontSize: 13 }}>
          {globalGitFixPreviewLines.join('\n')}
        </Box>
      </Paper>
      <Typography sx={{ color: 'text.secondary', mt: 2, maxWidth: 720 }}>
        gitid only owns the block between its sentinels — everything else in {globalGitTargetFile}{' '}
        is preserved verbatim.
      </Typography>
    </Shell>
  );
}

const route: RouteModule = {
  path: '/global-git/fix-preview',
  element: <FixPreviewScreen />,
  title: 'global-git/fix-preview',
};

export default route;
