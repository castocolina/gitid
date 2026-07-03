import { Alert, Box, List, ListItemButton, ListItemText, Paper, Stack, Typography } from '@mui/material';
import Shell from '../../shell/Shell';
import type { RouteModule } from '../../App';
import {
  healthFindingDetailTarget,
  healthFindings,
  healthInfoColor,
  healthReadOnlyNote,
  healthSeverityGlyph,
  healthSeverityWord,
  type HealthFinding,
  type HealthSeverity,
} from '../../data/recipeFixtures';
import { semanticColors } from '../../theme';

/** LOCKED glyph contract (02-UX-DIRECTION.md §2, this surface's own §4.6
 * addendum): warning is ALWAYS yellow `!`, error/critical are ALWAYS red
 * `✗` (distinguished by the word, not the glyph/color), info is cyan `~`
 * (theme.ts has no cyan role yet — `healthInfoColor` is defined locally in
 * recipeFixtures.ts rather than editing the shared theme file). */
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

/** Severity-sorted (critical -> error -> warning -> info), matching
 * internal/doctor/doctor.go's own severity ordering. */
const severityRank: Record<HealthSeverity, number> = { critical: 0, error: 1, warning: 2, info: 3 };
const bySeverity = (a: HealthFinding, b: HealthFinding) => severityRank[a.severity] - severityRank[b.severity];

function FindingRow({ finding, highlighted }: { finding: HealthFinding; highlighted: boolean }) {
  return (
    <ListItemButton
      selected={highlighted}
      sx={{ borderBottom: 1, borderColor: 'divider', '&:last-of-type': { borderBottom: 0 } }}
    >
      <ListItemText
        primary={
          <Stack direction="row" spacing={1} alignItems="center">
            <Box component="span" sx={{ color: severityColor(finding.severity), fontWeight: 700 }}>
              {healthSeverityGlyph[finding.severity]} {healthSeverityWord[finding.severity]}
            </Box>
            <Box component="span" sx={{ fontWeight: 700 }}>
              {finding.title}
            </Box>
          </Stack>
        }
        secondary={finding.family}
      />
    </ListItemButton>
  );
}

/**
 * health / health-with-findings — the entry screen (number key `4`):
 * SSH and Git sections (HLTH-01), each severity-sorted, split by
 * `internal/doctor/doctor.go`'s four-level Severity model. Master-detail
 * archetype (§2): both section lists left, a preview of the highlighted
 * finding's full explanation + suggested-fix hand-off right.
 *
 * Highest-risk affordance (§4.6): read-only integrity — every screen on
 * this surface carries the SAME explicit `healthReadOnlyNote`; nothing
 * here is a write ceremony (no confirm/backup/apply beat exists on this
 * surface at all).
 */
function HealthWithFindingsScreen() {
  const sshFindings = healthFindings.filter((f) => f.section === 'SSH').slice().sort(bySeverity);
  const gitFindings = healthFindings.filter((f) => f.section === 'Git').slice().sort(bySeverity);
  const highlighted = healthFindingDetailTarget;

  return (
    <Shell
      title="health/health-with-findings"
      headerContext={{ identityCount: 8, health: 'error' }}
      statusMessage={`${healthFindings.length} findings across SSH and Git — severity-sorted. Diagnosis only.`}
      statusTone="error"
      keybarEntries={[
        { key: 'h', label: 'All-green example' },
        { key: 'v', label: 'View full detail' },
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
      <Stack direction="row" spacing={3}>
        <Paper variant="outlined" sx={{ flex: 1, maxWidth: 620 }}>
          <Box sx={{ px: 2, py: 1, borderBottom: 1, borderColor: 'divider' }}>
            <Typography variant="subtitle2" sx={{ fontWeight: 700 }}>
              SSH
            </Typography>
          </Box>
          <List disablePadding>
            {sshFindings.map((f) => (
              <FindingRow key={f.id} finding={f} highlighted={f.id === highlighted.id} />
            ))}
          </List>
          <Box sx={{ px: 2, py: 1, borderTop: 1, borderColor: 'divider' }}>
            <Typography variant="subtitle2" sx={{ fontWeight: 700 }}>
              Git
            </Typography>
          </Box>
          <List disablePadding>
            {gitFindings.map((f) => (
              <FindingRow key={f.id} finding={f} highlighted={f.id === highlighted.id} />
            ))}
          </List>
        </Paper>
        <Paper variant="outlined" sx={{ flex: 1, p: 2, minHeight: 280 }}>
          <Typography variant="subtitle2" sx={{ color: 'text.secondary', mb: 1 }}>
            Preview — {highlighted.section}: {highlighted.title}
          </Typography>
          <Stack direction="row" spacing={1} alignItems="center" sx={{ mb: 1 }}>
            <Box component="span" sx={{ color: severityColor(highlighted.severity), fontWeight: 700 }}>
              {healthSeverityGlyph[highlighted.severity]} {healthSeverityWord[highlighted.severity]}
            </Box>
            <Typography component="span" sx={{ color: 'text.secondary' }}>
              {highlighted.family}
            </Typography>
          </Stack>
          <Typography>{highlighted.explanation}</Typography>
          {highlighted.suggestedFix && (
            <Typography sx={{ color: 'text.secondary', mt: 2 }}>{highlighted.suggestedFix}</Typography>
          )}
          <Typography sx={{ color: 'text.secondary', mt: 2 }}>
            Press <code>v</code> for the full finding detail.
          </Typography>
        </Paper>
      </Stack>
    </Shell>
  );
}

const route: RouteModule = {
  path: '/health/health-with-findings',
  element: <HealthWithFindingsScreen />,
  title: 'health/health-with-findings',
};

export default route;
