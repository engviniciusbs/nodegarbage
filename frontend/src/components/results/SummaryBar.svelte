<script lang="ts">
  import Panel from '../ui/Panel.svelte'
  import type { SummaryDTO } from '$lib/api'
  import { formatBytes } from '$lib/format'
  import { results, toggleCategory } from '$lib/stores/results.svelte'
  import { strings } from '$lib/strings'

  let { summary }: { summary: SummaryDTO } = $props()

  const label = (c: string) => (strings.category as Record<string, string>)[c] ?? c
</script>

<Panel class="space-y-3">
  <div class="flex flex-wrap items-baseline gap-x-6 gap-y-1">
    <p class="text-lg font-semibold text-primary">{strings.results.safeLine(formatBytes(summary.safeBytes))}</p>
    <p class="text-base text-primary">{strings.results.cautionLine(formatBytes(summary.cautionBytes))}</p>
    <p class="text-sm text-muted">{strings.results.keepLine(formatBytes(summary.keepBytes))}</p>
  </div>
  {#if summary.byCategory?.length}
    <div role="group" aria-label={strings.results.categories} class="flex flex-wrap gap-2">
      {#each summary.byCategory as c (c.category)}
        {@const pressed = results.categoryFilter === c.category}
        <button
          type="button"
          aria-pressed={pressed}
          onclick={() => toggleCategory(c.category)}
          class="cursor-pointer rounded-full border border-black/10 px-3 py-1 text-xs text-primary transition-colors hover:bg-black/5 {pressed
            ? 'bg-accent/15 border-accent font-medium'
            : ''}"
        >
          {label(c.category)} · <span class="tabular-nums">{formatBytes(c.bytes)}</span>
        </button>
      {/each}
    </div>
  {/if}
</Panel>
