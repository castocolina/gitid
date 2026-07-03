import { Box, Paper, Stack, TextField, Typography } from '@mui/material';
import Shell from '../../shell/Shell';
import type { RouteModule } from '../../App';

/**
 * create-flow / ssh-form-empty — the SSH form before any field is filled.
 * Field order (SSHUI-01): Alias prefix -> SSH Host -> Real hostname -> Port
 * (default 443, pre-filled even while the rest is empty). SSHUI-02: every
 * field is a real, always-visible, clickable input, none buried in overflow.
 */
function SshFormEmptyScreen() {
  return (
    <Shell
      title="create-flow/ssh-form-empty"
      headerContext={{ identityCount: 0, health: 'healthy' }}
      statusMessage="Fill in the SSH host details below."
      keybarEntries={[
        { key: 'f', label: 'Fill form (demo)' },
        { key: 'b', label: 'Blank-prefix demo' },
      ]}
    >
      <Typography variant="h6" component="h1" gutterBottom>
        2. SSH connection details
      </Typography>
      <Stack direction="row" spacing={3}>
        <Stack spacing={2} sx={{ flex: 1, maxWidth: 420 }}>
          <TextField
            label="Alias prefix"
            placeholder="e.g. personal"
            fullWidth
            size="small"
            helperText="Used to build the SSH Host alias below."
          />
          <TextField
            label="SSH Host"
            placeholder="<prefix>.<provider>"
            fullWidth
            size="small"
            helperText="Auto-joined from the alias prefix + provider — editable."
          />
          <TextField
            label="Real hostname"
            placeholder="e.g. ssh.github.com"
            fullWidth
            size="small"
            helperText="The true SSH endpoint (provider-linked, editable)."
          />
          <TextField
            label="Port"
            defaultValue="443"
            fullWidth
            size="small"
            helperText="Default 443 — bypasses restrictive firewalls."
          />
        </Stack>
        <Paper variant="outlined" sx={{ flex: 1, p: 2, minHeight: 200 }}>
          <Typography variant="subtitle2" sx={{ color: 'text.secondary', mb: 1 }}>
            Live Host block preview
          </Typography>
          <Box
            component="pre"
            sx={{ m: 0, color: 'text.secondary', fontFamily: 'inherit', fontSize: 13 }}
          >
            (fill in the fields to see the resulting Host block)
          </Box>
        </Paper>
      </Stack>
    </Shell>
  );
}

const route: RouteModule = {
  path: '/create-flow/ssh-form-empty',
  element: <SshFormEmptyScreen />,
  title: 'create-flow/ssh-form-empty',
};

export default route;
