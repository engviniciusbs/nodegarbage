<script lang="ts">
  import type { Snippet } from 'svelte'
  import Button from '../ui/Button.svelte'
  import { formatBytes } from '$lib/format'
  import { selection, selectedBytes, clear } from '$lib/stores/selection.svelte'
  import { strings } from '$lib/strings'

  /** `actions` receives the primary "Limpar" button (CleanAction). */
  let { actions }: { actions?: Snippet } = $props()

  const count = $derived(selection.paths.size)
</script>

<!-- Fixed bar at the bottom of the content layer: it takes its own row, never covering the list. -->
<footer class="flex shrink-0 items-center justify-between gap-4 border-t border-stroke bg-footer px-6 py-2.5">
  <p role="status" class="m-0 text-[13px] text-muted tabular-nums">
    {#if count > 0}
      <strong class="font-semibold text-primary">{strings.results.selectedCount(count)}</strong>
      · {formatBytes(selectedBytes())}
    {:else}
      {strings.results.noneSelected}
    {/if}
  </p>
  <div class="flex gap-2">
    <Button disabled={count === 0} onclick={clear}>{strings.results.clearSelection}</Button>
    {#if actions}{@render actions()}{/if}
  </div>
</footer>
