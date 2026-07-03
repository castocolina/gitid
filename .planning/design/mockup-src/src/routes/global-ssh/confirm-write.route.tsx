import { Alert, Box, Paper, Stack, Typography } from '@mui/material';
import Shell from '../../shell/Shell';
import type { RouteModule } from '../../App';
import {
  globalSshDeclinedOption,
  globalSshManagedBlockText,
  globalSshTargetFile,
} from '../../data/recipeFixtures';

/**
 * global-ssh / confirm-write — mutation-ceremony beats 1+2 (§5): a
 * read-only preview of the EXACT resulting text, with the target file
 * named and the `# BEGIN/END gitid managed:` sentinels visible. Nothing
 * has changed yet — the copy says so. An explicit, deliberate keystroke
 * confirms.
 */
function ConfirmWriteScreen() {
  return (
    <Shell
      title="global-ssh/confirm-write"
      headerContext={{ identityCount: 8, health: 'warning' }}
      statusMessage="Nothing has changed yet — review before confirming."
      statusTone="warning"
      keybarEntries={[{ key: 'y', label: 'Yes, write' }]}
    >
      <Typography variant="h6" component="h1" gutterBottom>
        Confirm write
      </Typography>
      <Stack spacing={2} sx={{ maxWidth: 720 }}>
        <Alert severity="warning" sx={{ borderRadius: 0 }}>
          Nothing has changed yet. Review the exact text below, then confirm to write it.{' '}
          <code>{globalSshDeclinedOption}</code> is intentionally absent — left unchanged.
        </Alert>
        <Paper variant="outlined" sx={{ p: 2 }}>
          <Typography variant="subtitle2" sx={{ color: 'text.secondary', mb: 1 }}>
            Will append to {globalSshTargetFile}
          </Typography>
          <Box component="pre" sx={{ m: 0, fontFamily: 'inherit', fontSize: 13 }}>
            {globalSshManagedBlockText}
          </Box>
        </Paper>
        <Typography sx={{ color: 'text.secondary' }}>
          gitid only owns the block between the sentinels above — everything else in{' '}
          {globalSshTargetFile} is preserved verbatim.
        </Typography>
      </Stack>
    </Shell>
  );
}

const route: RouteModule = {
  path: '/global-ssh/confirm-write',
  element: <ConfirmWriteScreen />,
  title: 'global-ssh/confirm-write',
};

export default route;
