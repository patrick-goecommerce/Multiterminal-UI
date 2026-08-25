<script lang="ts">
  import ThrowingGrid from './ThrowingGrid.svelte';
  export let cols = 1;
</script>

<svelte:boundary>
  <ThrowingGrid {cols} />
  {#snippet failed(error)}
    <div data-testid="failed">{(error as Error).message}</div>
  {/snippet}
</svelte:boundary>
<div data-testid="sibling">sibling still rendered</div>
