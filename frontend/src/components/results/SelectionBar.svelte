<script lang="ts">
  import type { Snippet } from 'svelte'
  import { formatBytes } from '$lib/format'
  import { selection, selectedBytes } from '$lib/stores/selection.svelte'
  import { strings } from '$lib/strings'

  /** Task 21 passes the "Limpar" button here. */
  let { actions }: { actions?: Snippet } = $props()
</script>

<div class="glass flex items-center justify-between gap-3 rounded-lg px-4 py-2">
  <p role="status" class="text-sm font-medium text-primary tabular-nums">
    {strings.results.selected(selection.paths.size, formatBytes(selectedBytes()))}
  </p>
  {#if actions}<div class="flex gap-2">{@render actions()}</div>{/if}
</div>
