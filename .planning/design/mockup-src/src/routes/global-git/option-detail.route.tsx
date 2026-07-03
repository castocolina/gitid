import { Alert, Box, Chip, List, ListItemText, Paper, Stack, Typography } from '@mui/material';
import Shell from '../../shell/Shell';
import type { RouteModule } from '../../App';
import {
  globalGitAdvisoryNote,
  globalGitDetailExplanation,
  globalGitDetailTarget,
  globalGitOptions,
} from '../../data/recipeFixtures';

/**
 * global-git / option-detail — the full, contractual (verbatim, §3)
 * explanation for one option. Targets init.defaultBranch — the option with
 * the dedicated main-vs-master highlight — mirroring global-ssh's
 * single-target `option-detail` precedent.
 */
function OptionDetailScreen() {
  const target = globalGitDetailTarget; // init.defaultBranch

  return (
    <Shell
      title="global-git/option-detail"
      headerContext={{ identityCount: 8, health: 'warning' }}
      statusMessage={`${target.key} — full explanation. Advisory only, nothing changed yet.`}
      statusTone="warning"
      keybarEntries={[{ key: 'f', label: 'Preview fix' }]}
    >
      <Typography variant="h6" component="h1" gutterBottom>
        {target.key}
      </Typography>
      <Stack direction="row" spacing={3}>
        <Paper variant="outlined" sx={{ flex: 1, maxWidth: 280, p: 1 }}>
          <List dense disablePadding>
            {globalGitOptions.map((opt) => (
              <ListItemText
                key={opt.key}
                sx={{ opacity: opt.key === target.key ? 1 : 0.5, px: 1, py: 0.5 }}
                primary={`${opt.needsAction ? '!' : '✓'} ${opt.key}`}
              />
            ))}
          </List>
        </Paper>
        <Stack spacing={2} sx={{ flex: 1, maxWidth: 640 }}>
          <Paper variant="outlined" sx={{ p: 2 }}>
            <Stack direction="row" spacing={2} alignItems="center">
              <Typography>Current: {target.currentValue}</Typography>
              <Chip size="small" variant="outlined" color="warning" label="main vs master" sx={{ borderRadius: 0 }} />
              <Typography>Recommended: {target.recommendedValue}</Typography>
            </Stack>
          </Paper>
          <Paper variant="outlined" sx={{ p: 2 }}>
            <Box component="pre" sx={{ m: 0, whiteSpace: 'pre-wrap', fontFamily: 'inherit', fontSize: 13 }}>
              {globalGitDetailExplanation}
            </Box>
          </Paper>
          <Alert severity="warning" sx={{ borderRadius: 0 }}>
            {globalGitAdvisoryNote}
          </Alert>
        </Stack>
      </Stack>
    </Shell>
  );
}

const route: RouteModule = {
  path: '/global-git/option-detail',
  element: <OptionDetailScreen />,
  title: 'global-git/option-detail',
};

export default route;
