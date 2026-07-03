import { Alert, Box, Chip, List, ListItemButton, ListItemText, Paper, Stack, Typography } from '@mui/material';
import Shell from '../../shell/Shell';
import type { RouteModule } from '../../App';
import { globalSshAdvisoryNote, globalSshDetailTarget, globalSshOptions } from '../../data/recipeFixtures';
import { semanticColors } from '../../theme';

const toneColor = (needsAction: boolean) => (needsAction ? semanticColors.warning : semanticColors.healthy);

/**
 * global-ssh / options-list — the entry screen (number key `2`): the
 * GSSH-01 dangerous-by-default option set, each row showing its current
 * value + risk + recommended value + a one-line explanation. Master-detail
 * archetype (§2): list left, a preview of the highlighted option's full
 * risk explanation right. Recommendations are ADVISORY, never blocking
 * (§4.4, §5) — a yellow `!`, never a red block, and the user may leave any
 * option unchanged.
 */
function OptionsListScreen() {
  const highlighted = globalSshDetailTarget; // IdentitiesOnly

  return (
    <Shell
      title="global-ssh/options-list"
      headerContext={{ identityCount: 8, health: 'warning' }}
      statusMessage="6 SSH options reviewed — 4 recommended, 2 already set. Advisory only."
      statusTone="warning"
      keybarEntries={[
        { key: 'v', label: 'View full explanation' },
        { key: 'f', label: 'Preview fix' },
      ]}
    >
      <Typography variant="h6" component="h1" gutterBottom>
        Global SSH options
      </Typography>
      <Alert severity="warning" sx={{ mb: 2, maxWidth: 900, borderRadius: 0 }}>
        {globalSshAdvisoryNote}
      </Alert>
      <Stack direction="row" spacing={3}>
        <Paper variant="outlined" sx={{ flex: 1, maxWidth: 560 }}>
          <List disablePadding>
            {globalSshOptions.map((opt) => (
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
                      <Chip
                        size="small"
                        variant="outlined"
                        label={opt.needsAction ? `${opt.risk} risk — recommended` : `${opt.risk} risk — already set`}
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
            <Typography>Risk: {highlighted.risk}</Typography>
            <Typography>Recommended: {highlighted.recommendedValue}</Typography>
          </Stack>
          <Typography sx={{ color: 'text.secondary', mt: 2 }}>
            Press <code>v</code> for the full explanation, <code>f</code> to preview applying the
            recommended fixes.
          </Typography>
        </Paper>
      </Stack>
    </Shell>
  );
}

const route: RouteModule = {
  path: '/global-ssh/options-list',
  element: <OptionsListScreen />,
  title: 'global-ssh/options-list',
};

export default route;
