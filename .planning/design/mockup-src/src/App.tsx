import type { ReactElement } from 'react';
import { HashRouter, Route, Routes } from 'react-router-dom';

/**
 * The shape every `src/routes/**\/*.route.tsx` module MUST default-export.
 *
 * `title` carries the `<surface>/<screen>` breadcrumb screen-ID id (see
 * `shell/Header.tsx`) — both the HTML capture and the TUI e2e assert against
 * it, so it must be a real, non-empty string on every route.
 */
export interface RouteModule {
  path: string;
  element: ReactElement;
  title: string;
}

// Route auto-discovery (review MEDIUM-9 baseline): surfaces add
// `*.route.tsx` files under `src/routes/`; this glob finds them at build
// time with zero edits to this file. `eager: true` resolves all modules
// synchronously so validation below runs at module load, not lazily.
const routeModules = import.meta.glob<{ default: RouteModule }>(
  './routes/**/*.route.tsx',
  { eager: true },
);

function isValidRouteModule(mod: unknown): mod is { default: RouteModule } {
  if (typeof mod !== 'object' || mod === null || !('default' in mod)) {
    return false;
  }
  const candidate = (mod as { default?: Partial<RouteModule> }).default;
  return (
    typeof candidate === 'object' &&
    candidate !== null &&
    typeof candidate.path === 'string' &&
    candidate.path.length > 0 &&
    candidate.element != null &&
    typeof candidate.title === 'string' &&
    candidate.title.length > 0
  );
}

/**
 * Validate the discovered route set at module load (review MEDIUM-9):
 *   - every module's default export has a non-empty `path`, an `element`,
 *     and a non-empty `title` (screen-id)
 *   - no two modules declare the SAME `path`
 *
 * A bad or duplicate route export throws here, at build/module-load time —
 * never silently at capture time. `scripts/verify-routes.mjs` re-checks the
 * same invariants statically before `vite build` runs (belt-and-suspenders:
 * a static prebuild gate plus this runtime assertion).
 */
function buildValidatedRoutes(
  modules: Record<string, unknown>,
): RouteModule[] {
  const routes: RouteModule[] = [];
  const seenPaths = new Map<string, string>();

  for (const [filePath, mod] of Object.entries(modules)) {
    if (!isValidRouteModule(mod)) {
      throw new Error(
        `App.tsx route validation: "${filePath}" does not default-export a valid ` +
          `RouteModule (requires non-empty path, element, and non-empty title).`,
      );
    }
    const route = mod.default;
    const existingFile = seenPaths.get(route.path);
    if (existingFile) {
      throw new Error(
        `App.tsx route validation: duplicate path "${route.path}" declared by both ` +
          `"${existingFile}" and "${filePath}". Every route path must be unique.`,
      );
    }
    seenPaths.set(route.path, filePath);
    routes.push(route);
  }

  return routes;
}

const routes = buildValidatedRoutes(routeModules);

export function App() {
  return (
    <HashRouter>
      <Routes>
        {routes.map((route) => (
          <Route key={route.path} path={route.path} element={route.element} />
        ))}
      </Routes>
    </HashRouter>
  );
}

export default App;
