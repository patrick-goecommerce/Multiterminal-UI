import '@xterm/xterm/css/xterm.css';
import { mount } from 'svelte';
import App from './App.svelte';

// Svelte 5 removed the `new Component({ target })` instantiation API — the app
// must be mounted via mount(). The legacy `new App()` form throws at startup,
// which left the window blank (no mount → no onMount → no LoadTabs).
const app = mount(App, {
  target: document.getElementById('app')!,
});

export default app;
