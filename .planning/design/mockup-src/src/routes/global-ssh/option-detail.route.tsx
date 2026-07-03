import { Alert, Box, Chip, List, ListItemText, Paper, Stack, Typography } from '@mui/material';
import Shell from '../../shell/Shell';
import type { RouteModule } from '../../App';
import {
  globalSshAdvisoryNote,
  globalSshDetailExplanation,
  globalSshDetailTarget,
  globalSshOptions,
} from '../../data/recipeFixtures';

/**
 * global-ssh / option-detail — the full, contractual (verbatim, §3) risk
 * explanation for one option. Targets IdentitiesOnly (the highest-risk
 * option in the set), mirroring identity-manager's single-target
 * `detail-ssh-first` precedent — the deep explanation is demonstrated once;
 * every option already carries a one-line risk summary on options-list.
 */
function OptionDetailScreen() {
  const target = globalSshDetailTarget; // IdentitiesOnly

  return (
    <Shell
      title="global-ssh/option-detail"
      headerContext={{ identityCount: 8, health: 'warning' }}
      statusMessage={`${target.key} — full explanation. Advisory only, nothing changed yet.`}
      statusTone="warning"
      keybarEntries={[{ key: 'f', label: 'Preview fix' }]}
    >
      <Typography variant="h6" component="h1" gutterBottom>
        {target.key}
      </Typography>
      <Stack direction="row" spacing={3}>
        <Paper variant="outlined" sx={{ flex: 1, maxWidth: 260, p: 1 }}>
          <List dense disablePadding>
            {globalSshOptions.map((opt) => (
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
            <Stack direction="row" spacing={2}>
              <Typography>Current: {target.currentValue}</Typography>
              <Chip size="small" variant="outlined" label={`${target.risk} risk`} sx={{ borderRadius: 0 }} />
              <Typography>Recommended: {target.recommendedValue}</Typography>
            </Stack>
          </Paper>
          <Paper variant="outlined" sx={{ p: 2 }}>
            <Box component="pre" sx={{ m: 0, whiteSpace: 'pre-wrap', fontFamily: 'inherit', fontSize: 13 }}>
              {globalSshDetailExplanation}
            </Box>
          </Paper>
          <Alert severity="warning" sx={{ borderRadius: 0 }}>
            {globalSshAdvisoryNote}
          </Alert>
        </Stack>
      </Stack>
    </Shell>
  );
}

const route: RouteModule = {
  path: '/global-ssh/option-detail',
  element: <OptionDetailScreen />,
  title: 'global-ssh/option-detail',
};

export default route;
