<script lang="ts">
  import Button from '../ui/Button.svelte'
  import AIItemRow from './AIItemRow.svelte'
  import type { AIItemDTO, NodeDTO } from '$lib/api'
  import { disabledReason, sumBytes, type AIGroupId } from '$lib/aireport'
  import { formatBytes } from '$lib/format'
  import { selectAll } from '$lib/stores/selection.svelte'
  import { strings } from '$lib/strings'

  let { id, items }: { id: AIGroupId; items: AIItemDTO[] } = $props()

  const title = $derived(strings.ai.groups[id])
  const selectable = $derived(items.filter((it) => disabledReason(it) === '').map((it) => it.node as NodeDTO))
  const tone: Record<AIGroupId, string> = { safe: 'text-safe', caution: 'text-warn-text', keep: 'text-keep' }
</script>

<section aria-label={title} class="flex flex-col gap-2">
  <div class="flex flex-wrap items-center justify-between gap-2">
    <h3 class="m-0 flex items-baseline gap-2 text-sm font-semibold {tone[id]}">
      {title}
      <span class="text-xs font-normal text-muted tabular-nums">{strings.ai.groupMeta(items.length, formatBytes(sumBytes(items)))}</span>
    </h3>
    {#if id === 'safe' && items.length}
      <!-- Explicit user action: nothing is ever selected on its own. -->
      <Button disabled={selectable.length === 0} onclick={() => selectAll(selectable)}>{strings.ai.selectAllSafe}</Button>
    {/if}
  </div>
  {#if items.length}
    <ul class="m-0 flex list-none flex-col gap-2 p-0">
      {#each items as item (item.path)}
        <AIItemRow {item} />
      {/each}
    </ul>
  {:else}
    <p class="m-0 text-xs text-muted">{strings.ai.groupEmpty}</p>
  {/if}
</section>
