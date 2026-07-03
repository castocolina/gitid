import { Alert, Box, Chip, Paper, Stack, Typography } from '@mui/material';
import Shell from '../../shell/Shell';
import type { RouteModule } from '../../App';
import {
  healthFindingDetailTarget,
  healthInfoColor,
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
 * health / finding-detail — the full detail of ONE finding: the
 * `IdentitiesOnly no` + explicit `IdentityFile` contradiction (HLTH-04),
 * the deep-dive target reached from health-with-findings (`v`). Reached
 * the same way global-ssh's option-detail deep-dives IdentitiesOnly.
 */
function FindingDetailScreen() {
  const f = healthFindingDetailTarget;

  return (
    <Shell
      title="health/finding-detail"
      headerContext={{ identityCount: 8, health: 'error' }}
      statusMessage={`${f.section} — ${f.title}`}
      statusTone="error"
      keybarEntries={[]}
    >
      <Typography variant="h6" component="h1" gutterBottom>
        {f.title}
      </Typography>
      <Alert severity="info" sx={{ mb: 2, maxWidth: 900, borderRadius: 0 }}>
        {healthReadOnlyNote}
      </Alert>
      <Paper variant="outlined" sx={{ p: 2, maxWidth: 720 }}>
        <Stack direction="row" spacing={1} alignItems="center" sx={{ mb: 2 }}>
          <Box component="span" sx={{ color: severityColor(f.severity), fontWeight: 700 }}>
            {healthSeverityGlyph[f.severity]} {healthSeverityWord[f.severity]}
          </Box>
          <Chip size="small" variant="outlined" label={f.section} sx={{ borderRadius: 0, fontFamily: 'inherit' }} />
          <Chip size="small" variant="outlined" label={f.family} sx={{ borderRadius: 0, fontFamily: 'inherit' }} />
        </Stack>
        <Typography sx={{ mb: 2 }}>{f.explanation}</Typography>
        {f.suggestedFix && (
          <Alert severity="warning" sx={{ borderRadius: 0 }}>
            {f.suggestedFix}
          </Alert>
        )}
      </Paper>
    </Shell>
  );
}

const route: RouteModule = {
  path: '/health/finding-detail',
  element: <FindingDetailScreen />,
  title: 'health/finding-detail',
};

export default route;
