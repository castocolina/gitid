import { Alert, Box, Paper, Typography } from '@mui/material';
import Shell from '../../shell/Shell';
import type { RouteModule } from '../../App';
import { fixerFixPreviewLines, fixerTarget, fixerTargetFile } from '../../data/recipeFixtures';

/**
 * fixer / fix-preview — mutation-ceremony beat 1 (§5): a read-only
 * before/after diff of the EXACT change (§4.7 "diff of the exact
 * change"). Unlike global-ssh/global-git's fix-preview (additions-only
 * `+` lines), this is a true `-`/`+` rewrite diff — the fixer's own
 * highest-risk affordance (T-02-FIX): fix-in-place rewrites of an
 * EXISTING directive's value, not merely an addition. Nothing has
 * changed yet.
 */
function FixPreviewScreen() {
  return (
    <Shell
      title="fixer/fix-preview"
      headerContext={{ identityCount: 8, health: 'error' }}
      statusMessage="Nothing has changed yet — review the diff before confirming."
      statusTone="warning"
      keybarEntries={[{ key: 'x', label: 'Confirm (rewrites an existing directive)' }]}
    >
      <Typography variant="h6" component="h1" gutterBottom>
        Preview fix — {fixerTarget.title}
      </Typography>
      <Alert severity="warning" sx={{ mb: 2, maxWidth: 720, borderRadius: 0 }}>
        This fix REWRITES a directive already present in {fixerTargetFile} — it is not a new
        addition. Review the exact before/after change below.
      </Alert>
      <Paper variant="outlined" sx={{ p: 2, maxWidth: 720 }}>
        <Typography variant="subtitle2" sx={{ color: 'text.secondary', mb: 1 }}>
          Diff — {fixerTargetFile}
        </Typography>
        <Box component="pre" sx={{ m: 0, fontFamily: 'inherit', fontSize: 13 }}>
          {fixerFixPreviewLines.join('\n')}
        </Box>
      </Paper>
      <Typography sx={{ color: 'text.secondary', mt: 2, maxWidth: 720 }}>
        Only the highlighted line changes — the rest of the Host block is untouched.
      </Typography>
    </Shell>
  );
}

const route: RouteModule = {
  path: '/fixer/fix-preview',
  element: <FixPreviewScreen />,
  title: 'fixer/fix-preview',
};

export default route;
