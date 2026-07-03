import { Alert, Box, Paper, Stack, Typography } from '@mui/material';
import Shell from '../../shell/Shell';
import type { RouteModule } from '../../App';
import {
  allowedSignersLine,
  defaultMatchStrategy,
  gitScreenManagedFragmentText,
  gitScreenMatchStrategyPreview,
  personalIdentityGitFragment,
} from '../../data/recipeFixtures';

/**
 * git-screen / review-readonly — GITUI-05: the read-only review before
 * write, showing the fragment + includeIf + allowed_signers TOGETHER. This
 * is the surface's highest-risk affordance (§4(2), GITUI-04): the
 * allowed_signers email MUST be byte-identical to user.email — shown side
 * by side so a mismatch is visible, not buried.
 */
function ReviewReadonlyScreen() {
  const emailsMatch = allowedSignersLine.startsWith(personalIdentityGitFragment.userEmail + ' ');

  return (
    <Shell
      title="git-screen/review-readonly"
      headerContext={{ identityCount: 1, health: 'healthy' }}
      statusMessage="Read-only review — nothing has been written yet."
      keybarEntries={[{ key: 'w', label: 'Confirm write' }]}
    >
      <Typography variant="h6" component="h1" gutterBottom>
        Review (read-only)
      </Typography>
      <Stack spacing={2} sx={{ maxWidth: 720 }}>
        <Paper variant="outlined" sx={{ p: 2 }}>
          <Typography variant="subtitle2" sx={{ color: 'text.secondary', mb: 1 }}>
            Git fragment
          </Typography>
          <Box component="pre" sx={{ m: 0, fontFamily: 'inherit', fontSize: 13 }}>
            {gitScreenManagedFragmentText}
          </Box>
        </Paper>
        <Paper variant="outlined" sx={{ p: 2 }}>
          <Typography variant="subtitle2" sx={{ color: 'text.secondary', mb: 1 }}>
            includeIf ({defaultMatchStrategy})
          </Typography>
          <Box component="pre" sx={{ m: 0, fontFamily: 'inherit', fontSize: 13 }}>
            {gitScreenMatchStrategyPreview[defaultMatchStrategy]}
          </Box>
        </Paper>
        <Paper
          variant="outlined"
          sx={{ p: 2, borderColor: emailsMatch ? 'success.main' : 'error.main', borderWidth: 2 }}
        >
          <Typography variant="subtitle2" sx={{ color: 'text.secondary', mb: 1 }}>
            ~/.ssh/allowed_signers
          </Typography>
          <Box component="pre" sx={{ m: 0, fontFamily: 'inherit', fontSize: 13 }}>
            {allowedSignersLine}
          </Box>
          <Stack direction="row" spacing={1} alignItems="center" sx={{ mt: 1.5 }}>
            <Typography sx={{ color: 'text.secondary' }}>user.email:</Typography>
            <Typography component="code" sx={{ fontFamily: 'inherit' }}>
              {personalIdentityGitFragment.userEmail}
            </Typography>
          </Stack>
          <Alert
            severity={emailsMatch ? 'success' : 'error'}
            sx={{ mt: 1, borderRadius: 0 }}
          >
            {emailsMatch
              ? '✓ Byte-identical — allowed_signers will accept commits signed under this identity.'
              : '✗ Mismatch — this signature will NOT be trusted for user.email.'}
          </Alert>
        </Paper>
      </Stack>
    </Shell>
  );
}

const route: RouteModule = {
  path: '/git-screen/review-readonly',
  element: <ReviewReadonlyScreen />,
  title: 'git-screen/review-readonly',
};

export default route;
