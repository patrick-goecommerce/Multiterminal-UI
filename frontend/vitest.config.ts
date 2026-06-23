import { defineConfig } from 'vitest/config';
import { svelte } from '@sveltejs/vite-plugin-svelte';
import { svelteTesting } from '@testing-library/svelte/vite';

export default defineConfig({
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
