#!/usr/bin/env node
// verify-routes.mjs — build-time route-uniqueness + shape gate (review MEDIUM-9).
//
// `import.meta.glob({ eager: true })` in App.tsx auto-discovers route
// modules but does not, by itself, enforce shape or uniqueness — a
// malformed or colliding route module would otherwise only fail (or worse,
// silently misbehave) at runtime. This script performs the same checks
// statically, wired into `pnpm build` (`node scripts/verify-routes.mjs &&
// vite build`), so a bad route fails the build loudly, before any capture
// or deploy step runs.
//
// Checks per file under src/routes/**/*.route.tsx:
//   - has a `default` export
//   - the exported object literal has a `path:` property
//   - the exported object literal has a `title:` property
//   - no two files declare the same `path` string

import { readFileSync } from 'node:fs';
import { globSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const routesDir = path.join(__dirname, '..', 'src', 'routes');
const globPattern = path.join(routesDir, '**', '*.route.tsx').replace(/\\/g, '/');

function findRouteFiles() {
  // Node 20.11+ / 22+ ships fs.globSync; this repo's toolchain is pinned to
  // Node 22 (Volta). Kept dependency-free (no extra npm glob package).
  return globSync(globPattern);
}

function checkFile(filePath) {
  const source = readFileSync(filePath, 'utf8');
  const errors = [];

  if (!/export\s+default\s+/.test(source)) {
    errors.push('missing a `default` export');
  }
  if (!/path\s*:/.test(source)) {
    errors.push('missing a `path:` property on the exported route object');
  }
  if (!/title\s*:/.test(source)) {
    errors.push('missing a `title:` property on the exported route object');
  }

  // Extract the path: "..." literal (best-effort static check; the
  // authoritative uniqueness/shape check is App.tsx's runtime validator —
  // this script is the fail-fast build-time companion).
  const pathMatch = source.match(/path\s*:\s*['"]([^'"]+)['"]/);
  const routePath = pathMatch ? pathMatch[1] : null;

  return { filePath, errors, routePath };
}

function main() {
  const files = findRouteFiles();

  if (files.length === 0) {
    console.error('verify-routes: no route files found under src/routes/**/*.route.tsx');
    process.exit(1);
  }

  const results = files.map(checkFile);
  const seenPaths = new Map();
  let hasError = false;

  for (const result of results) {
    const relFile = path.relative(process.cwd(), result.filePath);

    for (const err of result.errors) {
      console.error(`verify-routes: FAIL ${relFile}: ${err}`);
      hasError = true;
    }

    if (result.routePath) {
      const existing = seenPaths.get(result.routePath);
      if (existing) {
        console.error(
          `verify-routes: FAIL duplicate route path "${result.routePath}" declared by ` +
            `both ${existing} and ${relFile}`,
        );
        hasError = true;
      } else {
        seenPaths.set(result.routePath, relFile);
      }
    } else if (result.errors.length === 0) {
      console.error(
        `verify-routes: FAIL ${relFile}: could not statically extract a \`path:\` string literal`,
      );
      hasError = true;
    }
  }

  if (hasError) {
    process.exit(1);
  }

  console.log(`verify-routes: OK — ${files.length} route(s), all unique and well-shaped.`);
}

main();
