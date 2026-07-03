import { Alert, Box, Paper, Typography } from '@mui/material';
import Shell from '../../shell/Shell';
import type { RouteModule } from '../../App';
import {
  globalSshChosenToApply,
  globalSshDeclinedOption,
  globalSshFixPreviewLines,
  globalSshTargetFile,
} from '../../data/recipeFixtures';

/**
 * global-ssh / fix-preview — mutation-ceremony beat 1 (§5): a read-only
 * preview of the EXACT resulting diff to the `Host *` block in
 * ~/.ssh/config. Nothing has changed yet. Demonstrates the advisory
 * affordance concretely: the user applies 3 of the 4 recommended options
 * and deliberately LEAVES ForwardAgent unchanged — recommendations are
 * never required (§4.4).
 */
function FixPreviewScreen() {
  return (
    <Shell
      title="global-ssh/fix-preview"
      headerContext={{ identityCount: 8, health: 'warning' }}
      statusMessage="Nothing has changed yet — review before confirming."
      statusTone="warning"
      keybarEntries={[{ key: 'w', label: 'Confirm write' }]}
    >
      <Typography variant="h6" component="h1" gutterBottom>
        Preview fix
      </Typography>
      <Alert severity="warning" sx={{ mb: 2, maxWidth: 720, borderRadius: 0 }}>
        Applying {globalSshChosenToApply.length} of 4 recommended options to the <code>Host *</code>{' '}
        block in {globalSshTargetFile}. <code>{globalSshDeclinedOption}</code> was left unchanged —
        advisory, not required.
      </Alert>
      <Paper variant="outlined" sx={{ p: 2, maxWidth: 720 }}>
        <Typography variant="subtitle2" sx={{ color: 'text.secondary', mb: 1 }}>
          Diff — Host * in {globalSshTargetFile}
        </Typography>
        <Box component="pre" sx={{ m: 0, fontFamily: 'inherit', fontSize: 13 }}>
          {globalSshFixPreviewLines.join('\n')}
        </Box>
      </Paper>
      <Typography sx={{ color: 'text.secondary', mt: 2, maxWidth: 720 }}>
        gitid only owns the block between its sentinels — everything else in {globalSshTargetFile}{' '}
        is preserved verbatim.
      </Typography>
    </Shell>
  );
}

const route: RouteModule = {
  path: '/global-ssh/fix-preview',
  element: <FixPreviewScreen />,
  title: 'global-ssh/fix-preview',
};

export default route;
