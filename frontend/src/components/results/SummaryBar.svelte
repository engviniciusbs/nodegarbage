<script lang="ts">
  import type { SummaryDTO } from '$lib/api'
  import { formatBytes } from '$lib/format'
  import { results, toggleCategory } from '$lib/stores/results.svelte'
  import { strings } from '$lib/strings'

  let { summary }: { summary: SummaryDTO } = $props()

  const label = (c: string) => (strings.category as Record<string, string>)[c] ?? c
  const figs = $derived([
    { id: 'safe', bytes: summary.safeBytes, text: strings.results.safeLabel, tone: 'text-safe text-[22px]' },
    { id: 'caution', bytes: summary.cautionBytes, text: strings.results.cautionLabel, tone: 'text-warn-text text-[22px]' },
    { id: 'keep', bytes: summary.keepBytes, text: strings.results.keepLabel, tone: 'text-keep text-[15px]' },
  ])
</script>

<section
  aria-label={strings.results.summaryLabel}
  class="mx-6 grid shrink-0 grid-cols-[auto_1fr] items-center gap-x-6 gap-y-1.5 rounded-md border border-stroke bg-card px-4 py-3.5"
>
  <div class="flex flex-wrap items-baseline gap-x-6 gap-y-1">
    {#each figs as f (f.id)}
      <p class="m-0 flex items-baseline gap-2" data-fig={f.id}>
        <strong class="font-display font-semibold tabular-nums {f.tone}">{formatBytes(f.bytes)}</strong>
        <span class="text-[13px] text-muted">{f.text}</span>
      </p>
    {/each}
  </div>
  {#if summary.byCategory?.length}
    <div role="group" aria-label={strings.results.categories} class="flex flex-wrap justify-end gap-1.5">
      {#each summary.byCategory as c (c.category)}
        {@const pressed = results.categoryFilter === c.category}
        <button
          type="button"
          aria-pressed={pressed}
          onclick={() => toggleCategory(c.category)}
          class="cursor-pointer rounded-full border px-2.5 py-1 text-xs tabular-nums transition-colors {pressed
            ? 'border-accent bg-accent text-on-accent'
            : 'border-stroke bg-chip text-primary hover:bg-field'}"
        >
          {label(c.category)} · {formatBytes(c.bytes)}
        </button>
      {/each}
    </div>
  {/if}
</section>
