import { Box, Paper, Stack, Typography } from '@mui/material';
import Shell from '../../shell/Shell';
import type { RouteModule } from '../../App';
import { defaultMatchStrategy, gitScreenMatchStrategyPreview } from '../../data/recipeFixtures';

/**
 * git-screen / match-strategy-select — GITUI-03: choose how the fragment is
 * wired into `~/.gitconfig`. `gitdir` is the DEFAULT (02-UX-DIRECTION.md
 * §3/§6); `hasconfig:remote.*.url` and `both` are also available. A live
 * `includeIf` preview reflects the selected strategy — shown here for the
 * default `gitdir`.
 */
function MatchStrategySelectScreen() {
  return (
    <Shell
      title="git-screen/match-strategy-select"
      headerContext={{ identityCount: 1, health: 'healthy' }}
      statusMessage={`Match strategy: ${defaultMatchStrategy} (default).`}
      keybarEntries={[{ key: 'r', label: 'Review' }]}
    >
      <Typography variant="h6" component="h1" gutterBottom>
        Match strategy
      </Typography>
      <Stack direction="row" spacing={3}>
        <Stack spacing={1.5} sx={{ flex: 1, maxWidth: 420 }}>
          <Paper
            variant="outlined"
            sx={{
              p: 1.5,
              borderColor: defaultMatchStrategy === 'gitdir' ? 'success.main' : 'divider',
              borderWidth: defaultMatchStrategy === 'gitdir' ? 2 : 1,
            }}
          >
            <Typography sx={{ fontWeight: 700 }}>
              gitdir:{defaultMatchStrategy === 'gitdir' ? ' ✓ default' : ''}
            </Typography>
            <Typography sx={{ color: 'text.secondary' }}>
              Applies whenever the repository lives under a matching directory path — no
              dependency on the remote URL.
            </Typography>
          </Paper>
          <Paper variant="outlined" sx={{ p: 1.5 }}>
            <Typography sx={{ fontWeight: 700 }}>hasconfig:remote.*.url</Typography>
            <Typography sx={{ color: 'text.secondary' }}>
              Applies whenever any remote URL matches the identity's provider host —
              location-independent, combinable with gitdir.
            </Typography>
          </Paper>
          <Paper variant="outlined" sx={{ p: 1.5 }}>
            <Typography sx={{ fontWeight: 700 }}>both</Typography>
            <Typography sx={{ color: 'text.secondary' }}>
              Applies both includeIf blocks together — broadest match.
            </Typography>
          </Paper>
        </Stack>
        <Paper variant="outlined" sx={{ flex: 1, p: 2, minHeight: 200 }}>
          <Typography variant="subtitle2" sx={{ color: 'text.secondary', mb: 1 }}>
            Live includeIf preview ({defaultMatchStrategy})
          </Typography>
          <Box component="pre" sx={{ m: 0, fontFamily: 'inherit', fontSize: 13 }}>
            {gitScreenMatchStrategyPreview[defaultMatchStrategy]}
          </Box>
        </Paper>
      </Stack>
    </Shell>
  );
}

const route: RouteModule = {
  path: '/git-screen/match-strategy-select',
  element: <MatchStrategySelectScreen />,
  title: 'git-screen/match-strategy-select',
};

export default route;
