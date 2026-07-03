import { Box, Paper, Stack, TextField, Typography } from '@mui/material';
import Shell from '../../shell/Shell';
import type { RouteModule } from '../../App';
import { sshFormFilled, sshIdentityAliasBlockText } from '../../data/recipeFixtures';

/**
 * create-flow / ssh-form-filled — the SSH form filled in, with a live `Host`
 * block preview reflecting the current field values exactly as it will be
 * written (SSHUI-03). The preview is the SAME literal recipe-accurate block
 * text used across the fixtures, including `Port 443` and
 * `IdentitiesOnly yes`.
 */
function SshFormFilledScreen() {
  return (
    <Shell
      title="create-flow/ssh-form-filled"
      headerContext={{ identityCount: 0, health: 'healthy' }}
      statusMessage="Live preview updates as you type."
      keybarEntries={[
        { key: 't', label: 'Test connection' },
        { key: 'r', label: 'Reuse vs generate key' },
        { key: 'm', label: 'macOS globals' },
      ]}
    >
      <Typography variant="h6" component="h1" gutterBottom>
        2. SSH connection details
      </Typography>
      <Stack direction="row" spacing={3}>
        <Stack spacing={2} sx={{ flex: 1, maxWidth: 420 }}>
          <TextField
            label="Alias prefix"
            value={sshFormFilled.aliasPrefix}
            fullWidth
            size="small"
            helperText="Used to build the SSH Host alias below."
          />
          <TextField
            label="SSH Host"
            value={sshFormFilled.sshHost}
            fullWidth
            size="small"
            helperText="Auto-joined from the alias prefix + provider — editable."
          />
          <TextField
            label="Real hostname"
            value={sshFormFilled.realHostname}
            fullWidth
            size="small"
            helperText="The true SSH endpoint (provider-linked, editable)."
          />
          <TextField
            label="Port"
            value={String(sshFormFilled.port)}
            fullWidth
            size="small"
            helperText="Default 443 — bypasses restrictive firewalls."
          />
        </Stack>
        <Paper variant="outlined" sx={{ flex: 1, p: 2, minHeight: 200 }}>
          <Typography variant="subtitle2" sx={{ color: 'text.secondary', mb: 1 }}>
            Live Host block preview
          </Typography>
          <Box component="pre" sx={{ m: 0, fontFamily: 'inherit', fontSize: 13 }}>
            {sshIdentityAliasBlockText}
          </Box>
        </Paper>
      </Stack>
    </Shell>
  );
}

const route: RouteModule = {
  path: '/create-flow/ssh-form-filled',
  element: <SshFormFilledScreen />,
  title: 'create-flow/ssh-form-filled',
};

export default route;
