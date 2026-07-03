import { Alert, Box, List, ListItemButton, ListItemText, Paper, Stack, Typography } from '@mui/material';
import Shell from '../../shell/Shell';
import type { RouteModule } from '../../App';
import {
  fixerBatchFixNote,
  fixerFindings,
  fixerSafetyNote,
  fixerTarget,
  healthInfoColor,
  healthSeverityGlyph,
  healthSeverityWord,
  type HealthFinding,
  type HealthSeverity,
} from '../../data/recipeFixtures';
import { semanticColors } from '../../theme';

/** LOCKED glyph contract (02-UX-DIRECTION.md §2, reused byte-identically
 * from health's own §4.6 addendum — the fixer presents the SAME findings
 * Health diagnosed): warning is ALWAYS yellow `!`, error/critical are
 * ALWAYS red `✗` (distinguished by the word, not the glyph/color), info is
 * cyan `~`. */
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

function ProblemRow({ finding, highlighted }: { finding: HealthFinding; highlighted: boolean }) {
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
        secondary={finding.suggestedFix}
      />
    </ListItemButton>
  );
}

/**
 * fixer / fixer-list — the entry screen (number key `5`): SSH and Git
 * sections (§4.7 "two sections"), each problem showing severity + plain
 * explanation + suggested fix. Lists the SAME findings Health diagnosed
 * (`fixerFindings`, filtered to the ones that carry a `suggestedFix`) —
 * traceable, not re-derived (HLTH-04's own "available on the Fixer
 * screen" hand-off text, honored here). A batch-fix action is offered but
 * still walks the full per-change ceremony (§4.7's "batch-fix must still
 * preview every change").
 */
function FixerListScreen() {
  const sshFindings = fixerFindings.filter((f) => f.section === 'SSH');
  const gitFindings = fixerFindings.filter((f) => f.section === 'Git');
  const highlighted = fixerTarget;

  return (
    <Shell
      title="fixer/fixer-list"
      headerContext={{ identityCount: 8, health: 'error' }}
      statusMessage={`${fixerFindings.length} fixable problems across SSH and Git.`}
      statusTone="error"
      keybarEntries={[{ key: 'v', label: 'Preview fix (IdentitiesOnly contradiction)' }]}
    >
      <Typography variant="h6" component="h1" gutterBottom>
        Fixer
      </Typography>
      <Alert severity="info" sx={{ mb: 2, maxWidth: 900, borderRadius: 0 }}>
        {fixerSafetyNote}
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
              <ProblemRow key={f.id} finding={f} highlighted={f.id === highlighted.id} />
            ))}
          </List>
          <Box sx={{ px: 2, py: 1, borderTop: 1, borderColor: 'divider' }}>
            <Typography variant="subtitle2" sx={{ fontWeight: 700 }}>
              Git
            </Typography>
          </Box>
          <List disablePadding>
            {gitFindings.map((f) => (
              <ProblemRow key={f.id} finding={f} highlighted={f.id === highlighted.id} />
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
          <Typography sx={{ color: 'text.secondary', mt: 2 }}>{highlighted.suggestedFix}</Typography>
          <Typography sx={{ color: 'text.secondary', mt: 2 }}>
            Press <code>v</code> to preview this fix's exact diff.
          </Typography>
          <Alert severity="info" sx={{ mt: 2, borderRadius: 0 }}>
            {fixerBatchFixNote}
          </Alert>
        </Paper>
      </Stack>
    </Shell>
  );
}

const route: RouteModule = {
  path: '/fixer/fixer-list',
  element: <FixerListScreen />,
  title: 'fixer/fixer-list',
};

export default route;
