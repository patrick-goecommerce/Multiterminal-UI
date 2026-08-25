<script lang="ts">
  // Reproduces the shape of the alpha.24 crash: a reactive statement that
  // throws while the component updates (Array(NaN) -> RangeError).
  export let cols = 1;
  $: fractions = Array(Math.max(1, Math.ceil(0 / cols))).fill(1);
</script>

<div data-testid="grid">{fractions.length}</div>
