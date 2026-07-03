import { Box, Paper, Stack, TextField, Typography } from '@mui/material';
import Shell from '../../shell/Shell';
import type { RouteModule } from '../../App';

/**
 * git-screen / git-form-empty — the per-identity Git fragment form before
 * any field is filled (GITUI-01/02). This screen is a keyless modal launched
 * FROM Identities via the `g` LaunchKey, shown for the identity just
 * created by create-flow. Field order: user.name -> user.email ->
 * gpg.format (fixed, ssh) -> user.signingkey (path) -> commit.gpgsign.
 */
function GitFormEmptyScreen() {
  return (
    <Shell
      title="git-screen/git-form-empty"
      headerContext={{ identityCount: 1, health: 'healthy' }}
      statusMessage="Configure this identity's Git fragment."
      keybarEntries={[{ key: 'f', label: 'Fill form (demo)' }]}
    >
      <Typography variant="h6" component="h1" gutterBottom>
        Git identity (per-identity)
      </Typography>
      <Stack direction="row" spacing={3}>
        <Stack spacing={2} sx={{ flex: 1, maxWidth: 420 }}>
          <TextField
            label="user.name"
            placeholder="e.g. Personal Identity"
            fullWidth
            size="small"
            helperText="Commit author name for this identity."
          />
          <TextField
            label="user.email"
            placeholder="e.g. you@personal.example"
            fullWidth
            size="small"
            helperText="Commit author email — must match the allowed_signers email later (GITUI-04)."
          />
          <TextField
            label="gpg.format"
            value="ssh"
            fullWidth
            size="small"
            disabled
            helperText="Fixed — gitid signs via SSH keys, no GPG."
          />
          <TextField
            label="user.signingkey"
            placeholder="e.g. ~/.ssh/id_ed25519_personal.pub"
            fullWidth
            size="small"
            helperText="A PATH to the public key — never the key material itself."
          />
          <TextField
            label="commit.gpgsign"
            value="true"
            fullWidth
            size="small"
            helperText="Default true — sign every commit for this identity."
          />
        </Stack>
        <Paper variant="outlined" sx={{ flex: 1, p: 2, minHeight: 200 }}>
          <Typography variant="subtitle2" sx={{ color: 'text.secondary', mb: 1 }}>
            Live fragment preview
          </Typography>
          <Box
            component="pre"
            sx={{ m: 0, color: 'text.secondary', fontFamily: 'inherit', fontSize: 13 }}
          >
            (fill in the fields to see the resulting fragment)
          </Box>
        </Paper>
      </Stack>
    </Shell>
  );
}

const route: RouteModule = {
  path: '/git-screen/git-form-empty',
  element: <GitFormEmptyScreen />,
  title: 'git-screen/git-form-empty',
};

export default route;
