import { Alert, Box, Chip, List, ListItemButton, ListItemText, Paper, Stack, Typography } from '@mui/material';
import Shell from '../../shell/Shell';
import type { RouteModule } from '../../App';
import { globalGitAdvisoryNote, globalGitDetailTarget, globalGitOptions } from '../../data/recipeFixtures';
import { semanticColors } from '../../theme';

const toneColor = (needsAction: boolean) => (needsAction ? semanticColors.warning : semanticColors.healthy);

/**
 * global-git / options-list — the entry screen (number key `3`): the
 * GGIT-01 baseline + recipe-defaults option set, each row showing its
 * current value + recommended value + a one-line explanation.
 * Master-detail archetype (§2): list left, a preview of the highlighted
 * option's full detail right. `init.defaultBranch` carries a dedicated
 * "main vs master" highlight (GGIT-01's own highest-visibility default).
 * Recommendations are ADVISORY, never blocking (§4.5, §5) — same yellow
 * `!` language as global-ssh, never a red block.
 */
function OptionsListScreen() {
  const highlighted = globalGitDetailTarget; // init.defaultBranch

  return (
    <Shell
      title="global-git/options-list"
      headerContext={{ identityCount: 8, health: 'warning' }}
      statusMessage="11 git options reviewed — 10 recommended, 1 informational. Advisory only."
      statusTone="warning"
      keybarEntries={[
        { key: 'v', label: 'View full explanation' },
        { key: 'f', label: 'Preview fix' },
      ]}
    >
      <Typography variant="h6" component="h1" gutterBottom>
        Global Git options
      </Typography>
      <Alert severity="warning" sx={{ mb: 2, maxWidth: 900, borderRadius: 0 }}>
        {globalGitAdvisoryNote}
      </Alert>
      <Stack direction="row" spacing={3}>
        <Paper variant="outlined" sx={{ flex: 1, maxWidth: 560 }}>
          <List disablePadding>
            {globalGitOptions.map((opt) => (
              <ListItemButton
                key={opt.key}
                selected={opt.key === highlighted.key}
                sx={{ borderBottom: 1, borderColor: 'divider', '&:last-of-type': { borderBottom: 0 } }}
              >
                <ListItemText
                  primary={
                    <Stack direction="row" spacing={1} alignItems="center">
                      <Box component="span" sx={{ color: toneColor(opt.needsAction) }}>
                        {opt.needsAction ? '!' : '✓'}
                      </Box>
                      <Box component="span" sx={{ fontWeight: 700 }}>
                        {opt.key}
                      </Box>
                      {opt.highlight && (
                        <Chip
                          size="small"
                          variant="outlined"
                          color="warning"
                          label="main vs master"
                          sx={{ borderRadius: 0, fontFamily: 'inherit' }}
                        />
                      )}
                      <Chip
                        size="small"
                        variant="outlined"
                        label={opt.needsAction ? 'recommended' : 'informational'}
                        sx={{
                          borderRadius: 0,
                          fontFamily: 'inherit',
                          color: toneColor(opt.needsAction),
                          borderColor: toneColor(opt.needsAction),
                        }}
                      />
                    </Stack>
                  }
                  secondary={
                    <>
                      current: {opt.currentValue} — recommended: {opt.recommendedValue}
                      <br />
                      {opt.oneLiner}
                    </>
                  }
                />
              </ListItemButton>
            ))}
          </List>
        </Paper>
        <Paper variant="outlined" sx={{ flex: 1, p: 2, minHeight: 260 }}>
          <Typography variant="subtitle2" sx={{ color: 'text.secondary', mb: 1 }}>
            Preview — {highlighted.key}
          </Typography>
          <Stack spacing={0.5}>
            <Typography>Current: {highlighted.currentValue}</Typography>
            <Typography>Recommended: {highlighted.recommendedValue}</Typography>
          </Stack>
          <Typography sx={{ color: 'text.secondary', mt: 2 }}>
            Press <code>v</code> for the full explanation, <code>f</code> to preview applying the
            recommended defaults.
          </Typography>
        </Paper>
      </Stack>
    </Shell>
  );
}

const route: RouteModule = {
  path: '/global-git/options-list',
  element: <OptionsListScreen />,
  title: 'global-git/options-list',
};

export default route;
