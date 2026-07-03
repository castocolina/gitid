import { Alert, Paper, Stack, Typography } from '@mui/material';
import Shell from '../../shell/Shell';
import type { RouteModule } from '../../App';
import { fixerNothingToFixSummary, fixerSafetyNote } from '../../data/recipeFixtures';

/**
 * fixer / nothing-to-fix — the healthy empty state (§4.7): both sections
 * report zero fixable problems. Same two-section layout as fixer-list,
 * proving the SSH/Git split holds even when there is nothing to fix — the
 * fixer's own counterpart to health/health-all-green.
 */
function NothingToFixScreen() {
  return (
    <Shell
      title="fixer/nothing-to-fix"
      headerContext={{ identityCount: 8, health: 'healthy' }}
      statusMessage="0 fixable problems across SSH and Git."
      statusTone="healthy"
      keybarEntries={[]}
    >
      <Typography variant="h6" component="h1" gutterBottom>
        Fixer
      </Typography>
      <Alert severity="info" sx={{ mb: 2, maxWidth: 900, borderRadius: 0 }}>
        {fixerSafetyNote}
      </Alert>
      <Stack spacing={2} sx={{ maxWidth: 720 }}>
        <Paper variant="outlined" sx={{ p: 2, borderColor: 'success.main', borderWidth: 2 }}>
          <Typography variant="subtitle2" sx={{ fontWeight: 700, mb: 1 }}>
            SSH
          </Typography>
          <Alert severity="success" sx={{ borderRadius: 0 }}>
            ✓ {fixerNothingToFixSummary.ssh}
          </Alert>
        </Paper>
        <Paper variant="outlined" sx={{ p: 2, borderColor: 'success.main', borderWidth: 2 }}>
          <Typography variant="subtitle2" sx={{ fontWeight: 700, mb: 1 }}>
            Git
          </Typography>
          <Alert severity="success" sx={{ borderRadius: 0 }}>
            ✓ {fixerNothingToFixSummary.git}
          </Alert>
        </Paper>
      </Stack>
    </Shell>
  );
}

const route: RouteModule = {
  path: '/fixer/nothing-to-fix',
  element: <NothingToFixScreen />,
  title: 'fixer/nothing-to-fix',
};

export default route;
