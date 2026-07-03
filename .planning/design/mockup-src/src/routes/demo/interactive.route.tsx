/**
 * The SPA's index (`/`) is the INTERACTIVE demo (design-review checkpoint
 * feedback): a keyboard-driven, stateful walkthrough of every surface and
 * workflow with dummy data — create/test/save an identity, browse the
 * live list with state flags, per-identity detail/doctor/fix/delete/clone,
 * global SSH/Git option reviews, health → fixer hand-off, help, and a
 * Ctrl+P palette that also opens each of the 50 static reference mockups.
 *
 * The static reference routes (what the capture gates assert against) are
 * untouched — this route only claims `/`, previously the internal
 * shell-demo page (now at /_shell/shell-demo).
 */

import DemoApp from '../../demo/DemoApp';
import type { RouteModule } from '../../App';

const route: RouteModule = {
  path: '/',
  element: <DemoApp />,
  title: 'demo/interactive',
};

export default route;
