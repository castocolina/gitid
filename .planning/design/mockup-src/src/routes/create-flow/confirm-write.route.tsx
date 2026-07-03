import { Alert, Box, Paper, Stack, Typography } from '@mui/material';
import Shell from '../../shell/Shell';
import type { RouteModule } from '../../App';
import { confirmWriteTargetFile, createFlowManagedBlockText } from '../../data/recipeFixtures';

/**
 * create-flow / confirm-write — mutation-ceremony beats 1+2 (§5): a
 * read-only preview of the EXACT resulting config text, the target file
 * path named, the `# BEGIN/END gitid managed:` sentinels visible, and an
 * explicit confirm keystroke. Nothing has changed yet — the copy says so.
 */
function ConfirmWriteScreen() {
  return (
    <Shell
      title="create-flow/confirm-write"
      headerContext={{ identityCount: 0, health: 'healthy' }}
      statusMessage="Nothing has changed yet — review before confirming."
      statusTone="warning"
      keybarEntries={[{ key: 'y', label: 'Yes, write' }]}
    >
      <Typography variant="h6" component="h1" gutterBottom>
        5. Confirm write
      </Typography>
      <Stack spacing={2} sx={{ maxWidth: 640 }}>
        <Alert severity="warning" sx={{ borderRadius: 0 }}>
          Nothing has changed yet. Review the exact block below, then confirm to write it to{' '}
          <code>{confirmWriteTargetFile}</code>.
        </Alert>
        <Paper variant="outlined" sx={{ p: 2 }}>
          <Typography variant="subtitle2" sx={{ color: 'text.secondary', mb: 1 }}>
            Will write to {confirmWriteTargetFile}
          </Typography>
          <Box component="pre" sx={{ m: 0, fontFamily: 'inherit', fontSize: 13 }}>
            {createFlowManagedBlockText}
          </Box>
        </Paper>
        <Typography sx={{ color: 'text.secondary' }}>
          gitid only owns the block between the sentinels above — everything else in{' '}
          <code>{confirmWriteTargetFile}</code> is preserved verbatim.
        </Typography>
      </Stack>
    </Shell>
  );
}

const route: RouteModule = {
  path: '/create-flow/confirm-write',
  element: <ConfirmWriteScreen />,
  title: 'create-flow/confirm-write',
};

export default route;
