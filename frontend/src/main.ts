import '@xterm/xterm/css/xterm.css';
import App from './App.svelte';

const app = new App({
  target: document.getElementById('app')!,
});

export default app;
