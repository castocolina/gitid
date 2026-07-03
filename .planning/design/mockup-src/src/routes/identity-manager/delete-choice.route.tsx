import { Paper, Stack, Typography } from '@mui/material';
import Shell from '../../shell/Shell';
import type { RouteModule } from '../../App';
import { identityManagerActionTarget, identityManagerDeleteChoices } from '../../data/recipeFixtures';

/**
 * identity-manager / delete-choice — the surface's HIGHEST-RISK affordance
 * (§4(3), MGR-06): two destructive options. The safer option (Git identity
 * only) is DEFAULT-FOCUSED; the irreversible "everything" option is never
 * default-focused and carries the strongest confirm on the NEXT screen
 * (confirm-destructive, §5).
 */
function DeleteChoiceScreen() {
  const target = identityManagerActionTarget; // 'personal'

  return (
    <Shell
      title="identity-manager/delete-choice"
      headerContext={{ identityCount: 1, health: 'healthy' }}
      statusMessage={`Delete ${target.name} — choose what to remove.`}
      statusTone="warning"
      keybarEntries={[{ key: 'x', label: 'Continue' }]}
    >
      <Typography variant="h6" component="h1" gutterBottom>
        Delete {target.name}
      </Typography>
      <Stack spacing={1.5} sx={{ maxWidth: 480 }}>
        <Paper
          variant="outlined"
          sx={{ p: 2, borderColor: 'success.main', borderWidth: 2 }}
          aria-label="default-focused option"
        >
          <Typography sx={{ fontWeight: 700 }}>
            {identityManagerDeleteChoices.gitOnly} — ✓ default
          </Typography>
          <Typography sx={{ color: 'text.secondary' }}>
            Removes the Git `includeIf` block and fragment for this identity; the SSH Host block
            and key are kept. Reversible via the SSH-side re-run of the Git configuration screen.
          </Typography>
        </Paper>
        <Paper variant="outlined" sx={{ p: 2, borderColor: 'error.main', borderWidth: 1 }}>
          <Typography sx={{ fontWeight: 700 }}>{identityManagerDeleteChoices.everything}</Typography>
          <Typography sx={{ color: 'text.secondary' }}>
            Removes the SSH Host block, the Git configuration, AND the key file itself. This is
            irreversible — never default-focused. Continuing to this option requires the
            strongest confirm the medium allows (§5).
          </Typography>
        </Paper>
      </Stack>
    </Shell>
  );
}

const route: RouteModule = {
  path: '/identity-manager/delete-choice',
  element: <DeleteChoiceScreen />,
  title: 'identity-manager/delete-choice',
};

export default route;
