import { Box, Chip, Paper, Stack, Typography } from '@mui/material';
import Shell from '../../shell/Shell';
import type { RouteModule } from '../../App';
import { algorithmCatalog } from '../../data/recipeFixtures';

/**
 * create-flow / algo-catalog — KEY-01's top-5 algorithm catalog. ed25519 is
 * the best/default recommendation; the other four show real per-algorithm
 * security + macOS/Linux local-availability notes (KEY-03).
 */
function AlgoCatalogScreen() {
  return (
    <Shell
      title="create-flow/algo-catalog"
      headerContext={{ identityCount: 0, health: 'healthy' }}
      statusMessage="Choose a key algorithm to begin creating a new identity."
      keybarEntries={[{ key: 'c', label: 'Continue with ed25519 (default)' }]}
    >
      <Typography variant="h6" component="h1" gutterBottom>
        1. Choose a key algorithm
      </Typography>
      <Stack spacing={1.5} sx={{ maxWidth: 880 }}>
        {algorithmCatalog.map((algo) => (
          <Paper
            key={algo.id}
            variant="outlined"
            sx={{
              p: 1.5,
              borderColor: algo.recommended ? 'success.main' : 'divider',
              borderWidth: algo.recommended ? 2 : 1,
            }}
          >
            <Stack direction="row" spacing={1.5} alignItems="center" sx={{ mb: 0.5 }}>
              <Typography component="span" sx={{ fontWeight: 700 }}>
                {algo.label}
              </Typography>
              {algo.recommended && (
                <Chip
                  size="small"
                  label={
                    <Box component="span" sx={{ color: 'success.main' }}>
                      ✓ best / default
                    </Box>
                  }
                  variant="outlined"
                  sx={{ borderRadius: 0 }}
                />
              )}
            </Stack>
            <Typography sx={{ color: 'text.secondary' }}>{algo.security}</Typography>
            <Stack direction="row" spacing={3} sx={{ mt: 0.5 }}>
              <Typography variant="body2" sx={{ color: 'text.secondary' }}>
                macOS: {algo.macos}
              </Typography>
              <Typography variant="body2" sx={{ color: 'text.secondary' }}>
                Linux: {algo.linux}
              </Typography>
            </Stack>
          </Paper>
        ))}
      </Stack>
    </Shell>
  );
}

const route: RouteModule = {
  path: '/create-flow/algo-catalog',
  element: <AlgoCatalogScreen />,
  title: 'create-flow/algo-catalog',
};

export default route;
