import { Box, Paper, Typography } from '@mui/material';
import Shell from '../../shell/Shell';
import type { RouteModule } from '../../App';
import { sshMacGlobalsBlockText } from '../../data/recipeFixtures';

/**
 * create-flow / macos-globals-block — SSHUI-05: a `Host *` block emitting
 * `UseKeychain yes` + `AddKeysToAgent yes`, guarded by
 * `IgnoreUnknown UseKeychain` so it is a documented no-op on Linux.
 */
function MacosGlobalsBlockScreen() {
  return (
    <Shell
      title="create-flow/macos-globals-block"
      headerContext={{ identityCount: 0, health: 'healthy' }}
      statusMessage="macOS Keychain integration — a documented no-op on Linux via IgnoreUnknown."
      keybarEntries={[]}
    >
      <Typography variant="h6" component="h1" gutterBottom>
        macOS Keychain globals
      </Typography>
      <Typography sx={{ color: 'text.secondary', maxWidth: 640, mb: 2 }}>
        gitid also writes a one-time, global <code>Host *</code> block so macOS stores key
        passphrases in the system Keychain and agents pick up new keys automatically. The leading{' '}
        <code>IgnoreUnknown UseKeychain</code> line means this block is silently ignored by
        OpenSSH on Linux — nothing to configure differently per platform.
      </Typography>
      <Paper variant="outlined" sx={{ p: 2, maxWidth: 480 }}>
        <Typography variant="subtitle2" sx={{ color: 'text.secondary', mb: 1 }}>
          Resulting global block
        </Typography>
        <Box component="pre" sx={{ m: 0, fontFamily: 'inherit', fontSize: 13 }}>
          {sshMacGlobalsBlockText}
        </Box>
      </Paper>
    </Shell>
  );
}

const route: RouteModule = {
  path: '/create-flow/macos-globals-block',
  element: <MacosGlobalsBlockScreen />,
  title: 'create-flow/macos-globals-block',
};

export default route;
