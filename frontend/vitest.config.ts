import { defineConfig } from 'vitest/config';
import { svelte } from '@sveltejs/vite-plugin-svelte';
import { svelteTesting } from '@testing-library/svelte/vite';
import path from 'path';

// The Wails v3 runtime is only served by the Go backend at runtime. Modules
// under test may import it transitively (e.g. session.ts → wailsjs/App), so —
// like vite.config.ts in dev — alias the runtime URL to the local no-op stub.
const wailsRuntimeStub = path.resolve(__dirname, 'src/lib/wails-runtime-stub.ts');

export default defineConfig({
  resolve: {
    alias: { '/wails/runtime.js': wailsRuntimeStub },
  },
  // svelteTesting() resolves Svelte 5's client build under jsdom (otherwise
  // `mount()` hits the server build) and registers auto-cleanup between tests.
  // componentApi: 4 keeps the legacy instance API (component.$on) working for
  // existing tests — scoped to the test build; the production build is unaffected.
  plugins: [
    svelte({ compilerOptions: { compatibility: { componentApi: 4 } } }),
    svelteTesting(),
  ],
  test: {
    environment: 'jsdom',
    include: ['src/**/*.test.ts'],
    setupFiles: ['./src/test-setup.ts'],
  },
});
