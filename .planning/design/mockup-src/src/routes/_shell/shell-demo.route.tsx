import { Box, Typography } from '@mui/material';
import Shell from '../../shell/Shell';
import type { RouteModule } from '../../App';

/**
 * The only route this plan ships. Proves the shared shell + breadcrumb
 * render correctly with placeholder body text — surfaces (02-04..02-10) add
 * the real seven-surface screens on top of this foundation.
 */
function ShellDemoScreen() {
  return (
    <Shell
      title="_shell/shell-demo"
      headerContext={{ identityCount: 0, health: 'healthy' }}
      statusMessage="Foundation shell — no surfaces registered yet."
      keybarEntries={[
        { key: '1', label: 'Identities' },
        { key: '2', label: 'Global SSH' },
        { key: '3', label: 'Global Git' },
        { key: '4', label: 'Health' },
        { key: '5', label: 'Fixer' },
      ]}
    >
      <Typography variant="h6" component="h1" gutterBottom>
        Shared shell — demo screen
      </Typography>
      <Typography sx={{ color: 'text.secondary', maxWidth: 640 }}>
        This is the empty foundation shell (header / body / status line /
        keybar). Every one of the seven product surfaces renders inside this
        same four-region frame; new surfaces register as{' '}
        <Box component="code" sx={{ px: 0.5 }}>
          src/routes/&lt;surface&gt;/&lt;screen&gt;.route.tsx
        </Box>{' '}
        files without editing App.tsx.
      </Typography>
    </Shell>
  );
}

const route: RouteModule = {
  path: '/',
  element: <ShellDemoScreen />,
  title: '_shell/shell-demo',
};

export default route;
