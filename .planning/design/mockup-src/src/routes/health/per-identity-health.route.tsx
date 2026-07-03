import { Alert, Box, Paper, Stack, Typography } from '@mui/material';
import Shell from '../../shell/Shell';
import type { RouteModule } from '../../App';
import {
  healthInfoColor,
  healthPerIdentityGitFinding,
  healthPerIdentitySSHNote,
  healthPerIdentityTarget,
  healthReadOnlyNote,
  healthSeverityGlyph,
  healthSeverityWord,
  type HealthSeverity,
} from '../../data/recipeFixtures';
import { semanticColors } from '../../theme';

const severityColor = (severity: HealthSeverity): string => {
  switch (severity) {
    case 'critical':
    case 'error':
      return semanticColors.error;
    case 'warning':
      return semanticColors.warning;
    case 'info':
      return healthInfoColor;
    default:
      return semanticColors.dim;
  }
};

/**
 * health / per-identity-health — the per-identity slice (HLTH-05) that
 * feeds a Manager row: targets the `legacy` identity
 * (`identityManagerRows`, state `fragment-path-missing`) — SSH healthy,
 * Git broken (its includeIf targets the missing fragment). Traceably the
 * SAME finding health-with-findings' Git section shows, scoped to one
 * identity — proving HLTH-05's per-identity computation and MGR-07's
 * Identity Manager row badge are the SAME data, not re-derived.
 */
function PerIdentityHealthScreen() {
  const target = healthPerIdentityTarget;
  const gitFinding = healthPerIdentityGitFinding;

  return (
    <Shell
      title="health/per-identity-health"
      headerContext={{ identityCount: 8, health: 'error' }}
      statusMessage={`Per-identity health — ${target.name} (${target.state})`}
      statusTone="error"
      keybarEntries={[]}
    >
      <Typography variant="h6" component="h1" gutterBottom>
        Per-identity health — {target.name}
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
            ✓ {healthPerIdentitySSHNote}
          </Alert>
        </Paper>
        <Paper variant="outlined" sx={{ p: 2 }}>
          <Typography variant="subtitle2" sx={{ fontWeight: 700, mb: 1 }}>
            Git
          </Typography>
          <Stack direction="row" spacing={1} alignItems="center" sx={{ mb: 1 }}>
            <Box component="span" sx={{ color: severityColor(gitFinding.severity), fontWeight: 700 }}>
              {healthSeverityGlyph[gitFinding.severity]} {healthSeverityWord[gitFinding.severity]}
            </Box>
            <Typography component="span" sx={{ fontWeight: 700 }}>
              {gitFinding.title}
            </Typography>
          </Stack>
          <Typography>{gitFinding.explanation}</Typography>
        </Paper>
        <Typography sx={{ color: 'text.secondary' }}>
          This slice feeds the Identity Manager row for {target.name} (MGR-07): {target.note}
        </Typography>
      </Stack>
    </Shell>
  );
}

const route: RouteModule = {
  path: '/health/per-identity-health',
  element: <PerIdentityHealthScreen />,
  title: 'health/per-identity-health',
};

export default route;
