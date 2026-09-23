<script lang="ts">
  // Stands in for TerminalPane in PaneGrid tests: xterm.js needs a canvas that
  // jsdom does not have, and these tests are about where a pane goes.
  export let pane: { id: string };
  export let active = false;
  export let tabId = '';
  export let tabDir = '';
  export let paneIndex = 0;
</script>

<div data-testid="pane-{pane.id}" data-active={String(active)} data-tab={tabId} data-dir={tabDir} data-index={paneIndex}></div>
