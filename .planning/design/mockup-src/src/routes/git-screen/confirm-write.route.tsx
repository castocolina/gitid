import { Alert, Box, Paper, Stack, Typography } from '@mui/material';
import Shell from '../../shell/Shell';
import type { RouteModule } from '../../App';
import {
  gitScreenConfirmTargets,
  gitScreenGitconfigIncludeBlockText,
  gitScreenManagedFragmentText,
  allowedSignersLine,
} from '../../data/recipeFixtures';

/**
 * git-screen / confirm-write — mutation-ceremony beats 1+2 (§5): a
 * read-only preview of the EXACT text for all THREE targets this screen
 * writes (GITUI-05) — the fragment file, the includeIf block appended to
 * ~/.gitconfig, and the allowed_signers line — with the target file paths
 * named and the `# BEGIN/END gitid managed:` sentinels visible. Nothing has
 * changed yet — the copy says so.
 */
function ConfirmWriteScreen() {
  return (
    <Shell
      title="git-screen/confirm-write"
      headerContext={{ identityCount: 1, health: 'healthy' }}
      statusMessage="Nothing has changed yet — review before confirming."
      statusTone="warning"
      keybarEntries={[{ key: 'y', label: 'Yes, write' }]}
    >
      <Typography variant="h6" component="h1" gutterBottom>
        Confirm write
      </Typography>
      <Stack spacing={2} sx={{ maxWidth: 720 }}>
        <Alert severity="warning" sx={{ borderRadius: 0 }}>
          Nothing has changed yet. Review the exact text for each file below, then confirm to
          write all three.
        </Alert>
        <Paper variant="outlined" sx={{ p: 2 }}>
          <Typography variant="subtitle2" sx={{ color: 'text.secondary', mb: 1 }}>
            Will write to {gitScreenConfirmTargets.fragmentFile}
          </Typography>
          <Box component="pre" sx={{ m: 0, fontFamily: 'inherit', fontSize: 13 }}>
            {gitScreenManagedFragmentText}
          </Box>
        </Paper>
        <Paper variant="outlined" sx={{ p: 2 }}>
          <Typography variant="subtitle2" sx={{ color: 'text.secondary', mb: 1 }}>
            Will append to {gitScreenConfirmTargets.gitconfigFile}
          </Typography>
          <Box component="pre" sx={{ m: 0, fontFamily: 'inherit', fontSize: 13 }}>
            {gitScreenGitconfigIncludeBlockText}
          </Box>
        </Paper>
        <Paper variant="outlined" sx={{ p: 2 }}>
          <Typography variant="subtitle2" sx={{ color: 'text.secondary', mb: 1 }}>
            Will append to {gitScreenConfirmTargets.allowedSignersFile}
          </Typography>
          <Box component="pre" sx={{ m: 0, fontFamily: 'inherit', fontSize: 13 }}>
            {allowedSignersLine}
          </Box>
        </Paper>
        <Typography sx={{ color: 'text.secondary' }}>
          gitid only owns the block between the sentinels above in each file — everything else
          is preserved verbatim.
        </Typography>
      </Stack>
    </Shell>
  );
}

const route: RouteModule = {
  path: '/git-screen/confirm-write',
  element: <ConfirmWriteScreen />,
  title: 'git-screen/confirm-write',
};

export default route;
