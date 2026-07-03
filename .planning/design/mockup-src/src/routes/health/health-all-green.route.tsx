import { Alert, Paper, Stack, Typography } from '@mui/material';
import Shell from '../../shell/Shell';
import type { RouteModule } from '../../App';
import { healthAllGreenSummary, healthReadOnlyNote } from '../../data/recipeFixtures';

/**
 * health / health-all-green — the healthy empty state: both sections
 * report zero findings. Same two-section layout (HLTH-01) as
 * health-with-findings, proving the SSH/Git split holds even when there
 * is nothing to report.
 */
function HealthAllGreenScreen() {
  return (
    <Shell
      title="health/health-all-green"
      headerContext={{ identityCount: 8, health: 'healthy' }}
      statusMessage="0 findings across SSH and Git. Diagnosis only."
      statusTone="healthy"
      keybarEntries={[
        { key: 'v', label: 'View a finding example' },
        { key: 'i', label: 'Per-identity health' },
        { key: 'x', label: 'Parse-error example' },
      ]}
    >
      <Typography variant="h6" component="h1" gutterBottom>
        Health
      </Typography>
      <Alert severity="info" sx={{ mb: 2, maxWidth: 900, borderRadius: 0 }}>
        {healthReadOnlyNote}
      </Alert>
      <Stack spacing={2} sx={{ maxWidth: 720 }}>
        <Paper variant="outlined" sx={{ p: 2, borderColor: 'success.main', borderWidth: 2 }}>
          <Typography variant="subtitle2" sx={{ fontWeight: 700, mb: 1 }}>
            SSH
          </Typography>
          <Alert severity="success" sx={{ borderRadius: 0 }}>
            ✓ {healthAllGreenSummary.ssh}
          </Alert>
        </Paper>
        <Paper variant="outlined" sx={{ p: 2, borderColor: 'success.main', borderWidth: 2 }}>
          <Typography variant="subtitle2" sx={{ fontWeight: 700, mb: 1 }}>
            Git
          </Typography>
          <Alert severity="success" sx={{ borderRadius: 0 }}>
            ✓ {healthAllGreenSummary.git}
          </Alert>
        </Paper>
      </Stack>
    </Shell>
  );
}

const route: RouteModule = {
  path: '/health/health-all-green',
  element: <HealthAllGreenScreen />,
  title: 'health/health-all-green',
};

export default route;
