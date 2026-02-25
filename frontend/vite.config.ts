import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';
import path from 'path';

// During `npm run build` (and Vite dev mode) the Wails v3 runtime is not
// available at /wails/runtime.js because there is no running Go backend.
// We alias the URL to a local stub that provides compatible no-op shims.
// At actual Wails runtime the Go server serves the real runtime.js at that URL,
// so the alias has no effect in the final Wails-served app.
const wailsRuntimeStub = path.resolve(__dirname, 'src/lib/wails-runtime-stub.ts');

export default defineConfig({
  plugins: [svelte()],
  resolve: {
    alias: {
      '/wails/runtime.js': wailsRuntimeStub,
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
});
