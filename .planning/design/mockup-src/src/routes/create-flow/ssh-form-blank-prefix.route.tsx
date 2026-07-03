import { Alert, Box, Paper, Stack, TextField, Typography } from '@mui/material';
import Shell from '../../shell/Shell';
import type { RouteModule } from '../../App';
import { sshFormBlankPrefixHost } from '../../data/recipeFixtures';

/**
 * create-flow / ssh-form-blank-prefix — the blank-prefix WYSIWYG rule
 * (SSHUI-01): with no alias prefix, `SSH Host` is the provider host itself,
 * verbatim — never an invented suffix like ".github.com" with nothing in
 * front of it.
 */
function SshFormBlankPrefixScreen() {
  return (
    <Shell
      title="create-flow/ssh-form-blank-prefix"
      headerContext={{ identityCount: 0, health: 'healthy' }}
      statusMessage="Blank alias prefix — SSH Host defaults to the provider host verbatim."
      keybarEntries={[]}
    >
      <Typography variant="h6" component="h1" gutterBottom>
        2. SSH connection details — blank prefix (WYSIWYG)
      </Typography>
      <Stack spacing={2} sx={{ maxWidth: 480 }}>
        <TextField
          label="Alias prefix"
          value=""
          placeholder="(blank)"
          fullWidth
          size="small"
          helperText="Leave blank to use the provider host directly."
        />
        <TextField
          label="SSH Host"
          value={sshFormBlankPrefixHost}
          fullWidth
          size="small"
          helperText="WYSIWYG: with a blank prefix, this is the provider host itself — no invented suffix."
        />
        <Alert severity="info" sx={{ borderRadius: 0 }}>
          A blank alias prefix means gitid writes <code>Host {sshFormBlankPrefixHost}</code>{' '}
          exactly, not <code>Host .{sshFormBlankPrefixHost}</code> or any other invented
          combination — what you see is what gets written.
        </Alert>
        <Paper variant="outlined" sx={{ p: 2 }}>
          <Typography variant="subtitle2" sx={{ color: 'text.secondary', mb: 1 }}>
            Resulting Host line
          </Typography>
          <Box component="pre" sx={{ m: 0, fontFamily: 'inherit', fontSize: 13 }}>
            Host {sshFormBlankPrefixHost}
          </Box>
        </Paper>
      </Stack>
    </Shell>
  );
}

const route: RouteModule = {
  path: '/create-flow/ssh-form-blank-prefix',
  element: <SshFormBlankPrefixScreen />,
  title: 'create-flow/ssh-form-blank-prefix',
};

export default route;
