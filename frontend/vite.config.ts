import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';
import path from 'path';

// During Vite dev mode the Wails v3 runtime is not available because there
// is no running Go backend. We alias the URL to a local stub in dev mode only.
// For production builds the alias is removed and /wails/runtime.js is marked as
// external so Rollup keeps the import statement. Wails v3 serves the real
// runtime.js at that URL when the app runs inside the WebView2 window.
const wailsRuntimeStub = path.resolve(__dirname, 'src/lib/wails-runtime-stub.ts');

export default defineConfig(({ mode }) => {
  const isDev = mode !== 'production';
  return {
    plugins: [svelte()],
    resolve: {
      alias: isDev ? { '/wails/runtime.js': wailsRuntimeStub } : {},
    },
    build: {
      outDir: 'dist',
      emptyOutDir: true,
      rollupOptions: isDev ? {} : {
        external: ['/wails/runtime.js'],
      },
    },
  };
});
