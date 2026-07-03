import { Box, Paper, Stack, TextField, Typography } from '@mui/material';
import Shell from '../../shell/Shell';
import type { RouteModule } from '../../App';
import { personalIdentityGitFragment, personalIdentityGitFragmentText } from '../../data/recipeFixtures';

/**
 * git-screen / git-form-filled — the Git fragment form filled in, with a
 * live fragment-text preview reflecting the current field values exactly as
 * they will be written (GITUI-02, mirrors the guided-form + live-preview
 * archetype, §2).
 */
function GitFormFilledScreen() {
  return (
    <Shell
      title="git-screen/git-form-filled"
      headerContext={{ identityCount: 1, health: 'healthy' }}
      statusMessage="Live preview updates as you type."
      keybarEntries={[{ key: 'm', label: 'Match strategy' }]}
    >
      <Typography variant="h6" component="h1" gutterBottom>
        Git identity (per-identity)
      </Typography>
      <Stack direction="row" spacing={3}>
        <Stack spacing={2} sx={{ flex: 1, maxWidth: 420 }}>
          <TextField
            label="user.name"
            value={personalIdentityGitFragment.userName}
            fullWidth
            size="small"
            helperText="Commit author name for this identity."
          />
          <TextField
            label="user.email"
            value={personalIdentityGitFragment.userEmail}
            fullWidth
            size="small"
            helperText="Commit author email — must match the allowed_signers email later (GITUI-04)."
          />
          <TextField
            label="gpg.format"
            value={personalIdentityGitFragment.gpgFormat}
            fullWidth
            size="small"
            disabled
            helperText="Fixed — gitid signs via SSH keys, no GPG."
          />
          <TextField
            label="user.signingkey"
            value={personalIdentityGitFragment.signingKey}
            fullWidth
            size="small"
            helperText="A PATH to the public key — never the key material itself."
          />
          <TextField
            label="commit.gpgsign"
            value={String(personalIdentityGitFragment.commitGpgsign)}
            fullWidth
            size="small"
            helperText="Sign every commit for this identity."
          />
        </Stack>
        <Paper variant="outlined" sx={{ flex: 1, p: 2, minHeight: 200 }}>
          <Typography variant="subtitle2" sx={{ color: 'text.secondary', mb: 1 }}>
            Live fragment preview
          </Typography>
          <Box component="pre" sx={{ m: 0, fontFamily: 'inherit', fontSize: 13 }}>
            {personalIdentityGitFragmentText}
          </Box>
        </Paper>
      </Stack>
    </Shell>
  );
}

const route: RouteModule = {
  path: '/git-screen/git-form-filled',
  element: <GitFormFilledScreen />,
  title: 'git-screen/git-form-filled',
};

export default route;
