import { Alert, Box, Paper, Stack, Typography } from '@mui/material';
import Shell from '../../shell/Shell';
import type { RouteModule } from '../../App';
import { healthParseErrorTarget, healthReadOnlyNote } from '../../data/recipeFixtures';

/**
 * health / parse-error — a config file that will not parse (HLTH-02). The
 * one condition Health can ONLY report: there is no diff or fix-preview
 * possible until the file is syntactically valid again, reinforcing the
 * read-only-integrity affordance concretely rather than only by the
 * absence of a write-ceremony beat.
 */
function ParseErrorScreen() {
  const t = healthParseErrorTarget;

  return (
    <Shell
      title="health/parse-error"
      headerContext={{ identityCount: 8, health: 'error' }}
      statusMessage={`${t.file} does not parse. Diagnosis only.`}
      statusTone="error"
      keybarEntries={[]}
    >
      <Typography variant="h6" component="h1" gutterBottom>
        Parse error
      </Typography>
      <Alert severity="info" sx={{ mb: 2, maxWidth: 900, borderRadius: 0 }}>
        {healthReadOnlyNote}
      </Alert>
      <Paper variant="outlined" sx={{ p: 2, maxWidth: 720, borderColor: 'error.main', borderWidth: 2 }}>
        <Typography variant="subtitle2" sx={{ color: 'text.secondary', mb: 1 }}>
          Git — {t.file}
        </Typography>
        <Alert severity="error" sx={{ mb: 1.5, borderRadius: 0 }}>
          ✗ {t.rawError}
        </Alert>
        <Box component="pre" sx={{ m: 0, mb: 1.5, fontFamily: 'inherit', fontSize: 13 }}>
          line {t.line}: {t.snippet}
        </Box>
        <Stack spacing={0.5}>
          <Typography>{t.explanation}</Typography>
        </Stack>
      </Paper>
    </Shell>
  );
}

const route: RouteModule = {
  path: '/health/parse-error',
  element: <ParseErrorScreen />,
  title: 'health/parse-error',
};

export default route;
