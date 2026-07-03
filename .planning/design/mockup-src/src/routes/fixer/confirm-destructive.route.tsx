import { Alert, Box, Paper, Stack, Typography } from '@mui/material';
import Shell from '../../shell/Shell';
import type { RouteModule } from '../../App';
import { fixerConfirmDestructiveNote, fixerTarget, fixerTargetFile, fixerTargetHost } from '../../data/recipeFixtures';

/**
 * fixer / confirm-destructive — mutation-ceremony beat 2 (§5), specific to
 * fix-in-place rewrites of EXISTING directives (§4.7's highest-risk
 * affordance, T-02-FIX): the strongest confirm this medium allows short of
 * a typed confirmation (identity-manager's "delete everything" precedent).
 * Destructive actions never default-focus "yes" (§5) — the default-focused
 * option here is "No".
 */
function ConfirmDestructiveScreen() {
  return (
    <Shell
      title="fixer/confirm-destructive"
      headerContext={{ identityCount: 8, health: 'error' }}
      statusMessage="This rewrites an existing directive — review carefully."
      statusTone="error"
      keybarEntries={[{ key: 'y', label: 'Yes, apply the rewrite' }]}
    >
      <Typography variant="h6" component="h1" gutterBottom>
        Confirm: {fixerTarget.title}
      </Typography>
      <Stack spacing={2} sx={{ maxWidth: 560 }}>
        <Alert severity="error" sx={{ borderRadius: 0 }}>
          {fixerConfirmDestructiveNote}
        </Alert>
        <Paper variant="outlined" sx={{ p: 2 }}>
          <Stack spacing={0.5}>
            <Typography>
              • File: <code>{fixerTargetFile}</code>
            </Typography>
            <Typography>
              • Host block: <code>{fixerTargetHost}</code>
            </Typography>
            <Typography>• Directive rewritten: IdentitiesOnly no → yes</Typography>
          </Stack>
        </Paper>
        <Box>
          <Typography sx={{ color: 'text.secondary' }}>
            Default-focused: <strong>No, cancel</strong>. Destructive actions never default to
            "yes" (§5) — a backup is taken before anything is written.
          </Typography>
        </Box>
      </Stack>
    </Shell>
  );
}

const route: RouteModule = {
  path: '/fixer/confirm-destructive',
  element: <ConfirmDestructiveScreen />,
  title: 'fixer/confirm-destructive',
};

export default route;
