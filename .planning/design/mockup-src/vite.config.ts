import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

// gitid design mockup — static SPA build config.
//
// base: './' is required so the built dist/index.html can be opened directly
// via file:// (no web server): Vite's default base: '/' emits root-absolute
// asset paths that cannot resolve against a file:// origin. See
// 02-RESEARCH.md "Pitfall 2".
export default defineConfig({
  base: './',
  plugins: [react()],
  build: {
    outDir: 'dist',
  },
});
